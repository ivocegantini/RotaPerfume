package deploy_test

// BACKUP-01 (Lote 42, TestBrain): o que faltava da seção 7 da especificação e
// dos acréscimos posteriores a ela (par dump + imagens, --exigir-montagem,
// parciais, pre-restauracao-*, --sem-backup-motivo no seed). Usa os ensaios de
// backup01_backend_linux_test.go, mariadb_docker_test.go e
// tst21_seed_terminal_real_linux_test.go. Nada aqui fala com Docker, systemd ou
// banco de verdade, salvo TestBACKUP01_ContaBackup_Real (opt-in).
//
// Dono errado: no user namespace só o uid atual existe, então um arquivo "de
// outro dono" é simulado com o stubStat, que mente só para o caminho pedido. O
// que se testa é a decisão do script diante da resposta do stat, não o stat.

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tamanho bem acima do buffer de um pipe (64 KiB): quem fecha o pipe cedo (grep
// -q, head) leva SIGPIPE no gzip -dc e, sob pipefail, dá falso negativo.
const tamanhoSIGPIPE = 8 << 20

const stubStat = `#!/bin/bash
for ultimo; do :; done
if [[ "$*" == *"%u %a"* && -f "$FAKE/state/stat_alvo" ]]; then
  alvo=$(<"$FAKE/state/stat_alvo")
  # shellcheck disable=SC2053 # o alvo e um glob
  if [[ "$ultimo" == $alvo ]]; then cat "$FAKE/state/stat_resposta"; exit 0; fi
fi
PATH=${PATH#*:} exec stat "$@"
`

// gzip -c é a compressão do stream; -t e -dc seguem para o gzip real.
const stubGzipFalhaDepoisDeEscrever = `#!/bin/bash
if [[ "$1" == "-c" ]]; then head -c 200; exit 1; fi
PATH=${PATH#*:} exec gzip "$@"
`

const stubGzipDiscoCheio = `#!/bin/bash
if [[ "$1" == "-c" ]]; then cat >/dev/null; echo "gzip: stdout: No space left on device" >&2; exit 1; fi
PATH=${PATH#*:} exec gzip "$@"
`

const stubDateCarimbo = `#!/bin/bash
if [[ "$*" == "-u +%Y%m%d-%H%M%SZ" ]]; then cat "$FAKE/state/carimbo"; exit 0; fi
PATH=${PATH#*:} exec date "$@"
`

// Montado na primeira conferência, desmontado na segunda.
const stubMountpointSome = `#!/bin/bash
n=$(( $(cat "$FAKE/state/mountpoint_n" 2>/dev/null || echo 0) + 1 ))
echo "$n" >"$FAKE/state/mountpoint_n"
(( n < 2 ))
`

func (e *ensaioBackup) bin(nome, conteudo string) {
	e.t.Helper()
	escrever(e.t, filepath.Join(e.dir, "bin", nome), conteudo, 0o755)
}

// dockerCom troca trechos do stub de docker do BackBrain.
func (e *ensaioBackup) dockerCom(de, para string) {
	e.t.Helper()
	atual := e.ler("bin/docker")
	require.Contains(e.t, atual, de)
	e.bin("docker", strings.Replace(atual, de, para, 1))
}

func (e *ensaioBackup) conf(linhasConf ...string) {
	e.t.Helper()
	escrever(e.t, filepath.Join(e.dir, "etcrp/backup-banco.conf"), strings.Join(linhasConf, "\n")+"\n", 0o644)
}

func (e *ensaioBackup) rodarMontado(ponto, opcoes, script string, args ...string) (string, int) {
	e.t.Helper()
	cmd := e.comando(script, args...)
	cmd.Env = append(cmd.Env, "MONTAR="+ponto, "MONTAR_MODO="+opcoes)
	return saidaECodigo(e.t, cmd)
}

func saidaECodigo(t *testing.T, cmd *exec.Cmd) (string, int) {
	t.Helper()
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	ee, ok := err.(*exec.ExitError)
	require.True(t, ok, "%v", err)
	return string(out), ee.ExitCode()
}

// tresPares: três pares bons; com RETENCAO_DIARIOS=2, um deles sairia se a
// retenção rodasse.
func (e *ensaioBackup) tresPares() []string {
	e.t.Helper()
	e.par("20260101-033000Z", "")
	e.par("20260102-033000Z", "")
	e.par("20260103-033000Z", "")
	return e.arquivos()
}

func dumpDeTamanho(t *testing.T, n int) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("/*M!999999\\- enable the sandbox mode */ \n-- MariaDB dump\n")
	for _, tab := range tabelasVerificacao(13) {
		fmt.Fprintf(&b, "CREATE TABLE `%s` (\n  `id` bigint NOT NULL\n);\n", tab)
	}
	b.WriteString("INSERT INTO `usuarios` VALUES\n(1,'x');\n")
	base := b.Len() + len(marcadorDump)
	require.Greater(t, n, base+3)
	b.WriteString("--" + strings.Repeat("x", n-base-3) + "\n")
	b.WriteString(marcadorDump)
	require.Equal(t, n, b.Len())
	return b.String()
}

func dumpGrande() string {
	var b strings.Builder
	b.WriteString(strings.TrimSuffix(dumpBackupBom(), marcadorDump))
	linha := strings.Repeat("(1,'linha de preenchimento sintetico'),\n", 1000)
	for b.Len() < tamanhoSIGPIPE {
		b.WriteString(linha)
	}
	b.WriteString(marcadorDump)
	return b.String()
}

func tamanhoGzip(t *testing.T, texto string) int {
	t.Helper()
	cmd := exec.Command("gzip", "-c")
	cmd.Stdin = strings.NewReader(texto)
	out, err := cmd.Output()
	require.NoError(t, err)
	return len(out)
}

type entradaTar struct {
	nome, conteudo, link string
	tipo                 byte
}

func tarDe(t *testing.T, entradas ...entradaTar) string {
	t.Helper()
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	for _, x := range entradas {
		tipo := x.tipo
		if tipo == 0 {
			tipo = tar.TypeReg
		}
		require.NoError(t, w.WriteHeader(&tar.Header{Name: x.nome, Linkname: x.link, Mode: 0o644, Size: int64(len(x.conteudo)), Typeflag: tipo}))
		if x.conteudo != "" {
			_, err := w.Write([]byte(x.conteudo))
			require.NoError(t, err)
		}
	}
	require.NoError(t, w.Close())
	return b.String()
}

// parciais devolve só os nomes parcial-*.
func parciaisDe(nomes []string) []string {
	var out []string
	for _, n := range nomes {
		if strings.HasPrefix(n, "parcial-") {
			out = append(out, n)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// gzip e disco cheio
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_GzipFalha(t *testing.T) {
	for nome, stub := range map[string]string{"depois de escrever": stubGzipFalhaDepoisDeEscrever, "disco cheio": stubGzipDiscoCheio} {
		t.Run(nome, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia("RETENCAO_DIARIOS=2\n")
			antes := e.tresPares()
			e.bin("gzip", stub)
			out, rc := e.instalada("--agendado")
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, "o dump falhou")
			assert.Contains(t, out, "gzip=1")
			assert.Equal(t, antes, e.arquivos(), "nada novo e nada apagado: disco cheio nunca dispara apagamento")
			assert.NotContains(t, out, "retencao:")
			assert.NotContains(t, e.ler("log"), "--entrypoint tar")
		})
	}
}

// ENOSPC de verdade: tmpfs menor que o dump. O ponto de montagem some com a
// execução, então o que sobra para conferir é a saída e o disco do sistema.
func TestBACKUP01_TB_DiscoCheioReal(t *testing.T) {
	e := novoEnsaioBackup(t)
	ssd := filepath.Join(e.dir, "ssd")
	require.NoError(t, os.MkdirAll(ssd, 0o700))
	e.instalarCopia()
	e.conf("BACKUP_DIR="+ssd, "EXIGIR_MONTAGEM="+ssd)
	aleatorio := make([]byte, 4<<20)
	rand.New(rand.NewSource(42)).Read(aleatorio) //nolint:gosec // dado de teste
	e.state("dump", strings.TrimSuffix(dumpBackupBom(), marcadorDump)+"-- "+hex.EncodeToString(aleatorio)+"\n"+marcadorDump)

	out, rc := e.rodarMontado(ssd, "700,size=128k", copiaInstalada, "--agendado")
	assert.Equal(t, 1, rc, out)
	assert.Contains(t, out, "o dump falhou")
	assert.Contains(t, out, "gzip=1")
	assert.NotContains(t, out, "backup: ok")
	assert.NotContains(t, out, "retencao:")
	ents, err := os.ReadDir(ssd)
	require.NoError(t, err)
	assert.Empty(t, ents, "nada no disco do sistema")
}

// ---------------------------------------------------------------------------
// SIGPIPE com arquivo grande
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_ArquivoGrande(t *testing.T) {
	semMarcador := strings.TrimSuffix(dumpGrande(), marcadorDump)
	bloco := strings.Repeat("ab", 3<<20) // uma linha só, como um blob grande em --hex-blob
	casos := []struct {
		nome, dump string
		rc         int
		esperado   string
	}{
		{"dump grande e bom passa", dumpGrande(), 0, "backup: ok"},
		{"linha unica gigante passa", strings.Replace(dumpBackupBom(), marcadorDump, "(2,0x"+bloco+");\n"+marcadorDump, 1), 0, "backup: ok"},
		{"grande sem marcador reprova pelo motivo certo", semMarcador, 1, "nao tem o marcador"},
		{"grande com marcador so no comeco reprova", marcadorDump + semMarcador, 1, "nao tem o marcador"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia()
			e.state("dump", c.dump)
			out, rc := e.instalada("--agendado")
			assert.Equal(t, c.rc, rc, out)
			assert.Contains(t, out, c.esperado)
			assert.NotContains(t, out, "nao consegui ler o dump", "sem falso negativo por SIGPIPE")
			e.semTemporarios()
		})
	}

	t.Run("tar com muitas entradas no fluxo no status e na conferencia", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		entradas := make([]entradaTar, 0, 8000)
		for i := 0; i < 8000; i++ {
			entradas = append(entradas, entradaTar{nome: fmt.Sprintf("fotos/produto-%06d-com-nome-comprido.jpg", i), conteudo: "x"})
		}
		bruto := tarDe(t, entradas...)
		e.state("tar_bruto", bruto)
		e.dockerCom(`*"--entrypoint tar"*)`, `*"--entrypoint tar"*)
        if [[ -f "$S/tar_bruto" ]]; then cat "$S/tar_bruto"; exit 0; fi`)
		out, rc := e.instalada("--agendado")
		require.Equal(t, 0, rc, out)
		assert.Contains(t, out, "arquivos_imagens=8000")
		assert.Contains(t, out, "bytes_imagens=8000")

		escrever(t, filepath.Join(e.backups(), ".ultima-verificacao"), fmt.Sprintf("epoch=%d\n", time.Now().Unix()), 0o600)
		out, rc = e.instalada("--status")
		assert.Equal(t, 0, rc, out)
		assert.Contains(t, out, "Imagens: 8000 arquivo(s), 8000 bytes")

		p := filepath.Join(e.dir, "grande.tar.gz") // o chroot do ensaio só expõe o $FAKE
		escrever(t, p, comprimir(t, bruto), 0o600)
		out, rc = e.checkout("--conferir-imagens", p)
		assert.Equal(t, 0, rc, out)
		assert.Contains(t, out, "Arquivos: 8000\n")
	})

	t.Run("guarda do recriar aceita dump grande", func(t *testing.T) {
		g, dir := novaGuarda(t)
		escrever(t, filepath.Join(dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz"), comprimir(t, dumpGrande()), 0o600)
		r := g.rodar("--recriar", "--backup-dir", dir)
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "Backup recente conferido")
	})
}

// ---------------------------------------------------------------------------
// Fronteiras do piso e da queda (o caso "abaixo do piso" do ensaio antigo para
// antes, na contagem de tabelas)
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_PisoDoDump(t *testing.T) {
	casos := []struct {
		tamanho, rc int
		esperado    string
	}{
		{4095, 1, "dump descomprimido com 4095 bytes"},
		{4096, 0, "backup: ok"},
	}
	for _, c := range casos {
		t.Run(fmt.Sprint(c.tamanho), func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia()
			e.state("dump", dumpDeTamanho(t, c.tamanho))
			out, rc := e.instalada("--agendado")
			assert.Equal(t, c.rc, rc, out)
			assert.Contains(t, out, c.esperado)
		})
	}
	t.Run("aceitar-reducao nao dispensa o piso", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.state("dump", dumpDeTamanho(t, 4095))
		out, rc := e.instalada("--agora", "--aceitar-reducao")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "o piso e 4096")
	})
}

