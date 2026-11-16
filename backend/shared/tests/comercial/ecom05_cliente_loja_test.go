package comercial_test

// ECOM-05: cliente pessoa física do autocadastro da loja (CPF, CEP, UF).

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/comercial"
)

func TestECOM05_CPFValido(t *testing.T) {
	for _, ok := range []string{"52998224725", "11144477735"} {
		assert.True(t, comercial.CPFValido(ok), ok)
	}
	for _, ruim := range []string{"", "52998224724", "11111111111", "5299822472", "529982247250", "5299822472a"} {
		assert.False(t, comercial.CPFValido(ruim), ruim)
	}
}

func dadosOK() comercial.DadosClienteLoja {
	return comercial.DadosClienteLoja{Nome: " Ana ", CPF: "529.982.247-25", Endereco: " Praça da Sé ", Numero: " 10 ",
		Complemento: " ap 2 ", Bairro: " Sé ", CEP: "01001-000", Cidade: " São Paulo ", UF: "sp"}
}

func TestECOM05_NovoClienteLoja_Normaliza(t *testing.T) {
	c, err := comercial.NovoClienteLoja(dadosOK())
	require.NoError(t, err)
	assert.Equal(t, comercial.ClienteLoja{Nome: "Ana", CPF: "52998224725", Endereco: "Praça da Sé", Numero: "10",
		Complemento: "ap 2", Bairro: "Sé", CEP: "01001000", Cidade: "São Paulo", UF: "SP"}, *c)
}

// ECOM-09: endereço sem CEP (e sem bairro/complemento) é aceito.
func TestECOM09_NovoClienteLoja_SemCEP(t *testing.T) {
	d := dadosOK()
	d.CEP, d.Bairro, d.Complemento = " ", "", ""
	c, err := comercial.NovoClienteLoja(d)
	require.NoError(t, err)
	assert.Equal(t, "", c.CEP)
	assert.Equal(t, "", c.Bairro)
}

func TestECOM05_NovoClienteLoja_Recusas(t *testing.T) {
	casos := []struct {
		nome string
		mudar func(*comercial.DadosClienteLoja)
		erro  error
	}{
		{"cpf dv", func(d *comercial.DadosClienteLoja) { d.CPF = "529.982.247-24" }, comercial.ErrCPFInvalido},
		{"cpf com letra", func(d *comercial.DadosClienteLoja) { d.CPF = "52998224725x" }, comercial.ErrCPFInvalido},
		{"endereco vazio", func(d *comercial.DadosClienteLoja) { d.Endereco = " " }, comercial.ErrEnderecoObrigatorio},
		{"cep curto", func(d *comercial.DadosClienteLoja) { d.CEP = "0100-100" }, comercial.ErrCEPInvalido},
		{"cep com letra", func(d *comercial.DadosClienteLoja) { d.CEP = "01001-00a0" }, comercial.ErrCEPInvalido},
		{"cidade vazia", func(d *comercial.DadosClienteLoja) { d.Cidade = "" }, comercial.ErrCidadeObrigatoria},
		{"numero vazio", func(d *comercial.DadosClienteLoja) { d.Numero = " " }, comercial.ErrNumeroObrigatorio},
		{"numero longo", func(d *comercial.DadosClienteLoja) { d.Numero = strings.Repeat("1", 21) }, comercial.ErrNumeroMuitoLongo},
		{"complemento longo", func(d *comercial.DadosClienteLoja) { d.Complemento = strings.Repeat("a", 121) }, comercial.ErrComplementoMuitoLongo},
		{"bairro longo", func(d *comercial.DadosClienteLoja) { d.Bairro = strings.Repeat("a", 121) }, comercial.ErrBairroMuitoLongo},
		{"uf inexistente", func(d *comercial.DadosClienteLoja) { d.UF = "XX" }, comercial.ErrUFDesconhecida},
		{"uf tamanho", func(d *comercial.DadosClienteLoja) { d.UF = "S" }, comercial.ErrUFInvalida},
	}
	for _, c := range casos {
		d := dadosOK()
		c.mudar(&d)
		_, err := comercial.NovoClienteLoja(d)
		assert.ErrorIs(t, err, c.erro, c.nome)
	}
}
