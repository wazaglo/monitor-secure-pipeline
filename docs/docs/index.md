# Monitor Secure Pipeline

An end-to-end **observability platform** for the
[secure-pipeline](https://github.com/wazaglo/secure-pipeline) DevSecOps CI/CD stack.

It monitors 24+ API endpoints across 5 microservices plus the security tooling
(DefectDojo, SonarQube) using **Prometheus**, **Grafana**, **Loki**, **Tempo**,
the **OpenTelemetry Collector**, and **Alertmanager** — everything in Docker.

## What it does

| Capability | Tooling | Details |
| ---------- | ------- | ------- |
| Metrics | Prometheus + OpenTelemetry | Service metrics via OTLP, infra metrics via node-exporter + cAdvisor |
| Logs | Loki (no Promtail) | All application logs ship via OTel logs → otelcol → Loki `/otlp` |
| Traces | Tempo | OTLP traces with service graphs + span metrics |
| Alerting | Alertmanager + Prometheus rules | 16 rules across services, infrastructure, telemetry, and security |
| Dashboards | Grafana | 7 pre-provisioned dashboards + Tempo service map |
| Security | DefectDojo exporter | `defectdojo_findings{severity}` metrics published to Prometheus |

## Stack diagram

![Architecture](assets/architecture.svg)

See [Architecture](architecture.md) for the full walkthrough.

## Quick facts

- **13+ monitored endpoints** proxied through the API gateway, backed by 5 OTel-instrumented services
- **15+ services** running in Docker (apps, telemetry backends, exporters)
- **3 signals** — metrics, logs, and traces — correlated in Grafana
- **Zero Promtail** — logs flow through the OTel Collector using the OTLP/HTTP protocol

## Get started

```bash
git clone https://github.com/wazaglo/monitor-secure-pipeline.git
cd monitor-secure-pipeline
cp .env.example .env
docker compose up -d --build
```

Open:

- Grafana: http://localhost:3000 (`admin` / `admin`)
- Prometheus: http://localhost:9090
- API Gateway: http://localhost:8080/api/info

See [Quickstart](quickstart.md) for details.
