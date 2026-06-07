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

func TestGitChangedFiles_UnknownScope(t *testing.T) {
	g := NewGitChangedFiles("/repo")
	if _, err := g.Changed(context.Background(), Scope("bogus")); err == nil {
		t.Fatal("unknown scope: want error, got nil")
	}
}
