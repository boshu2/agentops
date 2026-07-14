// practices: [tdd, sre]
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/trackerexec"
)

func TestReconcileUsesResolvedTrackerContext(t *testing.T) {
	repo := t.TempDir()
	if output, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("initialize git repository: %v: %s", err, output)
	}
	nested := filepath.Join(repo, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	tracePath := filepath.Join(t.TempDir(), "tracker.trace")
	stub := `#!/bin/sh
printf 'binary=%s|pwd=%s|beads=%s|dolt=%s|argv=%s\n' "${0##*/}" "$(pwd -P)" "${BEADS_DIR-<unset>}" "${BEADS_DOLT_AUTO_START-<unset>}" "$*" > "$TRACKER_TRACE"
printf '[]\n'
`
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	t.Setenv("TRACKER_TRACE", tracePath)
	t.Setenv("AGENTOPS_TRACKER", "br")
	ambientLedger := filepath.Join(t.TempDir(), "ambient-ledger")
	t.Setenv("BEADS_DIR", ambientLedger)
	t.Setenv("BEADS_DOLT_AUTO_START", "1")
	originalLookPath := trackerLookPath
	trackerLookPath = exec.LookPath
	t.Cleanup(func() { trackerLookPath = originalLookPath })

	if _, err := defaultReconcileRunInDir(context.Background(), nested, "bd", "list", "--limit", "1", "--json"); err != nil {
		t.Fatalf("run reconcile BD query: %v", err)
	}
	physicalRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read tracker trace: %v", err)
	}
	want := "binary=bd|pwd=" + physicalRepo + "|beads=<unset>|dolt=0|argv=list --limit 1 --json"
	if got := strings.TrimSpace(string(data)); got != want {
		t.Fatalf("tracker trace = %q, want %q", got, want)
	}

	if err := os.Remove(tracePath); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := defaultReconcileRunInDir(canceled, nested, "bd", "ready", "-n", "20", "--json"); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled run error = %T %v, want context cancellation", err, err)
	}
	if _, err := os.Stat(tracePath); !os.IsNotExist(err) {
		t.Fatalf("pre-canceled context launched tracker: %v", err)
	}

	exitStub := `#!/bin/sh
exit 23
`
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(exitStub), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = defaultReconcileRunInDir(context.Background(), nested, "bd", "list", "--json")
	var exitErr *trackerexec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 23 {
		t.Fatalf("tracker error = %T %v, want *trackerexec.ExitError(23)", err, err)
	}
}

func TestReconcileFindsReleaseTagFailureAndOpenReleaseBeads(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 5, 26, 20, 0, 0, 0, time.UTC)
	writeRecentAgentEvidence(t, tmp, now)

	report := buildReconcileReport(context.Background(), reconcileOptions{
		Cwd:   tmp,
		Limit: 80,
		Since: 48 * time.Hour,
		Now:   now,
		Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			key := name + " " + strings.Join(args, " ")
			switch {
			case key == "git rev-parse HEAD":
				return []byte("f077c6bfbc21d0e9db466364f9b852b666b1324f\n"), nil
			case key == "git rev-parse --abbrev-ref HEAD":
				return []byte("main\n"), nil
			case key == "git rev-parse --abbrev-ref --symbolic-full-name @{u}":
				return []byte("origin/main\n"), nil
			case key == "git rev-parse --show-toplevel":
				return []byte(tmp + "\n"), nil
			case key == "git ls-remote origin refs/heads/main":
				return []byte("f077c6bfbc21d0e9db466364f9b852b666b1324f\trefs/heads/main\n"), nil
			case key == "git status --porcelain":
				return []byte(""), nil
			case key == "git rev-list --left-right --count HEAD...origin/main":
				return []byte("0\t0\n"), nil
			case strings.HasPrefix(key, "gh run list --branch main"):
				return []byte(`[{"databaseId":1,"workflowName":"Validate","status":"completed","conclusion":"success","headSha":"f077","displayTitle":"green","url":"https://example.test/main"}]`), nil
			case strings.HasPrefix(key, "gh release view"):
				return []byte(`{"tagName":"v3.0.1","name":"v3.0.1","publishedAt":"2026-05-25T13:02:06Z","url":"https://example.test/release","targetCommitish":"main"}`), nil
			case strings.HasPrefix(key, "gh run list --branch v3.0.1 --workflow Validate"):
				return []byte(`[{"databaseId":2,"workflowName":"Validate","status":"completed","conclusion":"failure","headSha":"tagsha","displayTitle":"tag validate","url":"https://example.test/tag"}]`), nil
			case strings.HasPrefix(key, "bd list"):
				return []byte(`[
					{"id":"soc-2ul4t","title":"3.0: release mechanics + tag v3.0.0","status":"open","priority":1},
					{"id":"soc-3csze","title":"Verify installer end-to-end","status":"open","priority":0}
				]`), nil
			case strings.HasPrefix(key, "bd ready"):
				return []byte(`[{"id":"soc-3csze","title":"Verify installer end-to-end","status":"open","priority":0}]`), nil
			default:
				t.Fatalf("unexpected command: %s", key)
			}
			return nil, nil
		},
	})

	if report.OverallStatus != "needs_attention" {
		t.Fatalf("OverallStatus = %q, want needs_attention", report.OverallStatus)
	}
	if report.Summary.High != 1 {
		t.Fatalf("High findings = %d, want 1: %+v", report.Summary.High, report.Findings)
	}
	releaseFinding := findFinding(t, report, "release-tag-validate-not-green")
	if !strings.Contains(releaseFinding.NextAction, "newer release tag") {
		t.Fatalf("release next action should offer superseding tag path, got %q", releaseFinding.NextAction)
	}
	assertFinding(t, report, "beads-release-stale")
	assertFinding(t, report, "beads-ready-p0")
	if !report.Agents.Available || report.Agents.RecentFiles == 0 {
		t.Fatalf("expected recent .agents evidence, got %+v", report.Agents)
	}
}

