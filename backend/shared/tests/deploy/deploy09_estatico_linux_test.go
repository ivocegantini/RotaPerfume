//go:build linux

package deploy_test

// DEPLOY-09: conferências que não dependem de namespace: chave versionada, paridade
// com os outros scripts, proibições, unit gerada e a lista de literais de IP.

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func scriptD09(t *testing.T) string { return arquivoObrigatorio(t, "deploy", "restaurar-maquina.sh") }

func funcaoShell(t *testing.T, texto, nome string) string {
	t.Helper()
	f := regexp.MustCompile(`(?ms)^` + nome + `\(\) \{.*?^\}\n`).FindString(texto)
	require.NotEmpty(t, f, nome)
	return f
}

func TestDEPLOY09_ChaveVersionadaProduzOFingerprint(t *testing.T) {
	ferramentaObrigatoria(t, "gpg")
	fp := arquivoObrigatorio(t, "deploy", "cloudflare-apt.fingerprint")
	assert.Contains(t, strings.ToLower(fp), "confirmacao por um segundo canal")
	util := []string{}
	for _, l := range linhas(fp) {
		if !strings.HasPrefix(l, "#") && strings.TrimSpace(l) != "" {
			util = append(util, l)
		}
	}
	require.Equal(t, []string{"CC94B39C77AE7342A68B89628A682D308D4E5E73"}, util)

	asc := caminhoRaiz(t, "deploy", "cloudflare-apt.asc")
	home, err := os.MkdirTemp("", "g09")
	require.NoError(t, err)
	defer os.RemoveAll(home)
	out, err := exec.Command("gpg", "--homedir", home, "--batch", "--no-autostart", "--with-colons", "--fixed-list-mode", "--show-keys", asc).Output()
	require.NoError(t, err)
	var pubs, primaria []string
	for i, l := range linhas(string(out)) {
		if strings.HasPrefix(l, "pub:") {
			pubs = append(pubs, l)
			primaria = strings.Split(linhas(string(out))[i+1], ":")
		}
	}
	require.Len(t, pubs, 1, "uma única chave")
	assert.Equal(t, util[0], primaria[9])
	assert.Contains(t, string(out), "CloudFlare Software Packaging 2025")
	assert.Contains(t, lerRaiz(t, "deploy", "cloudflare-apt.asc"), "BEGIN PGP PUBLIC KEY BLOCK")
}

func TestDEPLOY09_Proibicoes(t *testing.T) {
	sc := codigoSemComentarios(scriptD09(t))
	for _, p := range []string{"service install", "--token ", ".deb", "apt-key", "trusted=yes", "MYSQL_PWD", "--dev-ip", "--recriar", "find -delete", "seed-producao.sh --"} {
		assert.NotContains(t, sc, p)
	}
	assert.NotContains(t, sc, " -delete")
	assert.NotRegexp(t, `curl[^\n]*\|\s*(ba)?sh`, sc, "sem curl|bash")
	assert.NotRegexp(t, `(?m)^\s*set\s+-[a-z]*x`, sc, "nunca set -x")
	assert.NotRegexp(t, `(?m)^\s*source\s|^\s*\.\s+"`, sc, "nada é lido com source")
	assert.NotRegexp(t, `(?m)export[^\n]*(PEPPER|TOKEN=|TUNNEL_TOKEN)`, sc, "pepper e token sem export")
	assert.NotRegexp(t, `(?m)^\s*export\s+[^\n]*\bTOKEN\b`, sc)
	assert.Contains(t, sc, "set -euo pipefail")
	assert.Contains(t, sc, "umask 077")
	// Sem válvula: nenhuma variável de ambiente de teste ou de força é lida.
	assert.NotRegexp(t, `\$\{?(FORCAR|FORCE|RP_TESTE|TESTE|CI|DEBUG|FAKE)\b`, sc)
	assert.NotContains(t, sc, "BASH_SOURCE", "sem modo de biblioteca")

	// O script não se instala nem vira alvo do Makefile (SEC-46).
	assert.NotContains(t, sc, "install -m 700 -- \"$0\"")
	assert.NotRegexp(t, `(?m)^[a-z-]*(restaura|restore)[a-z-]*:`, lerRaiz(t, "Makefile"))
	assert.NotContains(t, lerRaiz(t, "Makefile"), "restaurar-maquina.sh\n\t")
	assert.NotRegexp(t, `(?m)^\s*[^/\s][^\n]*restaurar-maquina`, lerRaiz(t, "Jenkinsfile"), "nunca pelo Jenkins; só o comentário da lista de digest")
}

