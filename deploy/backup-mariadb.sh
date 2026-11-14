#!/usr/bin/env bash
# =============================================================================
# BACKUP-01: backup do MariaDB de producao (container rotaperfumes-mariadb do
# deploy/mariadb-docker.sh). Rodar como root NO SERVIDOR:
#   sudo bash deploy/backup-mariadb.sh                    # backup + retencao
#   sudo bash deploy/backup-mariadb.sh --retencao-dias 30
#   sudo bash deploy/backup-mariadb.sh --conferir-recente 24   # so confere
#
# mariadb-dump consistente (--single-transaction, rotinas, triggers, eventos)
# pelo socket do container (root.cnf montado ro), gzip, arquivo 600 num
# diretorio 700 do root. Erro em qualquer passo = codigo != 0 e nenhum arquivo
# parcial. Agendamento e restauracao: docs/deploy-servidor.md, secao 9.
# Regras: nunca "set -x"; nenhuma senha em argv, log ou tela.
# =============================================================================
set -euo pipefail
umask 077

CONTAINER="rotaperfumes-mariadb"
DB_NOME="rotaperfumes"
DESTINO="/var/backups/rotaperfumes-mariadb"
RETENCAO_DIAS=14
CONFERIR_HORAS=""
# root.cnf do mariadb-docker.sh (protocol=socket), dentro do container.
readonly C_ROOT_CNF="/run/rotaperfumes/root.cnf"
readonly MARCA_FIM="-- Dump completed"

usage() {
  cat <<EOF
Uso: sudo bash $0 [opcoes]

  --container N            container do MariaDB (padrao $CONTAINER)
  --banco N                banco (padrao $DB_NOME)
  --destino DIR            diretorio dos backups (padrao $DESTINO; root, 700)
  --retencao-dias N        apaga backups com mais de N dias (1-3650, padrao $RETENCAO_DIAS)
  --conferir-recente H     nao faz backup: sai 0 se houver backup valido com
                           menos de H horas (1-720), senao 1
  -h, --help               esta ajuda
EOF
}

die() { printf 'ERRO: %s\n' "$*" >&2; exit 1; }
info() { printf '  %s\n' "$*"; }
exigir_valor() { [[ $# -ge 2 && -n "$2" ]] || { printf 'ERRO: a opcao %s exige um valor (veja --help)\n' "$1" >&2; exit 2; }; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --container) exigir_valor "$@"; CONTAINER="$2"; shift 2 ;;
    --banco) exigir_valor "$@"; DB_NOME="$2"; shift 2 ;;
    --destino) exigir_valor "$@"; DESTINO="$2"; shift 2 ;;
    --retencao-dias) exigir_valor "$@"; RETENCAO_DIAS="$2"; shift 2 ;;
    --conferir-recente) exigir_valor "$@"; CONFERIR_HORAS="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Opcao desconhecida: $1" >&2; usage >&2; exit 2 ;;
  esac
done

