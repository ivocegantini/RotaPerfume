package deploy_test

// DEPLOY-10: /usr/local/sbin/rotaperfumes-dba, o script GERADO pelo
// mariadb-docker.sh, rodando no mesmo namespace do ensaio, com docker e logger
// falsos. A sentinela é a senha do DBA, sempre gerada de verdade pelo script.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dbaInstalado = "/usr/local/sbin/rotaperfumes-dba"

// ensaioDBA instala o banco e limpa os rastros do teste da etapa 8.
func ensaioDBA(t *testing.T) *ensaio {
	t.Helper()
	e := novoEnsaio(t)
	e.instalarComPortaPublicada()
	require.NoError(t, e.rodar("--dev-ip", "192.168.168.105").err)
	for _, nome := range []string{"dba_argv", "dba_env", "dba_stdin"} {
		_ = os.Remove(filepath.Join(e.dir, "state", nome))
	}
	_ = os.Remove(filepath.Join(e.dir, "log"))
	return e
}

func (e *ensaio) ambienteDBA() []string {
	return append(ambienteLimpo(), "FAKE="+e.dir, "SCRIPT="+dbaInstalado, "SUDO_USER=ivo")
}

// rodarDBA executa o comando instalado, com stdin e stdout em pipe.
func (e *ensaio) rodarDBA(stdin string, args ...string) saidaEnsaio {
	e.t.Helper()
	_ = os.Remove(filepath.Join(e.dir, "log"))
	cmd := exec.Command("unshare", append([]string{"-rm", "bash", filepath.Join(e.dir, "runner.sh")}, args...)...)
	cmd.Env = e.ambienteDBA()
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	return saidaEnsaio{out: string(out), err: err}
}

// rodarDBATTY executa dentro de um pty do script(1); o sufixo é shell, para
// desligar o TTY só do stdin ou só do stdout.
func (e *ensaio) rodarDBATTY(prefixo, sufixo string) saidaEnsaio {
	e.t.Helper()
	ferramentaObrigatoria(e.t, "script")
	linha := prefixo + "unshare -rm bash " + filepath.Join(e.dir, "runner.sh") + sufixo
	cmd := exec.Command("script", "-qec", linha, "/dev/null")
	cmd.Env = e.ambienteDBA()
	cmd.Stdin = bytes.NewReader(nil)
	out, err := cmd.CombinedOutput()
	return saidaEnsaio{out: string(out), err: err}
}

func TestDEPLOY10_Comando_ArgvDoCliente(t *testing.T) {
	e := ensaioDBA(t)
	r := e.rodarDBA("SELECT CURRENT_USER();\n")
	require.NoError(t, r.err, r.out)
	assert.Contains(t, r.out, "rotaperfumes-admin@172.30.42.22")
	assert.Equal(t, "SELECT CURRENT_USER();\n", e.state("dba_stdin"), "o SQL vai só pelo stdin")

	argv := e.state("dba_argv")
	for _, s := range []string{
		"run --rm -i --pull never --name rotaperfumes-dba-cliente --network rotaperfumes-db --ip 172.30.42.22 --read-only --cap-drop ALL",
		"--security-opt no-new-privileges:true --log-driver none --tmpfs /tmp:rw,nosuid,nodev,noexec",
		"-e MYSQL_HISTFILE=/dev/null",
		"type=bind,source=/root/rotaperfumes-mariadb/dba-cliente/cliente.cnf,target=/run/dba/cliente.cnf,readonly",
		"/ca.pem,target=/run/dba/ca.pem,readonly",
		"--entrypoint mariadb rotaperfumes-mariadb:11.4-",
		"--defaults-extra-file=/run/dba/cliente.cnf -h rotaperfumes-mariadb -P 3306 --ssl-ca=/run/dba/ca.pem --ssl-verify-server-cert --local-infile=0 --default-character-set=utf8mb4",
		"--init-command=SET time_zone='-03:00' --batch rotaperfumes",
	} {
		assert.Contains(t, argv, s)
	}
	// --defaults-extra-file só vale como primeira opção do cliente.
	assert.Regexp(t, `--entrypoint mariadb \S+ --defaults-extra-file=`, argv)
	assert.NotRegexp(t, `(^|\s)-t(\s|$)`, argv, "sem TTY")
	assert.NotContains(t, argv, "--safe-updates")
	assert.NotContains(t, argv, "--force", "para no primeiro erro")
	assert.NotRegexp(t, `(^|\s)-p\s`, argv, "nenhuma porta publicada")
}

