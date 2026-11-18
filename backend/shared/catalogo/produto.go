package catalogo

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rotaperfumes/shared/kernel"
)

// Produto é o agregado do catálogo. Nasce válido por NovoProduto (ou é lido
// do banco pelo adapter); os campos editáveis só mudam por
// NovosCamposProduto, e o status por DefinirAtivo.
type Produto struct {
	ID  int64
	SKU SKU
	CamposProduto
	Ativo bool
	// Imagem é o nome do arquivo da imagem (NomeImagemValido); "" = sem
	// imagem. Só muda pelos casos de uso de imagem (IMG-01).
	Imagem    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MaxDescricaoProduto é o limite (em caracteres) da descrição completa
// (ECOM-11; a coluna é TEXT, 65535 bytes).
const MaxDescricaoProduto = 5000

// CamposProduto são os campos editáveis (o PUT altera todos de uma vez; SKU
// e ativo ficam de fora).
type CamposProduto struct {
	Nome string
	// Descricao é o texto completo da página do produto (ECOM-11); "" grava NULL.
	Descricao     string
	Categoria     string
	Marca         string
	NotaOlfativa  string // "" grava NULL
	PrecoTabela   kernel.Dinheiro
	CustoUnitario kernel.Dinheiro
	Unidade       string
	// DataLancamento é a meia-noite em time.Local (BUG-01); nil = sem data.
	DataLancamento *time.Time
}

// DadosProduto é a entrada crua de criação/edição (o comando).
type DadosProduto struct {
	SKU            string // ignorado na edição
	Nome           string
	Descricao      string
	Categoria      string
	Marca          string
	NotaOlfativa   string
	PrecoTabela    kernel.Decimal
	CustoUnitario  kernel.Decimal
	Unidade        string
	DataLancamento string // AAAA-MM-DD, opcional
}

// NovoProduto valida e cria um produto ATIVO (regra de negócio: todo produto
// nasce ativo, como os importados por CSV). A ordem das validações é a do
// legado: SKU, depois os campos editáveis.
func NovoProduto(d DadosProduto) (*Produto, error) {
	sku, err := NovoSKU(d.SKU)
	if err != nil {
		return nil, err
	}
	campos, err := NovosCamposProduto(d)
	if err != nil {
		return nil, err
	}
	return &Produto{SKU: sku, CamposProduto: campos, Ativo: true}, nil
}

// NovosCamposProduto apara e valida os campos editáveis, na ordem nome,
// descrição (opcional, até MaxDescricaoProduto), categoria, marca, unidade,
// preço, custo e data de lançamento (opcional).
func NovosCamposProduto(d DadosProduto) (CamposProduto, error) {
	c := CamposProduto{
		Nome:         strings.TrimSpace(d.Nome),
		Descricao:    strings.TrimSpace(d.Descricao),
		Categoria:    strings.TrimSpace(d.Categoria),
		Marca:        strings.TrimSpace(d.Marca),
		NotaOlfativa: strings.TrimSpace(d.NotaOlfativa),
		Unidade:      strings.TrimSpace(d.Unidade),
	}
	if err := c.validarObrigatorios(); err != nil {
		return CamposProduto{}, err
	}
	var err error
	if c.PrecoTabela, err = d.PrecoTabela.Dinheiro(ErrPrecoTabelaInvalido, ErrPrecoTabelaCasas); err != nil {
		return CamposProduto{}, err
	}
	if c.CustoUnitario, err = d.CustoUnitario.Dinheiro(ErrCustoUnitarioInvalido, ErrCustoUnitarioCasas); err != nil {
		return CamposProduto{}, err
	}
	if strings.TrimSpace(d.DataLancamento) != "" {
		data, err := kernel.ParseDataLocal(d.DataLancamento, ErrDataLancamentoInvalida)
		if err != nil {
			return CamposProduto{}, err
		}
		c.DataLancamento = &data
	}
	return c, nil
}

func (c CamposProduto) validarObrigatorios() error {
	switch {
	case c.Nome == "":
		return ErrNomeObrigatorio
	case utf8.RuneCountInString(c.Descricao) > MaxDescricaoProduto:
		return ErrDescricaoMuitoLonga
	case c.Categoria == "":
		return ErrCategoriaObrigatoria
	case c.Marca == "":
		return ErrMarcaObrigatoria
	case c.Unidade == "":
		return ErrUnidadeObrigatoria
	default:
		return nil
	}
}

// DefinirAtivo aplica o PATCH /inativar: nil inverte o status (toggle); um
// valor o define.
func (p *Produto) DefinirAtivo(ativo *bool) {
	if ativo == nil {
		p.Ativo = !p.Ativo
		return
	}
	p.Ativo = *ativo
}

// FiltroProduto é o query object da listagem: dados que o adapter traduz em
// SQL parametrizado (C-03). Campos vazios/nil não filtram.
type FiltroProduto struct {
	Categoria string
	Marca     string
	// NotaOlfativa filtra pelo valor exato (ECOM-04, vitrine).
	NotaOlfativa string
	Ativo        *bool
	Busca        string // em nome OU sku (LIKE; %/_ literais, SEC-25)
	Ordenacao    kernel.Ordenacao[CampoProduto]
}

// OpcoesFiltroLoja são os valores oferecidos nos filtros da vitrine
// (ECOM-04): categorias, marcas e notas olfativas dos produtos ativos.
type OpcoesFiltroLoja struct {
	Categorias     []string
	Marcas         []string
	NotasOlfativas []string
}
