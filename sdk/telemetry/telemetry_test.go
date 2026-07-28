package telemetry

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func newForTest(t *testing.T, cfg Config, opts ...Option) *Telemetry {
	t.Helper()
	tel, err := New(t.Context(), "sdk-telemetry-test", cfg, append(opts, WithLogger(discardLogger()))...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := tel.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
	return tel
}

// newTolerant builds a Telemetry and registers a cleanup that ignores the
// shutdown error, for tests that point exporters at an unreachable endpoint (the
// final flush legitimately fails there).
func newTolerant(t *testing.T, cfg Config, opts ...Option) *Telemetry {
	t.Helper()
	tel, err := New(t.Context(), "sdk-telemetry-test", cfg, append(opts, WithLogger(discardLogger()))...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tel.Shutdown(ctx)
	})
	return tel
}

func scrape(t *testing.T, tel *Telemetry) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	tel.MetricsHandler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/metrics", nil))
	return rec.Code, rec.Body.String()
}

func TestNewEmptyServiceName(t *testing.T) {
	if _, err := New(t.Context(), "", Config{}); err == nil {
		t.Fatal("expected error for empty service name")
	}
}

func TestNewNoEndpointNonRecordingSpan(t *testing.T) {
	tel := newForTest(t, Config{})
	_, span := tel.Tracer("test").Start(t.Context(), "s")
	defer span.End()
	if span.IsRecording() {
		t.Error("span must be non-recording without an exporter")
	}
	if !span.SpanContext().TraceID().IsValid() {
		t.Error("non-recording span must still carry a valid trace id")
	}
}

func TestNewDefaultNeverSamplesWithEndpoint(t *testing.T) {
	// Endpoint set but no sampler configured: the exporter exists yet the default
	// ratio is 0, so spans must stay non-recording. Metrics/logs OTLP disabled to
	// keep the test off the network.
	tel := newForTest(t, Config{
		Endpoint:        "http://localhost:4317",
		MetricsExporter: "none",
		LogsExporter:    "none",
	})
	_, span := tel.Tracer("test").Start(t.Context(), "s")
	defer span.End()
	if span.IsRecording() {
		t.Error("span must be non-recording with the default 0 sample ratio")
	}
}

func TestNewTracesExporterNone(t *testing.T) {
	tel := newForTest(t, Config{
		Endpoint:        "http://localhost:4317",
		TracesExporter:  "none",
		MetricsExporter: "none",
		LogsExporter:    "none",
		TracesSampler:   "always_on",
	})
	_, span := tel.Tracer("test").Start(t.Context(), "s")
	defer span.End()
	if span.IsRecording() {
		t.Error("span must be non-recording when the trace exporter is \"none\"")
	}
}

func TestMetricsHandlerServesRuntimeMetrics(t *testing.T) {
	tel := newForTest(t, Config{})
	code, body := scrape(t, tel)
	if code != http.StatusOK {
		t.Fatalf("scrape status = %d, want 200", code)
	}
	if !strings.Contains(body, "goroutine") {
		t.Errorf("scrape output missing runtime goroutine metric:\n%s", body)
	}
	// go.schedule.duration comes from the runtime producer on the reader, not from
	// runtime.Start; its absence means the producer wiring was dropped.
	if !strings.Contains(body, "go_schedule_duration") {
		t.Errorf("scrape output missing runtime producer schedule-duration histogram:\n%s", body)
	}
}

func TestWithRuntimeMetricsOff(t *testing.T) {
	tel := newForTest(t, Config{}, WithRuntimeMetrics(false))
	code, body := scrape(t, tel)
	if code != http.StatusOK {
		t.Fatalf("scrape status = %d, want 200", code)
	}
	if strings.Contains(body, "goroutine") {
		t.Errorf("runtime metrics disabled but goroutine metric present:\n%s", body)
	}
	if strings.Contains(body, "go_schedule_duration") {
		t.Errorf("runtime metrics disabled but schedule-duration histogram present:\n%s", body)
	}
}

func TestNewRecordsInstrumentOnScrape(t *testing.T) {
	// An instrument built on the instance meter records to the always-on Prometheus
	// reader even without Publish, proving the isolated instance is usable.
	tel := newForTest(t, Config{})
	ctr, err := tel.Meter("test").Int64Counter("app.things")
	if err != nil {
		t.Fatalf("Int64Counter() error = %v", err)
	}
	ctr.Add(t.Context(), 1)

	code, body := scrape(t, tel)
	if code != http.StatusOK {
		t.Fatalf("scrape status = %d, want 200", code)
	}
	if !strings.Contains(body, "app_things") {
		t.Errorf("scrape output missing the app.things instrument:\n%s", body)
	}
}

func TestNewRepeatedNoDuplicateRegister(t *testing.T) {
	// A fresh Prometheus registry per New keeps repeated builds free of
	// duplicate-registration panics; each must serve /metrics.
	for i := range 2 {
		tel := newForTest(t, Config{})
		if code, _ := scrape(t, tel); code != http.StatusOK {
			t.Errorf("build %d scrape status = %d, want 200", i, code)
		}
	}
}

