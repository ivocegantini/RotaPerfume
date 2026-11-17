package deploy_test

// Lote 37 (TestBrain): cobertura complementar de TST-17 item 1 e SEC-58
// contra os "casos de teste mínimos" do spec do SecBrain:
//   - TST-17: matriz de diretórios inválidos nas três opções de diretório,
//     pares sobrepostos que faltavam (ssl x trabalho nos dois sentidos,
//     ssl == segredos), o par permitido (segredos dentro do trabalho),
//     limites do --container/--rede-nome/--ip-admin e a conferência da
//     área de trabalho depois do mktemp (modo diferente de 700 => recusa
//     antes de qualquer docker run, e a área é apagada);
//   - SEC-58 (sugestões do BackBrain-B): chave expirada (gerada com
//     --faked-system-time) recusada, chave com validade futura aceita, e o
//     registro do keyring já instalado divergente no --check
//     (preflight_keyring_mariadb; SEC-61: FALTA, AVISO se não for conferível).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLote37_TST17_MatrizDeDiretoriosInvalidos(t *testing.T) {
	invalidos := []struct{ nome, valor, msg string }{
		{"relativo", "rel/dir", "diretorio invalido: 'rel/dir'"},
		{"raiz", "/", "diretorio invalido: '/'"},
		{"com ..", "/root/a/../b", "diretorio invalido: '/root/a/../b'"},
		{"com vírgula", "/root/a,target=/x", "diretorio invalido: '/root/a,target=/x'"},
		{"com espaço", "/root/a b", "diretorio invalido: '/root/a b'"},
		{"com aspas", "/root/a'b", "diretorio invalido"},
		{"com =", "/root/a=b", "diretorio invalido"},
		// "//" passa no regex e no "!= /" e vira "/" ao tirar a barra final
		// (achado do Lote 37, reportado): a recusa vem de outra conferência
		// (modo 700, ca.pem, disjunção), sem efeito colateral.
		{"barra dupla vira /", "//", ""},
		{"barra tripla vira //", "///", ""},
	}
	for _, o := range []string{"--segredos-dir", "--ssl-dir", "--trabalho-dir"} {
		for _, c := range invalidos {
			t.Run(o+"/"+c.nome, func(t *testing.T) {
				e := novoEnsaioSeed(t)
				out, err := e.rodar("--pepper-file", "/root/pepper.txt", o, c.valor)
				e.recusaSemEfeito(t, out, err, c.msg)
			})
		}
		t.Run(o+"/inexistente", func(t *testing.T) {
			e := novoEnsaioSeed(t)
			out, err := e.rodar("--pepper-file", "/root/pepper.txt", o, filepath.Join(e.dir, "nao-existe"))
			e.recusaSemEfeito(t, out, err, "nao-existe")
		})
		t.Run(o+"/symlink no último componente", func(t *testing.T) {
			e := novoEnsaioSeed(t)
			seg, ssl, trab := filepath.Join(e.dir, "seg"), filepath.Join(e.dir, "ssl"), filepath.Join(e.dir, "trab")
			criarDirsTST17(t, seg, ssl, trab)
			alvo := map[string]string{"--segredos-dir": seg, "--ssl-dir": ssl, "--trabalho-dir": trab}[o]
			lnk := filepath.Join(e.dir, "lnk-final")
			require.NoError(t, os.Symlink(alvo, lnk))
			out, err := e.rodar("--pepper-file", "/root/pepper.txt", o, lnk)
			require.Error(t, err, out)
			e.recusaSemEfeito(t, out, err, "")
			assert.Regexp(t, `e um link simbolico: use o caminho real`, out)
		})
	}
}

func TestLote37_TST17_ParesDeDiretorios(t *testing.T) {
	recusas := []struct {
		nome string
		args func(seg, ssl, trab string) []string
		msg  string
	}{
		{"ssl igual a segredos", func(seg, _, _ string) []string { return []string{"--segredos-dir", seg, "--ssl-dir", seg} }, "precisam ser disjuntos"},
		{"ssl dentro de trabalho", func(_, _, trab string) []string {
			return []string{"--trabalho-dir", trab, "--ssl-dir", filepath.Join(trab, "ssl")}
		}, "--trabalho-dir (" + "/"},
		{"trabalho dentro de ssl", func(_, ssl, _ string) []string {
			return []string{"--ssl-dir", ssl, "--trabalho-dir", filepath.Join(ssl, "t")}
		}, "precisam ser disjuntos"},
		{"segredos com / final igual a trabalho", func(seg, _, _ string) []string {
			return []string{"--segredos-dir", seg + "/", "--trabalho-dir", seg}
		}, "nao pode ser --segredos-dir ("},
	}
	for _, c := range recusas {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioSeed(t)
			seg, ssl, trab := filepath.Join(e.dir, "seg"), filepath.Join(e.dir, "ssl"), filepath.Join(e.dir, "trab")
			criarDirsTST17(t, seg, ssl, trab)
			out, err := e.rodar(append([]string{"--pepper-file", "/root/pepper.txt"}, c.args(seg, ssl, trab)...)...)
			e.recusaSemEfeito(t, out, err, c.msg)
		})
	}
	// Permitido: segredos dentro do trabalho (o padrão /root/rotaperfumes-mariadb
	// está dentro de /root).
	t.Run("segredos dentro de trabalho é aceito", func(t *testing.T) {
		e := novoEnsaioSeed(t)
		trab := filepath.Join(e.dir, "trab")
		seg, ssl := filepath.Join(trab, "seg"), filepath.Join(e.dir, "ssl")
		criarDirsTST17(t, seg, ssl, trab)
		out, err := e.rodar("--pepper-file", "/root/pepper.txt", "--segredos-dir", seg, "--ssl-dir", ssl, "--trabalho-dir", trab)
		require.NoError(t, err, out)
		assert.Equal(t, "-d "+trab+"/rotaperfumes-seed.XXXXXX\n", e.ler("mktemp.log"))
		_, err = os.Stat(filepath.Join(seg, "admin-app.senha"))
		require.NoError(t, err)
		ents, err := os.ReadDir(trab)
		require.NoError(t, err)
		for _, en := range ents {
			assert.False(t, strings.HasPrefix(en.Name(), "rotaperfumes-seed."), "área de trabalho removida: %s", en.Name())
		}
	})
}

