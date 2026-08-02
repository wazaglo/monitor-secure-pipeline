# Quickstart

## Prerequisites

- Docker Engine 24+ with the Compose plugin
- At least 4 GB of RAM for the full stack

## Run

```bash
git clone https://github.com/wazaglo/monitor-secure-pipeline.git
cd monitor-secure-pipeline

cp .env.example .env
docker compose up -d --build
```

This builds the 5 services + load-generator and starts the full observability
stack in one command.

## Verify the stack

Check container health:

```bash
docker compose ps
```

Confirm the API gateway responds:

```bash
curl http://localhost:8080/api/info
```

Generate traffic manually if the load-generator is paused:

```bash
curl http://localhost:8080/api/users
curl http://localhost:8080/api/products/1/inventory
curl http://localhost:8080/api/orders
curl http://localhost:8080/api/payments/1/invoice
```

## Open the UIs

| Service | URL | Credentials |
| ------- | --- | ----------- |
| Grafana | http://localhost:3000 | `admin` / `admin` |
| Prometheus | http://localhost:9090 | — |
| Loki | http://localhost:3100 | — |
| Tempo | http://localhost:3200 | — |
| Alertmanager | http://localhost:9093 | — |
| API Gateway | http://localhost:8080/api/info | — |

## See data flowing

1. **Grafana → Explore → Prometheus**: query `http_requests_total`
2. **Grafana → Explore → Loki**: query `{service_name="order-service"}`
3. **Grafana → Explore → Tempo**: pick a trace, then jump from a span to its logs

## Tear down

```bash
docker compose down            # stops containers (keeps data)
docker compose down -v         # stops and removes named volumes
```
