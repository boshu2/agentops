package evolve

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writePrefs writes contents to <dir>/.agents/evolve/preferences.yaml.
func writePrefs(t *testing.T, dir, contents string) {
	t.Helper()
	full := filepath.Join(dir, ".agents", "evolve")
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(full, "preferences.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestDefaults_KnownValues(t *testing.T) {
	d := Defaults()
	if d.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion: want 1, got %d", d.SchemaVersion)
	}
	if d.ModeDefault != "burst" {
		t.Fatalf("ModeDefault: want burst, got %q", d.ModeDefault)
	}
	if d.ScopeFilter.ProductiveThreshold != 5 {
		t.Fatalf("ProductiveThreshold: want 5, got %d", d.ScopeFilter.ProductiveThreshold)
	}
	if d.ScopeFilter.ScoutStreakHalt != true {
		t.Fatalf("ScoutStreakHalt: want true, got %v", d.ScopeFilter.ScoutStreakHalt)
	}
	if d.RecommendedPointerStrict != true {
		t.Fatalf("RecommendedPointerStrict: want true, got %v", d.RecommendedPointerStrict)
	}
	if len(d.HaltSignals) != 2 {
		t.Fatalf("HaltSignals len: want 2, got %d", len(d.HaltSignals))
	}
	if d.HaltSignals[0] != ".agents/evolve/STOP" {
		t.Fatalf("HaltSignals[0]: want .agents/evolve/STOP, got %q", d.HaltSignals[0])
	}
	if d.HaltSignals[1] != ".agents/evolve/KILL" {
		t.Fatalf("HaltSignals[1]: want .agents/evolve/KILL, got %q", d.HaltSignals[1])
	}
	if d.GeneratorLayersEnabled != true {
		t.Fatalf("GeneratorLayersEnabled: want true, got %v", d.GeneratorLayersEnabled)
	}
}

func TestLoadFromDir_MissingFile_ReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadFromDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	want := Defaults()
	if got.SchemaVersion != want.SchemaVersion {
		t.Fatalf("SchemaVersion: want %d, got %d", want.SchemaVersion, got.SchemaVersion)
	}
	if got.ModeDefault != want.ModeDefault {
		t.Fatalf("ModeDefault: want %q, got %q", want.ModeDefault, got.ModeDefault)
	}
	if got.ScopeFilter.ProductiveThreshold != want.ScopeFilter.ProductiveThreshold {
		t.Fatalf("ProductiveThreshold: want %d, got %d",
			want.ScopeFilter.ProductiveThreshold, got.ScopeFilter.ProductiveThreshold)
	}
}

func TestLoadFromDir_ValidFile_OverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	writePrefs(t, dir, `schema_version: 1
mode_default: loop
scope_filter:
  productive_threshold: 12
  scout_streak_halt: false
recommended_pointer_strict: false
halt_signals:
  - .agents/evolve/STOP
  - .agents/evolve/CUSTOM
generator_layers_enabled: false
`)
	got, err := LoadFromDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if got.ModeDefault != "loop" {
		t.Fatalf("ModeDefault: want loop, got %q", got.ModeDefault)
	}
	if got.ScopeFilter.ProductiveThreshold != 12 {
		t.Fatalf("ProductiveThreshold: want 12, got %d", got.ScopeFilter.ProductiveThreshold)
	}
	if got.ScopeFilter.ScoutStreakHalt != false {
		t.Fatalf("ScoutStreakHalt: want false, got %v", got.ScopeFilter.ScoutStreakHalt)
	}
	if got.RecommendedPointerStrict != false {
		t.Fatalf("RecommendedPointerStrict: want false, got %v", got.RecommendedPointerStrict)
	}
	if len(got.HaltSignals) != 2 {
		t.Fatalf("HaltSignals len: want 2, got %d", len(got.HaltSignals))
	}
	if got.HaltSignals[1] != ".agents/evolve/CUSTOM" {
		t.Fatalf("HaltSignals[1]: want .agents/evolve/CUSTOM, got %q", got.HaltSignals[1])
	}
	if got.GeneratorLayersEnabled != false {
		t.Fatalf("GeneratorLayersEnabled: want false, got %v", got.GeneratorLayersEnabled)
	}
}

