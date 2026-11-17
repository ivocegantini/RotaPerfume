//go:build linux

package deploy_test

// Lote 42: achados da revisão do SecBrain sobre BACKUP-01/02 e DEPLOY-09/10.
// Cada teste falha sem a correção correspondente (mutação conferida numa cópia
// temporária). Usa os ensaios do mariadb-docker.sh (novoEnsaio), do
// seed-producao.sh (novoEnsaioSeed/novoEnsaioTTY) e do restaurar-maquina.sh
// (novoEnsaioD09). O opt-in LOTE42_DOCKER=1 roda o DROP da conta legada num
// MariaDB descartável de verdade (rp-quote-l42).

import (
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

// ---------------------------------------------------------------------------
// 1. --backup-janela-min acima de 60 exige --sem-backup-motivo; janela=N no registro
// ---------------------------------------------------------------------------
func TestLote42_Janela_Recriar(t *testing.T) {
	registro := func(e *ensaio) string { return e.arq(filepath.Join(segredos, "recriar.registro")) }
	const motivo = "dump das 03:30, vendas paradas desde ontem"
	dumpVelho := func(t *testing.T, dir string, idade time.Duration) string {
		p := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(-idade)+".sql.gz")
		velho := time.Now().Add(-idade)
		require.NoError(t, os.Chtimes(p, velho, velho))
		return p
	}
	for _, janela := range []string{"61", "180", "1440", "0090"} {
		t.Run("janela "+janela+" sem motivo recusa antes de mudar algo", func(t *testing.T) {
			e, dir := novaGuarda(t)
			dumpVelho(t, dir, 50*time.Minute)
			r := e.rodar("--recriar", "--backup-dir", dir, "--backup-janela-min", janela)
			require.Error(t, r.err, r.out)
			assert.Contains(t, r.out, "--backup-janela-min acima de 60 exige --sem-backup-motivo")
			assert.NotContains(t, e.log(), "docker build")
			assert.NotContains(t, e.log(), "docker rm")
			assert.NoFileExists(t, filepath.Join(e.dir, segredos, "recriar.registro"))
		})
	}
	t.Run("janela 180 com motivo confere o dump com a janela e registra", func(t *testing.T) {
		e, dir := novaGuarda(t)
		p := dumpVelho(t, dir, 2*time.Hour)
		r := e.rodar("--recriar", "--backup-dir", dir, "--backup-janela-min", "180", "--sem-backup-motivo", motivo)
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "--backup-janela-min 180 (acima de 60)")
		assert.Contains(t, r.out, "Backup conferido (janela de 180 min): "+p)
		assert.Contains(t, registro(e), "janela=180 backup="+p+" motivo="+motivo+" imagem_base=")
	})
	t.Run("janela 180 com motivo nao dispensa o dump", func(t *testing.T) {
		e, dir := novaGuarda(t)
		r := e.rodar("--recriar", "--backup-dir", dir, "--backup-janela-min", "180", "--sem-backup-motivo", motivo)
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "nenhum backup")
		assert.NotContains(t, e.log(), "docker rm")
	})
	t.Run("janela 180 com motivo e dump fora dela recusa", func(t *testing.T) {
		e, dir := novaGuarda(t)
		dumpVelho(t, dir, 4*time.Hour)
		r := e.rodar("--recriar", "--backup-dir", dir, "--backup-janela-min", "180", "--sem-backup-motivo", motivo)
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "tem mais de 180 min")
	})
	t.Run("motivo com a janela padrao segue dispensando e registra a janela 60", func(t *testing.T) {
		e, dir := novaGuarda(t)
		r := e.rodar("--recriar", "--backup-dir", dir, "--sem-backup-motivo", motivo)
		require.NoError(t, r.err, r.out)
		assert.Contains(t, registro(e), "janela=60 backup=nenhum motivo="+motivo)
	})
	t.Run("janela 60 sem motivo segue valendo", func(t *testing.T) {
		e, dir := novaGuarda(t)
		dumpVelho(t, dir, 30*time.Minute)
		r := e.rodar("--recriar", "--backup-dir", dir, "--backup-janela-min", "60")
		require.NoError(t, r.err, r.out)
		assert.Contains(t, registro(e), "janela=60 backup=")
	})
}

