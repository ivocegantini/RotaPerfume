// Pipeline de deploy do rotaperfumes no servidor da LAN (ivo-inspiron-15-3530).
//
// Pre-requisitos no agente (o proprio servidor):
//   - docker + plugin "docker compose" v2;
//     usuario do Jenkins no grupo "docker".
//   - curl, install (coreutils), bash, openssl, git (SEC-62: o
//     codigo dos estagios de teste sai do git archive HEAD).
//   - DEPLOY-06: banco = container MariaDB 11.4 "rotaperfumes-mariadb" criado
//     por deploy/mariadb-docker.sh, so na rede Docker EXTERNA "rotaperfumes-db"
//     (172.30.42.0/24; MariaDB em 172.30.42.2). A API entra nela com IP fixo
//     172.30.42.10 (docker-compose.yml) e os clientes do pipeline (CA do
//     banco e Migracoes) com 172.30.42.20 (conta rotaperfumes_admin@'172.30.42.20').
//   - SEC-29/SEC-40: o certificado PUBLICO da CA do banco no HOST em
//     /etc/mysql/ssl/ca.pem (constante DB_TLS_CA_HOST_PATH abaixo), modo 644,
//     com o fingerprint SHA-256 listado em deploy/db-ca.fingerprint. O Jenkins
//     pode rodar em container (Docker do host pelo socket): o arquivo e montado
//     pelo daemon direto do host, nunca passa pelo workspace.
// Credenciais Jenkins:
//   - rotaperfumes-api-env   (Secret file)              -> deploy/api.env (ver deploy/api.env.example)
//   - rotaperfumes-db-admin  (Username with password)   -> Migracoes (DEPLOY-10)
//
// Regras: nunca "set -x", nunca cat de segredo, nunca "-e SEGREDO=valor".
// Todo "sh" comeca com #!/bin/bash: com shebang o Jenkins NAO aplica o
// "sh -xe" padrao (que ecoaria os comandos).