func TestDEPLOY10_Comando_SentinelaDaSenha(t *testing.T) {
	e := ensaioDBA(t)
	senha := strings.TrimSpace(e.arq(dbaSenhaRel))
	require.Regexp(t, `^[A-Za-z0-9]{32}$`, senha)
	r := e.rodarDBA("SELECT 1;\n")
	require.NoError(t, r.err, r.out)
	require.NotEmpty(t, e.state("dba_env"), "o stub viu o ambiente")
	// O ambiente fica fora das mensagens do teste. Sob o make, variáveis têm
	// "MYSQL_PWD" no valor (env -u): só a variável em si conta.
	ambiente := e.state("dba_env")
	assert.False(t, strings.Contains(ambiente, senha), "senha no ambiente do cliente")
	assert.False(t, regexp.MustCompile(`(?m)^MYSQL_PWD=`).MatchString(ambiente), "MYSQL_PWD no ambiente do cliente")
	for nome, texto := range map[string]string{
		"saída": r.out, "argv": e.state("dba_argv"), "log": e.log(), "comando gerado": e.arq(dbaComando),
	} {
		assert.NotContains(t, texto, senha, nome)
		assert.NotContains(t, texto, "MYSQL_PWD", nome)
	}
}

func TestDEPLOY10_Comando_SoAceitaSeusArgumentos(t *testing.T) {
	e := ensaioDBA(t)
	for _, args := range [][]string{
		{"--skip-ssl"}, {"-e", "DROP DATABASE rotaperfumes"}, {"--force"}, {"rotaperfumes"}, {"-h"}, {""},
		{"--help", "x"}, {"--encerrar-sessao", "x"}, {"--encerrar-sessao", "--help"},
	} {
		r := e.rodarDBA("", args...)
		require.Error(t, r.err, "%q: %s", args, r.out)
		var ee *exec.ExitError
		require.ErrorAs(t, r.err, &ee)
		assert.Equal(t, 2, ee.ExitCode(), "%q", args)
		assert.Contains(t, r.out, "Uso, como root no servidor", "%q", args)
	}
	assert.Empty(t, e.state("dba_argv"), "nada chega ao cliente")
	assert.NotContains(t, e.log(), "docker")

	r := e.rodarDBA("", "--help")
	require.NoError(t, r.err, r.out)
	assert.Contains(t, r.out, avisoNomes)
	assert.Empty(t, e.state("dba_argv"))
}

func TestDEPLOY10_Comando_TTY(t *testing.T) {
	casos := []struct {
		nome, prefixo, sufixo string
		interativo            bool
	}{
		{"stdin e stdout TTY", "", "", true},
		{"stdin em pipe", "true | ", "", false},
		{"stdin em pipe e stdout em pipe", "true | ", " | cat", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := ensaioDBA(t)
			r := e.rodarDBATTY(c.prefixo, c.sufixo)
			require.NoError(t, r.err, r.out)
			argv := e.state("dba_argv")
			require.NotEmpty(t, argv, r.out)
			if c.interativo {
				assert.Regexp(t, `run --rm -i -t --pull never --name`, argv)
				assert.Contains(t, argv, "--safe-updates --prompt=PRD rotaperfumes-admin>  rotaperfumes")
				assert.NotContains(t, argv, "--batch")
				assert.Contains(t, r.out, avisoNomes, "banner")
				assert.Contains(t, e.log(), "modo=interativo")
				return
			}
			assert.Regexp(t, `run --rm -i --pull never --name`, argv)
			assert.Contains(t, argv, "--batch")
			assert.NotContains(t, argv, "--safe-updates")
			assert.NotContains(t, argv, "--prompt")
			assert.Contains(t, e.log(), "modo=stdin")
		})
	}
}

func TestDEPLOY10_Comando_NomeOcupado(t *testing.T) {
	e := ensaioDBA(t)
	e.setState("dba_ocupado", "")
	r := e.rodarDBA("SELECT 1;\n")
	require.Error(t, r.err, r.out)
	assert.Contains(t, r.out, "ja ha uma sessao aberta (rotaperfumes-dba-cliente)")
	assert.Contains(t, r.out, "sudo rotaperfumes-dba --encerrar-sessao")
	assert.NotContains(t, e.log(), "docker rm", "não remove sozinho")
	assert.NotContains(t, e.log(), "docker run")
	assert.Empty(t, e.state("dba_stdin"))
}

