package config_test

// TST-25 (2026-11-14): o "make test-integration" usa o próprio banco
// descartável (DB_TESTE = rotaperfumes_teste): apaga as sobras, cria o schema,
// roda os testes e apaga o banco no fim, mesmo com falha. O DB_NAME do
// arquivo (dev/produção) nunca é tocado. Harness do SEC-53 (go e mariadb
// falsos).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tst25Etapas reduz mariadb.log + env.log às etapas, na ordem em que rodaram.
func tst25Etapas(t *testing.T, dir string) []string {
	t.Helper()
	var etapas []string
	for _, l := range strings.Split(strings.TrimSpace(lerSeExiste(t, filepath.Join(dir, "etapas.log"))), "\n") {
		switch {
		case strings.Contains(l, "DROP DATABASE IF EXISTS rotaperfumes_teste;"):
			etapas = append(etapas, "DROP")
		case strings.Contains(l, "CREATE DATABASE IF NOT EXISTS rotaperfumes_teste "):
			etapas = append(etapas, "CREATE")
		case strings.Contains(l, "rotaperfumes_teste -- DDL-A"):
			etapas = append(etapas, "DDL")
		case strings.HasPrefix(l, "go test"):
			etapas = append(etapas, "GO-TEST")
		case strings.Contains(l, "mariadb") || strings.HasPrefix(l, "go "):
			if !strings.HasPrefix(l, "go build") {
				etapas = append(etapas, "OUTRO: "+l)
			}
		}
	}
	return etapas
}

// tst25Dir: harness do SEC-53 com go e mariadb gravando também em etapas.log
// (ordem global); o go test falha quando falharTeste.
func tst25Dir(t *testing.T, falharTeste bool) (dir, path, envLog string) {
	t.Helper()
	dir, path, envLog = sec53Dir(t)
	etapas := filepath.Join(dir, "etapas.log")
	falha := ""
	if falharTeste {
		falha = "case \"$1\" in test) exit 1 ;; esac\n"
	}
	goFalso := "#!/bin/sh\n{ echo \"=== go $*\"; env; } >> '" + envLog + "'\necho \"go $*\" >> '" + etapas + "'\n" + falha + "exit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bin-falso", "go"), []byte(goFalso), 0o755))
	mariadb := "#!/bin/sh\n[ \"$1\" = --version ] && { echo 'mariadb from 11.4.5-MariaDB, client 15.2'; exit 0; }\n" +
		"case \"$*\" in *' -e '*) echo \"mariadb $*\" >> '" + etapas + "' ;; *) echo \"mariadb $* $(head -n 1)\" >> '" + etapas + "' ;; esac\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bin-falso", "mariadb"), []byte(mariadb), 0o755))
	return dir, path, envLog
}

var tst25Ordem = []string{"DROP", "CREATE", "DDL", "GO-TEST", "GO-TEST", "DROP"}

func TestTST25_Makefile_BancoProprioNaOrdem(t *testing.T) {
	dir, path, envLog := tst25Dir(t, false)
	out, err := makeAlvo(t, dir, "test-integration", []string{path}, "DB_NAME=rotaperfumes")
	require.NoError(t, err, out)
	assert.Equal(t, tst25Ordem, tst25Etapas(t, dir), "apaga sobras -> schema -> testes (shared e API) -> apaga")
	_, texto := varsDoLog(t, envLog)
	for _, bloco := range strings.Split(texto, "=== go ")[1:] {
		if strings.HasPrefix(bloco, "test") {
			assert.Contains(t, bloco, "\nDB_NAME=rotaperfumes_teste\n", "o go test usa o banco descartável")
			assert.NotContains(t, bloco, "\nDB_NAME=rotaperfumes\n")
		}
	}
	assert.Contains(t, out, "banco descartavel rotaperfumes_teste")
}

func TestTST25_Makefile_FalhaNosTestesAindaApaga(t *testing.T) {
	for _, flags := range [][]string{nil, {"-k"}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			dir, path, _ := tst25Dir(t, true)
			out, err := makeAlvo(t, dir, "test-integration", []string{path}, append(flags, "DB_NAME=rotaperfumes")...)
			require.Error(t, err, out)
			etapas := tst25Etapas(t, dir)
			require.NotEmpty(t, etapas)
			assert.Equal(t, []string{"DROP", "CREATE", "DDL", "GO-TEST"}, etapas[:4])
			assert.Equal(t, "DROP", etapas[len(etapas)-1], "o banco é apagado mesmo com o teste falhando")
		})
	}
}

