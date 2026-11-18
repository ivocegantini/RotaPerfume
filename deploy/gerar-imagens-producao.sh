#!/usr/bin/env bash
# =============================================================================
# IMG-02: gera as imagens de produto (PNG com o nome na diagonal, IMG-01) no
# banco e no volume de PRODUCAO. Substitui o "make db-gerar-imagens-produtos",
# que nao alcanca a producao (3306 fechada, DEPLOY-10).
#
# Rodar como root NO SERVIDOR, a partir do checkout do repo:
#   sudo bash deploy/gerar-imagens-producao.sh --dry-run     # so lista
#   sudo bash deploy/gerar-imagens-producao.sh --confirmar   # grava
#   sudo bash deploy/gerar-imagens-producao.sh --help
#
# Etapas:
#   1. confere root, comandos, container/rede, CA, senha do rotaperfumes_admin,
#      volume rotaperfumes_produto_imagens e, com --confirmar, um backup de
#      ate 60 min (BACKUP-01); nada muda antes
#   2. compila o cmd/gerarimagens da API num container GO_IMAGE (sem acesso ao
#      banco nem a segredos)
#   3. roda o gerarimagens na rede rotaperfumes-db (IP do admin, .20), como o
#      uid 65532 dono do volume, com --read-only, --cap-drop ALL e --env-file
#      temporario 600 (shred no fim). Grava pelo mesmo caso de uso do upload.
#   4. confere: produtos com imagem no banco e arquivos no volume
#
# JWT_SECRET e PASSWORD_PEPPER: o config.Load os exige, mas o gerarimagens nao
# os usa (nao gera token nem hash). Vao valores aleatorios por execucao, so no
# env-file temporario; o pepper real nunca e lido.
#
# Regras: nunca "set -x"; a senha do banco nunca vai para argv, log ou tela.
# =============================================================================
set -euo pipefail
umask 077

GO_IMAGE="golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280"

readonly DB_NOME="rotaperfumes"
readonly DB_ADMIN="rotaperfumes_admin"
readonly C_CA="/etc/mysql/ssl/ca.pem"
readonly CONTAINER="rotaperfumes-mariadb"
readonly REDE_NOME="rotaperfumes-db"
readonly IP_ADMIN="172.30.42.20"
readonly SEGREDOS_DIR="/root/rotaperfumes-mariadb"
readonly CA_HOST="/etc/mysql/ssl/ca.pem"
readonly TRABALHO_DIR="/root"
readonly SENHA_DB_ARQ="$SEGREDOS_DIR/$DB_ADMIN.senha"
# Volume do compose "rotaperfumes" (deploy/docker-compose.yml) e o dono criado
# no backend/Dockerfile (nonroot da distroless).
readonly VOLUME="rotaperfumes_produto_imagens"
readonly UID_IMAGENS="65532"
readonly DIR_IMAGENS="/data/produtos-imagens"
readonly BACKUP_JANELA_MIN=60

MODO=""
SOBRESCREVER=false
BACKUP_DIR="/var/backups/rotaperfumes"
SEM_BACKUP_MOTIVO=""
BACKUP_USADO=""

RAIZ=$(cd "$(dirname "$0")/.." && pwd -P)
readonly RAIZ

usage() {
  cat <<'EOF'
Uso: sudo bash deploy/gerar-imagens-producao.sh (--dry-run | --confirmar) [opcoes]

  --dry-run                  so lista os produtos que receberiam imagem (nao grava)
  --confirmar                grava as imagens (exige backup de ate 60 min)
  --sobrescrever             troca tambem as imagens existentes (so com --confirmar)
  --backup-dir DIR           onde procurar o backup (padrao /var/backups/rotaperfumes)
  --sem-backup-motivo TEXTO  grava sem conferir backup (10 a 300 caracteres; registrado)
  -h, --help                 esta ajuda

Antes do --confirmar: sudo rotaperfumes-backup-banco --agora
EOF
}

