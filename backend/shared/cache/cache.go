// Package cache é o cache das leituras públicas do e-commerce (CACHE-01).
// Chave = appname:area:v<versao>[:variante]; a versão de cada área é um
// contador no armazém (appname:area:versao) que sobe a cada escrita que muda
// a área, então invalidar é um INCR e as chaves antigas morrem pelo TTL.
// Fail-open: erro no armazém desliga o cache por uma pausa e a API segue.
package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"
)

// Area é uma área das telas do e-commerce que vem da API.
type Area string

// Áreas das telas da loja (página inicial "/" e "/loja", que mostram a
// mesma vitrine). Login, cadastro, Google, confirmação de e-mail e /me são
// POST ou dados da sessão: nunca entram no cache. Termos e política são
// HTML estático do Next (frontend/public), fora da API.
const (
	AreaVitrine       Area = "vitrine"        // grade de produtos: cards, paginação e ordenação
	AreaFiltros       Area = "filtros"        // opções de categoria, marca e nota olfativa
	AreaImagemProduto Area = "imagem_produto" // foto do card do produto
)

// Definicao descreve uma área: tela, rota de origem, TTL e tamanho máximo.
type Definicao struct {
	Area     Area
	Tela     string
	Rota     string
	TTL      time.Duration
	MaxBytes int
}

// Areas é o catálogo das áreas cacheadas (fonte da verdade da doc e dos testes).
var Areas = []Definicao{
	{AreaVitrine, "Vitrine (/ e /loja)", "GET /api/loja/produtos", 5 * time.Minute, 512 << 10},
	{AreaFiltros, "Vitrine (/ e /loja): filtros", "GET /api/loja/filtros", 15 * time.Minute, 256 << 10},
	{AreaImagemProduto, "Vitrine (/ e /loja): foto do card", "GET /api/public/produtos/imagens/{arquivo}", time.Hour, 2 << 20},
}

// DefinicaoDe devolve a definição da área (false se não catalogada).
func DefinicaoDe(a Area) (Definicao, bool) {
	for _, d := range Areas {
		if d.Area == a {
			return d, true
		}
	}
	return Definicao{}, false
}

// Armazem é a porta do armazém chave-valor (o Redis satisfaz).
type Armazem interface {
	Get(ctx context.Context, chave string) ([]byte, bool, error)
	Set(ctx context.Context, chave string, valor []byte, ttl time.Duration) error
	Incr(ctx context.Context, chave string) (int64, error)
}

// ErrAppName: CACHE_APPNAME fora de [a-z0-9_-]{1,32}.
var ErrAppName = errors.New("cache: appname inválido (use 1 a 32 caracteres a-z, 0-9, _ ou -)")

var reAppName = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// ValidarAppName confere o appname da chave.
func ValidarAppName(app string) error {
	if !reAppName.MatchString(app) {
		return ErrAppName
	}
	return nil
}

// Chave são as propriedades da chave: appname, área e versão (+ variante,
// o hash dos parâmetros da requisição).
type Chave struct {
	AppName  string
	Area     Area
	Versao   int64
	Variante string
}

// String monta appname:area:v<versao>[:variante].
func (c Chave) String() string {
	s := fmt.Sprintf("%s:%s:v%d", c.AppName, c.Area, c.Versao)
	if c.Variante != "" {
		s += ":" + c.Variante
	}
	return s
}

// ChaveVersao é o contador de versão da área (appname:area:versao).
func ChaveVersao(app string, a Area) string { return app + ":" + string(a) + ":versao" }

// Variante resume as partes (parâmetros já normalizados) num hash curto, para
// a chave não carregar texto do usuário.
func Variante(partes ...string) string {
	h := sha256.New()
	for _, p := range partes {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)[:12])
}

// pausaPadrao: quanto tempo o cache fica desligado após um erro do armazém.
const pausaPadrao = 30 * time.Second

// Cache aplica a chave e a versão sobre o armazém. nil = cache desligado
// (todos os métodos aceitam receptor nil).
type Cache struct {
	app          string
	arm          Armazem
	pausa        time.Duration
	desligadoAte atomic.Int64 // unix nano
	agora        func() time.Time
}

// Novo cria o cache; arm nil devolve nil (desligado).
func Novo(app string, arm Armazem) (*Cache, error) {
	if arm == nil {
		return nil, nil
	}
	if err := ValidarAppName(app); err != nil {
		return nil, err
	}
	return &Cache{app: app, arm: arm, pausa: pausaPadrao, agora: time.Now}, nil
}

// ComPausa troca a pausa após erro (testes).
func (c *Cache) ComPausa(p time.Duration) *Cache { c.pausa = p; return c }

// ativo diz se o cache está ligado e fora da pausa.
func (c *Cache) ativo() bool {
	return c != nil && c.agora().UnixNano() >= c.desligadoAte.Load()
}

