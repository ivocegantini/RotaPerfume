package handlers_test

// ARQ-09 (HTTP): o contrato JSON segue em reais decimais. A entrada é lida
// sem float64 (mais de 2 casas e negativo são 400) e a resposta sai igual.

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func postJSONArq09(t *testing.T, url, token, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, string(readBody(t, resp))
}

const pedidoArq09 = `{"cliente_id":1,"vendedor_id":2,"data_pedido":"2024-01-15","canal":"App","status":"Em separação",
	"itens":[{"produto_id":10,"quantidade":3,"preco_praticado":%s,"desconto_pct":10}]}`

// 3 × 33.33 com 10% = 89.991: grava "89.99" (texto exato) e responde 89.99.
func TestARQ09_HTTP_CriarPedido_CentavosExatos(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO pedidos`).
		WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Em separação", "89.99").
		WillReturnResult(sqlmock.NewResult(100, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido`).
		WithArgs(int64(100), int64(10), 3, "33.33", 10.0, "89.99").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	now := time.Now()
	mock.ExpectQuery(`SELECT ` + pedidoColunasRegexH + pedidoFromRegexH + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows(pedidoColunasHeaderForHandler()).
			AddRow(int64(100), int64(1), int64(2), now, "App", "Em separação", []byte("89.99"), now, now, "C", "V"))
	mock.ExpectQuery(`SELECT ` + itemPedidoColunasRegexH + itemPedidoFromRegexH + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows(itemPedidoColunasHeaderForHandler()).
			AddRow(int64(1), int64(100), int64(10), 3, []byte("33.33"), []byte("10.00"), []byte("89.99"), now, now, "SKU-010", "A"))

	status, body := postJSONArq09(t, server.URL+"/api/pedidos", generateToken(t, testCfg(), 1, "admin"), strings.Replace(pedidoArq09, "%s", "33.33", 1))
	assert.Equal(t, http.StatusCreated, status, body)
	assert.Contains(t, body, `"valor_total":89.99,`)
	assert.Contains(t, body, `"preco_praticado":33.33,"desconto_pct":10,"valor_bruto":89.99,`)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Entrada inválida responde 400 antes de qualquer SQL.
func TestARQ09_HTTP_CriarPedido_EntradaRecusada(t *testing.T) {
	casos := map[string]string{
		"10.005":  "preco_praticado deve ter no máximo 2 casas decimais em todos os itens",
		"-0.01":   "preco_praticado deve ser maior ou igual a zero em todos os itens",
		"1e20":    "preco_praticado deve ser maior ou igual a zero em todos os itens",
		`"10.00"`: "body JSON inválido",
		"true":    "body JSON inválido",
	}
	for preco, msg := range casos {
		t.Run(preco, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			status, body := postJSONArq09(t, server.URL+"/api/pedidos", generateToken(t, testCfg(), 1, "admin"), strings.Replace(pedidoArq09, "%s", preco, 1))
			assert.Equal(t, http.StatusBadRequest, status, body)
			assert.Contains(t, body, `"error":"`+msg+`"`)
			assert.NoError(t, mock.ExpectationsWereMet(), "sem SQL")
		})
	}
}
