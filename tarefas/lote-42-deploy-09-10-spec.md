# DEPLOY-10 e DEPLOY-09: especificação do 🟣 SecBrain (2026-11-06)

Linhas referem-se ao estado da `lote-42` em 2026-11-06, com alterações não commitadas. Confira antes de editar. O DEPLOY-10 vem primeiro: o DEPLOY-09 importa o dump com a conta nova.

## Achados
1. `deploy/restaurar-banco.sh`: a única referência é `backup01_databrain_test.go:363-364`, que exige que o arquivo **não exista** (D7).
2. Sem `--recriar`, só as contas migram. Fechar a 3306 exige recriar o container (`mariadb-docker.sh:394,803`).
3. "Recusar checkout" não vale ao pé da letra: o compose compila de `../backend` e `../frontend`. O DEPLOY-09 é como o `--instalar`: execução manual, de um clone conferido.
4. O trio não sobe a API. Faltam, no mínimo, a secret do Turnstile e o `DB_SENHA` novo; SMTP vem do Secret file. Daí o `--api-env`.
5. O hostname público não está no repositório: é argumento obrigatório.
6. Não encadear `setup-servidor.sh`: ele reescreve `CORS_ALLOWED_ORIGINS` só com host e IP da LAN (`:48,296`).
7. Origem do túnel documentada de duas formas: `docker-compose.yml:11` (`localhost:8443`) e `Caddyfile:20-21`/`Jenkinsfile:313-316` (IP da LAN). O usuário confere no painel.

---

# DEPLOY-10: conta `rotaperfumes-admin`

## Conta
- `'rotaperfumes-admin'@'172.30.42.22'`, `ed25519`, `REQUIRE SSL`. Nome sempre entre aspas no SQL.
- `ALL PRIVILEGES ON rotaperfumes.*`, sem privilégio global, `GRANT OPTION`, `PROCESS` ou acesso a `mysql.*`.
- Nenhuma porta publicada.

## `deploy/mariadb-docker.sh`
- **Constantes:** `DB_DBA="rotaperfumes-admin"`, `IP_DBA=$REDE_PREFIXO.22`; `DB_MAKE` vira `DB_LEGADO="casaos"`, usado só para remover.
- **Convergência:** `DB_DBA` em `DESEJADAS`, sem `casaos`; bloco próprio em `sql_contas`, removendo o laço de `DEV_IPS`; `DB_DBA` e `DB_LEGADO` nas listas de remoção/conferência, de modo que qualquer `casaos@*` saia na primeira execução.
- **Sessão antiga:** depois do `DROP`, avisar se houver sessão `casaos` em `information_schema.PROCESSLIST`, sem matar.
- **Arquivos em `$SEGREDOS_DIR/dba-cliente/` (root, 700):** `conta.senha` (600) e `cliente.cnf` (400, gravado como o `.cnf` do backup). Nomes neutros, para não ficar `rotaperfumes-admin.senha` ao lado de `rotaperfumes_admin.senha`. Nada em `container/`.
- **Teste da etapa 8:** pelo comando pronto, `SELECT CURRENT_USER()` deve dar `rotaperfumes-admin@172.30.42.22`, e `--skip-ssl` deve ser recusado.

## Comando pronto: `/usr/local/sbin/rotaperfumes-dba` (700, root)
Gerado pelo `mariadb-docker.sh`, como `gerar_script_firewall`; usa `$IMAGEM` (sem outro digest a manter); só com o container padrão. Uso: `sudo rotaperfumes-dba` e `sudo rotaperfumes-dba < arquivo.sql`.

