// Package bootstrap é a composition root da API (ARQ-01, Facade): o ÚNICO
// lugar que conhece config, *sql.DB, os adapters e os singletons por
// processo. DI manual, sem framework (docs/arquitetura-ddd.md §5.11).
package bootstrap

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/analyticshttp"
	"github.com/rotaperfumes/rotaperfumes-api/internal/catalogohttp"
	"github.com/rotaperfumes/rotaperfumes-api/internal/comercialhttp"
	"github.com/rotaperfumes/rotaperfumes-api/internal/identidadehttp"
	"github.com/rotaperfumes/rotaperfumes-api/internal/vendashttp"
	"github.com/rotaperfumes/shared/adapters/ratelimit"
	"github.com/rotaperfumes/shared/adapters/rediscache"
	"github.com/rotaperfumes/shared/cache"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
	"github.com/rotaperfumes/shared/catalogo/catalogofs"
	"github.com/rotaperfumes/shared/catalogo/catalogoimagem"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
)

// Handlers agrupa os handlers HTTP, um campo por handler de contexto
// (Catálogo: ARQ-02; Identidade: ARQ-03; Comercial: ARQ-04; Vendas: ARQ-05;
// Analytics: ARQ-06). Desde o ARQ-06 nenhum handler legado resta.
type Handlers struct {
	Auth           *identidadehttp.AuthHandler
	Usuario        *identidadehttp.UsuarioHandler
	Dashboard      *analyticshttp.DashboardHandler
	SenhaHistorico *identidadehttp.SenhaHistoricoHandler
	Vendedor       *comercialhttp.VendedorHandler
	Cliente        *comercialhttp.ClienteHandler
	Produto        *catalogohttp.ProdutoHandler
	Pedido         *vendashttp.PedidoHandler
	Pagamento      *vendashttp.PagamentoHandler
	Oportunidade   *comercialhttp.OportunidadeHandler
	Visita         *comercialhttp.VisitaHandler
	Estoque        *catalogohttp.EstoqueHandler
	ImagemPublica  *catalogohttp.ImagemPublicaHandler
}

// NovosHandlers cria os handlers sobre o pool, com a Identidade padrão
// (Turnstile real, sem assinante de evento) e o e-mail informado.
// lojaMaxPorMinuto é o limite por IP da vitrine pública (ECOM-10).
const lojaMaxPorMinuto = 300

func NovosHandlers(pool *sql.DB, cfg *config.Config, email identidadeapp.EnviadorSenhaInicial) Handlers {
	return NovosHandlersCom(pool, cfg, NovaIdentidade(pool, cfg, extPadrao(cfg, email)))
}

// NovosHandlersCom cria os handlers com os casos de uso de Identidade já
// montados (produção: Montar; testes: captcha e publicador falsos).
func NovosHandlersCom(pool *sql.DB, cfg *config.Config, identidade *identidadeapp.Casos) Handlers {
	cat := NovoCatalogoHTTPCom(NovoCatalogo(pool, ArmazenamentoDoConfig(cfg)), cfg.CORSAllowedOrigins)
	// ECOM-10: vitrine pública com limite por IP (vitrine + filtros somados).
	cat.Produto.ComLimiteLoja(ratelimit.Nova(lojaMaxPorMinuto, time.Minute, time.Minute), cfg.TrustProxyHeaders)
	// CACHE-01: cache Redis das áreas da loja (REDIS_URL vazio = desligado).
	if c := NovoCache(cfg); c != nil {
		cat.Produto.ComCache(c)
		cat.ImagemPublica.ComCache(c)
	}
	comercialCasos := NovoComercial(pool)
	com := NovoComercialHTTP(comercialCasos)
	escopo := comercialCasos.Escopo() // a mesma regra de escopo para Vendas e para o Dashboard
	vendas := NovoVendasHTTP(NovoVendas(pool, escopo))
	ident := NovoIdentidadeHTTP(identidade, cfg)
	return Handlers{
		Auth:           ident.Auth,
		Usuario:        ident.Usuario,
		Dashboard:      NovoAnalyticsHTTP(NovoAnalytics(pool, escopo)),
		SenhaHistorico: ident.SenhaHistorico,
		Vendedor:       com.Vendedor,
		Cliente:        com.Cliente,
		Produto:        cat.Produto,
		Pedido:         vendas.Pedido,
		Pagamento:      vendas.Pagamento,
		Oportunidade:   com.Oportunidade,
		Visita:         com.Visita,
		Estoque:        cat.Estoque,
		ImagemPublica:  cat.ImagemPublica,
	}
}

