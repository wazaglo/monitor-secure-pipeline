# Services

The platform monitors five microservices plus a load generator. Every service is
instrumented with OpenTelemetry and exports **traces, metrics, and logs** over
OTLP/gRPC to the collector.

## Endpoints

| Method | Path | Service | Notes |
| ------ | ---- | ------- | ----- |
| GET | `/api/info` | api-gateway | Service inventory + uptime |
| GET | `/api/users` | user-service | List users |
| GET | `/api/users/{id}` | user-service | User detail |
| GET | `/api/users/{id}/profile` | user-service | Public profile |
| GET | `/api/users/{id}/settings` | user-service | User settings |
| GET | `/api/users/search?q=` | user-service | Search users |
| GET | `/api/products` | product-service | List products |
| GET | `/api/products?category=` | product-service | Filter by category |
| GET | `/api/products/{id}` | product-service | Product detail |
| GET | `/api/products/{id}/inventory` | product-service | Stock + low-stock flag |
| GET | `/api/orders` | order-service | List orders |
| POST | `/api/orders` | order-service | Create order |
| GET | `/api/orders/{id}` | order-service | Order detail |
| GET | `/api/orders/{id}/status` | order-service | Order status |
| POST | `/api/orders/{id}/cancel` | order-service | Cancel order |
| GET | `/api/payments` | payment-service | List payments |
| POST | `/api/payments` | payment-service | Process payment |
| GET | `/api/payments/{id}` | payment-service | Payment detail |
| GET | `/api/payments/{id}/status` | payment-service | Payment status |
| POST | `/api/payments/{id}/refund` | payment-service | Refund payment |
| GET | `/api/payments/{id}/invoice` | payment-service | Invoice |
| POST | `/api/payments/webhook` | payment-service | Webhook receiver |

That is **22 documented routes** plus health endpoints on every service,
for a total of **24+ monitored endpoints**.

## Instrumentation

Each service registers a resource with a stable `service.name`, then:

- emits **traces** (OTLPSpanExporter) for every request with status attributes
- records **metrics** (`http_requests_total`, `http_errors_total`,
  `http_request_duration_seconds`, and domain metrics such as
  `orders_created`, `payments_processed`, `products_stock`)
- ships **logs** (OTLP log exporter) through a LoggingHandler / slog bridge

The `api-gateway` additionally propagates the trace context to the backend
services so a single request forms one distributed trace.
