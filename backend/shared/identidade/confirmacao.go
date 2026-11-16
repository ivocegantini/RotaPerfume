package identidade

import "time"

// ValidadeConfirmacaoEmail é a validade do link de confirmação (ECOM-03).
const ValidadeConfirmacaoEmail = 24 * time.Hour

// ConfirmacaoEmail é o link de confirmação do autocadastro (ECOM-03). Só o
// hash do token é guardado; uso único (UsadoEm) e validade (ExpiraEm).
type ConfirmacaoEmail struct {
	ID        int64
	UsuarioID int64
	Hash      string
	ExpiraEm  time.Time
	UsadoEm   *time.Time
}

// Valida: ainda não usada nem invalidada, e dentro da validade.
func (c *ConfirmacaoEmail) Valida(agora time.Time) bool {
	return c.UsadoEm == nil && agora.Before(c.ExpiraEm)
}
