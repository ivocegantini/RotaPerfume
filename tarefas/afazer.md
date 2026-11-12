# A Fazer

> Atualizado em 2026-11-07 (abertura do Lote 43, 🔵 SubBrain). O 🤍 MegaBrain abriu o **Lote 43 de 2026-11-07** e 6 cards do bloco "Achados do Lote 42" saíram desta lista para `fazendo.md`, com o texto completo: **SEC-73** e **TST-28** (MÉDIA, agora com as decisões do usuário de 2026-11-07) e **BACKUP-03**, **BACKUP-04**, **BACKUP-05** e **BACKUP-07** (BAIXA). Ficam no bloco: **BACKUP-06**, **DEPLOY-11**, **SEC-74**, **DB-10**, **DEPLOY-12** e **SEC-75**. O **Lote 42** foi commitado em `4c32ab4`, na branch `lote-42`, sem push (push na `main` é deploy em produção viva). Restrição do lote: banco só no descartável local (`rp-descartavel-l36`, `127.0.0.1:33306`); nada no CasaOS nem na produção. O restante do backlog não mudou.
>
> Antes, ainda em 2026-11-07 (fechamento do Lote 42, 🔵 SubBrain). O **Lote 42** foi fechado: **BACKUP-01**, **BACKUP-02**, **DEPLOY-10** e **DEPLOY-09** concluídos (`feito.md`, "Lote 42 de 2026-11-06/07: 4 cards concluídos"; **sem commit e pendentes de validação no servidor**). Entraram 12 cards no bloco novo "Achados do Lote 42" (logo abaixo): **SEC-73** e **TST-28** (MÉDIA) e **BACKUP-03**, **BACKUP-04**, **BACKUP-05**, **BACKUP-06**, **DEPLOY-11**, **SEC-74**, **DB-10**, **DEPLOY-12**, **SEC-75** e **BACKUP-07** (BAIXA); os três últimos são os cards pedidos nas especificações do lote (`IMPORTAR_DUMP`, `-e MYSQL_PWD` e `--recriar --dados /vazio`). O **DEPLOY-08** ganhou a ordem da migração do DEPLOY-10 (entre os itens 6 e 7), o item 2 aponta para a seção "Backup do banco e das imagens de produto" e o item 7 já não remove a `casaos`. **Antes de tudo:** push (deploy) e `TestTST15_MariadbDocker_Real` como root, fora do servidor. O restante do backlog não mudou.
>
> Antes, ainda em 2026-11-06 (novos cards do Lote 42, 🔵 SubBrain). A pedido do usuário, **BACKUP-02** (BAIXA), **DEPLOY-09** (ALTA) e **DEPLOY-10** (MÉDIA) foram registrados direto em `fazendo.md`, no Lote 42. Nenhum card saiu desta lista e o backlog não mudou.
>
> Antes, ainda em 2026-11-06 (abertura do Lote 42, 🔵 SubBrain). A pedido do usuário ("siga para o próximo card"), o **Lote 42 de 2026-11-06** foi aberto e o **BACKUP-01** (ALTA, bloco "Achados dos Lotes 30 e 31") saiu desta lista para `fazendo.md`, com a descrição atualizada. O **DEPLOY-08** ganhou a decisão do usuário de 2026-11-06: **sem passo de descarte** de `casaos.senha` e `/root/rotaperfumes.antigo`; as senhas vão para pendrive seguro (itens 1 e 3 sem alteração, dúvidas registradas no card). O **Lote 41** foi commitado em `3685901`, na `main` local, sem push (`feito.md`). O restante do backlog não mudou.
>
> Antes, ainda em 2026-11-06 (fechamento do Lote 41, 🔵 SubBrain). O **Lote 41** foi fechado: **SEC-71** concluído (`feito.md`, "Lote 41 de 2026-11-06: 1 card concluído"; **não commitado**, pendente do ok do usuário). Entrou o **SEC-72** (BAIXA) no bloco novo "Achados do Lote 41" (logo abaixo). **DEPLOY-08** corrigido: a produção está **no ar** e o card passou a ser a limpeza pós-instalação, pendente do usuário; **um push na `main` agora é um deploy em sistema vivo**. **BACKUP-01** subiu para ALTA (produção no ar sem rotina de backup) e é o **próximo card recomendado**.
>
> Antes, ainda em 2026-11-06 (abertura do Lote 41, 🔵 SubBrain). **SEC-71** registrado direto em `fazendo.md`, em execução. Corrigidos aqui os cards **DEPLOY-08** (instalação parcial já feita no servidor, ver "Andamento 2026-11-06"; passos 2 e 4 valem para instalação do zero) e **TST-04** (`.env` aponta para o CasaOS :3307). O restante do backlog não mudou.
>
> Antes, em 2026-11-06 (fechamento do Lote 40, 🔵 SubBrain). O **Lote 40** foi fechado: **SEC-63** concluído (`feito.md`, "Lote 40 de 2026-11-06: 1 card concluído"; **não commitado**: Lotes 37 a 40 juntos). Entraram 5 cards BAIXA no bloco novo "Achados do Lote 40" (logo abaixo): **SEC-66**, **SEC-67**, **SEC-68**, **SEC-69** e **SEC-70**. O **SEC-64** ganhou um acréscimo. **Continuam aqui:** DEPLOY-08 (ALTA, pendente do usuário; bloqueia o primeiro deploy da `main`), TST-17 (resto, pendente do usuário), ARQ-09 (adiado), CHORE-14 (só 7 dias após o deploy do SEC-33), BUG-25 (decisão), BACKUP-01, SEC-47, SEC-48, SEC-51, SEC-52, TST-04, DEP-03, SEC-64, SEC-65, CHORE-20, TST-26, CI-03, TST-27 e os 5 novos.
>
> Antes, em 2026-11-05 (abertura do Lote 40, 🔵 SubBrain). A pedido do usuário ("siga para SEC-63"), o 🤍 MegaBrain abriu o **Lote 40 de 2026-11-05** e o **SEC-63** (MÉDIA, bloco "Achados do Lote 39") saiu desta lista para `fazendo.md`, com o texto completo. Testes só com cópia do `Makefile` e cliente falso, ou no descartável local (`rp-descartavel-l36`, `127.0.0.1:33306`); nada no CasaOS nem na produção. O restante do backlog não mudou.
>
> Antes, ainda em 2026-11-05 (fechamento do Lote 39, 🔵 SubBrain). O **Lote 39** foi fechado: **BUG-31**, **SEC-59**, **SEC-60**, **SEC-61**, **BUG-32**, **CHORE-19**, **CI-02**, **DB-09**, **TST-24**, **TST-25** e **SEC-62** concluídos e **BUG-33** fechado como aceito, sem correção (`feito.md`, "Lote 39 de 2026-11-05: 12 cards fechados (11 concluídos, 1 aceito)"; **não commitado**: Lotes 37, 38 e 39 juntos). Entraram 7 cards no bloco novo "Achados do Lote 39" (logo abaixo): **SEC-63** (MÉDIA; primeiro card do próximo lote) e **SEC-64**, **SEC-65**, **CHORE-20**, **TST-26**, **CI-03** e **TST-27** (BAIXA). O **SEC-47** ganhou um acréscimo. **Continuam aqui:** DEPLOY-08 (ALTA, pendente do usuário; bloqueia o primeiro deploy da `main`), TST-17 (resto, pendente do usuário), ARQ-09 (adiado), CHORE-14 (só 7 dias após o deploy do SEC-33), BUG-25 (decisão), BACKUP-01, SEC-47, SEC-48, SEC-51, SEC-52, TST-04, DEP-03 e os 7 novos.
>
> Antes, ainda em 2026-11-05 (abertura do Lote 39, 🔵 SubBrain). A pedido do usuário ("pode seguir com os de baixa prioridade"), o 🤍 MegaBrain abriu o **Lote 39 de 2026-11-05** e 12 cards BAIXA saíram desta lista para `fazendo.md`: **BUG-31**, **SEC-59**, **SEC-60**, **SEC-61**, **BUG-32** e **CHORE-19** (bloco "Achados do Lote 37") e **CI-02**, **DB-09**, **TST-24**, **TST-25**, **SEC-62** e **BUG-33** (bloco "Achados do Lote 38"); os dois blocos ficam sem cards pendentes. Banco só no descartável local (`rp-descartavel-l36`, `127.0.0.1:33306`); nada no CasaOS nem na produção. Passo atual do lote: "🟣 SecBrain especificando → 🌸 DataBrain (DB-09) + 🟡 BackBrain → 🔴 TestBrain → 🟣 SecBrain revisando → 🔵 SubBrain fechando". **Continuam aqui:** DEPLOY-08 (ALTA, pendente do usuário; bloqueia o primeiro deploy da `main`), TST-17 (resto, pendente do usuário), ARQ-09 (adiado), CHORE-14 (só 7 dias após o deploy do SEC-33), BUG-25 (decisão), BACKUP-01, SEC-47, SEC-48, SEC-51, SEC-52, TST-04 e DEP-03.
>
> Antes, ainda em 2026-11-05 (fechamento do Lote 38, 🔵 SubBrain). O **Lote 38** foi fechado: **BUG-30**, **DB-08** e **CI-01** concluídos (`feito.md`, "Lote 38 de 2026-11-05: 3 cards concluídos"; **não commitado**). Entraram 6 cards BAIXA no bloco novo "Achados do Lote 38" (logo abaixo): **CI-02**, **DB-09**, **TST-24**, **TST-25**, **SEC-62** e **BUG-33**. Pendência informativa do usuário: levantar `@@global.time_zone`/`@@system_time_zone` no CasaOS e na produção (comando em `docs/manual-base-de-dados.md` §1.1). **Continuam aqui:** DEPLOY-08 (ALTA, pendente do usuário; bloqueia o primeiro deploy da `main`), TST-17 (resto, pendente do usuário), ARQ-09 (adiado), CHORE-14 (só 7 dias após o deploy do SEC-33), BUG-25 (decisão), BACKUP-01, SEC-47, SEC-48, SEC-51, SEC-52, TST-04, DEP-03, os 6 BAIXA do Lote 37 (BUG-31, SEC-59, SEC-60, SEC-61, BUG-32, CHORE-19) e os 6 novos (CI-02, DB-09, TST-24, TST-25, SEC-62, BUG-33).
>
> Antes, ainda em 2026-11-05 (abertura do Lote 38, 🔵 SubBrain). A pedido do usuário ("siga com os cards média"), o 🤍 MegaBrain abriu o **Lote 38 de 2026-11-05** e 3 cards MÉDIA saíram do bloco "Achados do Lote 37" para `fazendo.md`: **BUG-30**, **DB-08** e **CI-01**. Testes de banco só no descartável local (`rp-descartavel-l36`, `127.0.0.1:33306`); nada no CasaOS nem na produção. Passo atual do lote: "🟣 SecBrain especificando → 🌸 DataBrain (DB-08) + 🟡 BackBrain (BUG-30, CI-01, DB-08) → 🔴 TestBrain → 🟣 SecBrain revisando → 🔵 SubBrain fechando". **Continuam aqui:** DEPLOY-08 (ALTA, pendente do usuário; bloqueia o primeiro deploy da `main`), TST-17 (resto, pendente do usuário), ARQ-09 (adiado), CHORE-14 (só 7 dias após o deploy do SEC-33), BUG-25 (decisão), BACKUP-01, SEC-47, SEC-48, SEC-51, SEC-52, TST-04, DEP-03 e os 6 BAIXA do Lote 37 (BUG-31, SEC-59, SEC-60, SEC-61, BUG-32, CHORE-19).
>
> Antes, ainda em 2026-11-05 (fechamento do Lote 37, 🔵 SubBrain). O **Lote 37** foi fechado: **BUG-29**, **BUG-28**, **SEC-56**, **SEC-57**, **TST-17** (item 1), **SEC-58** e **TST-23** concluídos (`feito.md`, "Lote 37 de 2026-11-05: 7 cards concluídos"; **não commitado**). O card "TST-17 (resto, pendente do usuário)" continua aqui, atualizado (o item 1 já foi feito; o `TestTST17_SeedProducao_Real` só pula por falta de `TST17_DOCKER=1`). Entraram 9 cards no bloco novo "Achados do Lote 37" (logo abaixo): **CI-01**, **BUG-30** e **DB-08** (MÉDIA) e **BUG-31**, **SEC-59**, **SEC-60**, **SEC-61**, **BUG-32** e **CHORE-19** (BAIXA). **Continuam aqui:** DEPLOY-08 (ALTA, pendente do usuário; bloqueia o primeiro deploy da `main`), TST-17 (resto, pendente do usuário), ARQ-09 (adiado), CHORE-14 (só 7 dias após o deploy do SEC-33), BUG-25 (decisão), BACKUP-01, SEC-47, SEC-48, SEC-51, SEC-52, TST-04, DEP-03 e os 9 novos (CI-01, BUG-30, DB-08, BUG-31, SEC-59, SEC-60, SEC-61, BUG-32, CHORE-19).
>
> Antes, ainda em 2026-11-05 (abertura do Lote 37, 🔵 SubBrain). A pedido do usuário ("continue com as tarefas a fazer"), o 🤍 MegaBrain abriu o **Lote 37 de 2026-11-05** e 7 cards saíram desta lista para `fazendo.md`: **BUG-29**, **TST-17** (só o item 1: opções de isolamento do `seed-producao.sh`), **SEC-58** e **TST-23** (bloco "Achados do Lote 36") e **SEC-56**, **SEC-57** e **BUG-28** (bloco "Achados do Lote 35", que fica sem cards pendentes). Os itens 2 e 3 do TST-17 (execução com `sudo` pelo usuário) ficaram aqui, no card "TST-17 (resto, pendente do usuário)". Passo atual do lote: "🟣 SecBrain especificando → 🟡 BackBrain → 🔴 TestBrain → 🟣 SecBrain revisando → 🔵 SubBrain fechando". **Continuam aqui:** DEPLOY-08 (ALTA, pendente do usuário; bloqueia o primeiro deploy da `main`), TST-17 (resto, pendente do usuário), ARQ-09 (adiado), CHORE-14 (só 7 dias após o deploy do SEC-33), BUG-25 (decisão), BACKUP-01, SEC-47, SEC-48, SEC-51, SEC-52, TST-04 e DEP-03.
>
> Antes, ainda em 2026-11-05 (fechamento do Lote 36, 🔵 SubBrain). O **Lote 36** foi fechado: **TST-22**, **DB-07** e **TST-16** concluídos (`feito.md`, "Lote 36 de 2026-11-05"; o TST-16 com a execução real como root pendente, coberta pelo TST-17). O **TST-17** voltou **parcial** para esta lista, no bloco novo "Achados do Lote 36" (logo abaixo), com o que falta: 🟡 BackBrain implementar as opções de isolamento do `seed-producao.sh` e o usuário rodar com `sudo` os testes reais TST15 e TST17 (nunca no servidor de produção). Entraram também **BUG-29** (MÉDIA), **SEC-58** e **TST-23** (BAIXA). **Continuam aqui:** DEPLOY-08 (ALTA, pendente do usuário; bloqueia o primeiro deploy da `main`), SEC-56, SEC-57, BUG-28, ARQ-09 (adiado), CHORE-14 (só 7 dias após o deploy do SEC-33), BUG-25 (decisão), BACKUP-01, SEC-47, SEC-48, SEC-51, SEC-52, TST-04 e DEP-03.
>
> Antes, ainda em 2026-11-05 (abertura do Lote 36, 🔵 SubBrain). O usuário alterou ele mesmo o `CLAUDE.md` (sugestão do Lote 35: SEC-54, SEC-55 e BUG-27; pendência atendida em `feito.md`), rodou o `usermod -aG docker` (sessão do agente via `sg docker`; `shellcheck` instalado) e autorizou os processos de banco descartável. O 🤍 MegaBrain abriu o **Lote 36 de 2026-11-05** e 4 cards saíram desta lista para `fazendo.md`: **TST-22** (bloco "Achados do Lote 34"), **TST-17** (bloco "Achados do Lote 33 (DEPLOY-07)") e **DB-07** e **TST-16** (bloco "Achados dos Lotes 30 e 31"). Banco descartável: container local `rp-descartavel-l36` (`127.0.0.1:33306`, `rotaperfumes_l36`); nada de CasaOS nem de produção. Passo atual do lote: "🔴 TestBrain (TST-22, TST-17) + 🌸 DataBrain (DB-07, TST-16) → 🟣 SecBrain revisando → 🔵 SubBrain fechando". **Continuam aqui:** DEPLOY-08 (ALTA, pendente do usuário; bloqueia o primeiro deploy da `main`), SEC-56, SEC-57, BUG-28, ARQ-09 (adiado), CHORE-14 (só 7 dias após o deploy do SEC-33), BUG-25 (decisão), BACKUP-01, SEC-47, SEC-48, SEC-51, SEC-52, TST-04 e DEP-03.
>
> Antes, ainda em 2026-11-05 (fechamento do Lote 35, 🔵 SubBrain). O **Lote 35** (BUG-27, SEC-54, TST-19, SEC-55, TST-20, TST-21, FE-18 e CHORE-16) foi concluído e está em `feito.md` (commitado na `main` pelo 🤍 MegaBrain, a pedido do usuário). Entraram 3 cards no bloco "Achados do Lote 35", logo abaixo: **SEC-56**, **SEC-57** e **BUG-28** (BAIXA). **Seguem pendentes do usuário:** **DEPLOY-08** (ALTA; bloqueia o primeiro deploy da `main`) e **TST-17** (precisa de Docker); a sugestão de citar SEC-54/SEC-55/BUG-27 no `CLAUDE.md` está em `feito.md` (bloco do Lote 35). O restante do backlog não mudou.
>
> Antes, ainda em 2026-11-05 (abertura do Lote 35, 🔵 SubBrain). O usuário pediu "continue com as tarefas a fazer, em seguida commit as alterações finalizadas". O 🤍 MegaBrain abriu o **Lote 35 de 2026-11-05** e 8 cards saíram desta lista para `fazendo.md`: **BUG-27**, **SEC-54**, **TST-19**, **SEC-55**, **TST-20** e **TST-21** (bloco "Achados do Lote 34") e **FE-18** e **CHORE-16** (bloco "Achados dos Lotes 30 e 31"). Critério: executáveis sem o usuário, sem Docker e sem banco. Passo atual do lote: "🟣 SecBrain especificando → 🟡 BackBrain + 🟢 FrontBrain → 🔴 TestBrain → 🔵 SubBrain fechando". **Continuam aqui:** DEPLOY-08 (ALTA, pendente do usuário; bloqueia o primeiro deploy da `main`), TST-17, TST-22, DB-07 e TST-16 (dependem de servidor/Docker/banco), ARQ-09 (adiado), CHORE-14 (só 7 dias após o deploy do SEC-33), BUG-25 (decisão), BACKUP-01, SEC-47, SEC-48, SEC-51, SEC-52, TST-04 e DEP-03.
>
> Antes, ainda em 2026-11-04 (decisões do usuário após o Lote 34, repassadas pelo 🤍 MegaBrain; 🔵 SubBrain). Saíram desta lista para `feito.md` (bloco "Decisões do usuário de 2026-11-04 (após o Lote 34)"): **OPS-02** (SMTP de produção configurado pelo usuário, por ora via Gmail; o passo 10 do DEPLOY-08 fica com o pré-requisito atendido) e **CHORE-18** (hostname `ivo-Inspiron-15-3530` confirmado; a guarda já o cobre em minúsculas, sem alteração; a conferência do SAN virou item opcional do DEPLOY-08). O **DEPLOY-08** ganhou a nota "Andamento 2026-11-04" (passo 11 rodado no servidor por engano; na máquina do admin, `db-check-env` ok, mas a `192.168.168.106:3306` não responde e o fingerprint da `deploy/ca-producao.pem` difere do `deploy/db-ca.fingerprint`). O **TST-17** ganhou nota (Docker na máquina do admin sem acesso ao socket; falta `usermod -aG docker`; sem `shellcheck`). O **SEC-54** deixou de citar o CHORE-18 como pendente. O **DOC-09** continua aqui: fatos conferidos, mas o `CLAUDE.md` não foi alterado pelo agente (autorização repassada por agente não vale; o usuário aplica direto). **Seguem pendentes do usuário:** **DEPLOY-08** (ALTA; bloqueia o primeiro deploy da `main`), **DOC-09** e **TST-17**. O restante do backlog não mudou.
>
> Antes, ainda em 2026-11-04 (fechamento do Lote 34, 🔵 SubBrain). O **Lote 34** (SEC-53, SEC-46, SEC-50, SEC-49, CHORE-15, CHORE-17 e BUG-26) foi concluído e está em `feito.md` (**não commitado**, pendente do usuário). Entraram 7 cards no bloco "Achados do Lote 34", logo abaixo: **BUG-27** (MÉDIA: `db-reset`/`db-rebuild` com `db-down` e `db-seed` como pré-requisitos irmãos) e **SEC-54**, **TST-19**, **SEC-55**, **TST-20**, **TST-21** e **TST-22** (BAIXA). O **DOC-09** ganhou o texto do SEC-46, do SEC-53 e do SEC-50 (continua aguardando a autorização do usuário). **Seguem pendentes do usuário:** **DEPLOY-08** (ALTA; bloqueia o primeiro deploy da `main`), **OPS-02** (SMTP de produção), **CHORE-18** (hostname do servidor), **DOC-09** (alteração do `CLAUDE.md`) e **TST-17** (precisa de uma máquina com Docker). O restante do backlog não mudou.
>
> Antes, ainda em 2026-11-04 (abertura do Lote 34, 🔵 SubBrain). O usuário pediu "continue com as tarefas a fazer e as fazendo". O 🤍 MegaBrain abriu o **Lote 34 de 2026-11-04** e 7 cards saíram desta lista para `fazendo.md`: **SEC-50**, **SEC-53**, **CHORE-17** e **BUG-26** (bloco "Achados do Lote 33 (DEPLOY-07)") e **SEC-46**, **SEC-49** e **CHORE-15** (bloco "Achados dos Lotes 30 e 31"). Passo atual do lote: "🟣 SecBrain especificando → 🟡 BackBrain → 🔴 TestBrain → 🔵 SubBrain fechando". **Continuam aqui, pendentes do usuário:** **DEPLOY-08** (ALTA; bloqueia o primeiro deploy da `main`), **OPS-02** (SMTP de produção), **CHORE-18** (hostname do servidor), **DOC-09** (alteração do `CLAUDE.md`, exige autorização) e **TST-17** (precisa de uma máquina com Docker). O restante do backlog não mudou.
>
> Antes, ainda em 2026-11-04 (🔴 TestBrain): o **TST-18** (estágio "Testes Frontend" no Jenkinsfile, pedido do usuário) foi criado e concluído no mesmo dia, direto em `feito.md`; o backlog abaixo não mudou.
>
> Antes, ainda em 2026-11-04 (fechamento do Lote 33, 🔵 SubBrain). O **DEPLOY-07** foi concluído no código e nas docs e está em `feito.md` (não commitado). O **DEPLOY-05** saiu desta lista (substituído pelo DEPLOY-07; registro em `feito.md`). Entraram 11 cards novos no bloco "Achados do Lote 33 (DEPLOY-07)", logo abaixo: **DEPLOY-08** (ALTA, pendente do usuário: executar a §11 de `docs/deploy-servidor.md` no servidor; **bloqueia o primeiro deploy da `main`**), **SEC-50**, **SEC-53**, **TST-17**, **OPS-02** e **CHORE-18** (MÉDIA) e **SEC-51**, **SEC-52** (risco aceito), **CHORE-17**, **BUG-26** e **DOC-09** (BAIXA). O restante do backlog não mudou.
>
> Antes, ainda em 2026-11-04 (abertura do Lote 33, 🔵 SubBrain). O primeiro build da `main` falhou no estágio "CA do banco" (`bind source path does not exist: /etc/mysql/ssl/ca.pem`): o `deploy/mariadb-docker.sh` ainda não rodou no servidor. O usuário informou que **não existe MySQL no ambiente**; o **DEPLOY-05** foi **substituído pelo DEPLOY-07** (instalação limpa do MariaDB 11.4 de produção, CA nova de produção, CA de dev preservada, seed em produção), card novo que foi direto para o **Lote 33 de 2026-11-04** (`fazendo.md`). O DEPLOY-05 fica abaixo só como histórico, com a nota de substituição. O restante do backlog não mudou.
>
> Antes, em 2026-11-02 (fechamento dos Lotes 30 e 31, 🔵 SubBrain). Os Lotes 30 e 31 foram fechados (`feito.md`). Voltaram para esta lista:
> - **DEPLOY-05 (ALTA, pendente do usuário):** a revisão do `deploy/mariadb-docker.sh` foi concluída no Lote 30; falta a **execução no servidor** (passo a passo em `docs/deploy-servidor.md` §11). **Bloqueia o primeiro deploy da `main`:** sem a rede `rotaperfumes-db` e o container `rotaperfumes-mariadb`, o compose falha e o pipeline para.
> - **ARQ-09 (Lote 32):** **adiado** pelo 🤍 MegaBrain por recomendação do 🟣 SecBrain (não executar antes do primeiro teste em produção).
>
> Entraram 12 cards novos no bloco "Achados dos Lotes 30 e 31": SEC-46, BACKUP-01 (MÉDIA) e SEC-47, CHORE-14, SEC-48, DB-07, SEC-49, CHORE-15, BUG-25, FE-18, TST-16, CHORE-16 (BAIXA). O **TST-14** foi cancelado (`feito.md`). Continuam aqui o **TST-04** (bloqueado) e o **DEP-03** (acompanhamento; agora 16 moderadas do `minimatch` via `eslint`, só dev, 0 altas). **Em seguida:** merge de todas as branches na `main` pelo 🤍 MegaBrain; antes do primeiro deploy, o usuário executa o DEPLOY-05.
>
> Antes, ainda em 2026-11-02 (abertura dos Lotes 30, 31 e 32, 🔵 SubBrain). O usuário pediu para executar **todas** as pendências, aprovou antecipadamente as decisões de produto/contrato pendentes nos cards (inclusive os textos do **CHORE-07** e os nomes de cookies do **SEC-33**) e pediu, ao final, o merge de todas as branches na `main` e o commit da `main`, para testar o site em produção. Saíram desta lista para `fazendo.md`:
> - **Lote 30 de 2026-11-02 (somente MariaDB + infra/TLS):** DEPLOY-05, CHORE-13, DEPLOY-06, DEPLOY-04, DOC-08, TST-15, SEC-45, SEC-39, SEC-31, SEC-30, SEC-40, SEC-41, SEC-42, TST-12, DB-04 e DB-06 (DB-04 e DB-06 tratados juntos).
> - **Lote 31 de 2026-11-02 (backend/frontend, BAIXA):** TST-06, BUG-18, TST-07, DOC-06, FE-15, UX-02, CHORE-07, BUG-22, SEC-32, SEC-33, BUG-23, CHORE-08, FE-16, CHORE-09, SEC-34, SEC-36, BUG-24, CHORE-10, FE-17, CHORE-11, SEC-38, LOG-03, CHORE-04 e CHORE-05.
> - **Lote 32 de 2026-11-02:** ARQ-09 (valores monetários em centavos).
>
> **Triagem do 🤍 MegaBrain (somente MariaDB):** **DOC-07** cancelado (substituído pelo DOC-08) e **DEPLOY-03** cancelado (sem MySQL), ambos em `feito.md` com o motivo. **Ficam aqui:** **TST-14** (bloqueado: o usuário desta máquina não tem acesso ao socket do Docker), **TST-04** (bloqueado: o `.env` local aponta para o banco de produção, e os agentes não rodam `make db-*` nele) e **DEP-03** (acompanhamento do upstream; a conferência entra no Lote 31 e, sem correção publicada, o card continua aqui). Os blocos de origem que ficaram sem cards saíram desta lista; o histórico está nas notas abaixo e em cada card (`fazendo.md`).
>
> Antes, em 2026-11-02 (pedido do usuário, sem execução nesta rodada). **Somente MariaDB:** o MySQL deixa de ser usado. Entrou o bloco "Migração para somente MariaDB", logo antes dos GAPs da ARQ-00, com **DEPLOY-05** (subir o MariaDB em Docker com `deploy/mariadb-docker.sh` e aposentar o MySQL, ALTA), **CHORE-13** (`Makefile`/`.env.example`/`CLAUDE.md`), **DEPLOY-06** (`setup-servidor.sh`, `Jenkinsfile`, compose), **DOC-08** (docs; substitui o DOC-07), **TST-15** (testes) e **SEC-45** (revisão do script), todos MÉDIA. O 🤍 MegaBrain deve fazer a triagem de DOC-07, DB-06, DEPLOY-03, TST-14, SEC-39 e SEC-31 ao abrir o lote.
>
> Atualizado em 2026-11-01 (fechamento do Lote 29, 🔵 SubBrain). O **Lote 29** foi fechado (`feito.md`; não commitado, pendente do usuário). Entraram quatro cards no bloco "Achados do Lote 29", logo após o DOC-07: **DB-06** (`ONLY_FULL_GROUP_BY` diferente entre dev MariaDB e prod MySQL 8, MÉDIA), **DEPLOY-03** (dump do MariaDB 11.x recusado pelo MySQL 8, MÉDIA), **DEPLOY-04** (`setup-servidor.sh` completo recusa Jenkins em contêiner; numeração dos passos, MÉDIA) e **TST-14** (`TestDEPLOY02_ValidateConfig_Docker` com Docker, BAIXA). O **TST-11** saiu desta lista (concluído). O DOC-07 ganhou nota sobre o `IMPORTAR_DUMP` e o SEC-40 ganhou nota sobre o `DB_TLS_CA_HOST_PATH`.
>
> Antes, ainda em 2026-11-01 (novas decisões do usuário no Lote 29, 🔵 SubBrain). Saíram desta lista para `feito.md`: **TST-13** (cancelado pelo usuário: "não irei conferir esse script agora") e **SEC-43** (fechado como "não corrigir": ambiente de dev, risco aceito pelo usuário). Entrou o **DOC-07** (MÉDIA, bloco "Achados do Lote 29"): revisar docs, `CLAUDE.md` e `setup-servidor.sh` para a realidade (prod MySQL 8.0, dev MariaDB). O card novo **SEC-44** (TLS do SEC-29 no ambiente real) foi direto para o Lote 29 (`fazendo.md`).
>
> Antes, ainda em 2026-11-01 (fechamento parcial do Lote 29, 🔵 SubBrain). SEC-29, TST-09, TST-08 e SETUP-01 foram concluídos (`feito.md`); SEC-37 e DEPLOY-02 seguem no Lote 29 (`fazendo.md`). Dos concluídos saíram sete cards, no bloco "Achados do Lote 29", logo após o LOG-03: **SEC-39** (`REQUIRE SSL` nos scripts de setup, MÉDIA), **SEC-40** (confiança na CA do workspace/`DB_TLS_CA_PATH`, BAIXA), **SEC-41** (`deploy/dump-local.ps1` sem TLS, BAIXA), **SEC-42** (aviso com TLS desligado e host remoto, BAIXA), **SEC-43** (credencial fraca no `.env` local, MÉDIA, ação do usuário), **TST-12** (testes de TLS faltantes, BAIXA) e **TST-13** (validar os scripts de setup no Windows e o MariaDB 11.4 no Linux, MÉDIA). O TST-11 ganhou nota de andamento (flake corrigido no TST-09; falta conferir e fechar).
>
> Antes, ainda em 2026-11-01 (decisões do usuário no Lote 29, 🔵 SubBrain). Backlog sem cards novos nem removidos: o **TST-10** foi cancelado (em `feito.md`), o **SETUP-01** (scripts de instalação do ambiente) entrou direto no Lote 29 (`fazendo.md`) e o **TST-09** foi desbloqueado (credencial do `.env` voltou a funcionar). No FE-16, a referência ao TST-10 passa a valer para o SETUP-01 (Node 22 via nvm).
>
> Antes, ainda em 2026-11-01 (abertura do Lote 29, 🔵 SubBrain). O 🤍 MegaBrain abriu o **Lote 29**, a pedido do usuário, com os 6 cards de prioridade MÉDIA do backlog, que saíram desta lista para `fazendo.md`: **TST-08** (bloco "Achados do Lote 24"), **SEC-29** e **DEPLOY-02** (bloco "Achados do DB-03"), **TST-09** e **TST-10** (bloco "Achados do Lote 25") e **SEC-37** (bloco "Achados do Lote 28"). O TST-09 segue bloqueado pela credencial do banco (depende do usuário). O CHORE-12 (rename `apis/` → `backend/`) foi commitado em `c4c87af` (branch `lote-27`).
>
> Antes, em 2026-10-31 (fechamento do Lote 28, 🔵 SubBrain). O **Lote 28** (SEC-19, SEC-23 e SEC-35) foi concluído e está em `feito.md` (não commitado, pendente do usuário). Dele saíram cinco cards, no bloco "Achados do Lote 28", logo após o TST-11: **SEC-37** (secret de teste do Turnstile aceita em produção, MÉDIA), **FE-17** (depois da troca de senha o frontend vai para `/dashboard` com a sessão já cortada, BAIXA), **CHORE-11** (`apiResetPassword` morta no frontend, BAIXA), **SEC-38** (`posseDoPagamento`/`posseDoPagamentoTravada` escondem erro de banco como 404, BAIXA) e **LOG-03** (recusa do captcha logada sem o motivo, BAIXA). O bloco "GAPs de segurança da auditoria ARQ-00" fica todo concluído.
>
> Antes, ainda em 2026-10-31 (abertura do Lote 28, 🔵 SubBrain). O 🤍 MegaBrain abriu o **Lote 28** e 3 cards saíram desta lista para `fazendo.md`: **SEC-35** (bloco "Achados do Lote 27") e **SEC-19** e **SEC-23** (bloco "GAPs de segurança da auditoria ARQ-00", que fica sem cards pendentes). O **Lote 27** foi commitado pelo usuário (`a7e2d4a`, branch `lote-27`).
>
> Antes, ainda em 2026-10-31 (fechamento do Lote 27, 🔵 SubBrain). O **Lote 27** (SEC-14, SEC-15, SEC-16, SEC-17, SEC-24, SEC-25, BUG-14, BUG-15 e SEC-26) foi concluído e está em `feito.md` (não commitado, pendente do usuário). Dele saíram cinco cards, no bloco "Achados do Lote 27", logo após o SEC-34: **SEC-35** (`ExcluirPagamento`/`CriarPagamento` checam fora de lock, MÉDIA), **SEC-36** (origens de dev sempre na allowlist de CORS, BAIXA), **BUG-24** (corrida entre troca e reset de senha, BAIXA), **CHORE-10** (`ErrImagemMuitoGrande` com `KindMuitoGrande` e `maxLength={100}` nas buscas do frontend, BAIXA) e **TST-11** (teste instável do IMG-01, BAIXA). SEC-19 e SEC-23 continuam no bloco "GAPs de segurança da auditoria ARQ-00".
>
> Antes, ainda em 2026-10-31 (abertura do Lote 27, 🔵 SubBrain). O 🤍 MegaBrain abriu o **Lote 27** e 9 cards saíram desta lista para `fazendo.md`: **BUG-14** e **BUG-15** (topo da lista), **SEC-26** (bloco "Achados do ARQ-05") e **SEC-14, SEC-15, SEC-16, SEC-17, SEC-24 e SEC-25** (bloco "GAPs de segurança da auditoria ARQ-00"). Critério: cards de backend/testes de prioridade MÉDIA (mais SEC-16 e SEC-25, BAIXA, que tocam os mesmos pontos), sem decisão de produto pendente. SEC-19 e SEC-23 (backend + frontend + config) continuam aqui, para o lote seguinte.
>
> Antes, ainda em 2026-10-31 (fechamento do Lote 26, 🔵 SubBrain). O **IMG-01** (imagem do produto) foi concluído e está em `feito.md` (não commitado, pendente do usuário). Dele saíram dois cards BAIXA, no bloco "Achados do IMG-01", logo após o FE-16: **CHORE-09** (job de limpeza de imagens órfãs, inclusive `.tmp-*` antigos e o caso de commit ambíguo) e **SEC-34** (WebP não é reencodado; EXIF e bytes anexados ficam no arquivo).
>
> Antes, ainda em 2026-10-31 (🔵 SubBrain). O 🤍 MegaBrain abriu o **Lote 26** com o card **IMG-01** (imagem do produto), pedido direto do usuário que foi para `fazendo.md` sem passar por esta lista. O backlog abaixo não mudou.
>
> Antes, em 2026-10-30 (fechamento do Lote 25, 🔵 SubBrain). O **Lote 25** (BUG-16, SEC-27, BUG-17, BUG-19, CHORE-03, CHORE-06, BUG-20 e BUG-21) foi concluído e está em `feito.md` (não commitado, pendente do usuário). Dele saíram quatro cards, no bloco "Achados do Lote 25", logo após o TST-08: **TST-09** (3 testes de integração e conferência do CHECK `chk_pagamentos_taxa_pct`, bloqueados pela credencial do banco; depende do usuário, MÉDIA), **TST-10** (`make test-frontend`/`make frontend-deps` em disco exFAT com Node 18, MÉDIA; card próprio, separado do TST-08), **CHORE-08** (4 observações do 🔴 TestBrain, BAIXA) e **FE-16** (rodar o `next build`, BAIXA). O UX-02 avançou em parte (a orientação "Edite o registro existente." no 409 do estoque entrou com o CHORE-06).
>
> Antes, em 2026-10-29 (especificação do Lote 25). Da especificação do 🟣 SecBrain para o Lote 25 saíram três cards BAIXA, no bloco "Achados da especificação do Lote 25", logo após o DEPLOY-02: **UX-02** (R4, UX do ajuste manual de estoque após o snapshot do faturamento), **CHORE-07** (R6, mensagens imprecisas de `vendas/erros.go`) e **BUG-22** (toggle de ativo de clientes e usuários). Dos achados do Lote 24 saíram, no bloco seguinte ("Achados do Lote 24"), **SEC-32** (enumeração por tempo com hash vazio/malformado, BAIXA), **SEC-33** (prefixo `__Host-` nos cookies, BAIXA), **BUG-23** (refresh de usuário apagado → 500, BAIXA) e **TST-08** (`make test` falha com `covdata`, MÉDIA). Os riscos R2 e R3 viraram BUG-20 e BUG-21 dentro do Lote 25 (`fazendo.md`).
>
> Antes, ainda em 2026-10-29: o **Lote 25** (BUG-16, SEC-27, BUG-17, BUG-19, CHORE-03 e CHORE-06) foi aberto pelo 🤍 MegaBrain depois de o usuário tomar as decisões de produto pendentes nesses cards; os seis saíram desta lista para `fazendo.md`, onde ficam na fila até o fechamento do Lote 24 (decisões registradas em cada card). No bloco "Achados do ARQ-05" ficam SEC-26 e TST-06; no bloco "Achados do DASH-01" ficam DOC-06 e FE-15. O **Lote 23** foi commitado pelo usuário (`c87a30d` e `186b15f`).
>
> Ainda em 2026-10-29: o **Lote 24** (SEC-18, SEC-20, SEC-21 e SEC-22, os 4 GAPs de segurança de prioridade ALTA da auditoria ARQ-00) foi aberto e os quatro cards saíram desta lista para `fazendo.md`. No bloco "GAPs de segurança da auditoria ARQ-00" ficam SEC-14 a SEC-17, SEC-19 e SEC-23 a SEC-25.
>
> O **Lote 23** (DB-03, MySQL → MariaDB 11.4) foi concluído e está em `feito.md`. Da avaliação de riscos do DB-03 saíram cinco cards no bloco "Achados do DB-03", logo após o FE-15: **SEC-29** (TLS Go ↔ MariaDB, MÉDIA), **SEC-30** (ed25519, BAIXA), **SEC-31** (imagem do Jenkins por digest, BAIXA), **DB-04** (`ONLY_FULL_GROUP_BY`, BAIXA) e **DEPLOY-02** (hardening do servidor mesmo com bind já aberto, MÉDIA). O SEC-28 foi resolvido pelo DB-03 e saiu desta lista (está em `feito.md`, Lote 23).
>
> O **Lote 22** (DASH-01, funil de vendas no Dashboard) foi concluído e está em `feito.md`. Da revisão do DASH-01 saíram três cards BAIXA, no bloco "Achados do DASH-01", logo após o TST-07: **DOC-06** (comentário "escopo por carteira" em `rotas.go`, do 🟣 SecBrain), **BUG-19** (`arredondar2` com negativos, do 🔴 TestBrain) e **FE-15** (aviso de desligado e seleção da janela após erro no `FunilVendas`, do 🔴 TestBrain). A limitação do setup local em disco sem symlink (`npm ci --no-bin-links`) ficou só documentada em `postman/README.md` ("Subindo o ambiente"), sem card.
>
> Em 2026-10-16: o Lote 21 (2026-10-13 a 2026-10-16, reestruturação da API em DDD + Clean Code + Design Patterns) foi **concluído**: ARQ-00 a ARQ-08 estão em `feito.md` (estrutura final em `docs/arquitetura-ddd.md` §10; ADR em `docs/adr-001-reestruturacao-ddd.md`). Os cards abaixo saíram do lote e ficam fora do escopo dele. Os cards SEC-14 a SEC-25 são GAPs de segurança levantados pelo 🟣 SecBrain na auditoria do ARQ-00 (`docs/arq-00-invariantes-seguranca.md`), fora do escopo do Lote 21. Também saíram do ARQ-00 os cards ARQ-09 (Dinheiro em centavos), BUG-14 (VincularCliente sem transação) e BUG-15 (troca de senha sem transação), no topo da lista. Do ARQ-05 (2026-10-15) saíram BUG-16, SEC-26, SEC-27, TST-06 e BUG-17, no bloco "Achados do ARQ-05", logo após o BUG-15. Do ARQ-06 (2026-10-15) saiu o BUG-18, logo após o BUG-17. Do ARQ-07 (2026-10-16) saíram SEC-28 e TST-07, logo após o BUG-18.
>
> **Regras do Lote 21 (decisões de 2026-10-13, ver ARQ-00 em `feito.md`), que continuam valendo para o código daqui em diante:**
> - **P1, logs (altera a regra LOG-02):** `vlog` em 4 níveis: rota automática no router, caso de uso via `defer`, pontos de decisão e 1 linha por query (~600 chamadas no lugar de ~3.726). Mesmo formato de log; TST-05 deve continuar verde. Nesses cards, o critério "logs `vlog` preservados" significa formato e TST-05 preservados e cobertura pelos 4 níveis, não a manutenção de cada chamada atual. Regra no código: comentário de `backend/shared/vlog/vlog.go`; documentação: `docs/logs-verbose.md` (atualizada no ARQ-08).
> - **X1, contrato HTTP:** a única exceção ao "contrato HTTP inalterado" é a do ARQ-01 (métodos das 13 rotas, `/api/senha-historico/2/extra` e, pelo parse estrito de `{user_id...}` (R-f), `/api/senha-historico/0` e `/-1` passando a 400).
>
> Histórico anterior (2026-10-11): o 🔴 TestBrain executou `docs/roteiro-teste-manual-lote12.md` (24 aprovados, 1 falhou, 13 não verificáveis de 38; ver a seção "Execução (2026-10-07, 🔴 TestBrain)" no fim do roteiro). Daí saíram os cards BUG-13, LOG-01, DOC-05 e TST-04. BUG-13, LOG-01 e DOC-05 foram concluídos no Lote 16 (2026-10-07; ver `feito.md`). O LOG-02 (Lote 17) gerou o TST-05, e o DEP-00 (Lote 19) gerou o DEP-01 e o DEP-02. No Lote 20 (2026-10-11; ver `feito.md`) foram concluídos DEP-01, DEP-02 e TST-05; o TST-04 avançou parcialmente (só faltam 1.1, 1.2 e 1.7, que dependem do usuário rodar o `make`) e do DEP-01 saiu o card DEP-03 (BAIXA). Em 2026-10-16 o DEPLOY-01 (Lote 13) foi concluído e o MAIL-01 foi cancelado (sem servidor de e-mail; a API usa o envio Noop). Os dois estão em `feito.md`.