func TestLote42_Janela_SeedForcar(t *testing.T) {
	const motivo = "dump das 03:30, vendas paradas desde ontem"
	t.Run("janela 120 sem motivo recusa antes de qualquer conexao", func(t *testing.T) {
		e := novoEnsaioSeed(t)
		out, err := e.rodar("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-janela-min", "120")
		require.Error(t, err, out)
		assert.Contains(t, out, "--backup-janela-min acima de 60 exige --sem-backup-motivo")
		assert.Empty(t, e.ler("log"))
	})
	t.Run("janela 120 com motivo confere o dump e registra a janela", func(t *testing.T) {
		e := novoEnsaioTTY(t)
		e.state("linhas", "5\n")
		dir := filepath.Join(e.dir, "backups")
		require.NoError(t, os.MkdirAll(dir, 0o700))
		p := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(-90*time.Minute)+".sql.gz")
		velho := time.Now().Add(-90 * time.Minute)
		require.NoError(t, os.Chtimes(p, velho, velho))
		s := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-dir", dir,
			"--backup-janela-min", "120", "--sem-backup-motivo", motivo)
		s.digitar(promptNomeDoBanco, "rotaperfumes")
		out, err := s.terminar()
		require.NoError(t, err, out)
		assert.Contains(t, out, "Backup conferido (janela de 120 min)")
		assert.Contains(t, e.ler("root/rotaperfumes-mariadb/seed-forcar.registro"), "janela=120 backup="+p+" motivo="+motivo)
	})
	t.Run("janela 120 com motivo sem dump recusa", func(t *testing.T) {
		e := novoEnsaioTTY(t)
		e.state("linhas", "5\n")
		dir := filepath.Join(e.dir, "backups")
		require.NoError(t, os.MkdirAll(dir, 0o700))
		s := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-dir", dir,
			"--backup-janela-min", "120", "--sem-backup-motivo", motivo)
		out, err := s.terminar()
		require.Error(t, err, out)
		assert.Contains(t, out, "nenhum backup")
		assert.NotContains(t, out, promptNomeDoBanco)
	})
	t.Run("padrao registra a janela 60", func(t *testing.T) {
		e := novoEnsaioTTY(t)
		e.state("linhas", "5\n")
		dir := filepath.Join(e.dir, "backups")
		require.NoError(t, os.MkdirAll(dir, 0o700))
		p := dumpRecente(t, dir, "rotaperfumes-"+carimboAgora(0)+".sql.gz")
		s := e.rodarComTerminal("--pepper-file", "/root/pepper.txt", "--forcar", "--backup-dir", dir)
		s.digitar(promptNomeDoBanco, "rotaperfumes")
		out, err := s.terminar()
		require.NoError(t, err, out)
		assert.Contains(t, e.ler("root/rotaperfumes-mariadb/seed-forcar.registro"), "janela=60 backup="+p+" motivo=nenhum")
	})
}

