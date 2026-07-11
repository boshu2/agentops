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
