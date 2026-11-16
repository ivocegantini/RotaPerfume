package config_test

// ECOM-03/07: CADASTRO_EMAIL_DOMINIOS (provedores do autocadastro) e APP_URL.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

func TestECOM07_Load_CadastroEmailDominios(t *testing.T) {
	casos := []struct {
		nome   string
		valor  *string
		want   []string
		recusa bool
	}{
		{"ausente → padrão", nil, config.DominiosEmailPadrao, false},
		{"só espaços → padrão", sp("  "), config.DominiosEmailPadrao, false},
		{"lista normalizada", sp(" Gmail.com, @outlook.com ,,gmail.com "), []string{"gmail.com", "outlook.com"}, false},
		{"sem ponto", sp("gmail"), nil, true},
		{"com espaço interno", sp("g mail.com"), nil, true},
		{"com arroba no meio", sp("a@gmail.com"), nil, true},
		{"só vírgulas", sp(",,"), nil, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			env := sec11Base()
			env["DB_USUARIO"] = sp("u")
			env["DB_SENHA"] = sp("s")
			env["CADASTRO_EMAIL_DOMINIOS"] = c.valor
			sec11Env(t, env)
			cfg, err := config.Load()
			if c.recusa {
				require.ErrorIs(t, err, config.ErrDominiosEmail)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, cfg.CadastroEmailDominios)
		})
	}
}

func TestECOM03_Load_AppURL(t *testing.T) {
	casos := []struct {
		nome   string
		valor  *string
		want   string
		recusa bool
	}{
		{"https com caminho e barra final", sp("https://loja.exemplo/app/"), "https://loja.exemplo/app", false},
		{"sem esquema", sp("loja.exemplo"), "", true},
		{"com query", sp("https://loja.exemplo/?x=1"), "", true},
		{"com usuário", sp("https://u@loja.exemplo"), "", true},
		{"javascript", sp("javascript:alert(1)"), "", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			env := sec11Base()
			env["DB_USUARIO"] = sp("u")
			env["DB_SENHA"] = sp("s")
			env["APP_URL"] = c.valor
			sec11Env(t, env)
			cfg, err := config.Load()
			if c.recusa {
				require.ErrorIs(t, err, config.ErrAppURL)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, cfg.AppURL)
		})
	}
}
