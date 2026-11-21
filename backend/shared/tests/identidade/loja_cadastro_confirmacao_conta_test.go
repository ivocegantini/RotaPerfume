package identidade_test

// ECOM-01/03: autocadastro do cliente da loja (role "usuario"): validação,
// ordem captcha → rate limit → validação, e-mail repetido neutro, conta
// pendente com link de confirmação (uso único, 24h) e login bloqueado.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/kernel"
)

const senhaForteECOM = "Loja#2026ok"

// clienteOK são dados válidos do cliente pessoa física (CPF de teste).
var clienteOK = identidadeapp.DadosClienteLoja{CPF: "529.982.247-25", Endereco: "Rua A", Numero: "10", CEP: "01001-000", Cidade: "São Paulo", UF: "sp"}

func cmdCadastro(nome, email, senha string) identidadeapp.ComandoCadastro {
	return identidadeapp.ComandoCadastro{Nome: nome, Email: email, Senha: identidade.NovaSenha(senha), Captcha: "tok",
		Cliente: clienteOK, Origem: identidadeapp.Origem{IP: "10.0.0.9"}}
}

// clientesLojaFake valida (erro configurável) e registra os clientes criados.
type clientesLojaFake struct {
	errValidar, errCriar error
	criados              []identidadeapp.DadosClienteLoja
	donos                []int64
}

func (f *clientesLojaFake) ValidarClienteLoja(string, identidadeapp.DadosClienteLoja) error { return f.errValidar }
func (f *clientesLojaFake) CriarClienteLoja(_ context.Context, uid int64, _ string, d identidadeapp.DadosClienteLoja) error {
	if f.errCriar != nil {
		return f.errCriar
	}
	f.criados, f.donos = append(f.criados, d), append(f.donos, uid)
	return nil
}

func TestECOM01_ValidarCadastroLoja(t *testing.T) {
	casos := []struct {
		nome, email, senha string
		erro               error
	}{
		{"", "a@b.com", senhaForteECOM, identidade.ErrNomeObrigatorio},
		{strings.Repeat("x", 121), "a@b.com", senhaForteECOM, identidade.ErrNomeMuitoLongo},
		{"Ana", " ", senhaForteECOM, identidade.ErrEmailObrigatorio},
		{"Ana", "sem-arroba", senhaForteECOM, identidade.ErrEmailInvalido},
		{"Ana", "a@@b.com", senhaForteECOM, identidade.ErrEmailInvalido},
		{"Ana", "a@semponto", senhaForteECOM, identidade.ErrEmailInvalido},
		{"Ana", "a@b..com", senhaForteECOM, identidade.ErrEmailInvalido},
		{"Ana", "a b@c.com", senhaForteECOM, identidade.ErrEmailInvalido},
		{"Ana", "a@b.com", "curta", identidade.ErrSenhaCurta},
		{"Ana", "a@b.com", "somenteminusculas", identidade.ErrSenhaFraca},
	}
	for _, c := range casos {
		_, err := identidade.ValidarCadastroLoja(c.nome, c.email, identidade.NovaSenha(c.senha))
		assert.ErrorIs(t, err, c.erro, "%q %q", c.nome, c.email)
	}
	v, err := identidade.ValidarCadastroLoja("  Ana  ", " Ana@Loja.COM ", identidade.NovaSenha(senhaForteECOM))
	require.NoError(t, err)
	assert.Equal(t, "Ana", v.Nome)
	assert.Equal(t, "ana@loja.com", v.Email)
	assert.Equal(t, kernel.RoleUsuario, v.Role)
}

// confirmacoesFake implementa as portas do ECOM-03 em memória.
type confirmacoesFake struct {
	pendente map[int64]bool
	links    map[string]*identidade.ConfirmacaoEmail
	proximo  int64
	errLer   error
}

func novoConfirmacoesFake() *confirmacoesFake {
	return &confirmacoesFake{pendente: map[int64]bool{}, links: map[string]*identidade.ConfirmacaoEmail{}}
}

