//go:build linux

package deploy_test

// IMG-02: deploy/gerar-imagens-producao.sh gera as imagens de produto na
// produção (3306 fechada desde o DEPLOY-10). Estático + opções (sem root).

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func img02Script(t *testing.T) string {
	t.Helper()
	return lerRaiz(t, "deploy", "gerar-imagens-producao.sh")
}

func img02Rodar(t *testing.T, args ...string) (string, int) {
	t.Helper()
	precisaBash(t)
	cmd := exec.Command("bash", append([]string{caminhoRaiz(t, "deploy", "gerar-imagens-producao.sh")}, args...)...)
	cmd.Env = ambienteLimpo()
	out, err := cmd.CombinedOutput()
	rc := 0
	if ee, ok := err.(*exec.ExitError); ok {
		rc = ee.ExitCode()
	} else {
		require.NoError(t, err)
	}
	return string(out), rc
}

// As guardas são cópia byte a byte das do seed-producao.sh (e, por tabela, do
// mariadb-docker.sh: TST-17 e BACKUP-01 já conferem as duas).
func TestIMG02_GuardasCopiadasDoSeed(t *testing.T) {
	sc, sp := img02Script(t), lerRaiz(t, "deploy", "seed-producao.sh")
	for _, nome := range []string{"arquivo_secreto_ok", "conferir_dir_privado", "conferir_backup_recente", "conferir_ca_host", "conferir_container_rede"} {
		re := regexp.MustCompile(`(?ms)^` + nome + `\(\) \{.*?^\}\n`)
		f := re.FindString(sc)
		require.NotEmpty(t, f, nome)
		assert.Equal(t, re.FindString(sp), f, nome)
	}
}

func TestIMG02_ImagemEVolumeIguaisAosDoDeploy(t *testing.T) {
	sc := img02Script(t)
	m := regexp.MustCompile(`(?m)^GO_IMAGE="([^"]+)"$`).FindStringSubmatch(sc)
	require.NotNil(t, m)
	assert.Equal(t, imagemJenkins(t, "GO_IMAGE"), m[1], "trocar junto com o Jenkinsfile (SEC-31)")

	mk := lerRaiz(t, "Makefile")
	fontes := regexp.MustCompile(`(?m)^IMAGENS_FIXADAS_EM\s*:?=\s*(.+)$`).FindStringSubmatch(mk)
	require.NotNil(t, fontes)
	assert.Contains(t, strings.Fields(fontes[1]), "deploy/gerar-imagens-producao.sh", "deps-audit-imagens")

	compose := lerRaiz(t, "deploy", "docker-compose.yml")
	assert.Regexp(t, `(?m)^name: rotaperfumes$`, compose)
	assert.Contains(t, compose, "- produto_imagens:/data/produtos-imagens")
	assert.Contains(t, compose, "PRODUTO_IMAGENS_DIR: /data/produtos-imagens")
	assert.Contains(t, sc, `readonly VOLUME="rotaperfumes_produto_imagens"`)
	assert.Contains(t, sc, `readonly DIR_IMAGENS="/data/produtos-imagens"`)
	assert.Contains(t, lerRaiz(t, "backend", "Dockerfile"), "--chown=65532:65532 /out/data /data")
	assert.Contains(t, sc, `readonly UID_IMAGENS="65532"`)
	assert.Contains(t, sc, `readonly IP_ADMIN="172.30.42.20"`)
}

