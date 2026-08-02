# Dashboards

Grafana is pre-provisioned with 7 dashboards under the
**Monitor Secure Pipeline** folder. The datasources (Prometheus, Loki, Tempo)
are configured automatically on startup.

## Overview

Cross-cutting view of the whole platform:

- Request rate and error rate by route
- P95 latency by route
- Target up/down status

## Services

Per-service request volume, error rates, and latency percentiles.

## Latency

- Latency distribution (bucket sums)
- P99 latency by route
- Average latency by route

## Infrastructure

Host-level and container-level metrics:

- Container CPU and memory
- Node load, memory, and disk
- Network I/O

## Logs

A live logs panel querying Loki for `ERROR`/`WARN` level entries across all
services (`{service_name=~".+"}`).

## Tracing

Service-map and span-metric panels backed by Tempo's metrics generator:

- Service graph request rate and failures
- Span counts per service

## Security (DefectDojo)

Findings published by the DefectDojo exporter:

- Findings by severity
- Findings by type and product
- Critical + high severity totals

## Jump to logs from a trace

In Tempo Explore, open any trace, then click **Logs for this span** — the
configured `tracesToLogs` mapping uses the span's `service.name`,
`http.route`, and `http.status_code` attributes to filter Loki.
