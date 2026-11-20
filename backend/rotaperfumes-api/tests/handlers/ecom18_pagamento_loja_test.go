package handlers_test

// ECOM-18: POST /api/loja/pagamentos/{id}/simular aprova ou recusa o pagamento
// de um pedido da loja "Aguardando pagamento" (só APP_ENV dev/qa; senão 404).
// GET /api/ambiente expõe o APP_ENV para o selo do rodapé.

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ecom18TravarPedido = `SELECT p.pedido_id_origem, p.canal, p.status, COALESCE\(c.usuario_id, 0\)\s+FROM pedidos p JOIN clientes c ON c.cliente_id_origem = p.cliente_id\s+WHERE p.pedido_id_origem = \? FOR UPDATE`
	ecom18TravarPag    = `SELECT pagamento_id, .* FROM pagamentos WHERE pedido_id = \? ORDER BY pagamento_id LIMIT 1 FOR UPDATE`
	ecom18StatusPedido = `UPDATE pedidos SET status = \? WHERE pedido_id_origem = \?`
	ecom18StatusPag    = `UPDATE pagamentos SET status_pagamento = \?, data_pagamento = \? WHERE pagamento_id = \?`
)

// setupServidorAmbiente é o setupTestServer com o APP_ENV informado.
func setupServidorAmbiente(t *testing.T, a config.Ambiente) (*httptest.Server, *sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	cfg := testCfg()
	cfg.Ambiente = a
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	hs := bootstrap.NovosHandlersCom(db, cfg, novaIdentidadeTeste(db, cfg).casos)
	server := httptest.NewServer(bootstrap.HandlerHTTP(cfg, fakeUserStatusChecker{}, hs))
	t.Cleanup(func() { server.Close(); _ = db.Close() })
	return server, db, mock
}

func postSimular(t *testing.T, url, tok, caminho, corpo string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url+caminho, bytes.NewBufferString(corpo))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, readBody(t, resp)
}

func linhaPedidoLoja(status string, usuarioID int64) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"pedido_id_origem", "canal", "status", "usuario_id"}).AddRow(int64(1000), "Loja", status, usuarioID)
}

func linhaPagamentoLoja(status string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{"pagamento_id", "pedido_id", "forma_pagamento", "parcelas", "valor", "taxa_pct", "valor_liquido",
		"data_vencimento", "data_pagamento", "status_pagamento", "created_at", "updated_at"}).
		AddRow(int64(77), int64(1000), "PIX", uint8(1), "250.50", 0.0, "250.50", now, nil, status, now, now)
}

func TestECOM18_Simulador_DesligadoEmProducaoDa404SemSQL(t *testing.T) {
	for _, a := range []config.Ambiente{"", config.AmbienteProducao} {
		server, _, mock := setupServidorAmbiente(t, a)
		tok := generateToken(t, testCfg(), 1, "admin")
		st, raw := postSimular(t, server.URL, tok, "/api/loja/pagamentos/1000/simular", `{"resultado":"aprovado"}`)
		assert.Equal(t, http.StatusNotFound, st, string(raw))
		assert.NoError(t, mock.ExpectationsWereMet(), "nenhum SQL com o simulador desligado")
	}
}

