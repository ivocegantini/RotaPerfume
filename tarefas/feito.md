# Concluídas ✅

> Histórico resumido: uma linha por card, lotes do mais novo ao mais antigo. O detalhe completo (entregas, testes, decisões) está no git, até o commit `4e862a0`.
> Regra: todo arquivo de `tarefas/` é resumido; o `fazendo.md` não guarda histórico (só o card em execução e o passo atual).

## Histórico do `fazendo.md` (movido em 2026-11-14)
- 2026-09-30 a 2026-10-29: Lotes 13 e 22 a 25 abertos e fechados; decisões de produto do usuário nos riscos R1 a R6 (Lote 25).
- 2026-10-31: Lotes 26 (IMG-01), 27 e 28; DB-05 e UX-03 avulsos.
- 2026-11-01: Lote 29 (TST-10 e TST-13 cancelados, SEC-43 aceito); CHORE-12 (`apis/` → `backend/`).
- 2026-11-02: Lotes 30 e 31 fechados; Lote 32 (ARQ-09) adiado; merge de todas as branches na `main`.
- 2026-11-04: Lote 33 (DEPLOY-07; 1º build falhou sem a CA do banco), TST-18 e Lote 34.
- 2026-11-05: Lotes 35 e 36 (banco descartável `rp-descartavel-l36`; incidente do `MAKEFLAGS=-k` sem escrita).
- 2026-11-13: Lote 37. 2026-11-14: Lote 38 (ARQ-09 aplicado na `main`); nenhum card em execução.

## Lote 38 de 2026-11-14: decisões do usuário (push feito pelo usuário)
- DEPLOY-08: §11 do `deploy-servidor.md` executada pelo usuário no servidor; libera o 1º deploy.
- SETUP-02 e sugestões de `CLAUDE.md` (Lote 37 e SETUP-02): confirmadas/aplicadas pelo usuário.
- BUG-25: conta apagada durante a troca de senha responde 400 (`ErrContaRemovidaNaTroca`).
- CHORE-14: removida a expiração dos cookies sem prefixo `__Host-`.
- SEC-52: senha inicial do admin sempre aleatória; build do `resetpassword` com `--cap-drop ALL`.
- TST-17: opções de isolamento do `seed-producao.sh` e testes reais TST15/TST17 verdes (execução assistida com `sudo`).
- ARQ-09: dinheiro em centavos `int64` (`SH/kernel/dinheiro.go`); mais de 2 casas ou acima de `DECIMAL(15,2)` dá 400.
- CA de dev do CasaOS trocada (`d150ee67…`); a antiga segue proibida.
- DEP-04: `make deps-audit` antes do deploy; Go da API para 1.26.11.
- TST-04, TST-24 e TST-25: integração real verde no banco próprio `rotaperfumes_teste`.

## Lote 37 de 2026-11-13: 5 cards concluídos
- LOG-04: logs do frontend com data/hora (`carimbo()` em `vlog.ts`).
- BUG-29: `MAKEFLAGS`/`MAKEOVERRIDES` na linha de comando abortam o make.
- BUG-28: `db-seed` com guarda recusada não roda mais o `db-up`.
- SEC-56: formas numéricas de IPv4 fora do padrão contam como produção.
- SEC-57: porta com espaço ou não numérica é recusada.

## Cards avulsos de 2026-11-13
- CHORE-19: limpeza do `afazer.md` — 2026-11-13
- SETUP-02: setup-linux.sh instala o Docker via apt (docker.io) — 2026-11-13

## Lote 36 de 2026-11-05: 3 cards concluídos, 1 parcial
- TST-22: testes de integração com o helper do SEC-50 contra um MariaDB descartável — 2026-11-05
- DB-07: `ONLY_FULL_GROUP_BY` no `SQL_MODE` do `mariadb-docker.sh` — 2026-11-05
- TST-16: isolamento do teste real do `mariadb-docker.sh` e concorrência real do BUG-22 — 2026-11-05
- TST-17 (registro do Lote 36, parcial): testes reais com Docker e `shellcheck` — 2026-11-05

