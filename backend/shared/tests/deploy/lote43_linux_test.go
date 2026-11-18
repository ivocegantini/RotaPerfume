package deploy_test

// Lote 43 (tarefas/lote-43-spec.md): BACKUP-03 (folga de relógio de 300 s nas
// guardas), BACKUP-04 (parcial-* vale na guarda, com aviso), BACKUP-05 (a
// retenção relê os pares que ficam), BACKUP-07 (--trocar-dados) e TST-28
// (-timeout=30m). Usa os ensaios com stub do BACKUP-01; nada fala com Docker.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const avisoParcial = "o backup usado e parcial"

// ---------------------------------------------------------------------------
// BACKUP-03 / BACKUP-04: paridade estática
// ---------------------------------------------------------------------------

func TestLOTE43_GuardaParidade(t *testing.T) {
	reTol := regexp.MustCompile(`(?m)^readonly TOLERANCIA_RELOGIO_S=(\d+)$`)
	for _, s := range []string{"backup-banco.sh", "mariadb-docker.sh", "seed-producao.sh", "gerar-imagens-producao.sh"} {
		m := reTol.FindAllStringSubmatch(lerRaiz(t, "deploy", s), -1)
		require.Len(t, m, 1, s)
		assert.Equal(t, "300", m[0][1], s)
	}
	for _, s := range []string{"mariadb-docker.sh", "seed-producao.sh", "gerar-imagens-producao.sh"} {
		f := funcaoShell(t, lerRaiz(t, "deploy", s), "conferir_backup_recente")
		assert.Contains(t, f, "agora + TOLERANCIA_RELOGIO_S", s)
		assert.NotContains(t, f, "f <= agora ))", s)
	}
	// BACKUP-04: o --recriar e o seed aceitam o parcial; o gerar-imagens (grava
	// no volume de imagens) continua sem aceitar.
	assert.Contains(t, funcaoShell(t, lerRaiz(t, "deploy", "mariadb-docker.sh"), "conferir_backup_recente"), "parcial-rotaperfumes-")
	assert.NotContains(t, funcaoShell(t, lerRaiz(t, "deploy", "gerar-imagens-producao.sh"), "conferir_backup_recente"), "parcial-rotaperfumes-")
}

// ---------------------------------------------------------------------------
// BACKUP-03 / BACKUP-04: --recriar
// ---------------------------------------------------------------------------

