# Alerts

Prometheus ships **16 rules** in 5 groups. Alerts are routed through
Alertmanager, which groups by `alertname`/`severity`, inhibits warnings when a
critical alert for the same signal is active, and is wired up for future
notification channels (email, Slack, PagerDuty).

## services

| Alert | Severity | Condition |
| ----- | -------- | --------- |
| ServiceDown | critical | A scrape target is down for 1m |
| HighErrorRate | critical | 5xx rate > 5% on any route for 5m |
| SlowRequests | warning | P95 latency > 1.5s for 10m |
| UpstreamUnavailable | critical | Gateway sees backend failures for 2m |

## application

| Alert | Severity | Condition |
| ----- | -------- | --------- |
| OrderFailures | warning | No orders created in 10m |
| PaymentFailures | warning | No payments processed in 10m |
| LowStock | warning | `products_stock < 20` for 10m |
## infrastructure

| Alert | Severity | Condition |
| ----- | -------- | --------- |
| HighCPUUsage | warning | Total container CPU > 4 cores for 10m |
| HighMemoryUsage | warning | Total container memory > 2 GiB for 10m |
| DiskSpaceRunningOut | critical | Free space < 10% |
| HighDiskIO | warning | Disk I/O time > 50% for 10m |
| LoadAverageHigh | warning | Load > 2x CPU count for 10m |

## telemetry

| Alert | Severity | Condition |
| ----- | -------- | --------- |
| MetricsPipelineDown | critical | otelcol/Prometheus/Loki/Tempo down for 2m |
| TraceVolumeDropped | warning | Tempo receives < 1 span/s for 15m |

## security

| Alert | Severity | Condition |
| ----- | -------- | --------- |
| HighSeverityFindings | critical | Any open critical/high DefectDojo finding |
| OpenMediumFindings | warning | More than 20 open medium findings |

## Sending notifications

Edit `monitoring/alertmanager/alertmanager.yml` and add a receiver, e.g.:

```yaml
receivers:
  - name: 'default'
    email_configs:
      - to: 'oncall@example.com'
        from: 'alertmanager@example.com'
        smarthost: 'smtp.example.com:587'
```

Then reload:

```bash
docker compose exec alertmanager kill -HUP 1
```

Or route critical alerts to a webhook (e.g. Slack) with `webhook_configs`.