## Lote 35 de 2026-11-05: 8 cards concluídos
- BUG-27: `db-reset`/`db-rebuild` rodavam o `db-seed` com o DROP recusado (`make -k`) e tinham corrida com `-j` — 2026-11-05
- SEC-54: guarda de produção normalizada e allowlist na integração — 2026-11-05
- TST-19: catraca para conexão com a config do ambiente sem o helper do SEC-50 — 2026-11-05
- SEC-55: `SEM_SEGREDOS` estrito para build, lint e npm — 2026-11-05
- TST-20: ramo de erro de `integracao.Abrir` — 2026-11-05
- TST-21: `seed-producao.sh` com terminal real (confirmação e pepper) — 2026-11-05
- FE-18: bloqueio de 1500 ms nos botões de ativo de clientes e usuários — 2026-11-05
- CHORE-16: `seedusers` renomeado; pacotes `*mysql` mantidos — 2026-11-05
- DOC-09: `CLAUDE.md` com o banco de dev, o `seed-producao.sh` e a guarda de produção (o usuário editou) — 2026-11-05

## Decisões do usuário de 2026-11-04 (após o Lote 34)
- OPS-02: SMTP de produção para o reset de senha dos vendedores — 2026-11-04
- CHORE-18: hostname do servidor na guarda `db-guard-producao` — 2026-11-04

## Lote 34 de 2026-11-04: 7 cards concluídos
- SEC-53: segredos do `.env` fora dos processos de test/build/lint/npm do Makefile — 2026-11-04
- SEC-46: alvos destrutivos recusados na produção e `CONFIRMAR=<DB_NAME>` para DROP fora do loopback — 2026-11-04
- SEC-50: guarda de produção nos testes Go de integração (`SH/testkit/integracao`) — 2026-11-04
- SEC-49: `resetpassword.DSN()` com `ValidarDBTLSObrigatorio` — 2026-11-04
- CHORE-15: código morto `argon2idPepper.verificar` removido — 2026-11-04
- CHORE-17: `mariadb-docker.sh` só gera `casaos.senha` com `--dev-ip` — 2026-11-04
- BUG-26: `seed-producao.sh` sem terminal falha na etapa 1 com mensagem clara — 2026-11-04
- TST-18: estágio de testes do frontend no Jenkins — 2026-11-04

## Lote 33 de 2026-11-04: 1 card concluído (DEPLOY-07) e 1 substituído (DEPLOY-05)
- DEPLOY-07: instalação limpa do MariaDB de produção, sem MySQL: §11 reescrita, CAs de dev e produção separadas, seed em produção — 2026-11-04
- DEPLOY-05: executar o `deploy/mariadb-docker.sh` no servidor e aposentar o MySQL — 🔁 SUBSTITUÍDO em 2026-11-04

## Lote 31 de 2026-11-02 (backend/frontend, BAIXA): 24 cards concluídos
- TST-06: asserções de tipo sem checagem nos testes de handler — 2026-11-02
- BUG-18: Analytics, IN sem limite nas metas e detecção de "tabela não existe" por texto — 2026-11-02
- TST-07: teste de AST de concatenação SQL em todos os adapters — 2026-11-02
- DOC-06: comentários de escopo das rotas em `rotas.go` — 2026-11-02
- FE-15: `FunilVendas`, aviso de desligado sem `forcarZeros` e seleção da janela após erro — 2026-11-02
- UX-02: orientar o ajuste manual de estoque depois do snapshot do faturamento — 2026-11-02
- CHORE-07: mensagens de pedido faturado/entregue depois do BUG-16 — 2026-11-02
- BUG-22: toggle de ativo de clientes e usuários atômico (padrão do BUG-17) — 2026-11-02
- SEC-32: hash vazio ou malformado no banco permitia enumerar contas pelo tempo — 2026-11-02
- SEC-33: prefixo `__Host-` nos cookies de sessão — 2026-11-02
- BUG-23: refresh de usuário apagado respondia 500 — 2026-11-02
- CHORE-08: observações do 🔴 TestBrain no Lote 25 (log, vlog, NaN e 1062 de carteiras) — 2026-11-02
- FE-16: `next build` do frontend — 2026-11-02
- CHORE-09: limpeza de imagens órfãs de produto — 2026-11-02
- SEC-34: WebP de produto não era reencodado — 2026-11-02
- SEC-36: origens de dev na allowlist de CORS em produção — 2026-11-02
- BUG-24: troca de senha concorrente com reset admin — 2026-11-02
- CHORE-10: `ErrImagemMuitoGrande` com `KindMuitoGrande` e `maxLength={100}` nas buscas — 2026-11-02
- FE-17: depois de trocar a senha, ir direto para `/login` — 2026-11-02
- CHORE-11: remover `apiResetPassword` morta do frontend — 2026-11-02
- SEC-38: posse do pagamento escondia erro de banco como 404 — 2026-11-02
- LOG-03: recusa do captcha logada com o motivo — 2026-11-02
- CHORE-04: gofmt/LF em 3 testes SEC12 de integração — 2026-11-02
- CHORE-05: logs de auditoria de escrita com o id do usuário — 2026-11-02

