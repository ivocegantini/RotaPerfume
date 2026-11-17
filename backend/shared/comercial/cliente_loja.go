package comercial

import (
	"slices"
	"strings"

	"github.com/rotaperfumes/shared/kernel"
)

// SegmentoLojaOnline é o segmento dos clientes criados pelo autocadastro.
const SegmentoLojaOnline = "Loja Online"

// Erros do cliente da loja (ECOM-05). Nunca ecoam o valor (CPF é PII).
var (
	ErrCPFInvalido           = kernel.NovoErro(kernel.KindInvalido, "cpf inválido")
	ErrCEPInvalido           = kernel.NovoErro(kernel.KindInvalido, "cep deve ter 8 dígitos")
	ErrEnderecoObrigatorio   = kernel.NovoErro(kernel.KindInvalido, "rua (logradouro) é obrigatória")
	ErrEnderecoMuitoLongo    = kernel.NovoErro(kernel.KindInvalido, "rua (logradouro) deve ter no máximo 255 caracteres")
	ErrNumeroObrigatorio     = kernel.NovoErro(kernel.KindInvalido, "número é obrigatório (use s/n se não houver)")
	ErrNumeroMuitoLongo      = kernel.NovoErro(kernel.KindInvalido, "número deve ter no máximo 20 caracteres")
	ErrComplementoMuitoLongo = kernel.NovoErro(kernel.KindInvalido, "complemento deve ter no máximo 120 caracteres")
	ErrBairroMuitoLongo      = kernel.NovoErro(kernel.KindInvalido, "bairro deve ter no máximo 120 caracteres")
	ErrCidadeMuitoLonga      = kernel.NovoErro(kernel.KindInvalido, "cidade deve ter no máximo 120 caracteres")
	ErrUFDesconhecida        = kernel.NovoErro(kernel.KindInvalido, "uf inválida")
	ErrCPFDuplicado          = kernel.NovoErro(kernel.KindConflito, "cpf já cadastrado")
	ErrDocumentoDuplo        = kernel.NovoErro(kernel.KindInvalido, "informe cnpj (pessoa jurídica) ou cpf (pessoa física), não os dois")
)

// ufsBrasil são as 27 UFs.
var ufsBrasil = []string{"AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO", "MA", "MG", "MS", "MT", "PA", "PB",
	"PE", "PI", "PR", "RJ", "RN", "RO", "RR", "RS", "SC", "SE", "SP", "TO"}

// DadosClienteLoja é a entrada crua do cliente da loja (texto digitado).
// Endereco é o logradouro (ECOM-09); CEP, Bairro e Complemento são opcionais.
type DadosClienteLoja struct {
	Nome, CPF, Endereco, Numero, Complemento, Bairro, CEP, Cidade, UF string
}

// ClienteLoja é o cliente pessoa física do autocadastro, já normalizado.
// CEP "" = endereço sem CEP.
type ClienteLoja struct {
	Nome, CPF, Endereco, Numero, Complemento, Bairro, CEP, Cidade, UF string
}

// NovoClienteLoja valida e normaliza (CPF e CEP só dígitos, UF maiúscula).
// Ordem: CPF, rua, número, complemento, bairro, CEP (se informado), cidade, UF.
func NovoClienteLoja(d DadosClienteLoja) (*ClienteLoja, error) {
	c := &ClienteLoja{
		Nome:        strings.TrimSpace(d.Nome),
		CPF:         soDigitos(d.CPF),
		Endereco:    strings.TrimSpace(d.Endereco),
		Numero:      strings.TrimSpace(d.Numero),
		Complemento: strings.TrimSpace(d.Complemento),
		Bairro:      strings.TrimSpace(d.Bairro),
		CEP:         soDigitos(d.CEP),
		Cidade:      strings.TrimSpace(d.Cidade),
	}
	if !soMascaraNumerica(d.CPF) || !CPFValido(c.CPF) {
		return nil, ErrCPFInvalido
	}
	switch {
	case c.Endereco == "":
		return nil, ErrEnderecoObrigatorio
	case len([]rune(c.Endereco)) > 255:
		return nil, ErrEnderecoMuitoLongo
	case c.Numero == "":
		return nil, ErrNumeroObrigatorio
	case len([]rune(c.Numero)) > 20:
		return nil, ErrNumeroMuitoLongo
	case len([]rune(c.Complemento)) > 120:
		return nil, ErrComplementoMuitoLongo
	case len([]rune(c.Bairro)) > 120:
		return nil, ErrBairroMuitoLongo
	case strings.TrimSpace(d.CEP) != "" && (len(c.CEP) != 8 || !soMascaraNumerica(d.CEP)):
		return nil, ErrCEPInvalido
	case c.Cidade == "":
		return nil, ErrCidadeObrigatoria
	case len([]rune(c.Cidade)) > 120:
		return nil, ErrCidadeMuitoLonga
	}
	uf, err := normalizarUF(d.UF)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(ufsBrasil, uf) {
		return nil, ErrUFDesconhecida
	}
	c.UF = uf
	return c, nil
}

// soMascaraNumerica: CPF/CEP digitado só pode ter dígitos, hífen, ponto e
// espaço (letras não são descartadas em silêncio).
func soMascaraNumerica(s string) bool {
	return strings.Trim(s, "0123456789-. ") == ""
}

// CPFValido confere 11 dígitos, rejeita todos iguais e os dois DVs.
func CPFValido(cpf string) bool {
	if len(cpf) != 11 || strings.Count(cpf, cpf[:1]) == 11 {
		return false
	}
	for _, c := range cpf {
		if c < '0' || c > '9' {
			return false
		}
	}
	dv := func(n int) byte {
		soma := 0
		for i := 0; i < n; i++ {
			soma += int(cpf[i]-'0') * (n + 1 - i)
		}
		r := soma * 10 % 11
		if r == 10 {
			r = 0
		}
		return byte('0' + r)
	}
	return cpf[9] == dv(9) && cpf[10] == dv(10)
}

// soDigitos remove tudo que não for 0-9 (máscaras de CPF/CEP). Um CPF com
// letras vira menos de 11 dígitos e é recusado.
func soDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
