#!/usr/bin/env python3
"""
Audit AWS Secrets Manager entries required by production K8s manifests.

It parses:
  - deploy/k8s/overlays/production/secretproviderclass-wallet-secrets.yaml
  - deploy/k8s/overlays/production/external-secrets-infra.yaml

and verifies:
  1) each referenced secret exists
  2) each referenced JSON property exists and is non-empty (""/null)
  3) for ExternalSecret refs without `property`, secret string is non-empty

This script NEVER prints secret values.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Optional


@dataclass
class RequiredSecret:
    # JSON properties required to exist in SecretString JSON.
    json_properties: set[str] = field(default_factory=set)
    # If true, require SecretString to be present (even if not JSON).
    require_non_empty_secret_string: bool = False


def _run_aws(args: list[str], *, region: str, profile: Optional[str]) -> str:
    cmd = ["aws"]
    if profile:
        cmd += ["--profile", profile]
    cmd += args + ["--region", region]
    try:
        p = subprocess.run(
            cmd,
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
    except FileNotFoundError as e:
        raise RuntimeError("aws CLI not found in PATH") from e
    except subprocess.CalledProcessError as e:
        # Surface stderr (but not any secret data).
        raise RuntimeError(e.stderr.strip() or f"aws command failed: {' '.join(cmd)}") from e
    return p.stdout


def _strip_quotes(s: str) -> str:
    s = s.strip()
    if len(s) >= 2 and ((s[0] == s[-1] == '"') or (s[0] == s[-1] == "'")):
        return s[1:-1]
    return s


def parse_secretproviderclass_required(path: Path) -> dict[str, RequiredSecret]:
    """
    Extract (secretName -> required JSON properties) from SecretProviderClass `objects: |` blocks.
    We parse YAML text heuristically (no PyYAML dependency).
    """
    required: dict[str, RequiredSecret] = {}
    current_secret: Optional[str] = None

    obj_re = re.compile(r'^\s*-\s*objectName:\s*(.+)\s*$')
    path_re = re.compile(r'^\s*-\s*path:\s*([A-Za-z0-9_]+)\s*$')

    for line in path.read_text(encoding="utf-8", errors="ignore").splitlines():
        m_obj = obj_re.match(line)
        if m_obj:
            current_secret = _strip_quotes(m_obj.group(1))
            required.setdefault(current_secret, RequiredSecret())
            continue

        if current_secret:
            m_path = path_re.match(line)
            if m_path:
                required[current_secret].json_properties.add(m_path.group(1))

    return required


def parse_externalsecrets_required(path: Path) -> dict[str, RequiredSecret]:
    """
    Extract (secretName -> required JSON properties) from ExternalSecret `remoteRef` blocks.
    If `property` is omitted, we require SecretString to be non-empty.
    """
    required: dict[str, RequiredSecret] = {}

    key_re = re.compile(r"^\s*key:\s*(.+?)\s*$")
    prop_re = re.compile(r"^\s*property:\s*([A-Za-z0-9_]+)\s*$")
    item_start_re = re.compile(r"^\s*-\s*secretKey:\s*.+\s*$")

    pending_key: Optional[str] = None
    pending_prop: Optional[str] = None
    in_item = False

    def flush_pending() -> None:
        nonlocal pending_key, pending_prop, in_item
        if not pending_key:
            pending_key, pending_prop, in_item = None, None, False
            return
        s = required.setdefault(pending_key, RequiredSecret())
        if pending_prop:
            s.json_properties.add(pending_prop)
        else:
            s.require_non_empty_secret_string = True
        pending_key, pending_prop, in_item = None, None, False

    for line in path.read_text(encoding="utf-8", errors="ignore").splitlines():
        # new list item in `data:`
        if item_start_re.match(line):
            flush_pending()
            in_item = True
            continue

        m_key = key_re.match(line)
        if m_key:
            # If multiple remoteRef blocks are present, flush previous first.
            flush_pending()
            pending_key = _strip_quotes(m_key.group(1))
            pending_prop = None
            in_item = True
            continue

        if in_item and pending_key:
            m_prop = prop_re.match(line)
            if m_prop:
                pending_prop = m_prop.group(1)
                continue

        if line.strip() == "---":
            flush_pending()

    flush_pending()
    return required


def merge_required(a: dict[str, RequiredSecret], b: dict[str, RequiredSecret]) -> dict[str, RequiredSecret]:
    out = {k: RequiredSecret(set(v.json_properties), v.require_non_empty_secret_string) for k, v in a.items()}
    for k, v in b.items():
        s = out.setdefault(k, RequiredSecret())
        s.json_properties |= set(v.json_properties)
        s.require_non_empty_secret_string = s.require_non_empty_secret_string or v.require_non_empty_secret_string
    return out


def _is_missing_value(v, *, strict_non_empty: bool) -> bool:
    """
    Missing-value semantics:
    - Always treat `null` as missing.
    - If strict_non_empty=True, also treat empty string as missing.
    - 0/false are valid.
    """
    if v is None:
        return True
    if strict_non_empty and isinstance(v, str) and v == "":
        return True
    return False


def audit(required: dict[str, RequiredSecret], *, region: str, profile: Optional[str], strict_non_empty: bool) -> int:
    missing_secrets: list[str] = []
    missing_keys: dict[str, list[str]] = {}
    empty_keys: dict[str, list[str]] = {}
    invalid_json: list[str] = []
    empty_secret_string: list[str] = []

    for secret_name in sorted(required.keys()):
        # 1) existence
        try:
            _run_aws(["secretsmanager", "describe-secret", "--secret-id", secret_name], region=region, profile=profile)
        except RuntimeError:
            missing_secrets.append(secret_name)
            continue

        # 2) fetch SecretString (do not print)
        try:
            raw = _run_aws(
                ["secretsmanager", "get-secret-value", "--secret-id", secret_name, "--query", "SecretString", "--output", "text"],
                region=region,
                profile=profile,
            ).strip()
        except RuntimeError as e:
            missing_keys.setdefault(secret_name, []).append(f"(get-secret-value failed: {e})")
            continue

        # AWS returns "None" literal for absent SecretString in text output sometimes.
        if raw in ("", "None", "null"):
            if required[secret_name].require_non_empty_secret_string or required[secret_name].json_properties:
                empty_secret_string.append(secret_name)
            continue

        if required[secret_name].require_non_empty_secret_string and not raw:
            empty_secret_string.append(secret_name)

        if required[secret_name].json_properties:
            try:
                data = json.loads(raw)
            except Exception:
                invalid_json.append(secret_name)
                continue

            if not isinstance(data, dict):
                invalid_json.append(secret_name)
                continue

            miss: list[str] = []
            empty: list[str] = []
            for k in sorted(required[secret_name].json_properties):
                if k not in data:
                    miss.append(k)
                    continue
                v = data.get(k)
                if v is None:
                    miss.append(k)
                    continue
                if strict_non_empty and isinstance(v, str) and v == "":
                    empty.append(k)
            if miss:
                missing_keys[secret_name] = miss
            if empty:
                empty_keys[secret_name] = empty

    # Output summary
    print("=== Secrets Manager Required Secrets Audit ===")
    print(f"Region: {region}")
    if profile:
        print(f"Profile: {profile}")
    print("")

    exit_code = 0

    if missing_secrets:
        exit_code = 2
        print("MISSING SECRETS:")
        for s in missing_secrets:
            print(f"  - {s}")
        print("")

    if empty_secret_string:
        exit_code = 2
        print("EMPTY SecretString (expected non-empty):")
        for s in empty_secret_string:
            print(f"  - {s}")
        print("")

    if invalid_json:
        exit_code = 2
        print("INVALID JSON SecretString (expected JSON object):")
        for s in invalid_json:
            print(f"  - {s}")
        print("")

    if missing_keys:
        exit_code = 2
        print("MISSING JSON KEYS (absent or null):")
        for s in sorted(missing_keys.keys()):
            print(f"  - {s}: {', '.join(missing_keys[s])}")
        print("")

    if strict_non_empty and empty_keys:
        exit_code = 2
        print('EMPTY JSON KEYS (present but "" in --strict mode):')
        for s in sorted(empty_keys.keys()):
            print(f"  - {s}: {', '.join(empty_keys[s])}")
        print("")

    if exit_code == 0:
        print("OK: all required secrets and keys are present (no values printed).")

    return exit_code


def main() -> int:
    repo_root = Path(__file__).resolve().parents[1]
    p = argparse.ArgumentParser(description="Audit required AWS Secrets Manager secrets for production manifests.")
    p.add_argument("--region", default="ap-northeast-1", help="AWS region (default: ap-northeast-1)")
    p.add_argument("--profile", default=None, help="AWS profile name (optional)")
    p.add_argument(
        "--strict",
        action="store_true",
        help='Also treat empty string "" as missing (default: only checks key presence / null).',
    )
    p.add_argument(
        "--secretproviderclass",
        default=str(repo_root / "deploy/k8s/overlays/production/secretproviderclass-wallet-secrets.yaml"),
        help="Path to SecretProviderClass YAML",
    )
    p.add_argument(
        "--externalsecrets",
        default=str(repo_root / "deploy/k8s/overlays/production/external-secrets-infra.yaml"),
        help="Path to ExternalSecrets YAML",
    )
    args = p.parse_args()

    spc_path = Path(args.secretproviderclass)
    es_path = Path(args.externalsecrets)
    if not spc_path.exists():
        raise SystemExit(f"File not found: {spc_path}")
    if not es_path.exists():
        raise SystemExit(f"File not found: {es_path}")

    req_spc = parse_secretproviderclass_required(spc_path)
    req_es = parse_externalsecrets_required(es_path)
    required = merge_required(req_spc, req_es)
    return audit(required, region=args.region, profile=args.profile, strict_non_empty=bool(args.strict))


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        raise SystemExit(130)
