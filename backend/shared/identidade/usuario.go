package identidade

import (
	"strings"
	"time"

	"github.com/rotaperfumes/shared/kernel"
)

// Usuario é o aggregate de identidade. Os campos são exportados (como no
// Catálogo, ARQ-02); o hash é o VO opaco HashSenha, que não serializa.
type Usuario struct {
	ID              int64
	Nome            string
	Email           string
	Role            kernel.Role
	Hash            HashSenha
	Ativo           bool
	DeveTrocarSenha bool
	IDVendedor      *int64
	// VendedorNome vem do LEFT JOIN com vendedores (só leitura).
	VendedorNome *string
	// EmailPendente (ECOM-07): autocadastro aguardando o link do e-mail. Só
	// é preenchido pela listagem do admin (consulta própria em lote).
	EmailPendente bool
	CriadoEm      time.Time
	AtualizadoEm  time.Time
	UltimoLoginEm *time.Time
}

// Status do usuário (ECOM-07): inativo prevalece; ativo com e-mail ainda
// não confirmado é "pendente_email" (o login é recusado).
const (
	StatusAtivo         = "ativo"
	StatusInativo       = "inativo"
	StatusPendenteEmail = "pendente_email"
)

// Status devolve o status exibido ao admin.
func (u *Usuario) Status() string {
	switch {
	case !u.Ativo:
		return StatusInativo
	case u.EmailPendente:
		return StatusPendenteEmail
	default:
		return StatusAtivo
	}
}

// ProvedorEmailPermitido (ECOM-07): o domínio do e-mail (já normalizado)
// precisa estar na lista de provedores aceitos no autocadastro.
func ProvedorEmailPermitido(email string, dominios []string) bool {
	_, dominio, ok := strings.Cut(email, "@")
	if !ok {
		return false
	}
	for _, d := range dominios {
		if dominio == d {
			return true
		}
	}
	return false
}

// EhAdmin é verdadeiro só para role "admin" exato (I-05).
func (u *Usuario) EhAdmin() bool { return u.Role.EhAdmin() }

// VendedorVinculado devolve o id do vendedor quando o usuário tem vínculo
// válido (id > 0).
func (u *Usuario) VendedorVinculado() (int64, bool) {
	if u.IDVendedor == nil || *u.IDVendedor <= 0 {
		return 0, false
	}
	return *u.IDVendedor, true
}

// DadosUsuario é o comando de criação/edição de usuário (texto digitado).
type DadosUsuario struct {
	Nome       string
	Email      string // ignorado na edição
	Role       string
	IDVendedor *int64
}

// UsuarioValidado é o resultado da validação de DadosUsuario: nome e role
// sem espaços nas pontas e e-mail normalizado (trim + minúsculas).
type UsuarioValidado struct {
	Nome       string
	Email      string
	Role       kernel.Role
	IDVendedor *int64
}

// ValidarCriacao confere nome, e-mail e role, na ordem do legado: nome →
// e-mail → role. O e-mail só precisa estar preenchido (o legado não validava
// formato); vai para o banco em minúsculas.
func ValidarCriacao(d DadosUsuario) (UsuarioValidado, error) {
	v, err := validarNomeERole(d, func() error {
		if strings.TrimSpace(d.Email) == "" {
			return ErrEmailObrigatorio
		}
		return nil
	})
	if err != nil {
		return UsuarioValidado{}, err
	}
	v.Email = strings.ToLower(strings.TrimSpace(d.Email))
	return v, nil
}

// ValidarEdicao confere nome e role (o e-mail não é editável).
func ValidarEdicao(d DadosUsuario) (UsuarioValidado, error) {
	return validarNomeERole(d, func() error { return nil })
}

func validarNomeERole(d DadosUsuario, depoisDoNome func() error) (UsuarioValidado, error) {
	v := UsuarioValidado{
		Nome:       strings.TrimSpace(d.Nome),
		Role:       kernel.RoleDoBanco(strings.TrimSpace(d.Role)),
		IDVendedor: d.IDVendedor,
	}
	if v.Nome == "" {
		return UsuarioValidado{}, ErrNomeObrigatorio
	}
	if err := depoisDoNome(); err != nil {
		return UsuarioValidado{}, err
	}
	if !v.Role.Conhecido() {
		return UsuarioValidado{}, ErrRoleInvalido
	}
	if v.Role.EhUsuarioLoja() && v.IDVendedor != nil {
		return UsuarioValidado{}, ErrUsuarioLojaComVendedor
	}
	return v, nil
}

// NovoUsuario monta o usuário recém-criado: ativo e com troca de senha
// obrigatória no primeiro acesso (a senha é gerada pelo sistema).
func NovoUsuario(v UsuarioValidado, hash HashSenha) *Usuario {
	return &Usuario{
		Nome:            v.Nome,
		Email:           v.Email,
		Role:            v.Role,
		Hash:            hash,
		Ativo:           true,
		DeveTrocarSenha: true,
		IDVendedor:      v.IDVendedor,
	}
}

// Limites do cadastro público (colunas VARCHAR(120) de usuarios).
const (
	NomeMaxRunes  = 120
	EmailMaxRunes = 120
)

