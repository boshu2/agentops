// Package config provides configuration management for AgentOps.
// Configuration is loaded from (highest to lowest priority):
// 1. Command-line flags
// 2. Environment variables (AGENTOPS_*)
// 3. Project config (.agentops/config.yaml in cwd)
// 4. Home config (~/.agentops/config.yaml)
// 5. Defaults
package config

import (
	"fmt"
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

	// Models settings
	Models ModelsConfig `yaml:"models" json:"models"`

	// Dream settings for private overnight runs.
	Dream DreamConfig `yaml:"dream" json:"dream"`

	// Compile settings for ao compile headless runtime preference.
	Compile CompileConfig `yaml:"compile" json:"compile"`
}

// CompileConfig holds ao compile runtime preferences.
type CompileConfig struct {
	// PreferredRuntime overrides the auto-detect runtime selection.
	// When set, ao compile uses this value instead of auto-detecting a
	// local `claude` binary. Useful for users who want to run Ollama
	// even when Claude Code is installed. Accepted values: "claude-cli",
	// "ollama", "claude", "openai", or empty (use auto-detect).
	//
	// Precedence (high → low):
	//   1. --runtime flag
	//   2. AGENTOPS_COMPILE_RUNTIME env var
	//   3. compile.preferred_runtime in ~/.agentops/config.yaml
	//   4. auto-detect (claude binary on PATH → claude-cli)
	//   5. empty (preflight will fail with an actionable error)
	PreferredRuntime string `yaml:"preferred_runtime" json:"preferred_runtime"`
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

	// GlobalLearningsDir is where cross-repo learnings are stored.
	// Default: ~/.agents/learnings
	GlobalLearningsDir string `yaml:"global_learnings_dir" json:"global_learnings_dir"`

	// GlobalPatternsDir is where cross-repo patterns are stored.
	// Default: ~/.agents/patterns
	GlobalPatternsDir string `yaml:"global_patterns_dir" json:"global_patterns_dir"`

	// GlobalWeight is the rank multiplier for global results (0.0-1.0).
	// Applied AFTER composite scoring. Lower values make local results win more aggressively.
	// Default: 0.8
	GlobalWeight float64 `yaml:"global_weight" json:"global_weight"`

	// HarvestRoots is the list of directories to scan for cross-rig knowledge harvest.
	// Default: [~/gt]
	HarvestRoots []string `yaml:"harvest_roots" json:"harvest_roots"`
}

// ModelsConfig holds model tier configuration.
type ModelsConfig struct {
	// DefaultTier is the default model tier (quality, balanced, budget, inherit).
	DefaultTier string `yaml:"default_tier" json:"default_tier"`

	// Tiers maps tier name to model config.
	Tiers map[string]TierConfig `yaml:"tiers" json:"tiers"`

	// SkillOverrides maps skill name to tier name.
	SkillOverrides map[string]string `yaml:"skill_overrides" json:"skill_overrides"`
}

// DreamConfig holds settings for the private overnight Dream operator mode.
type DreamConfig struct {
	// ReportDir is where overnight summaries and step artifacts are written.
	ReportDir string `yaml:"report_dir" json:"report_dir"`

	// RunTimeout is the maximum duration for a single overnight run.
	RunTimeout string `yaml:"run_timeout" json:"run_timeout"`

	// KeepAwake controls whether local runs should attempt keep-awake assistance.
	// Nil means "not explicitly configured".
	KeepAwake *bool `yaml:"keep_awake,omitempty" json:"keep_awake,omitempty"`

	// Runners is the ordered list of Dream runtimes to use for multimodel work.
	Runners []string `yaml:"runners,omitempty" json:"runners,omitempty"`

	// SchedulerMode is the chosen host scheduler mode (manual, launchd, cron, systemd).
	SchedulerMode string `yaml:"scheduler_mode,omitempty" json:"scheduler_mode,omitempty"`

	// ScheduleAt is the preferred local time (HH:MM) for host-scheduled Dream runs.
	ScheduleAt string `yaml:"schedule_at,omitempty" json:"schedule_at,omitempty"`

	// ConsensusPolicy controls how Dream Council synthesizes multiple runner outputs.
	ConsensusPolicy string `yaml:"consensus_policy,omitempty" json:"consensus_policy,omitempty"`

	// CreativeLane controls whether Dream Council should request wildcard ideas.
	// Nil means "not explicitly configured".
	CreativeLane *bool `yaml:"creative_lane,omitempty" json:"creative_lane,omitempty"`

	// LocalCurator configures a local Tier 1 curator such as Ollama/Gemma.
	LocalCurator DreamLocalCuratorConfig `yaml:"local_curator,omitempty" json:"local_curator,omitempty"`
}