func TestIMG02_ExecucaoIsolada(t *testing.T) {
	sc := img02Script(t)
	codigo := codigoSemComentarios(sc)
	assert.NotRegexp(t, `(?m)^\s*set -[a-z]*x`, codigo, "nunca set -x")
	assert.Contains(t, codigo, "set -euo pipefail")

	i := strings.Index(codigo, `docker run --rm --network "$REDE_NOME" --ip "$IP_ADMIN"`)
	require.GreaterOrEqual(t, i, 0, "docker run do gerarimagens")
	run := codigo[i : i+strings.Index(codigo[i:], `</dev/null`)]
	for _, s := range []string{`--user "$UID_IMAGENS:$UID_IMAGENS"`, "--cap-drop ALL", "--security-opt no-new-privileges:true", "--read-only",
		`--env-file "$ENV_ARQ"`, "target=$C_CA,readonly", "target=/usr/local/bin/gerarimagens,readonly", "target=$DIR_IMAGENS$VOL_MODO"} {
		assert.Contains(t, run, s)
	}
	assert.NotContains(t, run, "SENHA_DB", "a senha só vai pelo env-file")
	assert.NotContains(t, run, "-e DB_")

	// dry-run monta o volume só leitura; a senha só aparece no printf do env-file.
	assert.Regexp(t, `(?s)ARGS_GI=\(-dry-run\)\s+VOL_MODO=",readonly"`, codigo)
	usoSenha := regexp.MustCompile(`\$SENHA_DB([^_A-Za-z0-9]|$)`)
	for _, l := range strings.Split(codigo, "\n") {
		if usoSenha.MatchString(l) {
			assert.Regexp(t, `^\s*(SENHA_DB=|\[\[ "\$SENHA_DB" =~|printf 'DB_SENHA=%s\\n' "\$SENHA_DB")`, l)
		}
	}
	assert.Contains(t, codigo, `shred -u "$ENV_ARQ"`)
	// JWT_SECRET e PASSWORD_PEPPER aleatórios: o pepper real nunca é lido.
	assert.Contains(t, codigo, `printf 'JWT_SECRET=%s\n' "$(openssl rand -hex 32)"`)
	assert.Contains(t, codigo, `printf 'PASSWORD_PEPPER=%s\n' "$(openssl rand -hex 32)"`)
	assert.NotContains(t, codigo, "pepper-file")

	// Compilação sem rede do banco nem segredos; sem o .env e sem as imagens locais.
	j := strings.Index(codigo, `tar -C "$RAIZ"`)
	require.GreaterOrEqual(t, j, 0)
	build := codigo[j : j+strings.Index(codigo[j:], "falhou a compilacao")]
	for _, s := range []string{"--exclude='.env'", "--exclude='*.env'", "--exclude='backend/rotaperfumes-api/data'", "--cap-drop ALL", "tar -xof -"} {
		assert.Contains(t, build, s)
	}
	for _, s := range []string{"--network", "--env-file", "SENHA_DB"} {
		assert.NotContains(t, build, s)
	}

	// Com --confirmar: backup recente antes de gravar.
	assert.Regexp(t, `(?s)if \[\[ "\$MODO" == confirmar \]\]; then.*?conferir_backup_recente "\$BACKUP_DIR" "\$BACKUP_JANELA_MIN"`, codigo)
}

func TestIMG02_Opcoes(t *testing.T) {
	casos := []struct {
		nome string
		args []string
		rc   int
		msg  string
	}{
		{"sem modo", nil, 2, "escolha --dry-run ou --confirmar"},
		{"os dois modos", []string{"--dry-run", "--confirmar"}, 1, "use --dry-run OU --confirmar"},
		{"sobrescrever no dry-run", []string{"--dry-run", "--sobrescrever"}, 1, "--sobrescrever so vale com --confirmar"},
		{"motivo no dry-run", []string{"--dry-run", "--sem-backup-motivo", "motivo bem longo"}, 1, "--sem-backup-motivo so vale com --confirmar"},
		{"motivo curto", []string{"--confirmar", "--sem-backup-motivo", "curto"}, 1, "de 10 a 300 caracteres"},
		{"motivo longo", []string{"--confirmar", "--sem-backup-motivo", strings.Repeat("x", 301)}, 1, "de 10 a 300 caracteres"},
		{"valor ausente", []string{"--confirmar", "--backup-dir"}, 1, "a opcao --backup-dir exige um valor"},
		{"opcao desconhecida", []string{"--container", "x"}, 2, "Opcao desconhecida: --container"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out, rc := img02Rodar(t, c.args...)
			assert.Equal(t, c.rc, rc, out)
			assert.Contains(t, out, c.msg)
			assert.NotContains(t, out, "=== [1/4]", "recusa antes de qualquer etapa")
		})
	}
	t.Run("help", func(t *testing.T) {
		out, rc := img02Rodar(t, "--help")
		assert.Equal(t, 0, rc)
		assert.Contains(t, out, "--dry-run")
		assert.Contains(t, out, "sudo rotaperfumes-backup-banco --agora")
	})
	t.Run("sem root", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("rodando como root: a recusa não se aplica")
		}
		out, rc := img02Rodar(t, "--dry-run")
		assert.Equal(t, 1, rc, out)
		assert.Contains(t, out, "rode como root")
	})
}
