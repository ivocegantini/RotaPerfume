package vendas_test

// ECOM-18: regra pura da confirmação do pagamento da loja.

import (
	"testing"
	"time"

	"github.com/rotaperfumes/shared/vendas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pendenteLoja() (vendas.PedidoLoja, *vendas.Pagamento) {
	return vendas.PedidoLoja{PedidoID: 1, Canal: "Loja", Status: "Aguardando pagamento", UsuarioID: 9},
		&vendas.Pagamento{PagamentoID: 5, PedidoID: 1, StatusPagamento: "Em aberto"}
}

func TestECOM18_NovoResultadoPagamento(t *testing.T) {
	for _, s := range []string{"aprovado", " APROVADO ", "recusado"} {
		_, err := vendas.NovoResultadoPagamento(s)
		assert.NoError(t, err, s)
	}
	for _, s := range []string{"", "pago", "ok"} {
		_, err := vendas.NovoResultadoPagamento(s)
		assert.ErrorIs(t, err, vendas.ErrResultadoPagamentoInvalido, s)
	}
}

func TestECOM18_Aprovado_PagoEEmSeparacao(t *testing.T) {
	ped, pag := pendenteLoja()
	hoje := time.Date(2026, 11, 20, 15, 30, 0, 0, time.Local)
	st, err := vendas.AplicarPagamentoLoja(ped, pag, vendas.PagamentoAprovado, hoje)
	require.NoError(t, err)
	assert.Equal(t, vendas.StatusEmSeparacao, st)
	assert.Equal(t, "Pago", pag.StatusPagamento)
	require.NotNil(t, pag.DataPagamento)
	assert.Equal(t, time.Date(2026, 11, 20, 0, 0, 0, 0, time.Local), *pag.DataPagamento, "meia-noite local do dia")
}

func TestECOM18_Recusado_RecusadoECancelado(t *testing.T) {
	ped, pag := pendenteLoja()
	st, err := vendas.AplicarPagamentoLoja(ped, pag, vendas.PagamentoRecusado, time.Now())
	require.NoError(t, err)
	assert.Equal(t, vendas.StatusCancelado, st)
	assert.Equal(t, "Recusado", pag.StatusPagamento)
	assert.Nil(t, pag.DataPagamento)
}

func TestECOM18_SoPendenteDaLoja(t *testing.T) {
	casos := map[string]func(*vendas.PedidoLoja, **vendas.Pagamento){
		"canal ERP":         func(p *vendas.PedidoLoja, _ **vendas.Pagamento) { p.Canal = "App" },
		"já em separação":   func(p *vendas.PedidoLoja, _ **vendas.Pagamento) { p.Status = "Em separação" },
		"cancelado":         func(p *vendas.PedidoLoja, _ **vendas.Pagamento) { p.Status = "Cancelado" },
		"pagamento pago":    func(_ *vendas.PedidoLoja, g **vendas.Pagamento) { (*g).StatusPagamento = "Pago" },
		"pagamento ausente": func(_ *vendas.PedidoLoja, g **vendas.Pagamento) { *g = nil },
	}
	for nome, mudar := range casos {
		ped, pag := pendenteLoja()
		mudar(&ped, &pag)
		_, err := vendas.AplicarPagamentoLoja(ped, pag, vendas.PagamentoAprovado, time.Now())
		assert.ErrorIs(t, err, vendas.ErrPagamentoNaoPendente, nome)
	}
}

func TestECOM18_StatusNovosAceitos(t *testing.T) {
	_, err := vendas.NovoStatusPedido("Aguardando pagamento")
	assert.NoError(t, err)
	_, err = vendas.NovoStatusPagamento("Recusado")
	assert.NoError(t, err)
}

// ---- ECOM-19: expiração dos pedidos da loja sem pagamento ----

func TestECOM19_Vencido(t *testing.T) {
	hoje := time.Date(2026, 11, 20, 10, 0, 0, 0, time.Local)
	venc := func(d int) *vendas.Pagamento {
		return &vendas.Pagamento{DataVencimento: time.Date(2026, 11, d, 0, 0, 0, 0, time.Local)}
	}
	assert.False(t, venc(20).Vencido(hoje), "o dia do vencimento ainda vale")
	assert.False(t, venc(21).Vencido(hoje))
	assert.True(t, venc(19).Vencido(hoje))
}

func TestECOM19_ExpirarPagamentoLoja(t *testing.T) {
	hoje := time.Date(2026, 11, 20, 0, 0, 0, 0, time.Local)
	ped, pag := pendenteLoja()
	pag.DataVencimento = hoje.AddDate(0, 0, -1)
	st, err := vendas.ExpirarPagamentoLoja(ped, pag, hoje)
	require.NoError(t, err)
	assert.Equal(t, vendas.StatusCancelado, st)
	assert.Equal(t, "Recusado", pag.StatusPagamento)

	ped, pag = pendenteLoja()
	pag.DataVencimento = hoje
	_, err = vendas.ExpirarPagamentoLoja(ped, pag, hoje)
	assert.ErrorIs(t, err, vendas.ErrPagamentoNaoVencido)

	ped, pag = pendenteLoja()
	pag.DataVencimento, pag.StatusPagamento = hoje.AddDate(0, 0, -3), "Pago" // pago antes do lock
	_, err = vendas.ExpirarPagamentoLoja(ped, pag, hoje)
	assert.ErrorIs(t, err, vendas.ErrPagamentoNaoPendente)
}
