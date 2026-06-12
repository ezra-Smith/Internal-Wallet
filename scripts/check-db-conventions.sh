#!/usr/bin/env bash
set -euo pipefail

GO_ROOTS=(common services api-gateway pkg scripts)
SQL_ROOTS=(deploy)
PROTO_ROOTS=(proto)
WEB_ROOTS=(web/admin/src)
SWAGGER_ROOTS=(docs/swagger)

fail=0

check_rg() {
  local label="$1"
  local pattern="$2"
  shift 2

  local out
  out="$(rg -n --hidden --no-messages "$pattern" "$@" || true)"
  if [[ -n "$out" ]]; then
    echo "✗ $label"
    echo "$out"
    echo
    fail=1
  fi
}

pattern_backtick_deleted='`deleted`'

check_rg "Go: legacy timestamp column names (created_time/updated_time)" "\\bcreated_time\\b|\\bupdated_time\\b" --glob '*.go' "${GO_ROOTS[@]}"
check_rg "Go: legacy soft-delete flag usage (deleted = 0/1)" "\\bdeleted\\s*=\\s*[01]\\b" --glob '*.go' "${GO_ROOTS[@]}"

check_rg "SQL: legacy timestamp column names (created_time/updated_time)" "\\bcreated_time\\b|\\bupdated_time\\b" --glob '*.sql' "${SQL_ROOTS[@]}"
check_rg "SQL: legacy soft-delete flag column (deleted)" "$pattern_backtick_deleted" --glob '*.sql' "${SQL_ROOTS[@]}"

check_rg "Proto: legacy timestamp field names (created_time/updated_time)" "\\bcreated_time\\b|\\bupdated_time\\b" --glob '*.proto' "${PROTO_ROOTS[@]}"

check_rg "Web: legacy timestamp JSON keys (created_time/updated_time)" "\\bcreated_time\\b|\\bupdated_time\\b" --glob '*.ts' --glob '*.tsx' "${WEB_ROOTS[@]}"

check_rg "Swagger: legacy timestamp field names (created_time/updated_time)" "\\bcreated_time\\b|\\bupdated_time\\b" --glob '*.json' --glob '*.yaml' "${SWAGGER_ROOTS[@]}"

if [[ $fail -ne 0 ]]; then
  echo "Found legacy conventions. Please migrate to created_at/updated_at/deleted_at + gorm.DeletedAt."
  exit 1
fi

echo "✓ DB conventions OK (created_at/updated_at/deleted_at)"
