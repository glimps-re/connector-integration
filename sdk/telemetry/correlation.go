package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// correlationCtxKey keys the process-local correlation attributes stored in a
// context.
type correlationCtxKey struct{}

// WithCorrelation adds correlation attributes to ctx as a process-local value,
// never W3C baggage, so nothing crosses the wire. [NewCorrelationSpanProcessor]
// stamps them on every span started from the returned context, and a logging
// bridge can copy them onto log records (see telemetry/slogotel). Empty string
// values are skipped; a repeated key takes the latest value. The attribute keys
// are the caller's own convention (e.g. sdk/gmconv); this package bakes in none.
//
// Seed it once per unit of work (e.g. per analysis):
//
//	ctx = telemetry.WithCorrelation(ctx, gmconv.AnalysisSID(sid), gmconv.FileSHA256(sum))
func WithCorrelation(ctx context.Context, kvs ...attribute.KeyValue) context.Context {
	existing := CorrelationFromContext(ctx)
	// Copy-on-write: never append onto the parent value's backing array, or a
	// child seed or a concurrent goroutine sharing the parent context could alias
	// and race it.
	merged := make([]attribute.KeyValue, len(existing))
	copy(merged, existing)
	changed := false
	for _, kv := range kvs {
		if kv.Value.Type() == attribute.STRING && kv.Value.AsString() == "" {
			continue
		}
		replaced := false
		for i := range merged {
			if merged[i].Key == kv.Key {
				if merged[i].Value != kv.Value {
					merged[i] = kv
					changed = true
				}
				replaced = true
				break
			}
		}
		if !replaced {
			merged = append(merged, kv)
			changed = true
		}
	}
	if !changed {
		return ctx
	}
	return context.WithValue(ctx, correlationCtxKey{}, merged)
}

// CorrelationFromContext returns the correlation attributes seeded via
// [WithCorrelation] on ctx, or nil. The returned slice is owned by the context
// value and must not be mutated.
func CorrelationFromContext(ctx context.Context) []attribute.KeyValue {
	kvs, _ := ctx.Value(correlationCtxKey{}).([]attribute.KeyValue)
	return kvs
}

// correlationSpanProcessor stamps the process-local correlation attributes onto
// each span at start.
type correlationSpanProcessor struct{}

// NewCorrelationSpanProcessor returns a span processor that copies the
// [WithCorrelation]-seeded attributes from the starting context onto each span.
// Register it via [WithSpanProcessor]. With no attributes seeded it costs one
// context lookup, so unseeded services pay nothing.
func NewCorrelationSpanProcessor() sdktrace.SpanProcessor {
	return correlationSpanProcessor{}
}

// OnStart mirrors the seeded correlation attributes onto the span.
func (correlationSpanProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if kvs := CorrelationFromContext(parent); len(kvs) > 0 {
		s.SetAttributes(kvs...)
	}
}

// OnEnd is a no-op; the processor only enriches spans at start.
func (correlationSpanProcessor) OnEnd(sdktrace.ReadOnlySpan) {}

// Shutdown is a no-op; the processor holds no resources.
func (correlationSpanProcessor) Shutdown(context.Context) error { return nil }

// ForceFlush is a no-op; the processor buffers nothing.
func (correlationSpanProcessor) ForceFlush(context.Context) error { return nil }