---

# Achados do Lote 42, 2026-11-07

> Pendências do Lote 42 (`feito.md`, "Lote 42 de 2026-11-06/07: 4 cards concluídos"), levantadas nas especificações e na revisão do 🟣 SecBrain e na rodada final de testes. Nenhuma bloqueou o fechamento do lote nem o push. Especificações: `tarefas/lote-42-backup-01-spec.md` e `tarefas/lote-42-deploy-09-10-spec.md`. Caminhos: `SH` = `backend/shared`.
>
> Em 2026-11-07, **SEC-73**, **TST-28**, **BACKUP-03**, **BACKUP-04**, **BACKUP-05** e **BACKUP-07** saíram deste bloco para `fazendo.md` (Lote 43).

---

## BACKUP-06: `clientes` vazia só gera `AVISO` no backup — prioridade BAIXA

**Status:** pendente.
**Camadas:** Database, Deploy, Testes
**Responsável:** 🟣 SecBrain → 🌸 DataBrain → 🔴 TestBrain
**Origem:** desvio 2 do 🌸 DataBrain (`tarefas/lote-42-backup-01-spec.md`, "Desvios da especificação"): a produção nasceu sem clientes, e a reprovação estrita recusaria o backup de hoje.

**Descrição:** um banco com `clientes` esvaziada ainda passa como "backup bom", só com aviso.

