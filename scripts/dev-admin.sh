#!/usr/bin/env bash

# Zink Wallet - Admin local development runner (Mac/Linux).
#
# Starts:
# - Docker infra (MariaDB, Redis, ETCD, Jaeger, Swagger UI)
# - Go accounting RPC with hot reload (air)
# - Go business RPC with hot reload (air)
# - Go chainrpc RPC with hot reload (air)
# - Go chainsync RPC with hot reload (air)
# - Go admin RPC with hot reload (air)
# - Go swap RPC with hot reload (air)
# - Go api-gateway with hot reload (air)
# - Admin web frontend (bun dev)
#
# Usage:
#   ./scripts/dev-admin.sh infra-up
#   ./scripts/dev-admin.sh reset-db       # remove MariaDB data + re-init schema
#   ./scripts/dev-admin.sh migrate-db     # apply RBAC v2 schema/seed (non-destructive)
#   ./scripts/dev-admin.sh reset-db --force
#   ./scripts/dev-admin.sh reset-db --stop-app
#   ./scripts/dev-admin.sh up            # tmux if available, else background
#   ./scripts/dev-admin.sh up --bg       # force background mode
#   ./scripts/dev-admin.sh restart       # stop app processes then start again
#   ./scripts/dev-admin.sh logs-admin    # tail admin service logs
#   ./scripts/dev-admin.sh stop          # stop app processes only
#   ./scripts/dev-admin.sh down          # stop app processes + docker infra

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

COMPOSE_FILE="$PROJECT_ROOT/deploy/admin/docker-compose.infra.yml"
PID_DIR="$PROJECT_ROOT/.pids/dev-admin"
LOG_DIR="$PROJECT_ROOT/logs/dev-admin"
TMUX_SESSION="internal-wallet-admin-dev"
MARIADB_VOLUME_DIR="$PROJECT_ROOT/.docker/volumes/mariadb"
INIT_DB_DIR="$PROJECT_ROOT/deploy/docker/init-db"
BACKUP_DIR="$PROJECT_ROOT/.backups/mariadb"
BACKUP_KEEP_COUNT=5
# NOTE: keep this list minimal to avoid killing unrelated local services.
APP_PORTS=(8000 8080 8081 9001 9005 9013 9014 9015 9094 19091)

mkdir -p "$PID_DIR" "$LOG_DIR"

ENV_FILE_ADMIN="$PROJECT_ROOT/deploy/admin/.env"
GATEWAY_LOCAL_YAML="$PROJECT_ROOT/api-gateway/etc/gateway.local.yaml"

load_admin_env_if_present() {
  # Load deploy/admin/.env for local dev (host-run services). This file is intentionally NOT tracked by git.
  if [ -f "$ENV_FILE_ADMIN" ]; then
    set -a
    # shellcheck disable=SC1090
    . "$ENV_FILE_ADMIN"
    set +a
  fi

  # Ensure booleans used by config templates are always defined (avoid empty -> invalid YAML bool).
  : "${S3_ENABLED:=false}"
  : "${S3_USE_PATH_STYLE:=false}"
  : "${S3_PREFIX:=admin}"
  export S3_ENABLED S3_USE_PATH_STYLE S3_PREFIX
}

render_gateway_local_yaml() {
  mkdir -p "$PROJECT_ROOT/api-gateway/etc"
  cat >"$GATEWAY_LOCAL_YAML" <<'EOF'
Name: api-gateway
Mode: dev

Host: 0.0.0.0
Port: 8080

Log:
  ServiceName: api-gateway
  Mode: console
  Path: logs/gateway
  Level: info
  Compress: true
  KeepDays: 7
  StackCooldownMillis: 100

Auth:
  AccessSecret: "F0Fu3arY13GQFG1JinnlkHLxaJzHpZPavVKZjCJySx+d15GVCKmKGdJv4C1gnzqizhjgU9gkU6VeykpfYn5UUQ=="
  AccessExpire: 86400
  RefreshSecret: "Tcs10IiTu9GFUExWeqBvv+Z61k4GcM/5UKrV3rJHnVeitEI9eQc8dBV/V81CMONBGnrtPPPRU2saIJtfiM2mWA=="
  RefreshExpire: 604800

AdminAuth:
  AccessSecret: "4kKh3PGT4ByQyDZFny+T9ZXO0SHsWJVpddZCXzfOKo2t9XHn2nDp7xEiKviDTn8eJbz/k5kn8a//GRVUTHsGGg=="
  AccessExpire: 28800
  RefreshSecret: "GwOJj9HIRijQk34JYz7nZhMH8zsp/gIrtp3XsRdjyU8qjGIn57Rj/jaAapzoZ/tLUYB4ikxQc59Qujcpm+D5IA=="
  RefreshExpire: 604800

BusinessRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:15279
    Key: business.rpc
  Timeout: 30000

ChainRpcRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:15279
    Key: chainrpc.rpc
  Timeout: 30000

ChainSyncRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:15279
    Key: chainsync.rpc
  Timeout: 30000

SignerRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:15279
    Key: signer.rpc
  Timeout: 30000

AdminRpc:
  Etcd:
    Hosts:
      - 127.0.0.1:15279
    Key: admin.rpc
  Timeout: 30000

Redis:
  Host: 127.0.0.1
  Port: 16380
  Password: "redis123456"
  DB: 0

RateLimit:
  Enable: true
  UseRedis: true
  GlobalRate: 1000
  GlobalBurst: 2000

RequestTimeout: 30000

# S3 upload configuration (optional)
# Values are expanded from environment variables (deploy/admin/.env -> scripts/dev-admin.sh loads).
S3:
  Enabled: ${S3_ENABLED}
  Region: "${S3_REGION}"
  Bucket: "${S3_BUCKET}"
  Endpoint: "${S3_ENDPOINT}"
  AccessKeyID: "${S3_ACCESS_KEY_ID}"
  SecretAccessKey: "${S3_SECRET_ACCESS_KEY}"
  SessionToken: "${S3_SESSION_TOKEN}"
  PublicBaseURL: "${S3_PUBLIC_BASE_URL}"
  Prefix: "${S3_PREFIX}"
  UsePathStyle: ${S3_USE_PATH_STYLE}
  ACL: "${S3_ACL}"

Cors:
  AllowOrigins:
    - "*"
  AllowMethods:
    - "GET"
    - "POST"
    - "PUT"
    - "DELETE"
    - "PATCH"
    - "OPTIONS"
  AllowHeaders:
    - "Content-Type"
    - "Authorization"
    - "Idempotency-Key"
    - "X-Request-ID"
    - "X-Trace-ID"
  ExposeHeaders:
    - "X-Request-ID"
    - "X-Trace-ID"
  AllowCredentials: false
  MaxAge: 86400

TrustedProxies: []
EOF
}

