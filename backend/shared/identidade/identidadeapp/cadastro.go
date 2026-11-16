package identidadeapp

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/vlog"
)

const arqCadastro = "cadastro.go"

// ComandoCadastro é o POST /api/auth/cadastro (ECOM-01). Não tem role: o
// autocadastro cria sempre "usuario".
type ComandoCadastro struct {
	Nome  string
	Email string
	Senha identidade.Senha
	// Cliente são os dados do cliente pessoa física criado junto (ECOM-05).
	Cliente DadosClienteLoja
	Captcha string
	Origem  Origem
}

// confirmacaoLigada: as portas do ECOM-03 são obrigatórias para o fluxo da
// loja (fail-closed).
func (c *Casos) confirmacaoLigada() error {
	if c.d.Confirmacoes == nil || c.d.TxConfirmacao == nil || c.d.EmailConfirmacao == nil || c.o.URLApp == "" {
		log.Printf("[auth] confirmação de email indisponível: portas ou APP_URL ausentes")
		return identidade.ErrConfirmacaoIndisponivel
	}
	return nil
}

// clientesLigado: a porta do cliente da loja é obrigatória no cadastro.
func (c *Casos) clientesLigado() error {
	if c.d.ClientesLoja == nil {
		log.Printf("[auth] cadastro indisponível: porta de clientes da loja ausente")
		return identidade.ErrConfirmacaoIndisponivel
	}
	return nil
}

// Cadastrar cria o cliente da loja com o e-mail pendente e envia o link de
// confirmação (ECOM-03): NÃO abre sessão. Ordem: captcha → rate limit por IP
// (toda tentativa conta) → validação (login e cliente) → hash → tx (INSERT
// do usuário, pendente, cliente da loja ECOM-05, link) →
// e-mail. E-mail repetido: ErrCadastroIndisponivel (mensagem neutra).
// enviado=false (e-mail não saiu) não desfaz a conta: o reenvio resolve.
func (c *Casos) Cadastrar(ctx context.Context, cmd ComandoCadastro) (u *identidade.Usuario, enviado bool, err error) {
	defer vlog.Trace(arqCadastro, "Identidade.Cadastrar", "")(&err)
	if err := c.confirmacaoLigada(); err != nil {
		return nil, false, err
	}
	if err := c.clientesLigado(); err != nil {
		return nil, false, err
	}
	ip := cmd.Origem.IP
	if err := c.verificarCaptcha(ctx, cmd.Captcha, ip, identidade.AcaoCaptchaCadastro, "cadastro"); err != nil {
		return nil, false, err
	}
	chave := "cad:" + ip
	if bloqueado, retry := c.d.Limites.Cadastro.Bloqueado(chave); bloqueado {
		log.Printf("[auth] cadastro: IP bloqueado por rate limit: ip=%s retry_after=%s", ip, retry)
		return nil, false, bloqueio(retry)
	}
	c.d.Limites.Cadastro.RegistrarFalha(chave) // conta sucesso e falha: limita o volume
	v, err := identidade.ValidarCadastroLoja(cmd.Nome, cmd.Email, cmd.Senha)
	if err != nil {
		return nil, false, err
	}
	if !identidade.ProvedorEmailPermitido(v.Email, c.o.DominiosEmailCadastro) {
		log.Printf("[auth] cadastro: provedor de email não permitido: %s", vlog.MaskEmail(v.Email))
		return nil, false, identidade.ErrProvedorEmailNaoPermitido
	}
	if err := c.d.ClientesLoja.ValidarClienteLoja(v.Nome, cmd.Cliente); err != nil {
		return nil, false, err
	}
	hash, err := c.d.Hasher.Gerar(cmd.Senha)
	if err != nil {
		log.Printf("[auth] cadastro: HashPassword: %v", err)
		return nil, false, err
	}
	u = identidade.NovoUsuarioLoja(v, hash)
	token := c.d.Refresh.Novo()
	err = c.d.TxConfirmacao.WithTx(ctx, func(tx TxCadastro) error {
		if err := tx.Usuarios.Inserir(ctx, u); err != nil {
			return err
		}
		if err := tx.Confirmacoes.DefinirPendente(ctx, u.ID, true); err != nil {
			return err
		}
		if err := tx.Clientes.CriarClienteLoja(ctx, u.ID, u.Nome, cmd.Cliente); err != nil {
			return err
		}
		return tx.Confirmacoes.Inserir(ctx, c.novaConfirmacao(u.ID, token))
	})
	if err != nil {
		if errors.Is(err, identidade.ErrEmailDuplicado) {
			log.Printf("[auth] cadastro: email já cadastrado: %s", vlog.MaskEmail(v.Email))
			return nil, false, identidade.ErrCadastroIndisponivel
		}
		if errors.Is(err, identidade.ErrCadastroIndisponivel) {
			log.Printf("[auth] cadastro: CPF já cadastrado (email=%s)", vlog.MaskEmail(v.Email))
			return nil, false, err
		}
		return nil, false, err
	}
	enviado = c.enviarConfirmacao(ctx, u, token)
	log.Printf("[auth] cadastro OK (email pendente): user_id=%d email=%s email_enviado=%t", u.ID, vlog.MaskEmail(u.Email), enviado)
	return u, enviado, nil
}

