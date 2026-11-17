// Package rediscache é o adapter Redis da porta cache.Armazem (CACHE-01):
// cliente RESP2 mínimo (AUTH, SELECT, GET, SET PX, INCR), sem dependência
// externa, com pool pequeno e prazo curto por operação (o cache é fail-open).
package rediscache

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Opcoes do cliente.
type Opcoes struct {
	Endereco string        // host:porta
	Usuario  string        // ACL (opcional)
	Senha    string        // requirepass/ACL (opcional)
	DB       int           // SELECT
	Prazo    time.Duration // dial + ida e volta de cada operação
	MaxConns int           // conexões ociosas guardadas
}

// ErrURL: REDIS_URL fora de redis://host:porta[/db].
var ErrURL = errors.New("rediscache: REDIS_URL inválida (use redis://host:porta[/db]; o Redis fala RESP, não HTTP)")

// ParseURL lê redis://[usuario@]host:porta[/db]. Senha na URL é recusada
// (vai em REDIS_PASSWORD, fora de logs e de ps).
func ParseURL(raw string) (Opcoes, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "redis" || u.Host == "" || u.Port() == "" || u.RawQuery != "" || u.Fragment != "" {
		return Opcoes{}, ErrURL
	}
	if _, temSenha := u.User.Password(); temSenha {
		return Opcoes{}, fmt.Errorf("%w: a senha vai em REDIS_PASSWORD, não na URL", ErrURL)
	}
	if p, err := strconv.Atoi(u.Port()); err != nil || p < 1 || p > 65535 {
		return Opcoes{}, ErrURL
	}
	o := Opcoes{Endereco: u.Host, Prazo: 250 * time.Millisecond, MaxConns: 8}
	if u.User != nil {
		o.Usuario = u.User.Username()
	}
	if db := strings.Trim(u.Path, "/"); db != "" {
		n, err := strconv.Atoi(db)
		if err != nil || n < 0 || n > 15 {
			return Opcoes{}, ErrURL
		}
		o.DB = n
	}
	return o, nil
}

// Cliente é o armazém sobre o Redis.
type Cliente struct {
	o     Opcoes
	livre chan *conexao
}

type conexao struct {
	c net.Conn
	r *bufio.Reader
}

// Novo cria o cliente (não conecta; a primeira operação conecta).
func Novo(o Opcoes) *Cliente {
	if o.Prazo <= 0 {
		o.Prazo = 250 * time.Millisecond
	}
	if o.MaxConns <= 0 {
		o.MaxConns = 8
	}
	return &Cliente{o: o, livre: make(chan *conexao, o.MaxConns)}
}

// ErrRedis é a resposta de erro (-ERR ...) do servidor.
type ErrRedis string

func (e ErrRedis) Error() string { return "redis: " + string(e) }

// nulo marca a resposta nula ($-1).
var nulo = errors.New("nulo")

func (c *Cliente) pegar(ctx context.Context) (*conexao, error) {
	select {
	case cn := <-c.livre:
		return cn, nil
	default:
	}
	d := net.Dialer{Timeout: c.o.Prazo}
	nc, err := d.DialContext(ctx, "tcp", c.o.Endereco)
	if err != nil {
		return nil, err
	}
	cn := &conexao{c: nc, r: bufio.NewReader(nc)}
	if c.o.Senha != "" {
		args := []string{"AUTH", c.o.Senha}
		if c.o.Usuario != "" {
			args = []string{"AUTH", c.o.Usuario, c.o.Senha}
		}
		if _, err := c.trocar(cn, args...); err != nil {
			nc.Close()
			return nil, fmt.Errorf("AUTH: %w", err)
		}
	}
	if c.o.DB != 0 {
		if _, err := c.trocar(cn, "SELECT", strconv.Itoa(c.o.DB)); err != nil {
			nc.Close()
			return nil, fmt.Errorf("SELECT: %w", err)
		}
	}
	return cn, nil
}

func (c *Cliente) devolver(cn *conexao) {
	select {
	case c.livre <- cn:
	default:
		cn.c.Close()
	}
}

// executar roda um comando numa conexão do pool; erro de rede descarta a
// conexão, erro do servidor (-ERR) a devolve.
func (c *Cliente) executar(ctx context.Context, args ...string) (any, error) {
	cn, err := c.pegar(ctx)
	if err != nil {
		return nil, err
	}
	v, err := c.trocar(cn, args...)
	var errSrv ErrRedis
	if err != nil && !errors.As(err, &errSrv) && !errors.Is(err, nulo) {
		cn.c.Close()
		return nil, err
	}
	c.devolver(cn)
	return v, err
}

// trocar envia o comando (array de bulk strings) e lê uma resposta.
func (c *Cliente) trocar(cn *conexao, args ...string) (any, error) {
	if err := cn.c.SetDeadline(time.Now().Add(c.o.Prazo)); err != nil {
		return nil, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	if _, err := io.WriteString(cn.c, b.String()); err != nil {
		return nil, err
	}
	return lerResposta(cn.r)
}

// maxBulk limita o tamanho de uma resposta (protege a memória da API).
const maxBulk = 8 << 20

// lerResposta lê +simples, -erro, :inteiro e $bulk (o resto é recusado).
func lerResposta(r *bufio.Reader) (any, error) {
	linha, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if len(linha) < 3 || !strings.HasSuffix(linha, "\r\n") {
		return nil, errors.New("rediscache: resposta malformada")
	}
	tipo, corpo := linha[0], linha[1:len(linha)-2]
	switch tipo {
	case '+':
		return corpo, nil
	case '-':
		return nil, ErrRedis(corpo)
	case ':':
		return strconv.ParseInt(corpo, 10, 64)
	case '$':
		n, err := strconv.Atoi(corpo)
		if err != nil || n > maxBulk {
			return nil, errors.New("rediscache: tamanho de bulk inválido")
		}
		if n < 0 {
			return nil, nulo
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		return buf[:n], nil
	default:
		return nil, fmt.Errorf("rediscache: tipo de resposta %q não suportado", tipo)
	}
}

// Get devolve o valor (false se a chave não existe).
func (c *Cliente) Get(ctx context.Context, chave string) ([]byte, bool, error) {
	v, err := c.executar(ctx, "GET", chave)
	if errors.Is(err, nulo) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	b, ok := v.([]byte)
	if !ok {
		return nil, false, errors.New("rediscache: GET sem bulk string")
	}
	return b, true, nil
}

// Set grava com expiração em milissegundos (SET k v PX ttl).
func (c *Cliente) Set(ctx context.Context, chave string, valor []byte, ttl time.Duration) error {
	ms := ttl.Milliseconds()
	if ms < 1 {
		ms = 1
	}
	v, err := c.executar(ctx, "SET", chave, string(valor), "PX", strconv.FormatInt(ms, 10))
	if err != nil {
		return err
	}
	if v != "OK" {
		return errors.New("rediscache: SET sem OK")
	}
	return nil
}

// Incr soma 1 ao contador.
func (c *Cliente) Incr(ctx context.Context, chave string) (int64, error) {
	v, err := c.executar(ctx, "INCR", chave)
	if err != nil {
		return 0, err
	}
	n, ok := v.(int64)
	if !ok {
		return 0, errors.New("rediscache: INCR sem inteiro")
	}
	return n, nil
}

// Ping confere a conexão (subida da API).
func (c *Cliente) Ping(ctx context.Context) error {
	v, err := c.executar(ctx, "PING")
	if err != nil {
		return err
	}
	if v != "PONG" {
		return errors.New("rediscache: PING sem PONG")
	}
	return nil
}
