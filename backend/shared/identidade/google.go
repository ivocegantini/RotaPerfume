package identidade

import "github.com/rotaperfumes/shared/kernel"

// PerfilGoogle é a identidade confirmada pelo ID token do Google (ECOM-02),
// já validado (assinatura, emissor, audiência e expiração). Email em
// minúsculas.
type PerfilGoogle struct {
	Sub             string
	Email           string
	Nome            string
	EmailVerificado bool
}

// Erros do login/cadastro com Google (ECOM-02).
var (
	ErrGoogleIndisponivel       = kernel.NovoErro(kernel.KindProibido, "login com Google não está configurado")
	ErrGoogleTokenInvalido      = kernel.NovoErro(kernel.KindInvalido, "login com Google inválido ou expirado; tente novamente")
	ErrGoogleEmailNaoVerificado = kernel.NovoErro(kernel.KindProibido, "o email da conta Google não está verificado")
	// ErrGoogleSoLoja: admin e equipe interna entram só com email e senha.
	ErrGoogleSoLoja = kernel.NovoErro(kernel.KindProibido, "o login com Google é só para clientes da loja; use email e senha")
)
