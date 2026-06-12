#!/usr/bin/env python3
"""
Sanitize Navicat-style MariaDB dump SQL for init-db usage.

Goal:
- Keep ALL table structures (DDL).
- Drop ALL runtime/user/log/secret data inserts by default.
- Keep a minimal allowlist of seed/config/RBAC inserts required for infra to boot.
- Never keep secrets like master_seeds / swap_svc_api_keys records.

This script is intentionally deterministic and simple: it parses the dump line-by-line,
detects "Records of <table>" blocks, and either emits INSERT lines (possibly filtered)
or drops them.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path
from typing import Callable, Iterable


# Tables whose INSERTs are allowed to be kept as seed/config/RBAC data.
KEEP_INSERT_TABLES: set[str] = {
    # RBAC (keep menus/roles/permissions, but do NOT keep admin accounts)
    "admin_menu",
    "admin_permission",
    "admin_role",
    "admin_role_menu",
    "admin_role_permission",
    "admin_system_configs",
    # Core config
    "chain",
    "asset",
    "alert_configs",
    "blacklist_addresses",
    "language",
    "price_units",
    # Currency config
    "currency_settings",
    "currency_chain_settings",
    "currency_global_transfer_audit_rules",
    "currency_global_withdraw_audit_rules",
    "currency_global_withdraw_fee_rules",
    "currency_withdraw_audit_rules",
    "currency_withdraw_fee_rules",
    # Swap config
    "swap_provider",
    "swap_configs",
    # Vault config (no secrets, no addresses)
    "vault_networks",
    # Accounting base definitions (filtered)
    "acct_account_types",
    "acct_account_type_assets",
}


# Tables whose INSERTs MUST be dropped even if they look like config.
FORCE_DROP_TABLES: set[str] = {
    # Secrets
    "master_seeds",
    "swap_svc_api_keys",
    # Accounts / users
    "admin_users",
    "admin_user_role",
}


ALLOWED_ACCT_ACCOUNT_TYPE_CODES: set[str] = {
    "SYS_ADJUST_EXPENSE",
    "SYS_ADJUST_INCOME",
    "SYS_CAPITAL",
    "SYS_FEE_INCOME",
    "SYS_WALLET_HOT",
    "USER_LIABILITY",
}


INSERT_RE = re.compile(r"^INSERT INTO\s+`(?P<table>[^`]+)`\s+\(", re.IGNORECASE)
RECORDS_OF_RE = re.compile(r"^-- Records of (?P<table>\S+)\s*$")


def _acct_account_types_filter(line: str) -> bool:
    # INSERT INTO `acct_account_types` (...) VALUES ('SYS_CAPITAL', ...)
    # We keep only the canonical codes above.
    if "VALUES" not in line:
        return False
    m = re.search(r"VALUES\s*\(\s*'([^']+)'", line)
    if not m:
        return False
    code = m.group(1)
    return code in ALLOWED_ACCT_ACCOUNT_TYPE_CODES


def _acct_account_type_assets_filter(line: str) -> bool:
    # INSERT INTO `acct_account_type_assets` (...) VALUES (..., 'SYS_CAPITAL', 'USDT', ...)
    if "VALUES" not in line:
        return False
    # account_type_code is the 2nd value in this dump.
    m = re.search(r"VALUES\s*\(\s*[^,]+,\s*'([^']+)'", line)
    if not m:
        return False
    code = m.group(1)
    return code in ALLOWED_ACCT_ACCOUNT_TYPE_CODES


FILTERS: dict[str, Callable[[str], bool]] = {
    "acct_account_types": _acct_account_types_filter,
    "acct_account_type_assets": _acct_account_type_assets_filter,
}


def sanitize_lines(lines: Iterable[str]) -> list[str]:
    out: list[str] = []

    in_records = False
    current_records_table: str | None = None

    for raw in lines:
        line = raw.rstrip("\n")

        m_records = RECORDS_OF_RE.match(line)
        if m_records:
            in_records = True
            current_records_table = m_records.group("table").strip()
            out.append(line + "\n")
            continue

        if in_records and line == "COMMIT;":
            # End of records block.
            in_records = False
            current_records_table = None
            out.append(line + "\n")
            continue

        if in_records:
            # Always keep the transaction wrapper lines, even if empty.
            if line == "BEGIN;" or line.strip() == "":
                out.append(line + "\n")
                continue

            m_ins = INSERT_RE.match(line)
            if not m_ins:
                # Any other lines inside records block are preserved (rare).
                out.append(line + "\n")
                continue

            table = m_ins.group("table")

            # Hard drops first.
            if table in FORCE_DROP_TABLES:
                continue

            # Default drop unless explicitly allowlisted.
            if table not in KEEP_INSERT_TABLES:
                continue

            # Apply per-table filters (to drop obvious dev/test rows).
            f = FILTERS.get(table)
            if f is not None and not f(line):
                continue

            out.append(line + "\n")
            continue

        # Outside records blocks, keep everything (DDL + headers).
        out.append(line + "\n")

    return out


def main() -> int:
    p = argparse.ArgumentParser(description="Sanitize init-db SQL dump (remove user/runtime/secrets).")
    p.add_argument("--in", dest="in_path", required=True, help="Input SQL file")
    p.add_argument("--out", dest="out_path", required=True, help="Output SQL file")
    args = p.parse_args()

    in_path = Path(args.in_path)
    out_path = Path(args.out_path)

    src = in_path.read_text(encoding="utf-8", errors="replace").splitlines(keepends=True)
    sanitized = sanitize_lines(src)

    # Safety check: ensure we didn't keep secret-row INSERTs.
    #
    # NOTE: We intentionally do NOT forbid column names like `seed_encrypted` / `token_display`
    # appearing in DDL, because we keep all table structures.
    joined = "".join(sanitized)
    forbidden_insert_patterns = [
        r"INSERT INTO\s+`master_seeds`",
        r"INSERT INTO\s+`swap_svc_api_keys`",
    ]
    for pat in forbidden_insert_patterns:
        if re.search(pat, joined, flags=re.IGNORECASE):
            raise SystemExit(f"Refusing to write output: forbidden INSERT matched: {pat}")

    out_path.write_text("".join(sanitized), encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

