package deploy_test

// SEC-58 (Lote 39): scripts/setup-linux.sh só instala a chave do repositório
// MariaDB se o fingerprint (gpg --show-keys) for o de scripts/versoes.conf.
// Comportamento testado com chaves geradas num GNUPGHOME temporário e o script
// carregado por "source" (o main só roda quando executado diretamente).

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSEC58_VersoesConf_FingerprintFixado(t *testing.T) {
	conf := lerRaiz(t, "scripts", "versoes.conf")
	assert.Regexp(t, `(?m)^MARIADB_CHAVE_FINGERPRINT=177F4010FE56CA3336300305F1656F24C74CD1D8$`, conf,
		"fingerprint da MariaDB Signing Key (40 hex, sem espaços)")
}

func TestSEC58_SetupLinux_ConfereAntesDoInstall(t *testing.T) {
	sh := lerRaiz(t, "scripts", "setup-linux.sh")
	corpo := sh[strings.Index(sh, "configurar_repo_mariadb() {"):]
	iConf := strings.Index(corpo, `conferir_chave_mariadb "$tmp"`)
	iInst := strings.Index(corpo, "/etc/apt/keyrings/mariadb-keyring.pgp ||")
	require.GreaterOrEqual(t, iConf, 0, "configurar_repo_mariadb confere a chave")
	require.GreaterOrEqual(t, iInst, 0)
	assert.Less(t, iConf, iInst, "a conferência vem antes do install da chave")
	assert.Contains(t, sh, "--show-keys")
	assert.Contains(t, sh, `if [ "${BASH_SOURCE[0]}" = "$0" ]; then`, "main só ao executar")
}

// SecBrain (ressalva do Lote 39): o arquivo instalado é o export-minimal da
// chave conferida, nunca o baixado.
func TestSEC58_SetupLinux_InstalaOExportMinimal(t *testing.T) {
	sh := lerRaiz(t, "scripts", "setup-linux.sh")
	assert.Contains(t, sh, "--export --export-options export-minimal")
	assert.Contains(t, sh, `install -m 0644 "$chave" /etc/apt/keyrings/mariadb-keyring.pgp`)
	assert.NotContains(t, sh, `install -m 0644 "$tmp" /etc/apt/keyrings/`)
}

type sec58Chaves struct {
	dir                                       string
	a, ab, aSec, aMaisSec, revogada, expirada string
	fprA, fprB, fprRev, fprExp                string
}

// sec58GerarChaves gera as chaves de teste e os arquivos "baixados".
func sec58GerarChaves(t *testing.T) sec58Chaves {
	t.Helper()
	precisaBash(t)
	ferramentaObrigatoria(t, "gpg") // CI-02: no CI, gpg ausente falha em vez de pular
	gpg, err := exec.LookPath("gpg")
	require.NoError(t, err)
	dir := t.TempDir()
	home := filepath.Join(dir, "gnupg")
	require.NoError(t, os.Mkdir(home, 0o700))
	run := func(args ...string) string {
		cmd := exec.Command(gpg, append([]string{"--batch", "--no-tty", "--pinentry-mode", "loopback", "--passphrase", "", "--homedir", home}, args...)...)
		out, err := cmd.Output()
		require.NoError(t, err, "gpg %v", args)
		return string(out)
	}
	fpr := func(uid string) string {
		m := regexp.MustCompile(`(?m)^fpr:+([0-9A-F]{40}):`).FindStringSubmatch(run("--with-colons", "--list-keys", uid))
		require.NotNil(t, m)
		return m[1]
	}
	for _, uid := range []string{"sec58-a@teste.invalid", "sec58-b@teste.invalid", "sec58-rev@teste.invalid"} {
		run("--quick-gen-key", uid, "ed25519", "sign", "never")
	}
	// Expirada: criada em 2020 com validade de 1 dia.
	run("--faked-system-time", "20200101T000000!", "--quick-gen-key", "sec58-exp@teste.invalid", "ed25519", "sign", "1d")
	c := sec58Chaves{dir: dir, fprA: fpr("sec58-a@teste.invalid"), fprB: fpr("sec58-b@teste.invalid"),
		fprRev: fpr("sec58-rev@teste.invalid"), fprExp: fpr("sec58-exp@teste.invalid")}
	// Revogada: importa o certificado de revogação que o gpg gera junto.
	rev, err := os.ReadFile(filepath.Join(home, "openpgp-revocs.d", c.fprRev+".rev"))
	require.NoError(t, err)
	revArq := filepath.Join(dir, "rev.asc")
	escrever(t, revArq, strings.Replace(string(rev), ":-----BEGIN", "-----BEGIN", 1), 0o600)
	run("--import", revArq)

	arq := func(nome, conteudo string) string {
		p := filepath.Join(dir, nome)
		escrever(t, p, conteudo, 0o644)
		return p
	}
	pubA := run("--armor", "--export", c.fprA)
	secA := run("--armor", "--export-secret-keys", c.fprA)
	c.a = arq("a.pgp", pubA)
	c.ab = arq("ab.pgp", run("--armor", "--export", c.fprA, c.fprB))
	c.aSec = arq("a-sec.pgp", secA)
	c.aMaisSec = arq("a-mais-sec.pgp", pubA+secA)
	c.revogada = arq("rev.pgp", run("--armor", "--export", c.fprRev))
	c.expirada = arq("exp.pgp", run("--armor", "--export", c.fprExp))
	return c
}