func (c *Casos) novaConfirmacao(usuarioID int64, token identidade.TokenOpaco) *identidade.ConfirmacaoEmail {
	return &identidade.ConfirmacaoEmail{
		UsuarioID: usuarioID, Hash: c.d.Refresh.Hash(token), ExpiraEm: c.agora().Add(identidade.ValidadeConfirmacaoEmail),
	}
}

// enviarConfirmacao manda o link (token no fragmento #, que não vai a
// servidores nem ao Referer). Falha só loga.
func (c *Casos) enviarConfirmacao(ctx context.Context, u *identidade.Usuario, token identidade.TokenOpaco) bool {
	link := strings.TrimRight(c.o.URLApp, "/") + "/confirmar-email#token=" + token.Revelar()
	if err := c.d.EmailConfirmacao.EnviarConfirmacaoEmail(ctx, u.Email, u.Nome, link); err != nil {
		log.Printf("[auth] confirmação: falha ao enviar email user_id=%d: %v", u.ID, err)
		return false
	}
	return true
}

// ConfirmarEmail é o POST /api/auth/confirmar-email (público): valida o link
// (existe, não usado, dentro da validade) e libera o login, numa tx. Links
// inválidos contam no rate limit por IP.
func (c *Casos) ConfirmarEmail(ctx context.Context, token string, o Origem) (err error) {
	defer vlog.Trace(arqCadastro, "Identidade.ConfirmarEmail", "")(&err)
	if err := c.confirmacaoLigada(); err != nil {
		return err
	}
	chave := "conf:" + o.IP
	if bloqueado, retry := c.d.Limites.Confirmacao.Bloqueado(chave); bloqueado {
		log.Printf("[auth] confirmar-email: IP bloqueado por rate limit: ip=%s retry_after=%s", o.IP, retry)
		return bloqueio(retry)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return identidade.ErrConfirmacaoInvalida
	}
	conf, err := c.d.Confirmacoes.PorHash(ctx, c.d.Refresh.Hash(identidade.NovoTokenOpaco(token)))
	if err == nil && !conf.Valida(c.agora()) {
		err = identidade.ErrConfirmacaoInvalida
	}
	if err != nil {
		if errors.Is(err, identidade.ErrConfirmacaoInvalida) {
			c.d.Limites.Confirmacao.RegistrarFalha(chave)
			log.Printf("[auth] confirmar-email: link inválido, usado ou expirado: ip=%s", o.IP)
		}
		return err
	}
	agora := c.agora()
	err = c.d.TxConfirmacao.WithTx(ctx, func(tx TxCadastro) error {
		if err := tx.Confirmacoes.MarcarUsada(ctx, conf.ID, agora); err != nil {
			return err
		}
		return tx.Confirmacoes.DefinirPendente(ctx, conf.UsuarioID, false)
	})
	if err != nil {
		return err
	}
	log.Printf("[auth] email confirmado: user_id=%d", conf.UsuarioID)
	return nil
}

// ComandoReenvio é o POST /api/auth/reenviar-confirmacao (público).
type ComandoReenvio struct {
	Email   string
	Captcha string
	Origem  Origem
}

