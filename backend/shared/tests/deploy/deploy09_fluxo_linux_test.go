//go:build linux

package deploy_test

// DEPLOY-09: ordem das etapas, banco com dados, retomada, segredos, chave do apt e
// unit. Tudo com stubs (deploy09_ensaio): prova o controle do script, não o Docker,
// o apt, o systemd nem o cloudflared reais.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDEPLOY09_Fluxo_Ordem(t *testing.T) {
	e := novoEnsaioD09(t)
	s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)

	// Verifica antes de confirmar: na hora do prompt, nada foi mudado além da cópia.
	s.esperar(promptHostD09)
	log := e.ler("log")
	assert.Contains(t, log, "verificar-backup.sh /root/rotaperfumes-restauracao/entrada/rotaperfumes-"+carimboD09+".sql.gz", "verifica a CÓPIA")
	assert.Contains(t, log, "backup-banco.sh --conferir-imagens /root/rotaperfumes-restauracao/entrada/")
	assert.NotContains(t, log, "mariadb-docker.sh")
	assert.NotContains(t, log, "rotaperfumes-dba-cliente")
	assert.False(t, e.existe("fs/root/rotaperfumes-restauracao/estado"), "o marcador nasce depois da etapa 2")
	saidaPrompt := s.saida()
	assert.Contains(t, saidaPrompt, "usuarios")
	assert.Contains(t, saidaPrompt, carimboD09)
	assert.Contains(t, saidaPrompt, ipD09)
	assert.Contains(t, saidaPrompt, e.git("rev-parse", "HEAD"))
	assert.Contains(t, saidaPrompt, "Reemissao", "IP diferente: lista dos commits")

	// A troca do original depois da cópia não afeta o importado.
	escrever(t, e.dump(), string(gzipD09(t, "OUTRO-CONTEUDO-TROCADO\n-- Dump completed\n")), 0o644)

	s.digitar(promptHostD09, hostD09)
	s.digitar(promptMigrD09, "MIGRACOES CONFERIDAS")
	assert.Contains(t, e.ler("state/importado"), dadoDumpD09, "importa a cópia")
	assert.NotContains(t, e.ler("state/importado"), "OUTRO-CONTEUDO-TROCADO")

	e.antes("verificar-backup.sh", "mariadb-docker.sh")
	e.antes("mariadb-docker.sh", "rotaperfumes-dba-cliente")
	assert.Contains(t, e.ler("log"), "rotaperfumes-dba\n", "TRUNCATE pelo comando do DBA")
	assert.Contains(t, e.ler("state/dba_stdin"), "TRUNCATE TABLE refresh_tokens;")
	assert.Contains(t, s.saida(), "sql/08_alter_x.sql")
	assert.NotContains(t, s.saida(), "18_revert_x", "só as *_alter_*")

	s.digitar(promptLoginD09, "LOGIN OK")
	s.digitar(promptFimD09, "JENKINS ATUALIZADO")
	out, err := s.terminar()
	require.NoError(t, err, out)

	// Ordem do resto.
	e.antes("rotaperfumes-dba-cliente", "--entrypoint bash mariadb")
	e.antes("--entrypoint bash mariadb", "compose -f")
	e.antes("build --pull", "up --no-start")
	e.antes("up --no-start", "-mindepth 1")
	e.antes("-mindepth 1", "tar -xf")
	e.antes("tar -xf", "up -d --remove-orphans")
	e.antes("up -d --remove-orphans", "curl ")
	e.antes("curl ", "backup-banco.sh --instalar")
	e.antes("backup-banco.sh --instalar", "rotaperfumes-backup-banco --status")

	// Argumentos.
	md := e.ler("log")
	assert.Contains(t, md, "mariadb-docker.sh --ip "+ipD09+" --host-nome "+hostD09+" --nova-ca")
	assert.NotContains(t, md, "--dev-ip")
	assert.NotContains(t, md, "--recriar")
	assert.Contains(t, s.saida(), "IGNORE o seed", "o seed sugerido fica de fora")
	assert.NotContains(t, md, "seed-producao")

	argv := e.ler("state/import_argv")
	for _, f := range []string{"--sandbox", "--local-infile=0", "--batch", "--ssl-verify-server-cert", "--network rotaperfumes-db", "--ip 172.30.42.22",
		"--read-only", "--cap-drop ALL", "--log-driver none", "source=/root/rotaperfumes-mariadb/dba-cliente/cliente.cnf"} {
		assert.Contains(t, argv, f)
	}
	assert.NotRegexp(t, `(?m)^MYSQL_PWD=`, e.ler("state/import_env"), "a senha fica só no option file")
	assert.NotContains(t, argv, "MYSQL_PWD")
	assert.Contains(t, e.ler("state/extracao_argv"), "--user 65532:65532")
	assert.Contains(t, e.ler("state/extracao_argv"), "--no-same-owner --no-overwrite-dir")
	assert.NotContains(t, md, "-delete", "sem find -delete")

	// O smoke passa por SITE_IP e por localhost, com Host qualquer.
	assert.Contains(t, md, "curl -fsSk --max-time 5 -o /dev/null -w %{http_code} https://"+ipD09+":8443/api/health")
	assert.Contains(t, md, "-H Host: smoke-tunnel.invalid https://"+ipD09+":8443/api/health")
	assert.Contains(t, md, "-H Host: smoke-tunnel.invalid https://localhost:8443/api/health")

	// --nova-ca: a lista do validador é só o fingerprint novo, sem o versionado.
	_, _, fp := caFixture(t)
	assert.Contains(t, e.ler("state/validador_env"), "CA_FINGERPRINTS="+fp)
	assert.NotContains(t, e.ler("state/validador_env"), "# teste")
	ambiente := e.ler("state/compose_env")
	for _, kv := range []string{"SITE_IP=" + ipD09, "SITE_HOST=" + hostD09, "API_ENV_FILE=/root/rotaperfumes-restauracao/api.env", "TURNSTILE_SITE_KEY=0x4AAAAAAFACjc-LtM9NWlvP"} {
		assert.Contains(t, ambiente, kv)
	}
	assert.Contains(t, ambiente, "DB_CA_SHA256=abab")

	// Fim: entrada/ e api.env apagados; estado final; sem túnel e registrado.
	assert.False(t, e.existe("fs/root/rotaperfumes-restauracao/api.env"))
	assert.False(t, e.existe("fs/root/rotaperfumes-restauracao/entrada"))
	assert.Contains(t, e.ler("fs/root/rotaperfumes-restauracao/estado"), "etapa=concluida")
	assert.Contains(t, e.ler("fs/root/rotaperfumes-restauracao/registro"), "--sem-tunel escolhido")
	assert.False(t, e.existe("fs/etc/rotaperfumes/cloudflared.env"))
	assert.NotContains(t, md, "apt-get")
	assert.Contains(t, out, "rotaperfumes-db-admin", "imprime a lista do Jenkins")
	e.semSegredos(out)
}

