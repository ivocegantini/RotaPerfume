package deploy_test

// SEC-47 (Lote 39): as imagens base dos Dockerfile ficam fixadas por digest
// (tag@sha256), resolvendo os ARG padrão do próprio arquivo. As do Go e do
// Node são as mesmas strings do GO_IMAGE/NODE_IMAGE do Jenkinsfile.

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var reRefDigest = regexp.MustCompile(`^[a-z0-9._/-]+:[A-Za-z0-9._-]+@sha256:[0-9a-f]{64}$`)

// fromsResolvidos devolve as imagens dos FROM com os ${ARG} padrão trocados.
func fromsResolvidos(t *testing.T, texto string) []string {
	t.Helper()
	args := map[string]string{}
	var out []string
	for _, l := range strings.Split(texto, "\n") {
		f := strings.Fields(l)
		if len(f) < 2 {
			continue
		}
		switch strings.ToUpper(f[0]) {
		case "ARG":
			if k, v, ok := strings.Cut(f[1], "="); ok {
				args[k] = v
			}
		case "FROM":
			r := f[1]
			for k, v := range args {
				r = strings.ReplaceAll(r, "${"+k+"}", v)
			}
			out = append(out, r)
		}
	}
	require.NotEmpty(t, out, "Dockerfile sem FROM")
	return out
}

func imagemJenkins(t *testing.T, nome string) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^\s*` + nome + `\s*=\s*'([^']+)'`).FindStringSubmatch(lerRaiz(t, "Jenkinsfile"))
	require.NotNil(t, m, nome+" no Jenkinsfile")
	return m[1]
}

func TestSEC47_Dockerfiles_TodoFromComDigest(t *testing.T) {
	for _, df := range [][]string{{"backend", "Dockerfile"}, {"frontend", "Dockerfile"}, {"deploy", "Dockerfile.caddy"}} {
		t.Run(strings.Join(df, "/"), func(t *testing.T) {
			for _, r := range fromsResolvidos(t, lerRaiz(t, df...)) {
				assert.Regexp(t, reRefDigest, r, "FROM fixado por tag@sha256")
				assert.NotContains(t, r, "$", "ARG sem valor padrão no FROM")
			}
		})
	}
}

func TestSEC47_Backend_MesmaImagemDoJenkins(t *testing.T) {
	df := lerRaiz(t, "backend", "Dockerfile")
	froms := fromsResolvidos(t, df)
	require.Len(t, froms, 2)
	assert.Equal(t, imagemJenkins(t, "GO_IMAGE"), froms[0], "golang = GO_IMAGE do Jenkinsfile")
	assert.Regexp(t, `^gcr\.io/distroless/static-debian12:nonroot@sha256:`, froms[1])
	assert.Regexp(t, `(?m)^ARG GO_VERSION=\d+\.\d+\.\d+$`, df, "GO_VERSION com patch")
	assert.Contains(t, df, "FROM golang:${GO_VERSION}-alpine@${GO_DIGEST} AS build")
}

func TestSEC47_Frontend_MesmaImagemDoJenkins(t *testing.T) {
	df := lerRaiz(t, "frontend", "Dockerfile")
	froms := fromsResolvidos(t, df)
	require.Len(t, froms, 3, "deps, builder e runner")
	for _, r := range froms {
		assert.Equal(t, imagemJenkins(t, "NODE_IMAGE"), r, "node = NODE_IMAGE do Jenkinsfile")
	}
	assert.Regexp(t, `(?m)^ARG NODE_VERSION=\d+\.\d+\.\d+-alpine$`, df, "NODE_VERSION com patch")
	assert.Equal(t, 3, strings.Count(df, "FROM node:${NODE_VERSION}@${NODE_DIGEST} AS "))
}

func TestSEC47_Caddy_TagComPatch(t *testing.T) {
	froms := fromsResolvidos(t, lerRaiz(t, "deploy", "Dockerfile.caddy"))
	require.Len(t, froms, 1)
	assert.Regexp(t, `^caddy:2\.\d+\.\d+-alpine@sha256:[0-9a-f]{64}$`, froms[0], "sem a tag flutuante 2-alpine")
}