func TestLOTE43_GuardaRecriar_Bordas(t *testing.T) {
	registro := func(e *ensaio) string { return e.arq(filepath.Join(segredos, "recriar.registro")) }
	futuroMtime := func(d time.Duration) func(t *testing.T, dir string) string {
		return func(t *testing.T, dir string) string {
			p := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz")
			f := time.Now().Add(d)
			require.NoError(t, os.Chtimes(p, f, f))
			return p
		}
	}
	aceitos := map[string]func(t *testing.T, dir string) string{
		"carimbo mais 240s": func(t *testing.T, dir string) string {
			return dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(240*time.Second)+".sql.gz")
		},
		"mtime mais 240s": futuroMtime(240 * time.Second),
		"parcial sozinho": func(t *testing.T, dir string) string {
			return dumpRecente(t, dir, "parcial-rotaperfumes-"+carimboAgora(0)+".sql.gz")
		},
		"parcial manual": func(t *testing.T, dir string) string {
			return dumpRecente(t, dir, "parcial-rotaperfumes-"+carimboAgora(0)+"-manual.sql.gz")
		},
		"parcial mais novo que o completo": func(t *testing.T, dir string) string {
			dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(-time.Minute)+".sql.gz")
			return dumpRecente(t, dir, "parcial-rotaperfumes-"+carimboAgora(0)+".sql.gz")
		},
		"mesmo carimbo: vence o completo": func(t *testing.T, dir string) string {
			c := carimboAgora(0)
			dumpRecente(t, dir, "parcial-rotaperfumes-"+c+".sql.gz")
			return dumpRecente(t, dir, "rotaperfumes-"+c+".sql.gz")
		},
		"pre-restauracao mais novo segue ignorado": func(t *testing.T, dir string) string {
			p := dumpRecente(t, dir, "parcial-rotaperfumes-"+carimboAgora(-time.Minute)+".sql.gz")
			dumpRecente(t, dir, "pre-restauracao-"+carimboAgora(0)+".sql.gz")
			return p
		},
	}
	for nome, preparar := range aceitos {
		t.Run("aceita "+nome, func(t *testing.T) {
			e, dir := novaGuarda(t)
			p := preparar(t, dir)
			r := e.rodar("--recriar", "--backup-dir", dir)
			require.NoError(t, r.err, r.out)
			assert.Contains(t, registro(e), "backup="+p+" ")
			if strings.HasPrefix(filepath.Base(p), "parcial-") {
				assert.Contains(t, r.out, avisoParcial)
			} else {
				assert.NotContains(t, r.out, avisoParcial)
			}
		})
	}
	recusas := map[string]struct {
		preparar func(t *testing.T, dir string)
		esperado string
	}{
		"carimbo mais 360s": {func(t *testing.T, dir string) {
			dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(360*time.Second)+".sql.gz")
		}, "data futura (mais de 300 s"},
		"mtime mais 360s": {func(t *testing.T, dir string) { futuroMtime(360*time.Second)(t, dir) }, "data futura (mais de 300 s"},
		"parcial corrompido mais novo nao recua": {func(t *testing.T, dir string) {
			dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(-time.Minute)+".sql.gz")
			escrever(t, filepath.Join(dir, "parcial-rotaperfumes-"+carimboAgora(0)+".sql.gz"), "nao e gzip", 0o600)
		}, "corrompido"},
		"parcial em symlink": {func(t *testing.T, dir string) {
			alvo := dumpRecente(t, filepath.Dir(dir), "alvo.sql.gz")
			require.NoError(t, os.Symlink(alvo, filepath.Join(dir, "parcial-rotaperfumes-"+carimboAgora(0)+".sql.gz")))
		}, "nao e arquivo regular"},
		"parcial 644": {func(t *testing.T, dir string) {
			require.NoError(t, os.Chmod(dumpRecente(t, dir, "parcial-rotaperfumes-"+carimboAgora(0)+".sql.gz"), 0o644))
		}, "modo 600"},
		"parcial sem marcador": {func(t *testing.T, dir string) {
			escrever(t, filepath.Join(dir, "parcial-rotaperfumes-"+carimboAgora(0)+".sql.gz"), comprimir(t, "CREATE TABLE x;\n"), 0o600)
		}, "marcador"},
		"parcial futuro mais 360s": {func(t *testing.T, dir string) {
			dumpRecente(t, dir, "parcial-rotaperfumes-"+carimboAgora(360*time.Second)+".sql.gz")
		}, "data futura"},
		"parcial antigo": {func(t *testing.T, dir string) {
			p := dumpRecente(t, dir, "parcial-rotaperfumes-"+carimboAgora(-2*time.Hour)+".sql.gz")
			v := time.Now().Add(-2 * time.Hour)
			require.NoError(t, os.Chtimes(p, v, v))
		}, "tem mais de 60 min"},
	}
	for nome, c := range recusas {
		t.Run("recusa "+nome, func(t *testing.T) {
			e, dir := novaGuarda(t)
			c.preparar(t, dir)
			r := e.rodar("--recriar", "--backup-dir", dir)
			require.Error(t, r.err, r.out)
			assert.Contains(t, r.out, c.esperado)
			assert.NotContains(t, e.log(), "docker rm", "sem docker rm")
			assert.NoFileExists(t, filepath.Join(e.dir, segredos, "recriar.registro"))
		})
	}
}

