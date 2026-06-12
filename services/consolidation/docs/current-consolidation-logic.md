# Consolidation Service – Current Logic (dev)

This document describes the current consolidation workflow implemented by `services/consolidation/rpc`.

## Goals

- Consolidate funds from **user deposit addresses** to configured **hot wallet target addresses**.
- Ensure **one active consolidation task per (chain, from_address)** to avoid nonce/resource contention.
- Ensure retries are **bounded** and do not create “new task” loops that bypass `MaxRetries`.

## Task lifecycle (high level)

1. **Discovery**
   - Scans deposit addresses (`wallet_user_chain_addresses` via `AddressRepo`).
   - Skips an address if:
     - An **active task** already exists for `(chain, from_address)`.
     - A **terminal failed task** exists (`PermanentFailed`/`Failed`) for the same address (manual intervention required).
   - **Token-first behavior**:
     - If a token task is actually created, native sweep is skipped.
     - If token is above threshold but skipped due to cooldown, native sweep can still proceed.

2. **Execution (Executor)**
   - Re-checks thresholds and chain-specific prechecks (gas/activation/bandwidth/energy).
   - Builds an **unsigned** tx via direct node clients (`pkg/chainnode`).
   - Sends the unsigned tx to **Signer** for signing (`operation_type=3`).
   - Marks task `InProgress` (persist `tx_hash` + `signed_tx`) and broadcasts.
   - If broadcast fails, task is moved back to `Pending` with backoff + `retry_count++` (prevents indefinite lock).

3. **Tracking (StatusTracker)**
   - Polls `GetTransactionReceipt` until confirmations are met.
   - On `CONFIRMED`: marks task `Confirmed`.
   - On `FAILED/DROPPED/REPLACED`: moves back to `Pending` with backoff + `retry_count++`; after `MaxRetries` becomes `PermanentFailed`.
   - If no receipt:
     - May rebroadcast (and EVM may bump gas after configured delay).
     - Marks `Timeout` after `MaxPendingDuration`.
     - Abandons the tx and retries the task after `MaxNoReceiptDuration` (prevents “stuck forever” tasks).

## Required DB migrations

This service assumes these migrations are applied (fail-fast on startup if incompatible):

- `60-consolidation.sql`
- `63-consolidation-signed-tx.sql`
- `64-consolidation-address-lock.sql`
- `65-consolidation-evm-bump.sql`

