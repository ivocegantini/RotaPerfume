package deploy_test

// DEPLOY-10: conta rotaperfumes-admin (DBA, só local) e migração da casaos, com
// docker/iptables/systemctl falsos (mesmo ensaio do TST-15).

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	contaDBA     = "'rotaperfumes-admin'@'172.30.42.22'"
	dbaSenhaRel  = segredos + "/dba-cliente/conta.senha"
	dbaCnfRel    = segredos + "/dba-cliente/cliente.cnf"
	dbaComando   = "sbin/rotaperfumes-dba"
	avisoNomes   = "rotaperfumes-admin = DBA, .22; nao e rotaperfumes_admin = Jenkins e seed, .20"
	contaLegadaA = "casaos\t%\ncasaos\t192.168.168.50\ncasaos\t192.168.168.0/255.255.255.0\ncasaos\tlocalhost\ncasaos\t172.30.42.22\ncasaos\t\n"
)

func modoDe(t *testing.T, caminho string) os.FileMode {
	t.Helper()
	st, err := os.Stat(caminho)
	require.NoError(t, err)
	return st.Mode().Perm()
}

// O SQL do DBA é exato: quatro comandos, todos com a conta entre aspas, e nenhum
// privilégio além de ALL no banco rotaperfumes.
func TestDEPLOY10_Ensaio_SQLExatoSemPrivilegioGlobal(t *testing.T) {
	e := novoEnsaio(t)
	r := e.rodar("--nova-ca")
	require.NoError(t, r.err, r.out)

	senha := strings.TrimSpace(e.arq(dbaSenhaRel))
	require.Regexp(t, `^[A-Za-z0-9]{32}$`, senha)
	var doDBA []string
	for _, l := range linhas(e.state("sql")) {
		if strings.Contains(l, "rotaperfumes-admin") && !strings.HasPrefix(l, "--") {
			doDBA = append(doDBA, l)
		}
	}
	assert.Equal(t, []string{
		"CREATE USER IF NOT EXISTS " + contaDBA + " IDENTIFIED VIA ed25519 USING PASSWORD('" + senha + "') REQUIRE SSL;",
		"ALTER USER " + contaDBA + " IDENTIFIED VIA ed25519 USING PASSWORD('" + senha + "') REQUIRE SSL;",
		"REVOKE ALL PRIVILEGES, GRANT OPTION FROM " + contaDBA + ";",
		"GRANT ALL PRIVILEGES ON `rotaperfumes`.* TO " + contaDBA + ";",
	}, doDBA)
	sql := e.state("sql")
	assert.NotContains(t, sql, "ON *.*", "sem privilégio global")
	assert.NotContains(t, sql, "WITH GRANT OPTION")
	assert.NotContains(t, sql, "PROCESS")
	assert.NotContains(t, sql, "ON `mysql`")
	assert.NotRegexp(t, `(^|\s)-p\s`, e.state("run_args"), "nenhuma porta publicada")
	assert.Contains(t, r.out, "DBA 172.30.42.22")
	assert.Contains(t, r.out, avisoNomes, "resumo separa as duas contas")
}

