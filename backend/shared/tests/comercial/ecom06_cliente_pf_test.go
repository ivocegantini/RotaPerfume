package comercial_test

// ECOM-06: cliente pessoa física no CRM (CPF no lugar do CNPJ + endereço).

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/comercial"
)

func dadosPF() comercial.DadosCliente {
	return comercial.DadosCliente{RazaoSocial: "Ana Souza", CPF: "529.982.247-25", Segmento: comercial.SegmentoLojaOnline,
		Cidade: "Curitiba", UF: "pr", Endereco: " Rua XV ", Numero: "10", CEP: "80020-310", DataCadastro: "2026-11-01"}
}

func TestECOM06_ClientePF_NormalizaESemCNPJ(t *testing.T) {
	c, err := comercial.ClienteEditado(dadosPF())
	require.NoError(t, err)
	assert.Equal(t, "", c.CNPJ)
	assert.Equal(t, "52998224725", c.CPF)
	assert.Equal(t, "Rua XV", c.Endereco)
	assert.Equal(t, "80020310", c.CEP)
	assert.Equal(t, "PR", c.UF)
}

func TestECOM06_ClientePF_Recusas(t *testing.T) {
	casos := map[string]struct {
		muda func(*comercial.DadosCliente)
		erro error
	}{
		"cnpj e cpf juntos": {func(d *comercial.DadosCliente) { d.CNPJ = "11222333000181" }, comercial.ErrDocumentoDuplo},
		"cpf com DV errado": {func(d *comercial.DadosCliente) { d.CPF = "52998224724" }, comercial.ErrCPFInvalido},
		"cpf com letras":    {func(d *comercial.DadosCliente) { d.CPF = "5299822472a5" }, comercial.ErrCPFInvalido},
		"pf sem rua":        {func(d *comercial.DadosCliente) { d.Endereco = " " }, comercial.ErrEnderecoObrigatorio},
		"pf sem número":     {func(d *comercial.DadosCliente) { d.Numero = "" }, comercial.ErrNumeroObrigatorio},
		"cep curto":         {func(d *comercial.DadosCliente) { d.CEP = "8002" }, comercial.ErrCEPInvalido},
		"sem documento":     {func(d *comercial.DadosCliente) { d.CPF = "" }, comercial.ErrCNPJObrigatorio},
	}
	for nome, tc := range casos {
		t.Run(nome, func(t *testing.T) {
			d := dadosPF()
			tc.muda(&d)
			_, err := comercial.ClienteEditado(d)
			assert.ErrorIs(t, err, tc.erro)
		})
	}
}

func TestECOM06_ClientePJ_EnderecoOpcional(t *testing.T) {
	d := comercial.DadosCliente{RazaoSocial: "ACME", CNPJ: "11.222.333/0001-81", Segmento: "Varejo", Cidade: "Curitiba", UF: "PR"}
	c, err := comercial.NovoCliente(d, time.Now())
	require.NoError(t, err)
	assert.Equal(t, "11222333000181", c.CNPJ)
	assert.Equal(t, "", c.CPF)
	assert.Equal(t, "", c.Endereco)
}