func (f *confirmacoesFake) EmailPendente(_ context.Context, id int64) (bool, error) {
	return f.pendente[id], f.errLer
}
func (f *confirmacoesFake) PendentesEntre(_ context.Context, ids []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	for _, id := range ids {
		if f.pendente[id] {
			out[id] = true
		}
	}
	return out, f.errLer
}
func (f *confirmacoesFake) PorHash(_ context.Context, h string) (*identidade.ConfirmacaoEmail, error) {
	if c, ok := f.links[h]; ok {
		cp := *c
		return &cp, nil
	}
	return nil, identidade.ErrConfirmacaoInvalida
}
func (f *confirmacoesFake) Inserir(_ context.Context, c *identidade.ConfirmacaoEmail) error {
	f.proximo++
	c.ID = f.proximo
	f.links[c.Hash] = c
	return nil
}
func (f *confirmacoesFake) InvalidarDoUsuario(_ context.Context, uid int64, q time.Time) error {
	for _, c := range f.links {
		if c.UsuarioID == uid && c.UsadoEm == nil {
			t := q
			c.UsadoEm = &t
		}
	}
	return nil
}
func (f *confirmacoesFake) MarcarUsada(_ context.Context, id int64, q time.Time) error {
	for _, c := range f.links {
		if c.ID == id && c.UsadoEm == nil {
			t := q
			c.UsadoEm = &t
			return nil
		}
	}
	return identidade.ErrConfirmacaoInvalida
}
func (f *confirmacoesFake) DefinirPendente(_ context.Context, uid int64, p bool) error {
	f.pendente[uid] = p
	return nil
}

type txCadastroFake struct {
	u  *usuariosFake
	c  *confirmacoesFake
	cl *clientesLojaFake
}

func (t txCadastroFake) WithTx(_ context.Context, fn func(identidadeapp.TxCadastro) error) error {
	return fn(identidadeapp.TxCadastro{Usuarios: t.u, Confirmacoes: t.c, Clientes: t.cl})
}

type emailConfFake struct{ links *[]string }

func (e emailConfFake) EnviarConfirmacaoEmail(_ context.Context, _, _, link string) error {
	*e.links = append(*e.links, link)
	return nil
}

// casosECOM03 monta os casos com as portas da confirmação ligadas.
func casosECOM03(a *ambiente, conf *confirmacoesFake, links *[]string) *identidadeapp.Casos {
	return casosComClientes(a, conf, links, &clientesLojaFake{})
}

func casosComClientes(a *ambiente, conf *confirmacoesFake, links *[]string, cl *clientesLojaFake) *identidadeapp.Casos {
	d := a.deps()
	d.Confirmacoes, d.TxConfirmacao, d.EmailConfirmacao = conf, txCadastroFake{a.usuarios, conf, cl}, emailConfFake{links}
	d.ClientesLoja = cl
	return identidadeapp.Novo(d, identidadeapp.Opcoes{URLApp: "https://loja.exemplo/", DominiosEmailCadastro: []string{"b.com", "loja.com"}})
}

// ECOM-07: só provedores da lista; sem lista, ninguém se cadastra.
func TestECOM07_Cadastrar_ProvedorForaDaLista(t *testing.T) {
	a, cl := novoAmbiente(), &clientesLojaFake{}
	c := casosComClientes(a, novoConfirmacoesFake(), &[]string{}, cl)
	_, _, err := c.Cadastrar(context.Background(), cmdCadastro("Ana", "ana@empresa.com.br", senhaForteECOM))
	assert.ErrorIs(t, err, identidade.ErrProvedorEmailNaoPermitido)
	_, _, err = c.Cadastrar(context.Background(), cmdCadastro("Ana", "ana@sub.loja.com", senhaForteECOM))
	assert.ErrorIs(t, err, identidade.ErrProvedorEmailNaoPermitido, "subdomínio não é o provedor")
	assert.Empty(t, cl.criados)

	d := a.deps()
	conf := novoConfirmacoesFake()
	d.Confirmacoes, d.TxConfirmacao, d.EmailConfirmacao, d.ClientesLoja = conf, txCadastroFake{a.usuarios, conf, cl}, emailConfFake{&[]string{}}, cl
	semLista := identidadeapp.Novo(d, identidadeapp.Opcoes{URLApp: "https://x"})
	_, _, err = semLista.Cadastrar(context.Background(), cmdCadastro("Ana", "a@b.com", senhaForteECOM))
	assert.ErrorIs(t, err, identidade.ErrProvedorEmailNaoPermitido, "fail-closed sem lista")
}

