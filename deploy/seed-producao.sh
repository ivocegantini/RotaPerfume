#!/usr/bin/env bash
# =============================================================================
# DEPLOY-07: seed INICIAL do banco de PRODUCAO (container rotaperfumes-mariadb,
# criado pelo deploy/mariadb-docker.sh). Substitui o "make db-seed", que NUNCA
# roda contra este banco (senha conhecida, -all-users, DROP via db-reset).
#
# Rodar como root NO SERVIDOR (192.168.168.106), a partir do checkout do repo:
#   sudo bash deploy/seed-producao.sh                         # pepper pedido sem eco
#   sudo bash deploy/seed-producao.sh --pepper-file /root/pepper.txt
#   sudo bash deploy/seed-producao.sh --help
#
# Etapas:
#   1. confere root, comandos, container/rede, CA, senha do rotaperfumes_admin,
#      arquivos de sql/, pepper e o destino da senha do admin (nada muda antes)
#   2. estado do banco: tabela usuarios com linhas = recusa (fail closed),
#      salvo --forcar, que exige dump recente (BACKUP-01) e digitar o nome do banco
#   3. DDL na ordem de sql/ordem-ddl.txt (a mesma do make db-up)
#   4. sql/02_seed_admin.sql e sql/03_seed_vendedores.sql (INSERT IGNORE) e,
#      depois, todos os usuarios nao-admin INATIVOS (os 42 vendedores sao
#      reais; o admin os ativa pelo fluxo de reset de senha da aplicacao)
#   5. compila o backend/shared/cmd/resetpassword num container GO_IMAGE
#      (sem acesso ao banco nem a segredos)
#   6. roda o resetpassword -create-admin num container GO_IMAGE na rede
#      rotaperfumes-db (IP do admin), com --env-file temporario 600 (shred no
#      fim): senha gerada em arquivo (-password-out) ou digitada
#      (-password-prompt). deve_trocar_senha = 1. NAO roda -all-users: os
#      vendedores ficam com o hash placeholder (ninguem loga com ele).
#   7. confere: tabelas, admin ativo com Argon2id e troca obrigatoria, nenhum
#      nao-admin ativo
#
# Clientes (mesmo padrao do mariadb_cli do Jenkinsfile, estagio "Importar
# dump"): docker run --rm -i --network rotaperfumes-db --ip 172.30.42.20
# --cap-drop ALL, MYSQL_PWD pelo ambiente (-e MYSQL_PWD sem valor), CA do host
# montada read-only, --ssl-verify-server-cert, --local-infile=0.
#
# Imagens fixadas por digest (SEC-31): DB_CLIENT_IMAGE = IMAGEM_BASE do
# mariadb-docker.sh = DB_CLIENT_IMAGE do Jenkinsfile; GO_IMAGE = GO_IMAGE do
# Jenkinsfile (mesmas strings; o teste do TST-15/DEPLOY-07 confere).
#
# Regras: nunca "set -x"; segredos (senha do banco, pepper, senha do admin)
# nunca vao para argv, log ou tela.
# =============================================================================
set -euo pipefail
umask 077

DB_CLIENT_IMAGE="mariadb:11.4.13@sha256:5e73baf8179b29f115101bb102763a47a017361f3e9792daccba6646c3ab1532"
GO_IMAGE="golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280"

readonly DB_NOME="rotaperfumes"
readonly DB_ADMIN="rotaperfumes_admin"
readonly C_CA="/etc/mysql/ssl/ca.pem"
readonly ADMIN_EMAIL="admin@rotaperfumes.com.br"
readonly PEPPER_MIN_BYTES=32

# TST-17 (Lote 37): padroes das opcoes de isolamento (os mesmos do
# deploy/mariadb-docker.sh). Os valores so ficam readonly depois de validados
# na etapa 1.
readonly CONTAINER_PADRAO="rotaperfumes-mariadb"
readonly REDE_NOME_PADRAO="rotaperfumes-db"
readonly IP_ADMIN_PADRAO="172.30.42.20"
readonly SEGREDOS_DIR_PADRAO="/root/rotaperfumes-mariadb"
readonly SSL_DIR_PADRAO="/etc/mysql/ssl"
readonly TRABALHO_DIR_PADRAO="/root"
CONTAINER=$CONTAINER_PADRAO
REDE_NOME=$REDE_NOME_PADRAO
IP_ADMIN=$IP_ADMIN_PADRAO
SEGREDOS_DIR=$SEGREDOS_DIR_PADRAO
SSL_DIR=$SSL_DIR_PADRAO
TRABALHO_DIR=$TRABALHO_DIR_PADRAO

FORCAR=false
BACKUP_DIR="/var/backups/rotaperfumes"
BACKUP_JANELA_MIN=60
readonly BACKUP_JANELA_LIVRE_MAX=60
# BACKUP-03: folga para ajuste de relogio ao julgar "data futura" (igual ao backup-banco.sh).
readonly TOLERANCIA_RELOGIO_S=300
SEM_BACKUP_MOTIVO=""
BACKUP_USADO=""
PEPPER_FILE=""
# Vazio = <segredos-dir>/admin-app.senha (resolvido depois do parse).
ADMIN_SENHA_ARQ=""

RAIZ=$(cd "$(dirname "$0")/.." && pwd -P)
readonly RAIZ
readonly SQL_DIR="$RAIZ/sql"
readonly ORDEM_DDL="$SQL_DIR/ordem-ddl.txt"
readonly SEEDS=(02_seed_admin.sql 03_seed_vendedores.sql)

