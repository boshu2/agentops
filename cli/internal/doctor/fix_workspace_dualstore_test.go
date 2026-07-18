package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// knowledgeOrphanedLearningsFixerID is the knowledge-subsystem fixer that owns
// the dual-store repair; the dual-store finding must defer to it verbatim.
const knowledgeOrphanedLearningsFixerID = "fm-knowledge-orphaned-flywheel-learnings"

// dualStoreFixtureEnv builds a repo whose .agents holds BOTH learnings stores
// populated: two files (one nested) in the legacy store, one in the canonical.
func dualStoreFixtureEnv(t *testing.T) (*DetectEnv, string) {
	t.Helper()
	repo := t.TempDir()
	agents := filepath.Join(repo, ".agents")
	now := time.Now()
	writeWorkspaceFile(t, filepath.Join(agents, "learnings", "a.md"), "legacy-a", now)
	writeWorkspaceFile(t, filepath.Join(agents, "learnings", "sub", "b.txt"), "legacy-b", now)
	writeWorkspaceFile(t, filepath.Join(agents, "ao", "learnings", "c.md"), "canonical-c", now)
	return &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}, repo
}

func TestWorkspaceDualStore_DetectFinding(t *testing.T) {
	env, _ := dualStoreFixtureEnv(t)

	findings, err := workspaceDualStoreDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	f := findings[0]
	if f.ID != "fm-ws-dual-store" {
		t.Errorf("finding ID = %q, want fm-ws-dual-store", f.ID)
	}
	if f.Subsystem != "workspace" {
		t.Errorf("subsystem = %q, want workspace", f.Subsystem)
	}
	if f.Severity != "P2" {
		t.Errorf("severity = %q, want P2", f.Severity)
	}
	if f.Confidence != 1.0 {
		t.Errorf("confidence = %v, want 1.0", f.Confidence)
	}
	// Evidence carries the exact transitive file counts of both stores.
	if want := "dual learnings stores: 2 file(s) in .agents/learnings, 1 in .agents/ao/learnings"; f.Title != want {
		t.Errorf("title = %q, want %q", f.Title, want)
	}
	if want := "transitive regular-file counts: .agents/learnings=2 .agents/ao/learnings=1"; f.Evidence.Query != want {
		t.Errorf("evidence query = %q, want %q", f.Evidence.Query, want)
	}
	// Report-only: the knowledge fixer owns the repair, so the finding must
	// defer to it and never claim auto-fixability of its own.
	if f.Remediation.AutoFixable {
		t.Error("dual-store finding must not be auto-fixable")
	}
	if want := "ao doctor --fix --only " + knowledgeOrphanedLearningsFixerID; f.Remediation.Command != want {
		t.Errorf("remediation command = %q, want %q", f.Remediation.Command, want)
	}
	if want := "ao doctor explain " + knowledgeOrphanedLearningsFixerID; f.Remediation.ExplainCommand != want {
		t.Errorf("explain command = %q, want %q", f.Remediation.ExplainCommand, want)
	}
}

