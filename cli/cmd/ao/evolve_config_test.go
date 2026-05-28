package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// resetEvolveConfigFlags resets the package-level flags between subtests so
// state from a prior run doesn't leak.
func resetEvolveConfigFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		evolveConfigShow = false
		evolveConfigJSON = false
	})
}

// writeEvolvePrefsFile writes contents to <dir>/.agents/evolve/preferences.yaml.
func writeEvolvePrefsFile(t *testing.T, dir, contents string) {
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

// runEvolveConfigCmd executes `ao evolve config <args...>` with a fresh
// stdout/stderr buffer and returns (stdout, stderr, err).
func runEvolveConfigCmd(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	full := append([]string{"evolve", "config"}, args...)
	rootCmd.SetArgs(full)
	err := rootCmd.Execute()
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	return stdout.String(), stderr.String(), err
}

func TestEvolveConfig_MissingFile_PrintsDefaults_YAML(t *testing.T) {
	dir := chdirTemp(t)
	_ = dir
	resetEvolveConfigFlags(t)

	stdout, _, err := runEvolveConfigCmd(t, "--show")
	if err != nil {
		t.Fatalf("evolve config: %v", err)
	}

	// Round-trip the YAML back into a map and assert the canonical default values.
	var got map[string]any
	if uerr := yaml.Unmarshal([]byte(stdout), &got); uerr != nil {
		t.Fatalf("unmarshal yaml: %v\n--- output ---\n%s", uerr, stdout)
	}
	if v, _ := got["schema_version"].(int); v != 1 {
		t.Fatalf("schema_version: want 1, got %v", got["schema_version"])
	}
	if v, _ := got["mode_default"].(string); v != "burst" {
		t.Fatalf("mode_default: want burst, got %v", got["mode_default"])
	}
	sf, ok := got["scope_filter"].(map[string]any)
	if !ok {
		t.Fatalf("scope_filter missing or wrong type: %v", got["scope_filter"])
	}
	if v, _ := sf["productive_threshold"].(int); v != 5 {
		t.Fatalf("scope_filter.productive_threshold: want 5, got %v", sf["productive_threshold"])
	}
	if v, _ := sf["scout_streak_halt"].(bool); v != true {
		t.Fatalf("scope_filter.scout_streak_halt: want true, got %v", sf["scout_streak_halt"])
	}
	if v, _ := got["recommended_pointer_strict"].(bool); v != true {
		t.Fatalf("recommended_pointer_strict: want true, got %v", got["recommended_pointer_strict"])
	}
	if v, _ := got["generator_layers_enabled"].(bool); v != true {
		t.Fatalf("generator_layers_enabled: want true, got %v", got["generator_layers_enabled"])
	}
	signals, ok := got["halt_signals"].([]any)
	if !ok || len(signals) != 2 {
		t.Fatalf("halt_signals: want list of len 2, got %v", got["halt_signals"])
	}
	if s, _ := signals[0].(string); s != ".agents/evolve/STOP" {
		t.Fatalf("halt_signals[0]: want .agents/evolve/STOP, got %v", signals[0])
	}
}

func TestEvolveConfig_ValidFile_OverridesDefaults_YAML(t *testing.T) {
	dir := chdirTemp(t)
	resetEvolveConfigFlags(t)
	writeEvolvePrefsFile(t, dir, `schema_version: 1
mode_default: loop
scope_filter:
  productive_threshold: 17
  scout_streak_halt: false
recommended_pointer_strict: false
halt_signals:
  - .agents/evolve/STOP
generator_layers_enabled: false
`)

	stdout, _, err := runEvolveConfigCmd(t, "--show")
	if err != nil {
		t.Fatalf("evolve config: %v", err)
	}

	var got map[string]any
	if uerr := yaml.Unmarshal([]byte(stdout), &got); uerr != nil {
		t.Fatalf("unmarshal yaml: %v\n--- output ---\n%s", uerr, stdout)
	}
	if v, _ := got["mode_default"].(string); v != "loop" {
		t.Fatalf("mode_default: want loop, got %v", got["mode_default"])
	}
	sf := got["scope_filter"].(map[string]any)
	if v, _ := sf["productive_threshold"].(int); v != 17 {
		t.Fatalf("productive_threshold: want 17, got %v", sf["productive_threshold"])
	}
	if v, _ := sf["scout_streak_halt"].(bool); v != false {
		t.Fatalf("scout_streak_halt: want false, got %v", sf["scout_streak_halt"])
	}
	if v, _ := got["recommended_pointer_strict"].(bool); v != false {
		t.Fatalf("recommended_pointer_strict: want false, got %v", got["recommended_pointer_strict"])
	}
	if v, _ := got["generator_layers_enabled"].(bool); v != false {
		t.Fatalf("generator_layers_enabled: want false, got %v", got["generator_layers_enabled"])
	}
}

func TestEvolveConfig_JSONFlag_ProducesValidJSON(t *testing.T) {
	dir := chdirTemp(t)
	_ = dir
	resetEvolveConfigFlags(t)

	stdout, _, err := runEvolveConfigCmd(t, "--show", "--json")
	if err != nil {
		t.Fatalf("evolve config --json: %v", err)
	}

	var got map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &got); jerr != nil {
		t.Fatalf("json.Unmarshal: %v\n--- output ---\n%s", jerr, stdout)
	}
	// JSON numbers come back as float64.
	if v, _ := got["schema_version"].(float64); v != 1 {
		t.Fatalf("schema_version: want 1, got %v", got["schema_version"])
	}
	if v, _ := got["mode_default"].(string); v != "burst" {
		t.Fatalf("mode_default: want burst, got %v", got["mode_default"])
	}
	sf, ok := got["scope_filter"].(map[string]any)
	if !ok {
		t.Fatalf("scope_filter missing or wrong type: %v", got["scope_filter"])
	}
	if v, _ := sf["productive_threshold"].(float64); v != 5 {
		t.Fatalf("productive_threshold: want 5, got %v", sf["productive_threshold"])
	}
}

func TestEvolveConfig_MalformedFile_ErrorsWithFileLineContext(t *testing.T) {
	dir := chdirTemp(t)
	resetEvolveConfigFlags(t)
	writeEvolvePrefsFile(t, dir, `schema_version: 1
scope_filter:
  productive_threshold: "abc"
`)

	_, _, err := runEvolveConfigCmd(t, "--show")
	if err == nil {
		t.Fatal("expected error from malformed preferences.yaml, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "preferences.yaml:") {
		t.Errorf("error %q missing preferences.yaml: prefix (file:line context)", msg)
	}
	if !strings.Contains(msg, "scope_filter.productive_threshold") {
		t.Errorf("error %q missing field name", msg)
	}
	if !strings.Contains(msg, "expected int") {
		t.Errorf("error %q missing type-mismatch description", msg)
	}
}

func TestEvolveConfig_WithoutShowFlag_Errors(t *testing.T) {
	dir := chdirTemp(t)
	_ = dir
	resetEvolveConfigFlags(t)

	_, _, err := runEvolveConfigCmd(t)
	if err == nil {
		t.Fatal("expected error when --show is not set, got nil")
	}
	if !strings.Contains(err.Error(), "--show") {
		t.Errorf("error %q missing --show hint", err.Error())
	}
}
