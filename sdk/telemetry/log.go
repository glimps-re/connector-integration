package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
)

// newLoggerProvider builds the log SDK for res and cfg. It does not install the
// provider as a global.
//
// The provider is always built, so a logging bridge attached to it is always safe
// to use; an OTLP log exporter (batched) is attached only when cfg.Endpoint is
// set and cfg.LogsExporter is not "none". Without an exporter the provider has no
// processor and records are dropped.
func newLoggerProvider(ctx context.Context, res *resource.Resource, cfg Config) (*sdklog.LoggerProvider, error) {
	opts := []sdklog.LoggerProviderOption{sdklog.WithResource(res)}
	if otlpEnabled(cfg.Endpoint, cfg.LogsExporter) {
		exporter, expErr := newLogExporter(ctx, cfg)
		if expErr != nil {
			return nil, fmt.Errorf("telemetry: create log exporter: %w", expErr)
		}
		opts = append(opts, sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)))
	}
	return sdklog.NewLoggerProvider(opts...), nil
}

// newLogExporter builds an OTLP log exporter for cfg.Protocol, applying endpoint,
// security and headers from cfg. The underlying client connects lazily, so this
// does not block on an unreachable endpoint.
func newLogExporter(ctx context.Context, cfg Config) (sdklog.Exporter, error) {
	headers := parseKeyValues(cfg.Headers)
	switch p := otlpProtocol(cfg); p {
	case "grpc":
		opts := applyOTLPEndpoint(cfg, []otlploggrpc.Option{},
			otlploggrpc.WithEndpointURL, otlploggrpc.WithEndpoint, otlploggrpc.WithInsecure)
		if len(headers) > 0 {
			opts = append(opts, otlploggrpc.WithHeaders(headers))
		}
		return otlploggrpc.New(ctx, opts...)
	case "http/protobuf", "http":
		opts := applyOTLPEndpoint(cfg, []otlploghttp.Option{},
			otlploghttp.WithEndpointURL, otlploghttp.WithEndpoint, otlploghttp.WithInsecure)
		if len(headers) > 0 {
			opts = append(opts, otlploghttp.WithHeaders(headers))
		}
		return otlploghttp.New(ctx, opts...)
	default:
		return nil, fmt.Errorf("telemetry: unsupported OTLP protocol %q (want \"grpc\" or \"http/protobuf\")", p)
	}
}
