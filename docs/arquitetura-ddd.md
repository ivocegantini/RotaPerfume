# Arquitetura DDD da API

> Estado atual da arquitetura do backend (`backend/shared` e `backend/rotaperfumes-api`), conferido no código.
> Decisão em formato ADR: `docs/adr-001-reestruturacao-ddd.md`. Invariantes de segurança (IDs R-, F-, C-, I-, E-, V-, D-, M-): `docs/arq-00-invariantes-seguranca.md`.
> Caminhos relativos a `backend/` quando começam por `shared/` ou `rotaperfumes-api/`.

---

## 1. Visão geral

| # | Decisão | Seção |
|---|---|---|
| D1 | **Dois módulos Go**: `github.com/rotaperfumes/shared` e `github.com/rotaperfumes/rotaperfumes-api` (com `replace ../shared`). | §2 |
| D2 | **Cada bounded context vive em `shared/<ctx>/`**, em três camadas: `<ctx>` (domínio), `<ctx>app` (casos de uso + portas) e `<ctx>mysql` (adapters). Tudo público: a API e os CLIs usam os mesmos casos de uso. | §2, §3 |
| D3 | **A camada HTTP fica em `rotaperfumes-api/internal/`**: `httpx` (kit transversal), `<ctx>http` (handlers) e `bootstrap` (composition root). | §7, §8 |
| D4 | **Shared kernel `shared/kernel`**: `Principal`, `Role`, `Escopo`, `Pagina`/`Resultado[T]`, `Ordenacao[C]`, `Data`, `Email`, `Dinheiro`, erros tipados por `Kind`. | §3.2 |
| D5 | Pacotes técnicos ficam fora dos contextos: `config`, `db`, `vlog`, `tz`, `cnpj`, `cmdutil`, `appkit`, `cache`, `adapters/*`, `testkit`. | §2 |
| D6 | **Router com política tipada e método HTTP obrigatórios** (`Public`, `Authenticated`, `AdminOnly`, `AnyRole`). | §7.1 |
| D7 | **vlog em quatro níveis** (rota, caso de uso, decisão, query), nunca "linha a linha". | §6 |
| D8 | **Unit of Work genérico** (`db.Transactor[R]`): a transação é decidida pelo caso de uso, não pelo repositório. | §5.2 |
| D9 | **Mapper de erros único por `Kind`**, com mensagem pública fixa no sentinela. | §5.6 |

Regras herdadas que valem sempre:
- **Role desconhecido no banco (R-a):** tratado como restrito, com log `[auth] role desconhecido user_id=<id>`; nunca 401/500 novo.
- **Sem cache de usuário, escopo ou vendedor entre requests (R4)**; o único cache é o das leituras públicas da loja (§11.3).

---

## 2. Estrutura

```
backend/
├── Dockerfile                                (contexto backend/; compila ./cmd/server)
├── shared/                                   módulo github.com/rotaperfumes/shared
│   ├── kernel/          data dinheiro email erros escopo ordenacao pagina principal rota
│   ├── catalogo/        produto estoque sku ordenacao imagem erros                       DOMÍNIO
│   │   ├── catalogoapp/     casos portas produtos estoque imagens                       APLICAÇÃO
│   │   ├── catalogomysql/   produtos estoques faturamento linhas                        ADAPTER
│   │   ├── catalogofs/      armazenamento (imagens em disco via os.Root)                ADAPTER
│   │   └── catalogoimagem/  normalizador (validação e reencode de imagem)               ADAPTER
│   ├── identidade/      usuario refresh historico politica_senha segredos captcha confirmacao google erros
│   │   ├── identidadeapp/   casos portas login sessao senha usuarios massa limpeza cadastro google
│   │   ├── identidademysql/ usuarios refresh_tokens historico confirmacoes
│   │   └── identidadecrypto/ hasher jwt aleatorio
│   ├── comercial/       vendedor carteira cliente cliente_loja cnpj visita oportunidade politica ordenacao erros
│   │   ├── comercialapp/    casos portas escopo clientes carteiras vendedores visitas oportunidades
│   │   └── comercialmysql/  clientes carteiras vendedores visitas oportunidades linhas
│   ├── vendas/          pedido pagamento filtros entrega checkout compras pagamento_loja erros
│   │   ├── vendasapp/       casos portas uow pedidos pagamentos checkout compras pagamento_loja expiracao
│   │   └── vendasmysql/     pedidos pagamentos compras loja linhas
│   ├── analytics/       periodo dias janela metricas serie ranking clientes funil  (read models)
│   │   ├── analyticsapp/    consultas portas
│   │   └── analyticsmysql/  dashboard clientes oportunidades filtros datas sql
│   ├── notificacao/     notificacao destinatario modo_tls mensagem
│   │   ├── notificacaoapp/  notifier portas
│   │   └── notificacaosmtp/ smtp transporte conexao auth noop
│   ├── adapters/        turnstile/ ratelimit/ googleid/ rediscache/
│   ├── cache/           cache das leituras públicas da loja (CACHE-01)
│   ├── appkit/          releitura (RelerCriado/RelerAtualizado depois da escrita)
│   ├── db/              db tx consulta linhas duplicada fk like
│   ├── config/  vlog/ (vlog, trace)  tz/  cnpj/  cmdutil/ (cmdutil, db, cli)
│   ├── testkit/integracao/  helper único dos testes de integração (SEC-50)
│   ├── importers/       importkit/ + carteiras clientes estoque oportunidades pagamentos pedidos produtos visitas;
│   │                    clientesdedup (biblioteca)
│   ├── tools/           exportdados limpezaimagens resetpassword seedusers testeemail (cli.go + lógica)
│   ├── cmd/             import*, exportdados, limpezaimagens, resetpassword, seedusers, testeemail (main fino)
│   └── tests/           (inclui tests/arquitetura)
└── rotaperfumes-api/                         módulo github.com/rotaperfumes/rotaperfumes-api
    ├── cmd/server/       carrega .env e config, liga o vlog e chama bootstrap.Run
    ├── cmd/resetsenhas/  main fino → internal/resetsenhas
    ├── cmd/gerarimagens/ main fino → internal/gerarimagens
    ├── internal/
    │   ├── bootstrap/       server rotas handlers identidade comercial vendas analytics email background
    │   ├── httpx/           router chain cors secheaders auth principal errmap respond casos body query
    │   │                    clientip cookies logging cache
    │   ├── catalogohttp/    produto_handler estoque_handler imagem_handler loja_handler dto
    │   ├── identidadehttp/  auth_handler usuario_handler senha_historico_handler dto
    │   ├── comercialhttp/   cliente_handler vendedor_handler visita_handler oportunidade_handler dto
    │   ├── vendashttp/      pedido_handler pagamento_handler checkout_handler compras_handler
    │   │                    pagamento_loja_handler entrega dto
    │   ├── analyticshttp/   dashboard_handler dto
    │   ├── gerarimagens/    gerarimagens render
    │   └── resetsenhas/     resetsenhas
    └── tests/           (inclui tests/arquitetura)
```

