package produtos_test

// ECOM-11: a descrição gerada no seed tem pelo menos 500 caracteres, cita as
// características do produto e é determinística (todo o CSV real).

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rotaperfumes/shared/importers/produtos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestECOM11_DescricaoGerada_TodoOCSVComMinimo500(t *testing.T) {
	path, err := produtos.ResolveCSVPath("")
	require.NoError(t, err)
	rows, invalidas, err := produtos.ReadCSVFile(path)
	require.NoError(t, err)
	require.Zero(t, invalidas)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		n := utf8.RuneCountInString(r.Descricao)
		assert.GreaterOrEqual(t, n, produtos.MinDescricaoGerada, r.SKU)
		assert.LessOrEqual(t, n, 5000, r.SKU) // limite do domínio (catalogo.MaxDescricaoProduto)
		assert.Contains(t, r.Descricao, r.Nome, r.SKU)
		assert.Contains(t, r.Descricao, r.Marca, r.SKU)
		assert.Contains(t, r.Descricao, strings.ToLower(r.NotaOlfativa), r.SKU)
		assert.Equal(t, r.Descricao, produtos.GerarDescricao(r), "determinística: %s", r.SKU)
	}
}

func TestECOM11_DescricaoGerada_CategoriaENotaDesconhecidas(t *testing.T) {
	d := produtos.GerarDescricao(produtos.Row{Nome: "X", Categoria: "Nova", Marca: "M", Unidade: "PCT 2"})
	assert.GreaterOrEqual(t, utf8.RuneCountInString(d), produtos.MinDescricaoGerada)
	assert.Contains(t, d, "Embalagem de venda: PCT 2.")
}
