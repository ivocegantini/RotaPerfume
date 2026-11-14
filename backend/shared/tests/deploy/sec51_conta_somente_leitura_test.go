package deploy_test

// SEC-51 (Lote 39): com --dev-ip, o mariadb-docker.sh cria também a conta
// casaos_ro@IP (só SELECT no banco da aplicação, ed25519, REQUIRE SSL) com
// senha própria em casaos_ro.senha (600). O ALL do casaos fica para migrações.

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func senhaRO(e *ensaio) string {
	return strings.TrimSpace(e.arq(filepath.Join(segredos, "casaos_ro.senha")))
}

func TestSEC51_Ensaio_ContaSomenteLeitura(t *testing.T) {
	e := novoEnsaio(t)
	e.setState("contas", "casaos_ro\t%\n")
	// Banco com as tabelas de negocio (inclusive uma fora da lista do teste,
	// que o script nunca recebe do information_schema filtrado).
	e.setState("tabelas", "clientes\npedidos\nvendedores\n")
	r := e.rodar("--nova-ca", "--dev-ip", "192.168.168.50", "--dev-ip", "192.168.168.51")
	require.NoError(t, r.err, r.out)

	ro := senhaRO(e)
	assert.Regexp(t, `^[A-Za-z0-9]{32}$`, ro)
	assert.NotEqual(t, e.senhas()["casaos"], ro, "senha própria")
	st, err := os.Stat(filepath.Join(e.dir, segredos, "casaos_ro.senha"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())

	sql := e.state("sql")
	for _, ip := range []string{"192.168.168.50", "192.168.168.51"} {
		u := "'casaos_ro'@'" + ip + "'"
		limites := " REQUIRE SSL WITH MAX_USER_CONNECTIONS 5 MAX_STATEMENT_TIME 30;"
		assert.Contains(t, sql, "CREATE USER IF NOT EXISTS "+u+" IDENTIFIED VIA ed25519 USING PASSWORD('"+ro+"')"+limites)
		assert.Contains(t, sql, "ALTER USER "+u+" IDENTIFIED VIA ed25519 USING PASSWORD('"+ro+"')"+limites)
		assert.Contains(t, sql, "REVOKE ALL PRIVILEGES, GRANT OPTION FROM "+u+";")
		for _, tb := range []string{"clientes", "pedidos", "vendedores"} {
			assert.Contains(t, sql, "GRANT SELECT ON `rotaperfumes`.`"+tb+"` TO "+u+";")
		}
		assert.NotContains(t, sql, "GRANT SELECT ON `rotaperfumes`.* TO "+u, "nunca o banco inteiro")
		// O ALL continua só no casaos (migrações).
		assert.Contains(t, sql, "GRANT ALL PRIVILEGES ON `rotaperfumes`.* TO 'casaos'@'"+ip+"';")
	}
	for _, l := range strings.Split(sql, "\n") {
		if strings.Contains(l, "GRANT") && strings.Contains(l, "'casaos_ro'@") && !strings.HasPrefix(l, "REVOKE") {
			assert.Regexp(t, "^GRANT SELECT ON `rotaperfumes`\\.`[a-z_]+` TO$", l[:strings.Index(l, " 'casaos_ro'")], "casaos_ro só com SELECT por tabela: %s", l)
		}
	}
	assert.Contains(t, sql, "DROP USER IF EXISTS 'casaos_ro'@'%';", "convergência: host fora do desejado sai")
	assert.NotContains(t, r.out, ro, "senha nunca na saída")
	assert.NotContains(t, e.log(), ro, "senha nunca no argv")
	assert.Contains(t, r.out, "casaos_ro.senha")
	assert.Contains(t, r.out, "DB_USUARIO=casaos_ro")
	assert.Contains(t, r.out, "SEC-51: casaos_ro com SELECT em: clientes pedidos vendedores")
	for _, tb := range []string{"usuarios", "refresh_tokens", "senha_historico"} {
		assert.NotContains(t, sql, "`"+tb+"` TO 'casaos_ro'")
	}
	assert.NotContains(t, sql, "mysql_native_password", "sem --casaos-native-motivo")

	// Reexecução: a senha não muda.
	r = e.rodar("--dev-ip", "192.168.168.50", "--dev-ip", "192.168.168.51")
	require.NoError(t, r.err, r.out)
	assert.Equal(t, ro, senhaRO(e))

	// Sem --dev-ip: conta removida, arquivo mantido (como o casaos, CHORE-17).
	e.setState("contas", "casaos_ro\t192.168.168.50\ncasaos\t192.168.168.50\n")
	inicio := len(e.state("sql"))
	r = e.rodar("--recriar")
	require.NoError(t, r.err, r.out)
	trecho := e.state("sql")[inicio:]
	assert.Contains(t, trecho, "DROP USER IF EXISTS 'casaos_ro'@'192.168.168.50';")
	assert.NotContains(t, trecho, "CREATE USER IF NOT EXISTS 'casaos_ro'@")
	assert.Equal(t, ro, senhaRO(e))
	assert.Contains(t, r.out, "Sem --dev-ip: conta casaos_ro removida; /"+segredos+"/casaos_ro.senha mantido")
}

// Banco sem tabelas (antes do seed): conta sem GRANT e aviso para reexecutar.
func TestSEC51_BancoVazioAvisa(t *testing.T) {
	e := novoEnsaio(t)
	r := e.rodar("--nova-ca", "--dev-ip", "192.168.168.50")
	require.NoError(t, r.err, r.out)
	assert.NotContains(t, e.state("sql"), "GRANT SELECT ON `rotaperfumes`.`")
	assert.Contains(t, r.out, "casaos_ro sem acesso. Rode o script de novo depois do seed")
}

// A lista TABELAS_RO + as tabelas excluidas = todas as tabelas da DDL do
// sql/ordem-ddl.txt (tabela nova precisa ser classificada).
func TestSEC51_TabelasROConferemComDDL(t *testing.T) {
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	m := regexp.MustCompile(`(?m)^readonly TABELAS_RO=\(([^)]*)\)$`).FindStringSubmatch(md)
	require.NotNil(t, m, "TABELAS_RO no mariadb-docker.sh")
	ro := strings.Fields(m[1])
	excluidas := []string{"usuarios", "refresh_tokens", "senha_historico"}
	reTab := regexp.MustCompile("(?i)CREATE\\s+TABLE\\s+(?:IF\\s+NOT\\s+EXISTS\\s+)?`?([a-z_]+)`?")
	var ddl []string
	for _, arq := range linhasUteis(lerRaiz(t, "sql", "ordem-ddl.txt")) {
		for _, mm := range reTab.FindAllStringSubmatch(lerRaiz(t, "sql", arq), -1) {
			ddl = append(ddl, mm[1])
		}
	}
	require.NotEmpty(t, ddl)
	esperado := append(append([]string{}, ro...), excluidas...)
	sort.Strings(esperado)
	sort.Strings(ddl)
	assert.Equal(t, esperado, ddl, "classifique a tabela nova em TABELAS_RO ou nas excluidas")
	for _, x := range excluidas {
		assert.NotContains(t, ro, x)
	}
}

func TestSEC51_SemDevIPNaoCriaContaRO(t *testing.T) {
	e := novoEnsaio(t)
	r := e.rodar("--nova-ca")
	require.NoError(t, r.err, r.out)
	assert.NotContains(t, e.state("sql"), "casaos_ro")
	assert.NoFileExists(t, filepath.Join(e.dir, segredos, "casaos_ro.senha"))
	assert.NotContains(t, r.out, "casaos_ro")
}

func TestSEC51_ConvergenciaConheceContaRO(t *testing.T) {
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	assert.Contains(t, md, `readonly DB_MAKE_RO="casaos_ro"`)
	// Listas de contas do projeto (remoção, contas estranhas e REQUIRE SSL).
	assert.Equal(t, 3, strings.Count(md, "'$DB_MAKE', '$DB_MAKE_RO')"), "convergência, OUTRAS e FALTANDO")
	doc := lerRaiz(t, "docs", "deploy-servidor.md")
	assert.Contains(t, doc, "casaos_ro")
	assert.Contains(t, doc, "SEC-51")
}