// ---------------------------------------------------------------------------
// 4. Conta casaos com host fora do charset: DROP gerado pelo QUOTE do servidor
// ---------------------------------------------------------------------------
func TestLote42_Legado_HostForaDoCharset(t *testing.T) {
	const contas = "casaos\t%\ncasaos\tevil'h;DROP DATABASE mysql;--\ncasaos\tbarra\\invertida\n" +
		"rotaperfumes_app\tapp'estranho\n"
	t.Run("remove em qualquer host com o texto do QUOTE", func(t *testing.T) {
		e := novoEnsaio(t)
		e.setState("contas", contas)
		r := e.rodar("--nova-ca")
		require.NoError(t, r.err, r.out)
		sql := e.state("sql")
		for _, u := range []string{`'casaos'@'%'`, `'casaos'@'evil\'h;DROP DATABASE mysql;--'`, `'casaos'@'barra\\invertida'`} {
			assert.Contains(t, sql, "DROP USER IF EXISTS "+u+";")
		}
		assert.Equal(t, 1, strings.Count(sql, "DROP USER IF EXISTS 'casaos'@'%';"), "uma vez so, pelo QUOTE")
		assert.NotContains(t, sql, "DROP USER IF EXISTS 'casaos'@'evil'h", "nada montado pelo shell com o valor lido")
		assert.Contains(t, r.out, "casaos com host fora do padrao (DROP gerado pelo servidor, com QUOTE)")
		assert.NotContains(t, r.out, "conta casaos com host fora do padrao NAO removida")
		// As outras contas do projeto com host estranho seguem so com aviso.
		assert.Contains(t, r.out, "conta rotaperfumes_app com host fora do padrao NAO removida automaticamente")
		assert.NotContains(t, sql, "app'estranho")
		// O texto do servidor sem o escape do modo batch.
		assert.Contains(t, e.state("sql_legado"), "--raw -e SELECT CONCAT('DROP USER IF EXISTS ', QUOTE(User), '@', QUOTE(Host), ';') FROM mysql.global_priv WHERE User = 'casaos'")
	})
	t.Run("sobrou conta legada para", func(t *testing.T) {
		e := novoEnsaio(t)
		e.setState("contas", contas)
		e.setState("legado_resta", "1\n")
		r := e.rodar("--nova-ca")
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "a conta casaos continua no servidor (1 host(s)) depois do DROP USER")
		assert.NotContains(t, r.out, "=== [8/8]", "nao segue para o teste de conexao")
	})
	t.Run("OUTRAS nao exclui mais a conta legada", func(t *testing.T) {
		e := novoEnsaio(t)
		r := e.rodar("--nova-ca")
		require.NoError(t, r.err, r.out)
		outras := e.state("sql_outras")
		require.NotEmpty(t, outras)
		assert.NotContains(t, outras, "'casaos'")
	})
	t.Run("estatico: o DROP da conta legada nunca usa o valor lido pelo shell", func(t *testing.T) {
		md := codigoSemComentarios(lerRaiz(t, "deploy", "mariadb-docker.sh"))
		assert.NotRegexp(t, `DROP USER[^\n]*'\$DB_LEGADO'@'\$`, md)
		assert.Regexp(t, `QUOTE\(User\), '@', QUOTE\(Host\)`, md)
	})
}