func TestLoadFromDir_PartialOverride_KeepsOtherDefaults(t *testing.T) {
	dir := t.TempDir()
	writePrefs(t, dir, `schema_version: 1
mode_default: loop
`)
	got, err := LoadFromDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if got.ModeDefault != "loop" {
		t.Fatalf("ModeDefault: want loop, got %q", got.ModeDefault)
	}
	if got.ScopeFilter.ProductiveThreshold != 5 {
		t.Fatalf("ProductiveThreshold default preserved: want 5, got %d",
			got.ScopeFilter.ProductiveThreshold)
	}
	if got.RecommendedPointerStrict != true {
		t.Fatalf("RecommendedPointerStrict default preserved: want true, got %v",
			got.RecommendedPointerStrict)
	}
}

func TestLoadFromDir_Errors(t *testing.T) {
	cases := []struct {
		name     string
		contents string
		wantSub  []string // substrings expected in the error message
	}{
		{
			name:     "malformed_yaml",
			contents: "this is: not: valid: yaml: :",
			wantSub:  []string{"preferences.yaml", "parse yaml"},
		},
		{
			name: "wrong_schema_version",
			contents: `schema_version: 2
mode_default: burst
`,
			wantSub: []string{"preferences.yaml:1:", "schema_version: expected 1, got 2"},
		},
		{
			name: "productive_threshold_not_int",
			contents: `schema_version: 1
scope_filter:
  productive_threshold: "abc"
`,
			wantSub: []string{"preferences.yaml:3:", "scope_filter.productive_threshold: expected int"},
		},
		{
			name: "productive_threshold_out_of_range_low",
			contents: `schema_version: 1
scope_filter:
  productive_threshold: 0
`,
			wantSub: []string{"preferences.yaml:3:", "scope_filter.productive_threshold: expected int in [1..100], got 0"},
		},
		{
			name: "productive_threshold_out_of_range_high",
			contents: `schema_version: 1
scope_filter:
  productive_threshold: 101
`,
			wantSub: []string{"preferences.yaml:3:", "scope_filter.productive_threshold: expected int in [1..100], got 101"},
		},
		{
			name: "unknown_top_key",
			contents: `schema_version: 1
bogus_extra_key: hello
`,
			wantSub: []string{"preferences.yaml:2:", `unknown key "bogus_extra_key"`},
		},
		{
			name: "unknown_nested_key",
			contents: `schema_version: 1
scope_filter:
  bogus_inner: 1
`,
			wantSub: []string{"preferences.yaml:3:", `unknown key "bogus_inner" under scope_filter`},
		},
		{
			name: "wrong_mode",
			contents: `schema_version: 1
mode_default: turbo
`,
			wantSub: []string{"preferences.yaml:2:", "mode_default: expected one of [burst, loop], got \"turbo\""},
		},
		{
			name: "halt_signals_not_list",
			contents: `schema_version: 1
halt_signals: STOP
`,
			wantSub: []string{"preferences.yaml:2:", "halt_signals: expected list"},
		},
		{
			name: "scope_filter_not_mapping",
			contents: `schema_version: 1
scope_filter: nope
`,
			wantSub: []string{"preferences.yaml:2:", "scope_filter: expected mapping"},
		},
		{
			name: "generator_layers_not_bool",
			contents: `schema_version: 1
generator_layers_enabled: "yes please"
`,
			wantSub: []string{"preferences.yaml:2:", "generator_layers_enabled: expected bool"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writePrefs(t, dir, tc.contents)
			_, err := LoadFromDir(context.Background(), dir)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			msg := err.Error()
			for _, sub := range tc.wantSub {
				if !strings.Contains(msg, sub) {
					t.Fatalf("error %q missing expected substring %q", msg, sub)
				}
			}
		})
	}
}

func TestLoadFromDir_EmptyFile_ReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	writePrefs(t, dir, "")
	got, err := LoadFromDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if got.ModeDefault != "burst" {
		t.Fatalf("ModeDefault: want burst, got %q", got.ModeDefault)
	}
	if got.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion: want 1, got %d", got.SchemaVersion)
	}
}

func TestLoadFromDir_RootNotMapping_Errors(t *testing.T) {
	dir := t.TempDir()
	writePrefs(t, dir, "- a\n- b\n")
	_, err := LoadFromDir(context.Background(), dir)
	if err == nil {
		t.Fatal("expected error for non-mapping root")
	}
	if !strings.Contains(err.Error(), "expected mapping at root") {
		t.Fatalf("error %q missing 'expected mapping at root'", err.Error())
	}
}