pipeline {
    agent any

    options {
        disableConcurrentBuilds()
        buildDiscarder(logRotator(numToKeepStr: '20'))
        timeout(time: 60, unit: 'MINUTES')
    }

    // O Jenkins da LAN nao e alcancavel pelo GitHub (sem webhook): consulta a
    // main a cada ~2 min e so roda quando ha commit novo. Builds disparados
    // assim usam os defaultValue dos parametros abaixo.
    triggers {
        pollSCM('H/2 * * * *')
    }

    parameters {
        string(name: 'SITE_HOST', defaultValue: 'ivo-inspiron-15-3530', description: 'Hostname de acesso na LAN')
        string(name: 'SITE_IP', defaultValue: '192.168.168.106', description: 'IP do servidor na LAN (tambem aceito pelo Caddy)')
        string(name: 'HTTPS_PORT', defaultValue: '8443', description: 'Porta HTTPS publicada pelo Caddy')
        string(name: 'TURNSTILE_SITE_KEY', defaultValue: '0x4AAAAAAFACjc-LtM9NWlvP', description: 'Site key PUBLICA do Cloudflare Turnstile (embutida no bundle do frontend)')
    }

    environment {
        COMPOSE_PROJECT_NAME = 'rotaperfumes'
        // SEC-31: imagens fixadas por digest (indice multi-arch) - a tag sozinha
        // pode ser republicada. Como atualizar:
        //   docker buildx imagetools inspect mariadb:11.4 --format '{{.Manifest.Digest}}'
        //   (golang: idem com golang:1.27-alpine e golang:1.27-trixie; versao do
        //   Go = go.mod, a mesma no GO_IMAGE e no GO_TEST_IMAGE)
        //   (node: idem com node:26.13-alpine; versao do Node = frontend/.nvmrc,
        //   a mesma linha 26 do NODE_VERSION do frontend/Dockerfile)
        //   -> trocar "<tag com patch>@sha256:<digest>" aqui; o do mariadb TAMBEM
        //   no IMAGEM_BASE dos deploy/mariadb-docker.sh, deploy/backup-banco.sh,
        //   deploy/verificar-backup.sh e deploy/restaurar-maquina.sh e no
        //   DB_CLIENT_IMAGE do deploy/seed-producao.sh (mesma string) -> make test
        //   -> no servidor: sudo bash deploy/backup-banco.sh --instalar
        //   -> sudo rotaperfumes-backup-banco --agora
        //   -> sudo bash deploy/mariadb-docker.sh --recriar.
        // "make deps-audit-imagens" mostra se ha digest novo (so informa).
        GO_IMAGE             = 'golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280'
        // CI-02: estagio "Testes Go". Debian porque os testes de tests/deploy
        // precisam de bash e gpg, que a imagem ja traz (sem apk/apt no estagio).
        GO_TEST_IMAGE        = 'golang:1.27.4-trixie@sha256:f18c3d73a184fb0582828510722e723b63a93f7188d11dd223d9be6f6880e65d'
        // TST-18: estagio "Testes Frontend" (npm ci + lint + vitest).
        NODE_IMAGE           = 'node:26.13.1-alpine@sha256:6bd9ab43c0ec0190ca7b2d193906cb8b7edf6e7f4afa43ddeccd6452710bffb5'
        // Cliente do MariaDB 11.4 LTS (mesma imagem do servidor); a imagem 11.x
        // so tem o comando "mariadb" (sem o symlink "mysql").
        DB_CLIENT_IMAGE      = 'mariadb:11.4.13@sha256:5e73baf8179b29f115101bb102763a47a017361f3e9792daccba6646c3ab1532'
        DB_NAME              = 'rotaperfumes'
        // DEPLOY-06: o banco so e alcancado pela rede Docker rotaperfumes-db,
        // pelo nome do container (que esta no SAN do certificado). Porta = a
        // interna do container (3306), nao uma porta publicada no host.
        DB_REDE              = 'rotaperfumes-db'
        DB_HOST_INTERNO      = 'rotaperfumes-mariadb'
        DB_HOST_PORT         = '3306'
        // IP do cliente do pipeline na rede (conta rotaperfumes_admin@'172.30.42.20').
        DB_CLIENTE_IP        = '172.30.42.20'
        // SEC-40: constante (deixou de ser parametro do job). O compose monta
        // o mesmo caminho na API; o fingerprint da CA e conferido contra
        // deploy/db-ca.fingerprint no estagio "CA do banco".
        DB_TLS_CA_HOST_PATH  = '/etc/mysql/ssl/ca.pem'
        // Sem PUBLIC_URL: o compose usa "/" e o frontend chama "/api/..." na
        // mesma origem, o que vale para a LAN e para o dominio do Cloudflare Tunnel.
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Testes Go') {
            steps {
                // Codigo enviado por stdin (tar) em vez de bind mount: funciona
                // tambem com Jenkins em container. Testes de integracao so rodam
                // com INTEGRATION=1, entao aqui sao pulados (sem MariaDB).
                // SEC-62: o tar vem do git archive HEAD (so o que esta rastreado):
                // .env, chaves e dumps deixados no workspace nunca entram, e um
                // caminho da lista que nao esteja rastreado derruba o estagio.
                // O layout do repositorio e preservado (backend/..., sql/, Makefile,
                // deploy/, scripts/, Jenkinsfile, .gitignore): cmdutil.FindProjectRoot
                // procura backend/shared/go.mod; os testes de config leem o Makefile e
                // os .sql da raiz, e os de tests/deploy e o sec37 leem deploy/*, o
                // Jenkinsfile e o .gitignore (falham de proposito se ausentes). O
                // scripts/ (setup-linux.sh, setup-windows.ps1, versoes.conf; sem
                // segredos) e lido pelo SEC-58 e pelo TST-17 (CI-01). CI=true faz o
                // arquivo ausente falhar em vez de pular (os testes leem so
                // os.Getenv("CI"); o docker run so repassa -e explicitos). O TST-18
                // le frontend/.nvmrc, frontend/Dockerfile, frontend/package*.json e
                // frontend/tests (estagio "Testes Frontend"). SEC-47/SEC-58 leem o
                // backend/Dockerfile e scripts/; BACKUP-01/SEC-51 leem o
                // docs/deploy-servidor.md; o ECOM-11 le o dados/erp/produtos.csv
                // (catalogo, sem segredos). Ficam de fora os dumps e o api.env
                // (segredo) do deploy/.
                // CI-02: bash e gpg faltando no GO_TEST_IMAGE falham o teste
                // (precisaBash, SEC-58); os testes com unshare continuam pulando no
                // container (o seccomp padrao do Docker bloqueia user namespace).
                sh '''#!/bin/bash
                    set -euo pipefail
                    command -v git >/dev/null || { echo "git ausente no agente (SEC-62)" >&2; exit 1; }
                    git archive --format=tar HEAD -- \
                        backend/shared backend/rotaperfumes-api sql Makefile deploy scripts Jenkinsfile .gitignore \
                        frontend/.nvmrc frontend/Dockerfile frontend/package.json frontend/package-lock.json frontend/tests \
                        backend/Dockerfile docs/deploy-servidor.md dados/erp/produtos.csv \
                      | docker run --rm -i -e CI=true -e CGO_ENABLED=0 -e GOFLAGS=-mod=readonly "$GO_TEST_IMAGE" sh -ec '
                          mkdir -p /src && tar -xf - -C /src
                          cd /src/backend/shared && go vet ./... && go test ./... -timeout=30m
                          cd /src/backend/rotaperfumes-api && go vet ./... && go test ./... -timeout=30m
                        '
                '''
            }
        }

        stage('Testes Frontend') {
            steps {
                // TST-18: mesmo padrao do "Testes Go" (codigo por stdin/tar, sem
                // bind mount). SEC-62: git archive HEAD, so o frontend/ rastreado
                // (o npm ci reinstala pelo lockfile, reprodutivel). Dois testes leem arquivos do backend
                // pelo layout do repositorio (../backend/...): o contrato de
                // erros do PedidoModal (backend/shared/vendas/erros.go) e a massa
                // cruzada do CNPJ (backend/shared/cnpj/testdata/casos_cruzados.json);
                // vao junto, no mesmo caminho relativo.
                sh '''#!/bin/bash
                    set -euo pipefail
                    command -v git >/dev/null || { echo "git ausente no agente (SEC-62)" >&2; exit 1; }
                    git archive --format=tar HEAD -- \
                        frontend backend/shared/vendas/erros.go backend/shared/cnpj/testdata/casos_cruzados.json \
                      | docker run --rm -i -e CI=true -e NEXT_TELEMETRY_DISABLED=1 "$NODE_IMAGE" sh -ec '
                          mkdir -p /src && tar -xf - -C /src
                          cd /src/frontend
                          npm ci --no-audit --no-fund
                          npm run lint
                          npm test
                        '
                '''
            }
        }

        stage('Build') {
            steps {
                // DEPLOY-10: o frontend embute o GOOGLE_CLIENT_ID (publico) no
                // bundle; so essa linha e lida do api.env, nunca os segredos.
                withCredentials([file(credentialsId: 'rotaperfumes-api-env', variable: 'API_ENV')]) {
                    dir('deploy') {
                        sh '''#!/bin/bash
                            set -euo pipefail
                            GOOGLE_CLIENT_ID=$(sed -n 's/^GOOGLE_CLIENT_ID=//p' "$API_ENV" | tail -n 1 | tr -d '\\r"' | tr -d "'")
                            case "$GOOGLE_CLIENT_ID" in
                              ''|*.apps.googleusercontent.com) ;;
                              *) echo "GOOGLE_CLIENT_ID do api.env fora do formato <id>.apps.googleusercontent.com" >&2; exit 1 ;;
                            esac
                            [ -n "$GOOGLE_CLIENT_ID" ] || echo "AVISO: GOOGLE_CLIENT_ID vazio no api.env: login com Google desligado no frontend."
                            export GOOGLE_CLIENT_ID
                            API_ENV_FILE=/dev/null docker compose -p "$COMPOSE_PROJECT_NAME" build --pull
                        '''
                    }
                }
            }
        }

        stage('CA do banco') {
            steps {
                // SEC-29/SEC-40: valida a CA do HOST (DB_TLS_CA_HOST_PATH) dentro de
                // um container descartavel (DB_CLIENT_IMAGE tem bash + openssl),
                // como uid 65532 (o da API), com deploy/validar-ca-banco.sh via
                // stdin: so blocos CERTIFICATE, cada um valido, nao vencido e com
                // fingerprint listado em deploy/db-ca.fingerprint, e s_client
                // -starttls mysql pela rede rotaperfumes-db em
                // rotaperfumes-mariadb:3306 com -verify_hostname rotaperfumes-mariadb.
                // Roda ANTES das Migracoes. O sha256 sai em
                // deploy/db-ca.sha256 (label do compose, rotacao).
                sh '''#!/bin/bash
                    set -euo pipefail
                    rm -f deploy/db-ca.sha256
                    # SEC-40: fingerprints aceitos (versionados); o script falha se
                    # algum certificado do arquivo do host nao estiver na lista.
                    [ -r deploy/db-ca.fingerprint ] || { echo "deploy/db-ca.fingerprint ausente (SEC-40)" >&2; exit 1; }
                    # DEPLOY-07: so comentarios (instalacao nova sem o fingerprint
                    # commitado) = falha com mensagem clara, antes do container.
                    # "|| true": grep sem linha sai 1 e o pipefail derrubaria mudo.
                    FPS=$(grep -Ev '^[[:space:]]*(#|$)' deploy/db-ca.fingerprint || true)
                    if [ -z "$FPS" ]; then
                      echo "deploy/db-ca.fingerprint sem fingerprint: rode mariadb-docker.sh --nova-ca e grave o CA_FP (DEPLOY-07)" >&2
                      exit 1
                    fi
                    echo "Fingerprints SHA-256 aceitos para a CA do banco:"
                    printf '%s\\n' "$FPS" | sed 's/^/  /'
                    # DEPLOY-06: s_client em rotaperfumes-mariadb:3306 pela rede
                    # interna, conferindo o mesmo nome (-verify_hostname).
                    export CA_FINGERPRINTS="$(cat deploy/db-ca.fingerprint)" \\
                           DB_ALVO="$DB_HOST_INTERNO" DB_HOST_PORT=3306 TLS_NOME="$DB_HOST_INTERNO"
                    # --mount (e nao -v): caminho inexistente no host = erro, e
                    # nao um diretorio criado no lugar do arquivo.
                    # DEPLOY-08: --ip $DB_CLIENTE_IP (conta rotaperfumes_admin):
                    # de um IP sem conta o mariadbd manda o erro 1130 antes do
                    # greeting e o s_client nao faz handshake.
                    SHA=$(docker run --rm -i --user 65532:65532 --network "$DB_REDE" --ip "$DB_CLIENTE_IP" \\
                            --mount "type=bind,source=$DB_TLS_CA_HOST_PATH,target=/etc/rotaperfumes/db-ca.pem,readonly" \\
                            -e DB_ALVO -e DB_HOST_PORT -e TLS_NOME -e CA_FINGERPRINTS \\
                            --entrypoint bash "$DB_CLIENT_IMAGE" -s < deploy/validar-ca-banco.sh)
                    [[ "$SHA" =~ ^[0-9a-f]{64}$ ]] || { echo "CA do banco: sha256 inesperado da validacao" >&2; exit 1; }
                    echo "$SHA" > deploy/db-ca.sha256
                    echo "CA do banco validada: $DB_TLS_CA_HOST_PATH (sha256 $SHA)"
                '''
            }
        }

        stage('Migracoes') {
            // DEPLOY-10: migracoes da loja (ECOM-01/03/05/06/09/11/14), idempotentes,
            // antes de subir a API nova (sem elas a API nova falha nas colunas novas).
            // Depois dos testes: build vermelho nao altera o banco da API em uso.
            steps {
                withCredentials([usernamePassword(credentialsId: 'rotaperfumes-db-admin',
                                                  usernameVariable: 'DB_ADMIN_USER',
                                                  passwordVariable: 'MYSQL_PWD')]) {
                    sh '''#!/bin/bash
                        set -euo pipefail
                        [ -r deploy/db-ca.sha256 ] || { echo "Estagio CA do banco nao validou a CA" >&2; exit 1; }
                        MIGRACOES="27_alter_usuarios_role_usuario.sql 28_ddl_confirmacoes_email.sql 29_alter_usuarios_email_confirmacao.sql 30_alter_clientes_loja.sql 31_alter_clientes_numero_complemento.sql 33_alter_pedidos_canal_loja.sql 34_alter_produtos_nome_descricao.sql 35_alter_pedidos_chave_checkout.sql 36_alter_pedidos_endereco_entrega.sql 37_alter_pagamento_loja.sql"
                        for f in $MIGRACOES; do
                          [ -r "sql/$f" ] || { echo "Migracao ausente: sql/$f" >&2; exit 1; }
                        done
                        for f in $MIGRACOES; do
                          echo "Migracao sql/$f em $DB_NAME..."
                          # Cliente admin (DEPLOY-06): rede
                          # rotaperfumes-db, IP da conta admin, TLS verificado,
                          # senha so por MYSQL_PWD no ambiente.
                          docker run --rm -i \
                            --network "$DB_REDE" --ip "$DB_CLIENTE_IP" \
                            --mount "type=bind,source=$DB_TLS_CA_HOST_PATH,target=/etc/rotaperfumes/db-ca.pem,readonly" \
                            -e MYSQL_PWD \
                            "$DB_CLIENT_IMAGE" \
                            mariadb --local-infile=0 -h "$DB_HOST_INTERNO" -P "$DB_HOST_PORT" -u "$DB_ADMIN_USER" \
                                  --ssl-ca=/etc/rotaperfumes/db-ca.pem --ssl-verify-server-cert \
                                  --default-character-set=utf8mb4 "$DB_NAME" < "sql/$f"
                        done
                        echo "Migracoes concluidas."
                    '''
                }
            }
        }

        stage('Deploy') {
            steps {
                withCredentials([file(credentialsId: 'rotaperfumes-api-env', variable: 'API_ENV')]) {
                    sh '''#!/bin/bash
                        set -euo pipefail
                        install -m 600 "$API_ENV" deploy/api.env
                    '''
                }
                dir('deploy') {
                    // SEC-29 (rotacao da CA): o hash (do estagio "CA do banco")
                    // vira label do servico api; CA nova = label nova = container
                    // recriado (o bind de um arquivo fica preso ao inode antigo e
                    // o "up" nao recria so por mudanca de conteudo). O compose
                    // monta DB_TLS_CA_HOST_PATH (constante do environment, SEC-40)
                    // e liga a API na rede externa rotaperfumes-db (DEPLOY-06).
                    sh '''#!/bin/bash
                        set -euo pipefail
                        [ -r db-ca.sha256 ] || { echo "deploy/db-ca.sha256 ausente (estagio CA do banco)" >&2; exit 1; }
                        DB_CA_SHA256=$(cat db-ca.sha256)
                        export DB_CA_SHA256
                        echo "CA do banco no deploy: sha256 $DB_CA_SHA256"
                        docker compose -p "$COMPOSE_PROJECT_NAME" up -d --remove-orphans
                    '''
                }
            }
        }

        stage('Smoke test') {
            steps {
                // Acessa direto pelo IP da LAN (o Caddy aceita {$SITE_IP} e usa
                // default_sni para o IP): o hostname fica bloqueado no smoke test.
                sh '''#!/bin/bash
                    set -uo pipefail
                    BASE="https://${SITE_IP}:${HTTPS_PORT}"

                    check() {
                      curl -fsSk --max-time 5 -o /dev/null -w "%{http_code}" "$BASE$1"
                    }

                    # Como o cloudflared acessa (visto nos logs do Caddy): IP da
                    # LAN, sem SNI, com o Host do dominio publico -> site curinga.
                    # Nao use localhost: o Jenkins roda em container e o
                    # loopback dele nao e o do servidor.
                    tunnel() {
                      curl -fsSk --max-time 5 -o /dev/null -w "%{http_code}" \
                        -H "Host: smoke-tunnel.invalid" "$BASE$1"
                    }

                    for i in $(seq 1 30); do
                      api=$(check /api/health || true)
                      front=$(check /login || true)
                      tun=$(tunnel /api/health || true)
                      if [ "$api" = "200" ] && [ "$front" = "200" ] && [ "$tun" = "200" ]; then
                        echo "Smoke test OK: /api/health=$api /login=$front tunnel=$tun"
                        exit 0
                      fi
                      echo "Aguardando servicos (tentativa $i/30): /api/health=${api:-erro} /login=${front:-erro} tunnel=${tun:-erro}"
                      sleep 3
                    done

                    echo "Smoke test FALHOU. Ultimos logs:" >&2
                    cd deploy && API_ENV_FILE=/dev/null docker compose -p "$COMPOSE_PROJECT_NAME" logs --tail 40 api frontend caddy >&2 || true
                    exit 1
                '''
            }
        }
    }

    post {
        always {
            sh '''#!/bin/bash
                rm -f deploy/api.env deploy/db-ca.sha256
            '''
        }
    }
}
