package vendas_test

// ARQ-09: regressão dos totais. O cálculo em centavos reproduz o que o
// legado (float64 gravado em DECIMAL(15,2)) produzia nos valores típicos;
// as divergências de borda ficam explícitas abaixo, a favor do exato.

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
)

// legadoDecimal emula a gravação do float64 em DECIMAL(15,2): o texto com 15
// dígitos significativos (DBL_DIG), arredondado ao centavo longe do zero.
func legadoDecimal(v float64) string {
	r, _ := new(big.Rat).SetString(strconv.FormatFloat(v, 'g', 15, 64))
	d, _ := kernel.ParseDinheiro("1")
	c, _ := d.MultiplicarPor(r)
	return c.String()
}

// legadoBruto é a fórmula antiga do domínio, em float64.
func legadoBruto(q int, preco, desconto float64) float64 {
	return float64(q) * preco * (1 - desconto/100)
}

func pedidoCom(itens ...vendas.DadosItem) vendas.DadosPedido {
	d := pedidoValido()
	d.Itens = itens
	return d
}

// Grade de valores típicos: cada valor_bruto é o mesmo do legado.
func TestARQ09_Regressao_BrutoIgualAoLegado(t *testing.T) {
	precos := []string{"0.01", "0.10", "0.20", "0.99", "9.99", "19.90", "33.33", "49.95", "99.99", "129.90", "1234.56"}
	quantidades := []int{1, 2, 3, 7, 12, 50}
	descontos := []float64{0, 2.5, 5, 10, 12.5, 15, 33.33, 50, 100}
	for _, p := range precos {
		pf, _ := strconv.ParseFloat(p, 64)
		for _, q := range quantidades {
			for _, d := range descontos {
				ped, err := vendas.NovoPedido(pedidoCom(vendas.DadosItem{ProdutoID: 1, Quantidade: q, PrecoPraticado: kernel.Decimal(p), DescontoPct: d}))
				require.NoError(t, err)
				assert.Equal(t, legadoDecimal(legadoBruto(q, pf, d)), ped.Itens[0].ValorBruto.String(), "q=%d preço=%s desconto=%v", q, p, d)
			}
		}
	}
}

// Pedidos típicos: o valor_total é o mesmo do legado.
func TestARQ09_Regressao_TotalIgualAoLegado(t *testing.T) {
	type item struct {
		q        int
		preco    string
		desconto float64
	}
	casos := []struct {
		nome  string
		itens []item
		total string
	}{
		{"0.1 + 0.2 (float daria 0.30000000000000004)", []item{{1, "0.10", 0}, {1, "0.20", 0}}, "0.30"},
		{"3 × 33.33 com 10% (89.991)", []item{{3, "33.33", 10}}, "89.99"},
		{"pedido do teste legado", []item{{2, "100", 10}, {1, "50", 0}}, "230.00"},
		{"vários itens", []item{{12, "19.90", 5}, {3, "129.90", 0}, {1, "0.99", 0}, {7, "49.95", 15}}, "914.75"},
		{"desconto de 100%", []item{{5, "99.99", 100}, {1, "10.00", 0}}, "10.00"},
		{"dez itens de 0.10", []item{{1, "0.10", 0}, {1, "0.10", 0}, {1, "0.10", 0}, {1, "0.10", 0}, {1, "0.10", 0},
			{1, "0.10", 0}, {1, "0.10", 0}, {1, "0.10", 0}, {1, "0.10", 0}, {1, "0.10", 0}}, "1.00"},
		{"valor alto", []item{{50, "1234.56", 2.5}, {12, "9999.99", 0}}, "180184.68"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var dados []vendas.DadosItem
			var legado float64
			for i, it := range c.itens {
				dados = append(dados, vendas.DadosItem{ProdutoID: int64(i + 1), Quantidade: it.q, PrecoPraticado: kernel.Decimal(it.preco), DescontoPct: it.desconto})
				pf, _ := strconv.ParseFloat(it.preco, 64)
				legado += legadoBruto(it.q, pf, it.desconto)
			}
			p, err := vendas.NovoPedido(pedidoCom(dados...))
			require.NoError(t, err)
			assert.Equal(t, c.total, p.ValorTotal.String())
			assert.Equal(t, legadoDecimal(legado), p.ValorTotal.String(), "mesmo total do legado")
		})
	}
}

