//go:build linux

package deploy_test

// DEPLOY-09: recusas do deploy/restaurar-maquina.sh. Em cada caso, nenhuma chamada
// mutável ao Docker, nada no apt ou no systemd e /root intacto (semMutacao).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDEPLOY09_Recusa_SinaisDeProducao(t *testing.T) {
	casos := []struct {
		nome    string
		preparo func(e *ensaioD09)
		sinal   string
	}{
		{"container do banco", func(e *ensaioD09) { e.estado("sinal_container", "") }, "container rotaperfumes-mariadb"},
		{"container do compose", func(e *ensaioD09) { e.estado("sinal_compose", "") }, "container do compose rotaperfumes"},
		{"datadir", func(e *ensaioD09) {
			require.NoError(t, os.MkdirAll(filepath.Join(e.dir, "fs/var/lib/rotaperfumes/mariadb/mysql"), 0o755))
		}, "/var/lib/rotaperfumes/mariadb/mysql"},
		{"chave do servidor", func(e *ensaioD09) {
			escrever(t, filepath.Join(e.dir, "fs/etc/mysql/ssl/server-key.pem"), "x", 0o600)
		}, "/etc/mysql/ssl/server-key.pem"},
		{"unit do backup", func(e *ensaioD09) {
			escrever(t, filepath.Join(e.dir, "fs/etc/systemd/system/rotaperfumes-backup-banco.service"), "x", 0o644)
		}, "rotaperfumes-backup-banco.service"},
		{"timer do backup", func(e *ensaioD09) {
			escrever(t, filepath.Join(e.dir, "fs/etc/systemd/system/rotaperfumes-backup-banco.timer"), "x", 0o644)
		}, "rotaperfumes-backup-banco.timer"},
		{"binário do backup", func(e *ensaioD09) {
			escrever(t, filepath.Join(e.dir, "fs/sbin/rotaperfumes-backup-banco"), "x", 0o700)
		}, "/usr/local/sbin/rotaperfumes-backup-banco"},
		{"dump em /var/backups", func(e *ensaioD09) {
			escrever(t, filepath.Join(e.dir, "fs/var/backups/rotaperfumes/rotaperfumes-20260101-000000Z.sql.gz"), "x", 0o600)
		}, "backup em /var/backups/rotaperfumes"},
		{"imagens em /var/backups", func(e *ensaioD09) {
			escrever(t, filepath.Join(e.dir, "fs/var/backups/rotaperfumes/rotaperfumes-imagens-20260101-000000Z.tar.gz"), "x", 0o600)
		}, "backup em /var/backups/rotaperfumes"},
		{"cloudflared.service pelo systemctl", func(e *ensaioD09) { e.estado("unit_cloudflared.service", "") }, "unit cloudflared.service"},
		{"cloudflared.service em arquivo", func(e *ensaioD09) {
			escrever(t, filepath.Join(e.dir, "fs/etc/systemd/system/cloudflared.service"), "x", 0o644)
		}, "unit cloudflared.service"},
		{"unit própria do túnel", func(e *ensaioD09) { e.estado("unit_rotaperfumes-cloudflared.service", "") }, "unit rotaperfumes-cloudflared.service"},
		{"volume de imagens com conteúdo", func(e *ensaioD09) {
			mp := filepath.Join(e.dir, "vol")
			escrever(t, filepath.Join(mp, "a.jpg"), "x", 0o644)
			e.estado("volume_mp", mp+"\n")
		}, "volume rotaperfumes_produto_imagens com conteudo"},
		{"volume sem conferência possível", func(e *ensaioD09) {
			e.estado("volume_mp", filepath.Join(e.dir, "nao-existe")+"\n")
		}, "conteudo nao conferido"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioD09(t)
			c.preparo(e)
			out := e.recusa("parece um servidor em uso", e.argsBase()...)
			assert.Contains(t, out, c.sinal)
			assert.NotContains(t, out, "=== [1/12]")
		})
	}

	// Uma válvula por variável seria a regressão mais perigosa.
	t.Run("variáveis de ambiente não dispensam", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.estado("sinal_container", "")
		e.recusaEnv([]string{"FORCAR=1", "FORCE=1", "RP_TESTE=1", "TEST=1", "SEM_RECUSAS=1", "CONFIRMO_PRODUCAO=x", "CI=true"},
			"parece um servidor em uso", e.argsBase()...)
	})
	t.Run("opção de força não existe", func(t *testing.T) {
		e := novoEnsaioD09(t)
		for _, o := range []string{"--forcar", "--force", "--yes", "--recriar"} {
			out, err := e.tty(nil, append(e.argsBase(), o)...).terminar()
			require.Error(t, err)
			assert.Contains(t, out, "opcao desconhecida: "+o)
		}
	})
	t.Run("volume vazio não é sinal", func(t *testing.T) {
		e := novoEnsaioD09(t)
		mp := filepath.Join(e.dir, "vol")
		require.NoError(t, os.MkdirAll(mp, 0o755))
		e.estado("volume_mp", mp+"\n")
		s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
		s.esperar(promptHostD09)
	})
}

