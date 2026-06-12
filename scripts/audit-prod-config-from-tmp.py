#!/usr/bin/env python3

from __future__ import annotations

import argparse
import dataclasses
import re
import sys
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Tuple

import yaml


ROOT = Path(__file__).resolve().parents[1]

TMP_BASELINE_GLOB = "tmp/**/etc/*.yaml"
PROD_CONFIG_DIR = ROOT / "deploy/k8s/base/config/files"
KUSTOMIZATION_FILE = ROOT / "deploy/k8s/base/config/kustomization.yaml"
OVERLAY_ENV_PATCH = ROOT / "deploy/k8s/overlays/production/patches/app-env.yaml"
SPC_FILE = ROOT / "deploy/k8s/overlays/production/secretproviderclass-wallet-secrets.yaml"
EXTERNAL_SECRETS_FILE = ROOT / "deploy/k8s/overlays/production/external-secrets-infra.yaml"


def _load_yaml_docs(path: Path) -> List[dict]:
    text = path.read_text(encoding="utf-8")
    docs = []
    for doc in yaml.safe_load_all(text):
        if doc is None:
            continue
        if not isinstance(doc, dict):
            continue
        docs.append(doc)
    return docs


def _load_yaml_single(path: Path) -> dict:
    doc = yaml.safe_load(path.read_text(encoding="utf-8"))
    if doc is None:
        return {}
    if not isinstance(doc, dict):
        raise TypeError(f"expected dict yaml at {path}, got {type(doc)}")
    return doc


def _add_flat(out: Dict[str, List[Any]], key: str, value: Any) -> None:
    if key not in out:
        out[key] = []
    out[key].append(value)


def _flatten_yaml(obj: Any, prefix: str = "", out: Optional[Dict[str, List[Any]]] = None) -> Dict[str, List[Any]]:
    if out is None:
        out = {}

    if isinstance(obj, dict):
        for k, v in obj.items():
            k_str = str(k)
            next_prefix = f"{prefix}.{k_str}" if prefix else k_str
            _flatten_yaml(v, next_prefix, out)
        return out

    if isinstance(obj, list):
        if not obj:
            _add_flat(out, prefix, obj)
            return out
        for item in obj:
            next_prefix = f"{prefix}[*]" if prefix else "[*]"
            _flatten_yaml(item, next_prefix, out)
        return out

    _add_flat(out, prefix, obj)
    return out


def _is_filled(value: Any) -> bool:
    if value is None:
        return False
    if isinstance(value, str):
        s = value.strip()
        if s == "":
            return False
        if s.upper() in {"CHANGE_ME", "TODO"}:
            return False
        return True
    if isinstance(value, (bool, int, float)):
        return True
    if isinstance(value, (list, dict)):
        return len(value) > 0
    return True


def _values_filled(values: Iterable[Any]) -> bool:
    return any(_is_filled(v) for v in values)


def _parse_wallet_biz_env() -> Dict[str, str]:
    env: Dict[str, str] = {}

    if KUSTOMIZATION_FILE.exists():
        ks = _load_yaml_single(KUSTOMIZATION_FILE)
        for gen in ks.get("configMapGenerator", []) or []:
            if not isinstance(gen, dict):
                continue
            if gen.get("name") != "wallet-biz-env":
                continue
            for lit in gen.get("literals", []) or []:
                if not isinstance(lit, str) or "=" not in lit:
                    continue
                k, v = lit.split("=", 1)
                env[k] = v

    # Apply overlay patch override if present.
    if OVERLAY_ENV_PATCH.exists():
        for doc in _load_yaml_docs(OVERLAY_ENV_PATCH):
            if doc.get("kind") != "ConfigMap":
                continue
            md = doc.get("metadata") or {}
            if md.get("name") != "wallet-biz-env":
                continue
            data = doc.get("data") or {}
            if isinstance(data, dict):
                for k, v in data.items():
                    if isinstance(v, (str, int, float, bool)) or v is None:
                        env[str(k)] = "" if v is None else str(v)

    return env


@dataclasses.dataclass(frozen=True)
class CsiAlias:
    spc_name: str
    alias: str


@dataclasses.dataclass(frozen=True)
class Injection:
    kind: str  # csi|env|externalsecret|not_applicable
    details: Any


