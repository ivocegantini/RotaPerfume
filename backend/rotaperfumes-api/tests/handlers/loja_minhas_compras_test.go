package handlers_test

// ECOM-12/16: GET /api/loja/minhas-compras (AnyRole): um pedido por linha
// (ID, data, status, total, nomes dos produtos) do usuário do token (nunca
// da query); GET /api/loja/minhas-compras/{id}: itens e pagamento; de outro
// login → 404.

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
	ecom16Count    = `SELECT COUNT\(\*\) FROM pedidos p\s+JOIN clientes c ON c.cliente_id_origem = p.cliente_id\s+WHERE c.usuario_id = \?`
	ecom16Lista    = `SELECT p.pedido_id_origem, p.data_pedido, p.status, p.valor_total FROM pedidos p .*WHERE c.usuario_id = \? ORDER BY p.data_pedido DESC, p.pedido_id_origem DESC LIMIT \? OFFSET \?`
	ecom16Nomes    = `SELECT i.pedido_id, i.produto_id, pr.nome FROM itens_pedido i JOIN produtos pr .*WHERE i.pedido_id IN \(\?, \?\) ORDER BY i.pedido_id, i.item_id_origem`
	ecom16Cab      = `SELECT p.pedido_id_origem, p.data_pedido, p.status, p.valor_total, p.entrega_cep, .*p.entrega_uf FROM pedidos p .*WHERE c.usuario_id = \? AND p.pedido_id_origem = \? LIMIT 1`
	ecom16Itens    = `SELECT pr.id, pr.sku, pr.nome, .* FROM itens_pedido i JOIN produtos pr ON pr.id = i.produto_id WHERE i.pedido_id = \?`
	ecom16Pagament = `SELECT forma_pagamento, parcelas, valor, data_vencimento, data_pagamento, status_pagamento\s+FROM pagamentos WHERE pedido_id = \?`
)

var (
	ecom16Colunas    = []string{"pedido_id_origem", "data_pedido", "status", "valor_total"}
	ecom16ColunasCab = append(append([]string{}, ecom16Colunas...), "entrega_cep", "entrega_endereco", "entrega_numero",
		"entrega_complemento", "entrega_bairro", "entrega_cidade", "entrega_uf") // ECOM-17
	ecom16Dia = time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
)

func TestECOM16_MinhasCompras_PorPedidoDoUsuarioDoToken(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")

	mock.ExpectQuery(ecom16Count).WithArgs(idUsuarioLoja).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(2))
	mock.ExpectQuery(ecom16Lista).WithArgs(idUsuarioLoja, 20, 0).WillReturnRows(sqlmock.NewRows(ecom16Colunas).
		AddRow(int64(31), ecom16Dia, "Faturado", "299.70").
		AddRow(int64(30), ecom16Dia.AddDate(0, -1, 0), "Cancelado", "50.00"))
	mock.ExpectQuery(ecom16Nomes).WithArgs(int64(31), int64(30)).WillReturnRows(sqlmock.NewRows([]string{"pedido_id", "produto_id", "nome"}).
		AddRow(int64(30), int64(8), "Perfume B").
		AddRow(int64(31), int64(7), "Perfume A").
		AddRow(int64(31), int64(9), "Perfume C").
		AddRow(int64(31), int64(7), "Perfume A")) // repetido no pedido: sai uma vez

	// ?usuario_id e ?cliente_id da query são ignorados: vale só o token.
	resp := getLoja(t, server.URL+"/api/loja/minhas-compras?usuario_id=1&cliente_id=5", tok)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	raw := readBody(t, resp)
	assert.NotContains(t, string(raw), "vendedor")
	data := como[[]any](t, decodeResponse(t, raw)["data"])
	require.Len(t, data, 2)
	a := como[map[string]any](t, data[0])
	assert.Equal(t, float64(31), a["pedido_id"])
	assert.Equal(t, "2026-11-10", a["data"])
	assert.Equal(t, "Faturado", a["status"])
	assert.Equal(t, 299.7, a["valor_total"])
	assert.Equal(t, []any{"Perfume A", "Perfume C"}, a["produtos"])
	b := como[map[string]any](t, data[1])
	assert.Equal(t, []any{"Perfume B"}, b["produtos"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM16_MinhasCompras_SemComprasDaListaVazia(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), 1, "admin") // equipe: sem cliente da loja
	mock.ExpectQuery(ecom16Count).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
	mock.ExpectQuery(ecom16Lista).WithArgs(int64(1), 20, 0).WillReturnRows(sqlmock.NewRows(ecom16Colunas))

	resp := getLoja(t, server.URL+"/api/loja/minhas-compras", tok)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := como[[]any](t, decodeResponse(t, readBody(t, resp))["data"])
	assert.Empty(t, data)
	assert.NoError(t, mock.ExpectationsWereMet()) // sem a query dos nomes
}

