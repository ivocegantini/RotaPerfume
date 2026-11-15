# A Fazer

> Só cards pendentes, resumidos; o histórico fica em `feito.md`. Card com `sudo` vira **execução assistida** (comando pronto; o usuário aprova e digita a senha).
> Caminhos: `SH` = `backend/shared`.

## Resumo

| Card | Prioridade | Situação | Responsável |
| --- | --- | --- | --- |
| DEPLOY-09 | MÉDIA | execução assistida | usuário + 🌸 DataBrain |
| DEP-03 | BAIXA | acompanhamento do upstream | 🟢 FrontBrain |

---

## DEPLOY-09: aplicar os Lotes 39 e 40 (BACKUP-01, DEP-05, DEP-06, SEC-47, SEC-59) — MÉDIA

**Feito (2026-11-15):** build do Jenkins, clone em `/root/rotaperfumes`, `mariadb-docker.sh --dev-ip-ro`, buildx 0.39 no host e no container `Jenkins`, segredos da cópia antiga em `/root/rotaperfumes-segredos-antigos`, restauração real (`TestBACKUP01_RestauracaoReal_Docker`) verde.
**Falta (execução assistida):**
1. Apagar `/root/rotaperfumes.copia-20261114`.
2. Timer diário de backup (§9) e a chave da CA (`/root/rotaperfumes-ca/ca-key.pem`) para mídia offline.
3. SEC-59: `.env.producao.ro` (`casaos_ro`) + `deploy/ca-producao.pem` na 192.168.168.105, e teste `openssl s_client` impresso pelo script.

---

## DEP-03: vulnerabilidades de dev restantes no frontend — BAIXA (acompanhamento)

**Status (2026-11-14):** 0 altas; 8 moderadas do `minimatch` via `eslint` (só dev), correção só com `--force`.
**Ação:** quando o upstream corrigir, atualizar ou usar `overrides`; validar `tsc`, `eslint`, vitest, `next build` e `npm audit`.
**Também bloqueado:** `eslint` 10 (o 9 avisa "no longer supported"), até os plugins react/import/jsx-a11y aceitarem o 10.
**Sem `--force`:** ele rebaixa o `eslint-config-next` 16.4 → 15.5 (fora da linha do `next` 16); o `typescript-eslint` mais novo (8.72.2) ainda é vulnerável.
