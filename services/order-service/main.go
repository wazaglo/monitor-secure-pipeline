package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
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

type order struct {
	ID        string    `json:"id"`
	UserID    int       `json:"user_id"`
	ProductID int       `json:"product_id"`
	Quantity  int       `json:"quantity"`
	Total     float64   `json:"total"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	orders     = make(map[string]*order)
	ordersMu   sync.RWMutex
	orderIDSeq atomic.Int64
	reqCounter atomic.Int64
	errCounter atomic.Int64
	startTime  = time.Now()
	logger     *slog.Logger

	createdCounter metric.Int64Counter
	orderTotal     metric.Float64Histogram
)

const otelEndpoint = "otelcol:4317"

func main() {
	ctx := context.Background()

	res := resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceName("order-service"),
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

	m := otel.Meter("order-service")
	createdCounter, _ = m.Int64Counter("orders.created", metric.WithDescription("Orders created"))
	orderTotal, _ = m.Float64Histogram("order.total_amount", metric.WithDescription("Order total amount"), metric.WithUnit("USD"))

	logExp, _ := otlploggrpc.New(ctx, otlploggrpc.WithInsecure(), otlploggrpc.WithEndpoint(otelEndpoint))
	lp := log.NewLoggerProvider(log.WithProcessor(log.NewBatchProcessor(logExp)), log.WithResource(res))
	global.SetLoggerProvider(lp)
	logger = slog.New(otelslog.NewHandler("order-service"))

	for i := 1; i <= 5; i++ {
		seedOrder(i)
	}

	mux := http.NewServeMux()
	mux.Handle("/health", otelhttp.NewHandler(http.HandlerFunc(handleHealth), "GET /health"))
	mux.Handle("/orders", otelhttp.NewHandler(http.HandlerFunc(handleOrders), "GET/POST /orders"))
	mux.Handle("/orders/", otelhttp.NewHandler(http.HandlerFunc(handleOrderDetail), "/orders/{id}"))

	srv := &http.Server{
		Addr:         ":8003",
		Handler:      otelhttp.NewHandler(mux, "order-service.request"),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("order-service listening on :8003")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
		}
	}()

	<-stop
	logger.Info("shutting down order-service")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = tp.Shutdown(context.Background())
	_ = mp.Shutdown(context.Background())
	_ = lp.Shutdown(context.Background())
}

func seedOrder(i int) {
	o := &order{
		ID:        fmt.Sprintf("ORD-%04d", i),
		UserID:    rand.Intn(1000) + 1,
		ProductID: rand.Intn(50) + 1,
		Quantity:  rand.Intn(5) + 1,
		Total:     float64(rand.Intn(500)) + 9.99,
		Status:    "completed",
		CreatedAt: time.Now().Add(-time.Duration(i) * time.Hour),
	}
	orders[o.ID] = o
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	reqCounter.Add(1)
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleOrders(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("order-service").Start(r.Context(), "orders.list-or-create")
	defer span.End()

	switch r.Method {
	case http.MethodGet:
		reqCounter.Add(1)
		span.SetAttributes(attribute.String("http.route", "/orders"))
		ordersMu.RLock()
		list := make([]*order, 0, len(orders))
		for _, o := range orders {
			list = append(list, o)
		}
		ordersMu.RUnlock()
		writeJSON(w, http.StatusOK, map[string]any{"count": len(list), "orders": list})

	case http.MethodPost:
		reqCounter.Add(1)
		var body struct {
			UserID    int `json:"user_id"`
			ProductID int `json:"product_id"`
			Quantity  int `json:"quantity"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body"})
			return
		}
		if body.Quantity <= 0 {
			body.Quantity = 1
		}
		ordersMu.Lock()
		id := fmt.Sprintf("ORD-%05d", orderIDSeq.Add(1))
		o := &order{
			ID:        id,
			UserID:    body.UserID,
			ProductID: body.ProductID,
			Quantity:  body.Quantity,
			Total:     float64(body.Quantity) * (float64(rand.Intn(80)) + 5.99),
			Status:    "created",
			CreatedAt: time.Now(),
		}
		orders[id] = o
		ordersMu.Unlock()
		createdCounter.Add(ctx, 1)
		orderTotal.Record(ctx, o.Total)
		span.SetAttributes(attribute.String("order.id", id), attribute.Int("http.status_code", 201))
		logger.Info("order created", "order_id", id, "user_id", body.UserID, "total", o.Total)
		writeJSON(w, http.StatusCreated, o)

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
	}
}

func handleOrderDetail(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("order-service").Start(r.Context(), "orders.detail")
	defer span.End()

	id := strings.TrimPrefix(r.URL.Path, "/orders/")
	span.SetAttributes(attribute.String("order.id", id))
	ordersMu.RLock()
	o, ok := orders[id]
	ordersMu.RUnlock()
	if !ok {
		errCounter.Add(1)
		logger.Warn("order not found", "order_id", id)
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "order not found", "id": id})
		return
	}

	time.Sleep(time.Duration(rand.Intn(120)) * time.Millisecond)
	_ = ctx

	switch {
	case strings.HasSuffix(r.URL.Path, "/status"):
		writeJSON(w, http.StatusOK, map[string]any{"id": o.ID, "status": o.Status})
	case strings.HasSuffix(r.URL.Path, "/cancel") && r.Method == http.MethodPost:
		ordersMu.Lock()
		o.Status = "cancelled"
		ordersMu.Unlock()
		logger.Info("order cancelled", "order_id", o.ID)
		writeJSON(w, http.StatusOK, map[string]any{"id": o.ID, "status": o.Status})
	default:
		writeJSON(w, http.StatusOK, o)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
