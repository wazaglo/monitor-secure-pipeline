import logging
import random
import time
import urllib.error
import urllib.request

from opentelemetry import trace, metrics
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.exporter.otlp.proto.grpc.metric_exporter import OTLPMetricExporter
from opentelemetry.sdk._logs import LoggerProvider, LoggingHandler
from opentelemetry.sdk._logs.export import BatchLogRecordProcessor
from opentelemetry.exporter.otlp.proto.grpc._log_exporter import OTLPLogExporter
from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator

OTEL_HOST = "otelcol"
OTEL_PORT = 4317

resource = Resource.create({"service.name": "load-generator"})

trace.set_tracer_provider(TracerProvider(resource=resource))
trace.get_tracer_provider().add_span_processor(
    BatchSpanProcessor(OTLPSpanExporter(endpoint=f"{OTEL_HOST}:{OTEL_PORT}", insecure=True))
)

metric_reader = PeriodicExportingMetricReader(
    OTLPMetricExporter(endpoint=f"{OTEL_HOST}:{OTEL_PORT}", insecure=True),
    export_interval_millis=5000,
)
metrics.set_meter_provider(MeterProvider(resource=resource, metric_readers=[metric_reader]))

log_provider = LoggerProvider(resource=resource)
log_provider.add_log_record_processor(
    BatchLogRecordProcessor(OTLPLogExporter(endpoint=f"{OTEL_HOST}:{OTEL_PORT}", insecure=True))
)
logging.getLogger().addHandler(LoggingHandler(logger_provider=log_provider))
logging.getLogger().setLevel(logging.INFO)

tracer = trace.get_tracer("load-generator")
meter = metrics.get_meter("load-generator")

requests_counter = meter.create_counter("loadgen.requests_total", description="Loadgen requests")
errors_counter = meter.create_counter("loadgen.errors_total", description="Loadgen errors")

GATEWAY = "http://api-gateway:8080"

# (method, path, body, weight, status_codes)
ENDPOINTS = [
    ("GET", "/health", None, 4, [200]),
    ("GET", "/api/info", None, 2, [200]),
    ("GET", "/api/users", None, 6, [200]),
    ("GET", "/api/users/1", None, 5, [200]),
    ("GET", "/api/users/2", None, 5, [200]),
    ("GET", "/api/users/99", None, 2, [404]),
    ("GET", "/api/users/1/profile", None, 3, [200]),
    ("GET", "/api/users/1/settings", None, 3, [200]),
    ("GET", "/api/users/search?q=alice", None, 2, [200]),
    ("GET", "/api/users/notanumber", None, 1, [400]),
    ("GET", "/api/products", None, 6, [200]),
    ("GET", "/api/products?category=displays", None, 2, [200]),
    ("GET", "/api/products/1", None, 5, [200]),
    ("GET", "/api/products/3", None, 5, [200]),
    ("GET", "/api/products/1/inventory", None, 2, [200]),
    ("GET", "/api/products/999", None, 1, [404]),
    ("GET", "/api/orders", None, 6, [200]),
    ("GET", "/api/orders/ORD-0001", None, 4, [200]),
    ("GET", "/api/orders/ORD-0001/status", None, 3, [200]),
    ("GET", "/api/orders/ORD-9999", None, 1, [404]),
    ("GET", "/api/payments", None, 6, [200]),
    ("GET", "/api/payments/1", None, 4, [200]),
    ("GET", "/api/payments/1/status", None, 3, [200]),
    ("GET", "/api/payments/1/invoice", None, 2, [200]),
    ("GET", "/api/payments/999", None, 1, [404]),
    ("POST", "/api/orders", {"user_id": 1, "product_id": 2, "quantity": 1}, 4, [201]),
    ("POST", "/api/payments", {"order_id": "ORD-0001", "amount": 49.99, "method": "card"}, 3, [201]),
    ("POST", "/api/payments/2/refund", None, 1, [200]),
    ("POST", "/api/payments/webhook", {"event": "payment.settled", "id": 123}, 1, [200]),
    ("POST", "/api/orders/ORD-0001/cancel", None, 1, [200]),
]


def call(method, path, body):
    url = GATEWAY + path
    data = None
    headers = {}
    if body is not None:
        import json

        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"

    carrier = {}
    TraceContextTextMapPropagator().inject(carrier)
    headers.update(carrier)

    req = urllib.request.Request(url, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            return resp.status
    except urllib.error.HTTPError as e:
        return e.code
    except Exception as e:
        return None


def main():
    logging.info("load-generator started — driving traffic through api-gateway")
    while True:
        with tracer.start_as_current_span("loadgen.request") as span:
            method, path, body, weight, codes = random.choices(ENDPOINTS, weights=[w for *_, w, _ in ENDPOINTS])[0]
            status = call(method, path, body)
            span.set_attribute("http.method", method)
            span.set_attribute("http.route", path)
            span.set_attribute("http.status_code", status if status else 0)
            requests_counter.add(1, {"method": method, "route": path})
            if status is None or status >= 500:
                errors_counter.add(1, {"route": path})
                span.set_status(trace.Status(trace.StatusCode.ERROR))
                logging.error("loadgen request failed", extra={"route": path, "status": status})
            else:
                if status == 404:
                    logging.warning("resource not found", extra={"route": path, "status": status})
                elif status >= 400:
                    logging.info("client error", extra={"route": path, "status": status})
        time.sleep(random.uniform(0.4, 1.2))


if __name__ == "__main__":
    main()
