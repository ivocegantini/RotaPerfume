// Package googleid valida o ID token do Google Identity Services (ECOM-02):
// RS256 com as chaves públicas do Google (JWKS, em cache pelo max-age),
// emissor accounts.google.com, audiência = GOOGLE_CLIENT_ID e expiração.
// Satisfaz identidadeapp.VerificadorGoogle sem importá-lo. O token nunca é
// logado.
package googleid

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rotaperfumes/shared/identidade"
)

// URLChavesGoogle é o JWKS oficial das chaves dos ID tokens do Google.
const URLChavesGoogle = "https://www.googleapis.com/oauth2/v3/certs"

const (
	timeoutHTTP     = 5 * time.Second
	cachePadrao     = time.Hour
	cacheMinimo     = time.Minute
	tamanhoMaxToken = 8 << 10
)

var emissores = map[string]bool{"accounts.google.com": true, "https://accounts.google.com": true}

// Verificador valida ID tokens de um Client ID.
type Verificador struct {
	clientID string
	url      string
	client   *http.Client
	agora    func() time.Time

	mu     sync.Mutex
	chaves map[string]*rsa.PublicKey
	expira time.Time
}

// Novo cria o verificador do Client ID com o JWKS oficial.
func Novo(clientID string) *Verificador { return NovoComURL(clientID, URLChavesGoogle, nil) }

// NovoComURL permite trocar o JWKS e o cliente HTTP (testes).
func NovoComURL(clientID, url string, client *http.Client) *Verificador {
	if client == nil {
		client = &http.Client{Timeout: timeoutHTTP}
	}
	return &Verificador{clientID: clientID, url: url, client: client, agora: time.Now}
}

// Configurado informa se há Client ID.
func (v *Verificador) Configurado() bool { return v != nil && v.clientID != "" }

type claimsGoogle struct {
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"` // bool ou "true"
	Nome          string `json:"name"`
	jwt.RegisteredClaims
}

// Verificar valida o token e devolve o perfil. Qualquer falha de token é
// identidade.ErrGoogleTokenInvalido (o motivo fica no erro embrulhado).
func (v *Verificador) Verificar(ctx context.Context, token string) (identidade.PerfilGoogle, error) {
	if !v.Configurado() {
		return identidade.PerfilGoogle{}, identidade.ErrGoogleIndisponivel
	}
	token = strings.TrimSpace(token)
	if token == "" || len(token) > tamanhoMaxToken {
		return identidade.PerfilGoogle{}, identidade.ErrGoogleTokenInvalido
	}
	var c claimsGoogle
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithAudience(v.clientID),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
		jwt.WithTimeFunc(v.agora),
	)
	_, err := parser.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return v.chave(ctx, kid)
	})
	if err != nil {
		return identidade.PerfilGoogle{}, fmt.Errorf("%w: %v", identidade.ErrGoogleTokenInvalido, err)
	}
	if !emissores[c.Issuer] || c.Subject == "" || c.Email == "" {
		return identidade.PerfilGoogle{}, fmt.Errorf("%w: emissor/sub/email", identidade.ErrGoogleTokenInvalido)
	}
	return identidade.PerfilGoogle{
		Sub: c.Subject, Email: strings.ToLower(strings.TrimSpace(c.Email)), Nome: strings.TrimSpace(c.Nome),
		EmailVerificado: c.EmailVerified == true || c.EmailVerified == "true",
	}, nil
}

// chave devolve a chave pública do kid, recarregando o JWKS quando o cache
// expira ou o kid é novo (rotação de chaves do Google).
func (v *Verificador) chave(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if k, ok := v.chaves[kid]; ok && v.agora().Before(v.expira) {
		return k, nil
	}
	if err := v.recarregar(ctx); err != nil {
		return nil, err
	}
	if k, ok := v.chaves[kid]; ok {
		return k, nil
	}
	return nil, errors.New("googleid: kid desconhecido")
}

type jwks struct {
	Keys []struct {
		Kid, Kty, Alg, N, E string
	} `json:"keys"`
}

var reMaxAge = regexp.MustCompile(`max-age=(\d+)`)

func (v *Verificador) recarregar(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.url, nil)
	if err != nil {
		return fmt.Errorf("googleid: jwks: %w", err)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("googleid: jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("googleid: jwks: status %d", resp.StatusCode)
	}
	var j jwks
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&j); err != nil {
		return fmt.Errorf("googleid: jwks: %w", err)
	}
	chaves := map[string]*rsa.PublicKey{}
	for _, k := range j.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		pub, err := chaveRSA(k.N, k.E)
		if err != nil {
			continue
		}
		chaves[k.Kid] = pub
	}
	if len(chaves) == 0 {
		return errors.New("googleid: jwks sem chaves RSA")
	}
	ttl := cachePadrao
	if m := reMaxAge.FindStringSubmatch(resp.Header.Get("Cache-Control")); m != nil {
		if s, err := strconv.Atoi(m[1]); err == nil {
			ttl = max(time.Duration(s)*time.Second, cacheMinimo)
		}
	}
	v.chaves, v.expira = chaves, v.agora().Add(ttl)
	return nil
}

func chaveRSA(nB64, eB64 string) (*rsa.PublicKey, error) {
	n, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	e, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil || len(e) == 0 || len(e) > 4 {
		return nil, errors.New("googleid: expoente inválido")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}, nil
}
