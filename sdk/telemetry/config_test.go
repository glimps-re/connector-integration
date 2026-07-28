package telemetry

import (
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func TestResolveEnv(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "env-collector:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	t.Setenv("OTEL_TRACES_SAMPLER", "always_on")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")

	// Explicit config keeps precedence; unset fields fall back to the bare env.
	got := ResolveEnv(Config{Endpoint: "explicit-collector:4317"})
	if got.Endpoint != "explicit-collector:4317" {
		t.Errorf("Endpoint = %q, want explicit config to win", got.Endpoint)
	}
	if got.Protocol != "http/protobuf" {
		t.Errorf("Protocol = %q, want %q from env", got.Protocol, "http/protobuf")
	}
	if got.TracesSampler != "always_on" {
		t.Errorf("TracesSampler = %q, want %q from env", got.TracesSampler, "always_on")
	}
	if got.Insecure != "true" {
		t.Errorf("Insecure = %q, want %q from env", got.Insecure, "true")
	}
}

func TestResolveEnvResourceAttributesNotOverlaid(t *testing.T) {
	// OTEL_RESOURCE_ATTRIBUTES is applied to the resource via resource.WithFromEnv,
	// not overlaid onto cfg, so it merges with the connector attributes rather than
	// being shadowed.
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "team=core")
	if got := ResolveEnv(Config{}); got.ResourceAttributes != "" {
		t.Errorf("ResourceAttributes = %q, want empty (env flows through WithFromEnv)", got.ResourceAttributes)
	}
}

func TestParseKeyValues(t *testing.T) {
	got := parseKeyValues(" a=1, b = 2 ,malformed,, c=x=y ")
	want := map[string]string{"a": "1", "b": "2", "c": "x=y"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parseKeyValues mismatch (-want +got):\n%s", diff)
	}
	if got := parseKeyValues(""); len(got) != 0 {
		t.Errorf("parseKeyValues(\"\") = %v, want empty", got)
	}
}

func TestInsecureEnabled(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"true", true},
		{"false", false},
		{"1", true},
		{"0", false},
		{"garbage", false},
		{" true ", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := insecureEnabled(tt.in); got != tt.want {
				t.Errorf("insecureEnabled(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestOtlpProtocol(t *testing.T) {
	if got := otlpProtocol(Config{}); got != "grpc" {
		t.Errorf("otlpProtocol(empty) = %q, want grpc", got)
	}
	if got := otlpProtocol(Config{Protocol: "http/protobuf"}); got != "http/protobuf" {
		t.Errorf("otlpProtocol = %q, want http/protobuf", got)
	}
}

func TestOtlpEnabled(t *testing.T) {
	tests := []struct {
		endpoint, exporter string
		want               bool
	}{
		{"", "", false},
		{"", "otlp", false},
		{"collector:4317", "", true},
		{"collector:4317", "otlp", true},
		{"collector:4317", "none", false},
	}
	for _, tt := range tests {
		if got := otlpEnabled(tt.endpoint, tt.exporter); got != tt.want {
			t.Errorf("otlpEnabled(%q, %q) = %v, want %v", tt.endpoint, tt.exporter, got, tt.want)
		}
	}
}

func TestResolveEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		insecure     string
		wantValue    string
		wantURL      bool
		wantInsecure bool
	}{
		{"scheme-less stays host:port", "collector:4317", "", "collector:4317", false, false},
		{"scheme-less honors insecure flag", "collector:4317", "true", "collector:4317", false, true},
		{"http pathless reduces to host:port, insecure from scheme", "http://collector:4318", "", "collector:4318", false, true},
		{"https pathless reduces to host:port, stays secure", "https://collector:4318", "", "collector:4318", false, false},
		{"https scheme ignores insecure flag (no TLS downgrade)", "https://collector:4318", "true", "collector:4318", false, false},
		{"http scheme stays insecure despite insecure=false", "http://collector:4318", "false", "collector:4318", false, true},
		{"http root path treated as pathless", "http://collector:4318/", "", "collector:4318", false, true},
		{"real path honored verbatim as URL", "https://collector:4318/otlp", "", "https://collector:4318/otlp", true, false},
		{"unparseable url handed through as URL", "http://%zz", "", "http://%zz", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEndpoint(Config{Endpoint: tt.endpoint, Insecure: tt.insecure})
			if got.value != tt.wantValue || got.URL != tt.wantURL || got.Insecure != tt.wantInsecure {
				t.Errorf("resolveEndpoint(%q, insecure=%q) = %+v, want value=%q URL=%v Insecure=%v",
					tt.endpoint, tt.insecure, got, tt.wantValue, tt.wantURL, tt.wantInsecure)
			}
		})
	}
}

func TestApplyOTLPEndpoint(t *testing.T) {
	withURL := func(s string) string { return "url:" + s }
	withEndpoint := func(s string) string { return "endpoint:" + s }
	withInsecure := func() string { return "insecure" }

	tests := []struct {
		name     string
		endpoint string
		insecure string
		want     []string
	}{
		{"scheme-less secure", "collector:4317", "", []string{"endpoint:collector:4317"}},
		{"scheme-less insecure", "collector:4317", "true", []string{"endpoint:collector:4317", "insecure"}},
		{"http pathless adds insecure from scheme", "http://collector:4318", "", []string{"endpoint:collector:4318", "insecure"}},
		{"https pathless stays secure", "https://collector:4318", "", []string{"endpoint:collector:4318"}},
		{"real path uses URL form, no insecure option", "https://collector:4318/otlp", "", []string{"url:https://collector:4318/otlp"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyOTLPEndpoint(Config{Endpoint: tt.endpoint, Insecure: tt.insecure},
				[]string(nil), withURL, withEndpoint, withInsecure)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("applyOTLPEndpoint mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
