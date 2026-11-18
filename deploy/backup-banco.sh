#!/usr/bin/env bash
# =============================================================================
# BACKUP-01: backup do site no servidor. Cada backup bom e um PAR de arquivos,
# com o mesmo carimbo UTC:
#   rotaperfumes-AAAAMMDD-HHMMSSZ[-manual].sql.gz          dump do banco
#   rotaperfumes-imagens-AAAAMMDD-HHMMSSZ[-manual].tar.gz  imagens de produto
#
#   sudo bash deploy/backup-banco.sh --instalar   copia este script, cria as
#                                                 units e roda o 1o backup
#   sudo rotaperfumes-backup-banco --agora        backup na serie -manual
#   sudo rotaperfumes-backup-banco --status       sai com erro se algo parou
#
# O root nunca executa do checkout: --agora, --status e --agendado so rodam na
# copia /usr/local/sbin/rotaperfumes-backup-banco (700, root), senao um push na
# main viraria codigo rodando como root. Depois de trocar o digest da imagem
# (ou de atualizar este arquivo), rode --instalar de novo.
#
# Ordem do backup (qualquer falha encerra sem tocar nos backups bons):
#   1. trava (flock); ocupada e falha, nunca sucesso silencioso
#   2. remove clientes presos (so pelos nomes exatos) e temporarios antigos
#   3. espaco: livre >= 2x dump anterior + max(2x imagens anteriores, volume) +
#      1 GiB. O backup divide o disco com o datadir: encher derruba o banco.
#   4. dump: codigos do pipe, gzip -t, marcador na ULTIMA linha, tabelas,
#      dados em usuarios, piso de tamanho e queda de ate 50%
#   5. imagens: volume conferido pelas labels do compose, somente leitura, sem
#      rede; codigos do pipe, gzip -t e listagem do tar (nada absoluto, sem
#      "..", so arquivos e diretorios). A queda de 50% nao vale aqui: remover
#      produto e legitimo. Volume vazio e aceito com AVISO.
#   6. publica o par: imagens primeiro; o nome final do dump fecha o par
#   7. retencao, por contagem de pares bons e por serie. Falha sem apagar nada,
#      com o par novo intacto, se houver par com data futura, se ela fosse apagar o
#      par novo ou se o novo, um par que fica na serie ou um dos dois mais novos
#      estiver corrompido (BACKUP-05: rele todos os que ficam, so quando ha o que apagar).
#
# Falha nas imagens: o dump ja validado vira parcial-rotaperfumes-*.sql.gz,
# guardado para uso manual. Nao conta como backup bom e nao dispara a retencao;
# a guarda do --recriar e do seed --forcar o aceita, com aviso (BACKUP-04). O
# script sai com erro. So os
# $PARCIAIS_MANTER parciais mais novos ficam, para falhas repetidas nao encherem
# o disco. Um SIGTERM ou SIGINT apaga os temporarios, inclusive o dump.
#
# Volume ausente e sempre falha: ele nasce no 1o deploy (docker compose) e toda
# instalacao que serve o site o tem. Antes do 1o deploy, --instalar recusa.
#
# Destino em outro disco: --instalar --backup-dir /mnt/ssd/backups
# --exigir-montagem /mnt/ssd. Com o disco desmontado, o backup falha em vez de
# gravar no disco do sistema: a unit ganha RequiresMountsFor= e o script confere
# com mountpoint. Cifrar e abrir o disco fica fora deste script.
#
# Regras: nunca "set -x"; a senha fica so no option file montado (fora do argv,
# do "docker inspect" e do ambiente); o log traz caminho, tamanho, contagem,
# duracao, etapa que falhou e o OK do marcador (texto fixo), nunca linhas do dump
# nem nomes do tar.
# Codigos de saida: 0 sucesso, 2 uso incorreto, 1 qualquer outra falha.
# =============================================================================
set -euo pipefail
umask 077
export LC_ALL=C

# SEC-31: a mesma string do IMAGEM_BASE do deploy/mariadb-docker.sh (teste de
# paridade em backend/shared/tests/deploy).
IMAGEM_BASE="mariadb:11.4.13@sha256:5e73baf8179b29f115101bb102763a47a017361f3e9792daccba6646c3ab1532"
readonly IMAGEM_BASE

readonly DESTINO="/usr/local/sbin/rotaperfumes-backup-banco"
readonly CONF="/etc/rotaperfumes/backup-banco.conf"
readonly SERVICE="rotaperfumes-backup-banco.service"
readonly TIMER="rotaperfumes-backup-banco.timer"
readonly UNIT_DIR="/etc/systemd/system"

readonly CLIENTE="rotaperfumes-backup-cliente"
readonly CLIENTE_IMAGENS="rotaperfumes-backup-imagens"
readonly REDE_NOME="rotaperfumes-db"
readonly IP_BACKUP="172.30.42.21"
readonly DB_HOST="rotaperfumes-mariadb"
readonly DB_NOME="rotaperfumes"
readonly C_CA="/run/backup/ca.pem"
readonly C_CNF="/run/backup/cliente.cnf"

# Compose: "name: rotaperfumes" + volume produto_imagens (dono 65532, o usuario
# da API; testes conferem deploy/docker-compose.yml e backend/Dockerfile). O
# nome nao basta: as labels provam que o volume e o criado por esse compose.
readonly VOLUME_IMAGENS="rotaperfumes_produto_imagens"
readonly VOLUME_ROTULO="rotaperfumes/produto_imagens"
readonly USUARIO_IMAGENS="65532:65532"
readonly C_IMAGENS="/imagens"

readonly SSL_DIR="/etc/mysql/ssl"
readonly CA_HOST="$SSL_DIR/ca.pem"
readonly SEGREDOS_DIR="/root/rotaperfumes-mariadb"
readonly CNF_DIR="$SEGREDOS_DIR/backup-cliente"
readonly CNF="$CNF_DIR/rotaperfumes_backup.cnf"
readonly DADOS_DIR="/var/lib/rotaperfumes/mariadb"
readonly CA_DIR="/root/rotaperfumes-ca"

# Padroes; --instalar grava os valores escolhidos em $CONF.
readonly BACKUP_DIR_PADRAO="/var/backups/rotaperfumes"
readonly RETENCAO_DIARIOS_PADRAO=14
readonly RETENCAO_MANUAIS_PADRAO=3

readonly PARCIAIS_MANTER=3
readonly RETENCAO_MIN=2
readonly RESERVA_BYTES=$((1024 * 1024 * 1024))
readonly PISO_DUMP_BYTES=4096
readonly TABELAS_MIN=13
readonly IDADE_MAX_HORAS=26
readonly VERIFICACAO_MAX_DIAS=35
# Folga para ajuste de relogio ao julgar "data futura" nos pares ja publicados.
readonly TOLERANCIA_RELOGIO_S=300
readonly ESPERA_1O_BACKUP_S=$((3 * 3600))

