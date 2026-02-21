// Package config provides configuration management for AgentOps.
// Configuration is loaded from (highest to lowest priority):
// 1. Command-line flags
// 2. Environment variables (AGENTOPS_*)
// 3. Project config (.agentops/config.yaml in cwd)
// 4. Home config (~/.agentops/config.yaml)
// 5. Defaults
package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all AgentOps configuration.
type Config struct {
	// Output controls the default output format (table, json, yaml).
	Output string `yaml:"output" json:"output"`

	// BaseDir is the AgentOps data directory (default: .agents/ao).
	BaseDir string `yaml:"base_dir" json:"base_dir"`

	// Verbose enables verbose output.
	Verbose bool `yaml:"verbose" json:"verbose"`

	// Forge settings
	Forge ForgeConfig `yaml:"forge" json:"forge"`

	// Search settings
	Search SearchConfig `yaml:"search" json:"search"`

	// Paths settings for artifact locations (configurable, not hardcoded)
	Paths PathsConfig `yaml:"paths" json:"paths"`

	// RPI settings
	RPI RPIConfig `yaml:"rpi" json:"rpi"`

	// Flywheel settings
	Flywheel FlywheelConfig `yaml:"flywheel" json:"flywheel"`
}

// RPIConfig holds RPI-specific settings.
type RPIConfig struct {
	// WorktreeMode controls worktree behavior for phased runs.
	// Values: "auto" (default, creates worktree), "always" (force worktree), "never" (no worktree).
	WorktreeMode string `yaml:"worktree_mode" json:"worktree_mode"`
	// RuntimeMode controls phased executor selection.
	// Values: "auto" (default), "direct", "stream".
	RuntimeMode string `yaml:"runtime_mode" json:"runtime_mode"`
	// RuntimeCommand is the CLI command used to spawn phase sessions.
	// Default: "claude".
	RuntimeCommand string `yaml:"runtime_command" json:"runtime_command"`
	// AOCommand is the CLI command used for ao subcommands in orchestration.
	// Default: "ao".
	AOCommand string `yaml:"ao_command" json:"ao_command"`
	// BDCommand is the CLI command used for beads operations in orchestration.
	// Default: "bd".
	BDCommand string `yaml:"bd_command" json:"bd_command"`
	// TmuxCommand is the CLI command used for tmux liveness probes.
	// Default: "tmux".
	TmuxCommand string `yaml:"tmux_command" json:"tmux_command"`
}

// FlywheelConfig holds flywheel-specific settings.
type FlywheelConfig struct {
	// AutoPromoteThreshold controls default age gate for auto-promotion.
	// Default: 24h
	AutoPromoteThreshold string `yaml:"auto_promote_threshold" json:"auto_promote_threshold"`
}

// PathsConfig holds configurable paths for artifact locations.
// Fixes G5: paths are now configurable, not hardcoded.
type PathsConfig struct {
	// LearningsDir is where learning artifacts are stored.
	// Default: .agents/learnings
	LearningsDir string `yaml:"learnings_dir" json:"learnings_dir"`

	// PatternsDir is where pattern artifacts are stored.
	// Default: .agents/patterns
	PatternsDir string `yaml:"patterns_dir" json:"patterns_dir"`

	// RetrosDir is where retrospective artifacts are stored.
	// Default: .agents/retros
	RetrosDir string `yaml:"retros_dir" json:"retros_dir"`

	// ResearchDir is where research artifacts are stored.
	// Default: .agents/research
	ResearchDir string `yaml:"research_dir" json:"research_dir"`

	// PlansDir is where plan manifest is stored.
	// Default: .agents/plans
	PlansDir string `yaml:"plans_dir" json:"plans_dir"`

	// ClaudePlansDir is where Claude's native plans go.
	// Default: ~/.claude/plans
	ClaudePlansDir string `yaml:"claude_plans_dir" json:"claude_plans_dir"`

	// CitationsFile is where citation events are stored.
	// Default: .agents/ao/citations.jsonl
	CitationsFile string `yaml:"citations_file" json:"citations_file"`

	// TranscriptsDir is where Claude transcripts are located.
	// Default: ~/.claude/projects
	TranscriptsDir string `yaml:"transcripts_dir" json:"transcripts_dir"`
}