// DreamLocalCuratorConfig holds settings for a local Tier 1 Dream curator.
type DreamLocalCuratorConfig struct {
	// Enabled controls whether the local curator should be treated as active.
	// Nil means "not explicitly configured".
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// Engine identifies the local engine. V1 supports "ollama".
	Engine string `yaml:"engine,omitempty" json:"engine,omitempty"`

	// OllamaURL is the Ollama API root used when Engine is "ollama".
	OllamaURL string `yaml:"ollama_url,omitempty" json:"ollama_url,omitempty"`

	// Model is the Tier 1 model name.
	Model string `yaml:"model,omitempty" json:"model,omitempty"`

	// WorkerDir is the local Dream worker directory.
	WorkerDir string `yaml:"worker_dir,omitempty" json:"worker_dir,omitempty"`

	// VaultDir is the local knowledge vault written by the worker.
	VaultDir string `yaml:"vault_dir,omitempty" json:"vault_dir,omitempty"`

	// HourlyCap is the worker's bounded autonomy limit.
	HourlyCap int `yaml:"hourly_cap,omitempty" json:"hourly_cap,omitempty"`

	// AllowedJobKinds is the local curator queue allowlist.
	AllowedJobKinds []string `yaml:"allowed_job_kinds,omitempty" json:"allowed_job_kinds,omitempty"`
}

// TierConfig holds model names for a tier.
type TierConfig struct {
	// Claude is the Claude model name (opus, sonnet, haiku).
	Claude string `yaml:"claude" json:"claude"`

	// Codex is the Codex model name.
	Codex string `yaml:"codex" json:"codex"`
}

// ValidTiers is the set of recognized model cost tier names.
var ValidTiers = map[string]bool{
	"quality":  true,
	"balanced": true,
	"budget":   true,
	"inherit":  true,
}

