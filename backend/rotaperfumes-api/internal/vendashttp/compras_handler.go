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

// produtoCompradoJSON é o item de "Minhas compras" (ECOM-12): só dados do
// próprio cliente; sem custo, sem pedido nem vendedor.
type produtoCompradoJSON struct {
	ProdutoID    int64           `json:"produto_id"`
	SKU          string          `json:"sku"`
	Nome         string          `json:"nome"`
	ImagemURL    *string         `json:"imagem_url"`
	Ativo        bool            `json:"ativo"`
	Quantidade   int             `json:"quantidade"`
	Pedidos      int             `json:"pedidos"`
	TotalGasto   kernel.Dinheiro `json:"total_gasto"`
	UltimaCompra string          `json:"ultima_compra"` // AAAA-MM-DD
}

func novoProdutoCompradoJSON(c vendas.ProdutoComprado) produtoCompradoJSON {
	var img *string
	if catalogo.NomeImagemValido(c.Imagem) {
		u := PrefixoURLImagem + c.Imagem
		img = &u
	}
	return produtoCompradoJSON{
		ProdutoID: c.ProdutoID, SKU: c.SKU, Nome: c.Nome, ImagemURL: img, Ativo: c.Ativo,
		Quantidade: c.Quantidade, Pedidos: c.Pedidos, TotalGasto: c.TotalGasto,
		UltimaCompra: c.UltimaCompra.Format("2006-01-02"),
	}
}

// MinhasCompras GET /api/loja/minhas-compras?page&limit&order_by&order_dir
// (AnyRole, ECOM-12): os produtos comprados pelo usuário do token.
func (h *PedidoHandler) MinhasCompras(w http.ResponseWriter, r *http.Request) {
	const tag = "[loja] MinhasCompras"
	res, err := h.casos.MinhasCompras(r.Context(), principalDoEscopo(r),
		httpx.OrdenacaoDaQuery(r, vendas.CamposOrdenacaoCompra(), vendas.OrdenacaoCompraPadrao()), httpx.PaginaDaQuery(r))
	httpx.ResponderPaginado(w, tag, res, err, novoProdutoCompradoJSON)
}