func TestDEPLOY09_Fluxo_ApiEnvEModos(t *testing.T) {
	e := novoEnsaioD09(t)
	s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
	s.digitar(promptHostD09, hostD09)
	s.esperar(promptMigrD09)
	dir := filepath.Join(e.dir, "fs/root/rotaperfumes-restauracao")
	for caminho, modo := range map[string]os.FileMode{
		dir: 0o700, filepath.Join(dir, "entrada"): 0o700, filepath.Join(dir, "api.env"): 0o600, filepath.Join(dir, "estado"): 0o600,
		filepath.Join(dir, "entrada", "rotaperfumes-"+carimboD09+".sql.gz"): 0o600,
	} {
		st, err := os.Stat(caminho)
		require.NoError(t, err)
		assert.Equal(t, modo, st.Mode().Perm(), caminho)
	}
	env := e.ler("fs/root/rotaperfumes-restauracao/api.env")
	assert.Contains(t, env, "\nDB_SENHA="+senhaNovaD09+"\n")
	assert.NotContains(t, env, senhaAntigaD09)
	assert.NotContains(t, env, jwtAntigoD09)
	assert.Regexp(t, `(?m)^JWT_SECRET=[A-Za-z0-9+/=]{60,}$`, env)
	assert.Contains(t, env, "PASSWORD_PEPPER="+pepperD09)
	assert.Contains(t, env, "SMTP_PASSWORD='a$b'", "sem reescrita do resto")
	assert.Contains(t, env, "CORS_ALLOWED_ORIGINS=https://"+publicoD09)
	s.digitar(promptMigrD09, "ERRADO")
	out, err := s.terminar()
	require.Error(t, err, out)
	assert.Contains(t, out, "confirmacao diferente")
	assert.NotContains(t, e.ler("log"), "compose -f", "sem confirmar, não segue")
	e.semSegredos(out)
}

