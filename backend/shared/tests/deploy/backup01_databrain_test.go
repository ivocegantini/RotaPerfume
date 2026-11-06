package deploy_test

// BACKUP-01 (Lote 42, DataBrain): conta rotaperfumes_backup no
// deploy/mariadb-docker.sh e deploy/verificar-backup.sh. O restante da seção 7
// da especificação fica com o TestBrain.
//   - verificar-backup.sh: estáticos e ensaio com docker falso no PATH (não
//     exige root nem Docker);
//   - conta: ensaio do mariadb-docker.sh (user namespace, stubs do TST-15);
//   - real: TestBACKUP01_VerificarBackup_Real, com Docker e opt-in
//     (BACKUP01_DOCKER=1), sem root.

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sentinelaDado = "SENTINELA_DADO_DE_CLIENTE"

// Docker falso do verificar-backup.sh. O § vira crase (string crua do Go não
// aceita crase). $S/importado guarda o que chegou pelo stdin da importação.
var stubDockerVerificar = strings.ReplaceAll(`#!/bin/bash
S="$FAKE/state"
printf '%s\n' "docker $*" >>"$FAKE/log"
case "$1" in
  run)
    if [[ " $* " == *" -d "* ]]; then
      [[ -f "$S/run_falha" ]] && exit 1
      echo abc123def4567890
      exit 0
    fi
    case "$*" in
      *"-u mysql"*|*"-g mysql"*) cat "$S/uid_mysql" 2>/dev/null || echo 999 ;;
    esac
    exit 0 ;;
  inspect) echo true; exit 0 ;;
  rm) printf '%s\n' "$*" >>"$S/rm"; exit 0 ;;
  exec)
    a="$*"
    if [[ "$a" != *" -e "* ]]; then
      cat >"$S/importado"
      if [[ -f "$S/import_erro" ]]; then cat "$S/import_erro" >&2; exit 1; fi
      exit 0
    fi
    case "$a" in
      *"SELECT 1"*) cat "$S/select1" 2>/dev/null || echo 1 ;;
      *"CREATE DATABASE"*) ;;
      *SCHEMATA*) cat "$S/outros_bancos" 2>/dev/null || echo 0 ;;
      *"information_schema.TABLES"*) cat "$S/tabelas" ;;
      *"role = 'admin'"*) cat "$S/admins" 2>/dev/null || echo 1 ;;
      *'COUNT(*) FROM §'*)
        t=$(sed -E 's/.*FROM §([^§]+)§.*/\1/' <<<"$a")
        cat "$S/linhas_$t" 2>/dev/null || echo 5 ;;
    esac
    exit 0 ;;
esac
exit 0
`, "§", "`")

// 13 tabelas: as três obrigatórias e dez de enchimento.
func tabelasVerificacao(n int) []string {
	out := []string{"clientes", "usuarios", "vendedores"}
	for i := 0; len(out) < n; i++ {
		out = append(out, fmt.Sprintf("tabela_%02d", i))
	}
	return out
}

type ensaioVerificar struct {
	t   *testing.T
	dir string
}

func novoEnsaioVerificar(t *testing.T) *ensaioVerificar {
	t.Helper()
	precisaBash(t)
	ferramentaObrigatoria(t, "gzip")
	ferramentaObrigatoria(t, "sha256sum")
	dir := t.TempDir()
	escrever(t, filepath.Join(dir, "bin", "docker"), stubDockerVerificar, 0o755)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "state"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "backups"), 0o700))
	e := &ensaioVerificar{t: t, dir: dir}
	e.setState("tabelas", strings.Join(tabelasVerificacao(13), "\n")+"\n")
	return e
}

func (e *ensaioVerificar) setState(nome, v string) {
	e.t.Helper()
	escrever(e.t, filepath.Join(e.dir, "state", nome), v, 0o644)
}

func (e *ensaioVerificar) ler(rel string) string {
	e.t.Helper()
	b, err := os.ReadFile(filepath.Join(e.dir, rel))
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(e.t, err)
	return string(b)
}

