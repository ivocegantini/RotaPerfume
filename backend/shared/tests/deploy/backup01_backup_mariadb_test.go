package deploy_test

// BACKUP-01 (Lote 39): deploy/backup-mariadb.sh e a guarda de backup do
// --recriar no deploy/mariadb-docker.sh. Ensaio com o docker falso
// (mariadb_docker_stubs_test.go) no mesmo runner do TST-15; restauração real
// num MariaDB descartável via Docker só com BACKUP01_DOCKER=1.

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func backupScript(t *testing.T) string {
	t.Helper()
	precisaBash(t)
	return caminhoRaiz(t, "deploy", "backup-mariadb.sh")
}

// rodarScript roda outro script de deploy/ no runner do ensaio (EUID 0 no
// user namespace, chroot com os ancestrais do $FAKE do root).
func (e *ensaio) rodarScript(script string, args ...string) saidaEnsaio {
	e.t.Helper()
	_ = os.Remove(filepath.Join(e.dir, "log"))
	cmd := exec.Command("unshare", append([]string{"-rm", "bash", filepath.Join(e.dir, "runner.sh")}, args...)...)
	cmd.Env = append(ambienteLimpo(), "FAKE="+e.dir, "SCRIPT="+script)
	out, err := cmd.CombinedOutput()
	return saidaEnsaio{out: string(out), err: err}
}

func rcDe(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	require.ErrorAs(t, err, &ee)
	return ee.ExitCode()
}

func gz(t *testing.T, conteudo string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, err := w.Write([]byte(conteudo))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return b.Bytes()
}

func gunzip(t *testing.T, caminho string) string {
	t.Helper()
	f, err := os.Open(caminho)
	require.NoError(t, err)
	defer f.Close()
	r, err := gzip.NewReader(f)
	require.NoError(t, err)
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(b)
}

// gravarBackup cria em dir (700) um backup no formato do script: .sql.gz 600
// + .sha256, com a idade pedida (mtime).
func gravarBackup(t *testing.T, dir, nome string, dados []byte, idade time.Duration) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.Chmod(dir, 0o700))
	p := filepath.Join(dir, nome)
	require.NoError(t, os.WriteFile(p, dados, 0o600))
	soma := sha256.Sum256(dados)
	require.NoError(t, os.WriteFile(p+".sha256", []byte(hex.EncodeToString(soma[:])+"  "+nome+"\n"), 0o600))
	quando := time.Now().Add(-idade)
	require.NoError(t, os.Chtimes(p, quando, quando))
	require.NoError(t, os.Chtimes(p+".sha256", quando, quando))
	return p
}

const dumpValido = "-- MariaDB dump\nCREATE TABLE t (id int);\n-- Dump completed on 2026-11-14 10:00:00\n"

func backupsEm(t *testing.T, dir string) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(dir, "rotaperfumes-*.sql.gz"))
	require.NoError(t, err)
	return m
}

func semParciais(t *testing.T, dir string) {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(dir, ".parcial.*"))
	assert.Empty(t, m, "nenhum arquivo parcial")
}

// ---------------------------------------------------------------------------
// Estáticos e opções (sem root)
// ---------------------------------------------------------------------------

func TestBACKUP01_Backup_OpcoesInvalidas(t *testing.T) {
	sc := backupScript(t)
	casos := []struct {
		nome string
		args []string
		rc   int
		msg  string
	}{
		{"retencao zero", []string{"--retencao-dias", "0"}, 2, "--retencao-dias invalido"},
		{"retencao texto", []string{"--retencao-dias", "abc"}, 2, "--retencao-dias invalido"},
		{"retencao acima", []string{"--retencao-dias", "3651"}, 2, "--retencao-dias invalido"},
		{"conferir zero", []string{"--conferir-recente", "0"}, 2, "--conferir-recente invalido"},
		{"destino relativo", []string{"--destino", "backups"}, 2, "--destino invalido"},
		{"destino com ..", []string{"--destino", "/var/../tmp/x"}, 2, "--destino invalido"},
		{"banco com aspas", []string{"--banco", "x'y"}, 2, "--banco invalido"},
		{"opcao desconhecida", []string{"--tudo"}, 2, "Opcao desconhecida: --tudo"},
		{"destino sem valor", []string{"--destino"}, 2, "a opcao --destino exige um valor"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			cmd := exec.Command("bash", append([]string{sc}, c.args...)...)
			cmd.Env = ambienteLimpo()
			out, err := cmd.CombinedOutput()
			assert.Equal(t, c.rc, rcDe(t, err), "%s", out)
			assert.Contains(t, string(out), c.msg)
		})
	}
	t.Run("sem root", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("rodando como root")
		}
		cmd := exec.Command("bash", sc)
		cmd.Env = ambienteLimpo()
		out, err := cmd.CombinedOutput()
		assert.Equal(t, 1, rcDe(t, err), "%s", out)
		assert.Contains(t, string(out), "rode como root")
	})
	t.Run("--help", func(t *testing.T) {
		cmd := exec.Command("bash", sc, "--help")
		cmd.Env = ambienteLimpo()
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", out)
		for _, s := range []string{"--retencao-dias", "padrao 14", "--conferir-recente", "/var/backups/rotaperfumes-mariadb"} {
			assert.Contains(t, string(out), s)
		}
	})
}

