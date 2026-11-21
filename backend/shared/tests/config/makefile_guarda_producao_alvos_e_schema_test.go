package config_test

// DEPLOY-07 (Lote 33): Makefile.
//   - sql/schema.sql: tabelas com as FKs na ordem; o db-up o aplica (CHORE-20);
//   - db-guard-producao: dependência dos alvos que escrevem no banco; recusa
//     produção por host+porta (o dev do CasaOS, mesmo host na 3307, passa) salvo CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME> na linha
//     de comando (no ambiente não vale);
//   - nenhuma senha fixa (Admin@123) e fix-hash/fix-admin com -password-prompt.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tabelasDoSchema devolve, na ordem do sql/schema.sql, cada tabela e as tabelas
// que ela referencia por FK.
func tabelasDoSchema(t *testing.T) ([]string, map[string][]string) {
	t.Helper()
	texto := lerTexto(t, filepath.Join(raizProjeto(t), "sql", "schema.sql"))
	reTabela := regexp.MustCompile("(?m)^CREATE TABLE IF NOT EXISTS `?(\\w+)`?")
	reRef := regexp.MustCompile("REFERENCES `?(\\w+)`?")
	var ordem []string
	refs := map[string][]string{}
	idx := reTabela.FindAllStringSubmatchIndex(texto, -1)
	for i, m := range idx {
		nome := texto[m[2]:m[3]]
		fim := len(texto)
		if i+1 < len(idx) {
			fim = idx[i+1][0]
		}
		ordem = append(ordem, nome)
		for _, r := range reRef.FindAllStringSubmatch(texto[m[0]:fim], -1) {
			refs[nome] = append(refs[nome], r[1])
		}
	}
	return ordem, refs
}

// CHORE-20: o schema completo é um só arquivo (sql/schema.sql), com as tabelas
// referenciadas por FK antes de quem as referencia; o db-up o aplica.
func TestDEPLOY07_SchemaSQL(t *testing.T) {
	root := raizProjeto(t)
	ordem, refs := tabelasDoSchema(t)
	require.GreaterOrEqual(t, len(ordem), 14)
	pos := map[string]int{}
	for i, nome := range ordem {
		_, dup := pos[nome]
		assert.False(t, dup, "%s repetida", nome)
		pos[nome] = i
	}
	for nome, alvos := range refs {
		for _, alvo := range alvos {
			if alvo == nome {
				continue
			}
			require.Contains(t, pos, alvo, "%s referencia %s, que não está no schema", nome, alvo)
			assert.Less(t, pos[alvo], pos[nome], "%s antes de %s (FK)", alvo, nome)
		}
	}
	antigos, err := filepath.Glob(filepath.Join(root, "sql", "[0-9][0-9]_*.sql"))
	require.NoError(t, err)
	assert.Empty(t, antigos, "sem DDL/alter numerados: schema.sql + migracao.sql")
	assert.NoFileExists(t, filepath.Join(root, "sql", "ordem-ddl.txt"))

	mk := lerMakefile(t)
	receita := strings.Join(mk.regras["db-up"].receita, "\n")
	assert.Contains(t, receita, "$(DB_CLI) $(DB_NAME) < sql/schema.sql")
	assert.Contains(t, receita, "test -s sql/schema.sql")
	assert.Contains(t, strings.Join(mk.regras["db-migrar"].receita, "\n"), "$(DB_CLI) $(DB_NAME) < sql/migracao.sql")
}

func TestDEPLOY07_Makefile_GuardaDeProducaoNosAlvos(t *testing.T) {
	mk := lerMakefile(t)
	require.Contains(t, mk.regras, "db-guard-producao")
	assert.True(t, mk.phony["db-guard-producao"])
	assert.False(t, mk.dependeDe("db-check-env", "db-guard-producao"), "db-check-env continua só conferindo o ambiente")

	alvos := []string{"db-down", "db-up", "db-create", "db-migrar", "db-seed", "db-reset", "db-rebuild", "db-seed-mes-atual",
		"test-integration", "gen-hash", "reset-senhas-ativos", "db-gerar-imagens-produtos"}
	for _, a := range mk.ordem {
		if strings.HasPrefix(a, "db-import-") || strings.HasPrefix(a, "fix-") {
			alvos = append(alvos, a)
		}
	}
	require.GreaterOrEqual(t, len(alvos), 20)
	for _, a := range alvos {
		t.Run(a, func(t *testing.T) {
			require.Contains(t, mk.regras, a)
			assert.Contains(t, mk.regras[a].prereqs, "db-guard-producao", "%s depende direto da guarda", a)
		})
	}
	for _, a := range []string{"help", "build", "test", "test-shared", "test-api", "lint", "db-check-env", "db-export", "reset-senhas-simular"} {
		assert.False(t, mk.dependeDe(a, "db-guard-producao"), "%s não precisa da guarda", a)
	}
	lan := strings.Fields(strings.SplitN(mk.linhaDaVariavel(t, "PRODUCAO_HOSTS_LAN"), "=", 2)[1])
	rede := strings.Fields(strings.SplitN(mk.linhaDaVariavel(t, "PRODUCAO_HOSTS_REDE"), "=", 2)[1])
	assert.Contains(t, lan, "192.168.168.106")
	assert.Contains(t, rede, "rotaperfumes-mariadb")
	assert.Contains(t, rede, "172.30.42.2")
}

