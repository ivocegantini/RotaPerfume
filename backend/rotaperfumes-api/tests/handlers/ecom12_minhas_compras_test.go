package handlers_test

// ECOM-12: GET /api/loja/minhas-compras (AnyRole): produtos comprados pelo
// usuário do token (nunca da query), agrupados, sem pedidos cancelados.

import (
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/rotaperfumes/rotaperfumes-api/internal/catalogohttp"
	"github.com/rotaperfumes/rotaperfumes-api/internal/vendashttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ecom12Count = `SELECT COUNT\(DISTINCT i.produto_id\) FROM itens_pedido i .*WHERE c.usuario_id = \? AND p.status <> 'Cancelado'`
	ecom12Lista = `SELECT pr.id, pr.sku, pr.nome, .* WHERE c.usuario_id = \? AND p.status <> 'Cancelado'\s+GROUP BY .* ORDER BY ultima_compra DESC, pr.id DESC LIMIT \? OFFSET \?`
)

var ecom12Colunas = []string{"id", "sku", "nome", "imagem", "ativo", "quantidade", "pedidos", "total_gasto", "ultima_compra"}

func TestECOM12_MinhasCompras_DoUsuarioDoToken(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	dia := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(ecom12Count).WithArgs(idUsuarioLoja).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(2))
	mock.ExpectQuery(ecom12Lista).WithArgs(idUsuarioLoja, 20, 0).WillReturnRows(sqlmock.NewRows(ecom12Colunas).
		AddRow(int64(7), "SKU-007", "Perfume A", "7-abc.webp", true, 3, 2, "299.70", dia).
		AddRow(int64(8), "SKU-008", "Perfume B", "", false, 1, 1, "50.00", dia))

	// ?usuario_id e ?cliente_id da query são ignorados: vale só o token.
	resp := getLoja(t, server.URL+"/api/loja/minhas-compras?usuario_id=1&cliente_id=5", tok)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	raw := readBody(t, resp)
	assert.NotContains(t, string(raw), "custo")
	corpo := decodeResponse(t, raw)
	data := como[[]any](t, corpo["data"])
	require.Len(t, data, 2)
	a := como[map[string]any](t, data[0])
	assert.Equal(t, float64(7), a["produto_id"])
	assert.Equal(t, "Perfume A", a["nome"])
	assert.Equal(t, float64(3), a["quantidade"])
	assert.Equal(t, float64(2), a["pedidos"])
	assert.Equal(t, 299.7, a["total_gasto"])
	assert.Equal(t, "2026-11-10", a["ultima_compra"])
	assert.Equal(t, true, a["ativo"])
	b := como[map[string]any](t, data[1])
	assert.Nil(t, b["imagem_url"])
	assert.Equal(t, false, b["ativo"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM12_MinhasCompras_SemComprasDaListaVazia(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), 1, "admin") // equipe: sem cliente da loja
	mock.ExpectQuery(ecom12Count).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
	mock.ExpectQuery(ecom12Lista).WithArgs(int64(1), 20, 0).WillReturnRows(sqlmock.NewRows(ecom12Colunas))

	resp := getLoja(t, server.URL+"/api/loja/minhas-compras", tok)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := como[[]any](t, decodeResponse(t, readBody(t, resp))["data"])
	assert.Empty(t, data)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM12_MinhasCompras_SemTokenDa401(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	resp, err := http.Get(server.URL + "/api/loja/minhas-compras")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM12_PrefixoDaImagemIgualAoDoCatalogo(t *testing.T) {
	assert.Equal(t, catalogohttp.PrefixoURLImagem, vendashttp.PrefixoURLImagem)
}

func TestECOM12_MinhasCompras_OrdenacaoPelaWhitelist(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	casos := []struct{ query, ordem string }{
		{"order_by=total_gasto&order_dir=asc", `ORDER BY total_gasto ASC, pr.id ASC`},
		{"order_by=nome", `ORDER BY pr.nome DESC, pr.id DESC`},
		{"order_by=custo_unitario", `ORDER BY ultima_compra DESC, pr.id DESC`}, // fora da whitelist: padrão
	}
	for _, c := range casos {
		mock.ExpectQuery(ecom12Count).WithArgs(idUsuarioLoja).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
		mock.ExpectQuery(`GROUP BY .* `+c.ordem+` LIMIT \? OFFSET \?`).WithArgs(idUsuarioLoja, 20, 0).WillReturnRows(sqlmock.NewRows(ecom12Colunas))
		resp := getLoja(t, server.URL+"/api/loja/minhas-compras?"+c.query, tok)
		assert.Equal(t, http.StatusOK, resp.StatusCode, c.query)
		resp.Body.Close()
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}