func TestDEPLOY09_Paridade(t *testing.T) {
	sc := scriptD09(t)
	md := lerRaiz(t, "deploy", "mariadb-docker.sh")

	re := regexp.MustCompile(`(?m)^IMAGEM_BASE="([^"]+)"$`)
	v, m := re.FindStringSubmatch(sc), re.FindStringSubmatch(md)
	require.NotNil(t, v)
	require.NotNil(t, m)
	assert.Equal(t, m[1], v[1], "mesma string do IMAGEM_BASE (SEC-31)")
	assert.Regexp(t, reImagemMariaDB, v[1])

	fontes := regexp.MustCompile(`(?m)^IMAGENS_FIXADAS_EM\s*:?=\s*(.+)$`).FindStringSubmatch(lerRaiz(t, "Makefile"))
	require.NotNil(t, fontes)
	assert.Contains(t, strings.Fields(fontes[1]), "deploy/restaurar-maquina.sh")
	assert.Contains(t, md, "deploy/restaurar-maquina.sh", "lista de troca de digest do mariadb-docker.sh")
	assert.Contains(t, lerRaiz(t, "Jenkinsfile"), "deploy/restaurar-maquina.sh", "lista de troca de digest do Jenkinsfile")

	// Lista de CAs de dev igual à do mariadb-docker.sh.
	lista := func(texto string) []string {
		bloco := regexp.MustCompile(`(?s)readonly -a CA_DEV_FPS=\((.*?)\n\)`).FindStringSubmatch(texto)
		require.NotNil(t, bloco)
		return regexp.MustCompile(`"([0-9a-f]{64})"`).FindAllString(bloco[1], -1)
	}
	assert.Equal(t, lista(md), lista(sc))

	assert.Equal(t, funcaoShell(t, lerRaiz(t, "deploy", "seed-producao.sh"), "arquivo_secreto_ok"), funcaoShell(t, sc, "arquivo_secreto_ok"))

	// Caminhos e constantes dos scripts de que este depende.
	bb := lerRaiz(t, "deploy", "backup-banco.sh")
	for _, c := range []string{`VOLUME_IMAGENS="rotaperfumes_produto_imagens"`, `VOLUME_ROTULO="rotaperfumes/produto_imagens"`, `USUARIO_IMAGENS="65532:65532"`} {
		assert.Contains(t, sc, "readonly "+c)
		assert.Contains(t, bb, "readonly "+c)
	}
	assert.Contains(t, sc, `readonly REPO_URL="https://github.com/residentivo/RotaPerfume.git"`)
	assert.Contains(t, lerRaiz(t, "deploy", "setup-servidor.sh"), `readonly REPO_URL="https://github.com/residentivo/RotaPerfume.git"`)
	for _, c := range []string{`DBA_CLIENTE="rotaperfumes-dba-cliente"`, `C_DBA="/run/dba"`, `DBA_CMD="/usr/local/sbin/rotaperfumes-dba"`} {
		assert.Contains(t, sc, "readonly "+c)
		assert.Contains(t, md, "readonly "+c)
	}
	// O comando pronto do DBA e o cliente da importação usam os mesmos limites; o
	// único acréscimo do cliente é --sandbox.
	cliente := funcaoShell(t, sc, "cliente_importacao")
	gerado := funcaoShell2(t, md)
	for _, f := range []string{"--read-only --cap-drop ALL", "--security-opt no-new-privileges:true --log-driver none --tmpfs /tmp:rw,nosuid,nodev,noexec",
		"-e MYSQL_HISTFILE=/dev/null", "--local-infile=0 --default-character-set=utf8mb4", "--init-command=SET time_zone='-03:00'", "--ssl-verify-server-cert"} {
		assert.Contains(t, cliente, f)
		assert.Contains(t, gerado, f)
	}
	assert.Contains(t, cliente, "--sandbox")
	assert.Equal(t, 1, strings.Count(sc, "--sandbox --batch"))

	// A API continua pegando a senha do arquivo que este script lê.
	assert.Contains(t, md, `"$SEGREDOS_DIR/$DB_APP.senha"`)
	assert.Contains(t, sc, `SENHA_APP_ARQ="$SEGREDOS_DIR/rotaperfumes_app.senha"`)
}

func funcaoShell2(t *testing.T, md string) string {
	t.Helper()
	f := regexp.MustCompile(`(?ms)^gerar_script_dba\(\) \{.*?^EOF_DBA\n\}\n`).FindString(md)
	require.NotEmpty(t, f)
	return f
}