func TestDEPLOY09_Fluxo_ConfirmacaoErradaNaoImporta(t *testing.T) {
	e := novoEnsaioD09(t)
	s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
	s.digitar(promptHostD09, "outra-maquina")
	out, err := s.terminar()
	require.Error(t, err, out)
	assert.NotContains(t, e.ler("log"), "mariadb-docker.sh")
	assert.NotContains(t, e.ler("log"), "rotaperfumes-dba-cliente")
	assert.False(t, e.existe("fs/root/rotaperfumes-restauracao/estado"))
}

func TestDEPLOY09_Fluxo_AntigaParadaObrigatoriaComTunel(t *testing.T) {
	e := novoEnsaioD09(t)
	s := e.tty(nil, e.argsBase()...)
	s.digitar(promptHostD09, hostD09)
	s.digitar(promptAntigaD09, "sim")
	out, err := s.terminar()
	require.Error(t, err, out)
	assert.NotContains(t, e.ler("log"), "mariadb-docker.sh")
}

func TestDEPLOY09_Fluxo_VerificacaoReprovada(t *testing.T) {
	for _, c := range []struct{ nome, estado string }{
		{"dump reprovado", "verificar_reprova"},
		{"código 0 sem APROVADO", "verificar_sem_aprovado"},
		{"imagens reprovadas", "imagens_reprova"},
	} {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioD09(t)
			e.estado(c.estado, "")
			out, err := e.tty(nil, append(e.argsBase(), "--sem-tunel")...).terminar()
			require.Error(t, err, out)
			assert.NotContains(t, out, promptHostD09, "nem pede a confirmação")
			assert.NotContains(t, e.ler("log"), "mariadb-docker.sh")
		})
	}
}

func TestDEPLOY09_Tabelas_QualquerUmaRecusa(t *testing.T) {
	e := novoEnsaioD09(t)
	e.estado("tabelas_pre", "qualquer 0\n")
	s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
	s.digitar(promptHostD09, hostD09)
	out, err := s.terminar()
	require.Error(t, err, out)
	assert.Contains(t, out, "ja tem 1 tabela(s)")
	assert.NotContains(t, e.ler("log"), "rotaperfumes-dba-cliente", "nada importado")
	assert.NotContains(t, out, promptDropD09, "fora de importacao-iniciada, não oferece DROP")
	assert.Empty(t, e.ler("state/dba_stdin"), "sem DROP")
}

func TestDEPLOY09_Contagem_Diferente(t *testing.T) {
	e := novoEnsaioD09(t)
	e.estado("tabelas_dump", "usuarios 2\nrefresh_tokens 5\n")
	s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
	s.digitar(promptHostD09, hostD09)
	out, err := s.terminar()
	require.Error(t, err, out)
	assert.Contains(t, out, "contagem diferente na tabela usuarios")
	assert.NotContains(t, e.ler("state/dba_stdin"), "TRUNCATE")
}