// ResolveTier returns the effective tier for a given skill.
// Checks SkillOverrides first, falls back to DefaultTier.
// "inherit" means use DefaultTier; if DefaultTier is also "inherit" or empty,
// falls back to "balanced". Unrecognized tier names fall back to "balanced".
func (c *Config) ResolveTier(skillName string) string {
	tier := c.Models.DefaultTier
	if override, ok := c.Models.SkillOverrides[skillName]; ok && override != "" {
		if override != "inherit" {
			tier = override
		}
		// "inherit" in override → use DefaultTier (fall through)
	}
	if tier == "" || tier == "inherit" {
		return "balanced"
	}
	if !ValidTiers[tier] {
		return "balanced"
	}
	return tier
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
	defaultKeepAwake := true
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
		Models: ModelsConfig{
			DefaultTier: "balanced",
			Tiers: map[string]TierConfig{
				"quality":  {Claude: "opus", Codex: ""},
				"balanced": {Claude: "sonnet", Codex: ""},
				"budget":   {Claude: "haiku", Codex: ""},
			},
			SkillOverrides: map[string]string{},
		},
		Dream: DreamConfig{
			ReportDir:       ".agents/overnight/latest",
			RunTimeout:      "8h",
			KeepAwake:       &defaultKeepAwake,
			SchedulerMode:   "manual",
			ConsensusPolicy: "majority",
		},
		Paths: PathsConfig{
			LearningsDir:       ".agents/learnings",
			PatternsDir:        ".agents/patterns",
			RetrosDir:          ".agents/retros",
			ResearchDir:        ".agents/research",
			PlansDir:           ".agents/plans",
			ClaudePlansDir:     filepath.Join(homeDir, ".claude", "plans"),
			CitationsFile:      ".agents/ao/citations.jsonl",
			TranscriptsDir:     filepath.Join(homeDir, ".claude", "projects"),
			GlobalLearningsDir: filepath.Join(homeDir, ".agents", "learnings"),
			GlobalPatternsDir:  filepath.Join(homeDir, ".agents", "patterns"),
			GlobalWeight:       0.8,
			HarvestRoots:       []string{filepath.Join(homeDir, "gt")},
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

// getwdFunc is the function used to get the current working directory.
// It can be overridden in tests to simulate os.Getwd failures.
var getwdFunc = os.Getwd

// projectConfigPath returns the project config path.
func projectConfigPath() string {
	if override := strings.TrimSpace(os.Getenv("AGENTOPS_CONFIG")); override != "" {
		return override
	}
	cwd, err := getwdFunc()
	if err != nil {
		return ""
	}
	return filepath.Join(cwd, ".agentops", "config.yaml")
}

// loadHomeConfig loads the home directory config, returning nil on error.
func loadHomeConfig() *Config {
	cfg, _ := loadFromPath(homeConfigPath())
	return cfg
}

// loadProjectConfig loads the project config, returning nil on error.
func loadProjectConfig() *Config {
	cfg, _ := loadFromPath(projectConfigPath())
	return cfg
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

// applyEnvStr sets dst from the named environment variable when non-empty.
func applyEnvStr(dst *string, envKey string) {
	if v := os.Getenv(envKey); v != "" {
		*dst = v
	}
}

// envBool returns true when the named environment variable is "true" or "1".
func envBool(envKey string) bool {
	v := os.Getenv(envKey)
	return v == "true" || v == "1"
}

// applyEnv applies environment variable overrides.
func applyEnv(cfg *Config) *Config {
	applyEnvStr(&cfg.Output, "AGENTOPS_OUTPUT")
	applyEnvStr(&cfg.BaseDir, "AGENTOPS_BASE_DIR")
	if envBool("AGENTOPS_VERBOSE") {
		cfg.Verbose = true
	}
	if envBool("AGENTOPS_NO_SC") {
		cfg.Search.UseSmartConnections = false
		cfg.Search.UseSmartConnectionsSet = true
	}
	applyEnvStr(&cfg.RPI.WorktreeMode, "AGENTOPS_RPI_WORKTREE_MODE")
	applyEnvStr(&cfg.RPI.RuntimeMode, "AGENTOPS_RPI_RUNTIME")
	applyEnvStr(&cfg.RPI.RuntimeMode, "AGENTOPS_RPI_RUNTIME_MODE")
	applyEnvStr(&cfg.RPI.RuntimeCommand, "AGENTOPS_RPI_RUNTIME_COMMAND")
	applyEnvStr(&cfg.RPI.AOCommand, "AGENTOPS_RPI_AO_COMMAND")
	applyEnvStr(&cfg.RPI.BDCommand, "AGENTOPS_RPI_BD_COMMAND")
	applyEnvStr(&cfg.RPI.TmuxCommand, "AGENTOPS_RPI_TMUX_COMMAND")
	applyEnvStr(&cfg.Flywheel.AutoPromoteThreshold, "AGENTOPS_FLYWHEEL_AUTO_PROMOTE_THRESHOLD")
	applyEnvStr(&cfg.Models.DefaultTier, "AGENTOPS_MODEL_TIER")
	applyEnvStr(&cfg.Dream.ReportDir, "AGENTOPS_DREAM_REPORT_DIR")
	applyEnvStr(&cfg.Dream.RunTimeout, "AGENTOPS_DREAM_RUN_TIMEOUT")
	if v, ok := getEnvBoolValue("AGENTOPS_DREAM_KEEP_AWAKE"); ok {
		cfg.Dream.KeepAwake = boolPtr(v)
	}
	if v, ok := getEnvBoolValue("AGENTOPS_DREAM_CURATOR_ENABLED"); ok {
		cfg.Dream.LocalCurator.Enabled = boolPtr(v)
	}
	applyEnvStr(&cfg.Dream.LocalCurator.Engine, "AGENTOPS_DREAM_CURATOR_ENGINE")
	applyEnvStr(&cfg.Dream.LocalCurator.OllamaURL, "AGENTOPS_DREAM_CURATOR_OLLAMA_URL")
	applyEnvStr(&cfg.Dream.LocalCurator.Model, "AGENTOPS_DREAM_CURATOR_MODEL")
	applyEnvStr(&cfg.Dream.LocalCurator.WorkerDir, "AGENTOPS_DREAM_CURATOR_WORKER_DIR")
	applyEnvStr(&cfg.Dream.LocalCurator.VaultDir, "AGENTOPS_DREAM_CURATOR_VAULT_DIR")
	if v := os.Getenv("AGENTOPS_COUNCIL_MODEL_TIER"); v != "" {
		if cfg.Models.SkillOverrides == nil {
			cfg.Models.SkillOverrides = make(map[string]string)
		}
		cfg.Models.SkillOverrides["council"] = v
	}
	return cfg
}

// mergeStr overwrites dst with src when src is non-empty.
func mergeStr(dst *string, src string) {
	if src != "" {
		*dst = src
	}
}

// mergeInt overwrites dst with src when src is non-zero.
func mergeInt(dst *int, src int) {
	if src != 0 {
		*dst = src
	}
}

// merge merges src into dst, with src values taking precedence.
// For booleans, we need explicit tracking via pointer or separate "set" flag.
func merge(dst, src *Config) *Config {
	mergeStr(&dst.Output, src.Output)
	mergeStr(&dst.BaseDir, src.BaseDir)
	if src.Verbose {
		dst.Verbose = true
	}

	mergeForge(&dst.Forge, &src.Forge)
	mergeSearch(&dst.Search, &src.Search)
	mergeRPI(&dst.RPI, &src.RPI)
	mergeFlywheel(&dst.Flywheel, &src.Flywheel)
	mergeModels(&dst.Models, &src.Models)
	mergeDream(&dst.Dream, &src.Dream)
	mergePaths(&dst.Paths, &src.Paths)

	return dst
}

// mergeForge merges forge-specific config fields.
func mergeForge(dst, src *ForgeConfig) {
	mergeInt(&dst.MaxContentLength, src.MaxContentLength)
	mergeInt(&dst.ProgressInterval, src.ProgressInterval)
}

// mergeSearch merges search-specific config fields.
func mergeSearch(dst, src *SearchConfig) {
	mergeInt(&dst.DefaultLimit, src.DefaultLimit)
	// UseSmartConnections: src.UseSmartConnectionsSet tracks if explicitly configured
	if src.UseSmartConnectionsSet {
		dst.UseSmartConnections = src.UseSmartConnections
		dst.UseSmartConnectionsSet = true
	}
}

// mergeRPI merges RPI-specific config fields.
func mergeRPI(dst, src *RPIConfig) {
	mergeStr(&dst.WorktreeMode, src.WorktreeMode)
	mergeStr(&dst.RuntimeMode, src.RuntimeMode)
	mergeStr(&dst.RuntimeCommand, src.RuntimeCommand)
	mergeStr(&dst.AOCommand, src.AOCommand)
	mergeStr(&dst.BDCommand, src.BDCommand)
	mergeStr(&dst.TmuxCommand, src.TmuxCommand)
}

// mergeFlywheel merges flywheel-specific config fields.
func mergeFlywheel(dst, src *FlywheelConfig) {
	mergeStr(&dst.AutoPromoteThreshold, src.AutoPromoteThreshold)
}

// mergeModels merges models config fields.
func mergeModels(dst, src *ModelsConfig) {
	mergeStr(&dst.DefaultTier, src.DefaultTier)
	if len(src.Tiers) > 0 {
		if dst.Tiers == nil {
			dst.Tiers = make(map[string]TierConfig)
		}
		for k, v := range src.Tiers {
			dst.Tiers[k] = v
		}
	}
	if len(src.SkillOverrides) > 0 {
		if dst.SkillOverrides == nil {
			dst.SkillOverrides = make(map[string]string)
		}
		for k, v := range src.SkillOverrides {
			dst.SkillOverrides[k] = v
		}
	}
}

// mergeDream merges Dream config fields.
func mergeDream(dst, src *DreamConfig) {
	mergeStr(&dst.ReportDir, src.ReportDir)
	mergeStr(&dst.RunTimeout, src.RunTimeout)
	if src.KeepAwake != nil {
		dst.KeepAwake = boolPtr(*src.KeepAwake)
	}
	if len(src.Runners) > 0 {
		dst.Runners = append([]string{}, src.Runners...)
	}
	mergeStr(&dst.SchedulerMode, src.SchedulerMode)
	mergeStr(&dst.ScheduleAt, src.ScheduleAt)
	mergeStr(&dst.ConsensusPolicy, src.ConsensusPolicy)
	if src.CreativeLane != nil {
		dst.CreativeLane = boolPtr(*src.CreativeLane)
	}
	mergeDreamLocalCurator(&dst.LocalCurator, &src.LocalCurator)
}

func mergeDreamLocalCurator(dst, src *DreamLocalCuratorConfig) {
	if src.Enabled != nil {
		dst.Enabled = boolPtr(*src.Enabled)
	}
	mergeStr(&dst.Engine, src.Engine)
	mergeStr(&dst.OllamaURL, src.OllamaURL)
	mergeStr(&dst.Model, src.Model)
	mergeStr(&dst.WorkerDir, src.WorkerDir)
	mergeStr(&dst.VaultDir, src.VaultDir)
	mergeInt(&dst.HourlyCap, src.HourlyCap)
	if len(src.AllowedJobKinds) > 0 {
		dst.AllowedJobKinds = append([]string{}, src.AllowedJobKinds...)
	}
}

// mergePaths merges path config fields (G5: configurable paths, not hardcoded).
func mergePaths(dst, src *PathsConfig) {
	mergeStr(&dst.LearningsDir, src.LearningsDir)
	mergeStr(&dst.PatternsDir, src.PatternsDir)
	mergeStr(&dst.RetrosDir, src.RetrosDir)
	mergeStr(&dst.ResearchDir, src.ResearchDir)
	mergeStr(&dst.PlansDir, src.PlansDir)
	mergeStr(&dst.ClaudePlansDir, src.ClaudePlansDir)
	mergeStr(&dst.CitationsFile, src.CitationsFile)
	mergeStr(&dst.TranscriptsDir, src.TranscriptsDir)
	mergeStr(&dst.GlobalLearningsDir, src.GlobalLearningsDir)
	mergeStr(&dst.GlobalPatternsDir, src.GlobalPatternsDir)
	if src.GlobalWeight > 0 {
		dst.GlobalWeight = src.GlobalWeight
	}
	if len(src.HarvestRoots) > 0 {
		dst.HarvestRoots = src.HarvestRoots
	}
}

// Save writes the given config to the project config file.
// It merges with existing config to preserve fields not being changed.
func Save(cfg *Config) error {
	path := projectConfigPath()
	if path == "" {
		return fmt.Errorf("determining project config path: could not resolve working directory")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory %s: %w", dir, err)
	}

	// Load existing config to preserve fields not being changed.
	existing, _ := loadFromPath(path)
	if existing != nil {
		existing = merge(existing, cfg)
	} else {
		existing = cfg
	}

	data, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Write atomically: temp file then rename.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing temp config %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming temp config to %s: %w", path, err)
	}

	return nil
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

// getEnvBoolValue returns the boolean value and whether the env var was set to
// a recognized boolean literal.
func getEnvBoolValue(key string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "true", "1", "yes", "on":
		return true, true
	case "false", "0", "no", "off":
		return false, true
	default:
		return false, false
	}
}

func boolPtr(v bool) *bool {
	return &v
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

func resolveStringSliceField(home, project []string, def []string) resolved {
	result := resolved{Value: append([]string{}, def...), Source: SourceDefault}
	if len(home) > 0 {
		result = resolved{Value: append([]string{}, home...), Source: SourceHome}
	}
	if len(project) > 0 {
		result = resolved{Value: append([]string{}, project...), Source: SourceProject}
	}
	return result
}

// ResolvedConfig shows config values with their sources.
type ResolvedConfig struct {
	Output                resolved `json:"output"`
	BaseDir               resolved `json:"base_dir"`
	Verbose               resolved `json:"verbose"`
	RPIWorktreeMode       resolved `json:"rpi_worktree_mode"`
	RPIRuntimeMode        resolved `json:"rpi_runtime_mode"`
	RPIRuntimeCommand     resolved `json:"rpi_runtime_command"`
	RPIAOCommand          resolved `json:"rpi_ao_command"`
	RPIBDCommand          resolved `json:"rpi_bd_command"`
	RPITmuxCommand        resolved `json:"rpi_tmux_command"`
	ModelsDefaultTier     resolved `json:"models_default_tier"`
	DreamReportDir        resolved `json:"dream_report_dir"`
	DreamRunTimeout       resolved `json:"dream_run_timeout"`
	DreamKeepAwake        resolved `json:"dream_keep_awake"`
	DreamRunners          resolved `json:"dream_runners"`
	DreamScheduler        resolved `json:"dream_scheduler_mode"`
	DreamScheduleAt       resolved `json:"dream_schedule_at"`
	DreamConsensus        resolved `json:"dream_consensus_policy"`
	DreamCreativeLane     resolved `json:"dream_creative_lane"`
	DreamCuratorEnabled   resolved `json:"dream_curator_enabled"`
	DreamCuratorEngine    resolved `json:"dream_curator_engine"`
	DreamCuratorOllamaURL resolved `json:"dream_curator_ollama_url"`
	DreamCuratorModel     resolved `json:"dream_curator_model"`
	DreamCuratorWorkerDir resolved `json:"dream_curator_worker_dir"`
	DreamCuratorVaultDir  resolved `json:"dream_curator_vault_dir"`
	DreamCuratorJobKinds  resolved `json:"dream_curator_allowed_job_kinds"`
}

type resolved struct {
	Value  any    `json:"value"`
	Source Source `json:"source"`
}

// configFields holds extracted string fields from a Config for resolution.
type configFields struct {
	output, baseDir                 string
	verbose                         bool
	rpiWorktreeMode, rpiRuntimeMode string
	rpiRuntimeCommand, rpiAOCommand string
	rpiBDCommand, rpiTmuxCommand    string
	modelsDefaultTier               string
	dreamReportDir, dreamRunTimeout string
	dreamKeepAwake                  bool
	dreamKeepAwakeSet               bool
	dreamRunners                    []string
	dreamSchedulerMode              string
	dreamScheduleAt                 string
	dreamConsensusPolicy            string
	dreamCreativeLane               bool
	dreamCreativeLaneSet            bool
	dreamCuratorEnabled             bool
	dreamCuratorEnabledSet          bool
	dreamCuratorEngine              string
	dreamCuratorOllamaURL           string
	dreamCuratorModel               string
	dreamCuratorWorkerDir           string
	dreamCuratorVaultDir            string
	dreamCuratorJobKinds            []string
}

// extractFields pulls resolution-relevant fields from a Config.
// Returns zero-value fields when cfg is nil.
func extractFields(cfg *Config) configFields {
	if cfg == nil {
		return configFields{}
	}
	return configFields{
		output:                 cfg.Output,
		baseDir:                cfg.BaseDir,
		verbose:                cfg.Verbose,
		rpiWorktreeMode:        cfg.RPI.WorktreeMode,
		rpiRuntimeMode:         cfg.RPI.RuntimeMode,
		rpiRuntimeCommand:      cfg.RPI.RuntimeCommand,
		rpiAOCommand:           cfg.RPI.AOCommand,
		rpiBDCommand:           cfg.RPI.BDCommand,
		rpiTmuxCommand:         cfg.RPI.TmuxCommand,
		modelsDefaultTier:      cfg.Models.DefaultTier,
		dreamReportDir:         cfg.Dream.ReportDir,
		dreamRunTimeout:        cfg.Dream.RunTimeout,
		dreamKeepAwake:         cfg.Dream.KeepAwake != nil && *cfg.Dream.KeepAwake,
		dreamKeepAwakeSet:      cfg.Dream.KeepAwake != nil,
		dreamRunners:           append([]string{}, cfg.Dream.Runners...),
		dreamSchedulerMode:     cfg.Dream.SchedulerMode,
		dreamScheduleAt:        cfg.Dream.ScheduleAt,
		dreamConsensusPolicy:   cfg.Dream.ConsensusPolicy,
		dreamCreativeLane:      cfg.Dream.CreativeLane != nil && *cfg.Dream.CreativeLane,
		dreamCreativeLaneSet:   cfg.Dream.CreativeLane != nil,
		dreamCuratorEnabled:    cfg.Dream.LocalCurator.Enabled != nil && *cfg.Dream.LocalCurator.Enabled,
		dreamCuratorEnabledSet: cfg.Dream.LocalCurator.Enabled != nil,
		dreamCuratorEngine:     cfg.Dream.LocalCurator.Engine,
		dreamCuratorOllamaURL:  cfg.Dream.LocalCurator.OllamaURL,
		dreamCuratorModel:      cfg.Dream.LocalCurator.Model,
		dreamCuratorWorkerDir:  cfg.Dream.LocalCurator.WorkerDir,
		dreamCuratorVaultDir:   cfg.Dream.LocalCurator.VaultDir,
		dreamCuratorJobKinds:   append([]string{}, cfg.Dream.LocalCurator.AllowedJobKinds...),
	}
}

// envFields holds environment variable values for resolution.
type envFields struct {
	output, baseDir                 string
	verbose                         bool
	verboseSet                      bool
	rpiWorktreeMode, rpiRuntimeMode string
	rpiRuntimeCommand, rpiAOCommand string
	rpiBDCommand, rpiTmuxCommand    string
	modelsDefaultTier               string
	dreamReportDir, dreamRunTimeout string
	dreamKeepAwake                  bool
	dreamKeepAwakeSet               bool
	dreamCuratorEnabled             bool
	dreamCuratorEnabledSet          bool
	dreamCuratorEngine              string
	dreamCuratorOllamaURL           string
	dreamCuratorModel               string
	dreamCuratorWorkerDir           string
	dreamCuratorVaultDir            string
}

// loadEnvFields reads all resolution-relevant environment variables.
func loadEnvFields() envFields {
	ef := envFields{}
	ef.output, _ = getEnvString("AGENTOPS_OUTPUT")
	ef.baseDir, _ = getEnvString("AGENTOPS_BASE_DIR")
	ef.verbose, ef.verboseSet = getEnvBool("AGENTOPS_VERBOSE")
	ef.rpiWorktreeMode, _ = getEnvString("AGENTOPS_RPI_WORKTREE_MODE")
	ef.rpiRuntimeMode, _ = getEnvString("AGENTOPS_RPI_RUNTIME")
	if modeOverride, ok := getEnvString("AGENTOPS_RPI_RUNTIME_MODE"); ok {
		ef.rpiRuntimeMode = modeOverride
	}
	ef.rpiRuntimeCommand, _ = getEnvString("AGENTOPS_RPI_RUNTIME_COMMAND")
	ef.rpiAOCommand, _ = getEnvString("AGENTOPS_RPI_AO_COMMAND")
	ef.rpiBDCommand, _ = getEnvString("AGENTOPS_RPI_BD_COMMAND")
	ef.rpiTmuxCommand, _ = getEnvString("AGENTOPS_RPI_TMUX_COMMAND")
	ef.modelsDefaultTier, _ = getEnvString("AGENTOPS_MODEL_TIER")
	ef.dreamReportDir, _ = getEnvString("AGENTOPS_DREAM_REPORT_DIR")
	ef.dreamRunTimeout, _ = getEnvString("AGENTOPS_DREAM_RUN_TIMEOUT")
	ef.dreamKeepAwake, ef.dreamKeepAwakeSet = getEnvBoolValue("AGENTOPS_DREAM_KEEP_AWAKE")
	ef.dreamCuratorEnabled, ef.dreamCuratorEnabledSet = getEnvBoolValue("AGENTOPS_DREAM_CURATOR_ENABLED")
	ef.dreamCuratorEngine, _ = getEnvString("AGENTOPS_DREAM_CURATOR_ENGINE")
	ef.dreamCuratorOllamaURL, _ = getEnvString("AGENTOPS_DREAM_CURATOR_OLLAMA_URL")
	ef.dreamCuratorModel, _ = getEnvString("AGENTOPS_DREAM_CURATOR_MODEL")
	ef.dreamCuratorWorkerDir, _ = getEnvString("AGENTOPS_DREAM_CURATOR_WORKER_DIR")
	ef.dreamCuratorVaultDir, _ = getEnvString("AGENTOPS_DREAM_CURATOR_VAULT_DIR")
	return ef
}

// resolveVerbose resolves the verbose flag through the precedence chain.
func resolveVerbose(home, project configFields, env envFields, flagVerbose bool) resolved {
	r := resolved{Value: false, Source: SourceDefault}
	if home.verbose {
		r = resolved{Value: true, Source: SourceHome}
	}
	if project.verbose {
		r = resolved{Value: true, Source: SourceProject}
	}
	if env.verboseSet && env.verbose {
		r = resolved{Value: true, Source: SourceEnv}
	}
	if flagVerbose {
		r = resolved{Value: true, Source: SourceFlag}
	}
	return r
}

func resolveBoolField(
	homeVal bool, homeSet bool,
	projectVal bool, projectSet bool,
	envVal bool, envSet bool,
	def bool,
) resolved {
	r := resolved{Value: def, Source: SourceDefault}
	if homeSet {
		r = resolved{Value: homeVal, Source: SourceHome}
	}
	if projectSet {
		r = resolved{Value: projectVal, Source: SourceProject}
	}
	if envSet {
		r = resolved{Value: envVal, Source: SourceEnv}
	}
	return r
}

// Resolve returns configuration with source tracking.
// Uses precedence chain: flags > env > project > home > defaults.
func Resolve(flagOutput, flagBaseDir string, flagVerbose bool) *ResolvedConfig {
	home := extractFields(loadHomeConfig())
	project := extractFields(loadProjectConfig())
	env := loadEnvFields()

	return &ResolvedConfig{
		Output:            resolveStringField(home.output, project.output, env.output, flagOutput, defaultOutput),
		BaseDir:           resolveStringField(home.baseDir, project.baseDir, env.baseDir, flagBaseDir, defaultBaseDir),
		Verbose:           resolveVerbose(home, project, env, flagVerbose),
		RPIWorktreeMode:   resolveStringField(home.rpiWorktreeMode, project.rpiWorktreeMode, env.rpiWorktreeMode, "", "auto"),
		RPIRuntimeMode:    resolveStringField(home.rpiRuntimeMode, project.rpiRuntimeMode, env.rpiRuntimeMode, "", "auto"),
		RPIRuntimeCommand: resolveStringField(home.rpiRuntimeCommand, project.rpiRuntimeCommand, env.rpiRuntimeCommand, "", "claude"),
		RPIAOCommand:      resolveStringField(home.rpiAOCommand, project.rpiAOCommand, env.rpiAOCommand, "", "ao"),
		RPIBDCommand:      resolveStringField(home.rpiBDCommand, project.rpiBDCommand, env.rpiBDCommand, "", "bd"),
		RPITmuxCommand:    resolveStringField(home.rpiTmuxCommand, project.rpiTmuxCommand, env.rpiTmuxCommand, "", "tmux"),
		ModelsDefaultTier: resolveStringField(home.modelsDefaultTier, project.modelsDefaultTier, env.modelsDefaultTier, "", "balanced"),
		DreamReportDir:    resolveStringField(home.dreamReportDir, project.dreamReportDir, env.dreamReportDir, "", ".agents/overnight/latest"),
		DreamRunTimeout:   resolveStringField(home.dreamRunTimeout, project.dreamRunTimeout, env.dreamRunTimeout, "", "8h"),
		DreamKeepAwake: resolveBoolField(
			home.dreamKeepAwake, home.dreamKeepAwakeSet,
			project.dreamKeepAwake, project.dreamKeepAwakeSet,
			env.dreamKeepAwake, env.dreamKeepAwakeSet,
			true,
		),
		DreamRunners:    resolveStringSliceField(home.dreamRunners, project.dreamRunners, []string{}),
		DreamScheduler:  resolveStringField(home.dreamSchedulerMode, project.dreamSchedulerMode, "", "", "manual"),
		DreamScheduleAt: resolveStringField(home.dreamScheduleAt, project.dreamScheduleAt, "", "", ""),
		DreamConsensus:  resolveStringField(home.dreamConsensusPolicy, project.dreamConsensusPolicy, "", "", "majority"),
		DreamCreativeLane: resolveBoolField(
			home.dreamCreativeLane, home.dreamCreativeLaneSet,
			project.dreamCreativeLane, project.dreamCreativeLaneSet,
			false, false,
			false,
		),
		DreamCuratorEnabled: resolveBoolField(
			home.dreamCuratorEnabled, home.dreamCuratorEnabledSet,
			project.dreamCuratorEnabled, project.dreamCuratorEnabledSet,
			env.dreamCuratorEnabled, env.dreamCuratorEnabledSet,
			false,
		),
		DreamCuratorEngine:    resolveStringField(home.dreamCuratorEngine, project.dreamCuratorEngine, env.dreamCuratorEngine, "", ""),
		DreamCuratorOllamaURL: resolveStringField(home.dreamCuratorOllamaURL, project.dreamCuratorOllamaURL, env.dreamCuratorOllamaURL, "", ""),
		DreamCuratorModel:     resolveStringField(home.dreamCuratorModel, project.dreamCuratorModel, env.dreamCuratorModel, "", ""),
		DreamCuratorWorkerDir: resolveStringField(home.dreamCuratorWorkerDir, project.dreamCuratorWorkerDir, env.dreamCuratorWorkerDir, "", ""),
		DreamCuratorVaultDir:  resolveStringField(home.dreamCuratorVaultDir, project.dreamCuratorVaultDir, env.dreamCuratorVaultDir, "", ""),
		DreamCuratorJobKinds:  resolveStringSliceField(home.dreamCuratorJobKinds, project.dreamCuratorJobKinds, []string{}),
	}
}