func TestBACKUP01_TB_FronteiraDaQueda(t *testing.T) {
	atual := tamanhoGzip(t, dumpBackupBom())
	casos := []struct {
		nome          string
		anterior, rc  int
		textoEsperado string
	}{
		{"exatamente 50 por cento passa", 2 * atual, 0, "backup: ok"},
		{"um byte abaixo de 50 por cento reprova", 2*atual + 1, 1, "menos de 50%"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia()
			e.par(carimboAntigoDump, "")
			escrever(t, filepath.Join(e.backups(), "rotaperfumes-"+carimboAntigoDump+".sql.gz"), strings.Repeat("x", c.anterior), 0o600)
			out, rc := e.instalada("--agendado")
			assert.Equal(t, c.rc, rc, out)
			assert.Contains(t, out, c.textoEsperado)
		})
	}
	t.Run("registro da reducao e 600", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.par(carimboAntigoDump, "")
		escrever(t, filepath.Join(e.backups(), "rotaperfumes-"+carimboAntigoDump+".sql.gz"), strings.Repeat("x", 2*atual+1), 0o600)
		out, rc := e.instalada("--agora", "--aceitar-reducao")
		require.Equal(t, 0, rc, out)
		st, err := os.Stat(filepath.Join(e.backups(), ".registro-reducao"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	})
}

// ---------------------------------------------------------------------------
// Cliente preso, trava, temporários e sinais
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_ClientePreso(t *testing.T) {
	e := novoEnsaioBackup(t)
	e.instalarCopia()
	// Como no Docker real: o nome ocupado faz o docker run falhar até o rm.
	e.state("preso", "")
	e.dockerCom(`  info|rm) exit 0 ;;`, `  info) exit 0 ;;
  rm) rm -f "$S/preso"; exit 0 ;;`)
	e.dockerCom(`  run)
`, `  run)
    if [[ -f "$S/preso" ]]; then echo 'docker: Error response from daemon: Conflict. The container name is already in use' >&2; exit 125; fi
`)
	out, rc := e.instalada("--agendado")
	require.Equal(t, 0, rc, out)
	log := linhas(e.ler("log"))
	require.NotEmpty(t, log)
	var primeiroRm, primeiroRun = -1, -1
	for i, l := range log {
		if primeiroRm < 0 && l == "docker rm -f -- rotaperfumes-backup-cliente rotaperfumes-backup-imagens" {
			primeiroRm = i
		}
		if primeiroRun < 0 && strings.HasPrefix(l, "docker run") {
			primeiroRun = i
		}
	}
	require.GreaterOrEqual(t, primeiroRm, 0)
	require.GreaterOrEqual(t, primeiroRun, 0)
	assert.Less(t, primeiroRm, primeiroRun, "remove o preso antes do primeiro docker run")
}

func TestBACKUP01_TB_TravaETemporarios(t *testing.T) {
	segurar := func(t *testing.T, e *ensaioBackup) func() {
		t.Helper()
		f, err := os.OpenFile(filepath.Join(e.backups(), ".trava"), os.O_CREATE|os.O_RDWR, 0o600)
		require.NoError(t, err)
		require.NoError(t, syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB))
		return func() { _ = f.Close() }
	}
	t.Run("trava ocupada nao mexe nos temporarios de quem a tem", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		soltar := segurar(t, e)
		defer soltar()
		tmp := filepath.Join(e.backups(), ".tmp-dump.emuso")
		escrever(t, tmp, "dump de outra execucao", 0o600)
		antes := e.arquivos()
		for _, modo := range []string{"--agendado", "--agora"} {
			out, rc := e.instalada(modo)
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, "outro backup em andamento")
			assert.NotContains(t, out, "backup: ok", "trava ocupada nunca e sucesso")
		}
		assert.FileExists(t, tmp)
		assert.Equal(t, antes, e.arquivos())
		assert.Empty(t, e.ler("log"), "nem o cliente de quem tem a trava")
	})
	t.Run("sobra de execucao morta sai; link e diretorio ficam", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		escrever(t, filepath.Join(e.backups(), ".tmp-dump.morto"), "sobra", 0o600)
		alvo := filepath.Join(e.dir, "alvo-tmp")
		escrever(t, alvo, "nao apagar", 0o600)
		link := filepath.Join(e.backups(), ".tmp-link")
		require.NoError(t, os.Symlink(alvo, link))
		sub := filepath.Join(e.backups(), ".tmp-dir")
		require.NoError(t, os.MkdirAll(sub, 0o700))
		out, rc := e.instalada("--agendado")
		require.Equal(t, 0, rc, out)
		assert.NoFileExists(t, filepath.Join(e.backups(), ".tmp-dump.morto"))
		assert.FileExists(t, alvo)
		_, err := os.Lstat(link)
		assert.NoError(t, err, "symlink fica")
		assert.DirExists(t, sub)
	})
	t.Run("trava e um symlink", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		alvo := filepath.Join(e.dir, "alvo-trava")
		require.NoError(t, os.Symlink(alvo, filepath.Join(e.backups(), ".trava")))
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "link simbolico")
		assert.NoFileExists(t, alvo, "nao cria nada pelo link")
		assert.Empty(t, e.ler("log"))
	})
}

func TestBACKUP01_TB_Sinais(t *testing.T) {
	casos := []struct {
		nome     string
		preso    string
		iniciado string
		sinal    syscall.Signal
		rc       int
	}{
		{"SIGTERM no dump", "dump_preso", "dump_iniciado", syscall.SIGTERM, 143},
		{"SIGINT no dump", "dump_preso", "dump_iniciado", syscall.SIGINT, 130},
		{"SIGTERM no tar", "tar_preso", "tar_iniciado", syscall.SIGTERM, 143},
		{"SIGINT no tar", "tar_preso", "tar_iniciado", syscall.SIGINT, 130},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia("RETENCAO_DIARIOS=2\n")
			antes := e.tresPares()
			e.dockerCom(`*"--entrypoint tar"*)`, `*"--entrypoint tar"*)
        if [[ -f "$S/tar_preso" ]]; then touch "$S/tar_iniciado"; exec sleep 60; fi`)
			e.state(c.preso, "")
			cmd := e.comando(copiaInstalada, "--agendado")
			var saida bytes.Buffer
			cmd.Stdout, cmd.Stderr = &saida, &saida
			require.NoError(t, cmd.Start())
			require.Eventually(t, func() bool {
				_, err := os.Stat(filepath.Join(e.dir, "state", c.iniciado))
				return err == nil
			}, 20*time.Second, 50*time.Millisecond)
			inicio := time.Now()
			require.NoError(t, cmd.Process.Signal(c.sinal))
			err := cmd.Wait()
			var ee *exec.ExitError
			require.ErrorAs(t, err, &ee)
			assert.Equal(t, c.rc, ee.ExitCode(), saida.String())
			assert.Less(t, time.Since(inicio), 15*time.Second, "tratado na hora")
			e.semTemporarios()
			assert.Equal(t, antes, e.arquivos(), "sem parcial nem retenção")
			assert.Contains(t, strings.TrimSpace(e.ler("log")), "docker rm -f -- rotaperfumes-backup-cliente rotaperfumes-backup-imagens")
			assert.Equal(t, 2, strings.Count(e.ler("log"), "docker rm -f --"), "início e trap")
		})
	}
}

// ---------------------------------------------------------------------------
// Segredos
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_MensagemDoClienteNaoVaza(t *testing.T) {
	dado := "SENTINELA_DADO_CLIENTE"
	casos := []struct {
		nome, stderr string
		esperado     string
		rcDump       string
	}{
		{"erro com codigo", "mariadb-dump: Got error: 1045: \"Access denied for '" + dado + "' password=" + senhaBackupEnsaio + "\"\n", "error: 1045", "1"},
		{"erro sem codigo", "algo citou " + dado + " e password=" + senhaBackupEnsaio + "\n", "o dump falhou ()", "1"},
		{"linha do docker cortada", "docker: " + strings.Repeat("x", 200) + dado + senhaBackupEnsaio + "\n", "docker: xxx", "125"},
		{"aviso do cliente em dump bom", "Warning: " + dado + " " + senhaBackupEnsaio + "\n", "backup: ok", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia()
			e.state("dump_stderr", c.stderr)
			if c.rcDump != "" {
				e.state("dump_rc", c.rcDump)
			}
			out, _ := e.instalada("--agendado")
			assert.Contains(t, out, c.esperado)
			assert.NotContains(t, out, dado)
			assert.NotContains(t, out, "Access denied")
			e.semSenha(out)
			e.semTemporarios()
		})
	}
}

// Senha fora de tudo o que o fluxo inteiro gera; argv dos clientes sem as flags
// proibidas, sem TTY e sem montar credencial em quem não precisa.
func TestBACKUP01_TB_SegredoEmTodoOFluxo(t *testing.T) {
	e := novoEnsaioBackup(t)
	out, rc := e.checkout("--instalar", "--backup-dir", e.backups())
	require.Equal(t, 0, rc, out+e.ler("unit.log"))
	time.Sleep(1100 * time.Millisecond)
	agora, rc := e.instalada("--agora")
	require.Equal(t, 0, rc, agora)
	status, _ := e.instalada("--status")
	for _, texto := range []string{out, agora, status, e.ler("unit.log"), e.ler("log"),
		e.ler("etcrp/backup-banco.conf"), e.ler("systemd/rotaperfumes-backup-banco.service"), e.ler("systemd/rotaperfumes-backup-banco.timer")} {
		assert.NotContains(t, texto, senhaBackupEnsaio)
	}

	var dump, cliImagens int
	for _, l := range linhas(e.ler("log")) {
		if !strings.HasPrefix(l, "docker run") {
			continue
		}
		campos := " " + l + " "
		for _, proibido := range []string{" -e ", " --env ", " --env-file ", " -t ", " -it ", " -ti ", " --tty ", " --privileged ", " --pid ", " -v ", " --volume ", "MYSQL_PWD", "--network host"} {
			assert.NotContains(t, campos, proibido, l)
		}
		assert.Contains(t, l, "--cap-drop ALL", l)
		assert.Contains(t, l, "--read-only", l)
		if strings.Contains(l, "--entrypoint mariadb-dump") {
			dump++
			continue
		}
		cliImagens++
		assert.Contains(t, l, "--network none", l)
		assert.NotContains(t, l, "cliente.cnf", "imagens sem credencial")
		assert.NotContains(t, l, "ca.pem")
	}
	assert.Equal(t, 2, dump, "um dump por execução")
	assert.GreaterOrEqual(t, cliImagens, 4, "du e tar por execução")
}

func TestBACKUP01_TB_SemSetXNosScripts(t *testing.T) {
	for _, s := range []string{"backup-banco.sh", "verificar-backup.sh", "mariadb-docker.sh", "seed-producao.sh"} {
		t.Run(s, func(t *testing.T) {
			codigo := codigoSemComentarios(lerRaiz(t, "deploy", s))
			assert.NotRegexp(t, `(?m)^\s*set\s+(-[a-zA-Z]*x|-o\s+xtrace)`, codigo)
			assert.NotRegexp(t, `(?m)^#!.*\s-[a-zA-Z]*x`, lerRaiz(t, "deploy", s))
			assert.NotContains(t, codigo, "BASH_XTRACEFD")
		})
	}
}

// ---------------------------------------------------------------------------
// Retenção
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_RetencaoInvalidaNoConf(t *testing.T) {
	for _, chave := range []string{"RETENCAO_DIARIOS", "RETENCAO_MANUAIS"} {
		for i, v := range []string{"0", "1", "-1", "abc", "", "2x", "1.5", "99999", "$(touch /tmp/rp)"} {
			t.Run(fmt.Sprintf("%s caso %d", chave, i), func(t *testing.T) { // o nome vira o TempDir: sem = nem virgula
				e := novoEnsaioBackup(t)
				e.instalarCopia(chave + "=" + v + "\n")
				antes := e.tresPares()
				out, rc := e.instalada("--agendado")
				assert.Equal(t, 1, rc, out)
				assert.Contains(t, out, chave+" invalida")
				assert.Equal(t, antes, e.arquivos())
				assert.Empty(t, e.ler("log"), "recusa antes do docker")
			})
		}
	}
	for _, op := range []string{"--retencao-diarios", "--retencao-manuais"} {
		for _, v := range []string{"0", "1", "-3", "abc", "2x", "99999"} {
			t.Run(op+" "+v, func(t *testing.T) {
				e := novoEnsaioBackup(t)
				out, rc := e.checkout("--instalar", op, v)
				assert.Equal(t, 2, rc, out)
				assert.Contains(t, out, "retencao invalida")
				assert.NoFileExists(t, filepath.Join(e.dir, "sbin/rotaperfumes-backup-banco"))
			})
		}
	}
}

// Zero à esquerda é aceito na validação, então precisa valer como decimal na
// conta da retenção, e não como octal (ou erro, com 08 e 09).
func TestBACKUP01_TB_RetencaoComZeroAEsquerda(t *testing.T) {
	casos := []struct {
		valor           string
		paresAntigos    int
		paresDepois     int
		removidoEsperad string
	}{
		{"08", 3, 4, ""},
		{"010", 9, 10, ""},
		{"010", 10, 10, "rotaperfumes-20260101-033000Z.sql.gz"},
	}
	for _, c := range casos {
		t.Run(fmt.Sprintf("%s com %d pares", c.valor, c.paresAntigos), func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia("RETENCAO_DIARIOS=" + c.valor + "\n")
			for i := 1; i <= c.paresAntigos; i++ {
				e.par(fmt.Sprintf("202601%02d-033000Z", i), "")
			}
			out, rc := e.instalada("--agendado")
			require.Equal(t, 0, rc, out)
			assert.Equal(t, c.paresDepois, contarDiarias(e.arquivos()))
			if c.removidoEsperad != "" {
				assert.NotContains(t, e.arquivos(), c.removidoEsperad)
			}
		})
	}
}

