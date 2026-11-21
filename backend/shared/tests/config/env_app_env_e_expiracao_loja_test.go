package config_test

// Cards: ECOM-18, ECOM-19.

// ECOM-18: APP_ENV (dev/qa/producao); ausente = producao (simulador desligado).

import (
	"testing"
	"time"

	"github.com/rotaperfumes/shared/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestECOM18_LoadAmbiente(t *testing.T) {
	for env, want := range map[string]config.Ambiente{"": "producao", "dev": "dev", " QA ": "qa", "producao": "producao"} {
		t.Setenv("APP_ENV", env)
		a, err := config.LoadAmbiente()
		require.NoError(t, err, env)
		assert.Equal(t, want, a)
	}
	for _, env := range []string{"prod", "staging", "development"} {
		t.Setenv("APP_ENV", env)
		_, err := config.LoadAmbiente()
		assert.Error(t, err, env)
	}
}

func TestECOM18_SimuladorSoEmDevEQA(t *testing.T) {
	assert.True(t, config.AmbienteDev.PermiteSimuladorPagamento())
	assert.True(t, config.AmbienteQA.PermiteSimuladorPagamento())
	assert.False(t, config.AmbienteProducao.PermiteSimuladorPagamento())
	assert.False(t, config.Ambiente("").PermiteSimuladorPagamento())
}

func TestECOM19_LoadExpiracaoLoja(t *testing.T) {
	for env, want := range map[string]time.Duration{"": time.Hour, "0": 0, "0s": 0, "30m": 30 * time.Minute, " 2h ": 2 * time.Hour} {
		t.Setenv("LOJA_EXPIRACAO_INTERVAL", env)
		d, err := config.LoadExpiracaoLoja()
		require.NoError(t, err, env)
		assert.Equal(t, want, d, env)
	}
	for _, env := range []string{"30s", "abc", "-1h"} {
		t.Setenv("LOJA_EXPIRACAO_INTERVAL", env)
		_, err := config.LoadExpiracaoLoja()
		assert.Error(t, err, env)
	}
}
