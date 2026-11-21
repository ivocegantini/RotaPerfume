package catalogo_test

// Integração (INTEGRATION=1 + DB_USUARIO/DB_SENHA; senão t.Skip) do BUG-17
// e do BUG-21 contra o MariaDB do .env.
//
//   - BUG-17: toggles simultâneos de ativo (UPDATE … SET ativo = NOT ativo)
//     se compõem: um número par de toggles volta ao valor original, um
//     ímpar inverte (com ler-inverter-gravar, dois toggles podiam se anular).
//   - BUG-21 → BUG-32 (decisão do usuário, 2026-11-18): a baixa por
//     faturamento parte do último saldo (dia novo herda o saldo anterior),
//     aplica a baixa uma única vez, recalcula a ruptura e NUNCA deixa o saldo
//     negativo (sem saldo: ErrEstoqueInsuficienteFaturamento, nada gravado).
//
// Dados: produto(s) com SKU "ZZ-TEST-L25-<n>" e os registros de estoque
// deles, apagados no t.Cleanup.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/testkit/integracao"
)

// abrirDBLote25 abre o MariaDB de integração (SEC-50: integracao.Abrir pula
// sem INTEGRATION=1/credenciais e recusa a produção antes de conectar).
func abrirDBLote25(t *testing.T) *sql.DB {
	t.Helper()
	return integracao.Abrir(t)
}

// produtoLote25 cria um produto de teste e agenda a limpeza dele e do estoque.
func produtoLote25(t *testing.T, db *sql.DB, ativo bool) (int64, string) {
	t.Helper()
	sku := fmt.Sprintf("ZZ-TEST-L25-%d", time.Now().UnixNano())
	res, err := db.Exec(`INSERT INTO produtos (sku, nome, categoria, marca, preco_tabela, custo_unitario, unidade, ativo)
		VALUES (?, 'ZZ-TEST Lote 25', 'Teste', 'Teste', 1, 1, 'UN', ?)`, sku, ativo)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.Exec(`DELETE FROM estoque WHERE sku = ?`, sku)
		assert.NoError(t, err)
		_, err = db.Exec(`DELETE FROM produtos WHERE id = ?`, id)
		assert.NoError(t, err)
	})
	return id, sku
}

func ativoLote25(t *testing.T, db *sql.DB, id int64) bool {
	t.Helper()
	var ativo bool
	require.NoError(t, db.QueryRow(`SELECT ativo FROM produtos WHERE id = ?`, id).Scan(&ativo))
	return ativo
}

func TestIntegracaoBUG17_TogglesConcorrentes(t *testing.T) {
	db := abrirDBLote25(t)
	casos := catalogoapp.Novo(catalogomysql.NovoProdutos(db), catalogomysql.NovoEstoques(db))
	for _, c := range []struct {
		toggles  int
		original bool
	}{{2, true}, {2, false}, {10, true}, {7, true}, {7, false}} {
		t.Run(fmt.Sprintf("%d toggles a partir de ativo=%t", c.toggles, c.original), func(t *testing.T) {
			id, _ := produtoLote25(t, db, c.original)
			inicio := make(chan struct{})
			var wg sync.WaitGroup
			erros := make(chan error, c.toggles)
			for i := 0; i < c.toggles; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-inicio
					_, err := casos.AlternarAtivoProduto(context.Background(), id, nil)
					erros <- err
				}()
			}
			close(inicio)
			wg.Wait()
			close(erros)
			for err := range erros {
				require.NoError(t, err)
			}
			want := c.original
			if c.toggles%2 == 1 {
				want = !c.original
			}
			assert.Equal(t, want, ativoLote25(t, db, id), "cada toggle conta exatamente uma vez")
		})
	}
}

func TestIntegracaoBUG17_ToggleInexistente_404(t *testing.T) {
	db := abrirDBLote25(t)
	err := catalogomysql.NovoProdutos(db).AlternarAtivo(context.Background(), -1)
	assert.ErrorIs(t, err, catalogo.ErrProdutoNaoEncontrado, "0 linhas (clientFoundRows=true) → 404")
}

type snapshotLote25 struct {
	dia     string
	saldo   int
	ruptura bool
}

