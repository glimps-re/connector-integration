package telemetry

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// newMeterProvider builds the metric SDK for res and cfg and returns it alongside
// the Prometheus scrape handler (mount it at /metrics). It does not install the
// provider as a global. A Prometheus pull reader is always installed, so /metrics
// stays available regardless of OTLP configuration; an OTLP periodic push reader
// is added alongside it when configured. Both readers observe the same
// instruments and the views from [WithView].
//
// The OTLP path is built once from cfg. OTLP push is opt-in:
//
//   - cfg.MetricsExporter == "none" -> no OTLP reader, regardless of endpoint
//   - cfg.Endpoint is set           -> OTLP exporter (grpc default,
//     http/protobuf per cfg.Protocol) in a periodic reader
//   - otherwise                     -> Prometheus pull only
func newMeterProvider(ctx context.Context, res *resource.Resource, cfg Config, o options) (*sdkmetric.MeterProvider, http.Handler, error) {
	opts := []sdkmetric.Option{sdkmetric.WithResource(res)}
	for _, v := range o.views {
		opts = append(opts, sdkmetric.WithView(v))
	}

	// A fresh registry per call keeps repeated New calls (e.g. in tests) free of
	// duplicate-registration panics. The runtime producer supplies
	// go.schedule.duration (goroutine runnable-wait histogram), which runtime.Start
	// does not emit; it is gated with the rest of the runtime instrumentation.
	// Producers bypass SDK views, so its runtime-native buckets (~160) reach the
	// reader as-is. Each reader gets its own producer instance to keep collection
	// timestamps independent.
	registry := prometheus.NewRegistry()
	promOpts := []otelprom.Option{otelprom.WithRegisterer(registry)}
	if o.runtimeMetrics {
		promOpts = append(promOpts, otelprom.WithProducer(otelruntime.NewProducer()))
	}
	promExporter, err := otelprom.New(promOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("telemetry: create prometheus exporter: %w", err)
	}
	opts = append(opts, sdkmetric.WithReader(promExporter))

	if otlpEnabled(cfg.Endpoint, cfg.MetricsExporter) {
		exporter, expErr := newMetricExporter(ctx, cfg)
		if expErr != nil {
			return nil, nil, fmt.Errorf("telemetry: create metric exporter: %w", expErr)
		}
		var readerOpts []sdkmetric.PeriodicReaderOption
		if o.runtimeMetrics {
			readerOpts = append(readerOpts, sdkmetric.WithProducer(otelruntime.NewProducer()))
		}
		opts = append(opts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, readerOpts...)))
	}

	mp := sdkmetric.NewMeterProvider(opts...)
	return mp, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), nil
}

// newMetricExporter builds an OTLP metric exporter for cfg.Protocol, applying
// endpoint, security and headers from cfg. The underlying client connects lazily,
// so this does not block on an unreachable endpoint.
func newMetricExporter(ctx context.Context, cfg Config) (sdkmetric.Exporter, error) {
	headers := parseKeyValues(cfg.Headers)
	switch p := otlpProtocol(cfg); p {
	case "grpc":
		opts := applyOTLPEndpoint(cfg, []otlpmetricgrpc.Option{},
			otlpmetricgrpc.WithEndpointURL, otlpmetricgrpc.WithEndpoint, otlpmetricgrpc.WithInsecure)
		if len(headers) > 0 {
			opts = append(opts, otlpmetricgrpc.WithHeaders(headers))
		}
		return otlpmetricgrpc.New(ctx, opts...)
	case "http/protobuf", "http":
		opts := applyOTLPEndpoint(cfg, []otlpmetrichttp.Option{},
			otlpmetrichttp.WithEndpointURL, otlpmetrichttp.WithEndpoint, otlpmetrichttp.WithInsecure)
		if len(headers) > 0 {
			opts = append(opts, otlpmetrichttp.WithHeaders(headers))
		}
		return otlpmetrichttp.New(ctx, opts...)
	default:
		return nil, fmt.Errorf("telemetry: unsupported OTLP protocol %q (want \"grpc\" or \"http/protobuf\")", p)
	}
}