// rodar executa o script real com o docker falso; devolve saída e código.
func (e *ensaioVerificar) rodar(args ...string) (string, int) {
	e.t.Helper()
	cmd := exec.Command("bash", append([]string{caminhoRaiz(e.t, "deploy", "verificar-backup.sh")}, args...)...)
	cmd.Env = append(ambienteLimpo(), "FAKE="+e.dir, "PATH="+filepath.Join(e.dir, "bin")+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	ee, ok := err.(*exec.ExitError)
	require.True(e.t, ok, "%v", err)
	return string(out), ee.ExitCode()
}

// gz grava o texto comprimido em backups/nome e devolve o caminho.
func (e *ensaioVerificar) gz(nome, texto string) string {
	e.t.Helper()
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, err := w.Write([]byte(texto))
	require.NoError(e.t, err)
	require.NoError(e.t, w.Close())
	p := filepath.Join(e.dir, "backups", nome)
	escrever(e.t, p, b.String(), 0o600)
	return p
}

const marcadorDump = "-- Dump completed on 2026-11-06 03:30:00\n"

// Mesmo formato da 11.4.13 (validado no Lote 42): "INSERT ... VALUES" sozinho
// e um registro por linha, começando com "(".
func dumpBom() string {
	return "/*M!999999\\- enable the sandbox mode */ \n-- MariaDB dump\n" +
		"CREATE TABLE `usuarios` (\n  `id` bigint NOT NULL\n);\n" +
		"INSERT INTO `usuarios` VALUES\n(1,'" + sentinelaDado + "\\nUSE producao;\\nDROP DATABASE producao;'),\n(2,'-- Dump completed on 2020-01-01');\n" +
		marcadorDump
}

func TestBACKUP01_VerificarBackup_Estatico(t *testing.T) {
	sc := lerRaiz(t, "deploy", "verificar-backup.sh")
	// Nenhum caminho até a produção, nem em comentário.
	for _, proibido := range []string{"rotaperfumes-db", "rotaperfumes-mariadb", "SEGREDOS", "--ip", "--mount", "--volume", "MYSQL_PWD", "--env", "--name", "--publish"} {
		assert.NotContains(t, sc, proibido)
	}
	assert.NotRegexp(t, `(?m)^\s*set\s+-[a-z]*x`, sc, "nunca set -x")
	assert.NotRegexp(t, `\s-[ve]\s+"`, regexp.MustCompile(`(?m)^.*sql -N -B.*$|^sql\(\).*$|^.*sql -e.*$`).ReplaceAllString(sc, ""), "sem -v nem -e no docker run")
	// Todo docker run do script: sem rede.
	runs := regexp.MustCompile(`docker run [^\n]*`).FindAllString(sc, -1)
	require.Len(t, runs, 3)
	for _, r := range runs {
		assert.Contains(t, r, "--network none", r)
	}
	for _, f := range []string{"--cap-drop ALL", "--read-only", "--user mysql", "--security-opt no-new-privileges:true", "--skip-networking", "--sandbox"} {
		assert.Contains(t, sc, f)
	}
	assert.Regexp(t, `size=\$\{TMPFS_MB\}m`, sc, "tmpfs limitado")
	// Paridade da imagem (SEC-31).
	re := regexp.MustCompile(`(?m)^IMAGEM_BASE="([^"]+)"$`)
	v, m := re.FindStringSubmatch(sc), re.FindStringSubmatch(lerRaiz(t, "deploy", "mariadb-docker.sh"))
	require.NotNil(t, v)
	require.NotNil(t, m)
	assert.Equal(t, m[1], v[1], "mesma string do IMAGEM_BASE")
	assert.Regexp(t, reImagemMariaDB, v[1])
}

func TestBACKUP01_VerificarBackup_Aprovado(t *testing.T) {
	e := novoEnsaioVerificar(t)
	arq := e.gz("rotaperfumes-20261106-033000Z.sql.gz", dumpBom())
	out, rc := e.rodar(arq)
	require.Equal(t, 0, rc, out)
	assert.Contains(t, out, "Tabelas importadas: 13")
	assert.Contains(t, out, "Admins em usuarios: 1")
	assert.Contains(t, out, "APROVADO")
	assert.NotContains(t, out, "AVISO")
	assert.NotContains(t, out, sentinelaDado)

	log := e.ler("log")
	for _, l := range strings.Split(log, "\n") {
		if strings.HasPrefix(l, "docker run") {
			assert.Contains(t, l, "--network none", l)
		}
		assert.NotContains(t, l, sentinelaDado, "dado não vai para o argv")
	}
	assert.Equal(t, dumpBom(), e.ler("state/importado"), "dump inteiro pelo stdin")
	assert.Contains(t, log, "--sandbox verificacao_backup")
	assert.Contains(t, e.ler("state/rm"), "abc123def4567890", "trap remove o ID criado")
	assert.NotContains(t, e.ler("state/rm"), "rotaperfumes")

	reg := e.ler("backups/.ultima-verificacao")
	assert.Regexp(t, `^epoch=\d+\narquivo=rotaperfumes-20261106-033000Z\.sql\.gz\nsha256=[0-9a-f]{64}\n$`, reg)
	st, err := os.Stat(filepath.Join(e.dir, "backups", ".ultima-verificacao"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	tmps, _ := filepath.Glob(filepath.Join(e.dir, "backups", ".ultima-verificacao.*"))
	assert.Empty(t, tmps)
}

func TestBACKUP01_VerificarBackup_OpcoesRecusadas(t *testing.T) {
	e := novoEnsaioVerificar(t)
	arq := e.gz("b.sql.gz", dumpBom())
	for _, args := range [][]string{
		{"--host", "x", arq}, {"-h", "x", arq}, {"--port", "3306", arq}, {"--user", "root", arq},
		{"--password", "x", arq}, {"--container", "rotaperfumes-mariadb", arq}, {"--rede-nome", "x", arq},
		{"--network", "host", arq}, {"--segredos-dir", "/root", arq}, {"--imagem-base", "x", arq},
		{"--tmpfs-mb", "abc", arq}, {"--tmpfs-mb", "10", arq}, {"--tmpfs-mb", "999999", arq}, {"--tmpfs-mb"},
		{arq, arq}, {},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, rc := e.rodar(args...)
			assert.Equal(t, 2, rc, out)
			assert.Contains(t, out, "Uso:")
		})
	}
	assert.Empty(t, e.ler("log"), "uso incorreto não chama o docker")
	out, rc := e.rodar("--help")
	assert.Equal(t, 0, rc, out)
	assert.Contains(t, out, "--tmpfs-mb")
}

// Recusas antes de subir o container: nem um docker run.
func TestBACKUP01_VerificarBackup_ArquivoRecusado(t *testing.T) {
	grande := make([]byte, 6<<20)
	rand.New(rand.NewSource(42)).Read(grande) //nolint:gosec // dado de teste
	enchimento := fmt.Sprintf("-- %x\n", grande)

	casos := []struct {
		nome, texto, esperado string
	}{
		{"sem marcador", "CREATE TABLE `a` (id int);\n", "marcador"},
		{"dump vazio", "", "marcador"},
		{"marcador so no meio", marcadorDump + "CREATE TABLE `a` (id int);\n", "marcador"},
		{"marcador dentro de um dado", "(1,'-- Dump completed on x')\n", "marcador"},
		{"USE", "USE rotaperfumes;\n" + marcadorDump, "seleciona ou cria banco"},
		{"use minusculo com crase", "use `rotaperfumes`;\n" + marcadorDump, "seleciona ou cria banco"},
		{"CREATE DATABASE", "CREATE DATABASE /*!32312 IF NOT EXISTS*/ `x`;\n" + marcadorDump, "seleciona ou cria banco"},
		{"CREATE SCHEMA", "create schema x;\n" + marcadorDump, "seleciona ou cria banco"},
		{"DROP DATABASE", "DROP DATABASE rotaperfumes;\n" + marcadorDump, "seleciona ou cria banco"},
		{"USE no fim de arquivo grande", enchimento + "USE x;\n" + marcadorDump, "seleciona ou cria banco"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioVerificar(t)
			out, rc := e.rodar(e.gz("b.sql.gz", c.texto))
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, "REPROVADO")
			assert.Contains(t, out, c.esperado)
			assert.NotContains(t, out, "APROVADO\n")
			assert.Empty(t, e.ler("log"), "recusa antes de chamar o docker")
		})
	}

	t.Run("regressao SIGPIPE: arquivo grande e bom passa", func(t *testing.T) {
		e := novoEnsaioVerificar(t)
		out, rc := e.rodar(e.gz("b.sql.gz", enchimento+marcadorDump))
		assert.Equal(t, 0, rc, out)
	})

	t.Run("nao regular, symlink, gzip invalido", func(t *testing.T) {
		e := novoEnsaioVerificar(t)
		bom := e.gz("bom.sql.gz", dumpBom())
		link := filepath.Join(e.dir, "backups", "link.sql.gz")
		require.NoError(t, os.Symlink(bom, link))
		invalido := filepath.Join(e.dir, "backups", "invalido.sql.gz")
		escrever(t, invalido, "nao e gzip", 0o600)
		truncado := filepath.Join(e.dir, "backups", "truncado.sql.gz")
		b, err := os.ReadFile(bom)
		require.NoError(t, err)
		escrever(t, truncado, string(b[:len(b)/2]), 0o600)
		for arq, esperado := range map[string]string{
			link:                               "link simbolico",
			filepath.Join(e.dir, "backups"):    "nao e um arquivo regular",
			filepath.Join(e.dir, "nao-existe"): "nao e um arquivo regular",
			invalido:                           "gzip invalido",
			truncado:                           "gzip invalido",
		} {
			out, rc := e.rodar(arq)
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, esperado)
		}
		assert.Empty(t, e.ler("log"))
	})
}