**Critério de aceite (escolher uma):** (a) reprovar `clientes` vazia a partir do momento em que a produção tiver clientes; ou (b) comparar as contagens por tabela com o par anterior e reprovar queda brusca, com válvula registrada. Decisão registrada e testada.

---

## DEPLOY-11: fase 2 do `--dev-ip` (recusar e remover o código) — prioridade BAIXA

**Status:** pendente; **só depois do item 7 do DEPLOY-08**.
**Camadas:** Deploy, Testes, Documentação
**Responsável:** 🟡 BackBrain → 🔴 TestBrain → 🔵 SubBrain
**Origem:** DEPLOY-10, item "`--dev-ip`" (`tarefas/lote-42-deploy-09-10-spec.md`, "Itens antigos").

**Descrição:** no DEPLOY-10 (fase 1), o `--dev-ip` ficou só como transição, sem conta. Depois do `--recriar` do item 7 do DEPLOY-08, ele perde o uso.

**Critério de aceite:** `--dev-ip` recusado como o `--rede-lan`; código da porta publicada, da chain `ROTAPERFUMES-DB` e da unit de firewall removido (com a limpeza de uma unit antiga, se existir); testes e docs (§3.3 e §11, passo 11, de `docs/deploy-servidor.md`) atualizados.

---

