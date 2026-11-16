package googleid_test

// ECOM-02: validação do ID token do Google (RS256 + JWKS, iss, aud, exp).

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/adapters/googleid"
	"github.com/rotaperfumes/shared/identidade"
)

const clientID = "123-abc.apps.googleusercontent.com"

type ambiente struct {
	chave  *rsa.PrivateKey
	srv    *httptest.Server
	buscas int
}

func novoAmbiente(t *testing.T) *ambiente {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	a := &ambiente{chave: k}
	a.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		a.buscas++
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1", "kty": "RSA", "alg": "RS256",
			"n": base64.RawURLEncoding.EncodeToString(k.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.E)).Bytes()),
		}}})
	}))
	t.Cleanup(a.srv.Close)
	return a
}

func (a *ambiente) token(t *testing.T, mudar func(jwt.MapClaims), kid string, metodo jwt.SigningMethod, chave any) string {
	t.Helper()
	c := jwt.MapClaims{
		"iss": "https://accounts.google.com", "aud": clientID, "sub": "1098",
		"email": "Ana@Gmail.com", "email_verified": true, "name": "Ana Souza",
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
	}
	if mudar != nil {
		mudar(c)
	}
	tk := jwt.NewWithClaims(metodo, c)
	tk.Header["kid"] = kid
	s, err := tk.SignedString(chave)
	require.NoError(t, err)
	return s
}

func (a *ambiente) valido(t *testing.T, mudar func(jwt.MapClaims)) string {
	return a.token(t, mudar, "k1", jwt.SigningMethodRS256, a.chave)
}

func TestECOM02_Google_TokenValido(t *testing.T) {
	a := novoAmbiente(t)
	v := googleid.NovoComURL(clientID, a.srv.URL, nil)
	p, err := v.Verificar(context.Background(), a.valido(t, nil))
	require.NoError(t, err)
	assert.Equal(t, identidade.PerfilGoogle{Sub: "1098", Email: "ana@gmail.com", Nome: "Ana Souza", EmailVerificado: true}, p)
	_, err = v.Verificar(context.Background(), a.valido(t, func(c jwt.MapClaims) { c["iss"] = "accounts.google.com"; c["email_verified"] = "true" }))
	require.NoError(t, err)
	assert.Equal(t, 1, a.buscas, "JWKS em cache")
}

func TestECOM02_Google_EmailNaoVerificadoSaiNoPerfil(t *testing.T) {
	a := novoAmbiente(t)
	p, err := googleid.NovoComURL(clientID, a.srv.URL, nil).Verificar(context.Background(), a.valido(t, func(c jwt.MapClaims) { c["email_verified"] = false }))
	require.NoError(t, err)
	assert.False(t, p.EmailVerificado)
}

func TestECOM02_Google_Recusas(t *testing.T) {
	a := novoAmbiente(t)
	outra, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	casos := map[string]string{
		"vazio":              "",
		"lixo":               "a.b.c",
		"audiencia errada":   a.valido(t, func(c jwt.MapClaims) { c["aud"] = "outro-app" }),
		"emissor errado":     a.valido(t, func(c jwt.MapClaims) { c["iss"] = "https://evil.example" }),
		"expirado":           a.valido(t, func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }),
		"sem exp":            a.valido(t, func(c jwt.MapClaims) { delete(c, "exp") }),
		"sem email":          a.valido(t, func(c jwt.MapClaims) { delete(c, "email") }),
		"sem sub":            a.valido(t, func(c jwt.MapClaims) { delete(c, "sub") }),
		"assinado por outra": a.token(t, nil, "k1", jwt.SigningMethodRS256, outra),
		"kid desconhecido":   a.token(t, nil, "k9", jwt.SigningMethodRS256, a.chave),
		"HS256 com segredo":  a.token(t, nil, "k1", jwt.SigningMethodHS256, []byte("segredo-qualquer-32-bytes-......")),
	}
	v := googleid.NovoComURL(clientID, a.srv.URL, nil)
	for nome, tok := range casos {
		_, err := v.Verificar(context.Background(), tok)
		assert.ErrorIs(t, err, identidade.ErrGoogleTokenInvalido, nome)
	}
}

func TestECOM02_Google_SemClientID(t *testing.T) {
	_, err := googleid.Novo("").Verificar(context.Background(), "x")
	assert.ErrorIs(t, err, identidade.ErrGoogleIndisponivel)
}
