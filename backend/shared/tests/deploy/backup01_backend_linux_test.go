package deploy_test

// BACKUP-01 (Lote 42, BackBrain): deploy/backup-banco.sh e as guardas do
// --recriar e do seed --forcar. O restante da seção 7 da especificação fica com
// o TestBrain.
//   - ensaio: script real num user namespace, com docker, systemctl e df falsos;
//   - real (Docker): roda fora do make test e fica registrado no relatório.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const stubDockerBackup = `#!/bin/bash
S="$FAKE/state"
printf '%s\n' "docker $*" >>"$FAKE/log"
case "$1" in
  info|rm) exit 0 ;;
  volume)
    [[ -f "$S/sem_volume" ]] && exit 1
    cat "$S/volume_rotulo" 2>/dev/null || echo rotaperfumes/produto_imagens
    exit 0 ;;
  run)
    case "$*" in
      *"--entrypoint du"*) cat "$S/du" 2>/dev/null || printf '1000\t/imagens\n'; exit 0 ;;
      *"--entrypoint tar"*)
        if [[ -f "$S/tar_falha" ]]; then echo 'tar: ./nome-secreto.jpg: Permission denied' >&2; exit 2; fi
        exec tar -C "$S/volume" --numeric-owner -cf - . ;;
      *mariadb-dump*)
        [[ -f "$S/dump_stderr" ]] && cat "$S/dump_stderr" >&2
        if [[ -f "$S/dump_preso" ]]; then touch "$S/dump_iniciado"; exec sleep 60; fi
        cat "$S/dump"
        exit "$(cat "$S/dump_rc" 2>/dev/null || echo 0)" ;;
    esac ;;
esac
echo "docker stub: inesperado: $*" >&2
exit 9
`

// O systemd real roda a unit de forma assíncrona; aqui, "start" já roda a cópia
// instalada e guarda o Result.
const stubSystemctlBackup = `#!/bin/bash
S="$FAKE/state"
echo "systemctl $*" >>"$FAKE/log"
case "$*" in
  *"start --no-block"*)
    if bash /usr/local/sbin/rotaperfumes-backup-banco --agendado >>"$FAKE/unit.log" 2>&1; then
      echo success >"$S/result"
    else
      echo exit-code >"$S/result"
    fi ;;
  *"-p Result"*) cat "$S/result" 2>/dev/null || echo success ;;
  *"-p ActiveState"*) echo inactive ;;
  is-active*) [[ -f "$S/timer_parado" ]] && exit 3; exit 0 ;;
esac
exit 0
`

const stubDfBackup = `#!/bin/bash
echo "df $*" >>"$FAKE/log"
echo Avail
cat "$FAKE/state/df_livre" 2>/dev/null || echo 999999999999
`

// Como o runnerSeed, com os destinos fixos do backup-banco.sh.
const runnerBackup = `#!/bin/bash
set -euo pipefail
umask 022
NR="$FAKE/.raiz"
mkdir -p "$NR"
mount -t tmpfs -o mode=755 tmpfs "$NR"
topo=${FAKE#/}; topo=/${topo%%/*}
for x in /*; do
  [[ "$x" == "$topo" ]] && continue
  if [[ -L "$x" ]]; then ln -s "$(readlink "$x")" "$NR$x"
  elif [[ "$x" == /etc ]]; then
    mkdir "$NR/etc"
    for y in /etc/*; do
      [[ "$y" == /etc/mysql || "$y" == /etc/rotaperfumes ]] && continue
      if [[ -L "$y" ]]; then ln -s "$(readlink "$y")" "$NR$y"
      elif [[ -d "$y" ]]; then mkdir "$NR$y"; mount --rbind "$y" "$NR$y" 2>/dev/null || true
      elif [[ -e "$y" ]]; then : >"$NR$y"; mount --bind "$y" "$NR$y" 2>/dev/null || true
      fi
    done
  elif [[ -d "$x" ]]; then mkdir "$NR$x"; mount --rbind "$x" "$NR$x"
  fi
done
mkdir -p "$NR$topo" "$NR/etc/mysql" "$NR/etc/rotaperfumes"
expor() { [[ "$1" == "$topo"/* ]] || return 0; mkdir -p "$NR$1"; mount --bind "$1" "$NR$1"; }
expor "$FAKE"
expor "$(dirname "$SCRIPT")"
# Disco simulado: tmpfs real, que some junto com a execução.
if [[ -n "${MONTAR:-}" ]]; then mount -t tmpfs -o "mode=$MONTAR_MODO" tmpfs "$NR$MONTAR"; fi
mount --bind "$FAKE/root" "$NR/root"
mount --bind "$FAKE/etcmysql" "$NR/etc/mysql"
mount --bind "$FAKE/etcrp" "$NR/etc/rotaperfumes"
mount --bind "$FAKE/sbin" "$NR/usr/local/sbin"
mount --bind "$FAKE/systemd" "$NR/etc/systemd/system"
cd /
export PATH="$FAKE/bin:$PATH"
umask 077
exec chroot "$NR" /bin/bash -c 'cd / && exec bash "$0" "$@"' "$SCRIPT" "$@"
`

const (
	copiaInstalada     = "/usr/local/sbin/rotaperfumes-backup-banco"
	senhaBackupEnsaio  = "SentinelaSenhaBackupAbc123456789"
	carimboAntigoDump  = "20260101-033000Z"
	carimboAntigoDump2 = "20260102-033000Z"
)

type ensaioBackup struct {
	t   *testing.T
	dir string
}

func novoEnsaioBackup(t *testing.T) *ensaioBackup {
	t.Helper()
	precisaUnshare(t)
	for _, c := range []string{"flock", "gzip", "tar", "chroot", "realpath", "pkill"} {
		ferramentaObrigatoria(t, c)
	}
	e := &ensaioBackup{t: t, dir: t.TempDir()}
	require.NoError(t, os.Chmod(e.dir, 0o700))
	for _, d := range []string{"etcmysql/ssl", "etcrp", "sbin", "systemd", "state/volume", "bin"} {
		require.NoError(t, os.MkdirAll(filepath.Join(e.dir, d), 0o755))
	}
	for _, d := range []string{"root/rotaperfumes-mariadb/backup-cliente", "backups"} {
		require.NoError(t, os.MkdirAll(filepath.Join(e.dir, d), 0o700))
	}
	require.NoError(t, os.Chmod(filepath.Join(e.dir, "root"), 0o700))
	require.NoError(t, os.Chmod(filepath.Join(e.dir, "root/rotaperfumes-mariadb"), 0o700))
	escrever(t, filepath.Join(e.dir, "bin/docker"), stubDockerBackup, 0o755)
	escrever(t, filepath.Join(e.dir, "bin/systemctl"), stubSystemctlBackup, 0o755)
	escrever(t, filepath.Join(e.dir, "bin/df"), stubDfBackup, 0o755)
	escrever(t, filepath.Join(e.dir, "runner.sh"), runnerBackup, 0o755)
	escrever(t, filepath.Join(e.dir, "etcmysql/ssl/ca.pem"), "-----BEGIN CERTIFICATE-----\nx\n-----END CERTIFICATE-----\n", 0o644)
	cnf := filepath.Join(e.dir, "root/rotaperfumes-mariadb/backup-cliente/rotaperfumes_backup.cnf")
	escrever(t, cnf, "[client]\nuser=rotaperfumes_backup\npassword="+senhaBackupEnsaio+"\n", 0o400)
	e.state("dump", dumpBackupBom())
	e.state("volume/foto-1.jpg", "imagem sintetica\n")
	return e
}

func (e *ensaioBackup) state(nome, v string) {
	e.t.Helper()
	escrever(e.t, filepath.Join(e.dir, "state", nome), v, 0o644)
}

func (e *ensaioBackup) ler(rel string) string {
	e.t.Helper()
	b, err := os.ReadFile(filepath.Join(e.dir, rel))
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(e.t, err)
	return string(b)
}

func (e *ensaioBackup) backups() string { return filepath.Join(e.dir, "backups") }

// instalarCopia põe a cópia e o conf no lugar, sem passar pelo --instalar.
func (e *ensaioBackup) instalarCopia(conf ...string) {
	e.t.Helper()
	escrever(e.t, filepath.Join(e.dir, "sbin/rotaperfumes-backup-banco"), lerRaiz(e.t, "deploy", "backup-banco.sh"), 0o700)
	escrever(e.t, filepath.Join(e.dir, "etcrp/backup-banco.conf"),
		"BACKUP_DIR="+e.backups()+"\n"+strings.Join(conf, ""), 0o644)
}

func (e *ensaioBackup) comando(script string, args ...string) *exec.Cmd {
	e.t.Helper()
	cmd := exec.Command("unshare", append([]string{"-rm", "bash", filepath.Join(e.dir, "runner.sh")}, args...)...)
	cmd.Env = append(ambienteLimpo(), "FAKE="+e.dir, "SCRIPT="+script)
	return cmd
}

func (e *ensaioBackup) rodar(script string, args ...string) (string, int) {
	e.t.Helper()
	out, err := e.comando(script, args...).CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	ee, ok := err.(*exec.ExitError)
	require.True(e.t, ok, "%v", err)
	return string(out), ee.ExitCode()
}

func (e *ensaioBackup) instalada(args ...string) (string, int) {
	e.t.Helper()
	return e.rodar(copiaInstalada, args...)
}

func (e *ensaioBackup) checkout(args ...string) (string, int) {
	e.t.Helper()
	return e.rodar(caminhoRaiz(e.t, "deploy", "backup-banco.sh"), args...)
}

func (e *ensaioBackup) arquivos() []string {
	e.t.Helper()
	ents, err := os.ReadDir(e.backups())
	require.NoError(e.t, err)
	var out []string
	for _, x := range ents {
		if x.Name() != ".trava" {
			out = append(out, x.Name())
		}
	}
	return out
}

func (e *ensaioBackup) semTemporarios() {
	e.t.Helper()
	tmps, err := filepath.Glob(filepath.Join(e.backups(), ".tmp-*"))
	require.NoError(e.t, err)
	assert.Empty(e.t, tmps, "sem temporário no fim")
}

func (e *ensaioBackup) semSenha(out string) {
	e.t.Helper()
	assert.NotContains(e.t, out, senhaBackupEnsaio)
	assert.NotContains(e.t, e.ler("log"), senhaBackupEnsaio)
}

// par cria um backup bom, com o mesmo carimbo nos dois arquivos.
func (e *ensaioBackup) par(carimbo, sufixo string) {
	e.t.Helper()
	escrever(e.t, filepath.Join(e.backups(), "rotaperfumes-"+carimbo+sufixo+".sql.gz"), comprimir(e.t, dumpBackupBom()), 0o600)
	escrever(e.t, filepath.Join(e.backups(), "rotaperfumes-imagens-"+carimbo+sufixo+".tar.gz"),
		comprimir(e.t, tarSintetico(e.t, map[string]string{"foto-1.jpg": "x"})), 0o600)
}

func comprimir(t *testing.T, texto string) string {
	t.Helper()
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, err := w.Write([]byte(texto))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return b.String()
}

func tarSintetico(t *testing.T, arquivos map[string]string) string {
	t.Helper()
	var b bytes.Buffer
	w := tar.NewWriter(&b)
	for nome, conteudo := range arquivos {
		require.NoError(t, w.WriteHeader(&tar.Header{Name: nome, Mode: 0o644, Size: int64(len(conteudo)), Typeflag: tar.TypeReg}))
		_, err := w.Write([]byte(conteudo))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return b.String()
}

// Formato real da 11.4.13 (manual, seção 24.2), com 13 tabelas e mais de 4 KiB.
func dumpBackupBom() string {
	var b strings.Builder
	b.WriteString("/*M!999999\\- enable the sandbox mode */ \n-- MariaDB dump\n")
	for _, t := range tabelasVerificacao(13) {
		fmt.Fprintf(&b, "CREATE TABLE `%s` (\n  `id` bigint NOT NULL\n);\n", t)
	}
	b.WriteString("INSERT INTO `usuarios` VALUES\n(1,'-- Dump completed on dado de cliente');\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "-- preenchimento sintetico %d\n", i)
	}
	b.WriteString(marcadorDump)
	return b.String()
}

