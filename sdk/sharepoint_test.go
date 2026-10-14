package sdk

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
)

// durationFields lists the SharePoint duration fields bounded by validation tags.
var durationFields = []string{"drives_search_frequency", "graph_client_timeout"}

func spDurationField(t *testing.T, c *SharepointConfig, key string) *Duration {
	t.Helper()
	switch key {
	case "drives_search_frequency":
		return &c.DrivesSearchFrequency
	case "graph_client_timeout":
		return &c.GraphClientTimeout
	}
	t.Fatalf("unknown duration field %q", key)
	return nil
}

// invalidFields validates config with the SDK validator and returns the JSON
// names of the failing fields.
func invalidFields(t *testing.T, config *SharepointConfig) map[string]bool {
	t.Helper()
	v, err := DefaultValidator()
	if err != nil {
		t.Fatalf("DefaultValidator() error = %v", err)
	}
	failed := map[string]bool{}
	err = v.Validate(config)
	if err == nil {
		return failed
	}
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatalf("Validate() unexpected error type %T: %v", err, err)
	}
	for _, e := range verrs {
		failed[e.Field()] = true
	}
	return failed
}

func TestSharepointConfig_durationBounds(t *testing.T) {
	bounds := map[string]struct{ min, max time.Duration }{
		"drives_search_frequency": {time.Minute, 24 * time.Hour},
		"graph_client_timeout":    {time.Minute, time.Hour},
	}
	for _, key := range durationFields {
		b := bounds[key]
		tests := []struct {
			name    string
			value   time.Duration
			wantErr bool
		}{
			{name: "zero uses default (omitempty)", value: 0},
			{name: "minimum", value: b.min},
			{name: "maximum", value: b.max},
			{name: "inside bounds", value: 5 * time.Minute},
			{name: "below minimum", value: b.min - time.Nanosecond, wantErr: true},
			{name: "30s", value: 30 * time.Second, wantErr: true},
			{name: "above maximum", value: b.max + time.Nanosecond, wantErr: true},
			{name: "negative", value: -1, wantErr: true},
		}
		for _, tt := range tests {
			t.Run(key+"/"+tt.name, func(t *testing.T) {
				config := &SharepointConfig{}
				*spDurationField(t, config, key) = Duration(tt.value)
				failed := invalidFields(t, config)
				if got := failed[key]; got != tt.wantErr {
					t.Errorf("%s=%v: invalid = %v, want %v", key, tt.value, got, tt.wantErr)
				}
				other := durationFields[0]
				if key == other {
					other = durationFields[1]
				}
				if failed[other] {
					t.Errorf("unrelated field %s reported invalid", other)
				}
			})
		}
	}
}

func TestSharepointConfig_durationJSON(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		wantSearch  time.Duration
		wantTimeout time.Duration
		wantErr     bool
	}{
		{name: "string", payload: `{"drives_search_frequency":"5m"}`, wantSearch: 5 * time.Minute},
		{name: "string timeout", payload: `{"graph_client_timeout":"300s"}`, wantTimeout: 300 * time.Second},
		{name: "integer nanoseconds", payload: `{"drives_search_frequency":300000000000,"graph_client_timeout":120000000000}`, wantSearch: 5 * time.Minute, wantTimeout: 2 * time.Minute},
		{name: "zero", payload: `{"drives_search_frequency":0}`},
		{name: "absent", payload: `{}`},
		{name: "invalid string", payload: `{"drives_search_frequency":"abc"}`, wantErr: true},
		{name: "invalid type", payload: `{"graph_client_timeout":true}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config SharepointConfig
			err := json.Unmarshal([]byte(tt.payload), &config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if time.Duration(config.DrivesSearchFrequency) != tt.wantSearch {
				t.Errorf("DrivesSearchFrequency = %v, want %v", time.Duration(config.DrivesSearchFrequency), tt.wantSearch)
			}
			if time.Duration(config.GraphClientTimeout) != tt.wantTimeout {
				t.Errorf("GraphClientTimeout = %v, want %v", time.Duration(config.GraphClientTimeout), tt.wantTimeout)
			}
		})
	}
}

func TestSharepointConfig_durationRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		search time.Duration
		client time.Duration
		want   string
	}{
		{name: "values", search: 5 * time.Minute, client: 90 * time.Second, want: `"drives_search_frequency":"5m0s"`},
		{name: "zero", want: `"drives_search_frequency":"0s"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := SharepointConfig{}
			in.DrivesSearchFrequency = Duration(tt.search)
			in.GraphClientTimeout = Duration(tt.client)
			raw, err := json.Marshal(in)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if !strings.Contains(string(raw), tt.want) {
				t.Errorf("Marshal() = %s, want it to contain %s", raw, tt.want)
			}
			var out SharepointConfig
			if err = json.Unmarshal(raw, &out); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if out.DrivesSearchFrequency != in.DrivesSearchFrequency || out.GraphClientTimeout != in.GraphClientTimeout {
				t.Errorf("round trip = (%v, %v), want (%v, %v)", out.DrivesSearchFrequency, out.GraphClientTimeout, in.DrivesSearchFrequency, in.GraphClientTimeout)
			}
		})
	}
}

func findField(fields []ConfigField, key string) (ConfigField, bool) {
	for _, f := range fields {
		if f.Key == key {
			return f, true
		}
		if sub, ok := findField(f.Properties, key); ok {
			return sub, true
		}
	}
	return ConfigField{}, false
}

func TestSharepointConfig_durationFrontFields(t *testing.T) {
	loader, err := NewConnectorsTypesLoader(true)
	if err != nil {
		t.Fatalf("NewConnectorsTypesLoader() error = %v", err)
	}
	connectorType, err := loader.GetConnectorType(SharepointKey)
	if err != nil {
		t.Fatalf("GetConnectorType() error = %v", err)
	}
	defaultConfig, err := InitDefault(SharepointKey)
	if err != nil {
		t.Fatalf("InitDefault() error = %v", err)
	}
	spConfig, ok := defaultConfig.(*SharepointConfig)
	if !ok {
		t.Fatalf("InitDefault() type = %T, want *SharepointConfig", defaultConfig)
	}
	direct, err := getConfigFields(*spConfig)
	if err != nil {
		t.Fatalf("getConfigFields() error = %v", err)
	}

	sources := map[string][]ConfigField{"loader": connectorType.Configs, "getConfigFields": direct}
	for source, fields := range sources {
		for _, key := range durationFields {
			t.Run(source+"/"+key, func(t *testing.T) {
				f, ok := findField(fields, key)
				if !ok {
					t.Fatalf("field %s not exposed", key)
				}
				if f.Type != String {
					t.Errorf("Type = %q, want %q", f.Type, String)
				}
				if !f.Reconfigurable {
					t.Errorf("field %s should be reconfigurable", key)
				}
				if f.Description == "" {
					t.Errorf("field %s has no description", key)
				}
				if _, isDuration := f.DefaultValue.(Duration); !isDuration {
					t.Errorf("DefaultValue type = %T, want Duration", f.DefaultValue)
				}
			})
		}
	}
}
