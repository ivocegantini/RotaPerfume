package vendas

import (
	"strings"
	"time"
)

// ECOM-18: confirmação do pagamento da loja. O pedido nasce "Aguardando
// pagamento"; só com o pagamento aprovado ele vai para a separação.

// ResultadoPagamento é a resposta do gateway (ou do simulador de dev/qa).
type ResultadoPagamento string

// Resultados aceitos.
const (
	PagamentoAprovado ResultadoPagamento = "aprovado"
	PagamentoRecusado ResultadoPagamento = "recusado"
)

// NovoResultadoPagamento valida o resultado (minúsculas, sem espaços nas pontas).
func NovoResultadoPagamento(s string) (ResultadoPagamento, error) {
	switch v := ResultadoPagamento(strings.ToLower(strings.TrimSpace(s))); v {
	case PagamentoAprovado, PagamentoRecusado:
		return v, nil
	}
	return "", ErrResultadoPagamentoInvalido
}

// PedidoLoja é o que a confirmação lê do pedido travado: canal, status e
// o login dono do cliente (0 se o cliente não tem login).
type PedidoLoja struct {
	PedidoID  int64
	Canal     string
	Status    string
	UsuarioID int64
}

// AplicarPagamentoLoja decide a confirmação: só pedido da Loja "Aguardando
// pagamento" com pagamento "Em aberto". Aprovado → pagamento Pago (data de
// hoje) e pedido Em separação; recusado → Recusado e Cancelado (libera a
// reserva de estoque). Altera pag e devolve o novo status do pedido.
func AplicarPagamentoLoja(p PedidoLoja, pag *Pagamento, r ResultadoPagamento, hoje time.Time) (StatusPedido, error) {
	if p.Canal != string(CanalLoja) || p.Status != string(StatusAguardandoPagamento) ||
		pag == nil || pag.StatusPagamento != string(StatusEmAberto) {
		return "", ErrPagamentoNaoPendente
	}
	if r == PagamentoAprovado {
		h := hoje.In(time.Local)
		dia := time.Date(h.Year(), h.Month(), h.Day(), 0, 0, 0, 0, time.Local)
		pag.StatusPagamento, pag.DataPagamento = string(StatusPago), &dia
		return StatusEmSeparacao, nil
	}
	pag.StatusPagamento, pag.DataPagamento = string(StatusRecusado), nil
	return StatusCancelado, nil
}

// MaxExpiracoesPorRodada limita os pedidos cancelados por rodada do job (ECOM-19).
const MaxExpiracoesPorRodada = 500

// Vencido (ECOM-19): o pagamento venceu antes de hoje (o dia do vencimento
// ainda vale; Pix/cartão vencem no dia da compra, boleto em 14 dias).
func (p *Pagamento) Vencido(hoje time.Time) bool {
	h := hoje.In(time.Local)
	dia := time.Date(h.Year(), h.Month(), h.Day(), 0, 0, 0, 0, time.Local)
	v := p.DataVencimento.In(time.Local)
	return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.Local).Before(dia)
}

// ExpirarPagamentoLoja (ECOM-19): pedido pendente com pagamento vencido vira
// Cancelado + Recusado (libera a reserva). Não vencido → ErrPagamentoNaoVencido.
func ExpirarPagamentoLoja(p PedidoLoja, pag *Pagamento, hoje time.Time) (StatusPedido, error) {
	if pag != nil && !pag.Vencido(hoje) {
		return "", ErrPagamentoNaoVencido
	}
	return AplicarPagamentoLoja(p, pag, PagamentoRecusado, hoje)
}
