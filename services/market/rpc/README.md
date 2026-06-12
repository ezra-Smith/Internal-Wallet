# Market Service - Binance Mini Ticker → Redis

This service subscribes to Binance spot WebSocket all-market mini-ticker stream and writes the latest price
for each symbol into Redis for low-latency lookups.

## What it does

### Real-time Price Ticker

- Connects to `wss://stream.binance.com:9443/ws/!miniTicker@arr` (configurable).
- Handles Binance `PING` frames by replying with `PONG` frames that reuse the exact same payload.
- Reconnects automatically with exponential backoff, and forces a graceful reconnect before the 24h session expiry.
- Parses the mini-ticker array message (all symbols every ~1s).
- Stores updates in Redis hash `binance:tickers` (configurable):
  - field: `{symbol}`
  - value: `{"price":"<close>","ts":<eventTimeMs>}`
- Sets TTL on the hash key (configurable). When the service stops, data expires automatically.

### Sparkline (Mini Charts)

- Periodically samples prices (default: every 5 minutes) to build mini trend charts.
- Stores sparkline data in **Redis ZSET** `binance:sparkline:{SYMBOL}`:
  - Score: Unix timestamp in milliseconds
  - Member: Price string (e.g., "94250.00")
  - Sorted by timestamp (oldest to newest)
- **Time-range queries**: Use `ZRANGEBYSCORE` to get data for specific periods.
- Default: retains 24 hours of data (configurable via `RetentionHours`).
- Ideal for asset overview pages showing simple price trend visualizations.

### Fiat Exchange Rates (USDT → Fiat)

- Periodically fetches USDT→fiat mid rates via Binance C2C public API (configurable).
- Stores updates in Redis hash `fiat:tickers` (configurable):
  - field: `{ASSET}{FIAT}` (example: `USDTCNY`)
  - value: `{"buy":"<buy>","sell":"<sell>","mid":"<mid>","ts":<unixMs>,"provider":"binance_c2c","currency_symbol":"¥"}`
- Sets TTL on the hash key (configurable). When the service stops, data expires automatically.

## Run

```bash
cd services/market/rpc
go run . -f etc/market.yaml
```

## Redis query examples

### Real-time price

```bash
redis-cli HGET binance:tickers BTCUSDT
# Output: {"price":"94250.00","ts":1735297200000}

redis-cli HGET binance:tickers ETHUSDT
```

### Sparkline data (price history for mini charts)

```bash
# Get all sparkline points for a symbol (score=timestamp, member=price)
redis-cli ZRANGE binance:sparkline:BTCUSDT 0 -1 WITHSCORES

# Get last 10 points only
redis-cli ZRANGE binance:sparkline:BTCUSDT -10 -1 WITHSCORES

# Get data for last 1 hour (now - 3600000ms to now)
# Replace timestamps with actual values
redis-cli ZRANGEBYSCORE binance:sparkline:BTCUSDT 1735293600000 +inf WITHSCORES

# Check how many points exist
redis-cli ZCARD binance:sparkline:BTCUSDT

# Get oldest and newest timestamps
redis-cli ZRANGE binance:sparkline:BTCUSDT 0 0 WITHSCORES   # oldest
redis-cli ZRANGE binance:sparkline:BTCUSDT -1 -1 WITHSCORES  # newest
```

### Fiat rates (USDT → Fiat)

```bash
redis-cli HGET fiat:tickers USDTCNY
# Output: {"buy":"7.20","sell":"7.22","mid":"7.21","ts":1735297200000,"provider":"binance_c2c","currency_symbol":"¥"}
```

## Health check

- Default: `GET http://localhost:18080/healthz`
- Returns `200` when:
  - WS is connected AND
  - recent WS messages are observed (within `Health.StaleAfterMillis`) AND
  - Redis `PING` succeeds within `Health.RedisPingTimeoutMillis`

## Environment variable overrides

All env vars are optional and override the YAML config:

### Binance WebSocket
- `MARKET_BINANCE_WS_BASE_URL` (example: `wss://stream.binance.com:443`)
- `MARKET_BINANCE_WS_ENDPOINT` (default: `/ws/!miniTicker@arr`)

### Redis
- `MARKET_REDIS_ADDR` (example: `localhost:6379`)
- `MARKET_REDIS_PASS`
- `MARKET_REDIS_DB`
- `MARKET_REDIS_TICKER_HASH_KEY`
- `MARKET_REDIS_TTL_SECONDS`
- `MARKET_REDIS_SPARKLINE_KEY_PREFIX` (default: `binance:sparkline:`)

### Reconnect
- `MARKET_RECONNECT_INITIAL_BACKOFF_MS`
- `MARKET_RECONNECT_MAX_BACKOFF_MS`
- `MARKET_RECONNECT_MULTIPLIER`
- `MARKET_RECONNECT_JITTER`

### Health Check
- `MARKET_HEALTH_ENABLED`
- `MARKET_HEALTH_LISTEN_ON`
- `MARKET_HEALTH_PATH`
- `MARKET_HEALTH_STALE_AFTER_MS`

### Sparkline (Mini Charts - ZSET storage)
- `MARKET_SPARKLINE_ENABLED` (default: `true`)
- `MARKET_SPARKLINE_SAMPLE_INTERVAL_SECONDS` (default: `300` = 5 minutes)
- `MARKET_SPARKLINE_RETENTION_HOURS` (default: `24` = 24 hours, use `168` for 7 days)

### Fiat (USDT → Fiat)
- `MARKET_FIAT_ENABLED` (default: `false`)
- `MARKET_FIAT_UPDATE_INTERVAL_SECONDS` (default: `30`)
- `MARKET_FIAT_REQUEST_TIMEOUT_MILLIS` (default: `10000`)
- `MARKET_FIAT_MAX_RETRIES` (default: `3`)
- `MARKET_FIAT_BASE_URL` (default: `https://c2c.binance.com`)
- `MARKET_FIAT_ENDPOINT` (default: `/bapi/c2c/v2/public/c2c/adv/quoted-price`)
- `MARKET_FIAT_REDIS_HASH_KEY` (default: `fiat:tickers`)
- `MARKET_FIAT_TTL_SECONDS` (default: `600`)