docker_compose() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
    return
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    docker-compose "$@"
    return
  fi
  echo "[error] docker compose not found (need Docker Desktop / docker-compose)." >&2
  exit 1
}

wait_for_port() {
  local host="$1"
  local port="$2"
  local name="${3:-$host:$port}"
  local retries="${4:-60}"
  local delay="${5:-0.5}"

  for _ in $(seq 1 "$retries"); do
    if command -v nc >/dev/null 2>&1; then
      nc -z "$host" "$port" >/dev/null 2>&1 && return 0
    else
      (echo >/dev/tcp/"$host"/"$port") >/dev/null 2>&1 && return 0
    fi
    sleep "$delay"
  done

  echo "[error] timed out waiting for $name ($host:$port)" >&2
  return 1
}

list_listen_pids() {
  local port="$1"
  if ! command -v lsof >/dev/null 2>&1; then
    return 0
  fi
  # macOS / Linux variants.
  lsof -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null || lsof -nP -iTCP:"$port" -sTCP:LISTEN -t 2>/dev/null || true
}

maybe_add_parent_pid() {
  local pid="$1"
  local ppid=""
  ppid="$(ps -o ppid= -p "$pid" 2>/dev/null | tr -d ' ' || true)"
  if [ -z "$ppid" ] || [ "$ppid" -le 1 ]; then
    return 0
  fi
  local pcmd=""
  pcmd="$(ps -o command= -p "$ppid" 2>/dev/null || true)"
  # Only walk up to known dev runners to avoid killing unrelated parents.
  case "$pcmd" in
    *"air -c .air.local.toml"*|*"umi/bin/forkedDev.js dev"*|*"bun dev"*)
      echo "$ppid"
      ;;
  esac
}

kill_pids_term_then_kill9() {
  local label="$1"
  shift
  local pids=("$@")
  if [ "${#pids[@]}" -eq 0 ]; then
    return 0
  fi

  # De-dup.
  local uniq=()
  local seen=" "
  for pid in "${pids[@]}"; do
    if [[ "$seen" != *" $pid "* ]]; then
      uniq+=("$pid")
      seen+=" $pid "
    fi
  done
  pids=("${uniq[@]}")

  echo "[stop] killing $label: ${pids[*]}"
  kill "${pids[@]}" >/dev/null 2>&1 || true

  # Wait a bit for graceful shutdown.
  for _ in $(seq 1 10); do
    local alive=0
    for pid in "${pids[@]}"; do
      if kill -0 "$pid" >/dev/null 2>&1; then
        alive=1
        break
      fi
    done
    if [ "$alive" -eq 0 ]; then
      return 0
    fi
    sleep 0.2
  done

  echo "[stop] forcing kill -9 for $label: ${pids[*]}"
  kill -9 "${pids[@]}" >/dev/null 2>&1 || true
}

cleanup_app_ports() {
  # WARNING: this will terminate any processes listening on APP_PORTS.
  # Set DEV_ADMIN_SKIP_PORT_CLEANUP=1 to disable.
  if [ "${DEV_ADMIN_SKIP_PORT_CLEANUP:-}" = "1" ]; then
    return 0
  fi

  local -a all_to_kill=()
  for port in "${APP_PORTS[@]}"; do
    local pids
    pids="$(list_listen_pids "$port" | tr '\n' ' ' | xargs echo -n 2>/dev/null || true)"
    if [ -z "$pids" ]; then
      continue
    fi
    echo "[stop] port $port is in use: $pids"
    for pid in $pids; do
      all_to_kill+=("$pid")
      local parent=""
      parent="$(maybe_add_parent_pid "$pid" || true)"
      if [ -n "$parent" ]; then
        all_to_kill+=("$parent")
        # One more level up (air -> shell wrapper sometimes).
        local parent2=""
        parent2="$(maybe_add_parent_pid "$parent" || true)"
        if [ -n "$parent2" ]; then
          all_to_kill+=("$parent2")
        fi
      fi
    done
  done

  if [ "${#all_to_kill[@]}" -eq 0 ]; then
    return 0
  fi

  kill_pids_term_then_kill9 "dev ports (${APP_PORTS[*]})" "${all_to_kill[@]}"
}

