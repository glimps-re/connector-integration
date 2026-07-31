package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	otelruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otellog "go.opentelemetry.io/otel/log"
	logglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

// Telemetry owns a self-contained set of OpenTelemetry providers and the
// Prometheus scrape handler. It touches no process globals until [Telemetry.Publish]
// is called, so several instances can coexist in one process (e.g. tests).
type Telemetry struct {
	tp             *sdktrace.TracerProvider
	mp             *sdkmetric.MeterProvider
	lp             *sdklog.LoggerProvider
	metricsHandler http.Handler
	cfg            Config
	scope          string
	baggage        bool
}

// options collects the [Option] values applied to [New]/[Init].
type options struct {
	version        string
	logger         *slog.Logger
	views          []sdkmetric.View
	spanProcessors []sdktrace.SpanProcessor
	resourceAttrs  []attribute.KeyValue
	baggage        bool
	runtimeMetrics bool
}

func defaultOptions() options {
	return options{
		logger:         slog.Default(),
		runtimeMetrics: true,
	}
}

// Option customizes the providers built by [New] and [Init].
type Option func(*options)

// WithVersion sets service.version on the telemetry resource. Empty leaves the
// attribute unset.
func WithVersion(version string) Option {
	return func(o *options) { o.version = version }
}

// WithLogger routes the package's internal diagnostics (bad sampler, unparseable
// insecure flag, provider-shutdown failures) to l. It defaults to slog.Default();
// pass a discard handler for a silent SDK. The logger is used only for the SDK's
// own diagnostics, never as a telemetry signal.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) {
		if l != nil {
			o.logger = l
		}
	}
}

// WithView adds metric views, typically to tune histogram bucket boundaries for a
// connector's own instruments. Views are connector-specific and never defined by
// this package.
func WithView(views ...sdkmetric.View) Option {
	return func(o *options) { o.views = append(o.views, views...) }
}

// WithSpanProcessor registers additional span processors on the tracer provider,
// e.g. a correlation processor that mirrors context attributes onto every span.
func WithSpanProcessor(procs ...sdktrace.SpanProcessor) Option {
	return func(o *options) { o.spanProcessors = append(o.spanProcessors, procs...) }
}

// WithResourceAttributes adds resource attributes programmatically, alongside
// those from Config.ResourceAttributes and the environment.
func WithResourceAttributes(attrs ...attribute.KeyValue) Option {
	return func(o *options) { o.resourceAttrs = append(o.resourceAttrs, attrs...) }
}

// WithBaggage enables W3C baggage propagation in addition to trace context. It is
// off by default; enable it only when the connector relies on baggage crossing
// service boundaries. Effective only after [Telemetry.Publish].
func WithBaggage(enabled bool) Option {
	return func(o *options) { o.baggage = enabled }
}

// WithRuntimeMetrics toggles Go runtime instrumentation (heap, goroutines, GC,
// process CPU) on the meter provider. It is on by default.
func WithRuntimeMetrics(enabled bool) Option {
	return func(o *options) { o.runtimeMetrics = enabled }
}

// New builds isolated tracer, meter and logger providers from cfg for
// serviceName and returns them wrapped in a [Telemetry]. It installs nothing as a
// global: call [Telemetry.Publish] (or use [Init]) to do that. serviceName must
// not be empty; Config.ServiceName overrides it when set.
//
// A Prometheus pull reader is always installed, so [Telemetry.MetricsHandler]
// always serves. An OTLP exporter is added for a signal only when cfg.Endpoint is
// set and the signal's exporter is not "none". Runtime metrics are recorded on
// the instance meter provider unless disabled via [WithRuntimeMetrics].
//
// New does not apply [ResolveEnv]; pass an already-resolved cfg, or use [Init].
func New(ctx context.Context, serviceName string, cfg Config, opts ...Option) (*Telemetry, error) {
	if serviceName == "" {
		return nil, errors.New("telemetry: service name must not be empty")
	}
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	if s := strings.TrimSpace(cfg.Insecure); s != "" {
		if _, perr := strconv.ParseBool(s); perr != nil {
			o.logger.Warn("telemetry: invalid otel-exporter-otlp-insecure, treating as false (TLS)",
				slog.String("value", cfg.Insecure), slog.String("error", perr.Error()))
		}
	}

	scope := serviceName
	if cfg.ServiceName != "" {
		scope = cfg.ServiceName
	}

	res, err := newResource(ctx, scope, o.version, cfg, o.resourceAttrs)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build resource: %w", err)
	}

	tp, err := newTracerProvider(ctx, res, cfg, o)
	if err != nil {
		return nil, err
	}
	mp, handler, err := newMeterProvider(ctx, res, cfg, o)
	if err != nil {
		if sErr := tp.Shutdown(ctx); sErr != nil {
			o.logger.Warn("telemetry: shutdown tracer provider after meter provider failure",
				slog.String("error", sErr.Error()))
		}
		return nil, err
	}
	lp, err := newLoggerProvider(ctx, res, cfg)
	if err != nil {
		if sErr := errors.Join(mp.Shutdown(ctx), tp.Shutdown(ctx)); sErr != nil {
			o.logger.Warn("telemetry: shutdown providers after logger provider failure",
				slog.String("error", sErr.Error()))
		}
		return nil, err
	}

	if o.runtimeMetrics {
		if err = otelruntime.Start(otelruntime.WithMeterProvider(mp)); err != nil {
			if sErr := errors.Join(lp.Shutdown(ctx), mp.Shutdown(ctx), tp.Shutdown(ctx)); sErr != nil {
				o.logger.Warn("telemetry: shutdown providers after runtime instrumentation failure",
					slog.String("error", sErr.Error()))
			}
			return nil, fmt.Errorf("telemetry: start runtime instrumentation: %w", err)
		}
	}

	return &Telemetry{
		tp:             tp,
		mp:             mp,
		lp:             lp,
		metricsHandler: handler,
		cfg:            cfg,
		scope:          scope,
		baggage:        o.baggage,
	}, nil
}

