#!/usr/bin/env bash

# Production DB migrator for Zink Wallet.
# - Executes SQL files from MIGRATIONS_DIR in a deterministic order.
# - Records applied migrations with checksum (immutable migrations).
# - Uses a MySQL named lock to prevent concurrent runs.
#
# IMPORTANT:
# - Do NOT print SQL contents.
# - Fail fast on checksum mismatch (old migrations must never be modified).

set -euo pipefail

log() {
  # shellcheck disable=SC2059
  printf '[db-migrator] %s\n' "$*" >&2
}

die() {
  log "ERROR: $*"
  exit 1
}

sql_escape() {
  # Escape single quotes for SQL string literal.
  # NOTE: migration filenames are expected to be ASCII, but keep this safe.
  local s="$1"
  s="${s//\'/\'\'}"
  printf '%s' "$s"
}

require_env() {
  local name="$1"
  if [ -z "${!name:-}" ]; then
    die "missing required env: ${name}"
  fi
}

require_env DB_HOST
require_env DB_PASSWORD

: "${DB_PORT:=3306}"
: "${DB_USER:=root}"
: "${DB_NAME:=crypto_wallet}"
: "${PROFILE:=production}"
: "${GIT_SHA:=unknown}"
: "${MIGRATIONS_DIR:=/app/migrations}"
: "${LOCK_NAME:=internal-wallet-db-migrate}"
: "${LOCK_TIMEOUT_SECONDS:=600}"
: "${MYSQL_CONNECT_TIMEOUT_SECONDS:=10}"
: "${MYSQL_MAX_WAIT_SECONDS:=300}"

mysql_base_args=(
  --protocol=tcp
  -h "$DB_HOST"
  -P "$DB_PORT"
  -u"$DB_USER"
  "-p${DB_PASSWORD}"
  --connect-timeout="$MYSQL_CONNECT_TIMEOUT_SECONDS"
  --binary-mode
)

mysql_exec() {
  # Usage: mysql_exec [db] <sql>
  local db="${1:-}"
  local sql="${2:-}"
  if [ -z "$db" ]; then
    mariadb "${mysql_base_args[@]}" -e "$sql"
  else
    mariadb "${mysql_base_args[@]}" "$db" -e "$sql"
  fi
}

mysql_query_scalar() {
  # Usage: mysql_query_scalar <db> <sql>
  local db="$1"
  local sql="$2"
  mariadb "${mysql_base_args[@]}" "$db" -N -s -e "$sql" | tr -d '\r'
}

wait_for_mysql() {
  local deadline=$((SECONDS + MYSQL_MAX_WAIT_SECONDS))
  while true; do
    if mysql_exec "" "SELECT 1" >/dev/null 2>&1; then
      return 0
    fi
    if [ "$SECONDS" -ge "$deadline" ]; then
      die "mysql is not reachable at ${DB_HOST}:${DB_PORT} after ${MYSQL_MAX_WAIT_SECONDS}s"
    fi
    sleep 2
  done
}

ensure_database() {
  mysql_exec "" "CREATE DATABASE IF NOT EXISTS \`${DB_NAME}\` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
}

