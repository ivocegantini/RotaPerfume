//go:build linux

package deploy_test

// DEPLOY-09: ensaio do deploy/restaurar-maquina.sh, sem cópia alterada do script,
// nem válvula: user namespace (EUID 0), mount namespace e chroot numa raiz tmpfs,
// com os caminhos fixos (/root, /etc, /var, /usr/local/sbin, /usr/share/keyrings)
// apontando para diretórios do teste. O script roda de um clone git de verdade.
// Docker, compose, systemctl, curl, apt-get e os três scripts que ele chama são
// stubs. Fica provado aqui o controle do script (recusas, ordem, segredos,
// retomada), não o Docker nem o systemd reais: isso é o deploy09_real e, de ponta a
// ponta, só numa máquina real. Pula sem unshare ou sem script (util-linux).

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	repoURLD09      = "https://github.com/residentivo/RotaPerfume.git"
	ipD09           = "192.0.2.10"
	hostD09         = "maquina-d09"
	publicoD09      = "site.exemplo.test"
	carimboD09      = "20261105-033000Z"
	pepperD09       = "SENTINELA-PEPPER-D09-com-mais-de-32-bytes-0123456789"
	tokenD09        = "SENTINELAtokenD09abcdefghijklmnopqrstuvwxyz0123456789"
	jwtAntigoD09    = "SENTINELA-JWT-ANTIGO-D09-0123456789012345678901"
	senhaAntigaD09  = "SENTINELASENHAANTIGAD09012345678"
	senhaNovaD09    = "SENTINELASENHANOVAD09ABCDEF012345"
	turnstileD09    = "0x4SENTINELATURNSTILED09abcdefghijkl"
	dadoDumpD09     = "SENTINELA-DADO-DO-DUMP-D09"
	promptHostD09   = "Digite o hostname desta maquina para confirmar: "
	promptAntigaD09 = "Digite ANTIGA PARADA se a maquina antiga esta parada: "
	promptMigrD09   = "Digite MIGRACOES CONFERIDAS: "
	promptLoginD09  = "Digite LOGIN OK depois de conferir: "
	promptPainelD09 = "Digite PAINEL CONFERIDO: "
	promptTokenD09  = "TUNNEL_TOKEN (nao aparece na tela): "
	promptFimD09    = "Digite JENKINS ATUALIZADO para apagar api.env e entrada/: "
	promptDropD09   = "Digite o nome do banco (rotaperfumes) para apagar e recriar: "
)

// Chave OpenPGP pública descartável, para o caso da chave errada. Sem chave privada
// guardada em lugar nenhum. Fingerprint 93946AEDD37213FE9B65DE10DB888270E83108BF.
const chaveErradaD09 = `-----BEGIN PGP PUBLIC KEY BLOCK-----

mDMEau5AjxYJKwYBBAHaRw8BAQdAVcnfVbjXLHw+OmJpQAZqHo9BbYkMUyBczk5t
B3AAwgi0KEZpeHR1cmUgRXJyYWRhIEQwOSA8ZDA5QGV4YW1wbGUuaW52YWxpZD6I
kwQTFgoAOxYhBJOUau3TchP+m2XeENuIgnDoMQi/BQJq7kCPAhsDBQsJCAcCAiIC
BhUKCQgLAgQWAgMBAh4HAheAAAoJENuIgnDoMQi/n5sBAM2iSbSzgyRcG811Am3z
/vkYQ9dEk5Hq6kNYgEiGD4oVAP4ykQmJAoG3+wcZ2VZDdZQ+U4mSS6UPhbx0A8Tz
5eZcCA==
=Mib/
-----END PGP PUBLIC KEY BLOCK-----
`

