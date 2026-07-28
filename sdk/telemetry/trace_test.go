package telemetry

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestSamplerFromConfig(t *testing.T) {
	tests := []struct {
		name        string
		hasExporter bool
		sampler     string
		arg         string
		wantSample  bool
	}{
		{"no exporter never samples", false, "always_on", "", false},
		{"default is 0 (never)", true, "", "", false},
		{"default sampler with arg 1 samples", true, "", "1", true},
		{"default sampler with arg 0 never samples", true, "", "0", false},
		{"always_on", true, "always_on", "", true},
		{"always_off overrides arg", true, "always_off", "0.7", false},
		{"traceidratio ratio 1", true, "traceidratio", "1", true},
		{"traceidratio ratio 0", true, "traceidratio", "0", false},
		{"parentbased_always_on", true, "parentbased_always_on", "", true},
		{"parentbased_always_off", true, "parentbased_always_off", "1", false},
		{"parentbased_traceidratio ratio 1", true, "parentbased_traceidratio", "1", true},
		{"invalid arg falls back to 0", true, "traceidratio", "lots", false},
		{"unknown sampler defaults to parentbased ratio", true, "carrier-pigeon", "1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{TracesSampler: tt.sampler, TracesSamplerArg: tt.arg}
			d := samplerFromConfig(tt.hasExporter, cfg, discardLogger()).ShouldSample(sdktrace.SamplingParameters{
				ParentContext: context.Background(),
				Name:          "s",
			})
			if got := d.Decision == sdktrace.RecordAndSample; got != tt.wantSample {
				t.Fatalf("decision = %v, want sample=%v", d.Decision, tt.wantSample)
			}
		})
	}
}

func TestSamplerRatio(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want float64
	}{
		{"empty is 0", "", 0},
		{"ratio", "0.25", 0.25},
		{"whitespace trimmed", " 0.5 ", 0.5},
		{"one", "1", 1},
		{"invalid falls back to 0", "lots", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := samplerRatio(tt.arg, discardLogger()); got != tt.want {
				t.Errorf("samplerRatio(%q) = %v, want %v", tt.arg, got, tt.want)
			}
		})
	}
}
