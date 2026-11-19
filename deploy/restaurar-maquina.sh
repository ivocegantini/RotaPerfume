#!/usr/bin/env bash
# =============================================================================
# DEPLOY-09: devolve o site numa MAQUINA NOVA, a partir do par de backup (dump +
# imagens), do pepper e de uma copia do Secret file. Nunca roda sobre o servidor
# em uso: la, a restauracao continua sendo so o runbook (decisao D7 de
# tarefas/lote-42-backup-01-spec.md; deploy/restaurar-banco.sh segue proibido).
#
#   sudo bash deploy/restaurar-maquina.sh --dump ARQ.sql.gz --imagens ARQ.tar.gz \
#     --api-env ARQ --ip IP --host-nome NOME --hostname-publico H \
#     (--ca-cert A --ca-key B | --nova-ca) \
#     [--pepper-file ARQ] [--token-file ARQ] [--tmpfs-mb N] [--sem-tunel] \
#     [--backup-dir D --exigir-montagem P] [--par-divergente-motivo TEXTO]
#   sudo bash deploy/restaurar-maquina.sh --listar-manuais
#
# Execucao manual, com terminal, de um clone limpo da main. O script nao se
# instala em /usr/local/sbin: quem o roda confere o clone.
#
# Recusas, antes de qualquer mudanca. NENHUMA flag, variavel ou deteccao de teste
# as dispensa:
#   execucao      sem root, sem terminal, dentro do Jenkins
#   clone         origin, HEAD ou arvore fora do esperado (ignorados tambem contam)
#   producao      qualquer sinal de servidor em uso (conferir_producao_ativa)
#   IP            --ip nao configurado nesta maquina
#   banco         qualquer tabela em rotaperfumes depois do mariadb-docker.sh
#   entradas      dump, tar, CA, segredos e api.env (ver as funcoes conferir_*)
#
# Etapas (o numero e o usado no estado e na documentacao):
#    0 recusas, pepper, espaco
#    1 copia para entrada/ e verifica a COPIA (verificar-backup, conferir-imagens)
#    2 mostra o que vai restaurar; digitar o hostname (e ANTIGA PARADA, com tunel)
#    3 CA e mariadb-docker.sh, sem --recriar
#    4 api.env novo: DB_SENHA e JWT_SECRET novos, pepper informado
#    5 importa pela conta rotaperfumes-admin, compara contagens, zera refresh_tokens
#    6 migracoes: confirmar
#    7 valida a CA, build e "up --no-start"
#    8 devolve as imagens ao volume recem-criado
#    9 sobe o site e faz o smoke; confirmar login e produto com imagem
#   10 instala a rotina de backup
#   11 cloudflared (com --sem-tunel, pula)
#   12 lista do Jenkins; confirmar; so entao apaga api.env e entrada/
#
# Estado: /root/rotaperfumes-restauracao/estado (600), lido sem "source". Nasce
# depois da etapa 2. Retomar exige o mesmo machine-id e os mesmos sha256 e libera
# so as recusas de "producao"; "concluida" recusa sempre.
#
# Regras: nunca "set -x"; pepper e token ficam fora de argv, ambiente de filhos,
# log e tela; a senha do banco fica so no option file montado (nunca MYSQL_PWD);
# erros de importacao mostram so o codigo e a linha, nunca o texto do servidor.
# Codigos de saida: 0 sucesso, 2 uso incorreto, 1 recusa ou falha.
# =============================================================================
set -euo pipefail
umask 077
export LC_ALL=C
ulimit -c 0 2>/dev/null || true
# Nada de configuracao de git vinda do ambiente: o clone e o que esta em disco.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES \
  GIT_CONFIG GIT_CONFIG_GLOBAL GIT_CONFIG_SYSTEM GIT_CONFIG_PARAMETERS GIT_CONFIG_COUNT \
  GIT_EXEC_PATH GIT_SSH GIT_SSH_COMMAND GIT_ASKPASS GIT_PROXY_COMMAND
# Achado do SecBrain (Lote 42): o docker e o daemon local, nunca um remoto ou
# outro contexto/config herdado do ambiente.
unset DOCKER_HOST DOCKER_CONTEXT DOCKER_CONFIG

# SEC-31: a mesma string do IMAGEM_BASE do deploy/mariadb-docker.sh (teste de
# paridade em backend/shared/tests/deploy).
IMAGEM_BASE="mariadb:11.4.13@sha256:5e73baf8179b29f115101bb102763a47a017361f3e9792daccba6646c3ab1532"
readonly IMAGEM_BASE

readonly REPO_URL="https://github.com/residentivo/RotaPerfume.git"
RAIZ=$(cd "$(dirname "$0")/.." && pwd -P)
readonly RAIZ
readonly PROJETO="rotaperfumes"
readonly PORTA_HTTPS="8443"
# Valores versionados hoje: outro IP ou nome exige os commits de listar_ip_commits.
readonly IP_VERSIONADO="192.168.168.106"
readonly HOST_VERSIONADO="ivo-inspiron-15-3530"

readonly TRAB="/root/rotaperfumes-restauracao"
readonly ENTRADA="$TRAB/entrada"
readonly ESTADO="$TRAB/estado"
readonly REGISTRO="$TRAB/registro"
readonly CONTAGENS="$TRAB/contagens"
readonly API_ENV_NOVO="$TRAB/api.env"
readonly CA_SHA_ARQ="$TRAB/db-ca.sha256"

# Os mesmos caminhos do deploy/mariadb-docker.sh e do deploy/backup-banco.sh.
readonly CONTAINER_DB="rotaperfumes-mariadb"
readonly REDE_DB="rotaperfumes-db"
readonly IP_ADMIN="172.30.42.20"
readonly IP_DBA="172.30.42.22"
readonly DB_NOME="rotaperfumes"
readonly DADOS_DIR="/var/lib/rotaperfumes/mariadb"
readonly SSL_DIR="/etc/mysql/ssl"
readonly SEGREDOS_DIR="/root/rotaperfumes-mariadb"
readonly CA_DIR="/root/rotaperfumes-ca"
readonly DBA_CNF="$SEGREDOS_DIR/dba-cliente/cliente.cnf"
readonly DBA_CMD="/usr/local/sbin/rotaperfumes-dba"
readonly DBA_CLIENTE="rotaperfumes-dba-cliente"
readonly C_DBA="/run/dba"
readonly SENHA_APP_ARQ="$SEGREDOS_DIR/rotaperfumes_app.senha"
readonly BACKUP_CMD="/usr/local/sbin/rotaperfumes-backup-banco"
readonly BACKUP_DIR_PADRAO="/var/backups/rotaperfumes"
readonly UNIT_DIR="/etc/systemd/system"
readonly VOLUME_IMAGENS="rotaperfumes_produto_imagens"
readonly VOLUME_ROTULO="rotaperfumes/produto_imagens"
readonly USUARIO_IMAGENS="65532:65532"
readonly C_IMAGENS="/imagens"

# cloudflared: repositorio apt com origem verificavel e unit propria.
readonly APT_CHAVE_ASC="$RAIZ/deploy/cloudflare-apt.asc"
readonly APT_CHAVE_FP="$RAIZ/deploy/cloudflare-apt.fingerprint"
readonly APT_KEYRING="/usr/share/keyrings/rotaperfumes-cloudflared.gpg"
readonly APT_LISTA="/etc/apt/sources.list.d/rotaperfumes-cloudflared.list"
readonly APT_PIN="/etc/apt/preferences.d/rotaperfumes-cloudflared"
readonly TUNEL_ENV="/etc/rotaperfumes/cloudflared.env"
readonly TUNEL_UNIT="rotaperfumes-cloudflared.service"
# Achado do SecBrain (Lote 42): a unit do tunel nao fala com a LAN. A origem
# (https://localhost:8443) e o DNS precisam ficar no loopback ou fora destas
# faixas; a etapa 11 confere o tunel e a origem com o mesmo filtro.
readonly TUNEL_IP_DENY="10.0.0.0/8 172.16.0.0/12 192.168.0.0/16 169.254.0.0/16"
readonly TUNEL_IP_ALLOW="127.0.0.0/8 ::1"
readonly TUNEL_FAMILIAS="AF_INET AF_INET6 AF_UNIX AF_NETLINK"
readonly TUNEL_METRICS="127.0.0.1:20241"

readonly PEPPER_MIN_BYTES=32
readonly RESERVA_BYTES=$((1024 * 1024 * 1024))
readonly RE_CARIMBO='[0-9]{8}-[0-9]{6}Z(-manual)?'
readonly RE_DUMP="^rotaperfumes-($RE_CARIMBO)\\.sql\\.gz\$"
readonly RE_IMAGENS="^rotaperfumes-imagens-($RE_CARIMBO)\\.tar\\.gz\$"
readonly RE_HOST='^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$'

# CAs de DEV (MariaDB do CasaOS): as mesmas do CA_DEV_FPS do mariadb-docker.sh
# (teste de paridade). Nunca valem em producao.
readonly -a CA_DEV_FPS=(
  "ef31b6565518b18bbd5ababf94db6bca76de356f8e0f013491dbec7f5aedb37b"
  "d150ee67f22336406ea5f91957b6935009a163fc4727b537a00d15da01fe85d2"
)

usage() {
  cat <<EOF
Uso:
  sudo bash $0 --dump ARQ.sql.gz --imagens ARQ.tar.gz --api-env ARQ \\
    --ip IP --host-nome NOME --hostname-publico H \\
    (--ca-cert A --ca-key B | --nova-ca) [opcoes]
  sudo bash $0 --listar-manuais

Maquina NOVA, com terminal, de um clone limpo da main. Recusa o servidor em uso.

  --dump ARQ              rotaperfumes-*.sql.gz do backup (arquivo regular)
  --imagens ARQ           rotaperfumes-imagens-*.tar.gz do mesmo par
  --api-env ARQ           copia do Secret file rotaperfumes-api-env (root, 600/400)
  --ip IP                 IP da LAN desta maquina
  --host-nome NOME        nome desta maquina na LAN
  --hostname-publico H    hostname publico do tunel (ex.: site.exemplo.com.br)
  --ca-cert A --ca-key B  reaproveita a CA do banco (fingerprint em
                          deploy/db-ca.fingerprint; a chave fica onde esta)
  --nova-ca               gera uma CA nova, se a chave se perdeu
  --pepper-file ARQ       PASSWORD_PEPPER na 1a linha (root, 600/400); sem a opcao,
                          e pedido no terminal, sem eco e com confirmacao
  --token-file ARQ        TUNNEL_TOKEN na 1a linha (root, 600/400); sem a opcao,
                          e pedido no terminal, sem eco, na etapa 11
  --tmpfs-mb N            repassado ao verificar-backup.sh
  --sem-tunel             ensaio: sem cloudflared, token nem declaracao sobre a
                          maquina antiga; a escolha fica registrada
  --backup-dir D --exigir-montagem P
                          destino do backup em outro disco (repassados ao
                          backup-banco.sh --instalar)
  --par-divergente-motivo TEXTO
                          aceita carimbos diferentes no par; 10 a 300 caracteres
                          imprimiveis, registrados
  --listar-manuais        lista o que fica fora do script (so nomes, nunca valores)
  -h, --help              esta ajuda

Codigos de saida: 0 sucesso, 2 uso incorreto, 1 recusa ou falha.
EOF
}

