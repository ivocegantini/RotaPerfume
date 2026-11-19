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
- 2026-11-13: Lote 37. 2026-11-14: Lotes 38 (ARQ-09 aplicado na `main`) e 39.

## Avulso de 2026-11-19: ECOM-17 endereço de entrega obrigatório no pedido
- `pedidos.entrega_*` (CEP, rua, número, complemento opcional, bairro, cidade, UF) NOT NULL + `chk_pedidos_entrega` (sql/36, `make db-fix-pedidos-entrega`, no `Migracoes` do Jenkins); antigos, importador CSV e seed 25 recebem endereço fictício (10 capitais por `MOD(id*31,10)`).
- Checkout e POST/PUT `/api/pedidos` exigem `entrega` (400 sem gravar); respostas e "minhas compras/{id}" trazem o endereço. Telas: CEP pesquisa na ViaCEP e libera rua/bairro/cidade/estado (abertos); checkout e modal admin bloqueiam sem endereço. `make test`, `test-integration` e `test-frontend` (1927) rc=0.

## Avulso de 2026-11-19: ECOM-16 minhas compras por pedido, com detalhe
- `GET /api/loja/minhas-compras` passa a um pedido por linha (ID, data, status, total, nomes dos produtos sem repetir; todos os status; `order_by` data/pedido_id/valor_total). Novo `GET /api/loja/minhas-compras/{id}` (AnyRole; rotas 75): itens e pagamentos sem taxa/líquido; pedido alheio = 404. Sem mudança de schema.
- Tela: linha clicável (e link `#ID`) abre `/minhas-compras/[id]` com produtos e pagamento; `Table` ganhou `onRowClick`. Postman, matriz e docs atualizados; `make test`, `test-integration` e `test-frontend` (1923) rc=0.

## Avulso de 2026-11-18: DEPLOY-15 fim do "Importar dump"; migrações 34/35 no pipeline
- Saíram o estágio `Importar dump`, os parâmetros `IMPORTAR_DUMP`/`DUMP_PATH`, a `--dump` (e a etapa) do `setup-servidor.sh` (8 etapas) e o `deploy/dump-local.ps1`; docs ajustadas (seção 7 vira nota).
- `Migracoes` passa a aplicar `sql/34` e `35`; continua depois dos testes/CA e antes do Deploy (os testes do CI não usam banco). Testes DEPLOY-04/10/15 ajustados.

## Avulso de 2026-11-18: BUG-32 baixa do faturamento a partir do último saldo
- Decisão do usuário: o dia novo parte do último saldo − quantidade; o saldo nunca fica negativo; sem saldo, não fatura (`catalogo.ErrEstoqueInsuficienteFaturamento`, 409, rollback do pedido).
- `AjustarSaldoPorFaturamento`: trava o produto (mutex da baixa e do checkout), lê o último snapshot até o dia, UPDATE no mesmo dia ou INSERT do dia novo; itens baixados em ordem de produto. Substitui o upsert do BUG-21 (testes antigos reescritos); integração com 2 faturamentos simultâneos (1 baixa, 1 recusa). `make test-integration` e `lint` rc=0.

## Avulso de 2026-11-18: SEC-79 limites do checkout
- Limite: 10 tentativas por usuário e 30 por IP em 10 min (bloqueio 15 min, 429 + Retry-After), em memória (`adapters/ratelimit`), antes de qualquer SQL.
- Estoque na tx: produto travado (FOR UPDATE, ids em ordem); disponível = último snapshot − pedidos `Loja` "Em separação"; acima → 409. Integração com disputa pela última unidade (1 venda, 1 recusa). `make test-integration`, `lint` rc=0. Achado: BUG-32.

## Avulso de 2026-11-18: ECOM-14 checkout da loja (pedido + pagamento)
- Migração 35 (`make db-fix-pedidos-chave-checkout`): `pedidos.chave_checkout` CHAR(36) NULL UNIQUE (idempotência); DDL 04 no estado final; aplicada no CasaOS :3307.
- `POST /api/loja/checkout` (AnyRole; rotas 74): só produto/quantidade; cliente do token (FOR UPDATE), preço dos ativos no banco, pedido `Loja`/"Loja Online"/`Em separação` + pagamento `Em aberto` numa tx; mesma chave → 200 com o mesmo pedido; inativo 409, equipe 403.
- "Finalizar pedido" ligado (chave UUID, confirmação com nº do pedido, carrinho limpo). `make test`, `lint`, `test-integration` e `test-frontend` (1917) rc=0. Gateway → ECOM-15.

