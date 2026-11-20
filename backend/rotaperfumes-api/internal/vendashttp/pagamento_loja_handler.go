package vendashttp

import (
	"log"
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/vendas"
)

// simularPagamentoReq é o corpo do simulador (ECOM-18): "aprovado" ou "recusado".
type simularPagamentoReq struct {
	Resultado string `json:"resultado"`
}

// ComSimuladorPagamento liga o simulador de pagamento (só APP_ENV dev/qa).
func (h *PedidoHandler) ComSimuladorPagamento(ligado bool) *PedidoHandler {
	h.simulador = ligado
	return h
}

// SimularPagamento POST /api/loja/pagamentos/{id}/simular (AnyRole, ECOM-18):
// aprova ou recusa o pagamento de um pedido da loja "Aguardando pagamento".
// Fora de dev/qa → 404; só o admin ou o dono do pedido.
func (h *PedidoHandler) SimularPagamento(w http.ResponseWriter, r *http.Request) {
	const tag = "[loja] SimularPagamento"
	id, ok := httpx.IDOuErro(w, r, tag, "id") // CHORE-03: id inválido → 400 antes de tudo
	if !ok {
		return
	}
	if !h.simulador {
		log.Printf("%s: simulador desligado (APP_ENV fora de dev/qa)", tag)
		httpx.ResponderErro(w, tag, vendas.ErrSimuladorDesligado)
		return
	}
	c, err := h.casos.ConfirmarPagamentoLoja(r.Context(), principalDoEscopo(r), id,
		httpx.LeitorJSON(r, func(q simularPagamentoReq) string { return q.Resultado }))
	httpx.Responder(w, tag, http.StatusOK, c, err, novoCompraJSON)
}
