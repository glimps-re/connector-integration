package sdk

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gopkg.in/yaml.v3"
)

func newHostConfigWithNewKeys() HostConfig {
	return HostConfig{
		WorkDir: "/var/lib/gmhost/work",
		UnanalyzablePolicy: HostUnanalyzablePolicy{
			TooBig: "pass", Empty: "mitigate", Encrypted: "error", ExtractError: "as_file",
		},
		Monitoring: HostMonitoringConfig{
			DisableFSWatcher: true, MaxTrackedMemory: "256MiB", MaxTrackedDirPercent: 25,
		},
		Move: HostMoveConfig{Mode: "copy", MinThroughput: "10MiB"},
	}
}

func TestHostConfig_NewKeysRoundTrip(t *testing.T) {
	want := newHostConfigWithNewKeys()

	t.Run("json", func(t *testing.T) {
		b, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		var got HostConfig
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("json round trip diff = %s", diff)
		}
	})

	t.Run("yaml", func(t *testing.T) {
		b, err := yaml.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		var got HostConfig
		if err := yaml.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("yaml round trip diff = %s", diff)
		}
	})
}

func TestHostConfig_NewKeysValidation(t *testing.T) {
	validate := validator.New()
	tests := []struct {
		name    string
		mutate  func(c *HostConfig)
		wantErr bool
	}{
		{name: "zero values accepted", mutate: func(c *HostConfig) {}},
		{name: "all valid", mutate: func(c *HostConfig) { *c = newHostConfigWithNewKeys() }},
		{name: "too_big invalid", mutate: func(c *HostConfig) { c.UnanalyzablePolicy.TooBig = "skip" }, wantErr: true},
		{name: "empty invalid", mutate: func(c *HostConfig) { c.UnanalyzablePolicy.Empty = "error" }, wantErr: true},
		{name: "encrypted invalid", mutate: func(c *HostConfig) { c.UnanalyzablePolicy.Encrypted = "skip" }, wantErr: true},
		{name: "extract_error invalid", mutate: func(c *HostConfig) { c.UnanalyzablePolicy.ExtractError = "pass" }, wantErr: true},
		{name: "move mode invalid", mutate: func(c *HostConfig) { c.Move.Mode = "link" }, wantErr: true},
		{name: "dir percent negative", mutate: func(c *HostConfig) { c.Monitoring.MaxTrackedDirPercent = -1 }, wantErr: true},
		{name: "quarantine format invalid", mutate: func(c *HostConfig) { c.Quarantine.Format = "gz" }, wantErr: true},
		{name: "dir percent above 100", mutate: func(c *HostConfig) { c.Monitoring.MaxTrackedDirPercent = 101 }, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c HostConfig
			tt.mutate(&c)
			err := errors.Join(
				validate.Struct(c.UnanalyzablePolicy),
				validate.Struct(c.Monitoring),
				validate.Struct(c.Move),
				validate.Struct(c.Quarantine),
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate.Struct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInitDefault_HostNewKeys(t *testing.T) {
	config, err := InitDefault(HostKey)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := config.(*HostConfig)
	if !ok {
		t.Fatalf("InitDefault returned %T", config)
	}
	want := HostUnanalyzablePolicy{TooBig: "error", Empty: "pass"}
	if c.UnanalyzablePolicy != want || c.Move.Mode != "copy" {
		t.Errorf("defaults = %+v, move.mode %q; want %+v, copy", c.UnanalyzablePolicy, c.Move.Mode, want)
	}
	if c.WorkDir != "" || c.Monitoring.DisableFSWatcher {
		t.Errorf("other new keys must default to zero (connector default), got %+v", c)
	}
	if c.Move.MinThroughput != "10MiB" {
		t.Errorf("move.min_throughput = %q, want 10MiB", c.Move.MinThroughput)
	}
	if c.Monitoring.MaxTrackedMemory != "256MiB" || c.Monitoring.MaxTrackedDirPercent != 25 {
		t.Errorf("monitoring defaults = %q, %d; want 256MiB, 25", c.Monitoring.MaxTrackedMemory, c.Monitoring.MaxTrackedDirPercent)
	}
}

func TestGetConfigFields_HostExposesNewKeys(t *testing.T) {
	fields, err := getConfigFields(HostConfig{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ConfigField{}
	collectFields(fields, "", got)
	for _, k := range []string{"work_dir", "unanalyzable_policy", "unanalyzable_policy.too_big", "unanalyzable_policy.extract_error", "monitoring.disable_fs_watcher", "monitoring.max_tracked_memory", "monitoring.max_tracked_dir_percent", "move.mode", "move.min_throughput"} {
		f, ok := got[k]
		if !ok || f.Description == "" {
			t.Errorf("field %q missing or without description", k)
		}
	}
}

func collectFields(fields []ConfigField, prefix string, out map[string]ConfigField) {
	for _, f := range fields {
		key := prefix + f.Key
		out[key] = f
		collectFields(f.Properties, key+".", out)
	}
}

func TestHostConfig_DescMentionsDefault(t *testing.T) {
	raw, err := InitDefault(HostKey)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := raw.(*HostConfig)
	if !ok {
		t.Fatalf("InitDefault returned %T", raw)
	}
	fields, err := getConfigFields(HostConfig{})
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]ConfigField{}
	collectFields(fields, "", byKey)
	for key, def := range map[string]string{
		"unanalyzable_policy.too_big": c.UnanalyzablePolicy.TooBig,
		"unanalyzable_policy.empty":   c.UnanalyzablePolicy.Empty,
		"move.mode":                   c.Move.Mode,
		"move.min_throughput":         c.Move.MinThroughput,
	} {
		want := "'" + def + "'"
		if d := byKey[key].Description; !strings.Contains(d, want) || !strings.Contains(d, "default") {
			t.Errorf("%s description %q does not state default %s", key, d, want)
		}
	}
}

// empty: pass needs the move by copy: it is refused with the move action on in
// rename mode, and only then.
func TestHostConfig_emptyPassWithRename(t *testing.T) {
	tests := []struct {
		name    string
		empty   string
		move    bool
		mode    string
		wantErr bool
	}{
		{"pass, move on, rename", "pass", true, "rename", true},
		{"pass, move on, copy", "pass", true, "copy", false},
		{"pass, move on, no mode", "pass", true, "", false},
		{"pass, move off, rename", "pass", false, "rename", false},
		{"absent, move on, rename", "", true, "rename", false},
		{"skip, move on, rename", "skip", true, "rename", false},
		{"mitigate, move on, rename", "mitigate", true, "rename", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := DefaultValidator()
			if err != nil {
				t.Fatal(err)
			}
			base, err := InitDefault(HostKey)
			if err != nil {
				t.Fatal(err)
			}
			c, ok := base.(*HostConfig)
			if !ok {
				t.Fatalf("InitDefault returned %T", base)
			}
			c.GMalwareAPIURL, c.GMalwareAPIToken, c.Paths = "https://gmalware.example", "00000000-00000000-00000000-00000000-00000000", []string{"/in"}
			c.UnanalyzablePolicy.Empty, c.Actions.Move, c.Move.Mode = tt.empty, tt.move, tt.mode
			err = v.Validate(c)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				return
			}
			var verrs validator.ValidationErrors
			if !errors.As(err, &verrs) || len(verrs) != 1 {
				t.Fatalf("error = %v, want one validation error", err)
			}
			if got := verrs[0].Translate(v.Trans); got != HostEmptyPassWithRenameMessage {
				t.Errorf("message = %q, want the connector's wording", got)
			}
		})
	}
}
