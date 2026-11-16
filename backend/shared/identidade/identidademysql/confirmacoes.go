package identidademysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/vlog"
)

const arqConfirmacoes = "confirmacoes.go"

// ConfirmacoesEmail é o repositório de confirmacoes_email e da marca
// usuarios.email_confirmacao_pendente (ECOM-03; portas
// identidadeapp.ConfirmacoesEmail e RepoConfirmacaoTx).
type ConfirmacoesEmail struct{ q db.Querier }

// NovoConfirmacoesEmail liga o repositório ao pool ou à tx. q nil: panic.
func NovoConfirmacoesEmail(q db.Querier) *ConfirmacoesEmail {
	if q == nil {
		panic("identidademysql: NovoConfirmacoesEmail exige Querier")
	}
	return &ConfirmacoesEmail{q: q}
}

// EmailPendente lê usuarios.email_confirmacao_pendente.
func (r *ConfirmacoesEmail) EmailPendente(ctx context.Context, usuarioID int64) (bool, error) {
	var pendente bool
	err := r.q.QueryRowContext(ctx, `SELECT email_confirmacao_pendente FROM usuarios WHERE id = ? LIMIT 1`, usuarioID).Scan(&pendente)
	if errors.Is(err, sql.ErrNoRows) {
		return false, identidade.ErrUsuarioNaoEncontrado
	}
	if err != nil {
		return false, fmt.Errorf("identidademysql: email pendente: %w", err)
	}
	return pendente, nil
}

// DefinirPendente grava a marca de pendente do usuário.
func (r *ConfirmacoesEmail) DefinirPendente(ctx context.Context, usuarioID int64, pendente bool) error {
	res, err := r.q.ExecContext(ctx, `UPDATE usuarios SET email_confirmacao_pendente = ? WHERE id = ?`, pendente, usuarioID)
	if err != nil {
		return fmt.Errorf("identidademysql: definir email pendente: %w", err)
	}
	// DSN com clientFoundRows=true (BUG-04): 0 linhas = usuário inexistente.
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return identidade.ErrUsuarioNaoEncontrado
	}
	vlog.Printf(arqConfirmacoes, "ConfirmacoesEmail.DefinirPendente", "UPDATE usuarios id=%d pendente=%t", usuarioID, pendente)
	return nil
}

// Inserir grava o link (só o hash) e preenche c.ID.
func (r *ConfirmacoesEmail) Inserir(ctx context.Context, c *identidade.ConfirmacaoEmail) error {
	res, err := r.q.ExecContext(ctx,
		`INSERT INTO confirmacoes_email (usuario_id, token_hash, expira_em) VALUES (?, ?, ?)`,
		c.UsuarioID, c.Hash, c.ExpiraEm)
	if err != nil {
		return fmt.Errorf("identidademysql: inserir confirmacao: %w", err)
	}
	if c.ID, err = res.LastInsertId(); err != nil {
		return fmt.Errorf("identidademysql: last insert id confirmacao: %w", err)
	}
	vlog.Printf(arqConfirmacoes, "ConfirmacoesEmail.Inserir", "INSERT confirmacoes_email id=%d usuario_id=%d", c.ID, c.UsuarioID)
	return nil
}

// PorHash busca o link pelo hash do token.
func (r *ConfirmacoesEmail) PorHash(ctx context.Context, hash string) (*identidade.ConfirmacaoEmail, error) {
	var c identidade.ConfirmacaoEmail
	var usado sql.NullTime
	err := r.q.QueryRowContext(ctx,
		`SELECT id, usuario_id, token_hash, expira_em, usado_em FROM confirmacoes_email WHERE token_hash = ? LIMIT 1`, hash).
		Scan(&c.ID, &c.UsuarioID, &c.Hash, &c.ExpiraEm, &usado)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, identidade.ErrConfirmacaoInvalida
	}
	if err != nil {
		return nil, fmt.Errorf("identidademysql: buscar confirmacao: %w", err)
	}
	if usado.Valid {
		t := usado.Time
		c.UsadoEm = &t
	}
	return &c, nil
}

// MarcarUsada é condicional (usado_em IS NULL): a corrida de dois cliques
// tem um vencedor; o outro recebe ErrConfirmacaoInvalida.
func (r *ConfirmacoesEmail) MarcarUsada(ctx context.Context, id int64, quando time.Time) error {
	res, err := r.q.ExecContext(ctx, `UPDATE confirmacoes_email SET usado_em = ? WHERE id = ? AND usado_em IS NULL`, quando, id)
	if err != nil {
		return fmt.Errorf("identidademysql: marcar confirmacao usada: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return identidade.ErrConfirmacaoInvalida
	}
	return nil
}

// InvalidarDoUsuario fecha os links ainda abertos do usuário (reenvio).
func (r *ConfirmacoesEmail) InvalidarDoUsuario(ctx context.Context, usuarioID int64, quando time.Time) error {
	if _, err := r.q.ExecContext(ctx,
		`UPDATE confirmacoes_email SET usado_em = ? WHERE usuario_id = ? AND usado_em IS NULL`, quando, usuarioID); err != nil {
		return fmt.Errorf("identidademysql: invalidar confirmacoes: %w", err)
	}
	return nil
}

// PendentesEntre devolve os ids (dentre os informados) com o e-mail pendente
// (ECOM-07). A lista do IN é só de marcadores "?" (valores por placeholder).
func (r *ConfirmacoesEmail) PendentesEntre(ctx context.Context, ids []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := r.q.QueryContext(ctx,
		`SELECT id FROM usuarios WHERE email_confirmacao_pendente = 1 AND id IN (`+marcadores(len(ids))+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("identidademysql: pendentes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("identidademysql: pendentes scan: %w", err)
		}
		out[id] = true
	}
	return out, rows.Err()
}

// marcadores devolve "?,?,…" com n placeholders.
func marcadores(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
