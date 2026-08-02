import json
import logging
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from opentelemetry import trace, metrics
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics._internal.view import View
from opentelemetry.sdk.metrics._internal.aggregation import ExplicitBucketHistogramAggregation
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.exporter.otlp.proto.grpc.metric_exporter import OTLPMetricExporter
from opentelemetry.sdk._logs import LoggerProvider, LoggingHandler
from opentelemetry.sdk._logs.export import BatchLogRecordProcessor
from opentelemetry.exporter.otlp.proto.grpc._log_exporter import OTLPLogExporter
from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator
from opentelemetry.context import attach, detach

OTEL_HOST = "otelcol"
OTEL_PORT = 4317

resource = Resource.create({"service.name": "payment-service"})

trace.set_tracer_provider(TracerProvider(resource=resource))
trace.get_tracer_provider().add_span_processor(
    BatchSpanProcessor(OTLPSpanExporter(endpoint=f"{OTEL_HOST}:{OTEL_PORT}", insecure=True))
)

metric_reader = PeriodicExportingMetricReader(
    OTLPMetricExporter(endpoint=f"{OTEL_HOST}:{OTEL_PORT}", insecure=True),
    export_interval_millis=5000,
)
latency_view = View(
    instrument_name="http.request_duration_seconds",
    aggregation=ExplicitBucketHistogramAggregation(
        [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0]
    ),
)
metrics.set_meter_provider(
    MeterProvider(resource=resource, metric_readers=[metric_reader], views=[latency_view])
)

log_provider = LoggerProvider(resource=resource)
log_provider.add_log_record_processor(
    BatchLogRecordProcessor(OTLPLogExporter(endpoint=f"{OTEL_HOST}:{OTEL_PORT}", insecure=True))
)
logging.getLogger().addHandler(LoggingHandler(logger_provider=log_provider))
logging.getLogger().setLevel(logging.INFO)

tracer = trace.get_tracer("payment-service")
meter = metrics.get_meter("payment-service")

request_counter = meter.create_counter("http.requests_total", description="HTTP requests")
error_counter = meter.create_counter("http.errors_total", description="HTTP errors")
payment_counter = meter.create_counter("payments.processed_total", description="Payments processed")
amount_histogram = meter.create_histogram(
    "payments.amount", description="Payment amount", unit="USD"
)
latency_histogram = meter.create_histogram(
    "http.request_duration_seconds", description="Request latency", unit="s"
)

PAYMENTS = [
    {"id": 1, "order_id": "ORD-0001", "amount": 149.99, "status": "settled", "method": "card"},
    {"id": 2, "order_id": "ORD-0002", "amount": 89.50, "status": "settled", "method": "card"},
    {"id": 3, "order_id": "ORD-0003", "amount": 12.99, "status": "pending", "method": "paypal"},
    {"id": 4, "order_id": "ORD-0004", "amount": 259.00, "status": "settled", "method": "card"},
    {"id": 5, "order_id": "ORD-0005", "amount": 45.75, "status": "refunded", "method": "card"},
]


class PaymentHandler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, format, *args):
        pass

    def do_GET(self):
        path = self.path.split("?")[0]
        start = time.monotonic()
        token = self._attach_context()
        try:
            with tracer.start_as_current_span(
                f"payment-service.{self.command} {path}",
                attributes={"http.method": self.command, "http.route": path},
            ) as span:
                status = self._route(path)
                self._record(span, path, start, status)
        finally:
            detach(token)

    def do_POST(self):
        path = self.path.split("?")[0]
        start = time.monotonic()
        token = self._attach_context()
        try:
            with tracer.start_as_current_span(
                f"payment-service.{self.command} {path}",
                attributes={"http.method": self.command, "http.route": path},
            ) as span:
                status = self._route(path)
                self._record(span, path, start, status)
        finally:
            detach(token)

    def _attach_context(self):
        carrier = {}
        for key, value in self.headers.items():
            carrier[key.lower()] = value
        return attach(TraceContextTextMapPropagator().extract(carrier))

    def _route(self, path):
        if path == "/health":
            return self._send(200, {"status": "ok"})

        if path == "/payments":
            if self.command == "GET":
                return self._send(200, {"count": len(PAYMENTS), "payments": PAYMENTS})
            length = int(self.headers.get("Content-Length", 0))
            raw = self.rfile.read(length) if length else b"{}"
            try:
                body = json.loads(raw or b"{}")
            except json.JSONDecodeError:
                return self._send(400, {"error": "invalid json body"})
            pid = len(PAYMENTS) + 1
            payment = {
                "id": pid,
                "order_id": body.get("order_id", f"ORD-{pid:04d}"),
                "amount": body.get("amount", 0),
                "status": "settled",
                "method": body.get("method", "card"),
            }
            PAYMENTS.append(payment)
            payment_counter.add(1, {"method": payment["method"]})
            amount_histogram.record(payment["amount"])
            logging.info(
                "payment processed",
                extra={"payment_id": pid, "amount": payment["amount"], "method": payment["method"]},
            )
            return self._send(201, payment)

        if path == "/payments/webhook":
            length = int(self.headers.get("Content-Length", 0))
            raw = self.rfile.read(length) if length else b"{}"
            logging.info("payment webhook received", extra={"payload": raw[:200].decode("utf-8", "replace")})
            return self._send(200, {"status": "received"})

        parts = path.strip("/").split("/")
        if len(parts) == 2 and parts[0] == "payments":
            try:
                pid = int(parts[1])
            except ValueError:
                return self._send(400, {"error": "invalid payment id"})
            payment = next((p for p in PAYMENTS if p["id"] == pid), None)
            if payment is None:
                return self._send(404, {"error": "payment not found", "id": pid})
            return self._send(200, payment)

        if len(parts) == 3 and parts[0] == "payments":
            try:
                pid = int(parts[1])
            except ValueError:
                return self._send(400, {"error": "invalid payment id"})
            payment = next((p for p in PAYMENTS if p["id"] == pid), None)
            if payment is None:
                return self._send(404, {"error": "payment not found", "id": pid})
            sub = parts[2]
            if sub == "status":
                return self._send(200, {"id": pid, "status": payment["status"]})
            if sub == "refund" and self.command == "POST":
                payment["status"] = "refunded"
                logging.info("payment refunded", extra={"payment_id": pid, "amount": payment["amount"]})
                return self._send(200, {"id": pid, "status": payment["status"], "refunded": True})
            if sub == "invoice":
                return self._send(
                    200,
                    {"id": pid, "amount": payment["amount"], "invoice_number": f"INV-{pid:06d}"},
                )
            return self._send(404, {"error": "unknown resource", "sub": sub})

        return self._send(404, {"error": "not found"})

    def _send(self, status, payload):
        body = json.dumps(payload).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)
        return status

    def _record(self, span, path, start, status):
        latency = time.monotonic() - start
        span.set_attribute("http.status_code", status)
        if status >= 400:
            span.set_status(trace.Status(trace.StatusCode.ERROR))
            error_counter.add(1, {"route": path, "status": str(status)})
        request_counter.add(1, {"route": path, "status": str(status)})
        latency_histogram.record(latency, {"route": path})


def serve():
    server = ThreadingHTTPServer(("0.0.0.0", 8004), PaymentHandler)
    logging.info("payment-service listening on :8004")
    server.serve_forever()


if __name__ == "__main__":
    serve()