// ReenviarConfirmacao gera um link novo (invalida os anteriores) para conta
// ativa da loja ainda pendente. Resposta neutra: sem erro quando a conta não
// existe, já confirmou ou não é da loja (não revela cadastro).
func (c *Casos) ReenviarConfirmacao(ctx context.Context, cmd ComandoReenvio) (err error) {
	defer vlog.Trace(arqCadastro, "Identidade.ReenviarConfirmacao", "")(&err)
	if err := c.confirmacaoLigada(); err != nil {
		return err
	}
	ip := cmd.Origem.IP
	if err := c.verificarCaptcha(ctx, cmd.Captcha, ip, identidade.AcaoCaptchaReenvio, "reenviar-confirmacao"); err != nil {
		return err
	}
	email := strings.ToLower(strings.TrimSpace(cmd.Email))
	for _, chave := range []string{"reenv:" + ip, "reenv-conta:" + email} {
		if bloqueado, retry := c.d.Limites.Cadastro.Bloqueado(chave); bloqueado {
			log.Printf("[auth] reenviar-confirmacao: bloqueado por rate limit: ip=%s retry_after=%s", ip, retry)
			return bloqueio(retry)
		}
		c.d.Limites.Cadastro.RegistrarFalha(chave)
	}
	u, pendente := c.contaPendente(ctx, email)
	if !pendente {
		return nil
	}
	token := c.d.Refresh.Novo()
	agora := c.agora()
	err = c.d.TxConfirmacao.WithTx(ctx, func(tx TxCadastro) error {
		if err := tx.Confirmacoes.InvalidarDoUsuario(ctx, u.ID, agora); err != nil {
			return err
		}
		return tx.Confirmacoes.Inserir(ctx, c.novaConfirmacao(u.ID, token))
	})
	if err != nil {
		log.Printf("[auth] reenviar-confirmacao: gravar link user_id=%d: %v", u.ID, err)
		return nil // neutro
	}
	enviado := c.enviarConfirmacao(ctx, u, token)
	log.Printf("[auth] reenviar-confirmacao: user_id=%d email_enviado=%t", u.ID, enviado)
	return nil
}

// contaPendente: conta ativa da loja com o e-mail ainda pendente. Qualquer
// outro caso (inclusive erro de leitura, logado) é "não pendente".
func (c *Casos) contaPendente(ctx context.Context, email string) (*identidade.Usuario, bool) {
	if email == "" {
		return nil, false
	}
	u, err := c.d.Usuarios.PorEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, identidade.ErrUsuarioNaoEncontrado) {
			log.Printf("[auth] reenviar-confirmacao: PorEmail: %v", err)
		}
		return nil, false
	}
	if !u.Ativo || !u.Role.EhUsuarioLoja() {
		return nil, false
	}
	pendente, err := c.d.Confirmacoes.EmailPendente(ctx, u.ID)
	if err != nil {
		log.Printf("[auth] reenviar-confirmacao: EmailPendente user_id=%d: %v", u.ID, err)
		return nil, false
	}
	return u, pendente
}

// emailPendente informa se o "usuario" da loja ainda não confirmou o e-mail
// (ECOM-03/08). Os demais papéis (criados pelo admin) nunca consultam. Sem a
// porta, falha fechado. rotulo é só o prefixo do log.
func (c *Casos) emailPendente(ctx context.Context, u *identidade.Usuario, rotulo string) (bool, error) {
	if !u.Role.EhUsuarioLoja() {
		return false, nil
	}
	if c.d.Confirmacoes == nil {
		log.Printf("%s: confirmação de email indisponível (fail-closed) user_id=%d", rotulo, u.ID)
		return false, identidade.ErrConfirmacaoIndisponivel
	}
	pendente, err := c.d.Confirmacoes.EmailPendente(ctx, u.ID)
	if err != nil {
		log.Printf("%s: EmailPendente user_id=%d: %v", rotulo, u.ID, err)
		return false, err
	}
	return pendente, nil
}

// exigirEmailConfirmado barra o login do "usuario" da loja com o e-mail
// pendente (ECOM-03).
func (c *Casos) exigirEmailConfirmado(ctx context.Context, u *identidade.Usuario) error {
	pendente, err := c.emailPendente(ctx, u, "[auth] login")
	if err != nil {
		return err
	}
	if pendente {
		log.Printf("[auth] login: email não confirmado: user_id=%d", u.ID)
		return identidade.ErrEmailNaoConfirmado
	}
	return nil
}

// bloquearTrocaSeEmailPendente recusa troca/reset de senha da conta com o
// e-mail pendente (ECOM-08).
func (c *Casos) bloquearTrocaSeEmailPendente(ctx context.Context, u *identidade.Usuario, rotulo string) error {
	pendente, err := c.emailPendente(ctx, u, rotulo)
	if err != nil {
		return err
	}
	if pendente {
		log.Printf("%s: recusada, email pendente de confirmação: user_id=%d", rotulo, u.ID)
		return identidade.ErrTrocaSenhaEmailPendente
	}
	return nil
}
