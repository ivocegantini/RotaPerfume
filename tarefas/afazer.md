# A Fazer

> Só cards pendentes, resumidos; o histórico fica em `feito.md`. Card com `sudo` vira **execução assistida** (comando pronto; o usuário aprova e digita a senha).
> Caminhos: `SH` = `backend/shared`.

## Resumo

| Card | Prioridade | Situação | Responsável |
| --- | --- | --- | --- |
| DEPLOY-09 | MÉDIA | execução assistida | usuário + 🌸 DataBrain |
| DEP-06 | BAIXA | pendente | 🟡 BackBrain |
| SEC-59 | BAIXA | decisão do usuário | 🟣 → 🌸 |
| TST-26 | BAIXA | pendente | 🔴 TestBrain + 🟣 |
| DOC-10 | BAIXA | só o usuário edita | usuário |
| DEP-03 | BAIXA | acompanhamento do upstream | 🟢 FrontBrain |

---

## DEPLOY-09: aplicar o Lote 39 (BACKUP-01, DEP-05, SEC-47) — MÉDIA

**Ação (execução assistida):**
1. Restauração real, numa máquina com Docker que não seja o servidor: `cd backend/shared && sudo -E env "PATH=$PATH" BACKUP01_DOCKER=1 go test -count=1 -run TestBACKUP01_RestauracaoReal_Docker ./tests/deploy/`.
2. Build dos Dockerfiles (SEC-47): `docker build backend/`, `docker build frontend/`, `docker build -f deploy/Dockerfile.caddy deploy/`.
3. No servidor, depois do push: `sudo git -C /root/rotaperfumes pull --ff-only && cd /root/rotaperfumes && sudo bash deploy/mariadb-docker.sh --recriar --backup-agora`; timer diário da §9 de `docs/deploy-servidor.md`.

---

## DEP-06: patches novos do Go e do Node — BAIXA

**Descrição:** `make deps-audit-imagens` mostra `golang:1.26-alpine` e `node:22.23-alpine` mais novos que os fixados.
**Ação:** subir versão + digest juntos (`Jenkinsfile`, `Dockerfile`s, `go.mod`/`.nvmrc` se for o caso); `make test`.

---

## SEC-59: `casaos_ro` na produção e PII de `clientes` — BAIXA

**Decisão do usuário:** a produção roda sem `--dev-ip`, então a conta `_ro` não existe lá; ativar muda essa decisão (`--dev-ip 192.168.168.105`, rodar de novo após o seed).
**Ação opcional:** views sem CNPJ/contatos de `clientes` para a conta `_ro`.

---

## TST-26: fuso opt-in no `integracao.Abrir` e `test-integration` concorrente — BAIXA

**Descrição:** a rodada inteira em UTC (TST-23) só foi feita com patch local; duas execuções simultâneas apagam o mesmo `rotaperfumes_teste`. A catraca TST-23 age por arquivo.
**Ação:** gancho opt-in de fuso (revisado pelo 🟣, SEC-50) e trava/nome por execução no `make test-integration`.

---

## DOC-10: `CLAUDE.md` com as versões e digests do Lote 39 — BAIXA

**Sugestão (só o usuário edita):** na seção "Versões", citar `GO_VERSION`/`GO_DIGEST` do `backend/Dockerfile`, `NODE_DIGEST` do `frontend/Dockerfile` e `MARIADB_CHAVE_FINGERPRINT` em `scripts/versoes.conf` (mudam junto com a versão).

---

## DEP-03: vulnerabilidades de dev restantes no frontend — BAIXA (acompanhamento)

**Status (2026-11-14):** 0 altas; 8 moderadas do `minimatch` via `eslint` (só dev), correção só com `--force`.
**Ação:** quando o upstream corrigir, atualizar ou usar `overrides`; validar `tsc`, `eslint`, vitest, `next build` e `npm audit`. Nunca `npm audit fix --force`.
