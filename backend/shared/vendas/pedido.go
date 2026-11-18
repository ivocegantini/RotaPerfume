package vendas

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/kernel"
)

// StatusPedido é o ENUM status de sql/04_ddl_pedidos.sql (VO de validação
// do comando). A entidade guarda o texto (string, como o model legado e o
// precedente do ARQ-04): um valor lido do banco não é validado (o legado
// também não validava).
type StatusPedido string

// Status de pedido aceitos no payload.
const (
	StatusCancelado   StatusPedido = "Cancelado"
	StatusEmSeparacao StatusPedido = "Em separação"
	StatusEntregue    StatusPedido = "Entregue"
	StatusFaturado    StatusPedido = "Faturado"
)

// NovoStatusPedido valida o status (espaços nas pontas ignorados).
func NovoStatusPedido(s string) (StatusPedido, error) {
	switch v := StatusPedido(strings.TrimSpace(s)); v {
	case StatusCancelado, StatusEmSeparacao, StatusEntregue, StatusFaturado:
		return v, nil
	}
	return "", ErrStatusInvalido
}

// Canal é o ENUM canal de sql/04_ddl_pedidos.sql.
type Canal string

// Canais aceitos no payload.
const (
	CanalApp      Canal = "App"
	CanalTelefone Canal = "Telefone"
	CanalVisita   Canal = "Visita"
	CanalWhatsApp Canal = "WhatsApp"
	CanalLoja     Canal = "Loja" // checkout da loja online (ECOM-06, sql/33)
)

// NovoCanal valida o canal (espaços nas pontas ignorados).
func NovoCanal(s string) (Canal, error) {
	switch v := Canal(strings.TrimSpace(s)); v {
	case CanalApp, CanalTelefone, CanalVisita, CanalWhatsApp, CanalLoja:
		return v, nil
	}
	return "", ErrCanalInvalido
}

// DescontoPct é o desconto percentual de um item (0..100).
type DescontoPct struct{ v float64 }

// NovoDescontoPct valida a faixa 0..100 (NaN e ±Inf também são recusados).
func NovoDescontoPct(v float64) (DescontoPct, error) {
	if math.IsNaN(v) || v < 0 || v > 100 {
		return DescontoPct{}, ErrDescontoPctInvalido
	}
	return DescontoPct{v: v}, nil
}

// Float64 devolve o percentual.
func (d DescontoPct) Float64() float64 { return d.v }

// fator devolve quantidade × (1 - desconto/100) exato (ARQ-09): o percentual
// vira o decimal mais curto do float64, que é o que o cliente enviou.
func (d DescontoPct) fator(quantidade int) *big.Rat {
	pct, _ := new(big.Rat).SetString(strconv.FormatFloat(d.v, 'f', -1, 64))
	restante := new(big.Rat).Sub(big.NewRat(1, 1), pct.Quo(pct, big.NewRat(100, 1)))
	return restante.Mul(restante, new(big.Rat).SetInt64(int64(quantidade)))
}

// Pedido é o aggregate root de Vendas: o cabeçalho (com os nomes de cliente
// e vendedor da leitura, via JOIN) e os itens. PedidoIDOrigem é a PK
// AUTO_INCREMENT (sem coluna id separada). Itens fica nil na listagem (só o
// detalhe carrega os itens).
type Pedido struct {
	PedidoIDOrigem int64
	ClienteID      int64
	VendedorID     int64
	DataPedido     time.Time
	Canal          string
	Status         string
	ValorTotal     kernel.Dinheiro
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ClienteNome    string
	VendedorNome   string
	Itens          []ItemPedido
}

// ItemPedido é uma linha do pedido (com sku e descrição do produto na
// leitura, via JOIN). ValorBruto é sempre calculado pelo domínio (V-03).
type ItemPedido struct {
	ItemIDOrigem   int64
	PedidoID       int64
	ProdutoID      int64
	Quantidade     int
	PrecoPraticado kernel.Dinheiro
	DescontoPct    float64
	ValorBruto     kernel.Dinheiro
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ProdutoSKU     string
	ProdutoNome    string
}

