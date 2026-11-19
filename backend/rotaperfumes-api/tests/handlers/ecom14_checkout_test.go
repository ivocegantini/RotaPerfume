package handlers_test

// ECOM-14: POST /api/loja/checkout (AnyRole): fecha o carrinho do usuário do
// token. Preço e total vêm do banco (o corpo só manda produto e quantidade);
// mesma chave devolve o mesmo pedido; inativo → 409; equipe sem cliente → 403.

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ecom14Chave       = "0b7f3c1e-5d2a-4c8e-9f10-123456789abc"
	ecom14Cliente     = `SELECT cliente_id_origem FROM clientes WHERE usuario_id = \? FOR UPDATE`
	ecom14PorChave    = `SELECT pedido_id_origem, cliente_id FROM pedidos WHERE chave_checkout = \?`
	ecom14Vendedor    = `SELECT id FROM vendedores WHERE nome = \? ORDER BY id LIMIT 1`
	ecom14Produto     = `SELECT preco_tabela, sku FROM produtos WHERE id = \? AND ativo = 1 FOR UPDATE`
	ecom14Saldo       = `SELECT saldo FROM estoque WHERE sku = \? ORDER BY data_snapshot DESC, id DESC LIMIT 1 LOCK IN SHARE MODE`
	ecom14Reservado   = `SELECT COALESCE\(SUM\(i.quantidade\), 0\) FROM itens_pedido i JOIN pedidos p .* p.canal = 'Loja' AND p.status = 'Em separação' LOCK IN SHARE MODE`
	ecom14InsPedido   = `INSERT INTO pedidos \(cliente_id, vendedor_id, data_pedido, canal, status, valor_total,\s+entrega_cep, entrega_endereco, entrega_numero, entrega_complemento, entrega_bairro, entrega_cidade, entrega_uf\)`
	ecom14InsItem     = `INSERT INTO itens_pedido`
	ecom14Chavear     = `UPDATE pedidos SET chave_checkout = \? WHERE pedido_id_origem = \?`
	ecom14InsPag      = `INSERT INTO pagamentos`
	ecom14PagDoPedido = `SELECT pagamento_id, .* FROM pagamentos WHERE pedido_id = \? ORDER BY pagamento_id LIMIT 1`
)

func postCheckout(t *testing.T, url, tok string, corpo any) *http.Response {
	t.Helper()
	b, err := json.Marshal(corpo)
	require.NoError(t, err)
	req, _ := http.NewRequest("POST", url+"/api/loja/checkout", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	return resp
}

func corpoCheckout(forma string) map[string]any {
	return map[string]any{
		"chave":           ecom14Chave,
		"forma_pagamento": forma,
		// preco/valor_total do cliente são ignorados (não existem no contrato).
		"itens":       []map[string]any{{"produto_id": 7, "quantidade": 2, "preco": 0.01}, {"produto_id": 8, "quantidade": 1}},
		"valor_total": 0.01,
		"entrega":     entregaPayloadH(), // ECOM-17
	}
}

// esperarOferta: produto ativo travado, saldo do último snapshot e o já
// reservado em pedidos da Loja Em separação (SEC-79).
func esperarOferta(mock sqlmock.Sqlmock, id int64, preco string, saldo, reservado int) {
	sku := fmt.Sprintf("SKU-%d", id)
	mock.ExpectQuery(ecom14Produto).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"preco_tabela", "sku"}).AddRow(preco, sku))
	mock.ExpectQuery(ecom14Saldo).WithArgs(sku).WillReturnRows(sqlmock.NewRows([]string{"saldo"}).AddRow(saldo))
	mock.ExpectQuery(ecom14Reservado).WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"r"}).AddRow(reservado))
}

// esperarDetalhe é a releitura do pedido (cabeçalho + itens) depois do commit.
func esperarDetalhe(mock sqlmock.Sqlmock, id int64) {
	now := time.Now()
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).WithArgs(id).
		WillReturnRows(sqlmock.NewRows(pedidoColunasHeaderForHandler()).
			AddRow(id, int64(55), int64(168), time.Date(2026, 11, 18, 0, 0, 0, 0, time.Local), "Loja", "Em separação", "250.50",
				"01310100", "Avenida Paulista", "1000", "", "Bela Vista", "São Paulo", "SP", now, now, "Ana", "Loja Online"))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH).WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"item_id_origem", "pedido_id", "produto_id", "quantidade", "preco_praticado", "desconto_pct", "valor_bruto", "created_at", "updated_at", "sku", "nome"}).
			AddRow(int64(1), id, int64(7), 2, "100.25", 0.0, "200.50", now, now, "SKU-7", "Perfume Oud").
			AddRow(int64(2), id, int64(8), 1, "50.00", 0.0, "50.00", now, now, "SKU-8", "Perfume B"))
}