usage() {
  cat <<EOF
Uso: sudo bash $0 [opcoes]

Seed INICIAL do banco de producao ($DB_NOME no container $CONTAINER_PADRAO,
rede $REDE_NOME_PADRAO). Rode depois do deploy/mariadb-docker.sh e antes do
primeiro deploy da API. Nunca use "make db-seed" contra este banco.

  --pepper-file ARQ     le o PASSWORD_PEPPER da 1a linha de ARQ (arquivo
                        regular do root, modo 600 ou 400). Sem a opcao, o
                        pepper e pedido no terminal, sem eco e com
                        confirmacao. TEM de ser o mesmo PASSWORD_PEPPER do
                        Secret file rotaperfumes-api-env (deploy/api.env), sem
                        aspas; senao o admin nao consegue logar.
  --admin-senha-arquivo ARQ
                        destino da senha do admin, SEMPRE gerada aleatoria
                        e nunca impressa (SEC-52; padrao
                        <segredos-dir>/admin-app.senha). Nao pode existir (o
                        script recusa antes de mudar qualquer coisa) nem ficar
                        no --ssl-dir; o diretorio precisa ser privado (root,
                        700, sem symlink, ancestrais do root sem escrita de
                        grupo/outros: SEC-60). Fica com modo 600, dono root. Depois do
                        1o login (troca obrigatoria), apague com shred -u.

Isolamento (TST-17; os padroes sao os do deploy/mariadb-docker.sh; mude so
para uma instalacao de teste criada por ele com as mesmas opcoes):
  --container NOME      container do MariaDB (padrao $CONTAINER_PADRAO);
                        precisa estar rodando e ligado a --rede-nome
  --rede-nome NOME      rede docker interna (padrao $REDE_NOME_PADRAO)
  --ip-admin IP         IP do cliente na rede (padrao $IP_ADMIN_PADRAO); a rede
                        precisa ter uma unica subnet IPv4 A.B.C.0/24 e o IP
                        tem de ser A.B.C.20 (rotaperfumes_admin@A.B.C.20)
  --segredos-dir DIR    senhas do mariadb-docker.sh (padrao
                        $SEGREDOS_DIR_PADRAO; root, modo 700)
  --ssl-dir DIR         CA em DIR/ca.pem (padrao $SSL_DIR_PADRAO; root, sem
                        escrita para grupo/outros)
  --trabalho-dir DIR    onde fica a area temporaria rotaperfumes-seed.XXXXXX
                        (padrao $TRABALHO_DIR_PADRAO; root, modo 700)
                        Diretorios: caminho absoluto real (sem symlink, '..',
                        espacos ou virgulas), ancestrais do root sem escrita
                        para grupo/outros; --ssl-dir disjunto de
                        --segredos-dir e de --trabalho-dir; --trabalho-dir
                        nao pode ser --segredos-dir nem ficar dentro dele.

  --forcar              aceita rodar com a tabela usuarios ja com linhas
                        (reaplica a DDL e os seeds, troca a senha do admin e
                        INATIVA todos os usuarios nao-admin). Exige digitar o
                        nome do banco ($DB_NOME) no terminal e, com a tabela
                        preenchida, dump recente
                        (sudo rotaperfumes-backup-banco --agora).
  --backup-dir DIR      onde o --forcar procura o dump (padrao $BACKUP_DIR;
                        root, modo 700)
  --backup-janela-min N idade maxima do dump, em minutos (padrao $BACKUP_JANELA_MIN; 1 a 1440).
                        Acima de $BACKUP_JANELA_MIN exige --sem-backup-motivo (justificativa);
                        o dump continua conferido, com a janela informada.
  --sem-backup-motivo TEXTO
                        so com --forcar: dispensa o dump, ou justifica a
                        --backup-janela-min acima de $BACKUP_JANELA_MIN. 10 a 300 caracteres
                        imprimiveis, registrados com janela=N em
                        <segredos-dir>/seed-forcar.registro.
  -h, --help            esta ajuda

O que o script faz (nesta ordem):
  1. confere as opcoes, root, docker, container rodando na rede, CA em
     <ssl-dir>/ca.pem, senha em <segredos-dir>/$DB_ADMIN.senha,
     sql/ordem-ddl.txt e os seeds,
     pepper ($PEPPER_MIN_BYTES bytes ou mais) e o destino da senha do admin
  2. recusa se usuarios ja tem linhas (salvo --forcar)
  3. aplica a DDL na ordem de sql/ordem-ddl.txt (a mesma do make db-up)
  4. aplica sql/02_seed_admin.sql e sql/03_seed_vendedores.sql e roda
     UPDATE usuarios SET ativo = 0 WHERE role <> 'admin'
     (os 42 vendedores sao reais e entram INATIVOS, com hash placeholder;
     o admin os ativa pelo fluxo de reset de senha da aplicacao)
  5. compila o resetpassword (backend/shared) num container $GO_IMAGE
  6. resetpassword -create-admin num container na rede --rede-nome
     (--ip-admin, conta $DB_ADMIN, TLS verificado): admin ativo, Argon2id com
     o pepper e deve_trocar_senha = 1. NAO roda -all-users.
  7. confere o resultado

Clientes do banco: docker run --rm -i --network <rede-nome> --ip <ip-admin>
--cap-drop ALL --security-opt no-new-privileges:true, MYSQL_PWD pelo ambiente,
CA montada read-only, --ssl-verify-server-cert -h <container> --local-infile=0.
O resetpassword recebe DB_* e PASSWORD_PEPPER por --env-file temporario (600,
apagado com shred no fim, inclusive em erro). Docs: docs/deploy-servidor.md §11.
EOF
}

