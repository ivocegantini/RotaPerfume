package config_test

// SEC-77: o buildx 0.30 ignora o --format do "imagetools inspect" e imprime a
// saída inteira; o deps-audit-imagens acusava "DIFERENTE" em toda imagem.
// O digest agora sai da linha "Digest:" (ou da linha só com o digest).

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

func TestSEC77_DepsAuditImagens_SaidaCompletaDoBuildx(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("usa sh e docker/curl falsos em shell")
	}
	makeBin, err := exec.LookPath("make")
	if err != nil {
		t.Skip("make não encontrado")
	}
	d1 := strings.Repeat("a", 64)
	d2 := strings.Repeat("c", 64)
	completa := "Name:      docker.io/library/golang:X\nMediaType: application/vnd.oci.image.index.v1+json\nDigest:    sha256:" + d1 + "\n\nManifests:\n  Name: docker.io/library/golang:X@sha256:" + d2 + "\n"
	for _, c := range []struct{ nome, saida, esperado string }{
		{"buildx 0.30 (ignora --format)", completa, "golang:1.27.4-alpine: igual ao fixado"},
		{"buildx com --format", "sha256:" + d1 + "\n", "golang:1.27.4-alpine: igual ao fixado"},
		{"digest novo", strings.Replace(completa, "Digest:    sha256:"+d1, "Digest:    sha256:"+d2, 1), "golang:1.27.4-alpine: DIFERENTE: atual sha256:" + d2 + " (atualize"},
	} {
		t.Run(c.nome, func(t *testing.T) {
			dir := t.TempDir()
			esc := func(rel, conteudo string, modo os.FileMode) {
				p := filepath.Join(dir, rel)
				require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
				require.NoError(t, os.WriteFile(p, []byte(conteudo), modo))
			}
			mkf, err := os.ReadFile(filepath.Join(raizProjeto(t), "Makefile"))
			require.NoError(t, err)
			esc("Makefile", string(mkf), 0o644)
			esc("Jenkinsfile", "GO_IMAGE = 'golang:1.27.4-alpine@sha256:"+d1+"'\n", 0o644)
			esc("saida.txt", c.saida, 0o644)
			esc("bin/docker", "#!/bin/sh\n[ \"$1\" = buildx ] || exit 1\n[ \"$2\" = version ] && exit 0\ncat \""+filepath.Join(dir, "saida.txt")+"\"\n", 0o755)
			esc("bin/curl", "#!/bin/sh\nexit 7\n", 0o755)
			cmd := exec.Command(makeBin, "-s", "deps-audit-imagens")
			cmd.Dir = dir
			cmd.Env = []string{"PATH=" + filepath.Join(dir, "bin") + ":" + os.Getenv("PATH"), "HOME=" + dir, "LC_ALL=C"}
			out, err := cmd.CombinedOutput()
			s := string(out)
			require.NoError(t, err, s)
			assert.Contains(t, s, c.esperado, s)
			assert.NotContains(t, s, "atual Name:", "nunca compara o texto inteiro do inspect")
		})
	}
}
