package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Output != "table" {
		t.Errorf("Default Output = %q, want %q", cfg.Output, "table")
	}
	if cfg.BaseDir != ".agents/ao" {
		t.Errorf("Default BaseDir = %q, want %q", cfg.BaseDir, ".agents/ao")
	}
	if cfg.Verbose {
		t.Error("Default Verbose = true, want false")
	}
	if cfg.Search.DefaultLimit != 10 {
		t.Errorf("Default Search.DefaultLimit = %d, want %d", cfg.Search.DefaultLimit, 10)
	}
	if !cfg.Search.UseSmartConnections {
		t.Error("Default Search.UseSmartConnections = false, want true")
	}
	if cfg.Flywheel.AutoPromoteThreshold != "24h" {
		t.Errorf("Default Flywheel.AutoPromoteThreshold = %q, want %q", cfg.Flywheel.AutoPromoteThreshold, "24h")
	}
}

func TestMerge(t *testing.T) {
	dst := Default()
	src := &Config{
		Output:  "json",
		BaseDir: "/custom/path",
	}

	result := merge(dst, src)

	if result.Output != "json" {
		t.Errorf("merge Output = %q, want %q", result.Output, "json")
	}
	if result.BaseDir != "/custom/path" {
		t.Errorf("merge BaseDir = %q, want %q", result.BaseDir, "/custom/path")
	}
	// Defaults should be preserved when not overridden
	if result.Search.DefaultLimit != 10 {
		t.Errorf("merge preserved DefaultLimit = %d, want %d", result.Search.DefaultLimit, 10)
	}
}

func TestMerge_BooleanOverride(t *testing.T) {
	dst := Default()
	if !dst.Search.UseSmartConnections {
		t.Fatal("Precondition: default UseSmartConnections should be true")
	}

	// Test explicit false override
	src := &Config{
		Search: SearchConfig{
			UseSmartConnections:    false,
			UseSmartConnectionsSet: true,
		},
	}

	result := merge(dst, src)

	if result.Search.UseSmartConnections {
		t.Error("merge should override UseSmartConnections to false")
	}
	if !result.Search.UseSmartConnectionsSet {
		t.Error("merge should set UseSmartConnectionsSet")
	}
}

func TestMerge_BooleanNotSet(t *testing.T) {
	dst := Default()
	src := &Config{
		Output: "json",
		// UseSmartConnectionsSet is false (default)
	}

	result := merge(dst, src)

	// Should keep default (true) since not explicitly set
	if !result.Search.UseSmartConnections {
		t.Error("merge should preserve default UseSmartConnections when not set")
	}
}

func TestApplyEnv(t *testing.T) {
	// Save and restore env
	origOutput := os.Getenv("AGENTOPS_OUTPUT")
	origVerbose := os.Getenv("AGENTOPS_VERBOSE")
	origNoSC := os.Getenv("AGENTOPS_NO_SC")
	defer func() {
		_ = os.Setenv("AGENTOPS_OUTPUT", origOutput)   //nolint:errcheck // test env restore
		_ = os.Setenv("AGENTOPS_VERBOSE", origVerbose) //nolint:errcheck // test env restore
		_ = os.Setenv("AGENTOPS_NO_SC", origNoSC)      //nolint:errcheck // test env restore
	}()

	_ = os.Setenv("AGENTOPS_OUTPUT", "yaml")  //nolint:errcheck // test env setup
	_ = os.Setenv("AGENTOPS_VERBOSE", "true") //nolint:errcheck // test env setup
	_ = os.Setenv("AGENTOPS_NO_SC", "1")      //nolint:errcheck // test env setup

	cfg := Default()
	cfg = applyEnv(cfg)

	if cfg.Output != "yaml" {
		t.Errorf("applyEnv Output = %q, want %q", cfg.Output, "yaml")
	}
	if !cfg.Verbose {
		t.Error("applyEnv Verbose = false, want true")
	}
	if cfg.Search.UseSmartConnections {
		t.Error("applyEnv UseSmartConnections = true, want false")
	}
	if !cfg.Search.UseSmartConnectionsSet {
		t.Error("applyEnv should set UseSmartConnectionsSet when AGENTOPS_NO_SC is set")
	}
}

func TestLoadFromPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write test config
	content := `
output: json
base_dir: /custom/olympus
verbose: true
search:
  default_limit: 20
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFromPath(configPath)
	if err != nil {
		t.Fatalf("loadFromPath() error = %v", err)
	}

	if cfg.Output != "json" {
		t.Errorf("loadFromPath Output = %q, want %q", cfg.Output, "json")
	}
	if cfg.BaseDir != "/custom/olympus" {
		t.Errorf("loadFromPath BaseDir = %q, want %q", cfg.BaseDir, "/custom/olympus")
	}
	if !cfg.Verbose {
		t.Error("loadFromPath Verbose = false, want true")
	}
	if cfg.Search.DefaultLimit != 20 {
		t.Errorf("loadFromPath DefaultLimit = %d, want %d", cfg.Search.DefaultLimit, 20)
	}
}

func TestLoadFromPath_NotExists(t *testing.T) {
	cfg, err := loadFromPath("/nonexistent/config.yaml")
	// Should return nil config and error, but not panic
	if cfg != nil {
		t.Errorf("loadFromPath for nonexistent file should return nil config")
	}
	if err == nil {
		t.Errorf("loadFromPath for nonexistent file should return error")
	}
}

func TestLoadFromPath_Empty(t *testing.T) {
	cfg, err := loadFromPath("")
	if cfg != nil || err != nil {
		t.Errorf("loadFromPath(\"\") = %v, %v; want nil, nil", cfg, err)
	}
}

func TestResolve(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", "")
	// Test that flag overrides take precedence
	rc := Resolve("json", "/flag/path", true)

	if rc.Output.Value != "json" {
		t.Errorf("Resolve Output.Value = %v, want %q", rc.Output.Value, "json")
	}
	if rc.Output.Source != SourceFlag {
		t.Errorf("Resolve Output.Source = %v, want %v", rc.Output.Source, SourceFlag)
	}
	if rc.BaseDir.Value != "/flag/path" {
		t.Errorf("Resolve BaseDir.Value = %v, want %q", rc.BaseDir.Value, "/flag/path")
	}
	if rc.Verbose.Value != true {
		t.Errorf("Resolve Verbose.Value = %v, want true", rc.Verbose.Value)
	}
}

func TestResolve_Defaults(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", "")
	// No flags, no env — should get defaults
	for _, key := range []string{"AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE"} {
		t.Setenv(key, "")
	}

	rc := Resolve("", "", false)

	if rc.Output.Value != "table" {
		t.Errorf("Resolve default Output.Value = %v, want %q", rc.Output.Value, "table")
	}
	if rc.Verbose.Value != false {
		t.Errorf("Resolve default Verbose.Value = %v, want false", rc.Verbose.Value)
	}
}

func TestResolve_EnvOverride(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", "")
	t.Setenv("AGENTOPS_OUTPUT", "yaml")
	t.Setenv("AGENTOPS_BASE_DIR", "/env/path")
	t.Setenv("AGENTOPS_VERBOSE", "1")

	rc := Resolve("", "", false)

	if rc.Output.Value != "yaml" {
		t.Errorf("Resolve env Output.Value = %v, want %q", rc.Output.Value, "yaml")
	}
	if rc.Output.Source != SourceEnv {
		t.Errorf("Resolve env Output.Source = %v, want %v", rc.Output.Source, SourceEnv)
	}
	if rc.BaseDir.Value != "/env/path" {
		t.Errorf("Resolve env BaseDir.Value = %v, want %q", rc.BaseDir.Value, "/env/path")
	}
	if rc.BaseDir.Source != SourceEnv {
		t.Errorf("Resolve env BaseDir.Source = %v, want %v", rc.BaseDir.Source, SourceEnv)
	}
	if rc.Verbose.Value != true {
		t.Errorf("Resolve env Verbose.Value = %v, want true", rc.Verbose.Value)
	}
	if rc.Verbose.Source != SourceEnv {
		t.Errorf("Resolve env Verbose.Source = %v, want %v", rc.Verbose.Source, SourceEnv)
	}
}

func TestResolve_RPIEnvOverrides(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", "")
	t.Setenv("AGENTOPS_RPI_WORKTREE_MODE", "always")
	t.Setenv("AGENTOPS_RPI_RUNTIME", "direct")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "stream")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "runtime-env")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "ao-env")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "bd-env")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "tmux-env")

	rc := Resolve("", "", false)

	if rc.RPIWorktreeMode.Value != "always" || rc.RPIWorktreeMode.Source != SourceEnv {
		t.Fatalf("RPIWorktreeMode = (%v, %v), want (always, %v)", rc.RPIWorktreeMode.Value, rc.RPIWorktreeMode.Source, SourceEnv)
	}
	if rc.RPIRuntimeMode.Value != "stream" || rc.RPIRuntimeMode.Source != SourceEnv {
		t.Fatalf("RPIRuntimeMode = (%v, %v), want (stream, %v)", rc.RPIRuntimeMode.Value, rc.RPIRuntimeMode.Source, SourceEnv)
	}
	if rc.RPIRuntimeCommand.Value != "runtime-env" || rc.RPIRuntimeCommand.Source != SourceEnv {
		t.Fatalf("RPIRuntimeCommand = (%v, %v), want (runtime-env, %v)", rc.RPIRuntimeCommand.Value, rc.RPIRuntimeCommand.Source, SourceEnv)
	}
	if rc.RPIAOCommand.Value != "ao-env" || rc.RPIAOCommand.Source != SourceEnv {
		t.Fatalf("RPIAOCommand = (%v, %v), want (ao-env, %v)", rc.RPIAOCommand.Value, rc.RPIAOCommand.Source, SourceEnv)
	}
	if rc.RPIBDCommand.Value != "bd-env" || rc.RPIBDCommand.Source != SourceEnv {
		t.Fatalf("RPIBDCommand = (%v, %v), want (bd-env, %v)", rc.RPIBDCommand.Value, rc.RPIBDCommand.Source, SourceEnv)
	}
	if rc.RPITmuxCommand.Value != "tmux-env" || rc.RPITmuxCommand.Source != SourceEnv {
		t.Fatalf("RPITmuxCommand = (%v, %v), want (tmux-env, %v)", rc.RPITmuxCommand.Value, rc.RPITmuxCommand.Source, SourceEnv)
	}
}

func TestResolveStringField(t *testing.T) {
	tests := []struct {
		name       string
		home       string
		project    string
		env        string
		flag       string
		def        string
		wantValue  string
		wantSource Source
	}{
		{
			name:       "default only",
			def:        "table",
			wantValue:  "table",
			wantSource: SourceDefault,
		},
		{
			name:       "home overrides default",
			home:       "json",
			def:        "table",
			wantValue:  "json",
			wantSource: SourceHome,
		},
		{
			name:       "project overrides home",
			home:       "json",
			project:    "yaml",
			def:        "table",
			wantValue:  "yaml",
			wantSource: SourceProject,
		},
		{
			name:       "env overrides project",
			home:       "json",
			project:    "yaml",
			env:        "csv",
			def:        "table",
			wantValue:  "csv",
			wantSource: SourceEnv,
		},
		{
			name:       "flag overrides everything",
			home:       "json",
			project:    "yaml",
			env:        "csv",
			flag:       "text",
			def:        "table",
			wantValue:  "text",
			wantSource: SourceFlag,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveStringField(tt.home, tt.project, tt.env, tt.flag, tt.def)
			if got.Value != tt.wantValue {
				t.Errorf("resolveStringField() Value = %v, want %v", got.Value, tt.wantValue)
			}
			if got.Source != tt.wantSource {
				t.Errorf("resolveStringField() Source = %v, want %v", got.Source, tt.wantSource)
			}
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name     string
		envVal   string
		wantBool bool
		wantSet  bool
	}{
		{name: "true string", envVal: "true", wantBool: true, wantSet: true},
		{name: "1 string", envVal: "1", wantBool: true, wantSet: true},
		{name: "false string", envVal: "false", wantBool: false, wantSet: false},
		{name: "empty string", envVal: "", wantBool: false, wantSet: false},
		{name: "random string", envVal: "yes", wantBool: false, wantSet: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TEST_BOOL_KEY", tt.envVal)
			gotBool, gotSet := getEnvBool("TEST_BOOL_KEY")
			if gotBool != tt.wantBool {
				t.Errorf("getEnvBool() bool = %v, want %v", gotBool, tt.wantBool)
			}
			if gotSet != tt.wantSet {
				t.Errorf("getEnvBool() set = %v, want %v", gotSet, tt.wantSet)
			}
		})
	}
}

func TestGetEnvString(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		wantVal string
		wantSet bool
	}{
		{name: "set value", envVal: "hello", wantVal: "hello", wantSet: true},
		{name: "empty value", envVal: "", wantVal: "", wantSet: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TEST_STR_KEY", tt.envVal)
			gotVal, gotSet := getEnvString("TEST_STR_KEY")
			if gotVal != tt.wantVal {
				t.Errorf("getEnvString() val = %q, want %q", gotVal, tt.wantVal)
			}
			if gotSet != tt.wantSet {
				t.Errorf("getEnvString() set = %v, want %v", gotSet, tt.wantSet)
			}
		})
	}
}

func TestApplyEnv_BaseDir(t *testing.T) {
	t.Setenv("AGENTOPS_OUTPUT", "")
	t.Setenv("AGENTOPS_VERBOSE", "")
	t.Setenv("AGENTOPS_NO_SC", "")
	t.Setenv("AGENTOPS_BASE_DIR", "/env/base")

	cfg := Default()
	cfg = applyEnv(cfg)

	if cfg.BaseDir != "/env/base" {
		t.Errorf("applyEnv BaseDir = %q, want %q", cfg.BaseDir, "/env/base")
	}
}

func TestApplyEnv_VerboseVariants(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		wantVer bool
	}{
		{name: "true", envVal: "true", wantVer: true},
		{name: "1", envVal: "1", wantVer: true},
		{name: "false", envVal: "false", wantVer: false},
		{name: "empty", envVal: "", wantVer: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AGENTOPS_OUTPUT", "")
			t.Setenv("AGENTOPS_BASE_DIR", "")
			t.Setenv("AGENTOPS_NO_SC", "")
			t.Setenv("AGENTOPS_VERBOSE", tt.envVal)

			cfg := Default()
			cfg = applyEnv(cfg)

			if cfg.Verbose != tt.wantVer {
				t.Errorf("applyEnv Verbose = %v, want %v for AGENTOPS_VERBOSE=%q", cfg.Verbose, tt.wantVer, tt.envVal)
			}
		})
	}
}

func TestApplyEnv_NoSCVariants(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		wantSC  bool
		wantSet bool
	}{
		{name: "true disables SC", envVal: "true", wantSC: false, wantSet: true},
		{name: "1 disables SC", envVal: "1", wantSC: false, wantSet: true},
		{name: "false keeps SC", envVal: "false", wantSC: true, wantSet: false},
		{name: "empty keeps SC", envVal: "", wantSC: true, wantSet: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AGENTOPS_OUTPUT", "")
			t.Setenv("AGENTOPS_BASE_DIR", "")
			t.Setenv("AGENTOPS_VERBOSE", "")
			t.Setenv("AGENTOPS_NO_SC", tt.envVal)

			cfg := Default()
			cfg = applyEnv(cfg)

			if cfg.Search.UseSmartConnections != tt.wantSC {
				t.Errorf("applyEnv UseSmartConnections = %v, want %v", cfg.Search.UseSmartConnections, tt.wantSC)
			}
			if cfg.Search.UseSmartConnectionsSet != tt.wantSet {
				t.Errorf("applyEnv UseSmartConnectionsSet = %v, want %v", cfg.Search.UseSmartConnectionsSet, tt.wantSet)
			}
		})
	}
}

func TestMerge_Paths(t *testing.T) {
	dst := Default()
	src := &Config{
		Paths: PathsConfig{
			LearningsDir:   "/custom/learnings",
			PatternsDir:    "/custom/patterns",
			RetrosDir:      "/custom/retros",
			ResearchDir:    "/custom/research",
			PlansDir:       "/custom/plans",
			ClaudePlansDir: "/custom/claude-plans",
			CitationsFile:  "/custom/citations.jsonl",
			TranscriptsDir: "/custom/transcripts",
		},
	}

	result := merge(dst, src)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"LearningsDir", result.Paths.LearningsDir, "/custom/learnings"},
		{"PatternsDir", result.Paths.PatternsDir, "/custom/patterns"},
		{"RetrosDir", result.Paths.RetrosDir, "/custom/retros"},
		{"ResearchDir", result.Paths.ResearchDir, "/custom/research"},
		{"PlansDir", result.Paths.PlansDir, "/custom/plans"},
		{"ClaudePlansDir", result.Paths.ClaudePlansDir, "/custom/claude-plans"},
		{"CitationsFile", result.Paths.CitationsFile, "/custom/citations.jsonl"},
		{"TranscriptsDir", result.Paths.TranscriptsDir, "/custom/transcripts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("merge Paths.%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestMerge_PathsPreservedWhenEmpty(t *testing.T) {
	dst := Default()
	src := &Config{
		Output: "json",
		// All Paths fields are empty strings (zero value)
	}

	result := merge(dst, src)

	// Defaults should be preserved
	if result.Paths.LearningsDir != ".agents/learnings" {
		t.Errorf("merge should preserve default LearningsDir, got %q", result.Paths.LearningsDir)
	}
	if result.Paths.PatternsDir != ".agents/patterns" {
		t.Errorf("merge should preserve default PatternsDir, got %q", result.Paths.PatternsDir)
	}
}

func TestMerge_ForgeOverrides(t *testing.T) {
	dst := Default()
	src := &Config{
		Forge: ForgeConfig{
			MaxContentLength: 5000,
			ProgressInterval: 500,
		},
	}

	result := merge(dst, src)

	if result.Forge.MaxContentLength != 5000 {
		t.Errorf("merge Forge.MaxContentLength = %d, want 5000", result.Forge.MaxContentLength)
	}
	if result.Forge.ProgressInterval != 500 {
		t.Errorf("merge Forge.ProgressInterval = %d, want 500", result.Forge.ProgressInterval)
	}
}

func TestMerge_VerboseOverride(t *testing.T) {
	dst := Default()
	src := &Config{Verbose: true}

	result := merge(dst, src)

	if !result.Verbose {
		t.Error("merge Verbose = false, want true")
	}
}

func TestMerge_SearchDefaultLimit(t *testing.T) {
	dst := Default()
	src := &Config{
		Search: SearchConfig{DefaultLimit: 50},
	}

	result := merge(dst, src)

	if result.Search.DefaultLimit != 50 {
		t.Errorf("merge Search.DefaultLimit = %d, want 50", result.Search.DefaultLimit)
	}
}

func TestLoad_WithFlagOverrides(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", "")
	// Clear env vars to avoid interference
	t.Setenv("AGENTOPS_OUTPUT", "")
	t.Setenv("AGENTOPS_BASE_DIR", "")
	t.Setenv("AGENTOPS_VERBOSE", "")
	t.Setenv("AGENTOPS_NO_SC", "")

	overrides := &Config{
		Output:  "json",
		BaseDir: "/flag/base",
		Verbose: true,
	}

	cfg, err := Load(overrides)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Output != "json" {
		t.Errorf("Load Output = %q, want %q", cfg.Output, "json")
	}
	if cfg.BaseDir != "/flag/base" {
		t.Errorf("Load BaseDir = %q, want %q", cfg.BaseDir, "/flag/base")
	}
	if !cfg.Verbose {
		t.Error("Load Verbose = false, want true")
	}
}

func TestLoad_NilOverrides(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", "")
	t.Setenv("AGENTOPS_OUTPUT", "")
	t.Setenv("AGENTOPS_BASE_DIR", "")
	t.Setenv("AGENTOPS_VERBOSE", "")
	t.Setenv("AGENTOPS_NO_SC", "")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should get defaults
	if cfg.Output != "table" {
		t.Errorf("Load nil Output = %q, want %q", cfg.Output, "table")
	}
	if cfg.BaseDir != ".agents/ao" {
		t.Errorf("Load nil BaseDir = %q, want %q", cfg.BaseDir, ".agents/ao")
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", "")
	t.Setenv("AGENTOPS_OUTPUT", "yaml")
	t.Setenv("AGENTOPS_BASE_DIR", "/env/dir")
	t.Setenv("AGENTOPS_VERBOSE", "1")
	t.Setenv("AGENTOPS_NO_SC", "")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Output != "yaml" {
		t.Errorf("Load env Output = %q, want %q", cfg.Output, "yaml")
	}
	if cfg.BaseDir != "/env/dir" {
		t.Errorf("Load env BaseDir = %q, want %q", cfg.BaseDir, "/env/dir")
	}
	if !cfg.Verbose {
		t.Error("Load env Verbose = false, want true")
	}
}

func TestLoadFromPath_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `{{{invalid yaml`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFromPath(configPath)
	if err == nil {
		t.Error("loadFromPath for invalid YAML should return error")
	}
	if cfg != nil {
		t.Error("loadFromPath for invalid YAML should return nil config")
	}
}

func TestDefault_Paths(t *testing.T) {
	cfg := Default()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"LearningsDir", cfg.Paths.LearningsDir, ".agents/learnings"},
		{"PatternsDir", cfg.Paths.PatternsDir, ".agents/patterns"},
		{"RetrosDir", cfg.Paths.RetrosDir, ".agents/retros"},
		{"ResearchDir", cfg.Paths.ResearchDir, ".agents/research"},
		{"PlansDir", cfg.Paths.PlansDir, ".agents/plans"},
		{"CitationsFile", cfg.Paths.CitationsFile, ".agents/ao/citations.jsonl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("Default Paths.%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}

	// Home-relative paths should contain home dir
	homeDir, _ := os.UserHomeDir()
	if cfg.Paths.ClaudePlansDir != filepath.Join(homeDir, ".claude", "plans") {
		t.Errorf("Default Paths.ClaudePlansDir = %q, want suffix .claude/plans", cfg.Paths.ClaudePlansDir)
	}
	if cfg.Paths.TranscriptsDir != filepath.Join(homeDir, ".claude", "projects") {
		t.Errorf("Default Paths.TranscriptsDir = %q, want suffix .claude/projects", cfg.Paths.TranscriptsDir)
	}
}

func TestDefault_Forge(t *testing.T) {
	cfg := Default()

	if cfg.Forge.MaxContentLength != 0 {
		t.Errorf("Default Forge.MaxContentLength = %d, want 0", cfg.Forge.MaxContentLength)
	}
	if cfg.Forge.ProgressInterval != 1000 {
		t.Errorf("Default Forge.ProgressInterval = %d, want 1000", cfg.Forge.ProgressInterval)
	}
}

func TestDefault_RPI(t *testing.T) {
	cfg := Default()
	if cfg.RPI.WorktreeMode != "auto" {
		t.Errorf("Default RPI.WorktreeMode = %q, want %q", cfg.RPI.WorktreeMode, "auto")
	}
	if cfg.RPI.RuntimeMode != "auto" {
		t.Errorf("Default RPI.RuntimeMode = %q, want %q", cfg.RPI.RuntimeMode, "auto")
	}
	if cfg.RPI.RuntimeCommand != "claude" {
		t.Errorf("Default RPI.RuntimeCommand = %q, want %q", cfg.RPI.RuntimeCommand, "claude")
	}
	if cfg.RPI.AOCommand != "ao" {
		t.Errorf("Default RPI.AOCommand = %q, want %q", cfg.RPI.AOCommand, "ao")
	}
	if cfg.RPI.BDCommand != "bd" {
		t.Errorf("Default RPI.BDCommand = %q, want %q", cfg.RPI.BDCommand, "bd")
	}
	if cfg.RPI.TmuxCommand != "tmux" {
		t.Errorf("Default RPI.TmuxCommand = %q, want %q", cfg.RPI.TmuxCommand, "tmux")
	}
}

func TestMerge_RPI(t *testing.T) {
	dst := Default()
	src := &Config{
		RPI: RPIConfig{
			WorktreeMode:   "never",
			RuntimeMode:    "stream",
			RuntimeCommand: "codex",
			AOCommand:      "ao-custom",
			BDCommand:      "bd-custom",
			TmuxCommand:    "tmux-custom",
		},
	}

	result := merge(dst, src)
	if result.RPI.WorktreeMode != "never" {
		t.Errorf("merge RPI.WorktreeMode = %q, want %q", result.RPI.WorktreeMode, "never")
	}
	if result.RPI.RuntimeMode != "stream" {
		t.Errorf("merge RPI.RuntimeMode = %q, want %q", result.RPI.RuntimeMode, "stream")
	}
	if result.RPI.RuntimeCommand != "codex" {
		t.Errorf("merge RPI.RuntimeCommand = %q, want %q", result.RPI.RuntimeCommand, "codex")
	}
	if result.RPI.AOCommand != "ao-custom" {
		t.Errorf("merge RPI.AOCommand = %q, want %q", result.RPI.AOCommand, "ao-custom")
	}
	if result.RPI.BDCommand != "bd-custom" {
		t.Errorf("merge RPI.BDCommand = %q, want %q", result.RPI.BDCommand, "bd-custom")
	}
	if result.RPI.TmuxCommand != "tmux-custom" {
		t.Errorf("merge RPI.TmuxCommand = %q, want %q", result.RPI.TmuxCommand, "tmux-custom")
	}
}

func TestMerge_Flywheel(t *testing.T) {
	dst := Default()
	src := &Config{
		Flywheel: FlywheelConfig{
			AutoPromoteThreshold: "36h",
		},
	}

	result := merge(dst, src)
	if result.Flywheel.AutoPromoteThreshold != "36h" {
		t.Errorf("merge Flywheel.AutoPromoteThreshold = %q, want %q", result.Flywheel.AutoPromoteThreshold, "36h")
	}
}

func TestMerge_RPIPreservedWhenEmpty(t *testing.T) {
	dst := Default()
	src := &Config{
		Output: "json",
		// RPI config fields are empty strings
	}

	result := merge(dst, src)
	if result.RPI.WorktreeMode != "auto" {
		t.Errorf("merge should preserve default RPI.WorktreeMode, got %q", result.RPI.WorktreeMode)
	}
	if result.RPI.RuntimeMode != "auto" {
		t.Errorf("merge should preserve default RPI.RuntimeMode, got %q", result.RPI.RuntimeMode)
	}
	if result.RPI.RuntimeCommand != "claude" {
		t.Errorf("merge should preserve default RPI.RuntimeCommand, got %q", result.RPI.RuntimeCommand)
	}
	if result.RPI.AOCommand != "ao" {
		t.Errorf("merge should preserve default RPI.AOCommand, got %q", result.RPI.AOCommand)
	}
	if result.RPI.BDCommand != "bd" {
		t.Errorf("merge should preserve default RPI.BDCommand, got %q", result.RPI.BDCommand)
	}
	if result.RPI.TmuxCommand != "tmux" {
		t.Errorf("merge should preserve default RPI.TmuxCommand, got %q", result.RPI.TmuxCommand)
	}
}

func TestApplyEnv_RPIWorktreeMode(t *testing.T) {
	t.Setenv("AGENTOPS_OUTPUT", "")
	t.Setenv("AGENTOPS_BASE_DIR", "")
	t.Setenv("AGENTOPS_VERBOSE", "")
	t.Setenv("AGENTOPS_NO_SC", "")
	t.Setenv("AGENTOPS_RPI_WORKTREE_MODE", "never")
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "")

	cfg := Default()
	cfg = applyEnv(cfg)

	if cfg.RPI.WorktreeMode != "never" {
		t.Errorf("applyEnv RPI.WorktreeMode = %q, want %q", cfg.RPI.WorktreeMode, "never")
	}
}

func TestApplyEnv_FlywheelAutoPromoteThreshold(t *testing.T) {
	t.Setenv("AGENTOPS_OUTPUT", "")
	t.Setenv("AGENTOPS_BASE_DIR", "")
	t.Setenv("AGENTOPS_VERBOSE", "")
	t.Setenv("AGENTOPS_NO_SC", "")
	t.Setenv("AGENTOPS_RPI_WORKTREE_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "")
	t.Setenv("AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD", "48h")

	cfg := Default()
	cfg = applyEnv(cfg)

	if cfg.Flywheel.AutoPromoteThreshold != "48h" {
		t.Errorf("applyEnv Flywheel.AutoPromoteThreshold = %q, want %q", cfg.Flywheel.AutoPromoteThreshold, "48h")
	}
}

func TestApplyEnv_RPIWorktreeModeEmpty(t *testing.T) {
	t.Setenv("AGENTOPS_OUTPUT", "")
	t.Setenv("AGENTOPS_BASE_DIR", "")
	t.Setenv("AGENTOPS_VERBOSE", "")
	t.Setenv("AGENTOPS_NO_SC", "")
	t.Setenv("AGENTOPS_RPI_WORKTREE_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "")

	cfg := Default()
	cfg = applyEnv(cfg)

	if cfg.RPI.WorktreeMode != "auto" {
		t.Errorf("applyEnv RPI.WorktreeMode = %q, want %q (unchanged from default)", cfg.RPI.WorktreeMode, "auto")
	}
}

func TestApplyEnv_RPIRuntimeMode(t *testing.T) {
	t.Setenv("AGENTOPS_OUTPUT", "")
	t.Setenv("AGENTOPS_BASE_DIR", "")
	t.Setenv("AGENTOPS_VERBOSE", "")
	t.Setenv("AGENTOPS_NO_SC", "")
	t.Setenv("AGENTOPS_RPI_WORKTREE_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME", "direct")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "stream")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "")

	cfg := Default()
	cfg = applyEnv(cfg)

	// AGENTOPS_RPI_RUNTIME_MODE should win when both are set.
	if cfg.RPI.RuntimeMode != "stream" {
		t.Errorf("applyEnv RPI.RuntimeMode = %q, want %q", cfg.RPI.RuntimeMode, "stream")
	}
}

func TestApplyEnv_RPIRuntimeCommand(t *testing.T) {
	t.Setenv("AGENTOPS_OUTPUT", "")
	t.Setenv("AGENTOPS_BASE_DIR", "")
	t.Setenv("AGENTOPS_VERBOSE", "")
	t.Setenv("AGENTOPS_NO_SC", "")
	t.Setenv("AGENTOPS_RPI_WORKTREE_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "codex")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "")

	cfg := Default()
	cfg = applyEnv(cfg)

	if cfg.RPI.RuntimeCommand != "codex" {
		t.Errorf("applyEnv RPI.RuntimeCommand = %q, want %q", cfg.RPI.RuntimeCommand, "codex")
	}
}

func TestApplyEnv_RPICommandOverrides(t *testing.T) {
	t.Setenv("AGENTOPS_OUTPUT", "")
	t.Setenv("AGENTOPS_BASE_DIR", "")
	t.Setenv("AGENTOPS_VERBOSE", "")
	t.Setenv("AGENTOPS_NO_SC", "")
	t.Setenv("AGENTOPS_RPI_WORKTREE_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "aox")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "bdx")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "tmuxx")

	cfg := Default()
	cfg = applyEnv(cfg)

	if cfg.RPI.AOCommand != "aox" {
		t.Errorf("applyEnv RPI.AOCommand = %q, want %q", cfg.RPI.AOCommand, "aox")
	}
	if cfg.RPI.BDCommand != "bdx" {
		t.Errorf("applyEnv RPI.BDCommand = %q, want %q", cfg.RPI.BDCommand, "bdx")
	}
	if cfg.RPI.TmuxCommand != "tmuxx" {
		t.Errorf("applyEnv RPI.TmuxCommand = %q, want %q", cfg.RPI.TmuxCommand, "tmuxx")
	}
}

func TestLoadFromPath_WithRPI(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `
rpi:
  worktree_mode: always
  runtime_mode: stream
  runtime_command: codex
  ao_command: aox
  bd_command: bdx
  tmux_command: tmuxx
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFromPath(configPath)
	if err != nil {
		t.Fatalf("loadFromPath() error = %v", err)
	}
	if cfg.RPI.WorktreeMode != "always" {
		t.Errorf("loadFromPath RPI.WorktreeMode = %q, want %q", cfg.RPI.WorktreeMode, "always")
	}
	if cfg.RPI.RuntimeMode != "stream" {
		t.Errorf("loadFromPath RPI.RuntimeMode = %q, want %q", cfg.RPI.RuntimeMode, "stream")
	}
	if cfg.RPI.RuntimeCommand != "codex" {
		t.Errorf("loadFromPath RPI.RuntimeCommand = %q, want %q", cfg.RPI.RuntimeCommand, "codex")
	}
	if cfg.RPI.AOCommand != "aox" {
		t.Errorf("loadFromPath RPI.AOCommand = %q, want %q", cfg.RPI.AOCommand, "aox")
	}
	if cfg.RPI.BDCommand != "bdx" {
		t.Errorf("loadFromPath RPI.BDCommand = %q, want %q", cfg.RPI.BDCommand, "bdx")
	}
	if cfg.RPI.TmuxCommand != "tmuxx" {
		t.Errorf("loadFromPath RPI.TmuxCommand = %q, want %q", cfg.RPI.TmuxCommand, "tmuxx")
	}
}

func TestProjectConfigPath_UsesAgentOpsConfigEnv(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom.yaml")
	t.Setenv("AGENTOPS_CONFIG", configPath)

	got := projectConfigPath()
	if got != configPath {
		t.Fatalf("projectConfigPath() = %q, want %q", got, configPath)
	}
}

func TestProjectConfigPath_DefaultFromCwd(t *testing.T) {
	// When AGENTOPS_CONFIG is not set, should use cwd
	t.Setenv("AGENTOPS_CONFIG", "")
	got := projectConfigPath()
	cwd, _ := os.Getwd()
	expected := filepath.Join(cwd, ".agentops", "config.yaml")
	if got != expected {
		t.Errorf("projectConfigPath() = %q, want %q", got, expected)
	}
}

func TestProjectConfigPath_WhitespaceOnlyConfig(t *testing.T) {
	// Whitespace-only AGENTOPS_CONFIG should be treated as not set
	t.Setenv("AGENTOPS_CONFIG", "  \t  ")
	got := projectConfigPath()
	cwd, _ := os.Getwd()
	expected := filepath.Join(cwd, ".agentops", "config.yaml")
	if got != expected {
		t.Errorf("projectConfigPath() with whitespace = %q, want %q", got, expected)
	}
}

func TestProjectConfigPath_GetwdError(t *testing.T) {
	// When AGENTOPS_CONFIG is unset and cwd has been removed, Getwd fails
	// and projectConfigPath should return "".
	t.Setenv("AGENTOPS_CONFIG", "")

	// Save original directory so we can restore it.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get original cwd: %v", err)
	}

	// Create a standalone temp dir (not via t.TempDir which defers cleanup).
	tmp, err := os.MkdirTemp("", "test-getwd-err")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	if err := os.Chdir(tmp); err != nil {
		os.Remove(tmp)
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}
	// Remove the dir while we're inside it — makes Getwd fail on Linux.
	if err := os.Remove(tmp); err != nil {
		if err2 := os.Chdir(origDir); err2 != nil {
			t.Fatalf("chdir restore failed: %v", err2)
		}
		t.Skip("cannot remove cwd on this platform")
	}

	// On macOS, Getwd succeeds even after the directory is removed.
	// Detect that and skip — the error branch is only reachable on Linux.
	if _, getwdErr := os.Getwd(); getwdErr == nil {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("chdir restore failed: %v", err)
		}
		t.Skip("Getwd does not fail after removing cwd on this OS")
	}

	got := projectConfigPath()
	// Restore cwd before any assertions so subsequent tests aren't affected.
	if err := os.Chdir(origDir); err != nil {
		t.Fatalf("failed to restore cwd: %v", err)
	}

	if got != "" {
		t.Errorf("projectConfigPath() with removed cwd = %q, want %q", got, "")
	}
}

