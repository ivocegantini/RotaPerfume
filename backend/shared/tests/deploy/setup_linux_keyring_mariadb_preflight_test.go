package deploy_test

// SEC-61 (da lote-42, portado no MERGE-01): keyring do MariaDB já instalado
// que não confere com o MARIADB_CHAVE_FINGERPRINT. O mariadb.sources confia
// nele, então o preflight roda em main() antes de qualquer apt: no modo
// instalar aborta; no --check (ou sem apt) registra FALTA (divergente) ou
// AVISO (não conferível). Roda o scripts/setup-linux.sh com chaves
// descartáveis (sec58GerarChaves); nada é instalado.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// marcaDepoisPreflightSEC61: só aparece se o preflight deixou seguir para o apt.
const marcaDepoisPreflightSEC61 = "SEGUIU-PARA-O-APT"

func rodarSetupSEC61(t *testing.T, env []string, trecho string, args ...string) (string, error) {
	t.Helper()
	script := "s=$1; shift; ARGS=(\"$@\"); set --; source \"$s\"; set -- \"${ARGS[@]}\"; " +
		"registrar() { printf 'REGISTRAR %s|%s|%s\\n' \"$1\" \"$2\" \"$3\"; }; " + trecho
	cmd := exec.Command("bash", append([]string{"-c", script, "_", caminhoRaiz(t, "scripts", "setup-linux.sh")}, args...)...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ambienteSemGpgSEC61: o PATH do ambienteLimpo sem o gpg (links para o resto).
func ambienteSemGpgSEC61(t *testing.T) []string {
	t.Helper()
	bin := t.TempDir()
	for _, c := range []string{"tr", "mktemp", "awk", "rm", "grep", "cat", "sed", "uname", "dirname", "id", "head"} {
		if p, err := exec.LookPath(c); err == nil {
			require.NoError(t, os.Symlink(p, filepath.Join(bin, c)))
		}
	}
	var env []string
	for _, kv := range ambienteLimpo() {
		if !strings.HasPrefix(kv, "PATH=") {
			env = append(env, kv)
		}
	}
	return append(env, "PATH="+bin)
}

func codigoSaida(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var ee *exec.ExitError
	require.True(t, errors.As(err, &ee), "%v", err)
	return ee.ExitCode()
}

func TestSEC61_ConferirChave_CodigosDeRetorno(t *testing.T) {
	c := sec58GerarChaves(t)
	casos := []struct {
		nome, fpr string
		env       []string
		rc        int
	}{
		{"certa", c.fprA, ambienteLimpo(), 0},
		{"divergente", c.fprB, ambienteLimpo(), 1},
		{"FPR inválido", "curto", ambienteLimpo(), 2},
		{"sem gpg", c.fprA, ambienteSemGpgSEC61(t), 2},
	}
	for _, cs := range casos {
		t.Run(cs.nome, func(t *testing.T) {
			saida := filepath.Join(t.TempDir(), "saida.pgp")
			out, err := rodarSetupSEC61(t, cs.env, `conferir_chave_mariadb "$1" "$2" "$3"`, c.a, cs.fpr, saida)
			assert.Equal(t, cs.rc, codigoSaida(t, err), out)
		})
	}
}

func TestSEC61_PreflightKeyring(t *testing.T) {
	c := sec58GerarChaves(t)
	k := c.a
	ausente := filepath.Join(t.TempDir(), "nao-existe.pgp")
	remedio := "sudo rm " + k + " /etc/apt/sources.list.d/mariadb.sources && make setup"
	trecho := `MODO=$1; APT=$2; MARIADB_CHAVE_FINGERPRINT=$4; preflight_keyring_mariadb "$3"; echo ` + marcaDepoisPreflightSEC61

	casos := []struct {
		nome, modo, apt, k, fpr string
		env                     []string
		rc                      int
		seguiu                  bool
		msg                     string
	}{
		{"instalar: certo segue", "instalar", "1", k, c.fprA, ambienteLimpo(), 0, true, "confere com MARIADB_CHAVE_FINGERPRINT"},
		{"instalar: ausente segue", "instalar", "1", ausente, c.fprA, ambienteLimpo(), 0, true, ""},
		{"instalar: divergente aborta", "instalar", "1", k, c.fprB, ambienteLimpo(), 1, false, "SEC-61: " + k + " não confere com MARIADB_CHAVE_FINGERPRINT"},
		{"instalar: sem gpg aborta", "instalar", "1", k, c.fprA, ambienteSemGpgSEC61(t), 1, false, "não foi possível conferir " + k + ": gpg ausente"},
		{"instalar: FPR inválido aborta", "instalar", "1", k, "curto", ambienteLimpo(), 1, false, "MARIADB_CHAVE_FINGERPRINT inválido"},
		{"instalar sem apt: só registra", "instalar", "0", k, c.fprB, ambienteLimpo(), 0, true, "REGISTRAR mariadb-key|FALTA|" + k + " não confere"},
		{"check: divergente = FALTA", "check", "1", k, c.fprB, ambienteLimpo(), 0, true, "REGISTRAR mariadb-key|FALTA|" + k + " não confere"},
		{"check: sem gpg = AVISO", "check", "1", k, c.fprA, ambienteSemGpgSEC61(t), 0, true, "REGISTRAR mariadb-key|AVISO|não foi possível conferir " + k + ": gpg ausente"},
	}
	for _, cs := range casos {
		t.Run(cs.nome, func(t *testing.T) {
			out, err := rodarSetupSEC61(t, cs.env, trecho, cs.modo, cs.apt, cs.k, cs.fpr)
			assert.Equal(t, cs.rc, codigoSaida(t, err), out)
			assert.Contains(t, out, cs.msg)
			if cs.seguiu {
				assert.Contains(t, out, marcaDepoisPreflightSEC61)
			} else {
				assert.NotContains(t, out, marcaDepoisPreflightSEC61, "aborta antes do apt")
				assert.Contains(t, out, remedio)
			}
		})
	}
}

// O preflight fica em main() depois de detectar_distro (precisa do APT) e
// antes de etapa_basicas (primeiro apt-get), nos dois modos.
func TestSEC61_PreflightAntesDoApt(t *testing.T) {
	sh := lerRaiz(t, "scripts", "setup-linux.sh")
	i := strings.Index(sh, "\nmain() {")
	require.GreaterOrEqual(t, i, 0)
	principal := sh[i:]
	if j := strings.Index(principal, "\n}\n"); j >= 0 {
		principal = principal[:j]
	}
	const chamada = "\tpreflight_keyring_mariadb /etc/apt/keyrings/mariadb-keyring.pgp\n"
	iDistro := strings.Index(principal, "\tdetectar_distro\n")
	iPre := strings.Index(principal, chamada)
	iBasicas := strings.Index(principal, "\tetapa_basicas\n")
	require.GreaterOrEqual(t, iDistro, 0)
	require.GreaterOrEqual(t, iPre, 0)
	require.GreaterOrEqual(t, iBasicas, 0)
	assert.Less(t, iDistro, iPre)
	assert.Less(t, iPre, iBasicas)
	assert.Equal(t, 1, strings.Count(sh, chamada), "uma única chamada, sem condição de modo")
}
