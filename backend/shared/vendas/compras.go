package vendas

import (
	"time"

	"github.com/rotaperfumes/shared/kernel"
)

// ProdutoComprado é um produto da página "Minhas compras" (ECOM-12): os
// itens dos pedidos não cancelados do cliente da loja, somados por produto.
type ProdutoComprado struct {
	ProdutoID    int64
	SKU          string
	Nome         string
	Imagem       string // nome do arquivo ("" = sem imagem)
	Ativo        bool   // inativo: a página do produto dá 404
	Quantidade   int
	Pedidos      int
	TotalGasto   kernel.Dinheiro
	UltimaCompra time.Time
}

// CampoCompra é o enum de ordenação de "Minhas compras" (?order_by=); a
// coluna sai de um switch no adapter (whitelist).
type CampoCompra string

// Campos de ordenação de "Minhas compras".
const (
	OrdemCompraUltima     CampoCompra = "ultima_compra"
	OrdemCompraNome       CampoCompra = "nome"
	OrdemCompraQuantidade CampoCompra = "quantidade"
	OrdemCompraPedidos    CampoCompra = "pedidos"
	OrdemCompraTotal      CampoCompra = "total_gasto"
)

// CamposOrdenacaoCompra são os valores aceitos em ?order_by=.
func CamposOrdenacaoCompra() []CampoCompra {
	return []CampoCompra{OrdemCompraUltima, OrdemCompraNome, OrdemCompraQuantidade, OrdemCompraPedidos, OrdemCompraTotal}
}

// OrdenacaoCompraPadrao: da compra mais recente para a mais antiga.
func OrdenacaoCompraPadrao() kernel.Ordenacao[CampoCompra] {
	return kernel.NovaOrdenacao(OrdemCompraUltima, kernel.Desc)
}