// ---------------------------------------------------------------------------
// BACKUP-03 / BACKUP-04: seed --forcar (mesma função)
// ---------------------------------------------------------------------------

func TestLOTE43_GuardaSeed_Bordas(t *testing.T) {
	aceitos := map[string]string{
		"parcial":        "parcial-rotaperfumes-" + carimboAgora(0) + ".sql.gz",
		"carimbo mais 240s": "rotaperfumes-" + carimboAgora(240*time.Second) + ".sql.gz",
	}
	for nome, arq := range aceitos {
		t.Run("aceita "+nome, func(t *testing.T) {
			e, dir := novoSeedPreenchido(t)
			p := dumpRecente(t, dir, arq)
			s := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-dir", dir)
			s.digitar(promptNomeDoBanco, "rotaperfumes")
			out, err := s.terminar()
			require.NoError(t, err, out)
			assert.Contains(t, e.ler(registroSeed), "backup="+p)
			assert.Equal(t, strings.HasPrefix(arq, "parcial-"), strings.Contains(out, avisoParcial))
		})
	}
	recusas := map[string]struct {
		preparar func(t *testing.T, dir string)
		esperado string
	}{
		"carimbo mais 360s": {func(t *testing.T, dir string) {
			dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(360*time.Second)+".sql.gz")
		}, "data futura (mais de 300 s"},
		"parcial corrompido mais novo nao recua": {func(t *testing.T, dir string) {
			dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(-time.Minute)+".sql.gz")
			escrever(t, filepath.Join(dir, "parcial-rotaperfumes-"+carimboAgora(0)+".sql.gz"), "nao e gzip", 0o600)
		}, "corrompido"},
	}
	for nome, c := range recusas {
		t.Run("recusa "+nome, func(t *testing.T) {
			e, dir := novoSeedPreenchido(t)
			c.preparar(t, dir)
			out, err := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-dir", dir).terminar()
			require.Error(t, err, out)
			assert.Contains(t, out, c.esperado)
			assert.NotContains(t, out, promptNomeDoBanco)
			e.semSQLDeEscrita(t)
			assert.NoFileExists(t, filepath.Join(e.dir, registroSeed))
		})
	}
}

// ---------------------------------------------------------------------------
// BACKUP-05: a retenção relê todos os pares que ficam
// ---------------------------------------------------------------------------

func TestLOTE43_Retencao_ReleOsQueFicam(t *testing.T) {
	const d1, d2, d3, d4 = "20260101-033000Z", "20260102-033000Z", "20260103-033000Z", "20260104-033000Z"
	semMarcador := comprimir(t, strings.TrimSuffix(dumpBackupBom(), marcadorDump))
	novo := func(t *testing.T) *ensaioBackup {
		e := novoEnsaioBackup(t)
		e.instalarCopia("RETENCAO_DIARIOS=3\n")
		for _, c := range []string{d1, d2, d3, d4} {
			e.par(c, "")
		}
		return e
	}
	for nome, c := range map[string]struct{ arquivo, conteudo string }{
		"dump com gzip ruim":    {"rotaperfumes-" + d3 + ".sql.gz", "nao e gzip"},
		"dump sem marcador":     {"rotaperfumes-" + d3 + ".sql.gz", semMarcador},
		"imagens com gzip ruim": {"rotaperfumes-imagens-" + d3 + ".tar.gz", "nao e gzip"},
	} {
		t.Run("meio mantido corrompido: "+nome, func(t *testing.T) {
			e := novo(t)
			escrever(t, filepath.Join(e.backups(), c.arquivo), c.conteudo, 0o600)
			antes := e.arquivos()
			out, rc := e.instalada("--agendado")
			assert.Equal(t, 1, rc, out)
			assert.Contains(t, out, "o par rotaperfumes-"+d3+".sql.gz foi reprovado")
			assert.Contains(t, out, "mova o par (dump e imagens) para fora de")
			assert.NotContains(t, out, "par removido")
			assert.Subset(t, e.arquivos(), antes, "nada apagado")
			assert.FileExists(t, e.caminhoDump(d1))
			assert.FileExists(t, e.caminhoDump(d2))
			assert.Len(t, e.pares(), 5, "o par novo fica")
		})
	}
	t.Run("corrompido que sai nao e relido", func(t *testing.T) {
		e := novo(t)
		escrever(t, e.caminhoDump(d1), "nao e gzip", 0o600)
		out, rc := e.instalada("--agendado")
		require.Equal(t, 0, rc, out)
		assert.NoFileExists(t, e.caminhoDump(d1))
		assert.NoFileExists(t, e.caminhoDump(d2))
		assert.FileExists(t, e.caminhoDump(d3))
	})
	t.Run("todos bons", func(t *testing.T) {
		e := novo(t)
		out, rc := e.instalada("--agendado")
		require.Equal(t, 0, rc, out)
		assert.NoFileExists(t, e.caminhoDump(d1))
		assert.NoFileExists(t, e.caminhoDump(d2))
		assert.Len(t, e.pares(), 3)
	})
}