// ForgeConfig holds forge-specific settings.
type ForgeConfig struct {
	// MaxContentLength is the truncation limit (0 = no truncation).
	MaxContentLength int `yaml:"max_content_length" json:"max_content_length"`

	// ProgressInterval is how often to show progress (in lines).
	ProgressInterval int `yaml:"progress_interval" json:"progress_interval"`
}

// SearchConfig holds search-specific settings.
type SearchConfig struct {
	// DefaultLimit is the default number of results.
	DefaultLimit int `yaml:"default_limit" json:"default_limit"`

	// UseSmartConnections enables Smart Connections when available.
	UseSmartConnections bool `yaml:"use_smart_connections" json:"use_smart_connections"`

	// UseSmartConnectionsSet tracks whether UseSmartConnections was explicitly set.
	// This allows distinguishing between "not set" and "explicitly set to false".
	UseSmartConnectionsSet bool `yaml:"-" json:"-"`
}

// Default config values (used in resolution and validation).
const (
	defaultOutput  = "table"
	defaultBaseDir = ".agents/ao"
)

// Default returns the default configuration.
func Default() *Config {
	homeDir, _ := os.UserHomeDir()
	return &Config{
		Output:  defaultOutput,
		BaseDir: defaultBaseDir,
		Verbose: false,
		Forge: ForgeConfig{
			MaxContentLength: 0,
			ProgressInterval: 1000,
		},
		Search: SearchConfig{
			DefaultLimit:        10,
			UseSmartConnections: true,
		},
		RPI: RPIConfig{
			WorktreeMode:   "auto",
			RuntimeMode:    "auto",
			RuntimeCommand: "claude",
			AOCommand:      "ao",
			BDCommand:      "bd",
			TmuxCommand:    "tmux",
		},
		Flywheel: FlywheelConfig{
			AutoPromoteThreshold: "24h",
		},
		Paths: PathsConfig{
			LearningsDir:   ".agents/learnings",
			PatternsDir:    ".agents/patterns",
			RetrosDir:      ".agents/retros",
			ResearchDir:    ".agents/research",
			PlansDir:       ".agents/plans",
			ClaudePlansDir: filepath.Join(homeDir, ".claude", "plans"),
			CitationsFile:  ".agents/ao/citations.jsonl",
			TranscriptsDir: filepath.Join(homeDir, ".claude", "projects"),
		},
	}
}

// Load loads configuration with proper precedence.
// Priority: flags > env > project > home > defaults
func Load(flagOverrides *Config) (*Config, error) {
	cfg := Default()

	// Load home config
	homeConfig, _ := loadFromPath(homeConfigPath())
	if homeConfig != nil {
		cfg = merge(cfg, homeConfig)
	}

	// Load project config
	projectConfig, _ := loadFromPath(projectConfigPath())
	if projectConfig != nil {
		cfg = merge(cfg, projectConfig)
	}

	// Apply environment variables
	cfg = applyEnv(cfg)

	// Apply flag overrides
	if flagOverrides != nil {
		cfg = merge(cfg, flagOverrides)
	}

	return cfg, nil
}

// homeConfigPath returns the home config path.
func homeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".agentops", "config.yaml")
}

