#!/usr/bin/env bash
# =============================================================================
# BACKUP-01: confere um backup do banco restaurando-o num MariaDB DESCARTAVEL,
# isolado e sem nenhum caminho ate o banco de producao.
#
#   sudo bash deploy/verificar-backup.sh /var/backups/rotaperfumes/ARQ.sql.gz
#
# Isolamento (nao ha opcao para mudar nada disto):
#   - todo "docker run" usa rede nenhuma; a importacao usa so o socket local;
#   - nenhum volume montado: o dump entra pelo stdin e os dados ficam em tmpfs,
#     que some com o container;
#   - nenhuma credencial lida ou criada: o usuario do container entra no socket
#     pelo proprio uid (plugin unix_socket);
#   - o container nao tem nome fixo; o trap remove so o ID criado aqui.
#
# Antes de subir o container, recusa: arquivo que nao seja regular (symlink
# tambem), gzip invalido, ultima linha sem o marcador "-- Dump completed" e
# dump que selecione ou crie banco (USE, CREATE/DROP DATABASE no inicio da
# linha). Na imagem fixada, cada linha de dados do mariadb-dump comeca com "(" e
# as quebras dos dados saem escapadas (\n), entao um dado de cliente nao comeca
# linha (validado no Lote 42).
#
# Aprovado (codigo 0): importacao com codigo 0, nenhum banco alem do temporario,
# pelo menos 13 tabelas, usuarios/vendedores/clientes presentes, um admin em
# usuarios e linhas em vendedores. Clientes sem linhas gera so AVISO: o seed de
# producao nasce sem clientes (docs/deploy-servidor.md, secao 11).
# Codigos: 0 aprovado, 1 reprovado ou falha, 2 uso incorreto.
#
# A saida traz SO contagens: o texto de erro do cliente pode citar dados (ex.:
# "Duplicate entry"), entao so o codigo do erro e a linha sao impressos.
# Ao aprovar, registra data, nome e sha256 em .ultima-verificacao, ao lado do
# arquivo (lido pelo --status do backup-banco.sh). Nunca "set -x".
# =============================================================================
set -euo pipefail
umask 077
export LC_ALL=C

# SEC-31: a mesma string do IMAGEM_BASE do deploy/mariadb-docker.sh (teste de
# paridade em backend/shared/tests/deploy).
IMAGEM_BASE="mariadb:11.4.13@sha256:5e73baf8179b29f115101bb102763a47a017361f3e9792daccba6646c3ab1532"
readonly IMAGEM_BASE
readonly TABELAS_MIN=13
readonly BANCO_TEMP="verificacao_backup"
readonly ESPERA_SEGUNDOS=120
readonly REGISTRO_NOME=".ultima-verificacao"
TMPFS_MB=2048

usage() {
  cat <<EOF
Uso: sudo bash $0 [--tmpfs-mb N] ARQUIVO.sql.gz

Restaura o backup num container descartavel, sem rede e sem credenciais, e
imprime so contagens. Nao fala com nenhum banco existente.

  --tmpfs-mb N   limite do tmpfs dos dados, em MiB (padrao $TMPFS_MB; 256 a 262144).
                 So o que for usado ocupa memoria.
  --help         esta ajuda

Codigos de saida: 0 aprovado, 1 reprovado ou falha, 2 uso incorreto.
EOF
}

uso_errado() { printf 'ERRO: %s\n' "$*" >&2; usage >&2; exit 2; }
die() { printf 'REPROVADO: %s\n' "$*" >&2; exit 1; }
aviso() { printf 'AVISO: %s\n' "$*" >&2; }