## SEC-74: `/var/log/rotaperfumes-dba.log` num `/var/log` gravável pelo grupo `syslog` — prioridade BAIXA

**Status:** pendente.
**Camadas:** Deploy, Segurança
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** revisão do 🟣 SecBrain, Lote 42.

**Descrição:** o registro de reserva do `rotaperfumes-dba` (quando o `logger` falha) grava em `/var/log/rotaperfumes-dba.log`. No Ubuntu, `/var/log` tem escrita para o grupo `syslog`, e o `O_NOFOLLOW` cobre só o último componente do caminho.

**Critério de aceite:** registro de reserva num diretório próprio de root (ex.: `/var/log/rotaperfumes/`, 700) conferido antes de gravar, ou risco aceito e registrado; teste do caminho escolhido.

---

## DB-10: comparar o schema do dump com `sql/ordem-ddl.txt` em container sem rede — prioridade BAIXA

**Status:** pendente.
**Camadas:** Database, Deploy, Testes
**Responsável:** 🟣 SecBrain → 🌸 DataBrain → 🔴 TestBrain
**Origem:** sugestão do 🟣 SecBrain (`tarefas/lote-42-deploy-09-10-spec.md`, "Riscos que restam").

**Descrição:** na etapa 6 do `restaurar-maquina.sh` (e no runbook), as migrações `sql/*_alter_*` dependem de conferência humana. Um container sem rede pode aplicar a DDL de `sql/ordem-ddl.txt` e comparar o schema com o do dump, apontando as migrações que faltam.

**Critério de aceite:** comparação automatizada, sem rede e sem dados reais na saída; lista das migrações faltantes; testes com dump de schema antigo e atual.

---

## DEPLOY-12: `IMPORTAR_DUMP` do Jenkins sem a guarda de backup — prioridade BAIXA

**Status:** pendente.
**Camadas:** Deploy, CI
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** decisão D6 da especificação do BACKUP-01 (`tarefas/lote-42-backup-01-spec.md`): card próprio pedido e ainda não aberto (conferido no fechamento do Lote 42).

**Descrição:** o estágio `IMPORTAR_DUMP` (`Jenkinsfile:~223-255`) grava em produção sem a exigência de backup recente que o `--recriar` e o `seed-producao.sh --forcar` ganharam. O Jenkins não lê o diretório 700 dos backups e pode rodar em container, então a checagem é outro desenho. Com o dump do BACKUP-01 (`--skip-add-drop-table`), a importação já falha em banco populado.

**Critério de aceite:** desenho definido pelo 🟣 SecBrain (ex.: recusa com tabelas existentes, ou carimbo do último backup exposto ao Jenkins sem segredo), implementado e testado; ou risco aceito pelo usuário e registrado.

---

## SEC-75: senha do banco por `-e MYSQL_PWD` nos usos antigos — prioridade BAIXA

**Status:** pendente.
**Camadas:** Deploy, CI, Documentação
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** especificações do Lote 42 (`tarefas/lote-42-backup-01-spec.md`, seção 4; `tarefas/lote-42-deploy-09-10-spec.md`, "Riscos que restam"): card pedido e ainda não aberto (conferido no fechamento do Lote 42).

**Descrição:** com `-e MYSQL_PWD`, o valor fica no `Config.Env` do container e aparece em `docker inspect`. O risco prático é moderado (o grupo `docker` já equivale a root), mas evita vazamento em diagnósticos. Os scripts novos do Lote 42 usam option file montado. Usos antigos: `deploy/mariadb-docker.sh` (`cliente()`, `:~1131`), `deploy/seed-producao.sh` (`:~532`), `Jenkinsfile` (`:~227-255`, estágio "Importar dump") e o `admin_sql` de `docs/deploy-servidor.md` §11, passo 7 (`:~736-737`).

**Critério de aceite:** os quatro usos trocados por option file montado só para leitura (fora de `container/`), ou risco aceito e registrado por uso; teste estático de ausência de `-e MYSQL_PWD`; docs atualizadas.

---

# Achados do Lote 41, 2026-11-06

> Pendências do Lote 41 (`feito.md`, "Lote 41 de 2026-11-06: 1 card concluído"), levantadas na revisão do 🟣 SecBrain sobre o SEC-71. Nenhuma bloqueou o fechamento do lote. As linhas são as da árvore no fechamento do lote. Caminhos: `SH` = `backend/shared`.

---

## SEC-72: ressalvas do SEC-71 (avisos do `mariadb-docker.sh`, "Restart from Stage" e testes) — prioridade BAIXA

**Status:** pendente. O **C4 depende de decisão do usuário**.
**Camadas:** Deploy, Testes
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** revisão do 🟣 SecBrain no Lote 41 (aprovado com ressalvas; a C7 foi resolvida no fechamento).

**Itens:**
- **C1. `deploy/mariadb-docker.sh:961`:** com CA de dev no host, o cabeçalho chama a CA de "PRODUCAO" e mostra o fingerprint. Trocar o rótulo nesse caso, mantendo igual a saída de produção.
- **C2. `deploy/mariadb-docker.sh:1000-1002`:** com `--dev-ip` e CA de dev, o passo 3 manda copiar para `deploy/ca-producao.pem`. Omitir nesse caso (raro; `--dev-ip` não se aplica desde 2026-11-05).
- **C3. `deploy/mariadb-docker.sh:526` e `:949`:** só o primeiro certificado do `ca.pem` é conferido. Mitigado por `deploy/validar-ca-banco.sh:96-119`, que confere todos os blocos.
- **C4. `Jenkinsfile:172` e `deploy/validar-ca-banco.sh:62-73`:** "Restart from Stage" em "CA do banco" pula "Testes Go"; com `d150…` commitado e CA de dev no host, a validação passa. Exige ação de admin e duas falhas juntas. **Decisão do usuário:** recusar também nesse estágio (terceira cópia da lista) ou aceitar o risco.
- **C5. `SH/tests/deploy/mariadb_docker_test.go:321-322`:** falta conferir o aviso `contem o fingerprint da CA de DEV` nos casos "lista com essa CA" e "lista com as duas".
- **C6. `SH/tests/deploy/deploy07_seed_producao_test.go:200`:** o `Contains` aceita o literal em qualquer lugar; conferir dentro do bloco `CA_DEV_FPS=( … )`.

