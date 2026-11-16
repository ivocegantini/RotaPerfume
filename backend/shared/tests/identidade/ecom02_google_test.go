package identidade_test

// ECOM-02: entrar e criar conta com Google (casos de uso sobre fakes).

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/kernel"
)

type googleFake struct {
	perfil identidade.PerfilGoogle
	err    error
}

func (g googleFake) Verificar(context.Context, string) (identidade.PerfilGoogle, error) { return g.perfil, g.err }

func perfilAna() identidade.PerfilGoogle {
	return identidade.PerfilGoogle{Sub: "1", Email: "ana@loja.com", Nome: "Ana Souza", EmailVerificado: true}
}

func casosGoogle(a *ambiente, g identidadeapp.VerificadorGoogle, conf *confirmacoesFake, cl *clientesLojaFake) *identidadeapp.Casos {
	d := a.deps()
	d.Confirmacoes, d.TxConfirmacao, d.EmailConfirmacao = conf, txCadastroFake{a.usuarios, conf, cl}, emailConfFake{&[]string{}}
	d.ClientesLoja, d.Google = cl, g
	return identidadeapp.Novo(d, identidadeapp.Opcoes{URLApp: "https://x", DominiosEmailCadastro: []string{"loja.com"}})
}

var origemG = identidadeapp.Origem{IP: "8.8.8.8"}

func TestECOM02_EntrarComGoogle_ContaDaLoja(t *testing.T) {
	u := usuarioAtivo(5, "ana@loja.com", senhaForteECOM, kernel.RoleUsuario)
	a, conf := novoAmbiente(u), novoConfirmacoesFake()
	conf.pendente[5] = true
	r, err := casosGoogle(a, googleFake{perfil: perfilAna()}, conf, &clientesLojaFake{}).EntrarComGoogle(context.Background(), "tok", origemG)
	require.NoError(t, err)
	require.NotNil(t, r.Sessao)
	assert.Equal(t, "jwt", r.Sessao.Acesso)
	assert.False(t, conf.pendente[5], "o Google verificou o e-mail: conta confirmada")
	assert.Zero(t, a.usuarios.rehash, "sem senha: nada de rehash")
}

func TestECOM02_EntrarComGoogle_SemConta_PedeCadastro(t *testing.T) {
	a := novoAmbiente()
	r, err := casosGoogle(a, googleFake{perfil: perfilAna()}, novoConfirmacoesFake(), &clientesLojaFake{}).EntrarComGoogle(context.Background(), "tok", origemG)
	require.NoError(t, err)
	assert.True(t, r.CadastroNecessario)
	assert.Nil(t, r.Sessao)
	assert.Equal(t, "ana@loja.com", r.Perfil.Email)
}

func TestECOM02_EntrarComGoogle_Recusas(t *testing.T) {
	admin := usuarioAtivo(1, "ana@loja.com", "x", kernel.RoleAdmin)
	inativo := usuarioAtivo(2, "bia@loja.com", "x", kernel.RoleUsuario)
	inativo.Ativo = false
	casos := []struct {
		nome string
		u    *identidade.Usuario
		g    identidadeapp.VerificadorGoogle
		erro error
	}{
		{"admin não entra pelo Google", admin, googleFake{perfil: perfilAna()}, identidade.ErrGoogleSoLoja},
		{"inativo", inativo, googleFake{perfil: identidade.PerfilGoogle{Email: "bia@loja.com", EmailVerificado: true}}, identidade.ErrCredenciaisInvalidas},
		{"email não verificado", nil, googleFake{perfil: identidade.PerfilGoogle{Email: "x@loja.com"}}, identidade.ErrGoogleEmailNaoVerificado},
		{"token inválido", nil, googleFake{err: identidade.ErrGoogleTokenInvalido}, identidade.ErrGoogleTokenInvalido},
		{"sem Google", nil, nil, identidade.ErrGoogleIndisponivel},
	}
	for _, c := range casos {
		var us []*identidade.Usuario
		if c.u != nil {
			us = append(us, c.u)
		}
		a := novoAmbiente(us...)
		_, err := casosGoogle(a, c.g, novoConfirmacoesFake(), &clientesLojaFake{}).EntrarComGoogle(context.Background(), "tok", origemG)
		assert.ErrorIs(t, err, c.erro, c.nome)
		assert.Zero(t, a.tokens.inseridos, "%s: sem sessão", c.nome)
	}
}

func TestECOM02_CadastrarComGoogle_CriaContaConfirmadaECliente(t *testing.T) {
	a, conf, cl := novoAmbiente(), novoConfirmacoesFake(), &clientesLojaFake{}
	s, err := casosGoogle(a, googleFake{perfil: perfilAna()}, conf, cl).CadastrarComGoogle(context.Background(), "tok", clienteOK, origemG)
	require.NoError(t, err)
	require.NotNil(t, s.Usuario)
	assert.Equal(t, kernel.RoleUsuario, s.Usuario.Role)
	assert.Equal(t, "Ana Souza", s.Usuario.Nome)
	assert.Equal(t, "ana@loja.com", s.Usuario.Email)
	assert.False(t, conf.pendente[s.Usuario.ID], "e-mail já verificado pelo Google")
	assert.Empty(t, conf.links, "sem link de confirmação")
	require.Len(t, cl.criados, 1)
	assert.Equal(t, []int64{s.Usuario.ID}, cl.donos)
}

func TestECOM02_CadastrarComGoogle_Recusas(t *testing.T) {
	outro := identidade.PerfilGoogle{Email: "ana@empresa.com.br", Nome: "Ana", EmailVerificado: true}
	a, cl := novoAmbiente(), &clientesLojaFake{}
	_, err := casosGoogle(a, googleFake{perfil: outro}, novoConfirmacoesFake(), cl).CadastrarComGoogle(context.Background(), "tok", clienteOK, origemG)
	assert.ErrorIs(t, err, identidade.ErrProvedorEmailNaoPermitido)

	a.usuarios.errInserir = identidade.ErrEmailDuplicado
	_, err = casosGoogle(a, googleFake{perfil: perfilAna()}, novoConfirmacoesFake(), cl).CadastrarComGoogle(context.Background(), "tok", clienteOK, origemG)
	assert.ErrorIs(t, err, identidade.ErrCadastroIndisponivel)
	assert.Empty(t, cl.criados)
}