confirm_or_force() {
  local force="${1:-}"
  local prompt="${2:-Are you sure? Type 'yes' to continue: }"

  if [ "$force" = "--force" ]; then
    return 0
  fi

  if [ ! -t 0 ]; then
    echo "[error] non-interactive shell. Pass --force to continue." >&2
    exit 1
  fi

  local answer=""
  read -r -p "$prompt" answer
  if [ "$answer" != "yes" ]; then
    echo "[cancelled]"
    exit 0
  fi
}

infra_up() {
  echo "[infra] starting (docker)..."
  if docker_compose -f "$COMPOSE_FILE" up --help 2>/dev/null | grep -q -- "--wait"; then
    docker_compose -f "$COMPOSE_FILE" up -d --wait
  else
    docker_compose -f "$COMPOSE_FILE" up -d
  fi

  wait_for_port 127.0.0.1 15306 "MariaDB"
  wait_for_port 127.0.0.1 16380 "Redis"
  wait_for_port 127.0.0.1 15279 "ETCD"
  wait_for_port 127.0.0.1 16269 "Jaeger collector"
  echo "[infra] ready"
}

infra_down() {
  echo "[infra] stopping (docker)..."
  docker_compose -f "$COMPOSE_FILE" down
}

reset_db() {
  local force=""
  local stop_app=0

  while [ $# -gt 0 ]; do
    case "$1" in
      --force)
        force="--force"
        ;;
      --stop-app)
        stop_app=1
        ;;
      *)
        echo "[error] unknown option for reset-db: $1" >&2
        echo "Usage: ./scripts/dev-admin.sh reset-db [--force] [--stop-app]" >&2
        exit 1
        ;;
    esac
    shift
  done

  if [ ! -d "$INIT_DB_DIR" ]; then
    echo "[error] init-db directory not found: $INIT_DB_DIR" >&2
    exit 1
  fi

  echo "[db] This will DELETE local MariaDB data dir:"
  echo "     $MARIADB_VOLUME_DIR"
  echo "[db] And re-initialize from:"
  echo "     $INIT_DB_DIR"
  confirm_or_force "$force"

  # Create backup before reset
  mkdir -p "$BACKUP_DIR"
  local cid
  cid="$(docker_compose -f "$COMPOSE_FILE" ps -q mariadb 2>/dev/null || true)"
  if [ -n "$cid" ]; then
    local backup_file="$BACKUP_DIR/crypto_wallet_$(date +%Y%m%d_%H%M%S).sql.gz"
    echo "[backup] creating backup: $(basename "$backup_file")..."
    
    if docker exec "$cid" mysqldump \
         -uroot -proot123456 \
         --single-transaction \
         --routines \
         --triggers \
         --events \
         crypto_wallet 2>/dev/null | gzip > "$backup_file"; then
      echo "[backup] backup created successfully"
      
      # Clean up old backups (keep only the most recent $BACKUP_KEEP_COUNT)
      local old_backups
      old_backups="$(ls -t "$BACKUP_DIR"/crypto_wallet_*.sql.gz 2>/dev/null | tail -n +$((BACKUP_KEEP_COUNT + 1)) || true)"
      if [ -n "$old_backups" ]; then
        echo "[backup] removing old backups (keeping $BACKUP_KEEP_COUNT most recent)..."
        echo "$old_backups" | xargs rm -f
      fi
    else
      echo "[error] backup failed, aborting reset" >&2
      rm -f "$backup_file" 2>/dev/null || true
      exit 1
    fi
  else
    echo "[backup] skipping backup (mariadb not running)"
  fi

  if [ "$stop_app" -eq 1 ]; then
    # Stop app processes to avoid connection errors during reset.
    tmux_stop || true
    stop_bg || true
  else
    echo "[db] note: app services are NOT stopped; they may log DB connection errors during reset."
    echo "[db]       If needed, rerun with --stop-app."
  fi

  echo "[db] stopping mariadb (docker)..."
  docker_compose -f "$COMPOSE_FILE" stop mariadb >/dev/null 2>&1 || true
  docker_compose -f "$COMPOSE_FILE" rm -f mariadb >/dev/null 2>&1 || true

  if [ -z "$MARIADB_VOLUME_DIR" ] || [ "$MARIADB_VOLUME_DIR" = "/" ]; then
    echo "[error] unsafe mariadb volume dir: '$MARIADB_VOLUME_DIR'" >&2
    exit 1
  fi

  echo "[db] removing mariadb data dir..."
  rm -rf "$MARIADB_VOLUME_DIR"

  echo "[db] starting mariadb (docker)..."
  if docker_compose -f "$COMPOSE_FILE" up --help 2>/dev/null | grep -q -- "--wait"; then
    docker_compose -f "$COMPOSE_FILE" up -d --wait mariadb
  else
    docker_compose -f "$COMPOSE_FILE" up -d mariadb
  fi

  wait_for_port 127.0.0.1 15306 "MariaDB"
  echo "[db] reset complete"
}