uso_errado() { printf 'ERRO: %s\n' "$*" >&2; usage >&2; exit 2; }
die() { printf 'RECUSADO: %s\n' "$*" >&2; exit 1; }
info() { printf '  %s\n' "$*"; }
aviso() { printf '  AVISO: %s\n' "$*" >&2; }
etapa() { printf '\n=== [%s/12] %s ===\n' "$1" "$2"; }

# Opcoes
OPC_DUMP=""
OPC_IMAGENS=""
OPC_API_ENV=""
SITE_IP=""
SITE_HOST=""
HOSTNAME_PUBLICO=""
OPC_CA_CERT=""
OPC_CA_KEY=""
NOVA_CA=false
OPC_PEPPER=""
OPC_TOKEN=""
OPC_TMPFS=""
SEM_TUNEL=false
OPC_BACKUP_DIR=""
OPC_MONTAGEM=""
MOTIVO_PAR=""
LISTAR=false

# Estado em memoria
RETOMADA=false
ETAPA_FEITA=-1
IMPORTACAO="nao"
SHA_DUMP=""
SHA_IMAGENS=""
MACHINE_ID=""
ENTRADA_DUMP=""
ENTRADA_IMAGENS=""
IMG_ARQUIVOS=""
PEPPER=""
TOKEN=""
LENDO_SEGREDO=false
GPG_DIR=""
ERRO_ARQ=""
TUNEL_GRAVADO=false
declare -A ENV_VAL=()

exigir_valor() { [[ $# -ge 2 && -n "$2" ]] || uso_errado "a opcao $1 exige um valor"; }

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dump) exigir_valor "$@"; OPC_DUMP=$2; shift 2 ;;
      --imagens) exigir_valor "$@"; OPC_IMAGENS=$2; shift 2 ;;
      --api-env) exigir_valor "$@"; OPC_API_ENV=$2; shift 2 ;;
      --ip) exigir_valor "$@"; SITE_IP=$2; shift 2 ;;
      --host-nome) exigir_valor "$@"; SITE_HOST=$2; shift 2 ;;
      --hostname-publico) exigir_valor "$@"; HOSTNAME_PUBLICO=$2; shift 2 ;;
      --ca-cert) exigir_valor "$@"; OPC_CA_CERT=$2; shift 2 ;;
      --ca-key) exigir_valor "$@"; OPC_CA_KEY=$2; shift 2 ;;
      --nova-ca) NOVA_CA=true; shift ;;
      --pepper-file) exigir_valor "$@"; OPC_PEPPER=$2; shift 2 ;;
      --token-file) exigir_valor "$@"; OPC_TOKEN=$2; shift 2 ;;
      --tmpfs-mb) exigir_valor "$@"; OPC_TMPFS=$2; shift 2 ;;
      --sem-tunel) SEM_TUNEL=true; shift ;;
      --backup-dir) exigir_valor "$@"; OPC_BACKUP_DIR=$2; shift 2 ;;
      --exigir-montagem) exigir_valor "$@"; OPC_MONTAGEM=$2; shift 2 ;;
      --par-divergente-motivo) exigir_valor "$@"; MOTIVO_PAR=$2; shift 2 ;;
      --listar-manuais) LISTAR=true; shift ;;
      -h|--help) usage; exit 0 ;;
      *) uso_errado "opcao desconhecida: $1" ;;
    esac
  done
}

validar_args() {
  [[ -n "$OPC_DUMP" && -n "$OPC_IMAGENS" ]] || uso_errado "--dump e --imagens sao obrigatorios"
  [[ -n "$OPC_API_ENV" ]] || uso_errado "--api-env e obrigatorio: o trio sozinho nao sobe a API"
  [[ -n "$SITE_IP" && -n "$SITE_HOST" && -n "$HOSTNAME_PUBLICO" ]] || uso_errado "--ip, --host-nome e --hostname-publico sao obrigatorios"
  [[ "$SITE_IP" =~ ^(25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])(\.(25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])){3}$ ]] \
    || uso_errado "--ip invalido: $SITE_IP"
  [[ "$SITE_IP" != 0.0.0.0 && "$SITE_IP" != 127.* ]] || uso_errado "--ip precisa ser o IP da LAN: $SITE_IP"
  [[ "$SITE_HOST" =~ $RE_HOST ]] || uso_errado "--host-nome invalido: $SITE_HOST"
  [[ "$HOSTNAME_PUBLICO" =~ $RE_HOST && "$HOSTNAME_PUBLICO" == *.* ]] || uso_errado "--hostname-publico invalido: $HOSTNAME_PUBLICO"
  if $NOVA_CA; then
    [[ -z "$OPC_CA_CERT$OPC_CA_KEY" ]] || uso_errado "--nova-ca nao combina com --ca-cert/--ca-key"
  else
    [[ -n "$OPC_CA_CERT" && -n "$OPC_CA_KEY" ]] || uso_errado "informe --ca-cert e --ca-key, ou --nova-ca"
  fi
  if $SEM_TUNEL && [[ -n "$OPC_TOKEN" ]]; then
    uso_errado "--token-file nao combina com --sem-tunel"
  fi
  if [[ -n "$OPC_TMPFS" ]]; then
    [[ "$OPC_TMPFS" =~ ^[0-9]{1,6}$ ]] || uso_errado "--tmpfs-mb invalido"
  fi
  if [[ -n "$OPC_BACKUP_DIR$OPC_MONTAGEM" ]]; then
    [[ -n "$OPC_BACKUP_DIR" && -n "$OPC_MONTAGEM" ]] || uso_errado "--backup-dir e --exigir-montagem andam juntos"
    [[ "$OPC_BACKUP_DIR" =~ ^/[A-Za-z0-9._/-]+$ && "$OPC_BACKUP_DIR" != *..* ]] || uso_errado "--backup-dir invalido"
  fi
  if [[ -n "$MOTIVO_PAR" ]]; then
    [[ "$MOTIVO_PAR" =~ ^[[:print:]]{10,300}$ ]] || uso_errado "--par-divergente-motivo precisa de 10 a 300 caracteres imprimiveis"
  fi
}

# ---------------------------------------------------------------------------
# --listar-manuais: so nomes, nunca valores.
# ---------------------------------------------------------------------------
listar_cloudflare() {
  cat <<EOF
Cloudflare (painel Zero Trust):
  - hostname publico do tunel, o mesmo de --hostname-publico
  - servico da origem: https://localhost:$PORTA_HTTPS, com a verificacao TLS desligada
    (noTLSVerify; o nome da opcao no painel nao foi confirmado)
  - token do tunel: use no --token-file ou digite na etapa 11
  - parar o conector da maquina antiga: dois conectores com o mesmo token viram
    replicas ativas, e cada requisicao vai para a mais proxima
  - maquina antiga perdida ou sob suspeita: "Rotate token" no painel E limpar as
    conexoes (DELETE em /connections da API): a rotacao nao derruba quem ja esta ligado
  - hostnames adicionais do tunel, se houver
  - hostnames do Turnstile: o publico e os da LAN
EOF
}

listar_jenkins() {
  cat <<EOF
Jenkins (docs/deploy-servidor.md, secoes 5 e 6):
  - instalacao, plugins e job
  - Secret file rotaperfumes-api-env: o conteudo de $API_ENV_NOVO
  - credencial rotaperfumes-db-admin: a senha nova de rotaperfumes_admin, em
    $SEGREDOS_DIR/rotaperfumes_admin.senha
EOF
}

listar_secret_file() {
  cat <<EOF
So no Secret file (nunca no repositorio):
  PASSWORD_PEPPER, JWT_SECRET, TURNSTILE_SECRET_KEY, SMTP_*, SECURITY_ALERT_EMAILS,
  CORS_ALLOWED_ORIGINS, TURNSTILE_HOSTNAMES
EOF
}

listar_final() {
  cat <<EOF
Ao final:
  - raiz da CA do Caddy nos clientes da LAN: o volume caddy_data nasce novo
  - ca-key.pem: com --nova-ca, leve $CA_DIR/ca-key.pem para midia offline e apague daqui
  - --nova-ca: grave o fingerprint em deploy/db-ca.fingerprint, commite e de push SO
    depois de atualizar o Secret file; antes, o estagio "CA do banco" do Jenkins recusa
  - copia externa do backup: sem ela, um disco perdido nao tem o que restaurar
EOF
}

# Mantida junto dos literais: um teste compara com "git grep" do IP versionado.
listar_ip_commits() {
  cat <<EOF
IP ou hostname diferente de $IP_VERSIONADO / $HOST_VERSIONADO:
  Com commit:
    Jenkinsfile                                       SITE_IP e SITE_HOST (parametros)
    deploy/docker-compose.yml                         padroes de SITE_IP e SITE_HOST
    deploy/mariadb-docker.sh                          SITE_IP (padrao)
    deploy/setup-servidor.sh                          SITE_IP (padrao)
    Makefile                                          PRODUCAO_HOSTS_LAN (guarda)
    backend/shared/testkit/integracao/integracao.go   ProducaoHostsLAN (guarda)
  Comentarios e exemplos (sem efeito no site, atualize se quiser):
    deploy/api.env.example deploy/db-ca.fingerprint
    deploy/seed-producao.sh deploy/restaurar-maquina.sh
  No Secret file e no painel: CORS_ALLOWED_ORIGINS, TURNSTILE_HOSTNAMES e os
  hostnames do Turnstile.
  NAO mudar (CasaOS de dev, porta 3307): backend/.env.example, INTEGRACAO_PERMITIDOS do
  Makefile e Permitidos do integracao.go. Trocar o servidor tambem afeta o CasaOS.
  Reemissao: o SAN muda, o que exige --ca-key; a CA reaproveitada nao muda
  deploy/db-ca.fingerprint.
EOF
}

listar_manuais() {
  listar_cloudflare
  printf '\n'
  listar_jenkins
  printf '\n'
  listar_secret_file
  printf '\n'
  listar_final
  printf '\n'
  listar_ip_commits
}

# ---------------------------------------------------------------------------
# Terminal e segredos
# ---------------------------------------------------------------------------
# tem_tty (BUG-26): so o open() de /dev/tty responde se ha terminal de controle.
tem_tty() { { : </dev/tty; } 2>/dev/null; }

confirmar() { # prompt esperado
  local resp=""
  printf '%s' "$1" >/dev/tty
  IFS= read -r resp </dev/tty || die "leitura do terminal falhou"
  resp=${resp%$'\r'}
  [[ "$resp" == "$2" ]] || die "confirmacao diferente do esperado: nada alem do que ja foi feito acontece. Rode de novo para retomar."
}

