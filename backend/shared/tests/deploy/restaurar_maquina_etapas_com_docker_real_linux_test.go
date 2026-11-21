//go:build linux

package deploy_test

// DEPLOY-09, Docker real, dados sintéticos, sem root: etapas 5 a 8 com as funções
// reais do script (extraídas do texto), contra um servidor descartável com o TLS e o
// SQL de contas reais. Só recursos rp-*-d09, removidos ao fim. O script inteiro exige
// root e caminhos fixos que o daemon resolve no host: de ponta a ponta, só numa
// máquina real. O "docker compose" entra se o plugin existir; sem ele, o volume nasce
// de "docker volume create" (com as labels) e "docker create", que é o que o compose
// pede ao daemon, e o teste registra isso. Ex.:
// sg docker -c "DEPLOY09_DOCKER=1 go test -count=1 -v -run TestDEPLOY09_Real ./tests/deploy/"

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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
	imagemSrvD09  = "rp-img-srv-d09:teste"
	imagemApiD09  = "rp-img-api-d09:teste"
	servidorD09   = "rp-mariadb-d09"
	redeD09       = "rp-net-d09"
	prefixoD09    = "172.30.145"
	clienteD09    = "rp-dba-cliente-d09"
	volumeD09     = "rp-proj-d09_produto_imagens"
	volumeRootD09 = "rp-vol-root-d09"
	criadoD09     = "rp-create-d09"
	projetoD09    = "rp-proj-d09"
	senhaRootD09  = "SenhaRootD09Abc123456789XYZ012345"
)

type realD09 struct {
	t       *testing.T
	dir     string
	base    string
	prelude string
}

func (r *realD09) docker(stdin []byte, args ...string) (string, string, error) {
	cmd := exec.Command("docker", args...)
	cmd.Env = ambienteLimpo()
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	return so.String(), se.String(), err
}

func (r *realD09) root(banco, sql string) string {
	r.t.Helper()
	so, se, err := r.docker([]byte(sql), "exec", "-i", servidorD09, "mariadb", "--defaults-extra-file=/run/rotaperfumes/root.cnf", "-N", "-B", banco)
	require.NoError(r.t, err, "%s", se)
	return strings.TrimSpace(so)
}

func (r *realD09) limpar() {
	_, _, _ = r.docker(nil, "rm", "-fv", clienteD09, criadoD09, servidorD09)
	_, _, _ = r.docker(nil, "network", "rm", redeD09)
	_, _, _ = r.docker(nil, "volume", "rm", "-f", volumeD09, volumeRootD09)
	_, _, _ = r.docker(nil, "run", "--rm", "--network", "none", "-v", r.dir+":/x", "--entrypoint", "sh", r.base, "-c", "rm -rf /x/dba-cliente")
	_, _, _ = r.docker(nil, "image", "rm", imagemSrvD09, imagemApiD09)
}

// fn roda uma chamada com as funções reais do script. Devolve saída, erro e código.
func (r *realD09) fn(stdin []byte, chamada string) (string, string, error) {
	cmd := exec.Command("bash", "-c", r.prelude+"\n"+chamada+"\n")
	cmd.Env = ambienteLimpo()
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	return so.String(), se.String(), err
}

func dumpSinteticoD09(r *realD09) []byte {
	r.t.Helper()
	var sql strings.Builder
	sql.WriteString("CREATE DATABASE origem_d09; USE origem_d09;\n")
	sql.WriteString("CREATE TABLE usuarios (id INT PRIMARY KEY, role VARCHAR(20)); INSERT INTO usuarios VALUES (1,'admin'),(2,'vendedor'),(3,'vendedor');\n")
	sql.WriteString("CREATE TABLE vendedores (id INT PRIMARY KEY); INSERT INTO vendedores VALUES (1),(2);\n")
	sql.WriteString("CREATE TABLE clientes (id INT PRIMARY KEY, nome VARCHAR(40)); INSERT INTO clientes VALUES (1,'SINTETICO-D09');\n")
	sql.WriteString("CREATE TABLE refresh_tokens (id INT PRIMARY KEY); INSERT INTO refresh_tokens VALUES (1),(2),(3),(4);\n")
	for i := 0; i < 9; i++ {
		fmt.Fprintf(&sql, "CREATE TABLE extra_%d (id INT PRIMARY KEY); INSERT INTO extra_%d VALUES (%d);\n", i, i, i)
	}
	r.root("", sql.String())
	so, se, err := r.docker(nil, "exec", servidorD09, "mariadb-dump", "--defaults-extra-file=/run/rotaperfumes/root.cnf",
		"--single-transaction", "--skip-lock-tables", "--no-tablespaces", "--skip-add-drop-table", "--skip-triggers", "--hex-blob", "--default-character-set=utf8mb4", "origem_d09")
	require.NoError(r.t, err, "%s", se)
	r.root("", "DROP DATABASE origem_d09;")
	return gzipD09(r.t, so)
}