func TestDEPLOY10_Comando_EncerrarSessao(t *testing.T) {
	e := ensaioDBA(t)
	e.setState("dba_ocupado", "")
	r := e.rodarDBA("", "--encerrar-sessao")
	require.NoError(t, r.err, r.out)
	assert.Contains(t, r.out, "Sessao rotaperfumes-dba-cliente removida.")
	var rms []string
	for _, l := range linhas(e.log()) {
		if strings.HasPrefix(l, "docker rm") {
			rms = append(rms, l)
		}
	}
	assert.Equal(t, []string{"docker rm -f rotaperfumes-dba-cliente"}, rms, "só o nome exato")
	assert.Regexp(t, `docker ps -aq --filter name=\^rotaperfumes-dba-cliente\$`, e.log(), "filtro exato")
	assert.NotContains(t, e.log(), "docker run")
	assert.FileExists(t, filepath.Join(e.dir, "state", "criado"), "o banco não é tocado")

	r = e.rodarDBA("", "--encerrar-sessao")
	require.NoError(t, r.err, r.out)
	assert.Contains(t, r.out, "Nenhuma sessao rotaperfumes-dba-cliente aberta.")
	assert.NotContains(t, e.log(), "docker rm")
}

func TestDEPLOY10_Comando_RegistroSemSQL(t *testing.T) {
	e := ensaioDBA(t)
	const sql = "DROP TABLE segredo_do_sql;\n"
	r := e.rodarDBA(sql)
	require.NoError(t, r.err, r.out)
	assert.Regexp(t, `(?m)^logger -t rotaperfumes-dba -- 20\d\d-\d\d-\d\dT\S+ usuario=ivo sessao modo=stdin$`, e.log())
	assert.NotContains(t, e.log(), "segredo_do_sql")
	iLogger, iRun := strings.Index(e.log(), "logger -t"), strings.Index(e.log(), "docker run")
	assert.Less(t, iLogger, iRun, "registra antes de conectar")
}

func TestDEPLOY10_Comando_ConfereOOptionFile(t *testing.T) {
	cnf := func(e *ensaio) string { return filepath.Join(e.dir, dbaCnfRel) }
	t.Run("modo diferente de 400", func(t *testing.T) {
		e := ensaioDBA(t)
		require.NoError(t, os.Chmod(cnf(e), 0o440))
		r := e.rodarDBA("SELECT 1;\n")
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "precisa ser do root com modo 400")
		assert.Empty(t, e.state("dba_argv"))
	})
	t.Run("link simbolico", func(t *testing.T) {
		e := ensaioDBA(t)
		alvo := filepath.Join(e.dir, "outro.cnf")
		escrever(t, alvo, "[client]\n", 0o400)
		require.NoError(t, os.Remove(cnf(e)))
		require.NoError(t, os.Symlink(alvo, cnf(e)))
		r := e.rodarDBA("SELECT 1;\n")
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "nao e arquivo regular")
		assert.Empty(t, e.state("dba_argv"))
	})
	t.Run("ausente", func(t *testing.T) {
		e := ensaioDBA(t)
		require.NoError(t, os.Remove(cnf(e)))
		r := e.rodarDBA("SELECT 1;\n")
		require.Error(t, r.err, r.out)
		assert.Empty(t, e.state("dba_argv"))
	})
}

func TestDEPLOY10_Comando_Gerado(t *testing.T) {
	e := ensaioDBA(t)
	gerado := e.arq(dbaComando)
	assert.Contains(t, gerado, avisoNomes)
	// $IMAGEM vem do mariadb-docker.sh: nenhum digest a mais para manter.
	assert.NotRegexp(t, `sha256:[0-9a-f]{64}`, gerado)
	assert.NotRegexp(t, regexp.MustCompile(`"\$@"`), gerado, "nada é repassado")
	sc, err := exec.LookPath("shellcheck")
	if err != nil {
		return
	}
	out, err := exec.Command(sc, "-s", "bash", filepath.Join(e.dir, dbaComando)).CombinedOutput()
	assert.NoError(t, err, "%s", out)
}