invalida() { printf 'ERRO: %s\n' "$*" >&2; exit 2; }
[[ "$CONTAINER" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || invalida "--container invalido: $CONTAINER"
[[ "$DB_NOME" =~ ^[A-Za-z0-9_]{1,64}$ ]] || invalida "--banco invalido: $DB_NOME"
[[ "$DESTINO" =~ ^/[A-Za-z0-9._/-]+$ && "$DESTINO" != *..* ]] || invalida "--destino invalido: '$DESTINO' (absoluto, sem espacos/..)"
DESTINO=${DESTINO%/}
# shellcheck disable=SC2015 # invalida deve rodar se o formato OU a faixa falhar (pretendido)
[[ "$RETENCAO_DIAS" =~ ^[0-9]{1,4}$ ]] && (( 10#$RETENCAO_DIAS >= 1 && 10#$RETENCAO_DIAS <= 3650 )) \
  || invalida "--retencao-dias invalido: $RETENCAO_DIAS (1 a 3650)"
RETENCAO_DIAS=$((10#$RETENCAO_DIAS))
if [[ -n "$CONFERIR_HORAS" ]]; then
  # shellcheck disable=SC2015 # idem: formato ou faixa
  [[ "$CONFERIR_HORAS" =~ ^[0-9]{1,3}$ ]] && (( 10#$CONFERIR_HORAS >= 1 && 10#$CONFERIR_HORAS <= 720 )) \
    || invalida "--conferir-recente invalido: $CONFERIR_HORAS (1 a 720 horas)"
  CONFERIR_HORAS=$((10#$CONFERIR_HORAS))
fi

[[ $EUID -eq 0 ]] || die "rode como root: sudo bash $0"
for cmd in docker gzip sha256sum find stat realpath dirname mktemp tail flock; do
  command -v "$cmd" >/dev/null || die "comando '$cmd' nao encontrado"
done

# Mesmas regras do --segredos-dir do mariadb-docker.sh (TST-16/Lote 36): sem
# symlink no caminho, ancestrais do root sem escrita de grupo/outros e, se
# existir, diretorio real do root com modo 700. Falha fechada (nao corrige).
conferir_destino() { # [existe]
  local d=$DESTINO real p dono modo
  [[ -L "$d" ]] && die "--destino $d e um link simbolico: use o caminho real"
  real=$(realpath -m -- "$d") || die "--destino $d: realpath falhou"
  [[ "$real" == "$d" ]] || die "--destino $d nao e o caminho real ($real): nenhum componente pode ser link simbolico"
  p=$d
  while [[ "$p" != / ]]; do
    p=$(dirname -- "$p")
    [[ -e "$p" || -L "$p" ]] || continue
    [[ -d "$p" && ! -L "$p" ]] || die "--destino $d: o ancestral $p nao e diretorio real"
    read -r dono modo < <(stat -c '%u %a' -- "$p")
    [[ "$dono" == 0 ]] || die "--destino $d: o ancestral $p precisa ser do root (dono atual: $dono)"
    (( (8#$modo & 8#022) == 0 )) || die "--destino $d: o ancestral $p tem escrita para grupo/outros (modo $modo)"
  done
  if [[ -e "$d" ]]; then
    [[ -d "$d" ]] || die "--destino $d existe e nao e diretorio"
    [[ "$(stat -c '%u %a' -- "$d")" == "0 700" ]] \
      || die "--destino $d precisa ser do root com modo 700 (atual: $(stat -c 'dono %u, modo %a' -- "$d")); ajuste: chown root:root; chmod 700"
  elif [[ "${1:-}" == existe ]]; then
    die "--destino $d nao existe"
  fi
}

# backup_valido ARQ: arquivo real do root, 600, gzip integro, termina com a
# marca do mariadb-dump e confere com o .sha256 ao lado.
backup_valido() {
  local f=$1 nome esperado atual
  [[ -f "$f" && ! -L "$f" ]] || return 1
  [[ "$(stat -c '%u %a' -- "$f")" == "0 600" ]] || return 1
  gzip -t -- "$f" 2>/dev/null || return 1
  [[ "$(gzip -dc -- "$f" | tail -n 1)" == "$MARCA_FIM"* ]] || return 1
  [[ -f "$f.sha256" && ! -L "$f.sha256" ]] || return 1
  nome=$(basename -- "$f")
  read -r esperado _ <"$f.sha256" || return 1
  atual=$(sha256sum -- "$f" | cut -d' ' -f1)
  [[ "$esperado" == "$atual" && "$(cut -d' ' -f3- "$f.sha256")" == "$nome" ]]
}

# Backups deste banco, do mais novo para o mais antigo (nome com a data).
listar_backups() { # [minutos]: so os modificados ha menos de N minutos
  local filtro=()
  [[ -n "${1:-}" ]] && filtro=(-mmin "-$1")
  find "$DESTINO" -maxdepth 1 -type f -name "$DB_NOME-[0-9]*-[0-9]*.sql.gz" "${filtro[@]}" -print | sort -r
}

# ---------------------------------------------------------------------------
if [[ -n "$CONFERIR_HORAS" ]]; then
  [[ -d "$DESTINO" ]] || { printf 'Nenhum backup: %s nao existe.\n' "$DESTINO" >&2; exit 1; }
  conferir_destino existe
  while IFS= read -r f; do
    if backup_valido "$f"; then
      printf 'Backup recente valido (< %d h): %s\n' "$CONFERIR_HORAS" "$f"
      exit 0
    fi
    printf 'AVISO: backup invalido ignorado (dono/modo, gzip, marca final ou sha256): %s\n' "$f" >&2
  done < <(listar_backups $((CONFERIR_HORAS * 60)))
  printf 'Nenhum backup valido com menos de %d h em %s.\n' "$CONFERIR_HORAS" "$DESTINO" >&2
  exit 1
fi

# ---------------------------------------------------------------------------
echo "=== Backup do banco $DB_NOME (container $CONTAINER) ==="
docker info >/dev/null 2>&1 || die "o daemon do Docker nao responde"
[[ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER" 2>/dev/null)" == true ]] \
  || die "o container $CONTAINER nao esta rodando (docker start $CONTAINER)"

conferir_destino
install -d -m 700 -o root -g root "$DESTINO"
conferir_destino existe   # de novo depois do install -d (TOCTOU)

# Um backup por vez (o timer e uma execucao manual podem coincidir).
exec 9>"$DESTINO/.lock"
flock -n 9 || die "outro backup em andamento ($DESTINO/.lock)"

FINAL="$DESTINO/$DB_NOME-$(date +%Y%m%d-%H%M%S).sql.gz"
[[ ! -e "$FINAL" ]] || die "$FINAL ja existe (rode de novo em 1 s)"
TMP=$(mktemp "$DESTINO/.parcial.XXXXXX")
TMP_SHA="$TMP.sha256"
# Sem arquivo parcial em erro, interrupcao ou sinal.
trap 'rm -f -- "$TMP" "$TMP_SHA"' EXIT
trap 'exit 130' INT TERM HUP

# --single-transaction: leitura consistente do InnoDB sem travar a API.
# A senha do root fica no root.cnf do container (nunca no argv).
if ! docker exec "$CONTAINER" mariadb-dump --defaults-extra-file="$C_ROOT_CNF" \
      --single-transaction --quick --routines --triggers --events --hex-blob \
      --default-character-set=utf8mb4 --add-drop-table "$DB_NOME" | gzip -c >"$TMP"; then
  die "mariadb-dump ou gzip falhou (nenhum backup gravado)"
fi
gzip -t -- "$TMP" || die "gzip do backup corrompido (nenhum backup gravado)"
[[ "$(gzip -dc -- "$TMP" | tail -n 1)" == "$MARCA_FIM"* ]] \
  || die "dump incompleto: sem a linha final '$MARCA_FIM' (nenhum backup gravado)"
chmod 600 "$TMP"
printf '%s  %s\n' "$(sha256sum -- "$TMP" | cut -d' ' -f1)" "$(basename -- "$FINAL")" >"$TMP_SHA"
chmod 600 "$TMP_SHA"
mv -f -- "$TMP_SHA" "$FINAL.sha256"
mv -f -- "$TMP" "$FINAL"
backup_valido "$FINAL" || { rm -f -- "$FINAL" "$FINAL.sha256"; die "o backup gravado nao passou na conferencia (removido)"; }
info "Backup: $FINAL ($(stat -c %s -- "$FINAL") bytes, modo 600; sha256 em $(basename -- "$FINAL").sha256)."

# Retencao: so arquivos deste banco com o padrao de nome do script; o recem
# gravado nunca entra (tem menos de 1 dia). Parciais esquecidos (kill -9) saem
# depois de 1 h.
REMOVIDOS=0
while IFS= read -r f; do
  rm -f -- "$f" "$f.sha256"
  REMOVIDOS=$((REMOVIDOS + 1))
done < <(find "$DESTINO" -maxdepth 1 -type f -name "$DB_NOME-[0-9]*-[0-9]*.sql.gz" -mmin "+$((RETENCAO_DIAS * 1440))" -print)
find "$DESTINO" -maxdepth 1 -type f -name '.parcial.*' -mmin +60 -delete
info "Retencao $RETENCAO_DIAS dias: $REMOVIDOS backup(s) antigo(s) removido(s)."