func TestDEPLOY10_Ensaio_ArquivosEReexecucao(t *testing.T) {
	e := novoEnsaio(t)
	r := e.rodar("--nova-ca")
	require.NoError(t, r.err, r.out)

	senha := strings.TrimSpace(e.arq(dbaSenhaRel))
	cnf := "[client]\nuser=rotaperfumes-admin\npassword=" + senha + "\n"
	assert.Equal(t, cnf, e.arq(dbaCnfRel))
	assert.Equal(t, os.FileMode(0o700), modoDe(t, filepath.Join(e.dir, segredos, "dba-cliente")))
	assert.Equal(t, os.FileMode(0o600), modoDe(t, filepath.Join(e.dir, dbaSenhaRel)))
	assert.Equal(t, os.FileMode(0o400), modoDe(t, filepath.Join(e.dir, dbaCnfRel)))
	assert.Equal(t, os.FileMode(0o700), modoDe(t, filepath.Join(e.dir, dbaComando)))
	assert.False(t, e.existe(dbaCnfRel+".tmp"), "temporário removido")

	// Nada no container do banco; nomes neutros, sem rotaperfumes-admin.senha.
	assert.False(t, e.existe(filepath.Join(segredos, "container", "cliente.cnf")))
	assert.False(t, e.existe(filepath.Join(segredos, "container", "conta.senha")))
	assert.NotContains(t, e.state("run_args"), "dba-cliente")
	achados, err := filepath.Glob(filepath.Join(e.dir, segredos, "rotaperfumes-admin*"))
	require.NoError(t, err)
	assert.Empty(t, achados)

	assert.NotContains(t, r.out, senha, "senha nunca na tela")
	assert.NotContains(t, e.log(), senha, "senha nunca no argv")

	// Reexecução: mesma senha nos arquivos e no ALTER USER.
	inicio := len(e.state("sql"))
	r = e.rodar()
	require.NoError(t, r.err, r.out)
	assert.Equal(t, senha, strings.TrimSpace(e.arq(dbaSenhaRel)))
	assert.Equal(t, cnf, e.arq(dbaCnfRel))
	assert.Equal(t, os.FileMode(0o400), modoDe(t, filepath.Join(e.dir, dbaCnfRel)))
	assert.Contains(t, e.state("sql")[inicio:], "ALTER USER "+contaDBA+" IDENTIFIED VIA ed25519 USING PASSWORD('"+senha+"') REQUIRE SSL;")
}

func TestDEPLOY10_Ensaio_RemoveCasaosEmQualquerHost(t *testing.T) {
	for nome, transicao := range map[string]bool{"sem --dev-ip": false, "transicao do --dev-ip": true} {
		t.Run(nome, func(t *testing.T) {
			e := novoEnsaio(t)
			args := []string{"--nova-ca"}
			if transicao {
				e.instalarComPortaPublicada()
				args = []string{"--dev-ip", "192.168.168.105"}
			}
			e.setState("contas", contaLegadaA+
				"rotaperfumes-admin\t%\nrotaperfumes-admin\t172.30.42.22\n"+
				"rotaperfumes_admin\t172.30.42.20\nrotaperfumes_admin\t172.30.42.22\n")
			r := e.rodar(args...)
			require.NoError(t, r.err, r.out)
			sql := e.state("sql")
			for _, u := range []string{"'casaos'@'%'", "'casaos'@'192.168.168.50'", "'casaos'@'192.168.168.0/255.255.255.0'",
				"'casaos'@'localhost'", "'casaos'@'172.30.42.22'", "'casaos'@''",
				"'rotaperfumes-admin'@'%'", "'rotaperfumes_admin'@'172.30.42.22'"} {
				assert.Contains(t, sql, "DROP USER IF EXISTS "+u+";")
			}
			assert.NotContains(t, sql, "DROP USER IF EXISTS "+contaDBA+";")
			assert.NotContains(t, sql, "DROP USER IF EXISTS 'rotaperfumes_admin'@'172.30.42.20';")
			assert.NotContains(t, sql, "CREATE USER IF NOT EXISTS 'casaos'")
			assert.NotContains(t, sql, "GRANT ALL PRIVILEGES ON `rotaperfumes`.* TO 'casaos'")
		})
	}
}

