package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	doctorapp "github.com/boshu2/agentops/cli/internal/doctor"
)

func TestMutationRuntimeMapsApplicationRequest(t *testing.T) {
	options, err := (MutationRuntime{ToolVersion: "3.0.0"}).Options(context.Background(), doctorapp.MutationRequest{
		Only: []string{"one"}, Skip: []string{"two"}, Quick: true, Online: true,
		Severity: "P2", DryRun: true, JSON: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.ToolVersion != "3.0.0" || !options.Quick || !options.Online || !options.DryRun || !options.JSON {
		t.Fatalf("options=%+v", options)
	}
	if len(options.Only) != 1 || options.Only[0] != "one" || len(options.Skip) != 1 || options.Skip[0] != "two" {
		t.Fatalf("options=%+v", options)
	}
}

func TestMutationServiceDryRunThroughRealAdaptersDoesNotPersist(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	configDir := filepath.Join(root, ".agentops")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("broken: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := doctorapp.NewMutationService(MutationRuntime{ToolVersion: "test"}, MutationGateway{})
	report, err := service.Fix(context.Background(), doctorapp.MutationRequest{
		Only: []string{"fm-cli-config-invalid-config-yaml-swallowed"}, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ExitCode != doctorapp.ExitHealthy || report.Summary.TotalFindings != 1 {
		t.Fatalf("report=%+v", report)
	}
	if _, err := os.Stat(filepath.Join(root, ".doctor")); !os.IsNotExist(err) {
		t.Fatalf("service fix --dry-run created .doctor: %v", err)
	}
}