func TestBACKUP01_Backup_Estatico(t *testing.T) {
	sc := lerRaiz(t, "deploy", "backup-banco.sh")
	codigo := codigoSemComentarios(sc)
	assert.Contains(t, codigo, "set -euo pipefail")
	assert.Contains(t, codigo, "umask 077")
	assert.NotRegexp(t, `(?m)^\s*set\s+-[a-z]*x`, codigo, "nunca set -x")

	// Paridade da imagem (SEC-31) e auditoria dos digests.
	re := regexp.MustCompile(`(?m)^IMAGEM_BASE="([^"]+)"$`)
	v, m := re.FindStringSubmatch(sc), re.FindStringSubmatch(lerRaiz(t, "deploy", "mariadb-docker.sh"))
	require.NotNil(t, v)
	require.NotNil(t, m)
	assert.Equal(t, m[1], v[1], "mesma string do IMAGEM_BASE")
	fontes := regexp.MustCompile(`(?m)^IMAGENS_FIXADAS_EM\s*:?=\s*(.+)$`).FindStringSubmatch(lerRaiz(t, "Makefile"))
	require.NotNil(t, fontes)
	for _, f := range []string{"Jenkinsfile", "deploy/mariadb-docker.sh", "deploy/seed-producao.sh", "deploy/backup-banco.sh", "deploy/verificar-backup.sh", "deploy/restaurar-maquina.sh"} {
		assert.Contains(t, strings.Fields(fontes[1]), f)
	}

	// Conferências copiadas: iguais à fonte.
	funcao := func(texto, nome string) string {
		t.Helper()
		f := regexp.MustCompile(`(?ms)^` + nome + `\(\) \{.*?^\}\n`).FindString(texto)
		require.NotEmpty(t, f, nome)
		return f
	}
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	assert.Equal(t, funcao(md, "conferir_dir_privado"), funcao(sc, "conferir_dir_privado"))
	assert.Equal(t, funcao(md, "conferir_backup_recente"), funcao(lerRaiz(t, "deploy", "seed-producao.sh"), "conferir_backup_recente"))

	// O volume é o do compose, e o usuário é o dono criado no Dockerfile da API.
	compose := lerRaiz(t, "deploy", "docker-compose.yml")
	assert.Regexp(t, `(?m)^name: rotaperfumes$`, compose)
	assert.Regexp(t, `(?m)^  produto_imagens:$`, compose)
	assert.Contains(t, sc, `readonly VOLUME_IMAGENS="rotaperfumes_produto_imagens"`)
	assert.Contains(t, sc, `readonly VOLUME_ROTULO="rotaperfumes/produto_imagens"`)
	t.Run("dono do volume no Dockerfile da API", func(t *testing.T) {
		assert.Contains(t, lerForaDoTarDoCI(t, "backend", "Dockerfile"), "--chown=65532:65532 /out/data /data")
	})
	assert.Contains(t, sc, `readonly USUARIO_IMAGENS="65532:65532"`)

	// Sem aviso externo por enquanto (D2).
	for _, proibido := range []string{"curl", "wget", "MYSQL_PWD"} {
		assert.NotContains(t, codigo, proibido)
	}
}

func TestBACKUP01_Backup_ComandoEPar(t *testing.T) {
	e := novoEnsaioBackup(t)
	e.instalarCopia()
	out, rc := e.instalada("--agora")
	require.Equal(t, 0, rc, out)
	e.semTemporarios()
	e.semSenha(out)

	log := e.ler("log")
	var dump, imagens string
	for _, l := range linhas(log) {
		if strings.Contains(l, "--entrypoint mariadb-dump") {
			dump = l
		}
		if strings.Contains(l, "--entrypoint tar") {
			imagens = l
		}
		if strings.HasPrefix(l, "docker rm") {
			assert.Equal(t, "docker rm -f -- rotaperfumes-backup-cliente rotaperfumes-backup-imagens", l)
		}
	}
	require.NotEmpty(t, dump, log)
	for _, s := range []string{"--name rotaperfumes-backup-cliente", "--network rotaperfumes-db --ip 172.30.42.21", "--read-only", "--cap-drop ALL",
		"--security-opt no-new-privileges:true", "mariadb-dump mariadb:11.4.13@sha256:", " --defaults-extra-file=/run/backup/cliente.cnf -h rotaperfumes-mariadb -P 3306",
		"--ssl-verify-server-cert", "--single-transaction --skip-lock-tables --no-tablespaces --skip-add-drop-table --skip-triggers",
		"--hex-blob --default-character-set=utf8mb4 rotaperfumes"} {
		assert.Contains(t, dump, s)
	}
	for _, proibido := range []string{" -t ", "--databases", " -B ", "--all-databases", "--compact", "--skip-comments", "--skip-tz-utc", "MYSQL_PWD"} {
		assert.NotContains(t, dump, proibido)
	}
	require.NotEmpty(t, imagens, log)
	for _, s := range []string{"--network none", "--read-only", "--cap-drop ALL", "--user 65532:65532",
		"type=volume,source=rotaperfumes_produto_imagens,target=/imagens,readonly", "-C /imagens --numeric-owner -cf - ."} {
		assert.Contains(t, imagens, s)
	}

	nomes := e.arquivos()
	require.Len(t, nomes, 2, "%v", nomes)
	reD := regexp.MustCompile(`^rotaperfumes-(\d{8}-\d{6}Z)-manual\.sql\.gz$`)
	reI := regexp.MustCompile(`^rotaperfumes-imagens-(\d{8}-\d{6}Z)-manual\.tar\.gz$`)
	mI, mD := reI.FindStringSubmatch(nomes[0]), reD.FindStringSubmatch(nomes[1])
	if mD == nil {
		mI, mD = reI.FindStringSubmatch(nomes[1]), reD.FindStringSubmatch(nomes[0])
	}
	require.NotNil(t, mD, "%v", nomes)
	require.NotNil(t, mI, "%v", nomes)
	assert.Equal(t, mD[1], mI[1], "mesmo carimbo")
	for _, n := range nomes {
		st, err := os.Stat(filepath.Join(e.backups(), n))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), st.Mode().Perm(), n)
	}
	assert.Contains(t, out, "arquivos_imagens=1")
	assert.NotContains(t, out, "foto-1.jpg", "nome do tar fora do log")
	assert.NotContains(t, out, "AVISO")
}

// Falha no dump: nenhum arquivo novo, último par e retenção intactos.
func TestBACKUP01_Backup_FalhasDoDump(t *testing.T) {
	semMarcador := strings.TrimSuffix(dumpBackupBom(), marcadorDump)
	casos := []struct {
		nome, dump, rc, esperado string
	}{
		{"mariadb-dump falha depois de escrever", dumpBackupBom(), "1", "o dump falhou"},
		{"dump vazio", "", "", "marcador"},
		{"sem marcador", semMarcador, "", "marcador"},
		{"marcador no meio", marcadorDump + semMarcador, "", "marcador"},
		{"tabela faltando", strings.Replace(dumpBackupBom(), "CREATE TABLE `clientes`", "CREATE TABLE `outra`", 1), "", "nao tem usuarios"},
		{"poucas tabelas", strings.Replace(dumpBackupBom(), "CREATE TABLE `tabela_00`", "-- removida", 1), "", "tabela(s)"},
		{"usuarios sem dados", strings.Replace(dumpBackupBom(), "INSERT INTO `usuarios` VALUES\n", "", 1), "", "sem dados"},
		{"abaixo do piso", "CREATE TABLE `x` (\n);\n" + marcadorDump, "", "tabela(s)"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia()
			e.par(carimboAntigoDump, "")
			antes := e.arquivos()
			e.state("dump", c.dump)
			if c.rc != "" {
				e.state("dump_rc", c.rc)
			}
			out, rc := e.instalada("--agendado")
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, c.esperado)
			assert.Equal(t, antes, e.arquivos(), "nada novo e nada apagado")
			assert.NotContains(t, e.ler("log"), "--entrypoint tar", "para antes das imagens")
		})
	}
}