## Avulso de 2026-11-18: ECOM-13 botão Comprar e carrinho lateral
- Botão "Comprar" no cartão da vitrine e na página do produto; carrinho na barra lateral direita (fixa no desktop, gaveta no celular pelo botão 🛒 do topo) com +/−, remover, total e "Ir para o pagamento" → `/checkout`.
- Carrinho no `localStorage` (`lib/carrinho.ts`), validado na leitura (até 50 itens, 99 un.), limpo no "Sair". `/checkout`: resumo editável, forma de pagamento e "Finalizar pedido" desligado (ECOM-14). Só frontend; `make test-frontend` (1914) rc=0 com `NODE_ENV=test`.

## Avulso de 2026-11-18: ECOM-12 minhas compras (branch `ecom-12-minhas-compras`)
- `GET /api/loja/minhas-compras` (AnyRole): produtos dos pedidos não cancelados do cliente ligado ao login (`clientes.usuario_id`), somados por produto; usuário só do token; ordenação por whitelist; equipe sem cliente → `[]`. Sem mudança de schema.
- Tela `/minhas-compras` (tabela ordenável e paginada, link para o produto só se ativo; visitante vê "Entre na sua conta"); link "Minhas compras" no topo da loja. Postman e matriz de rotas (73) atualizados; `make test`, `test-integration` (CasaOS :3307, teste de integração ECOM-12) e `test-frontend` (1906) rc=0; query validada no dev :3307 com ONLY_FULL_GROUP_BY.

## Avulso de 2026-11-18: ECOM-11 página do produto
- `produtos.descricao` → `nome` e nova `descricao` TEXT (migração 34, `make db-fix-produtos-nome-descricao`; DDL 10 no estado final); domínio valida até 5000 caracteres; API/front/Postman com `nome`, `produto_nome` e `?order_by=nome`.
- `GET /api/loja/produtos/{id}` público (só ativo, inativo = 404 igual, limite por IP, cache `vitrine`); página `/produtos/[id]` aberta pelo cartão da vitrine; textarea "Descricao" no admin.
- Importer gera descrição ≥500 caracteres (categoria, nota, embalagem, marca, lançamento) e não sobrescreve a editada. `make test` e `make test-frontend` (1901) rc=0; migração 34 e `db-import-produtos` aplicados no CasaOS :3307 (292 atualizados).

## Avulso de 2026-11-18: DEPLOY-13 (imagem do container alinhada)
- `rotaperfumes-backup-banco --agora` ok (15:54Z, dump validado, 292 imagens) e `mariadb-docker.sh --recriar` do `/root/rotaperfumes-main`: container na `rotaperfumes-mariadb:11.4-5e73baf8179b` (11.4.13), sem o aviso de imagem diferente; TLS 1.3, sem-TLS recusado, DBA ok, fingerprint da CA confere. Segue o aviso antigo do uid 999 (`dnsmasq`).

## Avulso de 2026-11-18: DEPLOY-12 (Lote 43 no servidor)
- Checkout novo `/root/rotaperfumes-main` (clone da `main`, `52240ee`); o `/root/rotaperfumes` antigo não é git (item 6 do DEPLOY-08).
- `backup-banco.sh --instalar` (1º backup ok, 12:23Z) e `mariadb-docker.sh` sem `--recriar`: unit e chain do firewall antigo removidas (`not-found`, chain inexistente), container sem restart, TLS e DBA ok. Aviso de imagem diferente → DEPLOY-13.

