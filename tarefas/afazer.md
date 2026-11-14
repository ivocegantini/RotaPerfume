# A Fazer

> Atualizado em 2026-11-14 (🔵 SubBrain; Lote 38: DEPLOY-08, BUG-25, CHORE-14 e SEC-52 concluídos e ARQ-09 em `fazendo.md`, ver `feito.md`).
> Regra (usuário, 2026-11-14): card que precisa de `sudo` vira **execução assistida** (comando pronto; o usuário aprova e digita a senha). Este arquivo contém **só cards pendentes**; o histórico de lotes e cards concluídos fica em `feito.md`.
> Caminhos: `SH` = `backend/shared`.

## Resumo

| Card | Prioridade | Situação | Responsável |
| --- | --- | --- | --- |
| TST-17 | MÉDIA | aguarda execução assistida (`sudo`) | usuário → 🔴 TestBrain |
| BACKUP-01 | MÉDIA | pendente | 🌸 DataBrain |
| TST-04 | MÉDIA | 1.1/1.2 ok; 1.7 aguarda `deploy/ca-cert.pem` | usuário → 🔴 TestBrain |
| TST-24 | MÉDIA | aguarda `deploy/ca-cert.pem` | 🔴 TestBrain |
| DEP-05 | MÉDIA | pendente (backup + `--recriar`) | 🌸 DataBrain + usuário |
| SEC-58 | BAIXA | pendente | 🟣 → 🟡 → 🔴 |
| FE-19 | BAIXA | pendente | 🟢 FrontBrain → 🔴 |
| TST-23 | BAIXA | informativo | 🔴 TestBrain |
| SEC-47, SEC-48 | BAIXA | pendente | 🟡/🟢 → 🔴 |
| SEC-51 | BAIXA | pendente | 🟣 → 🌸/🟡 |
| DEP-03 | BAIXA | acompanhamento do upstream | 🟢 FrontBrain |

---

## TST-17: testes reais com Docker do `seed-producao.sh` e do `mariadb-docker.sh` — MÉDIA

**Status:** opções de isolamento do `seed-producao.sh` prontas (Lote 38). Falta só a execução real.
**Falta:** (1) execução assistida, nesta máquina (192.168.168.105, nunca no servidor de produção): `cd backend/shared && sudo -E env PATH="$PATH" TST15_DOCKER=1 TST17_DOCKER=1 go test -count=1 -v -run 'TestTST15_MariadbDocker_Real|TestTST17_SeedProducao_Real' ./tests/deploy/`; valida também o build do `resetpassword` com `--cap-drop ALL` (SEC-52); (2) usuário confere o `make setup-check`.
**Aceite:** os dois testes reais verdes como root fora do servidor; opções novas cobertas por testes.

---

## BACKUP-01: backup diário do MariaDB de produção — MÉDIA

**Descrição:** não há backup; um `--recriar`, troca de digest ou engano apaga os dados sem volta.
**Ação:** dump diário (`600`, retenção configurável), backup obrigatório antes de `--recriar`/upgrade, restauração testada.
**Aceite:** backup gerado, restauração testada em banco descartável, `docs/deploy-servidor.md` atualizado.

---

## TST-04: itens 1.1, 1.2 e 1.7 do roteiro do Lote 12 (testam o próprio `make`) — MÉDIA

**Status:** 1.1 e 1.2 aprovados em 2026-11-14 (env temporário com `DB_NAME=rotaperfumes_l12`, CasaOS :3307). O 1.7 aguarda a CA de dev em `deploy/ca-cert.pem` (ausente nesta máquina; gitignored).
**Regras:** usar só `DB_NAME=rotaperfumes_l12` (conferir com `make -n`), nunca `rotaperfumes`; apagar ao final.
**Aceite:** resultado na seção "Execução" de `docs/roteiro-teste-manual-lote12.md`.

---

## TST-24: integração real do ARQ-09 (DECIMAL ↔ centavos) e revisão dos testes ajustados — MÉDIA

**Ação:** `make test-integration ENV_ARQUIVO=<descartável>` para confirmar o scan do DECIMAL (`[]byte`) e a gravação por texto ("x.yy") no MariaDB; revisar os ~60 testes ajustados mecanicamente (helpers `reaisT`/`decT`).
**Bloqueio:** a CA de dev (`deploy/ca-cert.pem`, `CN=RotaPerfumes_MySQL_CA`, `ef31b656…`) não está nesta máquina; o `deploy/ca.pem` copiado em 2026-11-14 é a CA de produção e não serve. Alternativa: banco descartável local com Docker (execução assistida).
**Aceite:** integração verde; nenhuma asserção enfraquecida nos testes ajustados.

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