def _parse_secretproviderclasses() -> Tuple[Dict[str, Dict[str, dict]], Dict[str, set]]:
    # Returns:
    # - meta[spc_name][alias] -> {objectName, property}
    # - aliases_by_spc[spc_name] -> set(alias)
    meta: Dict[str, Dict[str, dict]] = {}
    aliases_by_spc: Dict[str, set] = {}

    if not SPC_FILE.exists():
        return meta, aliases_by_spc

    for doc in _load_yaml_docs(SPC_FILE):
        if doc.get("kind") != "SecretProviderClass":
            continue
        md = doc.get("metadata") or {}
        spc_name = md.get("name")
        if not spc_name:
            continue
        params = (((doc.get("spec") or {}).get("parameters") or {}))
        objects_str = params.get("objects", "")
        if not isinstance(objects_str, str) or objects_str.strip() == "":
            continue
        try:
            objects = yaml.safe_load(objects_str) or []
        except Exception:
            objects = []

        aliases: set = set()
        spc_meta: Dict[str, dict] = {}
        if isinstance(objects, list):
            for obj in objects:
                if not isinstance(obj, dict):
                    continue
                object_name = obj.get("objectName", "")
                for jp in obj.get("jmesPath", []) or []:
                    if not isinstance(jp, dict):
                        continue
                    alias = jp.get("objectAlias")
                    prop = jp.get("path")
                    if not alias:
                        continue
                    aliases.add(alias)
                    spc_meta[alias] = {"objectName": object_name, "property": prop}
        meta[spc_name] = spc_meta
        aliases_by_spc[spc_name] = aliases

    return meta, aliases_by_spc


def _parse_externalsecrets() -> Dict[str, dict]:
    # secretKey -> {remoteKey, remoteProperty, externalSecretName, namespace}
    out: Dict[str, dict] = {}
    if not EXTERNAL_SECRETS_FILE.exists():
        return out
    for doc in _load_yaml_docs(EXTERNAL_SECRETS_FILE):
        if doc.get("kind") != "ExternalSecret":
            continue
        md = doc.get("metadata") or {}
        es_name = md.get("name", "")
        ns = md.get("namespace", "")
        spec = doc.get("spec") or {}
        for item in spec.get("data", []) or []:
            if not isinstance(item, dict):
                continue
            sk = item.get("secretKey")
            rr = item.get("remoteRef") or {}
            if not sk or not isinstance(rr, dict):
                continue
            out[str(sk)] = {
                "externalSecret": es_name,
                "namespace": ns,
                "remoteKey": rr.get("key", ""),
                "remoteProperty": rr.get("property", ""),
            }
    return out


def _detect_etcd_replacement(prod_flat: Dict[str, List[Any]], prefix: str) -> bool:
    # prefix == "" means top-level
    endpoints_key = "Endpoints[*]" if prefix == "" else f"{prefix}.Endpoints[*]"
    return endpoints_key in prod_flat


