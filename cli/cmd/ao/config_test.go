package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/spf13/cobra"
)

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
	if parsed.DreamReportDir.Value == nil {
		t.Error("expected dream_report_dir in resolved config")
	}
}

func TestRunConfig_ShowTable(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

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
	if !strings.Contains(stdout, "dream.report_dir:") {
		t.Errorf("expected dream.report_dir in resolved values, got: %q", stdout)
	}
	if !strings.Contains(stdout, "Environment variables") {
		t.Errorf("expected 'Environment variables' section, got: %q", stdout)
	}
}

func TestRunConfig_ShowTable_NoConfigFiles(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

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
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

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

func TestRunConfigModels_Table(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	stdout, err := captureStdout(t, func() error {
		return runConfigModels(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runConfigModels: %v", err)
	}

	if !strings.Contains(stdout, "Model Cost Tiers") {
		t.Errorf("expected 'Model Cost Tiers' header, got: %q", stdout)
	}
	if !strings.Contains(stdout, "Default tier: balanced") {
		t.Errorf("expected default tier balanced, got: %q", stdout)
	}
	if !strings.Contains(stdout, "quality") {
		t.Errorf("expected 'quality' tier listed, got: %q", stdout)
	}
	if !strings.Contains(stdout, "opus") {
		t.Errorf("expected 'opus' model for quality tier, got: %q", stdout)
	}
}

func TestRunConfigModels_JSON(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	oldOutput := output
	output = "json"
	defer func() { output = oldOutput }()

	stdout, err := captureStdout(t, func() error {
		return runConfigModels(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runConfigModels --json: %v", err)
	}

	var parsed config.ModelsConfig
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got: %q (%v)", stdout, err)
	}

	if parsed.DefaultTier != "balanced" {
		t.Errorf("expected default_tier=balanced, got %q", parsed.DefaultTier)
	}
	if len(parsed.Tiers) != 3 {
		t.Errorf("expected 3 tiers, got %d", len(parsed.Tiers))
	}
}

func TestRunConfigModels_WithEnvOverride(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	t.Setenv("AGENTOPS_MODEL_TIER", "budget")

	oldOutput := output
	output = "table"
	defer func() { output = oldOutput }()

	stdout, err := captureStdout(t, func() error {
		return runConfigModels(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runConfigModels: %v", err)
	}

	if !strings.Contains(stdout, "Default tier: budget") {
		t.Errorf("expected default tier budget from env, got: %q", stdout)
	}
	if !strings.Contains(stdout, "AGENTOPS_MODEL_TIER=budget") {
		t.Errorf("expected env var in output, got: %q", stdout)
	}
}

func TestConfigModels_SetTier(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	// Clear env so project config is used from cwd
	t.Setenv("AGENTOPS_CONFIG", "")

	oldSetTier := modelsSetTier
	oldSetSkill := modelsSetSkill
	modelsSetTier = "quality"
	modelsSetSkill = ""
	defer func() {
		modelsSetTier = oldSetTier
		modelsSetSkill = oldSetSkill
	}()

	stdout, err := captureStdout(t, func() error {
		return runConfigModels(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runConfigModels --set-tier: %v", err)
	}

	if !strings.Contains(stdout, `Set default model tier to "quality"`) {
		t.Errorf("expected confirmation message, got: %q", stdout)
	}

	// Verify config was written
	cfg, loadErr := config.Load(nil)
	if loadErr != nil {
		t.Fatalf("Load after set-tier: %v", loadErr)
	}
	if cfg.Models.DefaultTier != "quality" {
		t.Errorf("saved DefaultTier = %q, want %q", cfg.Models.DefaultTier, "quality")
	}
}

func TestConfigModels_SetSkill(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	t.Setenv("AGENTOPS_CONFIG", "")

	oldSetTier := modelsSetTier
	oldSetSkill := modelsSetSkill
	modelsSetTier = ""
	modelsSetSkill = "council=quality"
	defer func() {
		modelsSetTier = oldSetTier
		modelsSetSkill = oldSetSkill
	}()

	stdout, err := captureStdout(t, func() error {
		return runConfigModels(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runConfigModels --set-skill: %v", err)
	}

	if !strings.Contains(stdout, `Set skill "council" tier to "quality"`) {
		t.Errorf("expected confirmation message, got: %q", stdout)
	}

	cfg, loadErr := config.Load(nil)
	if loadErr != nil {
		t.Fatalf("Load after set-skill: %v", loadErr)
	}
	if cfg.Models.SkillOverrides["council"] != "quality" {
		t.Errorf("saved SkillOverrides[council] = %q, want %q", cfg.Models.SkillOverrides["council"], "quality")
	}
}

func TestConfigModels_SetTierAndSkill_JSON(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	t.Setenv("AGENTOPS_CONFIG", "")

	oldOutput := output
	oldSetTier := modelsSetTier
	oldSetSkill := modelsSetSkill
	output = "json"
	modelsSetTier = "budget"
	modelsSetSkill = "council=quality"
	defer func() {
		output = oldOutput
		modelsSetTier = oldSetTier
		modelsSetSkill = oldSetSkill
	}()

	stdout, err := captureStdout(t, func() error {
		return runConfigModels(&cobra.Command{}, nil)
	})
	if err != nil {
		t.Fatalf("runConfigModels --set-tier --set-skill --json: %v", err)
	}
	if strings.Contains(stdout, "Set default model tier") || strings.Contains(stdout, "Set skill") {
		t.Fatalf("expected JSON-only output, got human text: %q", stdout)
	}

	var parsed configModelsWriteResult
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got: %q (%v)", stdout, err)
	}
	if !parsed.Updated {
		t.Fatal("expected updated=true")
	}
	if parsed.DefaultTier != "budget" {
		t.Errorf("default_tier = %q, want %q", parsed.DefaultTier, "budget")
	}
	if parsed.SkillOverrides["council"] != "quality" {
		t.Errorf("skill_overrides[council] = %q, want %q", parsed.SkillOverrides["council"], "quality")
	}

	cfg, loadErr := config.Load(nil)
	if loadErr != nil {
		t.Fatalf("Load after JSON write: %v", loadErr)
	}
	if cfg.Models.DefaultTier != "budget" {
		t.Errorf("saved DefaultTier = %q, want %q", cfg.Models.DefaultTier, "budget")
	}
	if cfg.Models.SkillOverrides["council"] != "quality" {
		t.Errorf("saved SkillOverrides[council] = %q, want %q", cfg.Models.SkillOverrides["council"], "quality")
	}
}

func TestConfigModels_SetTier_InvalidTier(t *testing.T) {
	oldSetTier := modelsSetTier
	oldSetSkill := modelsSetSkill
	defer func() {
		modelsSetTier = oldSetTier
		modelsSetSkill = oldSetSkill
	}()

	tests := []struct {
		name    string
		tier    string
		wantErr string
	}{
		{
			name:    "unknown tier",
			tier:    "premium",
			wantErr: "invalid tier",
		},
		{
			name:    "inherit not allowed for default",
			tier:    "inherit",
			wantErr: "inherit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelsSetTier = tt.tier
			modelsSetSkill = ""

			err := runConfigModels(&cobra.Command{}, nil)
			if err == nil {
				t.Fatal("expected error for invalid tier, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestRunConfig_ShowTable_NoEnvVars(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

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
