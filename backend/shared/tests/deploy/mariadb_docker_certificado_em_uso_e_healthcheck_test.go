package deploy_test

// DEPLOY-08 (2026-11-04): o certificado em uso e as contas healthcheck da
// imagem. A leitura da porta publicada saiu com a porta (DEPLOY-11).

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDEPLOY08_CertificadoEmUsoDiferenteAborta(t *testing.T) {
	e := novoEnsaio(t)
	e.setState("cert_em_uso", "Jan  1 00:00:00 2030 GMT|Jan  1 00:00:00 2028 GMT\n")
	r := e.rodar("--nova-ca")
	require.Error(t, r.err, r.out)
	assert.Contains(t, r.out, "nao e o de ")
	assert.Contains(t, r.out, "--novo-cert")
	assert.NotContains(t, r.out, "\nPronto. MariaDB")
}

func TestDEPLOY08_HealthcheckDaImagem(t *testing.T) {
	t.Run("so USAGE: sem aviso", func(t *testing.T) {
		e := novoEnsaio(t)
		e.instalarSemPorta()
		r := e.rodar()
		require.NoError(t, r.err, r.out)
		assert.NotContains(t, r.out, "contas fora do esperado")
		assert.NotContains(t, r.out, "healthcheck locais com privilegios")
		// A consulta das "outras" exclui só os healthcheck locais.
		assert.Contains(t, e.state("sql_outras"), "NOT (User = 'healthcheck' AND Host IN ('localhost', '127.0.0.1', '::1'))")
		hc := e.state("sql_healthcheck")
		for _, tab := range []string{"$.access", "mysql.db", "mysql.tables_priv", "mysql.columns_priv", "mysql.procs_priv", "mysql.roles_mapping", "mysql.proxies_priv"} {
			assert.Contains(t, hc, tab)
		}
		assert.NotContains(t, e.state("sql"), "healthcheck", "nunca remove as contas healthcheck")
	})
	t.Run("alem de USAGE: aviso", func(t *testing.T) {
		e := novoEnsaio(t)
		e.setState("healthcheck_alem_usage", "healthcheck@127.0.0.1\n")
		e.instalarSemPorta()
		r := e.rodar()
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "contas healthcheck locais com privilegios ALEM de USAGE: healthcheck@127.0.0.1")
		assert.NotContains(t, e.state("sql"), "DROP USER IF EXISTS 'healthcheck'")
	})
	t.Run("healthcheck remoto continua como conta fora do esperado", func(t *testing.T) {
		e := novoEnsaio(t)
		e.setState("outras", "healthcheck@%\n")
		r := e.rodar("--nova-ca")
		require.NoError(t, r.err, r.out)
		assert.True(t, strings.Contains(r.out, "contas fora do esperado") && strings.Contains(r.out, "healthcheck@%"), r.out)
	})
}
