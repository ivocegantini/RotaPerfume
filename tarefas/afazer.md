# A Fazer

> Só cards pendentes, resumidos; o histórico fica em `feito.md`. Card com `sudo` vira **execução assistida** (comando pronto; o usuário aprova e digita a senha).
> Caminhos: `SH` = `backend/shared`.

## Resumo

| Card | Prioridade | Situação | Responsável |
| --- | --- | --- | --- |
| DEPLOY-10 | ALTA | migrações 27, 28/29, 30, 31 e 33 + `APP_URL` e `GOOGLE_CLIENT_ID` na produção antes do deploy | 🌸 DataBrain |
| SEC-60 | MÉDIA | decisão do usuário | 🟣 SecBrain |
| SEC-76 | ALTA | Redis do cache sem senha na LAN | 🟣 SecBrain |
| MERGE-01 | MÉDIA | portar os cards da `lote-42` que ficaram fora do merge | 🔵 SubBrain |
| DEP-03 | BAIXA | acompanhamento do upstream | 🟢 FrontBrain |

---

## DEPLOY-10: migrações 27 e 28/29 (loja) na produção — ALTA
**Ação:** CasaOS :3307 já aplicado (27, 28/29, 33). Na produção: `make db-fix-role-usuario`, `make db-fix-confirmacao-email` `make db-fix-clientes-loja`, `make db-fix-clientes-numero` e `make db-fix-pedidos-canal-loja` (ECOM-06), com `CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME>`, **antes** de subir a API do ECOM-01. Google: criar o Client ID com a origem https de produção, pôr `GOOGLE_CLIENT_ID` no `api.env` e no build do frontend (compose já lê; falta o parâmetro no `Jenkinsfile`/`setup-servidor.sh`) (sem ela, o cadastro falha no INSERT).

## SEC-60: PII do cliente da loja na conta `casaos_ro` — MÉDIA
**Ação:** `clientes` está em `TABELAS_RO` e agora guarda CPF e endereço de pessoas físicas (e o admin agora os edita, ECOM-06). Decidir se a `casaos_ro` perde o SELECT em `clientes` (ou ganha uma view sem `cpf`/`endereco`/`cep`).

## SEC-76: Redis do cache (CACHE-01) sem senha — ALTA
**Ação:** o `192.168.168.106:48110` aceita comandos sem AUTH; qualquer host da LAN pode envenenar a vitrine ou apagar chaves. Ligar `requirepass`/ACL (usuário só com GET/SET/INCR em `rotaperfumes*`), pôr `REDIS_PASSWORD` no `.env`/`api.env` e restringir a porta no firewall.

## MERGE-01: cards da branch `lote-42` fora do merge de 2026-11-16 — MÉDIA
**Contexto:** a `lote-42` refez os Lotes 37–41 em paralelo; no merge venceu a versão atual (decisão do usuário). Entraram: DB-09 (migração renumerada para `sql/32`, `make db-fix-historico-created-at`), CI-01 (helper), specs `tarefas/lote-42-*.md`/`lote-43-spec.md`.
**Ação:** portar sobre a versão atual, card a card, com testes: BACKUP-01/02 (`backup-banco.sh`, `verificar-backup.sh`, `restaurar-maquina.sh` sem paridade com o `mariadb-docker.sh` atual), DEPLOY-10 (conta DBA, sem `--dev-ip`), BUG-30/31/32, DB-08, CI-02, SEC-58/61/62/63/71 da lote-42. Fonte: `git show origin/lote-42:<arquivo>`. Os números SEC-60 a 75 da lote-42 colidem com os daqui: renumerar ao portar.

## DEP-03: vulnerabilidades de dev restantes no frontend — BAIXA (acompanhamento)

**Status (2026-11-15):** reconferido; upstream sem correção (`typescript-eslint` 8.72.2 vulnerável, plugins sem `eslint` 10). Patches aplicados: tailwind 4.3.5, postcss 8.5.33, user-event 14.6.10. 0 altas; 8 moderadas do `minimatch` via `eslint` (só dev), correção só com `--force`.
**Ação:** quando o upstream corrigir, atualizar ou usar `overrides`; validar `tsc`, `eslint`, vitest, `next build` e `npm audit`.
**Também bloqueado:** `eslint` 10 (o 9 avisa "no longer supported"), até os plugins react/import/jsx-a11y aceitarem o 10.
**Sem `--force`:** ele rebaixa o `eslint-config-next` 16.4 → 15.5 (fora da linha do `next` 16); o `typescript-eslint` mais novo (8.72.2) ainda é vulnerável.