func TestBACKUP01_Backup_Estatico(t *testing.T) {
	sh := lerRaiz(t, "deploy", "backup-mariadb.sh")
	assert.Regexp(t, `(?m)^for cmd in [^\n]*\bflock\b`, sh, "flock obrigatorio (falha fechada)")
	assert.NotContains(t, sh, "command -v flock")
	for _, s := range []string{"set -euo pipefail", "umask 077", "--single-transaction", "--routines", "--triggers", "--events",
		"--defaults-extra-file=\"$C_ROOT_CNF\"", `trap 'rm -f -- "$TMP" "$TMP_SHA"' EXIT`} {
		assert.Contains(t, sh, s)
	}
	assert.NotRegexp(t, `(?m)^[^#]*set -x`, sh)
	assert.NotRegexp(t, `(?m)^[^#]*(-p\$|--password=|MYSQL_PWD)`, sh, "senha nunca no argv")
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	assert.Contains(t, md, `BACKUP_SCRIPT="$(dirname -- "$0")/backup-mariadb.sh"`)
	doc := lerRaiz(t, "docs", "deploy-servidor.md")
	for _, s := range []string{"deploy/backup-mariadb.sh", "rotaperfumes-backup.timer", "OnCalendar=", "--conferir-recente", "Restaurar", "--backup-agora"} {
		assert.Contains(t, doc, s, "docs/deploy-servidor.md")
	}
}

// Lote 39: o tar do "Testes Go" leva o que os testes novos leem (SEC-47:
// backend/Dockerfile; SEC-58: scripts/; BACKUP-01/SEC-51: o doc de deploy).
func TestBACKUP01_TesteGoEnviaArquivosLidos(t *testing.T) {
	corpo := estagioJenkins(t, lerRaiz(t, "Jenkinsfile"), "Testes Go")
	for _, a := range []string{"backend/Dockerfile", "scripts", "docs/deploy-servidor.md", "deploy", "Jenkinsfile"} {
		assert.Regexp(t, `\s`+regexp.QuoteMeta(a)+`(\s|$)`, corpo, a)
	}
}

// ---------------------------------------------------------------------------
// Ensaio do backup (docker falso)
// ---------------------------------------------------------------------------

func novoEnsaioBackup(t *testing.T) (*ensaio, string) {
	t.Helper()
	e := novoEnsaio(t)
	e.setState("criado", "") // container existe e roda
	return e, filepath.Join(e.dir, "backups")
}

func TestBACKUP01_Backup_GravaComprimido600(t *testing.T) {
	e, dest := novoEnsaioBackup(t)
	r := e.rodarScript(backupScript(t), "--destino", dest)
	require.NoError(t, r.err, r.out)

	bks := backupsEm(t, dest)
	require.Len(t, bks, 1, r.out)
	assert.Regexp(t, `/rotaperfumes-\d{8}-\d{6}\.sql\.gz$`, bks[0])
	st, err := os.Stat(bks[0])
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	st, err = os.Stat(dest)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), st.Mode().Perm())
	assert.True(t, strings.HasSuffix(strings.TrimSpace(gunzip(t, bks[0])), "-- Dump completed on 2026-11-14 10:00:00"))

	b, err := os.ReadFile(bks[0])
	require.NoError(t, err)
	soma := sha256.Sum256(b)
	sha, err := os.ReadFile(bks[0] + ".sha256")
	require.NoError(t, err)
	assert.Equal(t, hex.EncodeToString(soma[:])+"  "+filepath.Base(bks[0])+"\n", string(sha))
	st, err = os.Stat(bks[0] + ".sha256")
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	semParciais(t, dest)

	args := e.state("dump_args")
	assert.Contains(t, args, "rotaperfumes-mariadb mariadb-dump --defaults-extra-file=/run/rotaperfumes/root.cnf --single-transaction")
	for _, s := range []string{"--routines", "--triggers", "--events", "--hex-blob", "--default-character-set=utf8mb4"} {
		assert.Contains(t, args, s)
	}
	assert.True(t, strings.HasSuffix(strings.TrimSpace(args), " rotaperfumes"), args)
	assert.Contains(t, r.out, "Retencao 14 dias: 0 backup(s) antigo(s) removido(s).")

	r = e.rodarScript(backupScript(t), "--destino", dest, "--conferir-recente", "1")
	require.NoError(t, r.err, r.out)
	assert.Contains(t, r.out, "Backup recente valido (< 1 h): "+bks[0])
}