Por que essa forma:
1. **Aplicação em `shared`:** os CLIs (`resetsenhas`, importers, tools) e a API usam o mesmo caso de uso; `shared` não pode importar a API.
2. **Sem `shared/internal`:** o Go só deixa importar `internal/` de dentro do mesmo módulo; a disciplina de camadas fica com os testes de arquitetura (§4.2).
3. **Nomes sufixados** (`catalogo`, `catalogoapp`, `catalogomysql`): dispensam alias e deixam o `grep` óbvio; o domínio leva o nome do contexto (`catalogo.Produto`).
4. **Adapters `<ctx>mysql`** mantêm o sufixo por nome histórico; o banco é **MariaDB 11.4 LTS** com o driver `go-sql-driver/mysql` (erro 1062, `*mysql.MySQLError`).

Não existem mais: `rotaperfumes-api/{handlers,services,middleware,routes}` e `shared/{services,repositories,models}` (o teste de arquitetura falha se voltarem com importadores).

---

## 3. Bounded contexts e shared kernel

### 3.1 Mapa de contextos

| Contexto | Domínio | Aplicação | Adapters | HTTP |
|---|---|---|---|---|
| Catálogo (Produto, Estoque, Imagem, vitrine da loja) | `shared/catalogo` | `catalogoapp` | `catalogomysql` (inclui `faturamento.go`, baixa de estoque na tx de Vendas), `catalogofs`, `catalogoimagem` | `catalogohttp` |
| Identidade e Acesso (Usuário, RefreshToken, Histórico de senha, cadastro, confirmação de e-mail, Google) | `shared/identidade` | `identidadeapp` | `identidademysql`, `identidadecrypto`, `adapters/turnstile`, `adapters/ratelimit`, `adapters/googleid` | `identidadehttp` + `httpx/auth.go` |
| Comercial (Vendedor, Carteira, Cliente, cliente da loja, Visita, Oportunidade) | `shared/comercial` | `comercialapp` (inclui o `AutorizadorEscopo`) | `comercialmysql` | `comercialhttp` |
| Vendas (Pedido + itens, Pagamento, checkout e pagamento da loja, minhas compras) | `shared/vendas` | `vendasapp` (UoW `TxPedido`) | `vendasmysql` | `vendashttp` |
| Analytics (Dashboard, read models) | `shared/analytics` | `analyticsapp` | `analyticsmysql` | `analyticshttp` |
| Notificação (senha inicial, confirmação de e-mail, alerta de segurança) | `shared/notificacao` | `notificacaoapp` | `notificacaosmtp` (SMTP e Noop) | sem rotas; montagem em `bootstrap/email.go` |

Ligações entre contextos (sempre por porta declarada no `<ctx>app` consumidor e ligada no `bootstrap`):

| De → Para | Porta | Implementação |
|---|---|---|
| Vendas, Analytics → Comercial | `vendasapp.EscopoCarteira`, `analyticsapp.EscopoAnalytics` | `comercialapp.AutorizadorEscopo` (`comercialCasos.Escopo()`) |
| Vendas → Catálogo | `vendasapp.BaixaEstoque` (na mesma tx do pedido) | `catalogomysql/faturamento.go` |
| Identidade → Comercial | `TxCadastro.Clientes`, `identidadeapp.ClientesLoja` (cliente da loja no cadastro) | adapters de `comercialmysql`/`comercial` ligados no bootstrap |
| Identidade → Notificação | evento `identidade.ReusoDeRefreshDetectado` | `notificacaoapp.AlertaSegurancaNotifier` |
| Comercial → Identidade | `TxDesligamento` (inativa usuários + revoga tokens na tx do desligamento, I-09) | repositórios de Identidade na mesma `*sql.Tx` |

### 3.2 Shared kernel (`shared/kernel`)

O kernel é puro: só stdlib, sem `database/sql`, `net/http`, `log` nem pacotes do projeto ou de terceiros.

| Tipo | Regra |
|---|---|
| `Role` (`RoleAdmin`, `RoleNormal`, `RoleUsuario`) + `RoleDoBanco` | Guarda o valor bruto do banco. `EhAdmin()` só para `"admin"` exato; `EhUsuarioLoja()` para `"usuario"`; `Conhecido()` para o log de R-a. |
| `Principal{userID, role}` | Campos privados; zero value inválido (`Valido()` falso). **Só `internal/httpx` chama `NovoPrincipal`** (O5). Não existe "Principal de sistema": CLIs não recebem Principal. |
| `Escopo` | Tipo soma `Irrestrito \| Vendedor(id>0) \| SemAcesso`, zero value inválido (E-02). Construtores `Escopo*` só em `comercialapp`; tradução para SQL **só** por `Filtro()` (O1). |
| `Pagina`, `Resultado[T]` | page ≥ 1 (máx. `PaginaMaxima` = 10.000); limit < 1 vira 20; > 100 vira 100 (F-10). `Offset()`, `Paginas()`. |
| `Ordenacao[C ~string]` + `Direcao` | Enum por entidade; valor inválido cai no default (C-02). |
| `Data` | Layout `2006-01-02`; o parse devolve o erro do campo. |
| `Email` | Trim/lower, formato básico, recusa CR/LF (M-01); `Mascarado()` igual ao `vlog.MaskEmail`. |
| `Dinheiro` | **Centavos `int64`** (ARQ-09); JSON e banco em reais decimais, sem passar por `float64`. Teto `LimiteCentavos` (DECIMAL(15,2)); entrada da API ≥ 0. |
| `Kind` + `Erro` + `Bloqueio` | `KindInterno` (zero), `Invalido`, `NaoEncontrado`, `Conflito`, `Proibido`, `NaoAutenticado`, `Bloqueado` (429), `MuitoGrande` (413). |
| `ComRota`/`RotaDe`/`SanearRota` | Rota do request no ctx só para log; `SanearRota` escapa controles e quebras Unicode (log sem linhas forjadas). |