## Lote 30 de 2026-11-02 (somente MariaDB + infra/TLS): 15 cards concluídos, 1 cancelado, 1 devolvido ao backlog
- CHORE-13: MySQL fora do `Makefile`, `.env.example` e `CLAUDE.md` — 2026-11-02
- DEPLOY-06: deploy só com MariaDB — 2026-11-02
- DEPLOY-04: `setup-servidor.sh` com Jenkins em contêiner e numeração dos passos — 2026-11-02
- DOC-08: documentação só com MariaDB (substitui o DOC-07) — 2026-11-02
- TST-15: testes sem MySQL e teste do `deploy/mariadb-docker.sh` — 2026-11-02
- SEC-45: revisão de segurança do `deploy/mariadb-docker.sh` — 2026-11-02
- SEC-39: `REQUIRE SSL` nas contas do banco — 2026-11-02
- SEC-31: imagens fixadas por digest — 2026-11-02
- SEC-30: autenticação ed25519 nas contas do banco — 2026-11-02
- SEC-40: confiança na CA do banco no Jenkins — 2026-11-02
- SEC-41: `deploy/dump-local.ps1` com TLS — 2026-11-02
- SEC-42: TLS obrigatório com `DB_HOST` remoto — 2026-11-02
- TST-12: testes de TLS que faltaram no SEC-29 — 2026-11-02
- DB-04 + DB-06: `ONLY_FULL_GROUP_BY` no `sql_mode` — 2026-11-02
- TST-14: rodar `TestDEPLOY02_ValidateConfig_Docker` com Docker — ❌ CANCELADO em 2026-11-02

## Lote 30 de 2026-11-02: triagem (2 cancelados)
- DOC-07: docs dizem MariaDB 11.4, mas a produção real é MySQL 8.0 — ❌ CANCELADO em 2026-11-02
- DEPLOY-03: dump do MariaDB 11.x não importa no MySQL 8 (linha do sandbox mode) — ❌ CANCELADO em 2026-11-02

## Lote 29 de 2026-11-01: 7 cards concluídos, 2 cancelados, 1 fechado sem correção
- SEC-44: TLS do SEC-29 no ambiente real (produção MySQL 8.0, Jenkins em contêiner) — 2026-11-01
- SEC-37: secret de teste do Turnstile recusada em produção — 2026-11-01
- DEPLOY-02: hardening do banco (`local_infile`/`sql_mode`/collation) também com bind aberto — 2026-11-01
- TST-11: teste instável `TestIMG01_Upload_FilenameHostilNaoVaza/CRLF_por_RFC_2231` — 2026-11-01
- SEC-43: credencial fraca no `.env` local (senha contida no nome do usuário) — FECHADO SEM CORREÇÃO em 2026-11-01
- TST-13: validar `scripts/setup-windows.ps1` no Windows e o repositório MariaDB 11.4 no Linux — ❌ CANCELADO em 2026-11-01
- SEC-29: TLS na conexão Go ↔ MariaDB — 2026-11-01
- TST-09: 3 testes de integração do Lote 25 e CHECK `chk_pagamentos_taxa_pct` — 2026-11-01
- TST-08: `make test` falha com `go: no such tool "covdata"` — 2026-11-01
- SETUP-01: scripts de instalação do ambiente de desenvolvimento (Linux e Windows) — 2026-11-01
- TST-10: `make test-frontend` e `make frontend-deps` não funcionam no disco exFAT com o Node 18 do sistema — ❌ CANCELADO em 2026-11-01
- CHORE-12 de 2026-11-01: pasta `apis/` renomeada para `backend/`