// NovoCatalogoHTTP liga o contexto Catálogo (ARQ-02): adapters MariaDB sobre o
// pool → casos de uso → handlers HTTP. Sem armazenamento de imagens (as
// rotas de imagem respondem 500/404); a API usa NovoCatalogoHTTPCom.
func NovoCatalogoHTTP(pool *sql.DB) (*catalogohttp.ProdutoHandler, *catalogohttp.EstoqueHandler) {
	cat := NovoCatalogoHTTPCom(NovoCatalogo(pool, nil), nil)
	return cat.Produto, cat.Estoque
}

// CatalogoHTTP são os handlers do Catálogo.
type CatalogoHTTP struct {
	Produto       *catalogohttp.ProdutoHandler
	Estoque       *catalogohttp.EstoqueHandler
	ImagemPublica *catalogohttp.ImagemPublicaHandler
}

// NovoCatalogoHTTPCom monta os handlers sobre os casos de uso; origensUpload
// são as origens CORS aceitas no upload de imagem (D2).
func NovoCatalogoHTTPCom(casos *catalogoapp.Casos, origensUpload []string) CatalogoHTTP {
	return CatalogoHTTP{
		Produto:       catalogohttp.NovoProdutoHandler(casos).ComOrigensUpload(origensUpload),
		Estoque:       catalogohttp.NovoEstoqueHandler(casos),
		ImagemPublica: catalogohttp.NovoImagemPublicaHandler(casos),
	}
}

// NovoCatalogo monta os casos de uso do Catálogo. Com armazenamento (IMG-01)
// liga também a imagem de produto: normalizador de conteúdo + adapter de
// filesystem + UoW da troca. armazenamento nil: imagens desligadas.
func NovoCatalogo(pool *sql.DB, armazenamento *catalogofs.Armazenamento) *catalogoapp.Casos {
	produtos, estoques := catalogomysql.NovoProdutos(pool), catalogomysql.NovoEstoques(pool)
	if armazenamento == nil {
		return catalogoapp.Novo(produtos, estoques)
	}
	return catalogoapp.NovoComImagens(produtos, estoques, catalogoapp.Imagens{
		Armazenamento: armazenamento,
		Normalizador:  catalogoimagem.Novo(),
		Tx: db.NewTransactor(pool, func(q db.Querier) catalogoapp.TxImagemProduto {
			return catalogoapp.TxImagemProduto{Produtos: catalogomysql.NovoProdutos(q)}
		}),
	})
}

// ArmazenamentoDoConfig abre o diretório PRODUTO_IMAGENS_DIR (IMG-01) com
// fail-fast: diretório que não pode ser criado, aberto ou gravado derruba a
// subida. Config sem diretório (montada à mão nos testes; o config.Load
// sempre preenche) desliga as imagens.
func ArmazenamentoDoConfig(cfg *config.Config) *catalogofs.Armazenamento {
	if cfg.ProdutoImagensDir == "" {
		return nil
	}
	a, err := AbrirArmazenamentoImagens(cfg.ProdutoImagensDir)
	if err != nil {
		log.Fatalf("[server] imagens de produto: %v", err)
	}
	return a
}

// AbrirArmazenamentoImagens abre o adapter de filesystem das imagens.
func AbrirArmazenamentoImagens(dir string) (*catalogofs.Armazenamento, error) {
	a, err := catalogofs.Novo(dir)
	if err != nil {
		return nil, err
	}
	log.Printf("[server] imagens de produto em %s", dir)
	return a, nil
}

// NovoCache monta o cache das áreas da loja (CACHE-01) a partir do config:
// nil se REDIS_URL estiver vazio. Na subida, PING (só informa; o cache é
// fail-open) e nova versão de todas as áreas, para um deploy nunca servir
// JSON no formato antigo.
func NovoCache(cfg *config.Config) *cache.Cache {
	if !cfg.Redis.Ligado {
		log.Printf("[cache] REDIS_URL vazio: cache desligado")
		return nil
	}
	cli := rediscache.Novo(rediscache.Opcoes{Endereco: cfg.Redis.Endereco, Usuario: cfg.Redis.Usuario, Senha: cfg.Redis.Senha, DB: cfg.Redis.DB})
	c, err := cache.Novo(cfg.CacheAppName, cli)
	if err != nil {
		log.Fatalf("[cache] %v", err)
	}
	if cfg.Redis.Senha == "" {
		log.Printf("[cache] AVISO: Redis %s sem REDIS_PASSWORD (defina requirepass/ACL no servidor)", cfg.Redis.Endereco)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := cli.Ping(ctx); err != nil {
		log.Printf("[cache] Redis %s indisponível na subida (a API segue sem cache até ele voltar): %v", cfg.Redis.Endereco, err)
		return c
	}
	c.Invalidar(ctx, cache.TodasAsAreas()...)
	log.Printf("[cache] ligado: Redis %s db=%d appname=%s áreas=%v", cfg.Redis.Endereco, cfg.Redis.DB, cfg.CacheAppName, cache.TodasAsAreas())
	return c
}