func tarSinteticoD09(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	w := tar.NewWriter(z)
	require.NoError(t, w.WriteHeader(&tar.Header{Name: "./", Mode: 0o750, Typeflag: tar.TypeDir}))
	require.NoError(t, w.WriteHeader(&tar.Header{Name: "./sub/", Mode: 0o750, Typeflag: tar.TypeDir}))
	for _, a := range []struct{ nome, dado string }{{"./a.jpg", "AAA"}, {"./sub/b.jpg", "BBBB"}, {"./sub/c d.jpg", "CC"}} {
		require.NoError(t, w.WriteHeader(&tar.Header{Name: a.nome, Mode: 0o640, Size: int64(len(a.dado)), Typeflag: tar.TypeReg}))
		_, err := w.Write([]byte(a.dado))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	require.NoError(t, z.Close())
	return b.Bytes()
}

func TestDEPLOY09_Real(t *testing.T) {
	precisaBash(t)
	if os.Getenv("DEPLOY09_DOCKER") != "1" {
		t.Skip("opt-in: DEPLOY09_DOCKER=1, com Docker (não exige root)")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker inacessível: " + err.Error())
	}
	ferramentaObrigatoria(t, "openssl")

	sc := lerRaiz(t, "deploy", "restaurar-maquina.sh")
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	imagem := regexp.MustCompile(`(?m)^IMAGEM_BASE="([^"]+)"$`).FindStringSubmatch(sc)
	require.NotNil(t, imagem)
	sqlMode := regexp.MustCompile(`(?m)^readonly SQL_MODE="(.*)"$`).FindStringSubmatch(md)
	require.NotNil(t, sqlMode)

	dir, err := os.MkdirTemp("", "rp-d09-real-*")
	require.NoError(t, err)
	require.NoError(t, os.Chmod(dir, 0o711))
	r := &realD09{t: t, dir: dir, base: imagem[1]}
	r.limpar()
	t.Cleanup(func() {
		r.limpar()
		_ = os.RemoveAll(dir)
	})

	// Servidor: TLS e cnf como os do mariadb-docker.sh.
	ssl := filepath.Join(dir, "ssl")
	require.NoError(t, os.MkdirAll(ssl, 0o755))
	ossl := func(args ...string) {
		out, err := exec.Command("openssl", args...).CombinedOutput()
		require.NoError(t, err, "%s", out)
	}
	ossl("req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", ssl+"/ca-key.pem", "-out", ssl+"/ca.pem", "-days", "2", "-subj", "/CN=rp-d09-ca",
		"-addext", "basicConstraints=critical,CA:TRUE", "-addext", "keyUsage=critical,keyCertSign,cRLSign")
	ossl("req", "-new", "-newkey", "rsa:2048", "-nodes", "-keyout", ssl+"/server-key.pem", "-subj", "/CN=rp-d09", "-out", ssl+"/server.csr")
	escrever(t, ssl+"/ext", "basicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:"+servidorD09+"\n", 0o644)
	ossl("x509", "-req", "-in", ssl+"/server.csr", "-CA", ssl+"/ca.pem", "-CAkey", ssl+"/ca-key.pem", "-set_serial", "0x1234", "-days", "2", "-extfile", ssl+"/ext", "-out", ssl+"/server-cert.pem")
	for _, f := range []string{"ca.pem", "server-cert.pem", "server-key.pem"} {
		require.NoError(t, os.Chmod(filepath.Join(ssl, f), 0o644))
	}
	build := filepath.Join(dir, "build")
	escrever(t, build+"/zz.cnf", "[mariadb]\nssl_ca=/etc/mysql/ssl/ca.pem\nssl_cert=/etc/mysql/ssl/server-cert.pem\nssl_key=/etc/mysql/ssl/server-key.pem\nrequire_secure_transport=ON\ntls_version=TLSv1.2,TLSv1.3\nplugin_load_add=auth_ed25519\nlocal_infile=0\nsql_mode="+sqlMode[1]+"\ncharacter_set_server=utf8mb4\ncollation_server=utf8mb4_unicode_ci\nskip_name_resolve=ON\n", 0o644)
	escrever(t, build+"/Dockerfile", "FROM "+r.base+"\nCOPY zz.cnf /etc/mysql/conf.d/zz-rotaperfumes.cnf\nRUN chmod 644 /etc/mysql/conf.d/zz-rotaperfumes.cnf\n", 0o644)
	_, se, err := r.docker(nil, "build", "-q", "-t", imagemSrvD09, build)
	require.NoError(t, err, se)
	_, se, err = r.docker(nil, "network", "create", "--driver", "bridge", "--subnet", prefixoD09+".0/24", "--gateway", prefixoD09+".1", "--ip-range", prefixoD09+".128/25", redeD09)
	require.NoError(t, err, se)

	segredos := filepath.Join(dir, "root")
	escrever(t, segredos+"/root.senha", senhaRootD09, 0o644)
	escrever(t, segredos+"/root.cnf", "[client]\nuser=root\npassword="+senhaRootD09+"\nprotocol=socket\n", 0o644)
	require.NoError(t, os.Chmod(segredos, 0o755))
	_, se, err = r.docker(nil, "run", "-d", "--name", servidorD09, "--network", redeD09, "--ip", prefixoD09+".2", "--user", "999:999", "--cap-drop", "ALL",
		"--security-opt", "no-new-privileges:true", "--read-only",
		"--tmpfs", "/run/mysqld:rw,nosuid,nodev,noexec,mode=0755,uid=999,gid=999",
		"--tmpfs", "/tmp:rw,nosuid,nodev,noexec,mode=1777",
		"--tmpfs", "/var/lib/mysql:rw,nosuid,nodev,noexec,mode=0700,uid=999,gid=999",
		"-e", "MARIADB_ROOT_PASSWORD_FILE=/run/rotaperfumes/root.senha", "-e", "MARIADB_ROOT_HOST=localhost",
		"-v", ssl+":/etc/mysql/ssl:ro", "-v", segredos+":/run/rotaperfumes:ro", imagemSrvD09)
	require.NoError(t, err, se)
	require.Eventually(t, func() bool {
		so, _, err := r.docker(nil, "exec", servidorD09, "mariadb", "--defaults-extra-file=/run/rotaperfumes/root.cnf", "-N", "-B", "-e", "SELECT @@skip_networking")
		return err == nil && strings.TrimSpace(so) == "0"
	}, 180*time.Second, 2*time.Second, "o servidor não ficou pronto")

	// Contas: o SQL real do mariadb-docker.sh.
	sqlFn := regexp.MustCompile(`(?ms)^sql_contas\(\) \{.*?^\}\n`).FindString(md)
	require.NotEmpty(t, sqlFn)
	const senhaDBA = "SenhaDbaD09Abc123456789XYZ0123456"
	cmdSQL := exec.Command("bash", "-c", "DB_NOME=rotaperfumes COLLATION=utf8mb4_unicode_ci REDE_NOME="+redeD09+
		" DB_APP=rotaperfumes_app DB_ADMIN=rotaperfumes_admin DB_BACKUP=rotaperfumes_backup DB_DBA=rotaperfumes-admin"+
		" IP_API="+prefixoD09+".10 IP_ADMIN="+prefixoD09+".20 IP_BACKUP="+prefixoD09+".21 IP_DBA="+prefixoD09+".22"+
		" SENHA_APP=SenhaAppD09Abc123456789XYZ01234567 SENHA_ADMIN=SenhaAdmD09Abc123456789XYZ01234567 SENHA_BACKUP=SenhaBkpD09Abc123456789XYZ0123456 SENHA_DBA="+senhaDBA+"\n"+sqlFn+"sql_contas\n")
	cmdSQL.Env = ambienteLimpo()
	contas, err := cmdSQL.Output()
	require.NoError(t, err)
	r.root("", string(contas))

	// Option file como no servidor: root, 400.
	cnfDir := filepath.Join(dir, "dba-cliente")
	cnf := filepath.Join(cnfDir, "cliente.cnf")
	escrever(t, cnf, "[client]\nuser=rotaperfumes-admin\npassword="+senhaDBA+"\n", 0o600)
	_, se, err = r.docker(nil, "run", "--rm", "--network", "none", "-v", dir+":/x", "--entrypoint", "sh", r.base, "-c",
		"chown -R 0:0 /x/dba-cliente && chmod 400 /x/dba-cliente/cliente.cnf && chmod 700 /x/dba-cliente")
	require.NoError(t, err, se)

	// As funções reais, com as constantes apontando para os recursos rp-*-d09.
	contagens := filepath.Join(dir, "contagens")
	var fns strings.Builder
	fmt.Fprintf(&fns, "set -euo pipefail\numask 077\nIMAGEM_BASE=%q\nCONTAINER_DB=%s\nREDE_DB=%s\nIP_DBA=%s.22\nSSL_DIR=%s\nDBA_CNF=%s\nDBA_CLIENTE=%s\nC_DBA=/run/dba\nDB_NOME=rotaperfumes\nTRAB=%s\nERRO_ARQ=\"\"\nCONTAGENS=%s\nVOLUME_IMAGENS=%s\nVOLUME_ROTULO=%s/produto_imagens\nUSUARIO_IMAGENS=65532:65532\nC_IMAGENS=/imagens\n",
		r.base, servidorD09, redeD09, prefixoD09, ssl, cnf, clienteD09, dir, contagens, volumeD09, projetoD09)
	fns.WriteString(`die() { printf 'RECUSADO: %s\n' "$*" >&2; exit 1; }` + "\n")
	fns.WriteString(regexp.MustCompile(`(?m)^root_sql\(\) \{.*\}\n`).FindString(sc))
	for _, n := range []string{"contar_tabelas", "cliente_importacao", "importar_dump", "comparar_contagens", "volume_cliente", "conferir_volume_vazio", "extrair_imagens", "contar_arquivos_volume"} {
		fns.WriteString(funcaoShell(t, sc, n))
	}
	r.prelude = fns.String()

	dumpArq := filepath.Join(dir, "rotaperfumes-"+carimboD09+".sql.gz")
	require.NoError(t, os.WriteFile(dumpArq, dumpSinteticoD09(r), 0o644))

	t.Run("verificar-backup real aprova e dá as contagens", func(t *testing.T) {
		cmd := exec.Command("bash", caminhoRaiz(t, "deploy", "verificar-backup.sh"), dumpArq)
		cmd.Env = ambienteLimpo()
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s", out)
		require.True(t, strings.HasSuffix(strings.TrimSpace(string(out)), "APROVADO"), "%s", out)
		// O mesmo awk do script (etapa 1).
		const awkEtapa1 = `awk '/^  [A-Za-z0-9_]+ +[0-9]+$/ { print $1, $2 }'`
		require.Contains(t, sc, awkEtapa1)
		c := exec.Command("bash", "-c", awkEtapa1+" >'"+contagens+"'")
		c.Stdin = bytes.NewReader(out)
		require.NoError(t, c.Run())
		lidas := linhas(lerArquivo(t, contagens))
		assert.Len(t, lidas, 13)
		assert.Contains(t, lidas, "usuarios 3")
		assert.Contains(t, lidas, "refresh_tokens 4")
	})

	t.Run("banco vazio: zero tabelas", func(t *testing.T) {
		so, se, err := r.fn(nil, "contar_tabelas")
		require.NoError(t, err, se)
		assert.Equal(t, "0", so)
	})

	t.Run("sandbox: o dump não executa comando do cliente", func(t *testing.T) {
		arq := filepath.Join(dir, "maldoso.sql.gz")
		require.NoError(t, os.WriteFile(arq, gzipD09(t, "\\! touch /tmp/invadido\nsystem touch /tmp/invadido\n"), 0o644))
		_, se, err := r.fn(nil, "importar_dump '"+arq+"'")
		require.Error(t, err, "--sandbox precisa recusar")
		assert.Contains(t, se, "cliente=1")
		assert.Equal(t, "0", r.root("", "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = 'rotaperfumes'"))
	})

	t.Run("erro na importação mostra só código e linha", func(t *testing.T) {
		arq := filepath.Join(dir, "duplicado.sql.gz")
		require.NoError(t, os.WriteFile(arq, gzipD09(t, "CREATE TABLE d (id VARCHAR(40) PRIMARY KEY);\nINSERT INTO d VALUES ('DADO-SECRETO-D09'),('DADO-SECRETO-D09');\n"), 0o644))
		_, se, err := r.fn(nil, "importar_dump '"+arq+"'")
		require.Error(t, err)
		assert.Contains(t, se, "ERROR 1062 (23000) at line 2")
		assert.NotContains(t, se, "DADO-SECRETO-D09")
		assert.NotContains(t, se, "Duplicate")
		matches, _ := filepath.Glob(filepath.Join(dir, ".erro.*"))
		assert.Empty(t, matches, "o arquivo de erro não fica")
		r.root("", "DROP DATABASE rotaperfumes; CREATE DATABASE rotaperfumes CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;")
	})

	t.Run("etapa 5: importa pela conta real, compara e zera refresh_tokens", func(t *testing.T) {
		_, se, err := r.fn(nil, "importar_dump '"+dumpArq+"'")
		require.NoError(t, err, se)
		_, se, err = r.fn(nil, "comparar_contagens")
		require.NoError(t, err, se)
		so, se, err := r.fn([]byte("SELECT CURRENT_USER(); SHOW STATUS LIKE 'Ssl_version'; SELECT @@time_zone;\n"), "cliente_importacao")
		require.NoError(t, err, se)
		assert.Contains(t, so, "rotaperfumes-admin@"+prefixoD09+".22")
		assert.Contains(t, so, "TLSv1.3")
		assert.Contains(t, so, "-03:00")
		// O comando real (rotaperfumes-dba) tem teste próprio no DEPLOY-10.
		_, se, err = r.fn([]byte("TRUNCATE TABLE refresh_tokens;\n"), "cliente_importacao")
		require.NoError(t, err, se)
		assert.Equal(t, "0", r.root("rotaperfumes", "SELECT COUNT(*) FROM refresh_tokens"))
		_, se, err = r.fn(nil, "comparar_contagens refresh_tokens")
		require.NoError(t, err, se, "retomada: reconfere, ignorando a tabela zerada")
		_, se, err = r.fn(nil, "comparar_contagens")
		require.Error(t, err)
		assert.Contains(t, se, "contagem diferente na tabela refresh_tokens")
		so, _, err = r.fn(nil, "contar_tabelas")
		require.NoError(t, err)
		assert.Equal(t, "13", so, "o banco já tem dados: a recusa enxerga isto")
	})

	// Etapa 7: o volume nasce com labels e dono. Imagem no lugar da API, como no backend/Dockerfile.
	apiCtx := filepath.Join(dir, "api")
	escrever(t, apiCtx+"/Dockerfile", "FROM "+r.base+"\nRUN mkdir -p /data/produtos-imagens && chown 65532:65532 /data/produtos-imagens && chmod 0750 /data/produtos-imagens\n", 0o644)
	_, se, err = r.docker(nil, "build", "-q", "-t", imagemApiD09, apiCtx)
	require.NoError(t, err, se)

	if _, _, err := r.docker(nil, "compose", "version"); err == nil {
		t.Log("plugin docker compose presente: o volume nasce do up --no-start real")
		compose := filepath.Join(dir, "compose.yml")
		escrever(t, compose, "name: "+projetoD09+"\nservices:\n  api:\n    image: "+imagemApiD09+"\n    volumes:\n      - produto_imagens:/data/produtos-imagens\nvolumes:\n  produto_imagens:\n", 0o644)
		_, se, err := r.docker(nil, "compose", "-f", compose, "-p", projetoD09, "up", "--no-start")
		require.NoError(t, err, se)
		t.Cleanup(func() { _, _, _ = r.docker(nil, "compose", "-f", compose, "-p", projetoD09, "down", "-v") })
	} else {
		t.Log("SEM o plugin docker compose: volume por docker volume create + docker create. O up --no-start real NAO foi exercitado.")
		_, se, err := r.docker(nil, "volume", "create", "--label", "com.docker.compose.project="+projetoD09, "--label", "com.docker.compose.volume=produto_imagens", volumeD09)
		require.NoError(t, err, se)
		_, se, err = r.docker(nil, "create", "--name", criadoD09, "-v", volumeD09+":/data/produtos-imagens", imagemApiD09)
		require.NoError(t, err, se)
	}

	t.Run("etapa 8: volume recém-criado passa, com dono 65532 e vazio", func(t *testing.T) {
		_, se, err := r.fn(nil, "conferir_volume_vazio </dev/null")
		require.NoError(t, err, "a hipótese da especificação: %s", se)
	})

	t.Run("etapa 8: extrai como 65532, sem apagar nada, e a contagem bate", func(t *testing.T) {
		tgz := filepath.Join(dir, "rotaperfumes-imagens-"+carimboD09+".tar.gz")
		require.NoError(t, os.WriteFile(tgz, tarSinteticoD09(t), 0o644))
		_, se, err := r.fn(nil, "extrair_imagens '"+tgz+"'")
		require.NoError(t, err, se)
		so, se, err := r.fn(nil, "contar_arquivos_volume")
		require.NoError(t, err, se)
		assert.Equal(t, "3", strings.TrimSpace(so))
		lista, se, err := r.docker(nil, "run", "--rm", "--network", "none", "--read-only", "--user", "65532:65532",
			"--mount", "type=volume,source="+volumeD09+",target=/imagens,readonly", "--entrypoint", "bash", r.base, "-c",
			`cd /imagens && find . -type f -printf '%p %u:%g %m %s\n' | sort; stat -c '%u:%g %a' .`)
		require.NoError(t, err, se)
		assert.Contains(t, lista, "./a.jpg 65532:65532 640 3")
		assert.Contains(t, lista, "./sub/c d.jpg 65532:65532 640 2")
		assert.Contains(t, lista, "65532:65532 750", "a raiz segue como a imagem criou")
	})

	t.Run("volume com conteúdo é recusado, sem apagar", func(t *testing.T) {
		_, se, err := r.fn(nil, "conferir_volume_vazio </dev/null")
		require.Error(t, err)
		assert.Contains(t, se, "tem conteudo")
	})

	t.Run("volume de outro dono: para antes de escrever", func(t *testing.T) {
		_, se, err := r.docker(nil, "volume", "create", "--label", "com.docker.compose.project="+projetoD09, "--label", "com.docker.compose.volume=produto_imagens", volumeRootD09)
		require.NoError(t, err, se)
		prelude := r.prelude
		r.prelude = strings.Replace(prelude, "VOLUME_IMAGENS="+volumeD09, "VOLUME_IMAGENS="+volumeRootD09, 1)
		defer func() { r.prelude = prelude }()
		_, se, err = r.fn(nil, "conferir_volume_vazio </dev/null")
		require.Error(t, err)
		assert.Contains(t, se, "a copia da imagem nao aconteceu")
		// E a extração, se chegasse lá, falha em vez de seguir: volume do root, escrita como 65532.
		tgz := filepath.Join(dir, "rotaperfumes-imagens-"+carimboD09+".tar.gz")
		_, se, err = r.fn(nil, "extrair_imagens '"+tgz+"'")
		require.Error(t, err, "a escrita como 65532 tem de falhar")
		assert.Contains(t, se, "a extracao falhou")
	})

	t.Run("labels de outro projeto são recusadas", func(t *testing.T) {
		prelude := r.prelude
		r.prelude = strings.Replace(prelude, "VOLUME_ROTULO="+projetoD09+"/produto_imagens", "VOLUME_ROTULO=rotaperfumes/produto_imagens", 1)
		defer func() { r.prelude = prelude }()
		_, se, err := r.fn(nil, "conferir_volume_vazio </dev/null")
		require.Error(t, err)
		assert.Contains(t, se, "nao foi criado pelo compose")
	})
}

func lerArquivo(t *testing.T, caminho string) string {
	t.Helper()
	b, err := os.ReadFile(caminho)
	require.NoError(t, err)
	return string(b)
}