func TestBACKUP01_VerificarBackup_ConteudoReprovado(t *testing.T) {
	casos := []struct {
		nome     string
		preparar func(e *ensaioVerificar)
		esperado string
	}{
		{"12 tabelas", func(e *ensaioVerificar) {
			e.setState("tabelas", strings.Join(tabelasVerificacao(12), "\n")+"\n")
		}, "12 tabela(s); o minimo e 13"},
		{"sem tabelas", func(e *ensaioVerificar) { e.setState("tabelas", "") }, "0 tabela(s)"},
		{"sem usuarios", func(e *ensaioVerificar) {
			l := tabelasVerificacao(14)
			e.setState("tabelas", strings.Join(append([]string{l[0], l[2]}, l[3:]...), "\n")+"\n")
		}, "a tabela usuarios nao esta no dump"},
		{"sem admin", func(e *ensaioVerificar) { e.setState("admins", "0\n") }, "nenhum admin"},
		{"vendedores vazia", func(e *ensaioVerificar) { e.setState("linhas_vendedores", "0\n") }, "vendedores esta vazia"},
		{"banco extra", func(e *ensaioVerificar) { e.setState("outros_bancos", "1\n") }, "alem do temporario"},
		{"nome de tabela estranho", func(e *ensaioVerificar) {
			e.setState("tabelas", "usuarios\na;b\n")
		}, "nome de tabela fora do padrao"},
		{"servidor nao criou container", func(e *ensaioVerificar) { e.setState("run_falha", "") }, "container descartavel"},
		{"uid mysql root", func(e *ensaioVerificar) { e.setState("uid_mysql", "0\n") }, "uid/gid do mysql"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioVerificar(t)
			c.preparar(e)
			out, rc := e.rodar(e.gz("b.sql.gz", dumpBom()))
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, c.esperado)
			assert.NotContains(t, out, "APROVADO\n")
			assert.Empty(t, e.ler("backups/.ultima-verificacao"), "reprovado não registra")
		})
	}
}