def _resolve_injection(file_name: str, yaml_path: str, baseline_obj: dict) -> Optional[Injection]:
    # NOTE: this resolver is intentionally conservative and only covers the
    # known production injection sources used by this repo.

    # 0) Explicit not-applicable (IRSA replaces static creds)
    if file_name == "gateway.yaml" and yaml_path in {"S3.AccessKeyID", "S3.SecretAccessKey", "S3.SessionToken"}:
        return Injection(kind="not_applicable", details={"reason": "S3 uses IRSA in production; static credentials not used."})

    # 1) Common DB/Redis secrets
    if yaml_path == "MySQL.Password":
        spc = "wallet-signer-secrets" if file_name == "signer.yaml" else "wallet-app-secrets"
        return Injection(kind="csi", details=[CsiAlias(spc_name=spc, alias="mysql_password")])
    if yaml_path in {"Redis.Password", "CacheRedis[*].Pass"}:
        spc = "wallet-signer-secrets" if file_name == "signer.yaml" else "wallet-app-secrets"
        return Injection(kind="csi", details=[CsiAlias(spc_name=spc, alias="redis_password")])

    # 2) JWT secrets
    if file_name == "gateway.yaml" and yaml_path == "Auth.AccessSecret":
        return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", "jwt_access_secret")])
    if file_name == "gateway.yaml" and yaml_path == "Auth.RefreshSecret":
        return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", "jwt_refresh_secret")])
    if file_name == "gateway.yaml" and yaml_path == "AdminAuth.AccessSecret":
        return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", "admin_jwt_access_secret")])
    if file_name == "gateway.yaml" and yaml_path == "AdminAuth.RefreshSecret":
        return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", "admin_jwt_refresh_secret")])

    if file_name == "business.yaml" and yaml_path == "JWT.AccessSecret":
        return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", "jwt_access_secret")])
    if file_name == "business.yaml" and yaml_path == "JWT.RefreshSecret":
        return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", "jwt_refresh_secret")])

    if file_name == "admin.yaml" and yaml_path == "JWT.AccessSecret":
        return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", "admin_jwt_access_secret")])
    if file_name == "admin.yaml" and yaml_path == "JWT.RefreshSecret":
        return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", "admin_jwt_refresh_secret")])

    # 3) Signer encryption password
    if file_name == "signer.yaml" and yaml_path == "Security.EncryptionPassword":
        return Injection(kind="csi", details=[CsiAlias("wallet-signer-secrets", "signer_encryption_password")])

    # 4) SMTP secrets (Business/Admin)
    smtp_map = {
        "Email.SMTPHost": "smtp_host",
        "Email.SMTPPort": "smtp_port",
        "Email.FromAddress": "smtp_from_address",
        "Email.FromName": "smtp_from_name",
        "Email.FromPassword": "smtp_password",
    }
    if file_name in {"business.yaml", "admin.yaml"} and yaml_path in smtp_map:
        return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", smtp_map[yaml_path])])

    # 5) Swap secrets
    if file_name == "business.yaml" and yaml_path == "SwapAuth.ApiKey":
        return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", "business_swap_api_key")])
    if file_name == "admin.yaml" and yaml_path == "SwapAuth.AdminToken":
        return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", "admin_swap_admin_token")])

    if file_name == "swap.yaml":
        swap_map = {
            "Swap.Auth.ApiKeyPepper": "swap_api_key_pepper",
            "Swap.Auth.AdminTokenHash": "swap_admin_token_sha256",
            "Swap.Provider.OneInch.ApiKey": "oneinch_api_key",
            "Swap.Provider.Okx.ApiKey": "okx_api_key",
            "Swap.Provider.Okx.SecretKey": "okx_secret_key",
            "Swap.Provider.Okx.Passphrase": "okx_passphrase",
        }
        if yaml_path in swap_map:
            return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", swap_map[yaml_path])])

    # 6) S3 config (gateway)
    if file_name == "gateway.yaml" and yaml_path.startswith("S3."):
        s3_map = {
            "S3.Region": "s3_region",
            "S3.Bucket": "s3_bucket",
            "S3.Endpoint": "s3_endpoint",
            "S3.PublicBaseURL": "s3_public_base_url",
            "S3.Prefix": "s3_prefix",
            "S3.UsePathStyle": "s3_use_path_style",
            "S3.ACL": "s3_acl",
        }
        if yaml_path in s3_map:
            return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", s3_map[yaml_path])])

    # 7) ChainRPC upstream API keys (chainrpc service)
    if file_name == "chainrpc.yaml" and yaml_path in {"Chains[*].APIKey", "Chains[*].TronAPIKey"}:
        # Derive required aliases from baseline chain types.
        chains = baseline_obj.get("Chains") or []
        if not isinstance(chains, list):
            chains = []
        aliases: List[CsiAlias] = []
        for c in chains:
            if not isinstance(c, dict):
                continue
            ct = str(c.get("ChainType", "")).upper()
            if ct == "ETH":
                aliases.append(CsiAlias("wallet-app-secrets", "chainrpc_eth_rpc_api_key"))
            elif ct == "BSC":
                aliases.append(CsiAlias("wallet-app-secrets", "chainrpc_bsc_rpc_api_key"))
            elif ct == "TRON":
                aliases.append(CsiAlias("wallet-app-secrets", "chainrpc_tron_api_key"))
        # De-dup while preserving order
        seen = set()
        uniq = []
        for a in aliases:
            if a.alias in seen:
                continue
            seen.add(a.alias)
            uniq.append(a)
        return Injection(kind="csi", details=uniq)

    # 8) ChainSync provider API keys
    if file_name == "chainsync.yaml":
        provider_map = {
            "Providers.ethereum.APIKey": "chainsync_ethereum_api_key",
            "Providers.bsc.APIKey": "chainsync_bsc_api_key",
            "Providers.tron.APIKey": "chainsync_tron_api_key",
            "Providers.quickNode.APIKey": "chainsync_quicknode_api_key",
            "Providers.infura.APIKey": "chainsync_infura_api_key",
            "Providers.alchemy.APIKey": "chainsync_alchemy_api_key",
            "Providers.SelfHosted[*].APIKey": "chainsync_selfhosted_api_key",
        }
        if yaml_path in provider_map:
            return Injection(kind="csi", details=[CsiAlias("wallet-app-secrets", provider_map[yaml_path])])

    # 9) Consolidation chain upstream API keys
    # Reuse the existing ChainRPC secrets from the shared CSI mount.
    if file_name == "consolidation.yaml" and yaml_path in {"Chains[*].APIKey", "Chains[*].TronAPIKey"}:
        chains = baseline_obj.get("Chains") or []
        if not isinstance(chains, list):
            chains = []
        aliases: List[CsiAlias] = []
        for c in chains:
            if not isinstance(c, dict):
                continue
            ct = str(c.get("ChainType", "")).upper()
            if yaml_path == "Chains[*].APIKey":
                if ct == "ETH":
                    aliases.append(CsiAlias("wallet-app-secrets", "chainrpc_eth_rpc_api_key"))
                elif ct == "BSC":
                    aliases.append(CsiAlias("wallet-app-secrets", "chainrpc_bsc_rpc_api_key"))
            else:
                if ct == "TRON":
                    aliases.append(CsiAlias("wallet-app-secrets", "chainrpc_tron_api_key"))
        seen = set()
        uniq = []
        for a in aliases:
            if a.alias in seen:
                continue
            seen.add(a.alias)
            uniq.append(a)
        return Injection(kind="csi", details=uniq)

    return None