```bash
docker run --rm -i [-t só se stdin e stdout forem TTY] --name rotaperfumes-dba-cliente \
  --network rotaperfumes-db --ip 172.30.42.22 --read-only --cap-drop ALL \
  --security-opt no-new-privileges:true --log-driver none --tmpfs /tmp:rw,nosuid,nodev,noexec \
  -e MYSQL_HISTFILE=/dev/null \
  --mount type=bind,source=/etc/mysql/ssl/ca.pem,target=/run/dba/ca.pem,readonly \
  --mount type=bind,source=<SEGREDOS>/dba-cliente/cliente.cnf,target=/run/dba/cliente.cnf,readonly \
  --entrypoint mariadb "$IMAGEM" --defaults-extra-file=/run/dba/cliente.cnf \
  -h rotaperfumes-mariadb -P 3306 --ssl-ca=/run/dba/ca.pem --ssl-verify-server-cert \
  --local-infile=0 --default-character-set=utf8mb4 \
  --init-command="SET time_zone='-03:00'" rotaperfumes
```

Regras:
- Argumentos: só `--help` e `--encerrar-sessao` (remove apenas o nome exato do cliente). Nada é repassado ao cliente.
- Interativo: `--safe-updates` e `--prompt='PRD rotaperfumes-admin> '`. Por stdin: `--batch`, parando no primeiro erro.
- Nome ocupado: recusa com instrução, sem remover sozinho.
- Registro: `logger` com data e `SUDO_USER`, sem SQL.
- Antes de conectar: `cliente.cnf` com dono root e modo 400.

## Itens antigos
| Item | Decisão |
|---|---|
| `--dev-ip` | **Fase 1 (este card):** só transição, sem conta. Aceito apenas se o container já publica a porta; recusado em instalação nova e com `--recriar`. Remover o `-p` de `:803` e sempre avisar. **Fase 2 (card novo, depois do item 7 do DEPLOY-08):** recusar como `--rede-lan` e remover o restante. |
| `DB_MAKE` | Só para remoção (`DB_LEGADO`). |
| `casaos.senha` | Nenhum script lê ou apaga; senha não reaproveitada. Mensagem: "mantido; leve ao pendrive seguro". Corrigir `docs/deploy-servidor.md:610`. |
| `--casaos-native-motivo` | Recusa com mensagem própria. `casaos-native.motivo` existente fica como histórico. |
| Saídas de `.env.producao` | Remover. |

## Migração (entre os itens 6 e 7 do DEPLOY-08, sem `--recriar`)
1. Em horário calmo: `sudo bash deploy/mariadb-docker.sh --dev-ip 192.168.168.105`. Cria `rotaperfumes_backup` e `rotaperfumes-admin`, remove `casaos@*`, instala o comando, sem reiniciar o container.
2. `echo 'SELECT CURRENT_USER()' | sudo rotaperfumes-dba`.
3. `sudo bash deploy/backup-banco.sh --instalar`.
4. Item 7: `sudo rotaperfumes-backup-banco --agora`, depois `--recriar` sem `--dev-ip`.

Saída se algo falhar: `root@localhost`. Atualizar também o item 7 em `tarefas/afazer.md` e o passo 2 dos comandos em `tarefas/lote-42-backup-01-spec.md`.

## Evitar confusão com `rotaperfumes_admin`
- Teste cruzado: `rotaperfumes-admin` recusado do `.20`; `rotaperfumes_admin` recusado do `.22`.
- Teste estático: `rotaperfumes-admin` não aparece em `Jenkinsfile`, `seed-producao.sh`, `setup-servidor.sh`, `api.env.example` nem `.env.example`.
- Banner e resumo: "`rotaperfumes-admin` = DBA, `.22`; não é `rotaperfumes_admin` = Jenkins e seed, `.20`".
- `docs/deploy-servidor.md`: coluna "usado por" na tabela de contas, citando a credencial `rotaperfumes-db-admin`.

## Testes (`deploy10_*_linux_test.go`)
- **Stubs:** SQL exato, sem privilégio global; segunda execução sem troca de senha; `casaos@*` removido com qualquer host; `casaos.senha` intacto e nunca criado; `--dev-ip` recusado sem container e com `--recriar`; `--casaos-native-motivo` recusado; modos certos, fora de `container/`.
- **Comando gerado:** sentinela da senha fora de argv, ambiente e log; nenhum argumento repassado; `-t` só com TTY; `--log-driver none`, `--read-only`, `--cap-drop ALL`; sem `MYSQL_PWD`.
- **Real, no descartável:** `ALTER`/`CREATE` em `rotaperfumes` permitidos; `mysql.*`, outro banco e `GRANT` negados; TLS 1.3; `--safe-updates`; `MYSQL_HISTFILE` sem arquivo.
- **`DB_USUARIO`:** `rotaperfumes-admin` aceito no Makefile e em `ValidarFormaDB`; `-x` recusado.
- **Dev:** nada no script cita `:3307`; testes de `tests/config` sobre o CasaOS sem alteração.
- **A reescrever:** `mariadb_docker_test.go` (trechos de `casaos`/`--dev-ip`), `backup01_databrain_test.go:412-414`, `TestTST15_MariadbDocker_Real` se usar `--dev-ip`.