// ---------------------------------------------------------------------------
// BACKUP-07: --trocar-dados
// ---------------------------------------------------------------------------

func TestLOTE43_TrocarDados(t *testing.T) {
	registro := func(e *ensaio) string { return e.arq(filepath.Join(segredos, "recriar.registro")) }
	semMudanca := func(t *testing.T, e *ensaio, r saidaEnsaio, esperado string) {
		t.Helper()
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, esperado)
		assert.NotContains(t, e.log(), "docker rm", "sem docker rm")
		assert.NotContains(t, e.log(), "docker build", "nada mudado antes")
		assert.NoFileExists(t, filepath.Join(e.dir, segredos, "recriar.registro"))
	}
	novoDir := func(t *testing.T, e *ensaio, populado bool) string {
		d := filepath.Join(e.dir, "dados-novo")
		require.NoError(t, os.MkdirAll(d, 0o700))
		if populado {
			require.NoError(t, os.MkdirAll(filepath.Join(d, "mysql"), 0o700))
		}
		return d
	}
	t.Run("recriar com --dados vazio e backup recente: recusa", func(t *testing.T) {
		e, dir := novaGuarda(t)
		dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz")
		semMudanca(t, e, e.rodar("--recriar", "--backup-dir", dir, "--dados", novoDir(t, e, false)), "difere do datadir atual")
	})
	t.Run("recriar com --dados diferente e --sem-backup-motivo: recusa", func(t *testing.T) {
		e, dir := novaGuarda(t)
		semMudanca(t, e, e.rodar("--recriar", "--backup-dir", dir, "--dados", novoDir(t, e, true),
			"--sem-backup-motivo", "motivo com tamanho bom"), "difere do datadir atual")
	})
	t.Run("recriar com o caminho original: aceito", func(t *testing.T) {
		e, dir := novaGuarda(t)
		dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz")
		r := e.rodar("--recriar", "--backup-dir", dir, "--dados", filepath.Join(e.dir, "dados")+"/")
		require.NoError(t, r.err, r.out)
		assert.NotContains(t, registro(e), "dados_antes=")
	})
	t.Run("trocar para vazio: nunca", func(t *testing.T) {
		e, dir := novaGuarda(t)
		dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz")
		semMudanca(t, e, e.rodar("--recriar", "--trocar-dados", "--backup-dir", dir, "--dados", novoDir(t, e, false)), "nunca e aceito")
	})
	t.Run("trocar para populado sem backup: nenhum backup", func(t *testing.T) {
		e, dir := novaGuarda(t)
		semMudanca(t, e, e.rodar("--recriar", "--trocar-dados", "--backup-dir", dir, "--dados", novoDir(t, e, true)), "nenhum backup")
	})
	t.Run("trocar para populado com backup: aceito e registrado", func(t *testing.T) {
		e, dir := novaGuarda(t)
		dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz")
		novo := novoDir(t, e, true)
		r := e.rodar("--recriar", "--trocar-dados", "--backup-dir", dir, "--dados", novo)
		require.NoError(t, r.err, r.out)
		assert.Contains(t, e.state("run_args"), "-v "+novo+":/var/lib/mysql")
		assert.Contains(t, registro(e), " dados_antes="+filepath.Join(e.dir, "dados")+" dados="+novo+"\n")
	})
	t.Run("--trocar-dados sem --recriar ou com o mesmo caminho: recusa", func(t *testing.T) {
		e, dir := novaGuarda(t)
		semMudanca(t, e, e.rodar("--trocar-dados", "--dados", novoDir(t, e, true)), "--trocar-dados so com --recriar")
		semMudanca(t, e, e.rodar("--recriar", "--trocar-dados", "--backup-dir", dir), "--trocar-dados so com --recriar")
	})
	t.Run("sem --recriar com --dados diferente: diverge", func(t *testing.T) {
		e, _ := novaGuarda(t)
		novo := novoDir(t, e, true)
		r := e.rodar("--dados", novo)
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "datadir "+novo+" (atual: "+filepath.Join(e.dir, "dados")+")")
		assert.Contains(t, r.out, "rode de novo com --recriar")
		assert.NotContains(t, e.log(), "docker rm")
	})
	for nome, montado := range map[string]string{"vazio": "", "duas linhas": "bind|/a\nbind|/b\n", "volume": "volume|x\n"} {
		t.Run("montagem ilegivel: "+nome, func(t *testing.T) {
			e, dir := novaGuarda(t)
			dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz")
			e.setState("dados_montado", montado)
			semMudanca(t, e, e.rodar("--recriar", "--backup-dir", dir), "nao consegui ler o datadir atual")
		})
	}
}

