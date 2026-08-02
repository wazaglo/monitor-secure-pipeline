package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

var (
	reqCounter atomic.Int64
	errCounter atomic.Int64
	startTime  = time.Now()
	logger     *slog.Logger
)

const otelEndpoint = "otelcol:4317"

func main() {
	ctx := context.Background()

	res := resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceName("api-gateway"),
		semconv.ServiceVersion("1.0.0"),
	)

	// Traces
	traceExp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(otelEndpoint))
	if err != nil {
		slog.Error("trace exporter", "error", err)
		os.Exit(1)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Metrics
	metricExp, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithInsecure(), otlpmetricgrpc.WithEndpoint(otelEndpoint))
	if err != nil {
		slog.Error("metric exporter", "error", err)
		os.Exit(1)
	}
	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExp, metric.WithInterval(5*time.Second))),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// Logs via otelslog bridge -> global LoggerProvider
	logExp, err := otlploggrpc.New(ctx, otlploggrpc.WithInsecure(), otlploggrpc.WithEndpoint(otelEndpoint))
	if err != nil {
		slog.Error("log exporter", "error", err)
		os.Exit(1)
	}
	lp := log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(logExp)),
		log.WithResource(res),
	)
	global.SetLoggerProvider(lp)
	logger = slog.New(otelslog.NewHandler("api-gateway"))
	mux := http.NewServeMux()
	mux.Handle("/health", otelhttp.NewHandler(http.HandlerFunc(handleHealth), "GET /health"))
	mux.Handle("/api/info", otelhttp.NewHandler(http.HandlerFunc(handleInfo), "GET /api/info"))
	for _, r := range []struct {
		apiRoute string
		backend  string
		base     string
		svcRoute string
	}{
		{"/api/users", "user-service", "http://user-service:8001", "/users"},
		{"/api/products", "product-service", "http://product-service:8002", "/products"},
		{"/api/orders", "order-service", "http://order-service:8003", "/orders"},
		{"/api/payments", "payment-service", "http://payment-service:8004", "/payments"},
	} {
		h := otelhttp.NewHandler(proxy(r.backend, r.base, r.apiRoute, r.svcRoute), "proxy "+r.apiRoute)
		mux.Handle(r.apiRoute, h)
		mux.Handle(r.apiRoute+"/", h)
	}

	handler := otelhttp.NewHandler(mux, "api-gateway.request",
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents))

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("api-gateway listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	}()

	<-stop
	logger.Info("shutting down api-gateway")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = tp.Shutdown(context.Background())
	_ = mp.Shutdown(context.Background())
	_ = lp.Shutdown(context.Background())
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	reqCounter.Add(1)
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func handleInfo(w http.ResponseWriter, r *http.Request) {
	reqCounter.Add(1)
	writeJSON(w, http.StatusOK, map[string]any{
		"service":    "api-gateway",
		"version":    "1.0.0",
		"uptime":     time.Since(startTime).String(),
		"services":   []string{"user-service", "product-service", "order-service", "payment-service"},
		"endpoints":  24,
		"git_sha":    os.Getenv("GIT_SHA"),
		"build_time": os.Getenv("BUILD_TIME"),
	})
}

func proxy(name, base, apiRoute, svcRoute string) http.HandlerFunc {
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	tracer := otel.Tracer("api-gateway")
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "api-gateway.call."+name)
		defer span.End()

		reqCounter.Add(1)
		span.SetAttributes(
			attribute.String("http.route", r.URL.Path),
			attribute.String("backend", name),
		)

		trimmed := strings.TrimPrefix(r.URL.Path, apiRoute)
		target := base + svcRoute + trimmed
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}

		req, err := http.NewRequestWithContext(ctx, r.Method, target, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for k, vv := range r.Header {
			for _, v := range vv {
				req.Header.Add(k, v)
			}
		}
		req.Header.Set("X-Api-Gateway", "api-gateway")

		resp, err := client.Do(req)
		if err != nil {
			errCounter.Add(1)
			span.RecordError(err)
			logger.Error("backend unreachable", "backend", name, "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": "backend unreachable", "service": name})
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)

		span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
		if resp.StatusCode >= 500 {
			errCounter.Add(1)
			logger.Error("upstream error", "backend", name, "status", resp.StatusCode)
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