func TestResolve_WithProjectConfig(t *testing.T) {
	// Create a project config file and point AGENTOPS_CONFIG at it
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
output: yaml
base_dir: /project/base
verbose: true
rpi:
  worktree_mode: never
  runtime_mode: direct
  runtime_command: custom-claude
  ao_command: custom-ao
  bd_command: custom-bd
  tmux_command: custom-tmux
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Set project config path
	t.Setenv("AGENTOPS_CONFIG", configPath)
	// Clear all env overrides so project config values shine through
	for _, key := range []string{
		"AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE",
		"AGENTOPS_NO_SC",
		"AGENTOPS_RPI_WORKTREE_MODE", "AGENTOPS_RPI_RUNTIME",
		"AGENTOPS_RPI_RUNTIME_MODE", "AGENTOPS_RPI_RUNTIME_COMMAND",
		"AGENTOPS_RPI_AO_COMMAND", "AGENTOPS_RPI_BD_COMMAND",
		"AGENTOPS_RPI_TMUX_COMMAND",
		"AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD",
	} {
		t.Setenv(key, "")
	}

	rc := Resolve("", "", false)

	if rc.Output.Value != "yaml" || rc.Output.Source != SourceProject {
		t.Errorf("Output = (%v, %v), want (yaml, %v)", rc.Output.Value, rc.Output.Source, SourceProject)
	}
	if rc.BaseDir.Value != "/project/base" || rc.BaseDir.Source != SourceProject {
		t.Errorf("BaseDir = (%v, %v), want (/project/base, %v)", rc.BaseDir.Value, rc.BaseDir.Source, SourceProject)
	}
	if rc.Verbose.Value != true || rc.Verbose.Source != SourceProject {
		t.Errorf("Verbose = (%v, %v), want (true, %v)", rc.Verbose.Value, rc.Verbose.Source, SourceProject)
	}
	if rc.RPIWorktreeMode.Value != "never" || rc.RPIWorktreeMode.Source != SourceProject {
		t.Errorf("RPIWorktreeMode = (%v, %v), want (never, %v)", rc.RPIWorktreeMode.Value, rc.RPIWorktreeMode.Source, SourceProject)
	}
	if rc.RPIRuntimeMode.Value != "direct" || rc.RPIRuntimeMode.Source != SourceProject {
		t.Errorf("RPIRuntimeMode = (%v, %v), want (direct, %v)", rc.RPIRuntimeMode.Value, rc.RPIRuntimeMode.Source, SourceProject)
	}
	if rc.RPIRuntimeCommand.Value != "custom-claude" || rc.RPIRuntimeCommand.Source != SourceProject {
		t.Errorf("RPIRuntimeCommand = (%v, %v), want (custom-claude, %v)", rc.RPIRuntimeCommand.Value, rc.RPIRuntimeCommand.Source, SourceProject)
	}
	if rc.RPIAOCommand.Value != "custom-ao" || rc.RPIAOCommand.Source != SourceProject {
		t.Errorf("RPIAOCommand = (%v, %v), want (custom-ao, %v)", rc.RPIAOCommand.Value, rc.RPIAOCommand.Source, SourceProject)
	}
	if rc.RPIBDCommand.Value != "custom-bd" || rc.RPIBDCommand.Source != SourceProject {
		t.Errorf("RPIBDCommand = (%v, %v), want (custom-bd, %v)", rc.RPIBDCommand.Value, rc.RPIBDCommand.Source, SourceProject)
	}
	if rc.RPITmuxCommand.Value != "custom-tmux" || rc.RPITmuxCommand.Source != SourceProject {
		t.Errorf("RPITmuxCommand = (%v, %v), want (custom-tmux, %v)", rc.RPITmuxCommand.Value, rc.RPITmuxCommand.Source, SourceProject)
	}
}

