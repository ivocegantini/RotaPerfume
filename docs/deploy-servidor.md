# Deploy no servidor da LAN (`ivo-inspiron-15-3530`) com Docker + Jenkins

> Card **DEPLOY-01** (Lote 13, 2026-09-30); banco em Docker desde o Lote 30 (DEPLOY-06, DOC-08, 2026-11-02); instalação inicial da produção reescrita no **DEPLOY-07** (Lote 33, 2026-11-04). Arquivos envolvidos: `Jenkinsfile`, `deploy/docker-compose.yml`, `deploy/Caddyfile`, `deploy/Dockerfile.caddy`, `deploy/api.env.example`, `deploy/mariadb-docker.sh`, `deploy/backup-mariadb.sh`, `deploy/seed-producao.sh`, `deploy/validar-ca-banco.sh`, `deploy/db-ca.fingerprint`, `deploy/dump-local.ps1`, `deploy/setup-servidor.sh`, `sql/ordem-ddl.txt`, `backend/Dockerfile`, `frontend/Dockerfile`. O antigo `deploy/mariadb-setup.sql` saiu do repositório (SEC-39).
>
> **Primeira instalação da produção?** Siga a **[seção 11 (Instalação inicial da produção, DEPLOY-07)](#11-instalação-inicial-da-produção-deploy-07)** **antes** do primeiro deploy da `main`: MariaDB 11.4 novo com CA própria, seed com o admin e os 42 vendedores inativos, fingerprint da CA commitado e credenciais do Jenkins. Sem a rede `rotaperfumes-db` e o container `rotaperfumes-mariadb`, o estágio `CA do banco` para o job (e o `docker compose up` falharia).
>
> **Dev x produção:** o banco de **desenvolvimento** é o MariaDB 11.8 do CasaOS, no mesmo servidor, porta 3307 (`192.168.168.106:3307`), com a CA de dev em `deploy/ca-cert.pem` (`backend/.env` padrão). A **produção** é o container `rotaperfumes-mariadb` (porta 3306, CA de produção, fingerprint em `deploy/db-ca.fingerprint`). As duas CAs nunca se misturam.
>
> **Mapa dos ambientes de banco** (servidor `192.168.168.106`):
>
> | Ambiente | Servidor | Banco | Conta | Configuração | Uso |
> | --- | --- | --- | --- | --- | --- |
> | Desenvolvimento | CasaOS, MariaDB 11.8, porta 3307 | `rotaperfumes` | `casaos`@`192.168.168.%` | `.env` padrão; CA `deploy/ca-cert.pem` | `make dev-api` e trabalho do dia a dia |
> | Teste (descartável) | CasaOS, porta 3307 | `rotaperfumes_teste` | `casaos` (GRANT só nesse nome) | o próprio `make test-integration` (TST-25: `DB_TESTE`, servidor do `.env`) | testes de integração; recriado e apagado a cada execução. Roteiros com `make db-*`: env temporário com `DB_NAME=rotaperfumes_teste` |
> | Produção | container `rotaperfumes-mariadb`, MariaDB 11.4, porta 3306 | `rotaperfumes` | contas do seed (§11) | `.env.producao` + `CONFIRMO_PRODUCAO` | API publicada; nunca testes, `db-down`/`db-seed`/`db-reset` |
>
> Permissão do banco de teste (uma vez, como root no MariaDB do CasaOS): ``GRANT ALL PRIVILEGES ON `rotaperfumes\_teste`.* TO 'casaos'@'192.168.168.%';`` (o `\_` evita o curinga do `_`).

## 0. Instalação automática (recomendado)

A instalação tem **duas partes**, nesta ordem:

1. **Banco:** `deploy/mariadb-docker.sh` (seção 3). É o **único** caminho oficial do banco em produção: cria a imagem, a CA e o certificado, a rede `rotaperfumes-db`, o container `rotaperfumes-mariadb` e as contas. TLS em detalhe: [`docs/Instal_SSL.md`](Instal_SSL.md).
2. **Aplicação e Jenkins:** `deploy/setup-servidor.sh`, que **não instala nem configura banco**. Ele confere que o container do banco está no ar, lê a senha do `rotaperfumes_app` gerada pelo passo 1 e faz o resto (Jenkins, firewall da 8443, dump, `api.env`, job, primeiro build).

As **duas credenciais do Jenkins** continuam sendo cadastradas **à mão**: o `setup-servidor.sh` não cria, não altera e não apaga credenciais. Ele pausa, mostra as instruções e depois só **confere** (por leitura) se elas existem.

**Antes:** faça commit e push dos arquivos de deploy para o `main`, gere o dump (seção 7.1), se houver dados a levar, e deixe o Jenkins com os plugins Pipeline, Git e Credentials Binding. O Jenkins pode ser **nativo** (serviço systemd `jenkins`) ou **em container** (DEPLOY-04; o da produção roda em container), com os requisitos da seção 2.

1. **No servidor, o banco** (uma vez; reexecutável):
   ```bash
   sudo bash deploy/mariadb-docker.sh --nova-ca                              # primeira instalação, sem CA
   # ou, com uma CA existente cuja chave está na mídia offline:
   sudo bash deploy/mariadb-docker.sh --ca-key /media/<pendrive>/ca-key.pem
   ```
   O banco não publica porta: o DBA entra por SSH no servidor, com `sudo rotaperfumes-dba` (seção 3.4). O `--dev-ip` e o `--porta` saíram (DEPLOY-11, seção 3.3). Numa instalação nova, o seed inicial é o `deploy/seed-producao.sh` (seção 11, passo 6), nunca o `make db-seed`.
2. **No Windows**, na raiz do projeto, copie o dump e o `.env` para o `/tmp` do servidor:
   ```powershell
   scp deploy\dumps\rotaperfumes-AAAAMMDD-HHMMSS.sql.gz .env SEU_USUARIO@ivo-inspiron-15-3530:/tmp/
   ```
3. **No servidor**, baixe e rode o script:
   ```bash
   curl -fsSLO https://raw.githubusercontent.com/residentivo/RotaPerfume/main/deploy/setup-servidor.sh
   sudo bash setup-servidor.sh --env /tmp/.env --dump /tmp/rotaperfumes-AAAAMMDD-HHMMSS.sql.gz \
     --turnstile-site-key SUA_SITE_KEY --remove-inputs
   ```
   Opções: `--site-host` (padrão `ivo-inspiron-15-3530`), `--site-ip` (`192.168.168.106`), `--https-port` (`8443`), `--jenkins-url` (`http://localhost:8888`), `--jenkins-container NOME` (padrão: detectado pela imagem `jenkins/*`; sem container = Jenkins nativo), `--db-container` (`rotaperfumes-mariadb`), `--db-segredos` (`/root/rotaperfumes-mariadb`), `--skip-build` e `--remove-inputs` (apaga o `.env` e o dump originais no final). As antigas `--apenas-banco` e `--skip-mysql` **saíram** (DEPLOY-06): o script recusa e aponta para o `mariadb-docker.sh`. Veja `bash setup-servidor.sh --help`.
4. O script executa 9 etapas e mostra o andamento como `[n/9]` (numeração contínua com Jenkins nativo ou em container):
   1. **Pré-checagens:** root via `sudo` (a partir do seu usuário), comandos (`docker`, `curl`, `openssl`, `gzip`, `install`, `getent`), `docker compose` ≥ 2.17, integridade do dump (`gzip -t`), Jenkins nativo ou em container e o **banco do `mariadb-docker.sh`**: container `rotaperfumes-mariadb` rodando, na rede `rotaperfumes-db`, e as senhas `rotaperfumes_app.senha`/`rotaperfumes_admin.senha` em `/root/rotaperfumes-mariadb`. Faltando algo, para e manda rodar o `mariadb-docker.sh`.
   2. **Jenkins com acesso ao Docker:** nativo, coloca o usuário `jenkins` no grupo `docker` (só reinicia o Jenkins se o grupo mudou); em container, só confere que ele alcança o Docker do host e tem o `docker compose` ≥ 2.17 (não altera o container).
   3. **Firewall (ufw):** só libera a `HTTPS_PORT` (8443) se o ufw estiver ativo; inativo, só avisa e sugere comandos (nunca ativa o ufw). O banco não passa pelo ufw: o script não publica a porta do MariaDB; a porta de uma instalação anterior é restringida na chain `DOCKER-USER` até o `--recriar` (o ufw não enxerga portas publicadas pelo Docker).
   4. **Dump:** Jenkins nativo, instala em `/opt/rotaperfumes/dumps/rotaperfumes.sql.gz` (`root:jenkins`, 640). Em container, usa esse caminho se ele estiver num bind do Jenkins; senão copia para `/var/jenkins_home/rotaperfumes-dumps/` (dono do Jenkins, 600) e ajusta o `DUMP_PATH` do build.
   5. **`api.env`:** gera `~/rotaperfumes-api.env` (600) a partir do seu `.env`. Define `DB_HOST=rotaperfumes-mariadb`, `DB_PORT=3306`, `DB_NAME=rotaperfumes`, `DB_USUARIO=rotaperfumes_app`, `DB_SENHA` (lida de `/root/rotaperfumes-mariadb/rotaperfumes_app.senha`, nunca impressa), um `JWT_SECRET` novo (48 bytes aleatórios em base64), `CORS_ALLOWED_ORIGINS`, `TRUST_PROXY_HEADERS=true` e `COOKIE_SECURE=true` (SEC-21: sempre `true` no servidor). As demais chaves, inclusive `PASSWORD_PEPPER`, vêm do seu `.env` (o pepper precisa ser o que gerou os hashes do dump). Avisa (só o nome da chave) de valores com `$` sem aspas simples, de `TURNSTILE_SECRET_KEY` vazia e de secret de teste do Turnstile.
   6. **Job Jenkins:** pede seu usuário e um **API token** (Jenkins → seu usuário → Security → API Token), confere os plugins e cria ou atualiza o job `rotaperfumes-deploy`.
   7. **Pausa para as credenciais manuais.** Siga as instruções da tela:
      1. No Windows: `scp SEU_USUARIO@ivo-inspiron-15-3530:rotaperfumes-api.env .`
      2. Jenkins → Gerenciar Jenkins → Credentials → System → Global credentials → **Add Credentials**. Kind **Secret file**, ID `rotaperfumes-api-env`, arquivo `rotaperfumes-api.env`.
      3. **Add Credentials** de novo. Kind **Username with password**, ID `rotaperfumes-db-admin`, usuário `rotaperfumes_admin`, senha = conteúdo de `/root/rotaperfumes-mariadb/rotaperfumes_admin.senha` (leia no servidor com `sudo cat`, sem colar em chat ou log).
      4. Apague o `rotaperfumes-api.env` do Windows.

      Tecle ENTER. O script confere as duas credenciais e, se estiverem cadastradas, apaga o `~/rotaperfumes-api.env` do servidor.
   8. **Primeiro build:** dispara com `IMPORTAR_DUMP=true` (quando há `--dump`) e mostra o log ao vivo. Se o build não terminar em `SUCCESS`, o script sai com erro.
   9. **Resumo:** URL de acesso, exporta a CA do Caddy para `~/rotaperfumes-caddy-root.crt` (instale no Windows como na seção 8), lembra do Turnstile e de apagar o dump.
5. **Reexecução:** os dois scripts podem ser rodados de novo. O `mariadb-docker.sh` não troca senhas (ficam nos arquivos). O `setup-servidor.sh` sempre gera um `api.env` com `JWT_SECRET` novo (derruba as sessões abertas): **atualize** a credencial no Jenkins (abra a credencial → **Update** → envie o arquivo novo). Ele só confere se as credenciais existem, não o conteúdo.
6. **Confira o `JWT_SECRET` do Secret file (SEC-18, Lote 24):** a API **não sobe** com `JWT_SECRET` menor que 32 bytes ou igual ao placeholder do `.env.example` (log `config: JWT_SECRET é obrigatório (mínimo de 32 bytes)`), nem com `JWT_TTL` menor ou igual a zero. O `api.env` gerado pelo script já atende. Se o Secret file foi criado antes do Lote 24 ou editado à mão, confira antes do próximo deploy (para gerar: `openssl rand -base64 48`). Confira também que `COOKIE_SECURE` não está `false` e que **não** há `DB_TLS_REQUIRED=false` (SEC-42).

As seções abaixo descrevem o mesmo processo **manualmente**, como alternativa ou para diagnóstico.

## 1. Visão geral

```
Navegador (LAN)                          Navegador (internet)
   │  https://ivo-inspiron-15-3530:8443       │ https://<dominio do tunnel>
   │                                          ▼
   │                                 cloudflared (no host)
   │                                          │ https://localhost:8443 (noTLSVerify)
   ▼                                          ▼
┌──────────── docker compose -p rotaperfumes ────────────┐
│  caddy (tls internal, 443 no container → 8443 no host) │
│     hosts: localhost, 127.0.0.1, SITE_HOST, SITE_IP     │
│            + curinga (qualquer Host, ex. o do tunnel)  │
│     ├── /api/health → api:8080/health                  │
│     ├── /api/*      → api:8080       (API Go)          │
│     └── resto       → frontend:3000  (Next standalone) │
└──────────────────────────┬─────────────────────────────┘
                           │ api 172.30.42.10 → rotaperfumes-mariadb:3306 (TLS 1.3, CA própria)
                           ▼
        rede Docker externa "rotaperfumes-db" (172.30.42.0/24)
        ┌─────────────────────────────────────────────────┐
        │ rotaperfumes-mariadb  172.30.42.2  (MariaDB 11.4)│
        │ cliente do import     172.30.42.20 (Jenkins)     │
        └─────────────────────────────────────────────────┘
        nenhuma porta nova; 192.168.168.106:3306 só até o --recriar da migração (DEPLOY-10)
```

- O **Jenkins** (`http://ivo-inspiron-15-3530:8888/`) clona `residentivo/RotaPerfume` (branch `main`) e roda o `Jenkinsfile`, com estes estágios: `Checkout` → `Testes Go` (vet + test dentro de `GO_TEST_IMAGE`, Debian fixada por digest, com bash e gpg; CI-02. Código por `git archive HEAD`, só o que está commitado; SEC-62) → `Testes Frontend` (TST-18: `npm ci`, `npm run lint` e `npm test` do `frontend/` dentro de `NODE_IMAGE`, fixada por digest; código por `git archive HEAD` no stdin, sem bind mount nem segredos; SEC-62) → `Build` (imagens do compose; só o `GOOGLE_CLIENT_ID` é lido do `api.env` para o bundle do frontend, DEPLOY-10) → `CA do banco` (valida a CA do banco no host contra `deploy/db-ca.fingerprint` e contra o certificado servido pela rede interna, seção 3.5) → `Importar dump` (só com `IMPORTAR_DUMP=true`) → `Migracoes` (DEPLOY-10: `sql/27` a `31` e `33`, idempotentes, com a conta admin pelo mesmo cliente do `Importar dump`) → `Deploy` (`up -d`) → `Smoke test` (`/api/health` e `/login` pelo Caddy, acessando direto `https://SITE_IP:HTTPS_PORT`, e `/api/health` no mesmo endereço com um Host qualquer, como faz o tunnel).
- A API e o frontend **não** publicam porta no host. Só o Caddy publica a `HTTPS_PORT` (8443), que atende a LAN, `https://localhost:8443` no próprio servidor e o **Cloudflare Tunnel**. O banco não publica porta (a 3306 antiga só até o `--recriar` da migração, DEPLOY-10).
- **Cloudflare Tunnel:** no `cloudflared` do host, aponte o hostname público para a porta 8443 do servidor (hoje `https://192.168.168.106:8443`) com **No TLS Verify** ligado (`noTLSVerify: true`), porque o certificado é da CA interna do Caddy. Hosts desconhecidos caem no site curinga do Caddy. O Caddy só aceita o header `CF-Connecting-IP` (IP real do visitante) de conexões vindas do próprio servidor (o `SITE_IP` ou o gateway docker `172.16.0.0/12`). Cadastre também o domínio do tunnel no Turnstile.
- **HTTPS é obrigatório:** os cookies de autenticação são `Secure` (`COOKIE_SECURE=true`, o default; desde o SEC-21 a decisão não depende mais do `Host` da requisição). Por HTTP o login "funciona", mas a sessão não se mantém. (Pelo tunnel o navegador fala HTTPS com a Cloudflare, então funciona.)
- O frontend e a API ficam na **mesma origem**. O frontend chama `/api/...` com URL relativa (`PUBLIC_URL` padrão `/`, embutido no bundle **no build**), então o mesmo build serve a LAN e o domínio do tunnel.

### Parâmetros do job (definidos no `Jenkinsfile`)

| Parâmetro | Padrão | Uso |
|---|---|---|
| `SITE_HOST` | `ivo-inspiron-15-3530` | Hostname de acesso na LAN (Caddy) |
| `SITE_IP` | `192.168.168.106` | IP da LAN (o Caddy também aceita esse IP; usado no smoke test) |
| `HTTPS_PORT` | `8443` | Porta HTTPS publicada pelo Caddy |
| `TURNSTILE_SITE_KEY` | `0x4AAAAAAFACjc-LtM9NWlvP` | Site key **pública** do Turnstile, embutida no bundle |
| `IMPORTAR_DUMP` | `false` | Importa o dump no MariaDB do container `rotaperfumes-mariadb`. **Sobrescreve as tabelas** |
| `DUMP_PATH` | `/opt/rotaperfumes/dumps/rotaperfumes.sql.gz` | Caminho do dump visível para o agente Jenkins |

**Constantes do `environment` (não são parâmetros):** `DB_TLS_CA_HOST_PATH=/etc/mysql/ssl/ca.pem` (SEC-40; caminho absoluto no **host**), `DB_REDE=rotaperfumes-db`, `DB_HOST_INTERNO=rotaperfumes-mariadb`, `DB_HOST_PORT=3306` (porta interna do container), `DB_CLIENTE_IP=172.30.42.20`, `GO_IMAGE`, `GO_TEST_IMAGE`, `NODE_IMAGE` e `DB_CLIENT_IMAGE` (fixadas por digest, SEC-31, seção 3.6). Os antigos parâmetros `DB_TLS_CA_HOST_PATH` e `DB_TLS_SERVER_NAME` saíram: o nome conferido no certificado é sempre `rotaperfumes-mariadb`.

### Credenciais Jenkins exigidas

| ID | Tipo | Conteúdo |
|---|---|---|
| `rotaperfumes-api-env` | Secret file | `.env` de produção da API (modelo: `deploy/api.env.example`), com `DB_USUARIO=rotaperfumes_app` e a senha de `/root/rotaperfumes-mariadb/rotaperfumes_app.senha` |
| `rotaperfumes-db-admin` | Username with password | `rotaperfumes_admin` + senha de `/root/rotaperfumes-mariadb/rotaperfumes_admin.senha` (usada pelo `Importar dump` e pelo `Migracoes`, DEPLOY-10) |

---

## 2. Pré-requisitos no servidor

Rode no servidor (via SSH ou no terminal local dele):

1. Instale o Docker Engine e o plugin compose. O compose precisa ser **≥ 2.17** (checado pelo `setup-servidor.sh`). O build funciona com o builder clássico ou com BuildKit; a imagem do Caddy usa `deploy/Dockerfile.caddy`:
   ```bash
   docker --version
   docker compose version     # precisa ser v2.17 ou superior
   ```
   O banco não publica porta, então o script não precisa de `iptables` nem da chain `DOCKER-USER` (DEPLOY-11). Só se encontrar a unit `rotaperfumes-db-firewall` antiga ele usa `iptables` e `systemctl`, para removê-la (seção 3.3).
2. **Jenkins com acesso ao Docker do host:**
   - **Nativo:** coloque o usuário do Jenkins no grupo `docker` e **reinicie o Jenkins** (sem o restart, o processo continua sem o grupo novo):
     ```bash
     sudo usermod -aG docker jenkins
     sudo systemctl restart jenkins
     sudo -u jenkins docker ps          # deve listar sem "permission denied"
     ```
   - **Em container (produção):** monte o socket `/var/run/docker.sock`, instale na imagem o `docker-ce-cli` e o `docker-compose-plugin` (≥ 2.17) e dê ao usuário do Jenkins acesso ao socket. Confira com `docker exec <container_jenkins> docker ps` e `docker exec <container_jenkins> docker compose version`. Os caminhos montados pelo pipeline (`/etc/mysql/ssl/ca.pem`) são resolvidos pelo daemon do **host**: o container do Jenkins não precisa enxergá-los.
3. Confirme as ferramentas usadas pelo pipeline (no agente): `bash`, `curl`, `git`, `gzip`/`zcat`, `install` (coreutils) e `openssl`.
   ```bash
   which bash curl git gzip zcat install openssl
   ```
   O `git` é obrigatório (SEC-62): sem ele, os estágios de teste falham com "git ausente no agente (SEC-62)". Um caminho da lista do `git archive` que não esteja rastreado também derruba o estágio. **Tudo que os testes usam precisa estar commitado.**
4. No Jenkins, confirme que os plugins **Pipeline**, **Git** e **Credentials Binding** estão instalados (vêm nos "suggested plugins").
5. Confirme que a porta **8443** está livre:
   ```bash
   sudo ss -ltnp | grep ':8443' || echo "8443 livre"
   ```

---

## 3. Banco: MariaDB em Docker (`deploy/mariadb-docker.sh`)

O banco do projeto é o **MariaDB 11.4 LTS** (DB-03), em container criado **fora** do compose pelo `deploy/mariadb-docker.sh` (DEPLOY-06). O `setup-servidor.sh` não instala nem configura banco (sem drop-in, `sql_mode` ou `systemctl mysql` no host).

> **Driver Go:** a API continua usando o `go-sql-driver/mysql`, que é compatível com o MariaDB (mesmo protocolo) e suporta ed25519. Os pacotes `<ctx>mysql` mantêm o nome.

### 3.1 Instalar ou atualizar

```bash
sudo bash deploy/mariadb-docker.sh [--ca-key <mídia>/ca-key.pem | --nova-ca] [--recriar]
```

Etapas (`[n/8]`):

1. **Pré-requisitos:** root, `docker`/`openssl`/`sha256sum`, formato da `--imagem-base`, IPs e caminhos. Uma sessão do DBA aberta (container `rotaperfumes-dba-cliente`) recusa a execução, sem mudar nada. Se o container já existe com rede, IP, portas, `--read-only`, usuário ou datadir diferentes do desejado, **para** pedindo `--recriar` (nada foi mudado). Com `--recriar`, um `--dados` diferente do datadir atual é recusado, salvo `--recriar --trocar-dados` (BACKUP-07): o novo caminho precisa já ter dados (`mysql/`), a guarda do backup sempre dispara e o `recriar.registro` ganha `dados_antes=` e `dados=`.
2. **Imagem** `rotaperfumes-mariadb:11.4-<12 hex do digest>`: `FROM` a imagem base fixada por digest (SEC-31) + `zz-rotaperfumes.cnf` (TLS obrigatório, `tls_version = TLSv1.2,TLSv1.3`, `plugin_load_add = auth_ed25519`, `local_infile = 0`, `sql_mode` estrito, `utf8mb4_unicode_ci` com `character_set_collations`, `skip_name_resolve`). A chave privada **não** entra na imagem.
3. **Certificados** em `/etc/mysql/ssl` (`--ssl-dir`; CA própria com a chave fora do servidor; ver [`docs/Instal_SSL.md`](Instal_SSL.md)). A chave de uma CA gerada no servidor (`--nova-ca`) e o `server-ext.cnf` ficam em `/root/rotaperfumes-ca` (`--ca-dir`), onde também fica o temporário da emissão (TST-16; antes era `/root/rotaperfumes-ssl.*`).
4. **Senhas** em `/root/rotaperfumes-mariadb/` (`--segredos-dir`; 700; arquivos 600, só root): `root.senha`, `rotaperfumes_app.senha`, `rotaperfumes_admin.senha`, `rotaperfumes_backup.senha` e `dba-cliente/conta.senha`. Geradas uma vez (32 caracteres `[A-Za-z0-9]`); a reexecução **não** troca senha. Nada é impresso. **`casaos.senha` (DEPLOY-10):** nunca é criado, lido nem apagado por script, e a senha não é reaproveitada. Se existir, o script avisa `mantido; leve ao pendrive seguro`: mova o arquivo para a mídia cifrada, à mão, quando decidir.
5. **Rede `rotaperfumes-db` e firewall** (seções 3.2 e 3.3).
6. **Container** `rotaperfumes-mariadb`: só na rede `rotaperfumes-db` com IP `172.30.42.2`, `--user` do `mysql` da imagem, `--cap-drop ALL`, `no-new-privileges`, `--read-only` (com `tmpfs` em `/run/mysqld` e `/tmp`; `--sem-read-only` só se não subir), `restart unless-stopped`, datadir em `/var/lib/rotaperfumes/mariadb`, `MARIADB_ROOT_HOST=localhost`, `MARIADB_AUTO_UPGRADE=1`, logs `json-file` 5 × 10 MB.
7. **Banco `rotaperfumes` e contas** (seção 3.4), convergidas a cada execução.
8. **Teste de conexão TLS:** cliente descartável em `172.30.42.20` conecta com `--ssl-verify-server-cert`; uma conexão `--skip-ssl` aceita **aborta** o script. **DEPLOY-10:** instala `/usr/local/sbin/rotaperfumes-dba` (root, 700; só com o container padrão) e testa por ele: `SELECT CURRENT_USER()` precisa dar `rotaperfumes-admin@172.30.42.22`, e o `--skip-ssl` dessa conta precisa ser recusado. Confere também que o certificado **em uso** pelo `mariadbd` é o de `/etc/mysql/ssl/server-cert.pem` (mesma validade). As contas `healthcheck@localhost`, `@127.0.0.1` e `@::1`, criadas pelo entrypoint da imagem oficial (só `USAGE`), são esperadas; o script só avisa se tiverem mais que `USAGE`.

No fim, imprime o fingerprint da CA (formato do `deploy/db-ca.fingerprint`), onde está cada senha e os próximos passos. Avisa se a chave da CA ainda estiver no servidor.

**Diretórios (TST-16, Lote 36):** `--segredos-dir DIR` e `--ca-dir DIR` trocam os padrões `/root/rotaperfumes-mariadb` e `/root/rotaperfumes-ca` (no servidor, use os padrões: o `deploy/seed-producao.sh` só lê `/root/rotaperfumes-mariadb`; as opções são para o teste real). Na etapa 1, antes de qualquer efeito, os quatro diretórios (`--ssl-dir`, `--dados`, `--segredos-dir`, `--ca-dir`) precisam ser absolutos, sem `..`, espaços ou vírgulas, e disjuntos (nenhum igual ou dentro de outro: a chave da CA e as senhas nunca dentro do `--ssl-dir`, que é 755 e montado no container); segredos e CA que já existem precisam ser diretório real (não symlink), do root e com modo `700`; o script não corrige o modo, falha. **Lote 36 (SecBrain, TOCTOU):** a disjunção compara os caminhos reais (`realpath -m`), não o texto; em `--segredos-dir`/`--ca-dir` nenhum componente pode ser symlink (o `realpath -m` precisa ser igual ao informado; `//` e `/./` também são recusados) e cada ancestral existente, até `/`, precisa ser do root e sem escrita para grupo e outros (o `/tmp` 1777 é recusado: o sticky não basta). A checagem (com `-L` e dono/modo `0 700`) roda de novo logo depois do `install -d`, antes de gravar a chave da CA e as senhas. Os padrões em `/root/...` passam (`/root` 700 e `/` 755, do root).

**`ONLY_FULL_GROUP_BY` ligado (DB-07, Lote 36, 2026-11-05).** O `sql_mode` do `zz-rotaperfumes.cnf` é `ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES,NO_ZERO_IN_DATE,NO_ZERO_DATE,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION`: um `GROUP BY` incompleto dá erro 1055 (`ER_WRONG_FIELD_WITH_GROUP`) em vez de valores arbitrários. Antes (DB-04/DB-06) o modo ficava desligado; ele foi ligado depois de a série de vendas agrupar pelo alias (`GROUP BY data`) e de a suíte de integração passar num MariaDB 11.4 descartável com o modo ligado no global. O `TestIntegracaoDB07_GroupByComOnlyFullGroupBy` (`backend/shared/tests/analytics`) liga o modo na **sessão** e cobre os `GROUP BY` do `analyticsmysql` (funil, clientes por segmento/UF, série, metas/vendas do mês e ranking), em qualquer banco de teste. **Instalação existente:** o modo novo só vale depois de regravar a imagem e recriar o container (`sudo rotaperfumes-backup-banco --agora` e `sudo bash deploy/mariadb-docker.sh --recriar`); sem isso o script só avisa que o container usa outra imagem. O MariaDB de dev do CasaOS (3307) não é configurado por este script: para alinhar, `SET GLOBAL sql_mode` com o mesmo valor (e no cnf dele, para sobreviver ao restart).

### 3.2 Rede `rotaperfumes-db`

- `docker network create --subnet 172.30.42.0/24 --gateway 172.30.42.1 --ip-range 172.30.42.128/25`, idempotente (o script aborta se a rede já existir com outra subnet; `--rede-docker` muda a faixa).
- IPs fixos: **MariaDB `.2`**, **API `.10`** (`ipv4_address` no `docker-compose.yml`, rede declarada `external: true`), **admin/import `.20`** (cliente do estágio `Importar dump` e do teste do script). IPs dinâmicos só em `.128-.255`, para nada ocupar os fixos.
- A API fala com `rotaperfumes-mariadb:3306` (nome do container, que está no SAN); `DB_HOST`, `DB_PORT`, `DB_TLS_CA` e `DB_TLS_SERVER_NAME` são fixados no `environment` do compose e ganham do `api.env`.
- **A rede e o container precisam existir antes do `docker compose up`**: sem a rede externa o compose falha (`network rotaperfumes-db declared as external, but could not be found`).

### 3.3 Sem porta publicada (DEPLOY-10/11)

- **Nenhuma** porta publicada. Um container que ainda publique a 3306 diverge, e o script pede `--recriar`.
- **DEPLOY-11 (2026-11-18):** a 3306 foi fechada no servidor (`--recriar` sem `--dev-ip`). O `--dev-ip` e o `--porta` saíram e são recusados, como o `--rede-lan`.
- **Firewall antigo:** se a unit `rotaperfumes-db-firewall.service` ou o `/usr/local/sbin/rotaperfumes-db-firewall` ainda existirem (só com o container padrão), o script os remove, junto com a chain `ROTAPERFUMES-DB` e o salto em `DOCKER-USER`. Isso acontece depois de o container estar no ar sem porta publicada; se ele publicar porta, o script para e o firewall fica.
- Confira depois: `sudo iptables -S ROTAPERFUMES-DB` dá erro de chain inexistente, e `systemctl status rotaperfumes-db-firewall` dá unit não encontrada.

### 3.4 Contas e senhas

Todas `IDENTIFIED VIA ed25519` + `REQUIRE SSL` (SEC-30/SEC-39), criadas e alteradas pelo stdin do cliente (nunca no argv):

| Conta | Host | Privilégios | Senha | Usado por |
|---|---|---|---|---|
| `rotaperfumes_app` | `172.30.42.10` | `SELECT, INSERT, UPDATE, DELETE ON rotaperfumes.*` | `rotaperfumes_app.senha` | API: `DB_SENHA` do Secret file `rotaperfumes-api-env` |
| `rotaperfumes_admin` | `172.30.42.20` | `ALL ON rotaperfumes.*` | `rotaperfumes_admin.senha` | Jenkins, pela credencial `rotaperfumes-db-admin`, e `seed-producao.sh`. **Não é o DBA** |
| `rotaperfumes_backup` | `172.30.42.21` | `SELECT ON rotaperfumes.*` (BACKUP-01) | `rotaperfumes_backup.senha` | rotina de backup, pelo option file `backup-cliente/rotaperfumes_backup.cnf` (400, root; `docs/manual-base-de-dados.md`, seção 24) |
| `rotaperfumes-admin` | `172.30.42.22` | `ALL ON rotaperfumes.*`, sem privilégio global, `GRANT OPTION`, `PROCESS` ou acesso a `mysql.*` (DEPLOY-10) | `dba-cliente/conta.senha` | DBA, por SSH no servidor: `sudo rotaperfumes-dba`, com o option file `dba-cliente/cliente.cnf` (400, root). **Não é o `rotaperfumes_admin`** |
| `root` | `localhost` | (todos) | `root.senha` | só `docker exec` (socket); saída se o comando do DBA falhar |

**`casaos_ro` (SEC-51/SEC-59, removida no MERGE-01, 2026-11-17):** a conta de consultas da máquina .105 saiu junto com a porta da LAN (decisão do usuário). O `mariadb-docker.sh` a remove em qualquer host, como a `casaos`; consultas agora só no servidor, com `sudo rotaperfumes-dba`.

A cada execução: `CREATE USER IF NOT EXISTS` + `ALTER USER` (converge plugin, senha e TLS), `REVOKE ALL` + `GRANT` (converge privilégios), `DROP USER` de toda conta do projeto com host fora do desejado, remoção de contas anônimas e do banco `test`, e aviso de contas desconhecidas. Root no banco: `sudo docker exec -it rotaperfumes-mariadb mariadb --defaults-extra-file=/run/rotaperfumes/root.cnf`.

`rotaperfumes-admin` = DBA, `.22`; não é `rotaperfumes_admin` = Jenkins e seed, `.20`.

**Comando do DBA (`/usr/local/sbin/rotaperfumes-dba`, gerado pelo script):**

```bash
sudo rotaperfumes-dba                    # interativo: --safe-updates e prompt "PRD rotaperfumes-admin> "
sudo rotaperfumes-dba < arquivo.sql      # por stdin: --batch, para no primeiro erro
sudo rotaperfumes-dba > saida.txt        # RECUSADO: stdin no terminal com a saída redirecionada
sudo rotaperfumes-dba --encerrar-sessao  # remove o cliente rotaperfumes-dba-cliente, se ficou aberto
```

- Nenhum argumento é repassado ao cliente. O cliente é um container descartável, com `--read-only`, `--cap-drop ALL` e `--log-driver none`, sem histórico. A senha fica só no option file montado read-only.
- Interativo só com stdin **e** stdout no terminal; stdin no terminal com a saída redirecionada é recusado. O `docker run` usa `--pull never`: só a imagem local do script.
- Cada sessão registra data e `SUDO_USER` no syslog (`logger -t rotaperfumes-dba`), sem SQL. Se o `logger` falhar, o evento vai para `/var/log/rotaperfumes-dba.log` (root, 600, sem seguir link simbólico), com aviso na tela.
- `DROP DATABASE rotaperfumes` continua possível por esta conta: confira o que vai rodar.

**Removidos no DEPLOY-10:** a conta `casaos` em qualquer host (o `DROP USER` não derruba sessão aberta: o script só avisa) e a opção `--casaos-native-motivo`, agora recusada; um `casaos-native.motivo` existente fica como histórico. A `casaos` sai em **qualquer** host, inclusive com caractere fora de `[A-Za-z0-9.:/%_-]`: o `DROP USER` é gerado no próprio servidor, com `QUOTE(User)`/`QUOTE(Host)`, e o script para se ainda sobrar alguma linha da `casaos` (Lote 42). A conta e o CasaOS de dev (`:3307`) não mudam.

### 3.5 Como o pipeline usa a CA do banco (SEC-29/SEC-40)

A CA **não** fica no repositório nem passa pelo workspace do Jenkins. Ela fica **no host**, em `DB_TLS_CA_HOST_PATH` (constante `/etc/mysql/ssl/ca.pem`, modo 644; ver `docs/Instal_SSL.md`, passo 5), e quem a monta é o **daemon Docker do host**. Por isso funciona com o Jenkins em container. O que o repositório guarda é só o **fingerprint** aceito (`deploy/db-ca.fingerprint`, seção 3.7).

A cada build, o estágio **`CA do banco`**:

1. confere que `deploy/db-ca.fingerprint` existe e imprime os fingerprints aceitos;
2. roda `deploy/validar-ca-banco.sh` (enviado por stdin) num container descartável de `DB_CLIENT_IMAGE` (traz `bash` e `openssl`), como **uid 65532** (o da API distroless), na rede `rotaperfumes-db`, com o arquivo do host em `--mount type=bind,...,readonly` (caminho inexistente é erro, em vez de virar diretório). Se o uid 65532 não conseguir ler, a API também não conseguiria, e o job para;
3. o script recusa qualquer bloco PEM que não seja `CERTIFICATE` (uma chave privada não pode ser montada na API), separa **cada** certificado e valida todos: parse, `-checkend 0` (vencido = falha), aviso de 30 dias e de `CA:TRUE`, subject e fingerprint no log, e **todo** certificado precisa estar em `deploy/db-ca.fingerprint` (SEC-40; um fora da lista = falha);
4. **confere a CA contra o servidor**, antes de parar a API ou importar: `openssl s_client -starttls mysql -connect rotaperfumes-mariadb:3306 -CAfile <CA> -verify_return_error -verify_hostname rotaperfumes-mariadb`. O container de validação usa `--ip 172.30.42.20` (conta `rotaperfumes_admin`; de um IP sem conta o `mariadbd` responde com o erro 1130 antes do greeting) e exige handshake real (`Cipher is` ≠ `(NONE)`, certificado do peer presente) além do `Verify return code: 0 (ok)` (DEPLOY-08). Se a CA não validar o certificado servido hoje para esse nome, o job para e nada é alterado;
5. grava o sha256 do arquivo em `deploy/db-ca.sha256` (apagado no `post`).

No estágio **Deploy**, o pipeline exporta `DB_CA_SHA256` e o compose o grava na label `rotaperfumes.db-ca-sha256` do serviço `api`. O `docker compose up -d` não recria o container quando só o conteúdo de um arquivo montado muda (e o bind fica preso ao inode antigo quando o arquivo é substituído); com a label, CA nova significa container novo, já com a CA atual.

O estágio **`Importar dump`** roda o cliente `mariadb` de `DB_CLIENT_IMAGE` com `--network rotaperfumes-db --ip 172.30.42.20`, `-h rotaperfumes-mariadb -P 3306`, `--ssl-ca=/etc/rotaperfumes/db-ca.pem --ssl-verify-server-cert`, `--local-infile=0` e a senha por `MYSQL_PWD` (nunca no argv).

Na API, `DB_TLS_CA` definido liga o TLS verificado com **TLS 1.3 mínimo** (TST-12); arquivo ilegível ou sem certificado derruba a subida (`config: DB_TLS_CA inválido ...`), e CA errada, vencida ou nome fora do SAN derrubam a conexão (`x509: ...`); não há queda para texto claro. Sem `DB_TLS_CA` e com `DB_HOST` fora do loopback, a API não sobe (SEC-42, `ErrDBTLSObrigatorio`), salvo `DB_TLS_REQUIRED=false`, que **nunca** deve ir para produção. O log de subida mostra `[server] TLS do banco: ligado (ca=..., server_name=rotaperfumes-mariadb, min=TLS1.3)` e `[db] conexão MariaDB estabelecida (host=<DSN mascarado>, tls=ligado (custom))`.

**Rotação** (certificado e CA): `docs/Instal_SSL.md`, passo 9. Resumo: certificado novo com a mesma CA (`--novo-cert`, `FLUSH SSL`) não exige nada no pipeline; CA nova (`--nova-ca`) exige o fingerprint novo em `deploy/db-ca.fingerprint` e um deploy.

### 3.6 Atualizar a imagem do MariaDB (digest, SEC-31)

As imagens base são fixadas pelo digest do **índice multi-arch** (a tag sozinha pode ser republicada). A imagem do MariaDB aparece em **seis** lugares, com a **mesma string** (testes conferem): `IMAGEM_BASE` no `deploy/mariadb-docker.sh`, no `deploy/backup-banco.sh`, no `deploy/verificar-backup.sh` e no `deploy/restaurar-maquina.sh`, e `DB_CLIENT_IMAGE` no `Jenkinsfile` e no `deploy/seed-producao.sh` (mais as cópias de referência `IMG=` deste documento e do `docs/Instal_SSL.md`). O `GO_IMAGE` e o `NODE_IMAGE` do `Jenkinsfile` seguem o mesmo formato (`golang:<patch>-alpine@sha256:...` com a versão do `go.mod`; `node:<patch>-alpine@sha256:...` com a versão do `frontend/.nvmrc`, conferida por teste). Para eles basta trocar a string no `Jenkinsfile` (`docker buildx imagetools inspect node:26.13-alpine --format '{{.Manifest.Digest}}'`) e rodar `make test`; os passos 2 e 4 a 6 abaixo são só do MariaDB.

**SEC-47 (Dockerfiles):** `backend/Dockerfile` (`ARG GO_VERSION` + `ARG GO_DIGEST` = `GO_IMAGE`; distroless `nonroot@sha256:`), `frontend/Dockerfile` (`ARG NODE_VERSION` + `ARG NODE_DIGEST` = `NODE_IMAGE`) e `deploy/Dockerfile.caddy` (`caddy:<patch>-alpine@sha256:`) também ficam fixados; o `make deps-audit-imagens` resolve os `ARG` do `FROM` e avisa `FROM` sem digest. Com `tag@digest` o Docker usa só o digest: versão e digest mudam **juntos** (testes conferem a igualdade com o `Jenkinsfile`).

**`GO_TEST_IMAGE` (CI-02):** `golang:<patch>-trixie@sha256:...`, Debian com bash, gpg, tar e git, usada só no estágio `Testes Go`. Tem a mesma versão do `GO_IMAGE` e não recebe `apk`/`apt`. Para atualizar, obtenha o digest com `docker buildx imagetools inspect golang:1.26-trixie --format '{{.Manifest.Digest}}'` (sem buildx, por `curl` ao registry ou `docker pull`), troque a string no `Jenkinsfile` e rode `make test`. Os testes com `unshare` continuam pulando no container (seccomp padrão do Docker, limite aceito) e só rodam na máquina local.

1. Veja se há digest novo: `make deps-audit-imagens` (só informa, nunca falha; usa `docker buildx imagetools` ou o registry do Docker Hub via `curl`). Ou, numa máquina com Docker:
   ```bash
   docker buildx imagetools inspect mariadb:11.4 --format '{{.Manifest.Digest}}'
   ```
   e anote a tag do patch (ex.: `11.4.13`).
2. Troque `mariadb:<patch>@sha256:<digest>` no `IMAGEM_BASE` de `deploy/mariadb-docker.sh`, `deploy/backup-banco.sh` e `deploy/verificar-backup.sh` **e** no `DB_CLIENT_IMAGE` do `Jenkinsfile` e de `deploy/seed-producao.sh`.
3. `make test` (os testes de `tests/deploy` conferem o formato e a igualdade das strings).
4. Depois do merge, no servidor, atualize a cópia instalada, que usa a imagem antiga até isso: `sudo bash deploy/backup-banco.sh --instalar`.
5. Gere o backup que a guarda do `--recriar` exige (até 60 minutos) e confira-o: `sudo rotaperfumes-backup-banco --agora` e `sudo bash deploy/verificar-backup.sh <dump>`.
6. `sudo bash deploy/mariadb-docker.sh --recriar`. O banco fica fora do ar alguns segundos; os dados ficam no datadir. Sem `--recriar`, o script só avisa que o container usa uma imagem diferente.

Histórico: **DEP-06 (Lote 40, 2026-11-14)** subiu para `golang:1.27.4-alpine` (`0a5f7fe7…`) e `node:26.13.1-alpine` (`6bd9ab43…`, LTS 26). **DEP-05 (Lote 39, 2026-11-14)** trocou os digests de `mariadb:11.4.13` (`5e73baf8…`) e `node:22.23.4-alpine` (`0e175475…`), rebuilds das mesmas tags.

Só suba **patch** do 11.4 assim. Mudança de versão menor/maior é card próprio (backup, teste em banco descartável).

### 3.7 Pinagem da CA (`deploy/db-ca.fingerprint`, SEC-40)

- Conteúdo: SHA-256 do DER de cada certificado aceito no arquivo da CA do host, hex minúsculo sem `:`, um por linha (`#` e linhas vazias ignoradas). Normal: **um**; durante uma troca de CA, **dois** (o antigo sai num commit seguinte).
- Calcular: `openssl x509 -in /etc/mysql/ssl/ca.pem -noout -fingerprint -sha256 | cut -d= -f2 | tr -d : | tr A-F a-f` (o `mariadb-docker.sh` imprime o mesmo valor no fim).
- Toda troca de CA no servidor exige um commit revisado desse arquivo; sem ele, o estágio `CA do banco` barra o deploy e a API continua com a CA anterior. Isso impede que um arquivo trocado no host (por engano ou por um atacante) passe a ser a CA confiada pela API.
- Só a CA de **produção** entra na lista (DEPLOY-07). O `deploy/ca-cert.pem` é a CA de **dev** (MariaDB do CasaOS, porta 3307, `CN=MariaDB_CA`, fingerprint `d150ee67...`; SEC-71, 2026-11-06) e **não** pode estar no arquivo, nem a CA de dev anterior (`CN=RotaPerfumes_MySQL_CA`, fingerprint `ef31b656...`): os testes (`TestSEC45_FingerprintVersionadoConfereComCA`, `TestDEPLOY07_CADevNuncaEmProducao`) garantem que elas não estão, e o `mariadb-docker.sh` avisa se encontrar uma dessas linhas. Hoje a linha útil é a da CA de produção em uso (`f31ee820...`); numa instalação do zero, com CA nova, o `CA_FP` a substitui (§11, passo 4). Sem nenhuma linha útil, o estágio `CA do banco` falha com `deploy/db-ca.fingerprint sem fingerprint: rode mariadb-docker.sh --nova-ca e grave o CA_FP (DEPLOY-07)`.

### 3.8 Regra: integração só em banco descartável

- `INTEGRATION=1`, `make db-down`, `make db-reset`, `make db-rebuild`, `make db-seed` e `make db-import-*` **apagam ou sobrescrevem dados**. Só rode contra um banco **descartável**: um MariaDB 11.4 local em container (`DB_HOST=127.0.0.1`, `.env` separado) ou o container temporário do `TestTST15_MariadbDocker_Real`.
- **Nunca** contra o banco de produção, que tem dados reais.
- **Guarda `db-guard-producao` (DEPLOY-07):** todo alvo do `Makefile` que escreve no banco (`db-create`, `db-up`, `db-seed`, `db-down`, `db-reset`, `db-rebuild`, `db-fix-*`, `db-revert-*`, `db-import-*`, `db-seed-mes-atual`, `db-gerar-imagens-produtos`, `gen-hash`, `fix-hash`, `fix-admin`, `reset-senhas-ativos`, `test-integration`) depende dela. A guarda olha **host e porta**: é produção `DB_HOST` = `192.168.168.106` ou `ivo-inspiron-15-3530` (`PRODUCAO_HOSTS_LAN`) com `DB_PORT` vazio ou `3306`, ou `DB_HOST` = `rotaperfumes-mariadb` ou `172.30.42.2` (`PRODUCAO_HOSTS_REDE`, rede Docker) em qualquer porta. O MariaDB de dev do CasaOS, no mesmo host, porta `3307` (`192.168.168.106:3307`), **passa**. **SEC-54:** antes de comparar, o `DB_HOST` é normalizado sem DNS (espaços, minúsculas, `[ ]` das pontas, todos os `.` finais, um sufixo `.local` e IPv4 mapeado em IPv6, `::ffff:a.b.c.d` → `a.b.c.d`); qualquer outro literal IPv6 que não seja `::1` e porta não numérica contam como produção. Assim `ivo-inspiron-15-3530.local.` e `[::ffff:192.168.168.106]` são recusados. **SEC-56 (Lote 37):** IPv4 em forma numérica não canônica (decimal `3232278634`, hex `0xc0a8a86a`, octal `0300.0250.0250.0152`, curta `192.168.43114`/`127.1`, octeto com zero à esquerda `127.000.000.001`) é recusado **sem válvula** (nem com `CONFIRMO_PRODUCAO`), com a mensagem `SEC-56: DB_HOST=... e IPv4 em forma numerica nao canonica`; use `a.b.c.d`. O loopback `127.x.x.x` só conta com octetos canônicos. **SEC-57 (Lote 37):** `DB_PORT` fora de `+` opcional e dígitos (inclusive espaço no fim, `DB_PORT="3307 "` na linha de comando) é recusado sem válvula (`SEC-57: DB_PORT='...' invalida`). O `DB_PORT` do `.env` tem os espaços das pontas tirados; o da linha de comando só perde os do início (o próprio make tira). Outro IP da mesma máquina continua fora da lista (risco residual aceito: o destrutivo fora do loopback já exige `CONFIRMAR`). Em produção o alvo é recusado, salvo `CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME>` **na linha de comando** (no `.env` ou no ambiente não vale). O seed de produção é o `deploy/seed-producao.sh`, nunca `make db-seed`.
- **SEC-46:** `db-down`, `db-reset`, `db-rebuild` e `db-seed` também dependem de `db-guard-sem-producao`, que os recusa na produção **mesmo com** `CONFIRMO_PRODUCAO` (sem válvula; recriar a produção é `rotaperfumes-backup-banco --agora` + `mariadb-docker.sh --recriar` + `seed-producao.sh` no servidor). E `db-down`/`db-reset`/`db-rebuild` dependem de `db-guard-drop`: com `DB_NAME=rotaperfumes` (sem diferenciar maiúsculas) e `DB_HOST` fora do loopback (ex.: o dev do CasaOS, `192.168.168.106:3307`), o `DROP DATABASE` exige `CONFIRMAR=<DB_NAME>` **na linha de comando** (ex.: `make db-reset CONFIRMAR=rotaperfumes`; no `.env` ou no ambiente não vale). Loopback (`localhost`, `127.x.x.x`, `::1`, depois da normalização do SEC-54; nome `.local` nunca conta como loopback) ou outro nome de banco (ex.: `rotaperfumes_teste`) seguem sem pergunta. **BUG-27:** `db-reset` e `db-rebuild` têm só as três guardas como pré-requisitos e rodam as etapas em cadeia (`$(MAKE) db-down && $(MAKE) db-seed && $(MAKE) db-import-...`): com `make -k` ou `-j`, um DROP recusado ou com erro para tudo (nenhum `CREATE DATABASE`/seed/import) e cada etapa só começa depois da anterior. As guardas rodam de novo em cada etapa (o AVISO pode aparecer mais de uma vez). `CONFIRMAR`/`CONFIRMO_PRODUCAO` da linha de comando chegam às etapas; no `.env` ou no ambiente continuam sem valer. **BUG-28 (Lote 37):** o `db-seed` depende só das guardas e do `db-check-env` e roda o `db-up` (CREATE + DDL) na receita, por sub-make: com `make -k`/`-j` o schema não é criado junto de uma guarda que falhou. **BUG-29 (Lote 37):** `ENV_ARQUIVO`, `CONFIRMAR` e `CONFIRMO_PRODUCAO` chegam às etapas pelo `MAKEFLAGS`; por isso o `make` recusa no parse (antes de qualquer guarda ou cliente): `MAKEFLAGS=`, `MAKEOVERRIDES=` ou `GNUMAKEFLAGS=` **na linha de comando** (use `make -k ...` ou `MAKEFLAGS=-k make ...` no ambiente), `ENV_ARQUIVO` vindo do ambiente (`export ENV_ARQUIVO=...`), `ENV_ARQUIVO` fora de `[A-Za-z0-9._/-]` e `.env`/`ENV_ARQUIVO` que declare `MAKEFLAGS`, `MAKEOVERRIDES`, `GNUMAKEFLAGS`, `MAKEFILES`, `MAKELEVEL`, `MAKE`, `RP_ENV_PAI` ou `ENV_ARQUIVO_LIDO`. Cada sub-make recebe a sentinela `RP_ENV_PAI=<arquivo lido pelo pai>` e aborta se fosse ler outro arquivo.
- **SEC-59 (Lote 39):** o `.env` (ou o `ENV_ARQUIVO`) só aceita linhas vazias, comentários e `[export ]NOME [:?+]=valor` literal, conferidos antes do `include`. O `make` recusa no parse qualquer outra linha (`override`, `define`, `include`, `!=`, regras), linha terminada em `\` e `$` fora de comentário, inclusive num `#` depois do valor. A mensagem traz só os números das linhas, nunca o conteúdo. Efeitos colaterais: valor com `$` e `.env` com BOM UTF-8 são recusados. Também são proibidas `SHELL`, `MAKESHELL`, `MAKECMDGOALS`, `CURDIR`, `VPATH`, `PATH` e `VARS_DO_ENV`, além das do BUG-29. `ENV_ARQUIVO` com `-` inicial é recusado (use `./-foo`).
- **SEC-50:** os testes Go de integração (helper `backend/shared/testkit/integracao`) recusam a produção **antes de conectar**, com a mesma regra de host e porta, também quando o `go test` com `INTEGRATION=1` é chamado direto. **SEC-54:** a integração tem ainda uma allowlist (`INTEGRACAO_PERMITIDOS` no `Makefile`, guarda `db-guard-integracao`; `integracao.Permitidos` no Go): só loopback (`localhost`, `127.0.0.0/8`, `::1`, qualquer porta) e o CasaOS de dev `192.168.168.106:3307`; qualquer outro destino falha (`t.Fatal`). **SEC-53:** `make test`/`lint`/`build` e os alvos `npm` não recebem mais as variáveis do `.env` (só `make test-integration` passa as `DB_*` ao `go test`). **SEC-55:** só as linhas `go test` (`$(SEM_SEGREDOS_TESTE)`) recebem as `DB_*` no modo integração; `go build`, `go tool cover`, `go vet`, `arch-check` e `npm` usam o `$(SEM_SEGREDOS)` estrito, nem com `INTEGRATION=1`.
- Os agentes da equipe também seguem a regra: nenhum `make db-*` nem `INTEGRATION=1` com o `.env` atual.

---

## 4. Cloudflare Turnstile

1. No painel do Cloudflare (Turnstile → seu site), cadastre em **Hostnames** o hostname de acesso: `ivo-inspiron-15-3530`.
2. Anote a **Site Key**, que é pública e vai no parâmetro `TURNSTILE_SITE_KEY` do job, e a **Secret Key**, que vai em `TURNSTILE_SECRET_KEY` no Secret file.
3. **Acesse sempre pelo hostname**, e não pelo IP. O widget só valida nos hostnames cadastrados.
4. Se o PC cliente não resolver `ivo-inspiron-15-3530`, adicione esta linha em `C:\Windows\System32\drivers\etc\hosts` (como administrador):
   ```
   192.168.168.106   ivo-inspiron-15-3530
   ```
5. **Use a secret real.** Desde o SEC-37 (Lote 29), a API **não sobe** em produção (`COOKIE_SECURE=true`) com uma secret de **teste** da Cloudflare (`1x…`, `2x…`, `3x…` seguidas de zeros). O log mostra `config: TURNSTILE_SECRET_KEY é uma secret de teste da Cloudflare e não é aceita com COOKIE_SECURE=true ...`, porque com ela o captcha aceita qualquer token e não confere hostname nem action. Se o Cloudflare não aceitar o hostname da LAN e você **precisar** das chaves de teste (site `1x00000000000000000000AA`, secret `1x0000000000000000000000000000000AA`), existe uma válvula: `TURNSTILE_PERMITIR_SECRET_TESTE=true` no Secret file. **Aviso:** com ela o login fica **sem captcha** (só o rate limit protege contra força bruta), e a API loga a cada subida `[config] SEGURANCA: secret de teste do Turnstile em producao (TURNSTILE_PERMITIR_SECRET_TESTE=true): captcha desligado`. Use só nesta rede interna, sabendo do risco, e remova a válvula assim que tiver a secret real.

---

## 5. Credenciais no Jenkins

Caminho: **Gerenciar Jenkins → Credentials → System → Global credentials (unrestricted) → Add Credentials**.

### 5.1 Secret file `rotaperfumes-api-env`

1. Numa máquina de confiança, copie `deploy/api.env.example` para um arquivo temporário (ex.: `api.env`) **fora do repositório**.
2. Preencha:
   - `DB_USUARIO=rotaperfumes_app` e `DB_SENHA` = conteúdo de `/root/rotaperfumes-mariadb/rotaperfumes_app.senha` (leia no servidor com `sudo cat`). `DB_HOST`/`DB_PORT`/`DB_TLS_CA`/`DB_TLS_SERVER_NAME` são fixados pelo compose. **Não** inclua `DB_TLS_REQUIRED=false` (SEC-42).
   - `JWT_SECRET` = valor **novo**, diferente do ambiente local:
     ```bash
     openssl rand -base64 48
     ```
     Com menos de 32 bytes, ou com o placeholder do exemplo, a API não sobe (SEC-18, Lote 24).
   - `COOKIE_SECURE=true` (SEC-21, Lote 24; é o default se a chave faltar). Nunca `false` no servidor: com `false` os cookies de sessão saem sem `Secure`.
   - `TURNSTILE_SECRET_KEY` = secret key **real** do Turnstile (secret de teste impede a subida, SEC-37; ver seção 4, item 5).
   - `PASSWORD_PEPPER` = pepper das senhas (SEC-13). Use **o mesmo do `.env` que gerou os hashes do dump importado**: com outro valor, nenhuma senha Argon2id do dump funciona (hashes bcrypt antigos continuam valendo e migram no próximo login). Num banco novo, gere com `openssl rand -base64 48`. Sem ele, ou com menos de 32 bytes, a API não sobe. Depois de definido, **não troque**.
   - `SMTP_*` e `SECURITY_ALERT_EMAILS`, se for enviar e-mails. Se ficarem vazios, os e-mails vão só para o log.
   - Se mudar `SITE_HOST`/`HTTPS_PORT`, ajuste também o `CORS_ALLOWED_ORIGINS`.
   - Se algum valor tiver `$`, coloque-o entre aspas **simples**. Isso vale só para este Secret file; o `.env` lido pelo `Makefile` recusa `$` em qualquer valor (SEC-59, seção 3.8).
3. Em **Add Credentials**: Kind **Secret file**, envie o arquivo e informe o ID **`rotaperfumes-api-env`** (exatamente assim).
4. **Apague** o arquivo preenchido da máquina local.

O pipeline grava esse arquivo em `deploy/api.env` (modo 600) só durante o deploy e o apaga no final (`post { always }`).

### 5.2 Username with password `rotaperfumes-db-admin`

1. Kind **Username with password**.
2. Username `rotaperfumes_admin`, Password = conteúdo de `/root/rotaperfumes-mariadb/rotaperfumes_admin.senha`.
3. ID **`rotaperfumes-db-admin`**.

### 5.3 Credencial do GitHub (só se o repositório for privado)

Kind **Username with password**: o usuário do GitHub e um **Personal Access Token** (escopo de leitura do repositório) como senha. O ID é livre (ex.: `github-residentivo`).

---

## 6. Criar o job

1. **Novo item** → nome `rotaperfumes-deploy` → tipo **Pipeline** → OK.
2. Seção **Pipeline**:
   - Definition: **Pipeline script from SCM**
   - SCM: **Git**
   - Repository URL: `https://github.com/residentivo/RotaPerfume.git`
   - Credentials: a do passo 5.3 (se o repositório for privado)
   - Branch Specifier: `*/main`
   - Script Path: `Jenkinsfile`
3. Salve.
4. Antes de rodar, faça **commit e push** dos arquivos de deploy para o `main`. O Jenkins só enxerga o que está no GitHub.

> Na **primeira** execução, o Jenkins ainda não conhece os parâmetros: o botão é "Construir agora" e ele roda com os valores padrão (sem Turnstile e sem importar o dump). Pode deixar rodar ou abortar. Depois dela, aparece **"Construir com parâmetros"**.

---

## 7. Dump de um banco MariaDB de origem e primeira carga

Use esta seção só se for preciso levar dados de **outro MariaDB** para o servidor. A instalação inicial da produção (seção 11) **não** importa dump: o banco nasce com o seed do `deploy/seed-producao.sh`, e os dados do MariaDB do CasaOS são de desenvolvimento. Importar um dump **sobrescreve** as tabelas de produção (dados reais): faça backup antes (seção 9).

1. **No Windows**, na raiz do projeto:
   ```powershell
   powershell -ExecutionPolicy Bypass -File deploy\dump-local.ps1
   ```
   O script lê `DB_*` e `DB_TLS_CA` do `.env` da raiz e roda **só o `mariadb-dump`** (SEC-41; sem fallback; procura `-DumpExe`, o `PATH` e `C:\Program Files\MariaDB *\bin`, e confere que o `--version` diz MariaDB). Com `DB_TLS_CA`, usa `--ssl-ca=<caminho a partir da raiz> --ssl-verify-server-cert`; sem `DB_TLS_CA`, só aceita `DB_HOST` loopback (`localhost`, `127.x`, `::1`), sem TLS e com aviso, e **aborta** para host remoto. A senha vai por `MYSQL_PWD`. Grava `deploy\dumps\rotaperfumes-AAAAMMDD-HHMMSS.sql.gz` (a pasta é ignorada pelo git) e apaga o `.sql` intermediário.
2. **No servidor**, prepare a pasta. O Jenkins precisa conseguir **ler** o arquivo (com o Jenkins em container, o `setup-servidor.sh` cuida disso; ver seção 0, etapa 4):
   ```bash
   sudo mkdir -p /opt/rotaperfumes/dumps
   sudo chown "$USER":jenkins /opt/rotaperfumes/dumps
   sudo chmod 750 /opt/rotaperfumes/dumps
   ```
3. **No Windows**, copie o dump com scp:
   ```powershell
   scp deploy\dumps\rotaperfumes-AAAAMMDD-HHMMSS.sql.gz SEU_USUARIO@192.168.168.106:/opt/rotaperfumes/dumps/rotaperfumes.sql.gz
   ```
   Depois, no servidor:
   ```bash
   chmod 640 /opt/rotaperfumes/dumps/rotaperfumes.sql.gz
   ```
4. No Jenkins, **Construir com parâmetros**:
   - `TURNSTILE_SITE_KEY` = site key do passo 4
   - `IMPORTAR_DUMP` = **marcado**
   - demais parâmetros no padrão
5. O estágio **Importar dump**:
   - valida o gzip;
   - para o container da API;
   - confere que o estágio `CA do banco` validou a CA;
   - importa com o cliente `mariadb` de `DB_CLIENT_IMAGE`, pela rede `rotaperfumes-db` com IP `172.30.42.20`, em `rotaperfumes-mariadb:3306`, com `--local-infile=0 --ssl-ca=... --ssl-verify-server-cert` e o usuário da credencial `rotaperfumes-db-admin`;
   - roda `TRUNCATE TABLE refresh_tokens`, porque as sessões do ambiente de origem não valem no servidor.

   Em seguida vêm `Deploy` e `Smoke test`.
6. Com o build verde, **apague o dump** do servidor e da máquina local:
   ```bash
   rm /opt/rotaperfumes/dumps/rotaperfumes.sql.gz
   ```
   ```powershell
   Remove-Item deploy\dumps\*.sql.gz
   ```
7. Nos próximos deploys, deixe `IMPORTAR_DUMP` **desmarcado**. Se marcar de novo, o dump **sobrescreve** os dados do servidor.

---

## 8. Primeiro acesso e certificado (CA do Caddy)

O Caddy usa `tls internal`, ou seja, uma CA própria guardada no volume `caddy_data`. Até a raiz dessa CA ser instalada no cliente, o navegador mostra um aviso de certificado.

1. **No servidor**, exporte a raiz da CA:
   ```bash
   docker cp rotaperfumes-caddy-1:/data/caddy/pki/authorities/local/root.crt ./rotaperfumes-caddy-root.crt
   ```
   Alternativa com compose, rodada dentro de `deploy/` no workspace do job (ex.: `/var/lib/jenkins/workspace/rotaperfumes-deploy/deploy`):
   ```bash
   API_ENV_FILE=/dev/null docker compose -p rotaperfumes cp caddy:/data/caddy/pki/authorities/local/root.crt .
   ```
2. Copie o `.crt` para o PC Windows. Por exemplo, com o scp rodado **no Windows**:
   ```powershell
   scp SEU_USUARIO@192.168.168.106:~/rotaperfumes-caddy-root.crt .
   ```
3. **No Windows**: clique duas vezes no `.crt` → **Instalar Certificado** → **Máquina Local** (ou Usuário Atual) → **Colocar todos os certificados no repositório a seguir** → **Autoridades de Certificação Raiz Confiáveis** → Concluir.
   - O Firefox usa um repositório próprio. Importe em Configurações → Privacidade e Segurança → Certificados → Autoridades, ou ative `security.enterprise_roots.enabled`.
4. Feche e reabra o navegador e acesse **https://ivo-inspiron-15-3530:8443**. O cadeado deve aparecer sem aviso.
5. Faça login com um usuário do dump.

> A CA vive no volume `caddy_data`. **Não** rode `docker compose down -v` nem `docker volume rm rotaperfumes_caddy_data`: isso gera uma CA nova e obriga a reinstalar o certificado em todos os clientes.

---

## 9. Operações do dia a dia

Os comandos manuais rodam **no servidor**, dentro de `deploy/` no workspace do job, **sempre** com `API_ENV_FILE=/dev/null`. O `api.env` só existe durante o build e, sem essa variável, o compose falha de propósito.

```bash
cd /var/lib/jenkins/workspace/rotaperfumes-deploy/deploy
```

| Ação | Comando |
|---|---|
| Status | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes ps` |
| Logs (todos, seguindo) | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes logs -f --tail 100` |
| Logs da API | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes logs -f --tail 200 api` |
| Logs sem compose | `docker logs -f --tail 200 rotaperfumes-api-1` (ou `-frontend-1`, `-caddy-1`) |
| Logs do banco | `docker logs -f --tail 200 rotaperfumes-mariadb` |
| Parar (mantém containers) | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes stop` |
| Religar containers parados | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes start` |
| Remover (mantém o volume da CA) | `API_ENV_FILE=/dev/null docker compose -p rotaperfumes down` |

- **Nunca** rode `up` manualmente com `API_ENV_FILE=/dev/null`, porque a API seria recriada sem as variáveis. Para subir ou recriar, **rode o job**.
- O banco **não** faz parte do compose: `down` não o afeta. Para recriá-lo, use `sudo rotaperfumes-backup-banco --agora` e `sudo bash deploy/mariadb-docker.sh --recriar`.
- Os containers têm `restart: unless-stopped` e voltam sozinhos após um reboot do servidor. A exceção é se tiverem sido parados com `stop`.
- Os logs rodam em `json-file` com até 5 × 10 MB por serviço.

### Backup do banco e das imagens de produto

O backup é automático (BACKUP-01): um timer systemd roda às 03:30 (com até 15 min de atraso aleatório) e, se o servidor estava desligado, roda ao ligar. Cada backup bom é um **par** com o mesmo carimbo UTC, em `/var/backups/rotaperfumes` (root, 700):

- `rotaperfumes-AAAAMMDD-HHMMSSZ.sql.gz`: dump, com a conta só `SELECT` `rotaperfumes_backup@172.30.42.21`.
- `rotaperfumes-imagens-AAAAMMDD-HHMMSSZ.tar.gz`: volume `rotaperfumes_produto_imagens`, lido sem rede e somente leitura.

O `--agora` usa a série `-manual` nos dois nomes. **Isso não recupera o site sozinho:** veja o que fica fora em `docs/manual-base-de-dados.md`, seção 24.2. A restauração está nas seções 24.5 e 24.6 do mesmo manual.


**Rotina antiga (`backup-mariadb.sh`, MERGE-01):** antes do passo 3 da instalação abaixo, desligue e remova o timer antigo, que chamava o script removido:

```bash
sudo systemctl disable --now rotaperfumes-backup.timer
sudo rm -f /etc/systemd/system/rotaperfumes-backup.timer /etc/systemd/system/rotaperfumes-backup.service
sudo systemctl daemon-reload
```

Os dumps antigos em `/var/backups/rotaperfumes-mariadb/` ficam onde estão (a rotina nova grava em `/var/backups/rotaperfumes/`); apague-os à mão quando não precisar mais.

**Instalação numa produção já ativa, nesta ordem e em horário calmo:**

1. Atualize o checkout do servidor (`/root/rotaperfumes` ainda não é checkout git: card DEPLOY-08, item 6 da limpeza).
2. Crie as contas de backup e do DBA, **sem `--recriar`** (migração completa na seção 11, passo 11). O container não reinicia, mas há um `REVOKE` + `GRANT` rápido nas contas e a `casaos` sai:
   ```bash
   sudo bash deploy/mariadb-docker.sh
   ```
3. Instale a rotina. O volume de imagens precisa existir (ele nasce no 1º deploy). O 1º backup roda pela unit, e o comando falha se ele falhar:
   ```bash
   sudo bash deploy/backup-banco.sh --instalar
   ```
4. Confira:
   ```bash
   sudo rotaperfumes-backup-banco --status
   sudo bash deploy/verificar-backup.sh /var/backups/rotaperfumes/rotaperfumes-AAAAMMDD-HHMMSSZ.sql.gz
   ```
5. Só depois rode o `--recriar` do item 7 da limpeza, com um `sudo rotaperfumes-backup-banco --agora` logo antes. Nesta ordem, a guarda abaixo não trava a própria instalação.

**Comandos:**

| Comando | Efeito |
| --- | --- |
| `sudo rotaperfumes-backup-banco --agora` | backup na série `-manual`, com retenção própria |
| `sudo rotaperfumes-backup-banco --agora --aceitar-reducao` | aceita **uma vez** um dump com menos de 50% do anterior; fica em `.registro-reducao` |
| `sudo rotaperfumes-backup-banco --status` | sai com erro se o último par tiver mais de 26 h ou data futura (vale o carimbo do nome e o mtime, com 5 min de folga), se o dump (`gzip -t` e marcador na última linha) ou o tar de imagens for reprovado, se o timer estiver inativo ou se a última execução falhou; avisa se a última verificação tiver mais de 35 dias ou data futura |
| `sudo bash deploy/backup-banco.sh --conferir-imagens ARQ` | confere um tar de imagens e imprime só contagens; não usa rede, credencial nem Docker |
| `sudo bash deploy/backup-banco.sh --instalar` | instala ou atualiza a cópia, as units e o conf |

**O que você deve ver (BACKUP-02):** com o dump aprovado, o `--agora` e o `--agendado` (no `journalctl`) mostram `backup: dump validado: marcador '-- Dump completed' na ultima linha: OK`, e o `--status` mostra `Dump validado: marcador '-- Dump completed' na ultima linha: OK`; sem essa linha, o dump foi reprovado ou nem chegou a ser conferido, e ela só fala do dump: o resultado final é `backup: ok ...` ou `STATUS: ok`.

O root executa só a cópia `/usr/local/sbin/rotaperfumes-backup-banco` (700, root), nunca o checkout: um push na `main` não vira código rodando como root. Por isso, depois de atualizar o script ou o digest, rode `--instalar` de novo. A exceção é `--conferir-imagens`, que só lê o arquivo.

**Retenção:** 14 pares diários e 3 manuais, por contagem de pares bons e por série, nunca por data. Para mudar, use `--instalar --retencao-diarios N --retencao-manuais N` (mínimo 2); o valor fica em `/etc/rotaperfumes/backup-banco.conf`. A retenção só roda depois de um par novo publicado, então disco cheio nunca apaga backup. Ela **falha sem apagar nada**, com o par novo intacto, se houver par com data futura, se fosse apagar o próprio par novo ou se o novo, um par que fica na série ou um dos dois pares mais novos estiver corrompido (`gzip -t` nos dois arquivos e marcador no dump). A mensagem nomeia o arquivo: confira-o e remova-o à mão, se for o caso. Até lá, o backup continua sendo gravado, a retenção não roda e a execução termina com erro. Antes de apagar, relê todos os pares que ficam na série e os dois mais novos (BACKUP-05); os que saem não são relidos, e sem nada a apagar nada é relido. Par corrompido: mova o dump e as imagens para fora de `/var/backups/rotaperfumes`. Ela não toca em nomes parecidos, symlinks, subdiretórios, `pre-restauracao-*` nem `parcial-*`.

**Espaço:** antes de começar, exige livre ≥ 2× o dump anterior + o maior entre 2× as imagens anteriores e o tamanho atual do volume + 1 GiB. O backup divide o disco com o banco: encher o disco derruba o banco.

**Quando as imagens falham:** o dump já validado é guardado como `parcial-rotaperfumes-*.sql.gz`, e a execução termina com erro. Esse arquivo **não conta como backup bom**: não dispara a retenção nem satisfaz o `--status`; a guarda do `--recriar` e do `seed --forcar` o aceita, com `AVISO` (BACKUP-04), mas o `gerar-imagens-producao.sh` e o `restaurar-maquina.sh` não. Ele fica para uso manual, e só os 3 mais novos são mantidos. Volume ausente é falha. Volume vazio é aceito com `AVISO`. A regra dos 50% vale só para o dump, porque remover produto é legítimo; tamanho e contagem de arquivos ficam no log (`journalctl -u rotaperfumes-backup-banco.service`), que nunca traz linhas do dump nem nomes do tar.

**Guarda do `--recriar` e do `seed-producao.sh --forcar`:** com dados existentes, exigem um dump de até 60 minutos (`--backup-janela-min N` muda a janela; acima de 60, só com `--sem-backup-motivo TEXTO`, que justifica a janela maior sem dispensar a conferência do dump), regular, root, 600, sem data futura (folga de 300 s para ajuste de relógio, BACKUP-03), com `gzip -t` e marcador na última linha. O mais novo é escolhido pelo carimbo; só `pre-restauracao-*` não vale; `parcial-*` vale, com aviso (no mesmo carimbo vence o completo). Se o mais novo falhar, recusa sem recuar para um mais antigo. Exigem só o dump porque não tocam no volume de imagens. Para dispensar (ex.: container parado, que não gera dump novo), use `--sem-backup-motivo TEXTO`, com 10 a 300 caracteres, gravado em `/root/rotaperfumes-mariadb/recriar.registro` ou `seed-forcar.registro` (600), junto com `janela=N`. Com outro destino, passe também `--backup-dir DIR`.

**Destino em outro disco (ex.: SSD):**

```bash
sudo bash deploy/backup-banco.sh --instalar --backup-dir /mnt/ssd/rotaperfumes --exigir-montagem /mnt/ssd
```

- Com o disco desmontado, o backup **falha** em vez de gravar no disco do sistema. A unit ganha `RequiresMountsFor=`, e o script confere com `mountpoint` no início e antes de publicar. O `ReadWritePaths=` e a conferência de espaço seguem o destino.
- `--instalar` também recusa se o disco não estiver montado, sem criar o diretório. Os ancestrais precisam ser do root, sem escrita para grupo e outros, e o destino, 700.
- Cifrar e abrir o disco fica **fora do script**: com o disco fechado no horário do timer, a execução falha e o `--status` acusa. Para voltar ao disco do sistema: `--instalar --backup-dir /var/backups/rotaperfumes --sem-exigir-montagem`.
- A troca de destino não move os backups antigos, e a retenção do diretório anterior deixa de rodar.

**Falha invisível:** sem aviso externo (decisão D2 do BACKUP-01 em aberto), uma falha só aparece no `--status`, consultado à mão. E o backup local não protege contra perda do disco (D1 em aberto).

### Imagens de produto em produção (IMG-02)

O `make db-gerar-imagens-produtos` não alcança a produção (3306 fechada). No servidor, `deploy/gerar-imagens-producao.sh` compila o `gerarimagens` num container e o roda na rede `rotaperfumes-db` (conta `rotaperfumes_admin`, .20, TLS), como o uid 65532 dono do volume `rotaperfumes_produto_imagens`, com `--read-only` e `--cap-drop ALL`. Grava pelo mesmo caso de uso do upload e pula quem já tem imagem.

```bash
sudo bash deploy/gerar-imagens-producao.sh --dry-run      # só lista; volume montado só leitura
sudo rotaperfumes-backup-banco --agora                    # o --confirmar exige backup de até 60 min
sudo bash deploy/gerar-imagens-producao.sh --confirmar    # --sobrescrever troca também as existentes
```

A vitrine mostra as imagens em até 5 min (TTL do cache). Cada `--confirmar` fica em `/root/rotaperfumes-mariadb/gerar-imagens.registro`. Fotos reais seguem pelo upload do admin.

### Atualizar

1. Faça commit e push no `main`.
2. Em até ~2 minutos o Jenkins detecta o commit novo (`pollSCM('H/2 * * * *')` no `Jenkinsfile`) e roda o job sozinho, com os valores padrão dos parâmetros (`IMPORTAR_DUMP` desmarcado, `TURNSTILE_SITE_KEY` já preenchida). Para rodar na hora, use **Construir com parâmetros**.
3. O pipeline testa, reconstrói as imagens, valida a CA do banco, recria os containers e roda o smoke test.

O gatilho só passa a valer depois que o job roda uma vez com o `Jenkinsfile` que contém o bloco `triggers`. Confira em `rotaperfumes-deploy` → **Configurar** que "Consultar periodicamente o SCM" aparece marcado.

Para mudar só segredos (ex.: senha SMTP): atualize a credencial `rotaperfumes-api-env` no Jenkins e rode o job. Auditoria de dependências e de digests: `make deps-audit` mensalmente e antes de cada deploy (`CLAUDE.md`).

### Rollback

As imagens da aplicação são sempre `:latest`, sem tag por versão. Para voltar:

1. No repositório, `git revert <commit-problemático>` e push no `main`. Evite reescrever o histórico.
2. Rode o job de novo.
3. Se o problema for de dados, reimporte um dump bom (seção 7) com `IMPORTAR_DUMP=true`. O banco não é versionado pelo pipeline: faça backup (acima) antes de importar.
4. Banco (imagem, certificado ou CA): `docs/Instal_SSL.md`, passo 10.

---

## 10. Troubleshooting

| Sintoma | Causa provável | Como resolver |
|---|---|---|
| Login aceita a senha, mas volta para a tela de login ou perde a sessão ao recarregar | Acesso por HTTP, ou certificado não confiável (cookie `Secure` descartado) | Acesse **https://ivo-inspiron-15-3530:8443** e instale a CA do Caddy (seção 8) |
| Aviso "Sua conexão não é particular" | CA do Caddy não instalada no cliente, ou CA regenerada (volume apagado) | Reexporte e reinstale o `root.crt` (seção 8) |
| Widget do Turnstile não aparece ou dá erro, ou o login recusa o captcha | Hostname não cadastrado no Cloudflare; acesso pelo IP; site key vazia/errada no build; secret errada no `api.env` | Cadastre o hostname, acesse pelo hostname e rode o job com `TURNSTILE_SITE_KEY` correta (ela é embutida no **build**). Confira `TURNSTILE_SECRET_KEY` na credencial |
| Frontend chama URL errada da API | `PUBLIC_URL` definido no ambiente do build (o padrão `/` usa a mesma origem) | Remova `PUBLIC_URL` e rode o job de novo |
| Site não abre pelo Cloudflare Tunnel | `cloudflared` com origem errada ou validando o certificado | Origem na porta 8443 do servidor com `noTLSVerify: true`; teste no servidor com `curl -k https://192.168.168.106:8443/api/health` |
| Smoke test: `tunnel=000` com `/api/health=200` usando `localhost` | O Jenkins roda em container: o `localhost` dele é o próprio container, não o servidor | O smoke test testa o site curinga pelo `SITE_IP` com um `Host` qualquer, como o `cloudflared` faz |
| `docker compose up` falha com `network rotaperfumes-db declared as external, but could not be found` | O banco ainda não foi instalado (ou a rede foi apagada) | `sudo bash deploy/mariadb-docker.sh ...` (seção 3) antes do deploy |
| `compose up` falha com `Address already in use` para `172.30.42.10` | Outro container ocupou o IP fixo da API (rede criada sem `--ip-range`) | `docker network inspect rotaperfumes-db`; tire o intruso. O script avisa quando a rede não tem `--ip-range 172.30.42.128/25` |
| API não conecta no banco (`connection refused`, `no such host` ou timeout) | Container `rotaperfumes-mariadb` parado ou fora da rede `rotaperfumes-db` | `docker ps --filter name=rotaperfumes-mariadb`; `docker logs rotaperfumes-mariadb`; rode o script (com `--recriar` se ele pedir) |
| API: `Access denied for user 'rotaperfumes_app'` | `DB_SENHA` do Secret file diferente de `rotaperfumes_app.senha`, ou API fora do IP `172.30.42.10` | Confira a credencial `rotaperfumes-api-env` e o `ipv4_address` do compose |
| API não sobe: `config: DB_TLS_CA é obrigatório quando DB_HOST não é loopback ...` | `DB_TLS_CA` vazio com host remoto (SEC-42) | Defina `DB_TLS_CA` (o compose de produção já define). Não use `DB_TLS_REQUIRED=false` |
| `openssl s_client` com `no peer certificate available`, `Cipher is (NONE)` e `Verify return code: 0 (ok)` (rc 1) | Não houve handshake TLS: o `mariadbd` recusou o host de origem (erro 1130 antes do greeting). Do próprio servidor isso sempre acontece na porta publicada (DNAT, origem sem conta); pela rede interna, quando o cliente não usa `--ip 172.30.42.20` | Pela rede interna, use `docker run ... --ip 172.30.42.20`. Na porta publicada, é o esperado: nenhuma conta aceita a origem (DEPLOY-10). O `mariadb-docker.sh` não usa mais o `s_client` na porta publicada (DEPLOY-08) |
| Estágio **CA do banco**: "bind source path does not exist" ou "nao legivel pelo uid 65532" | `/etc/mysql/ssl/ca.pem` ausente, ou arquivo/diretório sem permissão para "outros" | `docs/Instal_SSL.md` passo 5 (`chmod 644`/`755` **no host**) |
| Estágio **CA do banco**: "deploy/db-ca.fingerprint sem fingerprint: rode mariadb-docker.sh --nova-ca e grave o CA_FP (DEPLOY-07)" | O arquivo não tem nenhuma linha útil (só o cabeçalho) | Seção 11, passos 2 e 4 (rodar o script, commitar o `CA_FP` como única linha útil) |
| Estágio **CA do banco**: "fora de deploy/db-ca.fingerprint" | CA trocada no servidor sem commit do fingerprint (ou arquivo adulterado) | Confira a CA do host; se a troca foi legítima, commit do fingerprint novo (seção 3.7) |
| `mariadb-docker.sh` avisa "contem o fingerprint da CA de DEV" | A linha de uma CA de dev (`d150ee67...`, a atual, em `deploy/ca-cert.pem`, ou `ef31b656...`, a anterior) está em `deploy/db-ca.fingerprint` | Remova essa linha e commite: CA de dev nunca vale em produção (seção 3.7) |
| `mariadb-docker.sh` avisa "`$SSL_DIR/ca.pem e uma CA de DEV`" (por padrão, `/etc/mysql/ssl/ca.pem`) | A CA instalada no host é uma CA de dev. O script não aborta: o passo 2 do resumo manda não gravar o fingerprint, e não sai "ja contem: OK" | Não grave esse fingerprint. Confira como ela chegou ao servidor e restaure a CA de produção. Use `--nova-ca` só se a CA de produção não puder ser restaurada, porque ela troca a CA em uso; nesse caso, o fingerprint novo precisa ir para `deploy/db-ca.fingerprint` antes do deploy (seção 3.7) |
| `make db-*`: "DEPLOY-07: DB_HOST=... DB_PORT=... e o banco de PRODUCAO: alvo recusado" | Guarda `db-guard-producao`: host do servidor com `DB_PORT` vazio/3306, ou host da rede Docker (o dev do CasaOS é a porta 3307) | Confira se era mesmo para a produção. Se sim (e o alvo não apaga dados), repita com `CONFIRMO_PRODUCAO=<DB_HOST>/<DB_NAME>` na linha de comando (seção 3.8) |
| `make db-down/db-reset/db-rebuild/db-seed`: "SEC-46: ... e PRODUCAO: ... sao recusados mesmo com CONFIRMO_PRODUCAO" | Guarda `db-guard-sem-producao`: esses alvos nunca rodam na produção | Na produção, use `sudo rotaperfumes-backup-banco --agora` (seção 9), `sudo bash deploy/mariadb-docker.sh --recriar` e `sudo bash deploy/seed-producao.sh` no servidor (seção 11) |
| `make db-down/db-reset/db-rebuild`: "SEC-46: DROP DATABASE rotaperfumes em ... (fora do loopback): repita com CONFIRMAR=rotaperfumes" | Guarda `db-guard-drop`: `DROP DATABASE rotaperfumes` fora do loopback (ex.: CasaOS :3307) | Confira o `DB_HOST`/`DB_PORT`. Se for mesmo para apagar esse banco, repita com `CONFIRMAR=rotaperfumes` na linha de comando |
| `make test-integration`: "SEC-54: DB_HOST=... DB_PORT=... fora da allowlist da integracao" (ou `go test` com `INTEGRATION=1`: `ErrHostNaoPermitido`) | Guarda `db-guard-integracao`/`integracao.Permitidos`: integração só no loopback ou no CasaOS `192.168.168.106:3307` | Use um MariaDB descartável no loopback (`DB_HOST=127.0.0.1`) ou o dev do CasaOS na porta 3307 |
| `make ...`: `*** BUG-29: ...  Stop.` | `MAKEFLAGS`/`MAKEOVERRIDES`/`GNUMAKEFLAGS` na linha de comando, `ENV_ARQUIVO` exportado no shell, `ENV_ARQUIVO` com caractere fora de `A-Z a-z 0-9 . _ / -`, variável do make no `.env` ou `RP_ENV_PAI` passado à mão | Use `make -k ...` (ou `MAKEFLAGS=-k make ...`), `unset ENV_ARQUIVO` e passe `ENV_ARQUIVO=` na linha de comando; tire a variável do `.env` |
| `SEC-56: DB_HOST=... e IPv4 em forma numerica nao canonica` / `SEC-57: DB_PORT='...' invalida` | `DB_HOST` IPv4 decimal/hex/octal/curto ou `DB_PORT` com espaço/letra | Use `a.b.c.d` e só dígitos na porta (sem válvula) |
| `make ...`: `*** SEC-59: <arquivo>:<linhas> nao e NOME=valor literal ...  Stop.` | Linha do `.env`/`ENV_ARQUIVO` fora da allowlist: diretiva do make, `\` no fim, `$` fora de comentário ou BOM UTF-8 (na linha 1) | Corrija as linhas indicadas (seção 3.8). Um valor com `$` precisa ser trocado: não há forma de escapar |
| `make setup`: `SEC-61: ... mariadb-keyring.pgp ... Nada foi instalado` (ou `make setup-check`: `mariadb-key` como FALTA ou AVISO) | Keyring do MariaDB já instalado diverge do `MARIADB_GPG_FPR` ou não pôde ser conferido. O `make setup` aborta antes de qualquer `apt`. No `make setup-check`, divergente é FALTA e sem `gpg` é AVISO | Confira a origem do arquivo e reinstale: `sudo rm /etc/apt/keyrings/mariadb-keyring.pgp /etc/apt/sources.list.d/mariadb.sources && make setup` |
| Estágios **Testes Go**/**Testes Frontend**: "git ausente no agente (SEC-62)" ou erro de pathspec do `git archive` | Agente sem `git`, ou caminho da lista do `Jenkinsfile` não rastreado | Instale o `git` no agente (seção 2, item 3). Commite o caminho que faltou: os testes só recebem o que está em `HEAD` |
| `seed-producao.sh`: "sem terminal para pedir o PASSWORD_PEPPER: use --pepper-file ARQ" ou "--forcar exige um terminal para a confirmacao" | Script rodado sem terminal de controle (`ssh` sem `-t`, cron, pipe); falha na etapa 1 (BUG-26) | Nada foi alterado. Rode de um terminal interativo ou use `--pepper-file` (o `--forcar` sempre exige terminal) |
| `seed-producao.sh`: "a tabela usuarios ja tem N linha(s)" | O banco já foi semeado | Nada foi alterado. Não rode de novo; `--forcar` só numa instalação ainda sem uso (inativa todos os não-admin e troca a senha do admin) |
| Estágio **CA do banco**: "nao e certificado", "invalido", "VENCIDO" ou "NAO valida o certificado servido" | Arquivo com chave, CA vencida, CA que não assinou o certificado em uso ou banco fora do ar | `docs/Instal_SSL.md` passos 2, 4 e 9 |
| API: `x509: certificate signed by unknown authority` | A API ainda monta a CA antiga depois de uma troca de CA (`--nova-ca`) | Rode o job (a label do sha256 recria a API) depois do commit do fingerprint (`docs/Instal_SSL.md` passo 9.2) |
| API: `x509: certificate is valid for ..., not rotaperfumes-mariadb` | Certificado do servidor emitido fora do script ou com SAN antigo | `sudo bash deploy/mariadb-docker.sh --novo-cert --ca-key ...` |
| Estágio **Importar dump** falha com "Dump nao encontrado/legivel" | Arquivo fora do `DUMP_PATH` ou sem permissão para o usuário do Jenkins | Seção 7.2/7.3 (`chown`/`chmod`), ou o `DUMP_PATH` informado pelo `setup-servidor.sh` (Jenkins em container) |
| Import ou `Migracoes` falha com `Access denied` (1045) | Credencial `rotaperfumes-db-admin` com senha diferente de `rotaperfumes_admin.senha` | Atualize a credencial (seção 5.2) |
| Máquina de dev na 3306: `Host ... is not allowed to connect` (erro 1130) ou timeout | Esperado desde o DEPLOY-10: nenhuma conta aceita a LAN | Por SSH no servidor: `sudo rotaperfumes-dba` |
| `rotaperfumes-dba`: `ja ha uma sessao aberta` ou `mariadb-docker.sh`: `ha uma sessao do DBA aberta` | Container `rotaperfumes-dba-cliente` em uso ou abandonado | Feche a sessão; se abandonada, `sudo rotaperfumes-dba --encerrar-sessao` |
| Cliente: `Authentication plugin 'client_ed25519' cannot be loaded` | Cliente sem ed25519 (ex.: `mysql` do Oracle, Workbench) | Use o cliente `mariadb` 11.x. Na produção, o `sudo rotaperfumes-dba` já traz o cliente certo |
| `mariadb-docker.sh`: "rode de novo com --recriar" | Container existente difere do desejado (rede, porta, read-only, usuário, datadir) | Rode `sudo rotaperfumes-backup-banco --agora` e depois `--recriar` (o banco fica fora do ar alguns segundos) |
| `mariadb-docker.sh`: "--dev-ip saiu" ou "--porta saiu" | Opções removidas no DEPLOY-11: o banco não publica porta | Rode sem a opção; DBA: `sudo rotaperfumes-dba` |
| `mariadb-docker.sh`: "difere do datadir atual do container" | `--recriar` com `--dados` diferente do datadir em uso (BACKUP-07) | Use o caminho atual; para mover o datadir: backup, copie os dados e use `--recriar --trocar-dados` |
| `mariadb-docker.sh --recriar` ou `seed-producao.sh --forcar`: "nenhum backup", "tem mais de 60 min" ou outra recusa do backup mais recente | Guarda do BACKUP-01: exige dump recente, antes de qualquer mudança | `sudo rotaperfumes-backup-banco --agora`. Com o container parado, `--sem-backup-motivo TEXTO` (seção 9) |
| `rotaperfumes-backup-banco`: "nao esta montado" | Destino com `--exigir-montagem` e o disco desmontado; nada foi gravado | Abra e monte o disco e rode `--agora` |
| `rotaperfumes-backup-banco`: "volume rotaperfumes_produto_imagens nao existe" | O volume nasce no 1º deploy; o dump ficou como `parcial-*`, sem contar como backup bom | Rode o job do Jenkins e depois `--agora` |
| `mariadb-docker.sh`: "o container parou" com escrita negada fora de `/var/lib/mysql` | `--read-only` incompatível com a imagem | `--recriar --sem-read-only` |
| `permission denied ... docker.sock` no build | Usuário do Jenkins sem acesso ao socket, ou Jenkins não reiniciado | Seção 2.2 |
| Erro de schema do compose | Compose < 2.17 | Atualize o plugin `docker-compose-plugin` |
| `Bind for 0.0.0.0:8443 failed: port is already allocated` | Outra aplicação usa a 8443 | `sudo ss -ltnp \| grep 8443`. Libere a porta ou rode o job com outro `HTTPS_PORT` e ajuste `CORS_ALLOWED_ORIGINS` no Secret file |
| Smoke test falha após 30 tentativas | API caiu na subida (env inválido, banco inacessível) | O próprio job imprime as últimas linhas de `api`, `frontend` e `caddy`. Veja também a seção 9 |
| `compose` reclama de `api.env` não encontrado em comando manual | Falta `API_ENV_FILE=/dev/null` | Prefixe o comando com a variável (seção 9) |

---

## 11. Instalação inicial da produção (DEPLOY-07)

> **Estado em 2026-11-06 (informado pelo usuário): a produção está NO AR.** `rotaperfumes-api-1`, `rotaperfumes-frontend-1`, `rotaperfumes-caddy-1` (porta 8443) e `rotaperfumes-mariadb` estão no ar, e o admin já fez login e trocou a senha. **Os passos abaixo descrevem a instalação do zero: não os repita neste servidor.** O que falta é a limpeza pós-instalação do card DEPLOY-08 (`tarefas/afazer.md`), na ordem de lá. Um push na `main` agora é um **deploy em sistema vivo**. A `192.168.168.106:3306` foi fechada em 2026-11-18 (item 7 da limpeza; DEPLOY-11).
>
> **Execução do usuário**, uma vez, no servidor `192.168.168.106` (`ivo-inspiron-15-3530`) e na máquina do admin. A equipe não tem acesso ao servidor nem ao Docker dele: nenhum agente executou esta seção contra o servidor real.
>
> **Cenário (decisões do usuário, 2026-11-04):** não há banco anterior a aproveitar. O MariaDB 11.4 de produção (`rotaperfumes-mariadb`) nasce **vazio**, com uma CA **nova** (`--nova-ca`), **sem porta publicada** (o banco só responde localmente no servidor, para o `rotaperfumes-api`, o DBA pelo `rotaperfumes-dba` e o root) e o seed inicial: schema de `sql/`, o admin (senha gerada, troca obrigatória no 1º login) e os 42 vendedores **reais**, todos **inativos**. **Não há importação de dados:** o MariaDB 11.8 do CasaOS (porta 3307) continua sendo o banco de **desenvolvimento**, com a CA de dev em `deploy/ca-cert.pem`, e os dados dele não vão para a produção.
>
> **Por que esta ordem (instalação do zero):** a `main` espera a rede `rotaperfumes-db`, o container `rotaperfumes-mariadb`, a CA de produção fixada em `deploy/db-ca.fingerprint`, o banco semeado e as credenciais do Jenkins. O push do passo 9 dispara o deploy (`pollSCM`, até ~2 min). Enquanto faltar algo, o estágio `CA do banco` recusa o build **antes** do `Deploy`, e nada muda no servidor.

| Passo | Onde | O quê |
|---|---|---|
| 1 | servidor | checkout do repositório com o DEPLOY-07 |
| 2 | servidor | `mariadb-docker.sh --nova-ca`; anotar o `CA_FP` |
| 3 | servidor | chave da CA para mídia offline e `shred` |
| 4 | máquina de dev | `CA_FP` no lugar da linha útil de `deploy/db-ca.fingerprint`, `make test`, commit (sem push) |
| 5 | servidor + Jenkins | `PASSWORD_PEPPER` novo, Secret file `rotaperfumes-api-env` e credencial `rotaperfumes-db-admin` |
| 6 | servidor | `seed-producao.sh --pepper-file ...` |
| 7 | servidor | conferências SQL |
| 8 | todos | checklist pré-push |
| 9 | máquina de dev | push na `main` (deploy) |
| 10 | navegador | 1º login do admin e ativação dos vendedores |
| 11 | servidor | migração da conta `casaos` para `rotaperfumes-admin` (só em instalação anterior ao DEPLOY-10) |
| 12 | máquina com Docker (não o servidor) | teste real do `mariadb-docker.sh` |

### Passo 1. Código no servidor

O checkout no servidor precisa ter o DEPLOY-07 (`deploy/seed-producao.sh`, `sql/ordem-ddl.txt` e o `deploy/mariadb-docker.sh` atual). Não é preciso branch especial:

```bash
sudo git clone --depth 1 https://github.com/residentivo/RotaPerfume.git /root/rotaperfumes
# já clonado antes:
#   sudo git -C /root/rotaperfumes pull --ff-only
sudo test -f /root/rotaperfumes/deploy/seed-producao.sh && echo "DEPLOY-07 presente"
```

- Se o commit do DEPLOY-07 já foi para a `main` do GitHub, o build que esse push disparou para no estágio `CA do banco` (`bind source path does not exist: /etc/mysql/ssl/ca.pem` ou, numa instalação do zero, com CA nova, enquanto o fingerprint dela não entra no arquivo (passo 4), `fora de deploy/db-ca.fingerprint`). É o esperado nesta fase: o estágio roda antes do `Deploy` e nada muda no servidor.
- Se ainda não está no GitHub, copie da máquina de dev `deploy/`, `sql/` e `backend/shared/` para `/root/rotaperfumes/` (`scp -r`/`rsync`; o `seed-producao.sh` compila o `resetpassword` a partir de `backend/shared`). Não copie `.env`, `.env.producao` nem `deploy/ca-cert.pem`.

Nos passos 2, 3, 5, 6 e 7, os comandos rodam **no servidor**, como root (`sudo -i`), com `umask 077`, a partir de `/root/rotaperfumes`.

### Passo 2. MariaDB de produção com CA nova

```bash
sudo -i
umask 077
cd /root/rotaperfumes
bash deploy/mariadb-docker.sh --nova-ca
```

> **Caminho ainda não testado no servidor.** O script só tem cobertura de teste com Docker simulado (`backend/shared/tests/deploy/mariadb_docker_test.go`) e pelo teste real opt-in do passo 12, em container isolado. Faça as conferências abaixo na primeira execução e registre o resultado no card DEPLOY-08. **Neste servidor,** o script já rodou e a 3306 já foi fechada pelo `--recriar` (2026-11-18): nunca rode o `--nova-ca` acima nele.

- `--nova-ca` gera a CA de produção (`CN=RotaPerfumes_MariaDB_CA`, chave em `/root/rotaperfumes-ca/ca-key.pem`, que sai do servidor no passo 3) e o certificado do servidor (SAN `IP:192.168.168.106, DNS:rotaperfumes-mariadb, DNS:localhost, IP:127.0.0.1, DNS:<hostname>`). O script cria a rede `rotaperfumes-db`, o container, o banco `rotaperfumes` **vazio**, as contas e as senhas em `/root/rotaperfumes-mariadb/` (seções 3.1 a 3.4).
- **Produção só local** (decisão de 2026-11-04): nenhuma porta é publicada, a chain `ROTAPERFUMES-DB` e a unit `rotaperfumes-db-firewall.service` não são criadas, e não há conta `casaos` nem `casaos.senha`. Um `casaos.senha` já existente é mantido, com o aviso `mantido; leve ao pendrive seguro`; nenhum script o lê ou apaga (DEPLOY-10). A API, o Jenkins e o DBA não precisam da porta (usam a rede interna).
- Confira:
  ```bash
  docker ps --filter name=rotaperfumes-mariadb --format '{{.Names}} {{.Image}} {{.Status}}'
  docker network inspect rotaperfumes-db -f '{{range .IPAM.Config}}{{.Subnet}} {{.IPRange}}{{end}}'   # 172.30.42.0/24 172.30.42.128/25
  docker port rotaperfumes-mariadb      # nenhuma saída
  ss -ltnp | grep ':3306 '              # nenhuma saída (se aparecer algo, pare e identifique o serviço)
  iptables -S ROTAPERFUMES-DB           # erro de chain inexistente (rc 1)
  iptables -S DOCKER-USER | grep ROTAPERFUMES-DB   # nenhuma saída
  ```
- **Se o script já rodou neste servidor,** não use `--nova-ca`. O datadir `/var/lib/rotaperfumes/mariadb` é preservado, então o banco **não** nasce vazio: pare e avalie antes de seguir. A unit e a chain do firewall antigo, se existirem, são removidas pelo script (seção 3.3).
- **Anote o `CA_FP`**: é o valor impresso no fim, em "CA do banco de PRODUCAO ... fingerprint SHA-256 (formato do deploy/db-ca.fingerprint)". Ele vai para o passo 4. Numa instalação do zero, com CA nova, o resumo também avisa que o `deploy/db-ca.fingerprint` "NAO contem" esse valor (o arquivo traz o fingerprint da CA que estava em uso, trocado no passo 4), que a CA é nova e que a chave da CA está no servidor (passo 3). Os três avisos são esperados. O texto "a linha antiga pode ficar ate o deploy" desse aviso vale para a rotação de CA; numa instalação do zero, a troca é feita no passo 4.
- **Não** copie esta CA para `deploy/ca-cert.pem`: esse arquivo é a CA de **dev** (CasaOS).

### Passo 3. Chave da CA para mídia offline

A chave da CA permite emitir certificados que a API aceita: ela **não fica no servidor** (`docs/Instal_SSL.md`, passo 3).

```bash
cp /root/rotaperfumes-ca/ca-key.pem /media/<pendrive-cifrado>/ca-key.pem
cp /etc/mysql/ssl/ca.pem /root/rotaperfumes-ca/server-ext.cnf /media/<pendrive-cifrado>/
cmp /root/rotaperfumes-ca/ca-key.pem /media/<pendrive-cifrado>/ca-key.pem && \
  shred -u /root/rotaperfumes-ca/ca-key.pem
```

Guarde uma segunda cópia (outro pendrive cifrado ou cofre de senhas). Para renovar o certificado depois, use `--ca-key /media/<pendrive>/ca-key.pem` (`docs/Instal_SSL.md`, passo 9.1).

### Passo 4. Fingerprint da CA no repositório (máquina de dev; sem push)

Na raiz do repositório, na máquina de dev, com o `CA_FP` do passo 2:

```bash
sed -i '/^[0-9a-f]\{64\}$/d' deploy/db-ca.fingerprint          # só numa instalação do zero, com CA nova: tira a linha útil; o cabeçalho (#) fica
printf '%s\n' '<CA_FP do passo 2>' >> deploy/db-ca.fingerprint
grep -Ev '^[[:space:]]*(#|$)' deploy/db-ca.fingerprint        # uma linha só: o CA_FP (64 hex minúsculos)
env -u INTEGRATION make test
git add deploy/db-ca.fingerprint
git commit -m "DEPLOY-07: fingerprint da CA de producao"
```

- **No estado atual (2026-11-06), não rode este passo:** a linha `f31ee820...` é a CA de produção em uso no servidor e **não deve ser apagada** (card DEPLOY-08). O `sed` serve numa instalação do zero, com CA nova, para não ficarem duas linhas.
- Mantenha as 3 linhas de cabeçalho (`#`); a **única** linha útil é o `CA_FP`. **Nunca** acrescente o fingerprint de uma CA de dev (`d150ee67...`, a atual, em `deploy/ca-cert.pem`, ou `ef31b656...`, a anterior): o teste `TestDEPLOY07_CADevNuncaEmProducao` falha e o `mariadb-docker.sh` avisa.
- Para conferir no servidor: `openssl x509 -in /etc/mysql/ssl/ca.pem -noout -fingerprint -sha256 | cut -d= -f2 | tr -d : | tr A-F a-f`.
- **Não faça push ainda**: o push é o passo 9, depois das credenciais e do seed.

### Passo 5. `PASSWORD_PEPPER` novo e credenciais do Jenkins

O banco é novo, então o pepper também é **novo** (nada a reaproveitar do dev). Ele vai para o Secret file **e** para o seed (passo 6): os dois precisam do **mesmo** valor, senão o admin não consegue logar. Depois de definido, **nunca troque** (todas as senhas Argon2id deixam de valer).

1. No servidor, gere o pepper e monte o `api.env` a partir de `deploy/api.env.example` (os segredos vão por `printf`, um builtin do bash, e nunca aparecem no argv):
   ```bash
   cd /root/rotaperfumes
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
   O arquivo já sai com `DB_HOST=rotaperfumes-mariadb`, `DB_PORT=3306`, `DB_NAME=rotaperfumes`, `DB_USUARIO=rotaperfumes_app`, `COOKIE_SECURE=true` e `TRUST_PROXY_HEADERS=true` do modelo (`DB_HOST`/`DB_PORT`/`DB_TLS_CA`/`DB_TLS_SERVER_NAME` são fixados pelo `docker-compose.yml` de qualquer forma). No editor, preencha o resto conforme a seção 5.1: `TURNSTILE_SECRET_KEY` **real** (seção 4), `SMTP_*` e `SECURITY_ALERT_EMAILS` (o SMTP é **necessário** para ativar os vendedores no passo 10) e confira `CORS_ALLOWED_ORIGINS`. **Não** inclua `DB_TLS_REQUIRED=false`.
2. Leve o arquivo para a máquina de onde você acessa o Jenkins, sem passar por chat ou e-mail. Por exemplo, no servidor: `install -m 600 -o <seu_usuario> "$A" /home/<seu_usuario>/rotaperfumes-api.env`; na sua máquina: `scp <seu_usuario>@192.168.168.106:rotaperfumes-api.env .`
3. Jenkins → Gerenciar Jenkins → Credentials → System → Global credentials:
   - **Secret file** com ID `rotaperfumes-api-env` (se já existir: abra → **Update** → envie o arquivo novo).
   - **Username with password** com ID `rotaperfumes-db-admin`: usuário `rotaperfumes_admin`, senha = `sudo cat /root/rotaperfumes-mariadb/rotaperfumes_admin.senha` (lida no terminal do servidor e colada direto no Jenkins).
4. Apague as cópias: na sua máquina, o `rotaperfumes-api.env`; no servidor, `shred -u /home/<seu_usuario>/rotaperfumes-api.env /root/rotaperfumes-api.env`. **Mantenha** o `/root/pepper-producao.txt` até o passo 6.

> Não use o `setup-servidor.sh` com o `.env` de dev para gerar o Secret file: ele copia o `PASSWORD_PEPPER` do `--env` (o de dev), que não é o do seed de produção.

### Passo 6. Seed inicial (`deploy/seed-producao.sh`)

```bash
cd /root/rotaperfumes
bash deploy/seed-producao.sh --pepper-file /root/pepper-producao.txt
```

- O `--pepper-file` precisa ser arquivo regular do root com modo 600 ou 400 (o do passo 5 já é); sem a opção, o script pede o pepper no terminal, sem eco e com confirmação. O valor tem de ser o **mesmo** `PASSWORD_PEPPER` do Secret file, com 32 caracteres ou mais.
- **Sem terminal (BUG-26):** rodado sem terminal de controle (ex.: `ssh` sem `-t`, cron, pipe), o script **falha logo na etapa 1**, antes do Docker e do banco, com mensagem clara: sem `--pepper-file`, `sem terminal para pedir o PASSWORD_PEPPER: use --pepper-file ARQ`; com `--forcar`, `--forcar exige um terminal para a confirmacao` (mesmo com o banco vazio). Nada é alterado. Rode de um terminal interativo (`sudo -i` numa sessão `ssh -t`) ou passe `--pepper-file`.
- As 7 etapas (`[n/7]`; detalhes em `bash deploy/seed-producao.sh --help`): (1) confere root, Docker, container, rede, CA, senha do `rotaperfumes_admin`, `sql/ordem-ddl.txt`, seeds, pepper e o destino da senha do admin, sem mudar nada; (2) **recusa** se a tabela `usuarios` já tem linhas (banco já semeado; `--forcar` exige digitar o nome do banco, reaplica tudo, troca a senha do admin e inativa **todos** os não-admin: só numa instalação ainda sem uso); (3) DDL na ordem de `sql/ordem-ddl.txt` (a mesma do `make db-up`); (4) `sql/02_seed_admin.sql` e `sql/03_seed_vendedores.sql` e `UPDATE usuarios SET ativo = 0 WHERE role <> 'admin'`; (5) compila o `resetpassword` num container `GO_IMAGE` (sem rede do banco nem segredos; baixa os módulos do proxy do Go, então o servidor precisa de internet); (6) `resetpassword -create-admin` num container na rede `rotaperfumes-db` (IP `172.30.42.20`, conta `rotaperfumes_admin`, TLS verificado), com os segredos num `--env-file` temporário 600 apagado com `shred`; (7) conferência.
- **Senha do admin:** gerada e **nunca impressa**, gravada em `/root/rotaperfumes-mariadb/admin-app.senha` (600, só root; o script recusa se o arquivo já existir). Outro destino: `--admin-senha-arquivo ARQ`, com caminho absoluto e diretório privado (SEC-60: root, `700`, sem symlink, ancestrais do root sem escrita de grupo/outros), conferido antes de ler os segredos; caminho em `/tmp` é recusado. O padrão `<segredos-dir>/admin-app.senha` continua valendo. Para digitar a senha em vez de gerar: `--admin-senha-prompt` (exige terminal). Em qualquer caso, o admin fica com `deve_trocar_senha = 1`.
- **Isolamento (TST-17 item 1, Lote 37):** `--container NOME`, `--rede-nome NOME`, `--ip-admin IP`, `--segredos-dir DIR`, `--ssl-dir DIR` (CA em `DIR/ca.pem`) e `--trabalho-dir DIR` (onde nasce a área temporária `rotaperfumes-seed.XXXXXX`). Padrões: `rotaperfumes-mariadb`, `rotaperfumes-db`, `172.30.42.20`, `/root/rotaperfumes-mariadb`, `/etc/mysql/ssl` e `/root`. **No servidor, use os padrões** (sem as opções); elas existem para o teste real (`TestTST17_SeedProducao_Real`, instalação descartável criada pelo `mariadb-docker.sh` com as mesmas opções). Sem `--admin-senha-arquivo`, a senha do admin vai para `<segredos-dir>/admin-app.senha`. Toda validação é na etapa 1, antes de qualquer escrita (só `docker inspect`/`docker network inspect`): container com o mesmo formato do `mariadb-docker.sh` (minúsculas, números e `-`, até 63), rodando e ligado à `--rede-nome` (nome exato); `--ip-admin` IPv4 canônico e igual a `A.B.C.20` da **única** subnet IPv4 `A.B.C.0/24` da rede (o `mariadb-docker.sh` só cria `rotaperfumes_admin@<prefixo>.20`); diretórios absolutos, sem `..`, espaços ou vírgulas (o caminho entra em `--mount type=bind,source=...`), com o caminho real igual ao informado (nenhum componente symlink) e todos os ancestrais do root sem escrita para grupo/outros; `--segredos-dir` e `--trabalho-dir` existentes, do root e `700` (mesmo `conferir_dir_privado` do `mariadb-docker.sh`); `--ssl-dir` existente, do root, sem escrita para grupo/outros (pode ser 755), e `ca.pem` arquivo regular (não symlink), não vazio, do root e sem escrita para grupo/outros. Pelos caminhos reais (`realpath -m`): `--ssl-dir` disjunto de `--segredos-dir` e de `--trabalho-dir` (nenhum igual ou dentro do outro), `--trabalho-dir` não pode ser `--segredos-dir` nem ficar dentro dele (o contrário pode, como nos padrões) e o destino da senha do admin não pode ficar no `--ssl-dir`. Depois do `mktemp -d`, a área de trabalho precisa ser caminho real, do root, `700`.
- **Mudança de comportamento (TST-17, fail closed):** se `/root` não for do root com modo `700`, ou `/root/rotaperfumes-mariadb` não for `700`, o seed agora **recusa** na etapa 1 com mensagem clara (`... precisa ser do root com modo 700 ...`). Ajuste (`chown root:root`, `chmod 700`) e rode de novo; nada foi alterado.
- O script **não** roda `-all-users`: os 42 vendedores ficam com o hash placeholder (ninguém loga com ele) e inativos até o passo 10.
- **Nunca** use `make db-seed`, `make db-reset` ou `make fix-hash` contra este banco (a guarda `db-guard-producao` recusa; seção 3.8).
- Depois do seed: guarde uma cópia do pepper num cofre de senhas ou na mídia offline do passo 3 (sem ele, nenhuma senha de produção funciona se o Secret file se perder) e apague o arquivo do servidor:
  ```bash
  shred -u /root/pepper-producao.txt
  ```
  O pepper continua no Secret file `rotaperfumes-api-env`.

### Passo 7. Conferências SQL

Pela rede interna, com o mesmo cliente do `seed-producao.sh` e do estágio `Importar dump` (imagem fixada = `IMAGEM_BASE`/`DB_CLIENT_IMAGE`, conta `rotaperfumes_admin` em `172.30.42.20`, senha por `MYSQL_PWD`, nunca no argv):

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

Esperado: `tabelas` = 14 (um por `CREATE TABLE` dos arquivos de `sql/ordem-ddl.txt`; era 13 antes do ECOM-03); uma linha de admin (`admin@rotaperfumes.com.br`) com `ativo` = 1, `deve_trocar_senha` = 1 e `hash` começando com `$argon2id$`; `vendedores` = 42; `nao_admin` = 42 e `nao_admin_ativos` = 0. Qualquer diferença: **não** siga para o push; veja a saída do `seed-producao.sh`.

### Passo 8. Checklist pré-push

> **Neste servidor (2026-11-06):** na limpeza do card DEPLOY-08, o push (item 5) vem antes da retirada da chave da CA (item 9). Por isso, o item "a chave da CA saiu do servidor" não bloqueia esse push; a retirada segue pendente até o item 9.

- [ ] `docker ps` mostra `rotaperfumes-mariadb` `Up` e `docker network inspect rotaperfumes-db` funciona;
- [ ] `/etc/mysql/ssl/ca.pem` legível por "outros" (`sudo -u nobody cat /etc/mysql/ssl/ca.pem >/dev/null && echo OK`; `docs/Instal_SSL.md`, passo 5);
- [ ] o fingerprint de `/etc/mysql/ssl/ca.pem` é a **única** linha útil do `deploy/db-ca.fingerprint` **commitado** (passo 4), e não é o de uma CA de dev (`d150ee67...` ou `ef31b656...`);
- [ ] a chave da CA saiu do servidor (`test ! -e /root/rotaperfumes-ca/ca-key.pem && echo OK`);
- [ ] Secret file `rotaperfumes-api-env` cadastrado ou atualizado (passo 5), com o **mesmo** `PASSWORD_PEPPER` do seed, `JWT_SECRET` novo, `TURNSTILE_SECRET_KEY` real, `SMTP_*`, `COOKIE_SECURE=true` e sem `DB_TLS_REQUIRED=false`; credencial `rotaperfumes-db-admin` cadastrada;
- [ ] seed conferido (passo 7);
- [ ] job `rotaperfumes-deploy` criado (seção 6) e o parâmetro **`IMPORTAR_DUMP` desmarcado** (padrão `false`, que é o valor usado pelo `pollSCM`). Não há dump a importar: marcado, ele **sobrescreveria** o banco de produção;
- [ ] `make deps-audit` rodado (`CLAUDE.md`: antes de cada deploy);
- [ ] tudo que os testes usam está commitado (SEC-62): arquivos novos, como `sql/27_*` e os testes, no mesmo commit do `Jenkinsfile`.

### Passo 9. Push na `main` (dispara o deploy)

Na máquina de dev:

```bash
git push origin main
```

Em até ~2 minutos o `pollSCM` dispara o job `rotaperfumes-deploy` com os valores padrão. Estágios esperados: `Checkout` → `Testes Go` → `Testes Frontend` → `Build` → `CA do banco` (lista o fingerprint aceito, cada certificado com `na lista (deploy/db-ca.fingerprint)`, `s_client: Verify return code: 0 (ok)` e `CA do banco validada: /etc/mysql/ssl/ca.pem (sha256 ...)`) → `Importar dump` (pulado: `IMPORTAR_DUMP=false`) → `Migracoes` (`Migracoes concluidas.`) → `Deploy` (a API entra na rede `rotaperfumes-db` com `172.30.42.10`) → `Smoke test` verde. Depois, no servidor:

```bash
docker logs rotaperfumes-api-1 2>&1 | grep -E 'TLS do banco|conexão MariaDB estabelecida'
# [server] TLS do banco: ligado (ca=/etc/rotaperfumes/db-ca.pem, server_name=rotaperfumes-mariadb, min=TLS1.3)
# [db] conexão MariaDB estabelecida (host=..., tls=ligado (custom))
```

Falhou? A seção 10 (Troubleshooting) cobre os erros de cada estágio. O `CA do banco` roda antes do `Deploy`: se ele falhar, nada foi alterado.

### Passo 10. Primeiro login do admin e ativação dos vendedores

1. No servidor, leia a senha inicial **no terminal** (não cole em chat, log ou ticket): `sudo cat /root/rotaperfumes-mariadb/admin-app.senha`.
2. Acesse `https://ivo-inspiron-15-3530:8443` (CA do Caddy instalada, seção 8) e faça login com `admin@rotaperfumes.com.br`. Com `deve_trocar_senha = 1`, o frontend leva para `/trocar-senha`, e a API responde `403` `"troca de senha obrigatória"` em todas as outras rotas protegidas até a troca (SEC-23). Troque a senha e faça login de novo.
3. Apague o arquivo da senha inicial: `sudo shred -u /root/rotaperfumes-mariadb/admin-app.senha`.
4. **Ativar os vendedores:** os 42 usuários de vendedor estão **inativos** e com hash placeholder (ninguém loga). Para cada vendedor que vai usar o sistema, como admin, na tela **Usuários**: confira o e-mail cadastrado (o seed usa `nome.sobrenome@rotaperfumes.com.br`), clique em **Resetar** (a API gera uma senha aleatória, grava com troca obrigatória no 1º login e a envia **por e-mail**; `POST /api/admin/reset-password`) e **ative** o usuário pelo botão de status (`PATCH /api/usuarios/{id}/inativar`). O reset sozinho não ativa o usuário.
   - O reset depende do **SMTP** do Secret file: sem SMTP, a resposta vem com `email_enviado = false` e a senha gerada não chega a ninguém (não aparece na tela nem no log). Configure o `SMTP_*` antes.
   - Usuários de vendedores desligados (vendedores 1, 3, 9, 30, 37 e 38) continuam inativos.
   - Não use `make reset-senhas-ativos` nem o `resetpassword -all-users` para isso.

### Passo 11. Migração da conta `casaos` para `rotaperfumes-admin` (DEPLOY-10)

Só em instalação anterior ao DEPLOY-10. Vem entre os itens 6 e 7 da limpeza do DEPLOY-08, **sem `--recriar`** até o último comando. Numa instalação nova, a conta já nasce certa, e este passo não se aplica.

1. Em horário calmo, com o checkout atualizado (item 6). Cria `rotaperfumes_backup` e `rotaperfumes-admin`, remove `casaos@*`, instala o comando, sem reiniciar o container:
   ```bash
   sudo bash deploy/mariadb-docker.sh
   ```
   (Na época, com a 3306 publicada, este comando levava `--dev-ip 192.168.168.105`; a opção saiu no DEPLOY-11.)
2. Confira. A saída precisa trazer `rotaperfumes-admin@172.30.42.22`:
   ```bash
   echo 'SELECT CURRENT_USER()' | sudo rotaperfumes-dba
   ```
3. Instale a rotina de backup:
   ```bash
   sudo bash deploy/backup-banco.sh --instalar
   ```
4. Item 7 da limpeza, em janela com queda rápida aceitável. O `--recriar` fecha a 3306:
   ```bash
   sudo rotaperfumes-backup-banco --agora
   sudo bash deploy/mariadb-docker.sh --recriar
   ```

**Saída se algo falhar:** `root@localhost`, com o `docker exec` da seção 3.4.

**Depois da migração:**

- A máquina de dev não acessa mais a produção: um `.env.producao` e um `deploy/ca-producao.pem` nela deixam de ter uso. O `.env` padrão continua apontando para o CasaOS de dev (`192.168.168.106:3307`), que não muda.
- `casaos.senha` e `casaos-native.motivo` ficam onde estão, sem uso. Nenhum script os lê ou apaga, e a senha não é reaproveitada. Leve o `casaos.senha` ao pendrive seguro.
- Fase 2 do `--dev-ip`: feita no DEPLOY-11 (2026-11-18); a opção é recusada e o firewall antigo sai na próxima execução do script.

### Passo 12. Teste real do script (`TestTST15_MariadbDocker_Real`)

Numa máquina **com Docker que não seja o servidor**, como root (o script recusa outro EUID), na raiz do repositório. **TST-16 (Lote 36):** o teste passa `--segredos-dir` e `--ca-dir` apontando para um diretório temporário 700 do root criado em `/root` (`/root/.tst15-*`, removido no fim; o `/tmp` 1777 é recusado desde o Lote 36), junto do datadir e do `--ssl-dir`, então não lê nem grava `/root/rotaperfumes-ca` nem `/root/rotaperfumes-mariadb`; o único caminho fixo é o diretório de build `/opt/rotaperfumes/mariadb-imagem` (removido no fim se o teste o criou):

```bash
cd backend/shared
sudo -E env "PATH=$PATH" TST15_DOCKER=1 go test -count=1 -run TestTST15_MariadbDocker_Real ./tests/deploy/
```

Ele sobe um container `tst15-mariadb` numa rede `tst15-db` (`172.30.243.0/24`) com `--nova-ca`, confere as contas ed25519, `REQUIRE SSL`, a recusa sem TLS, nenhuma porta, a conta e o comando do DBA, a reexecução sem troca de senha e o `--novo-cert` com `FLUSH SSL`, e limpa tudo no fim. Se puder, rode-o **antes** do passo 2. Registre o resultado no card TST-15.

O `deploy/seed-producao.sh` tem o teste real `TestTST17_SeedProducao_Real` (TST-17; opt-in `TST17_DOCKER=1`, como root, **nunca** no servidor de produção: o teste recusa o hostname e o container de produção), que usa as opções de isolamento do passo 6 numa instalação descartável. Enquanto ele não rodar numa máquina descartável, a primeira execução real é a do passo 6.

---

## 12. Restaurar em máquina nova (DEPLOY-09)

`deploy/restaurar-maquina.sh` devolve o site numa máquina **nova**: sobe o MariaDB e os containers, restaura dump, imagens e pepper, instala a rotina de backup e o `cloudflared`, e aponta o que você precisa levar ao site da Cloudflare e ao Jenkins. No servidor em uso, ele recusa: lá, a restauração continua sendo só o runbook de `docs/manual-base-de-dados.md`, seção 24.5, e `deploy/restaurar-banco.sh` continua proibido.

**Antes de começar, na máquina nova:** Docker com o plugin `compose`, `git`, `gnupg` e `curl`; um IP fixo; um clone limpo da `main` feito como root (`git clone` em `/root/rotaperfumes`, com `umask 022`). O script não se instala em `/usr/local/sbin`: ele roda do clone que você acabou de conferir.

**O que levar:**

| Item | Origem | Observação |
|---|---|---|
| `rotaperfumes-*.sql.gz` e `rotaperfumes-imagens-*.tar.gz` | Cópia externa do backup | Mesmo carimbo. Carimbos diferentes só com `--par-divergente-motivo TEXTO`, registrado |
| Cópia do Secret file `rotaperfumes-api-env` | Cofre | Root, modo 600 ou 400; o trio sozinho não sobe a API |
| `PASSWORD_PEPPER` | Cofre, longe do dump | Arquivo (`--pepper-file`) ou digitado, com confirmação |
| `ca.pem` e `ca-key.pem` | Mídia offline | Se perdeu a chave: `--nova-ca` |
| Token do túnel | Painel da Cloudflare | Arquivo (`--token-file`) ou digitado na etapa 11 |

```bash
sudo bash deploy/restaurar-maquina.sh --listar-manuais   # só nomes, nunca valores; não muda nada

sudo bash deploy/restaurar-maquina.sh \
  --dump /media/cofre/rotaperfumes-AAAAMMDD-HHMMSSZ.sql.gz \
  --imagens /media/cofre/rotaperfumes-imagens-AAAAMMDD-HHMMSSZ.tar.gz \
  --api-env /media/cofre/api.env \
  --ip IP_DA_LAN --host-nome NOME_NA_LAN --hostname-publico SITE_PUBLICO \
  --ca-cert /media/cofre/ca.pem --ca-key /media/cofre/ca-key.pem
```

O `--api-env` precisa trazer `https://SITE_PUBLICO`, `https://IP_DA_LAN:8443` e `https://NOME_NA_LAN:8443` em `CORS_ALLOWED_ORIGINS` e, se `TURNSTILE_HOSTNAMES` estiver preenchida, o hostname público nela. O script não reescreve nada: cita o nome da chave e para. Inclua também o nome ou o IP da LAN nessa lista, senão o login da etapa 9 é recusado pelo Turnstile (o script avisa). Um ensaio com o `api.env` real pode enviar e-mails verdadeiros.

Com `--sem-tunel` (ensaio), o script pula o `cloudflared` e o token e dispensa a declaração sobre a máquina antiga; a escolha fica em `/root/rotaperfumes-restauracao/registro`.

**Recusas, antes de qualquer mudança** (nenhuma flag ou variável as dispensa):

- **Execução:** sem root, sem terminal ou dentro do Jenkins.
- **Clone:** `origin` diferente, `HEAD` diferente da `main` remota, qualquer arquivo alterado, não rastreado ou **ignorado**, arquivo que não seja do root ou com escrita de grupo/outros, ancestral inseguro, link simbólico na raiz.
- **Produção ativa:** container `rotaperfumes-mariadb` ou do compose `rotaperfumes`, `/var/lib/rotaperfumes/mariadb/mysql`, `/etc/mysql/ssl/server-key.pem`, `/root/rotaperfumes-mariadb/root.senha`, `/root/rotaperfumes-ca/ca-key.pem`, unit ou binário do backup, par em `/var/backups/rotaperfumes` (ou no `--backup-dir`), `cloudflared.service` ou `rotaperfumes-cloudflared.service`, volume de imagens com conteúdo.
- **IP:** `--ip` não configurado nesta máquina.
- **Entradas:** nomes fora do padrão do backup (`parcial-*` e `pre-restauracao-*` não valem), link simbólico, CA de dev, chave que não é da CA, CA fora de `deploy/db-ca.fingerprint`, secret de teste do Turnstile, `$` sem aspas simples, chave obrigatória vazia, pepper diferente do `--api-env`, menos de 2× os arquivos + 1 GiB livres.
- **Banco com dados:** depois do `mariadb-docker.sh`, qualquer tabela em `rotaperfumes`.
- **`--ca-key` dentro de `/root/rotaperfumes-ca`** (direto ou por link): esse caminho é sinal de servidor em uso; deixe a chave na mídia externa e aponte o `--ca-key` para lá.
- **DNS da LAN, com túnel:** `nameserver` do `/etc/resolv.conf` em `10/8`, `172.16/12`, `192.168/16` ou `169.254/16`. A unit do túnel barra essas faixas; use o `systemd-resolved` (`127.0.0.53`) ou um DNS público.

**Etapas:**

| # | O que faz | Para e pede |
|---|---|---|
| 0 | Recusas, pepper, espaço | |
| 1 | Copia dump e tar para `/root/rotaperfumes-restauracao/entrada` (700/600) e verifica a cópia: `verificar-backup.sh` e `backup-banco.sh --conferir-imagens` | |
| 2 | Mostra contagens, carimbo, idade, IP, host, commit | O `hostname`; com túnel, `ANTIGA PARADA` |
| 3 | Instala o `ca.pem` e roda `mariadb-docker.sh`. **Ignore o seed sugerido no fim:** o `seed-producao.sh` criaria as tabelas, e a importação falharia | |
| 4 | Gera `/root/rotaperfumes-restauracao/api.env` (600), com `DB_SENHA` e `JWT_SECRET` novos e o pepper informado | |
| 5 | Importa pela `rotaperfumes-admin`, com `--sandbox --local-infile=0`; compara as contagens; `TRUNCATE refresh_tokens` | |
| 6 | Lista as `sql/*_alter_*`: aplique as mais novas que o dump com `sudo rotaperfumes-dba < arquivo` | `MIGRACOES CONFERIDAS` |
| 7 | `validar-ca-banco.sh`, como no Jenkinsfile (com `--nova-ca`, só o fingerprint novo, só nesta etapa); `docker compose build --pull` e `up --no-start` | |
| 8 | Confere labels, dono e que o volume está vazio; extrai como 65532, sem apagar nada; compara a contagem | |
| 9 | `up -d` e smoke por `SITE_IP` e `https://localhost:8443`, com `Host` qualquer | `LOGIN OK`: login de admin pela LAN e produto com imagem, a **única prova do pepper** |
| 10 | `backup-banco.sh --instalar` e `--status` | |
| 11 | `cloudflared`: chave conferida, repositório com `signed-by` e pin, unit `rotaperfumes-cloudflared.service` endurecida (Lote 42: `IPAddressDeny` da LAN, `SystemCallFilter`, `MemoryDenyWriteExecute` etc.), token em `/etc/rotaperfumes/cloudflared.env` (600). Depois de subir, confere `/ready` em `127.0.0.1:20241` e a origem `https://localhost:8443` numa unit transitória com o mesmo filtro de IP. Qualquer falha depois de gravar o token: `systemctl stop` e `systemctl disable` da unit (o mesmo que `disable --now`) e `shred -u` do token; rode de novo | `PAINEL CONFERIDO` e o token |
| 12 | Lista do Jenkins e do Secret file | `JENKINS ATUALIZADO`; só então apaga `api.env` e `entrada/` com `shred` |

**Retomar:** rode o mesmo comando. O estado fica em `/root/rotaperfumes-restauracao/estado` (600), nasce depois da etapa 2 e só vale com o mesmo `machine-id`, os mesmos sha256 e as mesmas opções confirmadas. A etapa concluída é pulada, mas o estado real é reconferido. Se a importação falhou e não há container do compose, o script oferece `DROP`/`CREATE DATABASE`, com o nome digitado. Se a extração das imagens foi interrompida, ele não apaga nada: remova o volume à mão (`docker volume rm rotaperfumes_produto_imagens`, depois de `API_ENV_FILE=/dev/null docker compose -p rotaperfumes rm -f api`) e rode de novo. Com o estado `concluida`, recusa sempre.

**Cloudflare, no painel:** o `--listar-manuais` e a etapa 11 trazem a lista: hostname público, serviço `https://localhost:8443` com a verificação TLS desligada (noTLSVerify), token, conector antigo parado, hostnames adicionais e os do Turnstile. Dois conectores com o mesmo token ficam ativos ao mesmo tempo, e cada requisição vai para o mais próximo: o script não enxerga a outra máquina, e a proteção é a declaração digitada. **Com a máquina antiga perdida ou sob suspeita,** "Rotate token" no painel não basta: limpe também as conexões (`DELETE` em `/connections` da API), porque os conectores atuais seguem ativos até reiniciar.

**Chave do apt:** `deploy/cloudflare-apt.asc` e `deploy/cloudflare-apt.fingerprint`. Fingerprint diferente encerra o script. O fingerprint veio do HTTPS da Cloudflare e foi confirmado em 2026-11-07 por um segundo canal (SEC-73): keyserver.ubuntu.com e keys.openpgp.org servem a mesma chave e a subchave `06C8…F50F`. Ao trocar a chave, confirme o fingerprint novo por um segundo canal antes do commit e mude os dois arquivos juntos.

**IP ou hostname diferentes:** o certificado do banco é reemitido, o que exige `--ca-key`, e o `--listar-manuais` lista os arquivos que precisam de commit (`Jenkinsfile`, `deploy/docker-compose.yml`, `deploy/mariadb-docker.sh`, `deploy/setup-servidor.sh`, guardas do `Makefile` e do `integracao.go`) e os que não mudam (CasaOS de dev). Com `--nova-ca`, grave o fingerprint em `deploy/db-ca.fingerprint`, faça o commit e o push **depois** de atualizar o Secret file, senão o estágio "CA do banco" recusa. O `caddy_data` nasce novo: reinstale a raiz da CA nos clientes da LAN.

**O que foi validado:**

- **Com stubs, em namespace (`deploy09_*_linux_test.go`):** cada recusa, a ordem das etapas, a retomada, os vazamentos de pepper e token, a chave do apt e a unit gerada (`systemd-analyze verify`). Provam o controle do script, não o Docker, o apt nem o systemd reais.
- **Com Docker real, dados sintéticos, sem root (`TestDEPLOY09_Real`, opt-in `DEPLOY09_DOCKER=1`):** importação com `--sandbox` pela conta real, comparação de contagens, erro só com código e linha, volume recém-criado com dono 65532, extração, recusas de volume com conteúdo, de outro dono e de outras labels. O volume nasce de `docker volume create` com as labels e `docker create`, se o plugin `compose` não existir na máquina do teste.
- **Só em máquina real (ainda não feito):** o script de ponta a ponta com `--sem-tunel`, inclusive o `docker compose up --no-start` de verdade, o smoke e a unit de backup; o `apt` real, o pin e a assinatura; o `cloudflared` com token e painel, incluindo `TUNNEL_TOKEN` por `EnvironmentFile` com `DynamicUser`, `Type=notify` e o caminho do binário no pacote; o endurecimento da unit com o `cloudflared` real (`IPAddressDeny`/`IPAddressAllow` com a origem pelo `docker-proxy`, `MemoryDenyWriteExecute` e `SystemCallFilter` com o binário dele) e o `/ready` do `--metrics`. Faça o ensaio numa máquina que não seja a de produção antes de confiar neste script num incidente.

**Riscos que restam:** pepper errado só aparece no login da etapa 9; sem cópia externa, um disco perdido não tem o que restaurar; as migrações dependem de conferência humana; quem tem root na máquina lê o `api.env` temporário, o token e o `.cnf` do DBA; o `DROP DATABASE` continua possível pela conta DBA; trocar o servidor também afeta o CasaOS de dev.