// DROP USER não derruba a sessão: o script só avisa e não mata ninguém.
func TestDEPLOY10_Ensaio_SessaoAntigaSoAvisa(t *testing.T) {
	e := novoEnsaio(t)
	e.setState("contas", "casaos\t192.168.168.50\n")
	e.setState("sessoes_legado", "2\n")
	r := e.rodar("--nova-ca")
	require.NoError(t, r.err, r.out)
	assert.Contains(t, r.out, "ha sessao aberta da conta casaos, removida (2)")
	assert.NotContains(t, strings.ToUpper(e.state("sql")), "KILL ")
	assert.Less(t, strings.Index(r.out, "Removendo conta fora do desejado: casaos@192.168.168.50"),
		strings.Index(r.out, "ha sessao aberta da conta casaos"), "o aviso vem depois do DROP")

	e2 := novoEnsaio(t)
	r = e2.rodar("--nova-ca")
	require.NoError(t, r.err, r.out)
	assert.NotContains(t, r.out, "ha sessao aberta da conta casaos")
}

func TestDEPLOY10_Ensaio_CasaosSenha(t *testing.T) {
	rel := filepath.Join(segredos, "casaos.senha")
	t.Run("nunca e criado", func(t *testing.T) {
		e := novoEnsaio(t)
		e.instalarComPortaPublicada()
		r := e.rodar("--dev-ip", "192.168.168.105")
		require.NoError(t, r.err, r.out)
		assert.False(t, e.existe(rel))
		assert.NotContains(t, r.out, "casaos.senha")
	})
	t.Run("existente fica intacto", func(t *testing.T) {
		e := novoEnsaio(t)
		e.instalarComPortaPublicada()
		escrever(t, filepath.Join(e.dir, rel), "NaoEFormatoDeSenha!", 0o600)
		r := e.rodar("--dev-ip", "192.168.168.105")
		require.NoError(t, r.err, "um arquivo fora do formato prova que ele nao e lido: %s", r.out)
		assert.Equal(t, "NaoEFormatoDeSenha!", e.arq(rel))
		assert.Equal(t, os.FileMode(0o600), modoDe(t, filepath.Join(e.dir, rel)))
		assert.Contains(t, r.out, "casaos.senha mantido; leve ao pendrive seguro.")
	})
	t.Run("estatico: so existe teste e aviso", func(t *testing.T) {
		var usos []string
		for _, l := range strings.Split(lerRaiz(t, "deploy", "mariadb-docker.sh"), "\n") {
			if strings.Contains(l, "$DB_LEGADO.senha") {
				usos = append(usos, strings.TrimSpace(l))
			}
		}
		assert.Equal(t, []string{
			`if [[ -e "$SEGREDOS_DIR/$DB_LEGADO.senha" ]]; then`,
			`info "$SEGREDOS_DIR/$DB_LEGADO.senha mantido; leve ao pendrive seguro."`,
		}, usos)
	})
}