func TestDEPLOY09_Recusa_Execucao(t *testing.T) {
	t.Run("sem terminal", func(t *testing.T) {
		e := novoEnsaioD09(t)
		cmd := e.comando(nil, e.argsBase()...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		out, err := cmd.CombinedOutput()
		require.Error(t, err, string(out))
		assert.Contains(t, string(out), "sem terminal")
		assert.Empty(t, e.ler("log"), "nem o Docker é consultado")
	})
	t.Run("sem root", func(t *testing.T) {
		precisaBash(t)
		cmd := exec.Command("bash", caminhoRaiz(t, "deploy", "restaurar-maquina.sh"), "--dump", "a", "--imagens", "b", "--api-env", "c",
			"--ip", ipD09, "--host-nome", hostD09, "--hostname-publico", publicoD09, "--nova-ca")
		cmd.Env = ambienteLimpo()
		if os.Geteuid() == 0 {
			t.Skip("este teste precisa de um usuário comum")
		}
		out, err := cmd.CombinedOutput()
		require.Error(t, err)
		assert.Contains(t, string(out), "rode como root")
	})
	for _, v := range []string{"JENKINS_HOME=/var/lib/jenkins", "BUILD_ID=1", "WORKSPACE=/x", "BUILD_ID="} {
		t.Run("variável "+v, func(t *testing.T) {
			e := novoEnsaioD09(t)
			e.recusaEnv([]string{v}, "dentro do Jenkins", e.argsBase()...)
		})
	}
	t.Run("xtrace", func(t *testing.T) {
		e := novoEnsaioD09(t)
		out, err := e.tty([]string{"SHELLOPTS=xtrace"}, e.argsBase()...).terminar()
		require.Error(t, err, out)
		assert.Contains(t, out, "rode sem xtrace")
		e.semSegredos(out)
	})
}

func TestDEPLOY09_Recusa_Clone(t *testing.T) {
	casos := []struct {
		nome, trecho string
		preparo      func(e *ensaioD09)
	}{
		{"origin diferente", "origin diferente", func(e *ensaioD09) { e.git("config", "remote.origin.url", "https://github.com/outro/repo.git") }},
		{"HEAD diferente da main", "diferente da main de origin", func(e *ensaioD09) { e.estado("remote_head", strings.Repeat("a", 40)+"\n") }},
		{"arquivo não rastreado", "nao esta limpo", func(e *ensaioD09) { e.clone("novo.txt", "x", 0o644) }},
		{"arquivo rastreado alterado", "nao esta limpo", func(e *ensaioD09) {
			e.clone("Jenkinsfile", "alterado\nstring(name: 'TURNSTILE_SITE_KEY', defaultValue: '0x4AAAAAAFACjc-LtM9NWlvP')\n", 0o644)
		}},
		{"ignorado presente", "nao esta limpo", func(e *ensaioD09) { e.clone("deploy/api.ignorado", "x", 0o644) }},
		{"escrita de grupo no clone", "escrita de grupo/outros", func(e *ensaioD09) {
			require.NoError(t, os.Chmod(filepath.Join(e.dir, "clone", "deploy", "docker-compose.yml"), 0o664))
		}},
		{"dono errado", "que nao e do root", func(e *ensaioD09) { e.estado("bind_estranho", "deploy/docker-compose.yml") }},
		{"link simbólico na raiz", "link simbolico na raiz", func(e *ensaioD09) {
			require.NoError(t, os.Symlink("deploy", filepath.Join(e.dir, "clone", "atalho")))
		}},
		{"ancestral com escrita", "tem escrita de grupo/outros", func(e *ensaioD09) {
			require.NoError(t, os.Chmod(e.dir, 0o770))
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioD09(t)
			c.preparo(e)
			e.recusa(c.trecho, e.argsBase()...)
		})
	}
}

func TestDEPLOY09_Recusa_IP(t *testing.T) {
	e := novoEnsaioD09(t)
	escrever(t, filepath.Join(e.dir, "bin", "ip"), "#!/bin/bash\necho '2: eth0    inet 198.51.100.7/24 scope global eth0'\n", 0o755)
	e.recusa("nao esta configurado nesta maquina", e.argsBase()...)
}

func TestDEPLOY09_Recusa_Entradas(t *testing.T) {
	copiar := func(e *ensaioD09, origem, nome string) string {
		destino := e.in(nome)
		b, err := os.ReadFile(origem)
		require.NoError(t, err)
		escrever(t, destino, string(b), 0o644)
		return destino
	}
	t.Run("dump parcial", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.recusa("nome fora do padrao", e.argsTrocando(e.dump(), copiar(e, e.dump(), "parcial-rotaperfumes-"+carimboD09+".sql.gz"))...)
	})
	t.Run("dump pre-restauracao", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.recusa("nome fora do padrao", e.argsTrocando(e.dump(), copiar(e, e.dump(), "pre-restauracao-"+carimboD09+".sql.gz"))...)
	})
	t.Run("dump symlink", func(t *testing.T) {
		e := novoEnsaioD09(t)
		l := e.in("rotaperfumes-20261105-040000Z.sql.gz")
		require.NoError(t, os.Symlink(e.dump(), l))
		e.recusa("sem link simbolico", e.argsTrocando(e.dump(), l)...)
	})
	t.Run("imagens com nome de dump", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.recusa("nome fora do padrao", e.argsTrocando(e.imagens(), copiar(e, e.imagens(), "imagens.tar.gz"))...)
	})
	t.Run("carimbos diferentes sem motivo", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.recusa("carimbos diferentes", e.argsTrocando(e.imagens(), copiar(e, e.imagens(), "rotaperfumes-imagens-20261104-033000Z.tar.gz"))...)
	})
	for _, motivo := range []string{"curto", strings.Repeat("x", 301)} {
		t.Run("motivo fora do tamanho", func(t *testing.T) {
			e := novoEnsaioD09(t)
			e.recusa("10 a 300 caracteres", append(e.argsBase(), "--par-divergente-motivo", motivo)...)
		})
	}
	t.Run("pepper com permissão errada", func(t *testing.T) {
		e := novoEnsaioD09(t)
		require.NoError(t, os.Chmod(e.in("pepper"), 0o644))
		e.recusa("--pepper-file: precisa ser arquivo regular", e.argsBase()...)
	})
	t.Run("token com permissão errada", func(t *testing.T) {
		e := novoEnsaioD09(t)
		require.NoError(t, os.Chmod(e.in("token"), 0o640))
		e.recusa("--token-file: precisa ser arquivo regular", append(e.argsBase(), "--token-file", e.in("token"))...)
	})
	t.Run("api.env com permissão errada", func(t *testing.T) {
		e := novoEnsaioD09(t)
		require.NoError(t, os.Chmod(e.in("api.env"), 0o644))
		e.recusa("--api-env: precisa ser arquivo regular", e.argsBase()...)
	})
	t.Run("pepper diferente do api.env", func(t *testing.T) {
		e := novoEnsaioD09(t)
		escrever(t, e.in("pepper"), "SENTINELA-OUTRO-PEPPER-com-mais-de-32-bytes-0123\n", 0o600)
		out := e.recusa("difere do PASSWORD_PEPPER", e.argsBase()...)
		assert.NotContains(t, out, "SENTINELA-OUTRO-PEPPER")
	})
	t.Run("pepper curto", func(t *testing.T) {
		e := novoEnsaioD09(t)
		escrever(t, e.in("pepper"), "curto\n", 0o600)
		e.recusa("menos de 32 caracteres", e.argsBase()...)
	})
	t.Run("sem espaço", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.estado("df_pouco", "")
		e.recusa("espaco insuficiente", e.argsBase()...)
	})
	t.Run("uso: --api-env é obrigatório", func(t *testing.T) {
		e := novoEnsaioD09(t)
		out, err := e.tty(nil, "--dump", e.dump(), "--imagens", e.imagens(), "--ip", ipD09, "--host-nome", hostD09, "--hostname-publico", publicoD09, "--nova-ca").terminar()
		require.Error(t, err)
		assert.Contains(t, out, "o trio sozinho nao sobe a API")
	})
}

