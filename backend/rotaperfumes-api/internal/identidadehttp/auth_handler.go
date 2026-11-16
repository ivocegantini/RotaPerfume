package identidadehttp

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/kernel"
)

// AuthHandler trata /api/auth/* (login, refresh e logout públicos; me e
// reset-password autenticados). Nome do arquivo e do tipo mantidos: são o
// rótulo do vlog de rota exigido pelo TST-05.
type AuthHandler struct {
	casos        *identidadeapp.Casos
	ttlAcesso    time.Duration
	confiarProxy bool
	cookieSeguro bool
}

// NovoAuthHandler exige os casos de uso (nil: panic na montagem). ttlAcesso
// é o TTL do JWT (cookie e expires_in); confiarProxy é TRUST_PROXY_HEADERS;
// cookieSeguro é COOKIE_SECURE (atributo Secure dos cookies, SEC-21).
func NovoAuthHandler(c *identidadeapp.Casos, ttlAcesso time.Duration, confiarProxy, cookieSeguro bool) *AuthHandler {
	if c == nil {
		panic("identidadehttp: NovoAuthHandler exige os casos de uso")
	}
	return &AuthHandler{casos: c, ttlAcesso: ttlAcesso, confiarProxy: confiarProxy, cookieSeguro: cookieSeguro}
}

// Login POST /api/auth/login → tokens em cookies HttpOnly, dados no corpo.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	const tag = "[auth] Login"
	var req loginRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	s, err := h.casos.Login(r.Context(), identidadeapp.ComandoLogin{
		Email: req.Email, Senha: identidade.NovaSenha(req.Password), Captcha: req.CaptchaToken, Origem: h.origem(r),
	})
	if err != nil {
		responderFalha(w, tag, err)
		return
	}
	httpx.DefinirCookiesSessao(w, s.Acesso, s.Refresh.Revelar(), h.ttlAcesso, h.cookieSeguro)
	httpx.JSON(w, http.StatusOK, novoLoginResposta(s, h.ttlAcesso), "")
}

// Cadastro POST /api/auth/cadastro (público, ECOM-01/03): cria o cliente da
// loja (role "usuario") com o e-mail pendente e envia o link de
// confirmação → 201, SEM sessão (o login fica bloqueado até confirmar).
func (h *AuthHandler) Cadastro(w http.ResponseWriter, r *http.Request) {
	const tag = "[auth] Cadastro"
	var req cadastroRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	u, enviado, err := h.casos.Cadastrar(r.Context(), identidadeapp.ComandoCadastro{
		Nome: req.Nome, Email: req.Email, Senha: identidade.NovaSenha(req.Password), Captcha: req.CaptchaToken, Origem: h.origem(r),
		Cliente: identidadeapp.DadosClienteLoja{CPF: req.CPF, Endereco: req.Endereco, Numero: req.Numero, Complemento: req.Complemento,
			Bairro: req.Bairro, CEP: req.CEP, Cidade: req.Cidade, UF: req.UF},
	})
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	msg := "conta criada: enviamos um link de confirmação para o seu email"
	if !enviado {
		msg = "conta criada, mas não foi possível enviar o email agora: use \"reenviar confirmação\""
	}
	httpx.JSON(w, http.StatusCreated, cadastroResposta{Email: u.Email, EmailEnviado: enviado, Mensagem: msg}, "")
}

// Google POST /api/auth/google (público, ECOM-02): com conta da loja, abre
// a sessão (cookies, resposta do login); sem conta, 200 com
// cadastro_necessario e o perfil para completar o cadastro.
func (h *AuthHandler) Google(w http.ResponseWriter, r *http.Request) {
	const tag = "[auth] Google"
	var req googleRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	res, err := h.casos.EntrarComGoogle(r.Context(), req.Credential, h.origem(r))
	if err != nil {
		responderFalha(w, tag, err)
		return
	}
	if res.CadastroNecessario {
		httpx.JSON(w, http.StatusOK, googleCadastroNecessario{CadastroNecessario: true, Email: res.Perfil.Email, Nome: res.Perfil.Nome}, "")
		return
	}
	httpx.DefinirCookiesSessao(w, res.Sessao.Acesso, res.Sessao.Refresh.Revelar(), h.ttlAcesso, h.cookieSeguro)
	httpx.JSON(w, http.StatusOK, novoLoginResposta(res.Sessao, h.ttlAcesso), "")
}

// GoogleCadastro POST /api/auth/google/cadastro (público, ECOM-02): cria o
// cliente da loja com os dados do Google + CPF/endereço e abre a sessão → 201.
func (h *AuthHandler) GoogleCadastro(w http.ResponseWriter, r *http.Request) {
	const tag = "[auth] GoogleCadastro"
	var req googleCadastroRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	s, err := h.casos.CadastrarComGoogle(r.Context(), req.Credential, identidadeapp.DadosClienteLoja{
		CPF: req.CPF, Endereco: req.Endereco, Numero: req.Numero, Complemento: req.Complemento,
		Bairro: req.Bairro, CEP: req.CEP, Cidade: req.Cidade, UF: req.UF,
	}, h.origem(r))
	if err != nil {
		responderFalha(w, tag, err)
		return
	}
	httpx.DefinirCookiesSessao(w, s.Acesso, s.Refresh.Revelar(), h.ttlAcesso, h.cookieSeguro)
	httpx.JSON(w, http.StatusCreated, novoLoginResposta(s, h.ttlAcesso), "")
}