```go
// kernel/escopo.go: a ÚNICA tradução de Escopo para SQL (O1)
func (e Escopo) Filtro() (vendedorID int64, filtra bool, err error) {
	switch e.tipo {
	case escopoIrrestrito:
		return 0, false, nil
	case escopoVendedor:
		if e.vendedorID <= 0 {
			return 0, false, ErrEscopoInvalido
		}
		return e.vendedorID, true, nil
	case escopoSemAcesso:
		return 0, false, ErrEscopoSemAcesso // não executar SQL
	default:
		return 0, false, ErrEscopoInvalido // zero value: não executar SQL
	}
}
```

- `VendedorID()` serve só para log e para forçar o vendedor em comandos (E-05); `Permite(id)` decide visibilidade de um registro (E-04).
- Todo caso de uso que recebe `Principal` falha fechado se `!p.Valido()` (`ErrPrincipalInvalido`, 500).

### 3.3 Catálogo

| Elemento | Conteúdo |
|---|---|
| Entidades / VOs | `Produto`, `RegistroEstoque`; `SKU` (obrigatório, imutável no update), `Dinheiro`, `OrdenacaoProduto`/`OrdenacaoEstoque` |
| Casos de uso | Produtos (listar, obter, criar, atualizar, alternar ativo), estoque (listar, obter, ajustar, atualizar saldo), imagem (`DefinirImagemProduto`, `RemoverImagemProduto`, `AbrirImagemProduto`), vitrine da loja |
| Portas | `Produtos`, `Estoques`, `ArmazenamentoImagens`, `NormalizadorImagens`, `Transactor[TxImagemProduto]` |
| Erros | `ErrProdutoNaoEncontrado`, `ErrSKUDuplicado`/`ErrEstoqueDuplicado` (Conflito), `ErrImagemInvalida` (400), `ErrImagemMuitoGrande` (`KindMuitoGrande`, 413), `ErrImagemNaoEncontrada` (404) |
| Acesso (C-01) | Leitura de produto Authenticated; escrita de produto e todo o estoque AdminOnly; vitrine e imagens Public |

### 3.4 Identidade e Acesso

| Elemento | Conteúdo |
|---|---|
| Aggregates | `Usuario` (role, ativo, deve trocar senha, id_vendedor, corte de sessão `tokensValidosDesde`); `RefreshToken` (hash, expira, revogado em/motivo, reuso) |
| VOs | `Senha`, `HashSenha`, `TokenOpaco` (`String()`="***", `MarshalJSON` com erro, I-12); `PoliticaSenha` (8 runes, 72 bytes, 3 de 4 classes); `MotivoRevogacao` (rotacao, logout, revogacao_massa, senha, inativacao) |
| Regras | `HistoricoSenha.Reutilizada` sem short-circuit; `RefreshToken.Avaliar` devolve estado (Valido, Expirado, RevogadoRecente, Revogado), expiração antes de revogação (I-06); `Usuario.TokenAnteriorAoCorte(iat)` (I-04) |
| Casos de uso | Login, renovar sessão (rotação + reuso), logout, trocar senha, reset admin, CRUD de usuário, `/me`, histórico de senha, `StatusParaAutenticacao` (checker por request), limpeza de refresh expirados, `ResetarSenhasAtivos` (CLI), cadastro da loja, confirmação/reenvio de e-mail, login e cadastro Google |
| Portas | `Usuarios`, `RefreshTokens`, `HistoricoSenhas`, `Hasher`, `EmissorAcesso`, `GeradorRefresh`, `GeradorSenha`, `VerificadorCaptcha` (com ação), `VerificadorGoogle`, `Limitador`/`Limites`, `ConfirmacoesEmail`, `Publicador`, `EnviadorSenhaInicial`, `EnviadorConfirmacaoEmail`, Transactors (`TxRotacao`, `TxSenha`, `TxAtivoUsuario`, `TxCadastro`) |
| Erros | `ErrCredenciaisInvalidas` (inexistente, inativo e senha errada, mesmo custo de hash), `ErrSessaoEncerrada`, `ErrSenhaFraca`, `ErrSenhaReutilizada`, `ErrRefresh*`, `ErrCaptchaInvalido`, `ErrMuitasTentativas` (Bloqueado + `*kernel.Bloqueio`) |
| Infra | `identidadecrypto` (Argon2id + pepper; bcrypt legado só verifica e marca rehash; JWT HS256; aleatoriedade); Turnstile confere `hostname` e `action` |
| HTTP | `identidadehttp/auth_handler.go` mantém o nome do arquivo e do tipo `AuthHandler` (exigência do TST05) |

### 3.5 Comercial

| Elemento | Conteúdo |
|---|---|
| Aggregates | `Vendedor` (desligado = `data_desligamento`, `Desligar` idempotente), `Carteira` (`Ativo()` = `data_fim` nula), `Cliente` (PJ com CNPJ ou PF da loja com CPF e endereço), `Visita`, `Oportunidade` |
| VOs | `CNPJ` (nunca logado, M-04), CPF/CEP/UF do cliente da loja (nunca ecoados), `EtapaOportunidade`, `ResultadoVisita` |
| Serviço de domínio | `politica.go`: dado `Escopo` e cliente, decide o vendedor da escrita (E-05), função pura |
| Casos de uso | `AutorizadorEscopo.Resolver` / `ResolverParaAnalytics` (D-01); CRUD de cliente, vendedor, carteira, visita e oportunidade |
| UoW | `TxDesligamento` (I-09), `TxClienteNaCarteira` (criar cliente + vínculo), `TxVinculo` (vincular com `FOR UPDATE`), `TxAtivoCliente` |
| Erros | `ErrClienteNaoEncontrado` (também "fora da carteira", E-04), `ErrCNPJDuplicado`/`ErrCPFDuplicado` (409 genérico, E-07), `ErrVendedorDesligado` (403 fixo), `ErrVinculoDuplicado` |

### 3.6 Vendas

| Elemento | Conteúdo |
|---|---|
| Aggregate root | `Pedido` com itens: valores calculados no domínio, nunca do payload (V-03); até `MaxItensPedido` = 200; máquina de status em `PlanejarAtualizacao` |
| Entidade | `Pagamento`: `PedidoID` imutável (V-02); `PodeEditar()`/`PodeExcluir()` (409 se quitado) |
| UoW `TxPedido` | `Pedidos`, `Pagamentos` (`PagamentosNaTx`), `Estoque` (`BaixaEstoque`) e `Loja` (`LojaNaTx`) na mesma `*sql.Tx`; posse checada sob lock; ordem de lock pedido → pagamento |
| Casos de uso | CRUD de pedido e pagamento (escopo herdado de `pedido.vendedor_id`); loja: `FinalizarCompra`, `MinhasCompras`, `MinhaCompra`, `ConfirmarPagamentoLoja`, `ExpirarPedidosLoja` (§11.1) |
| Escopo da loja | Checkout e minhas compras filtram pelo **usuário do token** (`p.UserID()`), não pelo `Escopo` de carteira |