func TestBACKUP01_Backup_FalhaSemArquivoParcial(t *testing.T) {
	casos := []struct {
		nome, estado, conteudo, msg string
	}{
		{"mariadb-dump falha", "dump_falha", "", "mariadb-dump ou gzip falhou (nenhum backup gravado)"},
		{"dump sem a marca final", "dump_incompleto", "", "dump incompleto"},
		{"container parado", "rodando", "false\n", "nao esta rodando"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e, dest := novoEnsaioBackup(t)
			e.setState(c.estado, c.conteudo)
			r := e.rodarScript(backupScript(t), "--destino", dest)
			assert.NotEqual(t, 0, rcDe(t, r.err), r.out)
			assert.Contains(t, r.out, c.msg)
			assert.Empty(t, backupsEm(t, dest))
			m, _ := filepath.Glob(filepath.Join(dest, "*.sha256"))
			assert.Empty(t, m)
			semParciais(t, dest)
		})
	}
}

func TestBACKUP01_Backup_DestinoInseguro(t *testing.T) {
	t.Run("modo 755", func(t *testing.T) {
		e, dest := novoEnsaioBackup(t)
		require.NoError(t, os.MkdirAll(dest, 0o755))
		require.NoError(t, os.Chmod(dest, 0o755))
		r := e.rodarScript(backupScript(t), "--destino", dest)
		assert.NotEqual(t, 0, rcDe(t, r.err), r.out)
		assert.Contains(t, r.out, "precisa ser do root com modo 700")
		assert.Empty(t, e.state("dump_args"), "nenhum dump")
	})
	t.Run("symlink", func(t *testing.T) {
		e, dest := novoEnsaioBackup(t)
		alvo := filepath.Join(e.dir, "alvo")
		require.NoError(t, os.Mkdir(alvo, 0o700))
		require.NoError(t, os.Symlink(alvo, dest))
		r := e.rodarScript(backupScript(t), "--destino", dest)
		assert.NotEqual(t, 0, rcDe(t, r.err), r.out)
		assert.Contains(t, r.out, "link simbolico")
		assert.Empty(t, e.state("dump_args"))
	})
}

func TestBACKUP01_Backup_Retencao(t *testing.T) {
	e, dest := novoEnsaioBackup(t)
	dia := 24 * time.Hour
	velho := gravarBackup(t, dest, "rotaperfumes-20261001-000000.sql.gz", gz(t, dumpValido), 20*dia)
	recente := gravarBackup(t, dest, "rotaperfumes-20261110-000000.sql.gz", gz(t, dumpValido), 4*dia)
	outro := gravarBackup(t, dest, "outro-20261001-000000.sql.gz", gz(t, dumpValido), 30*dia)
	parcial := filepath.Join(dest, ".parcial.abc123")
	require.NoError(t, os.WriteFile(parcial, []byte("x"), 0o600))
	duasHoras := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(parcial, duasHoras, duasHoras))

	r := e.rodarScript(backupScript(t), "--destino", dest)
	require.NoError(t, r.err, r.out)
	assert.NoFileExists(t, velho)
	assert.NoFileExists(t, velho+".sha256")
	assert.FileExists(t, recente)
	assert.FileExists(t, outro, "só o padrão de nome do banco")
	assert.NoFileExists(t, parcial, "parcial esquecido (> 1 h) sai")
	assert.Len(t, backupsEm(t, dest), 2, "o novo + o de 4 dias")
	assert.Contains(t, r.out, "Retencao 14 dias: 1 backup(s) antigo(s) removido(s).")

	// Retenção menor (1 s depois: o nome tem segundos).
	time.Sleep(1100 * time.Millisecond)
	r = e.rodarScript(backupScript(t), "--destino", dest, "--retencao-dias", "3")
	require.NoError(t, r.err, r.out)
	assert.NoFileExists(t, recente)
	assert.Len(t, backupsEm(t, dest), 2, "os dois de hoje")
}

