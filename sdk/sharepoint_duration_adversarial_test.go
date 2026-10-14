package sdk

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"gopkg.in/yaml.v3"
)

// sharepointJSON builds the payload, with or without the non reconfigurable
// "database" member (rejected by the strict reconfiguration binding).
func sharepointJSON(withDatabase bool, extra string) []byte {
	base := `{"client_name":"c","m365_tenant_id":"t","m365_client_id":"i","m365_client_secret":"s",` +
		`"gmalware_api_url":"https://detect.example.com","gmalware_api_token":"tok",` +
		`"mitigation_action":{}`
	if withDatabase {
		base += `,"database":"db"`
	}
	if extra != "" {
		base += "," + extra
	}
	return []byte(base + "}")
}

func TestSharepointConfig_durationJSONAdversarial(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    time.Duration
		wantErr bool
	}{
		{name: "negative string", payload: `"-5m"`, want: -5 * time.Minute},
		{name: "normalized string", payload: `"1h0m0s"`, want: time.Hour},
		{name: "empty string", payload: `""`, wantErr: true},
		{name: "leading space", payload: `" 5m "`, wantErr: true},
		{name: "inner space", payload: `"5 m"`, wantErr: true},
		{name: "no unit", payload: `"5"`, wantErr: true},
		// Known limitation (sdk/duration.go, out of CI37-1 scope): JSON null is
		// rejected instead of being a no-op. Pinned here; flip to success if fixed.
		{name: "null", payload: `null`, wantErr: true},
		{name: "object", payload: `{}`, wantErr: true},
		{name: "array", payload: `[]`, wantErr: true},
		{name: "bool", payload: `false`, wantErr: true},
		// Pinned behavior: decoded as-is, then rejected by validation.
		{name: "negative number", payload: `-1`, want: -1},
		{name: "float", payload: `1.5`, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config SharepointConfig
			err := json.Unmarshal([]byte(`{"drives_search_frequency":`+tt.payload+`}`), &config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && time.Duration(config.DrivesSearchFrequency) != tt.want {
				t.Errorf("got %v, want %v", time.Duration(config.DrivesSearchFrequency), tt.want)
			}
		})
	}
}

// Platform dependent: float64 -> int64 conversion of out of range values is
// implementation defined, so only the end result (rejected) is asserted.
// A value overflowing int64 must never be accepted as a valid duration: either
// decoding fails or validation rejects it.
func TestSharepointConfig_durationOverflowRejected(t *testing.T) {
	for _, key := range durationFields {
		for _, payload := range []string{`1e30`, `-1e30`, `9223372036854775807`, `9223372036854775808`, `"2562048h"`} {
			t.Run(key+"/"+payload, func(t *testing.T) {
				var config SharepointConfig
				if err := json.Unmarshal([]byte(`{"`+key+`":`+payload+`}`), &config); err != nil {
					return
				}
				if !invalidFields(t, &config)[key] {
					t.Errorf("%s=%s accepted (value %v)", key, payload, time.Duration(*spDurationField(t, &config, key)))
				}
			})
		}
	}
}

