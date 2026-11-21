package config_test

// CACHE-01: REDIS_URL/REDIS_PASSWORD/CACHE_APPNAME (vazio desliga; inválido
// derruba a subida; senha nunca na URL).

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/adapters/rediscache"
	"github.com/rotaperfumes/shared/cache"
	"github.com/rotaperfumes/shared/config"
)

func TestCACHE01_LoadCache(t *testing.T) {
	t.Setenv("REDIS_URL", "")
	t.Setenv("REDIS_PASSWORD", "")
	t.Setenv("CACHE_APPNAME", "")
	r, app, err := config.LoadCache()
	require.NoError(t, err)
	assert.False(t, r.Ligado)
	assert.Equal(t, "rotaperfumes", app)

	t.Setenv("REDIS_URL", "redis://192.168.168.106:48110/1")
	t.Setenv("REDIS_PASSWORD", "x")
	t.Setenv("CACHE_APPNAME", "rp-loja")
	r, app, err = config.LoadCache()
	require.NoError(t, err)
	assert.Equal(t, config.RedisConfig{Ligado: true, Endereco: "192.168.168.106:48110", Senha: "x", DB: 1}, r)
	assert.Equal(t, "rp-loja", app)

	t.Setenv("REDIS_URL", "http://192.168.168.106:48110")
	_, _, err = config.LoadCache()
	assert.ErrorIs(t, err, rediscache.ErrURL)

	t.Setenv("REDIS_URL", "redis://192.168.168.106:48110")
	t.Setenv("CACHE_APPNAME", "Rota:Perfume")
	_, _, err = config.LoadCache()
	assert.ErrorIs(t, err, cache.ErrAppName)
}
