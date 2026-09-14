# Partitioned API Throttler

A high-performance, partitioned sliding-window API rate limiter written in Go. Uses sharded locks to minimize contention while enforcing per-IP request limits.

## Architecture

```
Request → FNV-1a hash → Partition[shard] → slidingWindow(ip) → Allow / Deny
```

- **Sharded partitions** — IPs are distributed across N partitions (default 64), each with its own mutex. Contention is bounded to IPs that hash to the same shard.
- **Sliding window** — Each IP gets a timestamp-based window that tracks requests within a configurable duration. Old entries are compacted on each check.
- **Background cleanup** — A goroutine periodically evicts stale IP entries to bound memory usage.
- **Prometheus metrics** — Exposes request counts, throttle rates, latency histograms, and active IP gauges.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| ANY | `/api/*` | Throttled API endpoint |
| GET | `/api/echo` | Echoes query params + timestamp |
| GET | `/metrics` | Prometheus metrics |
| GET | `/health` | Health check |

## Quick Start

```bash
# Run directly
go run . -addr :8080 -partitions 64 -limit 10 -window 1m

# Docker
docker compose up --build

# Prometheus UI
open http://localhost:9090
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `:8080` | HTTP listen address |
| `-partitions` | `64` | Number of partitions (shards) |
| `-limit` | `10` | Per-IP request limit per window |
| `-window` | `1m` | Sliding window duration |
| `-cleanup` | `30s` | Stale-IP cleanup interval |

## Metrics

- `api_throttler_requests_total{status, partition}` — Total requests by status (allowed/denied) and partition
- `api_throttler_throttle_rate{partition}` — Denied/total ratio per partition
- `api_throttler_request_duration_seconds{endpoint}` — Request latency histogram
- `api_throttler_active_ips` — Number of unique IPs being tracked

## Testing

```bash
go test -v -race ./...
```

## License

MIT