func TestBACKUP01_TB_RetencaoSoPorContagem(t *testing.T) {
	t.Run("servidor desligado por anos nao perde nada", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		antigos := []string{"20200101-033000Z", "20200102-033000Z", "20200103-033000Z"}
		velho := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		for _, c := range antigos {
			e.par(c, "")
			for _, n := range []string{"rotaperfumes-" + c + ".sql.gz", "rotaperfumes-imagens-" + c + ".tar.gz"} {
				require.NoError(t, os.Chtimes(filepath.Join(e.backups(), n), velho, velho))
			}
		}
		out, rc := e.instalada("--agendado")
		require.Equal(t, 0, rc, out)
		for _, c := range antigos {
			assert.Contains(t, e.arquivos(), "rotaperfumes-"+c+".sql.gz")
		}
		assert.NotContains(t, out, "retencao:")
	})
	t.Run("mantem os N mais novos pelo carimbo, nao pelo mtime", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia("RETENCAO_DIARIOS=2\n")
		e.tresPares()
		// O carimbo mais antigo com mtime novo, o mais novo com mtime antigo.
		velho := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(filepath.Join(e.backups(), "rotaperfumes-20260103-033000Z.sql.gz"), velho, velho))
		out, rc := e.instalada("--agendado")
		require.Equal(t, 0, rc, out)
		nomes := e.arquivos()
		assert.Contains(t, nomes, "rotaperfumes-20260103-033000Z.sql.gz")
		assert.NotContains(t, nomes, "rotaperfumes-20260101-033000Z.sql.gz")
		assert.NotContains(t, nomes, "rotaperfumes-20260102-033000Z.sql.gz")
		assert.Equal(t, 2, contarDiarias(nomes), "o novo e o mais recente")
	})
	t.Run("agora remove so os manuais antigos e os dois lados do par", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia("RETENCAO_DIARIOS=2\nRETENCAO_MANUAIS=2\n")
		diarios := e.tresPares()
		e.par("20200101-040000Z", "-manual")
		e.par("20200102-040000Z", "-manual")
		out, rc := e.instalada("--agora")
		require.Equal(t, 0, rc, out)
		nomes := e.arquivos()
		assert.Subset(t, nomes, diarios, "as diárias ficam, mesmo acima de RETENCAO_DIARIOS")
		assert.NotContains(t, nomes, "rotaperfumes-20200101-040000Z-manual.sql.gz")
		assert.NotContains(t, nomes, "rotaperfumes-imagens-20200101-040000Z-manual.tar.gz")
		assert.Contains(t, nomes, "rotaperfumes-20200102-040000Z-manual.sql.gz")
	})
}

// Lado de par inválido: não conta na retenção e não é apagado.
func TestBACKUP01_TB_RetencaoLadosDoPar(t *testing.T) {
	casos := []struct {
		nome     string
		preparar func(e *ensaioBackup) []string
	}{
		{"imagens orfas", func(e *ensaioBackup) []string {
			n := "rotaperfumes-imagens-20190101-033000Z.tar.gz"
			escrever(e.t, filepath.Join(e.backups(), n), "x", 0o600)
			return []string{n}
		}},
		{"dump com imagens em symlink", func(e *ensaioBackup) []string {
			alvo := filepath.Join(e.dir, "alvo-imagens")
			escrever(e.t, alvo, "x", 0o600)
			d, i := "rotaperfumes-20190101-033000Z.sql.gz", "rotaperfumes-imagens-20190101-033000Z.tar.gz"
			escrever(e.t, filepath.Join(e.backups(), d), "x", 0o600)
			require.NoError(e.t, os.Symlink(alvo, filepath.Join(e.backups(), i)))
			return []string{d, i}
		}},
		{"dump com imagens em diretorio", func(e *ensaioBackup) []string {
			d, i := "rotaperfumes-20190101-033000Z.sql.gz", "rotaperfumes-imagens-20190101-033000Z.tar.gz"
			escrever(e.t, filepath.Join(e.backups(), d), "x", 0o600)
			require.NoError(e.t, os.MkdirAll(filepath.Join(e.backups(), i), 0o700))
			return []string{d, i}
		}},
		{"series trocadas", func(e *ensaioBackup) []string {
			d, i := "rotaperfumes-20190101-033000Z-manual.sql.gz", "rotaperfumes-imagens-20190101-033000Z.tar.gz"
			escrever(e.t, filepath.Join(e.backups(), d), "x", 0o600)
			escrever(e.t, filepath.Join(e.backups(), i), "x", 0o600)
			return []string{d, i}
		}},
		{"nomes parecidos com os dois lados", func(e *ensaioBackup) []string {
			var nomes []string
			for _, c := range []string{"2019", "20190101-033000", "20190101-033000Z-manual2"} {
				d, i := "rotaperfumes-"+c+".sql.gz", "rotaperfumes-imagens-"+c+".tar.gz"
				escrever(e.t, filepath.Join(e.backups(), d), "x", 0o600)
				escrever(e.t, filepath.Join(e.backups(), i), "x", 0o600)
				nomes = append(nomes, d, i)
			}
			return nomes
		}},
		{"nomes quase validos", func(e *ensaioBackup) []string {
			nomes := []string{"rotaperfumes-20190101-033000.sql.gz", "rotaperfumes-20190101-033000Z-manual2.sql.gz", "rotaperfumes-20190101-033000Z.sql.gz.bak",
				"xrotaperfumes-20190101-033000Z.sql.gz", "rotaperfumes-imagens-20190101-033000Z.tar", "rotaperfumes-2019010-033000Z.sql.gz"}
			for _, n := range nomes {
				escrever(e.t, filepath.Join(e.backups(), n), "x", 0o600)
			}
			return nomes
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia("RETENCAO_DIARIOS=2\nRETENCAO_MANUAIS=2\n")
			imunes := c.preparar(e)
			e.par("20260101-033000Z", "")
			e.par("20260102-033000Z", "")
			out, rc := e.instalada("--agendado")
			require.Equal(t, 0, rc, out)
			nomes := e.arquivos()
			assert.Subset(t, nomes, imunes)
			assert.NotContains(t, nomes, "rotaperfumes-20260101-033000Z.sql.gz", "a contagem ignora os inválidos: 2 pares ficam")
			assert.Contains(t, nomes, "rotaperfumes-20260102-033000Z.sql.gz")
		})
	}
}

// ---------------------------------------------------------------------------
// Par dump + imagens
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_TarPerigosoNoFluxo(t *testing.T) {
	casos := []struct {
		nome, bruto, esperado string
	}{
		{"pai", tarDe(t, entradaTar{nome: "../fora.jpg", conteudo: "x"}), "caminho absoluto ou '..'"},
		{"pai no meio", tarDe(t, entradaTar{nome: "a/../../fora.jpg", conteudo: "x"}), "caminho absoluto ou '..'"},
		{"absoluto", tarDe(t, entradaTar{nome: "/etc/cron.d/x", conteudo: "x"}), "caminho absoluto ou '..'"},
		{"symlink", tarDe(t, entradaTar{nome: "l", link: "/etc/passwd", tipo: tar.TypeSymlink}), "nao sao arquivo nem diretorio"},
		{"hardlink", tarDe(t, entradaTar{nome: "a", conteudo: "x"}, entradaTar{nome: "h", link: "a", tipo: tar.TypeLink}), "nao sao arquivo nem diretorio"},
		{"dispositivo", tarDe(t, entradaTar{nome: "d", tipo: tar.TypeChar}), "nao sao arquivo nem diretorio"},
		{"fifo", tarDe(t, entradaTar{nome: "f", tipo: tar.TypeFifo}), "nao sao arquivo nem diretorio"},
		{"lixo", strings.Repeat("isto nao e um tar valido\n", 100), "listagem"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia("RETENCAO_DIARIOS=2\n")
			antes := e.tresPares()
			e.state("tar_bruto", c.bruto)
			e.dockerCom(`*"--entrypoint tar"*)`, `*"--entrypoint tar"*)
        if [[ -f "$S/tar_bruto" ]]; then cat "$S/tar_bruto"; exit 0; fi`)
			out, rc := e.instalada("--agendado")
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, c.esperado)
			assert.Contains(t, out, "retencao nao executada")
			assert.NotContains(t, out, "/etc/", "nomes do tar fora da saída")
			depois := e.arquivos()
			assert.Subset(t, depois, antes)
			require.Len(t, parciaisDe(depois), 1, "%v", depois)
			assert.Len(t, depois, len(antes)+1, "sem o tar reprovado publicado")
			e.semTemporarios()
		})
	}
}

// Arquivos de verdade no volume, que o tar real empacota.
func TestBACKUP01_TB_VolumeReal(t *testing.T) {
	t.Run("fifo vira parcial", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		require.NoError(t, syscall.Mkfifo(filepath.Join(e.dir, "state/volume/fila"), 0o600))
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "nao sao arquivo nem diretorio")
		assert.Len(t, parciaisDe(e.arquivos()), 1)
	})
	t.Run("nomes estranhos nao vazam nem enganam a contagem", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		for _, n := range []string{"a..b.jpg", "..oculto", "com espaco.jpg", "quebra\n/etc/passwd", "acento-çã.jpg", "-traco.jpg"} {
			e.state("volume/"+n, "12345")
		}
		out, rc := e.instalada("--agendado")
		require.Equal(t, 0, rc, out)
		assert.Contains(t, out, "arquivos_imagens=7") // os 6 mais a foto-1.jpg
		assert.NotContains(t, out, "oculto")
		assert.NotContains(t, out, "/etc/passwd")
	})
}

func TestBACKUP01_TB_ConferirImagensTipos(t *testing.T) {
	precisaBash(t)
	ferramentaObrigatoria(t, "tar")
	ferramentaObrigatoria(t, "gzip")
	script := caminhoRaiz(t, "deploy", "backup-banco.sh")
	dir := t.TempDir()
	rodar := func(arq string) (string, int) {
		cmd := exec.Command("bash", script, "--conferir-imagens", arq)
		cmd.Env = ambienteLimpo()
		return saidaECodigo(t, cmd)
	}
	casos := []struct {
		nome    string
		tar     string
		rc      int
		trecho  string
		arquivo string
	}{
		{"dois pontos sozinho", tarDe(t, entradaTar{nome: "..", tipo: tar.TypeDir}), 1, "caminho absoluto ou '..'", ""},
		{"ponto barra pai", tarDe(t, entradaTar{nome: "./../x", conteudo: "x"}), 1, "caminho absoluto ou '..'", ""},
		{"pai no fim", tarDe(t, entradaTar{nome: "a/..", tipo: tar.TypeDir}), 1, "caminho absoluto ou '..'", ""},
		{"hardlink", tarDe(t, entradaTar{nome: "a", conteudo: "x"}, entradaTar{nome: "h", link: "a", tipo: tar.TypeLink}), 1, "nao sao arquivo nem diretorio", ""},
		{"dispositivo de bloco", tarDe(t, entradaTar{nome: "b", tipo: tar.TypeBlock}), 1, "nao sao arquivo nem diretorio", ""},
		{"fifo", tarDe(t, entradaTar{nome: "f", tipo: tar.TypeFifo}), 1, "nao sao arquivo nem diretorio", ""},
		{"symlink relativo", tarDe(t, entradaTar{nome: "l", link: "foto.jpg", tipo: tar.TypeSymlink}), 1, "nao sao arquivo nem diretorio", ""},
		{"dois pontos no nome nao e pai", tarDe(t, entradaTar{nome: "a..b.jpg", conteudo: "12"}, entradaTar{nome: "..oculto", conteudo: "3"}), 0, "Arquivos: 2\nBytes: 3\nAPROVADO\n", ""},
		{"so diretorio", tarDe(t, entradaTar{nome: "./", tipo: tar.TypeDir}), 0, "Arquivos: 0\nBytes: 0\nAPROVADO\n", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p := filepath.Join(dir, strings.ReplaceAll(c.nome, " ", "-")+".tar.gz")
			escrever(t, p, comprimir(t, c.tar), 0o600)
			out, rc := rodar(p)
			assert.Equal(t, c.rc, rc, out)
			assert.Contains(t, out, c.trecho)
		})
	}
	t.Run("diretorio e arquivo inexistente", func(t *testing.T) {
		for _, p := range []string{dir, filepath.Join(dir, "nao-existe")} {
			out, rc := rodar(p)
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, "nao e um arquivo regular")
		}
	})
	t.Run("conferir nao aceita opcoes de instalacao", func(t *testing.T) {
		for _, extra := range [][]string{{"--backup-dir", "/x"}, {"--retencao-diarios", "5"}, {"--exigir-montagem", "/x"}, {"--sem-exigir-montagem"}, {"--aceitar-reducao"}, {"--agora"}} {
			cmd := exec.Command("bash", append([]string{script, "--conferir-imagens", "/tmp/x"}, extra...)...)
			cmd.Env = ambienteLimpo()
			out, rc := saidaECodigo(t, cmd)
			assert.Equal(t, 2, rc, "%v: %s", extra, out)
		}
		cmd := exec.Command("bash", script, "--conferir-imagens")
		cmd.Env = ambienteLimpo()
		out, rc := saidaECodigo(t, cmd)
		assert.Equal(t, 2, rc, out)
	})
}

