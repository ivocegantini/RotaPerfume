# TLS do banco (MariaDB em Docker): passo a passo

> Roteiro para **você** executar no servidor de produção (`ivo-inspiron-15-3530`, `192.168.168.106`), no HOST, como root, nunca dentro de um container. O único caminho oficial é o `deploy/mariadb-docker.sh`.
> Não cole `*.senha` nem `ca-key.pem` em chat, log ou ticket. Primeira instalação: siga a [§11 do `docs/deploy-servidor.md`](deploy-servidor.md#11-instalação-inicial-da-produção-deploy-07), que usa estes passos na ordem certa.
> **Duas CAs, nunca misturadas:** produção = `CN=RotaPerfumes_MariaDB_CA` em `/etc/mysql/ssl/ca.pem`, única em `deploy/db-ca.fingerprint`; dev = CasaOS :3307 em `deploy/ca-cert.pem` (`CN=MariaDB_CA`, `d150ee67...`). A CA de dev anterior (`ef31b656...`) continua proibida.

## 0. Visão geral

| Item | Valor |
|---|---|
| Banco | MariaDB 11.4 LTS, container `rotaperfumes-mariadb`, imagem `rotaperfumes-mariadb:11.4-<12 hex do digest>` sobre `mariadb:11.4.13@sha256:5e73baf8…` (SEC-31) |
| Rede | `rotaperfumes-db`, `172.30.42.0/24`: MariaDB `.2`, API `.10`, admin/import `.20`, backup `.21`, DBA `.22` (dinâmicos em `.128/25`) |
| Porta no host | nenhuma |
| TLS no servidor | `require_secure_transport = ON`, `tls_version = TLSv1.2,TLSv1.3`; cliente Go exige TLS 1.3 (TST-12) |
| Contas | `IDENTIFIED VIA ed25519` + `REQUIRE SSL`; `root` só `localhost` |
| Certificados (host) | `/etc/mysql/ssl/` (`ca.pem`, `server-cert.pem` 644; `server-key.pem` 600, dono = uid do `mysql` da imagem), read-only no container |
| CA | própria, 10 anos, RSA 3072; **chave fora do servidor**, lida com `--ca-key` |
| Certificado do servidor | 2 anos, SAN `IP:192.168.168.106, DNS:rotaperfumes-mariadb, DNS:localhost, IP:127.0.0.1, DNS:<hostname>` |
| Senhas | `/root/rotaperfumes-mariadb/*.senha` (700/600); DBA em `dba-cliente/conta.senha` |

Confiam na CA: a **API** (`DB_TLS_CA=/etc/rotaperfumes/db-ca.pem`, montado do host; `DB_HOST` e `DB_TLS_SERVER_NAME` = `rotaperfumes-mariadb`, fixados no compose), o **Jenkins** (`CA do banco` e `Migracoes`) e o **DBA** (`sudo rotaperfumes-dba`).

## 1. Opções do script

| Opção | Uso |
|---|---|
| `--ca-key ARQ` | chave da CA na mídia offline, lida de lá e nunca copiada; exigida para (re)emitir o certificado |
| `--nova-ca` | CA nova (primeira instalação ou troca); implica `--novo-cert`; backup em `/etc/mysql/ssl.bak-AAAAMMDD-HHMMSS` |
| `--novo-cert` | reemite só o certificado do servidor (mesma CA) |
| `--recriar` | recria o container (dados em `/var/lib/rotaperfumes/mariadb`); exige dump recente ou `--sem-backup-motivo TEXTO`; `--trocar-dados` para outro datadir |
| `--imagem-base REF` | outra imagem base, `mariadb:<versão>@sha256:<64 hex>` |
| `--sem-read-only` | só se o container não subir com `--read-only` |

`--dev-ip`, `--porta` e `--casaos-native-motivo` são recusadas. A conta `casaos` sai em qualquer host, a cada execução. Veja `sudo bash deploy/mariadb-docker.sh --help`.

## 2. Primeira instalação: CA e certificado do servidor

| Caso | Situação | Comando |
|---|---|---|
| A | sem CA de produção (primeira instalação) | `sudo bash deploy/mariadb-docker.sh --nova-ca` |
| B | CA em `/etc/mysql/ssl/ca.pem` e chave na mídia offline | `sudo bash deploy/mariadb-docker.sh --ca-key /media/<pendrive>/ca-key.pem` |

- Certificado válido (CA e chave conferem, SAN exato, > 30 dias) é reaproveitado. Senão, emite RSA 3072, serial aleatório, 730 dias, `serverAuth`, conferido com `openssl verify`.
- Sem a chave da CA, ou com CA que vence antes de 730 dias, o script aborta. Avisa se o uid do `mysql` da imagem existir no host.

```bash
sudo openssl x509 -in /etc/mysql/ssl/server-cert.pem -noout -subject -issuer -enddate -ext subjectAltName
sudo openssl verify -CAfile /etc/mysql/ssl/ca.pem /etc/mysql/ssl/server-cert.pem      # server-cert.pem: OK
openssl x509 -in /etc/mysql/ssl/ca.pem -noout -fingerprint -sha256 | cut -d= -f2 | tr -d : | tr A-F a-f
```

O último valor precisa estar em `deploy/db-ca.fingerprint` (seção 6), senão o `CA do banco` recusa o deploy. Nunca reaproveite a CA de dev em produção.

## 3. Chave da CA fora do servidor (mídia offline)

Com a chave, qualquer um emite um certificado que a API aceita. No caso A, o script a grava em `/root/rotaperfumes-ca/ca-key.pem` e avisa a cada execução até ela sair:

```bash
sudo cp /root/rotaperfumes-ca/ca-key.pem /media/<pendrive-cifrado>/ca-key.pem
sudo cp /etc/mysql/ssl/ca.pem /root/rotaperfumes-ca/server-ext.cnf /media/<pendrive-cifrado>/
sudo cmp /root/rotaperfumes-ca/ca-key.pem /media/<pendrive-cifrado>/ca-key.pem && \
  sudo shred -u /root/rotaperfumes-ca/ca-key.pem
```

Guarde uma segunda cópia (outro pendrive cifrado ou cofre). Sem a chave, renovar o certificado exige trocar a CA (9.2).

## 4. Verificar

```bash
IMG='mariadb:11.4.13@sha256:5e73baf8179b29f115101bb102763a47a017361f3e9792daccba6646c3ab1532'   # = IMAGEM_BASE do script
docker run --rm --network rotaperfumes-db --ip 172.30.42.20 --cap-drop ALL \
  --mount type=bind,source=/etc/mysql/ssl/ca.pem,target=/ca.pem,readonly \
  --entrypoint openssl "$IMG" s_client -starttls mysql -connect rotaperfumes-mariadb:3306 \
  -CAfile /ca.pem -verify_return_error -verify_hostname rotaperfumes-mariadb </dev/null 2>/dev/null \
  | grep -E 'Verify return code|Cipher is'
# esperado: "Cipher is TLS_AES_..." (nunca "(NONE)") e "Verify return code: 0 (ok)"
```

- Exija as duas linhas: origem sem conta (erro 1130) dá `Cipher is (NONE)` com `Verify return code: 0`. Não rode junto com outro cliente no `.20`.
- A etapa `[8/8]` do script já testa TLS verificado com `rotaperfumes_admin` e aborta se `--skip-ssl` for aceito.

```bash
sudo docker exec rotaperfumes-mariadb mariadb --defaults-extra-file=/run/rotaperfumes/root.cnf -e "
  SELECT User, Host, JSON_VALUE(Priv,'$.plugin') AS plugin, JSON_VALUE(Priv,'$.ssl_type') AS ssl_type
  FROM mysql.global_priv WHERE User <> 'mariadb.sys' ORDER BY User, Host;
  SHOW GLOBAL VARIABLES WHERE Variable_name IN ('require_secure_transport','tls_version','have_ssl','ssl_cert');"
```

Esperado: as quatro contas da seção 8 com `ed25519` e `ssl_type = 1`, nenhuma `casaos`, `root@localhost`, `require_secure_transport = ON`. `echo 'SELECT CURRENT_USER()' | sudo rotaperfumes-dba` traz `rotaperfumes-admin@172.30.42.22`.
Da LAN, `timeout 5 bash -c '</dev/tcp/192.168.168.106/3306' && echo ABERTA || echo fechada` precisa dar `fechada`.

## 5. Permissão para a API e o Jenkins lerem a CA (no HOST)

A API e o `CA do banco` rodam com uid 65532, que não existe no host: `ca.pem` legível por "outros" e diretórios atravessáveis. Só o `ca.pem` é montado; a `server-key.pem` segue 600.

```bash
sudo chmod 755 /etc/mysql /etc/mysql/ssl
sudo chmod 644 /etc/mysql/ssl/ca.pem
sudo -u nobody cat /etc/mysql/ssl/ca.pem >/dev/null && echo "legivel por outros: OK"
```

## 6. Jenkins: caminho da CA e fingerprint aceito (SEC-40)

- `DB_TLS_CA_HOST_PATH` é constante no `Jenkinsfile` (`/etc/mysql/ssl/ca.pem`, caminho do host); o nome conferido é sempre `rotaperfumes-mariadb`.
- `deploy/db-ca.fingerprint`: SHA-256 do DER de cada certificado aceito, hex minúsculo, um por linha (no máximo dois, só na troca). Um certificado fora da lista falha o `validar-ca-banco.sh`.
- Hoje a linha útil é a CA em uso (`f31ee820...`). `TestSEC45_FingerprintVersionadoConfereComCA` proíbe as CAs de dev; sem linha útil, o estágio falha com `deploy/db-ca.fingerprint sem fingerprint: ...`.

Secret file `rotaperfumes-api-env` (modelo `deploy/api.env.example`): `DB_HOST=rotaperfumes-mariadb`, `DB_PORT=3306`, `DB_NAME=rotaperfumes`, `DB_USUARIO=rotaperfumes_app`, `DB_SENHA=<rotaperfumes_app.senha>`. O compose fixa `DB_HOST`/`DB_PORT`/`DB_TLS_CA`/`DB_TLS_SERVER_NAME`; nunca `DB_TLS_REQUIRED=false` (SEC-42).
Credencial `rotaperfumes-db-admin`: usuário `rotaperfumes_admin`, senha de `/root/rotaperfumes-mariadb/rotaperfumes_admin.senha` (lida com `sudo cat` e colada direto no Jenkins).

## 7. Primeiro deploy e conferência

Pré-condições: container e rede `rotaperfumes-db` no ar, fingerprint commitado, Secret file e credencial atualizados.

1. Push na `main` (o `pollSCM` dispara em até ~2 min). O `CA do banco` deve mostrar `na lista (deploy/db-ca.fingerprint)`, `s_client: Verify return code: 0 (ok)` e `CA do banco validada: ...`.
2. No host: `docker logs rotaperfumes-api-1 2>&1 | grep -E 'TLS do banco|conexão MariaDB estabelecida'` → `TLS do banco: ligado (... min=TLS1.3)` e `tls=ligado (custom)`.

Se o `CA do banco` falhar, a mensagem diz o motivo; ele roda antes de qualquer mudança.

## 8. Contas, ed25519 e `REQUIRE SSL`

A cada execução, pelo stdin: `CREATE USER IF NOT EXISTS ... IDENTIFIED VIA ed25519 ... REQUIRE SSL`, `ALTER USER`, `REVOKE ALL` + `GRANT`. Remove contas do projeto com host fora do conjunto e a `casaos`; falha se alguma ficar sem `REQUIRE SSL`.

| Conta | Host | Privilégios | Usado por |
|---|---|---|---|
| `rotaperfumes_app` | `172.30.42.10` | `SELECT, INSERT, UPDATE, DELETE ON rotaperfumes.*` | API (`DB_SENHA` do Secret file) |
| `rotaperfumes_admin` | `172.30.42.20` | `ALL ON rotaperfumes.*` | Jenkins (credencial `rotaperfumes-db-admin`), `deploy/seed-producao.sh` e migrações. **Não é o DBA** |
| `rotaperfumes_backup` | `172.30.42.21` | `SELECT ON rotaperfumes.*` | rotina de backup (`rotaperfumes-backup-banco`) |
| `rotaperfumes-admin` | `172.30.42.22` | `ALL ON rotaperfumes.*`, sem privilégio global, `GRANT OPTION`, `PROCESS` ou `mysql.*` | DBA: `sudo rotaperfumes-dba`. **Não é o `rotaperfumes_admin`** |

O cliente precisa do plugin `client_ed25519` (cliente `mariadb` 11.x, Connector/C, driver Go); Workbench e o `mysql` do Oracle não têm.

## 9. Rotação

Antes, faça backup (o `--novo-cert` sozinho não faz): `sudo cp -a /etc/mysql/ssl "/etc/mysql/ssl.bak-$(date +%Y%m%d-%H%M%S)"`.

### 9.1 Renovar só o certificado do servidor (mesma CA; a cada ~2 anos)

```bash
sudo bash deploy/mariadb-docker.sh --novo-cert --ca-key /media/<pendrive>/ca-key.pem
```

Recarrega sem reiniciar (`FLUSH SSL`). Execução comum também reemite com < 30 dias ou SAN mudado. Nada muda no Jenkins: a CA e o fingerprint são os mesmos.

### 9.2 Trocar a CA (`--nova-ca`; a cada ~10 anos ou se a chave vazar)

A API segue com a CA antiga até ser recriada pelo Jenkins: conexões novas falham entre a troca e o deploy. Faça numa janela curta.

1. Backup e `sudo bash deploy/mariadb-docker.sh --nova-ca`; anote o fingerprint.
2. Acrescente o fingerprint novo em `deploy/db-ca.fingerprint` (no máximo dois), commit e push: a label do sha256 muda e a API é recriada. O `deploy/ca-cert.pem` (dev) não muda.
3. Tire a chave nova do servidor (seção 3) e, num commit seguinte, remova o fingerprint antigo.

## 10. Rollback

- **Certificado renovado com problema:** restaure e recarregue (o script sem `--novo-cert` não desfaz):
  ```bash
  B=/etc/mysql/ssl.bak-AAAAMMDD-HHMMSS
  sudo cp -a "$B/server-cert.pem" "$B/server-key.pem" /etc/mysql/ssl/
  sudo docker exec rotaperfumes-mariadb mariadb --defaults-extra-file=/run/rotaperfumes/root.cnf -e 'FLUSH SSL'
  ```
- **CA trocada com problema:** restaure `ca.pem`, `server-cert.pem` e `server-key.pem` do backup, rode o `FLUSH SSL`, garanta o fingerprint antigo no arquivo e rode o job.
- **Imagem nova com problema:** `sudo bash deploy/mariadb-docker.sh --recriar --imagem-base '<string anterior>'` e reverta o commit; só patch dentro do 11.4 (downgrade de versão menor: restaure o backup).
- Não há volta para "sem TLS". Instalação inicial ainda sem dados reais: refaça com `deploy/seed-producao.sh --forcar` (`docs/deploy-servidor.md` §11, passo 6).

## 11. DEV (CasaOS) e acesso do DBA à produção

- **Dev:** MariaDB 11.8 do CasaOS, porta **3307**, CA `deploy/ca-cert.pem` (não versionada). O `backend/.env` padrão aponta para ele (`DB_HOST=192.168.168.106`, `DB_PORT=3307`, `DB_TLS_CA=deploy/ca-cert.pem`, como no `backend/.env.example`).
- **Produção:** só local, por SSH no servidor com `sudo rotaperfumes-dba` (conta `rotaperfumes-admin`, `172.30.42.22`), sem porta publicada (`docs/deploy-servidor.md`, seção 3.4).
- `casaos.senha`: nenhum script o lê ou apaga; leve-o ao pendrive seguro. Nunca copie a CA de produção para `deploy/ca-cert.pem`. Testes e recriação de banco só em dev ou descartável.