---

# DEPLOY-09: `deploy/restaurar-maquina.sh`

`restaurar-banco.sh` continua proibido: a D7 segue valendo para o **servidor em uso**. Em `backup01_databrain_test.go:362-364`, manter as asserções, atualizar a mensagem e acrescentar teste de que `restaurar-maquina.sh` existe e não tem alvo no Makefile (SEC-46). Incluir o script em `IMAGENS_FIXADAS_EM`, nos testes de paridade e nas listas de troca de digest (`mariadb-docker.sh`, `Jenkinsfile`, `CLAUDE.md`).

## Interface
```
sudo bash deploy/restaurar-maquina.sh --dump ARQ.sql.gz --imagens ARQ.tar.gz \
  --api-env ARQ --ip IP --host-nome NOME --hostname-publico H \
  (--ca-cert A --ca-key B | --nova-ca) \
  [--pepper-file ARQ] [--token-file ARQ] [--tmpfs-mb N] [--sem-tunel] \
  [--backup-dir D --exigir-montagem P] [--par-divergente-motivo TEXTO]
sudo bash deploy/restaurar-maquina.sh --listar-manuais
```
`--sem-tunel` (ensaio): pula `cloudflared` e token e dispensa a declaração sobre a máquina antiga; a escolha fica registrada.

## Recusas
Antes de qualquer mudança. **Nenhuma** flag, variável ou detecção de teste as dispensa.

| Grupo | Regra |
|---|---|
| Execução | Root e TTY; confirma por `/dev/tty`. Recusa `JENKINS_HOME`, `BUILD_ID`, `WORKSPACE` e caminho do Jenkins. |
| Clone | `origin` = `REPO_URL`; `HEAD` = `git ls-remote origin refs/heads/main`; `git status --porcelain --ignored` vazio; árvore e ancestrais do root, sem escrita de grupo/outros, sem symlink na raiz. O script não se instala em `/usr/local/sbin`. |
| Produção ativa | Recusa se existir: container `rotaperfumes-mariadb`, container com label `com.docker.compose.project=rotaperfumes`, `$DADOS_DIR/mysql`, `/etc/mysql/ssl/server-key.pem`, unit ou binário do backup, par em `/var/backups/rotaperfumes`, `cloudflared.service`, volume de imagens com conteúdo. |
| IP | `--ip` configurado nesta máquina (`ip -4 -o addr`). |
| Banco com dados | Depois do `mariadb-docker.sh`, **zero** tabelas em `rotaperfumes`. |
| Entradas | Dump e tar: arquivo regular, sem symlink, nomes do `backup-banco.sh`; recusa `parcial-*` e `pre-restauracao-*`. Carimbos diferentes só com motivo de 10 a 300 caracteres, registrado. Pepper, token e `--api-env`: `arquivo_secreto_ok` (`seed-producao.sh`). |
| CA | `--ca-cert` e `--nova-ca` excludentes, um obrigatório. Recusa CA de dev e chave que não seja da CA. Com `--ca-cert`, fingerprint em `deploy/db-ca.fingerprint`. |
| `api.env` | Leitura linha a linha, sem `source`. Recusa secret de teste do Turnstile, `$` sem aspas simples, chave obrigatória vazia. Exige HTTPS do hostname público em `CORS_ALLOWED_ORIGINS` (e em `TURNSTILE_HOSTNAMES`, quando preenchida), `--ip` e `--host-nome` em CORS. Sem reescrita silenciosa: cita só o nome da chave e para. |