// DadosItem é um item do comando de criação/edição.
type DadosItem struct {
	ProdutoID      int64
	Quantidade     int
	PrecoPraticado kernel.Decimal
	DescontoPct    float64
}

// DadosPedido é o comando de criação/edição. Para escopo restrito,
// VendedorID é sobrescrito pelo do escopo antes de NovoPedido (E-05).
// Não há valor_bruto nem valor_total: o backend calcula (V-03).
type DadosPedido struct {
	ClienteID  int64
	VendedorID int64
	DataPedido string // AAAA-MM-DD
	Canal      string
	Status     string
	Itens      []DadosItem
}

// NovoPedido valida o comando na ordem do legado (cliente, vendedor, data,
// canal, status, itens e, item a item, produto, quantidade, preço e
// desconto) e calcula o valor_bruto de cada item e o valor_total (V-03):
// bruto = quantidade * preco_praticado * (1 - desconto/100), exato e
// arredondado ao centavo (metade para longe do zero); total = soma dos brutos.
func NovoPedido(d DadosPedido) (*Pedido, error) {
	switch {
	case d.ClienteID <= 0:
		return nil, ErrClienteIDObrigatorio
	case d.VendedorID <= 0:
		return nil, ErrVendedorIDObrigatorio
	}
	data, err := kernel.ParseDataLocal(d.DataPedido, ErrDataPedidoInvalida)
	if err != nil {
		return nil, err
	}
	canal, err := NovoCanal(d.Canal)
	if err != nil {
		return nil, err
	}
	status, err := NovoStatusPedido(d.Status)
	if err != nil {
		return nil, err
	}
	itens, total, err := novosItens(d.Itens)
	if err != nil {
		return nil, err
	}
	return &Pedido{ClienteID: d.ClienteID, VendedorID: d.VendedorID, DataPedido: data,
		Canal: string(canal), Status: string(status), ValorTotal: total, Itens: itens}, nil
}

// MaxItensPedido é o teto de itens de um pedido, na criação e no PUT
// (SEC-14). O maior pedido histórico tinha 16 itens (conferido em 2026-10-31).
const MaxItensPedido = 200

// novosItens valida os itens e soma o valor_total em centavos (ARQ-09).
func novosItens(ds []DadosItem) ([]ItemPedido, kernel.Dinheiro, error) {
	var total kernel.Dinheiro
	if len(ds) == 0 {
		return nil, total, ErrItensObrigatorios
	}
	if len(ds) > MaxItensPedido {
		return nil, total, ErrItensDemais
	}
	itens := make([]ItemPedido, 0, len(ds))
	for _, d := range ds {
		it, bruto, err := novoItem(d)
		if err != nil {
			return nil, total, err
		}
		if total, err = total.Somar(bruto); err != nil {
			return nil, total, ErrValorForaDoLimite // N-9: a soma passou de DECIMAL(15,2)
		}
		itens = append(itens, it)
	}
	return itens, total, nil
}

func novoItem(d DadosItem) (ItemPedido, kernel.Dinheiro, error) {
	switch {
	case d.ProdutoID <= 0:
		return ItemPedido{}, kernel.Dinheiro{}, ErrProdutoIDObrigatorio
	case d.Quantidade <= 0:
		return ItemPedido{}, kernel.Dinheiro{}, ErrQuantidadeInvalida
	}
	preco, err := d.PrecoPraticado.Dinheiro(ErrPrecoPraticadoInvalido, ErrPrecoPraticadoCasas)
	if err != nil {
		return ItemPedido{}, kernel.Dinheiro{}, err
	}
	desconto, err := NovoDescontoPct(d.DescontoPct)
	if err != nil {
		return ItemPedido{}, kernel.Dinheiro{}, err
	}
	bruto, err := preco.MultiplicarPor(desconto.fator(d.Quantidade))
	if err != nil {
		return ItemPedido{}, kernel.Dinheiro{}, ErrValorForaDoLimite // N-9: o produto passou de DECIMAL(15,2)
	}
	return ItemPedido{ProdutoID: d.ProdutoID, Quantidade: d.Quantidade, PrecoPraticado: preco,
		DescontoPct: d.DescontoPct, ValorBruto: bruto}, bruto, nil
}