// TestReconcileMainCIMissingIsNotNeedsAttention pins age-yyse: under the 3.0
// CI-off model main is NOT CI-gated (validate.yml is a tag/PR/manual backstop;
// the local pre-push gate is release authority). A missing main Validate run is
// therefore informational (low), not HIGH — reconcile must NOT report
// needs_attention solely because main has no GitHub Actions run.
func TestReconcileMainCIMissingIsNotNeedsAttention(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 6, 24, 20, 0, 0, 0, time.UTC)
	writeRecentAgentEvidence(t, tmp, now)

	report := buildReconcileReport(context.Background(), reconcileOptions{
		Cwd:   tmp,
		Limit: 80,
		Since: 48 * time.Hour,
		Now:   now,
		Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			key := name + " " + strings.Join(args, " ")
			switch {
			case key == "git rev-parse HEAD":
				return []byte("f077c6bfbc21d0e9db466364f9b852b666b1324f\n"), nil
			case key == "git rev-parse --abbrev-ref HEAD":
				return []byte("main\n"), nil
			case key == "git rev-parse --abbrev-ref --symbolic-full-name @{u}":
				return []byte("origin/main\n"), nil
			case key == "git rev-parse --show-toplevel":
				return []byte(tmp + "\n"), nil
			case key == "git ls-remote origin refs/heads/main":
				return []byte("f077c6bfbc21d0e9db466364f9b852b666b1324f\trefs/heads/main\n"), nil
			case key == "git status --porcelain":
				return []byte(""), nil
			case key == "git rev-list --left-right --count HEAD...origin/main":
				return []byte("0\t0\n"), nil
			case strings.HasPrefix(key, "gh run list --branch main"):
				return []byte(`[]`), nil // no main Validate run — expected under CI-off
			case strings.HasPrefix(key, "gh release view"):
				return []byte(`{"tagName":"v3.1.0","name":"v3.1.0","publishedAt":"2026-06-20T13:02:06Z","url":"https://example.test/release","targetCommitish":"main"}`), nil
			case strings.HasPrefix(key, "gh run list --branch v3.1.0 --workflow Validate"):
				return []byte(`[{"databaseId":2,"workflowName":"Validate","status":"completed","conclusion":"success","headSha":"tagsha","displayTitle":"tag validate","url":"https://example.test/tag"}]`), nil // green release tag
			case strings.HasPrefix(key, "bd list"):
				return []byte(`[]`), nil
			case strings.HasPrefix(key, "bd ready"):
				return []byte(`[]`), nil
			default:
				t.Fatalf("unexpected command: %s", key)
			}
			return nil, nil
		},
	})

	ci := findFinding(t, report, "ci-main-validate-missing")
	if ci.Severity != "low" {
		t.Fatalf("ci-main-validate-missing severity = %q, want low (CI-off model)", ci.Severity)
	}
	// Pin the EXACT documented contract: the low CI finding is present AND drives
	// green_with_warnings — not needs_attention (regression up), and not plain
	// green (regression down: the finding silently dropped).
	if report.OverallStatus != "green_with_warnings" {
		t.Fatalf("OverallStatus = %q, want green_with_warnings (one low CI finding, no high/medium); findings: %+v", report.OverallStatus, report.Findings)
	}
	if report.Summary.High != 0 || report.Summary.Medium != 0 {
		t.Fatalf("High=%d Medium=%d, want 0/0 (clean main + green release tag); the only finding must be the low main-CI note: %+v", report.Summary.High, report.Summary.Medium, report.Findings)
	}
	if report.Summary.Low != 1 {
		t.Fatalf("Low findings = %d, want exactly 1 (the main-CI note): %+v", report.Summary.Low, report.Findings)
	}
}