**Critério de aceite:** C1 e C2 corrigidos, com a saída de produção inalterada e coberta por teste; C3 corrigido ou aceito, com a decisão registrada no card; decisão do usuário sobre o C4 registrada e aplicada; C5 e C6 nos testes; suíte verde.

---

# Achados do Lote 40, 2026-11-06

> Pendências do Lote 40 (`feito.md`, "Lote 40 de 2026-11-06: 1 card concluído"), levantadas no SEC-63 e nas revisões do 🟣 SecBrain. Nenhuma bloqueou o fechamento do lote. As linhas são as da árvore no fechamento do lote. Caminhos: `SH` = `backend/shared`.

---

## SEC-66: `config.load` e `DSN()` não validam as `DB_*`, e um `DB_NAME` malicioso desvia o SEC-42 (`SH/config/config.go:~203-206`) — prioridade BAIXA

**Status:** pendente.
**Camadas:** Backend (config, CLIs), Testes
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** Lote 40 (SEC-63).

**Descrição:** o ganho real seria desviar o SEC-42: com `DB_HOST=127.0.0.1`, `ValidarDBTLSObrigatorio` não exige TLS, enquanto um `DB_NAME` malicioso aponta o `Addr` para host remoto, sem TLS e sem aviso. O log de `backend/rotaperfumes-api/cmd/server/main.go:37` mostraria o host errado. O `TestSEC63_Go_DSNNaoTrocaEndereco` confirma a troca do `Addr`; hoje a proteção do Go vale só em `integracao`.

**Escopo:**
- `SH/config/config.go:~203-206` (leitura) e `:~682` (`DSN()`).
- `SH/tools/resetpassword/resetpassword.go:~121-133` (monta o DSN sem `load`).
- `SH/tools/seedusers/seedusers.go:~311-320` (`DB_NAME=--host=...` entra como argumento, e o último `-h` vence).
- `DB_PORT`, que o `ValidarFormaDB` atual não recebe (`SH/testkit/integracao/integracao.go:~163`).

**Correção:** mover `ValidarFormaDB` para `config`, com `integracao` reaproveitando.

**Critério de aceite:** a API, o `resetpassword` e o `seedusers` recusam valor fora do charset antes de conectar; teste confirma que o `Addr` não muda; paridade `Makefile` × Go do SEC-63 verde.

---

## SEC-67: `make setup` passa os segredos do `.env` a instaladores de terceiros (`Makefile:~264,267`) — prioridade BAIXA

**Status:** pendente.
**Camadas:** Config (Makefile), Testes
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** Lote 40.

**Descrição:** `setup` e `setup-check` rodam sem `$(SEM_SEGREDOS)`. `DB_SENHA`, `MYSQL_PWD`, `JWT_SECRET` e o pepper chegam a `scripts/setup-linux.sh:~435` (nvm), `:~647` (npm global) e `:~652` (Postman). `SH/tests/config/sec53_makefile_segredos_test.go` não cobre `setup`.

**Correção:** aplicar `$(SEM_SEGREDOS)` às duas receitas.

**Critério de aceite:** teste com script falso confirma que os segredos não chegam ao processo; `sec53_makefile_segredos_test.go` cobre `setup` e `setup-check`; `make setup-check` continua funcionando.

---

## SEC-68: `seedusers` ignora `DB_TLS_CA` no cliente (`SH/tools/seedusers/seedusers.go:~311-320`) — prioridade BAIXA (a confirmar)

**Status:** pendente; a confirmar com o cliente real.
**Camadas:** Backend (CLI), Testes
**Responsável:** 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** Lote 40.

**Descrição:** `ArgsClienteMariaDB` monta os argumentos sem TLS/SSL, embora o comentário diga "igual ao MYSQL_OPTS". O `make gen-hash` não conferiria a CA do projeto.

**Correção:** confirmar com o cliente real e usar `--ssl-ca` + `--ssl-verify-server-cert`.

**Critério de aceite:** resultado da confirmação registrado no card; com `DB_TLS_CA`, os argumentos incluem as duas opções, com teste; sem `DB_TLS_CA` no loopback, nada muda.

---

## SEC-69: desligamentos do gate do SEC-63 por ação do operador (`Makefile:~39,200`) — prioridade BAIXA (a provar)

**Status:** pendente; só para reavaliar. **[a provar]** por PoC antes de corrigir.
**Camadas:** Config (Makefile), Testes
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** ressalvas R1 e R2 do 🟣 SecBrain no Lote 40.

**Descrição:**
- **R1:** `--eval='undefine MAKECMDGOALS'` deixa a origem `undefined`, aceita pela allowlist em `Makefile:~39`.
- **R2:** `VARS_DO_ENV=`, `VARS_PROIBIDAS_NO_ENV=`, `RP_ENV_LINHAS_RUINS=`, `ENV_ARQUIVO_LIDO=/dev/null` e `CR=` digitadas ainda ficam sem `override`.

Os dois casos estão fora do modelo pela decisão registrada do 🤍 MegaBrain (`feito.md`, Lote 40).

**Correção proposta (R1):** `$(filter-out $(RP_ALVOS_SEM_BANCO),$(or $(MAKECMDGOALS),sem-goal))`, com o custo de `make` puro com `.env` inválido recusar em vez de mostrar o help.

**Critério de aceite:** PoC registrada; decisão de manter fora do modelo ou corrigir, registrada no card; se corrigir, teste com cliente falso.

---

## SEC-70: o arquivo conferido pode não ser o incluído (`Makefile:~87,92,107,109`) — prioridade BAIXA (a provar)

**Status:** pendente. **[a provar]** por PoC antes de corrigir.
**Camadas:** Config (Makefile), Testes
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** ressalva R3 do 🟣 SecBrain no Lote 40.

**Descrição:** o `grep`/`sed` leem o caminho relativo ao diretório atual, mas o `include` também procura em `-I` e nos diretórios padrão. `make help ENV_ARQUIVO=db-create` faria o make tentar refazer o include rodando a receita com o gate desligado.

**Correção:** condicionar a `$(wildcard ...)` no diretório atual; ajustar `SH/tests/config/sec11_makefile_sql_test.go:~211`.

**Critério de aceite:** PoC só com cliente falso confirma ou descarta os dois caminhos; se confirmados, recusa coberta por teste e `sec11_makefile_sql_test.go` verde.

---

# Achados do Lote 39, 2026-11-05

> Pendências do Lote 39 (`feito.md`, "Lote 39 de 2026-11-05: 12 cards fechados (11 concluídos, 1 aceito)"). Nenhuma bloqueou o fechamento do lote. As linhas do `Makefile` são as da árvore no fechamento do lote. Caminhos: `SH` = `backend/shared`.
>
> **2026-11-06 (fechamento do Lote 40):** o **SEC-63** foi concluído (`feito.md`). O **SEC-64** ganhou um acréscimo.
>
> **2026-11-05 (abertura do Lote 40):** o **SEC-63** foi para o **Lote 40 de 2026-11-05** (`fazendo.md`, com o texto completo). Seguem neste bloco SEC-64, SEC-65, CHORE-20, TST-26, CI-03 e TST-27.

---

## SEC-64: `export` global do `Makefile` entrega `LD_PRELOAD`, `GOFLAGS`, `NODE_OPTIONS` e `BASH_ENV` do `.env` aos filhos (`Makefile:~109`) — prioridade BAIXA

**Status:** pendente.
**Camadas:** Config (Makefile), Testes
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** Lote 39 (SEC-59).

**Descrição:** o `Makefile` exporta tudo. `LD_PRELOAD`, `GOFLAGS`, `NODE_OPTIONS` e `BASH_ENV` no `.env` chegam aos filhos sem `$`, e o `LD_PRELOAD` alcança o shell das guardas. Uma denylist não fecha o problema.

**Correção (decidir uma):** trocar o `export` global por uma lista explícita, ou documentar que as guardas protegem contra engano, não contra quem controla o `.env`.

**Critério de aceite:** decisão registrada; com a lista explícita, teste confirma que essas variáveis não chegam aos filhos e que os alvos `db-*` continuam recebendo as `DB_*`.

**Acréscimo (2026-11-06, Lote 40):** o `export` global está hoje em `Makefile:~131`.
- `GO_TEST_FLAGS`, `SETUP_ARGS`, `SHELLCHECK` e `GERAR_IMAGENS_FLAGS` do `.env` entram em receitas.
- `MAKEFILE_LIST=;cmd;` no `.env` pode injetar comando em `make help` (`Makefile:~247`, a confirmar em teste).
- `$(...)` no ambiente ou na linha de comando expande no parse (`DB_HOST='$(shell touch pwn)' make help`): BAIXA, residual.

**Nota do 🔵 SubBrain para a decisão deste card:** a fronteira registrada no Lote 40 diz que as guardas protegem contra o conteúdo do `.env`. A opção "documentar que as guardas não protegem contra quem controla o `.env`" contradiz essa frase, e os itens acima e o `LD_PRELOAD` são conteúdo do `.env`. O 🤍 MegaBrain precisa conciliar as duas ao decidir este card.

---

## SEC-65: tar do `seed-producao.sh` montado por lista de exclusão (`deploy/seed-producao.sh:~521`) — prioridade BAIXA

**Status:** pendente.
**Camadas:** Deploy (script), Testes
**Responsável:** 🟣 SecBrain → 🟡 BackBrain → 🔴 TestBrain
**Origem:** Lote 39 (SEC-62).

**Descrição:** mesma classe do SEC-62: o tar enviado ao container do build é montado por exclusão, e esse container tem rede. O alcance é só `backend/shared`.

**Correção:** montar o tar a partir do que está rastreado (como no SEC-62) ou por uma lista positiva.

**Critério de aceite:** teste estático confere a forma escolhida; nenhum arquivo não rastreado entra no tar; ensaio do `seed-producao.sh` e `make shellcheck` verdes.

---

## CHORE-20: conferir o digest novo de `golang:1.26.10-alpine` (`GO_IMAGE`) — prioridade BAIXA

**Status:** pendente.
**Camadas:** CI (Jenkinsfile), Deploy (script)
**Responsável:** 🟡 BackBrain (🟣 SecBrain revisa)
**Origem:** Lote 39 (CI-02).

**Descrição:** o `GO_IMAGE` está fixado em `a2012b4a…`, e `golang:1.26.10-alpine` apontou para `df23b876380eb556f22bbba2fa57493e8684fc4fcabf06399fbfb65359978d28`. Não se sabe se esse valor é um índice ou o manifesto de uma plataforma. A imagem roda na rede do banco no seed.

**Ação:** verificar o tipo do digest. Se for índice novo, atualizar nos dois arquivos (`Jenkinsfile` e `deploy/seed-producao.sh`) seguindo o SEC-31 (`docs/deploy-servidor.md` §3.6).

**Critério de aceite:** tipo do digest registrado no card; se houver troca, mesma string nos dois arquivos e `make test` verde.

---

## TST-26: cobertura de execução do SEC-60 no CI, sem `unshare` — prioridade BAIXA

**Status:** pendente.
**Camadas:** Deploy (script), Testes
**Responsável:** 🔴 TestBrain (🟡 BackBrain na extração)
**Origem:** limite do CI-02 no Lote 39.

**Descrição:** os testes com `unshare` pulam no container do CI (172 entradas, incluindo `TestSEC60_Ensaio_*`; o pai aparece PASS) e só rodam na máquina local. O CI roda como root.

**Ação:** extrair o `conferir_dir_privado` e exercitá-lo de verdade sem `unshare`, em diretório fora do `/tmp`.

**Critério de aceite:** no CI (`GO_TEST_IMAGE`, `CI=true`), teste do SEC-60 **executa** (não pula) os casos de diretório privado e de recusa; a cópia do `conferir_dir_privado` continua igual à do `mariadb-docker.sh`.

---

## CI-03: restringir a proibição de `--security-opt` e avaliar endurecimento do estágio "Testes Go" (`ci02_sec62_jenkinsfile_test.go:~62`) — prioridade BAIXA

**Status:** pendente.
**Camadas:** CI (Jenkinsfile), Testes
**Responsável:** 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** Lote 39 (CI-02/SEC-62).

**Descrição:** `SH/tests/deploy/ci02_sec62_jenkinsfile_test.go:~62` proíbe qualquer `--security-opt`, inclusive o `no-new-privileges:true`, que endurece o container.

**Ação:** restringir a proibição a `seccomp`/`apparmor` `unconfined` e avaliar `no-new-privileges` e `--cap-drop ALL` no estágio.

