package telemetry

import (
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config holds the OpenTelemetry options resolved by a connector and passed to
// [New] or [Init]. Every field maps to an OpenTelemetry specification option and
// its bare standard environment variable (e.g. OTEL_EXPORTER_OTLP_ENDPOINT); a
// connector binds its own flag/env layer onto this plain struct. Fields are
// strings so an unset value stays distinct from an explicit "false".
//
// Only json tags are set, for config dumps; Headers is never serialized because
// it frequently carries credentials.
type Config struct {
	// ServiceName sets service.name (OTEL_SERVICE_NAME). Empty falls back to the
	// service name passed to New/Init.
	ServiceName string `json:"otel_service_name"`
	// ResourceAttributes adds resource attributes (OTEL_RESOURCE_ATTRIBUTES), a
	// comma-separated list of key=value pairs. It merges with the standard
	// environment (via resource.WithFromEnv) and the [WithResourceAttributes]
	// option; explicit values win on key conflicts.
	ResourceAttributes string `json:"otel_resource_attributes"`
	// Endpoint is the OTLP endpoint shared by all signals
	// (OTEL_EXPORTER_OTLP_ENDPOINT). Empty disables OTLP export (silent noop). An
	// http:// or https:// scheme selects transport security; otherwise Insecure
	// applies.
	Endpoint string `json:"otel_exporter_otlp_endpoint"`
	// Protocol selects the OTLP transport: "grpc" (default) or "http/protobuf"
	// ("http" is accepted as an alias for "http/protobuf").
	Protocol string `json:"otel_exporter_otlp_protocol"`
	// Insecure disables transport security for a scheme-less Endpoint
	// (OTEL_EXPORTER_OTLP_INSECURE): "true" or "false". Empty means unset and is
	// treated as false (TLS on); it is parsed at use so an explicit "false" stays
	// distinct from unset.
	Insecure string `json:"otel_exporter_otlp_insecure"`
	// Headers are extra OTLP headers (OTEL_EXPORTER_OTLP_HEADERS), a
	// comma-separated list of key=value pairs.
	Headers string `json:"-"`
	// TracesExporter disables trace export when set to "none"
	// (OTEL_TRACES_EXPORTER); empty or "otlp" keeps OTLP export subject to
	// Endpoint.
	TracesExporter string `json:"otel_traces_exporter"`
	// MetricsExporter disables OTLP metric push when set to "none"
	// (OTEL_METRICS_EXPORTER); the Prometheus pull endpoint stays available
	// regardless.
	MetricsExporter string `json:"otel_metrics_exporter"`
	// LogsExporter disables OTLP log export when set to "none"
	// (OTEL_LOGS_EXPORTER).
	LogsExporter string `json:"otel_logs_exporter"`
	// TracesSampler selects the sampler (OTEL_TRACES_SAMPLER): always_on,
	// always_off, traceidratio, parentbased_always_on, parentbased_always_off or
	// parentbased_traceidratio. Empty defaults to parentbased_traceidratio.
	TracesSampler string `json:"otel_traces_sampler"`
	// TracesSamplerArg is the ratio for ratio-based samplers
	// (OTEL_TRACES_SAMPLER_ARG), parsed as a float in [0,1]. Empty or invalid
	// means 0 (never sample new roots).
	TracesSamplerArg string `json:"otel_traces_sampler_arg"`
}

// ResolveEnv overlays the bare, standard OTEL_* environment onto cfg for any
// field left unset, so SDK-style injection (e.g. the Kubernetes OpenTelemetry
// operator) works without a component prefix. Explicit configuration already in
// cfg keeps precedence; downstream defaults fill the rest. The signal-specific
// *_TRACES_ENDPOINT / *_METRICS_ENDPOINT variables are intentionally not
// consulted: a single shared endpoint is used.
//
// OTEL_RESOURCE_ATTRIBUTES is deliberately not overlaid here. newResource reads
// it from the environment via resource.WithFromEnv, so it merges with the
// operator-injected attributes rather than being shadowed by a set field;
// Config.ResourceAttributes carries only the connector-provided attributes.
//
// [Init] applies ResolveEnv; a caller using [New] directly should apply it first
// when SDK-style env injection is expected.
func ResolveEnv(cfg Config) Config {
	setIfEmpty(&cfg.ServiceName, "OTEL_SERVICE_NAME")
	setIfEmpty(&cfg.Endpoint, "OTEL_EXPORTER_OTLP_ENDPOINT")
	setIfEmpty(&cfg.Protocol, "OTEL_EXPORTER_OTLP_PROTOCOL")
	setIfEmpty(&cfg.Headers, "OTEL_EXPORTER_OTLP_HEADERS")
	setIfEmpty(&cfg.TracesExporter, "OTEL_TRACES_EXPORTER")
	setIfEmpty(&cfg.MetricsExporter, "OTEL_METRICS_EXPORTER")
	setIfEmpty(&cfg.LogsExporter, "OTEL_LOGS_EXPORTER")
	setIfEmpty(&cfg.TracesSampler, "OTEL_TRACES_SAMPLER")
	setIfEmpty(&cfg.TracesSamplerArg, "OTEL_TRACES_SAMPLER_ARG")
	setIfEmpty(&cfg.Insecure, "OTEL_EXPORTER_OTLP_INSECURE")
	return cfg
}

// setIfEmpty sets *dst from the environment variable envKey when *dst is empty
// and the variable is set to a non-blank value.
func setIfEmpty(dst *string, envKey string) {
	if *dst != "" {
		return
	}
	if v, ok := os.LookupEnv(envKey); ok {
		if v = strings.TrimSpace(v); v != "" {
			*dst = v
		}
	}
}

// otlpEnabled reports whether OTLP export is enabled for a signal: an endpoint is
// configured and the signal's exporter is not "none".
func otlpEnabled(endpoint, exporter string) bool {
	return endpoint != "" && exporter != "none"
}

// otlpProtocol returns cfg.Protocol, defaulting to "grpc" when empty.
func otlpProtocol(cfg Config) string {
	if cfg.Protocol == "" {
		return "grpc"
	}
	return cfg.Protocol
}

// hasScheme reports whether endpoint carries an http:// or https:// scheme.
func hasScheme(endpoint string) bool {
	return strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://")
}

// parseKeyValues parses a comma-separated list of key=value pairs (the OTLP
// headers and resource-attributes format) into a map, ignoring blank or
// malformed entries.
func parseKeyValues(s string) map[string]string {
	out := map[string]string{}
	for pair := range strings.SplitSeq(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		k, v, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}

// insecureEnabled reports whether the Insecure config value requests plaintext
// transport. Empty (unset) and unparseable values are false (TLS on); New warns
// once on an unparseable value, so this stays silent for its per-signal callers.
func insecureEnabled(s string) bool {
	b, _ := strconv.ParseBool(strings.TrimSpace(s))
	return b
}

// otlpEndpoint tells an OTLP exporter builder how to point at Config.Endpoint: as
// a full URL (WithEndpointURL, honoring its path) or as a bare host:port
// (WithEndpoint, plus Insecure).
type otlpEndpoint struct {
	// value is the URL when URL is true, otherwise host:port.
	value string
	// URL selects WithEndpointURL over WithEndpoint.
	URL bool
	// Insecure disables transport security; only consulted for the host:port
	// form, since WithEndpointURL derives security from the URL scheme.
	Insecure bool
}

// resolveEndpoint decides how to feed cfg.Endpoint to an exporter builder. A
// scheme-less endpoint stays host:port, with security from cfg.Insecure. A
// scheme-carrying but pathless endpoint is reduced to host:port with security
// derived purely from the scheme (http -> plaintext, https -> TLS), so the SDK
// keeps its per-signal default path (/v1/traces, /v1/metrics, /v1/logs); passing
// a pathless URL to WithEndpointURL would send every signal to "/" and 404. A URL
// with a real path (e.g. a reverse-proxy prefix) is honored verbatim via
// WithEndpointURL.
//
// cfg.Insecure applies only to a scheme-less endpoint: a scheme states the
// intent explicitly, so an https endpoint stays TLS even if Insecure is set.
func resolveEndpoint(cfg Config) otlpEndpoint {
	if !hasScheme(cfg.Endpoint) {
		return otlpEndpoint{value: cfg.Endpoint, Insecure: insecureEnabled(cfg.Insecure)}
	}
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || strings.Trim(u.Path, "/") != "" {
		// Unparseable, or an explicit path to preserve: hand the raw URL to the
		// exporter, which derives security from the scheme.
		return otlpEndpoint{value: cfg.Endpoint, URL: true}
	}
	return otlpEndpoint{value: u.Host, Insecure: u.Scheme != "https"}
}

// applyOTLPEndpoint appends the endpoint (and, for the host:port form, the
// insecure option) via the exporter-specific builders, so the trace, metric and
// log exporters point at cfg.Endpoint identically without a shared option type
// across the OTLP SDK packages.
func applyOTLPEndpoint[O any](cfg Config, opts []O, withURL, withEndpoint func(string) O, withInsecure func() O) []O {
	ep := resolveEndpoint(cfg)
	if ep.URL {
		return append(opts, withURL(ep.value))
	}
	opts = append(opts, withEndpoint(ep.value))
	if ep.Insecure {
		opts = append(opts, withInsecure())
	}
	return opts
}