// ---------------------------------------------------------------------------
// Parciais
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_Parciais(t *testing.T) {
	t.Run("parcial traz o dump validado", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.state("sem_volume", "")
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		ps := parciaisDe(e.arquivos())
		require.Len(t, ps, 1)
		saida, err := exec.Command("gzip", "-dc", filepath.Join(e.backups(), ps[0])).Output()
		require.NoError(t, err)
		assert.Equal(t, dumpBackupBom(), string(saida))
		assert.NotContains(t, e.ler("log"), "docker volume create", "não cria o volume")
		assert.NotContains(t, e.ler("log"), "--entrypoint tar")
	})
	t.Run("parcial da serie manual", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.state("tar_falha", "")
		out, rc := e.instalada("--agora")
		assert.Equal(t, 1, rc, out)
		ps := parciaisDe(e.arquivos())
		require.Len(t, ps, 1)
		assert.Regexp(t, `^parcial-rotaperfumes-\d{8}-\d{6}Z-manual\.sql\.gz$`, ps[0])
	})
	t.Run("dump invalido nao vira parcial", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.state("sem_volume", "")
		e.state("dump", strings.TrimSuffix(dumpBackupBom(), marcadorDump))
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		assert.Empty(t, parciaisDe(e.arquivos()))
	})
	t.Run("parcial nao e referencia da queda", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		escrever(t, filepath.Join(e.backups(), "parcial-rotaperfumes-20991231-235959Z.sql.gz"), strings.Repeat("x", 500000), 0o600)
		out, rc := e.instalada("--agendado")
		require.Equal(t, 0, rc, out)
	})
	t.Run("parcial nao satisfaz o status", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		escrever(t, filepath.Join(e.backups(), "parcial-rotaperfumes-"+carimboAgora(0)+".sql.gz"), comprimir(t, dumpBackupBom()), 0o600)
		out, rc := e.instalada("--status")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "nenhum backup bom")
	})
	t.Run("limite nao apaga symlink nem nome parecido", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.state("sem_volume", "")
		alvo := filepath.Join(e.dir, "alvo-parcial")
		escrever(t, alvo, "x", 0o600)
		link := "parcial-rotaperfumes-20100101-033000Z.sql.gz"
		require.NoError(t, os.Symlink(alvo, filepath.Join(e.backups(), link)))
		parecido := "parcial-rotaperfumes-2010.sql.gz"
		escrever(t, filepath.Join(e.backups(), parecido), "x", 0o600)
		for _, c := range []string{"20200101-033000Z", "20200102-033000Z", "20200103-033000Z"} {
			escrever(t, filepath.Join(e.backups(), "parcial-rotaperfumes-"+c+".sql.gz"), "x", 0o600)
		}
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		nomes := e.arquivos()
		assert.Contains(t, nomes, link)
		assert.Contains(t, nomes, parecido)
		assert.FileExists(t, alvo)
		assert.NotContains(t, nomes, "parcial-rotaperfumes-20200101-033000Z.sql.gz", "o mais antigo real sai")
		assert.Contains(t, nomes, "parcial-rotaperfumes-20200102-033000Z.sql.gz")
	})
}

// ---------------------------------------------------------------------------
// Publicação: nunca sobrescreve
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_PublicacaoNaoSobrescreve(t *testing.T) {
	const carimbo = "20261106-033000Z"
	dump, imagens := "rotaperfumes-"+carimbo+".sql.gz", "rotaperfumes-imagens-"+carimbo+".tar.gz"
	casos := []struct {
		nome       string
		existentes []string
		esperado   string
		ficam      []string
	}{
		{"imagens ja existem", []string{imagens}, "nao consegui publicar", []string{imagens}},
		{"dump ja existe", []string{dump}, "nao consegui publicar", []string{dump}},
		{"os dois ja existem", []string{dump, imagens}, "nao consegui publicar", []string{dump, imagens}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia("RETENCAO_DIARIOS=2\n")
			e.tresPares()
			e.bin("date", stubDateCarimbo)
			e.state("carimbo", carimbo+"\n")
			for _, n := range c.existentes {
				escrever(t, filepath.Join(e.backups(), n), "ORIGINAL", 0o600)
			}
			antes := e.arquivos()
			out, rc := e.instalada("--agendado")
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, c.esperado)
			assert.NotContains(t, out, "retencao:")
			assert.Equal(t, antes, e.arquivos(), "nada novo e nada apagado")
			for _, n := range c.ficam {
				assert.Equal(t, "ORIGINAL", e.ler("backups/"+n))
			}
			e.semTemporarios()
		})
	}
}

// ---------------------------------------------------------------------------
// --exigir-montagem
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_MontagemNoMeio(t *testing.T) {
	t.Run("disco some antes de publicar", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.conf("BACKUP_DIR="+e.backups(), "EXIGIR_MONTAGEM="+e.dir, "RETENCAO_DIARIOS=2")
		antes := e.tresPares()
		e.bin("mountpoint", stubMountpointSome)
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "etapa=publicacao")
		assert.Contains(t, out, "nao esta montado")
		assert.Contains(t, e.ler("log"), "--entrypoint tar", "chegou até a publicação")
		assert.Equal(t, antes, e.arquivos(), "nada publicado e sem retenção")
		e.semTemporarios()
	})
	t.Run("prefixo parecido nao conta como dentro", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		ssd := filepath.Join(e.dir, "ssd")
		require.NoError(t, os.MkdirAll(ssd, 0o700))
		out, rc := e.checkout("--instalar", "--backup-dir", ssd+"2/backups", "--exigir-montagem", ssd)
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "precisa ficar dentro de")
		assert.NoDirExists(t, ssd+"2")
	})
	t.Run("desmontado nao cria trava nem le credencial", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		ssd := filepath.Join(e.dir, "ssd")
		require.NoError(t, os.MkdirAll(ssd, 0o700))
		e.instalarCopia()
		e.conf("BACKUP_DIR="+ssd, "EXIGIR_MONTAGEM="+ssd)
		require.NoError(t, os.Remove(filepath.Join(e.dir, "root/rotaperfumes-mariadb/backup-cliente/rotaperfumes_backup.cnf")))
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "nao esta montado", "a montagem vem antes de tudo")
		assert.NoFileExists(t, filepath.Join(ssd, ".trava"))
	})
}

// ---------------------------------------------------------------------------
// Conf: lido sem executar
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_ConfRecusado(t *testing.T) {
	casos := []struct {
		nome, conteudo, esperado string
	}{
		{"comando", "BACKUP_DIR=$(touch /tmp/rp-conf)\n", "BACKUP_DIR invalido"},
		{"relativo", "BACKUP_DIR=relativo\n", "BACKUP_DIR invalido"},
		{"com ponto ponto", "BACKUP_DIR=/a/../b\n", "BACKUP_DIR invalido"},
		{"raiz", "BACKUP_DIR=/\n", "BACKUP_DIR invalido"},
		{"montagem raiz", "EXIGIR_MONTAGEM=/\n", "EXIGIR_MONTAGEM invalido"},
		{"montagem com barra", "EXIGIR_MONTAGEM=/mnt/\n", "EXIGIR_MONTAGEM invalido"},
		{"linha desconhecida", "PATH=/tmp\n", "linha nao reconhecida"},
		{"export", "export BACKUP_DIR=/x\n", "linha nao reconhecida"},
		{"comentario", "# nada\n", "linha nao reconhecida"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia()
			escrever(t, filepath.Join(e.dir, "etcrp/backup-banco.conf"), c.conteudo, 0o644)
			out, rc := e.instalada("--agendado")
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, c.esperado)
			assert.Empty(t, e.ler("log"))
		})
	}
	t.Run("conf nunca e executado", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		marca := filepath.Join(e.dir, "conf-executado")
		escrever(t, filepath.Join(e.dir, "etcrp/backup-banco.conf"), "touch "+marca+"\nBACKUP_DIR=$(touch "+marca+")\n", 0o644)
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		assert.NoFileExists(t, marca)
	})
	t.Run("modo gravavel", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		p := filepath.Join(e.dir, "etcrp/backup-banco.conf")
		require.NoError(t, os.Chmod(p, 0o666))
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "sem escrita para grupo/outros")
	})
	t.Run("symlink", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		p := filepath.Join(e.dir, "etcrp/backup-banco.conf")
		alvo := filepath.Join(e.dir, "etcrp/alvo.conf")
		require.NoError(t, os.Rename(p, alvo))
		require.NoError(t, os.Symlink("/etc/rotaperfumes/alvo.conf", p))
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "precisa ser arquivo regular")
	})
}

// ---------------------------------------------------------------------------
// Dono errado
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_DonoErrado(t *testing.T) {
	cnf := "/root/rotaperfumes-mariadb/backup-cliente/rotaperfumes_backup.cnf"
	casos := []struct {
		nome, resposta string
		alvo           func(e *ensaioBackup) string
		esperado       string
		chegaNoDocker  bool
	}{
		{"destino", "1000 700\n", func(e *ensaioBackup) string { return e.backups() }, "precisa ser do root com modo 700", false},
		{"ancestral do destino", "1000 755\n", func(e *ensaioBackup) string { return e.dir }, "precisa ser do root (dono atual: 1000)", false},
		{"cnf", "1000 400\n", func(*ensaioBackup) string { return cnf }, "precisa ser do root com modo 400", false},
		{"diretorio do cnf", "1000 700\n", func(*ensaioBackup) string { return "/root/rotaperfumes-mariadb/backup-cliente" }, "precisa ser do root com modo 700", false},
		{"CA", "1000 644\n", func(*ensaioBackup) string { return "/etc/mysql/ssl/ca.pem" }, "precisa ser do root sem escrita", false},
		{"conf", "1000 644\n", func(*ensaioBackup) string { return "/etc/rotaperfumes/backup-banco.conf" }, "precisa ser do root sem escrita", false},
		{"copia instalada", "1000 700\n", func(*ensaioBackup) string { return copiaInstalada }, "precisa ser do root com modo 700", false},
		{"dump no temporario", "1000 600\n", func(e *ensaioBackup) string { return e.backups() + "/.tmp-dump.*" }, "precisa ser do root com modo 600", true},
		{"imagens no temporario", "1000 600\n", func(e *ensaioBackup) string { return e.backups() + "/.tmp-imagens.*" }, "precisa ser do root com modo 600", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia("RETENCAO_DIARIOS=2\n")
			antes := e.tresPares()
			e.bin("stat", stubStat)
			e.state("stat_alvo", c.alvo(e))
			e.state("stat_resposta", c.resposta)
			out, rc := e.instalada("--agendado")
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, c.esperado)
			assert.Equal(t, antes, e.arquivos(), "nada publicado e sem retenção")
			e.semTemporarios()
			if !c.chegaNoDocker {
				assert.Empty(t, e.ler("log"), "recusa antes do docker")
			}
		})
	}
	t.Run("instalar recusa destino de outro dono sem copiar nada", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.bin("stat", stubStat)
		e.state("stat_alvo", e.backups())
		e.state("stat_resposta", "1000 700\n")
		out, rc := e.checkout("--instalar", "--backup-dir", e.backups())
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "precisa ser do root com modo 700")
		assert.NoFileExists(t, filepath.Join(e.dir, "sbin/rotaperfumes-backup-banco"))
		assert.NoFileExists(t, filepath.Join(e.dir, "systemd/rotaperfumes-backup-banco.service"))
	})
}

func TestBACKUP01_TB_DestinoDisjunto(t *testing.T) {
	e := novoEnsaioBackup(t)
	for _, d := range []string{"/root", "/root/rotaperfumes-mariadb", "/root/rotaperfumes-ca/b", "/etc/mysql/ssl", "/etc/mysql/ssl/b", "/etc/mysql",
		"/var/lib/rotaperfumes/mariadb", "/var/lib/rotaperfumes/mariadb/b", "/var/lib/rotaperfumes", "/var/lib", "/var"} {
		t.Run(d, func(t *testing.T) {
			out, rc := e.checkout("--instalar", "--backup-dir", d)
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, "disjunto")
			assert.NoFileExists(t, filepath.Join(e.dir, "sbin/rotaperfumes-backup-banco"))
		})
	}
}

