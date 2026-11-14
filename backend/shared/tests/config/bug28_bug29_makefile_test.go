package config_test

// BUG-28 (Lote 37): "make -k db-seed" com a guarda recusada não roda o
// db-up (CREATE DATABASE + DDL): o db-up saiu dos pré-requisitos e virou a
// 1a linha da receita, por sub-make.
// BUG-29 (Lote 37): MAKEFLAGS/MAKEOVERRIDES na linha de comando são
// recusados (os sub-makes perderiam o ENV_ARQUIVO e leriam o .env padrão).
// Reaproveita o harness do BUG-27 (cópia do Makefile, cliente e go falsos).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBUG28_Makefile_DbUpNaReceitaDoSeed(t *testing.T) {
	mk := lerMakefile(t)
	r := mk.regras["db-seed"]
	require.NotNil(t, r)
	assert.Equal(t, []string{"db-guard-producao", "db-guard-sem-producao", "db-check-env"}, r.prereqs,
		"db-seed: só guardas e db-check-env como pré-requisitos (db-up na receita)")
	require.NotEmpty(t, r.receita)
	assert.Equal(t, "$(MAKE) db-up", strings.TrimSpace(r.receita[0]), "db-up é a 1a linha da receita")
	receita := strings.Join(r.receita, " ")
	for _, v := range []string{"CONFIRMAR", "CONFIRMO_PRODUCAO", "ENV_ARQUIVO"} {
		assert.NotContains(t, receita, v, "db-seed: %s não é repassado à mão", v)
	}
}

func TestBUG28_Makefile_KSeedRecusadoNaoRodaDbUp(t *testing.T) {
	casos := map[string][]string{
		"produção com CONFIRMO_PRODUCAO":  {"DB_HOST=192.168.168.106", "DB_PORT=3306", "CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes"},
		"container com CONFIRMO_PRODUCAO": {"DB_HOST=rotaperfumes-mariadb", "DB_PORT=3306", "CONFIRMO_PRODUCAO=rotaperfumes-mariadb/rotaperfumes"},
	}
	for nome, extra := range casos {
		for _, flags := range [][]string{{"-k"}, {"-k", "-j4"}} {
			t.Run(nome+"/"+strings.Join(flags, " "), func(t *testing.T) {
				dir, path, cliente, log := bug27Dir(t, false)
				args := append(append([]string{}, flags...), bug27Args(cliente, extra...)...)
				out, err := makeAlvo(t, dir, "db-seed", []string{path}, args...)
				require.Error(t, err, out)
				assert.Contains(t, out, "mesmo com CONFIRMO_PRODUCAO")
				assert.Empty(t, lerSeExiste(t, log), "nenhum CREATE DATABASE/DDL/seed com a guarda recusada")
			})
		}
	}
}

func TestBUG28_Makefile_SeedOrdemNoLoopback(t *testing.T) {
	for _, flags := range [][]string{nil, {"-j4"}, {"-k", "-j8"}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			dir, path, cliente, log := bug27Dir(t, false)
			args := append(append([]string{}, flags...), bug27Args(cliente, "DB_HOST=127.0.0.1", "DB_PORT=3306")...)
			out, err := makeAlvo(t, dir, "db-seed", []string{path}, args...)
			require.NoError(t, err, out)
			assert.Equal(t, bug27Seed, bug27Etapas(lerSeExiste(t, log)), "CREATE -> DDL -> seed -> resetpassword")
		})
	}
}

func TestBUG28_Makefile_DbUpFalhouNaoSemeia(t *testing.T) {
	dir, path, cliente, log := bug27Dir(t, false)
	// O cliente falso falha no CREATE DATABASE: o seed não pode seguir.
	script := "#!/bin/sh\n[ \"$1\" = --version ] && { echo 'mariadb from 11.4.5-MariaDB, client 15.2'; exit 0; }\n" +
		"case \"$*\" in *CREATE*) echo \"mariadb $*\" >> '" + log + "'; exit 1 ;; esac\n" +
		"echo \"mariadb $(head -n 1)\" >> '" + log + "'\n"
	require.NoError(t, os.WriteFile(cliente, []byte(script), 0o755))
	args := append([]string{"-k", "-j4"}, bug27Args(cliente, "DB_HOST=127.0.0.1", "DB_PORT=3306")...)
	out, err := makeAlvo(t, dir, "db-seed", []string{path}, args...)
	require.Error(t, err, out)
	assert.Equal(t, []string{"CREATE"}, bug27Etapas(lerSeExiste(t, log)), "nada depois do db-up que falhou")
}

func TestBUG29_Makefile_MakeflagsNaLinhaDeComandoRecusado(t *testing.T) {
	for _, v := range []string{"MAKEFLAGS=-k", "MAKEFLAGS=", "MAKEOVERRIDES="} {
		for _, alvo := range []string{"db-reset", "test-integration", "help"} {
			t.Run(v+"/"+alvo, func(t *testing.T) {
				dir, path, cliente, log := bug27Dir(t, false)
				args := append(bug27Args(cliente, "DB_HOST=127.0.0.1", "DB_PORT=3306", "ENV_ARQUIVO=/dev/null"), v)
				out, err := makeAlvo(t, dir, alvo, []string{path}, args...)
				require.Error(t, err, out)
				assert.Contains(t, out, "BUG-29: MAKEFLAGS/MAKEOVERRIDES na linha de comando")
				assert.Empty(t, lerSeExiste(t, log), "nada roda")
			})
		}
	}
}

// MAKEFLAGS no ambiente (a forma indicada na mensagem) continua repassando o
// ENV_ARQUIVO aos sub-makes: o .env padrão não é lido.
func TestBUG29_Makefile_MakeflagsNoAmbienteMantemEnvArquivo(t *testing.T) {
	dir, path, cliente, log := bug27Dir(t, false)
	comum := "DB_HOST=127.0.0.1\nDB_PORT=3306\nDB_TLS_REQUIRED=false\nDB_USUARIO=u-bug29\nDB_SENHA=s-bug29\nDB_CLIENTE_BIN=" + cliente + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), []byte(comum+"DB_NAME=rp_env_padrao\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "integ.env"), []byte(comum+"DB_NAME=rp_bug29\n"), 0o600))
	for _, mf := range []string{"MAKEFLAGS=-k", "MAKEFLAGS=k"} {
		t.Run(mf, func(t *testing.T) {
			require.NoError(t, os.WriteFile(log, nil, 0o600))
			out, err := makeAlvo(t, dir, "db-reset", []string{path, mf}, "ENV_ARQUIVO=integ.env")
			require.NoError(t, err, out)
			texto := lerSeExiste(t, log)
			assert.Contains(t, texto, "DROP DATABASE IF EXISTS rp_bug29")
			assert.Contains(t, texto, "CREATE DATABASE IF NOT EXISTS rp_bug29")
			assert.NotContains(t, texto, "rp_env_padrao", "sub-make não cai no .env padrão")
		})
	}
}
