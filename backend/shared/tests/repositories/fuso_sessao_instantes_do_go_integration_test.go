package repositories_test

// TST-23 (Lote 39): integração (INTEGRATION=1; senão t.Skip) dos instantes
// gravados pelo Go com a SESSÃO do MariaDB em outro fuso (time_zone por
// conexão, no DSN só deste teste; o servidor compartilhado não muda).
// Grava com a sessão em +00:00/-03:00/+09:00 e relê pela conexão padrão.

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"github.com/rotaperfumes/shared/testkit/integracao"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// abrirComFusoTST23 abre um pool com time_zone fixo na sessão (o driver
// faz SET time_zone a cada conexão nova).
func abrirComFusoTST23(t *testing.T, fuso string) *sql.DB {
	t.Helper()
	cfg := integracao.Config(t) // SEC-50: pula sem INTEGRATION=1; recusa a produção
	conn, err := db.Open(cfg.DSN() + "&time_zone=" + url.QueryEscape("'"+fuso+"'"))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestIntegracaoTST23_InstantesDoGoIndependemDoFusoDaSessao(t *testing.T) {
	padrao := abrirDBSEC08(t)
	ctx := context.Background()
	var global, sistema string
	require.NoError(t, padrao.QueryRow(`SELECT @@global.time_zone, @@system_time_zone`).Scan(&global, &sistema))
	t.Logf("servidor: @@global.time_zone=%s @@system_time_zone=%s", global, sistema)

	for _, fuso := range []string{"+00:00", "-03:00", "+09:00"} {
		t.Run("sessão "+fuso, func(t *testing.T) {
			conn := abrirComFusoTST23(t, fuso)
			var sessao string
			require.NoError(t, conn.QueryRow(`SELECT @@session.time_zone`).Scan(&sessao))
			require.Equal(t, fuso, sessao, "o DSN deste teste fixa o fuso da sessão")

			n := time.Now().UnixNano()
			res, err := conn.Exec(`INSERT INTO usuarios (nome, email, password_hash, role, ativo) VALUES ('ZZ-TEST-TST23', ?, 'x', 'normal', 1)`,
				fmt.Sprintf("zz-test-tst23-%d@teste.local", n))
			require.NoError(t, err)
			uid, err := res.LastInsertId()
			require.NoError(t, err)
			t.Cleanup(func() {
				_, err := padrao.Exec(`DELETE FROM refresh_tokens WHERE usuario_id = ?`, uid)
				assert.NoError(t, err)
				_, err = padrao.Exec(`DELETE FROM usuarios WHERE id = ? AND nome = 'ZZ-TEST-TST23'`, uid)
				assert.NoError(t, err)
			})

			agora := time.Now().Truncate(time.Second)
			repo := identidademysql.NovoRefreshTokens(conn)
			rt := identidade.NovoRefreshToken(uid, fmt.Sprintf("zz-test-tst23-%d", n), agora, "127.0.0.1", "go-test-tst23")
			require.NoError(t, repo.Inserir(ctx, rt))
			require.NoError(t, repo.Revogar(ctx, rt.ID, identidade.MotivoRotacao))

			// Relido pela sessão em outro fuso e pela padrão: mesmo instante.
			for nome, q := range map[string]*sql.DB{"sessão " + fuso: conn, "conexão padrão": padrao} {
				lido, err := identidademysql.NovoRefreshTokens(q).PorHash(ctx, rt.HashToken)
				require.NoError(t, err, nome)
				assert.True(t, lido.ExpiraEm.Equal(agora.Add(identidade.TTLRefresh)), "%s: expires_at=%s", nome, lido.ExpiraEm)
				require.NotNil(t, lido.RevogadoEm, nome)
				assert.Equal(t, identidade.RefreshRevogadoRecente, lido.Avaliar(time.Now()),
					"%s: revogado agora cai na janela de graça (revoked_at=%s)", nome, lido.RevogadoEm)
			}

			ok, err := repo.MarcarReuso(ctx, rt.ID, agora, agora.Add(-30*time.Minute))
			require.NoError(t, err)
			require.True(t, ok)
			require.NoError(t, identidademysql.NovoUsuarios(conn).InvalidarSessoes(ctx, uid, agora))
			var marca, corte time.Time
			require.NoError(t, padrao.QueryRow(`SELECT reuso_detectado_em FROM refresh_tokens WHERE id = ?`, rt.ID).Scan(&marca))
			require.NoError(t, padrao.QueryRow(`SELECT tokens_validos_desde FROM usuarios WHERE id = ?`, uid).Scan(&corte))
			assert.True(t, marca.Equal(agora), "reuso_detectado_em=%s agora=%s", marca, agora)
			assert.True(t, corte.Equal(agora), "tokens_validos_desde=%s agora=%s", corte, agora)
		})
	}
}
