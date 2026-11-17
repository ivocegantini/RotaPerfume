# A Fazer

> Só cards pendentes, resumidos; o histórico fica em `feito.md`. Card com `sudo` vira **execução assistida** (comando pronto; o usuário aprova e digita a senha).
> Caminhos: `SH` = `backend/shared`.

## Resumo

| Card | Prioridade | Situação | Responsável |
| --- | --- | --- | --- |
| DEPLOY-10 | ALTA | estágio `Migracoes` pronto; falta o push na `main` e conferir o 1º deploy | usuário |
| MERGE-01 | MÉDIA | portar os cards da `lote-42` que ficaram fora do merge (BUG-30/31 e DB-08 feitos) | 🔵 SubBrain |
| DEP-03 | BAIXA | acompanhamento do upstream | 🟢 FrontBrain |

---

## DEPLOY-10: migrações da loja na produção — ALTA
**Feito (2026-11-17):** estágio `Migracoes` do `Jenkinsfile` (depois do `Importar dump`, antes do `Deploy`) aplica `sql/27`, `28/29`, `30`, `31` e `33` (idempotentes) com a `rotaperfumes-db-admin`; o `Build` lê só o `GOOGLE_CLIENT_ID` do `api.env` (credencial já atualizada no Jenkins) para o frontend.
**Falta (usuário):** 1º run deu `ERROR 1045` do `rotaperfumes_admin`: atualizar a credencial `rotaperfumes-db-admin` com `/root/rotaperfumes-mariadb/rotaperfumes_admin.senha` (`docs/deploy-servidor.md` §5.2), rodar o job de novo e conferir `Migracoes concluidas.` e o botão do Google na loja.

## MERGE-01: cards da branch `lote-42` fora do merge de 2026-11-16 — MÉDIA
**Contexto:** a `lote-42` refez os Lotes 37–41 em paralelo; no merge venceu a versão atual (decisão do usuário). Entraram: DB-09 (migração renumerada para `sql/32`, `make db-fix-historico-created-at`), CI-01 (helper), specs `tarefas/lote-42-*.md`/`lote-43-spec.md`.
**Ação:** portar sobre a versão atual, card a card, com testes: BACKUP-01/02 (`backup-banco.sh`, `verificar-backup.sh`, `restaurar-maquina.sh` sem paridade com o `mariadb-docker.sh` atual), DEPLOY-10 (conta DBA, sem `--dev-ip`), BUG-32, CI-02, SEC-61/62/63/71 da lote-42 (SEC-58 já existe na versão atual). BUG-30, BUG-31 e DB-08 já portados (2026-11-17). Fonte: `git show origin/lote-42:<arquivo>`. Os números SEC-60 a 75 da lote-42 colidem com os daqui: renumerar ao portar.

## DEP-03: vulnerabilidades de dev restantes no frontend — BAIXA (acompanhamento)

**Status (2026-11-15):** reconferido; upstream sem correção (`typescript-eslint` 8.72.2 vulnerável, plugins sem `eslint` 10). Patches aplicados: tailwind 4.3.5, postcss 8.5.33, user-event 14.6.10. 0 altas; 8 moderadas do `minimatch` via `eslint` (só dev), correção só com `--force`.
**Ação:** quando o upstream corrigir, atualizar ou usar `overrides`; validar `tsc`, `eslint`, vitest, `next build` e `npm audit`.
**Também bloqueado:** `eslint` 10 (o 9 avisa "no longer supported"), até os plugins react/import/jsx-a11y aceitarem o 10.
**Sem `--force`:** ele rebaixa o `eslint-config-next` 16.4 → 15.5 (fora da linha do `next` 16); o `typescript-eslint` mais novo (8.72.2) ainda é vulnerável.
