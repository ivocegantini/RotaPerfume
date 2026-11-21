# Deploy no servidor da LAN (`ivo-inspiron-15-3530`) com Docker + Jenkins

> Arquivos: `Jenkinsfile`, `deploy/docker-compose.yml`, `deploy/Caddyfile`, `deploy/Dockerfile.caddy`, `deploy/api.env.example`, `deploy/mariadb-docker.sh`, `deploy/seed-producao.sh`, `deploy/validar-ca-banco.sh`, `deploy/db-ca.fingerprint`, `deploy/setup-servidor.sh`, `sql/schema.sql`, `sql/migracao.sql`, `backend/Dockerfile`, `frontend/Dockerfile`.
>
> **Primeira instalação?** Siga a [seção 11](#11-instalação-inicial-da-produção-deploy-07) antes do primeiro deploy da `main`. Sem a rede `rotaperfumes-db` e o container `rotaperfumes-mariadb`, o estágio `CA do banco` para o job.

| Ambiente | Servidor | Banco | Conta | Configuração | Uso |
| --- | --- | --- | --- | --- | --- |
| Desenvolvimento | CasaOS, MariaDB 11.8, porta 3307 | `rotaperfumes` | `casaos`@`192.168.168.%` | `backend/.env`; CA `deploy/ca-cert.pem` | `make dev-api` e dia a dia |
| Teste (descartável) | CasaOS, porta 3307 | `rotaperfumes_teste` | `casaos` (GRANT só nesse nome) | `make test-integration` (TST-25) | recriado e apagado a cada execução |
| Produção | container `rotaperfumes-mariadb`, MariaDB 11.4, porta 3306 interna | `rotaperfumes` | contas da seção 3.4 | Jenkins / `deploy/*.sh` | API publicada; nunca testes, `db-down`/`db-seed`/`db-reset` |

GRANT do banco de teste (root no CasaOS): ``GRANT ALL PRIVILEGES ON `rotaperfumes\_teste`.* TO 'casaos'@'192.168.168.%';``. As CAs de dev e de produção nunca se misturam.

## 0. Instalação automática (recomendado)

Duas partes, nesta ordem: (1) banco pelo `deploy/mariadb-docker.sh` (seção 3; TLS em [`docs/Instal_SSL.md`](Instal_SSL.md)); (2) aplicação e Jenkins pelo `deploy/setup-servidor.sh`, que não mexe em banco.
As duas credenciais do Jenkins são cadastradas à mão: o script pausa, mostra as instruções e só confere se existem. Antes, faça push dos arquivos de deploy na `main` e instale os plugins Pipeline, Git e Credentials Binding.

```bash
sudo bash deploy/mariadb-docker.sh --nova-ca                              # primeira instalação
sudo bash deploy/mariadb-docker.sh --ca-key /media/<pendrive>/ca-key.pem  # CA existente
```

```powershell
scp backend\.env SEU_USUARIO@ivo-inspiron-15-3530:/tmp/      # no Windows
```

```bash
curl -fsSLO https://raw.githubusercontent.com/residentivo/RotaPerfume/main/deploy/setup-servidor.sh
sudo bash setup-servidor.sh --env /tmp/.env --turnstile-site-key SUA_SITE_KEY --remove-inputs
```

- Opções: `--site-host` (`ivo-inspiron-15-3530`), `--site-ip` (`192.168.168.106`), `--https-port` (`8443`), `--jenkins-url` (`http://localhost:8888`), `--jenkins-container`, `--db-container`, `--db-segredos`, `--skip-build`, `--remove-inputs`. Veja `--help`.
- Etapas `[n/8]`: pré-checagens (inclusive o banco no ar), Jenkins com acesso ao Docker, ufw (só libera a 8443 se ativo), `~/rotaperfumes-api.env` (600, `DB_*` do `rotaperfumes_app`, `JWT_SECRET` novo, `COOKIE_SECURE=true`; demais chaves, inclusive `PASSWORD_PEPPER`, do seu `.env`), job, pausa para as credenciais (seção 5), primeiro build e resumo.
- Reexecução: o `mariadb-docker.sh` não troca senhas; o `setup-servidor.sh` gera `JWT_SECRET` novo (atualize o Secret file no Jenkins). A API não sobe com `JWT_SECRET` < 32 bytes (SEC-18); no servidor, `COOKIE_SECURE=true` e nunca `DB_TLS_REQUIRED=false` (SEC-21/42).

## 1. Visão geral

```
Navegador (LAN) https://ivo-inspiron-15-3530:8443     Navegador (internet) → cloudflared (host) → https://localhost:8443
┌──────────── docker compose -p rotaperfumes ────────────┐
│  caddy (tls internal, 443 → 8443 no host; + curinga)   │
│     ├── /api/health → api:8080/health                  │
│     ├── /api/*      → api:8080       (API Go)          │
│     └── resto       → frontend:3000  (Next standalone) │
└──────────────────────────┬─────────────────────────────┘
          api 172.30.42.10 → rotaperfumes-mariadb:3306 (TLS 1.3, CA própria)
          rede Docker externa "rotaperfumes-db" (172.30.42.0/24), sem porta publicada
```

- Estágios do `Jenkinsfile`: `Checkout` → `Testes Go` (`GO_TEST_IMAGE`, código por `git archive HEAD`, SEC-62) → `Testes Frontend` (`npm ci`, lint e testes em `NODE_IMAGE`) → `Build` → `CA do banco` (3.5) → `Migracoes` (`sql/migracao.sql`, conta admin pela rede interna) → `Deploy` (`up -d`) → `Smoke test`.
- Só o Caddy publica porta (8443). Tunnel: hostname público → `https://192.168.168.106:8443` com `noTLSVerify: true`; o Caddy só aceita `CF-Connecting-IP` do próprio servidor. Cadastre o domínio no Turnstile.
- HTTPS é obrigatório (cookies `Secure`). Frontend e API na mesma origem: `/api/...` relativo (`PUBLIC_URL` padrão `/`, embutido no build).

| Parâmetro do job | Padrão | Uso |
|---|---|---|
| `SITE_HOST` | `ivo-inspiron-15-3530` | hostname na LAN (Caddy) |
| `SITE_IP` | `192.168.168.106` | IP da LAN; smoke test |
| `HTTPS_PORT` | `8443` | porta do Caddy |
| `TURNSTILE_SITE_KEY` | `0x4AAAAAAFACjc-LtM9NWlvP` | site key pública, no bundle |

Constantes do `environment`: `DB_TLS_CA_HOST_PATH=/etc/mysql/ssl/ca.pem`, `DB_REDE=rotaperfumes-db`, `DB_HOST_INTERNO=rotaperfumes-mariadb`, `DB_HOST_PORT=3306`, `DB_CLIENTE_IP=172.30.42.20`, `GO_IMAGE`, `GO_TEST_IMAGE`, `NODE_IMAGE`, `DB_CLIENT_IMAGE` (por digest).

| ID da credencial | Tipo | Conteúdo |
|---|---|---|
| `rotaperfumes-api-env` | Secret file | `.env` da API (modelo `deploy/api.env.example`), `DB_USUARIO=rotaperfumes_app` + `rotaperfumes_app.senha` |
| `rotaperfumes-db-admin` | Username with password | `rotaperfumes_admin` + `rotaperfumes_admin.senha` (estágios `CA do banco` e `Migracoes`) |

## 2. Pré-requisitos no servidor

1. Docker Engine com `docker compose` **≥ 2.17** (`docker compose version`).
2. Jenkins com acesso ao Docker do host. Nativo: `sudo usermod -aG docker jenkins && sudo systemctl restart jenkins`, conferir com `sudo -u jenkins docker ps`. Em container: socket montado, `docker-ce-cli` e `docker-compose-plugin` na imagem.
3. No agente: `bash curl git gzip zcat install openssl` (o `git` é obrigatório; tudo que os testes usam precisa estar commitado, SEC-62).
4. Plugins Pipeline, Git e Credentials Binding; porta 8443 livre (`sudo ss -ltnp | grep ':8443'`).

## 3. Banco: MariaDB em Docker (`deploy/mariadb-docker.sh`)

MariaDB 11.4 LTS em container criado fora do compose. O driver Go é o `go-sql-driver/mysql` (compatível, com ed25519).

### 3.1 Instalar ou atualizar

```bash
sudo bash deploy/mariadb-docker.sh [--ca-key <mídia>/ca-key.pem | --nova-ca] [--recriar]
```

1. **Pré-requisitos:** root, ferramentas, IPs e caminhos; sessão do DBA aberta recusa. Container divergente (rede, IP, portas, read-only, usuário, datadir) pede `--recriar`; trocar o datadir exige `--recriar --trocar-dados`.
2. **Imagem** `rotaperfumes-mariadb:11.4-<12 hex do digest>` com `zz-rotaperfumes.cnf` (TLS obrigatório, `TLSv1.2,TLSv1.3`, ed25519, `local_infile = 0`, `sql_mode` estrito com `ONLY_FULL_GROUP_BY`, `utf8mb4_unicode_ci`, `skip_name_resolve`).
3. **Certificados** em `/etc/mysql/ssl` (`--ssl-dir`); chave de CA gerada (`--nova-ca`) em `/root/rotaperfumes-ca` (`--ca-dir`), que deve sair do servidor.
4. **Senhas** em `/root/rotaperfumes-mariadb/` (`--segredos-dir`, 700/600): `root`, `rotaperfumes_app`, `rotaperfumes_admin`, `rotaperfumes_backup`, `dba-cliente/conta.senha`; geradas uma vez, nunca impressas. Um `casaos.senha` antigo é mantido (leve ao pendrive seguro).
5. **Rede** (3.2) e remoção do firewall antigo (3.3). 6. **Container**: IP `172.30.42.2`, `--cap-drop ALL`, `--read-only`, datadir `/var/lib/rotaperfumes/mariadb`, `MARIADB_AUTO_UPGRADE=1`, logs 5 × 10 MB.
7. **Contas** (3.4), convergidas a cada execução. 8. **Teste TLS** a partir de `172.30.42.20` (`--skip-ssl` aceito aborta) e instalação/teste do `/usr/local/sbin/rotaperfumes-dba`.

- No fim imprime o fingerprint da CA e avisa se a chave da CA ainda está no servidor. No servidor, use os diretórios padrão (o `seed-producao.sh` só lê `/root/rotaperfumes-mariadb`).
- Diretórios: absolutos, disjuntos (por `realpath -m`), sem symlink, do root e 700; ancestrais sem escrita de grupo/outros. Nada é corrigido: o script falha.
- Mudança de `sql_mode` ou imagem só vale após `sudo rotaperfumes-backup-banco --agora` e `sudo bash deploy/mariadb-docker.sh --recriar`. O CasaOS de dev não é configurado por este script.

### 3.2 Rede `rotaperfumes-db`

- `172.30.42.0/24`, gateway `.1`, `--ip-range 172.30.42.128/25` (dinâmicos). Fixos: MariaDB `.2`, API `.10`, admin/import `.20`, backup `.21`, DBA `.22`.
- A API usa `rotaperfumes-mariadb:3306`; `DB_HOST`, `DB_PORT`, `DB_TLS_CA`, `DB_TLS_SERVER_NAME` são fixados no compose. Rede e container precisam existir antes do `docker compose up`.

### 3.3 Sem porta publicada

- Nenhuma porta publicada; container que publique a 3306 diverge e pede `--recriar`. `--dev-ip`, `--porta` e `--rede-lan` são recusados.
- Se ainda existirem a unit `rotaperfumes-db-firewall.service` ou a chain `ROTAPERFUMES-DB`, o script as remove. Confira: `sudo iptables -S ROTAPERFUMES-DB` dá chain inexistente.

### 3.4 Contas e senhas

Todas `IDENTIFIED VIA ed25519` + `REQUIRE SSL`, criadas pelo stdin do cliente:

| Conta | Host | Privilégios | Senha | Usado por |
|---|---|---|---|---|
| `rotaperfumes_app` | `172.30.42.10` | `SELECT, INSERT, UPDATE, DELETE ON rotaperfumes.*` | `rotaperfumes_app.senha` | API (Secret file `rotaperfumes-api-env`) |
| `rotaperfumes_admin` | `172.30.42.20` | `ALL ON rotaperfumes.*` | `rotaperfumes_admin.senha` | Jenkins (`rotaperfumes-db-admin`), `seed-producao.sh`, `gerar-imagens-producao.sh`. **Não é o DBA** |
| `rotaperfumes_backup` | `172.30.42.21` | `SELECT ON rotaperfumes.*` | `rotaperfumes_backup.senha` | backup (`docs/manual-base-de-dados.md`, seção 24) |
| `rotaperfumes-admin` | `172.30.42.22` | `ALL ON rotaperfumes.*`, sem privilégio global, `GRANT OPTION`, `PROCESS` ou `mysql.*` | `dba-cliente/conta.senha` | DBA: `sudo rotaperfumes-dba`. **Não é o `rotaperfumes_admin`** |
| `root` | `localhost` | (todos) | `root.senha` | só `docker exec` |

- A cada execução: `CREATE USER IF NOT EXISTS` + `ALTER USER`, `REVOKE ALL` + `GRANT`, `DROP USER` de host fora do desejado e da `casaos` em qualquer host, sem anônimos nem banco `test`.
- Root: `sudo docker exec -it rotaperfumes-mariadb mariadb --defaults-extra-file=/run/rotaperfumes/root.cnf`. Para (re)convergir contas sem reiniciar o container:

```bash
sudo bash deploy/mariadb-docker.sh
```

```bash
sudo rotaperfumes-dba                    # interativo: --safe-updates, prompt "PRD rotaperfumes-admin> "
sudo rotaperfumes-dba < arquivo.sql      # por stdin: --batch, para no primeiro erro
sudo rotaperfumes-dba --encerrar-sessao  # remove o cliente rotaperfumes-dba-cliente que ficou aberto
echo 'SELECT CURRENT_USER()' | sudo rotaperfumes-dba   # rotaperfumes-admin@172.30.42.22
```

- Nenhum argumento vai ao cliente (container descartável, `--read-only`, `--log-driver none`, `--pull never`); stdin no terminal com saída redirecionada é recusado.
- Cada sessão vai ao syslog (`rotaperfumes-dba`) sem SQL. `DROP DATABASE rotaperfumes` é possível por esta conta: confira o que vai rodar.

### 3.5 Como o pipeline usa a CA do banco (SEC-29/SEC-40)

- A CA fica no host (`/etc/mysql/ssl/ca.pem`, 644) e é montada pelo daemon Docker; o repositório guarda só o fingerprint (3.7).
- `CA do banco`: roda `deploy/validar-ca-banco.sh` em `DB_CLIENT_IMAGE` como uid 65532; recusa PEM que não seja `CERTIFICATE`, vencido ou fora de `deploy/db-ca.fingerprint`, e confere com `openssl s_client -starttls mysql` a partir de `172.30.42.20` (exige handshake real).
- O sha256 da CA vai à label `rotaperfumes.db-ca-sha256` da API: CA nova recria o container. `Migracoes` usa o cliente `mariadb` com `--ssl-verify-server-cert`, `--local-infile=0` e `MYSQL_PWD`.
- A API exige TLS 1.3 com `DB_TLS_CA`; sem ele e host remoto não sobe (SEC-42). Log: `[server] TLS do banco: ligado (...)`. Rotação: `docs/Instal_SSL.md`, passo 9.

### 3.6 Atualizar a imagem do MariaDB (digest, SEC-31)

- A mesma string `mariadb:<patch>@sha256:...` aparece no `IMAGEM_BASE` de `deploy/mariadb-docker.sh`, `backup-banco.sh`, `verificar-backup.sh` e `restaurar-maquina.sh` e no `DB_CLIENT_IMAGE` do `Jenkinsfile` e do `seed-producao.sh` (testes conferem), mais o `IMG=` deste documento e do `docs/Instal_SSL.md`.
- `GO_IMAGE`, `GO_TEST_IMAGE` e `NODE_IMAGE` ficam só no `Jenkinsfile` (versão do `go.mod` / `frontend/.nvmrc`); os Dockerfiles usam `ARG` iguais (SEC-47). Troque a string e rode `make test`.

1. `make deps-audit-imagens` ou `docker buildx imagetools inspect mariadb:11.4 --format '{{.Manifest.Digest}}'`.
2. Troque a string nos seis arquivos acima e rode `make test`.
3. No servidor, após o merge: `sudo bash deploy/backup-banco.sh --instalar`, `sudo rotaperfumes-backup-banco --agora`, `sudo bash deploy/verificar-backup.sh <dump>` e `sudo bash deploy/mariadb-docker.sh --recriar`.

Só patch do 11.4 por este caminho; versão menor/maior é card próprio.

### 3.7 Pinagem da CA (`deploy/db-ca.fingerprint`, SEC-40)

- SHA-256 do DER de cada certificado aceito, hex minúsculo sem `:`, um por linha (`#` ignorado); normalmente um, dois durante a troca de CA.
- Calcular: `openssl x509 -in /etc/mysql/ssl/ca.pem -noout -fingerprint -sha256 | cut -d= -f2 | tr -d : | tr A-F a-f`. Toda troca de CA exige commit revisado deste arquivo.
- Só a CA de produção (hoje `f31ee820...`). CAs de dev (`d150ee67...` em `deploy/ca-cert.pem`, `ef31b656...` anterior) são proibidas (`TestDEPLOY07_CADevNuncaEmProducao`).

### 3.8 Regra: integração só em banco descartável

- `INTEGRATION=1`, `db-down`, `db-reset`, `db-rebuild`, `db-seed`, `db-import-*` apagam ou sobrescrevem dados: só em banco descartável ou de dev, nunca na produção.
- `db-guard-producao`: produção = `192.168.168.106`/`ivo-inspiron-15-3530` com porta vazia ou 3306, ou `rotaperfumes-mariadb`/`172.30.42.2` em qualquer porta; válvula só `CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME>` na linha de comando.
- SEC-54/56/57: host normalizado (minúsculas, `[ ]`, `.` finais, `.local`, `::ffff:`); IPv4 não canônico e porta não numérica recusados sem válvula.
- SEC-46: `db-down`/`db-reset`/`db-rebuild`/`db-seed` nunca na produção; DROP de `rotaperfumes` fora do loopback exige `CONFIRMAR=rotaperfumes`. BUG-27/28: etapas em cadeia, param na primeira recusa.
- BUG-29/SEC-59: use `make -k ...`; `MAKEFLAGS=` na linha de comando, `ENV_ARQUIVO` exportado e `.env` fora de `NOME=valor` literal (inclusive `$`) são recusados no parse.
- SEC-50/53/55: integração recusa a produção antes de conectar e só aceita loopback ou `192.168.168.106:3307`; segredos não chegam a test/build/lint/npm.

## 4. Cloudflare Turnstile

1. No painel, cadastre o hostname `ivo-inspiron-15-3530` (e o do tunnel). Site Key → parâmetro `TURNSTILE_SITE_KEY`; Secret Key → `TURNSTILE_SECRET_KEY` do Secret file.
2. Acesse pelo hostname, não pelo IP. Sem DNS, no Windows (`C:\Windows\System32\drivers\etc\hosts`): `192.168.168.106   ivo-inspiron-15-3530`.
3. Secret de teste (`1x…`, `2x…`, `3x…`) impede a subida com `COOKIE_SECURE=true` (SEC-37). A válvula `TURNSTILE_PERMITIR_SECRET_TESTE=true` desliga o captcha: só nesta rede interna e por tempo curto.

## 5. Credenciais no Jenkins

Caminho: **Gerenciar Jenkins → Credentials → System → Global credentials → Add Credentials**.

### 5.1 Secret file `rotaperfumes-api-env`

1. Copie `deploy/api.env.example` para fora do repositório e preencha: `DB_USUARIO=rotaperfumes_app`, `DB_SENHA` (`sudo cat /root/rotaperfumes-mariadb/rotaperfumes_app.senha`), `JWT_SECRET` novo (`openssl rand -base64 48`), `COOKIE_SECURE=true`.
2. `TURNSTILE_SECRET_KEY` real, `PASSWORD_PEPPER` igual ao que gerou os hashes do banco (nunca troque), `SMTP_*`/`SECURITY_ALERT_EMAILS`, `CORS_ALLOWED_ORIGINS` coerente. Sem `DB_TLS_REQUIRED=false`; `$` entre aspas simples.
3. Envie com ID **`rotaperfumes-api-env`** e apague o arquivo local. O pipeline grava `deploy/api.env` (600) só durante o deploy.

### 5.2 Username with password `rotaperfumes-db-admin`

Username `rotaperfumes_admin`, senha = `/root/rotaperfumes-mariadb/rotaperfumes_admin.senha`, ID **`rotaperfumes-db-admin`**.

### 5.3 GitHub (só repositório privado)

Username with password com um Personal Access Token de leitura; ID livre (ex.: `github-residentivo`).

## 6. Criar o job

Novo item `rotaperfumes-deploy` → **Pipeline** → *Pipeline script from SCM*, Git `https://github.com/residentivo/RotaPerfume.git`, branch `*/main`, Script Path `Jenkinsfile`.
Na primeira execução não há parâmetros ("Construir agora"); depois aparece "Construir com parâmetros".

## 7. Carga inicial e atualização do schema

A produção nasce pelo `deploy/seed-producao.sh` (seção 11); não há importação de dump pelo Jenkins. Colunas e tabelas novas chegam pelo `sql/migracao.sql` no estágio `Migracoes`.
Restaurar um backup: `docs/manual-base-de-dados.md`, seção 24.5.

## 8. Primeiro acesso e certificado (CA do Caddy)

```bash
docker cp rotaperfumes-caddy-1:/data/caddy/pki/authorities/local/root.crt ./rotaperfumes-caddy-root.crt
```

- Copie para o Windows (`scp SEU_USUARIO@192.168.168.106:~/rotaperfumes-caddy-root.crt .`) e instale em **Autoridades de Certificação Raiz Confiáveis** (Firefox: repositório próprio ou `security.enterprise_roots.enabled`).
- Acesse **https://ivo-inspiron-15-3530:8443** e entre com o admin do seed (seção 11, passo 10). Nunca rode `docker compose down -v`: a CA vive no volume `caddy_data`.

## 9. Operações do dia a dia

No servidor, em `/var/lib/jenkins/workspace/rotaperfumes-deploy/deploy`, sempre com `API_ENV_FILE=/dev/null`:

| Ação | Comando |
|---|---|
| Status | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes ps` |
| Logs | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes logs -f --tail 100 [api]` |
| Logs do banco | `docker logs -f --tail 200 rotaperfumes-mariadb` |
| Parar / religar | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes stop` / `start` |
| Remover (mantém a CA) | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes down` |

Nunca rode `up` à mão (a API subiria sem variáveis): rode o job. O banco não faz parte do compose; para recriá-lo: `sudo rotaperfumes-backup-banco --agora` e `sudo bash deploy/mariadb-docker.sh --recriar`.

### Backup do banco e das imagens de produto

- Timer às 03:30 (até 15 min de atraso; roda ao ligar se perdeu). Cada backup bom é um par com o mesmo carimbo em `/var/backups/rotaperfumes` (700): `rotaperfumes-AAAAMMDD-HHMMSSZ.sql.gz` e `rotaperfumes-imagens-AAAAMMDD-HHMMSSZ.tar.gz`.
- O que fica fora do dump e a restauração: `docs/manual-base-de-dados.md`, seções 24.2, 24.5 e 24.6. Instalar: `sudo bash deploy/backup-banco.sh --instalar` (o volume de imagens precisa existir).
- O root executa só a cópia `/usr/local/sbin/rotaperfumes-backup-banco`: depois de mudar o script ou o digest, rode `--instalar` de novo.

| Comando | Efeito |
| --- | --- |
| `sudo rotaperfumes-backup-banco --agora` | backup na série `-manual` |
| `sudo rotaperfumes-backup-banco --agora --aceitar-reducao` | aceita uma vez um dump < 50% do anterior |
| `sudo rotaperfumes-backup-banco --status` | erro se o último par tem > 26 h, data futura, dump/tar reprovado, timer inativo ou última execução com falha |
| `sudo bash deploy/backup-banco.sh --conferir-imagens ARQ` | confere um tar de imagens (só contagens) |
| `sudo bash deploy/backup-banco.sh --instalar` | instala ou atualiza a cópia, as units e o conf |

- **Retenção:** 14 diários e 3 manuais, por pares bons (`--retencao-diarios N --retencao-manuais N`, mínimo 2). Falha sem apagar nada com par corrompido ou data futura; não toca em `pre-restauracao-*` nem `parcial-*`.
- **Espaço:** exige livre ≥ 2× o dump anterior + o maior entre 2× as imagens e o volume atual + 1 GiB. Falha das imagens grava `parcial-rotaperfumes-*.sql.gz`, que não conta como backup bom.
- **Guarda do `--recriar` e do `seed-producao.sh --forcar`:** exigem dump de até 60 min, válido e sem data futura (folga 300 s); `--sem-backup-motivo TEXTO` (10 a 300 caracteres) dispensa ou justifica `--backup-janela-min` > 60, registrado em `/root/rotaperfumes-mariadb/`.
- **Outro disco:** `sudo bash deploy/backup-banco.sh --instalar --backup-dir /mnt/ssd/rotaperfumes --exigir-montagem /mnt/ssd`; disco desmontado faz o backup falhar. Volta: `--backup-dir /var/backups/rotaperfumes --sem-exigir-montagem`.
- Sem aviso externo, a falha só aparece no `--status`; o backup local não protege contra perda do disco.

### Imagens de produto em produção (IMG-02)

```bash
sudo bash deploy/gerar-imagens-producao.sh --dry-run      # só lista
sudo rotaperfumes-backup-banco --agora                    # o --confirmar exige backup de até 60 min
sudo bash deploy/gerar-imagens-producao.sh --confirmar    # --sobrescrever troca também as existentes
```

Roda na rede `rotaperfumes-db` (conta `rotaperfumes_admin`, uid 65532); a vitrine mostra em até 5 min (cache). Registro em `/root/rotaperfumes-mariadb/gerar-imagens.registro`.

### Atualizar e rollback

- Push na `main`: em até ~2 min o `pollSCM('H/2 * * * *')` roda o job (testa, constrói, valida a CA, migra, recria e faz o smoke). Só segredos: atualize `rotaperfumes-api-env` e rode o job.
- `make deps-audit` mensalmente e antes de cada deploy. Rollback: `git revert <commit>` + push (imagens são `:latest`).
- A `migracao.sql` não tem revert: problema de dados volta pelo backup (`docs/manual-base-de-dados.md`, 24.5). Banco (imagem, certificado, CA): `docs/Instal_SSL.md`, passo 10.

## 10. Troubleshooting

| Sintoma | Causa provável | Como resolver |
|---|---|---|
| Login aceita a senha, mas perde a sessão | Acesso por HTTP ou CA do Caddy não confiável | `https://ivo-inspiron-15-3530:8443` e seção 8 |
| Turnstile não aparece ou recusa | Hostname não cadastrado, acesso por IP, site key/secret errada | Seção 4; rode o job com a `TURNSTILE_SITE_KEY` certa |
| Site não abre pelo tunnel | Origem errada ou TLS verificado | 8443 com `noTLSVerify: true`; `curl -k https://192.168.168.106:8443/api/health` |
| `network rotaperfumes-db declared as external, but could not be found` | Banco não instalado | `sudo bash deploy/mariadb-docker.sh ...` (seção 3) |
| `Address already in use` para `172.30.42.10` | Outro container no IP fixo | `docker network inspect rotaperfumes-db` |
| API: `connection refused`, `no such host` | `rotaperfumes-mariadb` parado ou fora da rede | `docker ps`, `docker logs rotaperfumes-mariadb`, rode o script |
| API: `Access denied for user 'rotaperfumes_app'` | `DB_SENHA` diferente ou API fora do `.10` | Confira `rotaperfumes-api-env` e o compose |
| API: `DB_TLS_CA é obrigatório quando DB_HOST não é loopback` | SEC-42 | Defina `DB_TLS_CA`; nunca `DB_TLS_REQUIRED=false` |
| `s_client`: `Cipher is (NONE)` com `Verify return code: 0` | Host de origem sem conta (erro 1130) | Use `--ip 172.30.42.20` |
| `CA do banco`: "bind source path does not exist" / "nao legivel pelo uid 65532" / "nao e certificado" / "VENCIDO" / "NAO valida o certificado servido" | `ca.pem` ausente, sem leitura, com chave, vencido ou errado | `docs/Instal_SSL.md`, passos 2, 4, 5 e 9 |
| `CA do banco`: "sem fingerprint" / "fora de deploy/db-ca.fingerprint"; `mariadb-docker.sh` avisa CA de DEV | Arquivo vazio, CA trocada sem commit ou CA de dev | Seções 3.7 e 11 (passos 2 e 4) |
| `make db-*`: "DEPLOY-07 ... PRODUCAO", "SEC-46 ... mesmo com CONFIRMO_PRODUCAO", "SEC-46: DROP DATABASE ... CONFIRMAR=rotaperfumes", "SEC-54 ... allowlist" | Guardas da seção 3.8 | Confira o destino; `CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME>` (alvo não destrutivo), `CONFIRMAR=rotaperfumes`, ou banco descartável |
| `*** BUG-29` / `SEC-56` / `SEC-57` / `SEC-59` | `MAKEFLAGS` na linha de comando, `ENV_ARQUIVO` exportado, IPv4/porta inválidos, `.env` fora do formato | `make -k ...`, `unset ENV_ARQUIVO`, `a.b.c.d`, corrija as linhas citadas |
| `make setup`: `SEC-61: ... mariadb-keyring.pgp` | Keyring do MariaDB divergente | `sudo rm /etc/apt/keyrings/mariadb-keyring.pgp /etc/apt/sources.list.d/mariadb.sources && make setup` |
| Testes no Jenkins: "git ausente no agente (SEC-62)" ou pathspec | Sem `git` ou arquivo não commitado | Seção 2, item 3; commite o caminho |
| `migracao.sql`: "banco anterior a 2026-10-31" ou "senha_historico com created_at NULL" | Base antiga ou NULL no histórico | Nada foi alterado; restaure backup recente ou aplique o remédio do cabeçalho e rode de novo |
| `seed-producao.sh`: "sem terminal ..." ou "--forcar exige um terminal" | Sem TTY (BUG-26) | Terminal interativo ou `--pepper-file` |
| `seed-producao.sh`: "a tabela usuarios ja tem N linha(s)" | Banco já semeado | Não rode de novo; `--forcar` só em instalação sem uso |
| API: `x509: certificate signed by unknown authority` | API com a CA antiga | Commit do fingerprint e job (`docs/Instal_SSL.md`, 9.2) |
| API: `x509: ... not rotaperfumes-mariadb` | SAN antigo | `sudo bash deploy/mariadb-docker.sh --novo-cert --ca-key ...` |
| `Migracoes`: `Access denied` (1045) | Senha da `rotaperfumes-db-admin` | Seção 5.2 |
| Dev na 3306: erro 1130 ou timeout | Esperado: banco sem porta | `sudo rotaperfumes-dba` por SSH |
| `rotaperfumes-dba`: sessão já aberta | Container `rotaperfumes-dba-cliente` | `sudo rotaperfumes-dba --encerrar-sessao` |
| `client_ed25519 cannot be loaded` | Cliente sem ed25519 | Cliente `mariadb` 11.x |
| `mariadb-docker.sh`: "rode de novo com --recriar" / "--dev-ip saiu" / "difere do datadir" | Container divergente / opção removida / `--dados` diferente | Backup + `--recriar`; sem a opção; `--recriar --trocar-dados` |
| "nenhum backup" ou "tem mais de 60 min" | Guarda de backup | `sudo rotaperfumes-backup-banco --agora` ou `--sem-backup-motivo TEXTO` |
| Backup: "nao esta montado" / "volume ... nao existe" | Disco desmontado / volume ainda não criado | Monte o disco; rode o job; depois `--agora` |
| "o container parou" com escrita negada | `--read-only` incompatível | `--recriar --sem-read-only` |
| `permission denied ... docker.sock` | Jenkins sem grupo docker | Seção 2 |
| `port is already allocated` (8443) | Porta ocupada | Libere ou mude `HTTPS_PORT` e `CORS_ALLOWED_ORIGINS` |
| Smoke test falha | API caiu na subida | Veja as últimas linhas que o job imprime |
| `api.env` não encontrado em comando manual | Falta `API_ENV_FILE=/dev/null` | Seção 9 |

## 11. Instalação inicial da produção (DEPLOY-07)

> **A produção está no ar neste servidor: não repita estes passos aqui.** Um push na `main` é deploy em sistema vivo. Execução do usuário; nenhum agente tem acesso ao servidor.
>
> Cenário: MariaDB 11.4 novo e vazio, CA nova, sem porta publicada, seed com o admin (senha gerada, troca obrigatória) e os 42 vendedores reais inativos. Os dados do CasaOS de dev não vão para a produção.
> Ordem: enquanto faltar rede, container, fingerprint, seed ou credenciais, o estágio `CA do banco` recusa o build antes do `Deploy`.

| Passo | Onde | O quê |
|---|---|---|
| 1 | servidor | checkout do repositório |
| 2 | servidor | `mariadb-docker.sh --nova-ca`; anotar o `CA_FP` |
| 3 | servidor | chave da CA para mídia offline e `shred` |
| 4 | máquina de dev | `CA_FP` em `deploy/db-ca.fingerprint`, `make test`, commit (sem push) |
| 5 | servidor + Jenkins | `PASSWORD_PEPPER` novo, `rotaperfumes-api-env` e `rotaperfumes-db-admin` |
| 6 | servidor | `seed-producao.sh --pepper-file ...` |
| 7 | servidor | conferências SQL |
| 8 | todos | checklist pré-push |
| 9 | máquina de dev | push na `main` (deploy) |
| 10 | navegador | 1º login do admin e ativação dos vendedores |
| 11 | servidor | conta do DBA e rotina de backup (instalação já existente) |
| 12 | máquina com Docker (não o servidor) | testes reais dos scripts |

Nos passos 2, 3 e 5 a 7: no servidor, como root (`sudo -i`), `umask 077`, em `/root/rotaperfumes`.

### Passo 1. Código no servidor

```bash
sudo git clone --depth 1 https://github.com/residentivo/RotaPerfume.git /root/rotaperfumes
#   já clonado: sudo git -C /root/rotaperfumes pull --ff-only
```

Sem acesso ao GitHub, copie `deploy/`, `sql/` e `backend/shared/` (nunca `.env`, `.env.producao` ou `deploy/ca-cert.pem`). Um build disparado antes do passo 9 para no `CA do banco`, sem mudar nada.

### Passo 2. MariaDB de produção com CA nova

```bash
bash deploy/mariadb-docker.sh --nova-ca
docker ps --filter name=rotaperfumes-mariadb --format '{{.Names}} {{.Image}} {{.Status}}'
docker network inspect rotaperfumes-db -f '{{range .IPAM.Config}}{{.Subnet}} {{.IPRange}}{{end}}'   # 172.30.42.0/24 172.30.42.128/25
docker port rotaperfumes-mariadb      # nenhuma saída
ss -ltnp | grep ':3306 '              # nenhuma saída
```

- Gera `CN=RotaPerfumes_MariaDB_CA` (chave em `/root/rotaperfumes-ca/ca-key.pem`), o certificado do servidor, a rede, o container, o banco vazio e as contas. Se o script já rodou neste servidor, não use `--nova-ca`.
- **Anote o `CA_FP`** impresso no fim; os avisos de "NAO contem", CA nova e chave no servidor são esperados. Não copie esta CA para `deploy/ca-cert.pem` (é a de dev).

### Passo 3. Chave da CA para mídia offline

```bash
cp /root/rotaperfumes-ca/ca-key.pem /media/<pendrive-cifrado>/ca-key.pem
cp /etc/mysql/ssl/ca.pem /root/rotaperfumes-ca/server-ext.cnf /media/<pendrive-cifrado>/
cmp /root/rotaperfumes-ca/ca-key.pem /media/<pendrive-cifrado>/ca-key.pem && \
  shred -u /root/rotaperfumes-ca/ca-key.pem
```

Guarde uma segunda cópia. Para renovar o certificado: `--ca-key /media/<pendrive>/ca-key.pem` (`docs/Instal_SSL.md`, 9.1).

### Passo 4. Fingerprint da CA no repositório (máquina de dev; sem push)

```bash
sed -i '/^[0-9a-f]\{64\}$/d' deploy/db-ca.fingerprint          # só numa instalação do zero: tira a linha útil antiga
printf '%s\n' '<CA_FP do passo 2>' >> deploy/db-ca.fingerprint
grep -Ev '^[[:space:]]*(#|$)' deploy/db-ca.fingerprint        # uma linha: o CA_FP
env -u INTEGRATION make test
git add deploy/db-ca.fingerprint
git commit -m "DEPLOY-07: fingerprint da CA de producao"
```

Neste servidor, a linha `f31ee820...` é a CA em uso e não deve ser apagada. Nunca acrescente fingerprint de CA de dev.

### Passo 5. `PASSWORD_PEPPER` novo e credenciais do Jenkins

O pepper vai para o Secret file **e** para o seed, com o mesmo valor; depois de definido, nunca troque.

```bash
openssl rand -base64 48 | tr -d '\n' > /root/pepper-producao.txt
chmod 600 /root/pepper-producao.txt
A=/root/rotaperfumes-api.env
{
  grep -Ev '^(DB_SENHA|JWT_SECRET|PASSWORD_PEPPER)=' deploy/api.env.example
  printf 'DB_SENHA=%s\n' "$(cat /root/rotaperfumes-mariadb/rotaperfumes_app.senha)"
  printf 'JWT_SECRET=%s\n' "$(openssl rand -base64 48 | tr -d '\n')"
  printf 'PASSWORD_PEPPER=%s\n' "$(cat /root/pepper-producao.txt)"
  printf 'DB_TLS_SERVER_NAME=rotaperfumes-mariadb\n'
} > "$A"
chmod 600 "$A"
nano "$A"
```

- No editor: `TURNSTILE_SECRET_KEY` real, `SMTP_*` (necessário no passo 10), `SECURITY_ALERT_EMAILS`, `CORS_ALLOWED_ORIGINS`; sem `DB_TLS_REQUIRED=false`.
- Leve o arquivo à sua máquina (`install -m 600 -o <usuario> ...` + `scp`), cadastre `rotaperfumes-api-env` e `rotaperfumes-db-admin` (seção 5) e apague as cópias com `shred -u`. Mantenha o `pepper-producao.txt` até o passo 6.
- Não use o `setup-servidor.sh` com o `.env` de dev: ele copiaria o pepper de dev.

### Passo 6. Seed inicial (`deploy/seed-producao.sh`)

```bash
bash deploy/seed-producao.sh --pepper-file /root/pepper-producao.txt
```

- `--pepper-file`: arquivo regular do root, 600 ou 400, mesmo pepper do Secret file (≥ 32 caracteres); sem a opção, o pepper é pedido sem eco. Sem terminal, falha na etapa 1 sem alterar nada (BUG-26).
- Etapas `[n/7]`: (1) pré-requisitos, sem mudar nada; (2) recusa se `usuarios` tem linhas (`--forcar` exige dump recente e digitar o nome do banco); (3) `sql/schema.sql`; (4) `sql/seed_admin.sql`, `sql/seed_vendedores.sql` e `UPDATE usuarios SET ativo = 0 WHERE role <> 'admin'`.
- (5) compila o `resetpassword` em `GO_IMAGE` (precisa de internet); (6) `resetpassword -create-admin` na rede `rotaperfumes-db` (`172.30.42.20`, TLS), com env temporário 600 apagado por `shred`; (7) conferência.
- **Senha do admin:** sempre gerada (SEC-52; `--admin-senha-prompt` saiu), nunca impressa, em `/root/rotaperfumes-mariadb/admin-app.senha` (600; recusa se já existir) ou `--admin-senha-arquivo ARQ` (diretório privado do root). `deve_trocar_senha = 1`.
- Opções de isolamento (`--container`, `--rede-nome`, `--ip-admin`, `--segredos-dir`, `--ssl-dir`, `--trabalho-dir`) só para o teste real; no servidor, use os padrões. `/root` e `/root/rotaperfumes-mariadb` precisam ser do root e 700.
- Não roda `-all-users`: vendedores ficam com hash placeholder e inativos. Nunca use `make db-seed`, `db-reset` ou `fix-hash` aqui. Depois, guarde o pepper no cofre e `shred -u /root/pepper-producao.txt`.

### Passo 7. Conferências SQL

```bash
IMG='mariadb:11.4.13@sha256:5e73baf8179b29f115101bb102763a47a017361f3e9792daccba6646c3ab1532'   # = IMAGEM_BASE
admin_sql() {
  MYSQL_PWD=$(cat /root/rotaperfumes-mariadb/rotaperfumes_admin.senha) \
  docker run --rm -i -e MYSQL_PWD --network rotaperfumes-db --ip 172.30.42.20 \
    --cap-drop ALL --security-opt no-new-privileges:true \
    --mount type=bind,source=/etc/mysql/ssl/ca.pem,target=/ca.pem,readonly \
    --entrypoint mariadb "$IMG" --local-infile=0 -h rotaperfumes-mariadb -P 3306 -u rotaperfumes_admin \
      --ssl-ca=/ca.pem --ssl-verify-server-cert --default-character-set=utf8mb4 "$@"
}
admin_sql rotaperfumes <<'SQL'
SELECT COUNT(*) AS tabelas FROM information_schema.TABLES WHERE TABLE_SCHEMA = 'rotaperfumes';
SELECT id, email, role, ativo, deve_trocar_senha, LEFT(password_hash, 10) AS hash FROM usuarios WHERE role = 'admin';
SELECT COUNT(*) AS vendedores FROM vendedores;
SELECT COUNT(*) AS nao_admin, SUM(ativo <> 0) AS nao_admin_ativos FROM usuarios WHERE role <> 'admin';
SQL
```

Esperado: `tabelas` = 14 (as do `sql/schema.sql`); admin `admin@rotaperfumes.com.br` com `ativo` = 1, `deve_trocar_senha` = 1 e hash `$argon2id$`; `vendedores` = 43 (42 + "Loja Online", sem login); `nao_admin` = 42 e `nao_admin_ativos` = 0. Qualquer diferença: não faça o push.

### Passo 8. Checklist pré-push

- [ ] `rotaperfumes-mariadb` `Up` e `docker network inspect rotaperfumes-db` funciona;
- [ ] `/etc/mysql/ssl/ca.pem` legível por outros (`sudo -u nobody cat /etc/mysql/ssl/ca.pem >/dev/null && echo OK`);
- [ ] o fingerprint dele é a única linha útil do `deploy/db-ca.fingerprint` commitado, e não é de CA de dev;
- [ ] chave da CA fora do servidor (`test ! -e /root/rotaperfumes-ca/ca-key.pem && echo OK`);
- [ ] `rotaperfumes-api-env` com o mesmo pepper do seed, `JWT_SECRET` novo, Turnstile real, `SMTP_*`, `COOKIE_SECURE=true`; `rotaperfumes-db-admin` cadastrada;
- [ ] seed conferido (passo 7), job criado (seção 6), `make deps-audit` rodado e tudo que os testes usam commitado (SEC-62).

### Passo 9. Push na `main` (dispara o deploy)

```bash
git push origin main
docker logs rotaperfumes-api-1 2>&1 | grep -E 'TLS do banco|conexão MariaDB estabelecida'   # no servidor, depois do job
```

Estágios esperados até o `Smoke test` verde; `Migracoes` termina com `Migracoes concluidas.`. Se o `CA do banco` falhar, nada foi alterado (seção 10).

### Passo 10. Primeiro login do admin e ativação dos vendedores

1. Leia a senha no terminal (`sudo cat /root/rotaperfumes-mariadb/admin-app.senha`), entre com `admin@rotaperfumes.com.br` em `https://ivo-inspiron-15-3530:8443` e troque a senha em `/trocar-senha` (até lá, 403 nas outras rotas, SEC-23).
2. Apague o arquivo: `sudo shred -u /root/rotaperfumes-mariadb/admin-app.senha`.
3. Para cada vendedor que vai usar o sistema, na tela **Usuários**: **Resetar** (senha por e-mail, exige SMTP) e **ative** o usuário. Desligados (1, 3, 9, 30, 37, 38) continuam inativos; não use `make reset-senhas-ativos` nem `-all-users`.

### Passo 11. Conta do DBA e rotina de backup (instalação já existente)

Numa instalação anterior às contas `rotaperfumes_backup`/`rotaperfumes-admin`, em horário calmo: `sudo bash deploy/mariadb-docker.sh` (sem `--recriar`), conferir com `echo 'SELECT CURRENT_USER()' | sudo rotaperfumes-dba`, `sudo bash deploy/backup-banco.sh --instalar`.
Timer antigo `rotaperfumes-backup.timer`, se existir: `sudo systemctl disable --now rotaperfumes-backup.timer` e remova as units. Saída se algo falhar: `root@localhost` (seção 3.4).

### Passo 12. Testes reais dos scripts

Numa máquina com Docker que não seja o servidor, como root, na raiz do repositório:

```bash
cd backend/shared
sudo -E env "PATH=$PATH" TST15_DOCKER=1 go test -count=1 -run TestTST15_MariadbDocker_Real ./tests/deploy/
```

Sobe `tst15-mariadb` (rede `tst15-db`) com diretórios temporários em `/root/.tst15-*` e limpa tudo no fim. O `seed-producao.sh` tem o `TestTST17_SeedProducao_Real` (`TST17_DOCKER=1`, recusa o servidor de produção).

## 12. Restaurar em máquina nova (DEPLOY-09)

`deploy/restaurar-maquina.sh` devolve o site numa máquina **nova** (MariaDB, containers, dump, imagens, pepper, backup e `cloudflared`). No servidor em uso ele recusa: lá vale o runbook de `docs/manual-base-de-dados.md`, seção 24.5.
Antes: Docker com `compose`, `git`, `gnupg`, `curl`, IP fixo e clone limpo da `main` como root em `/root/rotaperfumes` (`umask 022`).

| Levar | Origem | Observação |
|---|---|---|
| `rotaperfumes-*.sql.gz` e `rotaperfumes-imagens-*.tar.gz` | cópia externa | mesmo carimbo (ou `--par-divergente-motivo TEXTO`) |
| Secret file `rotaperfumes-api-env` | cofre | root, 600 ou 400 |
| `PASSWORD_PEPPER` | cofre | `--pepper-file` ou digitado |
| `ca.pem` e `ca-key.pem` | mídia offline | sem a chave: `--nova-ca` |
| Token do túnel | painel da Cloudflare | `--token-file` ou digitado na etapa 11 |

```bash
sudo bash deploy/restaurar-maquina.sh --listar-manuais   # só nomes; não muda nada

sudo bash deploy/restaurar-maquina.sh \
  --dump /media/cofre/rotaperfumes-AAAAMMDD-HHMMSSZ.sql.gz \
  --imagens /media/cofre/rotaperfumes-imagens-AAAAMMDD-HHMMSSZ.tar.gz \
  --api-env /media/cofre/api.env \
  --ip IP_DA_LAN --host-nome NOME_NA_LAN --hostname-publico SITE_PUBLICO \
  --ca-cert /media/cofre/ca.pem --ca-key /media/cofre/ca-key.pem
```

- O `--api-env` precisa trazer as três origens (`https://SITE_PUBLICO`, `https://IP_DA_LAN:8443`, `https://NOME_NA_LAN:8443`) em `CORS_ALLOWED_ORIGINS` e, se houver, nos `TURNSTILE_HOSTNAMES`. `--sem-tunel` (ensaio) pula o `cloudflared`.
- **Recusas, antes de qualquer mudança:** sem root/terminal ou no Jenkins; clone alterado ou inseguro; sinais de produção ativa; `--ip` ausente na máquina; entradas inválidas (CA de dev, pepper diferente, `$` sem aspas, pouco espaço); banco com tabelas; `--ca-key` em `/root/rotaperfumes-ca`; DNS da LAN com túnel.

| # | O que faz | Para e pede |
|---|---|---|
| 0–1 | Recusas; copia e verifica dump e tar | |
| 2 | Mostra contagens, carimbo, IP, host, commit | `hostname`; com túnel, `ANTIGA PARADA` |
| 3 | `ca.pem` + `mariadb-docker.sh` (ignore o seed sugerido) | |
| 4 | `api.env` com `DB_SENHA` e `JWT_SECRET` novos | |
| 5 | Importa pela `rotaperfumes-admin` (`--sandbox`), compara contagens, `TRUNCATE refresh_tokens` | |
| 6 | Mostra `sudo rotaperfumes-dba < sql/migracao.sql` (leva o dump ao `schema.sql`) | `MIGRACOES CONFERIDAS` |
| 7 | `validar-ca-banco.sh`, `docker compose build --pull` e `up --no-start` | |
| 8 | Extrai as imagens como 65532 no volume vazio | |
| 9 | `up -d` e smoke | `LOGIN OK` (única prova do pepper) |
| 10 | `backup-banco.sh --instalar` e `--status` | |
| 11 | `cloudflared` com chave conferida e unit `rotaperfumes-cloudflared.service` endurecida; token em `/etc/rotaperfumes/cloudflared.env` | `PAINEL CONFERIDO` e o token |
| 12 | Lista do Jenkins e do Secret file | `JENKINS ATUALIZADO` |

- **Retomar:** rode o mesmo comando; o estado (`/root/rotaperfumes-restauracao/estado`) só vale com o mesmo `machine-id`, sha256 e opções. Estado `concluida` recusa sempre.
- **Cloudflare:** dois conectores com o mesmo token ficam ativos; com a máquina antiga perdida, além do "Rotate token", limpe as conexões (`DELETE` em `/connections`).
- **Chave do apt:** `deploy/cloudflare-apt.asc` e `.fingerprint`, confirmados por um segundo canal (keyserver.ubuntu.com e keys.openpgp.org); ao trocar, confirme por um segundo canal e mude os dois juntos.
- **IP ou hostname diferentes:** reemite o certificado (exige `--ca-key`) e o `--listar-manuais` lista os arquivos a commitar; com `--nova-ca`, commit do fingerprint depois de atualizar o Secret file.
- **Validado:** recusas, ordem, retomada e vazamentos com stubs (`deploy09_*_linux_test.go`); importação e extração com Docker real (`TestDEPLOY09_Real`). **Só em máquina real (ainda não feito):** ponta a ponta, `apt`, `cloudflared` e endurecimento da unit.
- Riscos: pepper errado só aparece no login da etapa 9; sem cópia externa não há o que restaurar; quem tem root lê os segredos temporários.