func TestResolve_FlagOverridesProjectConfig(t *testing.T) {
	// Create a project config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
output: yaml
base_dir: /project/base
verbose: true
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AGENTOPS_CONFIG", configPath)
	for _, key := range []string{
		"AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE",
		"AGENTOPS_NO_SC",
		"AGENTOPS_RPI_WORKTREE_MODE", "AGENTOPS_RPI_RUNTIME",
		"AGENTOPS_RPI_RUNTIME_MODE", "AGENTOPS_RPI_RUNTIME_COMMAND",
		"AGENTOPS_RPI_AO_COMMAND", "AGENTOPS_RPI_BD_COMMAND",
		"AGENTOPS_RPI_TMUX_COMMAND",
		"AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD",
	} {
		t.Setenv(key, "")
	}

	// Flags should override project config
	rc := Resolve("json", "/flag/dir", true)

	if rc.Output.Value != "json" || rc.Output.Source != SourceFlag {
		t.Errorf("Flag should override project: Output = (%v, %v)", rc.Output.Value, rc.Output.Source)
	}
	if rc.BaseDir.Value != "/flag/dir" || rc.BaseDir.Source != SourceFlag {
		t.Errorf("Flag should override project: BaseDir = (%v, %v)", rc.BaseDir.Value, rc.BaseDir.Source)
	}
	if rc.Verbose.Value != true || rc.Verbose.Source != SourceFlag {
		t.Errorf("Flag should override project: Verbose = (%v, %v)", rc.Verbose.Value, rc.Verbose.Source)
	}
}