func TestNewUnsupportedProtocol(t *testing.T) {
	_, err := New(t.Context(), "svc", Config{
		Endpoint: "http://localhost:4317",
		Protocol: "carrier-pigeon",
	}, WithLogger(discardLogger()))
	if err == nil {
		t.Fatal("expected error for unsupported OTLP protocol")
	}
}

func TestNewResourceServiceNameVersionAndAttributes(t *testing.T) {
	res, err := newResource(t.Context(), "svc", "1.2.3",
		Config{ResourceAttributes: "deployment.environment=prod,team=core"}, nil)
	if err != nil {
		t.Fatalf("newResource() error = %v", err)
	}
	got := resourceAttrs(res.Attributes())
	if got["service.name"] != "svc" {
		t.Errorf("service.name = %q, want svc", got["service.name"])
	}
	if got["service.version"] != "1.2.3" {
		t.Errorf("service.version = %q, want 1.2.3", got["service.version"])
	}
	if got["deployment.environment"] != "prod" || got["team"] != "core" {
		t.Errorf("resource attributes = %v, want deployment.environment=prod team=core", got)
	}
}

func TestNewResourceOmitsEmptyVersion(t *testing.T) {
	res, err := newResource(t.Context(), "svc", "", Config{}, nil)
	if err != nil {
		t.Fatalf("newResource() error = %v", err)
	}
	if _, ok := resourceAttrs(res.Attributes())["service.version"]; ok {
		t.Error("service.version must be absent when version is empty")
	}
}

func TestNewResourceMergesEnvAndConfigAttributes(t *testing.T) {
	// The environment (WithFromEnv) and Config.ResourceAttributes merge; the config
	// wins on key conflicts because it is applied last.
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "team=platform,region=eu")
	res, err := newResource(t.Context(), "svc", "", Config{ResourceAttributes: "team=core,tier=gold"}, nil)
	if err != nil {
		t.Fatalf("newResource() error = %v", err)
	}
	got := resourceAttrs(res.Attributes())
	if got["team"] != "core" {
		t.Errorf("team = %q, want core (config wins over env)", got["team"])
	}
	if got["region"] != "eu" {
		t.Errorf("region = %q, want eu (env-only)", got["region"])
	}
	if got["tier"] != "gold" {
		t.Errorf("tier = %q, want gold (config-only)", got["tier"])
	}
}

func resourceAttrs(kvs []attribute.KeyValue) map[string]string {
	out := map[string]string{}
	for _, kv := range kvs {
		out[string(kv.Key)] = kv.Value.String()
	}
	return out
}

func TestEndSpanRecordsError(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	_, span := tp.Tracer("test").Start(context.Background(), "op")
	err := errors.New("boom")
	EndSpan(span, &err)

	ended := sr.Ended()
	if len(ended) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(ended))
	}
	if got := ended[0].Status().Code; got != codes.Error {
		t.Errorf("status code = %v, want Error", got)
	}
	if got := ended[0].Status().Description; got != "boom" {
		t.Errorf("status description = %q, want boom", got)
	}
	if len(ended[0].Events()) == 0 {
		t.Error("expected the error recorded as a span event")
	}
}

func TestEndSpanNilErrorNoStatus(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	_, span1 := tp.Tracer("test").Start(context.Background(), "nil-pointer")
	EndSpan(span1, nil)
	var noErr error
	_, span2 := tp.Tracer("test").Start(context.Background(), "nil-error")
	EndSpan(span2, &noErr)

	for _, s := range sr.Ended() {
		if s.Status().Code == codes.Error {
			t.Errorf("span %q must not carry an Error status", s.Name())
		}
	}
}

func TestNewBuildsExportersGRPC(t *testing.T) {
	// All signals enabled over the default grpc transport exercises every exporter
	// builder, the batcher/periodic-reader/log-processor wiring, and the span
	// processor, view and resource-attribute options.
	sr := tracetest.NewSpanRecorder()
	view := sdkmetric.NewView(
		sdkmetric.Instrument{Name: "test.hist"},
		sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{Boundaries: []float64{1, 2, 3}}},
	)
	tel := newTolerant(t, Config{Endpoint: "localhost:4317", TracesSampler: "always_on"},
		WithSpanProcessor(sr), WithView(view), WithResourceAttributes(attribute.String("extra", "1")))

	hist, err := tel.Meter("test").Int64Histogram("test.hist")
	if err != nil {
		t.Fatalf("Int64Histogram() error = %v", err)
	}
	hist.Record(t.Context(), 2)

	_, span := tel.Tracer("test").Start(t.Context(), "s")
	span.End()
	if len(sr.Ended()) == 0 {
		t.Error("WithSpanProcessor: recorder saw no spans")
	}

	code, body := scrape(t, tel)
	if code != http.StatusOK {
		t.Fatalf("scrape status = %d, want 200", code)
	}
	if !strings.Contains(body, "test_hist") {
		t.Errorf("scrape output missing the test.hist instrument:\n%s", body)
	}
}

