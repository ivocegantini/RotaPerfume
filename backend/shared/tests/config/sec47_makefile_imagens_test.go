package config_test

// SEC-47 (Lote 39): "make deps-audit-imagens" cobre as bases dos Dockerfile
// (IMAGENS_DOCKERFILES), resolvendo os ARG padrão do FROM; FROM sem digest
// vira AVISO e o alvo continua informativo. Comportamento testado numa cópia do
// Makefile com Dockerfiles de mentira e um docker/curl falsos (sem rede).

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSEC47_Makefile_ImagensDockerfiles(t *testing.T) {
	mk := lerMakefile(t)
	assert.Contains(t, mk.texto, "IMAGENS_DOCKERFILES = backend/Dockerfile frontend/Dockerfile deploy/Dockerfile.caddy")
	receita := strings.Join(mk.regras["deps-audit-imagens"].receita, "\n")
	assert.Contains(t, receita, "$(IMAGENS_DOCKERFILES)")
	assert.Contains(t, receita, "AVISO (SEC-47): FROM sem digest")
}

func TestSEC47_Makefile_DepsAuditImagens_ResolveArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("usa sh e docker/curl falsos em shell")
	}
	makeBin, err := exec.LookPath("make")
	if err != nil {
		t.Skip("make não encontrado")
	}
	dir := t.TempDir()
	esc := func(rel, conteudo string, modo os.FileMode) {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(conteudo), modo))
	}
	mkf, err := os.ReadFile(filepath.Join(raizProjeto(t), "Makefile"))
	require.NoError(t, err)
	esc("Makefile", string(mkf), 0o644)
	d1 := strings.Repeat("a", 64)
	d2 := strings.Repeat("b", 64)
	esc("backend/Dockerfile", "ARG GO_VERSION=1.26.11\nARG GO_DIGEST=sha256:"+d1+"\n"+
		"FROM golang:${GO_VERSION}-alpine@${GO_DIGEST} AS build\nFROM gcr.io/distroless/static-debian12:nonroot@sha256:"+d2+"\n", 0o644)
	// O ARG de outro arquivo não vaza: NODE_DIGEST não existe aqui.
	esc("frontend/Dockerfile", "ARG NODE_VERSION=22.23.4-alpine\nFROM node:${NODE_VERSION}@${GO_DIGEST} AS deps\n", 0o644)
	esc("deploy/Dockerfile.caddy", "FROM caddy:2-alpine\n", 0o644)
	// docker falso: "buildx imagetools inspect" devolve o digest d1 para tudo.
	esc("bin/docker", "#!/bin/sh\n[ \"$1\" = buildx ] || exit 1\n[ \"$2\" = version ] && exit 0\necho sha256:"+d1+"\n", 0o755)
	esc("bin/curl", "#!/bin/sh\nexit 7\n", 0o755)

	cmd := exec.Command(makeBin, "-s", "deps-audit-imagens")
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + filepath.Join(dir, "bin") + ":" + os.Getenv("PATH"), "HOME=" + dir, "LC_ALL=C"}
	out, err := cmd.CombinedOutput()
	s := string(out)
	require.NoError(t, err, "informativo: nunca falha\n%s", s)

	assert.Contains(t, s, "=== golang:1.26.11-alpine (fixado sha256:"+d1+") ===", "ARG resolvido no FROM")
	assert.Contains(t, s, "golang:1.26.11-alpine: igual ao fixado")
	assert.Contains(t, s, "golang:1.26-alpine: igual ao fixado", "compara também a tag sem patch")
	assert.Contains(t, s, "gcr.io/distroless/static-debian12:nonroot: AVISO: nao foi possivel consultar",
		"fora do Docker Hub consulta o próprio registry (curl falso falha)")
	assert.Contains(t, s, "AVISO (SEC-47): FROM sem digest em frontend/Dockerfile: node:22.23.4-alpine@${GO_DIGEST}")
	assert.Contains(t, s, "AVISO (SEC-47): FROM sem digest em deploy/Dockerfile.caddy: caddy:2-alpine")
	assert.NotContains(t, s, "=== caddy:", "sem digest não entra na comparação")
}