die()   { printf 'ERRO: %s\n' "$*" >&2; exit 1; }
info()  { printf '  %s\n' "$*"; }
aviso() { printf '  AVISO: %s\n' "$*" >&2; }
exigir_valor() { [[ $# -ge 2 && -n "$2" ]] || die "a opcao $1 exige um valor (veja --help)"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --pepper-file) exigir_valor "$@"; PEPPER_FILE="$2"; shift 2 ;;
    --admin-senha-prompt)
      die "a opcao --admin-senha-prompt saiu (SEC-52): a senha inicial do admin e sempre gerada aleatoria (--admin-senha-arquivo)" ;;
    --admin-senha-arquivo) exigir_valor "$@"; ADMIN_SENHA_ARQ="$2"; shift 2 ;;
    --forcar) FORCAR=true; shift ;;
    --backup-dir) exigir_valor "$@"; BACKUP_DIR="$2"; shift 2 ;;
    --backup-janela-min) exigir_valor "$@"; BACKUP_JANELA_MIN="$2"; shift 2 ;;
    --sem-backup-motivo) exigir_valor "$@"; SEM_BACKUP_MOTIVO="$2"; shift 2 ;;
    --container) exigir_valor "$@"; CONTAINER="$2"; shift 2 ;;
    --rede-nome) exigir_valor "$@"; REDE_NOME="$2"; shift 2 ;;
    --ip-admin) exigir_valor "$@"; IP_ADMIN="$2"; shift 2 ;;
    --segredos-dir) exigir_valor "$@"; SEGREDOS_DIR="$2"; shift 2 ;;
    --ssl-dir) exigir_valor "$@"; SSL_DIR="$2"; shift 2 ;;
    --trabalho-dir) exigir_valor "$@"; TRABALHO_DIR="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Opcao desconhecida: $1" >&2; usage >&2; exit 2 ;;
  esac
done

ETAPA=0
readonly TOTAL_ETAPAS=7
etapa() { ETAPA=$((ETAPA + 1)); printf '\n=== [%d/%d] %s ===\n' "$ETAPA" "$TOTAL_ETAPAS" "$1"; }

# Area de trabalho (so root) e limpeza: env-file e senha com shred, sempre.
TRAB=""
ENV_ARQ=""
limpar() {
  [[ -n "$ENV_ARQ" && -f "$ENV_ARQ" ]] && shred -u "$ENV_ARQ" 2>/dev/null
  if [[ -n "$TRAB" && -d "$TRAB" ]]; then
    [[ -f "$TRAB/saida/admin.senha" ]] && shred -u "$TRAB/saida/admin.senha" 2>/dev/null
    rm -rf "$TRAB"
  fi
  return 0
}
trap limpar EXIT

# tem_tty (BUG-26): ha terminal de controle? [[ -r /dev/tty ]] usa access()
# e /dev/tty e 0666, entao passa mesmo sem terminal; so o open() responde
# (ENXIO sem terminal de controle).
tem_tty() { { : </dev/tty; } 2>/dev/null; }

# arquivo_secreto_ok ARQ: regular, dono root, modo 600 ou 400.
arquivo_secreto_ok() {
  [[ -f "$1" && ! -L "$1" ]] || return 1
  [[ "$(stat -c '%u' "$1")" == 0 ]] || return 1
  case "$(stat -c '%a' "$1")" in 600|400) return 0 ;; *) return 1 ;; esac
}

# ---------------------------------------------------------------------------
# TST-17 (Lote 37): validacao das opcoes de isolamento (etapa 1, antes de
# qualquer escrita). Os caminhos entram em --mount "type=bind,source=...":
# virgula injetaria opcoes, por isso o regex de caminho e obrigatorio.
readonly RE_OCTETO='(25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])'
readonly RE_IPV4_CANONICO="^$RE_OCTETO\.$RE_OCTETO\.$RE_OCTETO\.$RE_OCTETO\$"
dentro_de() { [[ "$1" == "$2" || "$1" == "$2"/* ]]; }

validar_sintaxe_opcoes() {
  local d
  [[ "$CONTAINER" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$ && ${#CONTAINER} -le 63 ]] \
    || die "--container invalido: $CONTAINER (minusculas, numeros e '-', ate 63; igual ao deploy/mariadb-docker.sh)"
  [[ "$REDE_NOME" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || die "--rede-nome invalido: $REDE_NOME"
  [[ "$IP_ADMIN" =~ $RE_IPV4_CANONICO ]] || die "--ip-admin invalido: $IP_ADMIN (IPv4 canonico a.b.c.d, sem zeros a esquerda)"
  for d in "$SEGREDOS_DIR" "$SSL_DIR" "$TRABALHO_DIR"; do
    # Antes do regex e do teste de "/": "//" em qualquer posicao e recusado
    # (senao "//" viraria "/" ao tirar a barra final).
    [[ "$d" != *//* ]] || die "diretorio invalido: '$d' (barras repetidas '//' nao sao aceitas)"
    [[ "$d" =~ ^/[A-Za-z0-9._/-]+$ && "$d" != / && "$d" != *..* ]] \
      || die "diretorio invalido: '$d' (caminho absoluto, sem espacos/virgulas/..)"
  done
  # BACKUP-01: a guarda do --forcar. O diretorio so e conferido se ela disparar.
  [[ "$BACKUP_DIR" =~ ^/[A-Za-z0-9._/-]+$ && "$BACKUP_DIR" != *..* ]] \
    || die "--backup-dir invalido: '$BACKUP_DIR' (caminho absoluto, sem espacos/virgulas/..)"
  # shellcheck disable=SC2015 # die deve rodar se o formato OU a faixa falhar (pretendido)
  [[ "$BACKUP_JANELA_MIN" =~ ^[0-9]{1,4}$ ]] && (( 10#$BACKUP_JANELA_MIN >= 1 && 10#$BACKUP_JANELA_MIN <= 1440 )) \
    || die "--backup-janela-min precisa ser um numero de 1 a 1440"
  BACKUP_JANELA_MIN=$((10#$BACKUP_JANELA_MIN))
  # Achado do SecBrain (Lote 42): a mesma regra do deploy/mariadb-docker.sh.
  if (( BACKUP_JANELA_MIN > BACKUP_JANELA_LIVRE_MAX )) && [[ -z "$SEM_BACKUP_MOTIVO" ]]; then
    die "--backup-janela-min acima de $BACKUP_JANELA_LIVRE_MAX exige --sem-backup-motivo TEXTO (a justificativa fica registrada; o dump continua conferido)"
  fi
  if [[ -n "$SEM_BACKUP_MOTIVO" ]]; then
    $FORCAR || die "--sem-backup-motivo so com --forcar"
    [[ "$SEM_BACKUP_MOTIVO" =~ ^[[:print:]]{10,300}$ ]] || die "--sem-backup-motivo precisa de 10 a 300 caracteres imprimiveis"
  fi
  # Sem a barra final: a comparacao de prefixo fica exata.
  SEGREDOS_DIR=${SEGREDOS_DIR%/}; SSL_DIR=${SSL_DIR%/}; TRABALHO_DIR=${TRABALHO_DIR%/}
  ADMIN_SENHA_ARQ=${ADMIN_SENHA_ARQ:-$SEGREDOS_DIR/admin-app.senha}
  # SEC-60: o caminho do arquivo da senha, mesmas regras dos diretorios.
  [[ "$ADMIN_SENHA_ARQ" == /* ]] || die "--admin-senha-arquivo precisa de caminho absoluto: $ADMIN_SENHA_ARQ"
  [[ "$ADMIN_SENHA_ARQ" =~ ^/[A-Za-z0-9._/-]+$ && "$ADMIN_SENHA_ARQ" != *..* && "$ADMIN_SENHA_ARQ" != *//* && "$ADMIN_SENHA_ARQ" != */ ]] \
    || die "--admin-senha-arquivo invalido: '$ADMIN_SENHA_ARQ' (caminho absoluto de arquivo, sem espacos/virgulas/.. nem '//')"
}