@dataclasses.dataclass
class Finding:
    group: str
    service: str
    yaml_path: str
    status: str  # missing|unfilled|missing_alias|missing_env|not_applicable|struct_diff|missing_prod_file
    baseline_file: str
    production_file: str
    details: Dict[str, Any]


def _render_markdown(findings: List[Finding], meta: Dict[str, Dict[str, dict]], env: Dict[str, str]) -> str:
    def _section(title: str) -> str:
        return f"\n## {title}\n"

    groups_order = [
        ("configmap_files", "ConfigMap / 服务配置文件"),
        ("configmap_env", "ConfigMap / 环境变量（wallet-biz-env）"),
        ("csi_wallet_app", "AWS Secrets Manager / Secrets Store CSI（wallet-app-secrets）"),
        ("csi_wallet_signer", "AWS Secrets Manager / Secrets Store CSI（wallet-signer-secrets）"),
        ("externalsecrets", "AWS Secrets Manager / ExternalSecrets（infra）"),
        ("struct_diff", "结构差异 / 已替代（不算缺口）"),
        ("other", "其他/解析错误"),
    ]

    by_group: Dict[str, List[Finding]] = {}
    for f in findings:
        by_group.setdefault(f.group, []).append(f)

    # Deterministic ordering inside groups.
    for g in by_group:
        by_group[g].sort(key=lambda x: (x.service, x.production_file, x.yaml_path, x.status))

    total_gaps = sum(1 for f in findings if f.group not in {"struct_diff"})

    out: List[str] = []
    out.append("# Production Config Gap Report (baseline: tmp/**/etc/*.yaml)")
    out.append("")
    out.append("- Regenerate: `python3 scripts/audit-prod-config-from-tmp.py`")
    out.append(f"- Baseline glob: `{TMP_BASELINE_GLOB}`")
    out.append(f"- Production config dir: `{PROD_CONFIG_DIR.relative_to(ROOT)}`")
    out.append(f"- SecretProviderClass: `{SPC_FILE.relative_to(ROOT)}`")
    out.append(f"- ExternalSecrets: `{EXTERNAL_SECRETS_FILE.relative_to(ROOT)}`")
    out.append("")
    out.append("## Summary")
    out.append(f"- Findings (gaps): `{total_gaps}`")
    out.append(f"- Findings (struct diff): `{len(by_group.get('struct_diff', []))}`")
    out.append("")
    out.append("Rules (high level):")
    out.append("- Etcd.Hosts/Key in tmp is treated as replaced by Endpoints in production (not a gap).")
    out.append("- Empty placeholders in production are not gaps if they are wired to CSI/ExternalSecrets/env.")
    out.append("- Secret values are never printed in this report; only field paths and wiring locations.")

    # Sections
    for key, title in groups_order:
        items = by_group.get(key, [])
        if not items:
            continue
        out.append(_section(title).rstrip())
        for f in items:
            svc = f.service
            ypath = f.yaml_path
            status = f.status
            base = f.baseline_file
            prod = f.production_file
            note = f.details.get("note", "")

            if key.startswith("csi_") and status == "missing_alias":
                alias = f.details.get("alias", "")
                spc = f.details.get("spc", "")
                src = meta.get(spc, {}).get(alias, {})
                src_s = ""
                if src:
                    src_s = f" (AWS secret: `{src.get('objectName','')}` / property: `{src.get('property','')}`)"
                note_s = ""
                if note:
                    note_s = f" Note: {note}"
                out.append(f"- `{svc}` `{ypath}` -> **missing CSI alias** `{alias}` in `{spc}`{src_s}. ({prod}){note_s}")
                continue

            if key == "configmap_env" and status == "missing_env":
                env_key = f.details.get("env_key", "")
                out.append(f"- `{env_key}` missing/empty in `wallet-biz-env`. (source: {prod})")
                continue

            if key == "struct_diff":
                out.append(f"- `{svc}` `{ypath}`: {note} ({prod})")
                continue

            if status == "missing_prod_file":
                out.append(f"- `{svc}`: missing production config file for `{base}`.")
                continue

            # configmap_files + others
            out.append(f"- `{svc}` `{ypath}` -> **{status}** (baseline: `{base}`, production: `{prod}`){(': ' + note) if note else ''}")

    # Append env snapshot (non-secret)
    if env:
        out.append("\n## wallet-biz-env snapshot (non-secret)")
        for k in sorted(env.keys()):
            v = env[k]
            shown = v if len(v) <= 80 else v[:77] + "..."
            out.append(f"- `{k}` = `{shown}`")

    return "\n".join(out).rstrip() + "\n"