// ---------------------------------------------------------------------------
// --status
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_StatusExtras(t *testing.T) {
	bom := func(t *testing.T) *ensaioBackup {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.par(carimboAgora(0), "")
		escrever(t, filepath.Join(e.backups(), ".ultima-verificacao"), fmt.Sprintf("epoch=%d\n", time.Now().Unix()), 0o600)
		return e
	}
	soImagens := func(e *ensaioBackup) string {
		m, err := filepath.Glob(filepath.Join(e.backups(), "rotaperfumes-imagens-*.tar.gz"))
		require.NoError(e.t, err)
		require.Len(e.t, m, 1)
		return m[0]
	}
	soDump := func(e *ensaioBackup) string {
		m, err := filepath.Glob(filepath.Join(e.backups(), "rotaperfumes-2*.sql.gz"))
		require.NoError(e.t, err)
		require.Len(e.t, m, 1)
		return m[0]
	}
	casos := []struct {
		nome     string
		preparar func(e *ensaioBackup)
		esperado string
	}{
		{"result vazio", func(e *ensaioBackup) { e.state("result", "\n") }, "Result=desconhecido"},
		{"mtime futuro", func(e *ensaioBackup) {
			futuro := time.Now().Add(2 * time.Hour)
			require.NoError(e.t, os.Chtimes(soDump(e), futuro, futuro))
		}, "data futura"},
		{"tar com caminho absoluto", func(e *ensaioBackup) {
			escrever(e.t, soImagens(e), comprimir(e.t, tarDe(e.t, entradaTar{nome: "/etc/x", conteudo: "x"})), 0o600)
		}, "caminho absoluto"},
		{"tar com link", func(e *ensaioBackup) {
			escrever(e.t, soImagens(e), comprimir(e.t, tarDe(e.t, entradaTar{nome: "l", link: "/etc/passwd", tipo: tar.TypeSymlink})), 0o600)
		}, "nao sao arquivo nem diretorio"},
		{"tar corrompido", func(e *ensaioBackup) { escrever(e.t, soImagens(e), "nao e gzip", 0o600) }, "gzip invalido"},
		{"imagens em symlink", func(e *ensaioBackup) {
			p := soImagens(e)
			alvo := filepath.Join(e.dir, "alvo-img")
			require.NoError(e.t, os.Rename(p, alvo))
			require.NoError(e.t, os.Symlink(alvo, p))
		}, "nenhum backup bom"},
		{"controle sem problema", func(e *ensaioBackup) {}, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := bom(t)
			c.preparar(e)
			out, rc := e.instalada("--status")
			if c.esperado == "" {
				assert.Equal(t, 0, rc, out)
				return
			}
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, c.esperado)
			assert.Contains(t, out, "STATUS: ")
			assert.NotContains(t, out, "STATUS: ok")
		})
	}
	t.Run("sem registro de verificacao: avisa e nao falha", func(t *testing.T) {
		e := bom(t)
		require.NoError(t, os.Remove(filepath.Join(e.backups(), ".ultima-verificacao")))
		out, rc := e.instalada("--status")
		assert.Equal(t, 0, rc, out)
		assert.Contains(t, out, "nenhuma verificacao registrada")
	})
	t.Run("status nao grava nem trava", func(t *testing.T) {
		e := bom(t)
		antes := fotoArvore(t, e.backups())
		out, rc := e.instalada("--status")
		require.Equal(t, 0, rc, out)
		assert.Equal(t, antes, fotoArvore(t, e.backups()))
		assert.NotContains(t, e.ler("log"), "docker", "status não usa Docker")
	})
}

// ---------------------------------------------------------------------------
// Units
// ---------------------------------------------------------------------------

func TestBACKUP01_TB_UnitsNaoApontamParaOCheckout(t *testing.T) {
	e := novoEnsaioBackup(t)
	out, rc := e.checkout("--instalar", "--backup-dir", e.backups())
	require.Equal(t, 0, rc, out+e.ler("unit.log"))
	for _, nome := range []string{"rotaperfumes-backup-banco.service", "rotaperfumes-backup-banco.timer"} {
		u := e.ler("systemd/" + nome)
		assert.NotContains(t, u, raiz(t), nome)
		assert.NotContains(t, u, "deploy/backup-banco.sh --", nome)
		for _, m := range regexp.MustCompile(`(?m)^Exec[A-Za-z]*=(\S+)`).FindAllStringSubmatch(u, -1) {
			assert.Equal(t, copiaInstalada, m[1], nome)
		}
	}
	svc := e.ler("systemd/rotaperfumes-backup-banco.service")
	require.Len(t, regexp.MustCompile(`(?m)^ReadWritePaths=`).FindAllString(svc, -1), 1)
	assert.Regexp(t, `(?m)^ExecStart=`+regexp.QuoteMeta(copiaInstalada)+` --agendado$`, svc)
	log := e.ler("log")
	assert.Less(t, strings.Index(log, "systemctl daemon-reload"), strings.Index(log, "systemctl start --no-block"))
}

// ---------------------------------------------------------------------------
// Guarda do --recriar
// ---------------------------------------------------------------------------

// fotoArvore: caminho -> tipo, modo e sha256. Ignora o que o stub ou o runner
// escrevem por conta própria.
func fotoArvore(t *testing.T, base string, ignorar ...string) map[string]string {
	t.Helper()
	foto := map[string]string{}
	require.NoError(t, filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(base, p)
		for _, ig := range ignorar {
			if rel == ig {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		v := info.Mode().String()
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			alvo, err := os.Readlink(p)
			if err != nil {
				return err
			}
			v += " -> " + alvo
		case info.Mode().IsRegular():
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			v += fmt.Sprintf(" %x", sha256.Sum256(b))
		}
		foto[rel] = v
		return nil
	}))
	return foto
}

// Recusa da guarda: nada mudou em lugar nenhum, e o docker só foi consultado.
func semNenhumaMudanca(t *testing.T, e *ensaio, antes map[string]string) {
	t.Helper()
	assert.Equal(t, antes, fotoArvore(t, e.dir, "log", "state", ".raiz"), "nada muda antes da guarda")
	log := e.log()
	for _, proibido := range []string{"docker rm", "docker build", "docker pull", "docker run", "docker start", "docker stop", "docker network create", "docker exec", "iptables-restore", "systemctl"} {
		assert.NotContains(t, log, proibido)
	}
}

func TestBACKUP01_TB_GuardaRecusaSemNenhumaMudanca(t *testing.T) {
	novo := func(t *testing.T, dir, nome string) string { return dumpRecente(t, dir, nome) }
	agora := func() string { return "rotaperfumes-" + carimboAgora(0) + ".sql.gz" }
	casos := []struct {
		nome     string
		preparar func(t *testing.T, e *ensaio, dir string)
		esperado string
	}{
		{"sem backup", func(*testing.T, *ensaio, string) {}, "nenhum backup"},
		{"antigo", func(t *testing.T, _ *ensaio, dir string) {
			p := novo(t, dir, "rotaperfumes-"+carimboAgora(-2*time.Hour)+".sql.gz")
			v := time.Now().Add(-2 * time.Hour)
			require.NoError(t, os.Chtimes(p, v, v))
		}, "tem mais de 60 min"},
		{"carimbo novo e mtime antigo", func(t *testing.T, _ *ensaio, dir string) {
			p := novo(t, dir, agora())
			v := time.Now().Add(-2 * time.Hour)
			require.NoError(t, os.Chtimes(p, v, v))
		}, "tem mais de 60 min"},
		{"carimbo novo e mtime futuro", func(t *testing.T, _ *ensaio, dir string) {
			p := novo(t, dir, agora())
			v := time.Now().Add(2 * time.Hour)
			require.NoError(t, os.Chtimes(p, v, v))
		}, "data futura"},
		{"61 minutos", func(t *testing.T, _ *ensaio, dir string) {
			p := novo(t, dir, "rotaperfumes-"+carimboAgora(-61*time.Minute)+".sql.gz")
			v := time.Now().Add(-61 * time.Minute)
			require.NoError(t, os.Chtimes(p, v, v))
		}, "tem mais de 60 min"},
		{"carimbo invalido", func(t *testing.T, _ *ensaio, dir string) {
			novo(t, dir, "rotaperfumes-20261399-250000Z.sql.gz")
		}, "carimbo invalido"},
		{"diretorio com nome de backup", func(t *testing.T, _ *ensaio, dir string) {
			require.NoError(t, os.MkdirAll(filepath.Join(dir, agora()), 0o700))
		}, "nao e arquivo regular"},
		{"o mais novo corrompido nao recua para o anterior", func(t *testing.T, _ *ensaio, dir string) {
			novo(t, dir, "rotaperfumes-"+carimboAgora(-5*time.Minute)+".sql.gz")
			escrever(t, filepath.Join(dir, agora()), "nao e gzip", 0o600)
		}, "corrompido"},
		{"o mais novo em symlink nao recua para o anterior", func(t *testing.T, _ *ensaio, dir string) {
			novo(t, dir, "rotaperfumes-"+carimboAgora(-5*time.Minute)+".sql.gz")
			alvo := novo(t, filepath.Dir(dir), "alvo.sql.gz")
			require.NoError(t, os.Symlink(alvo, filepath.Join(dir, agora())))
		}, "nao e arquivo regular"},
		{"truncado", func(t *testing.T, _ *ensaio, dir string) {
			z := comprimir(t, dumpBackupBom())
			escrever(t, filepath.Join(dir, agora()), z[:len(z)/2], 0o600)
		}, "corrompido"},
		{"vazio", func(t *testing.T, _ *ensaio, dir string) {
			escrever(t, filepath.Join(dir, agora()), comprimir(t, ""), 0o600)
		}, "marcador"},
		{"marcador so no meio", func(t *testing.T, _ *ensaio, dir string) {
			escrever(t, filepath.Join(dir, agora()), comprimir(t, marcadorDump+"(1,'x');\n"), 0o600)
		}, "marcador"},
		{"so imagens", func(t *testing.T, _ *ensaio, dir string) {
			escrever(t, filepath.Join(dir, "rotaperfumes-imagens-"+carimboAgora(0)+".tar.gz"), comprimir(t, dumpBackupBom()), 0o600)
		}, "nenhum backup"},
		{"parcial novo com bom antigo", func(t *testing.T, _ *ensaio, dir string) {
			p := novo(t, dir, "rotaperfumes-"+carimboAgora(-2*time.Hour)+".sql.gz")
			v := time.Now().Add(-2 * time.Hour)
			require.NoError(t, os.Chtimes(p, v, v))
			novo(t, dir, "parcial-rotaperfumes-"+carimboAgora(0)+".sql.gz")
		}, "tem mais de 60 min"},
		{"pre-restauracao novo com bom antigo", func(t *testing.T, _ *ensaio, dir string) {
			p := novo(t, dir, "rotaperfumes-"+carimboAgora(-2*time.Hour)+".sql.gz")
			v := time.Now().Add(-2 * time.Hour)
			require.NoError(t, os.Chtimes(p, v, v))
			novo(t, dir, "pre-restauracao-"+carimboAgora(0)+".sql.gz")
		}, "tem mais de 60 min"},
		{"dump de outro dono", func(t *testing.T, e *ensaio, dir string) {
			p := novo(t, dir, agora())
			escrever(t, filepath.Join(e.dir, "bin", "stat"), stubStat, 0o755)
			e.setState("stat_alvo", p)
			e.setState("stat_resposta", "1000 600\n")
		}, "precisa ser do root com modo 600"},
		{"diretorio de outro dono", func(t *testing.T, e *ensaio, dir string) {
			novo(t, dir, agora())
			escrever(t, filepath.Join(e.dir, "bin", "stat"), stubStat, 0o755)
			e.setState("stat_alvo", dir)
			e.setState("stat_resposta", "1000 700\n")
		}, "precisa ser do root com modo 700"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e, dir := novaGuarda(t)
			c.preparar(t, e, dir)
			antes := fotoArvore(t, e.dir, "log", "state", ".raiz")
			r := e.rodar("--recriar", "--backup-dir", dir)
			require.Error(t, r.err, r.out)
			assert.Contains(t, r.out, c.esperado)
			semNenhumaMudanca(t, e, antes)
		})
	}
	t.Run("diretorio em symlink", func(t *testing.T) {
		e, dir := novaGuarda(t)
		dumpRecente(t, dir, agora())
		link := filepath.Join(e.dir, "backups-link")
		require.NoError(t, os.Symlink(dir, link))
		antes := fotoArvore(t, e.dir, "log", "state", ".raiz")
		r := e.rodar("--recriar", "--backup-dir", link)
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "nenhum backup")
		semNenhumaMudanca(t, e, antes)
	})
	t.Run("ancestral gravavel", func(t *testing.T) {
		e, dir := novaGuarda(t)
		dumpRecente(t, dir, agora())
		require.NoError(t, os.Chmod(e.dir, 0o777))
		t.Cleanup(func() { _ = os.Chmod(e.dir, 0o700) })
		r := e.rodar("--recriar", "--backup-dir", dir)
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "escrita para grupo/outros")
		assert.NotContains(t, e.log(), "docker rm")
	})
	t.Run("container parado continua exigindo dump", func(t *testing.T) {
		e, dir := novaGuarda(t)
		e.setState("rodando", "false\n")
		antes := fotoArvore(t, e.dir, "log", "state", ".raiz")
		r := e.rodar("--recriar", "--backup-dir", dir)
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "nenhum backup")
		semNenhumaMudanca(t, e, antes)
	})
}

