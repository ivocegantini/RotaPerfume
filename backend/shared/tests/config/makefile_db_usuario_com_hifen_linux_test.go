package config_test

// DEPLOY-10: o hífen de rotaperfumes-admin vale nos dois validadores do SEC-63; o
// "-" inicial continua recusado, porque viraria opção do cliente mariadb.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/testkit/integracao"
)

func TestDEPLOY10_DBUsuario_Makefile(t *testing.T) {
	const env = "DB_SENHA=s\n"
	t.Run("rotaperfumes-admin aceito", func(t *testing.T) {
		dir, path, cliente, _ := sec63Dir(t, env)
		out, err := makeAlvo(t, dir, "db-check-env", []string{path}, "DB_CLIENTE_BIN="+cliente, "DB_USUARIO=rotaperfumes-admin", "DB_HOST=127.0.0.1")
		require.NoError(t, err, out)
		assert.NotContains(t, out, "SEC-63")
	})
	t.Run("rotaperfumes-admin chega ao cliente", func(t *testing.T) {
		dir, path, cliente, log := sec63Dir(t, env)
		out, err := makeAlvo(t, dir, "db-create", []string{path}, "DB_CLIENTE_BIN="+cliente, "DB_USUARIO=rotaperfumes-admin", "DB_HOST=127.0.0.1")
		require.NoError(t, err, out)
		assert.Contains(t, lerSeExiste(t, log), "[-u][rotaperfumes-admin]")
	})
	t.Run("-x recusado", func(t *testing.T) {
		sec63Recusa(t, "db-check-env", env, "SEC-63: DB_USUARIO invalido", "DB_USUARIO=-x")
	})
}

func TestDEPLOY10_DBUsuario_ValidarFormaDB(t *testing.T) {
	require.NoError(t, integracao.ValidarFormaDB("127.0.0.1", "rotaperfumes", "rotaperfumes-admin"))
	require.NoError(t, integracao.ValidarFormaDB("127.0.0.1", "rotaperfumes", "rotaperfumes_admin"))
	err := integracao.ValidarFormaDB("127.0.0.1", "rotaperfumes", "-x")
	require.ErrorIs(t, err, integracao.ErrDBValorInvalido)
	assert.Contains(t, err.Error(), "DB_USUARIO")
}