func TestBACKUP01_Backup_QuedaDoDump(t *testing.T) {
	e := novoEnsaioBackup(t)
	e.instalarCopia()
	// Sem compressão útil no anterior: o novo fica bem abaixo de 50%.
	grande := make([]byte, 200*1024)
	for i := range grande {
		grande[i] = byte(i*i*31 + i>>3)
	}
	e.par(carimboAntigoDump, "-manual")
	require.NoError(t, os.WriteFile(filepath.Join(e.backups(), "rotaperfumes-"+carimboAntigoDump+"-manual.sql.gz"), grande, 0o600))
	antes := e.arquivos()

	out, rc := e.instalada("--agora")
	assert.Equal(t, 1, rc, out)
	assert.Contains(t, out, "menos de 50%")
	assert.Equal(t, antes, e.arquivos())

	out, rc = e.instalada("--agora", "--aceitar-reducao")
	require.Equal(t, 0, rc, out)
	assert.Contains(t, e.ler("backups/.registro-reducao"), "referencia=rotaperfumes-"+carimboAntigoDump)

	// Só com --agora: o timer não aceita redução.
	_, rc = e.instalada("--agendado", "--aceitar-reducao")
	assert.Equal(t, 2, rc)
}

func TestBACKUP01_Backup_QuedaDeImagensNaoReprova(t *testing.T) {
	e := novoEnsaioBackup(t)
	e.instalarCopia()
	e.par(carimboAntigoDump, "-manual")
	grande := make([]byte, 400*1024)
	for i := range grande {
		grande[i] = byte(i*i*31 + i>>3)
	}
	require.NoError(t, os.WriteFile(filepath.Join(e.backups(), "rotaperfumes-imagens-"+carimboAntigoDump+"-manual.tar.gz"), grande, 0o600))
	out, rc := e.instalada("--agora")
	require.Equal(t, 0, rc, out)
}

// Falha nas imagens: dump guardado como parcial, sem contar como bom, sem
// retenção e com código de erro.
func TestBACKUP01_Backup_FalhaNasImagens(t *testing.T) {
	casos := []struct {
		nome     string
		preparar func(e *ensaioBackup)
		esperado string
	}{
		{"volume ausente", func(e *ensaioBackup) { e.state("sem_volume", "") }, "nao existe"},
		{"volume de outro projeto", func(e *ensaioBackup) { e.state("volume_rotulo", "outro/produto_imagens\n") }, "nao foi criado pelo compose"},
		{"tar falha", func(e *ensaioBackup) { e.state("tar_falha", "") }, "o tar do volume falhou"},
		{"symlink no volume", func(e *ensaioBackup) {
			require.NoError(e.t, os.Symlink("/etc/passwd", filepath.Join(e.dir, "state/volume/link")))
		}, "nao sao arquivo nem diretorio"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia("RETENCAO_DIARIOS=2\n")
			e.par(carimboAntigoDump, "")
			e.par(carimboAntigoDump2, "")
			e.par("20260103-033000Z", "")
			antes := e.arquivos()
			c.preparar(e)
			out, rc := e.instalada("--agendado")
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, c.esperado)
			assert.Contains(t, out, "retencao nao executada")
			assert.NotContains(t, out, "nome-secreto", "nome do tar fora do log")
			e.semTemporarios()
			depois := e.arquivos()
			assert.Subset(t, depois, antes, "nenhum backup bom apagado")
			require.Len(t, depois, len(antes)+1, "%v", depois)
			var parcial string
			for _, n := range depois {
				if strings.HasPrefix(n, "parcial-") {
					parcial = n
				}
			}
			assert.Regexp(t, `^parcial-rotaperfumes-\d{8}-\d{6}Z\.sql\.gz$`, parcial)
			st, err := os.Stat(filepath.Join(e.backups(), parcial))
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
		})
	}

	t.Run("parciais limitados", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.state("sem_volume", "")
		for _, c := range []string{"20260101-033000Z", "20260102-033000Z", "20260103-033000Z", "20260104-033000Z"} {
			escrever(t, filepath.Join(e.backups(), "parcial-rotaperfumes-"+c+".sql.gz"), "x", 0o600)
		}
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		var parciais []string
		for _, n := range e.arquivos() {
			if strings.HasPrefix(n, "parcial-") {
				parciais = append(parciais, n)
			}
		}
		require.Len(t, parciais, 3, "%v", parciais)
		assert.NotContains(t, parciais, "parcial-rotaperfumes-20260102-033000Z.sql.gz")
	})
}

func TestBACKUP01_Backup_VolumeVazioAceitoComAviso(t *testing.T) {
	e := novoEnsaioBackup(t)
	e.instalarCopia()
	require.NoError(t, os.Remove(filepath.Join(e.dir, "state/volume/foto-1.jpg")))
	out, rc := e.instalada("--agendado")
	require.Equal(t, 0, rc, out)
	assert.Contains(t, out, "AVISO: o volume rotaperfumes_produto_imagens esta vazio")
	assert.Contains(t, out, "arquivos_imagens=0")
}

func TestBACKUP01_Backup_Espaco(t *testing.T) {
	const gib = int64(1 << 30)
	novo := func(t *testing.T) (*ensaioBackup, int64, int64) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.par(carimboAntigoDump, "")
		e.state("du", "1\t/imagens\n")
		d, err := os.Stat(filepath.Join(e.backups(), "rotaperfumes-"+carimboAntigoDump+".sql.gz"))
		require.NoError(t, err)
		i, err := os.Stat(filepath.Join(e.backups(), "rotaperfumes-imagens-"+carimboAntigoDump+".tar.gz"))
		require.NoError(t, err)
		return e, d.Size(), i.Size()
	}
	t.Run("considera dump e imagens do ultimo par", func(t *testing.T) {
		e, d, i := novo(t)
		e.state("df_livre", fmt.Sprintf("%d\n", 2*d+2*i+gib-1))
		antes := e.arquivos()
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "espaco insuficiente")
		assert.Equal(t, antes, e.arquivos())
		assert.NotContains(t, e.ler("log"), "--entrypoint mariadb-dump", "para antes do dump")

		e.state("df_livre", fmt.Sprintf("%d\n", 2*d+2*i+gib))
		out, rc = e.instalada("--agendado")
		assert.Equal(t, 0, rc, out)
	})
	t.Run("volume maior que o ultimo par", func(t *testing.T) {
		e, d, _ := novo(t)
		volume := 5 * gib
		e.state("du", fmt.Sprintf("%d\t/imagens\n", volume))
		e.state("df_livre", fmt.Sprintf("%d\n", 2*d+volume+gib-1))
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "espaco insuficiente")
	})
}

func TestBACKUP01_Backup_Trava(t *testing.T) {
	e := novoEnsaioBackup(t)
	e.instalarCopia()
	f, err := os.OpenFile(filepath.Join(e.backups(), ".trava"), os.O_CREATE|os.O_RDWR, 0o600)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB))
	out, rc := e.instalada("--agendado")
	assert.Equal(t, 1, rc, out)
	assert.Contains(t, out, "trava")
	assert.NotContains(t, e.ler("log"), "docker rm", "não remove o cliente de quem tem a trava")
}

func TestBACKUP01_Backup_SIGTERM(t *testing.T) {
	e := novoEnsaioBackup(t)
	e.instalarCopia()
	e.par(carimboAntigoDump, "")
	antes := e.arquivos()
	e.state("dump_preso", "")
	cmd := e.comando(copiaInstalada, "--agendado")
	require.NoError(t, cmd.Start())
	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(e.dir, "state/dump_iniciado"))
		return err == nil
	}, 20*time.Second, 50*time.Millisecond)
	inicio := time.Now()
	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
	err := cmd.Wait()
	var ee *exec.ExitError
	require.ErrorAs(t, err, &ee)
	assert.Equal(t, 143, ee.ExitCode())
	assert.Less(t, time.Since(inicio), 15*time.Second, "tratado na hora, não ao fim do pipe")
	e.semTemporarios()
	assert.Equal(t, antes, e.arquivos())
	assert.Equal(t, 2, strings.Count(e.ler("log"), "docker rm -f -- rotaperfumes-backup-cliente"), "início e trap")
}