// Opt-in, com Docker: o SQL da conta legada, extraído do script, num MariaDB real.
func TestLote42_Legado_QuoteReal(t *testing.T) {
	precisaBash(t)
	if os.Getenv("LOTE42_DOCKER") != "1" {
		t.Skip("opt-in: LOTE42_DOCKER=1, com Docker (não exige root)")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("docker inacessível: " + err.Error())
	}
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	imagem := regexp.MustCompile(`(?m)^IMAGEM_BASE="([^"]+)"$`).FindStringSubmatch(md)
	require.NotNil(t, imagem)
	gera := regexp.MustCompile(`(?m)^LEGADO_SQL=\$\(root_sql --raw -e "(.+)"\)$`).FindStringSubmatch(md)
	require.NotNil(t, gera, "consulta que gera o DROP")
	conta := regexp.MustCompile(`(?m)^LEGADO_RESTA=\$\(root_sql -e "(.+)"\)$`).FindStringSubmatch(md)
	require.NotNil(t, conta)
	q := func(s string) string { return strings.ReplaceAll(s, "$DB_LEGADO", "casaos") }

	const nome = "rp-quote-l42"
	_ = exec.Command("docker", "rm", "-f", nome).Run()
	t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", nome).Run() })
	out, err := exec.Command("docker", "run", "-d", "--rm", "--pull", "never", "--name", nome, "--network", "none",
		"-e", "MARIADB_ROOT_PASSWORD=SenhaRootL42abc123", imagem[1]).CombinedOutput()
	require.NoError(t, err, "%s", out)
	sql := func(stdin string, args ...string) (string, error) {
		c := exec.Command("docker", append([]string{"exec", "-i", nome, "mariadb", "-uroot", "-pSenhaRootL42abc123", "-N", "-B"}, args...)...)
		c.Stdin = strings.NewReader(stdin)
		o, err := c.CombinedOutput()
		return string(o), err
	}
	pronto := false
	for i := 0; i < 90 && !pronto; i++ {
		if o, err := sql("", "-e", "SELECT 1"); err == nil && strings.TrimSpace(o) == "1" {
			pronto = true
			break
		}
		time.Sleep(time.Second)
	}
	require.True(t, pronto, "o MariaDB descartável não subiu")

	hosts := []string{"%", "evil'h;DROP DATABASE mysql;--", `barra\invertida`, "quebra\nde linha", "tab\taqui", "aspas\"duplas", "ponto;e virgula"}
	var cria strings.Builder
	for _, h := range hosts {
		hq := strings.NewReplacer(`\`, `\\`, `'`, `\'`, "\n", `\n`, "\t", `\t`).Replace(h)
		fmt.Fprintf(&cria, "CREATE USER 'casaos'@'%s' IDENTIFIED BY 'x';\n", hq)
	}
	cria.WriteString("CREATE USER 'outra'@'%' IDENTIFIED BY 'x';\nCREATE DATABASE sentinela;\n")
	o, err := sql(cria.String())
	require.NoError(t, err, o)
	o, err = sql("", "-e", "SELECT COUNT(*) FROM mysql.global_priv WHERE User = 'casaos'")
	require.NoError(t, err, o)
	require.Equal(t, fmt.Sprint(len(hosts)), strings.TrimSpace(o))

	drops, err := sql("", "--raw", "-e", q(gera[1]))
	require.NoError(t, err, drops)
	o, err = sql(drops + "\n")
	require.NoError(t, err, "%s\n---\n%s", o, drops)
	resta, err := sql("", "-e", q(conta[1]))
	require.NoError(t, err, resta)
	assert.Equal(t, "0", strings.TrimSpace(resta), "todas as casaos saem:\n%s", drops)
	o, err = sql("", "-e", "SELECT CONCAT(User, '@', Host) FROM mysql.global_priv WHERE User = 'outra'; SHOW DATABASES LIKE 'sentinela'; SHOW DATABASES LIKE 'mysql'")
	require.NoError(t, err, o)
	assert.Equal(t, "outra@%\nsentinela\nmysql\n", o, "nada além das casaos é tocado")
}

// ---------------------------------------------------------------------------
// 5. --porta sem --dev-ip
// ---------------------------------------------------------------------------
func TestLote42_PortaSemDevIP(t *testing.T) {
	t.Run("recusa antes de mudar algo", func(t *testing.T) {
		e := novoEnsaio(t)
		r := e.rodar("--nova-ca", "--porta", "3307")
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "--porta so com --dev-ip")
		assert.NotContains(t, e.log(), "docker build")
		assert.NotContains(t, e.log(), "docker run")
	})
	t.Run("na transicao do --dev-ip segue aceita", func(t *testing.T) {
		e := novoEnsaio(t)
		e.instalarComPortaPublicada()
		r := e.rodar("--dev-ip", "192.168.168.105", "--porta", "3306")
		require.NoError(t, r.err, r.out)
	})
}

// ---------------------------------------------------------------------------
// 6. rotaperfumes-dba: TTY, --pull never, registro de reserva
// ---------------------------------------------------------------------------
func TestLote42_DBA_StdinTTYComSaidaRedirecionada(t *testing.T) {
	e := ensaioDBA(t)
	// O codigo de saida e o do cat (fim do pipe): vale a mensagem e o cliente nao chamado.
	r := e.rodarDBATTY("", " | cat")
	assert.Contains(t, r.out, "stdin no terminal com a saida redirecionada")
	assert.Empty(t, e.state("dba_argv"), "nada chega ao cliente")
	assert.NotContains(t, e.log(), "docker run")
}

