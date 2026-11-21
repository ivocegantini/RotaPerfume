package vendas_test

// ECOM-14: domínio do checkout da loja (validação do carrinho e montagem do
// pedido + pagamento com os preços do servidor).

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
)

const chaveECOM14 = "0b7f3c1e-5d2a-4c8e-9f10-123456789abc"

func checkoutValido(forma string, itens ...vendas.ItemCheckout) vendas.DadosCheckout {
	return vendas.DadosCheckout{Chave: chaveECOM14, Forma: forma, Itens: itens, Entrega: entregaTeste}
}

func TestECOM14_Validar(t *testing.T) {
	it := func(id int64, q int) vendas.ItemCheckout { return vendas.ItemCheckout{ProdutoID: id, Quantidade: q} }
	v, err := vendas.DadosCheckout{Chave: " 0B7F3C1E-5D2A-4C8E-9F10-123456789ABC ", Forma: " cartao ", Itens: []vendas.ItemCheckout{it(7, 99)}, Entrega: entregaTeste}.Validar()
	require.NoError(t, err)
	assert.Equal(t, chaveECOM14, v.Chave, "chave normalizada")
	assert.Equal(t, vendas.CheckoutCartao, v.Forma)

	muitos := make([]vendas.ItemCheckout, vendas.MaxItensCheckout+1)
	for i := range muitos {
		muitos[i] = it(int64(i+1), 1)
	}
	casos := []struct {
		d   vendas.DadosCheckout
		err error
	}{
		{vendas.DadosCheckout{Chave: "nao-uuid", Forma: "pix", Itens: []vendas.ItemCheckout{it(1, 1)}}, vendas.ErrCheckoutChaveInvalida},
		// ECOM-17: sem endereço de entrega a compra não é finalizada.
		{vendas.DadosCheckout{Chave: chaveECOM14, Forma: "pix", Itens: []vendas.ItemCheckout{it(1, 1)}}, vendas.ErrEntregaCEPInvalido},
		{checkoutValido("dinheiro", it(1, 1)), vendas.ErrCheckoutFormaInvalida},
		{checkoutValido("pix"), vendas.ErrCheckoutCarrinhoVazio},
		{checkoutValido("pix", muitos...), vendas.ErrCheckoutItensDemais},
		{checkoutValido("pix", it(0, 1)), vendas.ErrProdutoIDObrigatorio},
		{checkoutValido("pix", it(1, 0)), vendas.ErrCheckoutQuantidadeInvalida},
		{checkoutValido("pix", it(1, 100)), vendas.ErrCheckoutQuantidadeInvalida},
		{checkoutValido("pix", it(1, 1), it(1, 2)), vendas.ErrCheckoutProdutoRepetido},
	}
	for _, c := range casos {
		_, err := c.d.Validar()
		assert.ErrorIs(t, err, c.err)
	}
}

func TestECOM14_MontarCompraLoja(t *testing.T) {
	hoje := time.Date(2026, 11, 18, 23, 30, 0, 0, time.Local)
	ofertas := map[int64]vendas.OfertaLoja{}
	for id, s := range map[int64]string{7: "100.25", 8: "0.10"} {
		d, err := kernel.ParseDinheiro(s)
		require.NoError(t, err)
		ofertas[id] = vendas.OfertaLoja{Preco: d, Disponivel: 3}
	}
	for forma, esperado := range map[string]struct {
		forma string
		venc  string
	}{
		"pix":    {"PIX", "2026-11-18"},
		"cartao": {"Cartão de crédito", "2026-11-18"},
		"boleto": {"Boleto 14 dias", "2026-12-02"},
	} {
		v, err := checkoutValido(forma, vendas.ItemCheckout{ProdutoID: 7, Quantidade: 3}, vendas.ItemCheckout{ProdutoID: 8, Quantidade: 1}).Validar()
		require.NoError(t, err)
		ped, pag, err := vendas.MontarCompraLoja(v, 55, 168, ofertas, hoje)
		require.NoError(t, err)
		assert.Equal(t, "Loja", ped.Canal)
		assert.Equal(t, "Aguardando pagamento", ped.Status, "ECOM-18")
		assert.Equal(t, vendas.Entrega{CEP: "01310100", Endereco: "Avenida Paulista", Numero: "1000", Bairro: "Bela Vista",
			Cidade: "São Paulo", UF: "SP"}, ped.Entrega, "ECOM-17: o pedido nasce com o endereço normalizado")
		assert.Equal(t, int64(55), ped.ClienteID)
		assert.Equal(t, int64(168), ped.VendedorID)
		assert.Equal(t, "2026-11-18", ped.DataPedido.Format("2006-01-02"))
		assert.Equal(t, "300.85", ped.ValorTotal.String())
		require.Len(t, ped.Itens, 2)
		assert.Equal(t, "300.75", ped.Itens[0].ValorBruto.String())
		assert.Equal(t, esperado.forma, pag.FormaPagamento)
		assert.Equal(t, esperado.venc, pag.DataVencimento.Format("2006-01-02"))
		assert.Equal(t, "Em aberto", pag.StatusPagamento)
		assert.Equal(t, "300.85", pag.Valor.String())
		assert.Equal(t, pag.Valor, pag.ValorLiquido)
		assert.Nil(t, pag.DataPagamento)
	}

	v, err := checkoutValido("pix", vendas.ItemCheckout{ProdutoID: 9, Quantidade: 1}).Validar()
	require.NoError(t, err)
	_, _, err = vendas.MontarCompraLoja(v, 55, 168, ofertas, hoje)
	assert.ErrorIs(t, err, vendas.ErrCheckoutProdutoIndisponivel, "produto fora do mapa (inativo/inexistente)")
}

// SEC-79: quantidade até o disponível passa; acima, ErrCheckoutSemEstoque.
func TestSEC79_MontarCompraLoja_Estoque(t *testing.T) {
	preco, err := kernel.ParseDinheiro("10.00")
	require.NoError(t, err)
	hoje := time.Date(2026, 11, 18, 10, 0, 0, 0, time.Local)
	ofertas := map[int64]vendas.OfertaLoja{7: {Preco: preco, Disponivel: 2}, 8: {Preco: preco, Disponivel: 0}}
	montar := func(id int64, q int) error {
		v, err := checkoutValido("pix", vendas.ItemCheckout{ProdutoID: id, Quantidade: q}).Validar()
		require.NoError(t, err)
		_, _, err = vendas.MontarCompraLoja(v, 55, 168, ofertas, hoje)
		return err
	}
	assert.NoError(t, montar(7, 2), "exatamente o disponível")
	assert.ErrorIs(t, montar(7, 3), vendas.ErrCheckoutSemEstoque)
	assert.ErrorIs(t, montar(8, 1), vendas.ErrCheckoutSemEstoque, "ativo mas zerado")
}
