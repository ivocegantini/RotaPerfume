package vendashttp

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
)

// PrefixoURLImagem é o mesmo de catalogohttp.PrefixoURLImagem (teste ECOM-12
// confere); repetido para o vendashttp não importar outro <ctx>http.
const PrefixoURLImagem = "/api/public/produtos/imagens/"

const formatoData = "2006-01-02"

// minhaCompraJSON é a linha de "Minhas compras" (ECOM-16): o pedido do próprio
// cliente com os nomes dos produtos; sem vendedor nem custo.
type minhaCompraJSON struct {
	PedidoID   int64           `json:"pedido_id"`
	Data       string          `json:"data"` // AAAA-MM-DD
	Status     string          `json:"status"`
	ValorTotal kernel.Dinheiro `json:"valor_total"`
	Produtos   []string        `json:"produtos"`
}

func novoMinhaCompraJSON(c vendas.Compra) minhaCompraJSON {
	prods := c.Produtos
	if prods == nil {
		prods = []string{}
	}
	return minhaCompraJSON{PedidoID: c.PedidoID, Data: c.Data.Format(formatoData), Status: c.Status, ValorTotal: c.ValorTotal, Produtos: prods}
}

type itemCompradoJSON struct {
	ProdutoID      int64           `json:"produto_id"`
	SKU            string          `json:"sku"`
	Nome           string          `json:"nome"`
	ImagemURL      *string         `json:"imagem_url"`
	Ativo          bool            `json:"ativo"`
	Quantidade     int             `json:"quantidade"`
	PrecoPraticado kernel.Dinheiro `json:"preco_praticado"`
	DescontoPct    float64         `json:"desconto_pct"`
	ValorBruto     kernel.Dinheiro `json:"valor_bruto"`
}

// pagamentoCompradoJSON: sem taxa nem valor líquido (dados internos).
type pagamentoCompradoJSON struct {
	Forma          string          `json:"forma_pagamento"`
	Parcelas       uint8           `json:"parcelas"`
	Valor          kernel.Dinheiro `json:"valor"`
	DataVencimento string          `json:"data_vencimento"`
	DataPagamento  *string         `json:"data_pagamento"`
	Status         string          `json:"status_pagamento"`
}

type compraDetalheJSON struct {
	PedidoID   int64                   `json:"pedido_id"`
	Data       string                  `json:"data"`
	Status     string                  `json:"status"`
	ValorTotal kernel.Dinheiro         `json:"valor_total"`
	Itens      []itemCompradoJSON      `json:"itens"`
	Pagamentos []pagamentoCompradoJSON `json:"pagamentos"`
}

func novoItemCompradoJSON(it vendas.ItemComprado) itemCompradoJSON {
	var img *string
	if catalogo.NomeImagemValido(it.Imagem) {
		u := PrefixoURLImagem + it.Imagem
		img = &u
	}
	return itemCompradoJSON{
		ProdutoID: it.ProdutoID, SKU: it.SKU, Nome: it.Nome, ImagemURL: img, Ativo: it.Ativo, Quantidade: it.Quantidade,
		PrecoPraticado: it.PrecoPraticado, DescontoPct: it.DescontoPct, ValorBruto: it.ValorBruto,
	}
}

func novoPagamentoCompradoJSON(p vendas.PagamentoComprado) pagamentoCompradoJSON {
	var pago *string
	if p.DataPagamento != nil {
		s := p.DataPagamento.Format(formatoData)
		pago = &s
	}
	return pagamentoCompradoJSON{Forma: p.Forma, Parcelas: p.Parcelas, Valor: p.Valor,
		DataVencimento: p.DataVencimento.Format(formatoData), DataPagamento: pago, Status: p.Status}
}

func novoCompraDetalheJSON(d vendas.CompraDetalhe) compraDetalheJSON {
	return compraDetalheJSON{
		PedidoID: d.PedidoID, Data: d.Data.Format(formatoData), Status: d.Status, ValorTotal: d.ValorTotal,
		Itens:      httpx.Lista(d.Itens, novoItemCompradoJSON),
		Pagamentos: httpx.Lista(d.Pagamentos, novoPagamentoCompradoJSON),
	}
}

// MinhasCompras GET /api/loja/minhas-compras?page&limit&order_by&order_dir
// (AnyRole, ECOM-12/16): os pedidos do usuário do token.
func (h *PedidoHandler) MinhasCompras(w http.ResponseWriter, r *http.Request) {
	const tag = "[loja] MinhasCompras"
	res, err := h.casos.MinhasCompras(r.Context(), principalDoEscopo(r),
		httpx.OrdenacaoDaQuery(r, vendas.CamposOrdenacaoCompra(), vendas.OrdenacaoCompraPadrao()), httpx.PaginaDaQuery(r))
	httpx.ResponderPaginado(w, tag, res, err, novoMinhaCompraJSON)
}

// MinhaCompra GET /api/loja/minhas-compras/{id} (AnyRole, ECOM-16): o
// pedido do usuário do token com itens e pagamentos; de outro login → 404.
func (h *PedidoHandler) MinhaCompra(w http.ResponseWriter, r *http.Request) {
	const tag = "[loja] MinhaCompra"
	if id, ok := httpx.IDOuErro(w, r, tag, "id"); ok {
		d, err := h.casos.MinhaCompra(r.Context(), principalDoEscopo(r), id)
		httpx.Responder(w, tag, http.StatusOK, d, err, novoCompraDetalheJSON)
	}
}