func TestLote37_TST17_LimitesDeNomeEIP(t *testing.T) {
	recusas := []struct{ nome, opcao, valor, msg string }{
		{"container terminando com -", "--container", "rota-", "--container invalido: rota-"},
		{"container com ponto", "--container", "rota.db", "--container invalido: rota.db"},
		{"container com /", "--container", "a/b", "--container invalido: a/b"},
		{"rede começando com _", "--rede-nome", "_rede", "--rede-nome invalido: _rede"},
		{"rede com vírgula", "--rede-nome", "a,b", "--rede-nome invalido: a,b"},
		{"ip com espaço", "--ip-admin", "172.30.42.20 ", "--ip-admin invalido"},
		{"ip hex", "--ip-admin", "0xac.30.42.20", "--ip-admin invalido"},
		{"ip decimal", "--ip-admin", "2887658004", "--ip-admin invalido"},
		{"ip com 5 octetos", "--ip-admin", "172.30.42.20.1", "--ip-admin invalido"},
		{"ip .20 de outra subnet", "--ip-admin", "172.30.43.20", "so existe em 172.30.42.20"},
	}
	for _, c := range recusas {
		t.Run(c.nome, func(t *testing.T) {
			e := novoEnsaioSeed(t)
			out, err := e.rodar("--pepper-file", "/root/pepper.txt", c.opcao, c.valor)
			e.recusaSemEfeito(t, out, err, c.msg)
		})
	}
	t.Run("container com 63 caracteres é aceito", func(t *testing.T) {
		e := novoEnsaioSeed(t)
		nome := strings.Repeat("a", 62) + "1"
		out, err := e.rodar("--pepper-file", "/root/pepper.txt", "--container", nome)
		require.NoError(t, err, out)
		assert.Contains(t, e.ler("log"), "inspect -f {{.State.Running}} "+nome)
		assert.Contains(t, e.ler("state/env_conteudo"), "DB_HOST="+nome+"\n")
	})
	t.Run("rede igual por prefixo não basta", func(t *testing.T) {
		e := novoEnsaioSeed(t)
		e.state("redes", "rotaperfumes-db-old\nxrotaperfumes-db\n")
		out, err := e.rodar("--pepper-file", "/root/pepper.txt")
		e.recusaSemEfeito(t, out, err, "nao esta ligado a rede rotaperfumes-db")
	})
}

// stubMktempModo755: cria a área de trabalho e abre o modo (simula um mktemp
// ou umask fora do esperado). O seed confere e recusa antes de qualquer
// docker run.
const stubMktempModo755 = `#!/bin/bash
printf '%s\n' "$*" >>"$FAKE/mktemp.log"
d=$(/usr/bin/mktemp "$@") || exit 1
chmod 755 "$d"
printf '%s' "$d" >"$FAKE/state/trab_criado"
printf '%s\n' "$d"
`

func TestLote37_TST17_AreaDeTrabalhoConferidaDepoisDoMktemp(t *testing.T) {
	e := novoEnsaioSeed(t)
	escrever(t, filepath.Join(e.dir, "bin", "mktemp"), stubMktempModo755, 0o755)
	out, err := e.rodar("--pepper-file", "/root/pepper.txt")
	require.Error(t, err, out)
	assert.Contains(t, out, "fora do esperado (caminho real, root, modo 700)")
	assert.Equal(t, "-d /root/rotaperfumes-seed.XXXXXX\n", e.ler("mktemp.log"))
	for _, c := range e.chamadas() {
		require.NotEmpty(t, c.argv)
		assert.NotEqual(t, "run", c.argv[0], "nenhum docker run: %v", c.argv)
	}
	criado := e.ler("state/trab_criado")
	require.True(t, strings.HasPrefix(criado, "/root/rotaperfumes-seed."), criado)
	// O /root do chroot é o $FAKE/root: a área recusada é apagada pelo trap.
	assert.NoDirExists(t, filepath.Join(e.dir, "root", filepath.Base(criado)))
	assert.NotContains(t, out, pepperSeed)
	assert.NotContains(t, out, senhaDBSeed)
}

// --- SEC-58 ---------------------------------------------------------------
