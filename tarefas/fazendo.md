# Fazendo

> Atualizado em 2026-11-06 (abertura do Lote 42, 🔵 SubBrain): **Lote 42** em execução, com o **BACKUP-01** (ALTA), vindo de `afazer.md`. O **Lote 41** (**SEC-71**) está em `feito.md`, commitado em `3685901`, na `main` local, sem push.

---

## Lote 42 de 2026-11-06: 1 card em execução

Aberto em 2026-11-06, a pedido do usuário ("siga para o próximo card").

**Passo atual:** 🟣 SecBrain especificando → 🌸 DataBrain/🟡 BackBrain → 🔴 TestBrain → 🟣 SecBrain revisando → 🔵 SubBrain fechando

---

## BACKUP-01: backup diário do MariaDB de produção — prioridade ALTA

**Status:** em execução (Lote 42). Subiu de MÉDIA para ALTA em 2026-11-06: produção no ar sem rotina de backup.
**Camadas:** Database, Deploy
**Responsável:** 🟣 SecBrain (especificação) → 🌸 DataBrain/🟡 BackBrain → 🔴 TestBrain → 🟣 SecBrain (revisão) → 🔵 SubBrain (fechamento)
**Origem:** 🟣 SecBrain, Lote 30 (2026-11-02).

**Descrição (atualizada em 2026-11-06):** a produção está no ar desde 2026-11-05, sem rotina de backup, e o banco tem dados reais. Um `--recriar` do `mariadb-docker.sh`, uma troca de digest (SEC-31) ou um erro de operação pode apagar os dados sem volta. O texto anterior citava o risco da conta `casaos` com escrita a partir da LAN.

**Ação esperada:** rotina diária de dump (arquivo com permissão `600`, retenção configurável), documentada em `docs/deploy-servidor.md`; exigir/registrar um backup antes de qualquer `--recriar` ou upgrade de digest; procedimento de restauração testado.

**Critério de aceite:** backup diário gerado com `600` e retenção; restauração testada num banco descartável; docs atualizadas.