### 3.7 Analytics

- VOs: `Periodo` (`Hoje | Semana | Mes`, parse cai em `Mes`), `Dias`/`Janela` (1..365, D-03).
- Read models tipados (`Metricas`, `SerieVendas`, `Ranking`, `MetricasClientes`, `Funil`) no lugar de `map[string]any`.
- Consultas recebem `Principal` e resolvem `Escopo` via porta; desligado, inexistente ou sem vendedor → 200 zerado com `vendedor_desligado` (D-01).
- `analyticsmysql`: `fmt.Sprintf` só de constantes escolhidas por `switch` sobre enum (D-02); valores por placeholder.

### 3.8 Notificação

- Domínio: `Destinatario` (sem CR/LF), `ModoTLS` (`Implicito | STARTTLS`), `Mensagem`, `Contato`.
- `notificacaoapp.AlertaSegurancaNotifier`: assina o evento de reuso; dedup 30 min, teto 20/h, 4 concorrentes, timeout 30 s (I-08); `Aguardar` no shutdown.
- `notificacaosmtp.SMTP` (AUTH só sobre TLS, M-01) e `Noop` (Null Object, loga e-mail mascarado, M-02).

---

## 4. Regras de dependência

### 4.1 O que pode importar o quê

| Pacote | Pode importar | Não pode importar |
|---|---|---|
| `kernel` | stdlib (sem `database/sql`, `net/http`, `log`) | qualquer pacote do projeto (inclusive `vlog`) ou de terceiros |
| `<ctx>` (domínio) | stdlib, `kernel`, `cnpj`, `tz` | `database/sql`, `net/http`, `encoding/json`, `log`, `config`, `vlog`, `db`, `<ctx>app`, `<ctx>mysql`, outros contextos |
| `<ctx>app` | `kernel`, `<ctx>`, `vlog`, `appkit` | `database/sql`, `net/http`, `config`, `db`, `adapters/*`, `<ctx>mysql`, a API, outros contextos |
| `<ctx>mysql` e demais adapters | `kernel`, `<ctx>`, `db`, `vlog`, drivers | `<ctx>app`, `net/http` (exceto Turnstile/Google), a API |
| `internal/<ctx>http` | `httpx`, `<ctx>app`, `<ctx>`, `kernel`, `vlog` | `database/sql`, `*mysql`, `config` |
| `internal/httpx` | `kernel`, `vlog`, `net/http`, domínio `identidade`, `shared/cache` | `database/sql`, `*mysql`, `config`, `bootstrap` |
| `internal/bootstrap` | tudo (composition root) | — |
| qualquer pacote de `shared` | — | o módulo da API |

- O domínio não tem tags JSON; quem serializa é o DTO HTTP (§5.7). Exceção: os VOs sensíveis implementam `MarshalJSON` com erro.
- A configuração entra na aplicação como struct de opções própria, preenchida pelo bootstrap a partir de `config.Config`.
- A infra satisfaz as portas por tipagem estrutural, sem importar `<ctx>app`; tipo compartilhado mora no domínio.

### 4.2 Testes de arquitetura

Rodam com `make arch-check` (e dentro de `make test`). Leem os pacotes com `go/build` e `go/parser`; toda regra tem controle negativo (fonte sintética que ela precisa reprovar).

| Arquivo | Regra |
|---|---|
| `shared/tests/arquitetura/dependencias_test.go` (+ `pacotes_test.go`) | Camadas por contexto (§4.1), contexto não importa contexto, domínio puro, nenhum importador do legado |
| `shared/tests/arquitetura/codigo_gofmt_e_fim_de_linha_lf_test.go` | Fontes formatadas pelo `gofmt` e com fim de linha LF |
| `shared/tests/arquitetura/logs_auditoria_com_id_sem_pii_test.go` | Logs de auditoria com id e sem PII |
| `rotaperfumes-api/tests/arquitetura/dependencias_test.go` + `varredura_test.go` | `httpx`/`<ctx>http` sem banco nem `config`; só o `bootstrap` liga `*mysql`; `shared` não importa a API; a varredura enxerga os dois módulos |
| `.../allowlists_test.go` | Allowlists de chamadores e ASTs (§4.3) |
| `.../vlog_test.go` | Conteúdo do vlog (O7, §6.3) |
| `.../regras_identidade_metodos_restritos_test.go` | `ResetarSenhasAtivos` só em `internal/resetsenhas`; `RegistrarReuso`/`DesfazerReuso` só em `identidadeapp` |
| `.../regras_escopo_comercial_test.go`, `regras_escopo_vendas_test.go`, `regras_escopo_analytics_test.go` | Listagens e consultas de Comercial, Vendas e Analytics recebem `kernel.Escopo`; `Sprintf` de SQL só com constantes (D-02) |
| `.../regras_sql_concatenacao_constante_test.go` | Sem concatenação `+` de SQL em `QueryContext`/`QueryRowContext`/`ExecContext` nos adapters `*mysql` e em `db` |
| `.../regras_id_parse_estrito_test.go` | Sem parse permissivo de id em `internal/` (só `httpx.IDDoPath`) |

### 4.3 Allowlists e ASTs (`rotaperfumes-api/tests/arquitetura/allowlists_test.go`)

| Teste | Regra |
|---|---|
| `TestARQ_NovoPrincipal_SoHttpx` | `kernel.NovoPrincipal` só em `internal/httpx`; o `httpx` não exporta setter de Principal (O5) |
| `TestARQ_Escopo_SoAutorizador` | Construtores `kernel.Escopo*` só em `comercialapp` e no `kernel` (O5) |
| `TestARQ_Filtro_SoAdapters` | Adapter não decide SQL por `VendedorID()`/`Irrestrito()`; só por `Filtro()` (O1) |
| `TestARQ_ChavesLegadas_SoMiddlewareLegado` | Zero gravadores de chaves antigas; um único `context.WithValue` com `chavePrincipal` no `httpx` (O4) |
| `TestARQ_NovoErro_SentinelaLiteral` | `kernel.NovoErro` só em `var` de pacote, com `Kind*` constante e mensagem literal (O6) |
| `TestARQ_SemErrorsJoinDeDominio` | `errors.Join` proibido em `<ctx>`, `<ctx>app`, `<ctx>mysql` (R-c) |
| `TestARQ_VlogSemConteudoSensivel` | Sem `%v`/`%+v`/`%#v`; sem identificador sensível (senha, token, cnpj, email, hash, captcha, chave…) fora de `MaskEmail`/`Mascarado()` (O7) |