func TestECOM16_MinhasCompras_SemTokenDa401(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	for _, u := range []string{"/api/loja/minhas-compras", "/api/loja/minhas-compras/31"} {
		resp, err := http.Get(server.URL + u)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, u)
		resp.Body.Close()
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM12_PrefixoDaImagemIgualAoDoCatalogo(t *testing.T) {
	assert.Equal(t, catalogohttp.PrefixoURLImagem, vendashttp.PrefixoURLImagem)
}

func TestECOM16_MinhasCompras_OrdenacaoPelaWhitelist(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	casos := []struct{ query, ordem string }{
		{"order_by=valor_total&order_dir=asc", `ORDER BY p.valor_total ASC, p.pedido_id_origem ASC`},
		{"order_by=pedido_id", `ORDER BY p.pedido_id_origem DESC, p.pedido_id_origem DESC`},
		{"order_by=custo_unitario", `ORDER BY p.data_pedido DESC, p.pedido_id_origem DESC`}, // fora da whitelist: padrão
	}
	for _, c := range casos {
		mock.ExpectQuery(ecom16Count).WithArgs(idUsuarioLoja).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
		mock.ExpectQuery(c.ordem+` LIMIT \? OFFSET \?`).WithArgs(idUsuarioLoja, 20, 0).WillReturnRows(sqlmock.NewRows(ecom16Colunas))
		resp := getLoja(t, server.URL+"/api/loja/minhas-compras?"+c.query, tok)
		assert.Equal(t, http.StatusOK, resp.StatusCode, c.query)
		resp.Body.Close()
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM16_MinhaCompra_ItensEPagamento(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")

	mock.ExpectQuery(ecom16Cab).WithArgs(idUsuarioLoja, int64(31)).WillReturnRows(sqlmock.NewRows(ecom16ColunasCab).
		AddRow(int64(31), ecom16Dia, "Faturado", "299.70", "01310100", "Avenida Paulista", "1000", "ap 5", "Bela Vista", "São Paulo", "SP"))
	mock.ExpectQuery(ecom16Itens).WithArgs(int64(31)).WillReturnRows(sqlmock.NewRows(
		[]string{"id", "sku", "nome", "imagem", "ativo", "quantidade", "preco_praticado", "desconto_pct", "valor_bruto"}).
		AddRow(int64(7), "SKU-007", "Perfume A", "0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d.webp", true, 3, "99.90", 0.0, "299.70"))
	mock.ExpectQuery(ecom16Pagament).WithArgs(int64(31)).WillReturnRows(sqlmock.NewRows(
		[]string{"forma_pagamento", "parcelas", "valor", "data_vencimento", "data_pagamento", "status_pagamento"}).
		AddRow("PIX", 1, "299.70", ecom16Dia, ecom16Dia, "Pago"))

	resp := getLoja(t, server.URL+"/api/loja/minhas-compras/31", tok)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	raw := readBody(t, resp)
	assert.NotContains(t, string(raw), "taxa")
	assert.NotContains(t, string(raw), "liquido")
	corpo := como[map[string]any](t, decodeResponse(t, raw)["data"])
	assert.Equal(t, float64(31), corpo["pedido_id"])
	assert.Equal(t, map[string]any{"cep": "01310100", "endereco": "Avenida Paulista", "numero": "1000", "complemento": "ap 5",
		"bairro": "Bela Vista", "cidade": "São Paulo", "uf": "SP"}, corpo["entrega"], "ECOM-17: endereço no detalhe")
	itens := como[[]any](t, corpo["itens"])
	require.Len(t, itens, 1)
	it := como[map[string]any](t, itens[0])
	assert.Equal(t, "Perfume A", it["nome"])
	assert.Equal(t, float64(3), it["quantidade"])
	assert.Equal(t, 99.9, it["preco_praticado"])
	assert.Equal(t, vendashttp.PrefixoURLImagem+"0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d.webp", it["imagem_url"])
	pags := como[[]any](t, corpo["pagamentos"])
	require.Len(t, pags, 1)
	pg := como[map[string]any](t, pags[0])
	assert.Equal(t, "PIX", pg["forma_pagamento"])
	assert.Equal(t, "Pago", pg["status_pagamento"])
	assert.Equal(t, "2026-11-10", pg["data_pagamento"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM16_MinhaCompra_PedidoAlheioDa404(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	mock.ExpectQuery(ecom16Cab).WithArgs(idUsuarioLoja, int64(5)).WillReturnRows(sqlmock.NewRows(ecom16ColunasCab))

	resp := getLoja(t, server.URL+"/api/loja/minhas-compras/5", tok)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp2 := getLoja(t, server.URL+"/api/loja/minhas-compras/abc", tok)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp2.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet()) // id inválido: sem SQL
}