func TestDEPLOY09_Retomada(t *testing.T) {
	falhaNaImportacao := func(t *testing.T) *ensaioD09 {
		e := novoEnsaioD09(t)
		e.estado("import_falha", "")
		s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
		s.digitar(promptHostD09, hostD09)
		out, err := s.terminar()
		require.Error(t, err, out)
		assert.Contains(t, out, "ERROR 1062 (23000) at line 7", "código e linha")
		assert.NotContains(t, out, "Duplicate", "nunca o texto do servidor")
		assert.NotContains(t, out, dadoDumpD09)
		assert.Contains(t, e.ler("fs/root/rotaperfumes-restauracao/estado"), "importacao=iniciada")
		return e
	}

	t.Run("falha na importação oferece DROP e pula o concluído", func(t *testing.T) {
		e := falhaNaImportacao(t)
		antes := strings.Count(e.ler("log"), "verificar-backup.sh")
		s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
		s.esperar(promptDropD09)
		assert.NotContains(t, s.saida(), promptHostD09, "a confirmação não se repete")
		assert.Equal(t, antes, strings.Count(e.ler("log"), "verificar-backup.sh"), "a verificação concluída é pulada")
		assert.Equal(t, 1, strings.Count(e.ler("log"), "mariadb-docker.sh"), "o MariaDB também")
		s.digitar(promptDropD09, "rotaperfumes")
		s.digitar(promptMigrD09, "MIGRACOES CONFERIDAS")
		assert.Contains(t, e.ler("state/dba_stdin"), "DROP DATABASE `rotaperfumes`;")
		assert.Contains(t, e.ler("state/dba_stdin"), "CREATE DATABASE `rotaperfumes`")
		assert.Contains(t, e.ler("fs/root/rotaperfumes-restauracao/estado"), "importacao=concluida")
		s.digitar(promptLoginD09, "LOGIN OK")
		s.digitar(promptFimD09, "JENKINS ATUALIZADO")
		out, err := s.terminar()
		require.NoError(t, err, out)
	})
	t.Run("nome errado não apaga", func(t *testing.T) {
		e := falhaNaImportacao(t)
		s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
		s.digitar(promptDropD09, "outra-coisa")
		out, err := s.terminar()
		require.Error(t, err, out)
		assert.NotContains(t, e.ler("state/dba_stdin"), "DROP")
	})
	t.Run("com a API, não oferece DROP", func(t *testing.T) {
		e := falhaNaImportacao(t)
		e.estado("sinal_compose", "")
		out, err := e.tty(nil, append(e.argsBase(), "--sem-tunel")...).terminar()
		require.Error(t, err, out)
		assert.NotContains(t, out, promptDropD09)
		assert.Contains(t, out, "ha container do compose")
		assert.NotContains(t, e.ler("state/dba_stdin"), "DROP")
	})
	t.Run("machine-id diferente recusa", func(t *testing.T) {
		e := falhaNaImportacao(t)
		escrever(t, filepath.Join(e.dir, "fs/etc/machine-id"), "ffffffffffffffffffffffffffffffff\n", 0o644)
		out, err := e.tty(nil, append(e.argsBase(), "--sem-tunel")...).terminar()
		require.Error(t, err, out)
		assert.Contains(t, out, "machine-id diferente")
		assert.NotContains(t, out, promptDropD09)
	})
	t.Run("sha256 diferente recusa", func(t *testing.T) {
		e := falhaNaImportacao(t)
		escrever(t, e.dump(), string(gzipD09(t, "OUTRO\n-- Dump completed\n")), 0o644)
		out, err := e.tty(nil, append(e.argsBase(), "--sem-tunel")...).terminar()
		require.Error(t, err, out)
		assert.Contains(t, out, "sha256 diferente")
	})
	t.Run("sem-tunel diferente recusa", func(t *testing.T) {
		e := falhaNaImportacao(t)
		out, err := e.tty(nil, e.argsBase()...).terminar()
		require.Error(t, err, out)
		assert.Contains(t, out, "--sem-tunel ou --nova-ca diferente")
	})
	t.Run("estado adulterado recusa", func(t *testing.T) {
		e := falhaNaImportacao(t)
		arq := filepath.Join(e.dir, "fs/root/rotaperfumes-restauracao/estado")
		atual := e.ler("fs/root/rotaperfumes-restauracao/estado")
		for _, ruim := range []string{atual + "x=$(id)\n", atual + "desconhecida=1\n", "etapa=2\n"} {
			require.NoError(t, os.WriteFile(arq, []byte(ruim), 0o600))
			out, err := e.tty(nil, append(e.argsBase(), "--sem-tunel")...).terminar()
			require.Error(t, err, out)
			assert.Contains(t, out, "RECUSADO:")
		}
	})
	t.Run("sem estado, com o container, recusa como produção", func(t *testing.T) {
		e := falhaNaImportacao(t)
		require.NoError(t, os.Remove(filepath.Join(e.dir, "fs/root/rotaperfumes-restauracao/estado")))
		out, err := e.tty(nil, append(e.argsBase(), "--sem-tunel")...).terminar()
		require.Error(t, err, out)
		assert.Contains(t, out, "parece um servidor em uso", "o marcador é o único que libera")
	})
	t.Run("concluida recusa sempre", func(t *testing.T) {
		e := novoEnsaioD09(t)
		out, err := e.fluxoSemTunel()
		require.NoError(t, err, out)
		chamadas := strings.Count(e.ler("log"), "mariadb-docker.sh")
		out, err = e.tty(nil, append(e.argsBase(), "--sem-tunel")...).terminar()
		require.Error(t, err, out)
		assert.Contains(t, out, "ja foi restaurada")
		assert.Equal(t, chamadas, strings.Count(e.ler("log"), "mariadb-docker.sh"))
	})
	t.Run("depois da importação, banco diferente recusa", func(t *testing.T) {
		e := novoEnsaioD09(t)
		s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
		s.digitar(promptHostD09, hostD09)
		s.digitar(promptMigrD09, "PARE")
		_, err := s.terminar()
		require.Error(t, err)
		e.estado("tabelas", "usuarios 1\nrefresh_tokens 5\n")
		out, err := e.tty(nil, append(e.argsBase(), "--sem-tunel")...).terminar()
		require.Error(t, err, out)
		assert.Contains(t, out, "contagem diferente", "o estado real é reconferido")
	})
	t.Run("cópia em entrada/ alterada recusa", func(t *testing.T) {
		e := falhaNaImportacao(t)
		arq := filepath.Join(e.dir, "fs/root/rotaperfumes-restauracao/entrada/rotaperfumes-"+carimboD09+".sql.gz")
		require.NoError(t, os.WriteFile(arq, []byte("alterado"), 0o600))
		out, err := e.tty(nil, append(e.argsBase(), "--sem-tunel")...).terminar()
		require.Error(t, err, out)
		assert.Contains(t, out, "a copia do dump")
	})
}

