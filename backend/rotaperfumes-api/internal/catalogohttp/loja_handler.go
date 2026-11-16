package catalogohttp

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/cache"
	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/kernel"
)

// lojaProdutoResposta é o item da vitrine (ECOM-01): só dados públicos (sem
// custo, sem ativo nem timestamps).
type lojaProdutoResposta struct {
	ID           int64           `json:"id"`
	SKU          string          `json:"sku"`
	Descricao    string          `json:"descricao"`
	Categoria    string          `json:"categoria"`
	Marca        string          `json:"marca"`
	NotaOlfativa string          `json:"nota_olfativa"`
	Preco        kernel.Dinheiro `json:"preco"`
	Unidade      string          `json:"unidade"`
	ImagemURL    *string         `json:"imagem_url"`
}

func produtoParaLoja(p catalogo.Produto) lojaProdutoResposta {
	return lojaProdutoResposta{
		ID: p.ID, SKU: p.SKU.String(), Descricao: p.Descricao, Categoria: p.Categoria, Marca: p.Marca,
		NotaOlfativa: p.NotaOlfativa, Preco: p.PrecoTabela, Unidade: p.Unidade, ImagemURL: urlImagem(p.Imagem),
	}
}

// limitadorLoja é o rate limit da vitrine (adapters/ratelimit satisfaz).
type limitadorLoja interface {
	Bloqueado(chave string) (bool, time.Duration)
	RegistrarFalha(chave string)
}

// ComLimiteLoja liga o limite por IP da vitrine pública (ECOM-10);
// confiarProxy é TRUST_PROXY_HEADERS (mesmo IP do resto da API).
func (h *ProdutoHandler) ComLimiteLoja(l limitadorLoja, confiarProxy bool) *ProdutoHandler {
	h.limiteLoja, h.confiarProxy = l, confiarProxy
	return h
}

// dentroDoLimite conta a requisição e responde 429 acima do limite.
func (h *ProdutoHandler) dentroDoLimite(w http.ResponseWriter, r *http.Request, tag string) bool {
	if h.limiteLoja == nil {
		return true
	}
	chave := "loja:" + httpx.IPDoCliente(r, h.confiarProxy)
	if bloqueado, retry := h.limiteLoja.Bloqueado(chave); bloqueado {
		log.Printf("%s: limite por IP atingido retry_after=%s", tag, retry)
		httpx.ResponderErro(w, tag, &kernel.Bloqueio{Sentinela: catalogo.ErrMuitasRequisicoesLoja, RetryAfter: retry})
		return false
	}
	h.limiteLoja.RegistrarFalha(chave)
	return true
}

// ListProdutosLoja GET /api/loja/produtos?page&limit&categoria&marca&nota_olfativa&q&order_by&order_dir
// (Public desde o ECOM-10, vitrine da página inicial): sempre só os ativos;
// ?ativo é ignorado.
func (h *ProdutoHandler) ListProdutosLoja(w http.ResponseWriter, r *http.Request) {
	const tag = "[loja] ListProdutosLoja"
	if !h.dentroDoLimite(w, r, tag) {
		return
	}
	busca, err := httpx.BuscaDaQuery(r)
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	q, ativo := r.URL.Query(), true
	filtro := catalogo.FiltroProduto{
		Categoria:    strings.TrimSpace(q.Get("categoria")),
		Marca:        strings.TrimSpace(q.Get("marca")),
		NotaOlfativa: strings.TrimSpace(q.Get("nota_olfativa")),
		Ativo:        &ativo,
		Busca:        busca,
		Ordenacao:    httpx.OrdenacaoDaQuery(r, catalogo.CamposOrdenacaoLoja(), catalogo.OrdenacaoLojaPadrao()),
	}
	pagina := httpx.PaginaDaQuery(r)
	gerar := func(w http.ResponseWriter) {
		res, err := h.casos.ListarProdutos(r.Context(), filtro, pagina)
		if err != nil {
			httpx.ResponderErro(w, tag, err)
			return
		}
		httpx.JSONPaginadoCom(w, res, produtoParaLoja)
	}
	// CACHE-01: a busca livre (?q) não entra no cache (variações sem fim
	// encheriam o Redis); a variante usa só os parâmetros já normalizados.
	if busca != "" {
		gerar(w)
		return
	}
	variante := cache.Variante(strconv.Itoa(pagina.Numero()), strconv.Itoa(pagina.Limite()),
		filtro.Categoria, filtro.Marca, filtro.NotaOlfativa, fmt.Sprintf("%v", filtro.Ordenacao))
	httpx.ServirComCache(w, r, h.cache, cache.AreaVitrine, variante, gerar)
}

// opcoesLojaResposta são as opções dos filtros da vitrine (ECOM-04); listas
// vazias saem como [] (nunca null).
type opcoesLojaResposta struct {
	Categorias     []string `json:"categorias"`
	Marcas         []string `json:"marcas"`
	NotasOlfativas []string `json:"notas_olfativas"`
}

func naoNulo(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// OpcoesFiltroLoja GET /api/loja/filtros (Public desde o ECOM-10, ECOM-04):
// categorias, marcas e notas olfativas dos produtos ativos.
func (h *ProdutoHandler) OpcoesFiltroLoja(w http.ResponseWriter, r *http.Request) {
	if !h.dentroDoLimite(w, r, "[loja] OpcoesFiltroLoja") {
		return
	}
	httpx.ServirComCache(w, r, h.cache, cache.AreaFiltros, "", func(w http.ResponseWriter) {
		o, err := h.casos.OpcoesFiltroLoja(r.Context())
		if err != nil {
			httpx.ResponderErro(w, "[loja] OpcoesFiltroLoja", err)
			return
		}
		httpx.JSON(w, http.StatusOK, opcoesLojaResposta{
			Categorias: naoNulo(o.Categorias), Marcas: naoNulo(o.Marcas), NotasOlfativas: naoNulo(o.NotasOlfativas),
		}, "")
	})
}

// ComCache liga o cache Redis das áreas da loja (CACHE-01); nil desliga.
func (h *ProdutoHandler) ComCache(c *cache.Cache) *ProdutoHandler {
	h.cache = c
	return h
}

// invalidar sobe a versão das áreas após uma escrita bem-sucedida.
func (h *ProdutoHandler) invalidar(r *http.Request, areas ...cache.Area) {
	h.cache.Invalidar(r.Context(), areas...)
}