func TestBACKUP01_Backup_Retencao(t *testing.T) {
	e := novoEnsaioBackup(t)
	e.instalarCopia("RETENCAO_DIARIOS=2\nRETENCAO_MANUAIS=2\n")
	e.par("20200101-033000Z", "")
	e.par("20200102-033000Z", "")
	e.par("20200103-033000Z", "")
	e.par("20200101-040000Z", "-manual")
	// Nada disto entra na contagem nem é apagado.
	imuneDump := "rotaperfumes-20190101-033000Z.sql.gz" // sem imagens: não é par
	parecidos := []string{imuneDump, "rotaperfumes-2019.sql.gz", "pre-restauracao-20190101-033000Z.sql.gz", "parcial-rotaperfumes-20190101-033000Z.sql.gz", ".ultima-verificacao"}
	for _, n := range parecidos {
		escrever(t, filepath.Join(e.backups(), n), "x", 0o600)
	}
	alvo := filepath.Join(e.dir, "alvo")
	escrever(t, alvo, "x", 0o600)
	link := "rotaperfumes-20190102-033000Z.sql.gz"
	require.NoError(t, os.Symlink(alvo, filepath.Join(e.backups(), link)))
	sub := "rotaperfumes-20190103-033000Z.sql.gz"
	require.NoError(t, os.MkdirAll(filepath.Join(e.backups(), sub), 0o700))
	// Datas antigas não apagam nada além da contagem.
	velho := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(filepath.Join(e.backups(), "rotaperfumes-20200103-033000Z.sql.gz"), velho, velho))

	out, rc := e.instalada("--agendado")
	require.Equal(t, 0, rc, out)
	nomes := e.arquivos()
	for _, n := range append(append([]string{}, parecidos...), link, sub) {
		assert.Contains(t, nomes, n)
	}
	assert.FileExists(t, alvo)
	for _, removido := range []string{"20200101-033000Z", "20200102-033000Z"} {
		assert.NotContains(t, nomes, "rotaperfumes-"+removido+".sql.gz")
		assert.NotContains(t, nomes, "rotaperfumes-imagens-"+removido+".tar.gz", "o par sai junto")
	}
	assert.Contains(t, nomes, "rotaperfumes-20200103-033000Z.sql.gz")
	assert.Contains(t, nomes, "rotaperfumes-imagens-20200103-033000Z.tar.gz")
	assert.Contains(t, nomes, "rotaperfumes-20200101-040000Z-manual.sql.gz", "o timer não expulsa a série manual")

	// E a série manual não expulsa as diárias.
	diariasAntes := contarDiarias(nomes)
	out, rc = e.instalada("--agora")
	require.Equal(t, 0, rc, out)
	assert.Equal(t, diariasAntes, contarDiarias(e.arquivos()))
}

func contarDiarias(nomes []string) int {
	re := regexp.MustCompile(`^rotaperfumes-\d{8}-\d{6}Z\.sql\.gz$`)
	n := 0
	for _, x := range nomes {
		if re.MatchString(x) {
			n++
		}
	}
	return n
}

func TestBACKUP01_Backup_Uso(t *testing.T) {
	e := novoEnsaioBackup(t)
	for _, args := range [][]string{
		{}, {"--xyz"}, {"--agora", "--status"}, {"--agora", "--backup-dir", "/x"}, {"--status", "--retencao-diarios", "5"},
		{"--instalar", "--retencao-diarios", "0"}, {"--instalar", "--retencao-manuais", "1"}, {"--instalar", "--retencao-diarios", "-3"},
		{"--instalar", "--retencao-diarios", "abc"}, {"--instalar", "--backup-dir", "/a b"}, {"--instalar", "--backup-dir", "/a/../b"},
		{"--status", "--aceitar-reducao"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, rc := e.checkout(args...)
			assert.Equal(t, 2, rc, out)
		})
	}
	out, rc := e.checkout("--help")
	assert.Equal(t, 0, rc, out)

	// Retenção inválida no conf: recusa.
	e.instalarCopia("RETENCAO_DIARIOS=0\n")
	out, rc = e.instalada("--agendado")
	assert.Equal(t, 1, rc, out)
	assert.Contains(t, out, "RETENCAO_DIARIOS invalida")
	e.instalarCopia("rm -rf /\n")
	out, rc = e.instalada("--agendado")
	assert.Equal(t, 1, rc, out)
	assert.Contains(t, out, "linha nao reconhecida")
}

func TestBACKUP01_Backup_RootNuncaDoCheckout(t *testing.T) {
	e := novoEnsaioBackup(t)
	for _, modo := range []string{"--agora", "--agendado", "--status"} {
		out, rc := e.checkout(modo)
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "nao executa do checkout", modo)
	}
	e.instalarCopia()
	out, rc := e.instalada("--instalar")
	assert.Equal(t, 1, rc, out)
	assert.Contains(t, out, "--instalar roda do checkout")
	require.NoError(t, os.Chmod(filepath.Join(e.dir, "sbin/rotaperfumes-backup-banco"), 0o755))
	out, rc = e.instalada("--agendado")
	assert.Equal(t, 1, rc, out)
	assert.Contains(t, out, "modo 700")
}

func TestBACKUP01_Backup_DestinoRecusado(t *testing.T) {
	casos := []struct {
		nome     string
		preparar func(e *ensaioBackup)
		esperado string
	}{
		{"modo 755", func(e *ensaioBackup) { require.NoError(e.t, os.Chmod(e.backups(), 0o755)) }, "modo 700"},
		{"symlink", func(e *ensaioBackup) {
			require.NoError(e.t, os.RemoveAll(e.backups()))
			require.NoError(e.t, os.MkdirAll(filepath.Join(e.dir, "real"), 0o700))
			require.NoError(e.t, os.Symlink(filepath.Join(e.dir, "real"), e.backups()))
		}, "link simbolico"},
		{"ancestral gravavel", func(e *ensaioBackup) { require.NoError(e.t, os.Chmod(e.dir, 0o777)) }, "escrita para grupo/outros"},
		{"cnf com modo 600", func(e *ensaioBackup) {
			require.NoError(e.t, os.Chmod(filepath.Join(e.dir, "root/rotaperfumes-mariadb/backup-cliente/rotaperfumes_backup.cnf"), 0o600))
		}, "modo 400"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia()
			c.preparar(e)
			out, rc := e.instalada("--agendado")
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, c.esperado)
			assert.Empty(t, e.ler("log"), "recusa antes do docker")
			require.NoError(t, os.Chmod(e.dir, 0o700))
		})
	}
	t.Run("dentro dos segredos", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		out, rc := e.checkout("--instalar", "--backup-dir", "/root/rotaperfumes-mariadb/backups")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "disjunto")
	})
}

func TestBACKUP01_Backup_Instalar(t *testing.T) {
	e := novoEnsaioBackup(t)
	out, rc := e.checkout("--instalar", "--backup-dir", e.backups(), "--retencao-diarios", "7", "--retencao-manuais", "4")
	require.Equal(t, 0, rc, out+e.ler("unit.log"))

	assert.Equal(t, lerRaiz(t, "deploy", "backup-banco.sh"), e.ler("sbin/rotaperfumes-backup-banco"))
	st, err := os.Stat(filepath.Join(e.dir, "sbin/rotaperfumes-backup-banco"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), st.Mode().Perm())
	assert.Equal(t, "BACKUP_DIR="+e.backups()+"\nRETENCAO_DIARIOS=7\nRETENCAO_MANUAIS=4\n", e.ler("etcrp/backup-banco.conf"))

	svc := e.ler("systemd/rotaperfumes-backup-banco.service")
	for _, s := range []string{"Requires=docker.service", "After=docker.service", "Type=oneshot", "UMask=0077", "NoNewPrivileges=yes", "PrivateTmp=yes",
		"ProtectSystem=strict", "ReadWritePaths=" + e.backups() + "\n", "TimeoutStartSec=2h", "Restart=on-failure", "RestartSec=15min",
		"StartLimitBurst=3", "ExecStart=/usr/local/sbin/rotaperfumes-backup-banco --agendado\n"} {
		assert.Contains(t, svc, s)
	}
	tim := e.ler("systemd/rotaperfumes-backup-banco.timer")
	for _, s := range []string{"OnCalendar=*-*-* 03:30", "Persistent=true", "RandomizedDelaySec=15m", "WantedBy=timers.target"} {
		assert.Contains(t, tim, s)
	}
	log := e.ler("log")
	assert.Less(t, strings.Index(log, "systemctl start --no-block"), strings.Index(log, "systemctl enable rotaperfumes-backup-banco.timer"), "timer depois do 1o backup")
	assert.Contains(t, log, "systemctl restart rotaperfumes-backup-banco.timer")
	assert.Len(t, e.arquivos(), 2, "o 1o par")

	t.Run("reinstalar mantem a escolha", func(t *testing.T) {
		time.Sleep(1100 * time.Millisecond) // outro carimbo: o nome tem resolução de 1 s
		out, rc := e.checkout("--instalar")
		require.Equal(t, 0, rc, out)
		assert.Contains(t, e.ler("etcrp/backup-banco.conf"), "RETENCAO_DIARIOS=7\n")
	})
}

func TestBACKUP01_Backup_InstalarFalha(t *testing.T) {
	t.Run("1o backup falha: timer nao liga", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.state("dump_rc", "1")
		out, rc := e.checkout("--instalar", "--backup-dir", e.backups())
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "o 1o backup falhou")
		assert.Contains(t, e.ler("log"), "systemctl stop rotaperfumes-backup-banco.service", "cancela as novas tentativas")
		assert.NotContains(t, e.ler("log"), "systemctl enable")
	})
	t.Run("sem volume: recusa antes de instalar", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.state("sem_volume", "")
		out, rc := e.checkout("--instalar", "--backup-dir", e.backups())
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "rode o job do Jenkins")
		assert.NoFileExists(t, filepath.Join(e.dir, "sbin/rotaperfumes-backup-banco"))
	})
}