# Disjuncao pelos caminhos reais (realpath -m, como o mariadb-docker.sh):
# o --ssl-dir (755, publico) nunca contem nem fica dentro dos diretorios
# privados; a area de trabalho nao fica nos segredos (o contrario pode: o
# padrao /root/rotaperfumes-mariadb esta dentro de /root).
validar_disjuncao() {
  local seg ssl trab adm
  seg=$(realpath -m -- "$SEGREDOS_DIR") || die "--segredos-dir $SEGREDOS_DIR: realpath falhou"
  ssl=$(realpath -m -- "$SSL_DIR") || die "--ssl-dir $SSL_DIR: realpath falhou"
  trab=$(realpath -m -- "$TRABALHO_DIR") || die "--trabalho-dir $TRABALHO_DIR: realpath falhou"
  if dentro_de "$ssl" "$seg" || dentro_de "$seg" "$ssl"; then
    die "--ssl-dir ($SSL_DIR) e --segredos-dir ($SEGREDOS_DIR) precisam ser disjuntos (caminhos reais: $ssl e $seg)"
  fi
  if dentro_de "$ssl" "$trab" || dentro_de "$trab" "$ssl"; then
    die "--ssl-dir ($SSL_DIR) e --trabalho-dir ($TRABALHO_DIR) precisam ser disjuntos (caminhos reais: $ssl e $trab)"
  fi
  dentro_de "$trab" "$seg" \
    && die "--trabalho-dir ($TRABALHO_DIR) nao pode ser --segredos-dir ($SEGREDOS_DIR) nem ficar dentro dele (caminhos reais: $trab e $seg)"
  adm=$(realpath -m -- "$(dirname -- "$ADMIN_SENHA_ARQ")") || die "--admin-senha-arquivo $ADMIN_SENHA_ARQ: realpath falhou"
  dentro_de "$adm" "$ssl" \
    && die "--admin-senha-arquivo ($ADMIN_SENHA_ARQ) nao pode ficar no --ssl-dir ($SSL_DIR), que e publico"
  return 0
}

# Copia byte a byte do conferir_dir_privado do deploy/mariadb-docker.sh
# (o teste do TST-17 confere a paridade): nenhum componente symlink,
# ancestrais do root sem escrita para grupo/outros, diretorio root 700.
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

