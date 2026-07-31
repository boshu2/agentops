package gates

import (
	"context"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern, path string
		want          bool
	}{
		{"go.mod", "go.mod", true},
		{"go.mod", "cli/go.mod", false},
		{"cli/**", "cli/cmd/ao/main.go", true},
		{"cli/**", "skills/x", false},
		{"**/*.go", "cli/internal/gates/gates.go", true},
		{"**/*.go", "README.md", false},
		{"*.sh", "scripts/check-x.sh", true},
		{"*.sh", "scripts/x.go", false},
		{"schemas/eval-*", "schemas/eval-outcomes.json", true},
		{"schemas/eval-*", "schemas/swarm.json", false},
	}
	for _, tc := range tests {
		if got := matchGlob(tc.pattern, tc.path); got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestCheck_Affected(t *testing.T) {
	goCheck := Check{ID: "go", Tiers: Fast, Match: []string{"cli/**", "go.mod"}, Backing: "x"}
	always := Check{ID: "a", Tiers: Fast, Backing: "x"} // no Match

	if !goCheck.affected([]string{"cli/cmd/ao/main.go"}) {
		t.Error("go check should be affected by a cli/ change")
	}
	if goCheck.affected([]string{"docs/x.md"}) {
		t.Error("go check should NOT be affected by a docs-only change")
	}
	if !always.affected([]string{"docs/x.md"}) {
		t.Error("always-run check should be affected by any change")
	}
	if !always.affected(nil) {
		t.Error("always-run check should be affected even with no changes")
	}
}

func TestInvalidatesAll(t *testing.T) {
	tests := []struct {
		name    string
		changed []string
		want    bool
	}{
		{"go.mod", []string{"go.mod"}, true},
		{"cli/go.sum", []string{"cli/go.sum"}, true},
		{"gate source", []string{"cli/internal/gates/registry.go"}, true},
		{"gate cmd", []string{"cli/cmd/ao/gate_check.go"}, true},
		{"ordinary go", []string{"cli/cmd/ao/main.go"}, false},
		{"docs only", []string{"docs/x.md", "README.md"}, false},
		{"mixed with gate src", []string{"docs/x.md", "cli/internal/gates/x.go"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := invalidatesAll(tc.changed); got != tc.want {
				t.Fatalf("invalidatesAll(%v) = %v, want %v", tc.changed, got, tc.want)
			}
		})
	}
}

func TestGitChangedFiles_ScopeMappingAndDedupe(t *testing.T) {
	var gotArgs []string
	g := &GitChangedFiles{
		RepoRoot: "/repo",
		run: func(_ context.Context, _ string, args ...string) (string, error) {
			gotArgs = args
			return "cli/a.go\n\ncli/a.go\nscripts/x.sh\n", nil
		},
	}
	changed, err := g.Changed(context.Background(), ScopeUpstream)
	if err != nil {
		t.Fatalf("Changed: %v", err)
	}
	want := []string{"cli/a.go", "scripts/x.sh"}
	if len(changed) != len(want) {
		t.Fatalf("Changed = %v, want %v (dedupe + drop blanks)", changed, want)
	}
	for i := range want {
		if changed[i] != want[i] {
			t.Fatalf("Changed[%d] = %q, want %q", i, changed[i], want[i])
		}
	}
	if len(gotArgs) == 0 || gotArgs[0] != "diff" {
		t.Fatalf("upstream scope args = %v, want a git diff", gotArgs)
	}
}

// TestIsInstalledSkillCopy pins which paths count as an installed copy of a
// skill package. The false rows matter most: a path that merely CONTAINS one of
// the runtime names, or the agentops repo's own skills/ source, must stay in
// the change set — over-filtering silently drops a gate.
func TestIsInstalledSkillCopy(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{".agents/skills/cass/scripts/multi_machine_search.sh", true},
		{".claude/skills/plan/SKILL.md", true},
		{".codex/skills/validate/scripts/run.sh", true},
		{".gemini/skills/x/y.sh", true},
		{".cursor/skills/x/y.sh", true},
		{".pi/skills/x/y.sh", true},
		{"agent/skills/x/y.sh", true},
		{"skills/cass/scripts/multi_machine_search.sh", false},
		{"scripts/check-x.sh", false},
		{".agents/ao/learnings/x.md", false},
		{".agents/handoff/x.md", false},
		{"vendor/.claude/skills/x/y.sh", false},
		{"cli/internal/skillsapp/link.go", false},
	}
	for _, tc := range tests {
		if got := IsInstalledSkillCopy(tc.path); got != tc.want {
			t.Errorf("IsInstalledSkillCopy(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestFilterInstalledSkillCopies(t *testing.T) {
	got := FilterInstalledSkillCopies([]string{
		".agents/skills/cass/scripts/multi_machine_search.sh",
		"scripts/deploy.sh",
		".claude/skills/plan/SKILL.md",
		"README.md",
	})
	want := []string{"scripts/deploy.sh", "README.md"}
	if len(got) != len(want) {
		t.Fatalf("FilterInstalledSkillCopies = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("FilterInstalledSkillCopies[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if got := FilterInstalledSkillCopies([]string{".claude/skills/plan/x.sh"}); got != nil {
		t.Fatalf("all-filtered set = %v, want nil", got)
	}
}

// TestOrchestrator_InstalledSkillCopiesDoNotRouteChecks is the routing-layer
// acceptance: a commit whose ONLY shell file is an installed skill copy must
// not select the shell gate. Observed live — a user's first commit after
// installing AgentOps failed shellcheck on AgentOps' own
// .agents/skills/cass/scripts/multi_machine_search.sh via the `**/*.sh` glob.
func TestOrchestrator_InstalledSkillCopiesDoNotRouteChecks(t *testing.T) {
	reg := NewRegistry()
	shell := Check{ID: "shell.shellcheck-changed", Tiers: Fast, Match: []string{"**/*.sh"}, Backing: "noop.sh"}
	if err := reg.Add(shell); err != nil {
		t.Fatal(err)
	}
	files := fakeFiles{files: []string{".agents/skills/cass/scripts/multi_machine_search.sh", "README.md"}}
	o := NewOrchestrator(reg, nil, files, t.TempDir())

	selected, changed, err := o.Select(context.Background(), RunOptions{Mode: Fast, Scope: ScopeHead})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if len(selected) != 0 {
		t.Fatalf("selected %v, want none (only in-scope change is README.md)", selected)
	}
	if !equalSet(changed, []string{"README.md"}) {
		t.Fatalf("routed change set = %v, want [README.md]", changed)
	}

	// Negative witness: a first-party shell file still routes the same check, so
	// the filter narrows the change set rather than disabling the gate.
	o = NewOrchestrator(reg, nil, fakeFiles{files: []string{"scripts/deploy.sh"}}, t.TempDir())
	selected, _, err = o.Select(context.Background(), RunOptions{Mode: Fast, Scope: ScopeHead})
	if err != nil {
		t.Fatalf("Select first-party: %v", err)
	}
	if len(selected) != 1 || selected[0].ID != shell.ID {
		t.Fatalf("first-party shell change selected %v, want [%s]", selected, shell.ID)
	}
}

func TestGitChangedFiles_UnknownScope(t *testing.T) {
	g := NewGitChangedFiles("/repo")
	if _, err := g.Changed(context.Background(), Scope("bogus")); err == nil {
		t.Fatal("unknown scope: want error, got nil")
	}
}