func TestBACKUP01_ConferirRecente(t *testing.T) {
	sc := backupScript(t)
	t.Run("destino inexistente", func(t *testing.T) {
		e, dest := novoEnsaioBackup(t)
		r := e.rodarScript(sc, "--destino", dest, "--conferir-recente", "24")
		assert.Equal(t, 1, rcDe(t, r.err), r.out)
		assert.Contains(t, r.out, "Nenhum backup")
	})
	t.Run("so backup antigo", func(t *testing.T) {
		e, dest := novoEnsaioBackup(t)
		gravarBackup(t, dest, "rotaperfumes-20261101-000000.sql.gz", gz(t, dumpValido), 30*time.Hour)
		r := e.rodarScript(sc, "--destino", dest, "--conferir-recente", "24")
		assert.Equal(t, 1, rcDe(t, r.err), r.out)
		assert.Contains(t, r.out, "Nenhum backup valido com menos de 24 h")
	})
	invalidos := []struct {
		nome  string
		prep  func(t *testing.T, p string)
		dados []byte
	}{
		{"gzip corrompido", nil, []byte("nao e gzip")},
		{"sem a marca final", nil, gz(t, "-- MariaDB dump\nCREATE TABLE t (id int);\n")},
		{"sha256 divergente", func(t *testing.T, p string) {
			require.NoError(t, os.WriteFile(p+".sha256", []byte(strings.Repeat("0", 64)+"  "+filepath.Base(p)+"\n"), 0o600))
		}, gz(t, dumpValido)},
		{"modo 644", func(t *testing.T, p string) { require.NoError(t, os.Chmod(p, 0o644)) }, gz(t, dumpValido)},
	}
	for _, c := range invalidos {
		t.Run(c.nome, func(t *testing.T) {
			e, dest := novoEnsaioBackup(t)
			p := gravarBackup(t, dest, "rotaperfumes-20261114-000000.sql.gz", c.dados, time.Hour)
			if c.prep != nil {
				c.prep(t, p)
			}
			r := e.rodarScript(sc, "--destino", dest, "--conferir-recente", "24")
			assert.Equal(t, 1, rcDe(t, r.err), r.out)
			assert.Contains(t, r.out, "backup invalido ignorado")
		})
	}
	t.Run("valido recente", func(t *testing.T) {
		e, dest := novoEnsaioBackup(t)
		p := gravarBackup(t, dest, "rotaperfumes-20261114-000000.sql.gz", gz(t, dumpValido), time.Hour)
		r := e.rodarScript(sc, "--destino", dest, "--conferir-recente", "24")
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, p)
		assert.Empty(t, e.state("dump_args"), "conferir nunca faz dump")
	})
}

// ---------------------------------------------------------------------------
// Guarda do --recriar no mariadb-docker.sh
// ---------------------------------------------------------------------------

// ensaioComDados: instalação feita (container existe) e datadir com dados.
func ensaioComDados(t *testing.T) (*ensaio, string) {
	t.Helper()
	e := novoEnsaio(t)
	r := e.rodar("--nova-ca")
	require.NoError(t, r.err, r.out)
	require.NoError(t, os.MkdirAll(filepath.Join(e.dir, "dados", "mysql"), 0o700))
	return e, filepath.Join(e.dir, "backups")
}