ensure_migrations_table() {
  mysql_exec "$DB_NAME" "
    CREATE TABLE IF NOT EXISTS \`schema_migrations\` (
      \`id\` BIGINT NOT NULL AUTO_INCREMENT,
      \`filename\` VARCHAR(255) NOT NULL,
      \`checksum\` CHAR(64) NOT NULL,
      \`git_sha\` CHAR(40) NOT NULL,
      \`applied_at\` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
      PRIMARY KEY (\`id\`),
      UNIQUE KEY \`uk_schema_migrations_filename\` (\`filename\`),
      KEY \`idx_schema_migrations_applied_at\` (\`applied_at\`)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='DB schema migrations (immutable, append-only).'
  "
}

acquire_lock() {
  local got
  got="$(mysql_query_scalar "$DB_NAME" "SELECT GET_LOCK('$(sql_escape "$LOCK_NAME")', ${LOCK_TIMEOUT_SECONDS});" || true)"
  if [ "$got" != "1" ]; then
    die "failed to acquire lock '${LOCK_NAME}' within ${LOCK_TIMEOUT_SECONDS}s (got: ${got:-empty})"
  fi
}

release_lock() {
  # best-effort
  mysql_exec "$DB_NAME" "SELECT RELEASE_LOCK('$(sql_escape "$LOCK_NAME")');" >/dev/null 2>&1 || true
}

strip_utf8_bom_if_present() {
  # Reads file from $1 and writes to stdout, stripping a leading UTF-8 BOM if present.
  # We intentionally avoid printing file content elsewhere.
  local f="$1"
  local bom
  bom="$(dd if="$f" bs=1 count=3 2>/dev/null | od -An -tx1 | tr -d ' \n' || true)"
  if [ "$bom" = "efbbbf" ]; then
    dd if="$f" bs=1 skip=3 2>/dev/null
  else
    cat "$f"
  fi
}

should_skip_file() {
  local base="$1"

  # Production safety guards.
  if [ "$PROFILE" = "production" ] || [ "$PROFILE" = "pro" ]; then
    # 01-init.sql hardcodes a placeholder password; never run in production.
    if [ "$base" = "01-init.sql" ]; then
      return 0
    fi

    # Never run mock/demo data in production.
    shopt -s nocasematch
    if [[ "$base" == *mock* ]]; then
      shopt -u nocasematch
      return 0
    fi
    shopt -u nocasematch
  fi

  return 1
}

list_sql_files_deterministic() {
  if [ ! -d "$MIGRATIONS_DIR" ]; then
    die "migrations dir not found: ${MIGRATIONS_DIR}"
  fi

  # Deterministic order:
  # 1) Files with numeric prefix "NN-" sorted by numeric NN asc, then full basename asc
  # 2) Remaining *.sql files sorted by basename asc
  python3 - "$MIGRATIONS_DIR" <<'PY'
import os, re, sys

root = sys.argv[1]
items = []
for name in os.listdir(root):
  if not name.endswith(".sql"):
    continue
  path = os.path.join(root, name)
  if not os.path.isfile(path):
    continue
  m = re.match(r"^(\d+)-", name)
  if m:
    items.append((0, int(m.group(1)), name))
  else:
    items.append((1, 0, name))

for _, _, name in sorted(items, key=lambda x: (x[0], x[1], x[2])):
  print(os.path.join(root, name))
PY
}

checksum_file() {
  sha256sum "$1" | awk '{print $1}'
}

apply_file() {
  local file="$1"
  local base
  base="$(basename "$file")"

  if should_skip_file "$base"; then
    log "skip: $base"
    return 0
  fi

  local esc_base
  esc_base="$(sql_escape "$base")"

  local current_checksum
  current_checksum="$(checksum_file "$file")"

  local existing_checksum
  existing_checksum="$(mysql_query_scalar "$DB_NAME" "SELECT \`checksum\` FROM \`schema_migrations\` WHERE \`filename\`='${esc_base}' LIMIT 1;" || true)"

  if [ -n "$existing_checksum" ]; then
    if [ "$existing_checksum" != "$current_checksum" ]; then
      die "checksum mismatch for already-applied migration '${base}'. Refusing to continue. (You must add a new migration SQL; modifying old SQL is forbidden.)"
    fi
    log "ok (already applied): $base"
    return 0
  fi

  log "apply: $base"

  # Execute without echoing SQL.
  if ! strip_utf8_bom_if_present "$file" | mariadb "${mysql_base_args[@]}" "$DB_NAME" >/dev/null; then
    die "failed to apply migration: ${base}"
  fi

  mysql_exec "$DB_NAME" "INSERT INTO \`schema_migrations\` (\`filename\`, \`checksum\`, \`git_sha\`) VALUES ('${esc_base}', '$(sql_escape "$current_checksum")', '$(sql_escape "$GIT_SHA")');"
  log "applied: $base"
}

main() {
  log "starting (profile=${PROFILE}, db=${DB_NAME}, host=${DB_HOST}:${DB_PORT}, git_sha=${GIT_SHA})"

  wait_for_mysql
  ensure_database
  ensure_migrations_table

  acquire_lock
  trap release_lock EXIT

  local files=()
  while IFS= read -r f; do
    files+=("$f")
  done < <(list_sql_files_deterministic)

  if [ "${#files[@]}" -eq 0 ]; then
    die "no migrations found under: ${MIGRATIONS_DIR}"
  fi

  local applied_any=0
  for f in "${files[@]}"; do
    apply_file "$f"
    applied_any=1
  done

  if [ "$applied_any" -eq 0 ]; then
    log "no migrations executed"
  fi

  log "done"
}

main "$@"