## Lote 28 de 2026-10-31: 3 cards concluídos
- SEC-19: Turnstile passa a conferir `hostname` e `action` — 2026-10-31
- SEC-23: `deve_trocar_senha` imposto pelo backend — 2026-10-31
- SEC-35: `CriarPagamento` e `ExcluirPagamento` sob lock — 2026-10-31

## Lote 27 de 2026-10-31: 9 cards concluídos
- SEC-14: limite de tamanho do corpo, ReadHeaderTimeout e limite de itens do pedido — 2026-10-31
- SEC-15: `maskDSN` não vaza mais parte da senha com `@` — 2026-10-31
- SEC-16: teto de `page` na paginação — 2026-10-31
- SEC-17: CORS sem curinga com credentials — 2026-10-31
- SEC-24: testes de injeção via `order_by` em todos os enums de ordenação — 2026-10-31
- SEC-25: buscas LIKE com escape de `%`/`_` e limite de `q` — 2026-10-31
- BUG-14: `VincularCliente` numa transação — 2026-10-31
- BUG-15: troca e reset de senha numa transação — 2026-10-31
- SEC-26: posse e regras de exclusão do pedido checadas sob lock — 2026-10-31

## Lote 26 de 2026-10-31: imagem do produto (1 card)
- IMG-01: imagem do produto — 2026-10-31
- DB-05: `make db-seed-mes-atual` (movimentações de exemplo no mês corrente) — 2026-10-31

## Lote 25 de 2026-10-30: 8 cards concluídos
- BUG-16: baixa de estoque dupla no ciclo Faturado → outro → Faturado — 2026-10-30
- SEC-27: pagamento quitado editável só por admin e teto de `taxa_pct` — 2026-10-30
- BUG-17: toggle de ativo do produto atômico e bloqueio do botão — 2026-10-30
- BUG-19: `arredondar2` com `math.Round` — 2026-10-30
- CHORE-03: parse estrito de id de path na API inteira — 2026-10-30
- CHORE-06: duplicidade → 409 Conflict, sem o valor no log — 2026-10-30
- BUG-20: pedido criado já como Faturado baixa o estoque — 2026-10-30
- BUG-21: ruptura do upsert de faturamento na ordem certa — 2026-10-30

## Lote 24 de 2026-10-29: 4 cards concluídos (SEC-18, SEC-20, SEC-21 e SEC-22)
- SEC-18: JWT com issuer, `exp` e HS256 obrigatórios e `JWT_SECRET` com tamanho mínimo — 2026-10-29
- SEC-20: IP do cliente pelo item mais à direita do `X-Forwarded-For` — 2026-10-29
- SEC-21: `Secure` do cookie por configuração e limpeza com os mesmos atributos — 2026-10-29
- SEC-22: login e refresh não revelam usuário inativo — 2026-10-29

## Lote 23 de 2026-10-29: 2 cards concluídos (DB-03 e SEC-28, este resolvido pelo DB-03)
- DB-03: trocar a conexão do projeto de MySQL para MariaDB (infra + docs) — 2026-10-29
- SEC-28: remover `--local-infile=1` do `MYSQL_OPTS` do `Makefile` da raiz — 2026-10-29

## Lote 22 de 2026-10-29: 1 card concluído (de 1)
- DASH-01: Funil de vendas no Dashboard — 2026-10-29

## Lote 13 (2026-09-30) e MAIL-01: fechados em 2026-10-16 — 1 card concluído, 1 cancelado
- DEPLOY-01: implantar o sistema no servidor `ivo-inspiron-15-3530` via Docker + Jenkins — 2026-10-16
- MAIL-01: programa de diagnóstico de envio de e-mail (`cmd/testeemail`) — ❌ CANCELADO em 2026-10-16

