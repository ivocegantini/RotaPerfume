package vendas

import (
	"time"

	"github.com/rotaperfumes/shared/kernel"
)

// Compra é uma linha de "Minhas compras" (ECOM-12/16): um pedido do cliente
// da loja ligado ao login, com os nomes dos produtos (sem repetir).
type Compra struct {
	PedidoID   int64
	Data       time.Time
	Status     string
	ValorTotal kernel.Dinheiro
	Produtos   []string
}

// ItemComprado é um item do detalhe da compra (ECOM-16): sem custo.
type ItemComprado struct {
	ProdutoID      int64
	SKU            string
	Nome           string
	Imagem         string // nome do arquivo ("" = sem imagem)
	Ativo          bool   // inativo: a página do produto dá 404
	Quantidade     int
	PrecoPraticado kernel.Dinheiro
	DescontoPct    float64
	ValorBruto     kernel.Dinheiro
}

// PagamentoComprado é o pagamento visto pelo cliente (ECOM-16): sem taxa
// nem valor líquido (dados internos da loja).
type PagamentoComprado struct {
	Forma          string
	Parcelas       uint8
	Valor          kernel.Dinheiro
	DataVencimento time.Time
	DataPagamento  *time.Time
	Status         string
}

// CompraDetalhe é o pedido aberto em "Minhas compras" (ECOM-16).
type CompraDetalhe struct {
	PedidoID   int64
	Data       time.Time
	Status     string
	ValorTotal kernel.Dinheiro
	Itens      []ItemComprado
	Pagamentos []PagamentoComprado
}

// CampoCompra é o enum de ordenação de "Minhas compras" (?order_by=); a
// coluna sai de um switch no adapter (whitelist).
type CampoCompra string

// Campos de ordenação de "Minhas compras".
const (
	OrdemCompraData   CampoCompra = "data"
	OrdemCompraPedido CampoCompra = "pedido_id"
	OrdemCompraTotal  CampoCompra = "valor_total"
)

// CamposOrdenacaoCompra são os valores aceitos em ?order_by=.
func CamposOrdenacaoCompra() []CampoCompra {
	return []CampoCompra{OrdemCompraData, OrdemCompraPedido, OrdemCompraTotal}
}

// OrdenacaoCompraPadrao: da compra mais recente para a mais antiga.
func OrdenacaoCompraPadrao() kernel.Ordenacao[CampoCompra] {
	return kernel.NovaOrdenacao(OrdemCompraData, kernel.Desc)
}