func TestBACKUP01_Recriar_ExigeBackup(t *testing.T) {
	t.Run("sem backup recusa sem mudar nada", func(t *testing.T) {
		e, bk := ensaioComDados(t)
		r := e.rodar("--recriar", "--backup-dir", bk)
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "--recriar sem backup recente (< 24 h) em "+bk)
		assert.Contains(t, r.out, "--backup-agora")
		assert.NotContains(t, e.log(), "docker rm")
		assert.NotContains(t, r.out, "=== [2/", "falha na etapa 1")
	})
	t.Run("backup antigo recusa", func(t *testing.T) {
		e, bk := ensaioComDados(t)
		gravarBackup(t, bk, "rotaperfumes-20261101-000000.sql.gz", gz(t, dumpValido), 25*time.Hour)
		r := e.rodar("--recriar", "--backup-dir", bk)
		require.Error(t, r.err, r.out)
		assert.NotContains(t, e.log(), "docker rm")
	})
	t.Run("backup recente segue", func(t *testing.T) {
		e, bk := ensaioComDados(t)
		gravarBackup(t, bk, "rotaperfumes-20261114-000000.sql.gz", gz(t, dumpValido), time.Hour)
		r := e.rodar("--recriar", "--backup-dir", bk)
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "backup recente conferido")
		assert.Contains(t, e.log(), "docker rm -f rotaperfumes-mariadb")
		assert.Empty(t, e.state("dump_args"), "só conferiu")
	})
	t.Run("--backup-agora faz o backup antes do rm", func(t *testing.T) {
		e, bk := ensaioComDados(t)
		r := e.rodar("--recriar", "--backup-agora", "--backup-dir", bk)
		require.NoError(t, r.err, r.out)
		require.Len(t, backupsEm(t, bk), 1, r.out)
		log := e.log()
		iDump := strings.Index(log, "mariadb-dump")
		iRm := strings.Index(log, "docker rm -f rotaperfumes-mariadb")
		require.GreaterOrEqual(t, iDump, 0, log)
		require.GreaterOrEqual(t, iRm, 0, log)
		assert.Less(t, iDump, iRm, "backup antes de remover o container")
	})
	t.Run("--backup-agora com falha nao remove", func(t *testing.T) {
		e, bk := ensaioComDados(t)
		e.setState("dump_falha", "")
		r := e.rodar("--recriar", "--backup-agora", "--backup-dir", bk)
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "o backup falhou: nada foi mudado")
		assert.NotContains(t, e.log(), "docker rm")
		assert.Empty(t, backupsEm(t, bk))
	})
	t.Run("--sem-backup registra o motivo", func(t *testing.T) {
		e, bk := ensaioComDados(t)
		r := e.rodar("--recriar", "--sem-backup", "disco cheio, backup copiado a mao", "--backup-dir", bk)
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "BACKUP-01: --recriar SEM backup")
		assert.Contains(t, e.log(), "docker rm -f rotaperfumes-mariadb")
		assert.Contains(t, e.log(), "logger -t rotaperfumes-mariadb -- BACKUP-01: container rotaperfumes-mariadb recriado SEM backup", "motivo no journal")
		assert.Contains(t, e.log(), "motivo: disco cheio, backup copiado a mao")
		p := filepath.Join(segredos, "recriar-sem-backup.motivo")
		assert.Contains(t, e.arq(p), "disco cheio, backup copiado a mao")
		st, err := os.Stat(filepath.Join(e.dir, p))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	})
	t.Run("datadir sem dados nao exige backup", func(t *testing.T) {
		e := novoEnsaio(t)
		require.NoError(t, e.rodar("--nova-ca").err)
		r := e.rodar("--recriar", "--backup-dir", filepath.Join(e.dir, "backups"))
		require.NoError(t, r.err, r.out)
		assert.Contains(t, e.log(), "docker rm -f rotaperfumes-mariadb")
	})
}

