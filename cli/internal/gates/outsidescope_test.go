package gates

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestParsePorcelainZ(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want UncommittedPaths
	}{
		{"empty output", "", UncommittedPaths{}},
		{"unstaged edit", " M a.go\x00", UncommittedPaths{Modified: []string{"a.go"}}},
		{"staged add and untracked file", "A  new.go\x00?? scratch.txt\x00",
			UncommittedPaths{Modified: []string{"new.go"}, Untracked: []string{"scratch.txt"}}},
		{"rename skips its source field", "R  to.go\x00from.go\x00 M b.go\x00",
			UncommittedPaths{Modified: []string{"to.go", "b.go"}}},
		{"path with space and newline is kept verbatim", " M dir/a b\n.go\x00",
			UncommittedPaths{Modified: []string{"dir/a b\n.go"}}},
		{"untracked directory and ignored entry", "?? newdir/\x00!! build/\x00",
			UncommittedPaths{Untracked: []string{"newdir/"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parsePorcelainZ(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePorcelainZ(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

var outsideScopePassVerdicts = map[ports.GateName]ports.GateVerdict{
	"go":       {Status: ports.GateStatusPass, Reason: "ok"},
	"docs":     {Status: ports.GateStatusPass, Reason: "ok"},
	"always":   {Status: ports.GateStatusPass, Reason: "ok"},
	"fullonly": {Status: ports.GateStatusPass, Reason: "ok"},
}

func writeRepoFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func humanOf(r *Report) string {
	var buf bytes.Buffer
	r.Human(&buf)
	return buf.String()
}

// outsideScopeJSON returns run.uncommitted_outside_scope from a report's JSON,
// and whether the key is present at all.
func outsideScopeJSON(t *testing.T, r *Report) (jsonOutsideScope, bool) {
	t.Helper()
	raw, err := r.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var doc struct {
		Run map[string]json.RawMessage `json:"run"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	field, ok := doc.Run["uncommitted_outside_scope"]
	if !ok {
		return jsonOutsideScope{}, false
	}
	var out jsonOutsideScope
	if err := json.Unmarshal(field, &out); err != nil {
		t.Fatalf("unmarshal uncommitted_outside_scope: %v", err)
	}
	return out, true
}

// TestOrchestratorOutsideScope_RealRepo is the acceptance for the green-that-
// covered-less defect: a fast run routed on scope head printed a clean summary
// while uncommitted edits selected no check. The run must now name those paths,
// and must say nothing extra when the scope already covers the worktree.
func TestOrchestratorOutsideScope_RealRepo(t *testing.T) {
	// HEAD adds cli/new.go; README.md comes from the base commit, so an edit to it
	// is outside scope head.
	root := initRepoWithHead(t, "cli/new.go")
	orch := testOrch(t, sampleRegistry(t), NewGitChangedFiles(root), outsideScopePassVerdicts)
	ctx := context.Background()
	run := func(mode Tier, scope Scope) *Report {
		t.Helper()
		rep, err := orch.Run(ctx, RunOptions{Mode: mode, Scope: scope})
		if err != nil {
			t.Fatalf("Run(%s): %v", scope, err)
		}
		return rep
	}

	clean := run(Fast, ScopeHead)
	if clean.OutsideScope != nil {
		t.Fatalf("clean worktree: OutsideScope = %+v, want nil", clean.OutsideScope)
	}
	if out := humanOf(clean); strings.Contains(out, "not considered by routing") {
		t.Errorf("clean worktree printed a routing note:\n%s", out)
	}
	if _, present := outsideScopeJSON(t, clean); present {
		t.Error("clean worktree: JSON carries uncommitted_outside_scope, want the key omitted")
	}

	writeRepoFile(t, root, "README.md", "edited\n")
	writeRepoFile(t, root, "notes.txt", "scratch\n")
	// An edit to a path already in HEAD's change set is routed, so it is not outside.
	writeRepoFile(t, root, "cli/new.go", "edited\n")

	dirty := run(Fast, ScopeHead)
	if dirty.OutsideScope == nil {
		t.Fatal("dirty worktree on scope head: OutsideScope = nil, want the uncommitted paths")
	}
	if got, want := dirty.OutsideScope.Modified, []string{"README.md"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Modified = %v, want %v", got, want)
	}
	if got, want := dirty.OutsideScope.Untracked, []string{"notes.txt"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Untracked = %v, want %v", got, want)
	}
	wantLine := `not considered by routing: 2 uncommitted paths outside scope head (1 modified, 1 untracked), first: "README.md", "notes.txt"; --scope worktree routes on modified paths; git add untracked paths to route on them` + "\n"
	if out := humanOf(dirty); !strings.Contains(out, wantLine) {
		t.Errorf("human report lacks the routing note.\nwant line: %sgot:\n%s", wantLine, out)
	}
	wire, present := outsideScopeJSON(t, dirty)
	wantWire := jsonOutsideScope{ModifiedCount: 1, UntrackedCount: 1, Examples: []string{"README.md", "notes.txt"}}
	if !present || !reflect.DeepEqual(wire, wantWire) {
		t.Errorf("JSON uncommitted_outside_scope = %+v (present=%v), want %+v", wire, present, wantWire)
	}
	// Reporting only: the note changes neither the selection nor the exit code.
	if !reflect.DeepEqual(ranIDs(dirty), ranIDs(clean)) {
		t.Errorf("selected checks changed: dirty %v, clean %v", ranIDs(dirty), ranIDs(clean))
	}
	if dirty.ExitCode() != 0 || clean.ExitCode() != 0 {
		t.Errorf("exit codes = dirty %d, clean %d, want 0 and 0", dirty.ExitCode(), clean.ExitCode())
	}

	worktree := run(Fast, ScopeWorktree)
	if worktree.OutsideScope == nil {
		t.Fatal("scope worktree with an untracked file: OutsideScope = nil, want the untracked path")
	}
	if len(worktree.OutsideScope.Modified) != 0 {
		t.Errorf("scope worktree: Modified = %v, want none (the scope routes on them)", worktree.OutsideScope.Modified)
	}
	wantWorktree := `not considered by routing: 1 uncommitted path outside scope worktree (0 modified, 1 untracked), first: "notes.txt"; git add untracked paths to route on them` + "\n"
	if out := humanOf(worktree); !strings.Contains(out, wantWorktree) {
		t.Errorf("scope worktree note.\nwant line: %sgot:\n%s", wantWorktree, out)
	}

	if full := run(Full, ScopeHead); full.OutsideScope != nil {
		t.Errorf("full mode: OutsideScope = %+v, want nil (every check runs)", full.OutsideScope)
	}

	runDiscoveryGit(t, root, "add", "notes.txt")
	if covered := run(Fast, ScopeWorktree); covered.OutsideScope != nil {
		t.Errorf("scope worktree after git add: OutsideScope = %+v, want nil", covered.OutsideScope)
	}
}

// TestOrchestratorOutsideScope_UnbornHead proves the note works before the
// first commit, where scope staged is the documented way to run.
func TestOrchestratorOutsideScope_UnbornHead(t *testing.T) {
	root := initRepoNoCommits(t, true)
	writeRepoFile(t, root, "later.txt", "not staged\n")
	orch := testOrch(t, sampleRegistry(t), NewGitChangedFiles(root), outsideScopePassVerdicts)
	rep, err := orch.Run(context.Background(), RunOptions{Mode: Fast, Scope: ScopeStaged})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.OutsideScope == nil {
		t.Fatal("OutsideScope = nil, want the untracked path")
	}
	if rep.OutsideScope.Err != "" {
		t.Fatalf("OutsideScope.Err = %q, want the worktree listed", rep.OutsideScope.Err)
	}
	if got, want := rep.OutsideScope.Untracked, []string{"later.txt"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Untracked = %v, want %v", got, want)
	}
}

// TestPlanOutsideScope proves the dry-run projection carries the same note a
// real run prints, so a plan cannot look complete where a run would warn.
func TestPlanOutsideScope(t *testing.T) {
	root := initRepoWithHead(t, "cli/new.go")
	writeRepoFile(t, root, "README.md", "edited\n")
	orch := testOrch(t, sampleRegistry(t), NewGitChangedFiles(root), outsideScopePassVerdicts)
	plan, err := orch.Plan(context.Background(), RunOptions{Mode: Fast, Scope: ScopeHead})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	var buf bytes.Buffer
	plan.Human(&buf)
	wantLine := `not considered by routing: 1 uncommitted path outside scope head (1 modified, 0 untracked), first: "README.md"; --scope worktree routes on modified paths` + "\n"
	if !strings.Contains(buf.String(), wantLine) {
		t.Errorf("plan lacks the routing note.\nwant line: %sgot:\n%s", wantLine, buf.String())
	}
	raw, err := plan.JSON()
	if err != nil {
		t.Fatalf("plan JSON: %v", err)
	}
	var doc struct {
		Plan struct {
			Outside *jsonOutsideScope `json:"uncommitted_outside_scope"`
		} `json:"plan"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal plan: %v", err)
	}
	want := &jsonOutsideScope{ModifiedCount: 1, Examples: []string{"README.md"}}
	if !reflect.DeepEqual(doc.Plan.Outside, want) {
		t.Errorf("plan uncommitted_outside_scope = %+v, want %+v", doc.Plan.Outside, want)
	}
}

// unlistableFiles routes normally but cannot list the worktree.
type unlistableFiles struct{ fakeFiles }

func (unlistableFiles) Uncommitted(context.Context) (UncommittedPaths, error) {
	return UncommittedPaths{}, errors.New("status failed")
}

// TestOrchestratorOutsideScope_UnlistableWorktreeReadsUnknown pins the
// fail-visible rule: when the worktree cannot be inspected the report says the
// coverage is unknown. It must never read as "nothing outside the scope", and it
// must not turn a passing run into a failing one.
func TestOrchestratorOutsideScope_UnlistableWorktreeReadsUnknown(t *testing.T) {
	orch := testOrch(t, sampleRegistry(t), unlistableFiles{fakeFiles{files: []string{"cli/x.go"}}}, outsideScopePassVerdicts)
	rep, err := orch.Run(context.Background(), RunOptions{Mode: Fast, Scope: ScopeHead})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.OutsideScope == nil || rep.OutsideScope.Err != "status failed" {
		t.Fatalf("OutsideScope = %+v, want Err %q", rep.OutsideScope, "status failed")
	}
	wantLine := "routing coverage unknown: could not list uncommitted paths (status failed)\n"
	if out := humanOf(rep); !strings.Contains(out, wantLine) {
		t.Errorf("human report lacks the unknown-coverage line.\nwant line: %sgot:\n%s", wantLine, out)
	}
	wire, present := outsideScopeJSON(t, rep)
	if !present || wire.Error != "status failed" {
		t.Errorf("JSON uncommitted_outside_scope = %+v (present=%v), want error %q", wire, present, "status failed")
	}
	if rep.ExitCode() != 0 {
		t.Errorf("ExitCode = %d, want 0: the note is reporting only", rep.ExitCode())
	}
}

// TestOrchestratorOutsideScope_PortWithoutListingHasNoNote covers a
// ChangedFilesPort that cannot list uncommitted paths at all.
func TestOrchestratorOutsideScope_PortWithoutListingHasNoNote(t *testing.T) {
	orch := testOrch(t, sampleRegistry(t), fakeFiles{files: []string{"cli/x.go"}}, outsideScopePassVerdicts)
	rep, err := orch.Run(context.Background(), RunOptions{Mode: Fast, Scope: ScopeHead})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.OutsideScope != nil {
		t.Errorf("OutsideScope = %+v, want nil", rep.OutsideScope)
	}
}