## Lote 43 de 2026-11-18: LOTE-43 e DEPLOY-11
> Testes: `make -k test` rc=0 (0 FAIL), sem `GO_TEST_FLAGS`; `shellcheck` ok. Spec em `tarefas/lote-43-spec.md`.
- BACKUP-03: folga de 300 s (`TOLERANCIA_RELOGIO_S`) na "data futura" das guardas do `--recriar`, do seed e do `gerar-imagens-producao.sh`.
- BACKUP-04: `parcial-*` vale na guarda do `--recriar`/`seed --forcar`, com aviso; o mais novo pelo carimbo, sem recuar. O `gerar-imagens-producao.sh` segue recusando (grava no volume de imagens).
- BACKUP-05: a retenção relê todos os pares que ficam na série antes de apagar; par reprovado → nada apagado.
- BACKUP-07: `--dados` diferente do datadir do container é recusado no `--recriar`; `--recriar --trocar-dados` só com `mysql/` no destino, guarda sempre e `dados_antes=`/`dados=` no registro.
- TST-28: `-timeout=30m` antes de `$(GO_TEST_FLAGS)` no Makefile e no Jenkinsfile (o último `-timeout` vale, conferido).
- SEC-73: fingerprint da chave apt da Cloudflare dado como confirmado (keyservers, subchave conferida no teste). R1: o keys.openpgp.org não mostrava a UID; risco aceito pelo usuário (2026-11-07, reaprovado em 2026-11-18).
- DEPLOY-11: `--dev-ip`/`--porta` recusados; código da transição e leitura da porta removidos; unit e chain antigas são removidas pelo script, depois do container sem porta.

## Avulso de 2026-11-18: IMG-02 (imagens de produto na produção)
- Causa: as imagens nunca foram geradas em produção (292 produtos, 0 com imagem); a 3306 fechada impedia o `make db-gerar-imagens-produtos`. Correção: `deploy/gerar-imagens-producao.sh` (build isolado; roda na rede do banco, uid 65532, read-only, conta admin .20 via TLS).
- Servidor: `--confirmar` gerou 292/292, 0 falhas; backup `--agora` com 292 imagens. Correção posterior: a conferência contava 0 (find como root sem CAP_DAC_OVERRIDE); agora roda como o dono do volume e com pipefail.

## Avulso de 2026-11-18: MERGE-01 concluído no servidor
- PR mesclado (`f78597d`); servidor: timer antigo removido, contas `rotaperfumes_backup`/`rotaperfumes-admin` criadas (`casaos_ro` removida), `backup-banco.sh --instalar` (1º backup ok), `verificar-backup.sh` APROVADO (14 tabelas), `--agora` + `--recriar` (imagem nova, 3306 fechada). `--status` ok.
- Observado: volume de imagens de produto vazio (0 arquivos); uid 999 do host é o `dnsmasq` (aviso antigo do script).

## Avulso de 2026-11-17: MERGE-01 etapa 2 (backup, DBA e restauração da `lote-42`)
- BACKUP-01/02: `backup-banco.sh` (timer + `rotaperfumes-backup-banco`) substitui o `backup-mariadb.sh`; `verificar-backup.sh` e `restaurar-maquina.sh` portados; guia com a remoção do timer antigo.
- DEPLOY-10: conta DBA `rotaperfumes-admin` (.22); sem porta publicada na LAN: `casaos` e `casaos_ro` removidas no `--recriar` (decisão do usuário; a .105 perde as consultas). SEC-60 sem efeito; SEC-71 já coberto.
- `seed-producao.sh` sem senha digitada (SEC-52) e `--cap-drop ALL`. `make test` rc=0 (13200 PASS, 0 FAIL), `test-integration` (CasaOS :3307), `lint` e `shellcheck` ok.
- Testes reais na MS-7E28 verdes: BACKUP01 (VerificarBackup, ContaBackup), DEPLOY09, DEPLOY10, Lote42_Legado_QuoteReal, TST15, TST17. ContaBackup passou a contar as tabelas pelo `sql/ordem-ddl.txt` (14).

## Avulso de 2026-11-17: testes reais de deploy (BACKUP-01, TST-15, TST-17) verdes
- Rodados pelo usuário na `residentivo-MS-7E28` (fora da produção), com `sudo` e Docker: `TestBACKUP01_RestauracaoReal_Docker`, `TestTST15_MariadbDocker_Real` e `TestTST17_SeedProducao_Real` (3 subtestes) PASS.
- TST-17: falhava esperando 13 tabelas (o ECOM-03 pôs `confirmacoes_email` no `sql/ordem-ddl.txt`); o esperado agora vem dos `CREATE TABLE` da ordem DDL (14) e o guia (Passo 7) diz 14.