func TestECOM18_Simulador_AprovadoVaiParaSeparacao(t *testing.T) {
	server, _, mock := setupServidorAmbiente(t, config.AmbienteQA)
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	mock.ExpectBegin()
	mock.ExpectQuery(ecom18TravarPedido).WithArgs(int64(1000)).WillReturnRows(linhaPedidoLoja("Aguardando pagamento", idUsuarioLoja))
	mock.ExpectQuery(ecom18TravarPag).WithArgs(int64(1000)).WillReturnRows(linhaPagamentoLoja("Em aberto"))
	mock.ExpectExec(ecom18StatusPedido).WithArgs("Em separação", int64(1000)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(ecom18StatusPag).WithArgs("Pago", sqlmock.AnyArg(), int64(77)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	esperarDetalheComStatus(mock, 1000, "Em separação")

	st, raw := postSimular(t, server.URL, tok, "/api/loja/pagamentos/1000/simular", `{"resultado":"aprovado"}`)
	require.Equal(t, http.StatusOK, st, string(raw))
	c := como[map[string]any](t, decodeResponse(t, raw)["data"])
	assert.Equal(t, "Em separação", c["status"])
	assert.Equal(t, "Pago", c["status_pagamento"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM18_Simulador_RecusadoCancelaOPedido(t *testing.T) {
	server, _, mock := setupServidorAmbiente(t, config.AmbienteDev)
	tok := generateToken(t, testCfg(), 1, "admin") // admin simula qualquer pedido
	mock.ExpectBegin()
	mock.ExpectQuery(ecom18TravarPedido).WithArgs(int64(1000)).WillReturnRows(linhaPedidoLoja("Aguardando pagamento", idUsuarioLoja))
	mock.ExpectQuery(ecom18TravarPag).WithArgs(int64(1000)).WillReturnRows(linhaPagamentoLoja("Em aberto"))
	mock.ExpectExec(ecom18StatusPedido).WithArgs("Cancelado", int64(1000)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(ecom18StatusPag).WithArgs("Recusado", nil, int64(77)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	esperarDetalheComStatus(mock, 1000, "Cancelado")

	st, raw := postSimular(t, server.URL, tok, "/api/loja/pagamentos/1000/simular", `{"resultado":"recusado"}`)
	require.Equal(t, http.StatusOK, st, string(raw))
	c := como[map[string]any](t, decodeResponse(t, raw)["data"])
	assert.Equal(t, "Cancelado", c["status"])
	assert.Equal(t, "Recusado", c["status_pagamento"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM18_Simulador_PedidoDeOutroLoginDa404(t *testing.T) {
	server, _, mock := setupServidorAmbiente(t, config.AmbienteQA)
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	mock.ExpectBegin()
	mock.ExpectQuery(ecom18TravarPedido).WithArgs(int64(1000)).WillReturnRows(linhaPedidoLoja("Aguardando pagamento", idUsuarioLoja+1))
	mock.ExpectRollback()

	st, raw := postSimular(t, server.URL, tok, "/api/loja/pagamentos/1000/simular", `{"resultado":"aprovado"}`)
	assert.Equal(t, http.StatusNotFound, st, string(raw))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM18_Simulador_PedidoJaConfirmadoDa409(t *testing.T) {
	server, _, mock := setupServidorAmbiente(t, config.AmbienteQA)
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	mock.ExpectBegin()
	mock.ExpectQuery(ecom18TravarPedido).WithArgs(int64(1000)).WillReturnRows(linhaPedidoLoja("Em separação", idUsuarioLoja))
	mock.ExpectQuery(ecom18TravarPag).WithArgs(int64(1000)).WillReturnRows(linhaPagamentoLoja("Pago"))
	mock.ExpectRollback()

	st, raw := postSimular(t, server.URL, tok, "/api/loja/pagamentos/1000/simular", `{"resultado":"recusado"}`)
	assert.Equal(t, http.StatusConflict, st, string(raw))
	assert.Contains(t, string(raw), "não está aguardando pagamento")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM18_Simulador_ResultadoInvalidoDa400SemSQL(t *testing.T) {
	server, _, mock := setupServidorAmbiente(t, config.AmbienteQA)
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	for _, corpo := range []string{`{"resultado":"talvez"}`, `{}`, `nao-json`} {
		st, raw := postSimular(t, server.URL, tok, "/api/loja/pagamentos/1000/simular", corpo)
		assert.Equal(t, http.StatusBadRequest, st, corpo+" → "+string(raw))
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM18_Ambiente_ExpoeAppEnv(t *testing.T) {
	for a, want := range map[config.Ambiente]string{"": "producao", config.AmbienteQA: "qa", config.AmbienteDev: "dev"} {
		server, _, _ := setupServidorAmbiente(t, a)
		resp, err := http.Get(server.URL + "/api/ambiente")
		require.NoError(t, err)
		raw := readBody(t, resp)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, want, como[map[string]any](t, decodeResponse(t, raw)["data"])["ambiente"])
	}
}