func TestDEPLOY09_Volume(t *testing.T) {
	ate8 := func(t *testing.T, e *ensaioD09) string {
		s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
		s.digitar(promptHostD09, hostD09)
		s.digitar(promptMigrD09, "MIGRACOES CONFERIDAS")
		out, err := s.terminar()
		require.Error(t, err, out)
		assert.NotContains(t, e.ler("log"), "up -d", "para antes de subir")
		return out
	}
	t.Run("labels erradas", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.estado("volume_rotulo", "outro/volume\n")
		assert.Contains(t, ate8(t, e), "nao foi criado pelo compose")
		assert.NotContains(t, e.ler("log"), "tar -xf")
	})
	t.Run("dono errado: a hipótese do up --no-start falhou", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.estado("volume_dono", "0:0\n")
		assert.Contains(t, ate8(t, e), "a copia da imagem nao aconteceu")
		assert.NotContains(t, e.ler("log"), "tar -xf")
	})
	t.Run("com conteúdo", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.estado("volume_itens", "3\n")
		assert.Contains(t, ate8(t, e), "tem conteudo")
		assert.NotContains(t, e.ler("log"), "tar -xf")
	})
	t.Run("contagem diferente depois de extrair", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.estado("volume_arquivos", "1\n")
		assert.Contains(t, ate8(t, e), "contagem diferente no volume")
	})
}