**Critério de aceite:** teste recusa `seccomp=unconfined` e `apparmor=unconfined` e aceita `no-new-privileges:true`; decisão sobre o endurecimento registrada; se aplicado, ensaio do estágio verde.

---

## TST-27: `TestIMG01_FS_Novo_CriaDiretorioESemProbe` falha com `umask 077` — prioridade BAIXA

**Status:** pendente.
**Camadas:** Testes
**Responsável:** 🔴 TestBrain
**Origem:** Lote 39.

**Descrição:** a asserção de `SH/tests/catalogofs/armazenamento_test.go:63` não considera o `umask` no valor esperado.

**Correção proposta:** `assert.Equal(t, fs.FileMode(0o750)&^umaskBits(t), info.Mode().Perm())`.

**Critério de aceite:** teste verde com `umask 022` e com `umask 077`.

---

# Achados do Lote 38, 2026-11-05

> Pendências do Lote 38 (`feito.md`, "Lote 38 de 2026-11-05"), levantadas pelo 🟣 SecBrain, pelo 🟡 BackBrain e pelo 🔴 TestBrain. Nenhuma bloqueou o fechamento do lote. Caminhos: `SH` = `backend/shared`.
>
> **2026-11-05 (fechamento do Lote 39):** CI-02, DB-09, TST-24, TST-25 e SEC-62 foram concluídos e BUG-33 foi fechado como aceito (`feito.md`).
>
> **2026-11-05 (abertura do Lote 39):** os 6 cards deste bloco (**CI-02**, **DB-09**, **TST-24**, **TST-25**, **SEC-62** e **BUG-33**) foram para o **Lote 39 de 2026-11-05** (`fazendo.md`, com o texto completo). O bloco fica sem cards pendentes.

---

# Achados do Lote 37, 2026-11-05

> Pendências do Lote 37 (`feito.md`, "Lote 37 de 2026-11-05"), levantadas pelo 🟣 SecBrain, pelo 🟡 BackBrain e pelo 🔴 TestBrain. Nenhuma bloqueou o fechamento do lote. Caminhos: `SH` = `backend/shared`.
>
> **2026-11-05 (fechamento do Lote 39):** BUG-31, SEC-59, SEC-60, SEC-61, BUG-32 e CHORE-19 foram concluídos (`feito.md`).
>
> **2026-11-05 (abertura do Lote 39):** os 6 BAIXA deste bloco (**BUG-31**, **SEC-59**, **SEC-60**, **SEC-61**, **BUG-32** e **CHORE-19**) foram para o **Lote 39 de 2026-11-05** (`fazendo.md`, com o texto completo). O bloco fica sem cards pendentes.
>
> **2026-11-05 (fechamento do Lote 38):** CI-01, BUG-30 e DB-08 foram concluídos (`feito.md`, "Lote 38 de 2026-11-05: 3 cards concluídos"). Ficam aqui os 6 BAIXA.
>
> **2026-11-05:** os 3 cards MÉDIA deste bloco (**CI-01**, **BUG-30** e **DB-08**) foram para o **Lote 38 de 2026-11-05** (`fazendo.md`). Ficam aqui os 6 BAIXA.

---

# Achados do Lote 36, 2026-11-05

> Pendências do Lote 36 (`feito.md`, "Lote 36 de 2026-11-05"), levantadas pelo 🔴 TestBrain, pelo 🟣 SecBrain e no incidente do `MAKEFLAGS=-k`. Nenhuma bloqueou o fechamento do lote. Caminhos: `SH` = `backend/shared`.

> **2026-11-05 (fechamento do Lote 37):** BUG-29, SEC-58, TST-23 e o item 1 do TST-17 foram concluídos (`feito.md`, "Lote 37 de 2026-11-05"). Fica aqui só o card-resto do TST-17 (itens 2 e 3, pendentes do usuário).
>
> **2026-11-05 (Lote 37):** BUG-29, SEC-58, TST-23 e o item 1 do TST-17 (opções de isolamento do `seed-producao.sh`) foram para o Lote 37 (`fazendo.md`, com o texto completo do TST-17). Fica aqui só o card-resto do TST-17 (itens 2 e 3, pendentes do usuário).

---

## TST-17 (resto, pendente do usuário): execução real com `sudo` dos testes TST15 e TST17 e conferência do `make setup-check` — prioridade MÉDIA

**Status:** pendente **do usuário**. O item 1 (opções de isolamento do `seed-producao.sh`: `--container`, `--rede-nome`, `--ip-admin`, `--segredos-dir`, `--ssl-dir`, `--trabalho-dir`) foi **concluído no Lote 37** (`feito.md`, card "TST-17 (item 1)"); o pré-requisito do teste real do TST17 está atendido e o `TestTST17_SeedProducao_Real` agora **só pula por falta de `TST17_DOCKER=1`**. **Atenção (mudança fail-closed do Lote 37):** o seed recusa se `/root` não for root com 700 ou se `/root/rotaperfumes-mariadb` não for 700.
**Responsável:** usuário (com `sudo`, **nunca no servidor de produção**) → 🔴 TestBrain (conferência)

**Falta:**
2. **Usuário:** rodar com `sudo` o `TestTST15_MariadbDocker_Real` (`sudo TST15_DOCKER=1 ...`) e, depois do item 1, o teste real do TST17 (`TST17_DOCKER=1`), **nunca no servidor de produção**. O agente não usa sudo.
3. **Usuário (observação):** o `make setup-check` já falhava antes do Lote 36 (Go 1.26.0 local ausente/`covdata`, segundo o 🔴 TestBrain); conferir.

**Critério de aceite:** `TestTST15_MariadbDocker_Real` e o teste real do TST17 verdes como root numa máquina que não seja o servidor.

---

# Achados do Lote 35, 2026-11-05

> Ressalvas da revisão final do 🟣 SecBrain no Lote 35 (`feito.md`, "Lote 35 de 2026-11-05"). Nenhuma bloqueou o fechamento do lote. Caminhos: `SH` = `backend/shared`.

> **2026-11-05 (fechamento do Lote 37):** SEC-56, SEC-57 e BUG-28 foram concluídos (`feito.md`, "Lote 37 de 2026-11-05").
>
> **2026-11-05 (Lote 37):** SEC-56, SEC-57 e BUG-28 foram para o Lote 37 (`fazendo.md`). Este bloco fica sem cards pendentes.

---

# Achados do Lote 34, 2026-11-04

> Pendências do Lote 34 (`feito.md`, "Lote 34 de 2026-11-04"), levantadas pelo 🟣 SecBrain e pelo 🔴 TestBrain. Nenhuma bloqueou o fechamento do lote. Caminhos: `SH` = `backend/shared`.

> **2026-11-05 (Lote 36):** TST-22 foi para o Lote 36 (`fazendo.md`). Este bloco fica sem cards pendentes.
>
> **2026-11-05 (Lote 35):** BUG-27, SEC-54, TST-19, SEC-55, TST-20 e TST-21 foram para o Lote 35 (`fazendo.md`).

---

# Achados do Lote 33 (DEPLOY-07), 2026-11-04

> Pendências do DEPLOY-07 (`feito.md`, "Lote 33 de 2026-11-04"). A produção está no ar; o DEPLOY-08 cuida da limpeza pós-instalação (correção de 2026-11-06). A OPS-02 e a CHORE-18 foram concluídas em 2026-11-04 (`feito.md`, "Decisões do usuário de 2026-11-04 (após o Lote 34)").

## DEPLOY-08: limpeza pós-instalação da produção (`docs/deploy-servidor.md` §11 no servidor) — prioridade ALTA (pendente do usuário)

**Status:** pendente **do usuário**. A produção já está **no ar**; **nenhum item da limpeza abaixo foi confirmado como feito**. Corrigido em 2026-11-06 (antes: "instalação parcial" e "bloqueia o primeiro deploy"). Código e docs prontos no DEPLOY-07 (Lote 33, `feito.md`). Substitui a parte de execução do antigo DEPLOY-05.
**Camadas:** Deploy, Database
**Responsável:** usuário (servidor `192.168.168.106`, Jenkins e máquina de dev) → 🔴 TestBrain (conferência dos resultados) → 🔵 SubBrain (fechamento)
**Origem:** fechamento do DEPLOY-07 (2026-11-04).

**Estado real (informado pelo usuário em 2026-11-06):**
- **Produção NO AR:** `rotaperfumes-api-1`, `rotaperfumes-frontend-1` e `rotaperfumes-caddy-1` (porta 8443) no ar; `rotaperfumes-mariadb` no ar.
- **Admin:** o usuário fez login como admin e trocou a senha; isso explica `deve_trocar_senha=0`.
- **Banco:** 13 tabelas, 42 vendedores inativos, admin ativo.
- **Porta:** `192.168.168.106:3306` segue publicada, com a chain `ROTAPERFUMES-DB` liberando só `192.168.168.105`, **contra a decisão de "banco só local"**.
- **Push:** um push na `main` agora é um **deploy em sistema vivo**, com os Lotes 37 a 41 (`main` local à frente de `origin/main`; nada enviado à `origin/main`). O Lote 41 foi commitado em `3685901`, na `main` local, sem push (atualizado na abertura do Lote 42).
- **Ainda no servidor:** `/root/rotaperfumes-ca/ca-key.pem`, `/root/pepper-producao.txt` e `/root/rotaperfumes-mariadb/admin-app.senha` (sem valor desde a troca). `/root/rotaperfumes` não é checkout git.
- **CA:** em uso `CN=RotaPerfumes_MariaDB_CA`, `F3:1E:E8:20:…:04:C2`, igual à linha útil `f31ee820…` de `deploy/db-ca.fingerprint`. **Essa linha está correta.** O desatualizado é o `deploy/ca-producao.pem` local (`F8:DB:AF:59…`, CA anterior).
- **Migração 27:** `senha_historico.created_at` com `IS_NULLABLE=YES` e 0 linhas NULL; `sql/27_*` ainda precisa ser aplicada.

**Limpeza pós-instalação, nesta ordem (todos os itens pendentes do usuário):**

> **Antes de tudo (atualizado em 2026-11-07, fechamento do Lote 42):** o código do Lote 42 (BACKUP-01, BACKUP-02, DEPLOY-10, DEPLOY-09) só chega ao servidor com o **push na `main`** (deploy em sistema vivo, item 5), e o `TestTST15_MariadbDocker_Real` (item 4) deve rodar **como root, fora do servidor**, antes da migração dos itens 6a a 6c. Os itens 6a a 6c entraram entre o 6 e o 7, sem renumerar, para manter as referências de `docs/deploy-servidor.md` §11, passo 11.

1. [ ] `shred` do `/root/rotaperfumes-mariadb/admin-app.senha`.
2. [ ] Backup do banco e das imagens: `docs/deploy-servidor.md`, seção "Backup do banco e das imagens de produto" (o backup manual antigo saiu). Depois do item 6c, `sudo rotaperfumes-backup-banco --agora`; copiar o par para fora do servidor (D1 em aberto).
3. [ ] Comparar o pepper de `/root/pepper-producao.txt` com o da API, **sem imprimir**. Se iguais, guardar no cofre e dar `shred`. Se diferentes, **não apagar** e avisar.
4. [ ] `TestTST15_MariadbDocker_Real` como root (`sudo`), na máquina de dev (ver **TST-17**; nunca no servidor). Agora também confere a conta e o comando do DBA (DEPLOY-10).
5. [ ] `make deps-audit`, depois push na `main` (`IMPORTAR_DUMP` desmarcado) e acompanhar o job. Efeitos: a API reinicia; `ultimo_login_at` gravado antes do deploy aparece 3 h mais cedo, cosmético (DB-08).
6. [ ] Atualizar o checkout: mover `/root/rotaperfumes` para `/root/rotaperfumes.antigo` e `git clone --depth 1` (§11, passo 1).
6a. [ ] **Migração da conta (DEPLOY-10, §11, passo 11), em horário calmo, sem `--recriar`:** `sudo bash deploy/mariadb-docker.sh --dev-ip 192.168.168.105`, uma vez. Cria `rotaperfumes_backup` e `rotaperfumes-admin`, remove `casaos@*` (qualquer host) e instala o `/usr/local/sbin/rotaperfumes-dba`, sem reiniciar o container.
6b. [ ] Conferir: `echo 'SELECT CURRENT_USER()' | sudo rotaperfumes-dba` deve trazer `rotaperfumes-admin@172.30.42.22`. Saída se algo falhar: `root@localhost` (§3.4).
6c. [ ] `sudo bash deploy/backup-banco.sh --instalar` (e `sudo rotaperfumes-backup-banco --status`).
7. [ ] `sudo rotaperfumes-backup-banco --agora` e, até 60 minutos depois, `sudo bash deploy/mariadb-docker.sh --recriar` sem `--dev-ip`, em janela com queda rápida aceitável; conferir que a 3306 sumiu do `docker ps` e que o site responde. **Nunca `--nova-ca`.** Comportamento documentado na §11, passo 2, e no script: a chain `ROTAPERFUMES-DB` fica, só com `-j DROP`, se a unit antiga existir. **A conta `casaos` já não é removida aqui** (saiu no item 6a); um `casaos.senha` existente é mantido, com o aviso "mantido; leve ao pendrive seguro". Depois deste item, o **DEPLOY-11** (fase 2 do `--dev-ip`) fica liberado.
8. [ ] Migração 27 pelo `admin_sql` (§11, passo 7).
9. [ ] §11, passo 3: tirar `ca-key.pem` do servidor, para pendrive cifrado.
10. [ ] Na máquina de dev, apagar `.env.producao` e `deploy/ca-producao.pem`.
11. [ ] Ativar vendedores pela tela Usuários, quando o usuário quiser (§11, passo 10, item 4; SMTP de produção configurado pelo usuário, por ora via Gmail, OPS-02).

