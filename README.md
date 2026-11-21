# RotaPerfumes

CRM/ERP de vendas de perfumes com loja online: API REST em Go e frontend Next.js, sobre MariaDB.
Equipe interna (admin e vendedores) gerencia clientes, carteiras, pedidos, pagamentos, estoque e dashboard; clientes compram pela loja.

## Stack

| Camada | Tecnologia |
| --- | --- |
| API | Go (versão da diretiva `go` dos `go.mod`), `net/http`, arquitetura DDD (`docs/arquitetura-ddd.md`) |
| Frontend | Next.js 16, React 19, TypeScript, Tailwind 4, Vitest |
| Banco | MariaDB 11.4 LTS (driver `go-sql-driver/mysql`, TLS obrigatório fora do loopback) |
| Cache | Redis, só para as leituras públicas da loja (fail-open; `docs/cache-redis.md`) |
| CI/Deploy | Jenkins (`Jenkinsfile`), Docker Compose + Caddy (`deploy/`) |

## Estrutura

| Pasta | Conteúdo |
| --- | --- |
| `backend/rotaperfumes-api/` | API HTTP (`cmd/server`), rotas e handlers (`internal/`), testes (`tests/`) |
| `backend/shared/` | Domínio por contexto (`catalogo`, `comercial`, `identidade`, `vendas`, `analytics`), banco, config, importadores e CLIs (`cmd/`) |
| `frontend/` | App Next.js (`src/app`, `src/components`, `src/lib`) e testes (`tests/`) |
| `sql/` | `schema.sql` (schema completo), `migracao.sql` (atualização idempotente) e `seed_*.sql` |
| `dados/` | CSVs de origem do CRM/ERP para os `make db-import-*` |
| `deploy/` | Scripts e arquivos do servidor (MariaDB em Docker, seed de produção, backup, restauração) |
| `docs/` | Arquitetura, manual do banco, deploy, SSL, logs e roteiros de teste manual |
| `postman/` | Collection da API e guia de uso (`postman/README.md`) |
| `tarefas/` | Kanban (`afazer.md`, `fazendo.md`, `feito.md`) |

## Começando

```bash
make setup-check                     # confere Go, Node, cliente mariadb, Postman CLI...
make setup                           # instala o que faltar (Linux: scripts/setup-linux.sh; Windows: scripts/setup-windows.ps1)
cp backend/.env.example backend/.env # preencha DB_*, JWT_SECRET, PASSWORD_PEPPER...
make db-reset                        # recria o banco de dev (schema + seeds + hashes)
make db-rebuild                      # ou: recria e importa todos os CSVs de dados/
make dev-api                         # API em http://localhost:8080
make dev-frontend                    # frontend em http://localhost:3000 (frontend/.env: NEXT_PUBLIC_API_URL)
```

O `.env` fica em `backend/` (nunca na raiz). Admin do seed: `admin@rotaperfumes.com.br`, senha do `SEED_ADMIN_PASSWORD` (ou a impressa pelo `db-reset`), com troca obrigatória no 1º login.

## Banco de dados

| Comando | O que faz |
| --- | --- |
| `make db-up` | Cria as tabelas vazias a partir de `sql/schema.sql` |
| `make db-reset` / `make db-rebuild` | Recria o banco de dev (o rebuild também importa `dados/`) |
| `make db-migrar` | Leva um banco existente à versão atual (`sql/migracao.sql`, idempotente; sem revert, volta pelo backup) |
| `make db-import-*` | Importa um CSV de `dados/` (upsert idempotente) |

Mudança de schema = editar `sql/schema.sql` **e** `sql/migracao.sql`. As guardas do Makefile recusam escrever na produção sem `CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME>` e nunca rodam `db-reset`/`db-seed` lá. Detalhes em `docs/manual-base-de-dados.md`.

## Testes

```bash
make test                                  # testes Go (unitários)
make test-integration                      # integração Go num banco descartável (rotaperfumes_teste)
make test-frontend                         # lint + vitest
make lint                                  # go vet
```

Os nomes dos arquivos de teste descrevem a característica testada (prefixo por tema); o card de origem fica no comentário do arquivo. Roteiros manuais por tipo em `docs/roteiro-teste-*.md`; a API pode ser exercitada pela collection do `postman/`.

## Deploy e ambientes

- `APP_ENV` (`dev`, `qa`, `producao`; ausente = `producao`): dev/qa ligam o simulador de pagamento da loja e o selo do rodapé.
- O Jenkins roda os testes, valida a CA do banco, aplica `sql/migracao.sql` (estágio `Migracoes`) e sobe os containers.
- Servidor, seed inicial da produção, backup e restauração: `docs/deploy-servidor.md`. SSL: `docs/Instal_SSL.md`.

## Como o projeto é tocado

O trabalho passa por uma equipe de agentes (MegaBrain, SubBrain, DataBrain, BackBrain, FrontBrain, TestBrain e SecBrain, em `.claude/agents/`) e pelo Kanban em `tarefas/`. Regras e comandos para quem trabalha no código: `CLAUDE.md`. Antes de cada deploy e mensalmente: `make deps-audit`.
