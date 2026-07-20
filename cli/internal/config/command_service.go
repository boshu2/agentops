package config

import (
	"context"
	"fmt"
	"strings"
)

// showEnvironmentKeys deliberately omits AGENTOPS_NO_SC: it only toggles
// Search.UseSmartConnections, a dead field nothing outside config plumbing
// consumes.
var showEnvironmentKeys = []string{
	"AGENTOPS_CONFIG", "AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE",
}

var modelEnvironmentKeys = []string{"AGENTOPS_MODEL_TIER", "AGENTOPS_COUNCIL_MODEL_TIER", "COUNCIL_CLAUDE_MODEL"}

type ConfigFiles struct {
	HomePath      string
	HomeExists    bool
	ProjectPath   string
	ProjectExists bool
	// HomeReadPath is the home-layer path actually consulted for reads. It
	// equals HomePath normally; when the deprecated ~/.agentops/config.yaml
	// fallback is active it is that legacy path and HomeLegacy is true.
	HomeReadPath string
	HomeLegacy   bool
	// ProjectReadPath mirrors HomeReadPath for the project layer
	// (legacy fallback: ./.agentops/config.yaml).
	ProjectReadPath string
	ProjectLegacy   bool
}

type ShowResult struct {
	Resolved    *ResolvedConfig
	ConfigFiles ConfigFiles
	Environment map[string]string
}

type ModelsResult struct {
	Config      *Config
	Environment map[string]string
}

type ModelsWriteRequest struct {
	DefaultTier string
	Skill       string
	DryRun      bool
}

type ModelsWriteResult struct {
	Updated        bool              `json:"updated"`
	DryRun         bool              `json:"dry_run"`
	DefaultTier    string            `json:"default_tier,omitempty"`
	SkillOverrides map[string]string `json:"skill_overrides,omitempty"`
}

type CommandGateway interface {
	Resolve(string, bool) *ResolvedConfig
	Files() (ConfigFiles, error)
	Environment([]string) map[string]string
	Load() (*Config, error)
	Save(*Config) error
	PreviewSave(*Config) error
}

type CommandService struct {
	gateway CommandGateway
}

func NewCommandService(gateway CommandGateway) *CommandService {
	return &CommandService{gateway: gateway}
}

func (service *CommandService) Show(_ context.Context, output string, verbose bool) (ShowResult, error) {
	files, err := service.gateway.Files()
	if err != nil {
		return ShowResult{}, err
	}
	// Overlay the read paths actually consulted (config.go owns the legacy
	// fallback) so the files panel reports the file that was really loaded
	// instead of contradicting the deprecation warning.
	files.HomeReadPath, files.HomeLegacy = homeConfigReadInfo()
	files.ProjectReadPath, files.ProjectLegacy = projectConfigReadInfo()
	return ShowResult{
		Resolved: service.gateway.Resolve(output, verbose), ConfigFiles: files,
		Environment: service.gateway.Environment(showEnvironmentKeys),
	}, nil
}

func (service *CommandService) Models(_ context.Context) (ModelsResult, error) {
	loaded, err := service.gateway.Load()
	if err != nil {
		return ModelsResult{}, fmt.Errorf("load config: %w", err)
	}
	return ModelsResult{Config: loaded, Environment: service.gateway.Environment(modelEnvironmentKeys)}, nil
}

func (service *CommandService) WriteModels(_ context.Context, request ModelsWriteRequest) (ModelsWriteResult, error) {
	save := &Config{}
	result := ModelsWriteResult{Updated: !request.DryRun, DryRun: request.DryRun}
	if request.DefaultTier != "" {
		if request.DefaultTier == "inherit" {
			return ModelsWriteResult{}, fmt.Errorf("invalid tier %q for default: \"inherit\" is only valid for skill overrides", request.DefaultTier)
		}
		if !ValidTiers[request.DefaultTier] {
			return ModelsWriteResult{}, fmt.Errorf("invalid tier %q: must be one of quality, balanced, budget", request.DefaultTier)
		}
		save.Models.DefaultTier = request.DefaultTier
		result.DefaultTier = request.DefaultTier
	}
	if request.Skill != "" {
		parts := strings.SplitN(request.Skill, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return ModelsWriteResult{}, fmt.Errorf("invalid --set-skill format %q: expected skill=tier (e.g. council=quality)", request.Skill)
		}
		skill, tier := parts[0], parts[1]
		if !ValidTiers[tier] {
			return ModelsWriteResult{}, fmt.Errorf("invalid tier %q for skill %q: must be one of quality, balanced, budget, inherit", tier, skill)
		}
		save.Models.SkillOverrides = map[string]string{skill: tier}
		result.SkillOverrides = map[string]string{skill: tier}
	}
	if request.DryRun {
		if err := service.gateway.PreviewSave(save); err != nil {
			return ModelsWriteResult{}, fmt.Errorf("preview config save: %w", err)
		}
		return result, nil
	}
	if err := service.gateway.Save(save); err != nil {
		return ModelsWriteResult{}, fmt.Errorf("saving config: %w", err)
	}
	return result, nil
}