## Etapas
| # | Ação | Parada |
|---|---|---|
| 0 | Recusas, leitura do pepper, `df` com no mínimo 2× os arquivos + 1 GiB | |
| 1 | Copia dump e tar para `/root/rotaperfumes-restauracao/entrada` (700/600), grava sha256; tudo depois usa a cópia. `verificar-backup.sh` com código 0 e `APROVADO`; `backup-banco.sh --conferir-imagens` | |
| 2 | Mostra contagens, carimbo, idade, IP, host, commit | **Digitar o `hostname`.** Sem `--sem-tunel`, também `ANTIGA PARADA` |
| 3 | Com `--ca-cert`, instala `ca.pem`. `mariadb-docker.sh --ip --host-nome [--ca-key\|--nova-ca]`, sem `--dev-ip`/`--recriar`; avisa para **ignorar** o seed sugerido | |
| 4 | Gera `/root/rotaperfumes-restauracao/api.env` (600): `DB_SENHA` novo, `JWT_SECRET` novo, pepper informado. Pepper diferente do `--api-env` gera recusa, sem imprimir | |
| 5 | Grava `importacao-iniciada`; importa pela `rotaperfumes-admin` com `--sandbox --local-infile=0`, conferindo `PIPESTATUS`. No erro, só código e linha. Compara contagens; `TRUNCATE refresh_tokens` | |
| 6 | Lista `sql/*_alter_*`; o DBA aplica o que falta com `sudo rotaperfumes-dba < arq` | **Confirmar** |
| 7 | `validar-ca-banco.sh`, como no Jenkinsfile. Com `--nova-ca`, lista só com o fingerprint novo, só nesta etapa. Exporta `DB_CA_SHA256`, `SITE_*`, `API_ENV_FILE`, `TURNSTILE_SITE_KEY` (lida do Jenkinsfile, formato validado). `docker compose -p rotaperfumes build --pull` e `up --no-start` | |
| 8 | Confere labels do volume e que está vazio. Extrai como no manual, **sem `find -delete`**; compara a contagem | |
| 9 | `up -d` e smoke do Jenkins, com `Host` qualquer, por `SITE_IP` e `https://localhost:8443` | **Login de admin pela LAN e produto com imagem**: única prova do pepper |
| 10 | `backup-banco.sh --instalar` e `--status` | |
| 11 | `cloudflared` (pula com `--sem-tunel`) | **Painel** e leitura do token |
| 12 | Imprime a lista do Jenkins | **Confirmar**; só então apaga `api.env` e `entrada/` |

**Hipótese a validar:** o `up --no-start` cria o volume com labels e dono 65532. Se a escrita como 65532 falhar, o script para.

## Idempotência
- Estado em `/root/rotaperfumes-restauracao/estado` (600), lido sem `source`: etapa, `/etc/machine-id`, dois sha256.
- Retomada só com mesmo `machine-id` e sha256; etapa concluída é pulada, mas o estado real é reconferido.
- O marcador nasce depois da etapa 2 e libera só a parte "produção ativa" das recusas, nunca "banco com dados" fora de `importacao-iniciada`.
- Falha na importação: só nesse estado e sem container da API, oferece `DROP`/`CREATE DATABASE` com o nome digitado.
- Estado `concluida`: recusa sempre.

## Segredos
- Pepper e token: de arquivo ou `read -rs < /dev/tty` (pepper com confirmação). Sem `export`, sem `set -x`, `stty echo` no `trap`, `unset` depois. Gravação com `printf` builtin.
- Conta do banco: nunca `-e MYSQL_PWD`.
- Token só na etapa 11, com o site local provado. Ver "Cloudflare" abaixo.
- Conferência depois de subir: contagem do token no journal (padrão pelo stdin do `grep -cFf`, sem mostrar a linha), `systemctl show` e `/proc/<pid>/cmdline`.

