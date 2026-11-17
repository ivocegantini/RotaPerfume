package comercial

import (
	"strings"
	"time"

	"github.com/rotaperfumes/shared/kernel"
)

// Cliente é o cliente do CRM. ClienteIDOrigem é a PK (AUTO_INCREMENT,
// 1:1 com o cliente_id do CSV de origem). Nasce válido por NovoCliente; a
// edição valida por ClienteEditado.
type Cliente struct {
	ClienteIDOrigem int64
	CNPJ            string // normalizado (VO CNPJ); "" = pessoa física; nunca logado (M-04)
	RazaoSocial     string
	Segmento        string
	Cidade          string
	UF              string
	Bairro          string
	DataCadastro    time.Time
	Ativo           bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CPF             string // só dígitos; "" = pessoa jurídica (ECOM-06); PII, nunca logado
	Endereco        string // logradouro (ECOM-09)
	Numero          string
	Complemento     string
	CEP             string // só dígitos; "" = sem CEP
}

// DadosCliente é a entrada crua de criação/edição (o comando).
type DadosCliente struct {
	CNPJ         string
	RazaoSocial  string
	Segmento     string
	Cidade       string
	UF           string
	Bairro       string
	DataCadastro string // AAAA-MM-DD; vazio = hoje só na criação
	CPF          string // pessoa física: CPF no lugar do CNPJ (ECOM-06)
	Endereco     string
	Numero       string
	Complemento  string
	CEP          string
}

// NovoCliente valida e cria um cliente ATIVO; data_cadastro vazia vira
// agora (o instante, como no legado).
func NovoCliente(d DadosCliente, agora time.Time) (*Cliente, error) {
	c, err := validarCliente(d, &agora)
	if err != nil {
		return nil, err
	}
	c.Ativo = true
	return c, nil
}

// ClienteEditado valida os campos editáveis de um PUT (data_cadastro é
// obrigatória; ativo e id não mudam por aqui).
func ClienteEditado(d DadosCliente) (*Cliente, error) {
	return validarCliente(d, nil)
}

// validarCliente aplica a ordem do legado: razão social, documento (CNPJ
// ou, na pessoa física, CPF), segmento, cidade, UF, endereço, data e, por
// último, o DV do CNPJ. CNPJ e CPF juntos: ErrDocumentoDuplo (ECOM-06).
func validarCliente(d DadosCliente, agora *time.Time) (*Cliente, error) {
	c := &Cliente{
		RazaoSocial: strings.TrimSpace(d.RazaoSocial),
		Segmento:    strings.TrimSpace(d.Segmento),
		Cidade:      strings.TrimSpace(d.Cidade),
		Bairro:      strings.TrimSpace(d.Bairro),
	}
	if c.RazaoSocial == "" {
		return nil, ErrRazaoSocialObrigatoria
	}
	pf, err := c.validarCPF(d)
	if err != nil {
		return nil, err
	}
	var doc CNPJ
	if !pf {
		if doc, err = NovoCNPJ(d.CNPJ); err != nil {
			return nil, err
		}
	}
	if err := c.validarLocal(d.UF); err != nil {
		return nil, err
	}
	if err := c.validarEndereco(d, pf); err != nil {
		return nil, err
	}
	if c.DataCadastro, err = dataOuAgora(d.DataCadastro, agora, ErrDataCadastroInvalida); err != nil {
		return nil, err
	}
	if !pf && !doc.DigitosValidos() {
		return nil, ErrCNPJInvalido
	}
	c.CNPJ = doc.String()
	return c, nil
}

// validarCPF: pessoa física é CPF preenchido com CNPJ vazio; os dois
// preenchidos é ErrDocumentoDuplo. Devolve pf e grava o CPF normalizado.
func (c *Cliente) validarCPF(d DadosCliente) (bool, error) {
	cpf, cnpjVazio := strings.TrimSpace(d.CPF), strings.TrimSpace(d.CNPJ) == ""
	switch {
	case cpf == "":
		return false, nil
	case !cnpjVazio:
		return false, ErrDocumentoDuplo
	}
	c.CPF = soDigitos(cpf)
	if !soMascaraNumerica(cpf) || !CPFValido(c.CPF) {
		return false, ErrCPFInvalido
	}
	return true, nil
}

// validarEndereco normaliza o endereço (opcional na PJ; rua e número
// obrigatórios na PF, como no cadastro da loja) e confere os tamanhos.
func (c *Cliente) validarEndereco(d DadosCliente, pf bool) error {
	c.Endereco, c.Numero = strings.TrimSpace(d.Endereco), strings.TrimSpace(d.Numero)
	c.Complemento, c.CEP = strings.TrimSpace(d.Complemento), soDigitos(d.CEP)
	switch {
	case pf && c.Endereco == "":
		return ErrEnderecoObrigatorio
	case len([]rune(c.Endereco)) > 255:
		return ErrEnderecoMuitoLongo
	case pf && c.Numero == "":
		return ErrNumeroObrigatorio
	case len([]rune(c.Numero)) > 20:
		return ErrNumeroMuitoLongo
	case len([]rune(c.Complemento)) > 120:
		return ErrComplementoMuitoLongo
	case len([]rune(c.Bairro)) > 120:
		return ErrBairroMuitoLongo
	case strings.TrimSpace(d.CEP) != "" && (len(c.CEP) != 8 || !soMascaraNumerica(d.CEP)):
		return ErrCEPInvalido
	}
	return nil
}

// validarLocal confere segmento, cidade e UF (normalizada).
func (c *Cliente) validarLocal(uf string) (err error) {
	switch {
	case c.Segmento == "":
		return ErrSegmentoObrigatorio
	case c.Cidade == "":
		return ErrCidadeObrigatoria
	}
	c.UF, err = normalizarUF(uf)
	return err
}

// DefinirAtivo aplica o PATCH /inativar: nil inverte o status (toggle); um
// valor o define.
func (c *Cliente) DefinirAtivo(ativo *bool) {
	if ativo == nil {
		c.Ativo = !c.Ativo
		return
	}
	c.Ativo = *ativo
}

// FiltroCliente é o query object da listagem. O escopo de carteira NÃO é
// filtro: vem obrigatório como kernel.Escopo (E-02/O1).
type FiltroCliente struct {
	UF        string
	Segmento  string
	Ativo     *bool
	Busca     string // razão social OU cnpj (LIKE; ver TermoBuscaCNPJ)
	Ordenacao kernel.Ordenacao[CampoCliente]
}

// dataOuAgora: vazio vira *agora (criação) ou errCampo (edição); senão é a
// meia-noite local da data (kernel.ParseDataLocal, BUG-01).
func dataOuAgora(s string, agora *time.Time, errCampo error) (time.Time, error) {
	if strings.TrimSpace(s) != "" {
		return kernel.ParseDataLocal(s, errCampo)
	}
	if agora == nil {
		return time.Time{}, errCampo
	}
	return *agora, nil
}
