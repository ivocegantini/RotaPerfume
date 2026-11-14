# A Fazer

> Só cards pendentes, resumidos; o histórico fica em `feito.md`. Card com `sudo` vira **execução assistida** (comando pronto; o usuário aprova e digita a senha).
> Caminhos: `SH` = `backend/shared`.

## Resumo

| Card | Prioridade | Situação | Responsável |
| --- | --- | --- | --- |
| BACKUP-01 | MÉDIA | pendente | 🌸 DataBrain |
| DEP-05 | MÉDIA | pendente (backup + `--recriar`) | 🌸 DataBrain + usuário |
| SEC-58 | BAIXA | pendente | 🟣 → 🟡 → 🔴 |
| FE-19 | BAIXA | pendente | 🟢 FrontBrain → 🔴 |
| TST-23 | BAIXA | informativo | 🔴 TestBrain |
| SEC-47, SEC-48 | BAIXA | pendente | 🟡/🟢 → 🔴 |
| SEC-51 | BAIXA | pendente | 🟣 → 🌸/🟡 |
| DEP-03 | BAIXA | acompanhamento do upstream | 🟢 FrontBrain |

---

## BACKUP-01: backup diário do MariaDB de produção — MÉDIA

**Descrição:** não há backup; um `--recriar`, troca de digest ou engano apaga os dados sem volta.
**Ação:** dump diário (`600`, retenção configurável), backup obrigatório antes de `--recriar`/upgrade, restauração testada.
**Aceite:** backup gerado, restauração testada em banco descartável, `docs/deploy-servidor.md` atualizado.

---

## DEP-05: digests novos do MariaDB 11.4.13 e do Node 22.23.4 (SEC-31) — MÉDIA

**Descrição:** o `make deps-audit-imagens` (2026-11-14) mostrou digests novos para `mariadb:11.4.13` e `node:22.23.4-alpine` (rebuilds das mesmas tags).
**Ação:** trocar as strings no `Jenkinsfile`/`deploy/mariadb-docker.sh`/`seed-producao.sh`, `make test`; em produção, backup e `sudo bash deploy/mariadb-docker.sh --recriar` (execução assistida). Depende do BACKUP-01.

---

## FE-19: validar no máximo 2 casas decimais nos campos de dinheiro — BAIXA

**Descrição:** desde o ARQ-09 a API responde 400 "<campo> deve ter no máximo 2 casas decimais"; o frontend pode validar antes (campos já têm `step="0.01"`).
**Aceite:** mensagem amigável no formulário; testes vitest.

---

## SEC-58: chave do repositório MariaDB sem conferir o fingerprint (`scripts/setup-linux.sh`) — BAIXA

**Correção:** fingerprint fixado em `scripts/versoes.conf`, conferido com `gpg --show-keys` antes do `install`.
**Aceite:** teste que recusa fingerprint divergente; `make shellcheck` verde.

---

## TST-23: testes que comparam `NOW()` do banco com o relógio da API dependem do `time_zone` — BAIXA (informativo)

**Ação:** decidir entre fixar o `time_zone` no DSN ou criar uma catraca nos testes contra esse padrão.
**Aceite:** decisão registrada; se corrigido, integração verde com o servidor em UTC e em -03.

---

## SEC-47: fixar por digest as imagens base dos `Dockerfile` — BAIXA

**Descrição:** `backend/Dockerfile`, `frontend/Dockerfile` e `deploy/Dockerfile.caddy` usam tags mutáveis.
**Aceite:** bases com `tag@sha256:`, cobertas pelo `make deps-audit-imagens`; build verde.

---

## SEC-48: reprocessar os `.webp` de produto já gravados — BAIXA

**Ação:** opção no `SH/cmd/limpezaimagens` (com `-dry-run`) que reprocessa os `.webp` antigos e atualiza `produtos.imagem` sem órfãos.
**Aceite:** testes com diretório temporário (conversão, referência, dry-run, falha sem perda); docs atualizadas.

---

## SEC-51: conta somente-leitura em produção para a máquina do admin — BAIXA

**Descrição:** com `--dev-ip`, a conta `casaos` tem `ALL` na produção; avaliar uma conta `*_ro` só com `SELECT` e deixar o `ALL` para migrações.

---

## DEP-03: vulnerabilidades de dev restantes no frontend — BAIXA (acompanhamento)

**Status:** 0 altas; 16 moderadas do `minimatch` via `eslint` (só dev).
**Ação:** quando o upstream corrigir, atualizar ou usar `overrides`; validar `tsc`, `eslint`, vitest, `next build` e `npm audit`. Nunca `npm audit fix --force`.
