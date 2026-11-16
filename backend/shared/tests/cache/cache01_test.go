package cache_test

// CACHE-01: chave appname:area:versao, catálogo de áreas, formato da
// entrada, invalidação por versão, limite de tamanho e fail-open com pausa.

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cache"
)

type mem struct {
	dados map[string][]byte
	falha error
	gets  int
}

func (m *mem) Get(_ context.Context, k string) ([]byte, bool, error) {
	m.gets++
	if m.falha != nil {
		return nil, false, m.falha
	}
	v, ok := m.dados[k]
	return v, ok, nil
}
func (m *mem) Set(_ context.Context, k string, v []byte, _ time.Duration) error {
	if m.falha != nil {
		return m.falha
	}
	m.dados[k] = v
	return nil
}
func (m *mem) Incr(_ context.Context, k string) (int64, error) {
	if m.falha != nil {
		return 0, m.falha
	}
	n, _ := strconv.ParseInt(string(m.dados[k]), 10, 64)
	n++
	m.dados[k] = []byte(strconv.FormatInt(n, 10))
	return n, nil
}

func novo(t *testing.T) (*cache.Cache, *mem) {
	m := &mem{dados: map[string][]byte{}}
	c, err := cache.Novo("rotaperfumes", m)
	require.NoError(t, err)
	return c, m
}

func TestCACHE01_Chave(t *testing.T) {
	assert.Equal(t, "rotaperfumes:vitrine:v3:abc", cache.Chave{AppName: "rotaperfumes", Area: cache.AreaVitrine, Versao: 3, Variante: "abc"}.String())
	assert.Equal(t, "rotaperfumes:filtros:v0", cache.Chave{AppName: "rotaperfumes", Area: cache.AreaFiltros}.String())
	assert.Equal(t, "rotaperfumes:filtros:versao", cache.ChaveVersao("rotaperfumes", cache.AreaFiltros))
	assert.Equal(t, cache.Variante("1", "20"), cache.Variante("1", "20"))
	assert.NotEqual(t, cache.Variante("1", "20"), cache.Variante("12", "0"), "separador entre partes")
	assert.Regexp(t, `^[0-9a-f]{24}$`, cache.Variante("x"))
}

func TestCACHE01_CatalogoDasAreas(t *testing.T) {
	vistos := map[cache.Area]bool{}
	for _, d := range cache.Areas {
		assert.False(t, vistos[d.Area], "área repetida %s", d.Area)
		vistos[d.Area] = true
		assert.Positive(t, d.TTL)
		assert.Positive(t, d.MaxBytes)
		assert.NotEmpty(t, d.Tela)
		assert.Contains(t, d.Rota, "GET /api/")
	}
	for _, a := range []cache.Area{cache.AreaVitrine, cache.AreaFiltros, cache.AreaImagemProduto} {
		assert.True(t, vistos[a], a)
	}
	assert.Len(t, cache.TodasAsAreas(), len(cache.Areas))
}

func TestCACHE01_AppName(t *testing.T) {
	for _, ok := range []string{"rotaperfumes", "rp-loja_2"} {
		assert.NoError(t, cache.ValidarAppName(ok))
	}
	for _, ruim := range []string{"", "Rota", "a:b", "a b", "abcdefghijabcdefghijabcdefghijabc"} {
		assert.ErrorIs(t, cache.ValidarAppName(ruim), cache.ErrAppName, ruim)
	}
	_, err := cache.Novo("a:b", &mem{})
	assert.ErrorIs(t, err, cache.ErrAppName)
	c, err := cache.Novo("x", nil)
	assert.NoError(t, err)
	assert.Nil(t, c, "armazém nil = cache desligado")
}

func TestCACHE01_Entrada_IdaEVolta(t *testing.T) {
	mod := time.Unix(1700000000, 123)
	e := cache.Entrada{ContentType: "image/png", Modificado: mod, Corpo: []byte("a\nb\x00c")}
	d, err := cache.Decodificar(e.Codificar())
	require.NoError(t, err)
	assert.Equal(t, e.ContentType, d.ContentType)
	assert.True(t, mod.Equal(d.Modificado))
	assert.Equal(t, e.Corpo, d.Corpo)
	d, err = cache.Decodificar(cache.Entrada{ContentType: "application/json"}.Codificar())
	require.NoError(t, err)
	assert.True(t, d.Modificado.IsZero())
	for _, ruim := range []string{"", "rpc1\n", "rpc1\nx\n", "rpc1\nx\nabc\n", "xxx"} {
		_, err := cache.Decodificar([]byte(ruim))
		assert.ErrorIs(t, err, cache.ErrEntradaInvalida, ruim)
	}
}

func TestCACHE01_LerGravarInvalidar(t *testing.T) {
	c, m := novo(t)
	ctx := context.Background()
	_, chave, ok := c.Ler(ctx, cache.AreaVitrine, "p1")
	assert.False(t, ok)
	assert.Equal(t, "rotaperfumes:vitrine:v0:p1", chave)
	c.Gravar(ctx, chave, cache.AreaVitrine, cache.Entrada{ContentType: "application/json", Corpo: []byte(`{}`)})
	e, _, ok := c.Ler(ctx, cache.AreaVitrine, "p1")
	require.True(t, ok)
	assert.Equal(t, `{}`, string(e.Corpo))

	c.Invalidar(ctx, cache.AreaVitrine)
	_, chave, ok = c.Ler(ctx, cache.AreaVitrine, "p1")
	assert.False(t, ok, "versão nova não vê a entrada antiga")
	assert.Equal(t, "rotaperfumes:vitrine:v1:p1", chave)

	_, chave, ok = c.Ler(ctx, cache.Area("carrinho"), "x")
	assert.False(t, ok)
	assert.Empty(t, chave, "área fora do catálogo não entra")

	m.dados[cache.ChaveVersao("rotaperfumes", cache.AreaFiltros)] = []byte("lixo")
	_, chave, _ = c.Ler(ctx, cache.AreaFiltros, "")
	assert.Empty(t, chave, "versão corrompida desliga")
}

func TestCACHE01_LimiteDeTamanho(t *testing.T) {
	c, m := novo(t)
	d, _ := cache.DefinicaoDe(cache.AreaFiltros)
	c.Gravar(context.Background(), "k", cache.AreaFiltros, cache.Entrada{Corpo: make([]byte, d.MaxBytes+1)})
	assert.Empty(t, m.dados)
}

func TestCACHE01_FailOpenComPausa(t *testing.T) {
	c, m := novo(t)
	c.ComPausa(time.Hour)
	m.falha = errors.New("fora")
	_, chave, ok := c.Ler(context.Background(), cache.AreaVitrine, "x")
	assert.False(t, ok)
	assert.Empty(t, chave)
	m.falha = nil
	antes := m.gets
	_, _, ok = c.Ler(context.Background(), cache.AreaVitrine, "x")
	assert.False(t, ok)
	assert.Equal(t, antes, m.gets, "na pausa nem consulta o armazém")

	var nulo *cache.Cache
	nulo.Invalidar(context.Background(), cache.AreaVitrine) // não entra em pânico
	_, _, ok = nulo.Ler(context.Background(), cache.AreaVitrine, "")
	assert.False(t, ok)
}
