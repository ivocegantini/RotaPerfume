package handlers_test

// ECOM-01: vitrine GET /api/loja/produtos (só ativos, sem custo) e
// autocadastro POST /api/auth/cadastro (role "usuario", sessão aberta).

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/adapters/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idUsuarioLoja é o id que o fakeUserStatusChecker devolve com role "usuario".
const idUsuarioLoja int64 = 900

func getLoja(t *testing.T, url, tok string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	return resp
}

func TestECOM01_Loja_SoAtivosESemCusto(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos WHERE ativo = \?`).WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT `+produtoColunasRegex+` FROM produtos WHERE ativo = \? ORDER BY descricao ASC LIMIT \? OFFSET \?`).
		WithArgs(true, 20, 0).WillReturnRows(produtoRowsForHandler())

	// ?ativo=false e order_by=custo_unitario são ignorados na vitrine.
	resp := getLoja(t, server.URL+"/api/loja/produtos?ativo=false&order_by=custo_unitario", tok)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	raw := readBody(t, resp)
	assert.NotContains(t, string(raw), "custo")
	data := como[[]any](t, decodeResponse(t, raw)["data"])
	require.Len(t, data, 1)
	item := como[map[string]any](t, data[0])
	assert.Equal(t, "Perfume Teste", item["descricao"])
	assert.Contains(t, item, "preco")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM01_UsuarioLoja_BarradoNasRotasInternas(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")
	for _, rota := range []string{"/api/produtos", "/api/pedidos", "/api/clientes", "/api/dashboard/metrics", "/api/usuarios"} {
		resp := getLoja(t, server.URL+rota, tok)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, rota)
		assert.Contains(t, string(readBody(t, resp)), "equipe interna", rota)
		resp.Body.Close()
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-03: o cadastro cria a conta PENDENTE (mesma tx: INSERT, marca e
// link), envia o link e não abre sessão (sem cookies).
func TestECOM03_Cadastro_ContaPendenteSemSessao(t *testing.T) {
	server, db, mock, ident := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO usuarios`).
		WithArgs("Ana", "ana@loja.com", sqlmock.AnyArg(), "usuario", nil, true, false).
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectExec(`UPDATE usuarios SET email_confirmacao_pendente = \? WHERE id = \?`).
		WithArgs(true, int64(42)).WillReturnResult(sqlmock.NewResult(0, 1))
	// ECOM-05: o cliente pessoa física nasce ligado ao login, na mesma tx.
	mock.ExpectExec(`INSERT INTO clientes \(cnpj, cpf, razao_social, segmento, cidade, uf, bairro, endereco, numero, complemento, cep, data_cadastro, ativo, usuario_id\)`).
		WithArgs("52998224725", "Ana", "Loja Online", "São Paulo", "SP", "Sé", "Praça da Sé", "10", nil, "01001000", sqlmock.AnyArg(), int64(42)).
		WillReturnResult(sqlmock.NewResult(5001, 1))
	mock.ExpectExec(`INSERT INTO confirmacoes_email \(usuario_id, token_hash, expira_em\)`).
		WithArgs(int64(42), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectCommit()

	resp, err := http.Post(server.URL+"/api/auth/cadastro", "application/json", makeJSON(cadastroBody(map[string]string{
		"nome": "Ana", "email": "Ana@Loja.com", "password": "Loja#2026ok", "captchaToken": "tok", "role": "admin",
	})))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	data := como[map[string]any](t, decodeResponse(t, readBody(t, resp))["data"])
	assert.Equal(t, true, data["email_enviado"])
	assert.Equal(t, "ana@loja.com", data["email"])
	assert.Empty(t, resp.Cookies(), "o cadastro não abre sessão antes da confirmação")
	require.Len(t, ident.email.links, 1)
	assert.Regexp(t, `^http://app\.teste/confirmar-email#token=[0-9a-f]{64}$`, ident.email.links[0],
		"token no fragmento, base APP_URL")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-03: login do "usuario" com e-mail pendente → 403 (só com a senha
// certa), sem emitir sessão.
// cadastroBody completa o corpo do cadastro com os dados do cliente (ECOM-05).
func cadastroBody(b map[string]string) map[string]string {
	base := map[string]string{"cpf": "529.982.247-25", "endereco": "Praça da Sé", "numero": "10", "bairro": "Sé",
		"cep": "01001-000", "cidade": "São Paulo", "uf": "sp"}
	for k, v := range b {
		base[k] = v
	}
	return base
}

// ECOM-05: CPF repetido desfaz a tx inteira (sem usuário órfão) e responde a
// recusa neutra, sem e-mail.
func TestECOM05_Cadastro_CPFRepetido_Rollback409(t *testing.T) {
	server, db, mock, ident := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO usuarios`).WillReturnResult(sqlmock.NewResult(43, 1))
	mock.ExpectExec(`UPDATE usuarios SET email_confirmacao_pendente`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO clientes`).WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry '52998224725' for key 'uq_clientes_cpf'"})
	mock.ExpectRollback()

	resp, err := http.Post(server.URL+"/api/auth/cadastro", "application/json", makeJSON(cadastroBody(map[string]string{
		"nome": "Bia", "email": "bia@loja.com", "password": "Loja#2026ok", "captchaToken": "tok",
	})))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	corpo := string(readBody(t, resp))
	assert.Contains(t, corpo, "não foi possível concluir o cadastro")
	assert.NotContains(t, corpo, "52998224725")
	assert.Empty(t, ident.email.links)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-05: CPF inválido é recusado antes de qualquer escrita.
func TestECOM05_Cadastro_CPFInvalido_400SemEscrita(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	resp, err := http.Post(server.URL+"/api/auth/cadastro", "application/json", makeJSON(cadastroBody(map[string]string{
		"nome": "Bia", "email": "bia@loja.com", "password": "Loja#2026ok", "captchaToken": "tok", "cpf": "111.111.111-11",
	})))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, string(readBody(t, resp)), "cpf inválido")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM03_Login_EmailPendente_403(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	hash, err := gerarHashSenha(testCfg().HashSenha, "Loja#2026ok")
	require.NoError(t, err)
	mock.ExpectQuery(`WHERE\s+u\.email\s+=\s+\?`).WithArgs("ana@loja.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(int64(42), "Ana", "ana@loja.com", hash, "usuario", nil, true, false, time.Now(), time.Now(), nil, nil))
	mock.ExpectQuery(`SELECT email_confirmacao_pendente FROM usuarios WHERE id = \?`).WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"p"}).AddRow(true))

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(map[string]string{"email": "ana@loja.com", "password": "Loja#2026ok", "captchaToken": "tok"}))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Contains(t, string(readBody(t, resp)), "email não confirmado")
	assert.Empty(t, resp.Cookies())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ECOM-03: link válido marca o uso (condicional) e libera o login, na tx.
func TestECOM03_ConfirmarEmail_LinkValido(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	mock.ExpectQuery(`FROM confirmacoes_email WHERE token_hash = \?`).WithArgs(sha256Hex("tok-abc")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "usuario_id", "token_hash", "expira_em", "usado_em"}).
			AddRow(int64(7), int64(42), sha256Hex("tok-abc"), time.Now().Add(time.Hour), nil))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE confirmacoes_email SET usado_em = \? WHERE id = \? AND usado_em IS NULL`).
		WithArgs(sqlmock.AnyArg(), int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE usuarios SET email_confirmacao_pendente = \?`).
		WithArgs(false, int64(42)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp, err := http.Post(server.URL+"/api/auth/confirmar-email", "application/json", makeJSON(map[string]string{"token": "tok-abc"}))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-03: link expirado, já usado ou inexistente → 400 sem escrita.
func TestECOM03_ConfirmarEmail_LinkInvalido_400(t *testing.T) {
	usado := time.Now().Add(-time.Minute)
	casos := []struct {
		nome string
		rows *sqlmock.Rows
	}{
		{"expirado", sqlmock.NewRows([]string{"id", "usuario_id", "token_hash", "expira_em", "usado_em"}).
			AddRow(int64(7), int64(42), "h", time.Now().Add(-time.Hour), nil)},
		{"usado", sqlmock.NewRows([]string{"id", "usuario_id", "token_hash", "expira_em", "usado_em"}).
			AddRow(int64(7), int64(42), "h", time.Now().Add(time.Hour), usado)},
		{"inexistente", sqlmock.NewRows([]string{"id", "usuario_id", "token_hash", "expira_em", "usado_em"})},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			mock.ExpectQuery(`FROM confirmacoes_email WHERE token_hash = \?`).WillReturnRows(c.rows)
			resp, err := http.Post(server.URL+"/api/auth/confirmar-email", "application/json", makeJSON(map[string]string{"token": "x"}))
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Contains(t, string(readBody(t, resp)), "inválido ou expirado")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ECOM-03: reenvio para e-mail sem conta responde 200 neutro e não envia.
func TestECOM03_Reenviar_SemConta_Neutro(t *testing.T) {
	server, db, mock, ident := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()
	mock.ExpectQuery(`WHERE\s+u\.email\s+=\s+\?`).WithArgs("ninguem@x.com").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	resp, err := http.Post(server.URL+"/api/auth/reenviar-confirmacao", "application/json",
		makeJSON(map[string]string{"email": "Ninguem@x.com", "captchaToken": "tok"}))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(readBody(t, resp)), "se houver um cadastro pendente")
	assert.Empty(t, ident.email.links)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-03: reenvio para conta pendente invalida os links anteriores, grava
// um novo e envia.
func TestECOM03_Reenviar_ContaPendente_NovoLink(t *testing.T) {
	server, db, mock, ident := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()
	mock.ExpectQuery(`WHERE\s+u\.email\s+=\s+\?`).WithArgs("ana@loja.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(int64(42), "Ana", "ana@loja.com", "h", "usuario", nil, true, false, time.Now(), time.Now(), nil, nil))
	mock.ExpectQuery(`SELECT email_confirmacao_pendente FROM usuarios`).WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"p"}).AddRow(true))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE confirmacoes_email SET usado_em = \? WHERE usuario_id = \? AND usado_em IS NULL`).
		WithArgs(sqlmock.AnyArg(), int64(42)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO confirmacoes_email`).WillReturnResult(sqlmock.NewResult(8, 1))
	mock.ExpectCommit()

	resp, err := http.Post(server.URL+"/api/auth/reenviar-confirmacao", "application/json",
		makeJSON(map[string]string{"email": "ana@loja.com", "captchaToken": "tok"}))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, ident.email.links, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestECOM01_Cadastro_SenhaFraca_400SemInsert(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	resp, err := http.Post(server.URL+"/api/auth/cadastro", "application/json", makeJSON(map[string]string{
		"nome": "Ana", "email": "ana@loja.com", "password": "fraca", "captchaToken": "tok",
	}))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-04: filtro por nota olfativa (junto com categoria e marca) na vitrine.
func TestECOM04_Loja_FiltraCategoriaMarcaENota(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos WHERE categoria = \? AND marca = \? AND nota_olfativa = \? AND ativo = \?`).
		WithArgs("Bakhoor", "Marca X", "Oud", true).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`FROM produtos WHERE categoria = \? AND marca = \? AND nota_olfativa = \? AND ativo = \? ORDER BY descricao ASC LIMIT`).
		WithArgs("Bakhoor", "Marca X", "Oud", true, 20, 0).WillReturnRows(produtoRowsForHandler())

	resp := getLoja(t, server.URL+"/api/loja/produtos?categoria=Bakhoor&marca=Marca+X&nota_olfativa=Oud", tok)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-04: opções dos filtros (distintos dos ativos); vazio sai como [].
func TestECOM04_Loja_OpcoesDosFiltros(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), idUsuarioLoja, "usuario")

	mock.ExpectQuery(`SELECT DISTINCT categoria FROM produtos WHERE ativo = 1`).
		WillReturnRows(sqlmock.NewRows([]string{"categoria"}).AddRow("Bakhoor").AddRow("Incenso"))
	mock.ExpectQuery(`SELECT DISTINCT marca FROM produtos WHERE ativo = 1`).
		WillReturnRows(sqlmock.NewRows([]string{"marca"}).AddRow("Marca X"))
	mock.ExpectQuery(`SELECT DISTINCT nota_olfativa FROM produtos WHERE ativo = 1`).
		WillReturnRows(sqlmock.NewRows([]string{"nota_olfativa"}))

	resp := getLoja(t, server.URL+"/api/loja/filtros", tok)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := como[map[string]any](t, decodeResponse(t, readBody(t, resp))["data"])
	assert.Equal(t, []any{"Bakhoor", "Incenso"}, data["categorias"])
	assert.Equal(t, []any{"Marca X"}, data["marcas"])
	assert.Equal(t, []any{}, data["notas_olfativas"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-07: e-mail fora dos grandes provedores é recusado antes de gravar.
func TestECOM07_Cadastro_ProvedorNaoPermitido_400(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	resp, err := http.Post(server.URL+"/api/auth/cadastro", "application/json", makeJSON(cadastroBody(map[string]string{
		"nome": "Bia", "email": "bia@empresa.com.br", "password": "Loja#2026ok", "captchaToken": "tok",
	})))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, string(readBody(t, resp)), "provedor conhecido")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-07: a listagem do admin traz o status; só consulta pendentes quando
// a página tem cliente da loja.
func TestECOM07_ListaUsuarios_StatusPendenteEmail(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	cols := []string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}
	agora := time.Now()
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM usuarios`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(3))
	mock.ExpectQuery(`FROM usuarios u`).WillReturnRows(sqlmock.NewRows(cols).
		AddRow(int64(1), "Admin", "a@x", "h", "admin", nil, true, false, agora, agora, nil, nil).
		AddRow(int64(50), "Ana", "ana@gmail.com", "h", "usuario", nil, true, false, agora, agora, nil, nil).
		AddRow(int64(51), "Bia", "bia@gmail.com", "h", "usuario", nil, false, false, agora, agora, nil, nil))
	mock.ExpectQuery(`SELECT id FROM usuarios WHERE email_confirmacao_pendente = 1 AND id IN \(\?,\?\)`).
		WithArgs(int64(50), int64(51)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(50)).AddRow(int64(51)))

	resp := getLoja(t, server.URL+"/api/usuarios", generateToken(t, testCfg(), 1, "admin"))
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := como[[]any](t, decodeResponse(t, readBody(t, resp))["data"])
	require.Len(t, data, 3)
	status := []any{}
	for _, d := range data {
		status = append(status, como[map[string]any](t, d)["status"])
	}
	assert.Equal(t, []any{"ativo", "pendente_email", "inativo"}, status, "inativo prevalece sobre pendente")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-08: o admin não reseta a senha de conta com o e-mail pendente (409,
// nada gravado, nenhum e-mail).
func TestECOM08_AdminReset_EmailPendente_409(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	cols := []string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}
	linha := func() *sqlmock.Rows {
		return sqlmock.NewRows(cols).AddRow(int64(50), "Ana", "ana@gmail.com", "h", "usuario", nil, true, false, time.Now(), time.Now(), nil, nil)
	}
	mock.ExpectQuery(`WHERE\s+u\.id\s+=\s+\?`).WithArgs(int64(50)).WillReturnRows(linha())
	mock.ExpectQuery(`WHERE\s+u\.id\s+=\s+\?`).WithArgs(int64(50)).WillReturnRows(linha())
	mock.ExpectQuery(`SELECT email_confirmacao_pendente FROM usuarios WHERE id = \?`).WithArgs(int64(50)).
		WillReturnRows(sqlmock.NewRows([]string{"p"}).AddRow(true))

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/admin/reset-password", makeJSON(map[string]int64{"usuario_id": 50}))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Contains(t, string(readBody(t, resp)), "email pendente de confirmação")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-02: sem GOOGLE_CLIENT_ID as rotas do Google respondem "não
// configurado" (403) sem tocar o banco.
func TestECOM02_Google_SemClientID_403(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	for _, rota := range []string{"/api/auth/google", "/api/auth/google/cadastro"} {
		resp, err := http.Post(server.URL+rota, "application/json", makeJSON(cadastroBody(map[string]string{"credential": "x.y.z"})))
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, rota)
		assert.Contains(t, string(readBody(t, resp)), "não está configurado", rota)
		resp.Body.Close()
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-09: endereço sem CEP (e sem bairro/complemento) grava NULL nesses campos.
func TestECOM09_Cadastro_SemCEP_GravaNulo(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO usuarios`).WillReturnResult(sqlmock.NewResult(44, 1))
	mock.ExpectExec(`UPDATE usuarios SET email_confirmacao_pendente`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO clientes`).
		WithArgs("52998224725", "Caio", "Loja Online", "Zona Rural", "MG", nil, "Estrada do Sítio", "s/n", nil, nil, sqlmock.AnyArg(), int64(44)).
		WillReturnResult(sqlmock.NewResult(5002, 1))
	mock.ExpectExec(`INSERT INTO confirmacoes_email`).WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectCommit()

	resp, err := http.Post(server.URL+"/api/auth/cadastro", "application/json", makeJSON(map[string]string{
		"nome": "Caio", "email": "caio@gmail.com", "password": "Loja#2026ok", "captchaToken": "tok",
		"cpf": "52998224725", "endereco": "Estrada do Sítio", "numero": "s/n", "cep": "", "bairro": "",
		"cidade": "Zona Rural", "uf": "MG",
	}))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-10: a vitrine é pública (página inicial): sem token responde 200.
func TestECOM10_Vitrine_SemLogin_200(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos WHERE ativo = \?`).WithArgs(true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`FROM produtos WHERE ativo = \? ORDER BY descricao ASC`).WithArgs(true, 20, 0).
		WillReturnRows(produtoRowsForHandler())
	resp, err := http.Get(server.URL + "/api/loja/produtos")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotContains(t, string(readBody(t, resp)), "custo")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ECOM-10: acima do limite por IP, 429 com Retry-After e sem consultar o banco.
func TestECOM10_Vitrine_LimitePorIP_429(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	h := bootstrap.NovoCatalogoHTTPCom(bootstrap.NovoCatalogo(db, nil), nil).Produto.
		ComLimiteLoja(ratelimit.Nova(2, time.Minute, time.Minute), false)
	for i := 0; i < 2; i++ {
		mock.ExpectQuery(`SELECT COUNT`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
		mock.ExpectQuery(`FROM produtos`).WillReturnRows(sqlmock.NewRows(produtoColunasHeaderForHandler()))
	}
	chamar := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/loja/produtos", nil)
		req.RemoteAddr = "203.0.113.7:5000"
		w := httptest.NewRecorder()
		h.ListProdutosLoja(w, req)
		return w
	}
	assert.Equal(t, http.StatusOK, chamar().Code)
	assert.Equal(t, http.StatusOK, chamar().Code)
	w := chamar()
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
	assert.NoError(t, mock.ExpectationsWereMet(), "a 3ª não chega ao banco")
}
