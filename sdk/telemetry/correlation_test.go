package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func corrMap(kvs []attribute.KeyValue) map[attribute.Key]attribute.Value {
	m := map[attribute.Key]attribute.Value{}
	for _, kv := range kvs {
		m[kv.Key] = kv.Value
	}
	return m
}

func TestWithCorrelationStoresAttributes(t *testing.T) {
	ctx := WithCorrelation(
		context.Background(),
		attribute.String("a", "1"),
		attribute.Bool("b", true),
		attribute.Int("c", 7),
	)
	got := corrMap(CorrelationFromContext(ctx))
	if got["a"].AsString() != "1" || !got["b"].AsBool() || got["c"].AsInt64() != 7 {
		t.Errorf("correlation = %v, want a=1 b=true c=7", CorrelationFromContext(ctx))
	}
}

func TestWithCorrelationSkipsEmptyStringKeepsTypedZero(t *testing.T) {
	ctx := WithCorrelation(
		context.Background(),
		attribute.String("empty", ""),
		attribute.Bool("flag", false),
		attribute.Int("zero", 0),
	)
	got := corrMap(CorrelationFromContext(ctx))
	if _, ok := got["empty"]; ok {
		t.Error("empty string attribute must be skipped")
	}
	if _, ok := got["flag"]; !ok {
		t.Error("false bool must be kept (not treated as empty)")
	}
	if _, ok := got["zero"]; !ok {
		t.Error("zero int must be kept (not treated as empty)")
	}
}

func TestWithCorrelationLatestKeyWins(t *testing.T) {
	ctx := WithCorrelation(context.Background(), attribute.String("k", "old"))
	ctx = WithCorrelation(ctx, attribute.String("k", "new"))
	if got := corrMap(CorrelationFromContext(ctx))["k"].AsString(); got != "new" {
		t.Errorf("k = %q, want new", got)
	}
}

func TestWithCorrelationCopyOnWrite(t *testing.T) {
	parent := WithCorrelation(context.Background(), attribute.String("a", "1"))
	// Seed two independent children from the same parent.
	child1 := WithCorrelation(parent, attribute.String("b", "2"))
	child2 := WithCorrelation(parent, attribute.String("c", "3"))

	if _, ok := corrMap(CorrelationFromContext(parent))["b"]; ok {
		t.Error("parent was mutated by a child seed")
	}
	if _, ok := corrMap(CorrelationFromContext(child1))["c"]; ok {
		t.Error("child1 leaked child2's attribute (aliased backing array)")
	}
	if _, ok := corrMap(CorrelationFromContext(child2))["b"]; ok {
		t.Error("child2 leaked child1's attribute (aliased backing array)")
	}
}

func TestWithCorrelationNoChangeReturnsSameContext(t *testing.T) {
	ctx := WithCorrelation(context.Background(), attribute.String("k", "v"))
	// Re-seeding the same value changes nothing, so the same context is returned.
	if got := WithCorrelation(ctx, attribute.String("k", "v")); got != ctx {
		t.Error("expected the same context when nothing changed")
	}
	// Seeding only an empty string also changes nothing.
	if got := WithCorrelation(ctx, attribute.String("x", "")); got != ctx {
		t.Error("expected the same context when only an empty value was seeded")
	}
}

func TestCorrelationSpanProcessorStampsSpan(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(NewCorrelationSpanProcessor()),
		sdktrace.WithSpanProcessor(sr),
	)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	ctx := WithCorrelation(
		context.Background(),
		attribute.String("gmalware.analysis.sid", "sid-1"),
		attribute.Bool("gmalware.analysis.malware", true),
	)
	_, span := tp.Tracer("t").Start(ctx, "op")
	span.End()

	ended := sr.Ended()
	if len(ended) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(ended))
	}
	got := corrMap(ended[0].Attributes())
	if got["gmalware.analysis.sid"].AsString() != "sid-1" || !got["gmalware.analysis.malware"].AsBool() {
		t.Errorf("span attributes = %v, want the seeded correlation attributes", ended[0].Attributes())
	}
}

func TestCorrelationSpanProcessorUnseededNoop(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(NewCorrelationSpanProcessor()),
		sdktrace.WithSpanProcessor(sr),
	)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	_, span := tp.Tracer("t").Start(context.Background(), "op")
	span.End()
	if got := len(sr.Ended()[0].Attributes()); got != 0 {
		t.Errorf("unseeded span has %d attributes, want 0", got)
	}
}