// ValidarCadastroLoja valida o autocadastro do cliente da loja (ECOM-01):
// nome, e-mail com formato mínimo (local@dominio.tld, sem espaços) e senha
// forte. O role é sempre "usuario", nunca vem do cliente.
func ValidarCadastroLoja(nome, email string, senha Senha) (UsuarioValidado, error) {
	v := UsuarioValidado{
		Nome:  strings.TrimSpace(nome),
		Email: strings.ToLower(strings.TrimSpace(email)),
		Role:  kernel.RoleUsuario,
	}
	switch {
	case v.Nome == "":
		return UsuarioValidado{}, ErrNomeObrigatorio
	case len([]rune(v.Nome)) > NomeMaxRunes:
		return UsuarioValidado{}, ErrNomeMuitoLongo
	case v.Email == "":
		return UsuarioValidado{}, ErrEmailObrigatorio
	case !emailPlausivel(v.Email):
		return UsuarioValidado{}, ErrEmailInvalido
	}
	if err := ValidarForca(senha); err != nil {
		return UsuarioValidado{}, err
	}
	return v, nil
}

// emailPlausivel: um @, parte local e domínio com ponto, sem espaço nem
// controle, até EmailMaxRunes. Não substitui a confirmação por e-mail.
func emailPlausivel(e string) bool {
	if len([]rune(e)) > EmailMaxRunes || strings.Count(e, "@") != 1 {
		return false
	}
	for _, r := range e {
		if r <= ' ' || r == 0x7f {
			return false
		}
	}
	local, dominio, _ := strings.Cut(e, "@")
	i := strings.LastIndex(dominio, ".")
	return local != "" && i > 0 && i < len(dominio)-1 && !strings.Contains(dominio, "..")
}

// NovoUsuarioLoja monta o cliente da loja: ativo, sem troca obrigatória (a
// senha foi escolhida por ele) e sem vendedor.
func NovoUsuarioLoja(v UsuarioValidado, hash HashSenha) *Usuario {
	return &Usuario{Nome: v.Nome, Email: v.Email, Role: kernel.RoleUsuario, Hash: hash, Ativo: true}
}

// StatusAcesso é o recorte do usuário consultado a cada request protegido
// (SEC-06/SEC-08): ativo, role vigente e corte de sessão. nil = sem corte.
// DeveTrocarSenha é a troca de senha pendente, imposta pelo Autenticador
// (SEC-23).
type StatusAcesso struct {
	Ativo              bool
	Role               kernel.Role
	TokensValidosDesde *time.Time
	DeveTrocarSenha    bool
}

// TokenAnteriorAoCorte reporta se um access token emitido em emitidoEm foi
// derrubado pelo corte de sessão (I-04): iat <= corte, em segundos (a
// precisão do iat). Sem corte, vale; com corte, token sem iat é recusado
// (fail-closed).
func (s StatusAcesso) TokenAnteriorAoCorte(emitidoEm *time.Time) bool {
	if s.TokensValidosDesde == nil {
		return false
	}
	if emitidoEm == nil {
		return true
	}
	return emitidoEm.Unix() <= s.TokensValidosDesde.Unix()
}

// ClaimsAcesso são os dados de um access token já validado (assinatura HS256
// e exp conferidos pelo adapter JWT).
type ClaimsAcesso struct {
	UserID    int64
	Role      kernel.Role
	EmitidoEm *time.Time
	ExpiraEm  *time.Time
}

// CorteDeSessao é o instante gravado em usuarios.tokens_validos_desde:
// sempre do relógio do Go (nunca NOW() do MariaDB) e truncado ao segundo, a
// precisão do iat do JWT (SEC-08).
func CorteDeSessao(agora time.Time) time.Time { return agora.Truncate(time.Second) }

// CampoUsuario é o enum de ordenação da listagem de usuários (C-02).
type CampoUsuario string

// Campos de ordenação de usuários.
const (
	OrdemUsuarioID           CampoUsuario = "id"
	OrdemUsuarioNome         CampoUsuario = "nome"
	OrdemUsuarioEmail        CampoUsuario = "email"
	OrdemUsuarioRole         CampoUsuario = "role"
	OrdemUsuarioAtivo        CampoUsuario = "ativo"
	OrdemUsuarioCriadoEm     CampoUsuario = "created_at"
	OrdemUsuarioAtualizadoEm CampoUsuario = "updated_at"
	OrdemUsuarioUltimoLogin  CampoUsuario = "ultimo_login_at"
)

// CamposOrdenacaoUsuario devolve a whitelist de campos de usuário.
func CamposOrdenacaoUsuario() []CampoUsuario {
	return []CampoUsuario{
		OrdemUsuarioID, OrdemUsuarioNome, OrdemUsuarioEmail, OrdemUsuarioRole,
		OrdemUsuarioAtivo, OrdemUsuarioCriadoEm, OrdemUsuarioAtualizadoEm, OrdemUsuarioUltimoLogin,
	}
}

// OrdenacaoUsuarioPadrao é "id ASC" (comportamento de sempre).
func OrdenacaoUsuarioPadrao() kernel.Ordenacao[CampoUsuario] {
	return kernel.NovaOrdenacao(OrdemUsuarioID, kernel.Asc)
}
