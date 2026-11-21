package deploy_test

// DEPLOY-10, Docker real: servidor descartável com o TLS e o cnf do script, o SQL
// real de sql_contas e o comando real de gerar_script_dba. Sem root: o comando
// roda em "unshare -r" e um stat falso responde pelo dono do option file (a
// conferência do dono e do modo tem teste próprio, em deploy10_comando_dba). Só
// recursos rp-*-d10, removidos ao fim. O mariadb-docker.sh inteiro exige root:
// isso é o TestTST15_MariadbDocker_Real. Ex.:
// sg docker -c "DEPLOY10_DOCKER=1 go test -count=1 -run TestDEPLOY10_Real ./tests/deploy/"

import (
	"bytes"
	"fmt"
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

const (
	imagemD10   = "rp-img-d10:teste"
	servidorD10 = "rp-mariadb-d10"
	redeD10     = "rp-net-d10"
	prefixoD10  = "172.30.144"
	clienteD10  = "rotaperfumes-dba-cliente"
	senhaRoot10 = "SenhaRootD10Abc123456789XYZ012345"
	senhaDBA10  = "SenhaDbaD10Abc123456789XYZ0123456"
	senhaAdm10  = "SenhaAdminD10Abc123456789XYZ01234"
)

type realD10 struct {
	t    *testing.T
	dir  string
	base string
	cmd  string
	path string
}

func (r *realD10) docker(stdin string, args ...string) (string, string, error) {
	cmd := exec.Command("docker", args...)
	cmd.Env = ambienteLimpo()
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	return so.String(), se.String(), err
}

func (r *realD10) root(sql string) string {
	r.t.Helper()
	so, se, err := r.docker(sql, "exec", "-i", servidorD10, "mariadb", "--defaults-extra-file=/run/rotaperfumes/root.cnf", "-N", "-B")
	require.NoError(r.t, err, "%s", se)
	return strings.TrimSpace(so)
}

func (r *realD10) limpar() {
	_, _, _ = r.docker("", "rm", "-fv", clienteD10)
	_, _, _ = r.docker("", "rm", "-fv", servidorD10)
	_, _, _ = r.docker("", "network", "rm", redeD10)
	_, _, _ = r.docker("", "run", "--rm", "--network", "none", "-v", r.dir+":/x", "--entrypoint", "sh", r.base, "-c", "rm -rf /x/dba-cliente /x/adm")
	_, _, _ = r.docker("", "image", "rm", imagemD10)
}

// dba roda o comando gerado por stdin, como o DBA faria.
func (r *realD10) dba(sql string) (string, error) {
	cmd := exec.Command("unshare", "-r", "bash", r.cmd)
	cmd.Env = append(ambienteLimpo(), "PATH="+r.path)
	cmd.Stdin = strings.NewReader(sql)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// dbaInterativo roda o comando num pty real (script), com a mesma stdin digitada.
func (r *realD10) dbaInterativo(path, digitado string) (string, error) {
	cmd := exec.Command("script", "-qec", "unshare -r bash "+r.cmd, "/dev/null")
	cmd.Env = append(ambienteLimpo(), "PATH="+path, "TERM=dumb")
	cmd.Stdin = strings.NewReader(digitado)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// cliente: conta e IP escolhidos, para o teste cruzado das duas contas.
func (r *realD10) cliente(ip, cnfHost string, args ...string) (string, string, error) {
	base := []string{"run", "--rm", "--network", redeD10, "--ip", ip, "--read-only", "--cap-drop", "ALL",
		"--mount", "type=bind,source=" + filepath.Join(r.dir, "ssl", "ca.pem") + ",target=/run/t/ca.pem,readonly",
		"--mount", "type=bind,source=" + cnfHost + ",target=/run/t/c.cnf,readonly",
		"--entrypoint", "mariadb", r.base, "--defaults-extra-file=/run/t/c.cnf", "-h", servidorD10, "-P", "3306", "-N", "-B"}
	return r.docker("", append(base, args...)...)
}

func TestDEPLOY10_Real(t *testing.T) {
	precisaBash(t)
	if os.Getenv("DEPLOY10_DOCKER") != "1" {
		t.Skip("opt-in: DEPLOY10_DOCKER=1, com Docker (não exige root)")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker inacessível: " + err.Error())
	}
	ferramentaObrigatoria(t, "openssl")
	ferramentaObrigatoria(t, "script")

	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	imagem := regexp.MustCompile(`(?m)^IMAGEM_BASE="([^"]+)"$`).FindStringSubmatch(md)
	require.NotNil(t, imagem)
	sqlMode := regexp.MustCompile(`(?m)^readonly SQL_MODE="(.*)"$`).FindStringSubmatch(md)
	require.NotNil(t, sqlMode)

	dir, err := os.MkdirTemp("", "rp-d10-*")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(dir, 0o711))
	r := &realD10{t: t, dir: dir, base: imagem[1]}
	r.limpar()
	t.Cleanup(func() {
		r.limpar()
		_ = os.RemoveAll(dir)
	})

	// TLS e cnf do servidor.
	ssl := filepath.Join(dir, "ssl")
	require.NoError(t, os.MkdirAll(ssl, 0o755))
	ossl := func(args ...string) {
		out, err := exec.Command("openssl", args...).CombinedOutput()
		require.NoError(t, err, "%s", out)
	}
	ossl("req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", ssl+"/ca-key.pem", "-out", ssl+"/ca.pem", "-days", "2", "-subj", "/CN=rp-d10-ca",
		"-addext", "basicConstraints=critical,CA:TRUE", "-addext", "keyUsage=critical,keyCertSign,cRLSign")
	ossl("req", "-new", "-newkey", "rsa:2048", "-nodes", "-keyout", ssl+"/server-key.pem", "-subj", "/CN=rp-d10", "-out", ssl+"/server.csr")
	escrever(t, ssl+"/ext", "basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:"+servidorD10+"\n", 0o644)
	ossl("x509", "-req", "-in", ssl+"/server.csr", "-CA", ssl+"/ca.pem", "-CAkey", ssl+"/ca-key.pem", "-set_serial", "0x1234", "-days", "2", "-extfile", ssl+"/ext", "-out", ssl+"/server-cert.pem")
	for _, f := range []string{"ca.pem", "server-cert.pem", "server-key.pem"} {
		require.NoError(t, os.Chmod(filepath.Join(ssl, f), 0o644))
	}
	build := filepath.Join(dir, "build")
	escrever(t, build+"/zz.cnf", "[mariadb]\nssl_ca=/etc/mysql/ssl/ca.pem\nssl_cert=/etc/mysql/ssl/server-cert.pem\nssl_key=/etc/mysql/ssl/server-key.pem\nrequire_secure_transport=ON\ntls_version=TLSv1.2,TLSv1.3\nplugin_load_add=auth_ed25519\nlocal_infile=0\nsql_mode="+sqlMode[1]+"\ncharacter_set_server=utf8mb4\ncollation_server=utf8mb4_unicode_ci\nskip_name_resolve=ON\n", 0o644)
	escrever(t, build+"/Dockerfile", "FROM "+r.base+"\nCOPY zz.cnf /etc/mysql/conf.d/zz-rotaperfumes.cnf\nRUN chmod 644 /etc/mysql/conf.d/zz-rotaperfumes.cnf\n", 0o644)
	_, se, err := r.docker("", "build", "-q", "-t", imagemD10, build)
	require.NoError(t, err, se)
	_, se, err = r.docker("", "network", "create", "--driver", "bridge", "--subnet", prefixoD10+".0/24", "--gateway", prefixoD10+".1", "--ip-range", prefixoD10+".128/25", redeD10)
	require.NoError(t, err, se)

	segredosHost := filepath.Join(dir, "root")
	escrever(t, segredosHost+"/root.senha", senhaRoot10, 0o644)
	escrever(t, segredosHost+"/root.cnf", "[client]\nuser=root\npassword="+senhaRoot10+"\nprotocol=socket\n", 0o644)
	require.NoError(t, os.Chmod(segredosHost, 0o755))
	_, se, err = r.docker("", "run", "-d", "--name", servidorD10, "--network", redeD10, "--ip", prefixoD10+".2", "--user", "999:999", "--cap-drop", "ALL",
		"--security-opt", "no-new-privileges:true", "--read-only",
		"--tmpfs", "/run/mysqld:rw,nosuid,nodev,noexec,mode=0755,uid=999,gid=999",
		"--tmpfs", "/tmp:rw,nosuid,nodev,noexec,mode=1777",
		"--tmpfs", "/var/lib/mysql:rw,nosuid,nodev,noexec,mode=0700,uid=999,gid=999",
		"-e", "MARIADB_ROOT_PASSWORD_FILE=/run/rotaperfumes/root.senha", "-e", "MARIADB_ROOT_HOST=localhost",
		"-v", ssl+":/etc/mysql/ssl:ro", "-v", segredosHost+":/run/rotaperfumes:ro", imagemD10)
	require.NoError(t, err, se)
	require.Eventually(t, func() bool {
		so, _, err := r.docker("", "exec", servidorD10, "mariadb", "--defaults-extra-file=/run/rotaperfumes/root.cnf", "-N", "-B", "-e", "SELECT @@skip_networking")
		return err == nil && strings.TrimSpace(so) == "0"
	}, 180*time.Second, 2*time.Second, "o servidor não ficou pronto")

	// Funções reais do script: SQL das contas e comando gerado.
	sqlFn := regexp.MustCompile(`(?ms)^sql_contas\(\) \{.*?^\}\n`).FindString(md)
	require.NotEmpty(t, sqlFn)
	genFn := regexp.MustCompile(`(?ms)^gerar_script_dba\(\) \{.*?^EOF_DBA\n\}\n`).FindString(md)
	require.NotEmpty(t, genFn)
	cnfDir := filepath.Join(dir, "dba-cliente")
	cnfHost := filepath.Join(cnfDir, "cliente.cnf")
	admDir := filepath.Join(dir, "adm")
	admCnf := filepath.Join(admDir, "c.cnf")
	vars := "DB_NOME=rotaperfumes COLLATION=utf8mb4_unicode_ci REDE_NOME=" + redeD10 + " CONTAINER=" + servidorD10 + "\n" +
		"DB_APP=rotaperfumes_app DB_ADMIN=rotaperfumes_admin DB_BACKUP=rotaperfumes_backup DB_DBA=rotaperfumes-admin\n" +
		"IP_API=" + prefixoD10 + ".10 IP_ADMIN=" + prefixoD10 + ".20 IP_BACKUP=" + prefixoD10 + ".21 IP_DBA=" + prefixoD10 + ".22\n" +
		"SENHA_APP=SenhaAppD10Abc123456789XYZ01234567 SENHA_ADMIN=" + senhaAdm10 + " SENHA_BACKUP=SenhaBkpD10Abc123456789XYZ0123456 SENHA_DBA=" + senhaDBA10 + "\n" +
		"DBA_CLIENTE=" + clienteD10 + " C_DBA=/run/dba DBA_CNF=" + cnfHost + " SSL_DIR=" + ssl + " IMAGEM=" + r.base + "\n" +
		"DBA_AVISO_NOMES='" + avisoNomes + "'\n"
	escrever(t, filepath.Join(dir, "fn.sh"), sqlFn+genFn, 0o600)
	rodarBash := func(chamada string) string {
		cmd := exec.Command("bash", "-c", vars+"source \""+filepath.Join(dir, "fn.sh")+"\"\n"+chamada+"\n")
		cmd.Env = ambienteLimpo()
		out, err := cmd.Output()
		require.NoError(t, err)
		return string(out)
	}
	contas := rodarBash("sql_contas")
	for i := 0; i < 2; i++ { // idempotente, como na reexecução
		r.root(contas)
	}
	r.cmd = filepath.Join(dir, "rotaperfumes-dba")
	escrever(t, r.cmd, rodarBash("gerar_script_dba"), 0o700)

	// Option files como no servidor: root, 400. dir 711, só para o -f do comando.
	escrever(t, cnfHost, "[client]\nuser=rotaperfumes-admin\npassword="+senhaDBA10+"\n", 0o600)
	escrever(t, admCnf, "[client]\nuser=rotaperfumes_admin\npassword="+senhaAdm10+"\n", 0o600)
	_, se, err = r.docker("", "run", "--rm", "--network", "none", "-v", dir+":/x", "--entrypoint", "sh", r.base, "-c",
		"chown -R 0:0 /x/dba-cliente /x/adm && chmod 400 /x/dba-cliente/cliente.cnf /x/adm/c.cnf && chmod 711 /x/dba-cliente /x/adm")
	require.NoError(t, err, se)

	// stat e logger falsos só para rodar sem root.
	bin := filepath.Join(dir, "bin")
	escrever(t, bin+"/stat", "#!/bin/sh\necho '0 400'\n", 0o755)
	escrever(t, bin+"/logger", "#!/bin/sh\necho \"$*\" >>'"+filepath.Join(dir, "logger.log")+"'\n", 0o755)
	r.path = bin + ":" + os.Getenv("PATH")

	consulta := func(sql string) string {
		t.Helper()
		out, err := r.dba(sql)
		require.NoError(t, err, "%s", out)
		return out
	}
	recusa := func(sql, msg string) {
		t.Helper()
		out, err := r.dba(sql)
		require.Error(t, err, "%s: %s", sql, out)
		assert.Contains(t, out, msg, sql)
	}

	t.Run("entra pelo comando, com TLS 1.3 e fuso", func(t *testing.T) {
		assert.Contains(t, consulta("SELECT CURRENT_USER();"), "rotaperfumes-admin@"+prefixoD10+".22")
		assert.Contains(t, consulta("SHOW STATUS LIKE 'Ssl_version';"), "TLSv1.3")
		assert.Contains(t, consulta("SELECT @@time_zone;"), "-03:00")
		assert.Contains(t, consulta("SELECT DATABASE();"), "rotaperfumes")
	})
	t.Run("ALTER e CREATE no banco rotaperfumes", func(t *testing.T) {
		consulta("CREATE TABLE d10_t (id INT PRIMARY KEY, v INT); INSERT INTO d10_t VALUES (1, 1), (2, 2); ALTER TABLE d10_t ADD COLUMN w INT;")
		assert.Contains(t, consulta("SHOW COLUMNS FROM d10_t;"), "w")
	})
	t.Run("fora de rotaperfumes negado", func(t *testing.T) {
		r.root("CREATE DATABASE IF NOT EXISTS outro_d10; CREATE TABLE IF NOT EXISTS outro_d10.x (id INT);")
		recusa("SELECT * FROM mysql.global_priv;", "denied")
		recusa("SELECT * FROM outro_d10.x;", "denied")
		recusa("CREATE DATABASE novo_d10;", "denied")
		recusa("GRANT SELECT ON rotaperfumes.* TO 'rotaperfumes_app'@'"+prefixoD10+".10';", "denied")
		recusa("CREATE USER 'x_d10'@'%' IDENTIFIED BY 'y';", "denied")
		recusa("SET GLOBAL max_connections = 100;", "denied")
		grants := consulta("SHOW GRANTS;")
		assert.NotContains(t, grants, "WITH GRANT OPTION")
		assert.Contains(t, grants, "GRANT USAGE ON *.*", "sem privilégio global")
		assert.Contains(t, grants, "REQUIRE SSL")
	})
	t.Run("sem PROCESS: so as proprias sessoes", func(t *testing.T) {
		assert.Contains(t, consulta("SELECT COUNT(*) AS outras FROM information_schema.PROCESSLIST WHERE USER <> 'rotaperfumes-admin';"), "outras\n0")
	})
	t.Run("stdin para no primeiro erro", func(t *testing.T) {
		out, err := r.dba("SELECT * FROM mysql.global_priv;\nCREATE TABLE d10_depois_do_erro (id INT);\n")
		require.Error(t, err, out)
		assert.Equal(t, "0", r.root("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='rotaperfumes' AND TABLE_NAME='d10_depois_do_erro';"))
	})
	t.Run("local infile desligado", func(t *testing.T) {
		out, err := r.dba("LOAD DATA LOCAL INFILE '/etc/hostname' INTO TABLE d10_t;")
		require.Error(t, err, out)
	})
	t.Run("safe-updates so no interativo", func(t *testing.T) {
		out, err := r.dbaInterativo(r.path, "DELETE FROM d10_t;\nquit\n")
		assert.NoError(t, err, out)
		assert.Contains(t, out, "safe update mode", out)
		assert.Contains(t, out, "PRD rotaperfumes-admin>", "prompt de produção")
		assert.Equal(t, "2", r.root("SELECT COUNT(*) FROM rotaperfumes.d10_t;"), "DELETE sem WHERE não apagou")
		// Por stdin vale o --batch: o UPDATE sem WHERE é aceito de propósito (scripts).
		consulta("UPDATE d10_t SET v = 3;")
	})
	t.Run("MYSQL_HISTFILE sem arquivo", func(t *testing.T) {
		// docker falso: tira --rm e --read-only, para o docker diff enxergar a escrita.
		sessao := func(semHist bool) string {
			shim := filepath.Join(dir, fmt.Sprintf("hist-%v", semHist))
			filtro := `[ "$a" = --rm ] || [ "$a" = --read-only ] && continue`
			env := ""
			if semHist {
				env = `[ "$a" = MYSQL_HISTFILE=/dev/null ] && a=HIST_DESLIGADO=1`
			}
			real, err := exec.LookPath("docker")
			require.NoError(t, err)
			escrever(t, shim+"/docker", "#!/bin/bash\nif [ \"$1\" != run ]; then exec "+real+" \"$@\"; fi\nnovo=()\nfor a in \"$@\"; do\n"+filtro+"\n"+env+"\nnovo+=(\"$a\")\ndone\nexec "+real+" \"${novo[@]}\"\n", 0o755)
			out, err := r.dbaInterativo(shim+":"+r.path, "SELECT 1;\nquit\n")
			require.NoError(t, err, out)
			diff, se, err := r.docker("", "diff", clienteD10)
			require.NoError(t, err, se)
			_, se, err = r.docker("", "rm", "-fv", clienteD10)
			require.NoError(t, err, se)
			return diff
		}
		assert.NotContains(t, sessao(false), ".mariadb_history", "com MYSQL_HISTFILE=/dev/null")
		assert.Contains(t, sessao(true), ".mariadb_history", "controle: sem a variável, o arquivo apareceria")
	})
	t.Run("teste cruzado das duas contas", func(t *testing.T) {
		tls := []string{"--ssl-ca=/run/t/ca.pem", "--ssl-verify-server-cert", "-e", "SELECT CURRENT_USER()"}
		so, se, err := r.cliente(prefixoD10+".20", admCnf, tls...)
		require.NoError(t, err, se)
		assert.Contains(t, so, "rotaperfumes_admin@"+prefixoD10+".20", "controle")
		so, se, err = r.cliente(prefixoD10+".22", cnfHost, tls...)
		require.NoError(t, err, se)
		assert.Contains(t, so, "rotaperfumes-admin@"+prefixoD10+".22", "controle")

		_, se, err = r.cliente(prefixoD10+".20", cnfHost, tls...)
		require.Error(t, err, "rotaperfumes-admin do .20")
		assert.Contains(t, se, "Access denied")
		_, se, err = r.cliente(prefixoD10+".22", admCnf, tls...)
		require.Error(t, err, "rotaperfumes_admin do .22")
		assert.Contains(t, se, "Access denied")
	})
	t.Run("--skip-ssl recusado", func(t *testing.T) {
		_, se, err := r.cliente(prefixoD10+".22", cnfHost, "--skip-ssl", "-e", "SELECT 1")
		require.Error(t, err)
		assert.NotEmpty(t, se)
	})
	t.Run("--log-driver none mantem a saida do cliente", func(t *testing.T) {
		assert.Contains(t, consulta("SELECT 'sem-log-d10' AS marca;"), "sem-log-d10")
	})
	t.Run("registro sem SQL", func(t *testing.T) {
		b, err := os.ReadFile(filepath.Join(dir, "logger.log"))
		require.NoError(t, err)
		assert.Contains(t, string(b), "sessao modo=stdin")
		assert.Contains(t, string(b), "sessao modo=interativo")
		assert.NotContains(t, string(b), "SELECT")
		assert.NotContains(t, string(b), senhaDBA10)
	})
}
