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

resource = Resource.create({"service.name": "user-service"})

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

tracer = trace.get_tracer("user-service")
meter = metrics.get_meter("user-service")

request_counter = meter.create_counter("http.requests_total", description="HTTP requests")
error_counter = meter.create_counter("http.errors_total", description="HTTP errors")
latency_histogram = meter.create_histogram(
    "http.request_duration_seconds", description="Request latency", unit="s"
)

USERS = [
    {"id": 1, "name": "Alice Chen", "email": "alice@example.com", "role": "admin"},
    {"id": 2, "name": "Bob Martinez", "email": "bob@example.com", "role": "user"},
    {"id": 3, "name": "Carol Nguyen", "email": "carol@example.com", "role": "user"},
    {"id": 4, "name": "Dave Wilson", "email": "dave@example.com", "role": "ops"},
    {"id": 5, "name": "Eve Patel", "email": "eve@example.com", "role": "user"},
]


class UserHandler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, format, *args):
        pass

    def do_GET(self):
        path = self.path.split("?")[0]
        start = time.monotonic()
        carrier = {}
        for key, value in self.headers.items():
            carrier[key.lower()] = value
        ctx = TraceContextTextMapPropagator().extract(carrier)
        token = attach(ctx)
        try:
            with tracer.start_as_current_span(
                f"user-service.{self.command} {path}",
                attributes={"http.method": self.command, "http.route": path},
            ) as span:
                status = self._route(path)
                self._record(span, path, start, status)
        finally:
            detach(token)

    def _route(self, path):
        if path == "/health":
            return self._send(200, {"status": "ok"})
        if path == "/users":
            return self._send(200, {"count": len(USERS), "users": USERS})
        if path == "/users/search":
            q = self.path.split("q=")[-1].split("&")[0]
            results = [u for u in USERS if q.lower() in u["name"].lower()]
            return self._send(200, {"query": q, "count": len(results), "users": results})

        parts = path.strip("/").split("/")
        if len(parts) == 2 and parts[0] == "users":
            try:
                uid = int(parts[1])
            except ValueError:
                return self._send(400, {"error": "invalid user id"})
            user = next((u for u in USERS if u["id"] == uid), None)
            if user is None:
                logging.warning("user not found", extra={"user_id": uid, "route": path})
                return self._send(404, {"error": "user not found", "id": uid})
            return self._send(200, user)

        if len(parts) == 3 and parts[0] == "users":
            try:
                uid = int(parts[1])
            except ValueError:
                return self._send(400, {"error": "invalid user id"})
            user = next((u for u in USERS if u["id"] == uid), None)
            if user is None:
                return self._send(404, {"error": "user not found", "id": uid})
            if parts[2] == "profile":
                return self._send(200, {"id": uid, "name": user["name"], "role": user["role"]})
            if parts[2] == "settings":
                return self._send(200, {"id": uid, "notifications": True, "theme": "dark"})
            return self._send(404, {"error": "unknown resource", "sub": parts[2]})

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
        logging.info(
            "request served",
            extra={"route": path, "status": status, "latency": round(latency, 4)},
        )


def serve():
    server = ThreadingHTTPServer(("0.0.0.0", 8001), UserHandler)
    logging.info("user-service listening on :8001")
    server.serve_forever()


if __name__ == "__main__":
    serve()