// O mountpoint é o real: o disco simulado é um tmpfs montado só na execução.
func TestBACKUP01_Backup_ExigirMontagem(t *testing.T) {
	novo := func(t *testing.T) (*ensaioBackup, string) {
		e := novoEnsaioBackup(t)
		ssd := filepath.Join(e.dir, "ssd")
		require.NoError(t, os.MkdirAll(ssd, 0o700))
		return e, ssd
	}
	montado := func(e *ensaioBackup, ssd, modo, script string, args ...string) (string, int) {
		e.t.Helper()
		cmd := e.comando(script, args...)
		cmd.Env = append(cmd.Env, "MONTAR="+ssd, "MONTAR_MODO="+modo)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return string(out), 0
		}
		ee, ok := err.(*exec.ExitError)
		require.True(e.t, ok, "%v", err)
		return string(out), ee.ExitCode()
	}
	vazio := func(t *testing.T, dir string) {
		t.Helper()
		ents, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Empty(t, ents, "nada no disco do sistema")
	}

	t.Run("desmontado falha sem gravar no disco do sistema", func(t *testing.T) {
		e, ssd := novo(t)
		e.instalarCopia()
		escrever(t, filepath.Join(e.dir, "etcrp/backup-banco.conf"), "BACKUP_DIR="+ssd+"\nEXIGIR_MONTAGEM="+ssd+"\n", 0o644)
		for _, modo := range []string{"--agendado", "--agora", "--status"} {
			out, rc := e.instalada(modo)
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, "nao esta montado", modo)
		}
		vazio(t, ssd)
		assert.Empty(t, e.ler("log"), "recusa antes do docker")
	})
	t.Run("montado grava no disco e o espaco segue o destino", func(t *testing.T) {
		e, ssd := novo(t)
		e.instalarCopia()
		escrever(t, filepath.Join(e.dir, "etcrp/backup-banco.conf"), "BACKUP_DIR="+ssd+"\nEXIGIR_MONTAGEM="+ssd+"\n", 0o644)
		out, rc := montado(e, ssd, "700", copiaInstalada, "--agendado")
		require.Equal(t, 0, rc, out)
		assert.Contains(t, out, "dump="+ssd+"/rotaperfumes-")
		assert.Contains(t, e.ler("log"), "df --output=avail -B1 -- "+ssd)
		vazio(t, ssd)
	})
	t.Run("instalar desmontado nao cria o diretorio", func(t *testing.T) {
		e, ssd := novo(t)
		out, rc := e.checkout("--instalar", "--backup-dir", ssd+"/backups", "--exigir-montagem", ssd)
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "nao esta montado")
		vazio(t, ssd)
		assert.NoFileExists(t, filepath.Join(e.dir, "sbin/rotaperfumes-backup-banco"))
	})
	t.Run("instalar montado: unit e conf seguem o destino", func(t *testing.T) {
		e, ssd := novo(t)
		dest := ssd + "/backups"
		out, rc := montado(e, ssd, "755", caminhoRaiz(t, "deploy", "backup-banco.sh"), "--instalar", "--backup-dir", dest, "--exigir-montagem", ssd)
		require.Equal(t, 0, rc, out+e.ler("unit.log"))
		svc := e.ler("systemd/rotaperfumes-backup-banco.service")
		assert.Contains(t, svc, "RequiresMountsFor="+dest+"\n")
		assert.Contains(t, svc, "ReadWritePaths="+dest+"\n")
		assert.Contains(t, svc, "ProtectSystem=strict\n")
		conf := e.ler("etcrp/backup-banco.conf")
		assert.Contains(t, conf, "BACKUP_DIR="+dest+"\n")
		assert.Contains(t, conf, "EXIGIR_MONTAGEM="+ssd+"\n")
		vazio(t, ssd)
		assert.Contains(t, e.ler("log"), "df --output=avail -B1 -- "+dest)

		t.Run("sem-exigir-montagem limpa o conf e a unit", func(t *testing.T) {
			out, rc := e.checkout("--instalar", "--backup-dir", e.backups(), "--sem-exigir-montagem")
			require.Equal(t, 0, rc, out+e.ler("unit.log"))
			assert.NotContains(t, e.ler("etcrp/backup-banco.conf"), "EXIGIR_MONTAGEM")
			assert.NotContains(t, e.ler("systemd/rotaperfumes-backup-banco.service"), "RequiresMountsFor")
		})
	})
	t.Run("sem a opcao a unit nao exige montagem", func(t *testing.T) {
		e, _ := novo(t)
		out, rc := e.checkout("--instalar", "--backup-dir", e.backups())
		require.Equal(t, 0, rc, out+e.ler("unit.log"))
		assert.NotContains(t, e.ler("systemd/rotaperfumes-backup-banco.service"), "RequiresMountsFor")
		assert.NotContains(t, e.ler("etcrp/backup-banco.conf"), "EXIGIR_MONTAGEM")
	})
	t.Run("destino fora do ponto de montagem", func(t *testing.T) {
		e, ssd := novo(t)
		out, rc := e.checkout("--instalar", "--backup-dir", e.backups(), "--exigir-montagem", ssd)
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "precisa ficar dentro de")
	})
	t.Run("uso incorreto", func(t *testing.T) {
		e, ssd := novo(t)
		for _, args := range [][]string{
			{"--agora", "--exigir-montagem", ssd}, {"--status", "--sem-exigir-montagem"},
			{"--instalar", "--exigir-montagem", ssd, "--sem-exigir-montagem"},
			{"--instalar", "--exigir-montagem", "/"}, {"--instalar", "--exigir-montagem", ssd + "/"}, {"--instalar", "--exigir-montagem", "rel"},
		} {
			out, rc := e.checkout(args...)
			assert.Equal(t, 2, rc, "%v: %s", args, out)
		}
	})
}

func TestBACKUP01_Backup_Status(t *testing.T) {
	bom := func(t *testing.T) *ensaioBackup {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.par(time.Now().UTC().Format("20060102-150405Z"), "")
		escrever(t, filepath.Join(e.backups(), ".ultima-verificacao"),
			fmt.Sprintf("epoch=%d\narquivo=x\nsha256=y\n", time.Now().Unix()), 0o600)
		return e
	}
	t.Run("ok", func(t *testing.T) {
		out, rc := bom(t).instalada("--status")
		assert.Equal(t, 0, rc, out)
		assert.Contains(t, out, "STATUS: ok")
		assert.Contains(t, out, "Imagens: 1 arquivo(s), 1 bytes")
		assert.NotContains(t, out, "foto-1.jpg", "só contagens")
		assert.NotContains(t, out, "AVISO")
	})
	falhas := []struct {
		nome     string
		preparar func(e *ensaioBackup)
		esperado string
	}{
		{"timer parado", func(e *ensaioBackup) { e.state("timer_parado", "") }, "timer"},
		{"ultima execucao falhou", func(e *ensaioBackup) { e.state("result", "exit-code\n") }, "Result=exit-code"},
		{"backup antigo", func(e *ensaioBackup) {
			velho := time.Now().Add(-27 * time.Hour)
			matches, _ := filepath.Glob(filepath.Join(e.backups(), "rotaperfumes-*.sql.gz"))
			for _, m := range matches {
				require.NoError(e.t, os.Chtimes(m, velho, velho))
			}
		}, "mais de 26 h"},
		{"sem par", func(e *ensaioBackup) {
			matches, _ := filepath.Glob(filepath.Join(e.backups(), "rotaperfumes-imagens-*.tar.gz"))
			for _, m := range matches {
				require.NoError(e.t, os.Remove(m))
			}
		}, "nenhum backup bom"},
		{"tar com entrada perigosa", func(e *ensaioBackup) {
			matches, _ := filepath.Glob(filepath.Join(e.backups(), "rotaperfumes-imagens-*.tar.gz"))
			require.Len(e.t, matches, 1)
			escrever(e.t, matches[0], comprimir(e.t, tarSintetico(e.t, map[string]string{"../fora.jpg": "x"})), 0o600)
		}, "entrada(s) com caminho absoluto"},
	}
	for _, c := range falhas {
		t.Run(c.nome, func(t *testing.T) {
			e := bom(t)
			c.preparar(e)
			out, rc := e.instalada("--status")
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, c.esperado)
		})
	}
	t.Run("verificacao antiga so avisa", func(t *testing.T) {
		e := bom(t)
		escrever(t, filepath.Join(e.backups(), ".ultima-verificacao"), "epoch=1000\n", 0o600)
		out, rc := e.instalada("--status")
		assert.Equal(t, 0, rc, out)
		assert.Contains(t, out, "mais de 35 dias")
	})
	t.Run("parcial e dump sem par: avisa", func(t *testing.T) {
		e := bom(t)
		escrever(t, filepath.Join(e.backups(), "rotaperfumes-20200101-033000Z.sql.gz"), "x", 0o600)
		out, rc := e.instalada("--status")
		assert.Equal(t, 0, rc, out)
		assert.Contains(t, out, "1 arquivo(s) sem o outro lado do par")
	})
}

func TestBACKUP01_Backup_ConferirImagens(t *testing.T) {
	precisaBash(t)
	ferramentaObrigatoria(t, "tar")
	ferramentaObrigatoria(t, "gzip")
	script := caminhoRaiz(t, "deploy", "backup-banco.sh")
	dir := t.TempDir()
	rodar := func(arq string) (string, int) {
		cmd := exec.Command("bash", script, "--conferir-imagens", arq)
		cmd.Env = ambienteLimpo()
		out, err := cmd.CombinedOutput()
		if err == nil {
			return string(out), 0
		}
		ee, ok := err.(*exec.ExitError)
		require.True(t, ok, "%v", err)
		return string(out), ee.ExitCode()
	}
	gravar := func(nome string, conteudo string) string {
		p := filepath.Join(dir, nome)
		escrever(t, p, conteudo, 0o600)
		return p
	}

	bom := gravar("bom.tar.gz", comprimir(t, tarSintetico(t, map[string]string{"NOME_SENTINELA.jpg": "12345", "dir/b.jpg": "678"})))
	out, rc := rodar(bom)
	require.Equal(t, 0, rc, out)
	assert.Contains(t, out, "Arquivos: 2\nBytes: 8\nAPROVADO\n")
	assert.NotContains(t, out, "NOME_SENTINELA")

	var comLink bytes.Buffer
	w := tar.NewWriter(&comLink)
	require.NoError(t, w.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0o777}))
	require.NoError(t, w.Close())

	for nome, c := range map[string]struct{ conteudo, esperado string }{
		"dotdot.tar.gz":   {comprimir(t, tarSintetico(t, map[string]string{"a/../../b": "x"})), "caminho absoluto"},
		"absoluto.tar.gz": {comprimir(t, tarSintetico(t, map[string]string{"/etc/x": "x"})), "caminho absoluto"},
		"link.tar.gz":     {comprimir(t, comLink.String()), "nao sao arquivo nem diretorio"},
		"truncado.tar.gz": {comprimir(t, tarSintetico(t, map[string]string{"a": "x"}))[:20], "gzip invalido"},
		"curto.tar.gz":    {comprimir(t, "isto nao e um tar valido, nem de longe"), "sem nenhuma entrada"},
		"lixo.tar.gz":     {comprimir(t, strings.Repeat("isto nao e um tar valido\n", 100)), "listagem"},
	} {
		t.Run(nome, func(t *testing.T) {
			out, rc := rodar(gravar(nome, c.conteudo))
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, c.esperado)
			assert.NotContains(t, out, "/etc/", "nomes fora da saída")
		})
	}
	t.Run("symlink no argumento", func(t *testing.T) {
		l := filepath.Join(dir, "l.tar.gz")
		require.NoError(t, os.Symlink(bom, l))
		out, rc := rodar(l)
		assert.Equal(t, 1, rc, out)
	})
}