func TestLote42_DBA_PullNever(t *testing.T) {
	e := ensaioDBA(t)
	r := e.rodarDBA("SELECT 1;\n")
	require.NoError(t, r.err, r.out)
	assert.Regexp(t, `(^|\s)--pull never(\s|$)`, e.state("dba_argv"))
	assert.Contains(t, funcaoShell(t, lerRaiz(t, "deploy", "restaurar-maquina.sh"), "cliente_importacao"),
		"docker run --rm -i --pull never --name", "o cliente da importação tem o mesmo limite")
}

func TestLote42_DBA_LoggerFalha(t *testing.T) {
	reserva := func(e *ensaio) string { return filepath.Join(e.dir, "varlog", "rotaperfumes-dba.log") }
	t.Run("grava a reserva 600 sem SQL e avisa", func(t *testing.T) {
		e := ensaioDBA(t)
		e.setState("logger_falha", "")
		r := e.rodarDBA("DROP TABLE segredo_do_sql;\n")
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "logger falhou; evento gravado em /var/log/rotaperfumes-dba.log")
		b, err := os.ReadFile(reserva(e))
		require.NoError(t, err)
		assert.Regexp(t, `(?m)^20\d\d-\d\d-\d\dT\S+ usuario=ivo sessao modo=stdin$`, string(b))
		assert.NotContains(t, string(b), "segredo_do_sql")
		assert.Equal(t, os.FileMode(0o600), modoDe(t, reserva(e)))

		// Segunda sessao: anexa, nao trunca.
		r = e.rodarDBA("SELECT 1;\n")
		require.NoError(t, r.err, r.out)
		b, err = os.ReadFile(reserva(e))
		require.NoError(t, err)
		assert.Equal(t, 2, strings.Count(string(b), "sessao modo=stdin"))
	})
	t.Run("reserva existente com modo aberto avisa sem chmod", func(t *testing.T) {
		e := ensaioDBA(t)
		e.setState("logger_falha", "")
		escrever(t, reserva(e), "antigo\n", 0o644)
		r := e.rodarDBA("SELECT 1;\n")
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "que precisa ser do root com modo 600 (chown root:root; chmod 600)")
		assert.Equal(t, os.FileMode(0o644), modoDe(t, reserva(e)), "sem chmod: ele seguiria um link")
		assert.Contains(t, e.arq("varlog/rotaperfumes-dba.log"), "antigo\n")
		assert.Contains(t, e.arq("varlog/rotaperfumes-dba.log"), "sessao modo=stdin")
	})
	t.Run("link simbolico na reserva nao e seguido", func(t *testing.T) {
		e := ensaioDBA(t)
		e.setState("logger_falha", "")
		alvo := filepath.Join(e.dir, "alvo.txt")
		escrever(t, alvo, "intacto\n", 0o644)
		require.NoError(t, os.Symlink(alvo, reserva(e)))
		r := e.rodarDBA("SELECT 1;\n")
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "logger falhou e /var/log/rotaperfumes-dba.log nao foi gravado")
		assert.Equal(t, "intacto\n", e.arq("alvo.txt"))
	})
	t.Run("logger ok nao grava a reserva", func(t *testing.T) {
		e := ensaioDBA(t)
		r := e.rodarDBA("SELECT 1;\n")
		require.NoError(t, r.err, r.out)
		assert.NoFileExists(t, reserva(e))
		assert.NotContains(t, r.out, "logger falhou")
	})
}

// ---------------------------------------------------------------------------
// 2 e 3. Túnel: conferência depois de subir; falha depois do token desfaz tudo
// ---------------------------------------------------------------------------
func (e *ensaioD09) ateTokenFile(extraEnv []string) (string, error) {
	e.t.Helper()
	s := e.tty(extraEnv, append(e.argsBase(), "--token-file", e.in("token"))...)
	e.confirmarInicio(s, true)
	s.digitar(promptMigrD09, "MIGRACOES CONFERIDAS")
	s.digitar(promptLoginD09, "LOGIN OK")
	s.digitar(promptPainelD09, "PAINEL CONFERIDO")
	s.digitar(promptFimD09, "JENKINS ATUALIZADO")
	return s.terminar()
}