migrate_db() {
  infra_up

  local cid
  cid="$(docker_compose -f "$COMPOSE_FILE" ps -q mariadb)"
  if [ -z "$cid" ]; then
    echo "[error] mariadb container not found (is infra running?)" >&2
    exit 1
  fi

  local db="crypto_wallet"
  local root_user="root"
  local root_pass="root123456"

  # Ensure database exists (migrate-db may be run on an existing volume).
  docker exec "$cid" mysql -u"$root_user" -p"$root_pass" -e "CREATE DATABASE IF NOT EXISTS ${db} DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" >/dev/null 2>&1 || true

  mysql_table_exists() {
    local table="$1"
    local cnt="0"
    cnt="$(docker exec "$cid" mysql -u"$root_user" -p"$root_pass" -N -s -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${db}' AND table_name='${table}';" 2>/dev/null | tr -d '\r' || echo "0")"
    if [[ "$cnt" =~ ^[0-9]+$ ]] && [ "$cnt" -gt 0 ]; then
      return 0
    fi
    return 1
  }

  apply_sql() {
    local file="$1"
    if [ ! -f "$file" ]; then
      echo "[error] sql file not found: $file" >&2
      exit 1
    fi
    echo "[db] applying: $(basename "$file")"
    docker exec -i "$cid" mysql -u"$root_user" -p"$root_pass" "$db" <"$file"
  }

  echo "[db] migrating RBAC v2 tables (non-destructive)..."

  mysql_table_exists "admin_menu" || apply_sql "$INIT_DB_DIR/04-admin-menu.sql"
  mysql_table_exists "admin_permission" || apply_sql "$INIT_DB_DIR/05-admin-permission.sql"
  mysql_table_exists "admin_role" || apply_sql "$INIT_DB_DIR/06-admin-role.sql"
  mysql_table_exists "admin_role_menu" || apply_sql "$INIT_DB_DIR/07-admin-role-menu.sql"
  mysql_table_exists "admin_role_permission" || apply_sql "$INIT_DB_DIR/08-admin-role-permission.sql"
  mysql_table_exists "admin_user_role" || apply_sql "$INIT_DB_DIR/09-admin-user-role.sql"

  echo "[db] applying init-db migrations (idempotent files)..."

  # Apply all ordered idempotent migrations/seeds under deploy/docker/init-db.
  # Convention: NN-*.sql (order matters). We intentionally skip 04-09 here because
  # they are mostly bootstrap tables handled above (RBAC v2), and/or executed on a fresh volume.
  # New migrations must be picked up automatically without changing this script again.
  #
  # Note: we DO allow 01-03 so migrations can start from 01 if desired.
  for file in "$INIT_DB_DIR"/0[1-3]-*.sql "$INIT_DB_DIR"/[1-9][0-9]-*.sql; do
    apply_sql "$file"
  done

  echo "[db] migrate complete"
}