---

## 5. Padrões e convenções

### 5.1 Repository (portas e adapters)

- Uma porta por agregado em `<ctx>app/portas.go`, declarada pelo consumidor; implementação em `<ctx>mysql` com o `db.Querier` no struct.
- O repositório recebe `kernel.Escopo` (nunca `int64` "0 = global") e o enum de ordenação (nunca string de coluna).
- **Não usar** `Repository[T]` genérico nem decorator de cache.

### 5.2 Unit of Work (`shared/db/tx.go`)

```go
type Transactor[R any] struct{ pool *sql.DB; bind func(Querier) R }

func (t *Transactor[R]) WithTx(ctx context.Context, fn func(R) error) error {
	tx, err := t.pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op após Commit
	if err := fn(t.bind(tx)); err != nil {
		return err
	}
	return tx.Commit()
}
```

| UoW | Onde | O que fica na mesma tx |
|---|---|---|
| `TxRotacao` | `identidadeapp` | revogação condicional do refresh + insert do novo (I-06) |
| `TxSenha` | `identidadeapp` | histórico → senha → corte → revogação |
| `TxAtivoUsuario`, `TxCadastro` | `identidadeapp` | inativação + revogação; usuário + confirmação + cliente da loja |
| `TxDesligamento` | `comercialapp` | `data_desligamento` + inativar usuários (corte) + revogar tokens (I-09) |
| `TxClienteNaCarteira`, `TxVinculo`, `TxAtivoCliente` | `comercialapp` | cliente + vínculo; vínculo com `FOR UPDATE` |
| `TxPedido` | `vendasapp` | pedido, itens, pagamentos, baixa de estoque e operações da loja |
| `TxImagemProduto` | `catalogoapp` | `SELECT … FOR UPDATE` + `UPDATE` da imagem |

Regras do `WithTx` (O8, R-d):
1. Dentro da `fn`, só os repositórios de `R`; nada do pool, de outro caso de uso ou capturado de fora.
2. Isolamento padrão (REPEATABLE READ); a tx é cancelada pelo `ctx` da requisição.
3. Nenhum efeito externo (e-mail, evento, cookie) dentro da `fn`: só depois do commit.
4. Condições ficam no `WHERE` (`revoked_at IS NULL`), nunca deduzidas de `RowsAffected` (F-07).

**Fora do UoW:** a detecção de reuso (I-07) é um UPDATE condicional isolado; falha ao marcar corta assim mesmo, e corte incompleto aciona `DesfazerReuso` (compensação).

### 5.3 Factory e Value Object

- A entidade só nasce válida: `catalogo.NovoProduto`, `vendas.NovoPedido`, `comercial.NovoCNPJ`, `kernel.NovoEmail`, `kernel.NovaPagina`.
- Construtores de caso de uso recebem todas as dependências; `nil` dá `panic` na montagem, nunca em runtime (sem setters).
- Só vira VO o que tem regra (CNPJ, CPF, UF, Email, Data, Dinheiro, Senha, Periodo); `Bairro`, `Cidade`, `Nome` ficam `string`.

### 5.4 Specification de escopo

- O escopo é parâmetro **obrigatório** de toda listagem e detalhe, traduzido só por `Escopo.Filtro()` (§3.2).
- O caso de uso trata `SemAcesso` antes do repositório: listagem → 200 vazio (E-03); detalhe → 404 (E-04); escrita → 403 (E-05).
- Teste obrigatório por repositório com escopo: `EscopoSemAcesso()` e `Escopo{}` devolvem erro ou vazio **sem nenhuma query** no sqlmock.

### 5.5 Strategy, Adapter, Observer, Null Object, Template Method

| Padrão | Onde |
|---|---|
| Strategy | hash em `identidadecrypto/hasher.go` (Argon2id gera; bcrypt só verifica); TLS em `notificacaosmtp/conexao.go`; ordenação por enum + `switch` de coluna |
| Adapter | `adapters/turnstile` (fail-closed, I-13), `adapters/googleid`, `adapters/ratelimit`, `adapters/rediscache`, `identidadecrypto/jwt.go`, `notificacaosmtp/smtp.go`, `catalogomysql/faturamento.go` |
| Observer | `identidade.ReusoDeRefreshDetectado` publicado por `identidadeapp` (`PublicadorLocal`); assinante ligado em `bootstrap/email.go` |
| Null Object | `identidadeapp.SemPublicador`, `notificacaosmtp.Noop` |
| Template Method | `importers/importkit` (`Executar`, `Etapas`, `Upsert[R]`); `appkit/releitura.go` (releitura pós-escrita) |
| Command adiado | `Leitor[T]` (`comercialapp`, `vendasapp`) + `httpx.LeitorJSON`: autoriza antes do decode (E-05) |
| CQRS leve | `analyticsapp/consultas.go` com read models próprios |

### 5.6 Erros

```go
// httpx/errmap.go: switch explícito com default fail-closed (O6)
switch de.Kind() {
case kernel.KindInvalido:       JSON(w, 400, nil, de.Error())
case kernel.KindNaoEncontrado:  JSON(w, 404, nil, de.Error())
case kernel.KindConflito:       JSON(w, 409, nil, de.Error())
case kernel.KindProibido:       JSON(w, 403, nil, de.Error())
case kernel.KindNaoAutenticado: JSON(w, 401, nil, de.Error())
case kernel.KindBloqueado:      ResponderBloqueado(w, err) // 429 + Retry-After >= 1s
case kernel.KindMuitoGrande:    JSON(w, 413, nil, de.Error())
default:                        interno(w, tag, err)       // log + 500 "erro interno"
}
```

- Sentinela no domínio: `var ErrX = kernel.NovoErro(kernel.KindY, "mensagem pública")`; a resposta usa a mensagem do sentinela, nunca o texto do wrap.
- Wrap de infra só com o nome da operação (`fmt.Errorf("comercialmysql: inserir cliente: %w", err)`), nunca valores (O6-iv).
- 1062 (duplicado) e 1452 (FK) traduzidos no adapter sem embrulhar o driver (`db.ChaveDuplicada`, `db.SemValorDuplicado`, `db.ViolacaoFK`).
- `errors.Join` de infra com sentinela é proibido (R-c); ninguém compara strings de erro.
- `ErrEscopoInvalido`/`SemAcesso` no 500 logam `[escopo] invalido <tag>`; `ErrPrincipalInvalido` loga `[auth] principal invalido <tag>`.

