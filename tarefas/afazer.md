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

**Ação (execução assistida):**
0. Uma vez, na máquina do build: `sudo apt install docker-buildx` (sem ele o `docker build` cai no legacy builder).
1. Restauração real, numa máquina com Docker que não seja o servidor: `cd backend/shared && sudo -E env "PATH=$PATH" BACKUP01_DOCKER=1 go test -count=1 -run TestBACKUP01_RestauracaoReal_Docker ./tests/deploy/`.
2. Build dos Dockerfiles (SEC-47, bases Go 1.27.4/Node 26.13.1): `sudo docker build backend/`, `sudo docker build frontend/`, `sudo docker build -f deploy/Dockerfile.caddy deploy/`.
3. No servidor, depois do push: `sudo git -C /root/rotaperfumes pull --ff-only && cd /root/rotaperfumes && sudo bash deploy/mariadb-docker.sh --recriar --backup-agora --dev-ip-ro 192.168.168.105`; timer diário da §9.
4. SEC-59: `.env.producao.ro` (`casaos_ro`) + `deploy/ca-producao.pem` na 192.168.168.105, e teste `openssl s_client` impresso pelo script.

---

## DEP-03: vulnerabilidades de dev restantes no frontend — BAIXA (acompanhamento)

**Status (2026-11-14):** 0 altas; 8 moderadas do `minimatch` via `eslint` (só dev), correção só com `--force`.
**Ação:** quando o upstream corrigir, atualizar ou usar `overrides`; validar `tsc`, `eslint`, vitest, `next build` e `npm audit`.
**Também bloqueado:** `eslint` 10 (o 9 avisa "no longer supported"), até os plugins react/import/jsx-a11y aceitarem o 10.
**Sem `--force`:** ele rebaixa o `eslint-config-next` 16.4 → 15.5 (fora da linha do `next` 16); o `typescript-eslint` mais novo (8.72.2) ainda é vulnerável.