func TestDEPLOY09_Smoke_Falha(t *testing.T) {
	e := novoEnsaioD09(t)
	e.estado("curl_falha", "")
	// sleep curto: o stub cobre só a espera entre tentativas.
	escrever(t, filepath.Join(e.dir, "bin", "sleep"), "#!/bin/bash\nexit 0\n", 0o755)
	s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
	s.digitar(promptHostD09, hostD09)
	s.digitar(promptMigrD09, "MIGRACOES CONFERIDAS")
	out, err := s.terminar()
	require.Error(t, err, out)
	assert.Contains(t, out, "o smoke falhou")
	assert.NotContains(t, out, promptLoginD09)
	assert.NotContains(t, e.ler("log"), "backup-banco.sh --instalar")
}

// CA reaproveitada: instala o ca.pem, repassa a chave e usa o arquivo versionado.
func TestDEPLOY09_Fluxo_CaCert(t *testing.T) {
	e := novoEnsaioD09(t)
	a := []string{}
	for _, v := range e.argsBase() {
		if v != "--nova-ca" {
			a = append(a, v)
		}
	}
	a = append(a, "--ca-cert", e.in("ca.pem"), "--ca-key", e.in("ca-key.pem"), "--sem-tunel", "--backup-dir", "/mnt/ssd/bk", "--exigir-montagem", "/mnt/ssd", "--tmpfs-mb", "4096")
	s := e.tty(nil, a...)
	s.digitar(promptHostD09, hostD09)
	s.digitar(promptMigrD09, "MIGRACOES CONFERIDAS")
	s.digitar(promptLoginD09, "LOGIN OK")
	s.digitar(promptFimD09, "JENKINS ATUALIZADO")
	out, err := s.terminar()
	require.NoError(t, err, out)
	log := e.ler("log")
	assert.Contains(t, log, "mariadb-docker.sh --ip "+ipD09+" --host-nome "+hostD09+" --ca-key "+e.in("ca-key.pem"))
	assert.Contains(t, log, "verificar-backup.sh --tmpfs-mb 4096 ")
	assert.Contains(t, log, "backup-banco.sh --instalar --backup-dir /mnt/ssd/bk --exigir-montagem /mnt/ssd")
	cert, _, _ := caFixture(t)
	assert.Equal(t, cert, e.ler("fs/etc/mysql/ssl/ca.pem"))
	assert.Contains(t, e.ler("state/validador_env"), "# teste", "a lista é o arquivo versionado")
	assert.NotContains(t, out, "BEGIN PRIVATE", "a chave da CA nunca aparece")
}