const runnerD09 = `#!/bin/bash
set -euo pipefail
umask 022
NR="$FAKE/.raiz"
mkdir -p "$NR"
mount -t tmpfs -o mode=755 tmpfs "$NR"
topo=${FAKE#/}; topo=/${topo%%/*}
for x in /*; do
  case "$x" in "$topo"|/etc|/var|/root) continue ;; esac
  if [[ -L "$x" ]]; then ln -s "$(readlink "$x")" "$NR$x"
  elif [[ -d "$x" ]]; then mkdir "$NR$x"; mount --rbind "$x" "$NR$x"
  fi
done
mkdir -p "$NR$topo" "$NR$FAKE" "$NR/etc" "$NR/var" "$NR/root"
mount --bind "$FAKE" "$NR$FAKE"
mount --bind "$FAKE/fs/etc" "$NR/etc"
mount --bind "$FAKE/fs/var" "$NR/var"
mount --bind "$FAKE/fs/root" "$NR/root"
mount --bind "$FAKE/fs/sbin" "$NR/usr/local/sbin"
mount --bind "$FAKE/fs/keyrings" "$NR/usr/share/keyrings"
# Um arquivo do clone com outro dono, de verdade: /etc/hostname do host é do root
# real, que no namespace aparece como o overflow uid.
if [[ -f "$FAKE/state/bind_estranho" ]]; then
  mount --bind /etc/hostname "$NR$FAKE/clone/$(<"$FAKE/state/bind_estranho")"
fi
if [[ -f "$FAKE/state/sem_apt" ]]; then
  # PATH sem o apt-get do host: o resto dos comandos, por links.
  mkdir -p "$FAKE/sem-apt"
  for x in /usr/bin/* /usr/sbin/*; do
    [[ "${x##*/}" == apt-get ]] || ln -sf "$x" "$FAKE/sem-apt/${x##*/}"
  done
  export PATH="$FAKE/bin:$FAKE/sem-apt" HOME=/root
else
  export PATH="$FAKE/bin:$PATH" HOME=/root
fi
umask 077
exec chroot "$NR" /bin/bash -c 'cd / && exec bash "$0" "$@"' "$SCRIPT" "$@"
`

const stubDockerD09 = `#!/bin/bash
S="$FAKE/state"
echo "docker $*" >>"$FAKE/log"
a="$*"
case "$1" in
  info) exit 0 ;;
  compose)
    case " $a " in
      *" version "*) exit 0 ;;
      *" --no-start "*) touch "$S/volume" "$S/projeto"; env >>"$S/compose_env"; exit 0 ;;
      *" up -d "*) touch "$S/compose_up"; env >>"$S/compose_env"; exit 0 ;;
    esac
    env >>"$S/compose_env"; exit 0 ;;
  ps)
    [[ "$a" == *"name=^rotaperfumes-mariadb"* && -f "$S/sinal_container" ]] && echo c1
    [[ "$a" == *"label=com.docker.compose.project=rotaperfumes"* && ( -f "$S/sinal_compose" || -f "$S/projeto" ) ]] && echo c2
    exit 0 ;;
  volume)
    [[ -f "$S/volume" || -f "$S/volume_mp" ]] || exit 1
    case "$a" in
      *Mountpoint*) cat "$S/volume_mp" ;;
      *Labels*) cat "$S/volume_rotulo" 2>/dev/null || echo rotaperfumes/produto_imagens ;;
    esac
    exit 0 ;;
  inspect) echo true; exit 0 ;;
  exec)
    case "$a" in
      *information_schema.TABLES*) if [[ -s "$S/tabelas" ]]; then grep -c . "$S/tabelas"; else echo 0; fi; exit 0 ;;
      *"SELECT 1 FROM refresh_tokens"*) exit 0 ;;
      *"SELECT COUNT(*) FROM"*)
        t=$(sed -E 's/.*FROM .([a-z_]+).*/\1/' <<<"$a")
        awk -v t="$t" '$1 == t { print $2 }' "$S/tabelas"
        exit 0 ;;
    esac
    exit 0 ;;
  run)
    case "$a" in
      *rotaperfumes-dba-cliente*)
        echo "$a" >"$S/import_argv"
        env >"$S/import_env"
        cat >"$S/importado"
        if [[ -f "$S/import_falha" ]]; then
          rm -f "$S/import_falha"
          printf 'usuarios 1\n' >"$S/tabelas"
          printf "ERROR 1062 (23000) at line 7: Duplicate entry 'SENTINELA-DADO-DO-DUMP-D09' for key\n" >&2
          exit 1
        fi
        cp "$S/tabelas_dump" "$S/tabelas"
        exit 0 ;;
      *"--entrypoint bash"*" -s"*)
        env >"$S/validador_env"
        cat >"$S/validador_stdin"
        printf 'abababababababababababababababababababababababababababababababab\n'
        exit 0 ;;
      *"tar -xf"*) echo "$a" >"$S/extracao_argv"; cat >"$S/extraido.tar"; exit 0 ;;
      *"-mindepth 1"*) cat "$S/volume_dono" 2>/dev/null || echo 65532:65532; cat "$S/volume_itens" 2>/dev/null || echo 0; exit 0 ;;
      *"-type f"*) cat "$S/volume_arquivos" 2>/dev/null || echo 2; exit 0 ;;
    esac
    exit 0 ;;
esac
exit 0
`

