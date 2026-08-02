# Contributing

## Repository layout

```text
services/
  api-gateway/        Go gateway, proxies to the four backends
  user-service/       Python, user endpoints
  product-service/    Go, product + inventory endpoints
  order-service/      Go, order lifecycle
  payment-service/    Python, payments/refunds/webhooks
load-generator/       Python, drives traffic through the gateway
monitoring/
  prometheus/         prometheus.yml + rules.yml (16 rules)
  grafana/provisioning/  datasources + 7 dashboards
  loki/               loki-config.yml (OTLP ingest, retention)
  tempo/              tempo-config.yml (v3, metrics generator)
  alertmanager/       alertmanager.yml (routing + inhibition)
  otelcol/            OTLP pipeline: traces→Tempo, metrics→Prometheus, logs→Loki
  exporters/defectdojo-exporter/   custom Prometheus exporter
terraform/            EC2 deployment
docs/                 MkDocs site + runbooks
```

## Adding an endpoint

1. Add the route to the relevant service and re-export telemetry (traces/metrics/logs).
2. Add the row to `docs/docs/services.md`.
3. Rebuild and confirm it appears in the Services dashboard.

## Adding an alert

1. Append a rule to `monitoring/prometheus/rules.yml`.
2. Document it in `docs/docs/alerts.md`.
3. Validate locally:

```bash
docker run --rm -v "$PWD/monitoring/prometheus:/etc/prometheus" \
  --entrypoint promtool prom/prometheus:latest check rules /etc/prometheus/rules.yml
```

## Adding a dashboard

1. Add a JSON dashboard under `monitoring/grafana/provisioning/dashboards/`.
2. Grafana auto-reloads it (30s refresh).
3. Document it in `docs/docs/dashboards.md`.

## Adding a runbook

1. Create a page under `docs/docs/runbooks/`.
2. Link it in `docs/mkdocs.yml` under `nav → Runbooks`.

## Running checks

```bash
docker compose config --quiet
docker run --rm -v "$PWD/monitoring/prometheus:/etc/prometheus" \
  --entrypoint promtool prom/prometheus:latest check rules /etc/prometheus/rules.yml
docker run --rm -v "$PWD/monitoring/otelcol:/config" \
  otel/opentelemetry-collector-contrib:latest validate --config=/config/config.yml
```