die()   { printf 'ERRO: %s\n' "$*" >&2; exit 1; }
info()  { printf '  %s\n' "$*"; }
aviso() { printf '  AVISO: %s\n' "$*" >&2; }
exigir_valor() { [[ $# -ge 2 && -n "$2" ]] || die "a opcao $1 exige um valor (veja --help)"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) [[ -z "$MODO" || "$MODO" == dry-run ]] || die "use --dry-run OU --confirmar"; MODO=dry-run; shift ;;
    --confirmar) [[ -z "$MODO" || "$MODO" == confirmar ]] || die "use --dry-run OU --confirmar"; MODO=confirmar; shift ;;
    --sobrescrever) SOBRESCREVER=true; shift ;;
    --backup-dir) exigir_valor "$@"; BACKUP_DIR="$2"; shift 2 ;;
    --sem-backup-motivo) exigir_valor "$@"; SEM_BACKUP_MOTIVO="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Opcao desconhecida: $1" >&2; usage >&2; exit 2 ;;
  esac
done
[[ -n "$MODO" ]] || { echo "ERRO: escolha --dry-run ou --confirmar" >&2; usage >&2; exit 2; }
if [[ "$MODO" == dry-run ]]; then
  $SOBRESCREVER && die "--sobrescrever so vale com --confirmar"
  [[ -z "$SEM_BACKUP_MOTIVO" ]] || die "--sem-backup-motivo so vale com --confirmar"