func TestResolve_EnvOverridesProjectConfig(t *testing.T) {
	// Create a project config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
output: yaml
base_dir: /project/base
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AGENTOPS_CONFIG", configPath)
	t.Setenv("AGENTOPS_OUTPUT", "csv")
	t.Setenv("AGENTOPS_BASE_DIR", "/env/dir")
	t.Setenv("AGENTOPS_VERBOSE", "true")
	// Clear other env vars
	for _, key := range []string{
		"AGENTOPS_NO_SC",
		"AGENTOPS_RPI_WORKTREE_MODE", "AGENTOPS_RPI_RUNTIME",
		"AGENTOPS_RPI_RUNTIME_MODE", "AGENTOPS_RPI_RUNTIME_COMMAND",
		"AGENTOPS_RPI_AO_COMMAND", "AGENTOPS_RPI_BD_COMMAND",
		"AGENTOPS_RPI_TMUX_COMMAND",
		"AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD",
	} {
		t.Setenv(key, "")
	}

	rc := Resolve("", "", false)

	if rc.Output.Value != "csv" || rc.Output.Source != SourceEnv {
		t.Errorf("Env should override project: Output = (%v, %v)", rc.Output.Value, rc.Output.Source)
	}
	if rc.BaseDir.Value != "/env/dir" || rc.BaseDir.Source != SourceEnv {
		t.Errorf("Env should override project: BaseDir = (%v, %v)", rc.BaseDir.Value, rc.BaseDir.Source)
	}
	if rc.Verbose.Value != true || rc.Verbose.Source != SourceEnv {
		t.Errorf("Env should override project: Verbose = (%v, %v)", rc.Verbose.Value, rc.Verbose.Source)
	}
}

func TestLoad_WithProjectConfig(t *testing.T) {
	// Create project config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
output: yaml
base_dir: /project/ao
rpi:
  worktree_mode: always
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AGENTOPS_CONFIG", configPath)
	for _, key := range []string{
		"AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE",
		"AGENTOPS_NO_SC",
		"AGENTOPS_RPI_WORKTREE_MODE", "AGENTOPS_RPI_RUNTIME",
		"AGENTOPS_RPI_RUNTIME_MODE", "AGENTOPS_RPI_RUNTIME_COMMAND",
		"AGENTOPS_RPI_AO_COMMAND", "AGENTOPS_RPI_BD_COMMAND",
		"AGENTOPS_RPI_TMUX_COMMAND",
		"AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD",
	} {
		t.Setenv(key, "")
	}

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Output != "yaml" {
		t.Errorf("Load with project config Output = %q, want %q", cfg.Output, "yaml")
	}
	if cfg.BaseDir != "/project/ao" {
		t.Errorf("Load with project config BaseDir = %q, want %q", cfg.BaseDir, "/project/ao")
	}
	if cfg.RPI.WorktreeMode != "always" {
		t.Errorf("Load with project config RPI.WorktreeMode = %q, want %q", cfg.RPI.WorktreeMode, "always")
	}
}

func TestResolve_RPIRuntimeModeOverridesRuntime(t *testing.T) {
	// When both AGENTOPS_RPI_RUNTIME and AGENTOPS_RPI_RUNTIME_MODE are set,
	// RUNTIME_MODE should take precedence
	t.Setenv("AGENTOPS_CONFIG", "")
	t.Setenv("AGENTOPS_OUTPUT", "")
	t.Setenv("AGENTOPS_BASE_DIR", "")
	t.Setenv("AGENTOPS_VERBOSE", "")
	t.Setenv("AGENTOPS_NO_SC", "")
	t.Setenv("AGENTOPS_RPI_WORKTREE_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME", "direct")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "stream")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "")
	t.Setenv("AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD", "")

	rc := Resolve("", "", false)

	// RUNTIME_MODE should override RUNTIME
	if rc.RPIRuntimeMode.Value != "stream" {
		t.Errorf("RPIRuntimeMode = %v, want stream (RUNTIME_MODE should override RUNTIME)", rc.RPIRuntimeMode.Value)
	}
}

func TestLoadFromPath_WithFlywheel(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `
flywheel:
  auto_promote_threshold: 72h
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFromPath(configPath)
	if err != nil {
		t.Fatalf("loadFromPath() error = %v", err)
	}
	if cfg.Flywheel.AutoPromoteThreshold != "72h" {
		t.Errorf("loadFromPath Flywheel.AutoPromoteThreshold = %q, want %q", cfg.Flywheel.AutoPromoteThreshold, "72h")
	}
}

func TestLoadFromPath_WithPaths(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `
output: json
paths:
  learnings_dir: /my/learnings
  patterns_dir: /my/patterns
  retros_dir: /my/retros
  research_dir: /my/research
  plans_dir: /my/plans
  claude_plans_dir: /my/claude-plans
  citations_file: /my/citations.jsonl
  transcripts_dir: /my/transcripts
forge:
  max_content_length: 10000
  progress_interval: 200
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadFromPath(configPath)
	if err != nil {
		t.Fatalf("loadFromPath() error = %v", err)
	}

	if cfg.Paths.LearningsDir != "/my/learnings" {
		t.Errorf("loadFromPath Paths.LearningsDir = %q, want %q", cfg.Paths.LearningsDir, "/my/learnings")
	}
	if cfg.Forge.MaxContentLength != 10000 {
		t.Errorf("loadFromPath Forge.MaxContentLength = %d, want 10000", cfg.Forge.MaxContentLength)
	}
	if cfg.Forge.ProgressInterval != 200 {
		t.Errorf("loadFromPath Forge.ProgressInterval = %d, want 200", cfg.Forge.ProgressInterval)
	}
}

func TestLoad_WithHomeConfig(t *testing.T) {
	// Create a temporary home config file at the actual home config path.
	homePath := homeConfigPath()
	if homePath == "" {
		t.Skip("cannot determine home config path")
	}

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(homePath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Backup any existing config
	origData, origErr := os.ReadFile(homePath)
	existed := origErr == nil

	// Write test home config
	content := `
output: markdown
base_dir: /home-base
verbose: true
rpi:
  worktree_mode: never
  runtime_mode: stream
  runtime_command: home-claude
  ao_command: home-ao
  bd_command: home-bd
  tmux_command: home-tmux
`
	if err := os.WriteFile(homePath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.WriteFile(homePath, origData, 0644)
		} else {
			_ = os.Remove(homePath)
		}
	})

	// Clear env vars and project config
	t.Setenv("AGENTOPS_CONFIG", "/nonexistent/project.yaml") // force no project config
	for _, key := range []string{
		"AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE",
		"AGENTOPS_NO_SC",
		"AGENTOPS_RPI_WORKTREE_MODE", "AGENTOPS_RPI_RUNTIME",
		"AGENTOPS_RPI_RUNTIME_MODE", "AGENTOPS_RPI_RUNTIME_COMMAND",
		"AGENTOPS_RPI_AO_COMMAND", "AGENTOPS_RPI_BD_COMMAND",
		"AGENTOPS_RPI_TMUX_COMMAND",
		"AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD",
	} {
		t.Setenv(key, "")
	}

	// Test Load picks up home config
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Output != "markdown" {
		t.Errorf("Load with home config: Output = %q, want %q", cfg.Output, "markdown")
	}
	if cfg.BaseDir != "/home-base" {
		t.Errorf("Load with home config: BaseDir = %q, want %q", cfg.BaseDir, "/home-base")
	}
	if !cfg.Verbose {
		t.Error("Load with home config: Verbose = false, want true")
	}
	if cfg.RPI.WorktreeMode != "never" {
		t.Errorf("Load with home config: RPI.WorktreeMode = %q, want %q", cfg.RPI.WorktreeMode, "never")
	}
}

func TestResolve_WithHomeConfig(t *testing.T) {
	writeTestHomeConfig(t, `
output: markdown
base_dir: /home-resolve
verbose: true
rpi:
  worktree_mode: always
  runtime_mode: direct
  runtime_command: home-runtime
  ao_command: home-ao
  bd_command: home-bd
  tmux_command: home-tmux
`)
	clearConfigResolutionEnv(t, "/nonexistent/project.yaml")

	// Test Resolve picks up home config values (covers lines 453-463 and 509-511)
	rc := Resolve("", "", false)
	assertResolvedHomeConfig(t, rc)
}