const stubSystemctlD09 = `#!/bin/bash
S="$FAKE/state"
echo "systemctl $*" >>"$FAKE/log"
case "$1" in
  cat) [[ -f "$S/unit_$3" ]]; exit $? ;;
  show)
    if [[ "$*" == *MainPID* ]]; then cat "$S/mainpid" 2>/dev/null || echo 1; exit 0; fi
    if [[ -f "$S/show_vaza" ]]; then sed -E 's/^/Environment=/' /etc/rotaperfumes/cloudflared.env; fi
    echo "ExecStart=/usr/bin/cloudflared tunnel --no-autoupdate run"
    exit 0 ;;
  enable)
    [[ -f "$S/unit_falha" ]] && exit 1
    if [[ -f "$S/cmdline_vaza" ]]; then
      (exec -a "$(sed 's/.*=//' /etc/rotaperfumes/cloudflared.env)" sleep 15) >/dev/null 2>&1 </dev/null &
      echo $! >"$S/mainpid"
      sleep 0.2
    fi
    exit 0 ;;
esac
exit 0
`

const stubJournalD09 = `#!/bin/bash
echo "journalctl $*" >>"$FAKE/log"
echo "cloudflared iniciou"
if [[ -f "$FAKE/state/journal_vaza" ]]; then sed 's/^/log: /' /etc/rotaperfumes/cloudflared.env; fi
exit 0
`

const stubAptD09 = `#!/bin/bash
echo "apt-get $*" >>"$FAKE/log"
case "$1" in
  install) printf '#!/bin/bash\nexit 0\n' >"$FAKE/bin/cloudflared"; chmod 755 "$FAKE/bin/cloudflared" ;;
esac
exit 0
`

const stubCurlD09 = `#!/bin/bash
echo "curl $*" >>"$FAKE/log"
[[ -f "$FAKE/state/curl_falha" ]] && exit 22
# Lote 42: tunel sem conexao (/ready) ou origem barrada pelo filtro da unit.
[[ -f "$FAKE/state/tunel_sem_ready" && "$*" == *127.0.0.1:20241/ready* ]] && exit 22
[[ -f "$FAKE/state/origem_barrada" && -n "${RP_SYSTEMD_RUN:-}" ]] && exit 7
[[ -f "$FAKE/state/publico_falha" && "$*" == *site.exemplo.test* ]] && exit 22
printf 200
`

// Lote 42: systemd-run transitorio da etapa 11; registra as propriedades e roda
// o comando (o curl falso), marcando que veio de dentro da unit transitoria.
const stubSystemdRunD09 = `#!/bin/bash
echo "systemd-run $*" >>"$FAKE/log"
while [[ $# -gt 0 && "$1" != -- ]]; do shift; done
shift
RP_SYSTEMD_RUN=1 exec "$@"
`

const stubVerificarD09 = `#!/bin/bash
echo "verificar-backup.sh $*" >>"$FAKE/log"
[[ -f "$FAKE/state/verificar_reprova" ]] && { echo "REPROVADO: teste" >&2; exit 1; }
printf 'Tabelas importadas: 2\n  usuarios    3\n  refresh_tokens    5\nAdmins em usuarios: 1\n'
[[ -f "$FAKE/state/verificar_sem_aprovado" ]] || echo APROVADO
`