restore_db() {
  local backup_file=""
  local force=""
  local stop_app=0
  local positional_args=()

  while [ $# -gt 0 ]; do
    case "$1" in
      --force)
        force="--force"
        ;;
      --stop-app)
        stop_app=1
        ;;
      --*)
        echo "[error] unknown option for restore-db: $1" >&2
        echo "Usage: ./scripts/dev-admin.sh restore-db [backup-file] [--force] [--stop-app]" >&2
        exit 1
        ;;
      *)
        positional_args+=("$1")
        ;;
    esac
    shift
  done

  # Get backup file from positional arg
  if [ "${#positional_args[@]}" -gt 0 ]; then
    backup_file="${positional_args[0]}"
  fi

  # If no backup file specified, list available backups and prompt
  if [ -z "$backup_file" ]; then
    if [ ! -d "$BACKUP_DIR" ]; then
      echo "[error] backup directory not found: $BACKUP_DIR" >&2
      echo "[error] no backups available" >&2
      exit 1
    fi

    local -a backups=()
    local backup_list
    backup_list=$(ls -t "$BACKUP_DIR"/crypto_wallet_*.sql.gz 2>/dev/null || true)
    if [ -n "$backup_list" ]; then
      while IFS= read -r file; do
        backups+=("$file")
      done <<< "$backup_list"
    fi

    if [ "${#backups[@]}" -eq 0 ]; then
      echo "[error] no backups found in $BACKUP_DIR" >&2
      exit 1
    fi

    echo "[restore] Available backups:"
    local i
    for i in "${!backups[@]}"; do
      local file="${backups[$i]}"
      local size
      size="$(du -h "$file" 2>/dev/null | cut -f1 || echo "unknown")"
      local date_str
      date_str="$(stat -f "%Sm" -t "%Y-%m-%d %H:%M:%S" "$file" 2>/dev/null || echo "unknown")"
      echo "  [$((i + 1))] $(basename "$file") ($size, $date_str)"
    done

    echo ""
    if [ -n "$force" ]; then
      echo "[error] --force requires backup file to be specified" >&2
      exit 1
    fi

    read -rp "Select backup number (1-${#backups[@]}): " selection
    if ! [[ "$selection" =~ ^[0-9]+$ ]] || [ "$selection" -lt 1 ] || [ "$selection" -gt "${#backups[@]}" ]; then
      echo "[error] invalid selection: $selection" >&2
      exit 1
    fi

    backup_file="${backups[$((selection - 1))]}"
  else
    # If backup_file is just a filename (not a path), look in BACKUP_DIR
    if [[ "$backup_file" != /* ]]; then
      backup_file="$BACKUP_DIR/$backup_file"
    fi
  fi

  if [ ! -f "$backup_file" ]; then
    echo "[error] backup file not found: $backup_file" >&2
    exit 1
  fi

  echo "[restore] This will DELETE local MariaDB data dir:"
  echo "          $MARIADB_VOLUME_DIR"
  echo "[restore] And restore from backup:"
  echo "          $(basename "$backup_file")"
  confirm_or_force "$force"

  # Backup current database before restore
  mkdir -p "$BACKUP_DIR"
  local cid
  cid="$(docker_compose -f "$COMPOSE_FILE" ps -q mariadb 2>/dev/null || true)"
  if [ -n "$cid" ]; then
    local pre_restore_backup="$BACKUP_DIR/crypto_wallet_pre_restore_$(date +%Y%m%d_%H%M%S).sql.gz"
    echo "[backup] creating pre-restore backup: $(basename "$pre_restore_backup")..."
    
    if docker exec "$cid" mysqldump \
         -uroot -proot123456 \
         --single-transaction \
         --routines \
         --triggers \
         --events \
         crypto_wallet 2>/dev/null | gzip > "$pre_restore_backup"; then
      echo "[backup] pre-restore backup created successfully"
    else
      echo "[warning] pre-restore backup failed, continuing..." >&2
      rm -f "$pre_restore_backup" 2>/dev/null || true
    fi
  fi

  if [ "$stop_app" -eq 1 ]; then
    # Stop app processes to avoid connection errors during restore.
    tmux_stop || true
    stop_bg || true
  else
    echo "[restore] note: app services are NOT stopped; they may log DB connection errors during restore."
    echo "[restore]       If needed, rerun with --stop-app."
  fi

  echo "[restore] stopping mariadb (docker)..."
  docker_compose -f "$COMPOSE_FILE" stop mariadb >/dev/null 2>&1 || true
  docker_compose -f "$COMPOSE_FILE" rm -f mariadb >/dev/null 2>&1 || true

  if [ -z "$MARIADB_VOLUME_DIR" ] || [ "$MARIADB_VOLUME_DIR" = "/" ]; then
    echo "[error] unsafe mariadb volume dir: '$MARIADB_VOLUME_DIR'" >&2
    exit 1
  fi

  echo "[restore] removing mariadb data dir..."
  rm -rf "$MARIADB_VOLUME_DIR"

  echo "[restore] starting mariadb (docker)..."
  if docker_compose -f "$COMPOSE_FILE" up --help 2>/dev/null | grep -q -- "--wait"; then
    docker_compose -f "$COMPOSE_FILE" up -d --wait mariadb
  else
    docker_compose -f "$COMPOSE_FILE" up -d mariadb
  fi

  wait_for_port 127.0.0.1 15306 "MariaDB"

  echo "[restore] importing backup..."
  cid="$(docker_compose -f "$COMPOSE_FILE" ps -q mariadb)"
  if [ -z "$cid" ]; then
    echo "[error] mariadb container not found after startup" >&2
    exit 1
  fi

  # Ensure database exists
  docker exec "$cid" mysql -uroot -proot123456 \
    -e "CREATE DATABASE IF NOT EXISTS crypto_wallet DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" \
    2>/dev/null || true

  # Import backup
  if gzip -dc "$backup_file" | docker exec -i "$cid" mysql -uroot -proot123456 crypto_wallet; then
    echo "[restore] restore complete"
  else
    echo "[error] restore failed" >&2
    exit 1
  fi
}

start_bg() {
  cleanup_app_ports || true

  load_admin_env_if_present
  render_gateway_local_yaml

  : >"$LOG_DIR/swap-rpc.log"
  : >"$LOG_DIR/accounting-rpc.log"
  : >"$LOG_DIR/business-rpc.log"
  : >"$LOG_DIR/admin-rpc.log"
  : >"$LOG_DIR/chainrpc-rpc.log"
  : >"$LOG_DIR/chainsync-rpc.log"
  : >"$LOG_DIR/api-gateway.log"
  : >"$LOG_DIR/admin-web.log"

  if ! command -v air >/dev/null 2>&1; then
    echo "[error] air not found. Install: go install github.com/air-verse/air@latest" >&2
    exit 1
  fi
  if ! command -v bun >/dev/null 2>&1; then
    echo "[error] bun not found. Install bun first: https://bun.sh" >&2
    exit 1
  fi
  if [ ! -d "$PROJECT_ROOT/web/admin/node_modules" ] || [ ! -x "$PROJECT_ROOT/web/admin/node_modules/.bin/cross-env" ]; then
    echo "[error] admin web deps not installed (missing web/admin/node_modules or cross-env)." >&2
    echo "        Fix: cd \"$PROJECT_ROOT/web/admin\" && bun install" >&2
    stop_bg || true
    exit 1
  fi

  echo "[app] starting swap-rpc (air)..."
  (
    cd "$PROJECT_ROOT/services/swap/rpc"
    nohup air -c .air.local.toml >"$LOG_DIR/swap-rpc.log" 2>&1 &
    echo $! >"$PID_DIR/swap-rpc.pid"
  )

  if ! wait_for_port 127.0.0.1 9014 "swap-rpc"; then
    echo "[error] swap-rpc failed to start. Logs: $LOG_DIR/swap-rpc.log" >&2
    tail -n 200 "$LOG_DIR/swap-rpc.log" || true
    stop_bg || true
    exit 1
  fi

  echo "[app] starting accounting-rpc (air)..."
  (
    cd "$PROJECT_ROOT/services/accounting/rpc"
    nohup air -c .air.local.toml >"$LOG_DIR/accounting-rpc.log" 2>&1 &
    echo $! >"$PID_DIR/accounting-rpc.pid"
  )

  if ! wait_for_port 127.0.0.1 9015 "accounting-rpc"; then
    echo "[error] accounting-rpc failed to start. Logs: $LOG_DIR/accounting-rpc.log" >&2
    tail -n 200 "$LOG_DIR/accounting-rpc.log" || true
    stop_bg || true
    exit 1
  fi

  echo "[app] starting business-rpc (air)..."
  (
    cd "$PROJECT_ROOT/services/business/rpc"
    nohup air -c .air.local.toml >"$LOG_DIR/business-rpc.log" 2>&1 &
    echo $! >"$PID_DIR/business-rpc.pid"
  )

  if ! wait_for_port 127.0.0.1 8081 "business-rpc"; then
    echo "[error] business-rpc failed to start. Logs: $LOG_DIR/business-rpc.log" >&2
    tail -n 200 "$LOG_DIR/business-rpc.log" || true
    stop_bg || true
    exit 1
  fi

  echo "[app] starting admin-rpc (air)..."
  (
    cd "$PROJECT_ROOT/services/admin/rpc"
    nohup air -c .air.local.toml >"$LOG_DIR/admin-rpc.log" 2>&1 &
    echo $! >"$PID_DIR/admin-rpc.pid"
  )

  if ! wait_for_port 127.0.0.1 9013 "admin-rpc"; then
    echo "[error] admin-rpc failed to start. Logs: $LOG_DIR/admin-rpc.log" >&2
    tail -n 200 "$LOG_DIR/admin-rpc.log" || true
    stop_bg || true
    exit 1
  fi

  echo "[app] starting chainrpc-rpc (air)..."
  (
    cd "$PROJECT_ROOT/services/chainrpc/rpc"
    nohup air -c .air.local.toml >"$LOG_DIR/chainrpc-rpc.log" 2>&1 &
    echo $! >"$PID_DIR/chainrpc-rpc.pid"
  )

  if ! wait_for_port 127.0.0.1 9005 "chainrpc-rpc"; then
    echo "[error] chainrpc-rpc failed to start. Logs: $LOG_DIR/chainrpc-rpc.log" >&2
    tail -n 200 "$LOG_DIR/chainrpc-rpc.log" || true
    stop_bg || true
    exit 1
  fi

  echo "[app] starting chainsync-rpc (air)..."
  (
    cd "$PROJECT_ROOT/services/chainsync/rpc"
    nohup air -c .air.local.toml >"$LOG_DIR/chainsync-rpc.log" 2>&1 &
    echo $! >"$PID_DIR/chainsync-rpc.pid"
  )

  if ! wait_for_port 127.0.0.1 9001 "chainsync-rpc"; then
    echo "[error] chainsync-rpc failed to start. Logs: $LOG_DIR/chainsync-rpc.log" >&2
    tail -n 200 "$LOG_DIR/chainsync-rpc.log" || true
    stop_bg || true
    exit 1
  fi

  echo "[app] starting api-gateway (air)..."
  (
    cd "$PROJECT_ROOT/api-gateway"
    GATEWAY_SWAGGER_UI_URL="${GATEWAY_SWAGGER_UI_URL:-http://localhost:18080}" \
      nohup air -c .air.local.toml >"$LOG_DIR/api-gateway.log" 2>&1 &
    echo $! >"$PID_DIR/api-gateway.pid"
  )

  if ! wait_for_port 127.0.0.1 8080 "api-gateway"; then
    echo "[error] api-gateway failed to start. Logs: $LOG_DIR/api-gateway.log" >&2
    tail -n 200 "$LOG_DIR/api-gateway.log" || true
    stop_bg || true
    exit 1
  fi

  echo "[app] starting admin web (bun dev)..."
  (
    cd "$PROJECT_ROOT/web/admin"
    INTERNAL_WALLET_GATEWAY_URL="${INTERNAL_WALLET_GATEWAY_URL:-http://localhost:8080}" \
      nohup bun dev >"$LOG_DIR/admin-web.log" 2>&1 &
    echo $! >"$PID_DIR/admin-web.pid"
  )

  if ! wait_for_port 127.0.0.1 8000 "admin-web"; then
    echo "[error] admin web failed to start (port 8000 not listening). Logs: $LOG_DIR/admin-web.log" >&2
    tail -n 200 "$LOG_DIR/admin-web.log" || true
    stop_bg || true
    exit 1
  fi

  echo "[app] started"
  echo ""
  echo "URLs:"
  echo "  - Admin web:        http://localhost:8000"
  echo "  - API gateway:      http://localhost:8080"
  echo "  - Swagger (gateway):http://localhost:8080/docs/"
  echo "  - Business RPC:     127.0.0.1:8081 (gRPC)"
  echo "  - ChainRPC RPC:     127.0.0.1:9005 (gRPC)"
  echo "  - ChainSync RPC:    127.0.0.1:9001 (gRPC)"
  echo "  - Swap RPC:         127.0.0.1:9014 (gRPC)"
  echo "  - Accounting RPC:   127.0.0.1:9015 (gRPC)"
  echo ""
  echo "Logs:"
  echo "  - Swap RPC:         $LOG_DIR/swap-rpc.log"
  echo "  - Accounting RPC:   $LOG_DIR/accounting-rpc.log"
  echo "  - Business RPC:     $LOG_DIR/business-rpc.log"
  echo "  - Admin RPC:        $LOG_DIR/admin-rpc.log"
  echo "  - ChainRPC RPC:     $LOG_DIR/chainrpc-rpc.log"
  echo "  - ChainSync RPC:    $LOG_DIR/chainsync-rpc.log"
  echo "  - API gateway:      $LOG_DIR/api-gateway.log"
  echo "  - Admin web:        $LOG_DIR/admin-web.log"
  echo ""
  echo "Tail admin logs:"
  echo "  ./scripts/dev-admin.sh logs-admin"
}

stop_bg() {
  local stopped=0
  for name in admin-web api-gateway chainsync-rpc chainrpc-rpc admin-rpc business-rpc accounting-rpc swap-rpc; do
    local pid_file="$PID_DIR/$name.pid"
    if [ -f "$pid_file" ]; then
      local pid
      pid="$(cat "$pid_file")"
      if kill -0 "$pid" >/dev/null 2>&1; then
        echo "[stop] $name (pid $pid)"
        kill "$pid" >/dev/null 2>&1 || true
        stopped=$((stopped + 1))
      fi
      rm -f "$pid_file"
    fi
  done
  if [ "$stopped" -eq 0 ]; then
    echo "[stop] no background app processes found"
  fi

  cleanup_app_ports || true
}

tmux_up() {
  if ! command -v tmux >/dev/null 2>&1; then
    return 1
  fi
  if ! command -v air >/dev/null 2>&1; then
    echo "[error] air not found. Install: go install github.com/air-verse/air@latest" >&2
    exit 1
  fi
  if ! command -v bun >/dev/null 2>&1; then
    echo "[error] bun not found. Install bun first: https://bun.sh" >&2
    exit 1
  fi

  : >"$LOG_DIR/admin-rpc.log"
  : >"$LOG_DIR/api-gateway.log"
  : >"$LOG_DIR/admin-web.log"
  : >"$LOG_DIR/business-rpc.log"
  : >"$LOG_DIR/swap-rpc.log"
  : >"$LOG_DIR/accounting-rpc.log"
  : >"$LOG_DIR/chainrpc-rpc.log"
  : >"$LOG_DIR/chainsync-rpc.log"

  if tmux has-session -t "$TMUX_SESSION" >/dev/null 2>&1; then
    echo "[tmux] attaching existing session: $TMUX_SESSION"
    tmux attach -t "$TMUX_SESSION"
    return 0
  fi

  cleanup_app_ports || true

  load_admin_env_if_present
  render_gateway_local_yaml

  echo "[tmux] starting session: $TMUX_SESSION"
  tmux new-session -d -s "$TMUX_SESSION" -n dev -c "$PROJECT_ROOT"

  # Window: swap-rpc
  tmux new-window -t "$TMUX_SESSION" -n swap -c "$PROJECT_ROOT/services/swap/rpc"
  tmux send-keys -t "$TMUX_SESSION:swap.0" "cd \"$PROJECT_ROOT/services/swap/rpc\" && air -c .air.local.toml 2>&1 | tee \"$LOG_DIR/swap-rpc.log\"" C-m

  # Window: accounting-rpc
  tmux new-window -t "$TMUX_SESSION" -n accounting -c "$PROJECT_ROOT/services/accounting/rpc"
  tmux send-keys -t "$TMUX_SESSION:accounting.0" "cd \"$PROJECT_ROOT/services/accounting/rpc\" && air -c .air.local.toml 2>&1 | tee \"$LOG_DIR/accounting-rpc.log\"" C-m

  # Window: chainrpc-rpc
  tmux new-window -t "$TMUX_SESSION" -n chainrpc -c "$PROJECT_ROOT/services/chainrpc/rpc"
  tmux send-keys -t "$TMUX_SESSION:chainrpc.0" "cd \"$PROJECT_ROOT/services/chainrpc/rpc\" && air -c .air.local.toml 2>&1 | tee \"$LOG_DIR/chainrpc-rpc.log\"" C-m

  # Window: chainsync-rpc
  tmux new-window -t "$TMUX_SESSION" -n chainsync -c "$PROJECT_ROOT/services/chainsync/rpc"
  tmux send-keys -t "$TMUX_SESSION:chainsync.0" "cd \"$PROJECT_ROOT/services/chainsync/rpc\" && air -c .air.local.toml 2>&1 | tee \"$LOG_DIR/chainsync-rpc.log\"" C-m

  # Pane 0: admin-rpc
  tmux send-keys -t "$TMUX_SESSION:dev.0" "bash -lc 'set -e; for _ in \\$(seq 1 60); do if command -v nc >/dev/null 2>&1; then nc -z 127.0.0.1 9015 >/dev/null 2>&1 && break; else (echo >/dev/tcp/127.0.0.1/9015) >/dev/null 2>&1 && break; fi; sleep 0.5; done; cd \"$PROJECT_ROOT/services/admin/rpc\" && air -c .air.local.toml 2>&1 | tee \"$LOG_DIR/admin-rpc.log\"'" C-m

  # Pane 1: api-gateway (right column)
  tmux split-window -h -t "$TMUX_SESSION:dev.0" -c "$PROJECT_ROOT/api-gateway"

  # Pane 2: business-rpc (bottom left)
  tmux split-window -v -t "$TMUX_SESSION:dev.0" -c "$PROJECT_ROOT/services/business/rpc"
  tmux send-keys -t "$TMUX_SESSION:dev.2" "cd \"$PROJECT_ROOT/services/business/rpc\" && air -c .air.local.toml 2>&1 | tee \"$LOG_DIR/business-rpc.log\"" C-m

  # Pane 3: admin web (bottom right)
  tmux split-window -v -t "$TMUX_SESSION:dev.1" -c "$PROJECT_ROOT/web/admin"
  tmux send-keys -t "$TMUX_SESSION:dev.3" "cd \"$PROJECT_ROOT/web/admin\" && INTERNAL_WALLET_GATEWAY_URL=\"${INTERNAL_WALLET_GATEWAY_URL:-http://localhost:8080}\" bun dev 2>&1 | tee \"$LOG_DIR/admin-web.log\"" C-m

  # Pane 1 command: api-gateway (wait for required RPC services so routes register)
  tmux send-keys -t "$TMUX_SESSION:dev.1" "bash -lc 'set -e; if [ -f \"$ENV_FILE_ADMIN\" ]; then set -a; . \"$ENV_FILE_ADMIN\"; set +a; fi; : \"\${S3_ENABLED:=false}\"; : \"\${S3_USE_PATH_STYLE:=false}\"; : \"\${S3_PREFIX:=admin}\"; export S3_ENABLED S3_USE_PATH_STYLE S3_PREFIX; for _ in \$(seq 1 60); do if command -v nc >/dev/null 2>&1; then nc -z 127.0.0.1 8081 >/dev/null 2>&1 && nc -z 127.0.0.1 9013 >/dev/null 2>&1 && nc -z 127.0.0.1 9005 >/dev/null 2>&1 && nc -z 127.0.0.1 9001 >/dev/null 2>&1 && break; else (echo >/dev/tcp/127.0.0.1/8081) >/dev/null 2>&1 && (echo >/dev/tcp/127.0.0.1/9013) >/dev/null 2>&1 && (echo >/dev/tcp/127.0.0.1/9005) >/dev/null 2>&1 && (echo >/dev/tcp/127.0.0.1/9001) >/dev/null 2>&1 && break; fi; sleep 0.5; done; GATEWAY_SWAGGER_UI_URL=\"${GATEWAY_SWAGGER_UI_URL:-http://localhost:18080}\" air -c .air.local.toml 2>&1 | tee \"$LOG_DIR/api-gateway.log\"'" C-m

  tmux select-pane -t "$TMUX_SESSION:dev.0"
  tmux attach -t "$TMUX_SESSION"
  return 0
}

tmux_stop() {
  if command -v tmux >/dev/null 2>&1 && tmux has-session -t "$TMUX_SESSION" >/dev/null 2>&1; then
    echo "[tmux] stopping session: $TMUX_SESSION"
    tmux kill-session -t "$TMUX_SESSION"
    cleanup_app_ports || true
    return 0
  fi
  return 1
}

usage() {
  cat <<EOF
Usage:
  ./scripts/dev-admin.sh infra-up
  ./scripts/dev-admin.sh infra-down
  ./scripts/dev-admin.sh reset-db [--force] [--stop-app]
  ./scripts/dev-admin.sh restore-db [backup-file] [--force] [--stop-app]
  ./scripts/dev-admin.sh migrate-db
  ./scripts/dev-admin.sh up [--bg]
  ./scripts/dev-admin.sh restart [--bg]
  ./scripts/dev-admin.sh stop
  ./scripts/dev-admin.sh down
  ./scripts/dev-admin.sh logs-admin

Commands:
  reset-db     Delete MariaDB data and re-initialize (auto-backup, keep 5 recent)
  restore-db   Restore from backup (auto-backup before restore)
               If no file specified, shows interactive selection menu
EOF
}

do_up() {
  local mode="${1:-}"
  # Keep DB schema in sync with repo `deploy/docker/init-db/*.sql` on local dev.
  # Set DEV_ADMIN_SKIP_DB_MIGRATE=1 to skip automatic migrations.
  if [ "${DEV_ADMIN_SKIP_DB_MIGRATE:-}" = "1" ] || [ "${DEV_ADMIN_SKIP_DB_MIGRATE:-}" = "true" ]; then
    infra_up
  else
    migrate_db
  fi
  if [ "$mode" = "--bg" ]; then
    start_bg
    return 0
  fi
  if ! tmux_up; then
    start_bg
  fi
}

cmd="${1:-}"
mode="${2:-}"

case "$cmd" in
  infra-up)
    infra_up
    ;;
  infra-down)
    infra_down
    ;;
  reset-db)
    reset_db "${@:2}"
    ;;
  restore-db)
    restore_db "${@:2}"
    ;;
  migrate-db)
    migrate_db
    ;;
  up|"")
    do_up "$mode"
    ;;
  restart)
    tmux_stop || true
    stop_bg || true
    do_up "$mode"
    ;;
  stop)
    tmux_stop || true
    stop_bg || true
    ;;
  down)
    tmux_stop || true
    stop_bg || true
    infra_down
    ;;
  logs-admin)
    if [ ! -f "$LOG_DIR/admin-rpc.log" ]; then
      echo "[error] log file not found: $LOG_DIR/admin-rpc.log" >&2
      exit 1
    fi
    tail -n 200 -f "$LOG_DIR/admin-rpc.log"
    ;;
  *)
    usage
    exit 1
    ;;
esac