def main(argv: List[str]) -> int:
    parser = argparse.ArgumentParser(description="Audit production config gaps based on tmp/**/etc/*.yaml baseline.")
    parser.add_argument(
        "--output",
        default=str(ROOT / "docs/ops/production-config-gap.md"),
        help="Output markdown path (default: docs/ops/production-config-gap.md).",
    )
    args = parser.parse_args(argv)

    output_path = Path(args.output)
    if not output_path.is_absolute():
        output_path = (ROOT / output_path).resolve()

    env_vars = _parse_wallet_biz_env()
    spc_meta, aliases_by_spc = _parse_secretproviderclasses()
    externalsecrets = _parse_externalsecrets()
    externalsecret_keys = set(externalsecrets.keys())

    # Map production yaml by filename.
    prod_files = sorted(PROD_CONFIG_DIR.glob("**/*.yaml"))
    prod_by_name: Dict[str, List[Path]] = {}
    for p in prod_files:
        prod_by_name.setdefault(p.name, []).append(p)

    baseline_files = sorted(ROOT.glob(TMP_BASELINE_GLOB))
    findings: List[Finding] = []

    # Always validate key shared env vars used by secrets loader in k8s mode.
    required_env_keys = ["APP_ENV", "MYSQL_HOST", "MYSQL_PORT", "MYSQL_USERNAME", "MYSQL_DATABASE", "REDIS_HOST", "REDIS_PORT", "KAFKA_BROKERS"]
    for k in required_env_keys:
        if k not in env_vars or env_vars[k].strip() == "":
            findings.append(
                Finding(
                    group="configmap_env",
                    service="shared",
                    yaml_path=k,
                    status="missing_env",
                    baseline_file="-",
                    production_file=str(KUSTOMIZATION_FILE.relative_to(ROOT)),
                    details={"env_key": k},
                )
            )

    # Track structural diffs de-dup.
    struct_seen: set = set()

    for bf in baseline_files:
        rel = bf.relative_to(ROOT / "tmp")
        service = bf.stem
        if len(rel.parts) >= 1 and rel.parts[0] == "api-gateway":
            service = "api-gateway"
        elif len(rel.parts) >= 2 and rel.parts[0] == "services":
            service = rel.parts[1]

        prod_candidates = prod_by_name.get(bf.name, [])
        if len(prod_candidates) != 1:
            findings.append(
                Finding(
                    group="other",
                    service=service,
                    yaml_path="(file)",
                    status="missing_prod_file",
                    baseline_file=str(bf.relative_to(ROOT)),
                    production_file=",".join(str(p.relative_to(ROOT)) for p in prod_candidates) or "-",
                    details={"note": "production config file not found or not unique by filename"},
                )
            )
            continue

        pf = prod_candidates[0]
        try:
            b_obj = _load_yaml_single(bf)
            p_obj = _load_yaml_single(pf)
        except Exception as e:
            findings.append(
                Finding(
                    group="other",
                    service=service,
                    yaml_path="(file)",
                    status="parse_error",
                    baseline_file=str(bf.relative_to(ROOT)),
                    production_file=str(pf.relative_to(ROOT)),
                    details={"note": f"YAML parse error: {e}"},
                )
            )
            continue

        b_flat = _flatten_yaml(b_obj)
        p_flat = _flatten_yaml(p_obj)

        for ypath in sorted(b_flat.keys()):
            # Handle Etcd.* as structural diff if Endpoints exist.
            m_hosts = re.match(r"^(.*)\.Etcd\.Hosts\[\*\]$", ypath)
            m_key = re.match(r"^(.*)\.Etcd\.Key$", ypath)
            is_top_level_hosts = ypath == "Etcd.Hosts[*]"
            is_top_level_key = ypath == "Etcd.Key"
            if m_hosts or m_key:
                prefix = (m_hosts or m_key).group(1)
                if _detect_etcd_replacement(p_flat, prefix):
                    struct_key = (service, prefix or "(root)", "etcd->endpoints")
                    if struct_key not in struct_seen:
                        struct_seen.add(struct_key)
                        endpoints_path = "Endpoints[*]" if prefix == "" else f"{prefix}.Endpoints[*]"
                        findings.append(
                            Finding(
                                group="struct_diff",
                                service=service,
                                yaml_path=f"{prefix + '.' if prefix else ''}Etcd.*",
                                status="struct_diff",
                                baseline_file=str(bf.relative_to(ROOT)),
                                production_file=str(pf.relative_to(ROOT)),
                                details={"note": f"replaced by `{endpoints_path}` in production"},
                            )
                        )
                    continue

                # If production still has Etcd.*, treat as covered.
                if ypath in p_flat:
                    continue

                # Neither Endpoints nor Etcd => treat as structural diff (Etcd disabled/omitted in k8s).
                findings.append(
                    Finding(
                        group="struct_diff",
                        service=service,
                        yaml_path=f"{prefix + '.' if prefix else ''}Etcd.*",
                        status="struct_diff",
                        baseline_file=str(bf.relative_to(ROOT)),
                        production_file=str(pf.relative_to(ROOT)),
                        details={"note": "Etcd is omitted in production; service discovery relies on K8s DNS/Endpoints"},
                    )
                )
                continue
            if is_top_level_hosts or is_top_level_key:
                # Top-level Etcd.* (server registration) is commonly omitted in k8s deployments.
                if "Endpoints[*]" in p_flat:
                    # Rare, but keep consistent
                    struct_key = (service, "(root)", "etcd->endpoints")
                    if struct_key not in struct_seen:
                        struct_seen.add(struct_key)
                        findings.append(
                            Finding(
                                group="struct_diff",
                                service=service,
                                yaml_path="Etcd.*",
                                status="struct_diff",
                                baseline_file=str(bf.relative_to(ROOT)),
                                production_file=str(pf.relative_to(ROOT)),
                                details={"note": "replaced by `Endpoints[*]` in production"},
                            )
                        )
                    continue
                if ypath in p_flat:
                    continue
                struct_key = (service, "(root)", "etcd-omitted")
                if struct_key not in struct_seen:
                    struct_seen.add(struct_key)
                    findings.append(
                        Finding(
                            group="struct_diff",
                            service=service,
                            yaml_path="Etcd.*",
                            status="struct_diff",
                            baseline_file=str(bf.relative_to(ROOT)),
                            production_file=str(pf.relative_to(ROOT)),
                            details={"note": "Etcd is omitted in production; service discovery relies on K8s DNS/Endpoints"},
                        )
                    )
                continue

            prod_present = ypath in p_flat
            prod_filled = _values_filled(p_flat.get(ypath, []))

            # If production value exists and is filled, it is covered.
            if prod_present and prod_filled:
                continue

            inj = _resolve_injection(bf.name, ypath, b_obj)
            if inj is not None:
                if inj.kind == "not_applicable":
                    struct_key = (service, ypath, "not_applicable")
                    if struct_key not in struct_seen:
                        struct_seen.add(struct_key)
                        findings.append(
                            Finding(
                                group="struct_diff",
                                service=service,
                                yaml_path=ypath,
                                status="not_applicable",
                                baseline_file=str(bf.relative_to(ROOT)),
                                production_file=str(pf.relative_to(ROOT)),
                                details={"note": inj.details.get("reason", "not applicable")},
                            )
                        )
                    continue

                if inj.kind == "env":
                    env_key = inj.details.get("env_key")
                    if not env_key:
                        continue
                    if env_key not in env_vars or env_vars[env_key].strip() == "":
                        findings.append(
                            Finding(
                                group="configmap_env",
                                service=service,
                                yaml_path=ypath,
                                status="missing_env",
                                baseline_file=str(bf.relative_to(ROOT)),
                                production_file=str(KUSTOMIZATION_FILE.relative_to(ROOT)),
                                details={"env_key": env_key},
                            )
                        )
                    continue

                if inj.kind == "externalsecret":
                    secret_key = inj.details.get("secretKey")
                    if secret_key and secret_key not in externalsecret_keys:
                        findings.append(
                            Finding(
                                group="externalsecrets",
                                service=service,
                                yaml_path=ypath,
                                status="missing_externalsecret_key",
                                baseline_file=str(bf.relative_to(ROOT)),
                                production_file=str(EXTERNAL_SECRETS_FILE.relative_to(ROOT)),
                                details={"note": f"missing ExternalSecret secretKey `{secret_key}`"},
                            )
                        )
                    continue

                if inj.kind == "csi":
                    missing_any = False
                    aliases: List[CsiAlias] = inj.details or []
                    for a in aliases:
                        spc_name = a.spc_name
                        alias = a.alias
                        if alias not in aliases_by_spc.get(spc_name, set()):
                            missing_any = True
                            group = "csi_wallet_signer" if spc_name == "wallet-signer-secrets" else "csi_wallet_app"
                            note = ""
                            if alias.startswith("consolidation_"):
                                note = (
                                    "Consolidation chain upstream API keys are empty in production config; "
                                    "add CSI aliases (and matching AWS Secrets properties) and wire them into "
                                    "the consolidation service, or fill them directly in the ConfigMap file."
                                )
                            findings.append(
                                Finding(
                                    group=group,
                                    service=service,
                                    yaml_path=ypath,
                                    status="missing_alias",
                                    baseline_file=str(bf.relative_to(ROOT)),
                                    production_file=str(pf.relative_to(ROOT)),
                                    details={"spc": spc_name, "alias": alias, "note": note},
                                )
                            )
                    if missing_any:
                        continue
                    # CSI wiring exists -> covered (even if prod yaml has placeholder or missing).
                    continue

            # No injection mapping: decide gap based on baseline being filled or key missing.
            baseline_filled = _values_filled(b_flat.get(ypath, []))
            if not prod_present:
                findings.append(
                    Finding(
                        group="configmap_files",
                        service=service,
                        yaml_path=ypath,
                        status="missing",
                        baseline_file=str(bf.relative_to(ROOT)),
                        production_file=str(pf.relative_to(ROOT)),
                        details={},
                    )
                )
                continue

            # prod_present but unfilled
            if baseline_filled:
                findings.append(
                    Finding(
                        group="configmap_files",
                        service=service,
                        yaml_path=ypath,
                        status="unfilled",
                        baseline_file=str(bf.relative_to(ROOT)),
                        production_file=str(pf.relative_to(ROOT)),
                        details={},
                    )
                )

    md = _render_markdown(findings, spc_meta, env_vars)

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(md, encoding="utf-8")
    print(f"Wrote report: {output_path}")

    # Non-zero exit if there are gaps (excluding struct diffs).
    gap_count = sum(1 for f in findings if f.group not in {"struct_diff"})
    return 2 if gap_count > 0 else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