readonly CARIMBO_RE='[0-9]{8}-[0-9]{6}Z(-manual)?'
readonly RE_DUMP="^rotaperfumes-$CARIMBO_RE\\.sql\\.gz\$"
readonly RE_IMAGENS="^rotaperfumes-imagens-$CARIMBO_RE\\.tar\\.gz\$"
readonly RE_PARCIAL="^parcial-rotaperfumes-$CARIMBO_RE\\.sql\\.gz\$"

usage() {
  cat <<EOF
Uso:
  sudo bash $0 --instalar [--backup-dir DIR] [--exigir-montagem DIR | --sem-exigir-montagem]
                          [--retencao-diarios N] [--retencao-manuais N]
  sudo rotaperfumes-backup-banco --agora [--aceitar-reducao]
  sudo rotaperfumes-backup-banco --status
  sudo bash $0 --conferir-imagens ARQUIVO.tar.gz

  --instalar            copia para $DESTINO, grava as units e o
                        $CONF, roda o 1o backup pela unit (falha se
                        ele falhar) e liga o timer. Repita depois de trocar o
                        digest da imagem. O volume $VOLUME_IMAGENS
                        precisa existir (rode o job do Jenkins antes).
  --agora               backup na serie -manual, com retencao propria
  --status              ultimo par com ate $IDADE_MAX_HORAS h pelo carimbo e pelo mtime, sem
                        data futura, dump e tar de imagens validos, timer ativo e
                        ultima execucao sem falha; avisa se a ultima verificacao
                        tem mais de $VERIFICACAO_MAX_DIAS dias ou data futura. Com o marcador
                        do dump valido, imprime uma linha fixa de OK
  --conferir-imagens A  confere um tar de imagens e imprime so contagens. Nao
                        usa rede, credencial nem Docker.
  --agendado            serie diaria (quem chama e o timer)

  --backup-dir DIR      destino (padrao $BACKUP_DIR_PADRAO; root, 700); com outro
                        valor, passe --backup-dir tambem ao mariadb-docker.sh
                        e ao seed-producao.sh. Se ja existir fora disso, recusa.
  --exigir-montagem DIR falha se DIR nao for ponto de montagem (destino em outro
                        disco; --backup-dir precisa ficar em DIR). Fica no conf.
  --sem-exigir-montagem tira essa exigencia do conf (destino de volta ao disco
                        do sistema)
  --retencao-diarios N  pares diarios mantidos (padrao $RETENCAO_DIARIOS_PADRAO; minimo $RETENCAO_MIN)
  --retencao-manuais N  pares manuais mantidos (padrao $RETENCAO_MANUAIS_PADRAO; minimo $RETENCAO_MIN)
  --aceitar-reducao     aceita UMA vez um dump com menos de 50% do anterior
                        (fica registrado em <dir>/.registro-reducao)
  -h, --help            esta ajuda

Codigos de saida: 0 sucesso, 2 uso incorreto, 1 qualquer outra falha.
EOF
}

ETAPA="inicio"
uso_errado() { printf 'ERRO: %s\n' "$*" >&2; usage >&2; exit 2; }
die() { printf 'ERRO: etapa=%s: %s\n' "$ETAPA" "$*" >&2; exit 1; }
log() { printf 'backup: %s\n' "$*"; }
aviso() { printf 'AVISO: %s\n' "$*" >&2; }

retencao_valida() { [[ "$1" =~ ^[0-9]{1,4}$ ]] && (( 10#$1 >= RETENCAO_MIN )); }
caminho_valido() { [[ "$1" =~ ^/[A-Za-z0-9._/-]+$ && "$1" != *..* ]]; }
montagem_valida() { caminho_valido "$1" && [[ "$1" != */ ]]; }

MODO=""
ACEITAR_REDUCAO=false
OPC_DIR=""
OPC_DIARIOS=""
OPC_MANUAIS=""
OPC_MONTAGEM=""
OPC_SEM_MONTAGEM=false
ARQ_CONFERIR=""

definir_modo() { [[ -z "$MODO" ]] || uso_errado "informe um unico modo"; MODO=$1; }
exigir_valor() { [[ $# -ge 2 && -n "$2" ]] || uso_errado "a opcao $1 exige um valor"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --instalar) definir_modo instalar; shift ;;
    --agora) definir_modo agora; shift ;;
    --agendado) definir_modo agendado; shift ;;
    --status) definir_modo status; shift ;;
    --conferir-imagens) exigir_valor "$@"; definir_modo conferir; ARQ_CONFERIR=$2; shift 2 ;;
    --aceitar-reducao) ACEITAR_REDUCAO=true; shift ;;
    --backup-dir) exigir_valor "$@"; OPC_DIR=$2; shift 2 ;;
    --exigir-montagem) exigir_valor "$@"; OPC_MONTAGEM=$2; shift 2 ;;
    --sem-exigir-montagem) OPC_SEM_MONTAGEM=true; shift ;;
    --retencao-diarios) exigir_valor "$@"; OPC_DIARIOS=$2; shift 2 ;;
    --retencao-manuais) exigir_valor "$@"; OPC_MANUAIS=$2; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) uso_errado "opcao desconhecida: $1" ;;
  esac
done
[[ -n "$MODO" ]] || uso_errado "informe --instalar, --agora, --status ou --conferir-imagens"
if $ACEITAR_REDUCAO && [[ "$MODO" != agora ]]; then
  uso_errado "--aceitar-reducao so com --agora"
fi
if [[ "$MODO" != instalar ]] && { [[ -n "$OPC_DIR$OPC_DIARIOS$OPC_MANUAIS$OPC_MONTAGEM" ]] || $OPC_SEM_MONTAGEM; }; then
  uso_errado "--backup-dir, --exigir-montagem, --sem-exigir-montagem e --retencao-* so com --instalar"
fi
if [[ -n "$OPC_MONTAGEM" ]] && $OPC_SEM_MONTAGEM; then
  uso_errado "use --exigir-montagem ou --sem-exigir-montagem, nao os dois"
fi
if [[ -n "$OPC_MONTAGEM" ]] && ! montagem_valida "$OPC_MONTAGEM"; then
  uso_errado "--exigir-montagem invalido: $OPC_MONTAGEM (caminho absoluto, sem espacos, virgulas, .. nem barra final; nao pode ser /)"
fi
if [[ -n "$OPC_DIR" ]] && ! caminho_valido "$OPC_DIR"; then
  uso_errado "--backup-dir invalido: $OPC_DIR (caminho absoluto, sem espacos, virgulas ou ..)"
fi
for v in "$OPC_DIARIOS" "$OPC_MANUAIS"; do
  if [[ -n "$v" ]] && ! retencao_valida "$v"; then
    uso_errado "retencao invalida: $v (numero inteiro, minimo $RETENCAO_MIN)"
  fi