// SecBrain (Lote 39): container NOVO sobre dados, sem --recriar (ex.: depois
// de um docker rm manual), também exige backup; --backup-agora não serve.
func TestBACKUP01_ContainerNovoSobreDados(t *testing.T) {
	semContainer := func(t *testing.T) (*ensaio, string) {
		e, bk := ensaioComDados(t)
		require.NoError(t, os.Remove(filepath.Join(e.dir, "state", "criado"))) // docker rm manual
		require.NoError(t, os.Remove(filepath.Join(e.dir, "state", "run_args")))
		return e, bk
	}
	t.Run("sem backup recusa", func(t *testing.T) {
		e, bk := semContainer(t)
		r := e.rodar("--backup-dir", bk)
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "container novo sobre os dados de")
		assert.Contains(t, r.out, "MARIADB_AUTO_UPGRADE")
		assert.Empty(t, e.state("run_args"), "nenhum container criado")
		assert.NotContains(t, r.out, "=== [2/")
	})
	t.Run("--backup-agora sem container recusa", func(t *testing.T) {
		e, bk := semContainer(t)
		r := e.rodar("--recriar", "--backup-agora", "--backup-dir", bk)
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "--backup-agora precisa do container rotaperfumes-mariadb (nao existe")
		assert.Empty(t, e.state("run_args"))
		assert.Empty(t, e.state("dump_args"))
	})
	t.Run("backup recente segue", func(t *testing.T) {
		e, bk := semContainer(t)
		gravarBackup(t, bk, "rotaperfumes-20261114-000000.sql.gz", gz(t, dumpValido), time.Hour)
		r := e.rodar("--backup-dir", bk)
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "backup recente conferido")
		assert.NotEmpty(t, e.state("run_args"))
	})
	t.Run("--sem-backup sem --recriar segue e registra", func(t *testing.T) {
		e, bk := semContainer(t)
		r := e.rodar("--sem-backup", "container apagado por engano, sem dump", "--backup-dir", bk)
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "SEM backup (--sem-backup)")
		assert.Contains(t, e.arq(filepath.Join(segredos, "recriar-sem-backup.motivo")), "container apagado por engano")
		assert.Contains(t, e.log(), "logger -t rotaperfumes-mariadb")
		assert.NotEmpty(t, e.state("run_args"))
	})
	t.Run("container existente sem --recriar nao exige", func(t *testing.T) {
		e, bk := ensaioComDados(t)
		r := e.rodar("--backup-dir", bk)
		require.NoError(t, r.err, r.out)
		assert.NotContains(t, r.out, "BACKUP-01")
	})
}

// SecBrain (Lote 39): o backup-mariadb.sh roda como root; dono/modo/symlink
// conferidos antes (mesma regra do conferir_dir_privado).
func TestBACKUP01_BackupScriptInseguroRecusado(t *testing.T) {
	casos := []struct {
		nome string
		prep func(t *testing.T, e *ensaio) string
		msg  string
	}{
		{"escrita de grupo", func(t *testing.T, e *ensaio) string {
			p := filepath.Join(e.dir, "bk", "backup-mariadb.sh")
			require.NoError(t, os.Chmod(p, 0o775))
			return p
		}, "tem escrita para grupo/outros (modo 775)"},
		{"diretorio com escrita de outros", func(t *testing.T, e *ensaio) string {
			require.NoError(t, os.Chmod(filepath.Join(e.dir, "bk"), 0o757))
			return filepath.Join(e.dir, "bk", "backup-mariadb.sh")
		}, "tem escrita para grupo/outros (modo 757)"},
		{"symlink", func(t *testing.T, e *ensaio) string {
			l := filepath.Join(e.dir, "bk", "link.sh")
			require.NoError(t, os.Symlink(filepath.Join(e.dir, "bk", "backup-mariadb.sh"), l))
			return l
		}, "e um link simbolico"},
		{"inexistente", func(t *testing.T, e *ensaio) string { return filepath.Join(e.dir, "bk", "nao.sh") }, "nao achei"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e, bk := ensaioComDados(t)
			sc := c.prep(t, e)
			r := e.rodar("--recriar", "--backup-agora", "--backup-dir", bk, "--backup-script", sc)
			require.Error(t, r.err, r.out)
			assert.Contains(t, r.out, c.msg)
			assert.Empty(t, e.state("dump_args"), "o script nao rodou")
			assert.NotContains(t, e.log(), "docker rm")
		})
	}
}

func TestBACKUP01_Recriar_OpcoesInvalidas(t *testing.T) {
	casos := []struct {
		nome string
		args []string
		msg  string
	}{
		{"--sem-backup curto", []string{"--recriar", "--sem-backup", "curto"}, "--sem-backup precisa de um motivo"},
		{"exclusivas", []string{"--recriar", "--backup-agora", "--sem-backup", "motivo bem explicado"}, "sao exclusivos"},
		{"--backup-agora sem --recriar", []string{"--backup-agora"}, "--backup-agora so vale com --recriar"},
		{"--backup-dir relativo", []string{"--recriar", "--backup-dir", "backups"}, "diretorio invalido"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaio(t)
			r := e.rodar(append([]string{"--nova-ca"}, c.args...)...)
			require.Error(t, r.err, r.out)
			assert.Contains(t, r.out, c.msg)
			assert.Empty(t, e.state("run_args"))
		})
	}
}

// ---------------------------------------------------------------------------
// Restauração real num MariaDB descartável (Docker; opt-in)
// ---------------------------------------------------------------------------

