#!/usr/bin/env bash

# Enforce "immutable migrations" policy for `deploy/docker/init-db/*.sql`.
#
# Rules:
# - Existing SQL migrations MUST NOT be modified, deleted, or renamed.
# - Only adding NEW SQL files is allowed.
# - New SQL filenames MUST:
#   - be ASCII, lowercase, and match: ^[0-9]{2,4}-[a-z0-9][a-z0-9._-]*\\.sql$
#   - NOT contain "mock" (production safety)
#
# Usage:
#   ./scripts/check-db-migrations-immutable.sh --base <sha> --head <sha>

set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: ./scripts/check-db-migrations-immutable.sh --base <sha> --head <sha>
EOF
}

BASE=""
HEAD=""

while [ $# -gt 0 ]; do
  case "$1" in
    --base)
      BASE="${2:-}"
      shift 2
      ;;
    --head)
      HEAD="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "[immutable-migrations] unknown arg: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [ -z "$BASE" ] || [ -z "$HEAD" ]; then
  usage
  exit 2
fi

if ! git cat-file -e "$BASE^{commit}" 2>/dev/null; then
  echo "[immutable-migrations] base sha not found: $BASE" >&2
  exit 2
fi
if ! git cat-file -e "$HEAD^{commit}" 2>/dev/null; then
  echo "[immutable-migrations] head sha not found: $HEAD" >&2
  exit 2
fi

ROOT_DIR="deploy/docker/init-db"
BASELINE_ALLOW_MUTABLE="$ROOT_DIR/01-crypto-wallet.sql"

changes="$(git diff --name-status --find-renames --diff-filter=ACDMRT "$BASE" "$HEAD" -- "$ROOT_DIR" || true)"

if [ -z "$changes" ]; then
  echo "[immutable-migrations] no changes under $ROOT_DIR"
  exit 0
fi

fail=0
added_files=()

echo "[immutable-migrations] checking changes under $ROOT_DIR"
while IFS=$'\t' read -r status path1 path2; do
  # status can be like "R100"
  code="${status:0:1}"

  case "$code" in
    A)
      if [[ "$path1" == *.sql ]]; then
        added_files+=("$path1")
      fi
      ;;
    M|D|R|T)
      # Any change to existing migrations is forbidden.
      #
      # Exception:
      # - During rapid iteration, we treat the baseline init DB dump as regeneratable,
      #   so we allow MODIFY (M) for the baseline file only.
      if [[ "$code" == "M" ]] && [[ "$path1" == "$BASELINE_ALLOW_MUTABLE" ]]; then
        echo "[immutable-migrations] ALLOW (baseline mutable): $status $path1" >&2
        continue
      fi
      if [[ "$path1" == *.sql ]] || [[ "${path2:-}" == *.sql ]]; then
        echo "[immutable-migrations] FORBIDDEN: $status $path1 ${path2:-}" >&2
        fail=1
      fi
      ;;
    *)
      # Defensive: block any other change types affecting sql.
      if [[ "$path1" == *.sql ]] || [[ "${path2:-}" == *.sql ]]; then
        echo "[immutable-migrations] FORBIDDEN(change-type): $status $path1 ${path2:-}" >&2
        fail=1
      fi
      ;;
  esac
done <<<"$changes"

if [ "$fail" -ne 0 ]; then
  echo "[immutable-migrations] FAILED: existing migrations must never change; only new files may be added." >&2
  exit 1
fi

if [ "${#added_files[@]}" -eq 0 ]; then
  echo "[immutable-migrations] OK: no .sql additions (and no forbidden changes)"
  exit 0
fi

name_re='^[0-9]{2,4}-[a-z0-9][a-z0-9._-]*\.sql$'
for f in "${added_files[@]}"; do
  base="$(basename "$f")"

  # ASCII + lowercase filename policy for new migrations.
  if ! [[ "$base" =~ $name_re ]]; then
    echo "[immutable-migrations] INVALID filename: $base (expected: $name_re)" >&2
    exit 1
  fi

  if [[ "$base" == *mock* ]]; then
    echo "[immutable-migrations] INVALID filename: $base (mock/demo data is forbidden in production migrations)" >&2
    exit 1
  fi
done

echo "[immutable-migrations] OK: immutable migrations policy satisfied"
