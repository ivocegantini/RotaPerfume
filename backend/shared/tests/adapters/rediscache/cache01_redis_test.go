package rediscache_test

// CACHE-01: adapter RESP (URL, AUTH/SELECT, GET/SET PX/INCR, nulo, erro do
// servidor, prazo). REDIS_TESTE_URL=redis://host:porta liga o teste real.

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/adapters/rediscache"
)

func TestCACHE01_ParseURL(t *testing.T) {
	o, err := rediscache.ParseURL("redis://192.168.168.106:48110")
	require.NoError(t, err)
	assert.Equal(t, "192.168.168.106:48110", o.Endereco)
	assert.Zero(t, o.DB)
	o, err = rediscache.ParseURL(" redis://loja@cache:6379/2 ")
	require.NoError(t, err)
	assert.Equal(t, "loja", o.Usuario)
	assert.Equal(t, 2, o.DB)
	for _, ruim := range []string{
		"http://192.168.168.106:48110", "redis://host", "redis://:s3nha@host:6379", "redis://host:0",
		"redis://host:6379/16", "redis://host:6379/x", "redis://host:6379?db=1", "host:6379", "",
	} {
		_, err := rediscache.ParseURL(ruim)
		assert.ErrorIs(t, err, rediscache.ErrURL, ruim)
	}
}

// falso é um servidor RESP mínimo em memória que registra os comandos.
type falso struct {
	mu        sync.Mutex
	dados     map[string]string
	cmds      []string
	senha     string
	atrasoGet time.Duration
}

func (f *falso) subir(t *testing.T) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.atender(c)
		}
	}()
	return ln.Addr().String()
}

func lerComando(r *bufio.Reader) ([]string, error) {
	l, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	n, _ := strconv.Atoi(strings.TrimSpace(l[1:]))
	args := make([]string, n)
	for i := range args {
		l, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		tam, _ := strconv.Atoi(strings.TrimSpace(l[1:]))
		buf := make([]byte, tam+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		args[i] = string(buf[:tam])
	}
	return args, nil
}

func (f *falso) atender(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	autenticado := f.senha == ""
	for {
		args, err := lerComando(r)
		if err != nil {
			return
		}
		f.mu.Lock()
		f.cmds = append(f.cmds, args[0])
		var resp string
		switch {
		case args[0] == "AUTH":
			if args[len(args)-1] == f.senha {
				autenticado, resp = true, "+OK\r\n"
			} else {
				resp = "-WRONGPASS invalid password\r\n"
			}
		case !autenticado:
			resp = "-NOAUTH Authentication required.\r\n"
		case args[0] == "PING":
			resp = "+PONG\r\n"
		case args[0] == "SELECT":
			resp = "+OK\r\n"
		case args[0] == "GET":
			if f.atrasoGet > 0 {
				f.mu.Unlock()
				time.Sleep(f.atrasoGet)
				f.mu.Lock()
			}
			if v, ok := f.dados[args[1]]; ok {
				resp = fmt.Sprintf("$%d\r\n%s\r\n", len(v), v)
			} else {
				resp = "$-1\r\n"
			}
		case args[0] == "SET":
			f.dados[args[1]], resp = args[2], "+OK\r\n"
		case args[0] == "INCR":
			n, err := strconv.Atoi(f.dados[args[1]] + "0")
			if err != nil {
				resp = "-ERR value is not an integer or out of range\r\n"
				break
			}
			f.dados[args[1]] = strconv.Itoa(n/10 + 1)
			resp = ":" + f.dados[args[1]] + "\r\n"
		default:
			resp = "-ERR unknown command\r\n"
		}
		f.mu.Unlock()
		if _, err := io.WriteString(c, resp); err != nil {
			return
		}
	}
}

func TestCACHE01_ComandosNoServidorFalso(t *testing.T) {
	f := &falso{dados: map[string]string{}, senha: "s3gredo"}
	cli := rediscache.Novo(rediscache.Opcoes{Endereco: f.subir(t), Senha: "s3gredo", DB: 1})
	ctx := context.Background()
	require.NoError(t, cli.Ping(ctx))

	_, ok, err := cli.Get(ctx, "nao-existe")
	require.NoError(t, err)
	assert.False(t, ok)
	valor := "rpc1\nimage/png\n0\n\x00\r\n binário"
	require.NoError(t, cli.Set(ctx, "k", []byte(valor), time.Minute))
	b, ok, err := cli.Get(ctx, "k")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, valor, string(b))
	n, err := cli.Incr(ctx, "v")
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	n, _ = cli.Incr(ctx, "v")
	assert.EqualValues(t, 2, n)

	f.dados["texto"] = "abc"
	_, err = cli.Incr(ctx, "texto")
	var errSrv rediscache.ErrRedis
	assert.ErrorAs(t, err, &errSrv, "-ERR vira ErrRedis")
	require.NoError(t, cli.Ping(ctx), "a conexão segue boa depois do -ERR")

	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Equal(t, "AUTH", f.cmds[0])
	assert.Equal(t, "SELECT", f.cmds[1])
}

func TestCACHE01_SenhaErrada(t *testing.T) {
	f := &falso{dados: map[string]string{}, senha: "certa"}
	cli := rediscache.Novo(rediscache.Opcoes{Endereco: f.subir(t), Senha: "errada"})
	assert.ErrorContains(t, cli.Ping(context.Background()), "AUTH")
}

func TestCACHE01_PrazoCurto(t *testing.T) {
	f := &falso{dados: map[string]string{}, atrasoGet: 300 * time.Millisecond}
	cli := rediscache.Novo(rediscache.Opcoes{Endereco: f.subir(t), Prazo: 50 * time.Millisecond})
	inicio := time.Now()
	_, _, err := cli.Get(context.Background(), "k")
	assert.Error(t, err)
	assert.Less(t, time.Since(inicio), 250*time.Millisecond)
}

func TestCACHE01_ServidorFora(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	ln.Close()
	assert.Error(t, rediscache.Novo(rediscache.Opcoes{Endereco: addr}).Ping(context.Background()))
}

// Teste real (opcional): REDIS_TESTE_URL=redis://192.168.168.106:48110.
func TestCACHE01_RedisReal(t *testing.T) {
	raw := os.Getenv("REDIS_TESTE_URL")
	if raw == "" {
		t.Skip("REDIS_TESTE_URL não definido")
	}
	o, err := rediscache.ParseURL(raw)
	require.NoError(t, err)
	o.Senha = os.Getenv("REDIS_TESTE_PASSWORD")
	cli := rediscache.Novo(o)
	ctx := context.Background()
	require.NoError(t, cli.Ping(ctx))
	chave := fmt.Sprintf("rotaperfumes_teste:vitrine:v0:%d", time.Now().UnixNano())
	require.NoError(t, cli.Set(ctx, chave, []byte("ok"), 2*time.Second))
	b, ok, err := cli.Get(ctx, chave)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "ok", string(b))
	n, err := cli.Incr(ctx, chave+":versao")
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	require.NoError(t, cli.Set(ctx, chave+":versao", []byte("1"), 2*time.Second)) // não deixa lixo

}