// Full configuration as received by connector and manager: BindAndValidateConfig.
func TestBindAndValidateConfig_sharepointDurations(t *testing.T) {
	tests := []struct {
		name      string
		extra     string
		wantField string // empty: valid
		wantErr   bool
	}{
		{name: "backward compatible without fields"},
		{name: "valid strings", extra: `"drives_search_frequency":"5m","graph_client_timeout":"300s"`},
		{name: "valid numbers", extra: `"drives_search_frequency":300000000000,"graph_client_timeout":120000000000`},
		{name: "bounds", extra: `"drives_search_frequency":"24h","graph_client_timeout":"1h"`},
		{name: "explicit zero", extra: `"drives_search_frequency":"0s","graph_client_timeout":0`},
		{name: "search too low", extra: `"drives_search_frequency":"59s"`, wantErr: true},
		{name: "search too high", extra: `"drives_search_frequency":"24h0m0.000000001s"`, wantErr: true},
		{name: "timeout too low", extra: `"graph_client_timeout":"30s"`, wantErr: true},
		{name: "timeout too high", extra: `"graph_client_timeout":"61m"`, wantErr: true},
		{name: "negative", extra: `"graph_client_timeout":"-5m"`, wantErr: true},
		{name: "garbage", extra: `"graph_client_timeout":"abc"`, wantErr: true},
		{name: "object", extra: `"graph_client_timeout":{}`, wantErr: true},
		{name: "array", extra: `"drives_search_frequency":[]`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BindAndValidateConfig(SharepointKey, sharepointJSON(true, tt.extra))
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Reconfiguration: patching an existing config must keep untouched fields and
// reject out-of-bounds values.
func TestPatchConfig_sharepointDurations(t *testing.T) {
	actual := func() *SharepointConfig {
		c, err := InitDefault(SharepointKey)
		if err != nil {
			t.Fatal(err)
		}
		sp, ok := c.(*SharepointConfig)
		if !ok {
			t.Fatalf("type %T", c)
		}
		sp.Database = "db"
		sp.DrivesSearchFrequency = Duration(10 * time.Minute)
		sp.GraphClientTimeout = Duration(2 * time.Minute)
		return sp
	}
	t.Run("string values applied", func(t *testing.T) {
		cfg, err := PatchConfig(SharepointKey, actual(), sharepointJSON(false, `"drives_search_frequency":"5m"`))
		if err != nil {
			t.Fatalf("PatchConfig() error = %v", err)
		}
		sp, ok := cfg.(*SharepointConfig)
		if !ok {
			t.Fatalf("config type = %T, want *SharepointConfig", cfg)
		}
		if time.Duration(sp.DrivesSearchFrequency) != 5*time.Minute {
			t.Errorf("DrivesSearchFrequency = %v", time.Duration(sp.DrivesSearchFrequency))
		}
	})
	t.Run("absent keeps previous", func(t *testing.T) {
		cfg, err := PatchConfig(SharepointKey, actual(), sharepointJSON(false, ""))
		if err != nil {
			t.Fatalf("PatchConfig() error = %v", err)
		}
		sp, ok := cfg.(*SharepointConfig)
		if !ok {
			t.Fatalf("config type = %T, want *SharepointConfig", cfg)
		}
		if time.Duration(sp.GraphClientTimeout) != 2*time.Minute {
			t.Errorf("GraphClientTimeout = %v", time.Duration(sp.GraphClientTimeout))
		}
	})
	t.Run("out of bounds rejected", func(t *testing.T) {
		if _, err := PatchConfig(SharepointKey, actual(), sharepointJSON(false, `"graph_client_timeout":"10s"`)); err == nil {
			t.Error("expected error")
		}
	})
}

// Local configuration (viper-like): YAML -> map -> mapstructure with the SDK hook.
func TestSharepointConfig_durationMapstructure(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantSearch time.Duration
		wantClient time.Duration
		wantErr    bool
	}{
		{name: "strings", yaml: "drives-search-frequency: 5m\ngraph-client-timeout: 300s\n", wantSearch: 5 * time.Minute, wantClient: 300 * time.Second},
		{name: "composite", yaml: "drives-search-frequency: 1h30m\n", wantSearch: 90 * time.Minute},
		{name: "integer ns", yaml: "graph-client-timeout: 120000000000\n", wantClient: 2 * time.Minute},
		{name: "absent", yaml: "timeout-factor: 2\n"},
		{name: "invalid", yaml: "drives-search-frequency: abc\n", wantErr: true},
		{name: "empty string", yaml: "drives-search-frequency: \"\"\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw map[string]any
			if err := yaml.Unmarshal([]byte(tt.yaml), &raw); err != nil {
				t.Fatalf("yaml: %v", err)
			}
			var config ReconfigurableSharepointConfig
			dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				DecodeHook: DurationMapstructureHook(),
				Result:     &config,
			})
			if err != nil {
				t.Fatal(err)
			}
			err = dec.Decode(raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if time.Duration(config.DrivesSearchFrequency) != tt.wantSearch || time.Duration(config.GraphClientTimeout) != tt.wantClient {
				t.Errorf("got (%v, %v), want (%v, %v)", time.Duration(config.DrivesSearchFrequency), time.Duration(config.GraphClientTimeout), tt.wantSearch, tt.wantClient)
			}
		})
	}
}

// The tags themselves are part of the contract (§7.2): nanosecond parameters,
// never textual ones (which panic on sdk.Duration).
func TestSharepointConfig_durationValidateTags(t *testing.T) {
	want := map[string]string{
		"drives_search_frequency": "omitempty,min=60000000000,max=86400000000000",
		"graph_client_timeout":    "omitempty,min=60000000000,max=3600000000000",
	}
	for key, tag := range want {
		var field string
		switch key {
		case "drives_search_frequency":
			field = "DrivesSearchFrequency"
		default:
			field = "GraphClientTimeout"
		}
		got, ok := reflectTag(ReconfigurableSharepointConfig{}, field, "validate")
		if !ok || got != tag {
			t.Errorf("%s validate tag = %q, want %q", field, got, tag)
		}
	}
}

func reflectTag(v any, field, tag string) (string, bool) {
	f, ok := reflect.TypeOf(v).FieldByName(field)
	if !ok {
		return "", false
	}
	return f.Tag.Get(tag), true
}