**Instalação do zero:** os 12 passos da §11 continuam como referência para esse caso e não devem ser repetidos neste servidor. O `--nova-ca` trocaria a CA em uso. O antigo passo 11 (`.env.producao`) segue **não aplicável**; desde o Lote 42, o passo 11 da §11 é a migração da conta `casaos` (itens 6a a 6c acima). Para máquina nova, use o `deploy/restaurar-maquina.sh` (§12, DEPLOY-09).

**Item opcional (vindo do CHORE-18):** conferir o SAN do certificado no servidor: `openssl x509 -in /etc/mysql/ssl/server-cert.pem -noout -ext subjectAltName` (deve conter `ivo-Inspiron-15-3530` e `192.168.168.106`).

**Andamento 2026-11-04:**
- O usuário rodou os comandos do passo 11 **no servidor, dentro de `deploy/`**: por isso `make: Sem regra para processar o alvo 'db-check-env'`, e `make 192.168.168.106 ...` porque o `<alvo>` do exemplo foi trocado pelo IP. O passo 11 é para a **máquina do admin** (`192.168.168.105`), **na raiz do repositório**.
- Conferido pelo 🤍 MegaBrain na máquina do admin: `make db-check-env ENV_ARQUIVO=.env.producao` → ok. **Porém:**
  - (a) a porta `192.168.168.106:3306` **não responde** desta máquina (timeout de TCP; a 3307 do CasaOS responde);
  - (b) o fingerprint de `deploy/ca-producao.pem` local (`f8dbaf59…aaf70`) **difere** da linha útil de `deploy/db-ca.fingerprint` (`f31ee820…a04c2`).
- ~~**Próximo passo do usuário:** no servidor, conferir `docker port rotaperfumes-mariadb`, o firewall (`ufw status`/`iptables`) e se `192.168.168.105` está no `--dev-ip`; conferir o fingerprint de `/etc/mysql/ssl/ca.pem`; depois recopiar a CA (passo 11.1) e repetir o passo 11.3 na máquina do admin.~~ Superado: o timeout de (a) é o esperado, pois o banco de produção só responde localmente no servidor (decisão do usuário de 2026-11-04). O `deploy/ca-producao.pem` e o `.env.producao` locais da máquina do admin não são usados (podem ser apagados).

**Andamento 2026-11-05:** o passo 11 foi marcado como **não aplicável** em `docs/deploy-servidor.md` (caixa de aviso; texto antigo mantido como referência, caso o `--dev-ip` seja habilitado) e em `docs/Instal_SSL.md` (§8 e §11). O **critério de aceite deixa de depender do passo 11**; as conferências da produção são feitas no servidor, como root, pelo `admin_sql` do passo 7.

**Andamento 2026-11-06 (Lote 41):** o estado registrado na abertura do lote como "instalação parcial" foi corrigido pelo usuário no mesmo dia: ver "Estado real", acima. O item (b) do andamento de 2026-11-04 fica resolvido: o `deploy/ca-producao.pem` local é que está desatualizado. O `deve_trocar_senha=0`, antes com "origem a confirmar", vem da troca de senha feita pelo usuário.

**Correção na §11 (mesma data):** o `sed` + `printf` do passo 4 e o `--nova-ca` sem `--dev-ip` do passo 2 valem para **instalação do zero, com CA nova**, e esse caminho não foi testado no servidor. Não se aplicam ao estado atual.

**Decisão do usuário (2026-11-06): sem passo de descarte de `casaos.senha` e `/root/rotaperfumes.antigo`.** Motivo, nas palavras do usuário: "no passo de validação será necessário colocar essas senhas em um pendrive seguro caso precise da senha". Portanto, **as senhas vão para pendrive seguro**. A decisão se refere a esses dois caminhos; os itens 1 e 3 (`shred` do `admin-app.senha` e do pepper) e o critério de aceite não foram alterados.

**Dúvidas em aberto dessa decisão (para o usuário):**
- Os itens 1 e 3 e o critério de aceite apagam `admin-app.senha` e `pepper-producao.txt`. O item 1 não prevê cópia antes do `shred`, e o item 3 guarda o pepper em "cofre", não em pendrive. Esses dois arquivos também vão para o pendrive seguro?
- A lista acima não tem um "passo de validação" com esse nome. Em que item entra a cópia para o pendrive? Nenhum item novo foi acrescentado.
- `/root/rotaperfumes.antigo` é o checkout antigo (item 6). A cópia para o pendrive é só das senhas que ele contiver, ou da pasta inteira?
- ~~`docs/deploy-servidor.md` §11, passo 2, ainda sugere descartar o `casaos.senha` com `shred -u` "se não for mais usar". O texto não foi alterado.~~ **Resolvido no Lote 42 (DEPLOY-10, conferido no fechamento em 2026-11-07):** a §11, passo 2, e o passo 11 dizem que o `casaos.senha` é mantido, sem uso, e vai para o pendrive seguro; nenhum script o lê ou apaga.
- **Recomendações do fechamento do Lote 42** (`feito.md`, decisões em aberto): `admin-app.senha` com `shred`, sem cópia (a senha já não vale); pepper copiado para pendrive cifrado, longe dos dumps, antes do `shred`; do `/root/rotaperfumes.antigo`, só as senhas.

**Critério de aceite:** `deploy/db-ca.fingerprint` commitado só com o fingerprint da CA de produção; backup copiado para fora do servidor; estágios "CA do banco", "Deploy" e "Smoke test" verdes no push dos Lotes 37 a 41; log da API com `tls=ligado (custom)` e `min=TLS1.3`; `TestTST15_MariadbDocker_Real` verde; `/root/rotaperfumes` como checkout git; porta 3306 fora do `docker ps` e site respondendo; migração 27 aplicada; `/root/rotaperfumes-ca/ca-key.pem`, `/root/pepper-producao.txt` e `/root/rotaperfumes-mariadb/admin-app.senha` apagados do servidor; `.env.producao` e `deploy/ca-producao.pem` apagados da máquina de dev. A ativação dos vendedores (item 11) não condiciona o aceite: fica a critério do usuário.

---

> **2026-11-04 (Lote 34):** SEC-50, SEC-53, CHORE-17 e BUG-26 foram para o **Lote 34** e estão concluídos (`feito.md`).

---

> **2026-11-05 (fechamento do Lote 36):** o TST-17 voltou **parcial** e está no bloco "Achados do Lote 36" (acima).
>
> **2026-11-05 (Lote 36):** TST-17 foi para o Lote 36 (`fazendo.md`); o acesso ao Docker foi resolvido pelo usuário (`usermod -aG docker`; sessão do agente via `sg docker`; `shellcheck` instalado).

---

> **2026-11-04 (decisões do usuário após o Lote 34):** **OPS-02** (SMTP de produção configurado, por ora via Gmail) e **CHORE-18** (hostname `ivo-Inspiron-15-3530` confirmado, sem alteração de código) foram concluídos e estão em `feito.md`.

---

## SEC-51: conta somente-leitura em produção para a máquina do admin — prioridade BAIXA

**Status:** pendente.
**Camadas:** Database, Deploy
**Responsável:** 🟣 SecBrain → 🌸 DataBrain/🟡 BackBrain (`mariadb-docker.sh`) → 🔴 TestBrain
**Origem:** DEPLOY-07 (Lote 33, 2026-11-04).

**Descrição:** com `--dev-ip`, a conta `casaos` da máquina do admin tem `ALL ON rotaperfumes.*` na produção. Para consultas do dia a dia, uma conta só com `SELECT` reduziria o estrago de um engano (a guarda do Makefile não cobre clientes externos, como o `mariadb` manual ou uma IDE). Avaliar uma conta `*_ro` por `--dev-ip`, com senha própria, e deixar o `ALL` só para migrações.

---

## SEC-52: build do `resetpassword` no `seed-producao.sh` sem `--cap-drop ALL` — prioridade BAIXA (risco aceito)

**Status:** **risco aceito** no DEPLOY-07 (2026-11-04); card para reavaliar.
**Camadas:** Deploy
**Responsável:** 🟣 SecBrain → 🟡 BackBrain
**Origem:** DEPLOY-07 (Lote 33, 2026-11-04).

**Descrição:** a etapa 5 do `deploy/seed-producao.sh` compila o `resetpassword` num container `GO_IMAGE` com `--security-opt no-new-privileges:true`, mas sem `--cap-drop ALL`/`--read-only` e com a rede padrão (baixa os módulos do proxy do Go). O container não tem rede do banco nem segredos, e o binário roda depois com `--cap-drop ALL` e `--read-only`. Avaliar endurecer (ex.: `--cap-drop ALL`, cache de módulos em `tmpfs`/volume, ou módulos pré-baixados) sem quebrar o build.

---

> **DOC-09** concluído em 2026-11-04 (autorização direta do usuário; ver `feito.md`, "Decisões do usuário de 2026-11-04 (após o Lote 34)").

---

## ARQ-09: valores monetários em centavos (`int64`) — prioridade BAIXA/MÉDIA (ADIADO)

**Status:** **adiado** (2026-11-02) pelo 🤍 MegaBrain, por recomendação do 🟣 SecBrain: refatoração ampla de dinheiro; **não executar antes do primeiro teste em produção** (validação dos Lotes 30/31). Voltou de `fazendo.md` (Lote 32, aberto e não executado). Antes: pendente (sai do Lote 21; executar após o ARQ-08, em lote próprio).
**Camadas:** Database, Backend (Frontend se o formato do JSON mudar)
**Responsável:** 🌸 DataBrain → 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** decisão P3 do 🤍 MegaBrain no ARQ-00 (2026-10-13).

**Descrição:** no ARQ-05, o value object `Dinheiro` permanece `float64` encapsulado. Este card migra o cálculo interno para centavos (`int64`).

**Invariantes do 🟣 SecBrain (2026-11-02), para quando o card for retomado:**
- **contrato JSON inalterado:** valores em **reais decimais** (sem passar a centavos no JSON);
- **schema inalterado:** colunas `DECIMAL(15,2)`, **sem migração**;
- entrada decodificada com **`json.Number`** (sem passar por `float64`);
- arredondamento **half away from zero**, explícito e testado;
- **overflow verificado** nas somas e multiplicações em `int64` (substitui a nota N-9 sobre `+Inf` no `kernel.Dinheiro.Somar`).

**Risco:** arredondamento em `valor_total` (itens, descontos e pagamentos); é preciso um teste de regressão comparando os totais antigos com os novos.

**Critério de aceite:** cálculos monetários sem `float64`; totais existentes reproduzidos sem diferença; contrato JSON e schema inalterados; testes verdes sem perda de cobertura.

---

## Achados dos Lotes 30 e 31

> Origem comum: especificação e revisão do 🟣 SecBrain, entregas dos devs e verificação do 🔴 TestBrain nos Lotes 30 e 31 (2026-11-02; ver `feito.md`). Nenhum deles bloqueou o fechamento dos lotes. Executar em lote próprio, por decisão do 🤍 MegaBrain/usuário. Caminhos: `SH` = `backend/shared`.

---

> **2026-11-06 (Lote 42):** BACKUP-01 foi para o Lote 42 (`fazendo.md`).
>
> **2026-11-05 (Lote 36):** DB-07 e TST-16 foram para o Lote 36 (`fazendo.md`) e foram concluídos (`feito.md`, "Lote 36 de 2026-11-05").
>
> **2026-11-05 (Lote 35):** FE-18 e CHORE-16 foram para o Lote 35 (`fazendo.md`).
>
> **2026-11-04 (Lote 34):** SEC-46, SEC-49 e CHORE-15 foram para o **Lote 34** e estão concluídos (`feito.md`).

---

## SEC-47: fixar por digest as imagens base dos `Dockerfile` — prioridade BAIXA

**Status:** pendente.
**Camadas:** Infra (build)
**Responsável:** 🟡 BackBrain / 🟢 FrontBrain (🟣 SecBrain revisa)
**Origem:** SEC-31 (Lote 30, 2026-11-02).

**Descrição:** o SEC-31 fixou por digest as imagens do `Jenkinsfile` e do `mariadb-docker.sh`, mas `backend/Dockerfile`, `frontend/Dockerfile` e `deploy/Dockerfile.caddy` ainda usam tags mutáveis.

**Ação esperada:** fixar as bases por `tag@sha256:` e incluí-las no `make deps-audit-imagens`.

**Critério de aceite:** todas as bases com digest; `deps-audit-imagens` cobrindo os três arquivos; build verde.