// Com túnel: pepper e token digitados, sem eco.
func TestDEPLOY09_Tunel_Completo(t *testing.T) {
	e := novoEnsaioD09(t)
	a := []string{}
	for _, v := range e.argsBase() {
		if v != "--pepper-file" && v != e.in("pepper") {
			a = append(a, v)
		}
	}
	s := e.tty(nil, a...)
	s.digitar("PASSWORD_PEPPER (nao aparece na tela): ", pepperD09)
	s.digitar("Confirme o PASSWORD_PEPPER: ", pepperD09)
	e.confirmarInicio(s, true)
	s.digitar(promptMigrD09, "MIGRACOES CONFERIDAS")
	s.digitar(promptLoginD09, "LOGIN OK")
	s.esperar(promptPainelD09)
	// Antes do painel, o token ainda não existe em disco nem no terminal.
	assert.False(t, e.existe("fs/etc/rotaperfumes/cloudflared.env"))
	log := e.ler("log")
	assert.Contains(t, log, "curl ", "o token só vem depois do site provado")
	saida := s.saida()
	for _, trecho := range []string{publicoD09, "https://localhost:8443", "noTLSVerify", "Rotate token", "/connections"} {
		assert.Contains(t, saida, trecho)
	}
	s.digitar(promptPainelD09, "PAINEL CONFERIDO")
	s.digitar(promptTokenD09, tokenD09)
	s.digitar(promptFimD09, "JENKINS ATUALIZADO")
	out, err := s.terminar()
	require.NoError(t, err, out)

	env := filepath.Join(e.dir, "fs/etc/rotaperfumes/cloudflared.env")
	st, err := os.Stat(env)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	assert.Equal(t, "TUNNEL_TOKEN="+tokenD09+"\n", e.ler("fs/etc/rotaperfumes/cloudflared.env"))
	unit := e.ler("fs/etc/systemd/system/rotaperfumes-cloudflared.service")
	assert.Contains(t, unit, "EnvironmentFile=/etc/rotaperfumes/cloudflared.env")
	assert.Contains(t, unit, "tunnel --no-autoupdate --metrics 127.0.0.1:20241 run")
	assert.NotContains(t, unit, tokenD09)
	assert.NotContains(t, unit, "--token")
	assert.Contains(t, e.ler("log"), "systemctl enable rotaperfumes-cloudflared.service")
	e.antes("systemctl enable rotaperfumes-cloudflared.service", "systemctl start rotaperfumes-cloudflared.service")
	assert.Contains(t, out, "Token fora do journal")

	lista := e.ler("fs/etc/apt/sources.list.d/rotaperfumes-cloudflared.list")
	assert.Equal(t, "deb [signed-by=/usr/share/keyrings/rotaperfumes-cloudflared.gpg] https://pkg.cloudflare.com/cloudflared any main\n", lista)
	pin := e.ler("fs/etc/apt/preferences.d/rotaperfumes-cloudflared")
	assert.Contains(t, pin, "Package: *\nPin: origin pkg.cloudflare.com\nPin-Priority: -1")
	assert.Contains(t, pin, "Package: cloudflared\nPin: origin pkg.cloudflare.com\nPin-Priority: 500")
	assert.True(t, e.existe("fs/keyrings/rotaperfumes-cloudflared.gpg"))
	assert.Contains(t, e.ler("log"), "apt-get install -y --no-install-recommends cloudflared")

	assert.Contains(t, e.ler("fs/root/rotaperfumes-restauracao/registro"), "ANTIGA PARADA declarada")
	e.semSegredos(out)
	// O unit, a lista e o pin ficam fora de qualquer vazamento: o token está só no env.
	assert.NotContains(t, e.ler("fs/etc/apt/sources.list.d/rotaperfumes-cloudflared.list"), tokenD09)
}

