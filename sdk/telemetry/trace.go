package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// newTracerProvider builds the trace SDK for res and cfg: an OTLP exporter when
// enabled by cfg, none otherwise (spans keep valid IDs but never export). Any
// span processors from [WithSpanProcessor] are registered. It does not install
// the provider as a global.
//
// Sampling defaults to 0 (never): even with an exporter, new root spans are only
// recorded when cfg.TracesSampler / cfg.TracesSamplerArg opt in.
func newTracerProvider(ctx context.Context, res *resource.Resource, cfg Config, o options) (*sdktrace.TracerProvider, error) {
	var exporter sdktrace.SpanExporter
	if otlpEnabled(cfg.Endpoint, cfg.TracesExporter) {
		var err error
		exporter, err = newTraceExporter(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("telemetry: create span exporter: %w", err)
		}
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(samplerFromConfig(exporter != nil, cfg, o.logger)),
	}
	if exporter != nil {
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}
	for _, sp := range o.spanProcessors {
		opts = append(opts, sdktrace.WithSpanProcessor(sp))
	}
	return sdktrace.NewTracerProvider(opts...), nil
}

// newTraceExporter builds an OTLP span exporter for cfg.Protocol, applying
// endpoint, security and headers from cfg. The underlying client connects lazily,
// so this does not block on an unreachable endpoint.
func newTraceExporter(ctx context.Context, cfg Config) (sdktrace.SpanExporter, error) {
	headers := parseKeyValues(cfg.Headers)
	switch p := otlpProtocol(cfg); p {
	case "grpc":
		opts := applyOTLPEndpoint(cfg, []otlptracegrpc.Option{},
			otlptracegrpc.WithEndpointURL, otlptracegrpc.WithEndpoint, otlptracegrpc.WithInsecure)
		if len(headers) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(headers))
		}
		return otlptracegrpc.New(ctx, opts...)
	case "http/protobuf", "http":
		opts := applyOTLPEndpoint(cfg, []otlptracehttp.Option{},
			otlptracehttp.WithEndpointURL, otlptracehttp.WithEndpoint, otlptracehttp.WithInsecure)
		if len(headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(headers))
		}
		return otlptracehttp.New(ctx, opts...)
	default:
		return nil, fmt.Errorf("telemetry: unsupported OTLP protocol %q (want \"grpc\" or \"http/protobuf\")", p)
	}
}

// samplerFromConfig selects the head-based sampler from cfg.TracesSampler and
// cfg.TracesSamplerArg. Without an exporter it never samples: spans stay
// non-recording so no attributes are allocated, yet still carry valid IDs for
// logs and propagation. With an exporter, ratio-based samplers resolve their
// ratio from cfg.TracesSamplerArg (<=0 never samples, >=1 always). Empty or
// unknown cfg.TracesSampler behaves as parentbased_traceidratio with a default
// ratio of 0, so new root traces are not exported while inherited-sampled traces
// still are.
func samplerFromConfig(hasExporter bool, cfg Config, logger *slog.Logger) sdktrace.Sampler {
	if !hasExporter {
		return sdktrace.NeverSample()
	}
	switch name := cfg.TracesSampler; name {
	case "always_on":
		return sdktrace.AlwaysSample()
	case "always_off":
		return sdktrace.NeverSample()
	case "traceidratio":
		return ratioSampler(samplerRatio(cfg.TracesSamplerArg, logger))
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample())
	case "parentbased_traceidratio", "":
		return sdktrace.ParentBased(ratioSampler(samplerRatio(cfg.TracesSamplerArg, logger)))
	default:
		logger.Warn("telemetry: unsupported otel-traces-sampler, defaulting to parentbased_traceidratio",
			slog.String("value", name))
		return sdktrace.ParentBased(ratioSampler(samplerRatio(cfg.TracesSamplerArg, logger)))
	}
}

// ratioSampler maps a sampling ratio to a root sampler: <=0 never samples, >=1
// always, otherwise samples the trace-id ratio.
func ratioSampler(ratio float64) sdktrace.Sampler {
	switch {
	case ratio <= 0:
		return sdktrace.NeverSample()
	case ratio >= 1:
		return sdktrace.AlwaysSample()
	default:
		return sdktrace.TraceIDRatioBased(ratio)
	}
}

// samplerRatio parses a sampler argument as a float ratio. Empty or invalid
// values yield 0 (never sample new roots), the deliberate opt-in default.
func samplerRatio(arg string, logger *slog.Logger) float64 {
	v := strings.TrimSpace(arg)
	if v == "" {
		return 0
	}
	ratio, err := strconv.ParseFloat(v, 64)
	if err != nil {
		logger.Warn("telemetry: invalid otel-traces-sampler-arg, sampling disabled",
			slog.String("value", v), slog.String("error", err.Error()))
		return 0
	}
	return ratio
}