func TestDEPLOY09_ListarManuais(t *testing.T) {
	precisaBash(t)
	cmd := exec.Command("bash", caminhoRaiz(t, "deploy", "restaurar-maquina.sh"), "--listar-manuais")
	cmd.Env = ambienteLimpo()
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	s := string(out)
	for _, trecho := range []string{
		"hostname publico", "https://localhost:8443", "noTLSVerify", "token do tunel", "parar o conector", "hostnames adicionais",
		"hostnames do Turnstile", "Rotate token", "/connections",
		"rotaperfumes-api-env", "rotaperfumes-db-admin", "rotaperfumes_admin", "secoes 5 e 6",
		"PASSWORD_PEPPER", "JWT_SECRET", "TURNSTILE_SECRET_KEY", "SMTP_*", "SECURITY_ALERT_EMAILS", "CORS_ALLOWED_ORIGINS", "TURNSTILE_HOSTNAMES",
		"raiz da CA do Caddy", "ca-key.pem", "copia externa",
	} {
		assert.Contains(t, s, trecho)
	}
	assert.NotRegexp(t, `(?m)^\s*(PASSWORD_PEPPER|JWT_SECRET|TUNNEL_TOKEN)=`, s, "só nomes, nunca valores")
}

// Todo arquivo com o literal do IP, fora de docs e testes, está na lista impressa.
func TestDEPLOY09_ListaCobreLiteraisDoIP(t *testing.T) {
	precisaBash(t)
	if emCI() {
		t.Skip("o estágio Testes Go recebe um tar, sem .git")
	}
	cmd := exec.Command("bash", caminhoRaiz(t, "deploy", "restaurar-maquina.sh"), "--listar-manuais")
	cmd.Env = ambienteLimpo()
	saida, err := cmd.CombinedOutput()
	require.NoError(t, err, string(saida))
	lista := string(saida)

	grep := exec.Command("git", "-C", raiz(t), "grep", "-lF", "192.168.168.106", "--", ":!docs", ":!tarefas", ":!postman", ":!CLAUDE.md", ":!*_test.go", ":!.claude")
	out, err := grep.Output()
	require.NoError(t, err)
	for _, f := range linhas(string(out)) {
		assert.Contains(t, lista, f, "%s tem o IP e não está na lista impressa", f)
	}
	for _, f := range []string{"Jenkinsfile", "deploy/docker-compose.yml", "deploy/mariadb-docker.sh", "deploy/setup-servidor.sh", "Makefile", "backend/shared/testkit/integracao/integracao.go"} {
		assert.Contains(t, lista, f)
	}
	assert.Contains(t, lista, "CasaOS de dev", "o que não muda também é dito")
}

