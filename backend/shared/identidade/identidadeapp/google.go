package identidadeapp

import (
	"context"
	"errors"
	"log"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/vlog"
)

const arqGoogle = "google.go"

// ResultadoGoogle é o desfecho do "Entrar com Google" (ECOM-02): a sessão
// aberta (conta existente) ou, sem conta, o perfil para completar o
// cadastro (CPF, endereço...).
type ResultadoGoogle struct {
	Sessao             *SessaoEmitida
	CadastroNecessario bool
	Perfil             identidade.PerfilGoogle
}

// perfilGoogle valida o token (e o rate limit por IP, que conta toda
// tentativa) e exige o e-mail verificado pelo Google.
func (c *Casos) perfilGoogle(ctx context.Context, token string, o Origem) (identidade.PerfilGoogle, error) {
	if c.d.Google == nil {
		return identidade.PerfilGoogle{}, identidade.ErrGoogleIndisponivel
	}
	chave := "google:" + o.IP
	if bloqueado, retry := c.d.Limites.Cadastro.Bloqueado(chave); bloqueado {
		log.Printf("[auth] google: IP bloqueado por rate limit: ip=%s retry_after=%s", o.IP, retry)
		return identidade.PerfilGoogle{}, bloqueio(retry)
	}
	p, err := c.d.Google.Verificar(ctx, token)
	if err != nil {
		c.d.Limites.Cadastro.RegistrarFalha(chave)
		log.Printf("[auth] google: token recusado: ip=%s: %v", o.IP, err)
		if errors.Is(err, identidade.ErrGoogleIndisponivel) {
			return identidade.PerfilGoogle{}, err
		}
		return identidade.PerfilGoogle{}, identidade.ErrGoogleTokenInvalido
	}
	if !p.EmailVerificado {
		log.Printf("[auth] google: email não verificado: %s", vlog.MaskEmail(p.Email))
		return identidade.PerfilGoogle{}, identidade.ErrGoogleEmailNaoVerificado
	}
	return p, nil
}

// EntrarComGoogle é o POST /api/auth/google. Conta existente: só cliente
// da loja ativo entra (admin/equipe usam senha); conta pendente fica
// confirmada (o Google verificou o e-mail). Sem conta: CadastroNecessario.
func (c *Casos) EntrarComGoogle(ctx context.Context, token string, o Origem) (r ResultadoGoogle, err error) {
	defer vlog.Trace(arqGoogle, "Identidade.EntrarComGoogle", "")(&err)
	p, err := c.perfilGoogle(ctx, token, o)
	if err != nil {
		return ResultadoGoogle{}, err
	}
	u, err := c.d.Usuarios.PorEmail(ctx, p.Email)
	if errors.Is(err, identidade.ErrUsuarioNaoEncontrado) {
		log.Printf("[auth] google: sem conta, cadastro necessário: %s", vlog.MaskEmail(p.Email))
		return ResultadoGoogle{CadastroNecessario: true, Perfil: p}, nil
	}
	if err != nil {
		return ResultadoGoogle{}, erroLeituraUsuario("[auth] google: PorEmail", err)
	}
	if !u.Ativo {
		log.Printf("[auth] google: usuário inativo: user_id=%d", u.ID)
		return ResultadoGoogle{}, identidade.ErrCredenciaisInvalidas
	}
	if !u.Role.EhUsuarioLoja() {
		log.Printf("[auth] google: recusado para role=%s user_id=%d", u.Role, u.ID)
		return ResultadoGoogle{}, identidade.ErrGoogleSoLoja
	}
	if err := c.confirmarPeloGoogle(ctx, u); err != nil {
		return ResultadoGoogle{}, err
	}
	s, err := c.abrirSessao(ctx, u, o)
	if err != nil {
		return ResultadoGoogle{}, err
	}
	log.Printf("[auth] google: login OK user_id=%d", u.ID)
	return ResultadoGoogle{Sessao: s}, nil
}

// confirmarPeloGoogle libera a conta da loja ainda pendente: o e-mail foi
// verificado pelo Google; os links abertos são invalidados.
func (c *Casos) confirmarPeloGoogle(ctx context.Context, u *identidade.Usuario) error {
	pendente, err := c.emailPendente(ctx, u, "[auth] google")
	if err != nil || !pendente {
		return err
	}
	if c.d.TxConfirmacao == nil {
		return identidade.ErrConfirmacaoIndisponivel
	}
	agora := c.agora()
	err = c.d.TxConfirmacao.WithTx(ctx, func(tx TxCadastro) error {
		if err := tx.Confirmacoes.InvalidarDoUsuario(ctx, u.ID, agora); err != nil {
			return err
		}
		return tx.Confirmacoes.DefinirPendente(ctx, u.ID, false)
	})
	if err == nil {
		log.Printf("[auth] google: email confirmado pelo Google: user_id=%d", u.ID)
	}
	return err
}

// CadastrarComGoogle é o POST /api/auth/google/cadastro: cria o cliente da
// loja com nome e e-mail do Google (e-mail já confirmado) e os dados do
// cliente digitados; abre a sessão. A conta não tem senha utilizável (hash
// de uma senha aleatória descartada): entra pelo Google.
func (c *Casos) CadastrarComGoogle(ctx context.Context, token string, d DadosClienteLoja, o Origem) (s *SessaoEmitida, err error) {
	defer vlog.Trace(arqGoogle, "Identidade.CadastrarComGoogle", "")(&err)
	if err := c.clientesLigado(); err != nil {
		return nil, err
	}
	if c.d.TxConfirmacao == nil {
		return nil, identidade.ErrConfirmacaoIndisponivel
	}
	p, err := c.perfilGoogle(ctx, token, o)
	if err != nil {
		return nil, err
	}
	senha, hash, err := c.gerarSenhaEHash()
	if err != nil {
		return nil, err
	}
	nome := p.Nome
	if nome == "" {
		nome = p.Email
	}
	v, err := identidade.ValidarCadastroLoja(nome, p.Email, senha)
	if err != nil {
		return nil, err
	}
	if !identidade.ProvedorEmailPermitido(v.Email, c.o.DominiosEmailCadastro) {
		log.Printf("[auth] google cadastro: provedor de email não permitido: %s", vlog.MaskEmail(v.Email))
		return nil, identidade.ErrProvedorEmailNaoPermitido
	}
	if err := c.d.ClientesLoja.ValidarClienteLoja(v.Nome, d); err != nil {
		return nil, err
	}
	u := identidade.NovoUsuarioLoja(v, hash)
	err = c.d.TxConfirmacao.WithTx(ctx, func(tx TxCadastro) error {
		if err := tx.Usuarios.Inserir(ctx, u); err != nil {
			return err
		}
		return tx.Clientes.CriarClienteLoja(ctx, u.ID, u.Nome, d)
	})
	if errors.Is(err, identidade.ErrEmailDuplicado) {
		log.Printf("[auth] google cadastro: email já cadastrado: %s", vlog.MaskEmail(v.Email))
		return nil, identidade.ErrCadastroIndisponivel
	}
	if err != nil {
		return nil, err
	}
	log.Printf("[auth] google cadastro OK: user_id=%d email=%s", u.ID, vlog.MaskEmail(u.Email))
	return c.abrirSessao(ctx, u, o)
}
