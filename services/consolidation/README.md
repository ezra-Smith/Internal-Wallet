# Consolidation Service (`services/consolidation/rpc`)

DB-backed multi-chain fund consolidation service for **user deposit addresses only** (TRON / ETH / BSC).

## What it does (Phase 1)

- **Discovery**: scans `wallet_user_chain_addresses` (deposit addresses) and requires a matching row in `address_generation_logs` (so Signer can sign), then excludes:
  - `company_wallets` (company hot/cold/collection)
  - `vault_addresses` (vault wallets)
- **Filtering**: threshold + cooldown + “only one active task per (chain, asset, token_contract, from_address)”.
- **Execution**:
  - **EVM (ETH/BSC)**: `ChainRPC.BuildTransaction` → `Signer.SignTransaction(operation_type=3)` → `ChainRPC.BroadcastTransaction`
  - **TRON**: `ChainRPC.BuildTronTransaction` → `Signer.SignTransaction(operation_type=3)` → `ChainRPC.BroadcastTransaction`
- **TRON TRC20 resources**: supports either **burning TRX** for bandwidth/energy fees, or **energy rental** via iTRX (create order → poll status → verify energy → return task to pending). Controlled by `Consolidation.TronTrc20FeeMode`.
- **Tracking**: polls `GetTransactionReceipt` + `GetBlockHeight` until confirmations reached.

## DB schema

- Migration: `deploy/docker/init-db/60-consolidation.sql`
- Tables:
  - `consolidation_tasks`
  - `consolidation_logs`
  - `energy_rental_records`

## Configuration

- Template: `services/consolidation/rpc/etc/consolidation.yaml`
- Secrets (recommended via env vars):
  - `CONSOLIDATION_ITRX_API_ENDPOINT`
  - `CONSOLIDATION_ITRX_API_KEY`
  - `CONSOLIDATION_ITRX_API_SECRET`

Key settings:
- `Consolidation.ToAddressMode`: destination selection (`target_addresses` or `system_hot_wallet`).
- `Consolidation.SystemHotWalletAddressType` / `Consolidation.SystemHotWalletTemperature`: selector used when `ToAddressMode=system_hot_wallet`.
- `Consolidation.TargetAddresses`: per chain/asset destination whitelist (required when `ToAddressMode=target_addresses`).
- `Consolidation.MinBalanceThresholds`: minimum balances (smallest units) for assets enabled in DB.
- `currency_chain_settings`: source of truth for (asset, chain) enablement + token metadata (`contract_address`, `token_decimals`).
- `Consolidation.ValidateTokenInfo`: if true, validates DB token metadata via `ChainRPC.GetTokenInfo` on startup (fail-fast on mismatch).
- `Consolidation.NativeReserves`: native coin reserve left on deposit addresses (prevents token sweeps from being stranded).
- `Consolidation.TronTrc20FeeMode`: TRON TRC20 fee mode when energy is insufficient (`energy_rental` or `trx_fee`).
- `Consolidation.ConsolidationCooldown`: do not re-consolidate within cooldown window.
- `Consolidation.MaxConcurrentPerChain`: rate limiting per chain.

## Detailed flow / runbook

- `services/consolidation/docs/current-consolidation-logic.md`

## How to run locally

1) Ensure infra + db schema exists (MariaDB/Redis/ETCD).  
2) Generate protos if needed: `make proto`  
3) Run dependencies: `make run-chainrpc`, `make run-signer`  
4) Run consolidation service:

```bash
cd services/consolidation/rpc
go run . -f etc/consolidation.yaml
```

## Notes / safety

- This service **never consolidates** from `company_wallets` or `vault_addresses`.
- All signing uses `Signer.SignTransaction` with `operation_type=3` (consolidation) for audit correctness.
- Token-first behavior: if token thresholds are met for an address, native sweep is skipped in discovery (prevents “no gas”).