ARQ=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --help) usage; exit 0 ;;
    --tmpfs-mb)
      if [[ $# -lt 2 || ! "$2" =~ ^[0-9]{1,6}$ ]] || (( 10#$2 < 256 || 10#$2 > 262144 )); then
        uso_errado "--tmpfs-mb exige um numero de 256 a 262144"
      fi
      TMPFS_MB=$((10#$2)); shift 2 ;;
    -*) uso_errado "opcao desconhecida: $1 (este script so restaura em container isolado)" ;;
    *)
      [[ -z "$ARQ" ]] || uso_errado "informe um unico arquivo"
      ARQ=$1; shift ;;
  esac
done
[[ -n "$ARQ" ]] || uso_errado "informe o arquivo do backup"

command -v docker >/dev/null || die "comando 'docker' nao encontrado"

# ---------------------------------------------------------------------------
# Arquivo, antes de qualquer container
# ---------------------------------------------------------------------------
[[ ! -L "$ARQ" ]] || die "o arquivo e um link simbolico: informe o arquivo real"
[[ -f "$ARQ" ]] || die "nao e um arquivo regular ou nao existe"
[[ -r "$ARQ" ]] || die "sem permissao de leitura (backups sao do root; use sudo)"
gzip -t -- "$ARQ" 2>/dev/null || die "gzip invalido ou truncado"

# grep -c e tail leem o stream inteiro: sem falso negativo por SIGPIPE sob
# pipefail (grep -q fecharia o pipe cedo). grep -c sai 1 quando conta 0.
N_BANCO=$(gzip -dc -- "$ARQ" \
  | grep -Eic '^[[:space:]]*(USE[[:space:]`]|\\u[[:space:]]|(CREATE|DROP)[[:space:]]+(DATABASE|SCHEMA)([[:space:]]|$))' || true)
[[ "$N_BANCO" == 0 ]] \
  || die "o dump seleciona ou cria banco ($N_BANCO linha(s) com USE ou CREATE/DROP DATABASE): na producao ele iria para o banco errado. Gere o dump sem --databases/--all-databases"
ULTIMA=$(gzip -dc -- "$ARQ" | tail -n 1)
[[ "$ULTIMA" == "-- Dump completed"* ]] || die "a ultima linha nao tem o marcador '-- Dump completed': dump incompleto"
unset ULTIMA

# ---------------------------------------------------------------------------
# Container descartavel
# ---------------------------------------------------------------------------
CID=""
limpar() {
  local rc=$?
  trap - EXIT TERM INT
  if [[ -n "$CID" ]]; then docker rm -f -- "$CID" >/dev/null 2>&1 || true; fi
  exit "$rc"
}
trap limpar EXIT
trap 'exit 143' TERM
trap 'exit 130' INT

MYSQL_UID=$(docker run --rm --network none --entrypoint id "$IMAGEM_BASE" -u mysql) \
  || die "nao consegui rodar a imagem $IMAGEM_BASE"
MYSQL_GID=$(docker run --rm --network none --entrypoint id "$IMAGEM_BASE" -g mysql) \
  || die "nao consegui rodar a imagem $IMAGEM_BASE"
[[ "$MYSQL_UID" =~ ^[0-9]+$ && "$MYSQL_GID" =~ ^[0-9]+$ && "$MYSQL_UID" != 0 ]] \
  || die "uid/gid do mysql da imagem inesperado"

# Sem o entrypoint da imagem: ele exige senha do root e sobe um servidor
# temporario antes do definitivo. O mariadb-install-db so cria a conta do uid
# mysql, por socket; e --skip-networking se soma a rede nenhuma do container.
readonly INICIAR='mariadb-install-db --datadir=/var/lib/mysql --skip-test-db --auth-root-authentication-method=socket --auth-root-socket-user=mysql >/dev/null 2>&1 || exit 70
exec mariadbd --datadir=/var/lib/mysql --skip-networking --local-infile=0 --character-set-server=utf8mb4 --collation-server=utf8mb4_unicode_ci --innodb-flush-log-at-trx-commit=0'

CID=$(docker run -d --rm --network none \
  --user mysql \
  --cap-drop ALL \
  --security-opt no-new-privileges:true \
  --read-only \
  --log-driver none \
  --label rotaperfumes.verificar-backup=1 \
  --tmpfs "/var/lib/mysql:rw,nosuid,nodev,noexec,size=${TMPFS_MB}m,mode=0700,uid=$MYSQL_UID,gid=$MYSQL_GID" \
  --tmpfs "/run/mysqld:rw,nosuid,nodev,noexec,size=16m,mode=0755,uid=$MYSQL_UID,gid=$MYSQL_GID" \
  --tmpfs "/tmp:rw,nosuid,nodev,noexec,size=${TMPFS_MB}m,mode=1777" \
  --entrypoint bash "$IMAGEM_BASE" -c "$INICIAR") \
  || die "nao consegui criar o container descartavel"
[[ "$CID" =~ ^[0-9a-f]{12,64}$ ]] || { CID=""; die "ID do container inesperado"; }

sql() { docker exec -i "$CID" mariadb --protocol=socket --local-infile=0 --default-character-set=utf8mb4 -u mysql "$@"; }

PRONTO=false
for ((i = 0; i < ESPERA_SEGUNDOS; i++)); do
  if [[ "$(sql -N -B -e 'SELECT 1' 2>/dev/null || true)" == 1 ]]; then PRONTO=true; break; fi
  # --rm: se o servidor morrer, o container some; nao adianta esperar.
  docker inspect -f '{{.State.Running}}' "$CID" >/dev/null 2>&1 || die "o servidor descartavel parou ao iniciar"
  sleep 1
done
$PRONTO || die "o servidor descartavel nao ficou pronto em $ESPERA_SEGUNDOS s"

sql -e "CREATE DATABASE $BANCO_TEMP CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci" >/dev/null 2>&1 \
  || die "nao consegui criar o banco temporario"

# So o codigo e a linha do erro: a mensagem pode citar dados do dump.
if ! ERRO=$( { gzip -dc -- "$ARQ" | sql --sandbox "$BANCO_TEMP" >/dev/null; } 2>&1 ); then
  CODIGO=$(awk 'match($0, /^ERROR [0-9]+ \([0-9A-Z]+\)( at line [0-9]+)?/) { print substr($0, RSTART, RLENGTH); exit }' <<<"$ERRO")
  unset ERRO
  die "a importacao falhou (${CODIGO:-sem codigo do servidor}). Se o tmpfs encheu, repita com --tmpfs-mb maior"
fi
unset ERRO

# ---------------------------------------------------------------------------
# Conferencia: so contagens
# ---------------------------------------------------------------------------
OUTROS_BANCOS=$(sql -N -B -e "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME NOT IN ('$BANCO_TEMP', 'mysql', 'information_schema', 'performance_schema', 'sys')" 2>/dev/null) \
  || die "nao consegui contar os bancos"
[[ "$OUTROS_BANCOS" == 0 ]] || die "o dump criou $OUTROS_BANCOS banco(s) alem do temporario"

mapfile -t TABELAS < <(sql -N -B -e "SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = '$BANCO_TEMP' AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME" 2>/dev/null || true)

declare -A LINHAS=()
printf 'Tabelas importadas: %d\n' "${#TABELAS[@]}"
for t in "${TABELAS[@]+"${TABELAS[@]}"}"; do
  [[ "$t" =~ ^[A-Za-z0-9_]+$ ]] || die "nome de tabela fora do padrao"
  n=$(sql -N -B "$BANCO_TEMP" -e "SELECT COUNT(*) FROM \`$t\`" 2>/dev/null) || die "nao consegui contar a tabela $t"
  [[ "$n" =~ ^[0-9]+$ ]] || die "contagem inesperada na tabela $t"
  LINHAS[$t]=$n
  printf '  %-45s %s\n' "$t" "$n"
done

(( ${#TABELAS[@]} >= TABELAS_MIN )) || die "${#TABELAS[@]} tabela(s); o minimo e $TABELAS_MIN"
for t in usuarios vendedores clientes; do
  [[ -n "${LINHAS[$t]:-}" ]] || die "a tabela $t nao esta no dump"
done
ADMINS=$(sql -N -B "$BANCO_TEMP" -e "SELECT COUNT(*) FROM usuarios WHERE role = 'admin'" 2>/dev/null) \
  || die "nao consegui contar os admins"
printf 'Admins em usuarios: %s\n' "$ADMINS"
[[ "$ADMINS" =~ ^[1-9][0-9]*$ ]] || die "nenhum admin em usuarios"
(( LINHAS[vendedores] > 0 )) || die "a tabela vendedores esta vazia"
if (( LINHAS[clientes] == 0 )); then
  aviso "a tabela clientes esta vazia: confira se a producao ainda nao tem clientes"
fi

# ---------------------------------------------------------------------------
# Registro para o --status (falhar aqui nao desfaz a aprovacao: o --status
# continua acusando a verificacao antiga).
# ---------------------------------------------------------------------------
DIR_ARQ=$(dirname -- "$ARQ")
if SHA=$(sha256sum -- "$ARQ" | awk '{print $1}') \
  && TMP_REG=$(mktemp "$DIR_ARQ/$REGISTRO_NOME.XXXXXX" 2>/dev/null); then
  if ! { printf 'epoch=%s\narquivo=%s\nsha256=%s\n' "$(date +%s)" "$(basename -- "$ARQ")" "$SHA" >"$TMP_REG" \
    && mv -f -- "$TMP_REG" "$DIR_ARQ/$REGISTRO_NOME"; }; then
    rm -f -- "$TMP_REG"
    aviso "nao consegui gravar $DIR_ARQ/$REGISTRO_NOME"
  fi
else
  aviso "nao consegui gravar $DIR_ARQ/$REGISTRO_NOME"
fi

printf 'APROVADO\n'