func (e *ensaioD09) ateTokenFileFalha() (string, error) {
	e.t.Helper()
	s := e.tty(nil, append(e.argsBase(), "--token-file", e.in("token"))...)
	e.confirmarInicio(s, true)
	s.digitar(promptMigrD09, "MIGRACOES CONFERIDAS")
	s.digitar(promptLoginD09, "LOGIN OK")
	s.digitar(promptPainelD09, "PAINEL CONFERIDO")
	return s.terminar()
}

func TestLote42_Tunel_ConfereDepoisDeSubir(t *testing.T) {
	e := novoEnsaioD09(t)
	out, err := e.ateTokenFile(nil)
	require.NoError(t, err, out)
	assert.Contains(t, out, "Tunel OK: /ready=200; origem com o filtro da unit=200.")
	e.antes("systemctl start rotaperfumes-cloudflared.service", "curl -fsS --max-time 5 -o /dev/null -w %{http_code} http://127.0.0.1:20241/ready")
	e.antes("systemctl start rotaperfumes-cloudflared.service", "systemd-run ")
	log := e.ler("log")
	for _, p := range []string{"-p DynamicUser=yes", "-p IPAddressDeny=10.0.0.0/8 172.16.0.0/12 192.168.0.0/16 169.254.0.0/16",
		"-p IPAddressAllow=127.0.0.0/8 ::1", "-p RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX AF_NETLINK", "https://localhost:8443/api/health"} {
		assert.Contains(t, log, p)
	}
	assert.NotContains(t, log, "systemctl disable", "sucesso nao desfaz")
	assert.True(t, e.existe("fs/etc/rotaperfumes/cloudflared.env"))
	e.semSegredos(out)
}

func TestLote42_Tunel_PublicoSoAvisa(t *testing.T) {
	e := novoEnsaioD09(t)
	e.estado("publico_falha", "")
	out, err := e.ateTokenFile(nil)
	require.NoError(t, err, out)
	assert.Contains(t, out, "https://"+publicoD09+"/api/health respondeu erro daqui")
}

func TestLote42_Tunel_FalhaDepoisDoTokenDesfaz(t *testing.T) {
	for _, c := range []struct {
		nome, estado, trecho string
		espera               bool
	}{
		{"unit nao sobe", "unit_falha", "nao subiu", false},
		{"token no journal", "journal_vaza", "no journal", false},
		{"token no cmdline", "cmdline_vaza", "no cmdline", false},
		{"sem ready", "tunel_sem_ready", "o tunel nao ficou pronto (/ready=erro", true},
		{"origem barrada pelo filtro", "origem_barrada", "origem com o filtro da unit=erro", true},
	} {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioD09(t)
			e.estado(c.estado, "")
			if c.espera {
				// sleep curto: o stub cobre so a espera entre as 40 tentativas.
				escrever(t, filepath.Join(e.dir, "bin", "sleep"), "#!/bin/bash\nexit 0\n", 0o755)
			}
			out, err := e.ateTokenFileFalha()
			require.Error(t, err, out)
			assert.Contains(t, out, c.trecho)
			assert.Contains(t, e.ler("log"), "systemctl stop rotaperfumes-cloudflared.service")
			assert.Contains(t, e.ler("log"), "systemctl disable rotaperfumes-cloudflared.service")
			assert.False(t, e.existe("fs/etc/rotaperfumes/cloudflared.env"), "token apagado")
			assert.False(t, e.existe("fs/etc/rotaperfumes/cloudflared.env.tmp"))
			assert.Contains(t, out, "etapa 11 interrompida")
			assert.Contains(t, e.ler("fs/root/rotaperfumes-restauracao/estado"), "etapa=10\n", "a etapa 11 fica pendente")
			e.semSegredos(out)
		})
	}
	t.Run("token invalido sem nada gravado nem a desfazer", func(t *testing.T) {
		e := novoEnsaioD09(t)
		escrever(t, e.in("token"), "curto com espaco\n", 0o600)
		out, err := e.ateTokenFileFalha()
		require.Error(t, err, out)
		assert.NotContains(t, e.ler("log"), "systemctl disable")
		assert.NotContains(t, out, "etapa 11 interrompida")
	})
	t.Run("retomada refaz a etapa 11", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.estado("unit_falha", "")
		out, err := e.ateTokenFileFalha()
		require.Error(t, err, out)
		require.NoError(t, os.Remove(filepath.Join(e.dir, "state", "unit_falha")))
		s := e.tty(nil, append(e.argsBase(), "--token-file", e.in("token"))...)
		s.digitar(promptPainelD09, "PAINEL CONFERIDO")
		s.digitar(promptFimD09, "JENKINS ATUALIZADO")
		out, err = s.terminar()
		require.NoError(t, err, out)
		assert.Equal(t, "TUNNEL_TOKEN="+tokenD09+"\n", e.ler("fs/etc/rotaperfumes/cloudflared.env"))
	})
}

