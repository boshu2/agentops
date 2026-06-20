package statememory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAdmissionCoreAdmitsValidFinding(t *testing.T) {
	root := repoRoot(t)
	tmp := copyStateSchemasAndFixtures(t, root)
	findingBytes := readFixture(t, tmp, "valid-finding.json")

	report, err := AdmitFinding(context.Background(), findingBytes, AdmissionRequest{
		SchemaVersion: 1,
		Kind:          AdmissionKind,
		CandidatePath: "schemas/fixtures/state-memory/valid-finding.json",
		Destination:   ".agents/state/findings/finding-age-membrane-valid.json",
		OperatorID:    "codex:validator-b",
		Reason:        "unit test admission",
	}, AdmissionOptions{
		Root:   tmp,
		Now:    mustParseTime(t, "2026-06-20T00:00:00Z"),
		MaxAge: 30 * 24 * time.Hour,
		Write:  true,
	})
	if err != nil {
		t.Fatalf("AdmitFinding(valid): %v", err)
	}
	if report.Verdict != "ADMITTED" {
		t.Fatalf("Verdict = %q, want ADMITTED", report.Verdict)
	}
	if report.FindingID != "finding-age-membrane-valid" {
		t.Fatalf("FindingID = %q", report.FindingID)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".agents", "state", "findings", "finding-age-membrane-valid.json")); err != nil {
		t.Fatalf("expected admitted finding to be written: %v", err)
	}
}

func TestAdmissionCoreDefaultsDestination(t *testing.T) {
	root := repoRoot(t)
	tmp := copyStateSchemasAndFixtures(t, root)
	findingBytes := readFixture(t, tmp, "valid-finding.json")

	report, err := AdmitFinding(context.Background(), findingBytes, AdmissionRequest{
		SchemaVersion: 1,
		Kind:          AdmissionKind,
		CandidatePath: "schemas/fixtures/state-memory/valid-finding.json",
		OperatorID:    "codex:validator-b",
		Reason:        "unit test admission",
	}, AdmissionOptions{
		Root:   tmp,
		Now:    mustParseTime(t, "2026-06-20T00:00:00Z"),
		MaxAge: 30 * 24 * time.Hour,
		Write:  false,
	})
	if err != nil {
		t.Fatalf("AdmitFinding(default destination): %v", err)
	}
	if report.Destination != ".agents/state/findings/finding-age-membrane-valid.json" {
		t.Fatalf("Destination = %q", report.Destination)
	}
	if report.Wrote {
		t.Fatal("dry admission with Write=false reported Wrote=true")
	}
}

func TestAdmissionCoreRejectsSelfReview(t *testing.T) {
	req, finding := validAdmissionSubject(t)
	finding.Source.AuthorID = "codex:same"
	finding.Review.ReviewerID = "codex:same"
	assertAdmissionRejected(t, req, finding, "self-review")
}

func TestAdmissionCoreRejectsStaleFinding(t *testing.T) {
	req, finding := validAdmissionSubject(t)
	finding.UpdatedAt = "2026-01-01T00:00:00Z"
	finding.Review.ReviewedAt = "2026-01-01T01:00:00Z"
	assertAdmissionRejected(t, req, finding, "stale")
}

func TestAdmissionCoreRejectsLeakyFinding(t *testing.T) {
	req, finding := validAdmissionSubject(t)
	finding.Body = "Do not admit this leaked credential: sk-1234567890abcdef"
	assertAdmissionRejected(t, req, finding, "leak")
}

func TestAdmissionCoreRejectsPathEscape(t *testing.T) {
	req, finding := validAdmissionSubject(t)
	req.Destination = ".agents/state/findings/../../outside.json"
	assertAdmissionRejected(t, req, finding, "path escapes")
}

func TestVerifyRepoValidatesSchemasFixturesAndStateStore(t *testing.T) {
	root := repoRoot(t)
	tmp := copyStateSchemasAndFixtures(t, root)

	report, err := VerifyRepo(context.Background(), tmp)
	if err != nil {
		t.Fatalf("VerifyRepo: %v", err)
	}
	if report.Verdict != "PASS" {
		t.Fatalf("Verdict = %q, want PASS; failures=%v", report.Verdict, report.Failures)
	}
	if report.BadFixturesRejected < 3 {
		t.Fatalf("BadFixturesRejected = %d, want >=3", report.BadFixturesRejected)
	}
}

func validAdmissionSubject(t *testing.T) (AdmissionRequest, Finding) {
	t.Helper()
	root := repoRoot(t)
	var finding Finding
	if err := json.Unmarshal(readFixture(t, root, "valid-finding.json"), &finding); err != nil {
		t.Fatal(err)
	}
	req := AdmissionRequest{
		SchemaVersion: 1,
		Kind:          AdmissionKind,
		CandidatePath: "schemas/fixtures/state-memory/valid-finding.json",
		Destination:   ".agents/state/findings/finding-age-membrane-valid.json",
		OperatorID:    "codex:validator-b",
		Reason:        "unit test admission",
	}
	return req, finding
}

func assertAdmissionRejected(t *testing.T, req AdmissionRequest, finding Finding, want string) {
	t.Helper()
	tmp := copyStateSchemasAndFixtures(t, repoRoot(t))
	data, err := json.Marshal(finding)
	if err != nil {
		t.Fatal(err)
	}
	_, err = AdmitFinding(context.Background(), data, req, AdmissionOptions{
		Root:   tmp,
		Now:    mustParseTime(t, "2026-06-20T00:00:00Z"),
		MaxAge: 30 * 24 * time.Hour,
		Write:  true,
	})
	if err == nil {
		t.Fatalf("AdmitFinding accepted invalid case %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want to contain %q", err, want)
	}
}

func readFixture(t *testing.T, root, name string) []byte {
	t.Helper()
	path := filepath.Join(root, "schemas", "fixtures", "state-memory", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func copyStateSchemasAndFixtures(t *testing.T, root string) string {
	t.Helper()
	tmp := t.TempDir()
	copyFile(t, root, tmp, filepath.Join("schemas", "state-finding.v1.schema.json"))
	copyFile(t, root, tmp, filepath.Join("schemas", "state-admission.v1.schema.json"))
	fixtures := filepath.Join(root, "schemas", "fixtures", "state-memory")
	entries, err := os.ReadDir(fixtures)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		rel := filepath.Join("schemas", "fixtures", "state-memory", entry.Name())
		copyFile(t, root, tmp, rel)
	}
	return tmp
}

func copyFile(t *testing.T, fromRoot, toRoot, rel string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fromRoot, rel))
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(toRoot, rel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}