// ConfirmarEmail POST /api/auth/confirmar-email (público, ECOM-03): token
// no corpo (o link leva no fragmento #, lido pelo frontend).
func (h *AuthHandler) ConfirmarEmail(w http.ResponseWriter, r *http.Request) {
	const tag = "[auth] ConfirmarEmail"
	var req confirmarEmailRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	if err := h.casos.ConfirmarEmail(r.Context(), req.Token, h.origem(r)); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	httpx.JSON(w, http.StatusOK, mensagemResposta{Mensagem: "email confirmado: você já pode entrar"}, "")
}

// ReenviarConfirmacao POST /api/auth/reenviar-confirmacao (público,
// ECOM-03): resposta neutra (não revela se há conta pendente).
func (h *AuthHandler) ReenviarConfirmacao(w http.ResponseWriter, r *http.Request) {
	const tag = "[auth] ReenviarConfirmacao"
	var req reenviarConfirmacaoRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	err := h.casos.ReenviarConfirmacao(r.Context(), identidadeapp.ComandoReenvio{Email: req.Email, Captcha: req.CaptchaToken, Origem: h.origem(r)})
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	httpx.JSON(w, http.StatusOK, mensagemResposta{Mensagem: "se houver um cadastro pendente para este email, enviamos um novo link"}, "")
}

// Refresh POST /api/auth/refresh: refresh_token no body ou no cookie;
// rotação com tokens novos em cookies.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	s, err := h.casos.RenovarSessao(r.Context(), refreshDoRequest(r, httpx.NomesCookiesPara(h.cookieSeguro)), h.origem(r))
	if err != nil {
		responderFalha(w, "[auth] Refresh", err)
		return
	}
	httpx.DefinirCookiesSessao(w, s.Acesso, s.Refresh.Revelar(), h.ttlAcesso, h.cookieSeguro)
	httpx.JSON(w, http.StatusOK, refreshResposta{ExpiresIn: int64(h.ttlAcesso.Seconds()), TokenType: tokenType}, "")
}

// Logout POST /api/auth/logout: revoga o refresh informado e limpa os
// cookies; sempre 200 (idempotente).
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.casos.Logout(r.Context(), refreshDoRequest(r, httpx.NomesCookiesPara(h.cookieSeguro)))
	httpx.LimparCookiesSessao(w, h.cookieSeguro)
	log.Printf("[auth] logout OK")
	httpx.JSON(w, http.StatusOK, mensagemResposta{Mensagem: "logout realizado"}, "")
}

// ResetPassword POST /api/auth/reset-password: o usuário (do Principal,
// nunca do body) troca a própria senha; limpa os cookies (novo login).
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	const tag = "[auth] ResetPassword"
	var req resetPasswordRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r) // inválido: o caso de uso falha fechado (500)
	err := h.casos.TrocarSenha(r.Context(), p, identidadeapp.ComandoTrocarSenha{
		SenhaAtual: identidade.NovaSenha(req.SenhaAtual), NovaSenha: identidade.NovaSenha(req.NovaSenha),
		Captcha: req.CaptchaToken, Origem: h.origem(r),
	})
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	httpx.LimparCookiesSessao(w, h.cookieSeguro)
	httpx.JSON(w, http.StatusOK, mensagemResposta{Mensagem: "senha alterada com sucesso"}, "")
}

// Me GET /api/auth/me: dados do usuário autenticado (consulta própria).
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r)
	u, desligado, err := h.casos.ObterSessao(r.Context(), p)
	if err != nil {
		httpx.ResponderErro(w, "[auth] Me", err)
		return
	}
	httpx.JSON(w, http.StatusOK, novoMeResposta(u, desligado), "")
}

func (h *AuthHandler) origem(r *http.Request) identidadeapp.Origem {
	return identidadeapp.Origem{IP: httpx.IPDoCliente(r, h.confiarProxy), UserAgent: r.UserAgent()}
}

// refreshDoRequest: body opcional (erro de decode é ignorado, como no
// legado); sem token no body, vale o cookie.
func refreshDoRequest(r *http.Request, nomes httpx.NomesCookies) identidade.TokenOpaco {
	var req refreshRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.RefreshToken != "" {
		return identidade.NovoTokenOpaco(req.RefreshToken)
	}
	return identidade.NovoTokenOpaco(httpx.CookieRefreshDe(r, nomes))
}

// responderFalha: as duas falhas de emissão de JWT são 500 com mensagem
// própria (legado); o resto passa pelo mapper.
func responderFalha(w http.ResponseWriter, tag string, err error) {
	var de *kernel.Erro
	if errors.As(err, &de) && (de == identidade.ErrGerarToken || de == identidade.ErrGerarAccessToken) {
		httpx.JSON(w, http.StatusInternalServerError, nil, de.Error())
		return
	}
	httpx.ResponderErro(w, tag, err)
}