// Divergências de borda (decisões do ARQ-09): o meio centavo exato arredonda
// para longe do zero, e o total é a soma dos brutos já arredondados (o que
// a API mostra item a item), não o arredondamento da soma crua do float64.
func TestARQ09_Regressao_BordasDeArredondamento(t *testing.T) {
	casos := []struct {
		nome          string
		itens         []vendas.DadosItem
		brutos        []string
		total, legado string
	}{
		{"meio centavo exato (0.005 → 0.01)", []vendas.DadosItem{{ProdutoID: 1, Quantidade: 1, PrecoPraticado: "0.01", DescontoPct: 50}},
			[]string{"0.01"}, "0.01", "0.01"},
		{"1 × 1.15 com 50% (0.575 → 0.58)", []vendas.DadosItem{{ProdutoID: 1, Quantidade: 1, PrecoPraticado: "1.15", DescontoPct: 50}},
			[]string{"0.58"}, "0.58", "0.58"},
		{"desconto com 3 casas (0.4995 → 0.50)", []vendas.DadosItem{{ProdutoID: 1, Quantidade: 1, PrecoPraticado: "1.00", DescontoPct: 50.05}},
			[]string{"0.50"}, "0.50", "0.50"},
		{"soma de meios centavos: 0.01 + 0.01 (legado: 0.01)", []vendas.DadosItem{
			{ProdutoID: 1, Quantidade: 1, PrecoPraticado: "0.01", DescontoPct: 50},
			{ProdutoID: 2, Quantidade: 1, PrecoPraticado: "0.01", DescontoPct: 50},
		}, []string{"0.01", "0.01"}, "0.02", "0.01"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p, err := vendas.NovoPedido(pedidoCom(c.itens...))
			require.NoError(t, err)
			for i, b := range c.brutos {
				assert.Equal(t, b, p.Itens[i].ValorBruto.String())
			}
			assert.Equal(t, c.total, p.ValorTotal.String())
			var legado float64
			for _, it := range c.itens {
				pf, _ := strconv.ParseFloat(string(it.PrecoPraticado), 64)
				legado += legadoBruto(it.Quantidade, pf, it.DescontoPct)
			}
			assert.Equal(t, c.legado, legadoDecimal(legado), "o que o legado gravaria")
		})
	}
}

// Entrada de dinheiro: mais de 2 casas e negativo são 400 com a mensagem do
// campo; o preço praticado fica exato no item (sem float64).
func TestARQ09_Pedido_EntradaDePreco(t *testing.T) {
	_, err := vendas.NovoPedido(pedidoCom(vendas.DadosItem{ProdutoID: 1, Quantidade: 1, PrecoPraticado: "10.005"}))
	assert.ErrorIs(t, err, vendas.ErrPrecoPraticadoCasas)
	assert.EqualError(t, err, "preco_praticado deve ter no máximo 2 casas decimais em todos os itens")
	_, err = vendas.NovoPedido(pedidoCom(vendas.DadosItem{ProdutoID: 1, Quantidade: 1, PrecoPraticado: "-1"}))
	assert.ErrorIs(t, err, vendas.ErrPrecoPraticadoInvalido)
	_, err = vendas.NovoPedido(pedidoCom(vendas.DadosItem{ProdutoID: 1, Quantidade: 1, PrecoPraticado: "1e20"}))
	assert.ErrorIs(t, err, vendas.ErrPrecoPraticadoInvalido, "acima de DECIMAL(15,2)")

	p, err := vendas.NovoPedido(pedidoCom(vendas.DadosItem{ProdutoID: 1, Quantidade: 1, PrecoPraticado: "19.9"}))
	require.NoError(t, err)
	assert.Equal(t, "19.90", p.Itens[0].PrecoPraticado.String())
	assert.Equal(t, "19.90", p.ValorTotal.String())
}

// Pagamento: valor e valor_liquido exatos; casas, negativo e limite.
func TestARQ09_Pagamento_Valores(t *testing.T) {
	base := vendas.DadosPagamento{FormaPagamento: "PIX", Parcelas: 1, Valor: "300.5", TaxaPct: 2.5, ValorLiquido: "293",
		DataVencimento: "2024-01-10", StatusPagamento: "Em aberto"}
	p, err := base.Pagamento(1)
	require.NoError(t, err)
	assert.Equal(t, "300.50", p.Valor.String())
	assert.Equal(t, "293.00", p.ValorLiquido.String())

	casos := map[string]struct {
		mudar func(*vendas.DadosPagamento)
		want  error
	}{
		"valor com 3 casas":         {func(d *vendas.DadosPagamento) { d.Valor = "0.001" }, vendas.ErrValorCasas},
		"valor negativo":            {func(d *vendas.DadosPagamento) { d.Valor = "-0.01" }, vendas.ErrValorInvalido},
		"valor acima do limite":     {func(d *vendas.DadosPagamento) { d.Valor = "10000000000000" }, vendas.ErrValorInvalido},
		"valor_liquido com 3 casas": {func(d *vendas.DadosPagamento) { d.ValorLiquido = "1.999" }, vendas.ErrValorLiquidoCasas},
		"valor_liquido negativo":    {func(d *vendas.DadosPagamento) { d.ValorLiquido = "-5" }, vendas.ErrValorLiquidoInvalido},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			d := base
			c.mudar(&d)
			_, err := d.Pagamento(1)
			assert.ErrorIs(t, err, c.want)
		})
	}
}