// Fase 1 do --dev-ip: só com container que já publica a porta.
func TestDEPLOY10_Ensaio_DevIPSoNaTransicao(t *testing.T) {
	semMudanca := func(t *testing.T, e *ensaio) {
		t.Helper()
		log := e.log()
		for _, s := range []string{"docker run", "docker rm", "docker build", "docker network create", "docker exec", "iptables-restore", "systemctl"} {
			assert.NotContains(t, log, s)
		}
		assert.Empty(t, e.state("sql"), "nenhum SQL")
	}
	t.Run("instalacao nova recusa", func(t *testing.T) {
		e := novoEnsaio(t)
		r := e.rodar("--nova-ca", "--dev-ip", "192.168.168.105")
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "--dev-ip recusado em instalacao nova (DEPLOY-10)")
		semMudanca(t, e)
		assert.False(t, e.existe(segredos), "nada gravado")
	})
	t.Run("--recriar recusa", func(t *testing.T) {
		e := novoEnsaio(t)
		e.instalarComPortaPublicada()
		r := e.rodar("--dev-ip", "192.168.168.105", "--recriar")
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "--dev-ip nao combina com --recriar (DEPLOY-10)")
		semMudanca(t, e)
	})
	t.Run("container sem porta recusa", func(t *testing.T) {
		e := novoEnsaio(t)
		require.NoError(t, e.rodar("--nova-ca").err)
		_ = os.Remove(filepath.Join(e.dir, "state", "sql"))
		r := e.rodar("--dev-ip", "192.168.168.105")
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "--dev-ip recusado (DEPLOY-10): o container rotaperfumes-mariadb nao publica porta")
		semMudanca(t, e)
	})
	t.Run("migra sem reiniciar e sempre avisa", func(t *testing.T) {
		e := novoEnsaio(t)
		e.instalarComPortaPublicada()
		e.setState("contas", "casaos\t192.168.168.105\n")
		r := e.rodar("--dev-ip", "192.168.168.105")
		require.NoError(t, r.err, r.out)
		assert.Contains(t, r.out, "--dev-ip e so transicao (DEPLOY-10)")
		assert.NotContains(t, e.log(), "docker run -d ")
		assert.NotContains(t, e.log(), "docker rm ")
		sql := e.state("sql")
		assert.Contains(t, sql, "CREATE USER IF NOT EXISTS 'rotaperfumes_backup'@'172.30.42.21'")
		assert.Contains(t, sql, "CREATE USER IF NOT EXISTS "+contaDBA)
		assert.Contains(t, sql, "DROP USER IF EXISTS 'casaos'@'192.168.168.105';")
		assert.True(t, e.existe(dbaComando), "instala o comando")
		assert.Contains(t, e.state("chain_ROTAPERFUMES-DB"), "-s 192.168.168.105/32 -j RETURN", "a porta continua restrita")
	})
	t.Run("porta divergente pede --recriar sem --dev-ip", func(t *testing.T) {
		e := novoEnsaio(t)
		e.instalarComPortaPublicada()
		e.setState("portas", `{"3306/tcp":[{"HostIp":"0.0.0.0","HostPort":"3306"}]}`+"\n")
		r := e.rodar("--dev-ip", "192.168.168.105")
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "rode de novo com --recriar, sem --dev-ip")
		semMudanca(t, e)
	})
	t.Run("--recriar sem --dev-ip fecha a porta", func(t *testing.T) {
		e := novoEnsaio(t)
		e.instalarComPortaPublicada()
		r := e.rodar("--recriar")
		require.NoError(t, r.err, r.out)
		assert.NotRegexp(t, `(^|\s)-p\s`, e.state("run_args"))
		assert.Equal(t, "{}\n", e.state("portas"))
	})
}

// Etapa 8: o teste passa pelo comando pronto, e o resultado importa.
func TestDEPLOY10_Ensaio_TesteDaEtapa8(t *testing.T) {
	t.Run("sucesso", func(t *testing.T) {
		e := novoEnsaio(t)
		r := e.rodar("--nova-ca")
		require.NoError(t, r.err, r.out)
		assert.Contains(t, e.state("dba_stdin"), "SELECT CURRENT_USER();")
		assert.Contains(t, r.out, "Comando do DBA conectado como rotaperfumes-admin@172.30.42.22: OK.")
		assert.Contains(t, r.out, "Conexao sem TLS do DBA recusada: OK.")
		assert.Contains(t, e.state("dba_argv"), "--skip-ssl", "a recusa foi mesmo testada")
	})
	casos := []struct{ nome, estado, valor, msg string }{
		{"outra conta", "dba_usuario", "rotaperfumes_admin@172.30.42.20\n", "o comando do DBA nao entrou como rotaperfumes-admin@172.30.42.22"},
		{"conexao falha", "dba_falha", "", "falhou a conexao do DBA pelo comando pronto"},
		{"sem TLS aceito", "aceita_sem_tls_dba", "", "conexao SEM TLS do DBA foi aceita"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaio(t)
			e.setState(c.estado, c.valor)
			r := e.rodar("--nova-ca")
			require.Error(t, r.err, r.out)
			assert.Contains(t, r.out, c.msg)
			assert.NotContains(t, r.out, "\nPronto. MariaDB")
		})
	}
	t.Run("sessao do DBA aberta recusa antes de mudar", func(t *testing.T) {
		e := novoEnsaio(t)
		e.setState("dba_ocupado", "")
		r := e.rodar("--nova-ca")
		require.Error(t, r.err, r.out)
		assert.Contains(t, r.out, "ha uma sessao do DBA aberta")
		assert.NotContains(t, e.log(), "docker rm")
		assert.NotContains(t, e.log(), "docker run")
		assert.False(t, e.existe(segredos))
	})
	t.Run("outro --container testa sem instalar", func(t *testing.T) {
		e := novoEnsaio(t)
		r := e.rodar("--nova-ca", "--container", "teste-db", "--rede-nome", "teste-rede", "--rede-docker", "172.30.99.0/24")
		require.NoError(t, r.err, r.out)
		assert.False(t, e.existe(dbaComando), "o caminho é único no host")
		assert.Contains(t, r.out, "Comando do DBA conectado como rotaperfumes-admin@172.30.99.22: OK.")
		assert.Contains(t, e.state("dba_argv"), "--network teste-rede --ip 172.30.99.22")
		assert.False(t, e.existe("opt/rotaperfumes/mariadb-imagem/dba.tmp"), "temporário removido")
	})
}

