package doctor

import (
	"context"
	"os"
	"time"

	doctorapp "github.com/boshu2/agentops/cli/internal/doctor"
)

type ReadRuntime struct{ ToolVersion string }

func (runtime ReadRuntime) RepoRoot(context.Context) (string, error) { return os.Getwd() }

func (runtime ReadRuntime) Options(ctx context.Context, request doctorapp.ReadRequest) (doctorapp.Options, error) {
	root, err := runtime.RepoRoot(ctx)
	if err != nil {
		return doctorapp.Options{}, err
	}
	home, _ := os.UserHomeDir()
	return doctorapp.Options{
		RepoRoot: root, CWD: root, HomeDir: home, ToolVersion: runtime.ToolVersion,
		Only: append([]string(nil), request.Only...), Skip: append([]string(nil), request.Skip...),
		Quick: request.Quick, Online: request.Online, Severity: request.Severity,
		DryRun: request.DryRun, JSON: request.JSON, Since: request.Since, Now: time.Now(),
	}, nil
}

type ReadGateway struct{}

func (ReadGateway) Diagnose(_ context.Context, options doctorapp.Options) (*doctorapp.Report, error) {
	return doctorapp.Diagnose(options)
}
func (ReadGateway) Triage(_ context.Context, options doctorapp.Options) (*doctorapp.RobotTriageResult, *doctorapp.Report, error) {
	return doctorapp.RobotTriage(options)
}
func (ReadGateway) Explain(_ context.Context, root, id string) (*doctorapp.Finding, error) {
	return doctorapp.Explain(root, id)
}
func (ReadGateway) Capabilities(_ context.Context, version string) *doctorapp.Capabilities {
	return doctorapp.NewCapabilities(version)
}
func (ReadGateway) Health(_ context.Context, root, version string) (string, *doctorapp.HealthResult, error) {
	return doctorapp.Health(root, version)
}
func (ReadGateway) RobotDocs(context.Context) string { return doctorapp.RobotDocs() }
func (ReadGateway) List(_ context.Context, root string) ([]doctorapp.RunSummary, error) {
	return doctorapp.Ls(root)
}
func (ReadGateway) Diff(_ context.Context, options doctorapp.Options) (*doctorapp.Report, error) {
	return doctorapp.Diff(options)
}