const stubBackupD09 = `#!/bin/bash
echo "backup-banco.sh $*" >>"$FAKE/log"
if [[ "$1" == --conferir-imagens ]]; then
  # Com a contagem na saída: só o código e o APROVADO podem barrar.
  [[ -f "$FAKE/state/imagens_reprova" ]] && { printf 'Arquivos: 2\nBytes: 10\n'; exit 1; }
  printf 'Arquivos: 2\nBytes: 10\nAPROVADO\n'
  exit 0
fi
printf '#!/bin/bash\necho "rotaperfumes-backup-banco $*" >>"$FAKE/log"\n' >/usr/local/sbin/rotaperfumes-backup-banco
chmod 700 /usr/local/sbin/rotaperfumes-backup-banco
`

const stubMariadbD09 = `#!/bin/bash
echo "mariadb-docker.sh $*" >>"$FAKE/log"
mkdir -p /etc/mysql/ssl /root/rotaperfumes-mariadb/dba-cliente
touch /etc/mysql/ssl/server-key.pem
[[ -f /etc/mysql/ssl/ca.pem ]] || cp "$FAKE/in/ca-gerada.pem" /etc/mysql/ssl/ca.pem
cp "$FAKE/in/senha-nova" /root/rotaperfumes-mariadb/rotaperfumes_app.senha
printf '[client]\nuser=rotaperfumes-admin\npassword=x\n' >/root/rotaperfumes-mariadb/dba-cliente/cliente.cnf
cat >/usr/local/sbin/rotaperfumes-dba <<'EOS'
#!/bin/bash
echo "rotaperfumes-dba" >>"$FAKE/log"
e=$(cat)
printf '%s\n' "$e" >>"$FAKE/state/dba_stdin"
[[ "$e" == *"DROP DATABASE"* ]] && : >"$FAKE/state/tabelas"
exit 0
EOS
chmod 700 /usr/local/sbin/rotaperfumes-dba
if [[ -f "$FAKE/state/tabelas_pre" ]]; then cp "$FAKE/state/tabelas_pre" "$FAKE/state/tabelas"; fi
`

// caFixture: uma CA EC, gerada uma vez por execução do pacote (é só um fixture).
var (
	caFixtureOnce sync.Once
	caFixtureCert string
	caFixtureKey  string
	caFixtureFP   string
	caFixtureErr  error
)

func caFixture(t *testing.T) (string, string, string) {
	t.Helper()
	caFixtureOnce.Do(func() {
		dir, err := os.MkdirTemp("", "rp-d09-ca-*")
		if err != nil {
			caFixtureErr = err
			return
		}
		defer os.RemoveAll(dir)
		k, c := filepath.Join(dir, "k.pem"), filepath.Join(dir, "c.pem")
		out, err := exec.Command("openssl", "req", "-x509", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:prime256v1", "-nodes",
			"-keyout", k, "-out", c, "-days", "2", "-subj", "/CN=rp-d09").CombinedOutput()
		if err != nil {
			caFixtureErr = err
			caFixtureCert = string(out)
			return
		}
		fp, err := exec.Command("sh", "-c", "openssl x509 -in '"+c+"' -noout -fingerprint -sha256 | cut -d= -f2 | tr -d : | tr A-F a-f").Output()
		if err != nil {
			caFixtureErr = err
			return
		}
		bc, _ := os.ReadFile(c)
		bk, _ := os.ReadFile(k)
		caFixtureCert, caFixtureKey, caFixtureFP = string(bc), string(bk), strings.TrimSpace(string(fp))
	})
	require.NoError(t, caFixtureErr, caFixtureCert)
	return caFixtureCert, caFixtureKey, caFixtureFP
}

type ensaioD09 struct {
	t   *testing.T
	dir string
}