## Lote 21 de 2026-10-13: 9 cards concluídos (de 9)
- ARQ-08: documentação e fechamento do Lote 21 — 2026-10-16
- ARQ-07: Notificação/SMTP, importadores, CLIs e remoção do legado — 2026-10-16
- ARQ-06: contexto Analytics (Dashboard como read model) — 2026-10-15
- ARQ-05: contexto Vendas (Pedido + itens, Pagamento) — 2026-10-15
- ARQ-04: contexto Comercial e escopo por carteira — 2026-10-15
- ARQ-03: contexto Identidade e Acesso — 2026-10-14
- ARQ-02: contexto Catálogo (Produto, Estoque), piloto — 2026-10-14
- ARQ-01: fundação / shared kernel — 2026-10-13
- ARQ-00: blueprint arquitetural DDD + Clean Code + Design Patterns — 2026-10-13

## Lote 20 de 2026-10-11: 3 cards concluídos (de 3)
- DEP-02: atualizar módulos Go desatualizados — 2026-10-11
- DEP-01: migrar Tailwind 3→4 — 2026-10-11
- TST-05: teste de runtime do vlog com `VERBOSE` ligado nos handlers de login/reset — 2026-10-11

## Lote 19 de 2026-10-11: 1 card concluído (de 1)
- DEP-00: política e auditoria de dependências (item 13 do `FalhasEncontradas.md`) — 2026-10-11

## Lote 18 de 2026-10-09: 1 card concluído (de 1)
- UX-01: menu hambúrguer e navegação mobile em todas as páginas — 2026-10-09

## Lote 17 de 2026-10-07: 1 card concluído (de 1)
- LOG-02: log verbose de cada linha de código (API, importers/tools e frontend) — 2026-10-07

## Lote 16 de 2026-10-07: 3 cards concluídos (de 3)
- BUG-13: `make help` mostrava `Makefile` no lugar do nome do alvo — 2026-10-07
- LOG-01: log de alerta de segurança dizia "enviado" quando o envio era pulado — 2026-10-07
- DOC-05: roteiro do Lote 12 dependia da ordem das chaves do JSON e tinha aviso desatualizado no 1.5 — 2026-10-07
- INFO-01: linhas não alcançáveis aceitas como fora da meta de cobertura — 2026-10-07

## Lote 15 de 2026-10-07: 1 card concluído (de 1)
- OPS-01: reset em massa das senhas dos usuários ativos (Argon2id + e-mail) — 2026-10-07

## Lote 14 de 2026-10-07: 1 card concluído (de 1)
- SEC-13: hash de senha bcrypt → Argon2id com pepper (`PASSWORD_PEPPER`) — 2026-10-07

## Lote 12 de 2026-09-27/30: 4 cards concluídos (de 4)
- SEC-11: credenciais do banco no Makefile e no `config.Load()` — 2026-09-27
- DB-01: tabela `estoque` fora do `make db-up`/`db-reset` — 2026-09-27
- SEC-12: reenvio de refresh rotacionado derrubava as sessões da vítima repetidamente — 2026-09-27
- CHORE-02: `refresh_tokens` sem limpeza automática — 2026-09-27

## Lote 11 de 2026-09-26: 4 cards concluídos (de 4)
- FE-14: formatador de data único — 2026-09-26
- SEC-10: endurecimentos da revisão do Lote 7 — 2026-09-26
- SEC-09: e-mail de alerta no reuso de refresh token rotacionado — 2026-09-26
- DOC-04: manual completo da base de dados — 2026-09-26

## Lote 10 de 2026-09-26: 1 card concluído (de 1)
- BUG-12: `scanSenhaHistorico` quebrava com colunas NULL — 2026-09-26

## Lote 9 de 2026-09-26: 3 cards concluídos (de 3)
- CHORE-01: `frontend/tsconfig.tsbuildinfo` rastreado pelo git — 2026-09-26
- BUG-11: `UsuarioRepository.Create` devolvia erro com o INSERT já gravado se só a releitura falhasse — 2026-09-26
- FE-13: colunas de `admin/senha-historico` pareciam ordenáveis, mas não reordenavam — 2026-09-26

