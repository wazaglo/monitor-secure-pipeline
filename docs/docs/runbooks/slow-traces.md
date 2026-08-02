# Runbook: Slow Traces

Alerts: **SlowRequests** (warning), **TraceVolumeDropped** (warning).

## Symptoms

- P95 latency > 1.5s on some route
- Or Tempo receives almost no trace requests for 15 minutes

## Part A — Slow spans

### 1. Isolate the slow hop

Grafana → Explore → Tempo. Query a recent trace for the slow route
(`service.name` + `http.route`), open it, and expand each span. The `api-gateway`
span shows `backend=<name>`; the slow segment is usually a child span (e.g.
`orders.detail` with a synthetic 0–120ms delay, or a `product-service` lookup).

### 2. Find the bottleneck

- **Application**: check per-route p50/p95 in the Latency dashboard.
- **Backend service**: `docker compose logs <service>` for slow handling.
- **Collector backpressure**: `otelcol` batches every 1s; if memory_limiter
  spikes (limit 512 MiB), traces may queue. Check `otelcol` logs.

### 3. Mitigate

- Reduce artificial delay in the demo service (they sleep a random 0–120ms).
- Tune `processors.batch` timeouts in `monitoring/otelcol/config.yml`.
- Rebuild the slow service: `docker compose up -d --build <service>`.

## Part B — Missing/low trace volume

### 1. Verify the pipeline

```bash
curl -s http://localhost:3200/ready
docker compose logs --tail=50 otelcol
```

### 2. Common causes

| Cause | Fix |
| ----- | --- |
| Tempo storage volume corrupted/full | Follow [Disk full](disk-full.md) |
| otelcol lost connection to Tempo | `docker compose restart otelcol` |
| Tempo restarted and lost WAL | Check Tempo logs; restart stack with volumes intact |
| `user: root` missing on Tempo | The compose file sets `user: root` to avoid volume ownership issues — do not remove it |

### 3. Re-generate traces

If the load-generator stopped, restart it:

```bash
docker compose restart load-generator
```

The alert clears once trace ingestion resumes for 15 minutes.
