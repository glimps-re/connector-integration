// Package telemetry provides the OpenTelemetry wiring shared by glimps
// connectors: a single [Config] builds three isolated SDK providers (tracer,
// meter with a Prometheus pull endpoint, logger) wrapped in a [Telemetry] value.
//
// The package is logger-agnostic. It pulls in no logging backend: internal
// diagnostics go to an injected *slog.Logger (stdlib, see [WithLogger]) and the
// built log provider is exposed via [Telemetry.LoggerProvider] so a logging
// backend can attach its own OTLP bridge and trace-id correlation without this
// package importing that backend.
//
// Providers are always built, so spans always carry valid IDs, instruments can
// always record and the Prometheus /metrics endpoint is always served. A signal
// exports over OTLP only when [Config.Endpoint] is set and its exporter is not
// "none"; no endpoint is a silent noop, never an error.
//
// Typical use installs everything onto the OpenTelemetry process globals with a
// single call and defers shutdown:
//
//	t, err := telemetry.Init(ctx, "my-connector", cfg, telemetry.WithVersion(version))
//	if err != nil {
//		return err
//	}
//	defer func() {
//		if err := t.Shutdown(context.Background()); err != nil {
//			slog.Error("telemetry shutdown", slog.String("error", err.Error()))
//		}
//	}()
//	mux.Handle("/metrics", t.MetricsHandler())
//
// Use [New] instead of [Init] to build an isolated instance without touching any
// global; the instance still serves /metrics and records runtime metrics.
package telemetry