func TestReconcileGracefullyHandlesUnavailableExternalTools(t *testing.T) {
	report := buildReconcileReport(context.Background(), reconcileOptions{
		Cwd:   t.TempDir(),
		Limit: 10,
		Since: time.Hour,
		Now:   time.Date(2026, 5, 26, 20, 0, 0, 0, time.UTC),
		Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			if name == "git" && strings.Join(args, " ") == "rev-parse HEAD" {
				return []byte("abc123\n"), nil
			}
			if name == "git" {
				return []byte(""), nil
			}
			return nil, errors.New(name + " unavailable")
		},
	})

	if report.CI.Available {
		t.Fatal("CI should be unavailable")
	}
	if report.Release.Available {
		t.Fatal("release should be unavailable")
	}
	if report.Beads.Available {
		t.Fatal("beads should be unavailable")
	}
	assertFinding(t, report, "ci-unavailable")
	assertFinding(t, report, "beads-unavailable")
	assertFinding(t, report, "agents-unavailable")
}

func TestRunReconcileWithOptionsEmitsJSON(t *testing.T) {
	var out bytes.Buffer
	err := runReconcileWithOptions(context.Background(), reconcileOptions{
		Cwd:    t.TempDir(),
		Output: "json",
		Writer: &out,
		Limit:  10,
		Since:  time.Hour,
		Now:    time.Date(2026, 5, 26, 20, 0, 0, 0, time.UTC),
		Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			if name == "git" && strings.Join(args, " ") == "rev-parse HEAD" {
				return []byte("abc123\n"), nil
			}
			if name == "git" {
				return []byte(""), nil
			}
			return nil, errors.New(name + " unavailable")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var report reconcileReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if report.SchemaVersion != reconcileSchemaVersion {
		t.Fatalf("schema version = %q", report.SchemaVersion)
	}
}

func TestReconcileHumanOutputLeadsWithStatus(t *testing.T) {
	var out bytes.Buffer
	writeReconcileHuman(&out, reconcileReport{
		OverallStatus: "needs_reconciliation",
		Git:           reconcileGitStatus{Available: true, Branch: "main", Head: "abcdef1234567890"},
		Findings: []reconcileFinding{{
			ID: "beads-release-stale", Severity: "medium", Surface: "beads",
			Title: "Open release-like beads may misrepresent shipped reality",
		}},
	})
	text := out.String()
	if !strings.Contains(text, "Status: needs_reconciliation") {
		t.Fatalf("missing status line:\n%s", text)
	}
	if !strings.Contains(text, "[MEDIUM] beads") {
		t.Fatalf("missing finding line:\n%s", text)
	}
}

func writeRecentAgentEvidence(t *testing.T, root string, when time.Time) {
	t.Helper()
	path := filepath.Join(root, ".agents", "rpi", "phase-3-summary.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

func assertFinding(t *testing.T, report reconcileReport, id string) {
	t.Helper()
	_ = findFinding(t, report, id)
}

func findFinding(t *testing.T, report reconcileReport, id string) reconcileFinding {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.ID == id {
			return finding
		}
	}
	t.Fatalf("finding %q not found in %+v", id, report.Findings)
	return reconcileFinding{}
}
