package deploy_test

// DEPLOY-10: estágio "Migracoes" (migrações da loja antes do Deploy, pela
// conta admin na rede rotaperfumes-db) e GOOGLE_CLIENT_ID no build do frontend.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var migracoesDEPLOY10 = []string{
	"27_alter_usuarios_role_usuario.sql", "28_ddl_confirmacoes_email.sql",
	"29_alter_usuarios_email_confirmacao.sql", "30_alter_clientes_loja.sql",
	"31_alter_clientes_numero_complemento.sql", "33_alter_pedidos_canal_loja.sql",
	"34_alter_produtos_nome_descricao.sql", "35_alter_pedidos_chave_checkout.sql",
}

func TestDEPLOY10_Jenkinsfile_MigracoesAntesDoDeploy(t *testing.T) {
	jf := lerRaiz(t, "Jenkinsfile")
	iTestes := strings.Index(jf, "stage('Testes Frontend')")
	iCA := strings.Index(jf, "stage('CA do banco')")
	iMig := strings.Index(jf, "stage('Migracoes')")
	iDeploy := strings.Index(jf, "stage('Deploy')")
	require.GreaterOrEqual(t, iMig, 0)
	assert.Less(t, iTestes, iMig, "depois dos testes: build vermelho não altera o banco")
	assert.Less(t, iCA, iMig, "depois da validação da CA")
	// DEPLOY-15: o estágio "Importar dump" e seus parâmetros saíram.
	assert.NotRegexp(t, `(?i)importar dump|IMPORTAR_DUMP|DUMP_PATH|zcat`, jf)
	assert.Less(t, iMig, iDeploy, "antes de subir a API nova")
	assert.Equal(t, 1, strings.Count(jf, "stage('Migracoes')"))

	corpo := estagioJenkins(t, jf, "Migracoes")
	assert.NotContains(t, corpo, "when {", "roda sempre (migrações idempotentes)")
	for _, f := range migracoesDEPLOY10 {
		assert.Contains(t, corpo, f)
		_, err := os.Stat(filepath.Join(raiz(t), "sql", f))
		assert.NoError(t, err, "sql/%s existe", f)
	}
	for _, s := range []string{"--network \"$DB_REDE\"", "--ip \"$DB_CLIENTE_IP\"", "--ssl-verify-server-cert", "--local-infile=0", "-e MYSQL_PWD \\", "credentialsId: 'rotaperfumes-db-admin'", "#!/bin/bash", "set -euo pipefail"} {
		assert.Contains(t, corpo, s)
	}
	assert.NotRegexp(t, `set -x|-e MYSQL_PWD=|--password|-p"`, corpo)
}

func TestDEPLOY10_Jenkinsfile_BuildSoLeGoogleClientID(t *testing.T) {
	corpo := estagioJenkins(t, lerRaiz(t, "Jenkinsfile"), "Build")
	assert.Contains(t, corpo, "credentialsId: 'rotaperfumes-api-env'")
	assert.Contains(t, corpo, "export GOOGLE_CLIENT_ID")
	assert.Contains(t, corpo, "API_ENV_FILE=/dev/null docker compose")
	// Só a linha GOOGLE_CLIENT_ID sai do api.env: nada de source/cat/--env-file.
	for _, l := range strings.Split(corpo, "\n") {
		if strings.Contains(l, "$API_ENV") {
			assert.Regexp(t, regexp.MustCompile(`sed -n 's/\^GOOGLE_CLIENT_ID=//p' "\$API_ENV"`), l)
		}
	}
	assert.NotRegexp(t, `(?m)^\s*(source|\.)\s|cat "\$API_ENV"|--env-file|set -x`, corpo)
	// O compose repassa o valor ao build do frontend (ECOM-02).
	assert.Contains(t, lerRaiz(t, "deploy/docker-compose.yml"), "NEXT_PUBLIC_GOOGLE_CLIENT_ID: ${GOOGLE_CLIENT_ID:-}")
}