// ECOM-07: status exibido ao admin.
func TestECOM07_StatusDoUsuario(t *testing.T) {
	assert.Equal(t, identidade.StatusAtivo, (&identidade.Usuario{Ativo: true}).Status())
	assert.Equal(t, identidade.StatusPendenteEmail, (&identidade.Usuario{Ativo: true, EmailPendente: true}).Status())
	assert.Equal(t, identidade.StatusInativo, (&identidade.Usuario{Ativo: false, EmailPendente: true}).Status())
}

// ECOM-05: o cadastro cria o cliente ligado ao login, na mesma tx.
func TestECOM05_Cadastrar_CriaClienteDoLogin(t *testing.T) {
	a, cl := novoAmbiente(), &clientesLojaFake{}
	u, _, err := casosComClientes(a, novoConfirmacoesFake(), &[]string{}, cl).Cadastrar(context.Background(), cmdCadastro("Ana", "a@b.com", senhaForteECOM))
	require.NoError(t, err)
	require.Len(t, cl.criados, 1)
	assert.Equal(t, []int64{u.ID}, cl.donos)
	assert.Equal(t, clienteOK, cl.criados[0])
}

// ECOM-05: dados do cliente inválidos barram ANTES de gravar; CPF repetido
// sai com a recusa neutra e não envia e-mail.
func TestECOM05_Cadastrar_ClienteInvalidoOuCPFRepetido(t *testing.T) {
	a, links := novoAmbiente(), &[]string{}
	errCPF := errors.New("cpf inválido")
	_, _, err := casosComClientes(a, novoConfirmacoesFake(), links, &clientesLojaFake{errValidar: errCPF}).
		Cadastrar(context.Background(), cmdCadastro("Ana", "a@b.com", senhaForteECOM))
	assert.ErrorIs(t, err, errCPF)
	assert.Equal(t, int64(100), a.usuarios.proximoID, "nada gravado")

	_, _, err = casosComClientes(a, novoConfirmacoesFake(), links, &clientesLojaFake{errCriar: identidade.ErrCadastroIndisponivel}).
		Cadastrar(context.Background(), cmdCadastro("Ana", "a@b.com", senhaForteECOM))
	assert.ErrorIs(t, err, identidade.ErrCadastroIndisponivel)
	assert.Empty(t, *links)
}

func TestECOM03_Cadastrar_ContaPendenteELink(t *testing.T) {
	a, conf, links := novoAmbiente(), novoConfirmacoesFake(), &[]string{}
	u, enviado, err := casosECOM03(a, conf, links).Cadastrar(context.Background(), cmdCadastro("Ana", "Ana@Loja.com", senhaForteECOM))
	require.NoError(t, err)
	assert.True(t, enviado)
	assert.Equal(t, kernel.RoleUsuario, u.Role)
	assert.Equal(t, "ana@loja.com", u.Email)
	assert.False(t, u.DeveTrocarSenha)
	assert.True(t, conf.pendente[u.ID], "conta nasce pendente")
	require.Len(t, *links, 1)
	assert.Regexp(t, `^https://loja\.exemplo/confirmar-email#token=novo$`, (*links)[0])
	assert.Equal(t, []string{identidade.AcaoCaptchaCadastro}, a.acoesCaptcha)
	c := conf.links["hash:novo"]
	require.NotNil(t, c, "só o hash do token é gravado")
	assert.Equal(t, a.agora.Add(identidade.ValidadeConfirmacaoEmail), c.ExpiraEm)
}

func TestECOM03_Cadastrar_SemPortas_FalhaFechado(t *testing.T) {
	a := novoAmbiente()
	_, _, err := a.casos().Cadastrar(context.Background(), cmdCadastro("Ana", "a@b.com", senhaForteECOM))
	assert.ErrorIs(t, err, identidade.ErrConfirmacaoIndisponivel)
}

func TestECOM01_Cadastrar_CaptchaAntesDeTudo(t *testing.T) {
	a := novoAmbiente()
	cmd := cmdCadastro("", "", "")
	cmd.Captcha = ""
	_, _, err := casosECOM03(a, novoConfirmacoesFake(), &[]string{}).Cadastrar(context.Background(), cmd)
	assert.ErrorIs(t, err, identidade.ErrCaptchaInvalido)
}

func TestECOM01_Cadastrar_RateLimitPorIP(t *testing.T) {
	a := novoAmbiente()
	a.limite = limitadorFake{bloquear: "cad:"}
	_, _, err := casosECOM03(a, novoConfirmacoesFake(), &[]string{}).Cadastrar(context.Background(), cmdCadastro("Ana", "a@b.com", senhaForteECOM))
	var b *kernel.Bloqueio
	require.ErrorAs(t, err, &b)
}

