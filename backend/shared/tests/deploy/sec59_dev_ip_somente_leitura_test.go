package deploy_test

// SEC-59: --dev-ip-ro IP publica a porta e libera o firewall como o --dev-ip,
// mas cria só casaos_ro@IP (sem o casaos ALL e sem casaos.senha). É o modo da
// produção: a máquina do admin só consulta.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSEC59_DevIPRO_SoContaSomenteLeitura(t *testing.T) {
	e := novoEnsaio(t)
	e.setState("contas", "casaos\t192.168.168.105\n")
	e.setState("tabelas", "clientes\npedidos\n")
	r := e.rodar("--nova-ca", "--dev-ip-ro", "192.168.168.105")
	require.NoError(t, r.err, r.out)

	sql := e.state("sql")
	ro := "'casaos_ro'@'192.168.168.105'"
	assert.Contains(t, sql, "CREATE USER IF NOT EXISTS "+ro)
	assert.Contains(t, sql, "GRANT SELECT ON `rotaperfumes`.`clientes` TO "+ro+";")
	assert.NotContains(t, sql, "CREATE USER IF NOT EXISTS 'casaos'@", "sem a conta ALL")
	assert.NotContains(t, sql, "GRANT ALL PRIVILEGES ON `rotaperfumes`.* TO 'casaos'@")
	assert.Contains(t, sql, "DROP USER IF EXISTS 'casaos'@'192.168.168.105';", "casaos antigo sai na convergência")
	assert.NoFileExists(t, filepath.Join(e.dir, segredos, "casaos.senha"))
	assert.FileExists(t, filepath.Join(e.dir, segredos, "casaos_ro.senha"))

	// Porta e firewall iguais ao --dev-ip.
	assert.Contains(t, e.arq("sbin/rotaperfumes-db-firewall"), "DEV_IPS=(192.168.168.105)")
	assert.Contains(t, r.out, "DB_USUARIO=casaos_ro")
	assert.NotContains(t, r.out, "DB_USUARIO=casaos\n")
	assert.NotContains(t, r.out, "casaos.senha ")
	assert.NotContains(t, r.out, senhaRO(e), "senha nunca na saída")

	// casaos.senha de uma execução antiga com --dev-ip: mantido, com o motivo certo.
	escrever(t, filepath.Join(e.dir, segredos, "casaos.senha"), strings.Repeat("A", 32), 0o600)
	r = e.rodar("--dev-ip-ro", "192.168.168.105")
	require.NoError(t, r.err, r.out)
	assert.Contains(t, r.out, "So --dev-ip-ro (SEC-59): conta casaos removida;")
	assert.NotContains(t, r.out, "Sem --dev-ip: conta casaos removida")
}

func TestSEC59_DevIPRO_Misturado(t *testing.T) {
	e := novoEnsaio(t)
	r := e.rodar("--nova-ca", "--dev-ip", "192.168.168.50", "--dev-ip-ro", "192.168.168.105")
	require.NoError(t, r.err, r.out)
	sql := e.state("sql")
	assert.Contains(t, sql, "GRANT ALL PRIVILEGES ON `rotaperfumes`.* TO 'casaos'@'192.168.168.50';")
	assert.NotContains(t, sql, "'casaos'@'192.168.168.105'")
	assert.Contains(t, sql, "CREATE USER IF NOT EXISTS 'casaos_ro'@'192.168.168.105'")
	assert.Contains(t, sql, "CREATE USER IF NOT EXISTS 'casaos_ro'@'192.168.168.50'")
	assert.FileExists(t, filepath.Join(e.dir, segredos, "casaos.senha"))
}

func TestSEC59_DevIPRO_Recusas(t *testing.T) {
	casos := map[string][]string{
		"mesmo IP nos dois": {"--nova-ca", "--dev-ip", "192.168.168.105", "--dev-ip-ro", "192.168.168.105"},
		"native sem casaos": {"--nova-ca", "--dev-ip-ro", "192.168.168.105", "--casaos-native-motivo", "cliente antigo sem ed25519"},
		"IP invalido":       {"--nova-ca", "--dev-ip-ro", "192.168.168.0/24"},
	}
	for nome, args := range casos {
		t.Run(nome, func(t *testing.T) {
			e := novoEnsaio(t)
			r := e.rodar(args...)
			require.Error(t, r.err, r.out)
			assert.False(t, strings.Contains(e.state("sql"), "CREATE USER"), "nada muda")
		})
	}
}