func TestLote42_Tunel_DNSDaLAN(t *testing.T) {
	t.Run("nameserver da LAN recusa antes de mudar algo", func(t *testing.T) {
		for _, ns := range []string{"192.168.168.1", "10.0.0.1", "172.20.0.1", "169.254.1.1"} {
			e := novoEnsaioD09(t)
			escrever(t, filepath.Join(e.dir, "fs/etc/resolv.conf"), "# x\nnameserver "+ns+"\n", 0o644)
			out := e.recusa("usa o nameserver "+ns+", da LAN", e.argsBase()...)
			assert.NotContains(t, out, "=== [1/12]")
		}
	})
	t.Run("resolved ou DNS publico seguem", func(t *testing.T) {
		for _, ns := range []string{"127.0.0.53", "1.1.1.1", "172.32.0.1", "::1"} {
			e := novoEnsaioD09(t)
			escrever(t, filepath.Join(e.dir, "fs/etc/resolv.conf"), "nameserver "+ns+"\n", 0o644)
			s := e.tty(nil, e.argsBase()...)
			s.esperar(promptHostD09)
		}
	})
	t.Run("com --sem-tunel nao importa", func(t *testing.T) {
		e := novoEnsaioD09(t)
		escrever(t, filepath.Join(e.dir, "fs/etc/resolv.conf"), "nameserver 192.168.168.1\n", 0o644)
		s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
		s.esperar(promptHostD09)
	})
}

// ---------------------------------------------------------------------------
// 7. restaurar-maquina.sh: avisos de shred, DOCKER_*, sinais de produção
// ---------------------------------------------------------------------------
func TestLote42_Restaurar_AvisoDeShredDeTodasAsEntradas(t *testing.T) {
	e := novoEnsaioD09(t)
	out, err := e.ateTokenFile(nil)
	require.NoError(t, err, out)
	for _, o := range []string{"--api-env (" + e.in("api.env") + ")", "--pepper-file (" + e.in("pepper") + ")", "--token-file (" + e.in("token") + ")"} {
		assert.Contains(t, out, "apague o "+o+" com shred -u")
	}

	e2 := novoEnsaioD09(t)
	out, err = e2.fluxoSemTunel()
	require.NoError(t, err, out)
	assert.Contains(t, out, "apague o --api-env (")
	assert.Contains(t, out, "apague o --pepper-file (")
	assert.NotContains(t, out, "apague o --token-file")
}

func TestLote42_Restaurar_SemDockerDoAmbiente(t *testing.T) {
	e := novoEnsaioD09(t)
	s := e.tty([]string{"DOCKER_HOST=tcp://203.0.113.9:2375", "DOCKER_CONTEXT=remoto", "DOCKER_CONFIG=/tmp/cfg-alheia"},
		append(e.argsBase(), "--sem-tunel")...)
	e.confirmarInicio(s, false)
	s.digitar(promptMigrD09, "MIGRACOES CONFERIDAS")
	s.digitar(promptLoginD09, "LOGIN OK")
	s.digitar(promptFimD09, "JENKINS ATUALIZADO")
	out, err := s.terminar()
	require.NoError(t, err, out)
	for _, arq := range []string{"state/compose_env", "state/import_env", "state/validador_env"} {
		env := e.ler(arq)
		require.NotEmpty(t, env, arq)
		assert.NotRegexp(t, `(?m)^DOCKER_(HOST|CONTEXT|CONFIG)=`, env, arq)
	}
}

