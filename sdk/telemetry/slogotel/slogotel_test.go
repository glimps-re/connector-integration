package slogotel

import (
	"bytes"
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/glimps-re/connector-integration/sdk/telemetry"
	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"
)

// captureHandler records the attributes of the last record it handles.
type captureHandler struct {
	attrs map[string]string
}

func (*captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.attrs = map[string]string{}
	r.Attrs(func(a slog.Attr) bool {
		h.attrs[a.Key] = a.Value.String()
		return true
	})
	return nil
}
func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

// sampledContext returns a context carrying a valid, sampled span context.
func sampledContext() (context.Context, trace.SpanContext) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		SpanID:     trace.SpanID{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18},
		TraceFlags: trace.FlagsSampled,
	})
	return trace.ContextWithSpanContext(context.Background(), sc), sc
}

func TestNewTraceHandlerAddsFields(t *testing.T) {
	base := &captureHandler{}
	l := slog.New(NewTraceHandler(base))
	ctx, sc := sampledContext()
	l.InfoContext(ctx, "msg")

	if got := base.attrs["trace_id"]; got != sc.TraceID().String() {
		t.Errorf("trace_id = %q, want %q", got, sc.TraceID())
	}
	if got := base.attrs["span_id"]; got != sc.SpanID().String() {
		t.Errorf("span_id = %q, want %q", got, sc.SpanID())
	}
}

func TestNewTraceHandlerNoSpanNoFields(t *testing.T) {
	base := &captureHandler{}
	slog.New(NewTraceHandler(base)).InfoContext(context.Background(), "msg")
	if _, ok := base.attrs["trace_id"]; ok {
		t.Error("trace_id must not be set without a span in context")
	}
}

func TestNewTraceHandlerWithGroupStillTags(t *testing.T) {
	base := &captureHandler{}
	l := slog.New(NewTraceHandler(base)).WithGroup("app")
	ctx, _ := sampledContext()
	l.InfoContext(ctx, "msg")
	if base.attrs["trace_id"] == "" {
		t.Error("trace_id must be tagged even after WithGroup")
	}
}

func TestNewTraceHandlerCopiesCorrelation(t *testing.T) {
	base := &captureHandler{}
	h := NewTraceHandler(base, WithCorrelationAttributes())
	ctx := telemetry.WithCorrelation(context.Background(),
		attribute.String("gmalware.analysis.sid", "sid-1"),
		attribute.Bool("gmalware.analysis.malware", true),
	)
	slog.New(h).InfoContext(ctx, "msg")

	if got := base.attrs["gmalware.analysis.sid"]; got != "sid-1" {
		t.Errorf("gmalware.analysis.sid = %q, want sid-1", got)
	}
	if got := base.attrs["gmalware.analysis.malware"]; got != "true" {
		t.Errorf("gmalware.analysis.malware = %q, want true", got)
	}
}

func TestNewTraceHandlerNoCorrelationByDefault(t *testing.T) {
	base := &captureHandler{}
	ctx := telemetry.WithCorrelation(context.Background(), attribute.String("k", "v"))
	slog.New(NewTraceHandler(base)).InfoContext(ctx, "msg")
	if _, ok := base.attrs["k"]; ok {
		t.Error("correlation attributes must not be copied without WithCorrelationAttributes")
	}
}

func TestNewFanoutHandlerKeepsAllSinks(t *testing.T) {
	var buf bytes.Buffer
	h := NewFanoutHandler(
		NewTraceHandler(slog.NewJSONHandler(&buf, nil)),
		slog.DiscardHandler,
	)
	slog.New(h).WithGroup("app").Info("fanned out", slog.String("k", "v"))

	out := buf.String()
	if !strings.Contains(out, "fanned out") {
		t.Errorf("stdout output = %q, want the message", out)
	}
	if !strings.Contains(out, `"app"`) {
		t.Errorf("stdout output = %q, want the app group applied", out)
	}
}

func TestNewFanoutHandlerLevelGatingPerHandler(t *testing.T) {
	var gated, open bytes.Buffer
	h := NewFanoutHandler(
		slog.NewJSONHandler(&gated, &slog.HandlerOptions{Level: slog.LevelError}),
		slog.NewJSONHandler(&open, &slog.HandlerOptions{Level: slog.LevelDebug}),
	)
	slog.New(h).Info("selective")

	if gated.Len() != 0 {
		t.Errorf("error-gated handler received %q, want nothing", gated.String())
	}
	if !strings.Contains(open.String(), "selective") {
		t.Errorf("permissive handler output = %q, want the message", open.String())
	}
}

func TestNewFanoutHandlerEnabledAny(t *testing.T) {
	h := NewFanoutHandler(
		slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}),
		slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelInfo}),
	)
	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Enabled(Info) = false, want true when any sub-handler accepts it")
	}
	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("Enabled(Debug) = true, want false when no sub-handler accepts it")
	}
}

