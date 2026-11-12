# Fazendo

> Atualizado em 2026-11-07 (abertura do Lote 43, 🔵 SubBrain): **Lote 43 de 2026-11-07** em execução, com 6 cards vindos do bloco "Achados do Lote 42" de `afazer.md`: **SEC-73** e **TST-28** (MÉDIA) e **BACKUP-03**, **BACKUP-04**, **BACKUP-05** e **BACKUP-07** (BAIXA). Decisões do usuário de 2026-11-07 registradas no SEC-73 (keyservers aceitos como segundo canal) e no TST-28 (`-timeout=30m`). O **Lote 42** foi commitado em `4c32ab4`, na branch `lote-42`, **sem push** (push na `main` é deploy em produção viva).
>
> **Restrição do lote:** banco só no descartável local (`rp-descartavel-l36`, `127.0.0.1:33306`); nada no CasaOS nem na produção.
>
> **Passo atual:** 🟣 SecBrain especificando → 🟡 BackBrain → 🔴 TestBrain → 🟣 SecBrain revisando → 🔵 SubBrain fechando.
>
> Antes, ainda em 2026-11-07 (fechamento do Lote 42, 🔵 SubBrain): o **Lote 42** foi fechado e os 4 cards (**BACKUP-01**, **BACKUP-02**, **DEPLOY-10** e **DEPLOY-09**) foram para `feito.md` ("Lote 42 de 2026-11-06/07: 4 cards concluídos"). **Sem commit e pendentes de validação no servidor.** Nenhum card em execução. (O commit veio depois: `4c32ab4`, ver a nota acima.)
>
> Antes, em 2026-11-06 (novos cards do Lote 42, 🔵 SubBrain): a pedido do usuário, entraram no Lote 42 o **BACKUP-02** (BAIXA), o **DEPLOY-09** (ALTA) e o **DEPLOY-10** (MÉDIA), junto do **BACKUP-01**.
>
> Antes, ainda em 2026-11-06 (abertura do Lote 42, 🔵 SubBrain): **Lote 42** em execução, com o **BACKUP-01** (ALTA), vindo de `afazer.md`. O **Lote 41** (**SEC-71**) está em `feito.md`, commitado em `3685901`, na `main` local, sem push.

---

# Lote 43 de 2026-11-07

Aberto em 2026-11-07 pelo 🤍 MegaBrain. Passos: 🟣 SecBrain especificando → 🟡 BackBrain → 🔴 TestBrain → 🟣 SecBrain revisando → 🔵 SubBrain fechando. Banco só no descartável local (`rp-descartavel-l36`, `127.0.0.1:33306`). Caminhos: `SH` = `backend/shared`.

---

## SEC-73: confirmar o fingerprint da chave apt da Cloudflare por um segundo canal — prioridade MÉDIA

**Status:** em execução (Lote 43). Conferência feita; falta o registro no código e nas docs.
**Camadas:** Deploy, Segurança, Documentação
**Responsável:** usuário (conferência) → 🟣 SecBrain (registro) → 🟡 BackBrain → 🔵 SubBrain
**Origem:** DEPLOY-09 (Lote 42); fatos conferidos pelo 🤍 MegaBrain em 2026-11-06 (`tarefas/lote-42-deploy-09-10-spec.md`, seção "Cloudflare").

**Descrição:** `deploy/cloudflare-apt.fingerprint` traz `CC94B39C77AE7342A68B89628A682D308D4E5E73` (`CloudFlare Software Packaging 2025`). A chave veio só pelo HTTPS da própria Cloudflare (`pkg.cloudflare.com`), e a página não publica o fingerprint. Um único canal comprometido levaria uma chave falsa ao `restaurar-maquina.sh`.

**Até a conferência:** rodar o `deploy/restaurar-maquina.sh` **só com `--sem-tunel`**.

**Critério de aceite:** fingerprint `CC94B39C…5E73` confirmado por um canal independente (ex.: keyserver público, outra rede ou outro meio indicado pela Cloudflare), com o canal registrado neste card; se divergir, chave e fingerprint trocados e testes do DEPLOY-09 verdes.

**Decisão do usuário (2026-11-07):** keyservers públicos aceitos como segundo canal.

**Conferência do 🤍 MegaBrain (2026-11-07):**
- `keyserver.ubuntu.com` e `keys.openpgp.org` servem a mesma chave primária `CC94B39C77AE7342A68B89628A682D308D4E5E73` e a mesma subchave `06C89DB3B80A8F4349697C76029E1444B7D9F50F`, iguais às de `deploy/cloudflare-apt.asc`.
- uid: "CloudFlare Software Packaging 2025 <help@cloudflare.com>" no `keyserver.ubuntu.com`; sem uid no `keys.openpgp.org` (o e-mail não foi verificado lá).
- `pgp.mit.edu` não respondeu (timeout).
- Busca web pelo fingerprint: nenhuma página da Cloudflare o publica.
- **Ressalva:** keyservers aceitam upload de qualquer pessoa. A conferência mostra que a chave servida pela Cloudflare é a mesma publicada nos keyservers, mas não prova que a Cloudflare a publicou lá.