// TestBACKUP01_RestauracaoReal_Docker sobe um container descartável da
// IMAGEM_BASE (sem porta publicada; mesmo layout do root.cnf da produção),
// carrega a DDL do sql/ordem-ddl.txt + amostra (utf8mb4, BLOB, trigger,
// procedure, view, evento), roda o backup-mariadb.sh REAL, restaura noutro
// banco e compara. Só com BACKUP01_DOCKER=1 e acesso ao Docker (grupo docker
// ou root). Ex.: BACKUP01_DOCKER=1 go test -count=1 -run
// TestBACKUP01_RestauracaoReal_Docker ./tests/deploy/
func TestBACKUP01_RestauracaoReal_Docker(t *testing.T) {
	sc := backupScript(t)
	if os.Getenv("BACKUP01_DOCKER") != "1" {
		t.Skip("opt-in: BACKUP01_DOCKER=1 (Docker acessível)")
	}
	if out, err := exec.Command("docker", "info").CombinedOutput(); err != nil {
		t.Skipf("docker inacessível: %v %s", err, out)
	}
	m := regexp.MustCompile(`(?m)^IMAGEM_BASE="([^"]+)"$`).FindStringSubmatch(lerRaiz(t, "deploy", "mariadb-docker.sh"))
	require.NotNil(t, m)
	imagem := m[1]

	var e *ensaio
	rodarBackup := func(args ...string) saidaEnsaio {
		if os.Geteuid() == 0 {
			cmd := exec.Command("bash", append([]string{sc}, args...)...)
			cmd.Env = ambienteLimpo()
			out, err := cmd.CombinedOutput()
			return saidaEnsaio{out: string(out), err: err}
		}
		return e.rodarScript(sc, args...)
	}
	var dest string
	if os.Geteuid() == 0 {
		dest = filepath.Join(baseRootPrivada(t, "backup01"), "backups")
	} else {
		e = novoEnsaioReal(t)
		dest = filepath.Join(e.dir, "backups")
	}

	segredosDir := t.TempDir()
	require.NoError(t, os.Chmod(segredosDir, 0o755))
	senha := fmt.Sprintf("Backup01Teste%d", time.Now().UnixNano())
	escrever(t, filepath.Join(segredosDir, "root.senha"), senha, 0o644)
	escrever(t, filepath.Join(segredosDir, "root.cnf"), "[client]\nuser=root\npassword="+senha+"\nprotocol=socket\n", 0o644)

	nome := fmt.Sprintf("backup01-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", "-v", nome).Run() })
	out, err := exec.Command("docker", "run", "-d", "--name", nome, "--network", "none",
		"-e", "MARIADB_ROOT_PASSWORD_FILE=/run/rotaperfumes/root.senha", "-e", "MARIADB_ROOT_HOST=localhost",
		"-e", "MARIADB_DATABASE=rotaperfumes",
		"-v", segredosDir+":/run/rotaperfumes:ro", imagem,
		"--character-set-server=utf8mb4", "--collation-server=utf8mb4_unicode_ci").CombinedOutput()
	require.NoError(t, err, "%s", out)

	sqlRoot := func(banco, stdin string, args ...string) string {
		// O cliente da imagem conecta em utf8mb3 por padrão: sem utf8mb4, o emoji
		// (4 bytes) da amostra dá ERROR 1366. O backup-mariadb.sh já usa utf8mb4.
		a := append([]string{"exec", "-i", nome, "mariadb", "--defaults-extra-file=/run/rotaperfumes/root.cnf",
			"--default-character-set=utf8mb4", "-N", "-B"}, args...)
		if banco != "" {
			a = append(a, banco)
		}
		cmd := exec.Command("docker", a...)
		cmd.Stdin = strings.NewReader(stdin)
		o, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", o)
		return strings.TrimSpace(string(o))
	}
	pronto := false
	for i := 0; i < 90 && !pronto; i++ {
		o, err := exec.Command("docker", "exec", nome, "mariadb", "--defaults-extra-file=/run/rotaperfumes/root.cnf",
			"-N", "-B", "-e", "SELECT @@skip_networking").CombinedOutput()
		pronto = err == nil && strings.TrimSpace(string(o)) == "0"
		if !pronto {
			time.Sleep(2 * time.Second)
		}
	}
	require.True(t, pronto, "MariaDB descartável não ficou pronto")

	// DDL real do projeto (ordem do seed) + amostra com objetos de banco.
	for _, l := range linhasUteis(lerRaiz(t, "sql", "ordem-ddl.txt")) {
		sqlRoot("rotaperfumes", lerRaiz(t, "sql", l))
	}
	sqlRoot("rotaperfumes", `
CREATE TABLE IF NOT EXISTS backup01_amostra (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  texto VARCHAR(100) NOT NULL,
  bin BLOB NULL,
  contador INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
CREATE TRIGGER backup01_tr BEFORE INSERT ON backup01_amostra FOR EACH ROW SET NEW.contador = NEW.contador + 1;
CREATE PROCEDURE backup01_proc() SELECT COUNT(*) FROM backup01_amostra;
CREATE VIEW backup01_view AS SELECT id, texto FROM backup01_amostra;
CREATE EVENT backup01_ev ON SCHEDULE EVERY 1 DAY DISABLE DO DELETE FROM backup01_amostra WHERE 1 = 0;
-- TST-23: instantes literais (o teste não lê o relógio do banco).
INSERT INTO backup01_amostra (texto, bin, created_at, updated_at) VALUES
  ('Perfume ação 🌸', UNHEX('00FF10E2'), '2026-11-14 10:00:00', '2026-11-14 10:00:00'),
  ('aspas '' e \\ barra', NULL, '2026-11-14 10:00:00', '2026-11-14 10:00:00');
`)

	r := rodarBackup("--container", nome, "--destino", dest)
	require.NoError(t, r.err, r.out)
	bks := backupsEm(t, dest)
	require.Len(t, bks, 1, r.out)
	r = rodarBackup("--destino", dest, "--conferir-recente", "1")
	require.NoError(t, r.err, r.out)

	// Restauração noutro banco do mesmo servidor descartável.
	sqlRoot("", "CREATE DATABASE rotaperfumes_restaurado CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;")
	sqlRoot("rotaperfumes_restaurado", gunzip(t, bks[0]))

	tabelas := linhas(sqlRoot("", "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA='rotaperfumes' AND TABLE_TYPE='BASE TABLE' ORDER BY 1;"))
	require.Greater(t, len(tabelas), 5)
	for _, tb := range tabelas {
		a := sqlRoot("", fmt.Sprintf("CHECKSUM TABLE rotaperfumes.`%s` EXTENDED;", tb))
		b := sqlRoot("", fmt.Sprintf("CHECKSUM TABLE rotaperfumes_restaurado.`%s` EXTENDED;", tb))
		assert.Equal(t, strings.Fields(a)[1], strings.Fields(b)[1], "checksum de %s", tb)
		ca := sqlRoot("rotaperfumes", fmt.Sprintf("SHOW CREATE TABLE `%s`;", tb))
		cb := sqlRoot("rotaperfumes_restaurado", fmt.Sprintf("SHOW CREATE TABLE `%s`;", tb))
		assert.Equal(t, ca, cb, "DDL de %s", tb)
	}
	for _, q := range []string{
		"SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA='%s';",
		"SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA='%s';",
		"SELECT COUNT(*) FROM information_schema.VIEWS WHERE TABLE_SCHEMA='%s';",
		"SELECT COUNT(*) FROM information_schema.EVENTS WHERE EVENT_SCHEMA='%s';",
	} {
		orig := sqlRoot("", fmt.Sprintf(q, "rotaperfumes"))
		assert.Equal(t, "1", orig, q)
		assert.Equal(t, orig, sqlRoot("", fmt.Sprintf(q, "rotaperfumes_restaurado")), q)
	}
	assert.Equal(t, "Perfume ação 🌸\t00FF10E2\t1", sqlRoot("rotaperfumes_restaurado", "SELECT texto, HEX(bin), contador FROM backup01_amostra WHERE id = 1;"))
}

// novoEnsaioReal: o runner do ensaio sem os stubs (docker de verdade).
func novoEnsaioReal(t *testing.T) *ensaio {
	t.Helper()
	precisaUnshare(t)
	e := &ensaio{t: t, dir: t.TempDir()}
	require.NoError(t, os.Chmod(e.dir, 0o700))
	for _, d := range []string{"root", "opt", "sbin", "systemd", "state", "bin"} {
		require.NoError(t, os.MkdirAll(filepath.Join(e.dir, d), 0o755))
	}
	escrever(t, filepath.Join(e.dir, "runner.sh"), runnerEnsaio, 0o755)
	return e
}