func TestDEPLOY09_UnitGerada(t *testing.T) {
	precisaBash(t)
	sc := scriptD09(t)
	gerador := funcaoShell(t, sc, "gerar_unit_tunel")
	dir := t.TempDir()
	bin := filepath.Join(dir, "cloudflared")
	escrever(t, bin, "#!/bin/sh\nexit 0\n", 0o755)
	unitPath := filepath.Join(dir, "rotaperfumes-cloudflared.service")
	// Lote 42: as constantes do script (readonly), com set -u: nenhuma pode faltar.
	constantes := regexp.MustCompile(`(?m)^readonly (TUNEL_[A-Z_]+|PORTA_HTTPS)=.*$`).FindAllString(sc, -1)
	require.NotEmpty(t, constantes)
	cmd := exec.Command("bash", "-c", "set -u\n"+strings.Join(constantes, "\n")+"\n"+gerador+"gerar_unit_tunel \"$1\"", "_", bin)
	cmd.Env = ambienteLimpo()
	out, err := cmd.Output()
	require.NoError(t, err)
	unit := string(out)
	escrever(t, unitPath, unit, 0o644)

	diretivas := []string{"EnvironmentFile=/etc/rotaperfumes/cloudflared.env",
		"ExecStart=" + bin + " tunnel --no-autoupdate --metrics 127.0.0.1:20241 run", "Type=notify",
		"Restart=on-failure", "DynamicUser=yes", "NoNewPrivileges=yes", "ProtectSystem=strict", "ProtectHome=yes", "PrivateTmp=yes", "CapabilityBoundingSet=",
		// Lote 42 (SecBrain): endurecimento.
		"PrivateDevices=yes", "ProtectKernelTunables=yes", "ProtectKernelModules=yes", "ProtectKernelLogs=yes", "ProtectControlGroups=yes",
		"ProtectClock=yes", "ProtectHostname=yes", "ProtectProc=invisible", "ProcSubset=pid",
		"RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX AF_NETLINK", "RestrictNamespaces=yes", "LockPersonality=yes", "RestrictRealtime=yes",
		"MemoryDenyWriteExecute=yes", "SystemCallArchitectures=native", "SystemCallFilter=@system-service", "UMask=0077",
		"IPAddressDeny=10.0.0.0/8 172.16.0.0/12 192.168.0.0/16 169.254.0.0/16", "IPAddressAllow=127.0.0.0/8 ::1",
		"WantedBy=multi-user.target"}
	for _, l := range diretivas {
		assert.Regexp(t, `(?m)^`+regexp.QuoteMeta(l)+`$`, unit, "linha exata")
	}
	assert.NotContains(t, unit, "--token")
	assert.NotContains(t, unit, "Environment=TUNNEL")
	assert.Contains(t, unit, "NAO com o cloudflared real", "o comentario diz o que falta provar")

	if _, err := exec.LookPath("systemd-analyze"); err != nil {
		t.Skip("systemd-analyze não está no PATH")
	}
	verif := exec.Command("systemd-analyze", "verify", unitPath)
	verif.Env = ambienteLimpo()
	saida, err := verif.CombinedOutput()
	// Só a própria unit importa: outras unidades do host podem reclamar de si mesmas.
	for _, l := range linhas(string(saida)) {
		assert.NotContains(t, l, "rotaperfumes-cloudflared", "systemd-analyze verify: %s", l)
	}
	assert.NoError(t, err, "%s", saida)

	// Lote 42: nota do systemd-analyze security, sem root (--offline=yes, se houver).
	sec := exec.Command("systemd-analyze", "security", "--offline=yes", unitPath)
	sec.Env = ambienteLimpo()
	saida, err = sec.CombinedOutput()
	if err != nil && strings.Contains(string(saida), "offline") {
		t.Skipf("systemd-analyze sem --offline: %s", saida)
	}
	require.NoError(t, err, "%s", saida)
	m := regexp.MustCompile(`Overall exposure level for \S+: ([0-9.]+) (\S+)`).FindStringSubmatch(string(saida))
	require.NotNil(t, m, "%s", saida)
	nota, err := strconv.ParseFloat(m[1], 64)
	require.NoError(t, err)
	t.Logf("systemd-analyze security: %s %s", m[1], m[2])
	assert.LessOrEqual(t, nota, 2.0, "exposicao da unit (antes do Lote 42 era MEDIUM): %s", saida)
}

func TestDEPLOY09_Documentacao(t *testing.T) {
	manual := lerForaDoTarDoCI(t, "docs", "manual-base-de-dados.md")
	i := strings.Index(manual, "### 24.6 Perda do servidor")
	require.GreaterOrEqual(t, i, 0)
	assert.Contains(t, manual[i:], "deploy/restaurar-maquina.sh")
	assert.Contains(t, manual[i:], "como referência")
	assert.NotContains(t, manual, "Nenhum script ou alvo do `Makefile` grava o backup em produção: a restauração real é só este runbook")

	dep := lerForaDoTarDoCI(t, "docs", "deploy-servidor.md")
	j := strings.Index(dep, "## 12. Restaurar em máquina nova (DEPLOY-09)")
	require.GreaterOrEqual(t, j, 0)
	for _, trecho := range []string{"restaurar-maquina.sh", "--listar-manuais", "--sem-tunel", "ANTIGA PARADA", "Só em máquina real", "segundo canal", "rotaperfumes-cloudflared.service"} {
		assert.Contains(t, dep[j:], trecho)
	}
}

// Textos corrigidos: --dev-ip não é mais o caminho normal de nada.
func TestDEPLOY09_TextosDeDevIP(t *testing.T) {
	setup := lerRaiz(t, "deploy", "setup-servidor.sh")
	assert.NotContains(t, setup, "[--dev-ip IP ...]")
	assert.NotContains(t, setup, "[--dev-ip ...]")
	assert.NotContains(t, setup, "so e publicada com --dev-ip")
	for _, f := range [][]string{{"Makefile"}, {"backend", "shared", "testkit", "integracao", "integracao.go"}} {
		assert.NotContains(t, lerRaiz(t, f...), "a porta publicada com --dev-ip", f)
	}
	assert.NotContains(t, lerRaiz(t, ".gitignore"), "maquina do admin (--dev-ip)")
}