// ---------------------------------------------------------------------------
// Guarda do --recriar
// ---------------------------------------------------------------------------
func carimboAgora(d time.Duration) string {
	return time.Now().Add(d).UTC().Format("20060102-150405Z")
}

func novaGuarda(t *testing.T) (*ensaio, string) {
	t.Helper()
	e := novoEnsaio(t)
	r := e.rodar("--nova-ca")
	require.NoError(t, r.err, r.out)
	require.NoError(t, os.MkdirAll(filepath.Join(e.dir, "dados", "mysql"), 0o700))
	dir := filepath.Join(e.dir, "backups")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	return e, dir
}

func dumpRecente(t *testing.T, dir, nome string) string {
	t.Helper()
	p := filepath.Join(dir, nome)
	escrever(t, p, comprimir(t, dumpBackupBom()), 0o600)
	return p
}

func TestBACKUP01_GuardaRecriar_Recusas(t *testing.T) {
	casos := []struct {
		nome     string
		preparar func(t *testing.T, dir string)
		esperado string
	}{
		{"sem backup", func(*testing.T, string) {}, "nenhum backup"},
		{"antigo", func(t *testing.T, dir string) {
			p := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(-2*time.Hour)+".sql.gz")
			velho := time.Now().Add(-2 * time.Hour)
			require.NoError(t, os.Chtimes(p, velho, velho))
		}, "tem mais de 60 min"},
		{"carimbo antigo com mtime novo", func(t *testing.T, dir string) {
			dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(-2*time.Hour)+".sql.gz")
		}, "tem mais de 60 min"},
		{"data futura", func(t *testing.T, dir string) {
			dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(2*time.Hour)+".sql.gz")
		}, "data futura"},
		{"modo 644", func(t *testing.T, dir string) {
			require.NoError(t, os.Chmod(dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz"), 0o644))
		}, "modo 600"},
		{"symlink", func(t *testing.T, dir string) {
			alvo := dumpRecente(t, filepath.Dir(dir), "alvo.sql.gz")
			require.NoError(t, os.Symlink(alvo, filepath.Join(dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz")))
		}, "nao e arquivo regular"},
		{"corrompido", func(t *testing.T, dir string) {
			escrever(t, filepath.Join(dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz"), "nao e gzip", 0o600)
		}, "corrompido"},
		{"sem marcador", func(t *testing.T, dir string) {
			escrever(t, filepath.Join(dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz"), comprimir(t, "CREATE TABLE x;\n"), 0o600)
		}, "marcador"},
		{"pre-restauracao nao vale", func(t *testing.T, dir string) {
			dumpRecente(t, dir, "pre-restauracao-"+carimboAgora(0)+".sql.gz")
		}, "nenhum backup"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e, dir := novaGuarda(t)
			c.preparar(t, dir)
			r := e.rodar("--recriar", "--backup-dir", dir)
			require.Error(t, r.err, r.out)
			assert.Contains(t, r.out, c.esperado)
			assert.Contains(t, r.out, "sudo rotaperfumes-backup-banco --agora")
			assert.NotContains(t, e.log(), "docker rm", "sem docker rm")
			assert.NotContains(t, e.log(), "docker build", "nada mudado antes")
			assert.NoFileExists(t, filepath.Join(e.dir, segredos, "recriar.registro"))
		})
	}
	t.Run("diretorio 755", func(t *testing.T) {
		e, dir := novaGuarda(t)
		dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz")
		require.NoError(t, os.Chmod(dir, 0o755))
		r := e.rodar("--recriar", "--backup-dir", dir)
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "modo 700")
		assert.NotContains(t, e.log(), "docker rm")
	})
}

func TestBACKUP01_GuardaRecriar_Aceita(t *testing.T) {
	registro := func(e *ensaio) string { return e.arq(filepath.Join(segredos, "recriar.registro")) }
	t.Run("backup recente sem exigir o arquivo de imagens", func(t *testing.T) {
		e, dir := novaGuarda(t)
		p := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(0)+"-manual.sql.gz")
		r := e.rodar("--recriar", "--backup-dir", dir)
		require.NoError(t, r.err, r.out)
		assert.Contains(t, e.log(), "docker rm -f rotaperfumes-mariadb")
		assert.Contains(t, registro(e), "backup="+p)
		assert.Contains(t, registro(e), "imagem_base=mariadb:11.4.13@sha256:")
		st, err := os.Stat(filepath.Join(e.dir, segredos, "recriar.registro"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	})
	t.Run("parcial vale, com aviso (BACKUP-04)", func(t *testing.T) {
		e, dir := novaGuarda(t)
		p := dumpRecente(t, dir, "parcial-rotaperfumes-"+carimboAgora(0)+".sql.gz")
		r := e.rodar("--recriar", "--backup-dir", dir)
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "o backup usado e parcial (parcial-rotaperfumes-")
		assert.Contains(t, e.log(), "docker rm -f rotaperfumes-mariadb")
		assert.Contains(t, registro(e), "backup="+p)
	})
	t.Run("janela configuravel ate 60", func(t *testing.T) {
		e, dir := novaGuarda(t)
		p := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(-50*time.Minute)+".sql.gz")
		velho := time.Now().Add(-50 * time.Minute)
		require.NoError(t, os.Chtimes(p, velho, velho))
		r := e.rodar("--recriar", "--backup-dir", dir, "--backup-janela-min", "40")
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "tem mais de 40 min")
		r = e.rodar("--recriar", "--backup-dir", dir, "--backup-janela-min", "60")
		require.NoError(t, r.err, r.out)
		assert.Contains(t, registro(e), "janela=60 backup="+p)
	})
	t.Run("motivo registrado antes do docker rm", func(t *testing.T) {
		e, dir := novaGuarda(t)
		escrever(t, filepath.Join(e.dir, "bin", "docker"), strings.Replace(stubDocker,
			`rm) rm -f "$S/criado"; exit 0 ;;`,
			`rm) cp "$FAKE/root/rotaperfumes-mariadb/recriar.registro" "$S/registro_no_rm"; rm -f "$S/criado"; exit 0 ;;`, 1), 0o755)
		motivo := "container parado, sem como gerar dump novo"
		r := e.rodar("--recriar", "--backup-dir", dir, "--sem-backup-motivo", motivo)
		require.NoError(t, r.err, r.out)
		assert.Contains(t, e.state("registro_no_rm"), "motivo="+motivo)
		assert.Contains(t, r.out, "AVISO: --sem-backup-motivo")
	})
	t.Run("variavel de ambiente nao ignora", func(t *testing.T) {
		e, dir := novaGuarda(t)
		cmd := exec.Command("unshare", "-rm", "bash", filepath.Join(e.dir, "runner.sh"),
			"--ssl-dir", filepath.Join(e.dir, "ssl"), "--dados", filepath.Join(e.dir, "dados"), "--host-nome", hostTeste,
			"--recriar", "--backup-dir", dir)
		cmd.Env = append(ambienteLimpo(), "FAKE="+e.dir, "SCRIPT="+mariadbDocker(t),
			"SEM_BACKUP_MOTIVO=variavel do ambiente aqui", "BACKUP_JANELA_MIN=9999", "RECRIAR=false")
		out, err := cmd.CombinedOutput()
		require.Error(t, err, string(out))
		assert.Contains(t, string(out), "nenhum backup")
	})
	t.Run("sem dados: a guarda nao dispara", func(t *testing.T) {
		e := novoEnsaio(t)
		require.NoError(t, e.rodar("--nova-ca").err)
		dirInvalido := filepath.Join(e.dir, "nao-existe")
		r := e.rodar("--recriar", "--backup-dir", dirInvalido)
		require.NoError(t, r.err, r.out)
		assert.Contains(t, registro(e), "janela=60 backup=nenhum motivo=nenhum")
	})
	t.Run("reexecucao sem --recriar nao dispara", func(t *testing.T) {
		e, _ := novaGuarda(t)
		r := e.rodar()
		require.NoError(t, r.err, r.out)
		assert.NotContains(t, r.out, "Backup recente conferido")
	})
}

func TestBACKUP01_GuardaRecriar_MotivoInvalido(t *testing.T) {
	e := novoEnsaio(t)
	for nome, args := range map[string][]string{
		"sem --recriar":  {"--sem-backup-motivo", "motivo com tamanho bom"},
		"curto":          {"--recriar", "--sem-backup-motivo", "curto"},
		"quebra":         {"--recriar", "--sem-backup-motivo", "motivo com\nquebra de linha"},
		"longo":          {"--recriar", "--sem-backup-motivo", strings.Repeat("x", 301)},
		"janela zero":    {"--recriar", "--backup-janela-min", "0"},
		"janela grande":  {"--recriar", "--backup-janela-min", "1441"},
		"dir com espaco": {"--recriar", "--backup-dir", "/a b"},
	} {
		t.Run(nome, func(t *testing.T) {
			r := e.rodar(args...)
			require.Error(t, r.err, r.out)
			assert.NotContains(t, e.log(), "docker build")
		})
	}
}

// ---------------------------------------------------------------------------
// Guarda do seed --forcar (D6)
// ---------------------------------------------------------------------------
func TestBACKUP01_GuardaSeedForcar(t *testing.T) {
	semMudanca := func(t *testing.T, e *ensaioSeed, out string, err error) {
		t.Helper()
		require.Error(t, err, out)
		assert.NotContains(t, out, "=== [3/7]")
		e.semSQLDeEscrita(t)
	}
	t.Run("sem backup recusa antes de perguntar", func(t *testing.T) {
		e := novoEnsaioTTY(t)
		e.state("linhas", "5\n")
		dir := filepath.Join(e.dir, "backups")
		require.NoError(t, os.MkdirAll(dir, 0o700))
		s := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-dir", dir)
		out, err := s.terminar()
		semMudanca(t, e, out, err)
		assert.Contains(t, out, "nenhum backup")
		assert.NotContains(t, out, promptNomeDoBanco)
	})
	t.Run("backup recente segue e registra", func(t *testing.T) {
		e := novoEnsaioTTY(t)
		e.state("linhas", "5\n")
		dir := filepath.Join(e.dir, "backups")
		require.NoError(t, os.MkdirAll(dir, 0o700))
		p := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz")
		s := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-dir", dir)
		s.digitar(promptNomeDoBanco, "rotaperfumes")
		out, err := s.terminar()
		require.NoError(t, err, out)
		assert.Contains(t, e.ler("root/rotaperfumes-mariadb/seed-forcar.registro"), "backup="+p)
	})
	t.Run("parcial recente segue, avisa e registra (BACKUP-04)", func(t *testing.T) {
		e := novoEnsaioTTY(t)
		e.state("linhas", "5\n")
		dir := filepath.Join(e.dir, "backups")
		require.NoError(t, os.MkdirAll(dir, 0o700))
		p := dumpRecente(t, dir, "parcial-rotaperfumes-"+carimboAgora(0)+".sql.gz")
		s := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-dir", dir)
		s.digitar(promptNomeDoBanco, "rotaperfumes")
		out, err := s.terminar()
		require.NoError(t, err, out)
		assert.Contains(t, out, "o backup usado e parcial (parcial-rotaperfumes-")
		assert.Contains(t, e.ler("root/rotaperfumes-mariadb/seed-forcar.registro"), "backup="+p)
	})
	t.Run("banco vazio nao exige backup", func(t *testing.T) {
		e := novoEnsaioTTY(t)
		s := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-dir", filepath.Join(e.dir, "nao-existe"))
		out, err := s.terminar()
		require.NoError(t, err, out)
	})
	t.Run("motivo sem --forcar e recusado", func(t *testing.T) {
		e := novoEnsaioSeed(t)
		out, err := e.rodar("--pepper-file", "/root/pepper.txt", "--sem-backup-motivo", "motivo com tamanho bom")
		require.Error(t, err, out)
		assert.Contains(t, out, "--sem-backup-motivo so com --forcar")
		assert.Empty(t, e.ler("log"))
	})
}

// ---------------------------------------------------------------------------
// Achados do TestBrain: carimbo futuro, status só pelo mtime, par corrompido.
// ---------------------------------------------------------------------------
func (e *ensaioBackup) pares() []string {
	e.t.Helper()
	var out []string
	for _, n := range e.arquivos() {
		if regexp.MustCompile(`^rotaperfumes-\d{8}-\d{6}Z(-manual)?\.sql\.gz$`).MatchString(n) {
			out = append(out, n)
		}
	}
	return out
}

func (e *ensaioBackup) caminhoDump(carimbo string) string {
	return filepath.Join(e.backups(), "rotaperfumes-"+carimbo+".sql.gz")
}

func TestBACKUP01_Retencao_DataFutura(t *testing.T) {
	semApagar := func(t *testing.T, e *ensaioBackup, antes []string, out string, rc int, esperado string) {
		t.Helper()
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, esperado)
		assert.Contains(t, out, "nada foi apagado")
		assert.NotContains(t, out, "par removido")
		assert.NotContains(t, out, "backup: ok")
		assert.Subset(t, e.arquivos(), antes, "nada apagado")
		assert.Len(t, e.pares(), len(antes)/2+1, "o par novo fica")
		var novo string
		for _, n := range e.pares() {
			if !strings.Contains(strings.Join(antes, " "), n) {
				novo = n
			}
		}
		require.NotEmpty(t, novo, "par novo publicado")
		cAtual := strings.TrimSuffix(strings.TrimPrefix(novo, "rotaperfumes-"), ".sql.gz")
		assert.FileExists(t, filepath.Join(e.backups(), "rotaperfumes-imagens-"+cAtual+".tar.gz"))
	}
	t.Run("dois pares futuros: reproducao do TestBrain", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia("RETENCAO_DIARIOS=2\n")
		e.par("20991231-235958Z", "")
		e.par("20991231-235959Z", "")
		antes := e.arquivos()
		out, rc := e.instalada("--agendado")
		semApagar(t, e, antes, out, rc, "rotaperfumes-20991231-235958Z.sql.gz tem data futura")
	})
	t.Run("par futuro falha mesmo sem nada a apagar", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.par("20991231-235959Z", "-manual")
		antes := e.arquivos()
		out, rc := e.instalada("--agendado")
		semApagar(t, e, antes, out, rc, "rotaperfumes-20991231-235959Z-manual.sql.gz tem data futura")
	})
	t.Run("mtime futuro tambem falha", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.par(carimboAntigoDump, "")
		futuro := time.Now().Add(2 * time.Hour)
		require.NoError(t, os.Chtimes(e.caminhoDump(carimboAntigoDump), futuro, futuro))
		antes := e.arquivos()
		out, rc := e.instalada("--agendado")
		semApagar(t, e, antes, out, rc, "rotaperfumes-"+carimboAntigoDump+".sql.gz tem data futura")
	})
	t.Run("carimbo que nao existe falha", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.par("20261399-033000Z", "")
		antes := e.arquivos()
		out, rc := e.instalada("--agendado")
		semApagar(t, e, antes, out, rc, "rotaperfumes-20261399-033000Z.sql.gz tem carimbo invalido")
	})
	t.Run("dentro da folga do relogio passa", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.par(carimboAgora(time.Minute), "")
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 0, rc, out)
	})
	t.Run("nunca apaga o par recem-publicado", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia("RETENCAO_DIARIOS=2\n")
		e.par(carimboAgora(2*time.Minute), "")
		e.par(carimboAgora(3*time.Minute), "")
		antes := e.arquivos()
		out, rc := e.instalada("--agendado")
		semApagar(t, e, antes, out, rc, "apagaria o par recem-publicado")
	})
}