func TestBACKUP01_VerificarBackup_ClientesVaziaSoAvisa(t *testing.T) {
	e := novoEnsaioVerificar(t)
	e.setState("linhas_clientes", "0\n")
	out, rc := e.rodar(e.gz("b.sql.gz", dumpBom()))
	assert.Equal(t, 0, rc, out)
	assert.Contains(t, out, "AVISO: a tabela clientes esta vazia")
	assert.Contains(t, out, "APROVADO")
}

// O texto de erro do cliente pode citar dados: só o código e a linha saem.
func TestBACKUP01_VerificarBackup_ErroDeImportacaoSemDados(t *testing.T) {
	e := novoEnsaioVerificar(t)
	e.setState("import_erro", "ERROR 1062 (23000) at line 495: Duplicate entry '"+sentinelaDado+"' for key 'PRIMARY'\n")
	out, rc := e.rodar(e.gz("b.sql.gz", dumpBom()))
	assert.Equal(t, 1, rc, out)
	assert.Contains(t, out, "ERROR 1062 (23000) at line 495")
	assert.NotContains(t, out, sentinelaDado)
	assert.NotContains(t, out, "Duplicate")
	assert.Contains(t, e.ler("state/rm"), "abc123def4567890", "falha também remove o container")
}

func TestBACKUP01_ManualTemRunbook(t *testing.T) {
	m := lerForaDoTarDoCI(t, "docs", "manual-base-de-dados.md")
	i := strings.Index(m, "## 24. Backup e restauração do banco")
	require.GreaterOrEqual(t, i, 0)
	s := m[i:]
	for _, trecho := range []string{"PASSWORD_PEPPER", "### 24.6 Perda do servidor", "verificar-backup.sh", "TRUNCATE TABLE refresh_tokens", "DROP DATABASE rotaperfumes", "produto_imagens"} {
		assert.Contains(t, s, trecho)
	}
	// Só runbook: nenhum alvo do Makefile restaura backup em produção.
	assert.NotRegexp(t, `(?m)^[a-z-]*(restaura|restore)[a-z-]*:`, lerRaiz(t, "Makefile"))
	_, err := os.Stat(caminhoRaiz(t, "deploy", "restaurar-banco.sh"))
	assert.True(t, os.IsNotExist(err), "sem script de restauração em produção (D7)")
}

