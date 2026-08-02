# Incident Response

When an alert fires, follow this flow. Keep the timeline in the incident notes.

## 1. Acknowledge

In Alertmanager (http://localhost:9093), acknowledge the alert. Note the
`alertname`, `severity`, and any labels (`route`, `job`, `instance`).

## 2. Triage from a single view

Open **Grafana → Overview** and answer three questions:

1. **Is it just one component or the whole stack?** Check the "Up targets"
   panel — if several targets are down, suspect infrastructure (disk, Docker,
   network) before a service bug.
2. **Errors or latency?** Use the error-rate and P95 latency panels to separate
   an error spike from a performance regression.
3. **Did something change?** Check recent deployments, config changes, or the
   load-generator's behavior (e.g. it stops if the gateway is unreachable).

## 3. Correlate the three signals

- **Logs**: Grafana → Explore → Loki. Query
  `{service_name=~".+"} |~ "ERROR|WARN"` for the failing component.
- **Traces**: Grafana → Explore → Tempo. Find a slow/failed trace and jump to
  its logs via the span (uses `tracesToLogs`).
- **Metrics**: check `http_errors_total` and
  `http_request_duration_seconds` for the affected route.

## 4. Act

| Situation | Action |
| --------- | ------ |
| Container crash-looping | `docker compose logs -f <service>`; fix config/code, then `docker compose up -d --build <service>` |
| Disk full | See [Disk full](disk-full.md) |
| High 5xx on one route | See [High error rate](high-error-rate.md) |
| Traces slow/missing | See [Slow traces](slow-traces.md) |
| DefectDojo findings | Create a DefectDojo finding, patch it, re-upload results from secure-pipeline CI |

## 5. Resolve and document

- Confirm the alert clears in Prometheus (http://localhost:9090/alerts).
- Optionally silence the alert in Alertmanager until recovery.
- Update this runbook with what you learned.