func TestBACKUP01_Retencao_ParCorrompido(t *testing.T) {
	const antigo, meio, recente = "20260101-033000Z", "20260102-033000Z", "20260103-033000Z"
	semMarcador := comprimir(t, strings.TrimSuffix(dumpBackupBom(), marcadorDump))
	novo := func(t *testing.T) *ensaioBackup {
		e := novoEnsaioBackup(t)
		e.instalarCopia("RETENCAO_DIARIOS=2\n")
		for _, c := range []string{antigo, meio, recente} {
			e.par(c, "")
		}
		return e
	}
	for nome, c := range map[string]struct{ arquivo, conteudo, esperado string }{
		"dump com gzip ruim":    {"rotaperfumes-" + recente + ".sql.gz", "nao e gzip", "gzip invalido"},
		"dump sem marcador":     {"rotaperfumes-" + recente + ".sql.gz", semMarcador, "sem o marcador"},
		"imagens com gzip ruim": {"rotaperfumes-imagens-" + recente + ".tar.gz", "nao e gzip", "imagens com gzip invalido"},
	} {
		t.Run(nome, func(t *testing.T) {
			e := novo(t)
			escrever(t, filepath.Join(e.backups(), c.arquivo), c.conteudo, 0o600)
			antes := e.arquivos()
			out, rc := e.instalada("--agendado")
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, "o par rotaperfumes-"+recente+".sql.gz foi reprovado")
			assert.Contains(t, out, c.esperado)
			assert.NotContains(t, out, "par removido")
			assert.Subset(t, e.arquivos(), antes, "nada apagado")
			assert.Len(t, e.pares(), 4, "o par novo fica")
		})
	}
	t.Run("corrompido e o mais novo no carimbo, dentro da folga", func(t *testing.T) {
		e := novo(t)
		c := carimboAgora(time.Minute)
		e.par(c, "")
		escrever(t, e.caminhoDump(c), "nao e gzip", 0o600)
		antes := e.arquivos()
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "o par rotaperfumes-"+c+".sql.gz foi reprovado")
		assert.Subset(t, e.arquivos(), antes)
	})
	t.Run("pares antigos nao sao relidos", func(t *testing.T) {
		e := novo(t)
		escrever(t, e.caminhoDump(meio), "nao e gzip", 0o600)
		out, rc := e.instalada("--agendado")
		require.Equal(t, 0, rc, out)
		assert.NoFileExists(t, e.caminhoDump(antigo))
		assert.NoFileExists(t, e.caminhoDump(meio))
		assert.FileExists(t, e.caminhoDump(recente))
	})
	t.Run("sem nada a apagar nao rele e nao falha", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.par(recente, "")
		escrever(t, e.caminhoDump(recente), "nao e gzip", 0o600)
		out, rc := e.instalada("--agendado")
		assert.Equal(t, 0, rc, out)
	})
}

