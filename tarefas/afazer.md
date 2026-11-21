# A Fazer

> Só cards pendentes, resumidos; o histórico fica em `feito.md`. Card com `sudo` vira **execução assistida** (comando pronto; o usuário aprova e digita a senha).
> Caminhos: `SH` = `backend/shared`.

## Resumo

| Card | Prioridade | Situação | Responsável |
| --- | --- | --- | --- |
| ECOM-15 | ALTA | aguarda escolha do gateway real (fluxo pronto no ECOM-18) | 🟡 BackBrain / 🟢 FrontBrain |
| DEPLOY-14 | ALTA | antes do próximo deploy na produção | 🌸 DataBrain |
| DOC-11 | BAIXA | quando houver folga | 🟡 BackBrain |
| DEP-03 | BAIXA | acompanhamento do upstream | 🟢 FrontBrain |

---

## ECOM-15: pagamento online no checkout — ALTA
**Ação:** ligar o gateway real (Pix/cartão/boleto): o webhook chama o mesmo `ConfirmarPagamentoLoja` do ECOM-18 (pedido nasce `Aguardando pagamento`).
**Pendente:** escolha do gateway pelo usuário.

## DEPLOY-14: banco da produção na versão atual — ALTA
**Ação:** antes do deploy na produção, rodar `make db-migrar ENV_ARQUIVO=backend/.env.producao CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME>` (idempotente; leva o banco ao `sql/schema.sql`). Sem ela, o checkout dá 500 (falta `pedidos.chave_checkout`). O QA já recebe o `sql/migracao.sql` pelo estágio `Migracoes`.

## DOC-11: descrições da `postman/collection.json` sem histórico — BAIXA
**Ação:** resumir as descrições da collection (ainda com "Mudanças do Lote N" e o nome "RotaPerfumes API — Autenticação") ao estado atual, como o `postman/README.md` do CHORE-20.

## DEP-03: vulnerabilidades de dev restantes no frontend — BAIXA (acompanhamento)
**Status (2026-11-18):** `npm audit --omit=dev`: 0. Só dev: 12 altas (GHSA-fc5q-39xc-vjpf, `picomatch` <=4.0.7, via vite/vitest/tinyglobby e `fast-glob` do `eslint-config-next`) e 6 moderadas (`minimatch` via `typescript-eslint` 8.73.0). O 4.0.7 é a última versão: o upstream ainda não corrigiu.
**Ação:** quando sair o `picomatch` corrigido, `npm audit fix` sem `--force` ou `overrides`; validar `tsc`, `eslint`, vitest, `next build` e `npm audit`.
**Também bloqueado:** `eslint` 10 (os plugins react/jsx-a11y aceitam até o 9), `vitest` 5 (major, card próprio).
**Sem `--force`:** ele rebaixa o `eslint-config-next` 16.4 → 14.2 (fora da linha do `next` 16).
