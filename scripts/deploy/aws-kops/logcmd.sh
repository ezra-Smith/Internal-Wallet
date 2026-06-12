#!/usr/bin/env bash
set -euo pipefail

LOG_FILE="docs/deploy/aws-kops/99-command-log.md"

if [ ! -f "$LOG_FILE" ]; then
  echo "ERROR: log file not found: $LOG_FILE" >&2
  exit 1
fi

if [ "${1:-}" != "--cmd" ] || [ -z "${2:-}" ]; then
  echo "Usage: $0 --cmd '<shell command>'" >&2
  exit 2
fi

CMD="$2"
TS_UTC="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

# Run command and capture output + exit code (do not inherit `set -e`).
set +e
OUT="$(bash -lc "$CMD" 2>&1)"
CODE=$?
set -e

# Best-effort guard: avoid logging obvious credential material.
# (Still: DO NOT run commands that print secrets.)
#
# Note: do NOT match generic words like "secret" to avoid false positives (k8s terminology).
if echo "$OUT" | grep -Eqi '("TunnelSecret"|TunnelSecret|Authorization:[[:space:]]*Bearer|gh[opsu]_[A-Za-z0-9_]{10,}|AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16}|-----BEGIN[[:space:]]+(RSA|EC|OPENSSH)[[:space:]]+PRIVATE[[:space:]]+KEY-----)'; then
  echo "ERROR: output appears to contain credential material; refusing to log. Re-run with a safe command/output." >&2
  exit 3
fi

{
  echo ""
  echo "## $TS_UTC"
  echo ""
  echo '```bash'
  echo "$CMD"
  echo '```'
  echo ""
  echo "Exit code: $CODE"
  echo ""
  echo "Output:"
  echo '```'
  echo "$OUT"
  echo '```'
} >> "$LOG_FILE"

printf '%s\n' "$OUT"
exit "$CODE"