// falhou registra o erro e pausa o cache (fail-open).
func (c *Cache) falhou(op string, err error) {
	c.desligadoAte.Store(c.agora().Add(c.pausa).UnixNano())
	log.Printf("[cache] %s falhou, cache pausado por %s: %v", op, c.pausa, err)
}

// versao lê o contador da área (ausente = 0).
func (c *Cache) versao(ctx context.Context, a Area) (int64, error) {
	b, ok, err := c.arm.Get(ctx, ChaveVersao(c.app, a))
	if err != nil || !ok {
		return 0, err
	}
	v, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("versão da área %s não numérica", a)
	}
	return v, nil
}

// Ler busca a área/variante. Devolve a chave para o Gravar (vazia = não
// gravar: cache desligado, pausado ou área desconhecida).
func (c *Cache) Ler(ctx context.Context, a Area, variante string) (Entrada, string, bool) {
	if _, ok := DefinicaoDe(a); !ok || !c.ativo() {
		return Entrada{}, "", false
	}
	v, err := c.versao(ctx, a)
	if err != nil {
		c.falhou("versão de "+string(a), err)
		return Entrada{}, "", false
	}
	chave := Chave{AppName: c.app, Area: a, Versao: v, Variante: variante}.String()
	b, ok, err := c.arm.Get(ctx, chave)
	if err != nil {
		c.falhou("leitura de "+string(a), err)
		return Entrada{}, "", false
	}
	if !ok {
		return Entrada{}, chave, false
	}
	e, err := Decodificar(b)
	if err != nil {
		log.Printf("[cache] entrada corrompida em %s descartada: %v", a, err)
		return Entrada{}, chave, false
	}
	return e, chave, true
}

// Gravar guarda a entrada na chave devolvida pelo Ler (TTL e limite da área).
func (c *Cache) Gravar(ctx context.Context, chave string, a Area, e Entrada) {
	d, ok := DefinicaoDe(a)
	if chave == "" || !ok || !c.ativo() || len(e.Corpo) > d.MaxBytes {
		return
	}
	if err := c.arm.Set(ctx, chave, e.Codificar(), d.TTL); err != nil {
		c.falhou("gravação de "+string(a), err)
	}
}

// Invalidar sobe a versão das áreas (as entradas antigas expiram pelo TTL).
// Roda mesmo durante a pausa: perder um INCR deixaria dado velho no ar.
func (c *Cache) Invalidar(ctx context.Context, areas ...Area) {
	if c == nil {
		return
	}
	for _, a := range areas {
		v, err := c.arm.Incr(ctx, ChaveVersao(c.app, a))
		if err != nil {
			c.falhou("invalidação de "+string(a), err)
			continue
		}
		log.Printf("[cache] área %s invalidada (versão %d)", a, v)
	}
}

// TodasAsAreas lista as áreas do catálogo (invalidação na subida da API).
func TodasAsAreas() []Area {
	out := make([]Area, 0, len(Areas))
	for _, d := range Areas {
		out = append(out, d.Area)
	}
	return out
}

// Entrada é o valor guardado: tipo do conteúdo, data de modificação e corpo.
type Entrada struct {
	ContentType string
	Modificado  time.Time
	Corpo       []byte
}

// prefixoEntrada versiona o formato do valor.
const prefixoEntrada = "rpc1\n"

// Codificar: "rpc1\n<content-type>\n<unix nano>\n<corpo>".
func (e Entrada) Codificar() []byte {
	var b bytes.Buffer
	b.WriteString(prefixoEntrada)
	b.WriteString(e.ContentType)
	b.WriteByte('\n')
	var mod int64
	if !e.Modificado.IsZero() {
		mod = e.Modificado.UnixNano()
	}
	b.WriteString(strconv.FormatInt(mod, 10))
	b.WriteByte('\n')
	b.Write(e.Corpo)
	return b.Bytes()
}

// ErrEntradaInvalida: valor fora do formato rpc1.
var ErrEntradaInvalida = errors.New("cache: entrada em formato inválido")

// Decodificar desfaz o Codificar.
func Decodificar(b []byte) (Entrada, error) {
	if !bytes.HasPrefix(b, []byte(prefixoEntrada)) {
		return Entrada{}, ErrEntradaInvalida
	}
	resto := b[len(prefixoEntrada):]
	ct, resto, ok := bytes.Cut(resto, []byte{'\n'})
	if !ok {
		return Entrada{}, ErrEntradaInvalida
	}
	modTxt, corpo, ok := bytes.Cut(resto, []byte{'\n'})
	if !ok {
		return Entrada{}, ErrEntradaInvalida
	}
	mod, err := strconv.ParseInt(string(modTxt), 10, 64)
	if err != nil {
		return Entrada{}, ErrEntradaInvalida
	}
	e := Entrada{ContentType: string(ct), Corpo: corpo}
	if mod != 0 {
		e.Modificado = time.Unix(0, mod)
	}
	return e, nil
}
