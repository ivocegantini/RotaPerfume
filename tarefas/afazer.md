# A Fazer

> Só cards pendentes, resumidos; o histórico fica em `feito.md`. Card com `sudo` vira **execução assistida** (comando pronto; o usuário aprova e digita a senha).
> Caminhos: `SH` = `backend/shared`.

## Resumo

| Card | Prioridade | Situação | Responsável |
| --- | --- | --- | --- |
| MERGE-01 | MÉDIA | código portado; faltam testes reais e o servidor (execução assistida) | 🔵 SubBrain |
| LOTE-43 | MÉDIA | BACKUP-03/04/05/07, SEC-73, TST-28 (`tarefas/lote-43-spec.md`) | 🟣 SecBrain |
| DEP-03 | BAIXA | acompanhamento do upstream | 🟢 FrontBrain |

---

## MERGE-01: finalizar o porte da `lote-42` — MÉDIA
**Feito:** etapas 1 e 2 na branch `lote-43-merge-01` (ver `feito.md`).
**Falta:** (1) usuário roda na MS-7E28 os testes reais BACKUP01, TST15, TST17, DEPLOY-09/10; (2) execução assistida no servidor: desligar o `rotaperfumes-backup.timer` antigo, `backup-banco.sh --instalar`, `rotaperfumes-backup-banco --agora`, `verificar-backup.sh`, `mariadb-docker.sh --recriar` (fecha a porta da LAN e remove `casaos`/`casaos_ro`); (3) PR para a `main`.

## LOTE-43: spec da `lote-42` ainda não implementada — MÉDIA
**Ação:** implementar conforme `tarefas/lote-43-spec.md`: BACKUP-03 (tolerância de relógio 300 s), BACKUP-04 (`parcial-*` na guarda), BACKUP-05 (retenção relê os pares), BACKUP-07 (`--trocar-dados`), SEC-73 (fingerprint da chave apt da Cloudflare por 2º canal), TST-28 (`-timeout=30m`). Depois do MERGE-01.

## DEP-03: vulnerabilidades de dev restantes no frontend — BAIXA (acompanhamento)

**Status (2026-11-15):** reconferido; upstream sem correção (`typescript-eslint` 8.72.2 vulnerável, plugins sem `eslint` 10). Patches aplicados: tailwind 4.3.5, postcss 8.5.33, user-event 14.6.10. 0 altas; 8 moderadas do `minimatch` via `eslint` (só dev), correção só com `--force`.
**Ação:** quando o upstream corrigir, atualizar ou usar `overrides`; validar `tsc`, `eslint`, vitest, `next build` e `npm audit`.
**Também bloqueado:** `eslint` 10 (o 9 avisa "no longer supported"), até os plugins react/import/jsx-a11y aceitarem o 10.
**Sem `--force`:** ele rebaixa o `eslint-config-next` 16.4 → 15.5 (fora da linha do `next` 16); o `typescript-eslint` mais novo (8.72.2) ainda é vulnerável.