func TestNewBuildsExportersHTTP(t *testing.T) {
	// All signals over http/protobuf exercises the HTTP exporter branches.
	tel := newTolerant(t, Config{
		Endpoint:      "localhost:4318",
		Protocol:      "http/protobuf",
		TracesSampler: "always_on",
	})
	if tel.TracerProvider() == nil || tel.MeterProvider() == nil || tel.LoggerProvider() == nil {
		t.Error("providers must be non-nil")
	}
}

func TestNewAcceptsHTTPProtocolAlias(t *testing.T) {
	// "http" is a lenient alias for "http/protobuf"; the exporter build must accept
	// it rather than fail (which would crash a connector at boot).
	tel := newTolerant(t, Config{Endpoint: "localhost:4318", Protocol: "http", TracesSampler: "always_on"})
	if tel.TracerProvider() == nil || tel.MeterProvider() == nil || tel.LoggerProvider() == nil {
		t.Error("providers must be non-nil for the http protocol alias")
	}
}

func TestInitPublishesAndAccessors(t *testing.T) {
	tel, err := Init(t.Context(), "sdk-telemetry-test", Config{}, WithLogger(discardLogger()))
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := tel.Shutdown(ctx); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
	if tel.ScopeName() != "sdk-telemetry-test" {
		t.Errorf("ScopeName() = %q, want sdk-telemetry-test", tel.ScopeName())
	}
	if tel.Config().ServiceName != "" {
		t.Errorf("Config().ServiceName = %q, want empty", tel.Config().ServiceName)
	}
	if tel.Tracer("x") == nil || tel.Meter("x") == nil {
		t.Error("Tracer/Meter must be non-nil")
	}
	// Init publishes, so the global propagator is installed.
	if !slices.Contains(otel.GetTextMapPropagator().Fields(), "traceparent") {
		t.Error("Init did not publish the trace-context propagator")
	}
}

func TestNewAppliesVersionAndResourceAttributes(t *testing.T) {
	// WithVersion and WithResourceAttributes must land on the resource. The
	// Prometheus target_info metric carries the resource attributes as labels.
	tel := newForTest(t, Config{},
		WithVersion("9.9.9"), WithResourceAttributes(attribute.String("extra_attr", "xyz")))
	code, body := scrape(t, tel)
	if code != http.StatusOK {
		t.Fatalf("scrape status = %d, want 200", code)
	}
	if !strings.Contains(body, `service_version="9.9.9"`) {
		t.Errorf("target_info missing service_version from WithVersion:\n%s", body)
	}
	if !strings.Contains(body, `extra_attr="xyz"`) {
		t.Errorf("target_info missing attribute from WithResourceAttributes:\n%s", body)
	}
}

func TestConfigServiceNameOverridesScope(t *testing.T) {
	tel := newForTest(t, Config{ServiceName: "override"})
	if tel.ScopeName() != "override" {
		t.Errorf("ScopeName() = %q, want override from Config.ServiceName", tel.ScopeName())
	}
}

func TestNewWarnsOnBadInsecure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	tel, err := New(t.Context(), "svc", Config{Insecure: "garbage"}, WithLogger(logger))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = tel.Shutdown(context.Background()) })
	if !strings.Contains(buf.String(), "invalid otel-exporter-otlp-insecure") {
		t.Errorf("expected a warning about the invalid insecure flag, got:\n%s", buf.String())
	}
}

type panicError struct{}

func (panicError) Error() string { panic("boom in Error()") }

func TestEndSpanRecoversPanic(t *testing.T) {
	// A telemetry helper must never crash the caller's request path: EndSpan
	// recovers a panic raised while recording the error and still ends the span.
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	_, span := tp.Tracer("test").Start(context.Background(), "op")
	err := error(panicError{})
	EndSpan(span, &err) // must not panic

	if len(sr.Ended()) != 1 {
		t.Errorf("recorded %d spans, want 1 (span must still end after a recovered panic)", len(sr.Ended()))
	}
}

func TestPublishPropagatorBaggageOff(t *testing.T) {
	tel := newForTest(t, Config{})
	tel.Publish()
	fields := otel.GetTextMapPropagator().Fields()
	if !slices.Contains(fields, "traceparent") {
		t.Errorf("propagator fields = %v, want traceparent", fields)
	}
	if slices.Contains(fields, "baggage") {
		t.Errorf("propagator fields = %v, want no baggage by default", fields)
	}
}

func TestPublishPropagatorBaggageOn(t *testing.T) {
	tel := newForTest(t, Config{}, WithBaggage(true))
	tel.Publish()
	fields := otel.GetTextMapPropagator().Fields()
	if !slices.Contains(fields, "traceparent") || !slices.Contains(fields, "baggage") {
		t.Errorf("propagator fields = %v, want traceparent and baggage", fields)
	}
}