## `cloudflared` com origem verificável
- Chave versionada `deploy/cloudflare-apt.asc` e `deploy/cloudflare-apt.fingerprint`. Fingerprint diferente encerra. Chave em `/usr/share/keyrings/rotaperfumes-cloudflared.gpg`.
- `sources.list.d` com `signed-by` e `https`, sem `trusted=yes`, sem `apt-key`. Pin: origem da Cloudflare com prioridade `-1`, exceto o pacote `cloudflared`. Sem apt: recusa.
- Proibidos, com teste estático: `curl|bash`, `.deb` baixado, `service install`, `--token `.

## Novo IP ou hostname
- O SAN muda e é reemitido, o que exige `--ca-key`. CA reaproveitada não muda `db-ca.fingerprint`.
- `--nova-ca`: commitar o fingerprint e dar push depois de atualizar o Secret file, senão o estágio "CA do banco" recusa.
- `caddy_data` nasce novo: a raiz é reinstalada nos clientes da LAN.
- IP diferente, com commit: `Jenkinsfile:43-44`, `docker-compose.yml:136-137`, `mariadb-docker.sh:73`, `setup-servidor.sh:56-57`, guardas em `Makefile:299` e `integracao.go:35`. No Secret file e no painel: CORS, `TURNSTILE_HOSTNAMES`, hostnames do Turnstile.
- Não mexer: `Makefile:398`, `integracao.go:45`, `.env.example:27` (CasaOS de dev).

## `--listar-manuais` (só nomes, nunca valores)
- **Cloudflare:** hostname público; serviço `https://localhost:8443` com noTLSVerify; token; parar o conector antigo; hostnames adicionais; hostnames do Turnstile.
- **Jenkins:** instalação, plugins, job (`docs/deploy-servidor.md` §§5-6); Secret file `rotaperfumes-api-env` com o `api.env` novo; credencial `rotaperfumes-db-admin` com a senha nova de `rotaperfumes_admin`.
- **Só no Secret file:** `PASSWORD_PEPPER`, `JWT_SECRET`, `TURNSTILE_SECRET_KEY`, `SMTP_*`, `SECURITY_ALERT_EMAILS`, `CORS_ALLOWED_ORIGINS`, `TURNSTILE_HOSTNAMES`.
- **Ao final:** raiz da CA do Caddy nos clientes; `ca-key.pem` offline, com `--nova-ca`; cópia externa.

## Testes (`deploy09_*_linux_test.go`)
- **Recusas com stubs e namespace:** cada sinal de produção, um por vez; clone sujo, ignorado presente, `HEAD` diferente, dono errado; sem TTY, com variáveis do Jenkins; em cada caso, **nenhuma** chamada mutável ao Docker; variáveis como `FORCAR` não mudam nada.
- **Ordem:** verificar antes de confirmar, confirmar antes de importar. Troca do original depois da cópia não afeta o importado.
- **Tabelas:** uma tabela qualquer gera recusa.
- **Retomada:** pula o concluído; sha256 ou `machine-id` diferentes recusam; `DROP` só em `importacao-iniciada` sem API; `concluida` recusa.
- **Segredos:** sentinelas de pepper e token fora de saída, log e argv dos stubs, incluindo erros; modo 600; `api.env` e `entrada/` apagados na etapa 12.
- **Chave do apt:** fixtures certa e errada, com `apt-get` stub; a errada não grava nada.
- **Unit:** `systemd-analyze verify` e conteúdo gerado.
- **IP:** a lista impressa cobre os literais de `192.168.168.106` fora de docs e testes.
- **Real, Docker local, dados sintéticos, sem root:** ida e volta das etapas 5 a 8, `up --no-start`, labels e dono do volume, importação com `--sandbox` pela conta real.
- **Geral:** `make shellcheck`. Testes com `unshare` pulam no CI; fixtures novas precisam estar rastreadas.
- **Só em máquina real:** Docker e systemd de ponta a ponta com `--sem-tunel`, apt real, `cloudflared` com token e painel.

**Documentação:** o trecho de restauração do `docs/manual-base-de-dados.md` aponta para o script e mantém o passo a passo como referência. Registrar em `feito.md` que a D7 foi revista só para máquina nova.