func TestDEPLOY09_Recusa_ApiEnv(t *testing.T) {
	base := apiEnvD09()
	casos := []struct {
		nome, de, para, trecho string
	}{
		{"secret de teste", "TURNSTILE_SECRET_KEY=" + turnstileD09, "TURNSTILE_SECRET_KEY=1x0000000000000000000000000000000AA", "TURNSTILE_SECRET_KEY e uma secret de teste"},
		{"válvula da secret de teste", "COOKIE_SECURE=true", "TURNSTILE_PERMITIR_SECRET_TESTE=true", "TURNSTILE_PERMITIR_SECRET_TESTE"},
		{"secret vazia", "TURNSTILE_SECRET_KEY=" + turnstileD09, "TURNSTILE_SECRET_KEY=", "chave obrigatoria vazia: TURNSTILE_SECRET_KEY"},
		{"pepper vazio", "PASSWORD_PEPPER=" + pepperD09, "PASSWORD_PEPPER=", "chave obrigatoria vazia: PASSWORD_PEPPER"},
		{"cifrão sem aspas", "SMTP_PASSWORD='a$b'", "SMTP_PASSWORD=a$b", "sem aspas simples em SMTP_PASSWORD"},
		{"cifrão em aspas duplas", "SMTP_PASSWORD='a$b'", `SMTP_PASSWORD="a$b"`, "dentro de aspas duplas em SMTP_PASSWORD"},
		{"sem o hostname público em CORS", "https://" + publicoD09 + ",", "", "CORS_ALLOWED_ORIGINS sem o HTTPS do --hostname-publico"},
		{"sem o IP em CORS", ",https://" + ipD09 + ":8443", "", "CORS_ALLOWED_ORIGINS sem o --ip"},
		{"sem o host em CORS", "https://" + hostD09 + ":8443,", "", "CORS_ALLOWED_ORIGINS sem o --host-nome"},
		{"Turnstile sem o público", "TURNSTILE_HOSTNAMES=" + publicoD09 + ",", "TURNSTILE_HOSTNAMES=", "TURNSTILE_HOSTNAMES sem o --hostname-publico"},
		{"outra conta", "DB_USUARIO=rotaperfumes_app", "DB_USUARIO=rotaperfumes_admin", "DB_USUARIO diferente"},
		{"linha fora do formato", "COOKIE_SECURE=true", "export COOKIE_SECURE=true", "linha 8 fora do formato"},
		{"chave repetida", "COOKIE_SECURE=true", "COOKIE_SECURE=true\nCOOKIE_SECURE=false", "chave COOKIE_SECURE repetida"},
		{"comentário no valor", "COOKIE_SECURE=true", "COOKIE_SECURE=true # nao", "comentario no meio do valor de COOKIE_SECURE"},
		{"CRLF", "COOKIE_SECURE=true", "COOKIE_SECURE=true\r", "com CRLF"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			require.Contains(t, base, c.de)
			e := novoEnsaioD09(t)
			escrever(t, e.in("api.env"), strings.Replace(base, c.de, c.para, 1), 0o600)
			out := e.recusa(c.trecho, e.argsBase()...)
			assert.NotContains(t, out, "1x0000000000", "cita só o nome da chave")
			assert.NotContains(t, out, "a$b")
		})
	}
	t.Run("sem TURNSTILE_HOSTNAMES passa", func(t *testing.T) {
		e := novoEnsaioD09(t)
		escrever(t, e.in("api.env"), strings.Replace(base, "TURNSTILE_HOSTNAMES="+publicoD09+","+hostD09, "TURNSTILE_HOSTNAMES=", 1), 0o600)
		e.tty(nil, append(e.argsBase(), "--sem-tunel")...).esperar(promptHostD09)
	})
	t.Run("Turnstile sem a LAN só avisa", func(t *testing.T) {
		e := novoEnsaioD09(t)
		escrever(t, e.in("api.env"), strings.Replace(base, "TURNSTILE_HOSTNAMES="+publicoD09+","+hostD09, "TURNSTILE_HOSTNAMES="+publicoD09, 1), 0o600)
		s := e.tty(nil, append(e.argsBase(), "--sem-tunel")...)
		s.esperar(promptHostD09)
		assert.Contains(t, s.saida(), "o login pela LAN da etapa 9 sera recusado")
	})
}