// Init is the convenience entry point: it applies [ResolveEnv] to cfg, builds the
// providers with [New] and installs them onto the OpenTelemetry globals with
// [Telemetry.Publish]. The caller mounts [Telemetry.MetricsHandler] and defers
// [Telemetry.Shutdown].
func Init(ctx context.Context, serviceName string, cfg Config, opts ...Option) (*Telemetry, error) {
	t, err := New(ctx, serviceName, ResolveEnv(cfg), opts...)
	if err != nil {
		return nil, err
	}
	t.Publish()
	return t, nil
}

// Publish installs the providers onto the OpenTelemetry process globals and sets
// the text-map propagator (W3C trace context, plus baggage when [WithBaggage] is
// enabled). It is the single writer of these globals; calling it from several
// instances in one process makes the last writer win.
func (t *Telemetry) Publish() {
	otel.SetTracerProvider(t.tp)
	otel.SetMeterProvider(t.mp)
	logglobal.SetLoggerProvider(t.lp)

	props := []propagation.TextMapPropagator{propagation.TraceContext{}}
	if t.baggage {
		props = append(props, propagation.Baggage{})
	}
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(props...))
}

// Shutdown flushes and stops the logger, meter and tracer providers in that
// order, joining their errors. It is safe to call once; the providers reject
// further use afterwards.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	return errors.Join(t.lp.Shutdown(ctx), t.mp.Shutdown(ctx), t.tp.Shutdown(ctx))
}

// Tracer returns a tracer from the instance tracer provider.
func (t *Telemetry) Tracer(name string) trace.Tracer { return t.tp.Tracer(name) }

// Meter returns a meter from the instance meter provider.
func (t *Telemetry) Meter(name string) metric.Meter { return t.mp.Meter(name) }

// TracerProvider returns the instance tracer provider.
func (t *Telemetry) TracerProvider() trace.TracerProvider { return t.tp }

// MeterProvider returns the instance meter provider.
func (t *Telemetry) MeterProvider() metric.MeterProvider { return t.mp }

// LoggerProvider returns the instance log provider, for attaching a logging
// backend's OTLP bridge.
func (t *Telemetry) LoggerProvider() otellog.LoggerProvider { return t.lp }

// MetricsHandler returns the Prometheus scrape handler; mount it at /metrics.
func (t *Telemetry) MetricsHandler() http.Handler { return t.metricsHandler }

// Config returns the effective configuration used to build the providers.
func (t *Telemetry) Config() Config { return t.cfg }

// ScopeName returns the instrumentation scope name (the resolved service name),
// suitable for a logging bridge's scope.
func (t *Telemetry) ScopeName() string { return t.scope }

// newResource builds the resource shared by all three providers. It layers, in
// increasing precedence, the telemetry SDK attributes, the standard
// OTEL_SERVICE_NAME / OTEL_RESOURCE_ATTRIBUTES environment (via
// resource.WithFromEnv), then the explicit attributes: service.name, optional
// service.version, Config.ResourceAttributes and the [WithResourceAttributes]
// values. The explicit attributes are applied last so they win over the
// environment-detected ones.
func newResource(ctx context.Context, serviceName, version string, cfg Config, extra []attribute.KeyValue) (*resource.Resource, error) {
	fromCfg := parseKeyValues(cfg.ResourceAttributes)
	attrs := make([]attribute.KeyValue, 0, len(fromCfg)+len(extra)+2)
	attrs = append(attrs, semconv.ServiceName(serviceName))
	if version != "" {
		attrs = append(attrs, semconv.ServiceVersion(version))
	}
	for k, v := range fromCfg {
		attrs = append(attrs, attribute.String(k, v))
	}
	attrs = append(attrs, extra...)
	return resource.New(
		ctx,
		resource.WithTelemetrySDK(),
		resource.WithFromEnv(),
		resource.WithAttributes(attrs...),
	)
}

// EndSpan ends span. When err points to a non-nil error, it records that error on
// the span and sets an Error status, so a failed operation surfaces on its span
// without the call site repeating RecordError/SetStatus. Pair it with a span
// start via defer, capturing a named error return: defer telemetry.EndSpan(span, &err).
func EndSpan(span trace.Span, err *error) {
	defer span.End()
	if err == nil || *err == nil {
		return
	}
	// Recording must never crash the caller's request path (e.g. a malformed error
	// whose Error() panics): recover and report to slog.Default() rather than
	// propagate.
	defer func() {
		if r := recover(); r != nil {
			slog.Default().Error("telemetry: recovered panic while recording span error",
				slog.String("error_type", fmt.Sprintf("%T", *err)), slog.Any("panic", r))
		}
	}()
	span.RecordError(*err)
	span.SetStatus(codes.Error, (*err).Error())
}
