# Logs verbose de rastreamento (`VERBOSE`)

> Código: `backend/shared/vlog/vlog.go` (formato, `MaskEmail`, `OcultarEmail`), `backend/shared/vlog/trace.go` (nível 2), `backend/rotaperfumes-api/internal/httpx/logging.go` (nível 1 e log `[http]`), `backend/shared/config/config.go` (`cfg.Verbose`), `frontend/src/lib/vlog.ts`.
> Detalhe do modelo de 4 níveis: `docs/arquitetura-ddd.md` §6.

## 1. O que é

Log de desenvolvimento que diz **o que** o código faz, sem expor **o valor** dos dados. Desligado (padrão), não custa nada: a flag é checada antes de formatar.

## 2. Formato e níveis (backend)

Formato (LOG-04): `2026/11/13 14:05:09.123456 [arquivo] [Funcao] descrição` (API com microssegundos; CLIs e frontend sem).

| Nível | Quem emite | Exemplo |
|---|---|---|
| 1. Rota | automático: `httpx.Router` envolve todo handler com `httpx.Trace`, dentro da autenticação (401/403 não emitem) | `[produto_handler.go] [ProdutoHandler.ListProdutos] início GET` / `fim status=200` |
| 2. Caso de uso | `defer vlog.Trace(...)(&err)` na 1ª linha dos métodos `<contexto>app`; no fim, duração e só o `Kind` do erro | `[produtos.go] [Catalogo.ListarProdutos] início page=1 limit=20 busca=false ordem=sku` |
| 3. Decisão | `vlog.Printf` em cada ramo de negócio ou segurança | `[produtos.go] [Catalogo.AlternarAtivoProduto] decisão ativo=false id=7` |
| 4. Query | uma linha por operação dos adapters `<contexto>mysql`: nome e contagens | `[produtos.go] [Produtos.Listar] SELECT produtos linhas=20 total=37` |

Sempre ligados (fora do `vlog`): `[http] MÉTODO caminho -> status (duração)`, `[auth] ...`, `[escopo] ...`, `[server] ...`; o caminho passa por `kernel.SanearRota` (controle escapado).

## 3. Como ligar

- **Backend:** `VERBOSE=true` (ou `LOG_LEVEL=debug`) em `backend/.env` e reinicie (`make dev-api`); a API avisa no startup. Os CLIs ligam via `cmdutil.Configurar` (`backend/shared/cmdutil/cli.go`).
- **Frontend:** `NEXT_PUBLIC_VERBOSE=true` em `frontend/.env.local` (modelo `frontend/.env.local.example`); embutido no build, aparece no DevTools.
## 4. Não usar em produção

Enche o disco, amplifica ataques de login e, no frontend, expõe o rastreamento a qualquer usuário. Só em dev ou investigação pontual.

## 5. O que pode e o que nunca é logado

| Nível | Pode | Nunca |
|---|---|---|
| 1. Rota | método, status, rótulo | query string, headers, cookies, corpo |
| 2. Caso de uso | IDs, contagens, `page`/`limit`, enums, booleanos, `Kind` | `%v`/`%+v`/`%#v` de comando, entidade, DTO ou filtro; texto digitado só como `len()` ou `busca=sim/não` |
| 3. Decisão | IDs, papel, booleanos, `retry`, `token_id`, e-mail só via `vlog.MaskEmail`/`Email.Mascarado()` | chave do rate limit, tokens, hash, JWT, senha (nem mascarados), CNPJ, IP, User-Agent |
| 4. Query | nome da operação e contagens | texto do SQL, args, valores de filtro |

- **Nunca (backend):** struct/map/slice de entidade (`json:"-"` não protege), corpo, headers, cookies, `cfg.DSN()` (só `maskDSN`), segredos (`JWT_SECRET`, pepper, SMTP, Turnstile), senhas e hashes, tokens, CNPJ, conteúdo de planilhas.
- Erro do SMTP pode citar o destinatário: passe por `vlog.OcultarEmail(err, emails...)` (`[email-usuario]`). No log operacional, texto livre (ex.: SKU) vai com `%q`.
- **Nunca (frontend):** senhas, token do captcha, `access_token`/`refresh_token`, objeto `User`, e-mail ou CNPJ de formulários, corpo de request/response no `apiClient`.

## 6. Proibido e como adicionar

- Proibido: vlog por `if`/atribuição, `if cfg.Verbose { log.Printf(...) }` e vlog no `kernel` ou no domínio. Em loops, logue antes e depois, com a contagem; `return` não tem log próprio.
- Handlers não precisam de vlog (nível 1 automático). CLIs novos: `cmdutil.Executar` no `main` e `cmdutil.Configurar` na lógica.

```go
func (c *Casos) ObterProduto(ctx context.Context, id int64) (p *catalogo.Produto, err error) {
	defer vlog.Trace(arqProdutos, "Catalogo.ObterProduto", "id=%d", id)(&err)
	return c.produtos.PorID(ctx, id)
}
vlog.Printf(arqProdutos, "Catalogo.AlternarAtivoProduto", "decisão ativo=%t id=%d", p.Ativo, id)
vlog.Printf(arqProdutos, "Produtos.Listar", "SELECT produtos linhas=%d total=%d", len(itens), total)
```

```ts
import { vlog } from "@/lib/vlog";
vlog("MeuComponente.tsx", "MeuComponente.handleSalvar", "salvando pedido itens=" + itens.length);
```

## 7. Testes

- `backend/shared/tests/vlog/` (`vlog_test.go`, `trace_test.go`, `vlog_runtime_test.go`, `ocultar_email_smtp_test.go`): flag, formato, `Trace`, `MaskEmail`, `OcultarEmail`.
- `backend/rotaperfumes-api/tests/arquitetura/vlog_test.go` (`TestARQ_VlogSemConteudoSensivel`): AST recusa `%v` e argumentos sensíveis sem máscara. `tests/handlers/auth_vlog_sem_dados_sensiveis_test.go`: login e reset não vazam nada.
- Rodar: `make test` (ou `make arch-check`).