## Avulso de 2026-11-17: SEC-78, SEC-77 e DEP-07
- SEC-78: fechado pelo usuário: as contas `gerente_admin`/`rh_admin` foram extintas (funções passaram ao `admin`).
- SEC-77: `deps-audit-imagens` tira o digest da linha `Digest:` (buildx 0.30 ignora o `--format`); teste com o docker falso nas duas saídas.
- DEP-07: `golang:1.27.4-alpine` no digest novo (`70bd7dde…b280`) em `Jenkinsfile`, `backend/Dockerfile` e `seed-producao.sh`; `go-sql-driver/mysql` 1.10.2, `x/crypto` 0.59, `x/net` 0.63, `x/sys`, `x/text`, `x/image`, `x/term`, `x/mod`, `x/tools` e `yaml` atualizados. `make test`, `lint`, `test-integration` e `deps-audit` rc=0.

## Avulso de 2026-11-17: reavaliação do `FalhasEncontradas.md` (arquivo apagado)
- Itens 1–13 seguem corrigidos no código atual (histórico sem `itenslocais.txt`/`.env`, sem hash na API, rate limit, security headers, CORS por allowlist, XFF só com `TRUST_PROXY_HEADERS`, e-mail mascarado, seed sem senha comum, `ProtectedRoute` com `/api/auth/me`, `make deps-audit`).
- Pendências operacionais resolvidas: senha de app SMTP preenchida, 0 forks, Go local 1.27.4, govulncheck instalado. Restantes viraram SEC-78, SEC-77 e DEP-07.

## Avulso de 2026-11-17: decisões do usuário (DB-08, SEC-60, SEC-76, DEPLOY-10)
- DB-08 (MERGE-01): DSN com `time_zone='-03:00'` (`tz.OffsetSQL`); `DEFAULT`/`ON UPDATE CURRENT_TIMESTAMP` coerentes com o Go. TIMESTAMP antigos gravados pelo Go na sessão UTC passam a aparecer 3 h mais cedo (ex.: `ultimo_login_at`). Integração `TestIntegracaoDB08_*` verde no CasaOS :3307.
- SEC-60: decisão do usuário: a `casaos_ro` mantém o SELECT em `clientes` (com CPF/endereço).
- SEC-76: aceito pelo usuário: a porta 48110 do Redis está fechada no firewall (o tunnel da Cloudflare não a expõe); `REDIS_PASSWORD` segue suportado e a API avisa sem ele.
- DEPLOY-10 (parte de código): estágio `Migracoes` no `Jenkinsfile` e `GOOGLE_CLIENT_ID` no `Build`; teste `tests/deploy/deploy10_jenkinsfile_migracoes_test.go`. Concluído: após a credencial `rotaperfumes-db-admin` corrigida, o estágio `Migracoes` rodou no Jenkins (confirmado pelo usuário).

## Avulso de 2026-11-17: MERGE-01 parcial (BUG-30 e BUG-31 da `lote-42`)
- BUG-30: analytics sem `CURDATE()`; datas de hoje/semana/mês/funil/série calculadas no Go em -03 (`analyticsmysql/datas.go`, relógio injetado em `NovoClientes`/`NovoOportunidades`); testes da `lote-42` adaptados ao ARQ-09. O INSERT do cliente da loja (ECOM-06) também passou a gravar `data_cadastro` com a data do Go. Exceção obsoleta tirada da catraca TST-23.
- BUG-31: `-P $(or $(DB_PORT),3306)` no `MYSQL_OPTS` (porta vazia engolia o `--default-character-set`); teste com mutação.
- `make test`, `make lint` e `make test-integration` (CasaOS :3307) rc=0.

## Avulso de 2026-11-17: ECOM-06 cliente PF no CRM e base do checkout
- ECOM-06: o admin cria/edita cliente pessoa física (CPF + rua/número obrigatórios, complemento/CEP opcionais; PJ igual); API devolve `cpf`/`endereco`/`numero`/`complemento`/`cep`; 1062 do CPF → 409. `sql/33` (`make db-fix-pedidos-canal-loja`): canal `Loja` e vendedor "Loja Online"; aplicado no CasaOS :3307 (id 168). `make test`, `test-integration` e `test-frontend` rc=0.

## Avulso de 2026-11-17: ENV-01 `.env` em `backend/`
- ENV-01: `.env`, `.env.example` e `.env.producao.ro` saíram da raiz para `backend/`; Makefile (`-include backend/.env`), `cmdutil.LoadEnvFromCwd` (API usa o mesmo), `testeemail`, `dump-local.ps1`, mensagens dos scripts, testes e docs ajustados. `ENV_ARQUIVO` é relativo à raiz (ex.: `backend/.env.producao`). `make test` rc=0.