fi
if [[ -n "$SEM_BACKUP_MOTIVO" ]]; then
  (( ${#SEM_BACKUP_MOTIVO} >= 10 && ${#SEM_BACKUP_MOTIVO} <= 300 )) || die "--sem-backup-motivo precisa ter de 10 a 300 caracteres"
  [[ "$SEM_BACKUP_MOTIVO" != *$'\n'* ]] || die "--sem-backup-motivo sem quebra de linha"
fi

ETAPA=0
readonly TOTAL_ETAPAS=4
etapa() { ETAPA=$((ETAPA + 1)); printf '\n=== [%d/%d] %s ===\n' "$ETAPA" "$TOTAL_ETAPAS" "$1"; }

TRAB=""
ENV_ARQ=""
limpar() {
  [[ -n "$ENV_ARQ" && -f "$ENV_ARQ" ]] && shred -u "$ENV_ARQ" 2>/dev/null
  [[ -n "$TRAB" && -d "$TRAB" ]] && rm -rf "$TRAB"
  return 0
}
trap limpar EXIT

arquivo_secreto_ok() {
  [[ -f "$1" && ! -L "$1" ]] || return 1
  [[ "$(stat -c '%u' "$1")" == 0 ]] || return 1
  case "$(stat -c '%a' "$1")" in 600|400) return 0 ;; *) return 1 ;; esac
}

readonly RE_OCTETO='(25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])'

conferir_dir_privado() { # opcao dir [existe]
  local op=$1 d=$2 exigir=${3:-} real p dono modo
  [[ -L "$d" ]] && die "$op $d e um link simbolico: use o caminho real"
  real=$(realpath -m -- "$d") || die "$op $d: realpath falhou"
  [[ "$real" == "$d" ]] \
    || die "$op $d nao e o caminho real ($real): nenhum componente pode ser link simbolico (nem '//' ou '.')"
  p=$d
  while [[ "$p" != / ]]; do
    p=$(dirname -- "$p")
    [[ -e "$p" || -L "$p" ]] || continue
    [[ -d "$p" && ! -L "$p" ]] || die "$op $d: o ancestral $p nao e diretorio real"
    read -r dono modo < <(stat -c '%u %a' -- "$p")
    [[ "$dono" == 0 ]] \
      || die "$op $d: o ancestral $p precisa ser do root (dono atual: $dono); outro usuario poderia trocar o caminho"
    (( (8#$modo & 8#022) == 0 )) \
      || die "$op $d: o ancestral $p tem escrita para grupo/outros (modo $modo; o sticky bit nao basta); use um caminho so com ancestrais do root sem escrita para grupo e outros"
  done
  if [[ -e "$d" ]]; then
    [[ -d "$d" ]] || die "$op $d existe e nao e diretorio"
    [[ "$(stat -c '%u %a' -- "$d")" == "0 700" ]] \
      || die "$op $d precisa ser do root com modo 700 (atual: dono $(stat -c '%u' -- "$d"), modo $(stat -c '%a' -- "$d")); confira e ajuste (chown root:root; chmod 700)"
  elif [[ "$exigir" == existe ]]; then
    die "$op $d nao existe depois do install -d"
  fi
}

conferir_backup_recente() { # dir janela_min -> define BACKUP_USADO
  local dir=$1 janela=$2 LC_ALL=C arq="" f nome agora mtime carimbo dica
  dica="gere um com: sudo rotaperfumes-backup-banco --agora (ou use --sem-backup-motivo TEXTO, que fica registrado)"
  command -v gzip >/dev/null || die "comando 'gzip' nao encontrado"
  [[ -d "$dir" && ! -L "$dir" ]] || die "nenhum backup: $dir nao existe. Instale com sudo bash deploy/backup-banco.sh --instalar; $dica"
  conferir_dir_privado --backup-dir "$dir" existe
  for f in "$dir"/rotaperfumes-*.sql.gz; do
    [[ "${f##*/}" =~ ^rotaperfumes-[0-9]{8}-[0-9]{6}Z(-manual)?\.sql\.gz$ ]] && arq=$f
  done
  [[ -n "$arq" ]] || die "nenhum backup em $dir; $dica"
  nome=${arq##*/}
  [[ ! -L "$arq" && -f "$arq" ]] || die "o backup mais recente ($nome) nao e arquivo regular; $dica"
  [[ "$(stat -c '%u %a' -- "$arq")" == "0 600" ]] || die "o backup mais recente ($nome) precisa ser do root com modo 600; $dica"
  [[ "$nome" =~ ^rotaperfumes-([0-9]{4})([0-9]{2})([0-9]{2})-([0-9]{2})([0-9]{2})([0-9]{2})Z ]] || die "carimbo invalido em $nome"
  carimbo=$(date -u -d "${BASH_REMATCH[1]}-${BASH_REMATCH[2]}-${BASH_REMATCH[3]} ${BASH_REMATCH[4]}:${BASH_REMATCH[5]}:${BASH_REMATCH[6]}" +%s) \
    || die "carimbo invalido em $nome"
  agora=$(date +%s)
  mtime=$(stat -c %Y -- "$arq")
  for f in "$carimbo" "$mtime"; do
    (( f <= agora )) || die "o backup mais recente ($nome) tem data futura; $dica"
    (( agora - f <= janela * 60 )) || die "o backup mais recente ($nome) tem mais de $janela min; $dica"
  done
  gzip -t -- "$arq" 2>/dev/null || die "o backup mais recente ($nome) esta corrompido; $dica"
  [[ "$(gzip -dc -- "$arq" | tail -n 1)" == "-- Dump completed"* ]] || die "o backup mais recente ($nome) nao tem o marcador na ultima linha; $dica"
  BACKUP_USADO=$arq
}

conferir_ca_host() {
  local dono modo
  [[ -f "$CA_HOST" && ! -L "$CA_HOST" && -s "$CA_HOST" ]] \
    || die "CA do banco nao encontrada em $CA_HOST (arquivo regular nao vazio, sem symlink; deploy/mariadb-docker.sh)"
  read -r dono modo < <(stat -c '%u %a' -- "$CA_HOST")
  if [[ "$dono" != 0 ]] || (( (8#$modo & 8#022) != 0 )); then
    die "CA $CA_HOST precisa ser do root sem escrita para grupo/outros (atual: dono $dono, modo $modo)"
  fi
}

conferir_container_rede() {
  local redes r ligado=false subnets s v4=() prefixo
  [[ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null || true)" == "true" ]] \
    || die "o container $CONTAINER nao esta rodando: rode antes sudo bash deploy/mariadb-docker.sh"
  docker network inspect "$REDE_NOME" >/dev/null 2>&1 || die "a rede docker $REDE_NOME nao existe (deploy/mariadb-docker.sh)"
  redes=$(docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{println $k}}{{end}}' "$CONTAINER" 2>/dev/null) \
    || die "nao foi possivel ler as redes do container $CONTAINER"
  while IFS= read -r r; do
    [[ "$r" == "$REDE_NOME" ]] && ligado=true
  done <<<"$redes"
  $ligado || die "o container $CONTAINER nao esta ligado a rede $REDE_NOME"
  subnets=$(docker network inspect -f '{{range .IPAM.Config}}{{.Subnet}} {{end}}' "$REDE_NOME" 2>/dev/null) \
    || die "nao foi possivel ler as subnets da rede $REDE_NOME"
  for s in $subnets; do
    [[ "$s" == *:* ]] && continue
    v4+=("$s")
  done
  (( ${#v4[@]} == 1 )) || die "a rede $REDE_NOME precisa ter exatamente uma subnet IPv4 (tem ${#v4[@]}: ${v4[*]:-nenhuma})"
  [[ "${v4[0]}" =~ ^($RE_OCTETO\.$RE_OCTETO\.$RE_OCTETO)\.0/24$ ]] \
    || die "a subnet da rede $REDE_NOME (${v4[0]}) nao e A.B.C.0/24 (deploy/mariadb-docker.sh)"
  prefixo=${BASH_REMATCH[1]}
  [[ "$IP_ADMIN" == "$prefixo.20" ]] \
    || die "--ip-admin $IP_ADMIN fora do esperado: na rede $REDE_NOME (${v4[0]}) a conta $DB_ADMIN so existe em $prefixo.20"
}

# conferir_volume: o volume do compose existe, e local e a raiz e do uid
# 65532 (o mesmo do container da API). So inspect.
conferir_volume() {
  local driver projeto dono
  driver=$(docker volume inspect -f '{{.Driver}}' "$VOLUME" 2>/dev/null) \
    || die "o volume $VOLUME nao existe: ele nasce no 1o deploy da API (Jenkins)"
  [[ "$driver" == local ]] || die "o volume $VOLUME usa o driver '$driver' (esperado local)"
  projeto=$(docker volume inspect -f '{{index .Labels "com.docker.compose.project"}}' "$VOLUME" 2>/dev/null || true)
  [[ "$projeto" == rotaperfumes ]] || die "o volume $VOLUME nao e do compose rotaperfumes (label: '${projeto:-nenhuma}')"
  dono=$(docker run --rm --network none --cap-drop ALL --security-opt no-new-privileges:true --read-only \
           --mount "type=volume,source=$VOLUME,target=/v,readonly" --entrypoint stat "$GO_IMAGE" -c '%u' /v </dev/null) \
    || die "nao consegui ler o dono do volume $VOLUME"
  [[ "$dono" == "$UID_IMAGENS" ]] || die "a raiz do volume $VOLUME e do uid $dono (esperado $UID_IMAGENS, o da API)"
}

# Como o dono (65532): root sem CAP_DAC_OVERRIDE nao le o volume; pipefail
# faz o erro do find derrubar a contagem em vez de imprimir 0.
contar_arquivos_volume() {
  docker run --rm --network none --cap-drop ALL --security-opt no-new-privileges:true --read-only \
    --user "$UID_IMAGENS:$UID_IMAGENS" \
    --mount "type=volume,source=$VOLUME,target=/v,readonly" --entrypoint sh "$GO_IMAGE" \
    -c 'set -o pipefail; find /v -type f | wc -l' </dev/null
}

# ---------------------------------------------------------------------------
etapa "Pre-requisitos"
[[ $EUID -eq 0 ]] || die "rode como root: sudo bash $0"
for cmd in docker tar shred stat install mktemp grep realpath dirname openssl; do
  command -v "$cmd" >/dev/null || die "comando '$cmd' nao encontrado"
done
docker info >/dev/null 2>&1 || die "o daemon do Docker nao responde"
[[ "$GO_IMAGE" =~ ^golang:[0-9.]+-alpine@sha256:[0-9a-f]{64}$ ]] || die "GO_IMAGE invalida: $GO_IMAGE"
[[ -r "$RAIZ/backend/rotaperfumes-api/go.mod" && -r "$RAIZ/backend/shared/go.mod" ]] \
  || die "backend/ nao encontrado em $RAIZ (rode a partir do checkout do repositorio)"
conferir_dir_privado --segredos-dir "$SEGREDOS_DIR" existe
conferir_dir_privado --trabalho-dir "$TRABALHO_DIR" existe
conferir_ca_host
conferir_container_rede
arquivo_secreto_ok "$SENHA_DB_ARQ" || die "$SENHA_DB_ARQ ausente ou com dono/modo errado (esperado root, 600)"
SENHA_DB=$(<"$SENHA_DB_ARQ")
[[ "$SENHA_DB" =~ ^[A-Za-z0-9]{20,}$ ]] || die "senha em $SENHA_DB_ARQ fora do formato [A-Za-z0-9]{20,}"
conferir_volume
if [[ "$MODO" == confirmar ]]; then
  if [[ -n "$SEM_BACKUP_MOTIVO" ]]; then
    aviso "--sem-backup-motivo: grava SEM conferir backup. Motivo: $SEM_BACKUP_MOTIVO"
  else
    conferir_backup_recente "$BACKUP_DIR" "$BACKUP_JANELA_MIN"
    info "Backup recente conferido: $BACKUP_USADO."
  fi
fi
ANTES=$(contar_arquivos_volume) || die "nao consegui contar os arquivos do volume $VOLUME"
info "Pre-requisitos OK (modo $MODO; volume $VOLUME com $ANTES arquivo(s); nada foi impresso)."

TRAB=$(mktemp -d "$TRABALHO_DIR/rotaperfumes-imagens.XXXXXX") || die "mktemp em $TRABALHO_DIR falhou"
[[ ! -L "$TRAB" && "$(realpath -e -- "$TRAB")" == "$TRAB" && "$(stat -c '%u %a' -- "$TRAB")" == "0 700" ]] \
  || die "area de trabalho $TRAB fora do esperado (caminho real, root, modo 700)"
install -d -m 700 -o root -g root "$TRAB/bin"

# ---------------------------------------------------------------------------
etapa "Compilando o gerarimagens ($GO_IMAGE)"
# Sem rede do banco e sem segredos: so o codigo de backend/ por stdin (sem
# .env). --cap-drop ALL; o tar extrai sem chown (-o).
tar -C "$RAIZ" -cf - --exclude='.env' --exclude='.env.*' --exclude='*.env' \
    --exclude='coverage*.txt' --exclude='coverage.html' --exclude='backend/rotaperfumes-api/data' \
    backend/shared backend/rotaperfumes-api \
  | docker run --rm -i --cap-drop ALL --security-opt no-new-privileges:true \
      -e CGO_ENABLED=0 -e GOFLAGS=-mod=readonly \
      --mount "type=bind,source=$TRAB/bin,target=/out" \
      "$GO_IMAGE" sh -ec '
        mkdir -p /src && tar -xof - -C /src
        cd /src/backend/rotaperfumes-api && go build -trimpath -o /out/gerarimagens ./cmd/gerarimagens
      ' || die "falhou a compilacao do gerarimagens"
[[ -x "$TRAB/bin/gerarimagens" ]] || die "binario do gerarimagens nao foi gerado"
chmod 755 "$TRAB/bin" "$TRAB/bin/gerarimagens"
info "gerarimagens compilado."

# ---------------------------------------------------------------------------
etapa "Gerando as imagens ($MODO)"
ENV_ARQ="$TRAB/gerarimagens.env"
{
  printf 'DB_HOST=%s\n' "$CONTAINER"
  printf 'DB_PORT=3306\n'
  printf 'DB_USUARIO=%s\n' "$DB_ADMIN"
  printf 'DB_SENHA=%s\n' "$SENHA_DB"
  printf 'DB_NAME=%s\n' "$DB_NOME"
  printf 'DB_TLS_CA=%s\n' "$C_CA"
  printf 'DB_TLS_SERVER_NAME=%s\n' "$CONTAINER"
  printf 'PRODUTO_IMAGENS_DIR=%s\n' "$DIR_IMAGENS"
  printf 'JWT_SECRET=%s\n' "$(openssl rand -hex 32)"
  printf 'PASSWORD_PEPPER=%s\n' "$(openssl rand -hex 32)"
} > "$ENV_ARQ"
chmod 600 "$ENV_ARQ"
unset SENHA_DB

ARGS_GI=()
if [[ "$MODO" == dry-run ]]; then
  ARGS_GI=(-dry-run)
  VOL_MODO=",readonly"
else
  ARGS_GI=(-confirmar)
  $SOBRESCREVER && ARGS_GI+=(-sobrescrever)
  VOL_MODO=""
  printf '%s modo=%s sobrescrever=%s backup=%s motivo=%s\n' "$(date -Is)" "$MODO" "$SOBRESCREVER" \
    "${BACKUP_USADO:-nenhum}" "${SEM_BACKUP_MOTIVO:-nenhum}" >>"$SEGREDOS_DIR/gerar-imagens.registro"
  chmod 600 "$SEGREDOS_DIR/gerar-imagens.registro"
fi
docker run --rm --network "$REDE_NOME" --ip "$IP_ADMIN" \
  --user "$UID_IMAGENS:$UID_IMAGENS" \
  --cap-drop ALL --security-opt no-new-privileges:true --read-only \
  --env-file "$ENV_ARQ" \
  --mount "type=bind,source=$CA_HOST,target=$C_CA,readonly" \
  --mount "type=bind,source=$TRAB/bin/gerarimagens,target=/usr/local/bin/gerarimagens,readonly" \
  --mount "type=volume,source=$VOLUME,target=$DIR_IMAGENS$VOL_MODO" \
  -w / --entrypoint /usr/local/bin/gerarimagens \
  "$GO_IMAGE" "${ARGS_GI[@]}" </dev/null \
  || die "o gerarimagens falhou (veja a mensagem acima; o que ja foi gravado fica, e rodar de novo pula os produtos com imagem)"
shred -u "$ENV_ARQ"
ENV_ARQ=""

# ---------------------------------------------------------------------------
etapa "Conferencia"
DEPOIS=$(contar_arquivos_volume) || die "nao consegui contar os arquivos do volume $VOLUME"
info "Arquivos no volume $VOLUME: $ANTES antes, $DEPOIS depois."
if [[ "$MODO" == confirmar ]]; then
  cat <<EOF

Pronto. A vitrine mostra as imagens em ate 5 min (TTL do cache, CACHE-01).
Depois rode: sudo rotaperfumes-backup-banco --agora (o backup leva as imagens novas).
EOF
else
  cat <<EOF

Simulacao: nada foi gravado. Para gravar:
  sudo rotaperfumes-backup-banco --agora
  sudo bash deploy/gerar-imagens-producao.sh --confirmar
EOF
fi