## Lote 8 de 2026-09-26: 6 cards concluídos (de 6)
- BUG-10: `seedusers.FindProjectRoot` não achava a raiz do projeto ao rodar de `apis/shared` — 2026-09-26
- BUG-09: creates devolviam o objeto em memória, com timestamps zerados — 2026-09-26
- SEC-08: access token seguia válido depois de inativação, troca/reset de senha e revogação em massa — 2026-09-26
- FE-11: timers não eram limpos no unmount — 2026-09-26
- FE-12: `sortValue` das colunas nunca era usado pelo `Table` — 2026-09-26
- FE-10: estados inconsistentes nas listagens quando a carga falhava — 2026-09-26

## Lote 7 de 2026-09-26: 3 cards concluídos (de 3)
- TST-01: testes do frontend em `frontend/tests/` e cobertura ≥80% — 2026-09-26
- TST-02: testes da rotaperfumes-api em `apis/rotaperfumes-api/tests/` e cobertura ≥80% — 2026-09-26
- TST-03: shared com importadores extraídos, testes em `apis/shared/tests/` e cobertura de ~80% — 2026-09-26

## Lote 6 de 2026-09-26: 7 cards concluídos (de 7)
- SEC-06: sessão de usuário inativado continua válida até o access token expirar — 2026-09-26
- SEC-07: revogar a família de refresh tokens em caso de reuso fora da janela — 2026-09-26
- FE-07: assimetria de logout entre falha de rede no refresh e falha de rede no retry — 2026-09-26
- BUG-08: `POST /api/clientes` devolve `created_at` e `updated_at` zerados — 2026-09-26
- FE-09: listagem de clientes mostra "Nenhum cliente cadastrado." quando há erro de rede — 2026-09-26
- TEST-01: nome de subteste e comentário de CNPJ desatualizados — 2026-09-26
- DOC-03: outros documentos e o Postman ainda usam `henrique.rodrigues` (inativo) — 2026-09-26
- BUG-07: modal "Editar Vendedor" sobrescreve a data de admissão e a meta mensal — prioridade ALTA — 2026-09-26

## Lote 5 de 2026-09-25: 7 cards concluídos (de 7)
- SEC-04: corrida legítima no refresh conta no rate limit por IP — 2026-09-25
- FE-06: mensagem enganosa quando o retry após o refresh falha por rede — 2026-09-25
- DOC-01: roteiro de vendedor desligado usa um usuário que não faz mais login — 2026-09-25
- DOC-02: manual da base de dados com o índice `uq_clientes_cnpj` — 2026-09-25
- NEG-02: aceitar o CNPJ alfanumérico da Receita — 2026-09-25
- DB-02: comentários imprecisos em SQL e no Makefile — 2026-09-25
- FE-08: mensagem amigável para erro de rede — 2026-09-25
- NEG-04: troca errada de CNPJ legado não pode ser desfeita pela tela/API (DECISÃO DE NEGÓCIO) — 2026-09-25
- NEG-01: validação e duplicidade de CNPJ (DECISÃO DE NEGÓCIO) — 2026-09-25
- NEG-03: CNPJ legado duplicado com DV inválido responde "cnpj inválido" em vez de "já cadastrado" (DECISÃO DE NEGÓCIO) — 2026-09-25
- DB-01: log de vínculos da migração 19 (`clientes_merge_backup_20260925_vinculos`) está vazio — 2026-09-25
- FE-04: resposta obsoleta sobrescreve a mais nova nas listagens — 2026-09-25
- FE-05: listagens chamam `fetch` direto, sem refresh automático no 401 — 2026-09-25
- SEC-02: corrida no refresh token gera dois pares de tokens — 2026-09-25
- SEC-05: refresh sem cookie responde `400` em vez de `401` (DECISÃO) — 2026-09-25
- BUG-06: `PATCH /inativar` com body inválido inverte o estado — 2026-09-25
- SEC-03: `GET /api/vendedores` sem escopo — 2026-09-25
- BUG-02: botão "Alterar Senha" nunca habilita em `/trocar-senha` (inclui BUG-03) — 2026-09-25
- UI-04: accordion de itens do pedido abre no fim da tabela — 2026-09-25
- Teste manual do Dashboard no navegador (normal com vendedor, normal sem vendedor, admin) — 2026-09-25
- UI-01: título do gráfico do Dashboard (admin) fixo em "Vendas nos Ultimos 30 Dias" — 2026-09-25
- UI-02: layout do Dashboard para normal sem vendedor / com vendedor desligado — 2026-09-25
- Segurança: vendedor desligado ainda podia criar/editar pedidos, pagamentos, oportunidades e visitas (DECIDIDO: bloquear) — 2026-09-25
- UX: `id_vendedor` do localStorage desatualizado após o admin mudar o vínculo do usuário — 2026-09-25
- BUG-05: inativar vendedor não inativava o usuário vinculado — 2026-09-25
- Teste manual/e2e do `PedidoModal` no navegador (usuário normal e admin) — 2026-09-25
- UI-03: coluna própria para o ID principal nas tabelas — 2026-09-25

