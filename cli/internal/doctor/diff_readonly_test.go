package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiffDoesNotCreateDoctorArtifactsOrEditGitignore(t *testing.T) {
	root := t.TempDir()
	gitignore := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(gitignore, []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Diff(Options{RepoRoot: root, CWD: root, HomeDir: root, ToolVersion: "test", Now: time.Unix(1_700_000_000, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if report.Findings == nil || report.RunID != "" || report.RunDir != "" {
		t.Fatalf("transient report = %+v", report)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var shape map[string]any
	if err := json.Unmarshal(encoded, &shape); err != nil {
		t.Fatal(err)
	}
	if _, ok := shape["findings"].([]any); !ok {
		t.Fatalf("diff JSON findings = %#v", shape["findings"])
	}
	if _, err := os.Stat(filepath.Join(root, ".doctor")); !os.IsNotExist(err) {
		t.Fatalf("diff created .doctor: %v", err)
	}
	body, err := os.ReadFile(gitignore)
	if err != nil || string(body) != "existing\n" {
		t.Fatalf("gitignore body=%q err=%v", body, err)
	}
}

func TestFindingsSinceReturnsOnlyNewFindingIDs(t *testing.T) {
	current := []Finding{{ID: "same"}, {ID: "new"}}
	prior := []Finding{{ID: "same"}, {ID: "gone"}}
	got := findingsSince(current, prior)
	if len(got) != 1 || got[0].ID != "new" {
		t.Fatalf("findingsSince = %+v", got)
	}
}

func TestDiagnoseSinceDoesNotReplaceLatestSnapshot(t *testing.T) {
	root := t.TempDir()
	runName := "2026-01-01T00-00-00Z__prior1"
	runDir := filepath.Join(root, ".doctor", "runs", runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prior := Report{
		SchemaVersion: SchemaVersion, RunID: "prior1", StartedAt: "2026-01-01T00:00:00Z",
		Summary:  ReportSummary{TotalFindings: 1, BySeverity: map[string]int{"P0": 0, "P1": 1, "P2": 0, "P3": 0}},
		Findings: []Finding{{ID: "persistent", Severity: "P1"}}, ExitCode: ExitFindings,
	}
	data, err := json.Marshal(prior)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "report.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("runs", runName), filepath.Join(root, ".doctor", "latest")); err != nil {
		t.Fatal(err)
	}

	report, err := Diagnose(Options{RepoRoot: root, CWD: root, HomeDir: root, ToolVersion: "test", Since: "prior1", Now: time.Unix(1_700_000_000, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if report.RunID != "" || report.RunDir != "" {
		t.Fatalf("delta report persisted identity: %+v", report)
	}
	target, err := os.Readlink(filepath.Join(root, ".doctor", "latest"))
	if err != nil || target != filepath.Join("runs", runName) {
		t.Fatalf("latest=%q err=%v", target, err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".doctor", "runs"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("runs=%v err=%v", entries, err)
	}
	_, health, err := Health(root, "test")
	if err != nil || health.Findings != 1 || health.ExitCode != ExitFindings {
		t.Fatalf("health=%+v err=%v", health, err)
	}
}