// PodeExcluir aplica as travas da exclusão (V-01), na ordem do legado:
// pagamentos vinculados primeiro, depois o status: Faturado e Entregue (pós-
// faturamento, estoque já baixado; BUG-16) não saem.
func (p *Pedido) PodeExcluir(temPagamentos bool) error {
	if temPagamentos {
		return ErrPedidoPossuiPagamentosVinculados
	}
	switch StatusPedido(p.Status) {
	case StatusFaturado:
		return ErrPedidoFaturadoNaoPodeSerExcluido
	case StatusEntregue:
		return ErrPedidoEntregueNaoPodeSerExcluido
	}
	return nil
}

// PlanejarCriacao decide se a criação do pedido baixa o estoque (BUG-20):
// criado já como Faturado, baixa na mesma tx (a mesma baixa da transição do
// PUT); criado como Entregue, é recusado, porque Entregue só vem depois de
// Faturado e criar direto nele pularia a baixa.
func PlanejarCriacao(novo *Pedido) (baixarEstoque bool, err error) {
	switch StatusPedido(novo.Status) {
	case StatusEntregue:
		return false, ErrPedidoEntregueSemFaturamento
	case StatusFaturado:
		return true, nil
	}
	return false, nil
}

// PlanoAtualizacao diz o que o PUT de pedido grava além do cabeçalho.
type PlanoAtualizacao struct {
	// ReescreverItens: os itens são substituídos (DELETE + INSERT). Falso
	// para pedido já faturado ou entregue, cujas linhas ficam como estão.
	ReescreverItens bool
	// BaixarEstoque: transição para Faturado (idempotente: reenviar
	// "Faturado" para um pedido já faturado não baixa de novo).
	BaixarEstoque bool
}

// PlanejarAtualizacao decide, a partir do status ATUAL (lido com lock na
// mesma tx) e dos itens atuais, o que a atualização para novo pode fazer.
// Primeiro a transição de status (BUG-16: Faturado só vai para Entregue,
// Entregue é terminal, Entregue só a partir de Faturado; 400), depois os
// itens: pedido faturado ou entregue não aceita itens diferentes (409).
func PlanejarAtualizacao(statusAtual string, itensAtuais []ItemPedido, novo *Pedido) (PlanoAtualizacao, error) {
	atual, destino := StatusPedido(statusAtual), StatusPedido(novo.Status)
	if err := validarTransicao(atual, destino); err != nil {
		return PlanoAtualizacao{}, err
	}
	posFaturamento := atual == StatusFaturado || atual == StatusEntregue
	if posFaturamento && !MesmosItens(itensAtuais, novo.Itens) {
		return PlanoAtualizacao{}, ErrPedidoJaFaturadoNaoPodeAlterarItens
	}
	return PlanoAtualizacao{
		ReescreverItens: !posFaturamento,
		BaixarEstoque:   !posFaturamento && destino == StatusFaturado,
	}, nil
}

// validarTransicao é a máquina de status do PUT. Reenviar o mesmo status
// é sempre permitido (idempotente). Status atual fora do ENUM (lido do
// banco, não validado) segue a regra dos status não faturados.
func validarTransicao(atual, destino StatusPedido) error {
	switch {
	case atual == destino:
		return nil
	case atual == StatusFaturado && destino != StatusEntregue:
		return ErrPedidoFaturadoSoPodeIrParaEntregue
	case atual == StatusEntregue:
		return ErrPedidoEntregueNaoPodeMudarStatus
	case destino == StatusEntregue && atual != StatusFaturado:
		return ErrPedidoEntregueSemFaturamento
	}
	return nil
}

// MesmosItens compara as duas listas pelo conteúdo de negócio (produto,
// quantidade, preço e desconto), na ordem, ignorando ids e timestamps.
func MesmosItens(a, b []ItemPedido) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ProdutoID != b[i].ProdutoID || a[i].Quantidade != b[i].Quantidade ||
			a[i].PrecoPraticado != b[i].PrecoPraticado || a[i].DescontoPct != b[i].DescontoPct {
			return false
		}
	}
	return true
}
