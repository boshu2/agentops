package config

import (
	"context"
)

// showEnvironmentKeys deliberately omits AGENTOPS_NO_SC: it only toggles
// Search.UseSmartConnections, a dead field nothing outside config plumbing
// consumes.
var showEnvironmentKeys = []string{
	"AGENTOPS_CONFIG", "AGENTOPS_OUTPUT", "AGENTOPS_BASE_DIR", "AGENTOPS_VERBOSE",
}

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

type CommandGateway interface {
	Resolve(string, bool) *ResolvedConfig
	Files() (ConfigFiles, error)
	Environment([]string) map[string]string
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