func writeTestHomeConfig(t *testing.T, content string) {
	t.Helper()

	homePath := homeConfigPath()
	if homePath == "" {
		t.Skip("cannot determine home config path")
	}

	if err := os.MkdirAll(filepath.Dir(homePath), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	origData, origErr := os.ReadFile(homePath)
	existed := origErr == nil
	if err := os.WriteFile(homePath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	t.Cleanup(func() {
		restoreTestHomeConfig(homePath, origData, existed)
	})
}

func restoreTestHomeConfig(homePath string, origData []byte, existed bool) {
	if existed {
		_ = os.WriteFile(homePath, origData, 0644)
	} else {
		_ = os.Remove(homePath)
	}
}

func clearConfigResolutionEnv(t *testing.T, projectConfigPath string) {
	t.Helper()
	t.Setenv("AGENTOPS_CONFIG", projectConfigPath)
	for _, key := range []string{
		"AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE",
		"AGENTOPS_NO_SC",
		"AGENTOPS_RPI_WORKTREE_MODE", "AGENTOPS_RPI_RUNTIME",
		"AGENTOPS_RPI_RUNTIME_MODE", "AGENTOPS_RPI_RUNTIME_COMMAND",
		"AGENTOPS_RPI_AO_COMMAND", "AGENTOPS_RPI_BD_COMMAND",
		"AGENTOPS_RPI_TMUX_COMMAND",
		"AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD",
	} {
		t.Setenv(key, "")
	}
}

func assertResolvedHomeConfig(t *testing.T, rc *ResolvedConfig) {
	t.Helper()
	assertResolvedValues(t, []resolvedFieldExpectation{
		{name: "Output", got: rc.Output, want: "markdown", source: SourceHome},
		{name: "BaseDir", got: rc.BaseDir, want: "/home-resolve", source: SourceHome},
		{name: "Verbose", got: rc.Verbose, want: true, source: SourceHome},
		{name: "RPIWorktreeMode", got: rc.RPIWorktreeMode, want: "always", source: SourceHome},
		{name: "RPIRuntimeMode", got: rc.RPIRuntimeMode, want: "direct", source: SourceHome},
		{name: "RPIRuntimeCommand", got: rc.RPIRuntimeCommand, want: "home-runtime", source: SourceHome},
		{name: "RPIAOCommand", got: rc.RPIAOCommand, want: "home-ao", source: SourceHome},
		{name: "RPIBDCommand", got: rc.RPIBDCommand, want: "home-bd", source: SourceHome},
		{name: "RPITmuxCommand", got: rc.RPITmuxCommand, want: "home-tmux", source: SourceHome},
	})
}

type resolvedFieldExpectation struct {
	name   string
	got    resolved
	want   any
	source Source
}

func assertResolvedValues(t *testing.T, fields []resolvedFieldExpectation) {
	t.Helper()
	for _, field := range fields {
		if !reflect.DeepEqual(field.got.Value, field.want) || field.got.Source != field.source {
			t.Errorf("Resolve with home config: %s = (%v, %v), want (%v, %v)",
				field.name, field.got.Value, field.got.Source, field.want, field.source)
		}
	}
}

func TestProjectConfigPath_GetwdFails(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", "")

	// Inject a failing getwdFunc to cover the error branch on all platforms.
	origGetwd := getwdFunc
	getwdFunc = func() (string, error) {
		return "", os.ErrNotExist
	}
	defer func() { getwdFunc = origGetwd }()

	got := projectConfigPath()
	if got != "" {
		t.Errorf("projectConfigPath() = %q, want empty string when getwd fails", got)
	}
}

// --- Benchmarks ---

func BenchmarkDefault(b *testing.B) {
	for range b.N {
		Default()
	}
}

func BenchmarkMerge(b *testing.B) {
	base := Default()
	overlay := &Config{
		Output:  "json",
		BaseDir: "/tmp/bench",
		Verbose: true,
		Forge:   ForgeConfig{MaxContentLength: 5000},
	}
	b.ResetTimer()
	for range b.N {
		dst := *base // copy
		merge(&dst, overlay)
	}
}

func TestDefault_GlobalPaths(t *testing.T) {
	cfg := Default()
	homeDir, _ := os.UserHomeDir()

	wantLearnings := filepath.Join(homeDir, ".agents", "learnings")
	if cfg.Paths.GlobalLearningsDir != wantLearnings {
		t.Errorf("Default Paths.GlobalLearningsDir = %q, want %q", cfg.Paths.GlobalLearningsDir, wantLearnings)
	}

	wantPatterns := filepath.Join(homeDir, ".agents", "patterns")
	if cfg.Paths.GlobalPatternsDir != wantPatterns {
		t.Errorf("Default Paths.GlobalPatternsDir = %q, want %q", cfg.Paths.GlobalPatternsDir, wantPatterns)
	}

	if cfg.Paths.GlobalWeight != 0.8 {
		t.Errorf("Default Paths.GlobalWeight = %f, want 0.8", cfg.Paths.GlobalWeight)
	}
}

func TestMerge_GlobalPaths(t *testing.T) {
	dst := Default()
	src := &Config{
		Paths: PathsConfig{
			GlobalLearningsDir: "/custom/learnings",
			GlobalPatternsDir:  "/custom/patterns",
		},
	}

	result := merge(dst, src)

	if result.Paths.GlobalLearningsDir != "/custom/learnings" {
		t.Errorf("merge GlobalLearningsDir = %q, want %q", result.Paths.GlobalLearningsDir, "/custom/learnings")
	}
	if result.Paths.GlobalPatternsDir != "/custom/patterns" {
		t.Errorf("merge GlobalPatternsDir = %q, want %q", result.Paths.GlobalPatternsDir, "/custom/patterns")
	}
}

func TestMerge_GlobalWeight(t *testing.T) {
	dst := Default()
	src := &Config{
		Paths: PathsConfig{
			GlobalWeight: 0.5,
		},
	}

	result := merge(dst, src)

	if result.Paths.GlobalWeight != 0.5 {
		t.Errorf("merge GlobalWeight = %f, want 0.5", result.Paths.GlobalWeight)
	}

	// Zero value should NOT overwrite
	dst2 := Default()
	src2 := &Config{
		Paths: PathsConfig{
			GlobalWeight: 0,
		},
	}
	result2 := merge(dst2, src2)
	if result2.Paths.GlobalWeight != 0.8 {
		t.Errorf("merge GlobalWeight with zero = %f, want 0.8 (preserved)", result2.Paths.GlobalWeight)
	}
}

func TestHomeConfigPath_UserHomeDirError(t *testing.T) {
	// When HOME is empty, os.UserHomeDir() returns an error and
	// homeConfigPath must return an empty string.
	t.Setenv("HOME", "")

	result := homeConfigPath()
	if result != "" {
		t.Errorf("homeConfigPath() = %q, want empty string when HOME is unset", result)
	}
}

func TestDefault_Models(t *testing.T) {
	cfg := Default()

	if cfg.Models.DefaultTier != "balanced" {
		t.Errorf("Default Models.DefaultTier = %q, want %q", cfg.Models.DefaultTier, "balanced")
	}

	wantTiers := map[string]TierConfig{
		"quality":  {Claude: "opus", Codex: ""},
		"balanced": {Claude: "sonnet", Codex: ""},
		"budget":   {Claude: "haiku", Codex: ""},
	}
	if len(cfg.Models.Tiers) != len(wantTiers) {
		t.Fatalf("Default Models.Tiers has %d entries, want %d", len(cfg.Models.Tiers), len(wantTiers))
	}
	for name, want := range wantTiers {
		got, ok := cfg.Models.Tiers[name]
		if !ok {
			t.Errorf("Default Models.Tiers missing tier %q", name)
			continue
		}
		if got.Claude != want.Claude {
			t.Errorf("Default Models.Tiers[%q].Claude = %q, want %q", name, got.Claude, want.Claude)
		}
		if got.Codex != want.Codex {
			t.Errorf("Default Models.Tiers[%q].Codex = %q, want %q", name, got.Codex, want.Codex)
		}
	}

	if cfg.Models.SkillOverrides == nil {
		t.Error("Default Models.SkillOverrides is nil, want empty map")
	}
	if len(cfg.Models.SkillOverrides) != 0 {
		t.Errorf("Default Models.SkillOverrides has %d entries, want 0", len(cfg.Models.SkillOverrides))
	}
}

func TestMergeModels_PartialOverrides(t *testing.T) {
	dst := Default()
	src := &Config{
		Models: ModelsConfig{
			DefaultTier: "quality",
			Tiers: map[string]TierConfig{
				"quality": {Claude: "opus-4", Codex: "codex-1"},
			},
			SkillOverrides: map[string]string{
				"council": "budget",
			},
		},
	}

	result := merge(dst, src)

	if result.Models.DefaultTier != "quality" {
		t.Errorf("merge Models.DefaultTier = %q, want %q", result.Models.DefaultTier, "quality")
	}
	// quality tier should be overridden
	if result.Models.Tiers["quality"].Claude != "opus-4" {
		t.Errorf("merge Models.Tiers[quality].Claude = %q, want %q", result.Models.Tiers["quality"].Claude, "opus-4")
	}
	if result.Models.Tiers["quality"].Codex != "codex-1" {
		t.Errorf("merge Models.Tiers[quality].Codex = %q, want %q", result.Models.Tiers["quality"].Codex, "codex-1")
	}
	// balanced tier should be preserved from defaults
	if result.Models.Tiers["balanced"].Claude != "sonnet" {
		t.Errorf("merge Models.Tiers[balanced].Claude = %q, want %q (preserved)", result.Models.Tiers["balanced"].Claude, "sonnet")
	}
	// budget tier should be preserved from defaults
	if result.Models.Tiers["budget"].Claude != "haiku" {
		t.Errorf("merge Models.Tiers[budget].Claude = %q, want %q (preserved)", result.Models.Tiers["budget"].Claude, "haiku")
	}
	// skill override should be set
	if result.Models.SkillOverrides["council"] != "budget" {
		t.Errorf("merge Models.SkillOverrides[council] = %q, want %q", result.Models.SkillOverrides["council"], "budget")
	}
}

func TestMergeModels_PreservedWhenEmpty(t *testing.T) {
	dst := Default()
	src := &Config{
		Output: "json",
		// Models fields are zero values
	}

	result := merge(dst, src)

	if result.Models.DefaultTier != "balanced" {
		t.Errorf("merge should preserve default Models.DefaultTier, got %q", result.Models.DefaultTier)
	}
	if len(result.Models.Tiers) != 3 {
		t.Errorf("merge should preserve default Models.Tiers, got %d entries", len(result.Models.Tiers))
	}
}

func TestApplyEnv_ModelTier(t *testing.T) {
	t.Setenv("AGENTOPS_OUTPUT", "")
	t.Setenv("AGENTOPS_BASE_DIR", "")
	t.Setenv("AGENTOPS_VERBOSE", "")
	t.Setenv("AGENTOPS_NO_SC", "")
	t.Setenv("AGENTOPS_RPI_WORKTREE_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "")
	t.Setenv("AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD", "")
	t.Setenv("AGENTOPS_MODEL_TIER", "quality")
	t.Setenv("AGENTOPS_COUNCIL_MODEL_TIER", "")

	cfg := Default()
	cfg = applyEnv(cfg)

	if cfg.Models.DefaultTier != "quality" {
		t.Errorf("applyEnv Models.DefaultTier = %q, want %q", cfg.Models.DefaultTier, "quality")
	}
}

func TestApplyEnv_CouncilModelTier(t *testing.T) {
	t.Setenv("AGENTOPS_OUTPUT", "")
	t.Setenv("AGENTOPS_BASE_DIR", "")
	t.Setenv("AGENTOPS_VERBOSE", "")
	t.Setenv("AGENTOPS_NO_SC", "")
	t.Setenv("AGENTOPS_RPI_WORKTREE_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_MODE", "")
	t.Setenv("AGENTOPS_RPI_RUNTIME_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_AO_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_BD_COMMAND", "")
	t.Setenv("AGENTOPS_RPI_TMUX_COMMAND", "")
	t.Setenv("AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD", "")
	t.Setenv("AGENTOPS_MODEL_TIER", "")
	t.Setenv("AGENTOPS_COUNCIL_MODEL_TIER", "quality")

	cfg := Default()
	cfg = applyEnv(cfg)

	if cfg.Models.SkillOverrides["council"] != "quality" {
		t.Errorf("applyEnv Models.SkillOverrides[council] = %q, want %q", cfg.Models.SkillOverrides["council"], "quality")
	}
	// DefaultTier should remain unchanged
	if cfg.Models.DefaultTier != "balanced" {
		t.Errorf("applyEnv Models.DefaultTier = %q, want %q (unchanged)", cfg.Models.DefaultTier, "balanced")
	}
}

func TestResolveTier(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		skillName string
		wantTier  string
	}{
		{
			name: "skill override takes precedence",
			cfg: Config{
				Models: ModelsConfig{
					DefaultTier:    "balanced",
					SkillOverrides: map[string]string{"council": "quality"},
				},
			},
			skillName: "council",
			wantTier:  "quality",
		},
		{
			name: "falls back to default tier",
			cfg: Config{
				Models: ModelsConfig{
					DefaultTier:    "budget",
					SkillOverrides: map[string]string{},
				},
			},
			skillName: "vibe",
			wantTier:  "budget",
		},
		{
			name: "inherit in override uses DefaultTier",
			cfg: Config{
				Models: ModelsConfig{
					DefaultTier:    "quality",
					SkillOverrides: map[string]string{"council": "inherit"},
				},
			},
			skillName: "council",
			wantTier:  "quality",
		},
		{
			name: "inherit as default tier falls back to balanced",
			cfg: Config{
				Models: ModelsConfig{
					DefaultTier:    "inherit",
					SkillOverrides: map[string]string{},
				},
			},
			skillName: "vibe",
			wantTier:  "balanced",
		},
		{
			name: "empty default tier falls back to balanced",
			cfg: Config{
				Models: ModelsConfig{
					DefaultTier:    "",
					SkillOverrides: map[string]string{},
				},
			},
			skillName: "vibe",
			wantTier:  "balanced",
		},
		{
			name: "nil skill overrides falls back to default",
			cfg: Config{
				Models: ModelsConfig{
					DefaultTier:    "quality",
					SkillOverrides: nil,
				},
			},
			skillName: "council",
			wantTier:  "quality",
		},
		{
			name: "unknown tier name in default falls back to balanced",
			cfg: Config{
				Models: ModelsConfig{
					DefaultTier:    "premium",
					SkillOverrides: map[string]string{},
				},
			},
			skillName: "vibe",
			wantTier:  "balanced",
		},
		{
			name: "unknown tier name in skill override falls back to balanced",
			cfg: Config{
				Models: ModelsConfig{
					DefaultTier:    "quality",
					SkillOverrides: map[string]string{"council": "ultra"},
				},
			},
			skillName: "council",
			wantTier:  "balanced",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ResolveTier(tt.skillName)
			if got != tt.wantTier {
				t.Errorf("ResolveTier(%q) = %q, want %q", tt.skillName, got, tt.wantTier)
			}
		})
	}
}

func TestResolve_ModelsDefaultTier(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", "")
	// Clear all env vars
	for _, key := range []string{
		"AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE",
		"AGENTOPS_NO_SC",
		"AGENTOPS_RPI_WORKTREE_MODE", "AGENTOPS_RPI_RUNTIME",
		"AGENTOPS_RPI_RUNTIME_MODE", "AGENTOPS_RPI_RUNTIME_COMMAND",
		"AGENTOPS_RPI_AO_COMMAND", "AGENTOPS_RPI_BD_COMMAND",
		"AGENTOPS_RPI_TMUX_COMMAND",
		"AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD",
		"AGENTOPS_MODEL_TIER", "AGENTOPS_COUNCIL_MODEL_TIER",
	} {
		t.Setenv(key, "")
	}

	// Default should be "balanced"
	rc := Resolve("", "", false)
	if rc.ModelsDefaultTier.Value != "balanced" || rc.ModelsDefaultTier.Source != SourceDefault {
		t.Errorf("Resolve ModelsDefaultTier = (%v, %v), want (balanced, %v)",
			rc.ModelsDefaultTier.Value, rc.ModelsDefaultTier.Source, SourceDefault)
	}
}

func TestResolve_ModelsDefaultTier_EnvOverride(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", "")
	for _, key := range []string{
		"AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE",
		"AGENTOPS_NO_SC",
		"AGENTOPS_RPI_WORKTREE_MODE", "AGENTOPS_RPI_RUNTIME",
		"AGENTOPS_RPI_RUNTIME_MODE", "AGENTOPS_RPI_RUNTIME_COMMAND",
		"AGENTOPS_RPI_AO_COMMAND", "AGENTOPS_RPI_BD_COMMAND",
		"AGENTOPS_RPI_TMUX_COMMAND",
		"AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD",
		"AGENTOPS_COUNCIL_MODEL_TIER",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("AGENTOPS_MODEL_TIER", "quality")

	rc := Resolve("", "", false)
	if rc.ModelsDefaultTier.Value != "quality" || rc.ModelsDefaultTier.Source != SourceEnv {
		t.Errorf("Resolve env ModelsDefaultTier = (%v, %v), want (quality, %v)",
			rc.ModelsDefaultTier.Value, rc.ModelsDefaultTier.Source, SourceEnv)
	}
}

func TestResolve_DreamDefaults(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", "")
	for _, key := range []string{
		"AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE",
		"AGENTOPS_NO_SC",
		"AGENTOPS_RPI_WORKTREE_MODE", "AGENTOPS_RPI_RUNTIME",
		"AGENTOPS_RPI_RUNTIME_MODE", "AGENTOPS_RPI_RUNTIME_COMMAND",
		"AGENTOPS_RPI_AO_COMMAND", "AGENTOPS_RPI_BD_COMMAND",
		"AGENTOPS_RPI_TMUX_COMMAND",
		"AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD",
		"AGENTOPS_MODEL_TIER", "AGENTOPS_COUNCIL_MODEL_TIER",
		"AGENTOPS_DREAM_REPORT_DIR", "AGENTOPS_DREAM_RUN_TIMEOUT", "AGENTOPS_DREAM_KEEP_AWAKE",
	} {
		t.Setenv(key, "")
	}

	rc := Resolve("", "", false)
	if rc.DreamReportDir.Value != ".agents/overnight/latest" || rc.DreamReportDir.Source != SourceDefault {
		t.Fatalf("DreamReportDir = (%v, %v), want (.agents/overnight/latest, %v)", rc.DreamReportDir.Value, rc.DreamReportDir.Source, SourceDefault)
	}
	if rc.DreamRunTimeout.Value != "8h" || rc.DreamRunTimeout.Source != SourceDefault {
		t.Fatalf("DreamRunTimeout = (%v, %v), want (8h, %v)", rc.DreamRunTimeout.Value, rc.DreamRunTimeout.Source, SourceDefault)
	}
	if rc.DreamKeepAwake.Value != true || rc.DreamKeepAwake.Source != SourceDefault {
		t.Fatalf("DreamKeepAwake = (%v, %v), want (true, %v)", rc.DreamKeepAwake.Value, rc.DreamKeepAwake.Source, SourceDefault)
	}
	if rc.DreamScheduler.Value != "manual" || rc.DreamScheduler.Source != SourceDefault {
		t.Fatalf("DreamScheduler = (%v, %v), want (manual, %v)", rc.DreamScheduler.Value, rc.DreamScheduler.Source, SourceDefault)
	}
	if rc.DreamConsensus.Value != "majority" || rc.DreamConsensus.Source != SourceDefault {
		t.Fatalf("DreamConsensus = (%v, %v), want (majority, %v)", rc.DreamConsensus.Value, rc.DreamConsensus.Source, SourceDefault)
	}
	if rc.DreamCreativeLane.Value != false || rc.DreamCreativeLane.Source != SourceDefault {
		t.Fatalf("DreamCreativeLane = (%v, %v), want (false, %v)", rc.DreamCreativeLane.Value, rc.DreamCreativeLane.Source, SourceDefault)
	}
	if got, ok := rc.DreamRunners.Value.([]string); !ok || len(got) != 0 || rc.DreamRunners.Source != SourceDefault {
		t.Fatalf("DreamRunners = (%#v, %v), want (empty, %v)", rc.DreamRunners.Value, rc.DreamRunners.Source, SourceDefault)
	}
}

func TestApplyEnv_DreamOverrides(t *testing.T) {
	cfg := Default()
	t.Setenv("AGENTOPS_DREAM_REPORT_DIR", "/tmp/dream")
	t.Setenv("AGENTOPS_DREAM_RUN_TIMEOUT", "10h")
	t.Setenv("AGENTOPS_DREAM_KEEP_AWAKE", "false")
	t.Setenv("AGENTOPS_DREAM_CURATOR_ENABLED", "true")
	t.Setenv("AGENTOPS_DREAM_CURATOR_ENGINE", "ollama")
	t.Setenv("AGENTOPS_DREAM_CURATOR_OLLAMA_URL", "http://127.0.0.1:11435")
	t.Setenv("AGENTOPS_DREAM_CURATOR_MODEL", "gemma4:e4b")
	t.Setenv("AGENTOPS_DREAM_CURATOR_WORKER_DIR", "D:\\dream")
	t.Setenv("AGENTOPS_DREAM_CURATOR_VAULT_DIR", "D:\\vault")

	got := applyEnv(cfg)
	if got.Dream.ReportDir != "/tmp/dream" {
		t.Fatalf("Dream.ReportDir = %q, want /tmp/dream", got.Dream.ReportDir)
	}
	if got.Dream.RunTimeout != "10h" {
		t.Fatalf("Dream.RunTimeout = %q, want 10h", got.Dream.RunTimeout)
	}
	if got.Dream.KeepAwake == nil || *got.Dream.KeepAwake != false {
		t.Fatalf("Dream.KeepAwake = %#v, want false", got.Dream.KeepAwake)
	}
	if got.Dream.LocalCurator.Enabled == nil || *got.Dream.LocalCurator.Enabled != true {
		t.Fatalf("Dream.LocalCurator.Enabled = %#v, want true", got.Dream.LocalCurator.Enabled)
	}
	if got.Dream.LocalCurator.Engine != "ollama" {
		t.Fatalf("Dream.LocalCurator.Engine = %q, want ollama", got.Dream.LocalCurator.Engine)
	}
	if got.Dream.LocalCurator.OllamaURL != "http://127.0.0.1:11435" {
		t.Fatalf("Dream.LocalCurator.OllamaURL = %q, want http://127.0.0.1:11435", got.Dream.LocalCurator.OllamaURL)
	}
	if got.Dream.LocalCurator.Model != "gemma4:e4b" {
		t.Fatalf("Dream.LocalCurator.Model = %q, want gemma4:e4b", got.Dream.LocalCurator.Model)
	}
	if got.Dream.LocalCurator.WorkerDir != "D:\\dream" {
		t.Fatalf("Dream.LocalCurator.WorkerDir = %q, want D:\\dream", got.Dream.LocalCurator.WorkerDir)
	}
	if got.Dream.LocalCurator.VaultDir != "D:\\vault" {
		t.Fatalf("Dream.LocalCurator.VaultDir = %q, want D:\\vault", got.Dream.LocalCurator.VaultDir)
	}
}

func TestResolve_DreamLocalCuratorEnv(t *testing.T) {
	t.Setenv("AGENTOPS_CONFIG", "")
	t.Setenv("AGENTOPS_DREAM_CURATOR_ENABLED", "true")
	t.Setenv("AGENTOPS_DREAM_CURATOR_ENGINE", "ollama")
	t.Setenv("AGENTOPS_DREAM_CURATOR_OLLAMA_URL", "http://127.0.0.1:11435")
	t.Setenv("AGENTOPS_DREAM_CURATOR_MODEL", "gemma4:e4b")
	t.Setenv("AGENTOPS_DREAM_CURATOR_WORKER_DIR", "D:\\dream")
	t.Setenv("AGENTOPS_DREAM_CURATOR_VAULT_DIR", "D:\\vault")

	rc := Resolve("", "", false)
	if rc.DreamCuratorEnabled.Value != true || rc.DreamCuratorEnabled.Source != SourceEnv {
		t.Fatalf("DreamCuratorEnabled = (%v, %v), want (true, %v)", rc.DreamCuratorEnabled.Value, rc.DreamCuratorEnabled.Source, SourceEnv)
	}
	if rc.DreamCuratorEngine.Value != "ollama" || rc.DreamCuratorEngine.Source != SourceEnv {
		t.Fatalf("DreamCuratorEngine = (%v, %v), want (ollama, %v)", rc.DreamCuratorEngine.Value, rc.DreamCuratorEngine.Source, SourceEnv)
	}
	if rc.DreamCuratorOllamaURL.Value != "http://127.0.0.1:11435" || rc.DreamCuratorOllamaURL.Source != SourceEnv {
		t.Fatalf("DreamCuratorOllamaURL = (%v, %v), want env URL", rc.DreamCuratorOllamaURL.Value, rc.DreamCuratorOllamaURL.Source)
	}
	if rc.DreamCuratorModel.Value != "gemma4:e4b" || rc.DreamCuratorModel.Source != SourceEnv {
		t.Fatalf("DreamCuratorModel = (%v, %v), want env model", rc.DreamCuratorModel.Value, rc.DreamCuratorModel.Source)
	}
	if rc.DreamCuratorWorkerDir.Value != "D:\\dream" || rc.DreamCuratorWorkerDir.Source != SourceEnv {
		t.Fatalf("DreamCuratorWorkerDir = (%v, %v), want env worker", rc.DreamCuratorWorkerDir.Value, rc.DreamCuratorWorkerDir.Source)
	}
	if rc.DreamCuratorVaultDir.Value != "D:\\vault" || rc.DreamCuratorVaultDir.Source != SourceEnv {
		t.Fatalf("DreamCuratorVaultDir = (%v, %v), want env vault", rc.DreamCuratorVaultDir.Value, rc.DreamCuratorVaultDir.Source)
	}
}

func TestMergeDream_ExtendedFields(t *testing.T) {
	dst := Default()
	src := &Config{
		Dream: DreamConfig{
			Runners:         []string{"codex", "claude"},
			SchedulerMode:   "launchd",
			ScheduleAt:      "01:30",
			ConsensusPolicy: "majority",
			CreativeLane:    boolPtr(true),
			LocalCurator: DreamLocalCuratorConfig{
				Enabled:         boolPtr(true),
				Engine:          "ollama",
				OllamaURL:       "http://127.0.0.1:11435",
				Model:           "gemma4:e4b",
				WorkerDir:       "D:\\dream",
				VaultDir:        "D:\\vault",
				HourlyCap:       20,
				AllowedJobKinds: []string{"ingest-claude-session", "lint-wiki", "dream-seed"},
			},
		},
	}

	got := merge(dst, src)
	if want := []string{"codex", "claude"}; len(got.Dream.Runners) != len(want) {
		t.Fatalf("Dream.Runners = %#v, want %#v", got.Dream.Runners, want)
	}
	if got.Dream.SchedulerMode != "launchd" {
		t.Fatalf("Dream.SchedulerMode = %q, want launchd", got.Dream.SchedulerMode)
	}
	if got.Dream.ScheduleAt != "01:30" {
		t.Fatalf("Dream.ScheduleAt = %q, want 01:30", got.Dream.ScheduleAt)
	}
	if got.Dream.ConsensusPolicy != "majority" {
		t.Fatalf("Dream.ConsensusPolicy = %q, want majority", got.Dream.ConsensusPolicy)
	}
	if got.Dream.CreativeLane == nil || !*got.Dream.CreativeLane {
		t.Fatalf("Dream.CreativeLane = %#v, want true", got.Dream.CreativeLane)
	}
	if got.Dream.LocalCurator.Enabled == nil || !*got.Dream.LocalCurator.Enabled {
		t.Fatalf("Dream.LocalCurator.Enabled = %#v, want true", got.Dream.LocalCurator.Enabled)
	}
	if got.Dream.LocalCurator.Model != "gemma4:e4b" {
		t.Fatalf("Dream.LocalCurator.Model = %q, want gemma4:e4b", got.Dream.LocalCurator.Model)
	}
	if got.Dream.LocalCurator.HourlyCap != 20 {
		t.Fatalf("Dream.LocalCurator.HourlyCap = %d, want 20", got.Dream.LocalCurator.HourlyCap)
	}
	if got := strings.Join(got.Dream.LocalCurator.AllowedJobKinds, ","); got != "ingest-claude-session,lint-wiki,dream-seed" {
		t.Fatalf("Dream.LocalCurator.AllowedJobKinds = %q", got)
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".agentops")
	configPath := filepath.Join(configDir, "config.yaml")
	t.Setenv("AGENTOPS_CONFIG", configPath)

	cfg := &Config{
		Models: ModelsConfig{DefaultTier: "quality"},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Directory should have been created
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Fatal("Save() did not create .agentops/ directory")
	}

	// File should exist and be valid YAML
	loaded, err := loadFromPath(configPath)
	if err != nil {
		t.Fatalf("loadFromPath after Save: %v", err)
	}
	if loaded.Models.DefaultTier != "quality" {
		t.Errorf("saved Models.DefaultTier = %q, want %q", loaded.Models.DefaultTier, "quality")
	}
}

func TestSave_MergesWithExisting(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".agentops")
	configPath := filepath.Join(configDir, "config.yaml")
	t.Setenv("AGENTOPS_CONFIG", configPath)

	// Write an initial config with output set
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	initial := "output: json\nmodels:\n  default_tier: balanced\n"
	if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	// Save with only DefaultTier changed — output should be preserved
	cfg := &Config{
		Models: ModelsConfig{DefaultTier: "quality"},
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := loadFromPath(configPath)
	if err != nil {
		t.Fatalf("loadFromPath after Save: %v", err)
	}
	if loaded.Output != "json" {
		t.Errorf("existing Output field not preserved: got %q, want %q", loaded.Output, "json")
	}
	if loaded.Models.DefaultTier != "quality" {
		t.Errorf("saved Models.DefaultTier = %q, want %q", loaded.Models.DefaultTier, "quality")
	}
}

func TestSave_WritesYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".agentops", "config.yaml")
	t.Setenv("AGENTOPS_CONFIG", configPath)

	cfg := &Config{
		Models: ModelsConfig{
			DefaultTier:    "budget",
			SkillOverrides: map[string]string{"council": "quality"},
		},
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile after Save: %v", err)
	}

	// Verify it's valid YAML by unmarshaling
	var parsed Config
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("saved file is not valid YAML: %v", err)
	}
	if parsed.Models.DefaultTier != "budget" {
		t.Errorf("parsed Models.DefaultTier = %q, want %q", parsed.Models.DefaultTier, "budget")
	}
	if parsed.Models.SkillOverrides["council"] != "quality" {
		t.Errorf("parsed Models.SkillOverrides[council] = %q, want %q", parsed.Models.SkillOverrides["council"], "quality")
	}
}