func TestECOM01_Cadastrar_EmailRepetido_MensagemNeutra(t *testing.T) {
	a, links := novoAmbiente(), &[]string{}
	a.usuarios.errInserir = identidade.ErrEmailDuplicado
	_, _, err := casosECOM03(a, novoConfirmacoesFake(), links).Cadastrar(context.Background(), cmdCadastro("Ana", "a@b.com", senhaForteECOM))
	assert.ErrorIs(t, err, identidade.ErrCadastroIndisponivel)
	assert.Empty(t, *links)
}

func loginCmd(email, senha string) identidadeapp.ComandoLogin {
	return identidadeapp.ComandoLogin{Email: email, Senha: identidade.NovaSenha(senha), Captcha: "tok", Origem: identidadeapp.Origem{IP: "1.2.3.4"}}
}

func TestECOM03_Login_PendenteBloqueado_SoComSenhaCerta(t *testing.T) {
	u := usuarioAtivo(5, "ana@loja.com", senhaForteECOM, kernel.RoleUsuario)
	a, conf := novoAmbiente(u), novoConfirmacoesFake()
	conf.pendente[5] = true
	c := casosECOM03(a, conf, &[]string{})

	_, err := c.Login(context.Background(), loginCmd("ana@loja.com", senhaForteECOM))
	assert.ErrorIs(t, err, identidade.ErrEmailNaoConfirmado)
	_, err = c.Login(context.Background(), loginCmd("ana@loja.com", "Errada#123"))
	assert.ErrorIs(t, err, identidade.ErrCredenciaisInvalidas, "senha errada não revela a pendência")

	conf.pendente[5] = false
	s, err := c.Login(context.Background(), loginCmd("ana@loja.com", senhaForteECOM))
	require.NoError(t, err)
	assert.Equal(t, "jwt", s.Acesso)
}

func TestECOM03_Login_CriadoPeloAdmin_NaoConsulta(t *testing.T) {
	for _, role := range []kernel.Role{kernel.RoleAdmin, kernel.RoleNormal} {
		u := usuarioAtivo(6, "x@y.com", "Senha#123", role)
		a := novoAmbiente(u)
		_, err := a.casos().Login(context.Background(), loginCmd("x@y.com", "Senha#123")) // sem portas do ECOM-03
		assert.NoError(t, err, role)
	}
}

func TestECOM03_ConfirmarEmail_UsoUnicoEExpiracao(t *testing.T) {
	a, conf, links := novoAmbiente(), novoConfirmacoesFake(), &[]string{}
	c := casosECOM03(a, conf, links)
	u, _, err := c.Cadastrar(context.Background(), cmdCadastro("Ana", "a@b.com", senhaForteECOM))
	require.NoError(t, err)

	o := identidadeapp.Origem{IP: "9.9.9.9"}
	assert.ErrorIs(t, c.ConfirmarEmail(context.Background(), "outro", o), identidade.ErrConfirmacaoInvalida)
	assert.ErrorIs(t, c.ConfirmarEmail(context.Background(), " ", o), identidade.ErrConfirmacaoInvalida)
	require.NoError(t, c.ConfirmarEmail(context.Background(), "novo", o))
	assert.False(t, conf.pendente[u.ID], "login liberado")
	assert.ErrorIs(t, c.ConfirmarEmail(context.Background(), "novo", o), identidade.ErrConfirmacaoInvalida, "uso único")

	conf.links["hash:novo"].UsadoEm = nil
	a.agora = a.agora.Add(identidade.ValidadeConfirmacaoEmail + time.Second)
	c = casosECOM03(a, conf, links)
	assert.ErrorIs(t, c.ConfirmarEmail(context.Background(), "novo", o), identidade.ErrConfirmacaoInvalida, "expirado")
}

