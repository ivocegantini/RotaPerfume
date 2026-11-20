package vendas

import (
	"regexp"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/kernel"
)

// ECOM-14: checkout da loja. O carrinho chega só com produto e quantidade;
// preço, ativo, total e pagamento saem do servidor. O pedido nasce com canal
// Loja e status "Aguardando pagamento" (ECOM-18); o pagamento, "Em aberto".

// Limites do carrinho (os mesmos do frontend, lib/carrinho.ts).
const (
	MaxItensCheckout      = 50
	MaxQuantidadeCheckout = 99
)

// FormaCheckout é a forma escolhida na tela de pagamento.
type FormaCheckout string

// Formas aceitas no checkout.
const (
	CheckoutPix    FormaCheckout = "pix"
	CheckoutCartao FormaCheckout = "cartao"
	CheckoutBoleto FormaCheckout = "boleto"
)

// pagamento devolve a forma do ENUM e os dias até o vencimento.
func (f FormaCheckout) pagamento() (FormaPagamento, int) {
	switch f {
	case CheckoutCartao:
		return FormaCartaoCredito, 0
	case CheckoutBoleto:
		return FormaBoleto14, 14
	default:
		return FormaPIX, 0
	}
}

// ItemCheckout é uma linha do carrinho enviado.
type ItemCheckout struct {
	ProdutoID  int64
	Quantidade int
}

// DadosCheckout é o comando do POST /api/loja/checkout.
type DadosCheckout struct {
	Itens   []ItemCheckout
	Forma   string
	Chave   string // UUID do navegador (idempotência)
	Entrega DadosEntrega
}

// CheckoutValido é o comando já validado (produtos sem repetição).
type CheckoutValido struct {
	Itens   []ItemCheckout
	Forma   FormaCheckout
	Chave   string
	Entrega DadosEntrega // validado (NovaEntrega aceitou)
}

var reChave = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Validar confere chave, forma, itens (1..50, quantidade 1..99, sem repetir)
// e, por último, o endereço de entrega (ECOM-17: sem ele não há compra).
func (d DadosCheckout) Validar() (CheckoutValido, error) {
	chave := strings.ToLower(strings.TrimSpace(d.Chave))
	if !reChave.MatchString(chave) {
		return CheckoutValido{}, ErrCheckoutChaveInvalida
	}
	forma := FormaCheckout(strings.TrimSpace(d.Forma))
	switch forma {
	case CheckoutPix, CheckoutCartao, CheckoutBoleto:
	default:
		return CheckoutValido{}, ErrCheckoutFormaInvalida
	}
	switch {
	case len(d.Itens) == 0:
		return CheckoutValido{}, ErrCheckoutCarrinhoVazio
	case len(d.Itens) > MaxItensCheckout:
		return CheckoutValido{}, ErrCheckoutItensDemais
	}
	vistos := make(map[int64]bool, len(d.Itens))
	for _, it := range d.Itens {
		switch {
		case it.ProdutoID <= 0:
			return CheckoutValido{}, ErrProdutoIDObrigatorio
		case it.Quantidade < 1 || it.Quantidade > MaxQuantidadeCheckout:
			return CheckoutValido{}, ErrCheckoutQuantidadeInvalida
		case vistos[it.ProdutoID]:
			return CheckoutValido{}, ErrCheckoutProdutoRepetido
		}
		vistos[it.ProdutoID] = true
	}
	if _, err := NovaEntrega(d.Entrega); err != nil {
		return CheckoutValido{}, err
	}
	return CheckoutValido{Itens: d.Itens, Forma: forma, Chave: chave, Entrega: d.Entrega}, nil
}

// ProdutoIDs devolve os produtos do carrinho, na ordem.
func (c CheckoutValido) ProdutoIDs() []int64 {
	ids := make([]int64, len(c.Itens))
	for i, it := range c.Itens {
		ids[i] = it.ProdutoID
	}
	return ids
}

// CompraLoja é o resultado do checkout: o pedido (com itens) e o pagamento.
// Repetida: a chave já tinha virado pedido (o reenvio devolve o mesmo).
type CompraLoja struct {
	Pedido    *Pedido
	Pagamento *Pagamento
	Repetida  bool
}

// OfertaLoja é o que o servidor sabe de um produto ativo no checkout
// (SEC-79): o preço de tabela e as unidades disponíveis (saldo do último
// snapshot menos o já reservado em pedidos da Loja Aguardando pagamento ou
// Em separação).
type OfertaLoja struct {
	Preco      kernel.Dinheiro
	Disponivel int
}

// MontarCompraLoja monta pedido e pagamento com os preços do servidor
// (ofertas: só produtos ativos). Produto fora do mapa → indisponível (409);
// quantidade acima do disponível → sem estoque (409, SEC-79).
// hoje é o dia do pedido (meia-noite local); o pedido fica Aguardando
// pagamento e o pagamento, Em aberto, até a confirmação (ECOM-18).
func MontarCompraLoja(c CheckoutValido, clienteID, vendedorID int64, ofertas map[int64]OfertaLoja, hoje time.Time) (*Pedido, *Pagamento, error) {
	itens := make([]DadosItem, 0, len(c.Itens))
	for _, it := range c.Itens {
		oferta, ok := ofertas[it.ProdutoID]
		if !ok {
			return nil, nil, ErrCheckoutProdutoIndisponivel
		}
		if it.Quantidade > oferta.Disponivel {
			return nil, nil, ErrCheckoutSemEstoque
		}
		itens = append(itens, DadosItem{ProdutoID: it.ProdutoID, Quantidade: it.Quantidade, PrecoPraticado: kernel.Decimal(oferta.Preco.String())})
	}
	dia := hoje.In(time.Local).Format(kernel.LayoutData)
	ped, err := NovoPedido(DadosPedido{ClienteID: clienteID, VendedorID: vendedorID, DataPedido: dia,
		Canal: string(CanalLoja), Status: string(StatusAguardandoPagamento), Itens: itens, Entrega: c.Entrega})
	if err != nil {
		return nil, nil, err
	}
	forma, dias := c.Forma.pagamento()
	total := kernel.Decimal(ped.ValorTotal.String())
	pag, err := DadosPagamento{FormaPagamento: string(forma), Parcelas: 1, Valor: total, ValorLiquido: total,
		DataVencimento:  hoje.In(time.Local).AddDate(0, 0, dias).Format(kernel.LayoutData),
		StatusPagamento: string(StatusEmAberto)}.Pagamento(0)
	if err != nil {
		return nil, nil, err
	}
	return ped, pag, nil
}