func TestTierResolution_Precedence(t *testing.T) {
	// Test that env > project > home > default for model tier
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := `
models:
  default_tier: budget
  skill_overrides:
    council: quality
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("AGENTOPS_CONFIG", configPath)
	for _, key := range []string{
		"AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE",
		"AGENTOPS_NO_SC",
		"AGENTOPS_RPI_WORKTREE_MODE", "AGENTOPS_RPI_RUNTIME",
		"AGENTOPS_RPI_RUNTIME_MODE", "AGENTOPS_RPI_RUNTIME_COMMAND",
		"AGENTOPS_RPI_AO_COMMAND", "AGENTOPS_RPI_BD_COMMAND",
		"AGENTOPS_RPI_TMUX_COMMAND",
		"AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD",
		"AGENTOPS_COUNCIL_MODEL_TIER",
	} {
		t.Setenv(key, "")
	}

	// Without env override, project config wins
	t.Setenv("AGENTOPS_MODEL_TIER", "")
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Models.DefaultTier != "budget" {
		t.Errorf("project config Models.DefaultTier = %q, want %q", cfg.Models.DefaultTier, "budget")
	}
	if cfg.Models.SkillOverrides["council"] != "quality" {
		t.Errorf("project config Models.SkillOverrides[council] = %q, want %q", cfg.Models.SkillOverrides["council"], "quality")
	}

	// With env override, env wins
	t.Setenv("AGENTOPS_MODEL_TIER", "quality")
	cfg2, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg2.Models.DefaultTier != "quality" {
		t.Errorf("env override Models.DefaultTier = %q, want %q", cfg2.Models.DefaultTier, "quality")
	}

	// Council env override
	t.Setenv("AGENTOPS_COUNCIL_MODEL_TIER", "budget")
	cfg3, err := Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg3.Models.SkillOverrides["council"] != "budget" {
		t.Errorf("env override Models.SkillOverrides[council] = %q, want %q", cfg3.Models.SkillOverrides["council"], "budget")
	}
}

func TestDefault_HarvestRoots(t *testing.T) {
	cfg := Default()
	home, _ := os.UserHomeDir()
	wantRoot := filepath.Join(home, "gt")
	if len(cfg.Paths.HarvestRoots) != 1 {
		t.Fatalf("Default Paths.HarvestRoots length = %d, want 1", len(cfg.Paths.HarvestRoots))
	}
	if cfg.Paths.HarvestRoots[0] != wantRoot {
		t.Errorf("Default Paths.HarvestRoots[0] = %q, want %q", cfg.Paths.HarvestRoots[0], wantRoot)
	}
}

func TestMerge_HarvestRoots(t *testing.T) {
	base := Default()
	override := &Config{
		Paths: PathsConfig{
			HarvestRoots: []string{"/custom/root1", "/custom/root2"},
		},
	}
	result := merge(base, override)
	if len(result.Paths.HarvestRoots) != 2 {
		t.Fatalf("merge HarvestRoots length = %d, want 2", len(result.Paths.HarvestRoots))
	}
	if result.Paths.HarvestRoots[0] != "/custom/root1" {
		t.Errorf("merge HarvestRoots[0] = %q, want %q", result.Paths.HarvestRoots[0], "/custom/root1")
	}
}