func TestNewFanoutHandlerCloneIsolation(t *testing.T) {
	// The trace handler mutates its record (adds trace fields); the other sink
	// must not see those mutations, thanks to per-sink record cloning.
	plain := &captureHandler{}
	h := NewFanoutHandler(
		NewTraceHandler(slog.DiscardHandler),
		plain,
	)
	ctx, _ := sampledContext()
	slog.New(h).InfoContext(ctx, "msg")

	if _, ok := plain.attrs["trace_id"]; ok {
		t.Error("plain sink saw trace_id: records are not cloned per sink")
	}
}

func TestNewLeveledHandlerGates(t *testing.T) {
	level := &slog.LevelVar{}
	level.Set(slog.LevelWarn)
	base := slog.NewJSONHandler(&bytes.Buffer{}, nil)
	h := NewLeveledHandler(level, base)

	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Enabled(Info) = true, want false below the Warn threshold")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Error("Enabled(Error) = false, want true above the threshold")
	}
	// The leveler is dynamic.
	level.Set(slog.LevelDebug)
	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Enabled(Info) = false after lowering the level to Debug")
	}
}

// recordExporter captures the bodies of exported log records.
type recordExporter struct {
	mu     sync.Mutex
	bodies []string
}

func (e *recordExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range records {
		e.bodies = append(e.bodies, r.Body().AsString())
	}
	return nil
}
func (e *recordExporter) Shutdown(context.Context) error   { return nil }
func (e *recordExporter) ForceFlush(context.Context) error { return nil }
func (e *recordExporter) all() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.bodies)
}

func TestNewBridgeHandlerRoutesToProvider(t *testing.T) {
	exp := &recordExporter{}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exp)))
	t.Cleanup(func() { _ = lp.Shutdown(context.Background()) })

	slog.New(NewBridgeHandler("test-scope", lp)).Info("bridged")

	if !slices.Contains(exp.all(), "bridged") {
		t.Errorf("bridge did not route the record to the provider; got %v", exp.all())
	}
}

func TestNewBridgeHandlerNilUsesGlobal(t *testing.T) {
	// A nil provider targets the global (delegating) provider; logging must not
	// panic before a real provider is installed.
	h := NewBridgeHandler("test-scope", nil)
	if h == nil {
		t.Fatal("NewBridgeHandler returned nil")
	}
	slog.New(h).Info("to global") // records are dropped by the global noop, but must not panic
}

func TestNewHandlerWithAttrsAndGroupPropagate(t *testing.T) {
	// logger.With(...).WithGroup(...) must re-wrap every handler in the pipeline so
	// trace tagging, the OTLP bridge and the attributes/group all survive.
	var buf bytes.Buffer
	exp := &recordExporter{}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exp)))
	t.Cleanup(func() { _ = lp.Shutdown(context.Background()) })

	level := &slog.LevelVar{}
	l := slog.New(NewHandler(&buf, level, "test-scope", lp)).
		With(slog.String("svc", "x")).
		WithGroup("g")

	ctx, sc := sampledContext()
	l.InfoContext(ctx, "grouped")

	out := buf.String()
	for _, want := range []string{"grouped", `"svc":"x"`, `"g":`, sc.TraceID().String()} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout sink = %q, want it to contain %q", out, want)
		}
	}
	if !slices.Contains(exp.all(), "grouped") {
		t.Errorf("OTLP bridge did not receive the grouped record; got %v", exp.all())
	}
}

func TestNewHandlerPipeline(t *testing.T) {
	var buf bytes.Buffer
	exp := &recordExporter{}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exp)))
	t.Cleanup(func() { _ = lp.Shutdown(context.Background()) })

	level := &slog.LevelVar{}
	level.Set(slog.LevelInfo)
	l := slog.New(NewHandler(&buf, level, "test-scope", lp))

	ctx, sc := sampledContext()
	l.InfoContext(ctx, "hello")

	out := buf.String()
	if !strings.Contains(out, "hello") {
		t.Errorf("stdout sink = %q, want the message", out)
	}
	if !strings.Contains(out, sc.TraceID().String()) {
		t.Errorf("stdout sink = %q, want the trace_id", out)
	}
	if !slices.Contains(exp.all(), "hello") {
		t.Errorf("OTLP bridge did not receive the record; got %v", exp.all())
	}

	// A record below the level reaches neither sink.
	buf.Reset()
	l.DebugContext(ctx, "quiet")
	if buf.Len() != 0 {
		t.Errorf("stdout sink = %q, want nothing below the level", buf.String())
	}
	if slices.Contains(exp.all(), "quiet") {
		t.Error("OTLP bridge received a below-level record")
	}
}
