package skillsapp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/skillshealth"
)

func TestAuditReportDestination(t *testing.T) {
	root := t.TempDir()
	root, _ = filepath.EvalSymlinks(root)
	repo := filepath.Join(root, "repo")
	out := filepath.Join(root, "reports")
	for _, p := range []string{repo, out, filepath.Join(root, "bare", "objects"), filepath.Join(root, "bare", "refs")} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	report := &skillshealth.EvidenceReport{Target: repo, SchemaVersion: "skill-audit.v2"}
	dest := filepath.Join(out, "report.json")
	if err := WriteAuditEvidence(repo, dest, report); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{dest, filepath.Join(repo, "report.json"), filepath.Join(root, "bare", "report.json")} {
		if err := WriteAuditEvidence(repo, bad, report); err == nil {
			t.Fatalf("unsafe destination accepted: %s", bad)
		}
	}
	after, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("existing report overwritten")
	}
}
