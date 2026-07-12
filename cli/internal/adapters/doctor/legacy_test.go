package doctor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
	"github.com/boshu2/agentops/cli/internal/reviewerhealth"
)

type fakeReviewerProbe map[string]reviewerhealth.ProbeResult

func (probe fakeReviewerProbe) Check(_ context.Context, reviewer reviewerhealth.Reviewer, _ time.Duration) reviewerhealth.ProbeResult {
	return probe[reviewer.Name]
}

func TestLegacyChecksIncludesEveryDoctorSafetySection(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("HOME", t.TempDir())
	reviewers := reviewerhealth.NewService(reviewerhealth.DefaultCatalog(), fakeReviewerProbe{
		"codex": {Status: "pass", Detail: "reachable", Live: true},
		"agy":   {Status: "warn", Detail: "missing"},
	})
	adapter := LegacyChecks{
		ToolVersion: "1.0.0", IndexDir: ".agents/index", IndexFile: "index.json", Reviewers: reviewers,
		WorkingDir:  func() (string, error) { return root, nil },
		LedgerPath:  func() string { return filepath.Join(root, "ledger.jsonl") },
		Environment: func() []string { return nil },
		LookPath:    func(string) (string, error) { return "", errors.New("missing") },
		Now:         time.Now,
	}
	checks := adapter.Checks(context.Background())
	byName := make(map[string]string, len(checks))
	for _, check := range checks {
		byName[check.Name] = check.Status
	}
	for _, name := range []string{
		"Plugin", "Codex Sync", "Skill Integrity", "Reviewer: codex", "Reviewer: agy",
		"Cross-Family Review", "Binary Freshness", "Ledger Health", "LAW-0 Guard",
	} {
		if _, ok := byName[name]; !ok {
			t.Errorf("LegacyChecks missing %q: %v", name, byName)
		}
	}
	if byName["Reviewer: codex"] != "pass" || byName["Reviewer: agy"] != "warn" || byName["Cross-Family Review"] != "pass" {
		t.Fatalf("reviewer integration = %v", byName)
	}
}

func TestCrossFamilyCheck(t *testing.T) {
	for _, test := range []struct {
		name, status, detail string
		live                 []string
	}{
		{name: "both", live: []string{"codex", "agy"}, status: "pass", detail: "live families: codex, agy"},
		{name: "one", live: []string{"agy"}, status: "pass", detail: "live families: agy"},
		{name: "none", status: "warn", detail: "npm install -g @openai/codex"},
	} {
		t.Run(test.name, func(t *testing.T) {
			check := CrossFamilyCheck(test.live)
			if check.Status != test.status || !strings.Contains(check.Detail, test.detail) {
				t.Fatalf("check = %+v", check)
			}
		})
	}
}

func makeFakeAgentopsRepo(t *testing.T, declared string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "cli", "cmd", "ao")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cli", "go.mod"), []byte(agentopsModuleLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nvar version = \""+declared+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestBinaryFreshnessCheck(t *testing.T) {
	root := makeFakeAgentopsRepo(t, "9.9.9")
	if check := BinaryFreshnessCheck(filepath.Join(root, "cli", "cmd"), "9.9.9"); check.Status != "pass" {
		t.Fatalf("matching check = %+v", check)
	}
	check := BinaryFreshnessCheck(root, "1.0.0")
	for _, want := range []string{"1.0.0", "9.9.9", "scripts/preflight-uat-binary.sh", "brew upgrade agentops"} {
		if check.Status != "warn" || !strings.Contains(check.Detail, want) {
			t.Fatalf("stale check lacks %q: %+v", want, check)
		}
	}
	if check := BinaryFreshnessCheck(t.TempDir(), "1.0.0"); check.Status != "pass" || !strings.Contains(check.Detail, "outside the agentops repo") {
		t.Fatalf("outside-repo check = %+v", check)
	}
}

func appendFixtureEdges(t *testing.T, path string, count int) {
	t.Helper()
	store := provenancegraph.NewStore(path)
	for index := range count {
		if _, err := store.Append(provenancegraph.Edge{
			FromID: fmt.Sprintf("age-test.%d", index), FromType: "bead", ToID: "abc1234", ToType: "commit",
			Relation: "wasGeneratedBy", TrustTier: "authored", TS: time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			t.Fatalf("append edge %d: %v", index, err)
		}
	}
}

func TestCheckLedgerHealthIntactMissingAndTampered(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "ledger.jsonl")
	if check := CheckLedgerHealth(missing, time.Now); check.Status != "pass" || !strings.Contains(check.Detail, "no ledger records yet") {
		t.Fatalf("missing ledger = %+v", check)
	}

	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	appendFixtureEdges(t, path, 2)
	if check := CheckLedgerHealth(path, time.Now); check.Status != "pass" || !strings.Contains(check.Detail, "2 records") || !strings.Contains(check.Detail, "newest record") {
		t.Fatalf("intact ledger = %+v", check)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(data, []byte(`"to_id":"abc1234"`), []byte(`"to_id":"abc1235"`), 1)
	if bytes.Equal(tampered, data) {
		t.Fatal("fixture did not contain to_id")
	}
	if err := os.WriteFile(path, tampered, 0o644); err != nil {
		t.Fatal(err)
	}
	check := CheckLedgerHealth(path, time.Now)
	if check.Status != "fail" || !check.Required || !strings.Contains(check.Detail, "chain breaks at line 1") || !strings.Contains(check.Detail, "ao provenance verify") {
		t.Fatalf("tampered ledger = %+v", check)
	}
}

func TestCheckLaw0Guard(t *testing.T) {
	for _, test := range []struct {
		name, status, detail string
		environment          []string
	}{
		{name: "clean", environment: []string{"PAWL_NO_SERVICE=1"}, status: "pass", detail: "no reviewer path configured"},
		{name: "empty", status: "pass", detail: "no reviewer path configured"},
		{name: "pawl print flag", environment: []string{"PAWL_REVIEWER_CMD=claude" + " -p review"}, status: "fail", detail: "unset PAWL_REVIEWER_CMD"},
		{name: "reviewer print word", environment: []string{"MY_REVIEWER_BIN=claude" + " --print"}, status: "fail", detail: "unset MY_REVIEWER_BIN"},
		{name: "unscoped ignored", environment: []string{"SHELL_HISTORY=claude -p review"}, status: "pass", detail: "no reviewer path configured"},
	} {
		t.Run(test.name, func(t *testing.T) {
			check := CheckLaw0Guard(test.environment)
			if check.Status != test.status || !check.Required || !strings.Contains(check.Detail, test.detail) {
				t.Fatalf("check = %+v", check)
			}
		})
	}
}

func TestBinaryFreshnessRejectsSymlinkedRepoMarkers(t *testing.T) {
	root := t.TempDir()
	external := filepath.Join(t.TempDir(), "go.mod")
	if err := os.WriteFile(external, []byte(agentopsModuleLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "cli", "go.mod")); err != nil {
		t.Fatal(err)
	}
	if _, ok := FindAgentopsRepoRoot(root); ok {
		t.Fatal("symlinked go.mod identified an AgentOps checkout")
	}
}

func TestRepoDeclaredVersionRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cli", "cmd", "ao")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, (1<<16)+100)
	copy(body[(1<<16)+1:], `var version = "forged"`)
	if err := os.WriteFile(filepath.Join(path, "main.go"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(path, "..", "..", ".."))
	if _, ok := RepoDeclaredVersion(root); ok {
		t.Fatal("oversized main.go yielded a version")
	}
}
