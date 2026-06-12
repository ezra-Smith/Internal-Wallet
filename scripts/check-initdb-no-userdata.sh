#!/usr/bin/env bash
set -euo pipefail

# Validate that init-db SQL does NOT contain user data, logs, or secrets.
#
# Usage:
#   bash scripts/check-initdb-no-userdata.sh [path]
#
# Default:
#   deploy/docker/init-db/01-crypto-wallet.sql

FILE="${1:-deploy/docker/init-db/01-crypto-wallet.sql}"

if [ ! -f "$FILE" ]; then
  echo "[check-initdb-no-userdata] ERROR: file not found: $FILE" >&2
  exit 2
fi

python3 - "$FILE" <<'PY'
import re
import sys
from pathlib import Path

path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8", errors="replace")

checks = [
    # Secrets
    (r"INSERT INTO\s+`master_seeds`", "must not insert master_seeds"),
    (r"INSERT INTO\s+`swap_svc_api_keys`", "must not insert swap_svc_api_keys"),
    # Admin accounts / PII
    (r"INSERT INTO\s+`admin_users`", "must not insert admin_users"),
    (r"INSERT INTO\s+`admin_user_role`", "must not insert admin_user_role"),
    (r"INSERT INTO\s+`admin_audit_logs`", "must not insert admin_audit_logs"),
    (r"INSERT INTO\s+`admin_login_logs`", "must not insert admin_login_logs"),
    # Users + security / verification
    (r"INSERT INTO\s+`users`", "must not insert users"),
    (r"INSERT INTO\s+`user_", "must not insert user_* tables"),
    (r"INSERT INTO\s+`verification_codes`", "must not insert verification_codes"),
    (r"INSERT INTO\s+`geetest_validation_log`", "must not insert geetest_validation_log"),
    # Runtime wallet/tx data
    (r"INSERT INTO\s+`wallet_", "must not insert wallet_* tables"),
    (r"INSERT INTO\s+`transactions`", "must not insert transactions"),
    (r"INSERT INTO\s+`transaction_logs`", "must not insert transaction_logs"),
    (r"INSERT INTO\s+`unconfirmed_transaction`", "must not insert unconfirmed_transaction"),
    # Base64 images commonly embedded in dev dumps
    (r"data:image/", "must not embed base64 images"),
    # Basic PII heuristics (keep strict; init-db should not contain these)
    (r"@", "must not contain email-like '@'"),
    (r"\+86", "must not contain CN phone prefix '+86'"),
]

bad = []
for pat, msg in checks:
    if re.search(pat, text, flags=re.IGNORECASE):
        bad.append((pat, msg))

if bad:
    print("[check-initdb-no-userdata] FAILED:", file=sys.stderr)
    for pat, msg in bad:
        print(f"  - {msg}: /{pat}/", file=sys.stderr)
    sys.exit(1)

print("[check-initdb-no-userdata] OK", file=sys.stderr)
PY