// ---------------------------------------------------------------------------
// TST-28: -timeout=30m no go test (Makefile e Jenkinsfile)
// ---------------------------------------------------------------------------

func TestTST28_TimeoutGoTest(t *testing.T) {
	mk := lerRaiz(t, "Makefile")
	jf := lerRaiz(t, "Jenkinsfile")
	for _, alvo := range []string{"test-shared", "test-api"} {
		assert.Regexp(t, `(?m)^`+alvo+`:.*\n(\t.*\n)*?\t.*go test \./\.\.\. -timeout=30m \$\(GO_TEST_FLAGS\)`, mk, alvo)
	}
	linhas := regexp.MustCompile(`(?m)^\t.*go test \./\.\.\. .*-coverprofile=.*$`).FindAllString(mk, -1)
	require.Len(t, linhas, 6, "test/cover shared e api, mais o cover-combinada")
	for _, l := range linhas {
		assert.Contains(t, l, "go test ./... -timeout=30m ", l)
	}
	estagio := estagioJenkins(t, jf, "Testes Go")
	require.Equal(t, 2, strings.Count(estagio, "go test ./..."))
	assert.Equal(t, 2, strings.Count(estagio, "go test ./... -timeout=30m"))

	job := regexp.MustCompile(`timeout\(time: (\d+), unit: 'MINUTES'\)`).FindStringSubmatch(jf)
	require.NotNil(t, job)
	limite, err := time.ParseDuration(job[1] + "m")
	require.NoError(t, err)
	valores := regexp.MustCompile(`go test \./\.\.\. -timeout=(\S+)`).FindAllStringSubmatch(mk+jf, -1)
	require.NotEmpty(t, valores)
	for _, v := range valores {
		assert.Equal(t, "30m", v[1])
		d, err := time.ParseDuration(v[1])
		require.NoError(t, err)
		assert.Less(t, d, limite, "o timeout por pacote estoura antes do job")
	}
}