func TestLote42_Restaurar_SinaisDeProducaoNovos(t *testing.T) {
	for _, c := range []struct{ rel, sinal string }{
		{"fs/root/rotaperfumes-mariadb/root.senha", "/root/rotaperfumes-mariadb/root.senha"},
		{"fs/root/rotaperfumes-ca/ca-key.pem", "/root/rotaperfumes-ca/ca-key.pem"},
	} {
		t.Run(c.sinal, func(t *testing.T) {
			e := novoEnsaioD09(t)
			require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(e.dir, c.rel)), 0o700))
			escrever(t, filepath.Join(e.dir, c.rel), "x\n", 0o600)
			out, err := e.tty(nil, e.argsBase()...).terminar()
			require.Error(t, err, out)
			assert.Contains(t, out, "parece um servidor em uso")
			assert.Contains(t, out, "Sinal de producao: "+c.sinal)
			assert.NotContains(t, out, "=== [1/12]")
			assert.False(t, e.existe("fs/root/rotaperfumes-restauracao"))
		})
	}
	t.Run("ca-key no caminho padrao recusa com a causa", func(t *testing.T) {
		_, key, _ := caFixture(t)
		for nome, caminho := range map[string]string{"direto": "/root/rotaperfumes-ca/ca-key.pem", "por link": "/root/link-ca.pem"} {
			e := novoEnsaioD09(t)
			require.NoError(t, os.MkdirAll(filepath.Join(e.dir, "fs/root/rotaperfumes-ca"), 0o700))
			escrever(t, filepath.Join(e.dir, "fs/root/rotaperfumes-ca/ca-key.pem"), key, 0o600)
			require.NoError(t, os.Symlink("/root/rotaperfumes-ca/ca-key.pem", filepath.Join(e.dir, "fs/root/link-ca.pem")))
			a := []string{}
			for _, v := range e.argsBase() {
				if v != "--nova-ca" {
					a = append(a, v)
				}
			}
			a = append(a, "--ca-cert", e.in("ca.pem"), "--ca-key", caminho)
			out, err := e.tty(nil, a...).terminar()
			require.Error(t, err, nome+": "+out)
			assert.Contains(t, out, "--ca-key dentro de /root/rotaperfumes-ca", nome)
			assert.Contains(t, out, "aponte --ca-key para la", nome)
			assert.NotContains(t, out, "BEGIN PRIVATE", nome)
		}
	})
	t.Run("retomada depois da etapa 3 nao e barrada pelos novos sinais", func(t *testing.T) {
		e := novoEnsaioD09(t)
		// O stub do mariadb-docker.sh passa a deixar os dois arquivos, como o real.
		e.clone("deploy/mariadb-docker.sh", stubMariadbD09+"mkdir -p /root/rotaperfumes-ca\ntouch /root/rotaperfumes-mariadb/root.senha /root/rotaperfumes-ca/ca-key.pem\n", 0o755)
		e.recommit()
		s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
		e.confirmarInicio(s, false)
		s.digitar(promptMigrD09, "errado")
		out, err := s.terminar()
		require.Error(t, err, out)
		require.True(t, e.existe("fs/root/rotaperfumes-mariadb/root.senha"), "o stub deixou o sinal")
		s = e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
		s.digitar(promptMigrD09, "MIGRACOES CONFERIDAS")
		s.digitar(promptLoginD09, "LOGIN OK")
		s.digitar(promptFimD09, "JENKINS ATUALIZADO")
		out, err = s.terminar()
		require.NoError(t, err, out)
		assert.Contains(t, out, "Retomando")
	})
}