// ---------------------------------------------------------------------------
// Conta rotaperfumes_backup no mariadb-docker.sh
// ---------------------------------------------------------------------------

func TestBACKUP01_Ensaio_ContaBackup(t *testing.T) {
	e := novoEnsaio(t)
	r := e.rodar("--nova-ca")
	require.NoError(t, r.err, r.out)

	senhaRel := filepath.Join(segredos, "rotaperfumes_backup.senha")
	senha := strings.TrimSpace(e.arq(senhaRel))
	assert.Regexp(t, `^[A-Za-z0-9]{32}$`, senha)
	st, err := os.Stat(filepath.Join(e.dir, senhaRel))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())

	// Conta: ed25519 + REQUIRE SSL, IP exclusivo, só SELECT.
	sql := e.state("sql")
	const conta = "'rotaperfumes_backup'@'172.30.42.21'"
	assert.Contains(t, sql, "CREATE USER IF NOT EXISTS "+conta+" IDENTIFIED VIA ed25519 USING PASSWORD('"+senha+"') REQUIRE SSL;")
	assert.Contains(t, sql, "ALTER USER "+conta+" IDENTIFIED VIA ed25519 USING PASSWORD('"+senha+"') REQUIRE SSL;")
	assert.Contains(t, sql, "REVOKE ALL PRIVILEGES, GRANT OPTION FROM "+conta+";")
	assert.Contains(t, sql, "GRANT SELECT ON `rotaperfumes`.* TO "+conta+";")
	grants := regexp.MustCompile(`(?m)^GRANT .* TO 'rotaperfumes_backup'.*$`).FindAllString(sql, -1)
	assert.Equal(t, []string{"GRANT SELECT ON `rotaperfumes`.* TO " + conta + ";"}, grants, "um único GRANT")

	// Option file: 400, diretório 700, fora de container/.
	cnfRel := filepath.Join(segredos, "backup-cliente", "rotaperfumes_backup.cnf")
	assert.Equal(t, "[client]\nuser=rotaperfumes_backup\npassword="+senha+"\n", e.arq(cnfRel))
	st, err = os.Stat(filepath.Join(e.dir, cnfRel))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o400), st.Mode().Perm())
	st, err = os.Stat(filepath.Join(e.dir, segredos, "backup-cliente"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), st.Mode().Perm())
	assert.False(t, e.existe(filepath.Join(segredos, "backup-cliente", "rotaperfumes_backup.cnf.tmp")), "temporário removido")
	assert.False(t, e.existe(filepath.Join(segredos, "container", "rotaperfumes_backup.cnf")))
	assert.False(t, e.existe(filepath.Join(segredos, "container", "rotaperfumes_backup.senha")))
	assert.NotContains(t, e.state("run_args"), "backup-cliente", "o container do banco não monta o option file")

	// Senha nunca na tela nem no argv; conta nas listas de convergência.
	log := e.log()
	assert.NotContains(t, r.out, senha)
	assert.NotContains(t, log, senha)
	assert.Contains(t, r.out, "backup 172.30.42.21")
	assert.Contains(t, log, "User IN ('rotaperfumes_app', 'rotaperfumes_admin', 'rotaperfumes_backup', 'casaos') ORDER BY", "convergência")
	assert.Contains(t, log, "User IN ('rotaperfumes_app', 'rotaperfumes_admin', 'rotaperfumes_backup', 'casaos') AND COALESCE", "conferência de REQUIRE SSL")
	assert.Contains(t, e.state("sql_outras"), "'rotaperfumes_backup', 'casaos')", "não é conta estranha")

	// Reexecução não troca a senha nem o option file.
	cnf1 := e.arq(cnfRel)
	r = e.rodar()
	require.NoError(t, r.err, r.out)
	assert.Equal(t, senha, strings.TrimSpace(e.arq(senhaRel)))
	assert.Equal(t, cnf1, e.arq(cnfRel))
	st, err = os.Stat(filepath.Join(e.dir, cnfRel))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o400), st.Mode().Perm())
}