done

# ---------------------------------------------------------------------------
# Tar de imagens. So contagens: nomes de arquivo nunca vao para a saida.
# GNU tar -t escapa quebra de linha no nome, entao cada entrada e uma linha.
# ---------------------------------------------------------------------------
IMG_ARQUIVOS=0
IMG_BYTES=0
ERRO_TAR=""
conferir_tar_imagens() { # arquivo -> define IMG_ARQUIVOS, IMG_BYTES, ERRO_TAR
  local arq=$1 nomes tipos entradas perigosos fora
  gzip -t -- "$arq" 2>/dev/null || { ERRO_TAR="gzip invalido ou truncado"; return 1; }
  nomes=$(tar -tzf "$arq" 2>/dev/null | awk '
    { n++ }
    /^\// || /(^|\/)\.\.(\/|$)/ { p++ }
    END { printf "%d %d\n", n, p }') || { ERRO_TAR="listagem do tar falhou"; return 1; }
  read -r entradas perigosos <<<"$nomes"
  # tar -t aceita lixo curto como arquivo vazio; o nosso sempre tem a entrada "./".
  (( entradas > 0 )) || { ERRO_TAR="tar sem nenhuma entrada"; return 1; }
  (( perigosos == 0 )) || { ERRO_TAR="$perigosos entrada(s) com caminho absoluto ou '..'"; return 1; }
  tipos=$(tar -tvzf "$arq" --numeric-owner 2>/dev/null | awk '
    { t = substr($0, 1, 1) }
    t == "-" { a++; b += $3; next }
    t == "d" { next }
    { f++ }
    END { printf "%d %.0f %d\n", a, b, f }') || { ERRO_TAR="listagem do tar falhou"; return 1; }
  read -r IMG_ARQUIVOS IMG_BYTES fora <<<"$tipos"
  (( fora == 0 )) || { ERRO_TAR="$fora entrada(s) que nao sao arquivo nem diretorio (link, dispositivo)"; return 1; }
}

if [[ "$MODO" == conferir ]]; then
  ETAPA="conferir-imagens"
  [[ ! -L "$ARQ_CONFERIR" && -f "$ARQ_CONFERIR" ]] || die "nao e um arquivo regular ou nao existe"
  [[ -r "$ARQ_CONFERIR" ]] || die "sem permissao de leitura (backups sao do root; use sudo)"
  conferir_tar_imagens "$ARQ_CONFERIR" || die "$ERRO_TAR"
  printf 'Arquivos: %s\nBytes: %s\nAPROVADO\n' "$IMG_ARQUIVOS" "$IMG_BYTES"
  exit 0
fi

ETAPA="pre-requisitos"
[[ $EUID -eq 0 ]] || die "rode como root"
for cmd in docker gzip tar flock df stat realpath mktemp sync awk ln dirname date; do
  command -v "$cmd" >/dev/null || die "comando '$cmd' nao encontrado"
done
[[ "$IMAGEM_BASE" =~ ^mariadb:[0-9.]+@sha256:[0-9a-f]{64}$ ]] || die "IMAGEM_BASE invalida"

ORIGEM=$(realpath -- "$0")
if [[ "$MODO" == instalar ]]; then
  [[ "$ORIGEM" != "$DESTINO" ]] || die "--instalar roda do checkout: sudo bash deploy/backup-banco.sh --instalar"
else
  [[ "$ORIGEM" == "$DESTINO" ]] \
    || die "o root nao executa do checkout: use sudo rotaperfumes-backup-banco --$MODO (instale com sudo bash deploy/backup-banco.sh --instalar)"
  [[ ! -L "$0" && "$(stat -c '%u %a' -- "$DESTINO")" == "0 700" ]] || die "$DESTINO precisa ser do root com modo 700"
fi

# Sem "source": cada linha do conf e conferida, nada e executado.
BACKUP_DIR=$BACKUP_DIR_PADRAO
RETENCAO_DIARIOS=$RETENCAO_DIARIOS_PADRAO
RETENCAO_MANUAIS=$RETENCAO_MANUAIS_PADRAO
EXIGIR_MONTAGEM=""
if [[ -e "$CONF" || -L "$CONF" ]]; then
  [[ -f "$CONF" && ! -L "$CONF" ]] || die "$CONF precisa ser arquivo regular"
  read -r _dono _modo < <(stat -c '%u %a' -- "$CONF")
  if [[ "$_dono" != 0 ]] || (( (8#$_modo & 8#022) != 0 )); then
    die "$CONF precisa ser do root sem escrita para grupo/outros"
  fi
  while IFS= read -r linha || [[ -n "$linha" ]]; do
    case "$linha" in
      BACKUP_DIR=*) BACKUP_DIR=${linha#*=}; caminho_valido "$BACKUP_DIR" || die "$CONF: BACKUP_DIR invalido" ;;
      RETENCAO_DIARIOS=*) RETENCAO_DIARIOS=${linha#*=}; retencao_valida "$RETENCAO_DIARIOS" || die "$CONF: RETENCAO_DIARIOS invalida (minimo $RETENCAO_MIN)" ;;
      RETENCAO_MANUAIS=*) RETENCAO_MANUAIS=${linha#*=}; retencao_valida "$RETENCAO_MANUAIS" || die "$CONF: RETENCAO_MANUAIS invalida (minimo $RETENCAO_MIN)" ;;
      EXIGIR_MONTAGEM=*) EXIGIR_MONTAGEM=${linha#*=}; montagem_valida "$EXIGIR_MONTAGEM" || die "$CONF: EXIGIR_MONTAGEM invalido" ;;
      '') ;;
      *) die "$CONF: linha nao reconhecida" ;;
    esac
  done <"$CONF"
fi
BACKUP_DIR=${OPC_DIR:-$BACKUP_DIR}
RETENCAO_DIARIOS=${OPC_DIARIOS:-$RETENCAO_DIARIOS}
RETENCAO_MANUAIS=${OPC_MANUAIS:-$RETENCAO_MANUAIS}
EXIGIR_MONTAGEM=${OPC_MONTAGEM:-$EXIGIR_MONTAGEM}
if $OPC_SEM_MONTAGEM; then EXIGIR_MONTAGEM=""; fi
# O zero a esquerda passa na validacao (10#), mas na conta da retencao "010" seria
# octal e "08" daria erro de aritmetica depois de publicar.
RETENCAO_DIARIOS=$((10#$RETENCAO_DIARIOS))
RETENCAO_MANUAIS=$((10#$RETENCAO_MANUAIS))
readonly BACKUP_DIR RETENCAO_DIARIOS RETENCAO_MANUAIS EXIGIR_MONTAGEM
readonly TRAVA="$BACKUP_DIR/.trava"
readonly REGISTRO_VERIFICACAO="$BACKUP_DIR/.ultima-verificacao"
readonly REGISTRO_REDUCAO="$BACKUP_DIR/.registro-reducao"

# Copia byte a byte do conferir_dir_privado do deploy/mariadb-docker.sh
# (um teste confere a paridade).
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

dentro_de() { [[ "$1" == "$2" || "$1" == "$2"/* ]]; }

conferir_disjunto() {
  local real outro r
  real=$(realpath -m -- "$BACKUP_DIR") || die "--backup-dir $BACKUP_DIR: realpath falhou"
  for outro in "$DADOS_DIR" "$SSL_DIR" "$SEGREDOS_DIR" "$CA_DIR"; do
    r=$(realpath -m -- "$outro") || die "$outro: realpath falhou"
    if dentro_de "$real" "$r" || dentro_de "$r" "$real"; then
      die "--backup-dir $BACKUP_DIR precisa ser disjunto de $outro"
    fi
  done
}

# Sem isto, com o disco desmontado o caminho cairia no disco do sistema.
conferir_montagem() {
  [[ -n "$EXIGIR_MONTAGEM" ]] || return 0
  dentro_de "$BACKUP_DIR" "$EXIGIR_MONTAGEM" || die "--backup-dir $BACKUP_DIR precisa ficar dentro de --exigir-montagem $EXIGIR_MONTAGEM"
  command -v mountpoint >/dev/null || die "comando 'mountpoint' nao encontrado"
  mountpoint -q -- "$EXIGIR_MONTAGEM" || die "$EXIGIR_MONTAGEM nao esta montado: nada foi gravado. Abra e monte o disco e rode de novo"
}

conferir_destino() {
  conferir_disjunto
  conferir_montagem
  [[ -d "$BACKUP_DIR" ]] || die "$BACKUP_DIR nao existe: rode sudo bash deploy/backup-banco.sh --instalar"
  conferir_dir_privado --backup-dir "$BACKUP_DIR" existe
}

conferir_credenciais() {
  local dono modo
  conferir_dir_privado --segredos-dir "$SEGREDOS_DIR" existe
  conferir_dir_privado --segredos-dir "$CNF_DIR" existe
  [[ -f "$CNF" && ! -L "$CNF" ]] || die "$CNF nao encontrado: rode deploy/mariadb-docker.sh (sem --recriar) antes"
  [[ "$(stat -c '%u %a' -- "$CNF")" == "0 400" ]] || die "$CNF precisa ser do root com modo 400"
  [[ -f "$CA_HOST" && ! -L "$CA_HOST" && -s "$CA_HOST" ]] || die "CA do banco nao encontrada em $CA_HOST"
  read -r dono modo < <(stat -c '%u %a' -- "$CA_HOST")
  if [[ "$dono" != 0 ]] || (( (8#$modo & 8#022) != 0 )); then
    die "CA $CA_HOST precisa ser do root sem escrita para grupo/outros"
  fi
}

# ---------------------------------------------------------------------------
# Backups no diretorio. Glob em LC_ALL=C: ordem do carimbo, mais novo por ultimo.
# Bom = dump e imagens regulares, com o mesmo carimbo e a mesma serie. Symlink,
# subdiretorio e nome parecido nunca entram na lista e nunca sao apagados.
# ---------------------------------------------------------------------------
arquivo_real() { [[ -f "$1" && ! -L "$1" ]]; }

par_imagens() { # caminho do dump -> caminho das imagens
  local nome
  nome=${1##*/}
  nome=${nome#rotaperfumes-}
  printf '%s/rotaperfumes-imagens-%s.tar.gz' "${1%/*}" "${nome%.sql.gz}"
}

listar_pares() { # todos|diario|manual -> caminho do dump de cada par bom
  local serie=$1 f nome
  for f in "$BACKUP_DIR"/rotaperfumes-*.sql.gz; do
    nome=${f##*/}
    [[ "$nome" =~ $RE_DUMP ]] || continue
    arquivo_real "$f" || continue
    arquivo_real "$(par_imagens "$f")" || continue
    case "$serie:$nome" in
      diario:*-manual.sql.gz|manual:*Z.sql.gz) continue ;;
    esac
    printf '%s\n' "$f"
  done
}

ultimo_par() {
  local f ultimo=""
  while IFS= read -r f; do ultimo=$f; done < <(listar_pares todos)
  printf '%s' "$ultimo"
}

contar_incompletos() { # arquivos com nome de backup, mas sem o outro lado do par
  local f nome dump n=0
  for f in "$BACKUP_DIR"/rotaperfumes-*.sql.gz; do
    nome=${f##*/}
    [[ "$nome" =~ $RE_DUMP ]] && arquivo_real "$f" && ! arquivo_real "$(par_imagens "$f")" && n=$((n + 1))
  done
  for f in "$BACKUP_DIR"/rotaperfumes-imagens-*.tar.gz; do
    nome=${f##*/}
    [[ "$nome" =~ $RE_IMAGENS ]] || continue
    arquivo_real "$f" || continue
    nome=${nome#rotaperfumes-imagens-}
    dump="$BACKUP_DIR/rotaperfumes-${nome%.tar.gz}.sql.gz"
    arquivo_real "$dump" || n=$((n + 1))
  done
  printf '%d' "$n"
}

carimbo_epoch() { # nome do arquivo -> epoch do carimbo; falha se a data nao existir
  [[ "$1" =~ ([0-9]{4})([0-9]{2})([0-9]{2})-([0-9]{2})([0-9]{2})([0-9]{2})Z ]] || return 1
  date -u -d "${BASH_REMATCH[1]}-${BASH_REMATCH[2]}-${BASH_REMATCH[3]} ${BASH_REMATCH[4]}:${BASH_REMATCH[5]}:${BASH_REMATCH[6]}" +%s 2>/dev/null
}

# A retencao ordena pelo carimbo: um par "do futuro" ficaria sempre entre os mais
# novos e expulsaria o par recem-publicado. Vale carimbo e mtime, como na guarda.
conferir_datas_dos_pares() {
  local f nome carimbo agora
  agora=$(date +%s)
  while IFS= read -r f; do
    nome=${f##*/}
    carimbo=$(carimbo_epoch "$nome") || die "o par $nome tem carimbo invalido: nada foi apagado e o par novo ficou intacto. Confira esse arquivo"
    if (( carimbo > agora + TOLERANCIA_RELOGIO_S || $(stat -c %Y -- "$f") > agora + TOLERANCIA_RELOGIO_S )); then
      die "o par $nome tem data futura: nada foi apagado e o par novo ficou intacto. Confira o relogio e esse arquivo"
    fi
  done < <(listar_pares todos)
}

ERRO_DUMP=""
conferir_dump_publicado() { # dump -> define ERRO_DUMP. tail le o stream inteiro: sem SIGPIPE.
  local ultima
  gzip -t -- "$1" 2>/dev/null || { ERRO_DUMP="gzip invalido ou truncado"; return 1; }
  ultima=$(gzip -dc -- "$1" | tail -n 1) || { ERRO_DUMP="leitura falhou"; return 1; }
  [[ "$ultima" == "-- Dump completed"* ]] || { ERRO_DUMP="ultima linha sem o marcador '-- Dump completed'"; return 1; }
}

conferir_par_publicado() { # dump -> define ERRO_DUMP
  conferir_dump_publicado "$1" || return 1
  gzip -t -- "$(par_imagens "$1")" 2>/dev/null || { ERRO_DUMP="imagens com gzip invalido ou truncado"; return 1; }
}

# Antes de apagar, rele o par novo, todos os pares que ficam na serie e os dois
# mais novos de todas (BACKUP-05): um par mantido corrompido faria a retencao
# apagar um par bom mais antigo. Os que saem nao sao relidos; sem nada a apagar,
# nada e relido.
aplicar_retencao() { # serie manter dump-recem-publicado
  local serie=$1 manter=$2 novo=$3 pares=() todos=() alvos=() i n alvo
  conferir_datas_dos_pares
  mapfile -t pares < <(listar_pares "$serie")
  n=$(( ${#pares[@]} - manter ))
  (( n > 0 )) || return 0
  for (( i = 0; i < n; i++ )); do
    [[ "${pares[i]}" != "$novo" ]] || die "a retencao apagaria o par recem-publicado ${novo##*/}: nada foi apagado. Confira os pares com carimbo mais novo"
  done
  mapfile -t todos < <(listar_pares todos)
  alvos=("$novo")
  for (( i = n; i < ${#pares[@]}; i++ )); do
    [[ " ${alvos[*]} " == *" ${pares[i]} "* ]] || alvos+=("${pares[i]}")
  done
  for (( i = ${#todos[@]} - 1; i >= 0 && i >= ${#todos[@]} - 2; i-- )); do
    [[ " ${alvos[*]} " == *" ${todos[i]} "* ]] || alvos+=("${todos[i]}")
  done
  for alvo in "${alvos[@]}"; do
    conferir_par_publicado "$alvo" || die "o par ${alvo##*/} foi reprovado ($ERRO_DUMP): nada foi apagado e o par novo ficou intacto. Confira esse arquivo; se estiver corrompido, mova o par (dump e imagens) para fora de $BACKUP_DIR"
  done
  for (( i = 0; i < n; i++ )); do
    rm -f -- "$(par_imagens "${pares[i]}")" "${pares[i]}"
    log "retencao: par removido ${pares[i]##*/}"
  done
}

# ---------------------------------------------------------------------------
# Volume de imagens
# ---------------------------------------------------------------------------
ERRO_VOLUME=""
conferir_volume_imagens() { # define ERRO_VOLUME
  local rotulo
  if ! rotulo=$(docker volume inspect -f '{{index .Labels "com.docker.compose.project"}}/{{index .Labels "com.docker.compose.volume"}}' "$VOLUME_IMAGENS" 2>/dev/null); then
    ERRO_VOLUME="volume $VOLUME_IMAGENS nao existe (ele nasce no 1o deploy: rode o job do Jenkins)"
    return 1
  fi
  if [[ "$rotulo" != "$VOLUME_ROTULO" ]]; then
    ERRO_VOLUME="volume $VOLUME_IMAGENS nao foi criado pelo compose do projeto (labels: $rotulo; esperado $VOLUME_ROTULO)"
    return 1
  fi
}

CLIENTE_IMAGENS_ARGS=(
  --rm --name "$CLIENTE_IMAGENS"
  --network none --read-only --cap-drop ALL
  --security-opt no-new-privileges:true
  --user "$USUARIO_IMAGENS"
  --mount "type=volume,source=$VOLUME_IMAGENS,target=$C_IMAGENS,readonly"
)

medir_volume() { # bytes usados; 0 se ausente ou sem resposta (a falha real vem na etapa das imagens)
  local b
  if conferir_volume_imagens; then
    b=$(docker run "${CLIENTE_IMAGENS_ARGS[@]}" --entrypoint du "$IMAGEM_BASE" -sb "$C_IMAGENS" 2>/dev/null | awk 'NR == 1 { print $1 }') || b=""
    [[ "$b" =~ ^[0-9]+$ ]] && { printf '%s' "$b"; return 0; }
  fi
  printf '0'
}

conferir_espaco() {
  local livre ultimo dump_ant=0 img_ant=0 volume img_prev necessario
  ultimo=$(ultimo_par)
  if [[ -n "$ultimo" ]]; then
    dump_ant=$(stat -c %s -- "$ultimo")
    img_ant=$(stat -c %s -- "$(par_imagens "$ultimo")")
  fi
  volume=$(medir_volume)
  # A 1a execucao nao tem par anterior; o tamanho do volume cobre esse caso, ja
  # que imagens quase nao comprimem.
  img_prev=$(( 2 * img_ant ))
  (( volume > img_prev )) && img_prev=$volume
  necessario=$(( 2 * dump_ant + img_prev + RESERVA_BYTES ))
  livre=$(df --output=avail -B1 -- "$BACKUP_DIR" | awk 'END { print $1 }')
  [[ "$livre" =~ ^[0-9]+$ ]] || die "nao consegui ler o espaco livre em $BACKUP_DIR"
  (( livre >= necessario )) || die "espaco insuficiente em $BACKUP_DIR: livre $livre, necessario $necessario bytes"
}

# ---------------------------------------------------------------------------
# Limpeza. O trap so entra depois da trava: quem nao a obteve nao pode remover
# o cliente de uma execucao em andamento.
# ---------------------------------------------------------------------------
TMP_DUMP=""
TMP_IMAGENS=""
TMP_ERR=""
PID_STREAM=""

limpar() {
  local rc=$?
  trap - EXIT TERM INT
  docker rm -f -- "$CLIENTE" "$CLIENTE_IMAGENS" >/dev/null 2>&1 || true
  if [[ -n "$PID_STREAM" ]]; then
    pkill -TERM -P "$PID_STREAM" 2>/dev/null || true
    kill "$PID_STREAM" 2>/dev/null || true
    wait "$PID_STREAM" 2>/dev/null || true
  fi
  rm -f -- "$TMP_DUMP" "$TMP_IMAGENS" "$TMP_ERR"
  exit "$rc"
}

adquirir_trava() {
  [[ ! -L "$TRAVA" ]] || die "$TRAVA e um link simbolico"
  exec 9>>"$TRAVA"
  flock -n 9 || die "outro backup em andamento (trava $TRAVA ocupada)"
  trap limpar EXIT
  trap 'exit 143' TERM
  trap 'exit 130' INT
}

# Em segundo plano + wait: um SIGTERM e tratado na hora, nao ao fim do pipe.
rodar_stream() { # saida erros args-do-docker-run...
  local saida=$1 erros=$2 rc=0
  shift 2
  (
    set +e
    exec 9>&-
    docker run "$@" </dev/null 2>"$erros" | gzip -c >"$saida"
    st=("${PIPESTATUS[@]}")
    [[ "${st[0]}" == 0 && "${st[1]}" == 0 ]] && exit 0
    printf 'codigos do pipe: docker=%s gzip=%s\n' "${st[0]}" "${st[1]}" >&2
    exit 1
  ) &
  PID_STREAM=$!
  wait "$PID_STREAM" || rc=$?
  PID_STREAM=""
  return "$rc"
}

# So o codigo do erro: a mensagem do cliente pode citar nomes e dados.
resumir_erro() { # arquivo
  awk 'match($0, /[Ee]rror:? [0-9]+/) { print substr($0, RSTART, RLENGTH); exit }
       /^docker: / { print substr($0, 1, 200); exit }' "$1"
}

# ---------------------------------------------------------------------------
# Dump
# ---------------------------------------------------------------------------
gerar_dump() {
  local args=(
    --rm -i --name "$CLIENTE"
    --network "$REDE_NOME" --ip "$IP_BACKUP"
    --read-only --cap-drop ALL
    --security-opt no-new-privileges:true
    --mount "type=bind,source=$CA_HOST,target=$C_CA,readonly"
    --mount "type=bind,source=$CNF,target=$C_CNF,readonly"
    --entrypoint mariadb-dump "$IMAGEM_BASE"
    "--defaults-extra-file=$C_CNF"
    -h "$DB_HOST" -P 3306
    "--ssl-ca=$C_CA" --ssl-verify-server-cert
    --single-transaction --skip-lock-tables --no-tablespaces --skip-add-drop-table --skip-triggers
    --hex-blob --default-character-set=utf8mb4 "$DB_NOME"
  )
  rodar_stream "$TMP_DUMP" "$TMP_ERR" "${args[@]}" \
    || die "o dump falhou ($(resumir_erro "$TMP_ERR"))"
}

# awk le o stream inteiro: sem falso negativo por SIGPIPE sob pipefail, como
# daria com grep -q. O marcador so vale na ULTIMA linha: esse texto pode estar
# em um dado de cliente.
conferir_dump() {
  local resumo tabelas usuarios clientes vendedores dados bytes marcador
  gzip -t -- "$TMP_DUMP" 2>/dev/null || die "gzip -t reprovou o dump"
  resumo=$(gzip -dc -- "$TMP_DUMP" | awk '
    /^CREATE TABLE / { n++ }
    /^CREATE TABLE `usuarios` \(/ { u = 1 }
    /^CREATE TABLE `clientes` \(/ { c = 1 }
    /^CREATE TABLE `vendedores` \(/ { v = 1 }
    /^INSERT INTO `usuarios` VALUES$/ { d = 1 }
    { ultima = $0; b += length($0) + 1 }
    END { printf "%d %d %d %d %d %.0f %d\n", n, u, c, v, d, b, index(ultima, "-- Dump completed") == 1 }') \
    || die "nao consegui ler o dump"
  read -r tabelas usuarios clientes vendedores dados bytes marcador <<<"$resumo"
  [[ "$marcador" == 1 ]] || die "a ultima linha do dump nao tem o marcador '-- Dump completed'"
  (( tabelas >= TABELAS_MIN )) || die "o dump tem $tabelas tabela(s); o minimo e $TABELAS_MIN"
  [[ "$usuarios$clientes$vendedores" == 111 ]] || die "o dump nao tem usuarios, clientes e vendedores"
  [[ "$dados" == 1 ]] || die "a tabela usuarios esta sem dados no dump"
  (( bytes >= PISO_DUMP_BYTES )) || die "dump descomprimido com $bytes bytes; o piso e $PISO_DUMP_BYTES"
}

conferir_queda_dump() { # compara o dump comprimido com o do ultimo par
  local ultimo atual anterior
  ultimo=$(ultimo_par)
  [[ -n "$ultimo" ]] || return 0
  atual=$(stat -c %s -- "$TMP_DUMP")
  anterior=$(stat -c %s -- "$ultimo")
  (( atual * 2 >= anterior )) && return 0
  $ACEITAR_REDUCAO || die "dump com $atual bytes, menos de 50% do anterior ($anterior): confira o banco; se a queda for legitima, use --agora --aceitar-reducao"
  printf '%s atual=%s anterior=%s referencia=%s\n' "$(date -Is)" "$atual" "$anterior" "${ultimo##*/}" >>"$REGISTRO_REDUCAO"
  chmod 600 "$REGISTRO_REDUCAO"
  aviso "--aceitar-reducao: dump com $atual bytes, anterior com $anterior (registrado em $REGISTRO_REDUCAO)"
}

# ---------------------------------------------------------------------------
# Imagens: funcoes que devolvem erro em vez de encerrar, para o chamador guardar
# o dump ja validado.
# ---------------------------------------------------------------------------
gerar_imagens() { # define ERRO_IMAGENS
  conferir_volume_imagens || { ERRO_IMAGENS=$ERRO_VOLUME; return 1; }
  if ! rodar_stream "$TMP_IMAGENS" "$TMP_ERR" "${CLIENTE_IMAGENS_ARGS[@]}" \
    --entrypoint tar "$IMAGEM_BASE" -C "$C_IMAGENS" --numeric-owner -cf - .; then
    ERRO_IMAGENS="o tar do volume falhou ($(resumir_erro "$TMP_ERR"))"
    return 1
  fi
  conferir_tar_imagens "$TMP_IMAGENS" || { ERRO_IMAGENS=$ERRO_TAR; return 1; }
}

fixar_arquivo() { # temporario
  chmod 600 -- "$1"
  [[ "$(stat -c '%u %a' -- "$1")" == "0 600" ]] || die "$1 precisa ser do root com modo 600"
  sync -- "$1"
}

# ln recusa destino existente de forma atomica, o que "mv -n" nao garante em
# todas as versoes do coreutils.
publicar() { # temporario final
  ln -T -- "$1" "$2" || die "nao consegui publicar $2 (ja existe?)"
  rm -f -- "$1"
}

guardar_dump_parcial() { # nome-do-dump
  local parciais=() i f
  ETAPA="parcial"
  fixar_arquivo "$TMP_DUMP"
  publicar "$TMP_DUMP" "$BACKUP_DIR/parcial-$1"
  TMP_DUMP=""
  sync -- "$BACKUP_DIR"
  for f in "$BACKUP_DIR"/parcial-rotaperfumes-*.sql.gz; do
    [[ "${f##*/}" =~ $RE_PARCIAL ]] && arquivo_real "$f" && parciais+=("$f")
  done
  for (( i = 0; i < ${#parciais[@]} - PARCIAIS_MANTER; i++ )); do
    rm -f -- "${parciais[i]}"
  done
  log "dump guardado como parcial-$1 (nao conta como backup bom)"
}

# ---------------------------------------------------------------------------
# Backup
# ---------------------------------------------------------------------------
executar_backup() { # diario|manual
  local serie=$1 sufixo="" inicio=$SECONDS carimbo nome_dump nome_imagens final_dump final_imagens f tam_dump tam_imagens
  [[ "$serie" == manual ]] && sufixo="-manual"

  ETAPA="destino"
  conferir_destino
  conferir_credenciais

  ETAPA="trava"
  adquirir_trava

  ETAPA="limpeza"
  docker rm -f -- "$CLIENTE" "$CLIENTE_IMAGENS" >/dev/null 2>&1 || true
  # Com a trava na mao, um .tmp-* so pode ser sobra de execucao morta (SIGKILL).
  for f in "$BACKUP_DIR"/.tmp-*; do
    if arquivo_real "$f"; then rm -f -- "$f"; fi
  done

  ETAPA="espaco"
  conferir_espaco

  carimbo=$(date -u +%Y%m%d-%H%M%SZ)
  nome_dump="rotaperfumes-$carimbo$sufixo.sql.gz"
  nome_imagens="rotaperfumes-imagens-$carimbo$sufixo.tar.gz"
  final_dump="$BACKUP_DIR/$nome_dump"
  final_imagens="$BACKUP_DIR/$nome_imagens"
  TMP_DUMP=$(mktemp "$BACKUP_DIR/.tmp-dump.XXXXXX")
  TMP_IMAGENS=$(mktemp "$BACKUP_DIR/.tmp-imagens.XXXXXX")
  TMP_ERR=$(mktemp "$BACKUP_DIR/.tmp-erros.XXXXXX")

  ETAPA="dump"
  gerar_dump
  ETAPA="conteudo-dump"
  conferir_dump
  log "dump validado: marcador '-- Dump completed' na ultima linha: OK"
  ETAPA="tamanho-dump"
  conferir_queda_dump

  ETAPA="imagens"
  ERRO_IMAGENS=""
  if ! gerar_imagens; then
    guardar_dump_parcial "$nome_dump"
    ETAPA="imagens"
    die "$ERRO_IMAGENS. Backup NAO concluido e retencao nao executada"
  fi
  if (( IMG_ARQUIVOS == 0 )); then
    aviso "o volume $VOLUME_IMAGENS esta vazio: 0 arquivos no backup. Confira se o site ainda nao tem imagens de produto"
  fi

  ETAPA="publicacao"
  conferir_montagem
  tam_dump=$(stat -c %s -- "$TMP_DUMP")
  tam_imagens=$(stat -c %s -- "$TMP_IMAGENS")
  fixar_arquivo "$TMP_IMAGENS"
  fixar_arquivo "$TMP_DUMP"
  publicar "$TMP_IMAGENS" "$final_imagens"
  TMP_IMAGENS=""
  if ! ln -T -- "$TMP_DUMP" "$final_dump"; then
    rm -f -- "$final_imagens"
    die "nao consegui publicar $final_dump"
  fi
  rm -f -- "$TMP_DUMP"
  TMP_DUMP=""
  sync -- "$BACKUP_DIR"

  ETAPA="retencao"
  if [[ "$serie" == manual ]]; then
    aplicar_retencao manual "$RETENCAO_MANUAIS" "$final_dump"
  else
    aplicar_retencao diario "$RETENCAO_DIARIOS" "$final_dump"
  fi

  log "ok dump=$final_dump tamanho_dump=$tam_dump imagens=$final_imagens tamanho_imagens=$tam_imagens arquivos_imagens=$IMG_ARQUIVOS bytes_imagens=$IMG_BYTES duracao_s=$((SECONDS - inicio))"
  # D2 (aguarda o usuario): o aviso externo de sucesso entra aqui, depois da
  # retencao, e a falha dele nao pode derrubar o backup.
}

# ---------------------------------------------------------------------------
# --status
# ---------------------------------------------------------------------------
mostrar_status() {
  local problemas=0 agora ultimo nome mtime carimbo idade resultado linha epoch="" dias incompletos

  problema() { printf 'PROBLEMA: %s\n' "$*"; problemas=$((problemas + 1)); }

  ETAPA="status"
  command -v systemctl >/dev/null || die "comando 'systemctl' nao encontrado"
  conferir_destino
  agora=$(date +%s)

  ultimo=$(ultimo_par)
  if [[ -z "$ultimo" ]]; then
    problema "nenhum backup bom (dump + imagens) em $BACKUP_DIR"
  else
    nome=${ultimo##*/}
    mtime=$(stat -c %Y -- "$ultimo")
    carimbo=$(carimbo_epoch "$nome") || { problema "o ultimo backup ($nome) tem carimbo invalido"; carimbo=$mtime; }
    if (( carimbo > agora + TOLERANCIA_RELOGIO_S || mtime > agora + TOLERANCIA_RELOGIO_S )); then
      printf 'Ultimo backup: %s (%s bytes)\n' "$nome" "$(stat -c %s -- "$ultimo")"
      problema "o ultimo backup ($nome) tem data futura"
    else
      # O mais antigo dos dois: um "touch" nao rejuvenesce o backup.
      idade=$(( agora - (carimbo < mtime ? carimbo : mtime) ))
      (( idade >= 0 )) || idade=0
      printf 'Ultimo backup: %s (%d min atras, %s bytes)\n' "$nome" "$(( idade / 60 ))" "$(stat -c %s -- "$ultimo")"
      (( idade <= IDADE_MAX_HORAS * 3600 )) || problema "o ultimo backup ($nome) tem mais de $IDADE_MAX_HORAS h"
    fi
    if conferir_dump_publicado "$ultimo"; then
      printf '%s\n' "Dump validado: marcador '-- Dump completed' na ultima linha: OK"
    else
      problema "o dump do ultimo backup ($nome) foi reprovado: $ERRO_DUMP"
    fi
    if conferir_tar_imagens "$(par_imagens "$ultimo")"; then
      printf 'Imagens: %s arquivo(s), %s bytes\n' "$IMG_ARQUIVOS" "$IMG_BYTES"
    else
      problema "o tar de imagens do ultimo backup foi reprovado: $ERRO_TAR"
    fi
  fi

  systemctl is-active --quiet "$TIMER" || problema "o timer $TIMER nao esta ativo"
  resultado=$(systemctl show -p Result --value "$SERVICE" 2>/dev/null || true)
  [[ "$resultado" == success ]] || problema "a ultima execucao de $SERVICE falhou (Result=${resultado:-desconhecido}; journalctl -u $SERVICE)"

  incompletos=$(contar_incompletos)
  (( incompletos == 0 )) || aviso "$incompletos arquivo(s) sem o outro lado do par em $BACKUP_DIR: nao contam como backup bom"

  if [[ -f "$REGISTRO_VERIFICACAO" && ! -L "$REGISTRO_VERIFICACAO" ]]; then
    while IFS= read -r linha; do
      if [[ "$linha" =~ ^epoch=([0-9]+)$ ]]; then epoch=${BASH_REMATCH[1]}; fi
    done <"$REGISTRO_VERIFICACAO"
  fi
  if [[ -z "$epoch" ]]; then
    aviso "nenhuma verificacao registrada: rode sudo bash deploy/verificar-backup.sh <arquivo .sql.gz>"
  else
    dias=$(( (agora - epoch) / 86400 ))
    if (( epoch > agora + TOLERANCIA_RELOGIO_S )); then
      aviso "a ultima verificacao tem data futura: nao vale; rode deploy/verificar-backup.sh"
    else
      (( dias >= 0 )) || dias=0
      printf 'Ultima verificacao: ha %d dia(s)\n' "$dias"
    fi
    (( dias <= VERIFICACAO_MAX_DIAS )) || aviso "a ultima verificacao tem mais de $VERIFICACAO_MAX_DIAS dias: rode deploy/verificar-backup.sh"
  fi

  if (( problemas > 0 )); then
    printf 'STATUS: %d problema(s)\n' "$problemas"
    exit 1
  fi
  printf 'STATUS: ok\n'
}

# ---------------------------------------------------------------------------
# --instalar
# ---------------------------------------------------------------------------
gerar_service() {
  local montagem=""
  [[ -z "$EXIGIR_MONTAGEM" ]] || montagem="RequiresMountsFor=$BACKUP_DIR"$'\n'
  cat <<EOF
# Gerado por deploy/backup-banco.sh (BACKUP-01). Nao edite: --instalar regrava.
[Unit]
Description=RotaPerfumes: backup do banco e das imagens de produto
Requires=docker.service
After=docker.service
${montagem}# 3 tentativas. O intervalo cobre 3 x (TimeoutStartSec + RestartSec).
StartLimitIntervalSec=8h
StartLimitBurst=3

[Service]
Type=oneshot
UMask=0077
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=$BACKUP_DIR
TimeoutStartSec=2h
Restart=on-failure
RestartSec=15min
ExecStart=$DESTINO --agendado
EOF
}

gerar_timer() {
  cat <<EOF
# Gerado por deploy/backup-banco.sh (BACKUP-01). Nao edite: --instalar regrava.
[Unit]
Description=RotaPerfumes: agenda do backup do banco e das imagens de produto

[Timer]
OnCalendar=*-*-* 03:30
Persistent=true
RandomizedDelaySec=15m
Unit=$SERVICE

[Install]
WantedBy=timers.target
EOF
}

gravar_arquivo_atomico() { # destino modo (conteudo pelo stdin)
  cat >"$1.tmp"
  chown root:root "$1.tmp"
  chmod "$2" "$1.tmp"
  mv -f -- "$1.tmp" "$1"
}

# Pela unit, para o sandbox ser testado ja na instalacao. Sem espera bloqueante:
# com Restart=on-failure, "systemctl start" de um oneshot ficaria preso durante
# as novas tentativas. O par novo evita ler como sucesso o estado anterior ao
# inicio da unit.
primeiro_backup_pela_unit() {
  local antes limite resultado estado
  antes=$(ultimo_par)
  systemctl reset-failed "$SERVICE" 2>/dev/null || true
  systemctl start --no-block "$SERVICE" || die "systemctl start $SERVICE falhou"
  limite=$((SECONDS + ESPERA_1O_BACKUP_S))
  while (( SECONDS < limite )); do
    resultado=$(systemctl show -p Result --value "$SERVICE")
    estado=$(systemctl show -p ActiveState --value "$SERVICE")
    if [[ -n "$resultado" && "$resultado" != success ]]; then
      systemctl stop "$SERVICE" 2>/dev/null || true
      die "o 1o backup falhou (Result=$resultado): journalctl -u $SERVICE. Timer nao ligado por esta execucao"
    fi
    if [[ "$estado" == inactive && "$(ultimo_par)" != "$antes" ]]; then
      return 0
    fi
    sleep 3
  done
  systemctl stop "$SERVICE" 2>/dev/null || true
  die "o 1o backup nao terminou em $ESPERA_1O_BACKUP_S s: journalctl -u $SERVICE"
}

instalar() {
  ETAPA="instalar"
  for cmd in systemctl install bash sha256sum; do
    command -v "$cmd" >/dev/null || die "comando '$cmd' nao encontrado"
  done
  docker info >/dev/null 2>&1 || die "o daemon do Docker nao responde"
  bash -n "$ORIGEM" || die "$ORIGEM com erro de sintaxe"

  conferir_disjunto
  conferir_montagem
  conferir_dir_privado --backup-dir "$BACKUP_DIR"
  install -d -m 700 -o root -g root "$BACKUP_DIR"
  conferir_destino
  conferir_credenciais
  conferir_volume_imagens || die "$ERRO_VOLUME"

  install -m 700 -o root -g root -- "$ORIGEM" "$DESTINO.tmp"
  mv -f -- "$DESTINO.tmp" "$DESTINO"

  install -d -m 755 -o root -g root "$(dirname -- "$CONF")"
  {
    printf 'BACKUP_DIR=%s\nRETENCAO_DIARIOS=%s\nRETENCAO_MANUAIS=%s\n' "$BACKUP_DIR" "$RETENCAO_DIARIOS" "$RETENCAO_MANUAIS"
    [[ -z "$EXIGIR_MONTAGEM" ]] || printf 'EXIGIR_MONTAGEM=%s\n' "$EXIGIR_MONTAGEM"
  } | gravar_arquivo_atomico "$CONF" 644
  gerar_service | gravar_arquivo_atomico "$UNIT_DIR/$SERVICE" 644
  gerar_timer | gravar_arquivo_atomico "$UNIT_DIR/$TIMER" 644
  systemctl daemon-reload

  log "instalado $DESTINO (sha256 $(sha256sum -- "$DESTINO" | awk '{print $1}')); rodando o 1o backup pela unit"
  primeiro_backup_pela_unit

  systemctl enable "$TIMER" >/dev/null 2>&1 || die "systemctl enable $TIMER falhou"
  systemctl restart "$TIMER" || die "systemctl restart $TIMER falhou"
  log "instalacao concluida: destino=$BACKUP_DIR exigir_montagem=${EXIGIR_MONTAGEM:-nao} retencao_diarios=$RETENCAO_DIARIOS retencao_manuais=$RETENCAO_MANUAIS"
  log "proximos passos: sudo rotaperfumes-backup-banco --status; sudo bash deploy/verificar-backup.sh $(ultimo_par)"
}

case "$MODO" in
  instalar) instalar ;;
  agora) executar_backup manual ;;
  agendado) executar_backup diario ;;
  status) mostrar_status ;;
esac
