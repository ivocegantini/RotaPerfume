# A Fazer

> Só cards pendentes, resumidos; o histórico fica em `feito.md`. Card com `sudo` vira **execução assistida** (comando pronto; o usuário aprova e digita a senha).
> Caminhos: `SH` = `backend/shared`.

## Resumo

| Card | Prioridade | Situação | Responsável |
| --- | --- | --- | --- |
| DEPLOY-13 | BAIXA | `--recriar` para alinhar a imagem do container (aviso do DEPLOY-12) | 🟡 BackBrain |
| DEP-03 | BAIXA | acompanhamento do upstream | 🟢 FrontBrain |

---

## DEPLOY-13: alinhar a imagem do container — BAIXA
**Contexto:** no DEPLOY-12 o script avisou que o container usa uma imagem diferente da `rotaperfumes-mariadb:11.4-5e73baf8179b` gerada agora (mesma tag, outro ID; causa provável: o `--recriar` de 2026-11-18 rodou do `/root/rotaperfumes` antigo, não confirmado).
**Ação (janela calma, execução assistida):** `sudo rotaperfumes-backup-banco --agora` e `sudo bash /root/rotaperfumes-main/deploy/mariadb-docker.sh --recriar`; conferir que o aviso some.

## DEP-03: vulnerabilidades de dev restantes no frontend — BAIXA (acompanhamento)
**Status (2026-11-18):** `npm audit --omit=dev`: 0. Só dev: 12 altas (GHSA-fc5q-39xc-vjpf, `picomatch` <=4.0.7, via vite/vitest/tinyglobby e `fast-glob` do `eslint-config-next`) e 6 moderadas (`minimatch` via `typescript-eslint` 8.73.0). O 4.0.7 é a última versão: o upstream ainda não corrigiu.
**Ação:** quando sair o `picomatch` corrigido, `npm audit fix` sem `--force` ou `overrides`; validar `tsc`, `eslint`, vitest, `next build` e `npm audit`.
**Também bloqueado:** `eslint` 10 (os plugins react/jsx-a11y aceitam até o 9), `vitest` 5 (major, card próprio).
**Sem `--force`:** ele rebaixa o `eslint-config-next` 16.4 → 14.2 (fora da linha do `next` 16).