### 5.7 DTO e handlers

- `internal/<ctx>http/dto.go` tem request/response com tags JSON; o DTO de usuário não tem campo de hash (I-12).
- Handler só decodifica, chama o caso de uso e responde (até ~30 linhas), via `httpx.Responder`/`ResponderPaginado`/`ResponderErro`.
- No handler: `p, ok := httpx.PrincipalFrom(r)`; em rota protegida, `!ok` → 500. O Principal vai explícito à aplicação.

### 5.8 Convenções gerais

- **Casos de uso:** um struct `Casos` por contexto, um método por caso de uso, `ctx` primeiro; 1ª linha `defer vlog.Trace(...)(&err)`; até ~40 linhas.
- **SQL:** constantes no adapter; valores por placeholder; identificadores só de `switch` sobre enum; buscas com `db.TermoContem` + `LIKE ? ESCAPE '!'`.
- **Arquivos:** um agregado por arquivo no domínio; `portas.go` em cada `<ctx>app`; `dto.go` em cada `<ctx>http`; rotas só em `bootstrap/rotas.go`.
- **Nomes:** português para negócio; inglês só para termos técnicos (`Querier`, `Transactor`, `Policy`).

### 5.9 Onde não aplicar

| Não fazer | Por quê |
|---|---|
| Container de DI / reflection | O bootstrap explícito é legível |
| `Repository[T]`, `BaseEntity`, interfaces no produtor | Cada agregado tem consultas próprias; interface pequena no consumidor |
| Decorator de cache em repositório, checker ou escopo | Proibido (R4, I-03, E-01) |
| Specification combinável, mediator, command bus | Nenhum caso precisa |
| Event sourcing, outbox, sagas | Um processo e um banco |
| Agregados nos importers | Carga em lote idempotente usa SQL de upsert próprio |

---

## 6. Logging (vlog)

### 6.1 Requisitos

- vlog desligado por padrão; formato `[arquivo] [Funcao] descrição`; nada de log por iteração (F-09). Detalhes em `docs/logs-verbose.md`.
- TST05: com vlog ligado, a saída contém `[auth_handler.go] [AuthHandler.Login]` e `[AuthHandler.ResetPassword]`, e nenhum segredo.
- Logs operacionais (`log.Printf`, sempre ligados): `[http]`, `[auth]`, `[escopo]`, `[server]`, `[loja]`; são contrato de operação.

### 6.2 Os quatro níveis

| Nível | Quem emite | Exemplo |
|---|---|---|
| 1. Rota | `httpx.Trace`, automático pelo router | `[produto_handler.go] [ProdutoHandler.ListProdutos] início GET` / `fim status=200` |
| 2. Caso de uso | `defer vlog.Trace(...)(&err)` (`shared/vlog/trace.go`) | `início page=1 limit=20` / `fim ok dur=3ms` ou `fim erro=nao_encontrado` |
| 3. Decisão | linha explícita em ramo de negócio/segurança | `escopo=vendedor id=7`, `rate limit bloqueado retry=42s` |
| 4. Query | 1 linha por operação de repositório | `SELECT produtos linhas=20 total=37` |

O `vlog.Trace` classifica o erro pelo `KindNome()` do `kernel.Erro` (interface declarada no próprio `vlog`), sem depender do kernel.

### 6.3 O que cada nível pode logar (O7)

| Nível | Pode | Nunca |
|---|---|---|
| Rota | método, status, rótulo | query string, headers, cookies, corpo |
| Caso de uso | ids, contagens, page/limit, enums, booleanos, `Kind`; quantidade numérica digitada (saldo) | `%v` de comando/entidade/filtro; texto digitado (no máximo `len()` ou `busca=sim/não`) |
| Decisão | ids, papel, `retry` em segundos, `token_id`, e-mail só mascarado | chave do rate limit, tokens, hash, JWT, senha, CNPJ/CPF, IP, User-Agent |
| Query | nome da operação e contagens | texto do SQL, args, valores de filtro |

- Proibido: vlog de sintaxe, `if cfg.Verbose { log.Printf }` e vlog no `kernel`/domínio.
- No log operacional, texto livre vai com `%q` (CR/LF não forja linhas).

---

## 7. Kit HTTP (`rotaperfumes-api/internal/httpx`)

### 7.1 Router e políticas

| Policy | Quem passa |
|---|---|
| `Public` | Sem `Autenticador`, sem Principal no contexto; não consulta o checker |
| `Authenticated` | `admin` e `normal`; o role `usuario` (loja) recebe 403 |
| `AdminOnly` | Só `admin` |
| `AnyRole` | Qualquer usuário autenticado, inclusive `usuario` |

- `Router.Handle` dá `panic` na montagem para política, método (fora de GET/POST/PUT/PATCH/DELETE), padrão, handler ou rótulo inválidos (O2).
- Cada rota é `Exigir(policy)(limitarCorpo(Trace(handler)))`: 401/403 saem antes do 413 e não emitem o rótulo vlog (O9).
- `Rota.MaxCorpo` (0 = 256 KiB; negativo → panic); `Rota.LiberaTrocaSenhaPendente` (só reset-password e `/me`; em Public → panic).
- `GET` também casa `HEAD`; método errado → 405 do `ServeMux`, sem consultar o checker.

### 7.2 Cadeia (Chain of Responsibility)

`Logging( CORS( SecurityHeaders( mux → Exigir(policy)( limitarCorpo( Trace( handler ))))))`

- **CORS (O3):** todo `OPTIONS` → 204 antes do mux; `Access-Control-Allow-*` só para origem da allowlist exata (sem `*`/`null`, F-02).
- **SecurityHeaders (F-03):** nosniff, DENY, Referrer-Policy, CSP, HSTS (exceto localhost); valem também no 405.
- **Logging:** `[http] método caminho -> status (duração)`, sem query string, caminho saneado por `kernel.SanearRota`.

### 7.3 Arquivos