func TestBACKUP01_Ensaio_OutroHostERede(t *testing.T) {
	t.Run("conta com outro host sai", func(t *testing.T) {
		e := novoEnsaio(t)
		e.setState("contas", "rotaperfumes_backup\t%\nrotaperfumes_backup\t172.30.42.21\n")
		r := e.rodar("--nova-ca")
		require.NoError(t, r.err, r.out)
		assert.Contains(t, e.state("sql"), "DROP USER IF EXISTS 'rotaperfumes_backup'@'%';")
		assert.NotContains(t, e.state("sql"), "DROP USER IF EXISTS 'rotaperfumes_backup'@'172.30.42.21';")
	})
	t.Run("IP segue o prefixo da rede", func(t *testing.T) {
		e := novoEnsaio(t)
		r := e.rodar("--nova-ca", "--rede-docker", "172.30.77.0/24")
		require.NoError(t, r.err, r.out)
		assert.Contains(t, e.state("sql"), "GRANT SELECT ON `rotaperfumes`.* TO 'rotaperfumes_backup'@'172.30.77.21';")
	})
	t.Run("diretorio do option file nao pode ser symlink", func(t *testing.T) {
		e := novoEnsaio(t)
		r := e.rodar("--nova-ca")
		require.NoError(t, r.err, r.out)
		dir := filepath.Join(e.dir, segredos, "backup-cliente")
		require.NoError(t, os.RemoveAll(dir))
		desvio := filepath.Join(e.dir, "desvio")
		require.NoError(t, os.MkdirAll(desvio, 0o700))
		require.NoError(t, os.Symlink(desvio, dir))
		r = e.rodar()
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "backup-cliente e um link simbolico")
		entradas, err := os.ReadDir(desvio)
		require.NoError(t, err)
		assert.Empty(t, entradas, "nada gravado pelo symlink")
	})
}

