package observe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdk "go.opentelemetry.io/otel/sdk/trace"
	"sirius.local/food/internal/eda"
)

var requests = promauto.NewCounterVec(prometheus.CounterOpts{Name: "http_requests_total", Help: "HTTP responses"}, []string{"method", "route", "status"})
var duration = promauto.NewHistogramVec(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP request duration", Buckets: prometheus.DefBuckets}, []string{"route"})
var pool atomic.Pointer[pgxpool.Pool]
var acquired = promauto.NewGaugeFunc(prometheus.GaugeOpts{Name: "db_acquired_connections", Help: "Acquired PostgreSQL connections"}, func() float64 {
	if p := pool.Load(); p != nil {
		return float64(p.Stat().AcquiredConns())
	}
	return 0
})

func SetPool(p *pgxpool.Pool) { pool.Store(p) }
func Metrics() http.Handler   { return promhttp.Handler() }

type writer struct {
	service string
	dst     io.Writer
}

func (w writer) Write(p []byte) (int, error) {
	var fields map[string]any
	if json.Unmarshal(p, &fields) != nil || fields == nil {
		fields = map[string]any{"message": string(p)}
	}
	fields["service"] = w.service
	fields["time"] = time.Now().UTC()
	b, e := json.Marshal(fields)
	if e != nil {
		return 0, e
	}
	_, e = w.dst.Write(append(b, '\n'))
	return len(p), e
}
func Install(name string) func() {
	if len(os.Args) > 1 {
		name += "-" + os.Args[1]
	}
	log.SetFlags(0)
	log.SetOutput(writer{name, os.Stdout})
	otel.SetTextMapPropagator(propagation.TraceContext{})
	exporter, e := otlptracehttp.New(context.Background())
	if e != nil {
		log.Print(e)
		return func() {}
	}
	provider := sdk.NewTracerProvider(sdk.WithBatcher(exporter), sdk.WithSampler(sdk.AlwaysSample()), sdk.WithResource(resource.NewSchemaless(attribute.String("service.name", name))))
	otel.SetTracerProvider(provider)
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = provider.Shutdown(ctx)
	}
}

type response struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *response) WriteHeader(status int) {
	if w.written {
		return
	}
	w.status = status
	w.written = true
	w.ResponseWriter.WriteHeader(status)
}
func (w *response) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *response) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

var valid = regexp.MustCompile(`^[a-zA-Z0-9._:-]{1,128}$`)

func HTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Correlation-ID")
		if !valid.MatchString(id) {
			id = eda.ID()
		}
		ctx := eda.WithCorrelation(r.Context(), id)
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))
		ctx, span := otel.Tracer("sirius.http").Start(ctx, r.Method+" "+r.URL.Path)
		defer span.End()
		span.SetAttributes(attribute.String("correlation_id", id))
		r = r.WithContext(ctx)
		w.Header().Set("X-Correlation-ID", id)
		rec := &response{ResponseWriter: w, status: 200}
		start := time.Now()
		next.ServeHTTP(rec, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		seconds := time.Since(start).Seconds()
		requests.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
		duration.WithLabelValues(route).Observe(seconds)
		log.Print(fmt.Sprintf(`{"event":"http_request","correlation_id":%q,"trace_id":%q,"method":%q,"route":%q,"status":%d,"duration_ms":%.3f}`, id, span.SpanContext().TraceID().String(), r.Method, route, rec.status, seconds*1000))
	})
}