func TestBACKUP01_TB_GuardaEscolheOBackupCerto(t *testing.T) {
	registro := func(e *ensaio) string { return e.arq(filepath.Join(segredos, "recriar.registro")) }
	casos := []struct {
		nome   string
		extras func(t *testing.T, dir string)
	}{
		{"parcial mais novo", func(t *testing.T, dir string) {
			dumpRecente(t, dir, "parcial-rotaperfumes-"+carimboAgora(time.Second)+".sql.gz")
		}},
		{"pre-restauracao mais novo", func(t *testing.T, dir string) {
			dumpRecente(t, dir, "pre-restauracao-"+carimboAgora(time.Second)+".sql.gz")
		}},
		{"nome parecido no fim da ordem", func(t *testing.T, dir string) {
			escrever(t, filepath.Join(dir, "rotaperfumes-zzz.sql.gz"), "lixo", 0o600)
		}},
		{"imagens com carimbo futuro nao contam", func(t *testing.T, dir string) {
			escrever(t, filepath.Join(dir, "rotaperfumes-imagens-20991231-235959Z.tar.gz"), "lixo", 0o600)
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e, dir := novaGuarda(t)
			p := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(-time.Minute)+".sql.gz")
			c.extras(t, dir)
			r := e.rodar("--recriar", "--backup-dir", dir)
			require.NoError(t, r.err, r.out)
			assert.Contains(t, registro(e), "backup="+p+" ")
			assert.Contains(t, registro(e), "motivo=nenhum")
		})
	}
	t.Run("59 minutos passa", func(t *testing.T) {
		e, dir := novaGuarda(t)
		p := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(-59*time.Minute)+".sql.gz")
		v := time.Now().Add(-59 * time.Minute)
		require.NoError(t, os.Chtimes(p, v, v))
		r := e.rodar("--recriar", "--backup-dir", dir)
		require.NoError(t, r.err, r.out)
	})
	t.Run("o manual mais novo vale sobre o diario antigo", func(t *testing.T) {
		e, dir := novaGuarda(t)
		d := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(-10*time.Minute)+".sql.gz")
		v := time.Now().Add(-10 * time.Minute)
		require.NoError(t, os.Chtimes(d, v, v))
		m := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(0)+"-manual.sql.gz")
		r := e.rodar("--recriar", "--backup-dir", dir)
		require.NoError(t, r.err, r.out)
		assert.Contains(t, registro(e), "backup="+m+" ")
	})
	t.Run("janela com zero a esquerda vale como decimal", func(t *testing.T) {
		e, dir := novaGuarda(t)
		p := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(-9*time.Minute)+".sql.gz")
		v := time.Now().Add(-9 * time.Minute)
		require.NoError(t, os.Chtimes(p, v, v))
		r := e.rodar("--recriar", "--backup-dir", dir, "--backup-janela-min", "010")
		require.NoError(t, r.err, r.out)
	})
}

func TestBACKUP01_TB_MotivoDoRecriar(t *testing.T) {
	const msgTamanho = "--sem-backup-motivo precisa de 10 a 300 caracteres imprimiveis"
	recusas := []struct {
		nome     string
		args     []string
		esperado string
	}{
		{"sem recriar", []string{"--sem-backup-motivo", "motivo com tamanho bom"}, "--sem-backup-motivo so com --recriar"},
		{"nove caracteres", []string{"--recriar", "--sem-backup-motivo", strings.Repeat("x", 9)}, msgTamanho},
		{"301 caracteres", []string{"--recriar", "--sem-backup-motivo", strings.Repeat("x", 301)}, msgTamanho},
		{"quebra de linha", []string{"--recriar", "--sem-backup-motivo", "motivo com\nquebra de linha"}, msgTamanho},
		{"tabulacao", []string{"--recriar", "--sem-backup-motivo", "motivo com\ttabulacao aqui"}, msgTamanho},
		{"escape de terminal", []string{"--recriar", "--sem-backup-motivo", "motivo \x1b[31mcom escape"}, msgTamanho},
		{"vazio", []string{"--recriar", "--sem-backup-motivo", ""}, "--sem-backup-motivo"},
		{"janela zero", []string{"--recriar", "--backup-janela-min", "0"}, "--backup-janela-min precisa ser"},
		{"janela grande", []string{"--recriar", "--backup-janela-min", "1441"}, "--backup-janela-min precisa ser"},
		{"janela texto", []string{"--recriar", "--backup-janela-min", "abc"}, "--backup-janela-min precisa ser"},
		{"janela negativa", []string{"--recriar", "--backup-janela-min", "-5"}, "--backup-janela-min precisa ser"},
		{"dir relativo", []string{"--recriar", "--backup-dir", "rel/x"}, "--backup-dir invalido"},
		{"dir com ponto ponto", []string{"--recriar", "--backup-dir", "/a/../b"}, "--backup-dir invalido"},
		{"dir com espaco", []string{"--recriar", "--backup-dir", "/a b"}, "--backup-dir invalido"},
	}
	for _, c := range recusas {
		t.Run(c.nome, func(t *testing.T) {
			e, _ := novaGuarda(t)
			antes := fotoArvore(t, e.dir, "log", "state", ".raiz")
			r := e.rodar(c.args...)
			require.Error(t, r.err, r.out)
			assert.Contains(t, r.out, c.esperado)
			semNenhumaMudanca(t, e, antes)
		})
	}
	aceitos := []struct{ nome, motivo string }{
		{"dez caracteres", strings.Repeat("x", 10)},
		{"300 caracteres", strings.Repeat("y", 300)},
		{"literal sem expansao", "motivo %s $(touch /tmp/rp) `id` %n"},
	}
	for _, c := range aceitos {
		t.Run(c.nome, func(t *testing.T) {
			e, dir := novaGuarda(t)
			// Backup ruim no diretório: o motivo é o que permite seguir.
			escrever(t, filepath.Join(dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz"), "nao e gzip", 0o600)
			r := e.rodar("--recriar", "--backup-dir", dir, "--sem-backup-motivo", c.motivo)
			require.NoError(t, r.err, r.out)
			reg := e.arq(filepath.Join(segredos, "recriar.registro"))
			assert.Contains(t, reg, "backup=nenhum motivo="+c.motivo+" imagem_base=")
			assert.Equal(t, 1, strings.Count(reg, "\n"), "uma linha por --recriar")
			assert.Contains(t, r.out, "AVISO: --sem-backup-motivo")
		})
	}
	t.Run("registros se acumulam", func(t *testing.T) {
		e, dir := novaGuarda(t)
		require.NoError(t, e.rodar("--recriar", "--backup-dir", dir, "--sem-backup-motivo", "primeiro motivo aqui").err)
		require.NoError(t, e.rodar("--recriar", "--backup-dir", dir, "--sem-backup-motivo", "segundo motivo aqui").err)
		assert.Equal(t, 2, strings.Count(e.arq(filepath.Join(segredos, "recriar.registro")), "\n"))
	})
	t.Run("sem conseguir registrar nao remove o container", func(t *testing.T) {
		e, dir := novaGuarda(t)
		require.NoError(t, os.MkdirAll(filepath.Join(e.dir, segredos, "recriar.registro"), 0o700))
		r := e.rodar("--recriar", "--backup-dir", dir, "--sem-backup-motivo", "motivo com tamanho bom")
		require.Error(t, r.err, r.out)
		assert.NotContains(t, e.log(), "docker rm")
	})
	t.Run("container parado segue com o motivo", func(t *testing.T) {
		e, dir := novaGuarda(t)
		e.setState("rodando", "false\n")
		escrever(t, filepath.Join(e.dir, "bin", "docker"), strings.Replace(stubDocker,
			`rm) rm -f "$S/criado"; exit 0 ;;`, `rm) rm -f "$S/criado" "$S/rodando"; exit 0 ;;`, 1), 0o755)
		r := e.rodar("--recriar", "--backup-dir", dir, "--sem-backup-motivo", "container parado, sem dump novo")
		require.NoError(t, r.err, r.out)
		assert.Contains(t, e.log(), "docker rm -f rotaperfumes-mariadb")
	})
}

// ---------------------------------------------------------------------------
// Seed --forcar e --sem-backup-motivo
// ---------------------------------------------------------------------------

const registroSeed = "root/rotaperfumes-mariadb/seed-forcar.registro"

// stubDockerRegistroSeed anota a primeira chamada ao docker que já vê o registro.
const stubDockerRegistroSeed = `#!/bin/bash
n=$(( $(cat "$FAKE/calls/n" 2>/dev/null || echo 0) + 1 ))
if [[ ! -f "$FAKE/state/primeira_com_registro" && -f /root/rotaperfumes-mariadb/seed-forcar.registro ]]; then
  echo "$n" >"$FAKE/state/primeira_com_registro"
fi
exec "$FAKE/bin/docker-tty" "$@"
`

func novoSeedPreenchido(t *testing.T) (*ensaioSeed, string) {
	t.Helper()
	e := novoEnsaioTTY(t)
	e.state("linhas", "5\n")
	dir := filepath.Join(e.dir, "backups")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	return e, dir
}

func TestBACKUP01_TB_SeedGuardaRecusas(t *testing.T) {
	agora := func() string { return "rotaperfumes-" + carimboAgora(0) + ".sql.gz" }
	casos := []struct {
		nome     string
		preparar func(t *testing.T, e *ensaioSeed, dir string)
		esperado string
	}{
		{"antigo", func(t *testing.T, _ *ensaioSeed, dir string) {
			p := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(-2*time.Hour)+".sql.gz")
			v := time.Now().Add(-2 * time.Hour)
			require.NoError(t, os.Chtimes(p, v, v))
		}, "tem mais de 60 min"},
		{"data futura", func(t *testing.T, _ *ensaioSeed, dir string) {
			dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(2*time.Hour)+".sql.gz")
		}, "data futura"},
		{"modo 644", func(t *testing.T, _ *ensaioSeed, dir string) {
			require.NoError(t, os.Chmod(dumpRecente(t, dir, agora()), 0o644))
		}, "modo 600"},
		{"symlink", func(t *testing.T, _ *ensaioSeed, dir string) {
			alvo := dumpRecente(t, filepath.Dir(dir), "alvo.sql.gz")
			require.NoError(t, os.Symlink(alvo, filepath.Join(dir, agora())))
		}, "nao e arquivo regular"},
		{"corrompido", func(t *testing.T, _ *ensaioSeed, dir string) {
			escrever(t, filepath.Join(dir, agora()), "nao e gzip", 0o600)
		}, "corrompido"},
		{"sem marcador", func(t *testing.T, _ *ensaioSeed, dir string) {
			escrever(t, filepath.Join(dir, agora()), comprimir(t, "CREATE TABLE x;\n"), 0o600)
		}, "marcador"},
		{"parcial", func(t *testing.T, _ *ensaioSeed, dir string) {
			dumpRecente(t, dir, "parcial-rotaperfumes-"+carimboAgora(0)+".sql.gz")
		}, "nenhum backup"},
		{"pre-restauracao", func(t *testing.T, _ *ensaioSeed, dir string) {
			dumpRecente(t, dir, "pre-restauracao-"+carimboAgora(0)+".sql.gz")
		}, "nenhum backup"},
		{"diretorio 755", func(t *testing.T, _ *ensaioSeed, dir string) {
			dumpRecente(t, dir, agora())
			require.NoError(t, os.Chmod(dir, 0o755))
		}, "modo 700"},
		{"dono errado", func(t *testing.T, e *ensaioSeed, dir string) {
			p := dumpRecente(t, dir, agora())
			escrever(t, filepath.Join(e.dir, "bin", "stat"), stubStat, 0o755)
			e.state("stat_alvo", p)
			e.state("stat_resposta", "1000 600\n")
		}, "precisa ser do root com modo 600"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e, dir := novoSeedPreenchido(t)
			c.preparar(t, e, dir)
			s := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-dir", dir)
			out, err := s.terminar()
			require.Error(t, err, out)
			assert.Contains(t, out, c.esperado)
			assert.NotContains(t, out, promptNomeDoBanco, "recusa antes de perguntar")
			assert.NotContains(t, out, "=== [3/7]")
			e.semSQLDeEscrita(t)
			assert.NoFileExists(t, filepath.Join(e.dir, registroSeed))
		})
	}
}

