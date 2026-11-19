package vendas_test

// ECOM-17: endereço de entrega obrigatório (domínio puro): normalização,
// ordem de validação e o pedido sem endereço recusado.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/vendas"
)

func TestECOM17_NovaEntrega_Normaliza(t *testing.T) {
	e, err := vendas.NovaEntrega(vendas.DadosEntrega{CEP: " 01310-100 ", Endereco: " Av. Paulista ", Numero: " s/n ",
		Complemento: " apto 5 ", Bairro: " Bela Vista ", Cidade: " São Paulo ", UF: " sp "})
	require.NoError(t, err)
	assert.Equal(t, vendas.Entrega{CEP: "01310100", Endereco: "Av. Paulista", Numero: "s/n", Complemento: "apto 5",
		Bairro: "Bela Vista", Cidade: "São Paulo", UF: "SP"}, e)
}

func TestECOM17_NovaEntrega_ValidacaoNaOrdem(t *testing.T) {
	ok := func() vendas.DadosEntrega { return entregaTeste }
	com := func(f func(*vendas.DadosEntrega)) vendas.DadosEntrega { d := ok(); f(&d); return d }
	longo := func(n int) string { return strings.Repeat("á", n) }
	casos := []struct {
		nome string
		d    vendas.DadosEntrega
		err  error
	}{
		{"tudo vazio: o CEP vem primeiro", vendas.DadosEntrega{}, vendas.ErrEntregaCEPInvalido},
		{"cep curto", com(func(d *vendas.DadosEntrega) { d.CEP = "0131010" }), vendas.ErrEntregaCEPInvalido},
		{"cep com letra", com(func(d *vendas.DadosEntrega) { d.CEP = "01310-10a" }), vendas.ErrEntregaCEPInvalido},
		{"cep longo", com(func(d *vendas.DadosEntrega) { d.CEP = "013101000" }), vendas.ErrEntregaCEPInvalido},
		{"rua vazia", com(func(d *vendas.DadosEntrega) { d.Endereco = "  " }), vendas.ErrEntregaEnderecoObrig},
		{"rua longa", com(func(d *vendas.DadosEntrega) { d.Endereco = longo(256) }), vendas.ErrEntregaEnderecoLongo},
		{"numero vazio", com(func(d *vendas.DadosEntrega) { d.Numero = "" }), vendas.ErrEntregaNumeroObrig},
		{"numero longo", com(func(d *vendas.DadosEntrega) { d.Numero = longo(21) }), vendas.ErrEntregaNumeroLongo},
		{"complemento longo", com(func(d *vendas.DadosEntrega) { d.Complemento = longo(121) }), vendas.ErrEntregaComplementoLongo},
		{"bairro vazio", com(func(d *vendas.DadosEntrega) { d.Bairro = "" }), vendas.ErrEntregaBairroObrig},
		{"bairro longo", com(func(d *vendas.DadosEntrega) { d.Bairro = longo(121) }), vendas.ErrEntregaBairroLongo},
		{"cidade vazia", com(func(d *vendas.DadosEntrega) { d.Cidade = "" }), vendas.ErrEntregaCidadeObrig},
		{"cidade longa", com(func(d *vendas.DadosEntrega) { d.Cidade = longo(121) }), vendas.ErrEntregaCidadeLonga},
		{"uf vazia", com(func(d *vendas.DadosEntrega) { d.UF = "" }), vendas.ErrEntregaUFInvalida},
		{"uf inexistente", com(func(d *vendas.DadosEntrega) { d.UF = "XX" }), vendas.ErrEntregaUFInvalida},
		{"uf por extenso", com(func(d *vendas.DadosEntrega) { d.UF = "São Paulo" }), vendas.ErrEntregaUFInvalida},
	}
	for _, c := range casos {
		_, err := vendas.NovaEntrega(c.d)
		assert.ErrorIs(t, err, c.err, c.nome)
	}
	// Limites exatos passam (contagem em runas, não bytes).
	_, err := vendas.NovaEntrega(com(func(d *vendas.DadosEntrega) {
		d.Endereco, d.Numero, d.Complemento, d.Bairro, d.Cidade = longo(255), longo(20), longo(120), longo(120), longo(120)
	}))
	assert.NoError(t, err)
}

// O pedido (admin ou loja) sem endereço é recusado; a validação do endereço
// vem depois dos itens (erros antigos continuam na mesma ordem).
func TestECOM17_NovoPedido_ExigeEntrega(t *testing.T) {
	d := pedidoValido()
	d.Entrega = vendas.DadosEntrega{}
	_, err := vendas.NovoPedido(d)
	assert.ErrorIs(t, err, vendas.ErrEntregaCEPInvalido)

	d.Itens = nil
	_, err = vendas.NovoPedido(d)
	assert.ErrorIs(t, err, vendas.ErrItensObrigatorios, "itens antes do endereço")

	p, err := vendas.NovoPedido(pedidoValido())
	require.NoError(t, err)
	assert.Equal(t, "01310100", p.Entrega.CEP)
	assert.Equal(t, "SP", p.Entrega.UF)
}