// projectConfigPath returns the project config path.
func projectConfigPath() string {
	if override := strings.TrimSpace(os.Getenv("AGENTOPS_CONFIG")); override != "" {
		return override
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(cwd, ".agentops", "config.yaml")
}

// loadFromPath loads config from a YAML file.
func loadFromPath(path string) (*Config, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// applyEnv applies environment variable overrides.
func applyEnv(cfg *Config) *Config {
	if v := os.Getenv("AGENTOPS_OUTPUT"); v != "" {
		cfg.Output = v
	}
	if v := os.Getenv("AGENTOPS_BASE_DIR"); v != "" {
		cfg.BaseDir = v
	}
	if os.Getenv("AGENTOPS_VERBOSE") == "true" || os.Getenv("AGENTOPS_VERBOSE") == "1" {
		cfg.Verbose = true
	}
	if v := os.Getenv("AGENTOPS_NO_SC"); v == "true" || v == "1" {
		cfg.Search.UseSmartConnections = false
		cfg.Search.UseSmartConnectionsSet = true
	}
	if v := os.Getenv("AGENTOPS_RPI_WORKTREE_MODE"); v != "" {
		cfg.RPI.WorktreeMode = v
	}
	if v := os.Getenv("AGENTOPS_RPI_RUNTIME"); v != "" {
		cfg.RPI.RuntimeMode = v
	}
	if v := os.Getenv("AGENTOPS_RPI_RUNTIME_MODE"); v != "" {
		cfg.RPI.RuntimeMode = v
	}
	if v := os.Getenv("AGENTOPS_RPI_RUNTIME_COMMAND"); v != "" {
		cfg.RPI.RuntimeCommand = v
	}
	if v := os.Getenv("AGENTOPS_RPI_AO_COMMAND"); v != "" {
		cfg.RPI.AOCommand = v
	}
	if v := os.Getenv("AGENTOPS_RPI_BD_COMMAND"); v != "" {
		cfg.RPI.BDCommand = v
	}
	if v := os.Getenv("AGENTOPS_RPI_TMUX_COMMAND"); v != "" {
		cfg.RPI.TmuxCommand = v
	}
	if v := os.Getenv("AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD"); v != "" {
		cfg.Flywheel.AutoPromoteThreshold = v
	}
	return cfg
}

// merge merges src into dst, with src values taking precedence.
// For booleans, we need explicit tracking via pointer or separate "set" flag.
func merge(dst, src *Config) *Config {
	if src.Output != "" {
		dst.Output = src.Output
	}
	if src.BaseDir != "" {
		dst.BaseDir = src.BaseDir
	}
	if src.Verbose {
		dst.Verbose = true
	}
	if src.Forge.MaxContentLength != 0 {
		dst.Forge.MaxContentLength = src.Forge.MaxContentLength
	}
	if src.Forge.ProgressInterval != 0 {
		dst.Forge.ProgressInterval = src.Forge.ProgressInterval
	}
	if src.Search.DefaultLimit != 0 {
		dst.Search.DefaultLimit = src.Search.DefaultLimit
	}
	// UseSmartConnections: src.UseSmartConnectionsSet tracks if explicitly configured
	if src.Search.UseSmartConnectionsSet {
		dst.Search.UseSmartConnections = src.Search.UseSmartConnections
		dst.Search.UseSmartConnectionsSet = true
	}

	// Merge RPI config
	if src.RPI.WorktreeMode != "" {
		dst.RPI.WorktreeMode = src.RPI.WorktreeMode
	}
	if src.RPI.RuntimeMode != "" {
		dst.RPI.RuntimeMode = src.RPI.RuntimeMode
	}
	if src.RPI.RuntimeCommand != "" {
		dst.RPI.RuntimeCommand = src.RPI.RuntimeCommand
	}
	if src.RPI.AOCommand != "" {
		dst.RPI.AOCommand = src.RPI.AOCommand
	}
	if src.RPI.BDCommand != "" {
		dst.RPI.BDCommand = src.RPI.BDCommand
	}
	if src.RPI.TmuxCommand != "" {
		dst.RPI.TmuxCommand = src.RPI.TmuxCommand
	}

	// Merge Flywheel config
	if src.Flywheel.AutoPromoteThreshold != "" {
		dst.Flywheel.AutoPromoteThreshold = src.Flywheel.AutoPromoteThreshold
	}

	// Merge paths (G5: configurable paths, not hardcoded)
	if src.Paths.LearningsDir != "" {
		dst.Paths.LearningsDir = src.Paths.LearningsDir
	}
	if src.Paths.PatternsDir != "" {
		dst.Paths.PatternsDir = src.Paths.PatternsDir
	}
	if src.Paths.RetrosDir != "" {
		dst.Paths.RetrosDir = src.Paths.RetrosDir
	}
	if src.Paths.ResearchDir != "" {
		dst.Paths.ResearchDir = src.Paths.ResearchDir
	}
	if src.Paths.PlansDir != "" {
		dst.Paths.PlansDir = src.Paths.PlansDir
	}
	if src.Paths.ClaudePlansDir != "" {
		dst.Paths.ClaudePlansDir = src.Paths.ClaudePlansDir
	}
	if src.Paths.CitationsFile != "" {
		dst.Paths.CitationsFile = src.Paths.CitationsFile
	}
	if src.Paths.TranscriptsDir != "" {
		dst.Paths.TranscriptsDir = src.Paths.TranscriptsDir
	}

	return dst
}

// Source represents where a config value came from.
type Source string

const (
	SourceDefault Source = "default"
	SourceHome    Source = "~/.agentops/config.yaml"
	SourceProject Source = ".agentops/config.yaml"
	SourceEnv     Source = "environment"
	SourceFlag    Source = "flag"
)

// getEnvString returns the value and whether the env var was set.
func getEnvString(key string) (string, bool) {
	v := os.Getenv(key)
	return v, v != ""
}

// getEnvBool returns the boolean value and whether it was truthy.
func getEnvBool(key string) (bool, bool) {
	v := os.Getenv(key)
	if v == "true" || v == "1" {
		return true, true
	}
	return false, false
}

// resolveStringField resolves a string through the precedence chain.
// Returns the resolved value and its source.
func resolveStringField(home, project, env, flag, def string) resolved {
	// Start with default
	result := resolved{Value: def, Source: SourceDefault}

	// Home config overrides default
	if home != "" {
		result = resolved{Value: home, Source: SourceHome}
	}

	// Project config overrides home
	if project != "" {
		result = resolved{Value: project, Source: SourceProject}
	}

	// Environment overrides project
	if env != "" {
		result = resolved{Value: env, Source: SourceEnv}
	}

	// Flag overrides everything (if set)
	if flag != "" {
		result = resolved{Value: flag, Source: SourceFlag}
	}

	return result
}

// ResolvedConfig shows config values with their sources.
type ResolvedConfig struct {
	Output            resolved `json:"output"`
	BaseDir           resolved `json:"base_dir"`
	Verbose           resolved `json:"verbose"`
	RPIWorktreeMode   resolved `json:"rpi_worktree_mode"`
	RPIRuntimeMode    resolved `json:"rpi_runtime_mode"`
	RPIRuntimeCommand resolved `json:"rpi_runtime_command"`
	RPIAOCommand      resolved `json:"rpi_ao_command"`
	RPIBDCommand      resolved `json:"rpi_bd_command"`
	RPITmuxCommand    resolved `json:"rpi_tmux_command"`
}

type resolved struct {
	Value  interface{} `json:"value"`
	Source Source      `json:"source"`
}

// Resolve returns configuration with source tracking.
// Uses precedence chain: flags > env > project > home > defaults.
func Resolve(flagOutput, flagBaseDir string, flagVerbose bool) *ResolvedConfig {
	// Load configs once
	homeConfig, _ := loadFromPath(homeConfigPath())
	projectConfig, _ := loadFromPath(projectConfigPath())

	// Get config values (empty string if not set)
	var homeOutput, homeBaseDir string
	var homeVerbose bool
	var homeRPIWorktreeMode, homeRPIRuntimeMode, homeRPIRuntimeCommand string
	var homeRPIAOCommand, homeRPIBDCommand, homeRPITmuxCommand string
	if homeConfig != nil {
		homeOutput = homeConfig.Output
		homeBaseDir = homeConfig.BaseDir
		homeVerbose = homeConfig.Verbose
		homeRPIWorktreeMode = homeConfig.RPI.WorktreeMode
		homeRPIRuntimeMode = homeConfig.RPI.RuntimeMode
		homeRPIRuntimeCommand = homeConfig.RPI.RuntimeCommand
		homeRPIAOCommand = homeConfig.RPI.AOCommand
		homeRPIBDCommand = homeConfig.RPI.BDCommand
		homeRPITmuxCommand = homeConfig.RPI.TmuxCommand
	}

	var projectOutput, projectBaseDir string
	var projectVerbose bool
	var projectRPIWorktreeMode, projectRPIRuntimeMode, projectRPIRuntimeCommand string
	var projectRPIAOCommand, projectRPIBDCommand, projectRPITmuxCommand string
	if projectConfig != nil {
		projectOutput = projectConfig.Output
		projectBaseDir = projectConfig.BaseDir
		projectVerbose = projectConfig.Verbose
		projectRPIWorktreeMode = projectConfig.RPI.WorktreeMode
		projectRPIRuntimeMode = projectConfig.RPI.RuntimeMode
		projectRPIRuntimeCommand = projectConfig.RPI.RuntimeCommand
		projectRPIAOCommand = projectConfig.RPI.AOCommand
		projectRPIBDCommand = projectConfig.RPI.BDCommand
		projectRPITmuxCommand = projectConfig.RPI.TmuxCommand
	}

	// Get environment values
	envOutput, _ := getEnvString("AGENTOPS_OUTPUT")
	envBaseDir, _ := getEnvString("AGENTOPS_BASE_DIR")
	envVerbose, envVerboseSet := getEnvBool("AGENTOPS_VERBOSE")
	envRPIWorktreeMode, _ := getEnvString("AGENTOPS_RPI_WORKTREE_MODE")
	envRPIRuntimeMode, _ := getEnvString("AGENTOPS_RPI_RUNTIME")
	if modeOverride, ok := getEnvString("AGENTOPS_RPI_RUNTIME_MODE"); ok {
		envRPIRuntimeMode = modeOverride
	}
	envRPIRuntimeCommand, _ := getEnvString("AGENTOPS_RPI_RUNTIME_COMMAND")
	envRPIAOCommand, _ := getEnvString("AGENTOPS_RPI_AO_COMMAND")
	envRPIBDCommand, _ := getEnvString("AGENTOPS_RPI_BD_COMMAND")
	envRPITmuxCommand, _ := getEnvString("AGENTOPS_RPI_TMUX_COMMAND")

	// Resolve string fields through precedence chain
	rc := &ResolvedConfig{
		Output:            resolveStringField(homeOutput, projectOutput, envOutput, flagOutput, defaultOutput),
		BaseDir:           resolveStringField(homeBaseDir, projectBaseDir, envBaseDir, flagBaseDir, defaultBaseDir),
		Verbose:           resolved{Value: false, Source: SourceDefault},
		RPIWorktreeMode:   resolveStringField(homeRPIWorktreeMode, projectRPIWorktreeMode, envRPIWorktreeMode, "", "auto"),
		RPIRuntimeMode:    resolveStringField(homeRPIRuntimeMode, projectRPIRuntimeMode, envRPIRuntimeMode, "", "auto"),
		RPIRuntimeCommand: resolveStringField(homeRPIRuntimeCommand, projectRPIRuntimeCommand, envRPIRuntimeCommand, "", "claude"),
		RPIAOCommand:      resolveStringField(homeRPIAOCommand, projectRPIAOCommand, envRPIAOCommand, "", "ao"),
		RPIBDCommand:      resolveStringField(homeRPIBDCommand, projectRPIBDCommand, envRPIBDCommand, "", "bd"),
		RPITmuxCommand:    resolveStringField(homeRPITmuxCommand, projectRPITmuxCommand, envRPITmuxCommand, "", "tmux"),
	}

	// Resolve verbose (boolean with OR semantics through chain)
	if homeVerbose {
		rc.Verbose = resolved{Value: true, Source: SourceHome}
	}
	if projectVerbose {
		rc.Verbose = resolved{Value: true, Source: SourceProject}
	}
	if envVerboseSet && envVerbose {
		rc.Verbose = resolved{Value: true, Source: SourceEnv}
	}
	if flagVerbose {
		rc.Verbose = resolved{Value: true, Source: SourceFlag}
	}

	return rc
}
