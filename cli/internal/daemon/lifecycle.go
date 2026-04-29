package daemon

import (
	"os"
	"path/filepath"
	"runtime"
)

type ServiceInstallPlan struct {
	ServiceName string   `json:"service_name"`
	Platform    string   `json:"platform"`
	DryRun      bool     `json:"dry_run"`
	Executable  string   `json:"executable"`
	RepoRoot    string   `json:"repo_root"`
	Address     string   `json:"address"`
	Args        []string `json:"args"`
	UnitPath    string   `json:"unit_path,omitempty"`
}

func BuildServiceInstallPlan(repoRoot, executable, address string, dryRun bool) ServiceInstallPlan {
	if executable == "" {
		executable = "ao"
	}
	if address == "" {
		address = "127.0.0.1:8765"
	}
	platform := runtime.GOOS
	plan := ServiceInstallPlan{
		ServiceName: "agentopsd",
		Platform:    platform,
		DryRun:      dryRun,
		Executable:  executable,
		RepoRoot:    repoRoot,
		Address:     address,
		Args:        []string{"daemon", "run", "--addr", address},
	}
	switch platform {
	case "darwin":
		home, _ := os.UserHomeDir()
		plan.UnitPath = filepath.Join(home, "Library", "LaunchAgents", "com.agentops.agentopsd.plist")
	case "linux":
		home, _ := os.UserHomeDir()
		plan.UnitPath = filepath.Join(home, ".config", "systemd", "user", "agentopsd.service")
	default:
		plan.UnitPath = filepath.Join(repoRoot, StoreDirRel, "agentopsd.service")
	}
	return plan
}