## Riscos que restam
- Dois conectores ativos com o mesmo token: o script não enxerga a outra máquina; a proteção é a declaração digitada.
- Pepper errado só aparece no login da etapa 9: essa parada não pode ser removida.
- Sem cópia externa (D1), com o disco perdido não há o que restaurar.
- Migrações dependem de conferência humana (card sugerido: comparar schema do dump com `sql/ordem-ddl.txt` em container sem rede).
- Root comprometido lê `api.env` temporário, token e `.cnf` do DBA.
- `DROP DATABASE` segue possível pela conta DBA.
- Ensaio com o `api.env` real pode enviar e-mails verdadeiros.
- Trocar o servidor também afeta o CasaOS de dev.
- `MYSQL_PWD`: usos antigos seguem em `mariadb-docker.sh`, `seed-producao.sh`, `Jenkinsfile` e `admin_sql` (card a abrir).

## Decisões do usuário (recomendação entre parênteses)
1. Futuro do `--dev-ip` (transição sem conta; remoção em card depois do item 7).
2. Privilégios do DBA (`ALL` só em `rotaperfumes`, sem `PROCESS`).
3. Na troca de máquina, senhas novas ou do pendrive (novas; Jenkins atualizado na etapa 12).
4. IP e hostname da máquina nova (manter).
5. CA (reaproveitar, com `ca-key.pem` offline; `--nova-ca` só se a chave se perdeu).
6. Token do túnel (novo, se a máquina antiga foi perdida ou está sob suspeita).
7. `--api-env` além do trio (aprovar; cópia do Secret file em cofre, longe do dump).
8. Escopo da D7 (rever só para máquina nova).
9. Ensaio com dump real (só em máquina com a proteção da produção).
10. D1 e D2 (decidir antes de considerar o DEPLOY-09 pronto).
11. Pendrive (pepper e senhas em pendrive cifrado; `admin-app.senha` com `shred`; de `/root/rotaperfumes.antigo`, só senhas).
12. Origem do túnel no painel (conferir e manter `https://localhost:8443`, salvo divergência).

## Cloudflare: o que foi conferido em 2026-11-06 (🤍 MegaBrain)
- **Repositório apt (página `pkg.cloudflare.com`):** `deb [signed-by=...] https://pkg.cloudflare.com/cloudflared any main`; o `InRelease` responde com `Origin: cloudflared`. Chave em `https://pkg.cloudflare.com/cloudflare-main.gpg`. A página não publica o fingerprint e avisa que a chave foi trocada.
- **Chave baixada:** `CloudFlare Software Packaging 2025`, fingerprint `CC94B39C77AE7342A68B89628A682D308D4E5E73`. Veio só pelo próprio HTTPS da Cloudflare: **confira por um segundo canal** antes de versionar em `deploy/cloudflare-apt.fingerprint`.
- **`service install <token>`:** segundo um PR na documentação da Cloudflare (cloudflare-docs #33157), grava o token na unit, e ele aparece em `/proc/<pid>/cmdline`. Mantém a proibição.
- **Token (página atual "run parameters", reaberta):** `TUNNEL_TOKEN` e `--token-file`/`TUNNEL_TOKEN_FILE` valem para túnel gerenciado pelo painel; o `--token-file` exige versão 2025.4.0 ou mais nova. Fica o `TUNNEL_TOKEN` no `EnvironmentFile` 600 da especificação: não depende de a conta `DynamicUser` conseguir ler o arquivo.
- **Dois conectores (documentação "tunnel availability"):** dois hosts com o mesmo token viram réplicas ativas ao mesmo tempo, e cada requisição vai para a mais próxima. O risco da máquina antiga receber tráfego é real; mantém a declaração `ANTIGA PARADA`.
- **Rotação (documentação "tunnel tokens"):** "Rotate token" no painel bloqueia conexões novas com o token antigo, mas os conectores atuais seguem ativos até reiniciar. Com máquina perdida ou suspeita, é preciso também limpar as conexões (`DELETE` em `/connections` da API). Incluir isso no `--listar-manuais`.
- **Não confirmados:** caminho do binário no pacote (use `command -v`) e o nome da opção `noTLSVerify` no painel.