## Lote 3 de 2026-09-24: 6 cards concluídos (de 6)
- BUG-04: salvar registro sem alterações retorna 404 "não encontrado" — 2026-09-24
- RISCO-01: datas de início de horário de verão com `TZ=America/Sao_Paulo` (Linux/Docker) — 2026-09-24
- FE-01: corrida em `refreshSessionUser` mantinha o bloqueio de vendedor desligado após a reativação — 2026-09-24
- FE-02: Dashboard dependia da API para zerar os dados de vendedor desligado / sem vendedor — 2026-09-24
- SEC-01: IDOR em Create/Update/Toggle de clientes — 2026-09-24
- FE-03: 34 warnings `react-hooks/set-state-in-effect` — 2026-09-24

## Lote 2 de 2026-09-24: 2 cards concluídos (de 6)
- BUG-01: `data_pedido` gravada com um dia a menos (fuso UTC vs `loc=Local`) — 2026-09-24
- Tooling frontend: `npm run lint` quebrado e ausência de runner de testes — 2026-09-24

## Lote de 2026-09-24: 8 cards concluídos (de 10)
- Elevar cobertura do pacote `handlers` para o mínimo de 80% — 2026-09-24
- Padronizar status de `POST /api/pagamentos` com pedido inexistente (400 admin vs 404 normal) — 2026-09-24
- Remover `meta_mensal` da resposta de `GET /api/vendedores` para usuário normal — 2026-09-24
- Aplicar `gofmt` em testes de Oportunidades, Visitas e Estoque — 2026-09-24
- Bug: total de vendas truncado para `int` em `enrichWithVendas` / `enrichMetasWithVendas` (perda de centavos) — 2026-09-24
- Padronizar `top_vendedores`/`metas_vendedores` vazios como `[]` (hoje `null`) — 2026-09-24
- Dashboard de vendedor desligado (`data_desligamento`): bloquear acesso — 2026-09-24
- `GET /api/dashboard/clientes`: escopo da carteira para usuário normal — 2026-09-24