func TestECOM03_Reenviar(t *testing.T) {
	pend := usuarioAtivo(5, "ana@loja.com", senhaForteECOM, kernel.RoleUsuario)
	normal := usuarioAtivo(6, "vend@x.com", "Senha#123", kernel.RoleNormal)
	a, conf, links := novoAmbiente(pend, normal), novoConfirmacoesFake(), &[]string{}
	conf.pendente[5] = true
	antigo := &identidade.ConfirmacaoEmail{UsuarioID: 5, Hash: "hash:antigo", ExpiraEm: a.agora.Add(time.Hour)}
	require.NoError(t, conf.Inserir(context.Background(), antigo))
	c := casosECOM03(a, conf, links)
	cmd := func(e string) identidadeapp.ComandoReenvio {
		return identidadeapp.ComandoReenvio{Email: e, Captcha: "tok", Origem: identidadeapp.Origem{IP: "1.1.1.1"}}
	}

	require.NoError(t, c.ReenviarConfirmacao(context.Background(), cmd(" ANA@loja.com ")))
	assert.Len(t, *links, 1)
	assert.NotNil(t, conf.links["hash:antigo"].UsadoEm, "link anterior invalidado")
	assert.Contains(t, a.acoesCaptcha, identidade.AcaoCaptchaReenvio)

	for _, e := range []string{"ninguem@x.com", "vend@x.com"} {
		require.NoError(t, c.ReenviarConfirmacao(context.Background(), cmd(e)), "neutro: %s", e)
	}
	conf.pendente[5] = false
	require.NoError(t, c.ReenviarConfirmacao(context.Background(), cmd("ana@loja.com")))
	assert.Len(t, *links, 1, "não reenvia para conta já confirmada nem para outras")
}

func TestECOM01_UsuarioLojaNaoAceitaVendedor(t *testing.T) {
	id := int64(7)
	_, err := identidade.ValidarCriacao(identidade.DadosUsuario{Nome: "A", Email: "a@b.com", Role: "usuario", IDVendedor: &id})
	assert.ErrorIs(t, err, identidade.ErrUsuarioLojaComVendedor)
	v, err := identidade.ValidarCriacao(identidade.DadosUsuario{Nome: "A", Email: "a@b.com", Role: "usuario"})
	require.NoError(t, err)
	assert.True(t, v.Role.EhUsuarioLoja())
	assert.False(t, v.Role.EhAdmin())
}

// ECOM-08: troca e reset de senha bloqueados com o e-mail pendente.
func TestECOM08_TrocaDeSenha_BloqueadaComEmailPendente(t *testing.T) {
	u := usuarioAtivo(5, "ana@loja.com", senhaForteECOM, kernel.RoleUsuario)
	a, conf := novoAmbiente(u), novoConfirmacoesFake()
	conf.pendente[5] = true
	c := casosECOM03(a, conf, &[]string{})

	err := c.TrocarSenha(context.Background(), principal(5, kernel.RoleUsuario), identidadeapp.ComandoTrocarSenha{
		SenhaAtual: identidade.NovaSenha(senhaForteECOM), NovaSenha: identidade.NovaSenha("Nova#2026ok"), Captcha: "tok",
		Origem: identidadeapp.Origem{IP: "1.1.1.1"},
	})
	assert.ErrorIs(t, err, identidade.ErrTrocaSenhaEmailPendente)
	assert.Zero(t, a.txSenha.chamadas, "nada gravado")

	_, err = c.AdminResetarSenha(context.Background(), principal(1, kernel.RoleAdmin), identidadeapp.ComandoAdminResetarSenha{UsuarioID: 5})
	assert.ErrorIs(t, err, identidade.ErrTrocaSenhaEmailPendente)
	assert.Zero(t, a.txSenha.chamadas, "reset do admin não grava")

	conf.pendente[5] = false
	_, err = c.AdminResetarSenha(context.Background(), principal(1, kernel.RoleAdmin), identidadeapp.ComandoAdminResetarSenha{UsuarioID: 5})
	assert.NoError(t, err, "confirmado: reset liberado")
}

// ECOM-08: o reset em massa pula a conta pendente e segue com as demais.
func TestECOM08_ResetEmMassa_PulaEmailPendente(t *testing.T) {
	pend := usuarioAtivo(5, "ana@loja.com", senhaForteECOM, kernel.RoleUsuario)
	vend := usuarioAtivo(6, "vend@x.com", "Senha#123", kernel.RoleNormal)
	a, conf := novoAmbiente(pend, vend), novoConfirmacoesFake()
	a.usuarios.ativos = []identidade.Usuario{*pend, *vend}
	conf.pendente[5] = true
	itens, err := casosECOM03(a, conf, &[]string{}).ResetarSenhasAtivos(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, itens, 2)
	assert.Equal(t, identidadeapp.StatusPuladoEmailPendente, itens[0].Status)
	assert.Equal(t, identidadeapp.StatusResetado, itens[1].Status)
}