// Os dois nomes parecidos não se misturam fora do mariadb-docker.sh.
func TestDEPLOY10_NomeDoDBAForaDosArquivosDoJenkinsESeed(t *testing.T) {
	for _, partes := range [][]string{{"Jenkinsfile"}, {"deploy", "seed-producao.sh"}, {"deploy", "setup-servidor.sh"}, {"deploy", "api.env.example"}} {
		assert.NotContains(t, lerRaiz(t, partes...), "rotaperfumes-admin", filepath.Join(partes...))
	}
	assert.NotContains(t, lerForaDoTarDoCI(t, ".env.example"), "rotaperfumes-admin")
	assert.NotContains(t, lerRaiz(t, "deploy", "backup-banco.sh"), "rotaperfumes-admin")
}

// A conta e o CasaOS de dev não mudam: o script de produção não trata do :3307.
func TestDEPLOY10_ScriptNaoCitaODev(t *testing.T) {
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")
	assert.NotContains(t, md, ":3307")
	assert.NotContains(t, md, "DB_MAKE", "conta de make db-* saiu")
	assert.NotContains(t, md, ".env.producao")
	assert.NotContains(t, md, "ca-producao.pem")
	assert.NotRegexp(t, `CREATE USER[^\n]*\$DB_LEGADO`, md, "casaos é só para remoção")
	assert.NotRegexp(t, `GRANT[^\n]*\$DB_LEGADO`, md)
	assert.Contains(t, lerRaiz(t, "Makefile"), "3307", "as guardas do CasaOS continuam no Makefile")
}

// Documentação: quem usa cada conta, e nenhuma sugestão de apagar o casaos.senha.
func TestDEPLOY10_Docs(t *testing.T) {
	ds := lerForaDoTarDoCI(t, "docs", "deploy-servidor.md")
	ssl := lerForaDoTarDoCI(t, "docs", "Instal_SSL.md")
	for nome, texto := range map[string]string{"deploy-servidor": ds, "Instal_SSL": ssl} {
		assert.Regexp(t, `(?i)\| *conta *\|.*usado por`, texto, nome)
		assert.Contains(t, texto, "`rotaperfumes-db-admin`", nome)
		assert.Contains(t, texto, "`rotaperfumes-admin`", nome)
		assert.Contains(t, texto, "`172.30.42.22`", nome)
		assert.NotRegexp(t, regexp.MustCompile(`casaos\.senha[^\n]{0,200}shred|shred[^\n]{0,200}casaos\.senha`), texto, nome)
	}
	assert.Contains(t, ds, "sudo bash deploy/mariadb-docker.sh --dev-ip 192.168.168.105")
	assert.Contains(t, ds, "echo 'SELECT CURRENT_USER()' | sudo rotaperfumes-dba")
}
