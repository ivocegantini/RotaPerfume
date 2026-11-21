package config_test

// BUG-31 (MERGE-01): com DB_PORT vazio, "-P $(DB_PORT)" fazia o cliente
// mariadb engolir o "--default-character-set=utf8mb4" como porta.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBUG31_MysqlOpts_PortaVaziaVira3306(t *testing.T) {
	alvo := `rp-bug31: ; @printf '%s\n' "$(MYSQL_OPTS)"`
	for _, c := range []struct{ porta, esperado string }{{"", "3306"}, {"3307", "3307"}} {
		out, err := makeAlvo(t, copiaDoMakefile(t), "rp-bug31", nil,
			"--eval", alvo, "DB_PORT="+c.porta, "DB_HOST=127.0.0.1", "DB_USUARIO=x")
		require.NoError(t, err, out)
		opts := strings.Fields(out)
		i := indiceDe(opts, "-P")
		require.GreaterOrEqual(t, i, 0, out)
		require.Less(t, i+1, len(opts), out)
		assert.Equal(t, c.esperado, opts[i+1], "DB_PORT=%q", c.porta)
		assert.Contains(t, opts, "--default-character-set=utf8mb4")
	}
}

func indiceDe(lista []string, v string) int {
	for i, s := range lista {
		if s == v {
			return i
		}
	}
	return -1
}