func TestECOM14_Checkout_CriaPedidoComPrecosDoBanco(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")

	mock.ExpectBegin()
	mock.ExpectQuery(ecom14Cliente).WithArgs(idUsuarioLoja).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(55)))
	mock.ExpectQuery(ecom14PorChave).WithArgs(ecom14Chave).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(ecom14Vendedor).WithArgs("Loja Online").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(168)))
	esperarOferta(mock, 7, "100.25", 10, 3) // 7 disponíveis, pede 2
	esperarOferta(mock, 8, "50.00", 1, 0)   // 1 disponível, pede 1
	mock.ExpectExec(ecom14InsPedido).WithArgs(int64(55), int64(168), sqlmock.AnyArg(), "Loja", "Em separação", "250.50", "01310100", "Avenida Paulista", "1000", "", "Bela Vista", "São Paulo", "SP").
		WillReturnResult(sqlmock.NewResult(1000, 1))
	mock.ExpectExec(ecom14InsItem).WithArgs(int64(1000), int64(7), 2, "100.25", 0.0, "200.50").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(ecom14InsItem).WithArgs(int64(1000), int64(8), 1, "50.00", 0.0, "50.00").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec(ecom14Chavear).WithArgs(ecom14Chave, int64(1000)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(ecom14InsPag).WithArgs(int64(1000), "Boleto 14 dias", uint8(1), "250.50", 0.0, "250.50", sqlmock.AnyArg(), nil, "Em aberto").
		WillReturnResult(sqlmock.NewResult(77, 1))
	mock.ExpectCommit()
	esperarDetalhe(mock, 1000)

	resp := postCheckout(t, server.URL, tok, corpoCheckout("boleto"))
	defer resp.Body.Close()
	raw := readBody(t, resp)
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(raw))
	assert.NotContains(t, string(raw), "vendedor")
	assert.NotContains(t, string(raw), "custo")
	c := como[map[string]any](t, decodeResponse(t, raw)["data"])
	assert.Equal(t, float64(1000), c["pedido_id"])
	assert.Equal(t, 250.5, c["valor_total"])
	assert.Equal(t, "Em separação", c["status"])
	assert.Equal(t, "Boleto 14 dias", c["forma_pagamento"])
	assert.Equal(t, "Em aberto", c["status_pagamento"])
	assert.Equal(t, false, c["repetida"])
	assert.Equal(t, map[string]any{"cep": "01310100", "endereco": "Avenida Paulista", "numero": "1000", "complemento": "",
		"bairro": "Bela Vista", "cidade": "São Paulo", "uf": "SP"}, c["entrega"], "ECOM-17")
	itens := como[[]any](t, c["itens"])
	require.Len(t, itens, 2)
	assert.Equal(t, 100.25, como[map[string]any](t, itens[0])["preco"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM14_Checkout_MesmaChaveDevolveOMesmoPedido(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	venc := time.Date(2026, 11, 18, 0, 0, 0, 0, time.Local)

	mock.ExpectBegin()
	mock.ExpectQuery(ecom14Cliente).WithArgs(idUsuarioLoja).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(55)))
	mock.ExpectQuery(ecom14PorChave).WithArgs(ecom14Chave).WillReturnRows(sqlmock.NewRows([]string{"p", "c"}).AddRow(int64(1000), int64(55)))
	mock.ExpectQuery(ecom14PagDoPedido).WithArgs(int64(1000)).WillReturnRows(sqlmock.NewRows(
		[]string{"pagamento_id", "pedido_id", "forma_pagamento", "parcelas", "valor", "taxa_pct", "valor_liquido", "data_vencimento", "data_pagamento", "status_pagamento", "created_at", "updated_at"}).
		AddRow(int64(77), int64(1000), "PIX", 1, "250.50", 0.0, "250.50", venc, nil, "Em aberto", venc, venc))
	mock.ExpectCommit()
	esperarDetalhe(mock, 1000)

	resp := postCheckout(t, server.URL, tok, corpoCheckout("pix"))
	defer resp.Body.Close()
	raw := readBody(t, resp)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(raw))
	c := como[map[string]any](t, decodeResponse(t, raw)["data"])
	assert.Equal(t, true, c["repetida"])
	assert.Equal(t, "PIX", c["forma_pagamento"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM14_Checkout_ChaveDeOutroClienteDa409(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	mock.ExpectBegin()
	mock.ExpectQuery(ecom14Cliente).WithArgs(idUsuarioLoja).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(55)))
	mock.ExpectQuery(ecom14PorChave).WithArgs(ecom14Chave).WillReturnRows(sqlmock.NewRows([]string{"p", "c"}).AddRow(int64(1000), int64(99)))
	mock.ExpectRollback()

	resp := postCheckout(t, server.URL, tok, corpoCheckout("pix"))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM14_Checkout_ProdutoInativoDa409SemGravar(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	mock.ExpectBegin()
	mock.ExpectQuery(ecom14Cliente).WithArgs(idUsuarioLoja).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(55)))
	mock.ExpectQuery(ecom14PorChave).WithArgs(ecom14Chave).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(ecom14Vendedor).WithArgs("Loja Online").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(168)))
	esperarOferta(mock, 7, "100.25", 10, 0)
	mock.ExpectQuery(ecom14Produto).WithArgs(int64(8)).WillReturnError(sql.ErrNoRows) // 8 inativo
	mock.ExpectRollback()

	resp := postCheckout(t, server.URL, tok, corpoCheckout("pix"))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Contains(t, string(readBody(t, resp)), "não estão mais disponíveis")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM14_Checkout_EquipeSemClienteDa403(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), 1, "admin")
	mock.ExpectBegin()
	mock.ExpectQuery(ecom14Cliente).WithArgs(int64(1)).WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	resp := postCheckout(t, server.URL, tok, corpoCheckout("pix"))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM14_Checkout_CorpoInvalidoDa400SemSQL(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	item := func(id, q int) map[string]any { return map[string]any{"produto_id": id, "quantidade": q} }
	casos := map[string]map[string]any{
		"chave invalida":   {"chave": "x' OR 1=1", "forma_pagamento": "pix", "itens": []any{item(7, 1)}},
		"forma invalida":   {"chave": ecom14Chave, "forma_pagamento": "dinheiro", "itens": []any{item(7, 1)}},
		"carrinho vazio":   {"chave": ecom14Chave, "forma_pagamento": "pix", "itens": []any{}},
		"quantidade 0":     {"chave": ecom14Chave, "forma_pagamento": "pix", "itens": []any{item(7, 0)}},
		"quantidade 100":   {"chave": ecom14Chave, "forma_pagamento": "pix", "itens": []any{item(7, 100)}},
		"produto 0":        {"chave": ecom14Chave, "forma_pagamento": "pix", "itens": []any{item(0, 1)}},
		"produto repetido": {"chave": ecom14Chave, "forma_pagamento": "pix", "itens": []any{item(7, 1), item(7, 2)}},
		// ECOM-17: sem endereço completo a compra não é finalizada (400, sem SQL).
		"sem entrega": {"chave": ecom14Chave, "forma_pagamento": "pix", "itens": []any{item(7, 1)}},
		"sem numero": {"chave": ecom14Chave, "forma_pagamento": "pix", "itens": []any{item(7, 1)},
			"entrega": map[string]any{"cep": "01310100", "endereco": "Av. Paulista", "bairro": "Bela Vista", "cidade": "São Paulo", "uf": "SP"}},
		"uf invalida": {"chave": ecom14Chave, "forma_pagamento": "pix", "itens": []any{item(7, 1)},
			"entrega": map[string]any{"cep": "01310100", "endereco": "Av. Paulista", "numero": "1", "bairro": "Bela Vista", "cidade": "São Paulo", "uf": "XX"}},
	}
	for nome, corpo := range casos {
		resp := postCheckout(t, server.URL, tok, corpo)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, nome)
		resp.Body.Close()
	}
	assert.NoError(t, mock.ExpectationsWereMet()) // nenhum SQL
}