| Arquivo | Responsabilidade |
|---|---|
| `router.go`, `chain.go` | `Policy`, `Rota`, `Router`, `Rotas()` (fonte da matriz); `Middleware`, `Chain` |
| `auth.go`, `principal.go` | `Autenticador`: JWT (`ValidadorAcesso`), ativo/role/corte/`deve_trocar_senha` no banco a cada request (`ConsultaStatus`); `PrincipalFrom` |
| `errmap.go` | `ResponderErro`, `ResponderBloqueado` (§5.6) |
| `respond.go`, `casos.go`, `body.go` | Envelope `{success, data, error, pagination}`; `Responder*`; `LeitorJSON`, `DecodificarJSON` |
| `query.go` | `PaginaDaQuery`, `OrdenacaoDaQuery`, `BoolDaQuery`, `BuscaDaQuery` (`q` > 100 runes → 400), `IDDoPath`/`ParseID` (só dígitos, > 0) |
| `clientip.go`, `cookies.go` | IP (XFF só com `TRUST_PROXY_HEADERS`); cookies `access_token` (Strict) e `refresh_token` (Lax), HttpOnly (I-15) |
| `logging.go` | `Logging` e `Trace` |
| `cache.go` | `ServirComCache`: só GET 200, cabeçalho `X-Cache` HIT/MISS/BYPASS (§11.3) |

- Ordem do `Autenticador`: 401/500 → 403 "troca de senha obrigatória" → 403 de papel. Bearer tem precedência sobre o cookie (I-02).
- Rota `/api/senha-historico/{user_id...}`: id vazio → 400 "obrigatório"; `/2/extra`, `0`, negativo → 400 "usuario_id inválido" (R-f).

### 7.4 Rotas (`internal/bootstrap/rotas.go`)

São **77** rotas na tabela declarativa: 14 Public, 34 Authenticated, 23 AdminOnly, 6 AnyRole. A matriz `rotaperfumes-api/tests/routes/rotas_matriz_politicas_test.go` compara método, padrão e política com `Router.Rotas()`, nos dois sentidos.

| Grupo | Política |
|---|---|
| `/api/auth/{login,refresh,logout,cadastro,google,google/cadastro,confirmar-email,reenviar-confirmacao}` | Public |
| `/api/auth/{me,reset-password}` | AnyRole (liberam troca de senha pendente) |
| `/api/usuarios*`, `/api/admin/reset-password`, `/api/senha-historico*`, `/api/estoque*` | AdminOnly |
| `/api/dashboard/{metrics,vendas,vendedores,clientes,funil}` | Authenticated |
| `/api/vendedores` (GET lista e `/{id}/clientes` GET) | Authenticated; demais rotas de vendedor AdminOnly |
| `/api/clientes*`, `/api/pedidos*`, `/api/pagamentos*`, `/api/oportunidades*`, `/api/visitas*` | Authenticated (com escopo por carteira) |
| `/api/produtos` GET; POST/PUT/PATCH e `/{id}/imagem` | Authenticated; AdminOnly |
| `/api/loja/{produtos,produtos/{id},filtros}`, `/api/public/produtos/imagens/{arquivo}` | Public |
| `/api/loja/{minhas-compras,minhas-compras/{id},checkout,pagamentos/{id}/simular}` | AnyRole |
| `/health`, `/api/ambiente` | Public |

---

## 8. Composition root (`rotaperfumes-api/internal/bootstrap`)

| Arquivo | Conteúdo |
|---|---|
| `server.go` | `Run`: abre o pool, `Montar`, sobe limpeza de refresh e expiração da loja, `http.Server`; `Montar` liga e-mail, notifier, Identidade, handlers e a cadeia |
| `rotas.go` | `HandlerHTTP` (CORS → SecurityHeaders → mux, sem Logging; usado nos testes), `NovoRouter`, tabela de rotas, `/health`, `/api/ambiente` |
| `handlers.go` | Handlers de todos os contextos, armazenamento de imagens, limites da vitrine e do checkout, cache Redis |
| `identidade.go`, `comercial.go`, `vendas.go`, `analytics.go`, `email.go` | Um `Novo<Contexto>` (casos de uso sobre o pool) por contexto; SMTP ou Noop |
| `background.go` | Parada das tarefas de fundo |

- **Singletons por processo (I-14):** limitadores (login, refresh, reset, cadastro, confirmação, vitrine, checkout), notifier, hasher, JWT, Turnstile, Google, cache.
- **Timeouts (F-04):** HTTP 15/15/60 s, `ReadHeaderTimeout` 5 s, cabeçalhos até 64 KiB; ping do banco 5 s.
- **Shutdown:** servidor (10 s) → tarefas de fundo (até 10 s) → alertas em andamento (até 10 s) → `pool.Close()`.
- O checker de status por request é o próprio `*identidadeapp.Casos`; o escopo de Vendas e Analytics é o mesmo `comercialCasos.Escopo()`.
- Falhas de montagem (SMTP inválido, diretório de imagens inacessível) → `log.Fatalf` na subida.

---

## 9. Banco e infraestrutura compartilhada

### 9.1 Schema e scripts (`sql/`)

| Arquivo | Uso |
|---|---|
| `sql/schema.sql` | Schema completo na versão atual (`make db-up`, `deploy/seed-producao.sh`) |
| `sql/migracao.sql` | Idempotente e não destrutivo: leva um banco existente até o schema atual (`make db-migrar`, estágio `Migracoes` do Jenkins) |
| `sql/seed_*.sql` | Seeds de desenvolvimento (`seed_admin`, `seed_vendedores`, `seed_pedidos`, `seed_movimentacoes_mes_atual`; `make db-seed`) |

- Mudança de schema = editar `schema.sql` **e** `migracao.sql`. Não há revert: volta pelo backup.
- Guardas de produção, TLS e regras dos alvos `make db-*`: ver `CLAUDE.md` e `docs/deploy-servidor.md`.

### 9.2 `shared/db`

| Arquivo | Conteúdo |
|---|---|
| `db.go` | `Open` (pool, ping 5 s, DSN mascarado no log; se a senha aparecer, só `****`, F-08) |
| `tx.go` | `Querier` (`*sql.DB` ou `*sql.Tx`), `Transactor[R]`/`NewTransactor` (§5.2) |
| `consulta.go` | `Linha`, `Consultar[T]`, `Paginada`, `Where`, `DirecaoSQL` |
| `linhas.go` | `ExecPorID`, `Existe`, `Inserir`, `LerUma[T]` |
| `duplicada.go`, `fk.go` | `ChaveDuplicada`, `SemValorDuplicado` (1062); `ViolacaoFK` (1452) |
| `like.go` | `TermoContem`: escapa `!`, `%`, `_` para `LIKE ? ESCAPE '!'` |

---

## 10. Importadores, CLIs e testes

### 10.1 Importadores e CLIs