**Falta (🟡 BackBrain / 🔵 SubBrain):** atualizar o comentário de `deploy/cloudflare-apt.fingerprint` (canais conferidos, data e ressalva) e os docs que mandam rodar o `restaurar-maquina.sh` só com `--sem-tunel`.

---

## TST-28: timeout do `make test` (`tests/deploy` perto dos 10 min padrão do `go test`) — prioridade MÉDIA

**Status:** em execução (Lote 43). Decisão do usuário tomada.
**Camadas:** Testes, CI
**Responsável:** usuário (decisão) → 🟡 BackBrain → 🔴 TestBrain
**Origem:** rodada final do Lote 42.

**Descrição:** o pacote `SH/tests/deploy` leva cerca de 580 s, perto dos 10 min padrão do `go test`, e estourou com a máquina carregada. A rodada final do lote só passou com `GO_TEST_FLAGS=-timeout=45m`. Hoje o `test-shared` (`Makefile:~620`) não fixa `-timeout`, e o `Jenkinsfile` (`:~132-133`, `go test ./...` no estágio "Testes Go") também não; o job tem `timeout(time: 60, unit: 'MINUTES')` (`:~32`).

**Proposta:** `-timeout=30m` no `go test` do `test-shared`. Conferir também o `Jenkinsfile`: o mesmo `-timeout` nas duas linhas, dentro dos 60 min do job.

**Decisão do usuário (2026-11-07):** `-timeout=30m` no `go test` do `test-shared` (`Makefile`) e nas linhas `go test` do estágio "Testes Go" do `Jenkinsfile`, com teste de paridade entre os dois.

**Critério de aceite:** decisão do usuário registrada; `-timeout=30m` aplicado no `Makefile` e no `Jenkinsfile`; teste de paridade (mesmo valor nos dois arquivos); `make -k test` verde sem `GO_TEST_FLAGS`.

---

## BACKUP-03: a mesma tolerância de relógio (300 s) nas guardas do `mariadb-docker.sh` e do seed — prioridade BAIXA

**Status:** em execução (Lote 43).
**Camadas:** Deploy, Testes
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** revisão do 🟣 SecBrain, Lote 42.

**Descrição:** o `deploy/backup-banco.sh` usa `TOLERANCIA_RELOGIO_S=300` ao conferir o carimbo e o mtime. As guardas do `--recriar` (`deploy/mariadb-docker.sh`) e do `--forcar` (`deploy/seed-producao.sh`) precisam da mesma folga, com a mesma regra para carimbo futuro.

**Critério de aceite:** a mesma tolerância nos três scripts, com valor único conferido por teste de paridade; testes de borda (dentro e fora da folga, carimbo futuro) nas duas guardas.

---

## BACKUP-04: aceitar `parcial-*` na guarda do `--recriar`/seed — prioridade BAIXA

**Status:** em execução (Lote 43).
**Camadas:** Deploy, Testes
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** revisão do 🟣 SecBrain, Lote 42 ("parciais fora da guarda").

**Descrição:** hoje a guarda do `--recriar` e do `seed-producao.sh --forcar` não aceita um par `parcial-*` como backup recente.

**Critério de aceite:** `parcial-*` aceito pela guarda com as mesmas conferências do par completo (idade, carimbo, mtime, marcador, tamanho) e registro igual; testes de aceite e de recusa.

---

## BACKUP-05: retenção relendo todos os pares mantidos da série — prioridade BAIXA

**Status:** em execução (Lote 43).
**Camadas:** Deploy, Testes
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** revisão do 🟣 SecBrain, Lote 42.

**Descrição:** a retenção do `deploy/backup-banco.sh` deve reler todos os pares que mantém na série, não só o mais recente, para não guardar como "bom" um par corrompido depois de gravado.

**Critério de aceite:** a retenção confere cada par mantido; par corrompido sinalizado sem apagar o último bom; testes com par corrompido no meio da série.

---

## BACKUP-07: `--recriar --dados /vazio` não dispara a guarda de backup — prioridade BAIXA

**Status:** em execução (Lote 43).
**Camadas:** Deploy, Testes
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** especificação e entrega do BACKUP-01 (`tarefas/lote-42-backup-01-spec.md`, seção 5 e desvio 8 do 🟡 BackBrain): card próprio pedido e ainda não aberto (conferido no fechamento do Lote 42).

**Descrição:** o `mariadb-docker.sh --recriar` com `--dados` apontando para um diretório vazio não dispara a guarda e sobe um banco vazio, embora os dados antigos continuem no caminho original. O site passaria a usar o banco vazio.

**Critério de aceite:** troca de `--dados` num container existente recusada ou sujeita à mesma guarda (backup recente ou motivo registrado); testes com diretório vazio e com o caminho original.