func TestECOM14_Checkout_SemTokenDa401(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	resp := postCheckout(t, server.URL, "", corpoCheckout("pix"))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// SEC-79: o reservado em pedidos da Loja Em separação sai do saldo; acima do
// disponível, 409 sem gravar. Produtos travados em ordem crescente de id.
func TestSEC79_Checkout_SemEstoqueDa409SemGravar(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	mock.ExpectBegin()
	mock.ExpectQuery(ecom14Cliente).WithArgs(idUsuarioLoja).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(55)))
	mock.ExpectQuery(ecom14PorChave).WithArgs(ecom14Chave).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(ecom14Vendedor).WithArgs("Loja Online").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(168)))
	esperarOferta(mock, 7, "100.25", 5, 4) // só 1 disponível; o carrinho pede 2
	esperarOferta(mock, 8, "50.00", 10, 0)
	mock.ExpectRollback()

	// itens fora de ordem no corpo: o lock continua 7 → 8.
	corpo := corpoCheckout("pix")
	corpo["itens"] = []map[string]any{{"produto_id": 8, "quantidade": 1}, {"produto_id": 7, "quantidade": 2}}
	resp := postCheckout(t, server.URL, tok, corpo)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Contains(t, string(readBody(t, resp)), "estoque insuficiente")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// SEC-79: no máximo 10 tentativas de checkout por usuário em 10 min; a 11ª
// é 429 com Retry-After, sem tocar no banco (corpo inválido: 400 sem SQL).
func TestSEC79_Checkout_LimitePorUsuarioDa429(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	invalido := map[string]any{"chave": "x", "forma_pagamento": "pix", "itens": []any{}}
	for i := 1; i <= 10; i++ {
		resp := postCheckout(t, server.URL, tok, invalido)
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, "tentativa %d", i)
		resp.Body.Close()
	}
	resp := postCheckout(t, server.URL, tok, invalido)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("Retry-After"))
	assert.NoError(t, mock.ExpectationsWereMet())

	// Outro usuário (admin) no mesmo IP ainda passa: o limite de IP é 30.
	outro := postCheckout(t, server.URL, generateToken(t, testCfg(), 1, "admin"), invalido)
	defer outro.Body.Close()
	assert.Equal(t, http.StatusBadRequest, outro.StatusCode)
}