- `importers/importkit`: `Executar(Etapas, dryRun, abrir)` fixa ler → (dry-run para) → abrir/pingar → gravar; cada importer mantém o próprio SQL de upsert.
- `LerCSV`/`LerArquivo` pulam linhas inválidas sem logar valores (M-04); `Upsert[R]` com prepare único e contagens.
- **`main` fino:** os 15 `main` de `shared/cmd/*` e `rotaperfumes-api/cmd/{resetsenhas,gerarimagens}` só chamam `os.Exit(cmdutil.Executar(...))`.
- `shared/cmdutil/cli.go`: `Ambiente` (stdio, `.env`, config com/sem credenciais, abertura do banco), `Comando`, `Executar` (saída 0/1/2), `Configurar`.
- `resetsenhas` usa `identidadeapp` (simula por padrão, `-executar`; M-03); `resetpassword`/`seedusers` usam `identidadecrypto` para o hash.

### 10.2 Como rodar os testes

- **Unitários:** `make test` (dois módulos). Cobertura combinada API+shared: `make cover-combinada`.
- **Integração:** `make test-integration ENV_ARQUIVO=<arquivo do banco descartável>` (`-p 1 -count=1`, banco `rotaperfumes_teste` recriado e apagado no fim).
- Todos os testes de integração entram por `shared/testkit/integracao` (`Abrir`/`Config`): sem `INTEGRATION=1` e `DB_*` do ambiente, são pulados.
- O helper recusa a produção antes de conectar e só aceita loopback ou o CasaOS `192.168.168.106:3307` (SEC-50, SEC-54); nenhuma válvula libera produção.
- Não carregue o `.env` no shell: os segredos só chegam ao `go test` pelo `make test-integration` (SEC-53, SEC-55).

---

## 11. E-commerce, imagens e cache

### 11.1 Loja

- **Cadastro (`identidadeapp/cadastro.go`):** cria usuário `usuario`, confirmação de e-mail e cliente PF da loja (`comercial/cliente_loja.go`, segmento "Loja Online") na mesma `TxCadastro`.
- **Google (`identidadeapp/google.go` + `adapters/googleid`):** valida o ID token; sem Client ID → `ErrGoogleIndisponivel`.
- **Vitrine (`catalogohttp/loja_handler.go`):** só produtos ativos, sem custo, com limite por IP; detalhe e filtros públicos.
- **Checkout (`vendasapp/checkout.go`):** preços do banco, produtos travados `FOR UPDATE` em ordem crescente, chave de idempotência, vendedor "Loja Online", endereço de entrega obrigatório.
- **Pagamento (`vendasapp/pagamento_loja.go`):** pedido nasce `Aguardando pagamento`; aprovado → `Em separação` + `Pago`; recusado → `Cancelado` + `Recusado`.
- O simulador `POST /api/loja/pagamentos/{id}/simular` só existe com `APP_ENV` dev/qa (senão 404); dono do pedido ou admin.
- **Expiração (`vendasapp/expiracao.go`):** job na API (`LOJA_EXPIRACAO_INTERVAL`, padrão 1h, `0` desliga) cancela pedido com pagamento vencido e libera a reserva.

### 11.2 Imagem do produto

- **HTTP (`catalogohttp/imagem_handler.go`):** policy → `IDDoPath` → anti-CSRF (`X-Requested-With` e `Origin`) → multipart em streaming, uma parte `imagem`, teto 1 MB.
- **Leitura pública:** sem `http.FileServer`; nome por regex `<uuid-v4>.(jpg|png|webp)`, `http.ServeContent`, `Cache-Control: immutable`, CORP `same-site`.
- **Aplicação (`catalogoapp/imagens.go`):** grava o novo → tx com `FOR UPDATE` → `UPDATE` → commit → remove o antigo (best-effort); falha antes do commit apaga o novo.
- **`catalogoimagem`:** magic bytes → `DecodeConfig` (jpeg, png, webp) → limites (1..4096 px, W×H ≤ 16M) → decode → reencode de JPEG/PNG. Fica em adapter porque usa `net/http`.
- **`catalogofs`:** tudo via `*os.Root` em `PRODUTO_IMAGENS_DIR`; nome UUID v4; escrita atômica (`.tmp-<uuid>`, `O_EXCL`, `Sync`, `Rename`); 0750/0640.
- **CLIs:** `cmd/gerarimagens` (PNG com o nome do produto; `make db-gerar-imagens-produtos`) e `cmd/limpezaimagens` (`tools/limpezaimagens`).

### 11.3 Cache (CACHE-01)

- `shared/cache` + `adapters/rediscache`: só as leituras públicas da loja (vitrine 5 min, filtros 15 min, imagem 1 h), catálogo em `cache.Areas`.
- Chave `CACHE_APPNAME:area:vN:hash`; invalidar = `INCR` da versão da área; fail-open (erro no Redis desliga o cache por uma pausa).
- Detalhes e operação em `docs/cache-redis.md`.

---

## 12. Invariantes de segurança (resumo)

Referência completa: `docs/arq-00-invariantes-seguranca.md`.

| Grupo | Regra principal |
|---|---|
| R1–R4 | Nenhum nome de teste some; matriz de rotas verde; erro de infra → 500 "erro interno"; sem cache de usuário/escopo |
| F-01..F-10 | Ordem da cadeia; CORS exato; headers; timeouts; envelope e mapper únicos; DSN mascarado; vlog desligado; paginação limitada |
| C-01..C-03 | Produto: leitura Authenticated, escrita AdminOnly; ordenação por enum; LIKE por placeholder com escape |
| I-01..I-16 | Só HS256; Bearer antes do cookie; banco a cada request; corte de sessão; refresh 32 B com SHA-256, TTL 7 d e rotação; reuso fail-closed; notifier limitado; Argon2id + pepper; política de senha; Turnstile; rate limit 429 + `Retry-After`; cookies HttpOnly; login sem enumeração |
| E-01..E-07 | Escopo por request; sem zero value útil; SemAcesso → vazio/404/403; fora da carteira = 404; escrita força o vendedor do escopo; CNPJ duplicado 409 genérico |
| V-01..V-03 | Escopo de pedido e pagamento; `PedidoID` imutável; totais calculados no domínio |
| D-01..D-03 | Desligado → 200 zerado; `Sprintf` só de constantes; janela 1..365 |
| M-01..M-04 | SMTP com TLS e AUTH só sobre TLS; senha gerada nunca em log; `resetsenhas` simula por padrão; CNPJ nunca logado |

---

## 13. Ressalvas aceitas

- **Ponteiro nil tipado (N-7):** construtores que checam `dep == nil` não detectam interface com ponteiro nil tipado; o panic viria na 1ª chamada.
- A proteção é o composition root, que só passa ponteiros novos (`NovoDashboard`, `NovoAutorizadorEscopo`, `*identidadeapp.Casos`). Sem card aberto.