## Avulso de 2026-11-16: merge de todas as branches
- `origin/main`, `lote-37-40` e `lote-42` mescladas na `lote-42-ecommerce-loja` e na `main` local; nos conflitos venceu a versão atual; extras compatíveis entraram (DB-09 como `sql/32`); o resto virou MERGE-01. `make test` e `make lint` rc=0.

## Avulso de 2026-11-16: CACHE-01 cache Redis do e-commerce
- CACHE-01: cache na API (Redis `192.168.168.106:48110`, cliente RESP próprio, sem dependência nova) das áreas `vitrine`, `filtros` e `imagem_produto`; chave `appname:area:vN:hash`, versão por área (INCR na escrita do produto/imagem e na subida); fail-open; `X-Cache`. `make test` rc=0; teste real MISS→HIT ok. Doc: `docs/cache-redis.md`.

## Avulso de 2026-11-16: LEGAL-01 páginas legais (não commitado)
- LEGAL-01: `frontend/public/politica-de-privacidade.html` (LGPD) e `termos-de-servico.html` (CDC), HTML estático com texto genérico. Falta trocar `[e-mail de contato]` e revisão jurídica.

## Lote 42 de 2026-11-16: ECOM-01 início do e-commerce (não commitado)
> Testes: `make test` rc=0; `make lint` ok; frontend 1840 ok; `next build` ok (`/cadastro`, `/loja`).
- ECOM-10: a página inicial (`/`) é a vitrine pública: `GET /api/loja/produtos` e `/api/loja/filtros` viraram Public (só ativos, sem custo, 300 req/min por IP → 429); topo com Entrar/Criar conta ou Olá/Sair (Painel para a equipe), sem mismatch de hydration; `/loja` mostra a mesma vitrine; o cliente da loja volta para `/`. Frontend 1893 ok.
- ECOM-09: endereço pelo CEP (ViaCEP no navegador, só o CEP sai; preenche rua/bairro/cidade/UF, tudo editável); CEP opcional (endereço sem CEP); rua, número ("s/n") e complemento separados; bairro gravado (migração 31: `numero`, `complemento`). Vale no cadastro normal e no com Google. Frontend 1891 ok.
- ECOM-02: "Entrar/Criar conta com Google" (Google Identity Services): ID token validado no backend (RS256/JWKS, iss, aud, exp, email_verified; `adapters/googleid`); só cliente da loja entra pelo Google (pendente vira confirmada); sem conta, o cadastro abre preenchido pedindo CPF/endereço; conta nasce confirmada, sem senha utilizável. `GOOGLE_CLIENT_ID`/`NEXT_PUBLIC_GOOGLE_CLIENT_ID` (vazio = desligado). Frontend 1879 ok.
- ECOM-08: troca e reset de senha bloqueados com o e-mail pendente: própria troca e reset do admin → 409 sem gravar; reset em massa pula a conta (`pulado_email_pendente`, contado no resumo da CLI); botão "Resetar" desabilitado no admin. Só o role `usuario` consulta. Frontend 1873 ok.
- ECOM-07: status `pendente_email` na listagem do admin (badge e filtro; consulta só quando a página tem cliente da loja); só grandes provedores no cadastro (`CADASTRO_EMAIL_DOMINIOS` no `.env`, padrão embutido, domínio exato); UF pelo nome do estado. Login pendente segue bloqueado e o link é de uso único (já do ECOM-03). Login 199 e cliente 3070 apagados a pedido. Frontend 1872 ok.
- ECOM-05: o cadastro da loja cria o cliente pessoa física ligado ao login (migração 30: `cnpj` NULL, `cpf` UNIQUE, `endereco`, `cep`, `usuario_id`), na mesma tx; CPF validado (DV) no front e no back; CPF repetido → 409 neutro com rollback. SELECT do CRM com `COALESCE(cnpj,'')`. Frontend 1869 ok.
- ECOM-03: confirmação de e-mail do autocadastro: conta nasce pendente (migrações 28/29), link de uso único com token no `#fragmento` (24h, só o hash no banco), login do `usuario` pendente → 403 só com a senha certa; `confirmar-email` e `reenviar-confirmacao` públicos (captcha, rate limit, resposta neutra); `APP_URL`; tela `/confirmar-email`. Conta criada pelo admin não muda. Frontend 1854 ok.
- ECOM-04: filtros da vitrine por categoria, marca e nota olfativa (`?nota_olfativa=` e `GET /api/loja/filtros`, AnyRole, distintos dos ativos); selects + "Limpar filtros" na `/loja`. Frontend 1842 ok.
- ECOM-01: role `usuario` (migração 27); política `AnyRole` (me, troca de senha, vitrine) e 403 do `usuario` em toda rota interna; `POST /api/auth/cadastro` (captcha `cadastro`, 5/IP em 10 min, e-mail repetido 409 neutro); `GET /api/loja/produtos` (só ativos, sem custo); telas `/cadastro` e `/loja`; login do `usuario` → `/loja`.