func TestBACKUP01_Status_CarimboEDump(t *testing.T) {
	verificado := func(e *ensaioBackup) {
		escrever(e.t, filepath.Join(e.backups(), ".ultima-verificacao"), fmt.Sprintf("epoch=%d\n", time.Now().Unix()), 0o600)
	}
	novo := func(t *testing.T, carimbo string) *ensaioBackup {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		e.par(carimbo, "")
		verificado(e)
		return e
	}
	t.Run("carimbo antigo com mtime atual: reproducao do TestBrain", func(t *testing.T) {
		out, rc := novo(t, "20200101-033000Z").instalada("--status")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "rotaperfumes-20200101-033000Z.sql.gz) tem mais de 26 h")
		assert.NotContains(t, out, "STATUS: ok")
	})
	t.Run("carimbo futuro", func(t *testing.T) {
		out, rc := novo(t, "20991231-235959Z").instalada("--status")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "rotaperfumes-20991231-235959Z.sql.gz) tem data futura")
		assert.NotContains(t, out, "min atras")
	})
	t.Run("carimbo que nao existe", func(t *testing.T) {
		out, rc := novo(t, "20261399-033000Z").instalada("--status")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "carimbo invalido")
	})
	t.Run("carimbo recente com mtime antigo", func(t *testing.T) {
		c := carimboAgora(0)
		e := novo(t, c)
		velho := time.Now().Add(-27 * time.Hour)
		require.NoError(t, os.Chtimes(e.caminhoDump(c), velho, velho))
		out, rc := e.instalada("--status")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "tem mais de 26 h")
	})
	t.Run("dentro da folga do relogio", func(t *testing.T) {
		out, rc := novo(t, carimboAgora(time.Minute)).instalada("--status")
		assert.Equal(t, 0, rc, out)
		assert.Contains(t, out, "(0 min atras")
	})
	semMarcador := strings.TrimSuffix(dumpBackupBom(), marcadorDump)
	for nome, c := range map[string]struct{ conteudo, esperado string }{
		"gzip ruim":        {"nao e gzip", "gzip invalido"},
		"sem marcador":     {comprimir(t, semMarcador), "sem o marcador"},
		"marcador no meio": {comprimir(t, marcadorDump+semMarcador), "sem o marcador"},
	} {
		t.Run("dump: "+nome, func(t *testing.T) {
			cb := carimboAgora(0)
			e := novo(t, cb)
			escrever(t, e.caminhoDump(cb), c.conteudo, 0o600)
			out, rc := e.instalada("--status")
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, "foi reprovado")
			assert.Contains(t, out, c.esperado)
		})
	}
	t.Run("dump grande sem falso negativo por SIGPIPE", func(t *testing.T) {
		cb := carimboAgora(0)
		e := novo(t, cb)
		var b strings.Builder
		for i := 0; i < 400000; i++ {
			fmt.Fprintf(&b, "(%d,'linha sintetica de preenchimento'),\n", i)
		}
		escrever(t, e.caminhoDump(cb), comprimir(t, dumpBackupBom()+b.String()+marcadorDump), 0o600)
		out, rc := e.instalada("--status")
		assert.Equal(t, 0, rc, out)
	})
	t.Run("verificacao com data futura avisa sem dia negativo", func(t *testing.T) {
		e := novo(t, carimboAgora(0))
		escrever(t, filepath.Join(e.backups(), ".ultima-verificacao"), "epoch=9999999999\n", 0o600)
		out, rc := e.instalada("--status")
		assert.Equal(t, 0, rc, out)
		assert.Contains(t, out, "AVISO: a ultima verificacao tem data futura")
		assert.NotContains(t, out, "ha -")
		assert.NotContains(t, out, "Ultima verificacao: ha")
	})
}

// ---------------------------------------------------------------------------
// BACKUP-02: OK visível do marcador. O texto é fixo: nada do dump vai para a saída.
// ---------------------------------------------------------------------------
const (
	okMarcadorBackup = "backup: dump validado: marcador '-- Dump completed' na ultima linha: OK\n"
	okMarcadorStatus = "Dump validado: marcador '-- Dump completed' na ultima linha: OK\n"
	// Última linha válida para o script, mas que não pode aparecer na saída.
	ultimaLinhaSentinela = "-- Dump completed SENTINELA_ULTIMA_LINHA\n"
)

func dumpComUltimaLinhaSentinela() string {
	return strings.TrimSuffix(dumpBackupBom(), marcadorDump) + ultimaLinhaSentinela
}

func semConteudoDoDump(t *testing.T, out string) {
	t.Helper()
	assert.NotContains(t, out, "SENTINELA_ULTIMA_LINHA", "a última linha do dump fica fora da saída")
	assert.NotContains(t, out, "dado de cliente", "linha do dump fora da saída")
	assert.NotContains(t, out, "-- Dump completed on", "linha do dump fora da saída")
}

func TestBACKUP02_Backup_MostraOKDoMarcador(t *testing.T) {
	for _, modo := range []string{"--agora", "--agendado"} {
		t.Run(modo, func(t *testing.T) {
			e := novoEnsaioBackup(t)
			e.instalarCopia()
			e.state("dump", dumpComUltimaLinhaSentinela())
			out, rc := e.instalada(modo)
			require.Equal(t, 0, rc, out)
			assert.Equal(t, 1, strings.Count(out, okMarcadorBackup), out)
			assert.Less(t, strings.Index(out, okMarcadorBackup), strings.Index(out, "backup: ok "), "o OK vem antes do fim do backup")
			semConteudoDoDump(t, out)
		})
	}
}

// Todas as reprovações do conferir_dump, no --agora e no --agendado.
func TestBACKUP02_Backup_DumpReprovadoSemOK(t *testing.T) {
	semMarcador := strings.TrimSuffix(dumpBackupBom(), marcadorDump)
	casos := []struct {
		nome, dump, rc, esperado string
	}{
		{"mariadb-dump falha", dumpBackupBom(), "1", "o dump falhou"},
		{"dump vazio", "", "", "a ultima linha do dump nao tem o marcador '-- Dump completed'"},
		{"sem marcador", semMarcador, "", "a ultima linha do dump nao tem o marcador '-- Dump completed'"},
		{"marcador no meio", marcadorDump + semMarcador, "", "a ultima linha do dump nao tem o marcador '-- Dump completed'"},
		{"marcador valido e poucas tabelas", strings.Replace(dumpBackupBom(), "CREATE TABLE `tabela_00`", "-- removida", 1), "", "tabela(s)"},
		{"marcador valido e usuarios sem dados", strings.Replace(dumpBackupBom(), "INSERT INTO `usuarios` VALUES\n", "", 1), "", "sem dados"},
	}
	for _, modo := range []string{"--agora", "--agendado"} {
		for _, c := range casos {
			t.Run(modo+" "+c.nome, func(t *testing.T) {
				e := novoEnsaioBackup(t)
				e.instalarCopia()
				e.state("dump", c.dump)
				if c.rc != "" {
					e.state("dump_rc", c.rc)
				}
				out, rc := e.instalada(modo)
				assert.Equal(t, 1, rc, out)
				assert.Contains(t, out, c.esperado)
				assert.NotContains(t, out, "dump validado")
				assert.NotContains(t, out, ": OK")
			})
		}
	}
}

// O OK fala só da validação do dump: as etapas seguintes ainda podem falhar.
func TestBACKUP02_Backup_OKNaoSignificaBackupConcluido(t *testing.T) {
	e := novoEnsaioBackup(t)
	e.instalarCopia()
	e.state("sem_volume", "")
	out, rc := e.instalada("--agendado")
	assert.Equal(t, 1, rc, out)
	assert.Contains(t, out, okMarcadorBackup)
	assert.Contains(t, out, "Backup NAO concluido")
	assert.NotContains(t, out, "backup: ok ")
}

func TestBACKUP02_Status_MostraOKDoMarcador(t *testing.T) {
	novo := func(t *testing.T, dump string) *ensaioBackup {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		c := carimboAgora(0)
		e.par(c, "")
		escrever(t, e.caminhoDump(c), dump, 0o600)
		escrever(t, filepath.Join(e.backups(), ".ultima-verificacao"), fmt.Sprintf("epoch=%d\n", time.Now().Unix()), 0o600)
		return e
	}
	t.Run("marcador valido", func(t *testing.T) {
		out, rc := novo(t, comprimir(t, dumpComUltimaLinhaSentinela())).instalada("--status")
		assert.Equal(t, 0, rc, out)
		assert.Equal(t, 1, strings.Count(out, okMarcadorStatus), out)
		assert.Contains(t, out, "STATUS: ok")
		semConteudoDoDump(t, out)
	})

	semMarcador := strings.TrimSuffix(dumpBackupBom(), marcadorDump)
	for nome, c := range map[string]struct{ conteudo, esperado string }{
		"gzip ruim":        {"nao e gzip", "foi reprovado: gzip invalido ou truncado"},
		"sem marcador":     {comprimir(t, semMarcador), "foi reprovado: ultima linha sem o marcador '-- Dump completed'"},
		"marcador no meio": {comprimir(t, marcadorDump+semMarcador), "foi reprovado: ultima linha sem o marcador '-- Dump completed'"},
	} {
		t.Run(nome, func(t *testing.T) {
			out, rc := novo(t, c.conteudo).instalada("--status")
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, c.esperado)
			assert.Contains(t, out, "STATUS: 1 problema(s)")
			assert.NotContains(t, out, "Dump validado")
			assert.NotContains(t, out, ": OK")
		})
	}
	t.Run("sem backup", func(t *testing.T) {
		e := novoEnsaioBackup(t)
		e.instalarCopia()
		out, rc := e.instalada("--status")
		assert.Equal(t, 1, rc, out)
		assert.NotContains(t, out, "Dump validado")
	})
}
