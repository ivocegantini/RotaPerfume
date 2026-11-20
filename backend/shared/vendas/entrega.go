package vendas

import (
	"slices"
	"strings"

	"github.com/rotaperfumes/shared/kernel"
)

// ECOM-17: endereço de entrega obrigatório em todo pedido (sql/36). Sem ele
// a compra não é finalizada nem o pedido vai para separação. Só o
// complemento é opcional. Os erros nunca ecoam o valor (endereço é PII).

// Erros do endereço de entrega (400).
var (
	ErrEntregaCEPInvalido      = kernel.NovoErro(kernel.KindInvalido, "cep de entrega deve ter 8 dígitos")
	ErrEntregaEnderecoObrig    = kernel.NovoErro(kernel.KindInvalido, "rua de entrega é obrigatória")
	ErrEntregaEnderecoLongo    = kernel.NovoErro(kernel.KindInvalido, "rua de entrega deve ter no máximo 255 caracteres")
	ErrEntregaNumeroObrig      = kernel.NovoErro(kernel.KindInvalido, "número de entrega é obrigatório (use s/n se não houver)")
	ErrEntregaNumeroLongo      = kernel.NovoErro(kernel.KindInvalido, "número de entrega deve ter no máximo 20 caracteres")
	ErrEntregaComplementoLongo = kernel.NovoErro(kernel.KindInvalido, "complemento de entrega deve ter no máximo 120 caracteres")
	ErrEntregaBairroObrig      = kernel.NovoErro(kernel.KindInvalido, "bairro de entrega é obrigatório")
	ErrEntregaBairroLongo      = kernel.NovoErro(kernel.KindInvalido, "bairro de entrega deve ter no máximo 120 caracteres")
	ErrEntregaCidadeObrig      = kernel.NovoErro(kernel.KindInvalido, "cidade de entrega é obrigatória")
	ErrEntregaCidadeLonga      = kernel.NovoErro(kernel.KindInvalido, "cidade de entrega deve ter no máximo 120 caracteres")
	ErrEntregaUFInvalida       = kernel.NovoErro(kernel.KindInvalido, "estado (uf) de entrega inválido")
)

// ufsEntrega são as 27 UFs.
var ufsEntrega = []string{"AC", "AL", "AM", "AP", "BA", "CE", "DF", "ES", "GO", "MA", "MG", "MS", "MT", "PA", "PB",
	"PE", "PI", "PR", "RJ", "RN", "RO", "RR", "RS", "SC", "SE", "SP", "TO"}

// DadosEntrega é o endereço digitado (CEP com ou sem máscara).
type DadosEntrega struct {
	CEP, Endereco, Numero, Complemento, Bairro, Cidade, UF string
}

// Entrega é o endereço já validado: CEP só dígitos, UF maiúscula,
// complemento "" quando não houver.
type Entrega struct {
	CEP, Endereco, Numero, Complemento, Bairro, Cidade, UF string
}

// NovaEntrega valida na ordem do formulário: CEP, rua, número, complemento,
// bairro, cidade e UF.
func NovaEntrega(d DadosEntrega) (Entrega, error) {
	e := Entrega{
		CEP:         digitosCEP(d.CEP),
		Endereco:    strings.TrimSpace(d.Endereco),
		Numero:      strings.TrimSpace(d.Numero),
		Complemento: strings.TrimSpace(d.Complemento),
		Bairro:      strings.TrimSpace(d.Bairro),
		Cidade:      strings.TrimSpace(d.Cidade),
		UF:          strings.ToUpper(strings.TrimSpace(d.UF)),
	}
	tam := func(s string) int { return len([]rune(s)) }
	switch {
	case strings.Trim(d.CEP, "0123456789-. ") != "" || len(e.CEP) != 8:
		return Entrega{}, ErrEntregaCEPInvalido
	case e.Endereco == "":
		return Entrega{}, ErrEntregaEnderecoObrig
	case tam(e.Endereco) > 255:
		return Entrega{}, ErrEntregaEnderecoLongo
	case e.Numero == "":
		return Entrega{}, ErrEntregaNumeroObrig
	case tam(e.Numero) > 20:
		return Entrega{}, ErrEntregaNumeroLongo
	case tam(e.Complemento) > 120:
		return Entrega{}, ErrEntregaComplementoLongo
	case e.Bairro == "":
		return Entrega{}, ErrEntregaBairroObrig
	case tam(e.Bairro) > 120:
		return Entrega{}, ErrEntregaBairroLongo
	case e.Cidade == "":
		return Entrega{}, ErrEntregaCidadeObrig
	case tam(e.Cidade) > 120:
		return Entrega{}, ErrEntregaCidadeLonga
	case !slices.Contains(ufsEntrega, e.UF):
		return Entrega{}, ErrEntregaUFInvalida
	}
	return e, nil
}

func digitosCEP(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
