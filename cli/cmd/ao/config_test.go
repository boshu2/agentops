// practices: [twelve-factor-app, pragmatic-programmer]
package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/spf13/cobra"
)

// TestConfigFlag_MaterializesAndIsHonored proves the END-TO-END --config chain
// (the cross-family pawl's concern on age-or2c) by executing the REAL
// rootCmd.PersistentPreRunE — NOT syncConfigFlagToEnv directly. If PersistentPreRunE
// is absent or stops materializing --config into AGENTOPS_CONFIG, this test fails,
// guarding the actual flag-threading wiring. It then verifies config.Load honors the
// explicit file and the ambient home config does NOT leak underneath it.
func TestConfigFlag_MaterializesAndIsHonored(t *testing.T) {
	// A home config that sets output=json — the explicit file is silent on it.
	home := t.TempDir()
	t.Setenv("HOME", home)
	homeDir := filepath.Join(home, ".agents", "ao")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, "config.yaml"), []byte("output: json\n"), 0o644); err != nil {
		t.Fatalf("write home config: %v", err)
	}

	// The explicit --config file sets base_dir, silent on output.
	explicit := filepath.Join(t.TempDir(), "explicit.yaml")
	if err := os.WriteFile(explicit, []byte("base_dir: /explicit/base\n"), 0o644); err != nil {
		t.Fatalf("write explicit config: %v", err)
	}

	// Run from an empty, non-git temp cwd so PersistentPreRunE's worktree/git
	// repair is a safe no-op; isolate ambient env.
	t.Chdir(t.TempDir())
	t.Setenv("AGENTOPS_OUTPUT", "")
	t.Setenv("AGENTOPS_BASE_DIR", "")
	t.Setenv("AGENTOPS_CONFIG", "")

	// Save/restore the package globals + rootCmd context PersistentPreRunE mutates
	// (cmd.Context() is nil when no command has run, and WithValue panics on a nil
	// parent — so seed a context first).
	origCfg, origOutput, origJSON := cfgFile, output, jsonFlag
	origCtx := rootCmd.Context()
	rootCmd.SetContext(context.Background())
	t.Cleanup(func() {
		cfgFile, output, jsonFlag = origCfg, origOutput, origJSON
		rootCmd.SetContext(origCtx)
	})

	// Set --config through the REAL rootCmd flag set, then run the REAL pre-run hook.
	if err := rootCmd.PersistentFlags().Set("config", explicit); err != nil {
		t.Fatalf("set --config flag: %v", err)
	}
	t.Cleanup(func() { _ = rootCmd.PersistentFlags().Set("config", "") })

	if rootCmd.PersistentPreRunE == nil {
		t.Fatal("rootCmd has no PersistentPreRunE — the --config flag chain is unwired")
	}
	if err := rootCmd.PersistentPreRunE(rootCmd, nil); err != nil {
		t.Fatalf("PersistentPreRunE: %v", err)
	}

	// The real pre-run must have materialized --config into AGENTOPS_CONFIG...
	if got := os.Getenv("AGENTOPS_CONFIG"); got != explicit {
		t.Fatalf("PersistentPreRunE did not materialize --config into AGENTOPS_CONFIG: got %q, want %q", got, explicit)
	}
	// ...so config.Load honors the explicit file and home does NOT leak.
	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseDir != "/explicit/base" {
		t.Errorf("BaseDir = %q, want /explicit/base — --config not honored via the real pre-run", cfg.BaseDir)
	}
	if cfg.Output != "table" {
		t.Errorf("Output = %q, want \"table\" — home leaked under --config via the real pre-run", cfg.Output)
	}
}

func TestRunConfig_NoFlags_ShowsHelp(t *testing.T) {
	// When configShow is false, runConfig should call cmd.Help()
	oldShow := configShow
	configShow = false
	defer func() { configShow = oldShow }()

	cmd := &cobra.Command{}
	cmd.SetOut(&strings.Builder{})

	// cmd.Help() returns nil, so this should succeed
	if err := runConfig(cmd, nil); err != nil {
		t.Fatalf("runConfig without --show: %v", err)
	}
}