func TestBACKUP01_TB_SeedMotivo(t *testing.T) {
	const msgTamanho = "--sem-backup-motivo precisa de 10 a 300 caracteres imprimiveis"
	recusas := []struct {
		nome     string
		args     []string
		esperado string
	}{
		{"sem forcar", []string{"--sem-backup-motivo", "motivo com tamanho bom"}, "--sem-backup-motivo so com --forcar"},
		{"nove caracteres", []string{"--forcar", "--sem-backup-motivo", strings.Repeat("x", 9)}, msgTamanho},
		{"301 caracteres", []string{"--forcar", "--sem-backup-motivo", strings.Repeat("x", 301)}, msgTamanho},
		{"quebra de linha", []string{"--forcar", "--sem-backup-motivo", "motivo com\nquebra de linha"}, msgTamanho},
		{"escape", []string{"--forcar", "--sem-backup-motivo", "motivo \x1b[31mcom escape"}, msgTamanho},
		{"janela zero", []string{"--forcar", "--backup-janela-min", "0"}, "--backup-janela-min precisa ser"},
		{"janela texto", []string{"--forcar", "--backup-janela-min", "abc"}, "--backup-janela-min precisa ser"},
		{"janela grande", []string{"--forcar", "--backup-janela-min", "1441"}, "--backup-janela-min precisa ser"},
		{"dir invalido", []string{"--forcar", "--backup-dir", "/a b"}, "--backup-dir invalido"},
	}
	for _, c := range recusas {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioSeed(t)
			out, err := e.rodar(append([]string{"--pepper-file", "/root/pepper.txt"}, c.args...)...)
			require.Error(t, err, out)
			assert.Contains(t, out, c.esperado)
			assert.Empty(t, e.ler("log"), "antes de qualquer docker")
			assert.NoFileExists(t, filepath.Join(e.dir, registroSeed))
		})
	}

	t.Run("motivo valido registra antes da primeira escrita", func(t *testing.T) {
		e, dir := novoSeedPreenchido(t)
		bin := filepath.Join(e.dir, "bin")
		require.NoError(t, os.Rename(filepath.Join(bin, "docker"), filepath.Join(bin, "docker-tty")))
		escrever(t, filepath.Join(bin, "docker"), stubDockerRegistroSeed, 0o755)
		// Backup ruim no diretório: quem permite seguir é o motivo.
		escrever(t, filepath.Join(dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz"), "nao e gzip", 0o600)
		const motivo = "motivo %s $(touch /tmp/rp) literal"
		s := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-dir", dir, "--sem-backup-motivo", motivo)
		s.digitar(promptNomeDoBanco, "rotaperfumes")
		out, err := s.terminar()
		require.NoError(t, err, out)
		assert.Contains(t, out, "AVISO: --sem-backup-motivo")
		reg := e.ler(registroSeed)
		assert.Contains(t, reg, "backup=nenhum motivo="+motivo+"\n")
		st, err := os.Stat(filepath.Join(e.dir, registroSeed))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())

		primeira := strings.TrimSpace(e.ler("state/primeira_com_registro"))
		require.NotEmpty(t, primeira, "o registro existe antes de alguma chamada ao docker")
		var limite int
		_, err = fmt.Sscan(primeira, &limite)
		require.NoError(t, err)
		chamadas := e.chamadas()
		require.GreaterOrEqual(t, len(chamadas), limite)
		for i, c := range chamadas[:limite-1] {
			if c.mariadb() {
				assert.True(t, strings.HasPrefix(c.stdin, "SELECT "), "chamada %d antes do registro: %q", i+1, c.stdin)
			}
			assert.False(t, c.tem("-create-admin"))
		}
		var ddl bool
		for _, c := range chamadas[limite-1:] {
			ddl = ddl || (c.mariadb() && !strings.HasPrefix(c.stdin, "SELECT "))
		}
		assert.True(t, ddl, "a escrita vem depois do registro")
	})
	t.Run("confirmacao errada nao registra", func(t *testing.T) {
		e, dir := novoSeedPreenchido(t)
		s := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-dir", dir, "--sem-backup-motivo", "motivo com tamanho bom")
		s.digitar(promptNomeDoBanco, "errado")
		out, err := s.terminar()
		require.Error(t, err, out)
		e.semSQLDeEscrita(t)
		assert.NoFileExists(t, filepath.Join(e.dir, registroSeed))
	})
	t.Run("banco vazio ignora o motivo e nao registra", func(t *testing.T) {
		e := novoEnsaioTTY(t)
		out, err := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--sem-backup-motivo", "motivo com tamanho bom").terminar()
		require.NoError(t, err, out)
		assert.NotContains(t, out, "--sem-backup-motivo")
		assert.NoFileExists(t, filepath.Join(e.dir, registroSeed))
	})
	t.Run("variavel de ambiente nao ignora a guarda", func(t *testing.T) {
		e, dir := novoSeedPreenchido(t)
		escrever(t, filepath.Join(e.dir, "runner.sh"), strings.Replace(runnerSeed, "umask 077\n",
			"umask 077\nexport SEM_BACKUP_MOTIVO='variavel do ambiente' BACKUP_JANELA_MIN=9999 BACKUP_USADO=/x\n", 1), 0o755)
		out, err := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-dir", dir).terminar()
		require.Error(t, err, out)
		assert.Contains(t, out, "nenhum backup")
		assert.NotContains(t, out, promptNomeDoBanco)
	})
	t.Run("sem forcar a tabela preenchida segue recusada, com motivo ou nao", func(t *testing.T) {
		e := novoEnsaioSeed(t)
		e.state("linhas", "5\n")
		out, err := e.rodar("--pepper-file", "/root/pepper.txt")
		require.Error(t, err, out)
		assert.Contains(t, out, "ja tem 5 linha(s)")
		e.semSQLDeEscrita(t)
	})
}

// ---------------------------------------------------------------------------
// Repositório
// ---------------------------------------------------------------------------

// A conta só SELECT e as flags do dump valem enquanto não houver objeto que
// exija outro privilégio. Se surgir, revise o GRANT e --skip-triggers.
var reObjetoFora = regexp.MustCompile(`(?i)\bCREATE\s+(OR\s+REPLACE\s+)?(ALGORITHM\s*=\s*\w+\s+)?(DEFINER\s*=\s*\S+\s+)?(SQL\s+SECURITY\s+\w+\s+)?(TRIGGER|VIEW|PROCEDURE|FUNCTION|EVENT)\b`)

func TestBACKUP01_TB_SqlSemObjetosForaDoDump(t *testing.T) {
	for _, amostra := range []string{"CREATE TRIGGER t", "create or replace view v", "CREATE DEFINER=`a`@`b` PROCEDURE p", "CREATE FUNCTION f", "CREATE EVENT e", "CREATE ALGORITHM=MERGE VIEW v"} {
		assert.Regexp(t, reObjetoFora, amostra, "o detector pega a forma")
	}
	assert.NotRegexp(t, reObjetoFora, "CREATE TABLE eventos")

	var total int
	require.NoError(t, filepath.WalkDir(caminhoRaiz(t, "sql"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".sql" {
			return err
		}
		total++
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sem := regexp.MustCompile(`(?m)^\s*--.*$`).ReplaceAllString(string(b), "")
		sem = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(sem, "")
		assert.NotRegexp(t, reObjetoFora, sem, filepath.Base(p))
		return nil
	}))
	assert.Greater(t, total, 0, "sql/ tem arquivos")
}

func TestBACKUP01_TB_FluxoDocumentado(t *testing.T) {
	doc := lerForaDoTarDoCI(t, "docs", "deploy-servidor.md")
	assert.NotRegexp(t, `mariadb-dump[^\n]*--(routines|triggers)`, doc, "a conta só SELECT não usa essas flags")
	for _, trecho := range []string{"rotaperfumes-backup-banco --agora", "--exigir-montagem", "--sem-backup-motivo"} {
		assert.Contains(t, doc, trecho)
	}
}

// ---------------------------------------------------------------------------
// Real: conta só SELECT num MariaDB descartável
// ---------------------------------------------------------------------------

const (
	imagemReal  = "rp-img-l42t:teste"
	servidorRe  = "rp-mariadb-l42t"
	redeReal    = "rp-net-l42t"
	prefixoReal = "172.30.143"
)

// Dados sintéticos, com o que costuma quebrar: acentos, emoji, fuso e texto que
// imita comando ou marcador.
const dadosReais = `SET time_zone = '-03:00';
SET NAMES utf8mb4;
INSERT INTO vendedores (id,nome,regiao,uf,data_admissao,meta_mensal,created_at,updated_at) VALUES
 (1,'João Ação Çedilha','São Paulo','SP','2020-02-29',1234.56,'2026-03-08 02:30:00','2026-03-08 02:30:00'),
 (2,'Émile Ñandú 香水 😀','Região','RJ','2021-01-01',0,'2026-01-01 00:00:00','2026-06-15 23:59:59');
INSERT INTO usuarios (id,nome,email,password_hash,role,id_vendedor,ativo,tokens_validos_desde,created_at,updated_at,ultimo_login_at) VALUES
 (1,'Admin Sintético','admin@exemplo.invalid','x','admin',NULL,1,'2026-11-05 12:34:56','2026-01-02 03:04:05','2026-06-07 08:09:10','2026-11-05 22:00:00'),
 (2,'Linha\nUSE producao;\nDROP DATABASE producao;','quebra@exemplo.invalid','x','normal',1,0,NULL,'2026-01-02 03:04:05','2026-01-02 03:04:05',NULL),
 (3,'-- Dump completed on 2020-01-01','marcador@exemplo.invalid','x','normal',2,1,NULL,'2026-01-02 03:04:05','2026-01-02 03:04:05',NULL);
`

const consultaIdaEVolta = `SET time_zone='-03:00';
SELECT id,nome,regiao,uf,data_admissao,meta_mensal,created_at,updated_at FROM vendedores ORDER BY id;
SELECT id,nome,email,role,ativo,tokens_validos_desde,created_at,updated_at,ultimo_login_at FROM usuarios ORDER BY id;`

type realBackup struct {
	t       *testing.T
	dir     string
	base    string
	senha   string
	cnfHost string
}

func (r *realBackup) docker(stdin string, args ...string) (string, string, error) {
	cmd := exec.Command("docker", args...)
	cmd.Env = ambienteLimpo()
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	return so.String(), se.String(), err
}

func (r *realBackup) rootSQL(stdin string, args ...string) (string, error) {
	base := []string{"exec", "-i", servidorRe, "mariadb", "--defaults-extra-file=/run/rotaperfumes/root.cnf", "-N", "-B", "--default-character-set=utf8mb4"}
	so, se, err := r.docker(stdin, append(base, args...)...)
	if err != nil {
		return so, fmt.Errorf("%w: %s", err, se)
	}
	return so, nil
}

// clienteBackup: os mesmos parâmetros de docker run do gerar_dump.
func (r *realBackup) clienteBackup(ip string, entrypoint string, args ...string) (string, string, error) {
	base := []string{"run", "--rm", "-i", "--network", redeReal, "--ip", ip, "--read-only", "--cap-drop", "ALL",
		"--security-opt", "no-new-privileges:true",
		"--mount", "type=bind,source=" + filepath.Join(r.dir, "ssl", "ca.pem") + ",target=/run/backup/ca.pem,readonly",
		"--mount", "type=bind,source=" + r.cnfHost + ",target=/run/backup/cliente.cnf,readonly",
		"--entrypoint", entrypoint, r.base, "--defaults-extra-file=/run/backup/cliente.cnf", "-h", servidorRe, "-P", "3306"}
	return r.docker("", append(base, args...)...)
}

func (r *realBackup) limpar() {
	_, _, _ = r.docker("", "rm", "-fv", servidorRe)
	_, _, _ = r.docker("", "network", "rm", redeReal)
	_, _, _ = r.docker("", "run", "--rm", "--network", "none", "-v", r.dir+":/x", "--entrypoint", "sh", r.base, "-c", "rm -rf /x/backup-cliente")
	_, _, _ = r.docker("", "image", "rm", imagemReal)
}

