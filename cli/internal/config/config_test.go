package config

import (
	"os"
	"path/filepath"
	"testing"
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