// TestConfigModelsRemoved_UnknownCommand pins the removal of the `ao config
// models` surface: "models" must now be rejected exactly like any other
// unknown token under config, and the config tree must not register a models
// subcommand or its --set-tier/--set-skill flags.
func TestConfigModelsRemoved_UnknownCommand(t *testing.T) {
	command := newConfigCommand()
	for _, child := range command.Commands() {
		if child.Name() == "models" {
			t.Fatal("config still registers a models subcommand")
		}
	}

	fresh := newConfigCommand()
	fresh.SetOut(&strings.Builder{})
	fresh.SetErr(&strings.Builder{})
	fresh.SetArgs([]string{"models"})
	err := fresh.Execute()
	if err == nil {
		t.Fatal("expected error for removed `config models` subcommand, got nil")
	}
	if want := `unknown command "models" for "config"`; err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestRunConfig_ShowJSON(t *testing.T) {
	oldShow := configShow
	configShow = true
	defer func() { configShow = oldShow }()

	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	stdout, err := captureStdout(t, func() error {
		return runConfig(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runConfig --show --json: %v", err)
	}

	var parsed config.ResolvedConfig
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got: %q (%v)", stdout, err)
	}

	// Verify key fields are present
	if parsed.Output.Value == nil {
		t.Error("expected output value in resolved config")
	}
	// Removed-subsystem config (rpi.*, dream.*) must not serialize: no `ao rpi`
	// or `ao dream` command exists in 3.3 (novice-test edge 6).
	if strings.Contains(stdout, "dream_report_dir") || strings.Contains(stdout, "rpi_") {
		t.Errorf("resolved config JSON leaks removed-subsystem keys:\n%s", stdout)
	}
}

func TestRunConfig_ShowTable(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	oldShow := configShow
	configShow = true
	defer func() { configShow = oldShow }()

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	stdout, err := captureStdout(t, func() error {
		return runConfig(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runConfig --show: %v", err)
	}

	if !strings.Contains(stdout, "AgentOps Configuration") {
		t.Errorf("expected 'AgentOps Configuration' header, got: %q", stdout)
	}
	if !strings.Contains(stdout, "Resolved values:") {
		t.Errorf("expected 'Resolved values:' section, got: %q", stdout)
	}
	if !strings.Contains(stdout, "output:") {
		t.Errorf("expected 'output:' in resolved values, got: %q", stdout)
	}
	// Removed-subsystem config (rpi.*, dream.*) must not render (edge 6).
	if strings.Contains(stdout, "dream.") || strings.Contains(stdout, "rpi.") {
		t.Errorf("resolved values render removed-subsystem keys, got: %q", stdout)
	}
	if !strings.Contains(stdout, "Environment variables") {
		t.Errorf("expected 'Environment variables' section, got: %q", stdout)
	}
}

func TestRunConfig_ShowTable_NoConfigFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	oldShow := configShow
	configShow = true
	defer func() { configShow = oldShow }()

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	stdout, err := captureStdout(t, func() error {
		return runConfig(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runConfig: %v", err)
	}

	// With no config files, should show "not found" markers
	if !strings.Contains(stdout, "not found") {
		t.Errorf("expected 'not found' markers for missing config files, got: %q", stdout)
	}
}

func TestRunConfig_ShowTable_WithEnvVars(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("AGENTOPS_OUTPUT", "json")

	oldShow := configShow
	configShow = true
	defer func() { configShow = oldShow }()

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	stdout, err := captureStdout(t, func() error {
		return runConfig(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runConfig: %v", err)
	}

	if !strings.Contains(stdout, "AGENTOPS_OUTPUT=json") {
		t.Errorf("expected AGENTOPS_OUTPUT=json in output, got: %q", stdout)
	}
}

func TestRunConfig_ShowTable_NoEnvVars(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Clear all known env vars
	for _, env := range []string{
		"AGENTOPS_CONFIG", "AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR",
		"AGENTOPS_VERBOSE", "AGENTOPS_NO_SC", "AGENTOPS_RPI_WORKTREE_MODE",
		"AGENTOPS_RPI_RUNTIME", "AGENTOPS_RPI_RUNTIME_MODE",
		"AGENTOPS_RPI_RUNTIME_COMMAND", "AGENTOPS_RPI_AO_COMMAND",
		"AGENTOPS_RPI_BD_COMMAND", "AGENTOPS_RPI_TMUX_COMMAND",
		"AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD",
	} {
		t.Setenv(env, "")
	}

	oldShow := configShow
	configShow = true
	defer func() { configShow = oldShow }()

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	stdout, err := captureStdout(t, func() error {
		return runConfig(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runConfig: %v", err)
	}

	if !strings.Contains(stdout, "(none set)") {
		t.Errorf("expected '(none set)' for no env vars, got: %q", stdout)
	}
}

func TestConfigCommandsRejectPositionalArgs(t *testing.T) {
	if err := configCmd.Args(configCmd, []string{"junk"}); err == nil {
		t.Fatal("config accepted an unexpected positional argument")
	}
}