func TestTST25_Makefile_DbTesteRecusado(t *testing.T) {
	casos := map[string][]string{
		"igual ao DB_NAME":         {"DB_NAME=rotaperfumes_teste"},
		"igual ao DB_NAME (caixa)": {"DB_NAME=RotaPerfumes_Teste"},
		"sem _teste":               {"DB_NAME=rotaperfumes", "DB_TESTE=rotaperfumes"},
		"maiúsculas":               {"DB_NAME=rotaperfumes", "DB_TESTE=Rotaperfumes_teste"},
		"injeção":                  {"DB_NAME=rotaperfumes", "DB_TESTE=x_teste;DROP"},
		"vazio":                    {"DB_NAME=rotaperfumes", "DB_TESTE="},
	}
	for nome, args := range casos {
		t.Run(nome, func(t *testing.T) {
			dir, path, _ := tst25Dir(t, false)
			out, err := makeAlvo(t, dir, "test-integration", []string{path}, args...)
			require.Error(t, err, out)
			assert.Contains(t, out, "TST-25: DB_TESTE=")
			assert.Empty(t, lerSeExiste(t, filepath.Join(dir, "etapas.log")), "nada roda com o DB_TESTE recusado")
		})
	}
}

// DB_TESTE no .env (ou no ambiente) não vale: só a linha de comando muda.
func TestTST25_Makefile_DbTesteDoArquivoIgnorado(t *testing.T) {
	dir, path, _ := tst25Dir(t, false)
	f, err := os.OpenFile(filepath.Join(dir, "backend", ".env"), os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = f.WriteString("DB_TESTE=outro_teste\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	out, err := makeAlvo(t, dir, "test-integration", []string{path, "DB_TESTE=ambiente_teste"}, "DB_NAME=rotaperfumes")
	require.NoError(t, err, out)
	assert.Equal(t, tst25Ordem, tst25Etapas(t, dir))
}

func TestTST25_Makefile_ProducaoRecusadaAntesDeTudo(t *testing.T) {
	for _, flags := range [][]string{nil, {"-k"}} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			dir, path, _ := tst25Dir(t, false)
			args := append(flags, "DB_NAME=rotaperfumes", "DB_HOST=192.168.168.106", "DB_PORT=3306",
				"CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes", "DB_TLS_REQUIRED=false")
			out, err := makeAlvo(t, dir, "test-integration", []string{path}, args...)
			require.Error(t, err, out)
			assert.Empty(t, lerSeExiste(t, filepath.Join(dir, "etapas.log")), "nenhum DROP/CREATE/go test na produção")
		})
	}
}

func TestTST25_Makefile_Contrato(t *testing.T) {
	mk := lerMakefile(t)
	assert.True(t, mk.phony["db-guard-banco-teste"])
	assert.Equal(t, "DB_TESTE := rotaperfumes_teste", strings.TrimSpace(mk.linhaDaVariavel(t, "DB_TESTE")))
	receita := strings.Join(mk.regras["test-integration"].receita, "\n")
	for _, v := range []string{"CONFIRMAR", "CONFIRMO_PRODUCAO", "ENV_ARQUIVO"} {
		assert.NotContains(t, receita, v, "%s não é repassado à mão (BUG-27)", v)
	}
	for _, trecho := range []string{
		"$(MAKE) db-down DB_NAME=$(DB_TESTE) && $(MAKE) db-up DB_NAME=$(DB_TESTE) &&",
		`$(MAKE) test INTEGRATION=1 GO_TEST_FLAGS="-p 1 -count=1" DB_NAME=$(DB_TESTE) || rc=$$?`,
		"$(MAKE) db-down DB_NAME=$(DB_TESTE) || {",
		"exit $$rc",
	} {
		assert.Contains(t, receita, trecho)
	}
	assert.Equal(t, 2, strings.Count(receita, "$(MAKE) db-down DB_NAME=$(DB_TESTE)"), "apaga antes e depois")
}
