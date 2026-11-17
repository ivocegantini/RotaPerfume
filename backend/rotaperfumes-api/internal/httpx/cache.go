package httpx

import (
	"bytes"
	"net/http"

	"github.com/rotaperfumes/shared/cache"
)

// CabecalhoCache informa HIT/MISS/BYPASS do cache (CACHE-01).
const CabecalhoCache = "X-Cache"

// ServirComCache (CACHE-01) responde a área do cache ou roda gerar e guarda
// a resposta. Só GET e só 200 entram (HEAD, erros e 304 passam direto);
// c nil (cache desligado) só chama gerar.
func ServirComCache(w http.ResponseWriter, r *http.Request, c *cache.Cache, area cache.Area, variante string, gerar func(http.ResponseWriter)) {
	if c == nil {
		gerar(w)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set(CabecalhoCache, "BYPASS")
		gerar(w)
		return
	}
	e, chave, ok := c.Ler(r.Context(), area, variante)
	if ok {
		w.Header().Set("Content-Type", e.ContentType)
		w.Header().Set(CabecalhoCache, "HIT")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(e.Corpo)
		return
	}
	w.Header().Set(CabecalhoCache, "MISS")
	d, _ := cache.DefinicaoDe(area)
	g := &gravadorCache{ResponseWriter: w, max: d.MaxBytes}
	gerar(g)
	if chave != "" && g.status == http.StatusOK && !g.estourou {
		c.Gravar(r.Context(), chave, area, cache.Entrada{ContentType: w.Header().Get("Content-Type"), Corpo: g.buf.Bytes()})
	}
}

// gravadorCache repassa a resposta e guarda uma cópia (até max bytes).
type gravadorCache struct {
	http.ResponseWriter
	status   int
	buf      bytes.Buffer
	max      int
	estourou bool
}

func (g *gravadorCache) WriteHeader(s int) {
	if g.status == 0 {
		g.status = s
	}
	g.ResponseWriter.WriteHeader(s)
}

func (g *gravadorCache) Write(b []byte) (int, error) {
	if g.status == 0 {
		g.status = http.StatusOK
	}
	if !g.estourou {
		if g.buf.Len()+len(b) > g.max {
			g.estourou, g.buf = true, bytes.Buffer{}
		} else {
			g.buf.Write(b)
		}
	}
	return g.ResponseWriter.Write(b)
}
