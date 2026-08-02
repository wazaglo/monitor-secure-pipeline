# Runbook: High Error Rate

Alert: **HighErrorRate** (critical) — 5xx rate > 5% on a route for 5 minutes.

## Symptoms

- Alert fires with `route` label (e.g. `/api/users/99`)
- Grafana error-rate panel shows a spike
- Gateway may also fire **UpstreamUnavailable** if backends fail

## Steps

### 1. Find the failing route

The `route` label on the alert tells you the endpoint. The demo load-generator
deliberately exercises 4xx/5xx paths (`/api/users/99`, `/api/products/999`,
`/api/orders/ORD-9999`, invalid IDs). Distinguish:

- **4xx** = client errors. Expected for the "not found" test routes — check the
  status distribution in the Services dashboard rather than chasing them.
- **5xx** = real service faults worth investigating.

### 2. Correlate logs and traces

```text
Loki:    {service_name="order-service"} |= "ERROR"
Tempo:   find a trace with error status, open span → Logs for this span
```

The `api-gateway` logs `backend unreachable` when a service is down.

### 3. Check the specific service

```bash
docker compose logs --tail=100 <service>
curl -s http://localhost:8080/api/info   # gateway health
```

### 4. Fix and redeploy

```bash
# patch code in services/<service>, then
docker compose up -d --build <service>
```

### 5. Verify

Watch Grafana error-rate panel for the route return under 5% for 5 minutes so
the alert clears automatically.

> A synthetic spike can be reproduced by scaling the load-generator's 404/500
> weights, useful for demonstrating the alerting pipeline.