## Lote 41 de 2026-11-15: DEPLOY-09 concluído
- DEPLOY-09: Lotes 39/40 no servidor; timer diário de backup (03:15, 14 dias; 1º backup ok); chave da CA só no pendrive `BACKUP` (conferida por chave pública); `casaos_ro` + `deploy/ca-producao.pem` na .105 com TLS 1.3 (`usuarios` negado, ERROR 1142). 2ª cópia cifrada da chave dispensada pelo usuário neste servidor (fica no guia).
- DEP-03 (parcial): patches tailwind 4.3.5, postcss 8.5.33, user-event 14.6.10; card segue aberto.

## Lote 40 de 2026-11-14: decisões do usuário sobre o Lote 39 (não commitado)
> Testes: `make test` rc=0; `shellcheck` ok; frontend 1816 ok (Node 26 e 22); `next build`, `npm audit --omit=dev` e `govulncheck` sem achados.
- DEP-06: Go 1.27.4 (`go.mod`, `golang:1.27.4-alpine`) e Node 26.13.1 LTS (`.nvmrc`, `engines`, `@types/node` 26); vitest com `--no-experimental-webstorage`.
- SEC-59: `mariadb-docker.sh --dev-ip-ro IP` (porta + firewall, só `casaos_ro`, sem o `casaos` ALL); produção usa `--dev-ip-ro 192.168.168.105`. PII de `clientes` segue legível.
- TST-26: cancelado pelo usuário (UTC já coberto pelo TST-23; o `test-integration` não roda em paralelo nem no Jenkins).
- DOC-10: cancelado pelo usuário (sugestão de `CLAUDE.md` ignorada).
- DEPLOY-09 (passos 1-2): teste de restauração com `--default-character-set=utf8mb4` (cliente da imagem em utf8mb3 recusava o emoji); `setup-linux.sh` instala o `docker-buildx`.
- DEP-03: `npm audit fix --force` desfeito (rebaixou `eslint-config-next` para 15.5); `next` e `eslint-config-next` 16.4.6, lock refeito.

## Lote 39 de 2026-11-14: 8 cards concluídos (não commitado)
> Testes: `make test` rc=0 (10529 PASS, 0 FAIL, 50 SKIP); `lint`, `shellcheck` ok; frontend 1816 ok. 🟣 SecBrain: aprovado com ressalvas, corrigidas no lote.
- BACKUP-01: `deploy/backup-mariadb.sh` (dump conferido, 600/700, `flock`, retenção 14 d); container novo sobre dados exige backup recente ou `--sem-backup` (vai ao journal). Restauração real pendente (ver DEPLOY-09).
- SEC-51: conta `casaos_ro` com `--dev-ip`: SELECT por tabela (sem `usuarios`, `refresh_tokens`, `senha_historico`), ed25519, SSL, 5 conexões, 30 s.
- DEP-05: digests novos do `mariadb:11.4.13` e do `node:22.23.4-alpine`; o tar do "Testes Go" leva `backend/Dockerfile`, `scripts` e a doc.
- SEC-47: `Dockerfile`s da API, frontend e Caddy com `tag@sha256`, conferidos pelo `deps-audit-imagens`.
- SEC-58: chave do MariaDB conferida (fingerprint, sem `sec`/`ssb`, 1 primária, não revogada/expirada); instala só o export-minimal.
- SEC-48: `limpezaimagens -reprocessar-webp` (com `-dry-run`), UPDATE condicional, sem órfãos; rodar com o UID 65532.
- FE-19: `frontend/src/lib/dinheiro.ts` valida até 2 casas e o teto do `DECIMAL(15,2)` nos 7 campos de dinheiro.
- TST-23 (decisão: corrigir testes): `sec12` sem `NOW()`; teste em 3 fusos de sessão; catraca `tst23_catraca_relogio_banco_test.go`. DSN sem `time_zone`.

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