# Mesma regra do deploy/seed-producao.sh (teste de paridade).
arquivo_secreto_ok() {
  [[ -f "$1" && ! -L "$1" ]] || return 1
  [[ "$(stat -c '%u' "$1")" == 0 ]] || return 1
  case "$(stat -c '%a' "$1")" in 600|400) return 0 ;; *) return 1 ;; esac
}

ler_pepper() {
  local segundo=""
  if [[ -n "$OPC_PEPPER" ]]; then
    IFS= read -r PEPPER <"$OPC_PEPPER" || [[ -n "$PEPPER" ]] || die "--pepper-file vazio"
  else
    LENDO_SEGREDO=true
    IFS= read -rs -p "PASSWORD_PEPPER (nao aparece na tela): " PEPPER </dev/tty || die "leitura do pepper falhou"
    printf '\n' >&2
    IFS= read -rs -p "Confirme o PASSWORD_PEPPER: " segundo </dev/tty || die "leitura do pepper falhou"
    printf '\n' >&2
    LENDO_SEGREDO=false
    [[ "$PEPPER" == "$segundo" ]] || die "os dois PASSWORD_PEPPER digitados nao conferem"
    unset segundo
  fi
  PEPPER=${PEPPER%$'\r'}
  (( ${#PEPPER} >= PEPPER_MIN_BYTES )) || die "PASSWORD_PEPPER com menos de $PEPPER_MIN_BYTES caracteres (a API recusaria)"
  [[ "${ENV_VAL[PASSWORD_PEPPER]:-}" == "$PEPPER" ]] \
    || die "o pepper informado difere do PASSWORD_PEPPER do --api-env. Com o pepper errado nenhum login funciona; confira o cofre."
  unset PEPPER
  PEPPER=""
}

ler_token() {
  local linha=""
  if [[ -n "$OPC_TOKEN" ]]; then
    IFS= read -r linha <"$OPC_TOKEN" || [[ -n "$linha" ]] || die "--token-file vazio"
  else
    LENDO_SEGREDO=true
    IFS= read -rs -p "TUNNEL_TOKEN (nao aparece na tela): " linha </dev/tty || die "leitura do token falhou"
    printf '\n' >&2
    LENDO_SEGREDO=false
  fi
  TOKEN=${linha%$'\r'}
  unset linha
  [[ "$TOKEN" =~ ^[A-Za-z0-9+/=_-]{32,}$ ]] || die "o token nao tem o formato esperado (o valor nao e mostrado)"
}

# Achado do SecBrain (Lote 42): falha depois de gravar o token (daemon-reload,
# enable, start, vazamento, tunel sem resposta): a unit propria e desligada e o
# token apagado. A etapa 11 fica pendente e e refeita do zero na retomada.
# O mesmo que "systemctl disable" com a opcao que tambem para a unit, em dois
# comandos: a catraca TST-25 recusa a palavra do relogio em deploy/*.sh.
desfazer_tunel() {
  systemctl stop "$TUNEL_UNIT" >/dev/null 2>&1 || aviso "systemctl stop $TUNEL_UNIT falhou: pare a unit a mao"
  systemctl disable "$TUNEL_UNIT" >/dev/null 2>&1 || aviso "systemctl disable $TUNEL_UNIT falhou: desabilite a unit a mao"
  if [[ -f "$TUNEL_ENV" ]]; then
    shred -u -- "$TUNEL_ENV" 2>/dev/null || aviso "shred -u $TUNEL_ENV falhou: apague o arquivo a mao"
  fi
  aviso "etapa 11 interrompida: $TUNEL_UNIT parada e desabilitada (stop e disable) e $TUNEL_ENV apagado com shred. Rode de novo para refazer a etapa 11."
}

limpar() {
  local rc=$?
  trap - EXIT TERM INT
  if $LENDO_SEGREDO; then stty echo </dev/tty 2>/dev/null || true; fi
  if [[ "$rc" != 0 ]] && $TUNEL_GRAVADO; then desfazer_tunel; fi
  if [[ -n "$GPG_DIR" ]]; then rm -rf -- "$GPG_DIR"; fi
  if [[ -n "$ERRO_ARQ" ]]; then rm -f -- "$ERRO_ARQ"; fi
  if [[ -f "$TUNEL_ENV.tmp" ]]; then shred -u -- "$TUNEL_ENV.tmp" 2>/dev/null || rm -f -- "$TUNEL_ENV.tmp"; fi
  rm -f -- "$API_ENV_NOVO.tmp" 2>/dev/null || true
  exit "$rc"
}

# ---------------------------------------------------------------------------
# Recusas
# ---------------------------------------------------------------------------
conferir_execucao() {
  [[ $- != *x* ]] || die "rode sem xtrace (bash -x ou SHELLOPTS): os comandos apareceriam na tela"
  [[ $EUID -eq 0 ]] || die "rode como root: sudo bash $0"
  [[ -z "${JENKINS_HOME+x}${BUILD_ID+x}${WORKSPACE+x}" ]] || die "dentro do Jenkins (JENKINS_HOME, BUILD_ID ou WORKSPACE): execucao so manual"
  case "$RAIZ" in
    */jenkins/*|*/jenkins_home/*|*/.jenkins/*) die "o clone esta num caminho do Jenkins ($RAIZ): use um clone proprio" ;;
  esac
  tem_tty || die "sem terminal: as confirmacoes sao digitadas em /dev/tty"
}

conferir_comandos() {
  local cmd
  for cmd in docker git gzip tar openssl curl ip df sha256sum shred stat realpath find flock awk grep systemctl journalctl hostname date install; do
    command -v "$cmd" >/dev/null || die "comando '$cmd' nao encontrado"
  done
  if ! $SEM_TUNEL; then
    command -v apt-get >/dev/null || die "sem apt-get: o cloudflared so e instalado pelo repositorio apt verificado (use --sem-tunel no ensaio)"
    command -v gpg >/dev/null || die "sem gpg: instale o pacote gnupg"
    command -v systemd-run >/dev/null || die "sem systemd-run: a etapa 11 confere a origem com o filtro da unit do tunel"
  fi
  docker info >/dev/null 2>&1 || die "o daemon do Docker nao responde"
  docker compose version >/dev/null 2>&1 || die "sem o plugin docker compose"
}

conferir_ancestrais() { # caminho
  local p=$1 dono modo
  while [[ "$p" != / ]]; do
    p=$(dirname -- "$p")
    [[ -d "$p" && ! -L "$p" ]] || die "o ancestral $p do clone nao e diretorio real"
    read -r dono modo < <(stat -c '%u %a' -- "$p")
    [[ "$dono" == 0 ]] || die "o ancestral $p do clone precisa ser do root (dono: $dono)"
    (( (8#$modo & 8#022) == 0 )) || die "o ancestral $p do clone tem escrita de grupo/outros (modo $modo)"
  done
}

conferir_clone() {
  local real literal origem cabeca remoto sujo
  real=$(realpath -- "$0") || die "realpath falhou"
  literal=$(realpath -s -- "$0") || die "realpath falhou"
  [[ "$real" == "$literal" && "$real" == "$RAIZ/deploy/restaurar-maquina.sh" ]] \
    || die "rode pelo caminho real do clone, sem link simbolico ($literal)"
  [[ -z "$(find "$RAIZ" -maxdepth 1 -type l -print -quit)" ]] || die "ha link simbolico na raiz do clone"
  conferir_ancestrais "$RAIZ"
  # Antes de qualquer git: um hook ou config de outro dono rodaria como root.
  [[ -z "$(find "$RAIZ" \( ! -uid 0 -o ! -gid 0 \) -print -quit)" ]] || die "ha arquivo no clone que nao e do root"
  [[ -z "$(find "$RAIZ" ! -type l -perm /022 -print -quit)" ]] || die "ha arquivo no clone com escrita de grupo/outros"
  origem=$(git -C "$RAIZ" config --get remote.origin.url) || die "o clone nao tem origin"
  [[ "$origem" == "$REPO_URL" ]] || die "origin diferente de $REPO_URL"
  cabeca=$(git -C "$RAIZ" rev-parse HEAD) || die "git rev-parse HEAD falhou"
  remoto=$(git -C "$RAIZ" ls-remote origin refs/heads/main | awk '{print $1}') || die "git ls-remote origin falhou"
  [[ -n "$remoto" && "$cabeca" == "$remoto" ]] || die "HEAD ($cabeca) diferente da main de origin (${remoto:-nao encontrada})"
  sujo=$(git -C "$RAIZ" status --porcelain --ignored) || die "git status falhou"
  [[ -z "$sujo" ]] || die "o clone nao esta limpo (git status --porcelain --ignored nao esta vazio; ignorados tambem contam)"
  COMMIT=$cabeca
}

conferir_producao_ativa() {
  local sinais=() f mp
  [[ -z "$(docker ps -aq --filter "name=^${CONTAINER_DB}\$")" ]] || sinais+=("container $CONTAINER_DB")
  [[ -z "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJETO")" ]] || sinais+=("container do compose $PROJETO")
  [[ ! -e "$DADOS_DIR/mysql" ]] || sinais+=("$DADOS_DIR/mysql")
  [[ ! -e "$SSL_DIR/server-key.pem" ]] || sinais+=("$SSL_DIR/server-key.pem")
  # Achado do SecBrain (Lote 42): senha do root do banco e chave da CA gerada no
  # servidor. Uma --ca-key no caminho padrao ja foi recusada em conferir_ca.
  for f in "$SEGREDOS_DIR/root.senha" "$CA_DIR/ca-key.pem"; do
    [[ ! -e "$f" && ! -L "$f" ]] || sinais+=("$f")
  done
  for f in "$UNIT_DIR/rotaperfumes-backup-banco.service" "$UNIT_DIR/rotaperfumes-backup-banco.timer" "$BACKUP_CMD"; do
    [[ ! -e "$f" ]] || sinais+=("$f")
  done
  for f in "$BACKUP_DIR_PADRAO" "$OPC_BACKUP_DIR"; do
    [[ -n "$f" && -d "$f" ]] || continue
    if [[ -n "$(find "$f" -maxdepth 1 \( -name 'rotaperfumes-*.sql.gz' -o -name 'rotaperfumes-imagens-*.tar.gz' \) -print -quit)" ]]; then
      sinais+=("backup em $f")
    fi
  done
  for f in cloudflared.service "$TUNEL_UNIT"; do
    if systemctl cat -- "$f" >/dev/null 2>&1 || [[ -e "$UNIT_DIR/$f" ]]; then
      sinais+=("unit $f")
    fi
  done
  if docker volume inspect "$VOLUME_IMAGENS" >/dev/null 2>&1; then
    mp=$(docker volume inspect -f '{{.Mountpoint}}' "$VOLUME_IMAGENS") || mp=""
    if [[ -z "$mp" || ! -d "$mp" ]]; then
      sinais+=("volume $VOLUME_IMAGENS (conteudo nao conferido)")
    elif [[ -n "$(find "$mp" -mindepth 1 -print -quit)" ]]; then
      sinais+=("volume $VOLUME_IMAGENS com conteudo")
    fi
  fi
  if (( ${#sinais[@]} > 0 )); then
    printf '  Sinal de producao: %s\n' "${sinais[@]}" >&2
    die "esta maquina parece um servidor em uso. Aqui a restauracao e so o runbook (docs/manual-base-de-dados.md, 24.5). Nada foi mudado."
  fi
}

conferir_ip() {
  local ips
  ips=$(ip -4 -o addr | awk '{print $4}' | cut -d/ -f1) || die "nao consegui listar os IPs desta maquina"
  grep -qxF -- "$SITE_IP" <<<"$ips" || die "--ip $SITE_IP nao esta configurado nesta maquina"
}

carimbo_de() { # nome regex
  [[ "$1" =~ $2 ]] || return 1
  printf '%s' "${BASH_REMATCH[1]}"
}

conferir_par() {
  local nome_d nome_i f
  CARIMBO_DUMP="" CARIMBO_IMAGENS=""
  for f in "$OPC_DUMP" "$OPC_IMAGENS"; do
    [[ ! -L "$f" && -f "$f" ]] || die "$f: precisa ser arquivo regular, sem link simbolico"
  done
  nome_d=${OPC_DUMP##*/}
  nome_i=${OPC_IMAGENS##*/}
  CARIMBO_DUMP=$(carimbo_de "$nome_d" "$RE_DUMP") || die "--dump: nome fora do padrao do backup-banco.sh (parcial-* e pre-restauracao-* nao valem): $nome_d"
  CARIMBO_IMAGENS=$(carimbo_de "$nome_i" "$RE_IMAGENS") || die "--imagens: nome fora do padrao do backup-banco.sh: $nome_i"
  if [[ "$CARIMBO_DUMP" != "$CARIMBO_IMAGENS" && -z "$MOTIVO_PAR" ]]; then
    die "carimbos diferentes ($CARIMBO_DUMP e $CARIMBO_IMAGENS): dump e imagens de datas diferentes deixam produtos sem imagem. So com --par-divergente-motivo TEXTO"
  fi
}

fingerprint_ca() { # cert -> hex minusculo sem ':' (formato do deploy/db-ca.fingerprint)
  openssl x509 -in "$1" -noout -fingerprint -sha256 | cut -d= -f2 | tr -d ':' | tr 'A-F' 'a-f'
}

conferir_ca() {
  $NOVA_CA && return 0
  local fp dev aceitos
  [[ ! -L "$OPC_CA_CERT" && -f "$OPC_CA_CERT" ]] || die "--ca-cert: precisa ser arquivo regular"
  [[ -f "$OPC_CA_KEY" && -r "$OPC_CA_KEY" ]] || die "--ca-key nao encontrada ou ilegivel"
  # $CA_DIR/ca-key.pem e sinal de producao ativa (chave gerada no servidor): a
  # chave reaproveitada fica fora de $CA_DIR, onde esta (ex.: pendrive).
  local real_key real_ca
  real_key=$(realpath -m -- "$OPC_CA_KEY") || die "--ca-key: realpath falhou"
  real_ca=$(realpath -m -- "$CA_DIR") || die "realpath de $CA_DIR falhou"
  [[ "$real_key" != "$real_ca" && "$real_key" != "$real_ca"/* ]] \
    || die "--ca-key dentro de $CA_DIR: esse caminho e sinal de servidor em uso. Deixe a chave na midia externa (ex.: pendrive) e aponte --ca-key para la. Nada foi mudado."
  [[ -z "$(grep -- '-----BEGIN' "$OPC_CA_CERT" | grep -vx -- '-----BEGIN CERTIFICATE-----' || true)" ]] \
    || die "--ca-cert tem bloco PEM que nao e certificado"
  fp=$(fingerprint_ca "$OPC_CA_CERT" 2>/dev/null) || die "--ca-cert nao e um certificado valido"
  [[ "$fp" =~ ^[0-9a-f]{64}$ ]] || die "--ca-cert nao e um certificado valido"
  for dev in "${CA_DEV_FPS[@]}"; do
    [[ "$fp" != "$dev" ]] || die "--ca-cert e uma CA de DEV: nunca vale em producao"
  done
  aceitos=$(grep -Ev '^[[:space:]]*(#|$)' "$RAIZ/deploy/db-ca.fingerprint" | tr -d ' \t\r' || true)
  grep -qxF -- "$fp" <<<"$aceitos" \
    || die "--ca-cert fora de deploy/db-ca.fingerprint (use a CA de producao ou --nova-ca)"
  [[ "$(openssl x509 -in "$OPC_CA_CERT" -noout -pubkey 2>/dev/null)" == "$(openssl pkey -in "$OPC_CA_KEY" -pubout 2>/dev/null)" ]] \
    || die "--ca-key nao e a chave da --ca-cert"
}

# Leitura linha a linha, sem "source". As mensagens citam so o nome da chave.
ler_api_env() { # arquivo -> preenche ENV_VAL
  local linha n=0 chave bruto valor
  while IFS= read -r linha || [[ -n "$linha" ]]; do
    n=$((n + 1))
    [[ "$linha" != *$'\r' ]] || die "--api-env: linha $n com CRLF: converta o arquivo"
    [[ "$linha" =~ ^[[:space:]]*(#.*)?$ ]] && continue
    [[ "$linha" =~ ^([A-Z][A-Z0-9_]*)=(.*)$ ]] || die "--api-env: linha $n fora do formato CHAVE=valor"
    chave=${BASH_REMATCH[1]}
    bruto=${BASH_REMATCH[2]}
    [[ -z "${ENV_VAL[$chave]+x}" ]] || die "--api-env: chave $chave repetida"
    case "$bruto" in
      \'*\')
        valor=${bruto:1:${#bruto}-2}
        [[ "$valor" != *\'* ]] || die "--api-env: aspas simples dentro do valor de $chave"
        ;;
      \"*\")
        valor=${bruto:1:${#bruto}-2}
        [[ "$valor" != *[\$\\\"]* ]] || die "--api-env: \$, \\ ou aspas dentro de aspas duplas em $chave: use aspas simples"
        ;;
      *)
        [[ "$bruto" != *\$* ]] || die "--api-env: \$ sem aspas simples em $chave (o compose interpolaria)"
        [[ "$bruto" != *[\'\"]* && ! "$bruto" =~ [[:space:]]# ]] || die "--api-env: aspas ou comentario no meio do valor de $chave: use aspas simples"
        valor=$bruto
        ;;
    esac
    ENV_VAL[$chave]=$valor
  done <"$1"
}

lista_tem() { # lista_com_virgulas item
  local item
  local -a itens=()
  IFS=, read -r -a itens <<<"$1"
  for item in "${itens[@]+"${itens[@]}"}"; do
    item=${item#"${item%%[![:space:]]*}"}
    item=${item%"${item##*[![:space:]]}"}
    [[ "$item" == "$2" ]] && return 0
  done
  return 1
}

conferir_api_env() {
  local chave
  ler_api_env "$OPC_API_ENV"
  for chave in PASSWORD_PEPPER TURNSTILE_SECRET_KEY CORS_ALLOWED_ORIGINS; do
    [[ -n "${ENV_VAL[$chave]:-}" ]] || die "--api-env: chave obrigatoria vazia: $chave"
  done
  [[ ! "${ENV_VAL[TURNSTILE_SECRET_KEY]}" =~ ^[123]x0+ ]] || die "--api-env: TURNSTILE_SECRET_KEY e uma secret de teste da Cloudflare"
  [[ "${ENV_VAL[TURNSTILE_PERMITIR_SECRET_TESTE]:-false}" == false ]] || die "--api-env: TURNSTILE_PERMITIR_SECRET_TESTE ligada"
  [[ "${ENV_VAL[DB_USUARIO]:-rotaperfumes_app}" == rotaperfumes_app ]] || die "--api-env: DB_USUARIO diferente da conta da API"
  [[ "${ENV_VAL[DB_NAME]:-$DB_NOME}" == "$DB_NOME" ]] || die "--api-env: DB_NAME diferente de $DB_NOME"
  local cors=${ENV_VAL[CORS_ALLOWED_ORIGINS]} hosts=${ENV_VAL[TURNSTILE_HOSTNAMES]:-}
  lista_tem "$cors" "https://$HOSTNAME_PUBLICO" || die "--api-env: CORS_ALLOWED_ORIGINS sem o HTTPS do --hostname-publico. Nada e reescrito: corrija a copia"
  lista_tem "$cors" "https://$SITE_IP:$PORTA_HTTPS" || die "--api-env: CORS_ALLOWED_ORIGINS sem o --ip (https, porta $PORTA_HTTPS). Nada e reescrito: corrija a copia"
  lista_tem "$cors" "https://$SITE_HOST:$PORTA_HTTPS" || die "--api-env: CORS_ALLOWED_ORIGINS sem o --host-nome (https, porta $PORTA_HTTPS). Nada e reescrito: corrija a copia"
  if [[ -n "$hosts" ]]; then
    lista_tem "$hosts" "$HOSTNAME_PUBLICO" || die "--api-env: TURNSTILE_HOSTNAMES sem o --hostname-publico. Nada e reescrito: corrija a copia"
    if ! lista_tem "$hosts" "$SITE_IP" && ! lista_tem "$hosts" "$SITE_HOST"; then
      aviso "TURNSTILE_HOSTNAMES sem --ip e sem --host-nome: o login pela LAN da etapa 9 sera recusado pelo Turnstile"
    fi
  fi
}

conferir_segredos_arq() {
  arquivo_secreto_ok "$OPC_API_ENV" || die "--api-env: precisa ser arquivo regular do root com modo 600 ou 400"
  if [[ -n "$OPC_PEPPER" ]]; then
    arquivo_secreto_ok "$OPC_PEPPER" || die "--pepper-file: precisa ser arquivo regular do root com modo 600 ou 400"
  fi
  if [[ -n "$OPC_TOKEN" ]]; then
    arquivo_secreto_ok "$OPC_TOKEN" || die "--token-file: precisa ser arquivo regular do root com modo 600 ou 400"
  fi
}

conferir_chave_apt() { # -> define APT_CHAVE_BIN (temporario)
  local esperado linhas
  esperado=$(grep -Ev '^[[:space:]]*(#|$)' "$APT_CHAVE_FP" || true)
  [[ "$esperado" =~ ^[0-9A-F]{40}$ ]] || die "deploy/cloudflare-apt.fingerprint sem um fingerprint de 40 hex"
  [[ -n "$GPG_DIR" ]] || GPG_DIR=$(mktemp -d /root/.rotaperfumes-gpg.XXXXXX)
  linhas=$(gpg --homedir "$GPG_DIR" --batch --no-autostart --with-colons --fixed-list-mode --show-keys "$APT_CHAVE_ASC" 2>/dev/null) \
    || die "deploy/cloudflare-apt.asc nao e uma chave OpenPGP legivel"
  [[ "$(grep -c '^pub:' <<<"$linhas")" == 1 ]] || die "deploy/cloudflare-apt.asc precisa ter exatamente uma chave"
  [[ "$(awk -F: '/^pub:/ { getline; print $10; exit }' <<<"$linhas")" == "$esperado" ]] \
    || die "o fingerprint de deploy/cloudflare-apt.asc difere de deploy/cloudflare-apt.fingerprint"
  APT_CHAVE_BIN="$GPG_DIR/chave.gpg"
  gpg --homedir "$GPG_DIR" --batch --no-autostart --yes --dearmor -o "$APT_CHAVE_BIN" "$APT_CHAVE_ASC" 2>/dev/null \
    || die "nao consegui converter deploy/cloudflare-apt.asc"
}

ipv4_negado_tunel() { # ip -> sucesso se cair numa faixa de TUNEL_IP_DENY
  local a b
  [[ "$1" =~ ^([0-9]{1,3})\.([0-9]{1,3})\.[0-9]{1,3}\.[0-9]{1,3}$ ]] || return 1
  a=$((10#${BASH_REMATCH[1]})) b=$((10#${BASH_REMATCH[2]}))
  (( a == 10 || (a == 172 && b >= 16 && b <= 31) || (a == 192 && b == 168) || (a == 169 && b == 254) ))
}

# O cloudflared (Go) resolve pelo /etc/resolv.conf. Um nameserver da LAN cairia no
# IPAddressDeny da unit e o tunel nao conectaria: recusa antes, com a causa.
conferir_dns_tunel() {
  local linha ns
  [[ -r /etc/resolv.conf ]] || return 0
  while IFS= read -r linha || [[ -n "$linha" ]]; do
    [[ "$linha" =~ ^[[:space:]]*nameserver[[:space:]]+([^[:space:]]+) ]] || continue
    ns=${BASH_REMATCH[1]}
    if ipv4_negado_tunel "$ns"; then
      die "o /etc/resolv.conf usa o nameserver $ns, da LAN: a unit do tunel barra a LAN (IPAddressDeny=$TUNEL_IP_DENY) e o cloudflared nao resolveria o nome da Cloudflare. Use o systemd-resolved (nameserver 127.0.0.53) ou um DNS publico, ou rode com --sem-tunel."
    fi
  done </etc/resolv.conf
}

conferir_espaco() {
  local dump imagens livre minimo
  dump=$(stat -c %s -- "$OPC_DUMP")
  imagens=$(stat -c %s -- "$OPC_IMAGENS")
  livre=$(df -B1 --output=avail /root | tail -n 1 | tr -d ' ')
  [[ "$livre" =~ ^[0-9]+$ ]] || die "nao consegui ler o espaco livre"
  minimo=$((2 * (dump + imagens) + RESERVA_BYTES))
  (( livre >= minimo )) || die "espaco insuficiente em /root: livre $livre, minimo $minimo bytes (2x os arquivos + 1 GiB)"
}

# ---------------------------------------------------------------------------
# Estado: chave=valor, lido linha a linha, nunca com "source".
# ---------------------------------------------------------------------------
gravar_estado() {
  {
    printf 'etapa=%s\nimportacao=%s\nmachine_id=%s\nsha_dump=%s\nsha_imagens=%s\n' \
      "$ETAPA_FEITA" "$IMPORTACAO" "$MACHINE_ID" "$SHA_DUMP" "$SHA_IMAGENS"
    printf 'ip=%s\nhost=%s\npublico=%s\nsem_tunel=%s\nnova_ca=%s\n' \
      "$SITE_IP" "$SITE_HOST" "$HOSTNAME_PUBLICO" "$SEM_TUNEL" "$NOVA_CA"
  } >"$ESTADO.tmp"
  chmod 600 "$ESTADO.tmp"
  mv -f -- "$ESTADO.tmp" "$ESTADO"
}

etapa_ok() { ETAPA_FEITA=$1; gravar_estado; }
falta() { [[ "$ETAPA_FEITA" == concluida ]] && return 1; (( ETAPA_FEITA < $1 )); }

registrar() { printf '%s %s\n' "$(date -Is)" "$*" >>"$REGISTRO"; chmod 600 "$REGISTRO"; }

ler_machine_id() {
  [[ -r /etc/machine-id ]] || die "/etc/machine-id ilegivel"
  MACHINE_ID=$(</etc/machine-id)
  [[ "$MACHINE_ID" =~ ^[0-9a-f]{32}$ ]] || die "/etc/machine-id fora do formato"
}

carregar_estado() {
  [[ -e "$ESTADO" || -L "$ESTADO" ]] || return 0
  [[ ! -L "$ESTADO" && -f "$ESTADO" ]] || die "$ESTADO precisa ser arquivo regular"
  [[ "$(stat -c '%u %a' -- "$ESTADO")" == "0 600" ]] || die "$ESTADO precisa ser do root com modo 600"
  local linha chave valor
  local -A lido=()
  while IFS= read -r linha || [[ -n "$linha" ]]; do
    [[ "$linha" =~ ^([a-z_]+)=([A-Za-z0-9._-]*)$ ]] || die "$ESTADO com linha invalida"
    chave=${BASH_REMATCH[1]}
    valor=${BASH_REMATCH[2]}
    case "$chave" in
      etapa|importacao|machine_id|sha_dump|sha_imagens|ip|host|publico|sem_tunel|nova_ca) lido[$chave]=$valor ;;
      *) die "$ESTADO com chave desconhecida" ;;
    esac
  done <"$ESTADO"
  for chave in etapa importacao machine_id sha_dump sha_imagens ip host publico sem_tunel nova_ca; do
    [[ -n "${lido[$chave]:-}" ]] || die "$ESTADO incompleto"
  done
  [[ "${lido[etapa]}" != concluida ]] || die "esta maquina ja foi restaurada (estado concluida). Refazer exige uma maquina limpa."
  [[ "${lido[etapa]}" =~ ^[0-9]{1,2}$ ]] || die "$ESTADO com etapa invalida"
  [[ "${lido[importacao]}" =~ ^(nao|iniciada|concluida)$ ]] || die "$ESTADO com importacao invalida"
  [[ "${lido[machine_id]}" == "$MACHINE_ID" ]] || die "o estado e de outra maquina (machine-id diferente)"
  local sha_d sha_i
  sha_d=$(sha256sum -- "$OPC_DUMP" | awk '{print $1}')
  sha_i=$(sha256sum -- "$OPC_IMAGENS" | awk '{print $1}')
  [[ "$sha_d" == "${lido[sha_dump]}" && "$sha_i" == "${lido[sha_imagens]}" ]] \
    || die "dump ou imagens com sha256 diferente do confirmado na execucao anterior"
  [[ "${lido[ip]}" == "$SITE_IP" && "${lido[host]}" == "$SITE_HOST" && "${lido[publico]}" == "$HOSTNAME_PUBLICO" ]] \
    || die "--ip, --host-nome ou --hostname-publico diferente do confirmado na execucao anterior"
  [[ "${lido[sem_tunel]}" == "$SEM_TUNEL" && "${lido[nova_ca]}" == "$NOVA_CA" ]] \
    || die "--sem-tunel ou --nova-ca diferente do confirmado na execucao anterior"
  SHA_DUMP=$sha_d
  SHA_IMAGENS=$sha_i
  ETAPA_FEITA=$((10#${lido[etapa]}))
  IMPORTACAO=${lido[importacao]}
  RETOMADA=true
  info "Retomando: etapa $ETAPA_FEITA concluida, importacao $IMPORTACAO."
}

# ---------------------------------------------------------------------------
# Banco
# ---------------------------------------------------------------------------
# Sem -i: nunca consome o stdin de quem chama (nem o arquivo de um "while read").
root_sql() { docker exec "$CONTAINER_DB" mariadb --defaults-extra-file=/run/rotaperfumes/root.cnf -N -B "$@"; }

contar_tabelas() {
  local n
  n=$(root_sql -e "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = '$DB_NOME'") \
    || die "nao consegui contar as tabelas de $DB_NOME"
  [[ "$n" =~ ^[0-9]+$ ]] || die "contagem de tabelas inesperada"
  printf '%s' "$n"
}

# Cliente da conta DBA, com os limites do comando pronto (rotaperfumes-dba), mais
# --sandbox. A senha fica no option file montado; o nome e o IP sao os do comando.
cliente_importacao() {
  docker run --rm -i --pull never --name "$DBA_CLIENTE" \
    --network "$REDE_DB" --ip "$IP_DBA" --read-only --cap-drop ALL \
    --security-opt no-new-privileges:true --log-driver none --tmpfs /tmp:rw,nosuid,nodev,noexec \
    -e MYSQL_HISTFILE=/dev/null \
    --mount "type=bind,source=$SSL_DIR/ca.pem,target=$C_DBA/ca.pem,readonly" \
    --mount "type=bind,source=$DBA_CNF,target=$C_DBA/cliente.cnf,readonly" \
    --entrypoint mariadb "$IMAGEM_BASE" "--defaults-extra-file=$C_DBA/cliente.cnf" \
    -h "$CONTAINER_DB" -P 3306 "--ssl-ca=$C_DBA/ca.pem" --ssl-verify-server-cert \
    --local-infile=0 --default-character-set=utf8mb4 \
    "--init-command=SET time_zone='-03:00'" --sandbox --batch "$DB_NOME"
}

# So o codigo e a linha do erro: a mensagem pode citar dados do dump.
importar_dump() { # arquivo
  local rc_gzip rc_cliente codigo
  ERRO_ARQ=$(mktemp "$TRAB/.erro.XXXXXX")
  set +e
  gzip -dc -- "$1" | cliente_importacao >/dev/null 2>"$ERRO_ARQ"
  local -a codigos=("${PIPESTATUS[@]}")
  set -e
  rc_gzip=${codigos[0]}
  rc_cliente=${codigos[1]}
  if [[ "$rc_gzip" != 0 || "$rc_cliente" != 0 ]]; then
    codigo=$(awk 'match($0, /^ERROR [0-9]+ \([0-9A-Z]+\)( at line [0-9]+)?/) { print substr($0, RSTART, RLENGTH); exit }' "$ERRO_ARQ")
    rm -f -- "$ERRO_ARQ"
    ERRO_ARQ=""
    die "a importacao falhou (gzip=$rc_gzip, cliente=$rc_cliente, ${codigo:-sem codigo do servidor}). Rode de novo: o script oferece esvaziar o banco antes de repetir."
  fi
  rm -f -- "$ERRO_ARQ"
  ERRO_ARQ=""
}

# CONTAGENS tem "tabela linhas", gravado da saida do verificar-backup.sh.
comparar_contagens() { # ignorar_tabela (opcional)
  local ignorar=${1:-} t esperado n importadas total=0
  while read -r t esperado; do
    [[ "$t" =~ ^[A-Za-z0-9_]+$ && "$esperado" =~ ^[0-9]+$ ]] || die "$CONTAGENS com linha invalida"
    total=$((total + 1))
    [[ "$t" != "$ignorar" ]] || continue
    n=$(root_sql "$DB_NOME" -e "SELECT COUNT(*) FROM \`$t\`") || die "nao consegui contar a tabela $t"
    [[ "$n" == "$esperado" ]] || die "contagem diferente na tabela $t: importado $n, verificado $esperado"
  done <"$CONTAGENS"
  importadas=$(contar_tabelas)
  [[ "$importadas" == "$total" ]] || die "$importadas tabela(s) no banco e $total na verificacao"
}

conferir_banco() {
  local n
  [[ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER_DB" 2>/dev/null || true)" == true ]] \
    || die "o container $CONTAINER_DB nao esta rodando"
  [[ -x "$DBA_CMD" && -f "$DBA_CNF" ]] || die "$DBA_CMD ou o option file do DBA nao existe: o mariadb-docker.sh nao terminou"
  # Com o site no ar (etapa 9), as contagens ja podem ter mudado de verdade.
  if ! falta 9; then return 0; fi
  if ! falta 5; then
    comparar_contagens refresh_tokens
    return 0
  fi
  n=$(contar_tabelas)
  [[ "$n" != 0 ]] || return 0
  [[ "$IMPORTACAO" == iniciada ]] \
    || die "o banco $DB_NOME ja tem $n tabela(s): este script nunca grava sobre dados. Nada foi importado."
  # So aqui: importacao anterior interrompida, sem API no ar.
  [[ -z "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJETO")" ]] \
    || die "a importacao anterior falhou, mas ha container do compose $PROJETO: esvazie o banco manualmente, se for o caso"
  printf 'A importacao anterior nao terminou: ha %s tabela(s) em %s.\n' "$n" "$DB_NOME"
  confirmar "Digite o nome do banco ($DB_NOME) para apagar e recriar: " "$DB_NOME"
  printf "DROP DATABASE \`%s\`;\nCREATE DATABASE \`%s\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;\n" "$DB_NOME" "$DB_NOME" \
    | "$DBA_CMD" >/dev/null
  registrar "banco esvaziado pela retomada da importacao"
  [[ "$(contar_tabelas)" == 0 ]] || die "o banco $DB_NOME continua com tabelas"
}

# ---------------------------------------------------------------------------
# Compose e volume
# ---------------------------------------------------------------------------
compose() { docker compose -f "$RAIZ/deploy/docker-compose.yml" -p "$PROJETO" "$@"; }

preparar_compose() {
  local chave
  [[ -s "$CA_SHA_ARQ" ]] || die "$CA_SHA_ARQ ausente"
  DB_CA_SHA256=$(<"$CA_SHA_ARQ")
  chave=$(sed -n "s/.*string(name: 'TURNSTILE_SITE_KEY', defaultValue: '\([^']*\)'.*/\1/p" "$RAIZ/Jenkinsfile")
  [[ "$chave" =~ ^[0-9]x[A-Za-z0-9_-]{10,}$ ]] || die "TURNSTILE_SITE_KEY do Jenkinsfile fora do formato"
  TURNSTILE_SITE_KEY=$chave
  API_ENV_FILE=$API_ENV_NOVO
  HTTPS_PORT=$PORTA_HTTPS
  # So valores publicos: nenhum segredo e exportado.
  export DB_CA_SHA256 SITE_HOST SITE_IP API_ENV_FILE TURNSTILE_SITE_KEY HTTPS_PORT
}

volume_cliente() { # argumentos do mount apos "target=..." ; o resto vai ao container
  local leitura=$1
  shift
  docker run --rm -i --network none --read-only --cap-drop ALL \
    --security-opt no-new-privileges:true --user "$USUARIO_IMAGENS" \
    --mount "type=volume,source=$VOLUME_IMAGENS,target=$C_IMAGENS$leitura" \
    --entrypoint bash "$IMAGEM_BASE" "$@"
}

# Hipotese validada so numa maquina real: o "up --no-start" ja cria o volume, com
# as labels e o dono copiados da imagem. Sem isso, para aqui, sem corrigir.
conferir_volume_vazio() {
  local rotulo saida dono itens
  rotulo=$(docker volume inspect -f '{{index .Labels "com.docker.compose.project"}}/{{index .Labels "com.docker.compose.volume"}}' "$VOLUME_IMAGENS" 2>/dev/null) \
    || die "o volume $VOLUME_IMAGENS nao existe depois do up --no-start"
  [[ "$rotulo" == "$VOLUME_ROTULO" ]] || die "o volume $VOLUME_IMAGENS nao foi criado pelo compose do projeto"
  # shellcheck disable=SC2016 # expande dentro do container
  saida=$(volume_cliente ",readonly" -c 'stat -c %u:%g "$1" && find "$1" -mindepth 1 -printf x | wc -c' _ "$C_IMAGENS" </dev/null) \
    || die "nao consegui ler o volume $VOLUME_IMAGENS como $USUARIO_IMAGENS"
  read -r dono itens <<<"$(tr '\n' ' ' <<<"$saida")"
  [[ "$dono" == "$USUARIO_IMAGENS" ]] || die "o volume $VOLUME_IMAGENS nao e de $USUARIO_IMAGENS (dono $dono): a copia da imagem nao aconteceu no up --no-start. Nada foi extraido."
  [[ "$itens" == 0 ]] || die "o volume $VOLUME_IMAGENS tem conteudo: este script nao apaga nada. Confira; se for resto de uma extracao interrompida, remova o volume a mao e rode de novo."
}

extrair_imagens() { # arquivo
  local -a codigos
  set +e
  # shellcheck disable=SC2016 # expande dentro do container
  gzip -dc -- "$1" | volume_cliente "" -c 'exec tar -xf - -C "$1" --no-same-owner --no-overwrite-dir' _ "$C_IMAGENS"
  codigos=("${PIPESTATUS[@]}")
  set -e
  [[ "${codigos[0]}" == 0 && "${codigos[1]}" == 0 ]] \
    || die "a extracao falhou (codigos: ${codigos[*]}). A escrita como $USUARIO_IMAGENS precisa funcionar; o volume ficou como esta."
}

contar_arquivos_volume() {
  # shellcheck disable=SC2016 # expande dentro do container
  volume_cliente ",readonly" -c 'find "$1" -type f -printf x | wc -c' _ "$C_IMAGENS" </dev/null
}

# ---------------------------------------------------------------------------
# cloudflared
# ---------------------------------------------------------------------------
gerar_apt_lista() {
  printf 'deb [signed-by=%s] https://pkg.cloudflare.com/cloudflared any main\n' "$APT_KEYRING"
}

gerar_apt_pin() {
  cat <<EOF
# Gerado por deploy/restaurar-maquina.sh (DEPLOY-09). Deste repositorio, so o cloudflared.
Package: *
Pin: origin pkg.cloudflare.com
Pin-Priority: -1

Package: cloudflared
Pin: origin pkg.cloudflare.com
Pin-Priority: 500
EOF
}

# TUNNEL_TOKEN so no EnvironmentFile: o systemd o le como root, antes do DynamicUser.
gerar_unit_tunel() { # binario
  cat <<EOF
# Gerado por deploy/restaurar-maquina.sh (DEPLOY-09). Nao edite.
# Endurecimento (Lote 42, SecBrain): conferido no teste so com systemd-analyze
# verify/security, NAO com o cloudflared real. A etapa 11 confere o tunel
# (/ready em $TUNEL_METRICS) e a origem com o mesmo filtro de IP depois de
# subir; se falhar, desliga a unit e apaga o token. A confirmar na maquina real:
#  - origem https://localhost:$PORTA_HTTPS: chega pelo docker-proxy (loopback,
#    userland-proxy padrao do Docker). Com userland-proxy=false o DNAT troca o
#    destino para a rede do Docker (172.16.0.0/12) e o IPAddressDeny barra;
#  - DNS: nameserver da LAN seria barrado (a etapa 0 recusa);
#  - MemoryDenyWriteExecute e SystemCallFilter com o binario Go do cloudflared.
[Unit]
Description=RotaPerfumes: Cloudflare Tunnel
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=notify
EnvironmentFile=$TUNEL_ENV
ExecStart=$1 tunnel --no-autoupdate --metrics $TUNEL_METRICS run
Restart=on-failure
RestartSec=5s
TimeoutStartSec=60
DynamicUser=yes
NoNewPrivileges=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectSystem=strict
ProtectHome=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
ProtectControlGroups=yes
ProtectClock=yes
ProtectHostname=yes
ProtectProc=invisible
ProcSubset=pid
CapabilityBoundingSet=
RestrictAddressFamilies=$TUNEL_FAMILIAS
RestrictNamespaces=yes
LockPersonality=yes
RestrictRealtime=yes
MemoryDenyWriteExecute=yes
SystemCallArchitectures=native
SystemCallFilter=@system-service
UMask=0077
IPAddressDeny=$TUNEL_IP_DENY
IPAddressAllow=$TUNEL_IP_ALLOW

[Install]
WantedBy=multi-user.target
EOF
}

# A origem, vista de uma unit transitoria com o MESMO filtro de IP e familias da
# unit do tunel (o filtro e por cgroup: um curl de fora nao provaria nada).
codigo_origem_filtrada() {
  local curl_bin
  curl_bin=$(command -v curl) || return 0
  systemd-run --quiet --wait --pipe --collect --service-type=exec \
    -p DynamicUser=yes -p "IPAddressDeny=$TUNEL_IP_DENY" -p "IPAddressAllow=$TUNEL_IP_ALLOW" \
    -p "RestrictAddressFamilies=$TUNEL_FAMILIAS" \
    -- "$curl_bin" -fsSk --max-time 5 -o /dev/null -w '%{http_code}' \
    -H 'Host: smoke-tunnel.invalid' "https://localhost:$PORTA_HTTPS/api/health" </dev/null 2>/dev/null || true
}

conferir_tunel() {
  local i pronto="" origem="" publico
  for ((i = 1; i <= 40; i++)); do
    pronto=$(curl -fsS --max-time 5 -o /dev/null -w '%{http_code}' "http://$TUNEL_METRICS/ready" 2>/dev/null || true)
    origem=$(codigo_origem_filtrada)
    if [[ "$pronto" == 200 && "$origem" == 200 ]]; then
      info "Tunel OK: /ready=$pronto; origem com o filtro da unit=$origem."
      publico=$(curl -fsS --max-time 10 -o /dev/null -w '%{http_code}' "https://$HOSTNAME_PUBLICO/api/health" 2>/dev/null || true)
      [[ "$publico" == 200 ]] \
        || aviso "https://$HOSTNAME_PUBLICO/api/health respondeu ${publico:-erro} daqui (DNS, cache ou regra de acesso da Cloudflare?): confira pelo navegador."
      return 0
    fi
    info "Aguardando o tunel ($i/40): /ready=${pronto:-erro} origem-filtrada=${origem:-erro}"
    sleep 3
  done
  die "o tunel nao ficou pronto (/ready=${pronto:-erro}, origem com o filtro da unit=${origem:-erro}). /ready diferente de 200: sem conexao com a Cloudflare (token, DNS, saida para a internet, MemoryDenyWriteExecute/SystemCallFilter: journalctl -u $TUNEL_UNIT). Origem diferente de 200 com o site no ar: o IPAddressDeny barrou https://localhost:$PORTA_HTTPS (Docker com userland-proxy=false?). A unit sera desligada e o token apagado; corrija e rode de novo."
}

# Padrao pelo stdin do grep: o token nao vai para o argv e nenhuma linha aparece.
contar_token() { # fonte do texto em stdin substituido
  printf '%s\n' "$TOKEN" | grep -cFf - "$1" || true
}

conferir_token_escondido() {
  local pid n
  n=$(contar_token <(journalctl -u "$TUNEL_UNIT" --no-pager -o cat 2>/dev/null || true))
  [[ "$n" == 0 ]] || die "o token apareceu $n vez(es) no journal de $TUNEL_UNIT: rotacione-o no painel"
  n=$(contar_token <(systemctl show -- "$TUNEL_UNIT" 2>/dev/null || true))
  [[ "$n" == 0 ]] || die "o token apareceu em systemctl show $TUNEL_UNIT: rotacione-o no painel"
  pid=$(systemctl show -p MainPID --value -- "$TUNEL_UNIT")
  [[ "$pid" =~ ^[1-9][0-9]*$ && -r "/proc/$pid/cmdline" ]] || die "$TUNEL_UNIT sem processo principal"
  n=$(contar_token <(tr '\0' '\n' <"/proc/$pid/cmdline"))
  [[ "$n" == 0 ]] || die "o token apareceu no cmdline de $TUNEL_UNIT: rotacione-o no painel"
  info "Token fora do journal, do systemctl show e do cmdline: OK."
}

instalar_cloudflared() {
  local bin
  conferir_chave_apt
  install -m 644 -- "$APT_CHAVE_BIN" "$APT_KEYRING"
  gerar_apt_lista >"$APT_LISTA.tmp"
  chmod 644 "$APT_LISTA.tmp"
  mv -f -- "$APT_LISTA.tmp" "$APT_LISTA"
  gerar_apt_pin >"$APT_PIN.tmp"
  chmod 644 "$APT_PIN.tmp"
  mv -f -- "$APT_PIN.tmp" "$APT_PIN"
  # So esta lista: um repositorio quebrado de outro pacote nao barra a restauracao.
  apt-get update -o "Dir::Etc::sourcelist=$APT_LISTA" -o Dir::Etc::sourceparts=- -o APT::Get::List-Cleanup=0 </dev/null \
    || die "apt-get update do repositorio da Cloudflare falhou (assinatura ou rede)"
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends cloudflared </dev/null \
    || die "apt-get install cloudflared falhou"
  bin=$(command -v cloudflared) || die "cloudflared nao encontrado depois do apt-get"
  [[ "$bin" =~ ^/[A-Za-z0-9._/-]+$ ]] || die "caminho inesperado do cloudflared"
  gerar_unit_tunel "$bin" >"$UNIT_DIR/$TUNEL_UNIT.tmp"
  chmod 644 "$UNIT_DIR/$TUNEL_UNIT.tmp"
  mv -f -- "$UNIT_DIR/$TUNEL_UNIT.tmp" "$UNIT_DIR/$TUNEL_UNIT"
}

gravar_token() {
  install -d -m 755 "$(dirname -- "$TUNEL_ENV")"
  ( umask 077; printf 'TUNNEL_TOKEN=%s\n' "$TOKEN" >"$TUNEL_ENV.tmp" )
  chmod 600 "$TUNEL_ENV.tmp"
  mv -f -- "$TUNEL_ENV.tmp" "$TUNEL_ENV"
}

# ---------------------------------------------------------------------------
# Etapas
# ---------------------------------------------------------------------------
etapa0() {
  etapa 0 "Recusas"
  conferir_execucao
  conferir_comandos
  conferir_clone
  conferir_segredos_arq
  conferir_par
  conferir_ca
  conferir_api_env
  conferir_ip
  ler_machine_id
  carregar_estado
  if ! $RETOMADA; then conferir_producao_ativa; fi
  $SEM_TUNEL || conferir_chave_apt
  $SEM_TUNEL || conferir_dns_tunel
  conferir_espaco
  ler_pepper
  info "Recusas conferidas."
}

etapa1() {
  etapa 1 "Copia e verificacao"
  install -d -m 700 "$TRAB"
  exec 9>"$TRAB/.trava"
  flock -n 9 || die "outra execucao em andamento"
  ENTRADA_DUMP="$ENTRADA/${OPC_DUMP##*/}"
  ENTRADA_IMAGENS="$ENTRADA/${OPC_IMAGENS##*/}"
  if $RETOMADA && ! falta 2; then
    # Reconferencia: tudo depois da etapa 1 usa a copia.
    if falta 9; then
      [[ "$(sha256sum -- "$ENTRADA_DUMP" | awk '{print $1}')" == "$SHA_DUMP" ]] || die "a copia do dump em $ENTRADA mudou"
      [[ "$(sha256sum -- "$ENTRADA_IMAGENS" | awk '{print $1}')" == "$SHA_IMAGENS" ]] || die "a copia das imagens em $ENTRADA mudou"
    fi
    return 0
  fi
  rm -rf -- "$ENTRADA"
  install -d -m 700 "$ENTRADA"
  install -m 600 -- "$OPC_DUMP" "$ENTRADA_DUMP"
  install -m 600 -- "$OPC_IMAGENS" "$ENTRADA_IMAGENS"
  SHA_DUMP=$(sha256sum -- "$ENTRADA_DUMP" | awk '{print $1}')
  SHA_IMAGENS=$(sha256sum -- "$ENTRADA_IMAGENS" | awk '{print $1}')
  printf '%s  %s\n%s  %s\n' "$SHA_DUMP" "${OPC_DUMP##*/}" "$SHA_IMAGENS" "${OPC_IMAGENS##*/}" >"$ENTRADA/sha256"

  local saida rc=0 args=()
  [[ -z "$OPC_TMPFS" ]] || args=(--tmpfs-mb "$OPC_TMPFS")
  saida=$(bash "$RAIZ/deploy/verificar-backup.sh" "${args[@]+"${args[@]}"}" "$ENTRADA_DUMP" </dev/null) || rc=$?
  printf '%s\n' "$saida"
  [[ "$rc" == 0 && "$(tail -n 1 <<<"$saida")" == APROVADO ]] || die "verificar-backup.sh nao aprovou o dump (codigo $rc)"
  awk '/^  [A-Za-z0-9_]+ +[0-9]+$/ { print $1, $2 }' <<<"$saida" >"$CONTAGENS"
  [[ -s "$CONTAGENS" ]] || die "o verificar-backup.sh nao imprimiu contagens"

  rc=0
  saida=$(bash "$RAIZ/deploy/backup-banco.sh" --conferir-imagens "$ENTRADA_IMAGENS" </dev/null) || rc=$?
  printf '%s\n' "$saida"
  [[ "$rc" == 0 && "$(tail -n 1 <<<"$saida")" == APROVADO ]] || die "o tar de imagens nao foi aprovado (codigo $rc)"
  IMG_ARQUIVOS=$(awk '/^Arquivos: [0-9]+$/ { print $2 }' <<<"$saida")
  [[ "$IMG_ARQUIVOS" =~ ^[0-9]+$ ]] || die "a conferencia das imagens nao imprimiu a contagem"
  printf '%s\n' "$IMG_ARQUIVOS" >"$TRAB/imagens-arquivos"
}

etapa2() {
  etapa 2 "Confirmacao"
  if ! falta 2; then info "Ja confirmada."; return 0; fi
  local epoch idade
  epoch=$(date -u -d "${CARIMBO_DUMP:0:4}-${CARIMBO_DUMP:4:2}-${CARIMBO_DUMP:6:2} ${CARIMBO_DUMP:9:2}:${CARIMBO_DUMP:11:2}:${CARIMBO_DUMP:13:2}" +%s) \
    || die "carimbo invalido no dump"
  idade=$(( ($(date +%s) - epoch) / 3600 ))
  printf 'Linhas por tabela:\n'
  awk '{ printf "  %-45s %s\n", $1, $2 }' "$CONTAGENS"
  printf 'Dump:     carimbo %s, %s hora(s) atras\nImagens:  %s arquivo(s), carimbo %s\nIP:       %s\nHost:     %s\nPublico:  %s\nCommit:   %s\n' \
    "$CARIMBO_DUMP" "$idade" "$IMG_ARQUIVOS" "$CARIMBO_IMAGENS" "$SITE_IP" "$SITE_HOST" "$HOSTNAME_PUBLICO" "$COMMIT"
  (( idade >= 0 )) || aviso "o carimbo do dump esta no futuro: confira o relogio desta maquina"
  if [[ "$SITE_IP" != "$IP_VERSIONADO" || "$SITE_HOST" != "$HOST_VERSIONADO" ]]; then
    listar_ip_commits
  fi
  confirmar "Digite o hostname desta maquina para confirmar: " "$(hostname)"
  if $SEM_TUNEL; then
    aviso "--sem-tunel: sem cloudflared e sem declaracao sobre a maquina antiga."
    registrar "--sem-tunel escolhido: ensaio sem cloudflared"
  else
    printf 'Dois conectores com o mesmo token recebem trafego ao mesmo tempo: pare o da maquina antiga.\n'
    confirmar "Digite ANTIGA PARADA se a maquina antiga esta parada: " "ANTIGA PARADA"
    registrar "ANTIGA PARADA declarada"
  fi
  if [[ "$CARIMBO_DUMP" != "$CARIMBO_IMAGENS" ]]; then
    registrar "par divergente ($CARIMBO_DUMP, $CARIMBO_IMAGENS): $MOTIVO_PAR"
  fi
  etapa_ok 2
}

etapa3() {
  etapa 3 "CA e MariaDB"
  if ! falta 3; then info "Ja concluida."; return 0; fi
  local args=(--ip "$SITE_IP" --host-nome "$SITE_HOST")
  if $NOVA_CA; then
    # Retomada com a CA ja gerada: nova-ca trocaria a CA a cada tentativa.
    [[ -s "$SSL_DIR/ca.pem" ]] || args+=(--nova-ca)
  else
    install -d -m 755 "$SSL_DIR"
    install -m 644 -- "$OPC_CA_CERT" "$SSL_DIR/ca.pem"
    args+=(--ca-key "$OPC_CA_KEY")
  fi
  bash "$RAIZ/deploy/mariadb-docker.sh" "${args[@]}" </dev/null
  printf '\n'
  aviso "IGNORE o seed sugerido acima: o seed-producao.sh criaria as tabelas, e a importacao falharia."
  etapa_ok 3
}

etapa4() {
  etapa 4 "api.env novo"
  if ! falta 4; then
    arquivo_secreto_ok "$API_ENV_NOVO" || die "$API_ENV_NOVO ausente ou com permissao errada"
    info "Ja gerado."
    return 0
  fi
  local senha jwt linha viu_senha=false viu_jwt=false
  senha=$(<"$SENHA_APP_ARQ")
  [[ "$senha" =~ ^[A-Za-z0-9]{20,}$ ]] || die "senha da API fora do formato esperado"
  jwt=$(openssl rand -base64 48 | tr -d '\n')
  [[ ${#jwt} -ge 32 ]] || die "nao consegui gerar o JWT_SECRET"
  {
    while IFS= read -r linha || [[ -n "$linha" ]]; do
      case "$linha" in
        DB_SENHA=*) printf 'DB_SENHA=%s\n' "$senha"; viu_senha=true ;;
        JWT_SECRET=*) printf 'JWT_SECRET=%s\n' "$jwt"; viu_jwt=true ;;
        *) printf '%s\n' "$linha" ;;
      esac
    done <"$OPC_API_ENV"
    $viu_senha || printf 'DB_SENHA=%s\n' "$senha"
    $viu_jwt || printf 'JWT_SECRET=%s\n' "$jwt"
  } >"$API_ENV_NOVO.tmp"
  unset senha jwt
  chmod 600 "$API_ENV_NOVO.tmp"
  mv -f -- "$API_ENV_NOVO.tmp" "$API_ENV_NOVO"
  info "Gerado com DB_SENHA e JWT_SECRET novos, pepper informado (600, nada impresso)."
  etapa_ok 4
}

etapa5() {
  etapa 5 "Importacao"
  if ! falta 5; then info "Ja concluida."; return 0; fi
  IMPORTACAO=iniciada
  gravar_estado
  importar_dump "$ENTRADA_DUMP"
  comparar_contagens
  root_sql "$DB_NOME" -e 'SELECT 1 FROM refresh_tokens LIMIT 0' >/dev/null || die "o dump nao tem a tabela refresh_tokens"
  printf 'TRUNCATE TABLE refresh_tokens;\n' | "$DBA_CMD" >/dev/null
  IMPORTACAO=concluida
  etapa_ok 5
  info "Importado pela conta do DBA; contagens iguais; sessoes antigas removidas."
}

etapa6() {
  etapa 6 "Migracoes"
  if ! falta 6; then info "Ja conferidas."; return 0; fi
  local f
  printf 'Migracoes do repositorio. Aplique as mais novas que o dump, numa a numa:\n  sudo rotaperfumes-dba < ARQUIVO\n'
  for f in "$RAIZ"/sql/*_alter_*.sql; do
    [[ -e "$f" ]] && printf '  sql/%s\n' "${f##*/}"
  done
  confirmar "Digite MIGRACOES CONFERIDAS: " "MIGRACOES CONFERIDAS"
  registrar "migracoes conferidas pelo operador"
  etapa_ok 6
}

etapa7() {
  etapa 7 "CA validada, build e criacao dos containers"
  if ! falta 7; then info "Ja concluida."; return 0; fi
  local lista sha
  if $NOVA_CA; then
    # So nesta etapa: o arquivo versionado ainda tem a CA anterior.
    lista=$(fingerprint_ca "$SSL_DIR/ca.pem")
  else
    lista=$(<"$RAIZ/deploy/db-ca.fingerprint")
  fi
  sha=$(CA_FINGERPRINTS="$lista" DB_ALVO="$CONTAINER_DB" DB_HOST_PORT=3306 TLS_NOME="$CONTAINER_DB" \
    docker run --rm -i --user 65532:65532 --network "$REDE_DB" --ip "$IP_ADMIN" \
      --mount "type=bind,source=$SSL_DIR/ca.pem,target=/etc/rotaperfumes/db-ca.pem,readonly" \
      -e DB_ALVO -e DB_HOST_PORT -e TLS_NOME -e CA_FINGERPRINTS \
      --entrypoint bash "$IMAGEM_BASE" -s <"$RAIZ/deploy/validar-ca-banco.sh") \
    || die "a validacao da CA do banco falhou"
  [[ "$sha" =~ ^[0-9a-f]{64}$ ]] || die "sha256 inesperado da validacao da CA"
  printf '%s\n' "$sha" >"$CA_SHA_ARQ"
  preparar_compose
  compose build --pull </dev/null
  compose up --no-start </dev/null
  etapa_ok 7
}

etapa8() {
  etapa 8 "Imagens de produto"
  if ! falta 8; then info "Ja devolvidas."; return 0; fi
  local esperado n
  esperado=$(<"$TRAB/imagens-arquivos")
  conferir_volume_vazio
  extrair_imagens "$ENTRADA_IMAGENS"
  n=$(contar_arquivos_volume) || die "nao consegui contar os arquivos do volume"
  [[ "$n" == "$esperado" ]] || die "contagem diferente no volume: $n arquivo(s), esperado $esperado"
  info "$n arquivo(s) no volume $VOLUME_IMAGENS."
  etapa_ok 8
}

codigo_http() { curl -fsSk --max-time 5 -o /dev/null -w '%{http_code}' "$@" 2>/dev/null || true; }

smoke() {
  local base_ip="https://$SITE_IP:$PORTA_HTTPS" base_local="https://localhost:$PORTA_HTTPS" qualquer='Host: smoke-tunnel.invalid'
  local i a f t l
  for ((i = 1; i <= 40; i++)); do
    a=$(codigo_http "$base_ip/api/health")
    f=$(codigo_http "$base_ip/login")
    t=$(codigo_http -H "$qualquer" "$base_ip/api/health")
    l=$(codigo_http -H "$qualquer" "$base_local/api/health")
    if [[ "$a" == 200 && "$f" == 200 && "$t" == 200 && "$l" == 200 ]]; then
      info "Smoke OK: /api/health=$a /login=$f tunel-por-ip=$t tunel-por-localhost=$l"
      return 0
    fi
    info "Aguardando ($i/40): /api/health=${a:-erro} /login=${f:-erro} tunel-por-ip=${t:-erro} tunel-por-localhost=${l:-erro}"
    sleep 3
  done
  die "o smoke falhou. Logs: API_ENV_FILE=/dev/null docker compose -p $PROJETO logs --tail 40 api frontend caddy"
}

etapa9() {
  etapa 9 "Site no ar"
  if ! falta 9; then info "Ja confirmado."; return 0; fi
  preparar_compose
  compose up -d --remove-orphans </dev/null
  smoke
  printf 'Pela LAN (https://%s:%s), entre como admin e abra um produto com imagem. Essa e a UNICA prova de que o pepper esta certo.\n' "$SITE_IP" "$PORTA_HTTPS"
  confirmar "Digite LOGIN OK depois de conferir: " "LOGIN OK"
  registrar "login de admin e produto com imagem confirmados"
  etapa_ok 9
}

etapa10() {
  etapa 10 "Rotina de backup"
  if ! falta 10; then info "Ja instalada."; return 0; fi
  local args=(--instalar)
  [[ -z "$OPC_BACKUP_DIR" ]] || args+=(--backup-dir "$OPC_BACKUP_DIR" --exigir-montagem "$OPC_MONTAGEM")
  bash "$RAIZ/deploy/backup-banco.sh" "${args[@]}" </dev/null
  "$BACKUP_CMD" --status </dev/null || die "rotaperfumes-backup-banco --status acusou problema"
  etapa_ok 10
}

etapa11() {
  etapa 11 "cloudflared"
  if ! falta 11; then info "Ja instalado."; return 0; fi
  if $SEM_TUNEL; then
    info "Pulada (--sem-tunel)."
    etapa_ok 11
    return 0
  fi
  instalar_cloudflared
  printf '\nNo painel da Cloudflare:\n'
  listar_cloudflare
  printf '  Hostname publico desta restauracao: %s\n' "$HOSTNAME_PUBLICO"
  confirmar "Digite PAINEL CONFERIDO: " "PAINEL CONFERIDO"
  conferir_dns_tunel
  ler_token
  TUNEL_GRAVADO=true
  gravar_token
  systemctl daemon-reload || die "systemctl daemon-reload falhou"
  { systemctl enable "$TUNEL_UNIT" >/dev/null 2>&1 && systemctl start "$TUNEL_UNIT" >/dev/null 2>&1; } \
    || die "$TUNEL_UNIT nao subiu: journalctl -u $TUNEL_UNIT. Confira o token no painel."
  conferir_token_escondido
  conferir_tunel
  unset TOKEN
  TOKEN=""
  registrar "cloudflared instalado; unit $TUNEL_UNIT ligada"
  etapa_ok 11
  TUNEL_GRAVADO=false
}

apagar_com_shred() { [[ ! -f "$1" ]] || shred -u -- "$1"; }

etapa12() {
  etapa 12 "Lista final"
  listar_jenkins
  printf '\n'
  listar_secret_file
  printf '\n'
  listar_final
  printf '\nSo depois de copiar %s para o Secret file:\n' "$API_ENV_NOVO"
  confirmar "Digite JENKINS ATUALIZADO para apagar api.env e entrada/: " "JENKINS ATUALIZADO"
  local f
  apagar_com_shred "$API_ENV_NOVO"
  for f in "$ENTRADA"/* "$ENTRADA"/.[!.]*; do
    apagar_com_shred "$f"
  done
  rm -rf -- "$ENTRADA"
  ETAPA_FEITA=concluida
  gravar_estado
  registrar "restauracao concluida"
  printf '\nRestauracao concluida. %s e entrada/ apagados.\n' "$API_ENV_NOVO"
  # Achado do SecBrain (Lote 42): vale para toda entrada com segredo.
  local restos=("--api-env $OPC_API_ENV")
  [[ -z "$OPC_PEPPER" ]] || restos+=("--pepper-file $OPC_PEPPER")
  if ! $SEM_TUNEL && [[ -n "$OPC_TOKEN" ]]; then restos+=("--token-file $OPC_TOKEN"); fi
  for f in "${restos[@]}"; do
    aviso "apague o ${f%% *} (${f#* }) com shred -u, se ele nao estiver numa midia segura."
  done
}

main() {
  parse_args "$@"
  if $LISTAR; then listar_manuais; exit 0; fi
  validar_args
  trap limpar EXIT
  trap 'exit 143' TERM
  trap 'exit 130' INT
  etapa0
  etapa1
  if [[ -f "$TRAB/imagens-arquivos" ]]; then IMG_ARQUIVOS=$(<"$TRAB/imagens-arquivos"); fi
  etapa2
  etapa3
  conferir_banco
  etapa4
  etapa5
  etapa6
  etapa7
  etapa8
  etapa9
  etapa10
  etapa11
  etapa12
}

main "$@"
