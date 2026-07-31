package slogotel

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/glimps-re/connector-integration/sdk/telemetry"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

// NewHandler assembles the common connector logging pipeline: a JSON handler on w
// (gated at level) carrying trace_id/span_id fields, fanned out alongside a
// level-gated OTLP bridge for scope. Point it at os.Stdout for the usual setup:
//
//	logger := slog.New(slogotel.NewHandler(os.Stdout, level, "my-connector", nil))
//
// provider selects the log provider the bridge feeds; nil uses the global one
// (the delegating provider that telemetry.Publish installs), so the handler can
// be built at package init before telemetry is initialized. The trace fields are
// added only to the JSON sink; the OTLP bridge carries trace context natively.
//
// Trace-handler options apply to the JSON sink, e.g.
// [WithCorrelationAttributes] to stamp the process-local correlation attributes
// onto every stdout record.
func NewHandler(w io.Writer, level slog.Leveler, scope string, provider log.LoggerProvider, opts ...TraceOption) slog.Handler {
	return NewFanoutHandler(
		NewTraceHandler(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}), opts...),
		NewLeveledHandler(level, NewBridgeHandler(scope, provider)),
	)
}

// NewBridgeHandler returns an slog.Handler that feeds records into the
// OpenTelemetry log provider under the instrumentation scope. A nil provider uses
// the global log provider, so the handler is safe to build before
// telemetry.Publish installs the real one (records before then are dropped).
func NewBridgeHandler(scope string, provider log.LoggerProvider) slog.Handler {
	if provider == nil {
		return otelslog.NewHandler(scope)
	}
	return otelslog.NewHandler(scope, otelslog.WithLoggerProvider(provider))
}

// TraceOption configures [NewTraceHandler].
type TraceOption func(*traceHandler)

// WithCorrelationAttributes makes the handler also copy the process-local
// correlation attributes (see telemetry.WithCorrelation) onto every record, so a
// connector that seeds them once per unit of work need not stamp them on each log
// call. Unseeded contexts add nothing.
func WithCorrelationAttributes() TraceOption {
	return func(h *traceHandler) { h.correlation = true }
}

// traceHandler adds trace_id and span_id, read from the record's context, to
// every record whose context carries a valid span, then delegates to the wrapped
// handler. With correlation enabled it also copies the WithCorrelation-seeded
// attributes.
type traceHandler struct {
	slog.Handler
	correlation bool
}

// NewTraceHandler wraps base so records carry trace_id/span_id when their context
// has a valid span. Call sites only need to pass the context (via slog's
// *Context log methods); no manual trace-id plumbing.
func NewTraceHandler(base slog.Handler, opts ...TraceOption) slog.Handler {
	h := traceHandler{Handler: base}
	for _, o := range opts {
		o(&h)
	}
	return h
}

func (h traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	if h.correlation {
		for _, kv := range telemetry.CorrelationFromContext(ctx) {
			r.AddAttrs(attrToSlog(kv))
		}
	}
	return h.Handler.Handle(ctx, r)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{Handler: h.Handler.WithAttrs(attrs), correlation: h.correlation}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{Handler: h.Handler.WithGroup(name), correlation: h.correlation}
}

// attrToSlog converts an OpenTelemetry attribute to a typed slog.Attr, preserving
// bool/int/float rather than stringifying them.
func attrToSlog(kv attribute.KeyValue) slog.Attr {
	key := string(kv.Key)
	switch kv.Value.Type() {
	case attribute.BOOL:
		return slog.Bool(key, kv.Value.AsBool())
	case attribute.INT64:
		return slog.Int64(key, kv.Value.AsInt64())
	case attribute.FLOAT64:
		return slog.Float64(key, kv.Value.AsFloat64())
	case attribute.STRING:
		return slog.String(key, kv.Value.AsString())
	default:
		return slog.String(key, kv.Value.String())
	}
}

// fanoutHandler dispatches every record to a set of sub-handlers, so one logger
// can write to stdout and export over OTLP at the same time. Each sub-handler
// keeps its own level gating.
type fanoutHandler struct {
	handlers []slog.Handler
}

// NewFanoutHandler returns an slog.Handler that fans records out to handlers.
func NewFanoutHandler(handlers ...slog.Handler) slog.Handler {
	return fanoutHandler{handlers: handlers}
}

// Enabled reports whether any sub-handler accepts records at level.
func (h fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, sub := range h.handlers {
		if sub.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle delivers a clone of the record to every sub-handler whose Enabled
// accepts its level, joining any errors. Cloning keeps a sub-handler that mutates
// the record (e.g. the trace handler adding fields) from affecting the others.
func (h fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, sub := range h.handlers {
		if !sub.Enabled(ctx, r.Level) {
			continue
		}
		if err := sub.Handle(ctx, r.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (h fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	next := make([]slog.Handler, len(h.handlers))
	for i, sub := range h.handlers {
		next[i] = sub.WithAttrs(attrs)
	}
	return fanoutHandler{handlers: next}
}

func (h fanoutHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := make([]slog.Handler, len(h.handlers))
	for i, sub := range h.handlers {
		next[i] = sub.WithGroup(name)
	}
	return fanoutHandler{handlers: next}
}

// leveledHandler drops records below a minimum level taken from a slog.Leveler,
// so the threshold can change at runtime. In a fanout this lets the OTLP export
// honor the same level as stdout without coupling the two sinks.
type leveledHandler struct {
	slog.Handler
	level slog.Leveler
}

// NewLeveledHandler wraps base so records below level.Level() are rejected.
func NewLeveledHandler(level slog.Leveler, base slog.Handler) slog.Handler {
	return leveledHandler{Handler: base, level: level}
}

// Enabled reports whether level clears the minimum and the wrapped handler
// accepts it.
func (h leveledHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level.Level() && h.Handler.Enabled(ctx, level)
}

func (h leveledHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return leveledHandler{Handler: h.Handler.WithAttrs(attrs), level: h.level}
}

func (h leveledHandler) WithGroup(name string) slog.Handler {
	return leveledHandler{Handler: h.Handler.WithGroup(name), level: h.level}
}
