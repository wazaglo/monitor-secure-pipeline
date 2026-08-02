package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
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
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Price       float64 `json:"price"`
	Stock       int     `json:"stock"`
	Rating      float64 `json:"rating"`
	Description string  `json:"description"`
}

var (
	products    = make(map[int]*product)
	productsMu  sync.RWMutex
	reqCounter  atomic.Int64
	errCounter  atomic.Int64
	startTime   = time.Now()
	logger      *slog.Logger
	stockMetric metric.Int64Gauge
)

const otelEndpoint = "otelcol:4317"

func main() {
	ctx := context.Background()

	res := resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceName("product-service"),
		semconv.ServiceVersion("1.0.0"),
	)

	traceExp, _ := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint(otelEndpoint))
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	metricExp, _ := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithInsecure(), otlpmetricgrpc.WithEndpoint(otelEndpoint))
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(5*time.Second))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	m := otel.Meter("product-service")
	stockMetric, _ = m.Int64Gauge("products.stock", metric.WithDescription("Current product stock"))

	logExp, _ := otlploggrpc.New(ctx, otlploggrpc.WithInsecure(), otlploggrpc.WithEndpoint(otelEndpoint))
	lp := log.NewLoggerProvider(log.WithProcessor(log.NewBatchProcessor(logExp)), log.WithResource(res))
	global.SetLoggerProvider(lp)
	logger = slog.New(otelslog.NewHandler("product-service"))

	names := []string{
		"Wireless Mouse", "Mechanical Keyboard", "4K Monitor", "USB-C Dock", "Noise-Cancelling Headphones",
		"Webcam 1080p", "Ergonomic Chair", "Standing Desk", "Laptop Stand", "External SSD 1TB",
	}
	categories := []string{"peripherals", "displays", "accessories", "furniture", "storage"}
	for i := 1; i <= len(names); i++ {
		p := &product{
			ID:          i,
			Name:        names[i-1],
			Category:    categories[i%len(categories)],
			Price:       float64(rand.Intn(400)) + 9.99,
			Stock:       rand.Intn(200),
			Rating:      float64(rand.Intn(40)+10) / 10,
			Description: "High quality product",
		}
		products[i] = p
	}

	mux := http.NewServeMux()
	mux.Handle("/health", otelhttp.NewHandler(http.HandlerFunc(handleHealth), "GET /health"))
	mux.Handle("/products", otelhttp.NewHandler(http.HandlerFunc(handleProducts), "GET /products"))
	mux.Handle("/products/", otelhttp.NewHandler(http.HandlerFunc(handleProductDetail), "/products/{id}"))

	srv := &http.Server{
		Addr:         ":8002",
		Handler:      otelhttp.NewHandler(mux, "product-service.request"),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("product-service listening on :8002")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	}()

	<-stop
	logger.Info("shutting down product-service")
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
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleProducts(w http.ResponseWriter, r *http.Request) {
	_, span := otel.Tracer("product-service").Start(r.Context(), "products.list")
	defer span.End()
	reqCounter.Add(1)

	productsMu.RLock()
	list := make([]*product, 0, len(products))
	for _, p := range products {
		list = append(list, p)
	}
	productsMu.RUnlock()

	for _, p := range list {
		stockMetric.Record(r.Context(), int64(p.Stock), metric.WithAttributes(attribute.Int("product.id", p.ID)))
	}

	// support ?category= filter
	if cat := r.URL.Query().Get("category"); cat != "" {
		filtered := make([]*product, 0)
		for _, p := range list {
			if p.Category == cat {
				filtered = append(filtered, p)
			}
		}
		list = filtered
	}

	writeJSON(w, http.StatusOK, map[string]any{"count": len(list), "products": list})
}

func handleProductDetail(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("product-service").Start(r.Context(), "products.detail")
	defer span.End()

	idStr := strings.TrimPrefix(r.URL.Path, "/products/")
	id, err := strconv.Atoi(strings.TrimSuffix(idStr, "/inventory"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid id"})
		return
	}
	span.SetAttributes(attribute.Int("product.id", id))

	productsMu.RLock()
	p, ok := products[id]
	productsMu.RUnlock()
	if !ok {
		errCounter.Add(1)
		logger.Warn("product not found", "product_id", id)
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "product not found", "id": id})
		return
	}

	if strings.HasSuffix(r.URL.Path, "/inventory") {
		stockMetric.Record(ctx, int64(p.Stock), metric.WithAttributes(attribute.Int("product.id", p.ID)))
		writeJSON(w, http.StatusOK, map[string]any{"id": p.ID, "stock": p.Stock, "low_stock": p.Stock < 20})
		return
	}

	time.Sleep(time.Duration(rand.Intn(80)) * time.Millisecond)
	writeJSON(w, http.StatusOK, p)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