// ---------------------------------------------------------------------------
// Real: Docker de verdade, sem root
// ---------------------------------------------------------------------------

// dumpSinteticoReal: 13 tabelas e o formato real do dump, com dados sintéticos.
func dumpSinteticoReal() string {
	var b strings.Builder
	b.WriteString("/*M!999999\\- enable the sandbox mode */ \n/*!40101 SET NAMES utf8mb4 */;\n")
	b.WriteString("CREATE TABLE `vendedores` (`id` bigint PRIMARY KEY, `nome` varchar(120)) DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;\n")
	b.WriteString("INSERT INTO `vendedores` VALUES\n(1,'João Ação 😀'),\n(2,'Émile');\n")
	b.WriteString("CREATE TABLE `usuarios` (`id` bigint PRIMARY KEY, `email` varchar(120), `role` enum('admin','normal')) DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;\n")
	b.WriteString("INSERT INTO `usuarios` VALUES\n(1,'" + sentinelaDado + "@exemplo.invalid','admin');\n")
	b.WriteString("CREATE TABLE `clientes` (`id` bigint PRIMARY KEY) DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;\n")
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&b, "CREATE TABLE `t%d` (`id` bigint PRIMARY KEY);\n", i)
	}
	b.WriteString(marcadorDump)
	return b.String()
}

// TestBACKUP01_VerificarBackup_Real restaura de verdade, na imagem fixada. Só
// usa containers descartáveis sem rede. Ex.:
// sg docker -c "BACKUP01_DOCKER=1 go test -count=1 -run TestBACKUP01_VerificarBackup_Real ./tests/deploy/"
func TestBACKUP01_VerificarBackup_Real(t *testing.T) {
	precisaBash(t)
	if os.Getenv("BACKUP01_DOCKER") != "1" {
		t.Skip("opt-in: BACKUP01_DOCKER=1, com Docker (não exige root)")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker inacessível: " + err.Error())
	}
	e := &ensaioVerificar{t: t, dir: t.TempDir()}
	require.NoError(t, os.MkdirAll(filepath.Join(e.dir, "backups"), 0o700))
	rodar := func(arq string) (string, int) {
		cmd := exec.Command("bash", caminhoRaiz(t, "deploy", "verificar-backup.sh"), arq)
		cmd.Env = ambienteLimpo()
		out, err := cmd.CombinedOutput()
		if err == nil {
			return string(out), 0
		}
		ee, ok := err.(*exec.ExitError)
		require.True(t, ok, "%v", err)
		return string(out), ee.ExitCode()
	}
	restos := func() string {
		out, err := exec.Command("docker", "ps", "-aq", "--filter", "label=rotaperfumes.verificar-backup=1").Output()
		require.NoError(t, err)
		return strings.TrimSpace(string(out))
	}

	out, rc := rodar(e.gz("bom.sql.gz", dumpSinteticoReal()))
	require.Equal(t, 0, rc, out)
	assert.Contains(t, out, "Tabelas importadas: 13")
	assert.Contains(t, out, "Admins em usuarios: 1")
	assert.Contains(t, out, "AVISO: a tabela clientes esta vazia")
	assert.NotContains(t, out, sentinelaDado)
	assert.NotContains(t, out, "João")
	assert.Empty(t, restos(), "nenhum container sobra")

	// O erro real do cliente cita o dado; a saída não pode citar.
	dup := strings.Replace(dumpSinteticoReal(), marcadorDump,
		"INSERT INTO `usuarios` VALUES\n(1,'"+sentinelaDado+"','admin');\n"+marcadorDump, 1)
	out, rc = rodar(e.gz("dup.sql.gz", dup))
	assert.Equal(t, 1, rc, out)
	assert.Contains(t, out, "ERROR 1062")
	assert.NotContains(t, out, sentinelaDado)
	assert.Empty(t, restos(), "nenhum container sobra")
}