// TestBACKUP01_ContaBackup_Real é a parte "Real" da seção 7, com o SQL de contas
// e as flags dos scripts. Sem root; só recursos rp-*-l42t, removidos ao fim. Ex.:
// sg docker -c "BACKUP01_DOCKER=1 go test -count=1 -run TestBACKUP01_ContaBackup_Real ./tests/deploy/"
func TestBACKUP01_ContaBackup_Real(t *testing.T) {
	precisaBash(t)
	if os.Getenv("BACKUP01_DOCKER") != "1" {
		t.Skip("opt-in: BACKUP01_DOCKER=1, com Docker (não exige root)")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker inacessível: " + err.Error())
	}
	ferramentaObrigatoria(t, "openssl")

	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	imagem := regexp.MustCompile(`(?m)^IMAGEM_BASE="([^"]+)"$`).FindStringSubmatch(md)
	require.NotNil(t, imagem)
	sc := lerRaiz(t, "deploy", "backup-banco.sh")

	dir, err := os.MkdirTemp("", "rp-l42t-*")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(dir, 0o711))
	r := &realBackup{t: t, dir: dir, base: imagem[1], senha: "SenhaBackupRealAbc123456789XYZ01", cnfHost: filepath.Join(dir, "backup-cliente", "rotaperfumes_backup.cnf")}
	r.limpar()
	t.Cleanup(func() {
		r.limpar()
		_ = os.RemoveAll(dir)
	})

	// Certificados e cnf do servidor: mesmo TLS e sql_mode do script.
	ssl := filepath.Join(dir, "ssl")
	require.NoError(t, os.MkdirAll(ssl, 0o755))
	ossl := func(args ...string) {
		out, err := exec.Command("openssl", args...).CombinedOutput()
		require.NoError(t, err, "%s", out)
	}
	ossl("req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", ssl+"/ca-key.pem", "-out", ssl+"/ca.pem", "-days", "2", "-subj", "/CN=rp-l42t-ca",
		"-addext", "basicConstraints=critical,CA:TRUE", "-addext", "keyUsage=critical,keyCertSign,cRLSign")
	ossl("req", "-new", "-newkey", "rsa:2048", "-nodes", "-keyout", ssl+"/server-key.pem", "-subj", "/CN=rp-l42t", "-out", ssl+"/server.csr")
	escrever(t, ssl+"/ext", "basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:"+servidorRe+"\n", 0o644)
	ossl("x509", "-req", "-in", ssl+"/server.csr", "-CA", ssl+"/ca.pem", "-CAkey", ssl+"/ca-key.pem", "-set_serial", "0x1234", "-days", "2", "-extfile", ssl+"/ext", "-out", ssl+"/server-cert.pem")
	require.NoError(t, os.Chmod(ssl, 0o755))
	for _, f := range []string{"ca.pem", "server-cert.pem", "server-key.pem"} {
		require.NoError(t, os.Chmod(filepath.Join(ssl, f), 0o644))
	}
	sqlMode := regexp.MustCompile(`(?m)^readonly SQL_MODE="(.*)"$`).FindStringSubmatch(md)
	require.NotNil(t, sqlMode)
	build := filepath.Join(dir, "build")
	escrever(t, build+"/zz.cnf", "[mariadb]\nssl_ca=/etc/mysql/ssl/ca.pem\nssl_cert=/etc/mysql/ssl/server-cert.pem\nssl_key=/etc/mysql/ssl/server-key.pem\nrequire_secure_transport=ON\ntls_version=TLSv1.2,TLSv1.3\nplugin_load_add=auth_ed25519\nlocal_infile=0\nsql_mode="+sqlMode[1]+"\ncharacter_set_server=utf8mb4\ncollation_server=utf8mb4_unicode_ci\ncharacter_set_collations=utf8mb4=utf8mb4_unicode_ci\nskip_name_resolve=ON\ndefault_time_zone=+05:00\n", 0o644)
	escrever(t, build+"/Dockerfile", "FROM "+r.base+"\nCOPY zz.cnf /etc/mysql/conf.d/zz-rotaperfumes.cnf\nRUN chmod 644 /etc/mysql/conf.d/zz-rotaperfumes.cnf\n", 0o644)
	_, se, err := r.docker("", "build", "-q", "-t", imagemReal, build)
	require.NoError(t, err, se)

	_, se, err = r.docker("", "network", "create", "--driver", "bridge", "--subnet", prefixoReal+".0/24", "--gateway", prefixoReal+".1", "--ip-range", prefixoReal+".128/25", redeReal)
	require.NoError(t, err, se)

	// Servidor: só tmpfs, nada persiste.
	segredosHost := filepath.Join(dir, "root")
	require.NoError(t, os.MkdirAll(segredosHost, 0o755))
	require.NoError(t, os.Chmod(segredosHost, 0o755))
	const senhaRoot = "SenhaRootRealAbc123456789XYZ0123"
	escrever(t, segredosHost+"/root.senha", senhaRoot, 0o644)
	escrever(t, segredosHost+"/root.cnf", "[client]\nuser=root\npassword="+senhaRoot+"\nprotocol=socket\n", 0o644)
	_, se, err = r.docker("", "run", "-d", "--name", servidorRe, "--network", redeReal, "--ip", prefixoReal+".2", "--user", "999:999", "--cap-drop", "ALL",
		"--security-opt", "no-new-privileges:true", "--read-only",
		"--tmpfs", "/run/mysqld:rw,nosuid,nodev,noexec,mode=0755,uid=999,gid=999",
		"--tmpfs", "/tmp:rw,nosuid,nodev,noexec,mode=1777",
		"--tmpfs", "/var/lib/mysql:rw,nosuid,nodev,noexec,mode=0700,uid=999,gid=999",
		"-e", "MARIADB_ROOT_PASSWORD_FILE=/run/rotaperfumes/root.senha", "-e", "MARIADB_ROOT_HOST=localhost",
		"-v", ssl+":/etc/mysql/ssl:ro", "-v", segredosHost+":/run/rotaperfumes:ro", imagemReal)
	require.NoError(t, err, se)
	require.Eventually(t, func() bool {
		out, err := r.rootSQL("", "-e", "SELECT @@skip_networking")
		return err == nil && strings.TrimSpace(out) == "0"
	}, 180*time.Second, 2*time.Second, "o servidor não ficou pronto")

	// Contas: a função sql_contas real, extraída do script.
	fn := regexp.MustCompile(`(?ms)^sql_contas\(\) \{.*?^\}\n`).FindString(md)
	require.NotEmpty(t, fn)
	escrever(t, filepath.Join(dir, "sql_contas.fn"), fn, 0o600)
	roteiro := "export DB_NOME=rotaperfumes COLLATION=utf8mb4_unicode_ci REDE_NOME=" + redeReal + "\n" +
		"DB_APP=rotaperfumes_app DB_ADMIN=rotaperfumes_admin DB_BACKUP=rotaperfumes_backup DB_DBA=rotaperfumes-admin\n" +
		"IP_API=" + prefixoReal + ".10 IP_ADMIN=" + prefixoReal + ".20 IP_BACKUP=" + prefixoReal + ".21 IP_DBA=" + prefixoReal + ".22\n" +
		"SENHA_DBA=SenhaDbaRealAbc123456789XYZ012345\n" +
		"SENHA_APP=SenhaAppRealAbc123456789XYZ012345 SENHA_ADMIN=SenhaAdminRealAbc123456789XYZ012 SENHA_BACKUP=" + r.senha + "\n" +
		"source \"" + filepath.Join(dir, "sql_contas.fn") + "\"\nsql_contas\n"
	cmd := exec.Command("bash", "-c", roteiro)
	cmd.Env = ambienteLimpo()
	contas, err := cmd.Output()
	require.NoError(t, err)
	for i := 0; i < 2; i++ { // idempotente
		_, err = r.rootSQL(string(contas))
		require.NoError(t, err)
	}

	// Schema real e dados sintéticos.
	var ddl strings.Builder
	ddl.WriteString("USE rotaperfumes;\n")
	for _, l := range strings.Split(lerRaiz(t, "sql", "ordem-ddl.txt"), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		ddl.WriteString(lerRaiz(t, "sql", l) + "\n")
	}
	_, err = r.rootSQL(ddl.String())
	require.NoError(t, err)
	_, err = r.rootSQL(dadosReais, "rotaperfumes")
	require.NoError(t, err)

	// Option file 400 do root, como no servidor.
	escrever(t, r.cnfHost, "[client]\nuser=rotaperfumes_backup\npassword="+r.senha+"\n", 0o600)
	_, se, err = r.docker("", "run", "--rm", "--network", "none", "-v", filepath.Dir(r.cnfHost)+":/x", "--entrypoint", "sh", r.base, "-c", "chown -R 0:0 /x && chmod 400 /x/rotaperfumes_backup.cnf && chmod 700 /x")
	require.NoError(t, err, se)

	ipBackup := prefixoReal + ".21"
	tls := []string{"--ssl-ca=/run/backup/ca.pem", "--ssl-verify-server-cert"}
	flags := []string{"--single-transaction", "--skip-lock-tables", "--no-tablespaces", "--skip-add-drop-table", "--skip-triggers", "--hex-blob", "--default-character-set=utf8mb4"}
	for _, f := range flags {
		assert.Contains(t, sc, f, "a flag testada é a do script")
	}

	t.Run("root sem capabilities le o cnf e conecta com TLS", func(t *testing.T) {
		out, se, err := r.clienteBackup(ipBackup, "mariadb", append(tls, "-N", "-B", "-e",
			"SELECT CONCAT(CURRENT_USER(),' ',(SELECT VARIABLE_VALUE FROM information_schema.SESSION_STATUS WHERE VARIABLE_NAME='Ssl_version'))")...)
		require.NoError(t, err, se)
		assert.Equal(t, "rotaperfumes_backup@"+ipBackup+" TLSv1.3", strings.TrimSpace(out))
	})

	var dump string
	t.Run("dump completo com as flags exatas", func(t *testing.T) {
		args := append(append(append([]string{}, tls...), flags...), "rotaperfumes")
		out, se, err := r.clienteBackup(ipBackup, "mariadb-dump", args...)
		require.NoError(t, err, se)
		assert.Empty(t, strings.TrimSpace(se))
		dump = out
		ls := linhas(dump)
		require.NotEmpty(t, ls)
		assert.True(t, strings.HasPrefix(ls[len(ls)-1], "-- Dump completed"), "marcador na última linha")
		assert.Equal(t, tabelasDaOrdemDDL(t), len(regexp.MustCompile(`(?m)^CREATE TABLE `).FindAllString(dump, -1)))
		for _, tab := range []string{"usuarios", "clientes", "vendedores"} {
			assert.Regexp(t, "(?m)^CREATE TABLE `"+tab+"` \\(", dump)
		}
		assert.Regexp(t, "(?m)^INSERT INTO `usuarios` VALUES$", dump)
		assert.NotRegexp(t, `(?mi)^(USE|CREATE DATABASE|DROP TABLE)\b`, dump)
	})

	t.Run("escrita e criacao negadas", func(t *testing.T) {
		antes, err := r.rootSQL("", "rotaperfumes", "-e", "SELECT COUNT(*) FROM usuarios")
		require.NoError(t, err)
		for nome, q := range map[string]string{
			"INSERT":  "INSERT INTO usuarios (id) VALUES (99)",
			"UPDATE":  "UPDATE usuarios SET nome='x'",
			"DELETE":  "DELETE FROM usuarios",
			"DROP":    "DROP TABLE usuarios",
			"CREATE":  "CREATE TABLE t_l42t (id int)",
			"TRIGGER": "CREATE TRIGGER t_l42t BEFORE INSERT ON usuarios FOR EACH ROW SET NEW.nome = 'x'",
		} {
			t.Run(nome, func(t *testing.T) {
				_, se, err := r.clienteBackup(ipBackup, "mariadb", append(tls, "rotaperfumes", "-e", q)...)
				require.Error(t, err)
				assert.Regexp(t, `(?i)denied`, se)
			})
		}
		depois, err := r.rootSQL("", "rotaperfumes", "-e", "SELECT COUNT(*) FROM usuarios")
		require.NoError(t, err)
		assert.Equal(t, antes, depois)
	})

	t.Run("sem TLS recusado", func(t *testing.T) {
		_, se, err := r.clienteBackup(ipBackup, "mariadb", "--skip-ssl", "-e", "SELECT 1")
		require.Error(t, err)
		assert.Regexp(t, `ERROR (3159|1045)`, se, "recusado pelo servidor, não por outro motivo")
	})
	t.Run("outro IP recusado", func(t *testing.T) {
		_, se, err := r.clienteBackup(prefixoReal+".22", "mariadb", append(tls, "-e", "SELECT 1")...)
		require.Error(t, err)
		assert.Regexp(t, `ERROR 1045|1130 - Host`, se, "recusado pelo servidor, não por outro motivo")
	})

	t.Run("ida e volta preserva dados", func(t *testing.T) {
		require.NotEmpty(t, dump)
		_, err := r.rootSQL("", "-e", "CREATE DATABASE restaurado_l42t CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
		require.NoError(t, err)
		_, err = r.rootSQL(dump, "restaurado_l42t")
		require.NoError(t, err)

		orig, err := r.rootSQL(consultaIdaEVolta, "rotaperfumes")
		require.NoError(t, err)
		rest, err := r.rootSQL(consultaIdaEVolta, "restaurado_l42t")
		require.NoError(t, err)
		assert.Equal(t, orig, rest)
		assert.Contains(t, rest, "João Ação Çedilha")
		assert.Contains(t, rest, "Émile Ñandú 香水 😀")
		assert.Contains(t, rest, "2026-03-08 02:30:00")

		somas := func(banco string) []string {
			tabelas, err := r.rootSQL("", "-e", "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA='"+banco+"'")
			require.NoError(t, err)
			var out []string
			for _, tab := range linhas(tabelas) {
				s, err := r.rootSQL("", banco, "-e", "CHECKSUM TABLE `"+tab+"`")
				require.NoError(t, err)
				out = append(out, strings.ReplaceAll(strings.TrimSpace(s), banco+".", ""))
			}
			sort.Strings(out)
			return out
		}
		assert.Equal(t, somas("rotaperfumes"), somas("restaurado_l42t"))
		assert.Len(t, somas("restaurado_l42t"), tabelasDaOrdemDDL(t))
	})

	t.Run("restaurar sobre tabela existente falha sem mexer nos dados", func(t *testing.T) {
		require.NotEmpty(t, dump)
		antes, err := r.rootSQL(consultaIdaEVolta, "restaurado_l42t")
		require.NoError(t, err)
		_, err = r.rootSQL(dump, "restaurado_l42t")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "1050")
		depois, err := r.rootSQL(consultaIdaEVolta, "restaurado_l42t")
		require.NoError(t, err)
		assert.Equal(t, antes, depois)
	})
}
