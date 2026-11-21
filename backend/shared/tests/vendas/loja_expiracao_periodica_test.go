package vendas_test

// ECOM-19: caso de uso ExpirarPedidosLoja com os adapters reais sobre sqlmock.

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas/vendasapp"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

const (
	ecom19Vencidos  = `SELECT p.pedido_id_origem FROM pedidos p JOIN pagamentos g .* g.data_vencimento < \? ORDER BY p.pedido_id_origem LIMIT \?`
	ecom19TravarPed = `SELECT p.pedido_id_origem, p.canal, p.status, COALESCE\(c.usuario_id, 0\)`
	ecom19TravarPag = `FROM pagamentos WHERE pedido_id = \? ORDER BY pagamento_id LIMIT 1 FOR UPDATE`
)

func casosLoja(t *testing.T, hoje time.Time) (*vendasapp.Casos, sqlmock.Sqlmock) {
	t.Helper()
	pool, mock := novoMock(t)
	tx := db.NewTransactor(pool, func(q db.Querier) vendasapp.TxPedido {
		return vendasapp.TxPedido{Pedidos: vendasmysql.NovoPedidos(q), Pagamentos: vendasmysql.NovoPagamentos(q),
			Estoque: catalogomysql.NovoEstoques(q), Loja: vendasmysql.NovaLoja(q)}
	})
	c := vendasapp.Novo(vendasapp.Deps{Pedidos: vendasmysql.NovoPedidos(pool), Pagamentos: vendasmysql.NovoPagamentos(pool),
		Escopo: escopoFixo{e: kernel.Escopo{}}, TxPedido: tx, Relogio: func() time.Time { return hoje }})
	return c, mock
}

func linhaPag(venc time.Time, status string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{"pagamento_id", "pedido_id", "forma_pagamento", "parcelas", "valor", "taxa_pct", "valor_liquido",
		"data_vencimento", "data_pagamento", "status_pagamento", "created_at", "updated_at"}).
		AddRow(int64(5), int64(1), "PIX", uint8(1), "10.00", 0.0, "10.00", venc, nil, status, now, now)
}

func TestECOM19_ExpirarPedidosLoja_CancelaVencidoEIgnoraPagoNoMeioTempo(t *testing.T) {
	hoje := time.Date(2026, 11, 20, 3, 0, 0, 0, time.Local)
	c, mock := casosLoja(t, hoje)
	ontem := hoje.AddDate(0, 0, -1)
	mock.ExpectQuery(ecom19Vencidos).WithArgs("2026-11-20", 500).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)).AddRow(int64(2)))
	// 1: vencido e pendente → Cancelado + Recusado.
	mock.ExpectBegin()
	mock.ExpectQuery(ecom19TravarPed).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"a", "b", "c", "d"}).AddRow(int64(1), "Loja", "Aguardando pagamento", int64(9)))
	mock.ExpectQuery(ecom19TravarPag).WithArgs(int64(1)).WillReturnRows(linhaPag(ontem, "Em aberto"))
	mock.ExpectExec(`UPDATE pedidos SET status = \?`).WithArgs("Cancelado", int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE pagamentos SET status_pagamento = \?`).WithArgs("Recusado", nil, int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	// 2: aprovado entre a leitura e o lock → nada muda (rollback).
	mock.ExpectBegin()
	mock.ExpectQuery(ecom19TravarPed).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"a", "b", "c", "d"}).AddRow(int64(2), "Loja", "Em separação", int64(9)))
	mock.ExpectQuery(ecom19TravarPag).WithArgs(int64(2)).WillReturnRows(linhaPag(ontem, "Pago"))
	mock.ExpectRollback()

	n, err := c.ExpirarPedidosLoja(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM19_IniciarExpiracaoPeriodica_DesligadaFechaNaHora(t *testing.T) {
	c, mock := casosLoja(t, time.Now())
	select {
	case <-vendasapp.IniciarExpiracaoPeriodica(context.Background(), c, 0):
	case <-time.After(time.Second):
		t.Fatal("intervalo 0 deveria fechar o canal sem rodar")
	}
	assert.NoError(t, mock.ExpectationsWereMet(), "nenhum SQL com o job desligado")
}

func TestECOM19_IniciarExpiracaoPeriodica_RodaNaSubidaEParaNoCancel(t *testing.T) {
	c, mock := casosLoja(t, time.Now())
	mock.ExpectQuery(ecom19Vencidos).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	ctx, cancel := context.WithCancel(context.Background())
	done := vendasapp.IniciarExpiracaoPeriodica(ctx, c, time.Hour)
	require.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, time.Second, 10*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("o job não parou no cancel")
	}
}