func TestDEPLOY09_Recusa_CA(t *testing.T) {
	com := func(e *ensaioD09, cert, chave string) []string {
		a := e.argsBase()
		saida := []string{}
		for _, v := range a {
			if v != "--nova-ca" {
				saida = append(saida, v)
			}
		}
		return append(saida, "--ca-cert", cert, "--ca-key", chave)
	}
	t.Run("excludentes", func(t *testing.T) {
		e := novoEnsaioD09(t)
		out, err := e.tty(nil, append(e.argsBase(), "--ca-cert", e.in("ca.pem"), "--ca-key", e.in("ca-key.pem"))...).terminar()
		require.Error(t, err)
		assert.Contains(t, out, "--nova-ca nao combina")
	})
	t.Run("nenhuma", func(t *testing.T) {
		e := novoEnsaioD09(t)
		a := []string{}
		for _, v := range e.argsBase() {
			if v != "--nova-ca" {
				a = append(a, v)
			}
		}
		out, err := e.tty(nil, a...).terminar()
		require.Error(t, err)
		assert.Contains(t, out, "informe --ca-cert e --ca-key, ou --nova-ca")
	})
	t.Run("chave de outra CA", func(t *testing.T) {
		e := novoEnsaioD09(t)
		outra := e.in("outra-key.pem")
		out, err := exec.Command("openssl", "genpkey", "-algorithm", "EC", "-pkeyopt", "ec_paramgen_curve:prime256v1", "-out", outra).CombinedOutput()
		require.NoError(t, err, string(out))
		e.recusa("nao e a chave da --ca-cert", com(e, e.in("ca.pem"), outra)...)
	})
	t.Run("chave privada no lugar do certificado", func(t *testing.T) {
		e := novoEnsaioD09(t)
		e.recusa("bloco PEM que nao e certificado", com(e, e.in("ca-key.pem"), e.in("ca-key.pem"))...)
	})
	t.Run("CA de dev", func(t *testing.T) {
		e := novoEnsaioD09(t)
		_, _, fp := caFixture(t)
		// O fingerprint do fixture entra na lista de CAs de dev do script; a lista real
		// é igual à do mariadb-docker.sh (paridade no teste estático).
		sc := e.ler("clone/deploy/restaurar-maquina.sh")
		const ancora = "readonly -a CA_DEV_FPS=(\n"
		require.Contains(t, sc, ancora)
		e.clone("deploy/restaurar-maquina.sh", strings.Replace(sc, ancora, ancora+"  \""+fp+"\"\n", 1), 0o755)
		e.recommit()
		e.recusa("CA de DEV", com(e, e.in("ca.pem"), e.in("ca-key.pem"))...)
	})
}

func TestDEPLOY09_Recusa_CAForaDoFingerprintVersionado(t *testing.T) {
	e := novoEnsaioD09(t)
	e.clone("deploy/db-ca.fingerprint", "# nada\n"+strings.Repeat("a", 64)+"\n", 0o644)
	e.recommit()
	a := []string{}
	for _, v := range e.argsBase() {
		if v != "--nova-ca" {
			a = append(a, v)
		}
	}
	e.recusa("fora de deploy/db-ca.fingerprint", append(a, "--ca-cert", e.in("ca.pem"), "--ca-key", e.in("ca-key.pem"))...)
}
