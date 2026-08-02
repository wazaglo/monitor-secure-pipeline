# Architecture

![Architecture](assets/architecture.svg)

## Components

### Application layer

Five OTel-instrumented microservices expose a REST API. All telemetry is exported
over OTLP/gRPC to the OpenTelemetry Collector.

| Service | Language | Port | Role |
| ------- | -------- | ---- | ---- |
| `api-gateway` | Go | 8080 | Entry point, proxies to the four backend services |
| `user-service` | Python | 8001 | User management (6 endpoints) |
| `product-service` | Go | 8002 | Product catalog + inventory (4 endpoints) |
| `order-service` | Go | 8003 | Order lifecycle (5 endpoints) |
| `payment-service` | Python | 8004 | Payments, refunds, webhooks (7 endpoints) |
| `load-generator` | Python | — | Generates realistic traffic, including 4xx/5xx |

> **24+ endpoints** in total are exposed via the gateway and monitored.

### Telemetry pipeline

All three signals travel from the services to the **OpenTelemetry Collector** over
OTLP. There is **no Promtail** — logs use the collector's OTLP/HTTP export to Loki.

```text
app services ──OTLP──> otelcol ──traces──> Tempo
                        │        ──metrics─> Prometheus (on :8889)
                        │        ──logs────> Loki /otlp
```

### Observability backends

| Component | Port | Purpose |
| --------- | ---- | ------- |
| Prometheus | 9090 | Scrapes otelcol (service metrics), node-exporter, cAdvisor, Loki, Tempo, and the DefectDojo exporter |
| Loki | 3100 | Log aggregation; ingests OTLP logs |
| Tempo | 3200 | Distributed tracing; remote-writes span metrics + service graphs to Prometheus |
| Alertmanager | 9093 | Routes and inhibits alerts |
| Grafana | 3000 | Pre-provisioned dashboards and datasources |

### Infrastructure & security exporters

- **node-exporter** — host CPU, memory, disk, network
- **cAdvisor** — per-container resource usage
- **defectdojo-exporter** — pulls active findings from the DefectDojo API and
  publishes `defectdojo_findings{severity, finding_type, product}` to Prometheus

## Deployment

Everything runs under a single `docker-compose.yml` with named persistent volumes.
The same stack can be deployed to AWS EC2 via Terraform — see [Deploy](deploy.md).