func TestDEPLOY07_Makefile_SemSenhaFixa(t *testing.T) {
	mk := lerMakefile(t)
	assert.NotContains(t, mk.texto, "Admin@123")
	assert.NotRegexp(t, `resetpassword[^\n]*-password=`, mk.texto)
	for _, a := range []string{"fix-hash", "fix-admin"} {
		receita := strings.Join(mk.regras[a].receita, "\n")
		assert.Contains(t, receita, "-create-admin -password-prompt", a)
		assert.NotContains(t, receita, "@echo.", "%s: echo. não existe no sh", a)
	}
}

// deploy07Make roda "make db-guard-producao" numa cópia do Makefile, sem .env.
func deploy07Make(t *testing.T, env []string, args ...string) (string, error) {
	t.Helper()
	return makeAlvo(t, copiaDoMakefile(t), "db-guard-producao", env, args...)
}

// copiaDoMakefile copia o Makefile do projeto para um diretório temporário
// (sem backend/.env, sem sql/; backend/ vazio, onde os testes gravam o .env
// falso, ENV-01): nada ali conecta em banco.
// dirTempMake (SEC-63): o t.TempDir() carrega o nome do subteste, com "(", "=" e
// acentos; esse caminho vira DB_CLIENTE_BIN, que o Makefile recusaria.
func dirTempMake(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "rp-make-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func copiaDoMakefile(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("usa sh")
	}
	dir := dirTempMake(t)
	dados, err := os.ReadFile(filepath.Join(raizProjeto(t), "Makefile"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Makefile"), dados, 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "backend"), 0o755))
	return dir
}

// makeAlvo roda "make -s <alvo> args..." em dir com ambiente mínimo (PATH,
// HOME=dir, LC_ALL=C) mais env; nada do ambiente do go test (MAKEFLAGS,
// DB_*, .env real) vaza para o make.
func makeAlvo(t *testing.T, dir, alvo string, env []string, args ...string) (string, error) {
	t.Helper()
	makeBin, err := exec.LookPath("make")
	if err != nil {
		t.Skip("make não encontrado")
	}
	cmd := exec.Command(makeBin, append([]string{"-s", alvo}, args...)...)
	cmd.Dir = dir
	cmd.Env = append([]string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "LC_ALL=C"}, env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// casoGuardaProducao: produção = host do servidor na porta 3306 (ou sem
// porta) ou o nome/IP do container na rede docker (qualquer porta). O dev do
// CasaOS é o mesmo host na porta 3307 e passa. A mesma tabela roda contra o
// db-guard-producao do Makefile (DEPLOY-07) e contra o
// integracao.ValidarNaoProducao dos testes Go (SEC-50).
type casoGuardaProducao struct {
	host, porta string // porta "" = DB_PORT vazio na linha de comando
	recusa      bool
}

var casosGuardaProducao = []casoGuardaProducao{
	{"192.168.168.106", "3306", true},
	{"192.168.168.106", "", true},
	{"192.168.168.106", "3307", false},
	{"192.168.168.106", "03306", true}, // porta como número (o cliente conecta na 3306)
	{"192.168.168.106", "+3306", true},
	{"192.168.168.106", "03307", false},
	{"IVO-INSPIRON-15-3530", "3306", true},
	{"ivo-inspiron-15-3530", "3307", false},
	{"rotaperfumes-mariadb", "3306", true},
	{"rotaperfumes-mariadb", "3307", true},
	{"ROTAPERFUMES-MARIADB.", "3306", true},
	{"172.30.42.2", "3306", true},
	{"172.30.42.2", "13306", true},
	{"127.0.0.1", "3306", false},
	{"localhost", "3306", false},
	{"192.168.168.107", "3306", false},
}

func TestDEPLOY07_Makefile_GuardaRecusaProducao(t *testing.T) {
	casos := casosGuardaProducao
	for _, c := range casos {
		t.Run(c.host+":"+c.porta, func(t *testing.T) {
			out, err := deploy07Make(t, nil, "DB_HOST="+c.host, "DB_PORT="+c.porta)
			if c.recusa {
				require.Error(t, err, out)
				assert.Contains(t, out, "e o banco de PRODUCAO")
				assert.Contains(t, out, "deploy/seed-producao.sh")
			} else {
				assert.NoError(t, err, out)
				assert.NotContains(t, out, "PRODUCAO")
			}
		})
	}
	t.Run("confirmado na linha de comando", func(t *testing.T) {
		out, err := deploy07Make(t, nil, "DB_HOST=192.168.168.106", "DB_PORT=3306", "DB_NAME=rotaperfumes", "CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes")
		assert.NoError(t, err, out)
		assert.Contains(t, out, "AVISO (DEPLOY-07)")
	})
	t.Run("confirmação errada", func(t *testing.T) {
		out, err := deploy07Make(t, nil, "DB_HOST=192.168.168.106", "DB_PORT=3306", "DB_NAME=rotaperfumes", "CONFIRMO_PRODUCAO=192.168.168.106/outro")
		assert.Error(t, err, out)
	})
	t.Run("confirmação pelo ambiente não vale", func(t *testing.T) {
		out, err := deploy07Make(t, []string{"CONFIRMO_PRODUCAO=192.168.168.106/rotaperfumes"}, "DB_HOST=192.168.168.106", "DB_PORT=3306", "DB_NAME=rotaperfumes")
		assert.Error(t, err, out)
	})
}

// ENV_ARQUIVO (só pela linha de comando) troca o .env por outro arquivo.
func TestDEPLOY07_Makefile_EnvArquivo(t *testing.T) {
	mk := lerMakefile(t)
	assert.Regexp(t, regexp.MustCompile(`(?m)^ifeq \(\$\(origin ENV_ARQUIVO\),command line\)\n(#.*\n)*include \$\(ENV_ARQUIVO\)\nelse\n-include backend/\.env\nendif$`), mk.texto)
}
