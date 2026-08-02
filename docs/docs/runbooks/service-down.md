# Runbook: Service Down

Alert: **ServiceDown** (critical) — `up == 0` for 1 minute.

## Symptoms

- "ServiceDown" fires in Alertmanager
- Grafana "Up targets" panel shows the target offline
- `curl` to the affected service fails

## Steps

### 1. Identify the target

From the alert labels read `job` and `instance` (e.g. `job="order-service"`,
`instance="order-service:8003"`). The demo services expose no native `/metrics`,
so they do **not** appear as `up` targets — this alert applies to the
telemetry/exporters (`otelcol`, `prometheus`, `loki`, `tempo`, `alertmanager`,
`node-exporter`, `cadvisor`, `defectdojo-exporter`).

### 2. Inspect the container

```bash
docker compose ps
docker compose logs --tail=200 <job>
```

Common causes:

| Cause | Evidence |
| ----- | -------- |
| Crash-loop | Container restarts repeatedly; check logs for a panic/exception |
| OOM | Container exits with code 137; `docker inspect` shows OOMKilled |
| Bad config | Component refuses to start; logs show config parse error |
| Depends-on failure | Check `depends_on` ordering — a backend never started |

### 3. Restart

```bash
docker compose up -d <job>
# or rebuild if the image changed
docker compose up -d --build <job>
```

### 4. Verify

```bash
docker compose ps <job>
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job=="<job>") | .health'
```

Wait for `up == 1` to persist for 1m so the alert clears on its own.
