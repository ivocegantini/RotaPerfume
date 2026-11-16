package handlers_test

// CACHE-01: cache Redis das áreas da loja (vitrine, filtros), com armazém em
// memória: MISS → HIT sem tocar no banco, ?q fora do cache, invalidação na
// escrita do produto, HEAD não grava e fail-open com o armazém caído.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/cache"
)

type armazemMem struct {
	mu    sync.Mutex
	dados map[string][]byte
	falha error
}

func novoArmazemMem() *armazemMem { return &armazemMem{dados: map[string][]byte{}} }

func (a *armazemMem) Get(_ context.Context, k string) ([]byte, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.falha != nil {
		return nil, false, a.falha
	}
	v, ok := a.dados[k]
	return v, ok, nil
}

func (a *armazemMem) Set(_ context.Context, k string, v []byte, _ time.Duration) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.falha != nil {
		return a.falha
	}
	a.dados[k] = append([]byte(nil), v...)
	return nil
}

func (a *armazemMem) Incr(_ context.Context, k string) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.falha != nil {
		return 0, a.falha
	}
	n, _ := strconv.ParseInt(string(a.dados[k]), 10, 64)
	n++
	a.dados[k] = []byte(strconv.FormatInt(n, 10))
	return n, nil
}

func (a *armazemMem) chaves(prefixo string) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []string
	for k := range a.dados {
		if strings.HasPrefix(k, prefixo) {
			out = append(out, k)
		}
	}
	return out
}

func setupServidorComCache(t *testing.T, arm cache.Armazem) (*httptest.Server, *sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	cfg := testCfg()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	hs := bootstrap.NovosHandlersCom(db, cfg, novaIdentidadeTeste(db, cfg).casos)
	c, err := cache.Novo("rotaperfumes", arm)
	require.NoError(t, err)
	hs.Produto.ComCache(c.ComPausa(time.Hour))
	hs.ImagemPublica.ComCache(c)
	server := httptest.NewServer(bootstrap.HandlerHTTP(cfg, fakeUserStatusChecker{}, hs))
	t.Cleanup(func() { server.Close(); db.Close() })
	return server, db, mock
}

func esperarVitrine(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos WHERE ativo = \?`).WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT `+produtoColunasRegex+` FROM produtos WHERE ativo = \? ORDER BY descricao ASC LIMIT \? OFFSET \?`).
		WithArgs(true, 20, 0).WillReturnRows(produtoRowsForHandler())
}

func pedir(t *testing.T, metodo, url, tok string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(metodo, url, nil)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp, string(readBody(t, resp))
}

func TestCACHE01_Vitrine_MissDepoisHitSemBanco(t *testing.T) {
	arm := novoArmazemMem()
	server, _, mock := setupServidorComCache(t, arm)
	esperarVitrine(mock) // só UMA ida ao banco

	r1, b1 := pedir(t, "GET", server.URL+"/api/loja/produtos", "")
	assert.Equal(t, http.StatusOK, r1.StatusCode)
	assert.Equal(t, "MISS", r1.Header.Get("X-Cache"))
	r2, b2 := pedir(t, "GET", server.URL+"/api/loja/produtos", "")
	assert.Equal(t, http.StatusOK, r2.StatusCode)
	assert.Equal(t, "HIT", r2.Header.Get("X-Cache"))
	assert.Equal(t, b1, b2, "o HIT devolve o mesmo corpo")
	assert.Equal(t, "application/json", r2.Header.Get("Content-Type"))
	assert.NotContains(t, b2, "custo")
	assert.NoError(t, mock.ExpectationsWereMet())

	ks := arm.chaves("rotaperfumes:vitrine:v0:")
	require.Len(t, ks, 1, "chave = appname:area:versao:variante")
	assert.Regexp(t, `^rotaperfumes:vitrine:v0:[0-9a-f]{24}$`, ks[0])
}

func TestCACHE01_Vitrine_BuscaLivreNaoEntraNoCache(t *testing.T) {
	arm := novoArmazemMem()
	server, _, mock := setupServidorComCache(t, arm)
	for range 2 {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos`).WillReturnRows(sqlmock.NewRows(produtoColunasHeaderForHandler()))
	}
	for range 2 {
		r, _ := pedir(t, "GET", server.URL+"/api/loja/produtos?q=rosa", "")
		assert.Equal(t, http.StatusOK, r.StatusCode)
		assert.Empty(t, r.Header.Get("X-Cache"))
	}
	assert.Empty(t, arm.chaves("rotaperfumes:vitrine:"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCACHE01_EscritaDoProdutoInvalidaVitrineEFiltros(t *testing.T) {
	arm := novoArmazemMem()
	server, _, mock := setupServidorComCache(t, arm)
	esperarVitrine(mock)
	r, _ := pedir(t, "GET", server.URL+"/api/loja/produtos", "")
	require.Equal(t, "MISS", r.Header.Get("X-Cache"))

	mock.ExpectExec(`UPDATE produtos SET ativo = NOT ativo WHERE id = \?`).WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).WithArgs(int64(1)).
		WillReturnRows(produtoRowsAtivoForHandler(false))
	r, _ = pedir(t, "PATCH", server.URL+"/api/produtos/1/inativar", generateToken(t, testCfg(), 1, "admin"))
	require.Equal(t, http.StatusOK, r.StatusCode)

	v, _, _ := arm.Get(context.Background(), cache.ChaveVersao("rotaperfumes", cache.AreaVitrine))
	assert.Equal(t, "1", string(v))
	v, _, _ = arm.Get(context.Background(), cache.ChaveVersao("rotaperfumes", cache.AreaFiltros))
	assert.Equal(t, "1", string(v))

	esperarVitrine(mock) // a versão nova volta ao banco
	r, _ = pedir(t, "GET", server.URL+"/api/loja/produtos", "")
	assert.Equal(t, "MISS", r.Header.Get("X-Cache"))
	assert.Len(t, arm.chaves("rotaperfumes:vitrine:v1:"), 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCACHE01_HeadNaoGravaEErroNaoEntra(t *testing.T) {
	arm := novoArmazemMem()
	server, _, mock := setupServidorComCache(t, arm)
	esperarVitrine(mock)
	r, _ := pedir(t, "HEAD", server.URL+"/api/loja/produtos", "")
	assert.Equal(t, "BYPASS", r.Header.Get("X-Cache"))

	mock.ExpectQuery(`SELECT DISTINCT`).WillReturnError(errors.New("banco fora"))
	r, _ = pedir(t, "GET", server.URL+"/api/loja/filtros", "")
	assert.Equal(t, http.StatusInternalServerError, r.StatusCode)
	assert.Empty(t, arm.chaves("rotaperfumes:"), "nem HEAD nem 500 entram no cache")
}

func TestCACHE01_ArmazemCaido_FailOpen(t *testing.T) {
	arm := novoArmazemMem()
	arm.falha = errors.New("conexão recusada")
	server, _, mock := setupServidorComCache(t, arm)
	esperarVitrine(mock)
	esperarVitrine(mock)
	for range 2 {
		r, _ := pedir(t, "GET", server.URL+"/api/loja/produtos", "")
		assert.Equal(t, http.StatusOK, r.StatusCode, "Redis fora não derruba a vitrine")
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}