func snapshotsLote25(t *testing.T, db *sql.DB, sku string) []snapshotLote25 {
	t.Helper()
	rows, err := db.Query(`SELECT data_snapshot, saldo, ruptura FROM estoque WHERE sku = ? ORDER BY data_snapshot`, sku)
	require.NoError(t, err)
	defer rows.Close()
	var out []snapshotLote25
	for rows.Next() {
		var d time.Time
		var sn snapshotLote25
		require.NoError(t, rows.Scan(&d, &sn.saldo, &sn.ruptura))
		sn.dia = d.Format("2006-01-02")
		out = append(out, sn)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestIntegracaoBUG32_BaixaDoFaturamento(t *testing.T) {
	db := abrirDBLote25(t)
	dia := time.Date(2001, 2, 3, 12, 0, 0, 0, time.Local) // data isolada, só deste teste
	const hoje, antes, depois = "2001-02-03", "2001-01-29", "2001-02-10"
	casos := []struct {
		nome     string
		iniciais []snapshotLote25
		baixa    int
		insuf    bool
		finais   []snapshotLote25
	}{
		{"mesmo dia: 10 − 6 = 4", []snapshotLote25{{hoje, 10, false}}, 6, false, []snapshotLote25{{hoje, 4, false}}},
		{"mesmo dia: 6 − 6 = 0 com ruptura", []snapshotLote25{{hoje, 6, false}}, 6, false, []snapshotLote25{{hoje, 0, true}}},
		{"mesmo dia: limpa ruptura antiga", []snapshotLote25{{hoje, 7, true}}, 6, false, []snapshotLote25{{hoje, 1, false}}},
		{"dia novo herda o último saldo", []snapshotLote25{{antes, 9, false}}, 6, false, []snapshotLote25{{antes, 9, false}, {hoje, 3, false}}},
		{"snapshot futuro é ignorado", []snapshotLote25{{antes, 9, false}, {depois, 1, false}}, 6, false,
			[]snapshotLote25{{antes, 9, false}, {hoje, 3, false}, {depois, 1, false}}},
		{"5 − 6 nunca fica negativo", []snapshotLote25{{hoje, 5, false}}, 6, true, []snapshotLote25{{hoje, 5, false}}},
		{"sem snapshot não fatura", nil, 6, true, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, sku := produtoLote25(t, db, true)
			for _, sn := range c.iniciais {
				_, err := db.Exec(`INSERT INTO estoque (data_snapshot, sku, saldo, ruptura) VALUES (?, ?, ?, ?)`, sn.dia, sku, sn.saldo, sn.ruptura)
				require.NoError(t, err)
			}
			tx, err := db.Begin()
			require.NoError(t, err)
			err = catalogomysql.NovoEstoques(tx).AjustarSaldoPorFaturamento(context.Background(), sku, dia, c.baixa)
			if c.insuf {
				assert.ErrorIs(t, err, catalogo.ErrEstoqueInsuficienteFaturamento)
				require.NoError(t, tx.Rollback())
			} else {
				require.NoError(t, err)
				require.NoError(t, tx.Commit())
			}
			assert.Equal(t, c.finais, snapshotsLote25(t, db, sku))
		})
	}
}

// BUG-32: dois faturamentos simultâneos de 3 com saldo 5 (dia anterior): o
// lock do último snapshot serializa; um baixa (5 → 2) e o outro é recusado.
func TestIntegracaoBUG32_BaixasConcorrentesNaoNegativam(t *testing.T) {
	db := abrirDBLote25(t)
	dia := time.Date(2001, 3, 3, 12, 0, 0, 0, time.Local)
	_, sku := produtoLote25(t, db, true)
	_, err := db.Exec(`INSERT INTO estoque (data_snapshot, sku, saldo, ruptura) VALUES ('2001-03-01', ?, 5, 0)`, sku)
	require.NoError(t, err)
	inicio := make(chan struct{})
	var wg sync.WaitGroup
	erros := make([]error, 2)
	for i := range erros {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-inicio
			tx, err := db.Begin()
			if err != nil {
				erros[i] = err
				return
			}
			if erros[i] = catalogomysql.NovoEstoques(tx).AjustarSaldoPorFaturamento(context.Background(), sku, dia, 3); erros[i] != nil {
				_ = tx.Rollback()
				return
			}
			erros[i] = tx.Commit()
		}(i)
	}
	close(inicio)
	wg.Wait()
	ok, recusado := 0, 0
	for _, e := range erros {
		switch {
		case e == nil:
			ok++
		case errors.Is(e, catalogo.ErrEstoqueInsuficienteFaturamento):
			recusado++
		default:
			t.Errorf("erro inesperado: %v", e)
		}
	}
	assert.Equal(t, []int{1, 1}, []int{ok, recusado})
	assert.Equal(t, []snapshotLote25{{"2001-03-01", 5, false}, {"2001-03-03", 2, false}}, snapshotsLote25(t, db, sku))
}
