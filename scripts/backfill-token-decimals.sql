-- One-off backfill for token_decimals (safe to rerun).
-- Run against MariaDB 10.11 database.
--
-- Example:
--   mysql -h <host> -P <port> -u <user> -p <db> < scripts/backfill-token-decimals.sql

UPDATE `currency_chain_settings`
SET `token_decimals` = 6
WHERE `deleted_at` IS NULL
  AND UPPER(TRIM(`asset_code`)) = 'USDT'
  AND UPPER(TRIM(`chain_code`)) = 'ETH'
  AND `contract_address` IS NOT NULL
  AND TRIM(`contract_address`) <> ''
  AND `token_decimals` IS NULL;

UPDATE `currency_chain_settings`
SET `token_decimals` = 18
WHERE `deleted_at` IS NULL
  AND UPPER(TRIM(`asset_code`)) = 'USDT'
  AND UPPER(TRIM(`chain_code`)) = 'BSC'
  AND `contract_address` IS NOT NULL
  AND TRIM(`contract_address`) <> ''
  AND `token_decimals` IS NULL;

UPDATE `currency_chain_settings`
SET `token_decimals` = 6
WHERE `deleted_at` IS NULL
  AND UPPER(TRIM(`asset_code`)) = 'USDT'
  AND UPPER(TRIM(`chain_code`)) = 'TRON'
  AND `contract_address` IS NOT NULL
  AND TRIM(`contract_address`) <> ''
  AND `token_decimals` IS NULL;