# BACKUP-01: copia byte a byte da funcao do deploy/mariadb-docker.sh (um teste
# confere). Vale o que esta la: so o dump, parcial-* vale com aviso (BACKUP-04),
# pre-restauracao-* nunca, e a idade conta pelo carimbo e pelo mtime (folga de
# TOLERANCIA_RELOGIO_S para o futuro, BACKUP-03).
conferir_backup_recente() { # dir janela_min -> define BACKUP_USADO
  local dir=$1 janela=$2 LC_ALL=C arq="" melhor="" chave f nome agora mtime carimbo dica
  dica="gere um com: sudo rotaperfumes-backup-banco --agora (ou use --sem-backup-motivo TEXTO, que fica registrado)"
  command -v gzip >/dev/null || die "comando 'gzip' nao encontrado"
  [[ -d "$dir" && ! -L "$dir" ]] || die "nenhum backup: $dir nao existe. Instale com sudo bash deploy/backup-banco.sh --instalar; $dica"
  conferir_dir_privado --backup-dir "$dir" existe
  for f in "$dir"/rotaperfumes-*.sql.gz "$dir"/parcial-rotaperfumes-*.sql.gz; do
    if [[ "${f##*/}" =~ ^rotaperfumes-([0-9]{8}-[0-9]{6}Z)(-manual)?\.sql\.gz$ ]]; then
      chave="${BASH_REMATCH[1]}2"
    elif [[ "${f##*/}" =~ ^parcial-rotaperfumes-([0-9]{8}-[0-9]{6}Z)(-manual)?\.sql\.gz$ ]]; then
      chave="${BASH_REMATCH[1]}1"
    else
      continue
    fi
    if [[ -z "$melhor" || $chave > $melhor ]]; then melhor=$chave; arq=$f; fi
  done
  [[ -n "$arq" ]] || die "nenhum backup em $dir; $dica"
  nome=${arq##*/}
  [[ ! -L "$arq" && -f "$arq" ]] || die "o backup mais recente ($nome) nao e arquivo regular; $dica"
  [[ "$(stat -c '%u %a' -- "$arq")" == "0 600" ]] || die "o backup mais recente ($nome) precisa ser do root com modo 600; $dica"
  [[ "$nome" =~ ^(parcial-)?rotaperfumes-([0-9]{4})([0-9]{2})([0-9]{2})-([0-9]{2})([0-9]{2})([0-9]{2})Z ]] || die "carimbo invalido em $nome"
  carimbo=$(date -u -d "${BASH_REMATCH[2]}-${BASH_REMATCH[3]}-${BASH_REMATCH[4]} ${BASH_REMATCH[5]}:${BASH_REMATCH[6]}:${BASH_REMATCH[7]}" +%s) \
    || die "carimbo invalido em $nome"
  agora=$(date +%s)
  mtime=$(stat -c %Y -- "$arq")
  for f in "$carimbo" "$mtime"; do
    (( f <= agora + TOLERANCIA_RELOGIO_S )) || die "o backup mais recente ($nome) tem data futura (mais de ${TOLERANCIA_RELOGIO_S} s a frente do relogio); $dica"
    (( agora - f <= janela * 60 )) || die "o backup mais recente ($nome) tem mais de $janela min; $dica"
  done
  gzip -t -- "$arq" 2>/dev/null || die "o backup mais recente ($nome) esta corrompido; $dica"
  [[ "$(gzip -dc -- "$arq" | tail -n 1)" == "-- Dump completed"* ]] || die "o backup mais recente ($nome) nao tem o marcador na ultima linha; $dica"
  [[ "$nome" != parcial-* ]] \
    || aviso "o backup usado e parcial ($nome): o dump vale para o banco, mas o tar de imagens desse backup falhou; confira com journalctl -u rotaperfumes-backup-banco.service"
  BACKUP_USADO=$arq
}

# conferir_dir_root_sem_escrita: como o conferir_dir_privado, mas para o
# --ssl-dir (publico, 755): precisa existir, ser do root e sem escrita para
# grupo/outros (o modo 700 nao e exigido).
conferir_dir_root_sem_escrita() { # opcao dir
  local op=$1 d=$2 real p dono modo
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
      || die "$op $d: o ancestral $p tem escrita para grupo/outros (modo $modo; o sticky bit nao basta)"
  done
  [[ -d "$d" ]] || die "$op $d nao existe ou nao e diretorio (deploy/mariadb-docker.sh)"
  read -r dono modo < <(stat -c '%u %a' -- "$d")
  if [[ "$dono" != 0 ]] || (( (8#$modo & 8#022) != 0 )); then
    die "$op $d precisa ser do root sem escrita para grupo/outros (atual: dono $dono, modo $modo)"
  fi
}

# conferir_ca_host: <ssl-dir>/ca.pem regular (nao symlink), nao vazio, do
# root e sem escrita para grupo/outros.
conferir_ca_host() {
  local dono modo
  [[ -f "$CA_HOST" && ! -L "$CA_HOST" && -s "$CA_HOST" ]] \
    || die "CA do banco nao encontrada em $CA_HOST (arquivo regular nao vazio, sem symlink; deploy/mariadb-docker.sh)"
  read -r dono modo < <(stat -c '%u %a' -- "$CA_HOST")
  if [[ "$dono" != 0 ]] || (( (8#$modo & 8#022) != 0 )); then
    die "CA $CA_HOST precisa ser do root sem escrita para grupo/outros (atual: dono $dono, modo $modo)"
  fi
}

# conferir_container_rede: container rodando, ligado a rede (nome exato) e
# IP do admin = A.B.C.20 da unica subnet IPv4 A.B.C.0/24 da rede (o
# mariadb-docker.sh so cria rotaperfumes_admin@<prefixo>.20). So inspect.
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

# ---------------------------------------------------------------------------
etapa "Pre-requisitos"
validar_sintaxe_opcoes
[[ $EUID -eq 0 ]] || die "rode como root: sudo bash $0"
# BUG-26: sem terminal, falha ja aqui (antes do Docker e do banco) quando a
# execucao vai precisar dele (pepper sem --pepper-file; confirmacao do --forcar).
if [[ -z "$PEPPER_FILE" ]]; then
  tem_tty || die "sem terminal para pedir o PASSWORD_PEPPER: use --pepper-file ARQ"
fi
if $FORCAR; then
  tem_tty || die "--forcar exige um terminal para a confirmacao"
fi
for cmd in docker tar shred stat install mktemp grep realpath dirname; do
  command -v "$cmd" >/dev/null || die "comando '$cmd' nao encontrado"
done
docker info >/dev/null 2>&1 || die "o daemon do Docker nao responde"
[[ "$DB_CLIENT_IMAGE" =~ ^mariadb:[0-9.]+@sha256:[0-9a-f]{64}$ ]] || die "DB_CLIENT_IMAGE invalida: $DB_CLIENT_IMAGE"
[[ "$GO_IMAGE" =~ ^golang:[0-9.]+-alpine@sha256:[0-9a-f]{64}$ ]] || die "GO_IMAGE invalida: $GO_IMAGE"


# TST-17: diretorios (caminho real, ancestrais do root, donos e modos).
# Mudanca de comportamento (fail closed): /root fora de root 700 ou
# /root/rotaperfumes-mariadb fora de 700 agora recusam.
validar_disjuncao
[[ -d "$SEGREDOS_DIR" ]] || die "--segredos-dir $SEGREDOS_DIR nao existe (deploy/mariadb-docker.sh)"
conferir_dir_privado --segredos-dir "$SEGREDOS_DIR" existe
[[ -d "$TRABALHO_DIR" ]] || die "--trabalho-dir $TRABALHO_DIR nao existe"
conferir_dir_privado --trabalho-dir "$TRABALHO_DIR" existe
conferir_dir_root_sem_escrita --ssl-dir "$SSL_DIR"
# Destino/forma da senha do admin. SEC-60: o diretorio do arquivo e privado
# (root 700, ancestrais sem escrita, sem symlink), conferido antes de ler a
# senha do banco e o pepper: num diretorio gravavel por outros, o arquivo
# poderia ser trocado entre a conferencia e o install -m 600.
[[ -d "$(dirname -- "$ADMIN_SENHA_ARQ")" ]] || die "--admin-senha-arquivo: diretorio de $ADMIN_SENHA_ARQ nao existe"
conferir_dir_privado --admin-senha-arquivo "$(dirname -- "$ADMIN_SENHA_ARQ")" existe
[[ ! -e "$ADMIN_SENHA_ARQ" && ! -L "$ADMIN_SENHA_ARQ" ]] \
  || die "$ADMIN_SENHA_ARQ ja existe: confira se a senha anterior ainda e necessaria, apague com shred -u ou use --admin-senha-arquivo"
readonly CONTAINER REDE_NOME IP_ADMIN SEGREDOS_DIR SSL_DIR TRABALHO_DIR ADMIN_SENHA_ARQ
readonly SENHA_DB_ARQ="$SEGREDOS_DIR/$DB_ADMIN.senha"
readonly CA_HOST="$SSL_DIR/ca.pem"
conferir_ca_host

conferir_container_rede
arquivo_secreto_ok "$SENHA_DB_ARQ" || die "$SENHA_DB_ARQ ausente ou com dono/modo errado (esperado root, 600)"
SENHA_DB=$(<"$SENHA_DB_ARQ")
[[ "$SENHA_DB" =~ ^[A-Za-z0-9]{20,}$ ]] || die "senha em $SENHA_DB_ARQ fora do formato [A-Za-z0-9]{20,}"

# sql/ordem-ddl.txt: so nomes de DDL de sql/, todos existentes.
[[ -r "$ORDEM_DDL" ]] || die "$ORDEM_DDL nao encontrado (rode a partir do checkout do repositorio)"
DDLS=()
while IFS= read -r linha || [[ -n "$linha" ]]; do
  linha=${linha%$'\r'}
  linha=${linha#"${linha%%[![:space:]]*}"}
  linha=${linha%"${linha##*[![:space:]]}"}
  [[ -z "$linha" || "$linha" == \#* ]] && continue
  [[ "$linha" =~ ^[0-9]{2}_ddl_[a-z_]+\.sql$ ]] || die "linha invalida em $ORDEM_DDL: '$linha' (esperado NN_ddl_nome.sql)"
  [[ -r "$SQL_DIR/$linha" ]] || die "$SQL_DIR/$linha (listado em $ORDEM_DDL) nao encontrado"
  DDLS+=("$linha")
done < "$ORDEM_DDL"
(( ${#DDLS[@]} > 0 )) || die "$ORDEM_DDL sem nenhum arquivo"
for f in "${SEEDS[@]}"; do
  [[ -r "$SQL_DIR/$f" ]] || die "$SQL_DIR/$f nao encontrado"
done
[[ -r "$RAIZ/backend/shared/go.mod" ]] || die "$RAIZ/backend/shared/go.mod nao encontrado"

# PASSWORD_PEPPER: arquivo (600/400 do root) ou terminal sem eco.
PEPPER=""
if [[ -n "$PEPPER_FILE" ]]; then
  arquivo_secreto_ok "$PEPPER_FILE" || die "--pepper-file $PEPPER_FILE: precisa ser arquivo regular do root com modo 600 ou 400"
  IFS= read -r PEPPER < "$PEPPER_FILE" || [[ -n "$PEPPER" ]] || die "--pepper-file $PEPPER_FILE vazio"
else
  tem_tty || die "sem terminal para pedir o PASSWORD_PEPPER: use --pepper-file ARQ"
  IFS= read -rs -p "PASSWORD_PEPPER (o mesmo do rotaperfumes-api-env; nao aparece na tela): " PEPPER < /dev/tty \
    || die "leitura do PASSWORD_PEPPER no terminal falhou (use --pepper-file ARQ); nada foi alterado"
  printf '\n' >&2
  IFS= read -rs -p "Confirme o PASSWORD_PEPPER: " PEPPER2 < /dev/tty \
    || die "leitura da confirmacao do PASSWORD_PEPPER falhou; nada foi alterado"
  printf '\n' >&2
  [[ "$PEPPER" == "$PEPPER2" ]] || die "os dois PASSWORD_PEPPER digitados nao conferem"
  unset PEPPER2
fi
PEPPER=${PEPPER%$'\r'}
(( ${#PEPPER} >= PEPPER_MIN_BYTES )) || die "PASSWORD_PEPPER com menos de $PEPPER_MIN_BYTES caracteres (a API recusaria)"
[[ "$PEPPER" != *$'\n'* ]] || die "PASSWORD_PEPPER com quebra de linha"
case "$PEPPER" in
  \"*\"|\'*\') aviso "o PASSWORD_PEPPER informado comeca e termina com aspas: confira se elas fazem parte do valor no rotaperfumes-api-env." ;;
esac
info "Pre-requisitos OK (container $CONTAINER, rede $REDE_NOME, ${#DDLS[@]} DDL, pepper lido; nada foi impresso)."

TRAB=$(mktemp -d "$TRABALHO_DIR/rotaperfumes-seed.XXXXXX") || die "mktemp em $TRABALHO_DIR falhou"
# TST-17: a area de trabalho e o caminho real, do root, modo 700.
[[ ! -L "$TRAB" && "$(realpath -e -- "$TRAB")" == "$TRAB" && "$(stat -c '%u %a' -- "$TRAB")" == "0 700" ]] \
  || die "area de trabalho $TRAB fora do esperado (caminho real, root, modo 700)"
install -d -m 700 -o root -g root "$TRAB/bin" "$TRAB/saida"

# Cliente mariadb descartavel na rede interna (padrao do mariadb_cli do Jenkinsfile).
mariadb_cli() {
  MYSQL_PWD="$SENHA_DB" docker run --rm -i -e MYSQL_PWD \
    --network "$REDE_NOME" --ip "$IP_ADMIN" \
    --cap-drop ALL --security-opt no-new-privileges:true \
    --mount "type=bind,source=$CA_HOST,target=$C_CA,readonly" \
    --entrypoint mariadb "$DB_CLIENT_IMAGE" \
    --local-infile=0 -h "$CONTAINER" -P 3306 -u "$DB_ADMIN" \
    --ssl-ca="$C_CA" --ssl-verify-server-cert \
    --default-character-set=utf8mb4 "$@"
}
# </dev/null: o docker run -i leria o terminal e comeria o que o operador ja
# digitou (ex.: a confirmacao do --forcar; travou o teste real do TST-17).
consulta() { mariadb_cli -N -B "$DB_NOME" -e "$1" </dev/null; }

# ---------------------------------------------------------------------------
etapa "Estado do banco $DB_NOME"
consulta 'SELECT 1' >/dev/null \
  || die "sem conexao TLS verificada em $CONTAINER:3306 como $DB_ADMIN@$IP_ADMIN (confira o IP $IP_ADMIN livre e o deploy/mariadb-docker.sh)"
TEM_USUARIOS=$(consulta "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = '$DB_NOME' AND TABLE_NAME = 'usuarios'")
LINHAS=0
if [[ "$TEM_USUARIOS" == 1 ]]; then
  LINHAS=$(consulta 'SELECT COUNT(*) FROM usuarios')
fi
[[ "$LINHAS" =~ ^[0-9]+$ ]] || die "resposta inesperada ao contar usuarios: '$LINHAS'"
if (( LINHAS > 0 )); then
  $FORCAR || die "a tabela usuarios ja tem $LINHAS linha(s): este banco ja foi semeado. Nada foi alterado. (--forcar reaplica e INATIVA todos os nao-admin)"
  tem_tty || die "--forcar exige um terminal para a confirmacao"
  # BACKUP-01 (D6): a mesma exigencia do --recriar, antes de pedir a confirmacao.
  if (( BACKUP_JANELA_MIN > BACKUP_JANELA_LIVRE_MAX )); then
    aviso "--backup-janela-min $BACKUP_JANELA_MIN (acima de $BACKUP_JANELA_LIVRE_MAX): dump de ate $BACKUP_JANELA_MIN min aceito. Motivo: $SEM_BACKUP_MOTIVO"
    conferir_backup_recente "$BACKUP_DIR" "$BACKUP_JANELA_MIN"
    info "Backup conferido (janela de $BACKUP_JANELA_MIN min): $BACKUP_USADO."
  elif [[ -n "$SEM_BACKUP_MOTIVO" ]]; then
    aviso "--sem-backup-motivo: --forcar SEM conferir backup. Motivo: $SEM_BACKUP_MOTIVO"
  else
    conferir_backup_recente "$BACKUP_DIR" "$BACKUP_JANELA_MIN"
    info "Backup recente conferido: $BACKUP_USADO."
  fi
  aviso "--forcar: usuarios tem $LINHAS linha(s). A DDL e os seeds serao reaplicados, a senha do admin trocada e TODOS os usuarios nao-admin INATIVADOS."
  IFS= read -r -p "Digite o nome do banco ($DB_NOME) para confirmar: " RESPOSTA < /dev/tty \
    || die "--forcar: leitura da confirmacao falhou; nada foi alterado"
  [[ "$RESPOSTA" == "$DB_NOME" ]] || die "confirmacao nao confere; nada foi alterado"
  printf '%s janela=%s backup=%s motivo=%s\n' "$(date -Is)" "$BACKUP_JANELA_MIN" "${BACKUP_USADO:-nenhum}" "${SEM_BACKUP_MOTIVO:-nenhum}" >>"$SEGREDOS_DIR/seed-forcar.registro"
  chmod 600 "$SEGREDOS_DIR/seed-forcar.registro"
else
  info "Banco sem usuarios (instalacao nova): seguindo."
fi

# ---------------------------------------------------------------------------
etapa "DDL (sql/ordem-ddl.txt)"
for f in "${DDLS[@]}"; do
  info "Aplicando sql/$f"
  mariadb_cli "$DB_NOME" < "$SQL_DIR/$f" >/dev/null || die "falhou a DDL sql/$f"
done

# ---------------------------------------------------------------------------
etapa "Seeds (admin e vendedores) e vendedores inativos"
for f in "${SEEDS[@]}"; do
  info "Aplicando sql/$f"
  mariadb_cli "$DB_NOME" < "$SQL_DIR/$f" >/dev/null || die "falhou o seed sql/$f"
done
# Os 42 vendedores sao reais: entram INATIVOS (o admin ativa pelo reset).
mariadb_cli "$DB_NOME" -e "UPDATE usuarios SET ativo = 0 WHERE role <> 'admin'" </dev/null \
  || die "falhou a inativacao dos usuarios nao-admin"
info "Usuarios nao-admin inativos: $(consulta "SELECT COUNT(*) FROM usuarios WHERE role <> 'admin' AND ativo = 0")."

# ---------------------------------------------------------------------------
etapa "Compilando o resetpassword ($GO_IMAGE)"
# Sem rede do banco e sem segredos: so o codigo de backend/shared por stdin
# (sem .env). Rede padrao (baixa os modulos do proxy do Go). SEC-52:
# --cap-drop ALL; o tar extrai sem chown (-o), que exigiria CAP_CHOWN.
tar -C "$RAIZ" -cf - --exclude='.env' --exclude='.env.*' --exclude='*.env' \
    --exclude='coverage*.txt' --exclude='coverage.html' backend/shared \
  | docker run --rm -i --cap-drop ALL --security-opt no-new-privileges:true \
      -e CGO_ENABLED=0 -e GOFLAGS=-mod=readonly \
      --mount "type=bind,source=$TRAB/bin,target=/out" \
      "$GO_IMAGE" sh -ec '
        mkdir -p /src && tar -xof - -C /src
        cd /src/backend/shared && go build -trimpath -o /out/resetpassword ./cmd/resetpassword
      ' || die "falhou a compilacao do resetpassword"
[[ -x "$TRAB/bin/resetpassword" ]] || die "binario do resetpassword nao foi gerado"
info "resetpassword compilado."

# ---------------------------------------------------------------------------
etapa "Admin ($ADMIN_EMAIL)"
ENV_ARQ="$TRAB/resetpassword.env"
{
  printf 'DB_HOST=%s\n' "$CONTAINER"
  printf 'DB_PORT=3306\n'
  printf 'DB_USUARIO=%s\n' "$DB_ADMIN"
  printf 'DB_SENHA=%s\n' "$SENHA_DB"
  printf 'DB_NAME=%s\n' "$DB_NOME"
  printf 'DB_TLS_CA=%s\n' "$C_CA"
  printf 'DB_TLS_SERVER_NAME=%s\n' "$CONTAINER"
  printf 'PASSWORD_PEPPER=%s\n' "$PEPPER"
} > "$ENV_ARQ"
chmod 600 "$ENV_ARQ"
unset PEPPER

RP_ARGS=(
  --rm --network "$REDE_NOME" --ip "$IP_ADMIN"
  --cap-drop ALL --security-opt no-new-privileges:true --read-only
  --env-file "$ENV_ARQ"
  --mount "type=bind,source=$CA_HOST,target=$C_CA,readonly"
  --mount "type=bind,source=$TRAB/bin/resetpassword,target=/usr/local/bin/resetpassword,readonly"
  -w / --entrypoint /usr/local/bin/resetpassword
)
# SEC-52: senha SEMPRE gerada (aleatoria), so em arquivo 600; nunca digitada.
docker run -i "${RP_ARGS[@]}" \
  --mount "type=bind,source=$TRAB/saida,target=/saida" \
  "$GO_IMAGE" -create-admin -password-out /saida/admin.senha </dev/null \
  || die "o resetpassword -create-admin falhou"
[[ -s "$TRAB/saida/admin.senha" ]] || die "o resetpassword nao gravou a senha do admin"
[[ ! -e "$ADMIN_SENHA_ARQ" ]] || die "$ADMIN_SENHA_ARQ apareceu durante a execucao; a senha nova ficou so no banco: rode de novo com --forcar"
install -m 600 -o root -g root "$TRAB/saida/admin.senha" "$ADMIN_SENHA_ARQ"
shred -u "$ENV_ARQ"
ENV_ARQ=""

# ---------------------------------------------------------------------------
etapa "Conferencia"
N_TABELAS=$(consulta "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = '$DB_NOME'")
ADMIN_OK=$(mariadb_cli -N -B "$DB_NOME" <<'SQL'
SELECT CONCAT(ativo, ' ', deve_trocar_senha, ' ', password_hash LIKE CONCAT(CHAR(36), 'argon2id', CHAR(36), '%'))
FROM usuarios WHERE email = 'admin@rotaperfumes.com.br';
SQL
)
ATIVOS_NAO_ADMIN=$(consulta "SELECT COUNT(*) FROM usuarios WHERE role <> 'admin' AND ativo <> 0")
[[ "$ADMIN_OK" == "1 1 1" ]] || die "admin fora do esperado (ativo, deve_trocar_senha, argon2id) = '$ADMIN_OK'"
[[ "$ATIVOS_NAO_ADMIN" == 0 ]] || die "$ATIVOS_NAO_ADMIN usuario(s) nao-admin ativo(s); esperado 0"
info "Tabelas em $DB_NOME: $N_TABELAS. Admin ativo, Argon2id, troca obrigatoria no 1o login: OK."
info "Usuarios nao-admin ativos: 0 (o admin ativa cada vendedor pelo reset de senha)."

cat <<EOF

Pronto. Seed inicial aplicado em $DB_NOME ($CONTAINER).
EOF
cat <<EOF
Senha inicial do admin ($ADMIN_EMAIL), gerada e NAO impressa:
  $ADMIN_SENHA_ARQ (600, so root): sudo cat $ADMIN_SENHA_ARQ
  Faca o 1o login (troca obrigatoria) e depois: sudo shred -u $ADMIN_SENHA_ARQ
EOF
cat <<EOF
Proximos passos: confira o deploy/db-ca.fingerprint (so a CA de producao),
o Secret file rotaperfumes-api-env (mesmo PASSWORD_PEPPER) e rode o pipeline
da main (docs/deploy-servidor.md §11).
EOF