// sec58Bash carrega o setup-linux.sh e roda o trecho com $K (chave baixada),
// $F (fingerprint) e $O (arquivo de saída do export-minimal).
func sec58Bash(t *testing.T, trecho, chave, fingerprint, saida string) (string, error) {
	t.Helper()
	script := "s=$1; K=$2; F=$3; O=$4; set --; source \"$s\"; " + trecho
	cmd := exec.Command("bash", "-c", script, "_", caminhoRaiz(t, "scripts", "setup-linux.sh"), chave, fingerprint, saida)
	cmd.Env = ambienteLimpo()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestSEC58_ConferirChave(t *testing.T) {
	c := sec58GerarChaves(t)
	divergente := c.fprA[:39] + map[bool]string{true: "1", false: "0"}[c.fprA[39] == '0']
	comEspacos := strings.ToLower(c.fprA[:20] + " " + c.fprA[20:])
	invalida := filepath.Join(c.dir, "nada.pgp")
	escrever(t, invalida, "não é uma chave\n", 0o644)

	casos := []struct {
		nome, chave, fpr string
		ok               bool
		msg              string
	}{
		{"fingerprint certo", c.a, c.fprA, true, "conferida"},
		{"minúsculas e espaços", c.a, comEspacos, true, "conferida"},
		{"divergente", c.a, divergente, false, "divergente"},
		{"de outra chave", c.a, c.fprB, false, "divergente"},
		{"vazio", c.a, "", false, "ausente ou inválido"},
		{"curto", c.a, c.fprA[:16], false, "ausente ou inválido"},
		{"duas chaves primárias", c.ab, c.fprA, false, "2 chaves primárias"},
		{"arquivo só com a secreta", c.aSec, c.fprA, false, "chave secreta"},
		{"pública certa + bloco secreto", c.aMaisSec, c.fprA, false, "chave secreta"},
		{"revogada", c.revogada, c.fprRev, false, "revogada"},
		{"expirada", c.expirada, c.fprExp, false, "expirada"},
		{"arquivo sem chave", invalida, c.fprA, false, "0 chaves primárias"},
	}
	for _, cs := range casos {
		t.Run(cs.nome, func(t *testing.T) {
			saida := filepath.Join(t.TempDir(), "min.pgp")
			out, err := sec58Bash(t, `conferir_chave_mariadb "$K" "$F" "$O"`, cs.chave, cs.fpr, saida)
			assert.Contains(t, out, cs.msg)
			if !cs.ok {
				assert.Error(t, err, out)
				return
			}
			require.NoError(t, err, out)
			min, err := os.ReadFile(saida)
			require.NoError(t, err)
			assert.NotEmpty(t, min)
			assert.NotContains(t, string(min), "-----BEGIN", "export binário do gpg, não o arquivo baixado")
			out, err = sec58Bash(t, `resumo_chaves "$O"`, "", "", saida)
			require.NoError(t, err, out)
			assert.Regexp(t, `^pub:[^:]*:`+c.fprA+`\n(sub:[^\n]*\n)*$`, out, "só a pública esperada")
		})
	}
}

// configurar_repo_mariadb inteiro com baixar/como_root falsos: recusada → nada
// instalado; certa → instala o export-minimal (não o baixado) e o .sources.
func TestSEC58_ConfigurarRepo_RecusaSemInstalar(t *testing.T) {
	c := sec58GerarChaves(t)
	trecho := `log=$(mktemp); baixar() { cp "$K" "$2"; }
como_root() { echo "ROOT $*" >>"$log"; [ "$1" = install ] && [ "${*: -1}" = /etc/apt/keyrings/mariadb-keyring.pgp ] && cp "${@: -2:1}" "$O"; return 0; }
DISTRO_ID=debian; DISTRO_LIKE=; VERSION_CODENAME=bookworm; MARIADB_SERIE=11.4; MARIADB_CHAVE_FINGERPRINT=$F
configurar_repo_mariadb; rc=$?; cat "$log"; rm -f "$log"; exit $rc`

	for nome, caso := range map[string]struct{ chave, fpr string }{
		"divergente":        {c.a, c.fprB},
		"com bloco secreto": {c.aMaisSec, c.fprA},
		"duas chaves":       {c.ab, c.fprA},
	} {
		t.Run(nome, func(t *testing.T) {
			saida := filepath.Join(t.TempDir(), "instalada.pgp")
			out, err := sec58Bash(t, trecho, caso.chave, caso.fpr, saida)
			assert.Error(t, err, out)
			assert.NotContains(t, out, "ROOT ", "nada instalado")
			assert.NoFileExists(t, saida)
		})
	}

	saida := filepath.Join(t.TempDir(), "instalada.pgp")
	out, err := sec58Bash(t, trecho, c.a, c.fprA, saida)
	require.NoError(t, err, out)
	assert.Contains(t, out, "/etc/apt/keyrings/mariadb-keyring.pgp")
	assert.Contains(t, out, "/etc/apt/sources.list.d/mariadb.sources")
	instalada, err := os.ReadFile(saida)
	require.NoError(t, err)
	baixada, err := os.ReadFile(c.a)
	require.NoError(t, err)
	assert.NotEqual(t, baixada, instalada, "instala o export-minimal, não o arquivo baixado")
	assert.NotContains(t, string(instalada), "-----BEGIN")
}
