package config

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	configadapter "github.com/boshu2/agentops/cli/internal/adapters/config"
	"github.com/boshu2/agentops/cli/internal/clicontract"
	configapp "github.com/boshu2/agentops/cli/internal/config"
)

type fakeUseCases struct {
	showResult   configapp.ShowResult
	modelsResult configapp.ModelsResult
	writeRequest configapp.ModelsWriteRequest
	writeResult  configapp.ModelsWriteResult
}

func (useCases *fakeUseCases) Show(context.Context, string, bool) (configapp.ShowResult, error) {
	return useCases.showResult, nil
}

func (useCases *fakeUseCases) Models(context.Context) (configapp.ModelsResult, error) {
	return useCases.modelsResult, nil
}

func (useCases *fakeUseCases) WriteModels(_ context.Context, request configapp.ModelsWriteRequest) (configapp.ModelsWriteResult, error) {
	useCases.writeRequest = request
	return useCases.writeResult, nil
}

func TestModuleShowJSONUsesCommandWriter(t *testing.T) {
	useCases := &fakeUseCases{showResult: configapp.ShowResult{Resolved: &configapp.ResolvedConfig{}}}
	command := NewModule(useCases, clicontract.HostOptions{OutputMode: func() string { return "json" }, Verbose: func() bool { return false }, DryRun: func() bool { return false }}).Command()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"--show"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stdout.String(), "{\n") || !strings.Contains(stdout.String(), `"output"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestModuleModelsWriteParsesDelegatesAndRenders(t *testing.T) {
	useCases := &fakeUseCases{writeResult: configapp.ModelsWriteResult{Updated: true, DefaultTier: "quality"}}
	command := NewModule(useCases, clicontract.HostOptions{OutputMode: func() string { return "table" }, Verbose: func() bool { return false }, DryRun: func() bool { return false }}).Command()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"models", "--set-tier", "quality", "--set-skill", "council=budget"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if useCases.writeRequest.DefaultTier != "quality" || useCases.writeRequest.Skill != "council=budget" {
		t.Fatalf("request = %+v", useCases.writeRequest)
	}
	if got := stdout.String(); got != "Set default model tier to \"quality\"\nSet skill \"council\" tier to \"budget\"\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestModuleModelsDryRunDelegatesAndRendersPreview(t *testing.T) {
	useCases := &fakeUseCases{writeResult: configapp.ModelsWriteResult{DryRun: true, DefaultTier: "quality"}}
	command := NewModule(useCases, clicontract.HostOptions{OutputMode: func() string { return "table" }, Verbose: func() bool { return false }, DryRun: func() bool { return true }}).Command()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetArgs([]string{"models", "--set-tier", "quality"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !useCases.writeRequest.DryRun {
		t.Fatal("dry-run was not passed to the use case")
	}
	if got := stdout.String(); got != "Would set default model tier to \"quality\"\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestModuleContractDeclaresConfigEffects(t *testing.T) {
	contract := (Module{}).Contract()
	if contract.ExitClasses[0] == "" || contract.ExitClasses[1] == "" {
		t.Fatalf("exit classes = %+v", contract.ExitClasses)
	}
	if contract.Effects == 0 {
		t.Fatal("config effects are undeclared")
	}
}

// realHost returns host seams mirroring cmd/ao wiring for L2 tests.
func realHost(outputMode string) clicontract.HostOptions {
	return clicontract.HostOptions{
		OutputMode: func() string { return outputMode },
		Verbose:    func() bool { return false },
		DryRun:     func() bool { return false },
	}
}

// clearShowEnv blanks env vars that would leak into config resolution.
func clearShowEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"AGENTOPS_CONFIG", "AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE", "AGENTOPS_NO_SC"} {
		t.Setenv(key, "")
	}
}

// writeLegacyHome creates a sandbox HOME holding ONLY the deprecated
// ~/.agentops/config.yaml and chdirs into an empty project dir.
func writeLegacyHome(t *testing.T) (home, legacyPath string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	legacyPath = filepath.Join(home, ".agentops", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("output: json\nbase_dir: /legacy-base\n"), 0o644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}
	return home, legacyPath
}

func runRealShow(t *testing.T, outputMode string, args []string) string {
	t.Helper()
	command := NewModule(
		configapp.NewCommandService(configadapter.Gateway{}),
		realHost(outputMode),
	).Command()
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stdout)
	command.SetArgs(args)
	if err := command.Execute(); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
	return stdout.String()
}

// TestModuleShowLegacyFallbackTable_L2 pins novice edge 8 end to end: with
// only the deprecated ~/.agentops/config.yaml present, the files panel names
// the file actually read (labeled deprecated) and values loaded from it are
// attributed to that path — not to a missing canonical file or to "flag".
func TestModuleShowLegacyFallbackTable_L2(t *testing.T) {
	clearShowEnv(t)
	home, legacyPath := writeLegacyHome(t)

	got := runRealShow(t, "table", []string{"--show"})

	canonical := filepath.Join(home, ".agents", "ao", "config.yaml")
	wantFiles := "  ✓ Home:    " + legacyPath + " (deprecated location; move to " + canonical + ")\n"
	if !strings.Contains(got, wantFiles) {
		t.Errorf("files panel missing legacy read path line %q in:\n%s", wantFiles, got)
	}
	if strings.Contains(got, canonical+" (not found)") {
		t.Errorf("files panel still reports canonical home path as not found:\n%s", got)
	}
	if !strings.Contains(got, "output:   json  (from ~/.agentops/config.yaml (deprecated location))") {
		t.Errorf("output value not attributed to the deprecated location:\n%s", got)
	}
	if !strings.Contains(got, "base_dir: /legacy-base  (from ~/.agentops/config.yaml (deprecated location))") {
		t.Errorf("base_dir value not attributed to the deprecated location:\n%s", got)
	}
	if strings.Contains(got, "(from flag)") {
		t.Errorf("no flag was passed, yet a value is attributed to flag:\n%s", got)
	}
}

// TestModuleShowPrunesRPIAndDream_L2 pins novice edge 6: no `ao rpi` or
// `ao dream` command exists, so --show must not render rpi.* / dream.*
// resolved values in table or JSON output.
func TestModuleShowPrunesRPIAndDream_L2(t *testing.T) {
	clearShowEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())

	table := runRealShow(t, "table", []string{"--show"})
	for _, banned := range []string{"rpi.", "dream."} {
		if strings.Contains(table, banned) {
			t.Errorf("table output still renders %q section:\n%s", banned, table)
		}
	}

	raw := runRealShow(t, "json", []string{"--show"})
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("JSON output invalid: %v\n%s", err, raw)
	}
	gotKeys := make([]string, 0, len(parsed))
	for key := range parsed {
		gotKeys = append(gotKeys, key)
	}
	sort.Strings(gotKeys)
	wantKeys := []string{"base_dir", "models_default_tier", "output", "verbose"}
	if !slices.Equal(gotKeys, wantKeys) {
		t.Errorf("JSON keys = %v, want exactly %v", gotKeys, wantKeys)
	}
}

// TestModuleShowOutputFlagAttribution_L2 covers both sides of the flag
// misattribution: no flag passed → source is default, flag actually passed
// (via an inherited persistent --output, as cmd/ao mounts it) → source is flag.
func TestModuleShowOutputFlagAttribution_L2(t *testing.T) {
	t.Run("no flag passed resolves to default", func(t *testing.T) {
		clearShowEnv(t)
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Chdir(t.TempDir())

		raw := runRealShow(t, "json", []string{"--show"})
		var parsed struct {
			Output struct {
				Value  string `json:"value"`
				Source string `json:"source"`
			} `json:"output"`
		}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			t.Fatalf("JSON output invalid: %v\n%s", err, raw)
		}
		if parsed.Output.Value != "table" || parsed.Output.Source != "default" {
			t.Errorf("output = (%q, %q), want (table, default)", parsed.Output.Value, parsed.Output.Source)
		}
	})

	t.Run("explicit --output attributes to flag", func(t *testing.T) {
		clearShowEnv(t)
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Chdir(t.TempDir())

		root := &cobra.Command{Use: "ao"}
		root.PersistentFlags().StringP("output", "o", "table", "")
		root.PersistentFlags().Bool("json", false, "")
		module := NewModule(configapp.NewCommandService(configadapter.Gateway{}), realHost("json"))
		root.AddCommand(module.Command())
		var stdout bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stdout)
		root.SetArgs([]string{"config", "--show", "--output", "json"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		var parsed struct {
			Output struct {
				Value  string `json:"value"`
				Source string `json:"source"`
			} `json:"output"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
			t.Fatalf("JSON output invalid: %v\n%s", err, stdout.String())
		}
		if parsed.Output.Value != "json" || parsed.Output.Source != "flag" {
			t.Errorf("output = (%q, %q), want (json, flag)", parsed.Output.Value, parsed.Output.Source)
		}
	})
}

// TestModuleShowOmitsDeadNoSCKey_L2: AGENTOPS_NO_SC only feeds the dead
// Search.UseSmartConnections field, so --show must not surface it even when
// set, and the help text must not document it.
func TestModuleShowOmitsDeadNoSCKey_L2(t *testing.T) {
	clearShowEnv(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir())
	t.Setenv("AGENTOPS_NO_SC", "1")

	got := runRealShow(t, "table", []string{"--show"})
	if strings.Contains(got, "AGENTOPS_NO_SC") {
		t.Errorf("--show surfaces dead AGENTOPS_NO_SC key:\n%s", got)
	}
	if !strings.Contains(got, "(none set)") {
		t.Errorf("environment section should report (none set) when only dead keys are set:\n%s", got)
	}
	if strings.Contains(configLong, "AGENTOPS_NO_SC") {
		t.Error("config help text still documents dead AGENTOPS_NO_SC")
	}
}
