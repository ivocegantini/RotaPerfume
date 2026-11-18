# A Fazer

> Só cards pendentes, resumidos; o histórico fica em `feito.md`. Card com `sudo` vira **execução assistida** (comando pronto; o usuário aprova e digita a senha).
> Caminhos: `SH` = `backend/shared`.

## Resumo

| Card | Prioridade | Situação | Responsável |
| --- | --- | --- | --- |
| ECOM-15 | ALTA | aguarda escolha do gateway | 🟡 BackBrain / 🟢 FrontBrain |
| DEPLOY-14 | ALTA | antes do próximo deploy | 🌸 DataBrain |
| DEP-03 | BAIXA | acompanhamento do upstream | 🟢 FrontBrain |

---

## ECOM-15: pagamento online no checkout — ALTA
**Ação:** integrar o gateway (Pix/cartão/boleto) ao `POST /api/loja/checkout`; webhook marca o pagamento `Pago`. Hoje o pedido nasce `Em separação` com pagamento `Em aberto`.
**Pendente:** escolha do gateway pelo usuário.

## DEPLOY-14: migração 35 na produção — ALTA
**Ação:** pôr `35_alter_pedidos_chave_checkout.sql` no `MIGRACOES` do estágio `Migracoes` do Jenkinsfile (e no teste DEPLOY-10), ou rodar `make db-fix-pedidos-chave-checkout` como admin antes do deploy. Sem ela, o checkout dá 500 na produção.

## DEP-03: vulnerabilidades de dev restantes no frontend — BAIXA (acompanhamento)
**Status (2026-11-18):** `npm audit --omit=dev`: 0. Só dev: 12 altas (GHSA-fc5q-39xc-vjpf, `picomatch` <=4.0.7, via vite/vitest/tinyglobby e `fast-glob` do `eslint-config-next`) e 6 moderadas (`minimatch` via `typescript-eslint` 8.73.0). O 4.0.7 é a última versão: o upstream ainda não corrigiu.
**Ação:** quando sair o `picomatch` corrigido, `npm audit fix` sem `--force` ou `overrides`; validar `tsc`, `eslint`, vitest, `next build` e `npm audit`.
**Também bloqueado:** `eslint` 10 (os plugins react/jsx-a11y aceitam até o 9), `vitest` 5 (major, card próprio).
**Sem `--force`:** ele rebaixa o `eslint-config-next` 16.4 → 14.2 (fora da linha do `next` 16).
