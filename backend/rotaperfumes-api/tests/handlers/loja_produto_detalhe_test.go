package handlers_test

// ECOM-11: GET /api/loja/produtos/{id} (pública): produto ativo com a
// descrição completa e sem custo; inativo e inexistente dão o mesmo 404.

import (
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ecom11QueryPorID = `SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`

func ecom11Get(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url) // sem token: a rota é pública
	require.NoError(t, err)
	return resp
}

func TestECOM11_ProdutoLoja_AtivoComDescricaoESemCusto(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	agora := time.Now()
	mock.ExpectQuery(ecom11QueryPorID).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows(produtoColunasHeaderForHandler()).
		AddRow(int64(1), "SKU-001", "Perfume Teste", "Texto completo do perfume.", "Perfumaria", "Marca X", "Cítrico",
			"99.90", "45.00", "UN", nil, true, "", agora, agora))

	resp := ecom11Get(t, server.URL+"/api/loja/produtos/1")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	raw := readBody(t, resp)
	assert.NotContains(t, string(raw), "custo")
	assert.NotContains(t, string(raw), "ativo")
	data := como[map[string]any](t, decodeResponse(t, raw)["data"])
	assert.Equal(t, "Perfume Teste", data["nome"])
	assert.Equal(t, "Texto completo do perfume.", data["descricao"])
	assert.Equal(t, 99.9, data["preco"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM11_ProdutoLoja_InativoEInexistenteDao404(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	mock.ExpectQuery(ecom11QueryPorID).WithArgs(int64(1)).WillReturnRows(produtoRowsAtivoForHandler(false))
	mock.ExpectQuery(ecom11QueryPorID).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows(produtoColunasHeaderForHandler()))

	var corpos []string
	for _, id := range []string{"1", "2"} {
		resp := ecom11Get(t, server.URL+"/api/loja/produtos/"+id)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, id)
		corpos = append(corpos, string(readBody(t, resp)))
		resp.Body.Close()
	}
	assert.Equal(t, corpos[0], corpos[1], "inativo e inexistente com a mesma resposta")
	assert.Contains(t, corpos[0], "produto não encontrado")
	assert.NoError(t, mock.ExpectationsWereMet())
}