**Acréscimo (2026-11-05, Lote 39):** alinhar `GO_VERSION=1.26.9` (`backend/Dockerfile:15`) com 1.26.10, com teste de paridade. Rodar `govulncheck` com o toolchain 1.26.9, o da produção: vulnerabilidade alcançável na stdlib eleva a prioridade deste card. **Cuidado:** push na `main` dispara deploy.

---

## CHORE-14: remover a limpeza dos cookies sem prefixo (transição do SEC-33) — prioridade BAIXA

**Status:** pendente; executar **7 dias ou mais depois do deploy do SEC-33** em produção.
**Camadas:** Backend, Testes
**Responsável:** 🟡 BackBrain → 🔴 TestBrain
**Origem:** SEC-33 (Lote 31, 2026-11-02).

**Descrição:** no modo seguro, o logout e a transição ainda limpam os cookies antigos (`access_token`/`refresh_token`, sem `__Host-`). Depois que os cookies antigos expirarem, esse código fica sem uso.

**Ação esperada:** remover a limpeza dos nomes antigos no modo seguro (o dev continua com os nomes sem prefixo); ajustar os testes.

**Critério de aceite:** modo seguro só emite/limpa `__Host-…`; testes verdes.

---

## SEC-48: reprocessar os `.webp` de produto já gravados — prioridade BAIXA

**Status:** pendente.
**Camadas:** Backend (shared/tools), Testes
**Responsável:** 🟡 BackBrain → 🔴 TestBrain (🟣 SecBrain revisa)
**Origem:** SEC-34 e CHORE-09 (Lote 31, 2026-11-02).

**Descrição:** o SEC-34 converte o WebP novo (PNG/JPEG), mas os `.webp` gravados antes continuam com EXIF e eventuais bytes anexados.

**Ação esperada:** opção no comando `backend/shared/cmd/limpezaimagens` (com `-dry-run`) para reprocessar os `.webp` existentes pelo normalizador e atualizar `produtos.imagem` na mesma operação, sem deixar órfão nem referência quebrada.

**Critério de aceite:** testes com diretório temporário (convertido, referência atualizada, `-dry-run` sem efeito, falha sem perda do original); docs atualizadas.

---

## BUG-25: usuário apagado no meio da troca da própria senha responde 500 — prioridade BAIXA (decisão)

**Status:** pendente; **decisão** do 🟣 SecBrain/🤍 MegaBrain.
**Camada:** Backend (shared)
**Responsável:** 🟣 SecBrain (decisão) → 🟡 BackBrain → 🔴 TestBrain
**Origem:** BUG-23/BUG-24 (Lote 31, 2026-11-02).

**Descrição:** se o usuário for apagado durante a troca da própria senha, a resposta é 500; no reset admin o mesmo caso é 404.

**Ação esperada:** decidir se a troca da própria senha passa a 404 (ou 401 de sessão inválida, como no refresh do BUG-23); aplicar e testar.

**Critério de aceite:** decisão registrada; teste do caso; contrato documentado.

---

## Achados do Lote 29 (sem cards pendentes)

> Origem comum: revisão do 🟣 SecBrain e entregas do 🟡 BackBrain e do 🔴 TestBrain no Lote 29 (SEC-29, TST-09, TST-08 e SETUP-01, 2026-11-01; ver `feito.md`). Nenhum deles bloqueou o fechamento desses cards. Executar em lote próprio, por decisão do 🤍 MegaBrain/usuário. Caminhos: `SH` = `backend/shared`.
>
> **2026-11-02 (fechamento do Lote 30):** o **TST-14** foi **cancelado** (os drop-ins do `setup-servidor.sh` deixaram de existir; coberto pelo `TestTST15_MariadbDocker_Real`; card em `feito.md`). Este bloco fica sem cards pendentes.
>
> **2026-11-02 (abertura do Lote 30):** SEC-39, SEC-40, SEC-41, SEC-42, TST-12, DB-06 e DEPLOY-04 foram para o **Lote 30** (`fazendo.md`). DOC-07 (substituído pelo DOC-08) e DEPLOY-03 (sem MySQL) foram **cancelados** (`feito.md`). Aqui fica só o TST-14, bloqueado.
>
> **2026-11-01 (decisões do usuário):** TST-13 cancelado e SEC-43 fechado como "não corrigir" (ambos em `feito.md`). DOC-07 entrou aqui (divergência MariaDB 11.4 × MySQL 8.0 em produção). SEC-39 a SEC-42 e TST-12 devem considerar a produção MySQL 8.0 e o SEC-44 (concluído, em `feito.md`).
>
> **2026-11-01 (fechamento do Lote 29):** entraram DB-06, DEPLOY-03, DEPLOY-04 e TST-14 (achados do SEC-44 e do DEPLOY-02), logo após o DOC-07.

---

## GAPs de segurança da auditoria ARQ-00 (SEC-14 a SEC-25)

> Origem comum: 🟣 SecBrain, auditoria ARQ-00, Lote 21 (2026-10-13). Relatório completo em `docs/arq-00-invariantes-seguranca.md` (seção "GAPs"; itens 1 a 11). O item 12 (teste de matriz de rotas) não virou card: é feito pelo 🔴 TestBrain dentro do ARQ-00.
>
> **Todos os cards abaixo estão fora do escopo do Lote 21: não corrigir dentro dos cards ARQ. Executar após o ARQ-07 ou em lote próprio, por decisão do 🤍 MegaBrain/usuário.** Durante a refatoração, a regra é não piorar nenhum destes pontos.
>
> Caminhos: `API` = `backend/rotaperfumes-api`, `SH` = `backend/shared`. As linhas citadas são do código antes da refatoração DDD; depois do ARQ-07 o dev deve localizar o ponto equivalente na nova estrutura.
>
> **2026-10-31 (fechamento do Lote 28):** SEC-19 e SEC-23 foram **concluídos no Lote 28** (cards em `feito.md`). Todos os GAPs deste bloco estão resolvidos (Lotes 24, 27 e 28).
>
> **2026-10-31 (abertura do Lote 28):** SEC-19 e SEC-23 foram para o **Lote 28** (cards em `fazendo.md`). Com isso, nenhum card deste bloco fica pendente aqui: SEC-14 a SEC-17 e SEC-24/SEC-25 foram concluídos no Lote 27, SEC-18 e SEC-20 a SEC-22 no Lote 24.
>
> **2026-10-31 (fechamento do Lote 27):** SEC-14, SEC-15, SEC-16, SEC-17, SEC-24 e SEC-25 foram **concluídos no Lote 27** (cards em `feito.md`).
>
> **2026-10-31:** SEC-14, SEC-15, SEC-16, SEC-17, SEC-24 e SEC-25 foram para o **Lote 27** (cards em `fazendo.md`). Aqui ficam SEC-19 e SEC-23 (backend + frontend + config), para o lote seguinte. SEC-18 e SEC-20 a SEC-22 já foram concluídos no Lote 24.
>
> **Onde fica hoje (conferido no ARQ-08, 2026-10-16; mapa completo em `docs/arquitetura-ddd.md` §10):**
> - SEC-14: **concluído no Lote 27 (card em `feito.md`)**; `http.Server` em `API/internal/bootstrap/server.go`; decode de JSON em `API/internal/httpx/body.go` (`DecodificarJSON`); itens do pedido em `SH/vendas/pedido.go` (`NovoPedido`).
> - SEC-15: **concluído no Lote 27 (card em `feito.md`)**; `maskDSN` continua em `SH/db/db.go`.
> - SEC-16: **concluído no Lote 27 (card em `feito.md`)**; `SH/kernel/pagina.go` (`Pagina`, `Offset()`).
> - SEC-17: **concluído no Lote 27 (card em `feito.md`)**; `API/internal/httpx/cors.go`.
> - SEC-18: **concluído no Lote 24 (card em `feito.md`)**; `SH/identidade/identidadecrypto/jwt.go` e `SH/config/config.go`.
> - SEC-19: **concluído no Lote 28 (card em `feito.md`)**; `SH/adapters/turnstile/turnstile.go`.
> - SEC-20: **concluído no Lote 24 (card em `feito.md`)**; `API/internal/httpx/clientip.go` (`IPDoCliente`).
> - SEC-21: **concluído no Lote 24 (card em `feito.md`)**; `API/internal/httpx/cookies.go` (`DefinirCookiesSessao`, `LimparCookiesSessao`).
> - SEC-22: **concluído no Lote 24 (card em `feito.md`)**; `SH/identidade/identidadeapp/login.go` (`Login`) e `sessao.go` (refresh).
> - SEC-23: **concluído no Lote 28 (card em `feito.md`)**; `deve_trocar_senha` em `SH/identidade/usuario.go` e `SH/identidade/identidademysql/usuarios.go`; autenticação por request em `API/internal/httpx/auth.go`.
> - SEC-24: **concluído no Lote 27 (card em `feito.md`)**; enums de ordenação em `SH/catalogo/ordenacao.go`, `SH/comercial/ordenacao.go`, `SH/vendas/filtros.go`, `SH/identidade/usuario.go` e `SH/identidade/historico.go`, traduzidos por `switch` nos adapters `*mysql`; testes em `SH/tests/repositories/sort_test.go`.
> - SEC-25: **concluído no Lote 27 (card em `feito.md`)**; buscas `LIKE ?` em `SH/catalogo/catalogomysql/produtos.go`, `SH/comercial/comercialmysql/{clientes,oportunidades,visitas}.go` e `SH/vendas/vendasmysql/pedidos.go`.

---

## TST-04: itens do roteiro do Lote 12 que testam o próprio `make` (1.1, 1.2 e 1.7) — prioridade MÉDIA

**Status:** pendente, **bloqueado** (depende do usuário); avançou parcialmente no Lote 20 (2026-10-11). **Corrigido em 2026-11-06:** o `.env` local aponta para o banco de **dev** do CasaOS (`192.168.168.106:3307`), não para a produção como dizia a reconfirmação de 2026-11-02. Continua bloqueado até o usuário rodar os itens ou liberar `make db-*` para o 🔴 TestBrain ("Falta", abaixo).
**Camada:** Testes
**Responsável:** 🔴 TestBrain com o usuário
**Origem:** 🔴 TestBrain, roteiro manual do Lote 12 (2026-10-07).

**Já feito (Lote 20, 2026-10-11):** execução equivalente sem `make` no banco descartável `rotaperfumes_l12` (criado e depois apagado; o banco `rotaperfumes` ficou intacto, com o mesmo MD5 de schema e `estoque` com 8401 linhas). Aprovados: 3.1, 3.2, 3.3, 3.5, 3.6, 3.7, 4.1, 4.2 e 4.3. O 3.4 não se aplica (só com `db-rebuild`). Resultado em `docs/roteiro-teste-manual-lote12.md`, seção "Execução complementar do TST-04".

**Falta:** 1.1, 1.2 e 1.7 (SEC-11: `db-check-env` e precedência de variáveis). Eles testam o próprio `make`, então não há execução equivalente: continuam bloqueados até o usuário rodá-los com o `make` (ou liberar a permissão de `make db-*` para o 🔴 TestBrain).

**Regras:**
- usar **sempre** `DB_NAME=rotaperfumes_l12`, conferindo antes com `make -n` que o banco alvo é o descartável;
- **nunca** rodar no banco `rotaperfumes`;
- apagar o `rotaperfumes_l12` ao final.

Registrar o resultado na seção "Execução" do roteiro.

---

## DEP-03: vulnerabilidades altas de dev restantes (`braces` via `eslint-config-next`) — prioridade BAIXA

**Status:** pendente (acompanhar o upstream). **2026-11-02 (fechamento do Lote 31):** upstream conferido pelo 🟢 FrontBrain; as altas do `braces` saíram (**0 altas**), restam **16 moderadas** do `minimatch` via `eslint` (só dev). O card fica como acompanhamento. Antes: a conferência do upstream entrou no Lote 31.
**Camada:** Frontend
**Responsável:** 🟢 FrontBrain
**Origem:** residual do DEP-01 (Lote 20, 2026-10-11; ver `feito.md`).

**Descrição:** após a migração para Tailwind 4, o `npm audit` completo caiu de 9 para 5 vulnerabilidades (5 altas); `npm audit --omit=dev` = 0. As 5 vêm da cadeia `braces` → `micromatch` → `fast-glob@3.3.1` → `@next/eslint-plugin-next` → `eslint-config-next@16.4.0`, que só é usada pela ferramenta de lint (dev). O `braces` não tem versão corrigida, e o `npm audit fix --force` rebaixaria o `eslint-config-next` para 14.2.35 (rejeitado).

**Ação esperada:** acompanhar o upstream (`eslint-config-next`/`@next/eslint-plugin-next`, `fast-glob`, `micromatch`, `braces`). Quando houver correção, atualizar ou usar `overrides` de `fast-glob`/`micromatch` no `package.json` e validar com `tsc`, `eslint`, vitest, `next build` e `npm audit`. Não usar `npm audit fix --force`.

---

> **2026-11-02:** CHORE-04 e CHORE-05 foram para o **Lote 31** (`fazendo.md`).
>
> **2026-10-29:** CHORE-03 (id de path → 400) e CHORE-06 (duplicidade → 409) receberam a aprovação do usuário para a mudança de contrato HTTP e foram para o **Lote 25** (cards em `fazendo.md`). Aqui ficam CHORE-04 e CHORE-05.