## Antes dos lotes numerados (2026-09-07 a 2026-09-23)
- Validar com o negócio: pedido com cliente transferido (visibilidade de pedidos antigos + escopo do Dashboard) — 2026-09-23
- Aplicar scope check (owner check + client scoping) em Create/Update de Pedidos — 2026-09-23
- Rotas de Vendedores (`/api/vendedores*`) restritas a admin no backend — 2026-09-23
- Bugfix: sem mensagem clara ao trocar senha pela mesma senha atual — 2026-09-23
- Bugfix: nome do usuário não aparecia na Auditoria de Senha — 2026-09-23
- Bugfix: tipo_reset errado na Auditoria de Senha + nota obsoleta — 2026-09-23
- Autoatendimento de troca de senha (menu) + política de senha forte + bloqueio de reuso — 2026-09-23
- Seleção de pedido por combobox com busca na criação de pagamento — 2026-09-22
- Botão de excluir (delete) nas telas de Pedidos, Pagamentos, Oportunidades e Visitas — 2026-09-22
- Bugfix menu "Vendedores" visível para vendedor + restrição de Estoque a admin — 2026-09-22
- Menu do vendedor: Visitas/Oportunidades + escopo por carteira; Produtos movido para Administração — 2026-09-22
- Bugfix: "Failed to fetch" (unhandledRejection) no logout — 2026-09-22
- Bugfix: warning Cloudflare Turnstile "Cannot find Widget" — 2026-09-22
- Controle de Estoque — 2026-09-22
- CAPTCHA (Cloudflare Turnstile) em Login e Troca de Senha — 2026-09-22
- Auditoria de Segurança (Site + API) e Correção de Falhas — 2026-09-17
- Ranking de vendedores deve ordenar por meta e desempatar por atingimento — 2026-09-17
- Bug: barras do gráfico "Vendas nos Últimos 30 Dias" invisíveis (CSS) — 2026-09-16
- Bug: erro 500 (only_full_group_by) em GET /api/dashboard/vendas — 2026-09-16
- Bug: gráfico de vendas mostra "NaN/09" no eixo de datas — 2026-09-16
- Bug: gráfico "Vendas nos Últimos 30 Dias" não mostra nada — 2026-09-16
- Bug: Total de Vendas exibe NaN e Ranking de Vendedores sempre 0 — 2026-09-16
- Filtro "Semana" do dashboard retorna os mesmos dados que "Hoje" — 2026-09-16
- Dashboard 100% orientado a dados reais da base — 2026-09-16
- Cadastro de Visitas (CRM) — 2026-09-15
- Importador de Oportunidades (CRM) — complemento da feature — 2026-09-15
- Cadastro de Oportunidades (CRM) — 2026-09-15
- Validação de build do frontend (resolve pendência recorrente) — 2026-09-15
- Padronizar botão de status ativo/inativo da tela de vendedores com a de usuários — 2026-09-15
- Filtros (região/UF/status) e ação ativar/inativar na tela de vendedores — 2026-09-15
- Listar vendedores inativos com marcador [X] (corrige bug de vínculo "sumido") — 2026-09-15
- Promover colunas *_id_origem a PK autoincremento — 2026-09-15
- Vincular/Desvincular Cliente a Vendedor (Carteiras) — 2026-09-14
- Cadastro de Vendedor com Lista de Clientes (Carteiras) — 2026-09-14
- [BUG] Dashboard não carrega para vendedores — 2026-09-14
- Ordenação via API (server-side sort) — 2026-09-14
- Tela de Pagamentos (CRUD + Importação CSV) — 2026-09-14
- Cobertura de Testes — shared/repositories — 2026-09-14
- Cobertura de Testes Backend — 2026-09-14
- Tela de Pedidos — 2026-09-13
- Produtos (CRUD + Importação CSV + Exclusão lógica) — 2026-09-13
- Clientes — Criação e Edição — 2026-09-13
- Gestão de Clientes — 2026-09-13
- [REVERTIDO] Normalização de e-mails para padrão plus-addressing — 2026-09-13
- Fix: coluna deve_trocar_senha ausente no banco + admin duplicado com email antigo — 2026-09-13
- Senha inicial aleatória enviada por email + correção da flag "trocar senha no primeiro acesso" — 2026-09-12
- Identificação do Vendedor na tela de Usuários — 2026-09-12
- Gerenciamento de Usuários — correção de acesso e completude — 2026-09-12
- Auditoria e atualização da collection Postman — 2026-09-07
- Correção de emails dos vendedores — 2026-09-07
- Reset de senha exige senha atual + ID do token — 2026-09-07
- Cor do site em azul claro — 2026-09-07
- Forçar troca de senha no primeiro login do vendedor — 2026-09-07
- Proteção de rotas no Next.js — 2026-09-07
- Dashboard administrativo completo — 2026-09-07
- CRUD de usuários (admin) — 2026-09-07
- Refresh de token JWT — 2026-09-07
- Histórico de alterações de senha — 2026-09-07
- UX-03: dica flutuante (tooltip) no filtro "Pesquisar por data" do estoque — 2026-10-31