func TestDEPLOY09_Tunel_TokenFileEVazamentos(t *testing.T) {
	ateFim := func(t *testing.T, e *ensaioD09) (string, error) {
		s := e.tty(nil, append(e.argsBase(), "--token-file", e.in("token"))...)
		e.confirmarInicio(s, true)
		s.digitar(promptMigrD09, "MIGRACOES CONFERIDAS")
		s.digitar(promptLoginD09, "LOGIN OK")
		s.digitar(promptPainelD09, "PAINEL CONFERIDO")
		return s.terminar()
	}
	for _, c := range []struct{ nome, estado, trecho string }{
		{"no journal", "journal_vaza", "no journal"},
		{"no systemctl show", "show_vaza", "em systemctl show"},
		{"no cmdline", "cmdline_vaza", "no cmdline"},
	} {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioD09(t)
			e.estado(c.estado, "")
			out, err := ateFim(t, e)
			require.Error(t, err, out)
			assert.Contains(t, out, c.trecho)
			assert.Contains(t, out, "rotacione-o no painel")
			e.semSegredos(out) // nem o token, nem a linha do journal.
			assert.NotContains(t, out, "log: ", "só a contagem, sem mostrar a linha")
		})
	}
	t.Run("token inválido não grava", func(t *testing.T) {
		e := novoEnsaioD09(t)
		escrever(t, e.in("token"), "curto com espaco\n", 0o600)
		out, err := ateFim(t, e)
		require.Error(t, err, out)
		assert.Contains(t, out, "o token nao tem o formato esperado")
		assert.False(t, e.existe("fs/etc/rotaperfumes/cloudflared.env"))
		assert.NotContains(t, out, "curto com espaco")
	})
	t.Run("unit que não sobe", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.estado("unit_falha", "")
		out, err := ateFim(t, e)
		require.Error(t, err, out)
		assert.Contains(t, out, "nao subiu")
		e.semSegredos(out)
	})
}

func TestDEPLOY09_ChaveApt(t *testing.T) {
	semGravar := func(t *testing.T, e *ensaioD09) {
		t.Helper()
		assert.False(t, e.existe("fs/keyrings/rotaperfumes-cloudflared.gpg"), "a errada não grava nada")
		assert.False(t, e.existe("fs/etc/apt/sources.list.d/rotaperfumes-cloudflared.list"))
		assert.False(t, e.existe("fs/etc/apt/preferences.d/rotaperfumes-cloudflared"))
	}
	t.Run("chave errada recusa antes de tudo", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.clone("deploy/cloudflare-apt.asc", chaveErradaD09, 0o644)
		e.recommit()
		e.recusa("difere de deploy/cloudflare-apt.fingerprint", e.argsBase()...)
		semGravar(t, e)
	})
	t.Run("fingerprint errado", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.clone("deploy/cloudflare-apt.fingerprint", "93946AEDD37213FE9B65DE10DB888270E83108BF\n", 0o644)
		e.recommit()
		e.recusa("difere de deploy/cloudflare-apt.fingerprint", e.argsBase()...)
		semGravar(t, e)
	})
	t.Run("duas chaves no arquivo", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.clone("deploy/cloudflare-apt.asc", lerRaiz(t, "deploy", "cloudflare-apt.asc")+chaveErradaD09, 0o644)
		e.recommit()
		e.recusa("exatamente uma chave", e.argsBase()...)
		semGravar(t, e)
	})
	t.Run("sem apt-get", func(t *testing.T) {
		e := novoEnsaioD09(t)
		require.NoError(t, os.Remove(filepath.Join(e.dir, "bin", "apt-get")))
		e.estado("sem_apt", "") // o runner tira o apt-get real do PATH
		e.recusa("sem apt-get", e.argsBase()...)
	})
	t.Run("com a chave certa, o keyring é a chave do repositório", func(t *testing.T) {
		e := novoEnsaioD09(t)
		s := e.tty(nil, append(e.argsBase(), "--token-file", e.in("token"))...)
		e.confirmarInicio(s, true)
		s.digitar(promptMigrD09, "MIGRACOES CONFERIDAS")
		s.digitar(promptLoginD09, "LOGIN OK")
		s.esperar(promptPainelD09)
		home := t.TempDir()
		out, err := exec.Command("gpg", "--homedir", home, "--batch", "--no-autostart", "--with-colons", "--show-keys",
			filepath.Join(e.dir, "fs/keyrings/rotaperfumes-cloudflared.gpg")).CombinedOutput()
		require.NoError(t, err, string(out))
		assert.Contains(t, string(out), "CC94B39C77AE7342A68B89628A682D308D4E5E73")
		st, err := os.Stat(filepath.Join(e.dir, "fs/keyrings/rotaperfumes-cloudflared.gpg"))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o644), st.Mode().Perm())
	})
}