// TestWorkspaceDualStore_DetectSelection is the table-driven firing matrix:
// the finding requires BOTH stores present as directories with content.
func TestWorkspaceDualStore_DetectSelection(t *testing.T) {
	tests := []struct {
		name           string
		legacyFiles    []string // paths under .agents/learnings; nil = dir absent
		legacyEmptyDir bool     // create .agents/learnings with no files
		canonFiles     []string // paths under .agents/ao/learnings; nil = dir absent
		canonEmptyDir  bool
		want           int
	}{
		{"both populated", []string{"a.md"}, false, []string{"c.md"}, false, 1},
		{"legacy empty dir", nil, true, []string{"c.md"}, false, 0},
		{"canonical empty dir", []string{"a.md"}, false, nil, true, 0},
		{"ao/learnings absent", []string{"a.md"}, false, nil, false, 0},
		{"legacy absent", nil, false, []string{"c.md"}, false, 0},
		{"no .agents at all", nil, false, nil, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := t.TempDir()
			agents := filepath.Join(repo, ".agents")
			now := time.Now()
			for _, n := range tt.legacyFiles {
				writeWorkspaceFile(t, filepath.Join(agents, "learnings", n), "x", now)
			}
			if tt.legacyEmptyDir {
				if err := os.MkdirAll(filepath.Join(agents, "learnings"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for _, n := range tt.canonFiles {
				writeWorkspaceFile(t, filepath.Join(agents, "ao", "learnings", n), "y", now)
			}
			if tt.canonEmptyDir {
				if err := os.MkdirAll(filepath.Join(agents, "ao", "learnings"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}
			findings, err := workspaceDualStoreDetector{}.Detect(env)
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			if len(findings) != tt.want {
				t.Fatalf("findings = %d, want %d", len(findings), tt.want)
			}
		})
	}
}

// nestedTreeFixtureEnv builds a repo with two real nested runtime trees
// (cli/.agents, tools/.agents), every skip-listed sibling carrying a decoy
// .agents with content, an empty nested .agents, and the root .agents itself.
func nestedTreeFixtureEnv(t *testing.T) (*DetectEnv, string) {
	t.Helper()
	repo := t.TempDir()
	now := time.Now()
	// Real nested trees (ReadDir order: cli before tools).
	writeWorkspaceFile(t, filepath.Join(repo, "cli", ".agents", "state.json"), "nested-cli", now)
	writeWorkspaceFile(t, filepath.Join(repo, "tools", ".agents", "sub", "log.txt"), "nested-tools", now)
	// Skip-listed children: never flagged even with populated .agents inside.
	for _, skip := range []string{".git", "node_modules", "vendor", ".claude"} {
		writeWorkspaceFile(t, filepath.Join(repo, skip, ".agents", "decoy.txt"), "decoy", now)
	}
	// The root runtime tree itself is not a nested tree.
	writeWorkspaceFile(t, filepath.Join(repo, ".agents", "handoff", "h.md"), "root", now)
	// An empty nested .agents dir is not flagged.
	if err := os.MkdirAll(filepath.Join(repo, "docs", ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A child with no .agents at all.
	writeWorkspaceFile(t, filepath.Join(repo, "scripts", "run.sh"), "#!/bin/sh", now)
	return &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}, repo
}

func TestWorkspaceNestedTree_DetectFindings(t *testing.T) {
	env, _ := nestedTreeFixtureEnv(t)

	findings, err := workspaceNestedTreeDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("findings = %d, want 2 (one per nested tree)", len(findings))
	}
	wantFiles := []string{filepath.Join("cli", ".agents"), filepath.Join("tools", ".agents")}
	for i, f := range findings {
		if f.ID != "fm-ws-nested-tree" {
			t.Errorf("finding[%d] ID = %q, want fm-ws-nested-tree", i, f.ID)
		}
		if f.Subsystem != "workspace" {
			t.Errorf("finding[%d] subsystem = %q, want workspace", i, f.Subsystem)
		}
		if f.Severity != "P2" {
			t.Errorf("finding[%d] severity = %q, want P2", i, f.Severity)
		}
		if f.Evidence.File != wantFiles[i] {
			t.Errorf("finding[%d] evidence file = %q, want %q", i, f.Evidence.File, wantFiles[i])
		}
		if f.Remediation.AutoFixable {
			t.Errorf("finding[%d] must not be auto-fixable (merging is a human call)", i)
		}
		if f.Remediation.Command == "" {
			t.Errorf("finding[%d] remediation command is empty; want a manual-review hint", i)
		}
		if !strings.Contains(f.Remediation.Command, wantFiles[i]) {
			t.Errorf("finding[%d] remediation command %q does not name %q", i, f.Remediation.Command, wantFiles[i])
		}
		if want := "ao doctor explain fm-ws-nested-tree"; f.Remediation.ExplainCommand != want {
			t.Errorf("finding[%d] explain command = %q, want %q", i, f.Remediation.ExplainCommand, want)
		}
	}
	// Each nested tree's file count is exact in the title.
	if want := "nested runtime tree " + filepath.Join("cli", ".agents") + " holds 1 file(s)"; findings[0].Title != want {
		t.Errorf("title = %q, want %q", findings[0].Title, want)
	}
}

func TestWorkspaceNestedTree_DetectCleanRepo(t *testing.T) {
	repo := t.TempDir()
	writeWorkspaceFile(t, filepath.Join(repo, ".agents", "handoff", "h.md"), "root", time.Now())
	env := &DetectEnv{RepoRoot: repo, CWD: repo, HomeDir: t.TempDir(), Logger: os.Stderr}
	findings, err := workspaceNestedTreeDetector{}.Detect(env)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("clean repo produced %d findings, want 0", len(findings))
	}
}

// TestWorkspaceDualStoreNested_FixPerformsZeroMutations proves the engine
// tolerates both fixerless findings: a full detect+fix pass scoped to the two
// IDs finds them, mutates NOTHING, and reports them unfixed (fix_partial).
func TestWorkspaceDualStoreNested_FixPerformsZeroMutations(t *testing.T) {
	env, repo := dualStoreFixtureEnv(t)
	writeWorkspaceFile(t, filepath.Join(repo, "cli", ".agents", "state.json"), "nested-cli", time.Now())

	rep, err := Fix(Options{
		RepoRoot: repo, CWD: repo, HomeDir: env.HomeDir, ToolVersion: "2.0.0",
		Only: []string{"fm-ws-dual-store", "fm-ws-nested-tree"},
	})
	if err != nil {
		t.Fatalf("Fix: %v", err)
	}
	if got := len(rep.Findings); got != 2 {
		t.Fatalf("fix pass findings = %d, want 2", got)
	}
	if rep.ActionsTaken != 0 {
		t.Fatalf("ActionsTaken = %d, want 0 (no fixer registered)", rep.ActionsTaken)
	}
	// Findings present, none fixable/fixed → fix_partial, never a mutation.
	if rep.ExitCode != ExitFixPartial {
		t.Fatalf("exit code = %d, want %d (fix_partial)", rep.ExitCode, ExitFixPartial)
	}
	// actions.jsonl exists but recorded zero actions.
	recs, err := readActions(filepath.Join(rep.RunDir, "actions.jsonl"))
	if err != nil {
		t.Fatalf("readActions: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("action records = %d, want 0", len(recs))
	}
	// Every fixture file is untouched, byte for byte.
	for path, content := range map[string]string{
		filepath.Join(repo, ".agents", "learnings", "a.md"):         "legacy-a",
		filepath.Join(repo, ".agents", "learnings", "sub", "b.txt"): "legacy-b",
		filepath.Join(repo, ".agents", "ao", "learnings", "c.md"):   "canonical-c",
		filepath.Join(repo, "cli", ".agents", "state.json"):         "nested-cli",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != content {
			t.Errorf("file %s = %q err=%v, want %q untouched", path, got, err, content)
		}
	}
}

func TestWorkspaceDualStoreNested_Registration(t *testing.T) {
	byID := make(map[string]Detector)
	for _, d := range Detectors() {
		byID[d.ID()] = d
	}

	ds, ok := byID["fm-ws-dual-store"]
	if !ok {
		t.Fatal("detector fm-ws-dual-store not registered")
	}
	if ds.Subsystem() != "workspace" {
		t.Errorf("dual-store subsystem = %q, want workspace", ds.Subsystem())
	}
	if ds.Severity() != "P2" {
		t.Errorf("dual-store severity = %q, want P2", ds.Severity())
	}
	if !ds.QuickPath() {
		t.Error("dual-store QuickPath() = false, want true")
	}
	if ds.OnlineRequired() {
		t.Error("dual-store OnlineRequired() = true, want false")
	}

	nt, ok := byID["fm-ws-nested-tree"]
	if !ok {
		t.Fatal("detector fm-ws-nested-tree not registered")
	}
	if nt.Subsystem() != "workspace" {
		t.Errorf("nested-tree subsystem = %q, want workspace", nt.Subsystem())
	}
	if nt.Severity() != "P2" {
		t.Errorf("nested-tree severity = %q, want P2", nt.Severity())
	}
	if nt.QuickPath() {
		t.Error("nested-tree QuickPath() = true, want false")
	}
	if nt.OnlineRequired() {
		t.Error("nested-tree OnlineRequired() = true, want false")
	}

	// Report-only contract: neither ID has a registered fixer.
	for _, id := range []string{"fm-ws-dual-store", "fm-ws-nested-tree"} {
		if fx := FixerByID(id); fx != nil {
			t.Errorf("FixerByID(%q) = %T, want nil (report-only finding)", id, fx)
		}
	}
}