func precisaEnsaioD09(t *testing.T) {
	t.Helper()
	precisaBash(t)
	precisaScript(t)
	for _, f := range []string{"git", "openssl", "gpg", "shred", "flock"} {
		if _, err := exec.LookPath(f); err != nil {
			t.Skip(f + " não está no PATH")
		}
	}
	if st, err := os.Stat("/usr/share/keyrings"); err != nil || !st.IsDir() {
		t.Skip("/usr/share/keyrings não existe neste host")
	}
	if err := exec.Command("unshare", "-r", "-m", "mount", "-t", "tmpfs", "tmpfs", "/mnt").Run(); err != nil {
		t.Skip("sem user+mount namespace: " + err.Error())
	}
}

func (e *ensaioD09) git(args ...string) string {
	e.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", filepath.Join(e.dir, "clone"), "-c", "user.name=t", "-c", "user.email=t@t.invalid"}, args...)...)
	cmd.Env = append(ambienteLimpo(), "HOME="+e.dir, "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	require.NoError(e.t, err, "%s", out)
	// O git do teste herda o umask do go test: a árvore fica como a de um clone do root.
	require.NoError(e.t, exec.Command("chmod", "-R", "go-w", filepath.Join(e.dir, "clone")).Run())
	return strings.TrimSpace(string(out))
}

func (e *ensaioD09) estado(nome, conteudo string) {
	escrever(e.t, filepath.Join(e.dir, "state", nome), conteudo, 0o644)
}

func (e *ensaioD09) ler(rel string) string {
	b, err := os.ReadFile(filepath.Join(e.dir, rel))
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(e.t, err)
	return string(b)
}

func (e *ensaioD09) existe(rel string) bool {
	_, err := os.Lstat(filepath.Join(e.dir, rel))
	return err == nil
}

func (e *ensaioD09) clone(rel, conteudo string, modo os.FileMode) {
	escrever(e.t, filepath.Join(e.dir, "clone", rel), conteudo, modo)
}

// recommit: depois de mudar o clone, ele fica limpo e igual à main remota (stub).
func (e *ensaioD09) recommit() {
	e.t.Helper()
	e.git("add", "-A")
	e.git("commit", "-qm", "fixture", "--allow-empty")
	e.estado("remote_head", e.git("rev-parse", "HEAD")+"\n")
}

func gzipD09(t *testing.T, dados string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	_, err := z.Write([]byte(dados))
	require.NoError(t, err)
	require.NoError(t, z.Close())
	return b.Bytes()
}

func tarGzD09(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	z := gzip.NewWriter(&b)
	w := tar.NewWriter(z)
	for _, n := range []string{"a.jpg", "b.jpg"} {
		require.NoError(t, w.WriteHeader(&tar.Header{Name: "./" + n, Mode: 0o640, Size: 1, Typeflag: tar.TypeReg}))
		_, err := w.Write([]byte("x"))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	require.NoError(t, z.Close())
	return b.Bytes()
}

func apiEnvD09() string {
	return "# copia do Secret file\nDB_HOST=rotaperfumes-mariadb\nDB_NAME=rotaperfumes\nDB_USUARIO=rotaperfumes_app\n" +
		"DB_SENHA=" + senhaAntigaD09 + "\nJWT_SECRET=" + jwtAntigoD09 + "\nPASSWORD_PEPPER=" + pepperD09 + "\n" +
		"COOKIE_SECURE=true\nCORS_ALLOWED_ORIGINS=https://" + publicoD09 + ",https://" + hostD09 + ":8443,https://" + ipD09 + ":8443\n" +
		"SMTP_PASSWORD='a$b'\nTURNSTILE_SECRET_KEY=" + turnstileD09 + "\nTURNSTILE_HOSTNAMES=" + publicoD09 + "," + hostD09 + "\n"
}

func novoEnsaioD09(t *testing.T) *ensaioD09 {
	t.Helper()
	precisaEnsaioD09(t)
	// Nome só ASCII: o script recusa caminhos estranhos, e o nome do subteste vira caminho no t.TempDir.
	dir, err := os.MkdirTemp("", "rp-d09-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = exec.Command("chmod", "-R", "u+rwx", dir).Run()
		_ = os.RemoveAll(dir)
	})
	e := &ensaioD09{t: t, dir: dir}
	require.NoError(t, os.Chmod(e.dir, 0o700))
	for _, d := range []string{"bin", "state", "in", "clone", "fs/root", "fs/sbin", "fs/keyrings", "fs/var/lib", "fs/var/backups",
		"fs/etc/systemd/system", "fs/etc/apt/sources.list.d", "fs/etc/apt/preferences.d"} {
		require.NoError(t, os.MkdirAll(filepath.Join(e.dir, d), 0o755))
	}
	require.NoError(t, os.Chmod(filepath.Join(e.dir, "fs/root"), 0o700))
	copia := exec.Command("bash", "-c", `for f in passwd group nsswitch.conf ld.so.cache ld.so.conf alternatives ssl; do [ -e "/etc/$f" ] && cp -aL "/etc/$f" "$1/" 2>/dev/null; done; exit 0`, "_", filepath.Join(e.dir, "fs/etc"))
	out, err := copia.CombinedOutput()
	require.NoError(t, err, "%s", out)
	escrever(t, filepath.Join(e.dir, "fs/etc/machine-id"), "0123456789abcdef0123456789abcdef\n", 0o644)

	gitReal, err := exec.LookPath("git")
	require.NoError(t, err)
	bins := map[string]string{
		"docker": stubDockerD09, "systemctl": stubSystemctlD09, "journalctl": stubJournalD09,
		"apt-get": stubAptD09, "curl": stubCurlD09, "systemd-run": stubSystemdRunD09,
		"git":      "#!/bin/bash\nif [[ \"$*\" == *ls-remote* ]]; then echo \"git $*\" >>\"$FAKE/log\"; printf '%s\\trefs/heads/main\\n' \"$(cat \"$FAKE/state/remote_head\")\"; exit 0; fi\nexec " + gitReal + " \"$@\"\n",
		"ip":       "#!/bin/bash\necho '2: eth0    inet " + ipD09 + "/24 brd 192.0.2.255 scope global eth0'\n",
		"hostname": "#!/bin/bash\necho " + hostD09 + "\n",
		"df":       "#!/bin/bash\nif [[ -f \"$FAKE/state/df_pouco\" ]]; then printf 'Avail\\n1000\\n'; else printf 'Avail\\n99999999999999\\n'; fi\n",
	}
	for nome, conteudo := range bins {
		escrever(t, filepath.Join(e.dir, "bin", nome), conteudo, 0o755)
	}
	escrever(t, filepath.Join(e.dir, "runner.sh"), runnerD09, 0o755)

	// O clone: o script e a chave de verdade, o resto como stub.
	real := lerRaiz(t, "deploy", "restaurar-maquina.sh")
	cert, key, fp := caFixture(t)
	require.NoError(t, exec.Command("sh", "-c", "umask 022; git init -q -b main '"+filepath.Join(e.dir, "clone")+"'").Run())
	e.clone("deploy/restaurar-maquina.sh", real, 0o755)
	e.clone("deploy/cloudflare-apt.asc", lerRaiz(t, "deploy", "cloudflare-apt.asc"), 0o644)
	e.clone("deploy/cloudflare-apt.fingerprint", lerRaiz(t, "deploy", "cloudflare-apt.fingerprint"), 0o644)
	e.clone("deploy/db-ca.fingerprint", "# teste\n"+fp+"\n", 0o644)
	e.clone("deploy/docker-compose.yml", "name: rotaperfumes\n", 0o644)
	e.clone("deploy/validar-ca-banco.sh", "#!/bin/bash\n# validador\n", 0o755)
	e.clone("deploy/verificar-backup.sh", stubVerificarD09, 0o755)
	e.clone("deploy/backup-banco.sh", stubBackupD09, 0o755)
	e.clone("deploy/mariadb-docker.sh", stubMariadbD09, 0o755)
	e.clone("Jenkinsfile", "string(name: 'TURNSTILE_SITE_KEY', defaultValue: '0x4AAAAAAFACjc-LtM9NWlvP', description: 'x')\n", 0o644)
	e.clone("sql/08_alter_x.sql", "SELECT 1;\n", 0o644)
	e.clone("sql/18_revert_x.sql", "SELECT 2;\n", 0o644)
	e.clone(".gitignore", "*.ignorado\n", 0o644)
	e.recommit()
	e.git("config", "remote.origin.url", repoURLD09)

	// Entradas.
	in := filepath.Join(e.dir, "in")
	escrever(t, filepath.Join(in, "rotaperfumes-"+carimboD09+".sql.gz"), string(gzipD09(t, "CREATE TABLE x;\nINSERT "+dadoDumpD09+";\n-- Dump completed\n")), 0o644)
	escrever(t, filepath.Join(in, "rotaperfumes-imagens-"+carimboD09+".tar.gz"), string(tarGzD09(t)), 0o644)
	escrever(t, filepath.Join(in, "api.env"), apiEnvD09(), 0o600)
	escrever(t, filepath.Join(in, "senha-nova"), senhaNovaD09, 0o600)
	escrever(t, filepath.Join(in, "pepper"), pepperD09+"\n", 0o600)
	escrever(t, filepath.Join(in, "token"), tokenD09+"\n", 0o600)
	escrever(t, filepath.Join(in, "ca-gerada.pem"), cert, 0o644)
	escrever(t, filepath.Join(in, "ca.pem"), cert, 0o644)
	escrever(t, filepath.Join(in, "ca-key.pem"), key, 0o600)

	e.estado("tabelas_dump", "usuarios 3\nrefresh_tokens 5\n")
	return e
}

func (e *ensaioD09) in(nome string) string { return filepath.Join(e.dir, "in", nome) }

func (e *ensaioD09) dump() string { return e.in("rotaperfumes-" + carimboD09 + ".sql.gz") }

func (e *ensaioD09) imagens() string { return e.in("rotaperfumes-imagens-" + carimboD09 + ".tar.gz") }

// argsBase: opções válidas, com --nova-ca e --pepper-file, sem túnel escolhido.
func (e *ensaioD09) argsBase() []string {
	return []string{"--dump", e.dump(), "--imagens", e.imagens(), "--api-env", e.in("api.env"),
		"--ip", ipD09, "--host-nome", hostD09, "--hostname-publico", publicoD09,
		"--nova-ca", "--pepper-file", e.in("pepper")}
}

func (e *ensaioD09) argsTrocando(de, para string) []string {
	a := e.argsBase()
	for i, v := range a {
		if v == de {
			a[i] = para
		}
	}
	return a
}

func (e *ensaioD09) comando(extraEnv []string, args ...string) *exec.Cmd {
	interno := append([]string{"-r", "-m", "bash", filepath.Join(e.dir, "runner.sh")}, args...)
	cmd := exec.Command("unshare", interno...)
	var env []string
	for _, kv := range ambienteLimpo() {
		if strings.HasPrefix(kv, "TMPDIR=") || strings.HasPrefix(kv, "JENKINS_HOME=") || strings.HasPrefix(kv, "BUILD_ID=") ||
			strings.HasPrefix(kv, "WORKSPACE=") || strings.HasPrefix(kv, "SHELLOPTS=") {
			continue
		}
		env = append(env, kv)
	}
	cmd.Env = append(append(env, "FAKE="+e.dir, "SCRIPT="+filepath.Join(e.dir, "clone", "deploy", "restaurar-maquina.sh")), extraEnv...)
	return cmd
}

// tty inicia o script com terminal real (pty do script -qec).
func (e *ensaioD09) tty(extraEnv []string, args ...string) *sessaoTTY {
	e.t.Helper()
	interno := e.comando(extraEnv, args...)
	partes := make([]string, 0, len(interno.Args))
	for _, a := range interno.Args {
		partes = append(partes, aspasShell(a))
	}
	s := &sessaoTTY{t: e.t, fim: make(chan error, 1)}
	s.cmd = exec.Command("script", "-qec", strings.Join(partes, " "), "/dev/null")
	s.cmd.Env = append(interno.Env, "SHELL=/bin/bash", "TERM=dumb")
	s.cmd.Stdout = escritorTTY{s}
	s.cmd.Stderr = escritorTTY{s}
	var err error
	s.stdin, err = s.cmd.StdinPipe()
	require.NoError(e.t, err)
	require.NoError(e.t, s.cmd.Start())
	go func() { s.fim <- s.cmd.Wait() }()
	e.t.Cleanup(func() {
		_ = s.stdin.Close()
		if s.cmd.ProcessState == nil {
			_ = s.cmd.Process.Kill()
		}
	})
	return s
}

// recusa: roda com terminal, espera o erro e confere que nada foi mudado.
func (e *ensaioD09) recusa(trecho string, args ...string) string {
	e.t.Helper()
	return e.recusaEnv(nil, trecho, args...)
}

func (e *ensaioD09) recusaEnv(env []string, trecho string, args ...string) string {
	e.t.Helper()
	out, err := e.tty(env, args...).terminar()
	require.Error(e.t, err, out)
	assert.Contains(e.t, out, trecho)
	e.semMutacao(out)
	return out
}

var somenteLeituraD09 = regexp.MustCompile(`^docker (info|ps |compose .*version|volume inspect )`)

// semMutacao: só leitura no Docker, nada no apt nem no systemd, nenhum script
// chamado, /root vazio, nenhum segredo na saída.
func (e *ensaioD09) semMutacao(out string) {
	e.t.Helper()
	for _, l := range linhas(e.ler("log")) {
		switch {
		case strings.HasPrefix(l, "docker "):
			assert.Regexp(e.t, somenteLeituraD09, l, "chamada mutável ao Docker")
		case strings.HasPrefix(l, "systemctl "):
			assert.True(e.t, strings.HasPrefix(l, "systemctl cat "), l)
		case strings.HasPrefix(l, "git "):
		default:
			assert.Fail(e.t, "chamada fora das recusas", l)
		}
	}
	assert.False(e.t, e.existe("fs/root/rotaperfumes-restauracao"), "nem o diretório de trabalho")
	entradas, err := os.ReadDir(filepath.Join(e.dir, "fs/root"))
	require.NoError(e.t, err)
	assert.Empty(e.t, entradas, "/root limpo, inclusive do temporário do gpg")
	e.semSegredos(out)
}

func (e *ensaioD09) semSegredos(saidas ...string) {
	e.t.Helper()
	textos := append([]string{e.ler("log"), e.ler("state/import_argv"), e.ler("state/import_env"), e.ler("state/compose_env"),
		e.ler("state/validador_env")}, saidas...)
	for nome, v := range map[string]string{"pepper": pepperD09, "token": tokenD09, "jwt antigo": jwtAntigoD09, "senha antiga": senhaAntigaD09,
		"senha nova": senhaNovaD09, "turnstile": turnstileD09, "dado do dump": dadoDumpD09} {
		for _, texto := range textos {
			assert.NotContains(e.t, texto, v, "%s vazou", nome)
		}
	}
}

// ateImportacao digita até a importação, sem túnel.
func (e *ensaioD09) confirmarInicio(s *sessaoTTY, comTunel bool) {
	e.t.Helper()
	s.digitar(promptHostD09, hostD09)
	if comTunel {
		s.digitar(promptAntigaD09, "ANTIGA PARADA")
	}
}

func (e *ensaioD09) fluxoSemTunel() (string, error) {
	e.t.Helper()
	s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
	e.confirmarInicio(s, false)
	s.digitar(promptMigrD09, "MIGRACOES CONFERIDAS")
	s.digitar(promptLoginD09, "LOGIN OK")
	s.digitar(promptFimD09, "JENKINS ATUALIZADO")
	return s.terminar()
}

func (e *ensaioD09) posicao(trecho string) int {
	e.t.Helper()
	i := strings.Index(e.ler("log"), trecho)
	require.GreaterOrEqual(e.t, i, 0, "%q fora do log:\n%s", trecho, e.ler("log"))
	return i
}

func (e *ensaioD09) antes(a, b string) {
	e.t.Helper()
	assert.Less(e.t, e.posicao(a), e.posicao(b), "%q antes de %q", a, b)
}
