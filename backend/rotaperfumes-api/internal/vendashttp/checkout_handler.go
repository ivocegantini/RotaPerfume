package vendashttp

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
)

// checkoutReq é o corpo do POST /api/loja/checkout (ECOM-14): só produto e
// quantidade (preço e total nunca vêm do cliente).
type checkoutReq struct {
	Itens []struct {
		ProdutoID  int64 `json:"produto_id"`
		Quantidade int   `json:"quantidade"`
	} `json:"itens"`
	FormaPagamento string      `json:"forma_pagamento"`
	Chave          string      `json:"chave"`
	Entrega        entregaJSON `json:"entrega"` // ECOM-17: obrigatório
}

func dadosCheckout(r checkoutReq) vendas.DadosCheckout {
	d := vendas.DadosCheckout{Forma: r.FormaPagamento, Chave: r.Chave, Entrega: dadosEntrega(r.Entrega),
		Itens: make([]vendas.ItemCheckout, len(r.Itens))}
	for i, it := range r.Itens {
		d.Itens[i] = vendas.ItemCheckout{ProdutoID: it.ProdutoID, Quantidade: it.Quantidade}
	}
	return d
}

type itemCompraJSON struct {
	ProdutoID  int64           `json:"produto_id"`
	Nome       string          `json:"nome"`
	Quantidade int             `json:"quantidade"`
	Preco      kernel.Dinheiro `json:"preco"`
	Valor      kernel.Dinheiro `json:"valor"`
}

// compraJSON é a resposta: só dados do próprio pedido (sem vendedor/custo).
type compraJSON struct {
	PedidoID        int64            `json:"pedido_id"`
	DataPedido      string           `json:"data_pedido"`
	Status          string           `json:"status"`
	ValorTotal      kernel.Dinheiro  `json:"valor_total"`
	FormaPagamento  string           `json:"forma_pagamento"`
	StatusPagamento string           `json:"status_pagamento"`
	DataVencimento  string           `json:"data_vencimento"`
	Repetida        bool             `json:"repetida"`
	Entrega         *entregaJSON     `json:"entrega"`
	Itens           []itemCompraJSON `json:"itens"`
}

func novoCompraJSON(c vendas.CompraLoja) compraJSON {
	p := c.Pedido
	j := compraJSON{PedidoID: p.PedidoIDOrigem, DataPedido: p.DataPedido.Format("2006-01-02"), Status: p.Status,
		ValorTotal: p.ValorTotal, Repetida: c.Repetida, Itens: make([]itemCompraJSON, 0, len(p.Itens))}
	if p.Entrega.CEP != "" {
		e := novoEntregaJSON(p.Entrega)
		j.Entrega = &e
	}
	if c.Pagamento != nil {
		j.FormaPagamento, j.StatusPagamento = c.Pagamento.FormaPagamento, c.Pagamento.StatusPagamento
		j.DataVencimento = c.Pagamento.DataVencimento.Format("2006-01-02")
	}
	for _, it := range p.Itens {
		j.Itens = append(j.Itens, itemCompraJSON{ProdutoID: it.ProdutoID, Nome: it.ProdutoNome,
			Quantidade: it.Quantidade, Preco: it.PrecoPraticado, Valor: it.ValorBruto})
	}
	return j
}

// limitador é o rate limit do checkout (adapters/ratelimit satisfaz).
type limitador interface {
	Bloqueado(chave string) (bool, time.Duration)
	RegistrarFalha(chave string)
}

// ComLimiteCheckout liga os limites do checkout (SEC-79): por usuário do
// token e por IP (confiarProxy = TRUST_PROXY_HEADERS). Toda tentativa conta.
func (h *PedidoHandler) ComLimiteCheckout(porUsuario, porIP limitador, confiarProxy bool) *PedidoHandler {
	h.limiteUsuario, h.limiteIP, h.confiarProxy = porUsuario, porIP, confiarProxy
	return h
}

// checkoutPermitido conta a tentativa e responde 429 acima de um dos limites.
func (h *PedidoHandler) checkoutPermitido(w http.ResponseWriter, r *http.Request, tag string) bool {
	chaves := []struct {
		l     limitador
		chave string
	}{
		{h.limiteUsuario, "checkout:u:" + strconv.FormatInt(principalDoEscopo(r).UserID(), 10)},
		{h.limiteIP, "checkout:ip:" + httpx.IPDoCliente(r, h.confiarProxy)},
	}
	for _, c := range chaves {
		if c.l == nil {
			continue
		}
		if bloqueado, retry := c.l.Bloqueado(c.chave); bloqueado {
			log.Printf("%s: limite de checkout atingido retry_after=%s", tag, retry)
			httpx.ResponderErro(w, tag, &kernel.Bloqueio{Sentinela: vendas.ErrMuitosCheckouts, RetryAfter: retry})
			return false
		}
	}
	for _, c := range chaves {
		if c.l != nil {
			c.l.RegistrarFalha(c.chave)
		}
	}
	return true
}

// FinalizarCompra POST /api/loja/checkout (AnyRole, ECOM-14) → 201 com o
// pedido criado; 200 quando a mesma chave já tinha virado pedido.
func (h *PedidoHandler) FinalizarCompra(w http.ResponseWriter, r *http.Request) {
	const tag = "[loja] FinalizarCompra"
	if !h.checkoutPermitido(w, r, tag) {
		return
	}
	c, err := h.casos.FinalizarCompra(r.Context(), principalDoEscopo(r), httpx.LeitorJSON(r, dadosCheckout))
	status := http.StatusCreated
	if err == nil && c.Repetida {
		status = http.StatusOK
	}
	httpx.Responder(w, tag, status, c, err, novoCompraJSON)
}
