package checks

import (
	"context"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
)

// parityFiles is a fake ChangedFilesPort returning a fixed change set.
type parityFiles struct{ files []string }

func (f parityFiles) Changed(context.Context, gates.Scope) ([]string, error) { return f.files, nil }

// selectedIDs runs the orchestrator's Select (no execution) over the real seed
// registry for a given change set and returns the chosen check IDs.
func selectedIDs(t *testing.T, changed []string) map[string]bool {
	t.Helper()
	o := gates.NewOrchestrator(gates.Default, nil, parityFiles{files: changed}, "/repo")
	sel, _, err := o.Select(context.Background(), gates.RunOptions{Mode: gates.Fast, Scope: gates.ScopeHead})
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	ids := map[string]bool{}
	for _, c := range sel {
		ids[c.ID] = true
	}
	return ids
}

// alwaysIDs are the no-Match checks that must run for any change.
var alwaysIDs = []string{
	"always.mutation-route",
	"always.no-tracked-agents",
	"always.embedded-sync",
}

func assertHas(t *testing.T, ids map[string]bool, want ...string) {
	t.Helper()
	for _, id := range want {
		if !ids[id] {
			t.Errorf("expected check %q to be selected; selected=%v", id, ids)
		}
	}
}

func assertNot(t *testing.T, ids map[string]bool, notWant ...string) {
	t.Helper()
	for _, id := range notWant {
		if ids[id] {
			t.Errorf("check %q should NOT be selected (wrong change-class — #634 class); selected=%v", id, ids)
		}
	}
}

// TestPredicateParity_PerChangeClass is the GA7 guard: each change class routes
// to exactly the checks that guard it, and NOT to unrelated ones.
func TestPredicateParity_PerChangeClass(t *testing.T) {
	t.Run("go change", func(t *testing.T) {
		ids := selectedIDs(t, []string{"cli/cmd/ao/main.go"})
		assertHas(t, ids, "go.build", "go.command-test-pair")
		assertHas(t, ids, alwaysIDs...)
		assertNot(t, ids, "skill.schema", "contract.skill-mesh")
	})

	t.Run("skill change", func(t *testing.T) {
		ids := selectedIDs(t, []string{"skills/foo/SKILL.md"})
		assertHas(t, ids, "skill.schema", "skill.no-operator-leakage", "derived.changed-scope")
		assertHas(t, ids, alwaysIDs...)
		assertHas(t, ids, "contract.skill-mesh")
		assertNot(t, ids, "go.build", "go.command-test-pair")
	})

	t.Run("skill conformance selector self-change", func(t *testing.T) {
		ids := selectedIDs(t, []string{"scripts/regen-changed-scope.sh"})
		assertHas(t, ids, "derived.changed-scope")
	})

	// The operator-leakage guard also protects the generated published surfaces
	// (docs/SKILLS.md, registry.json), so a registry-only diff that could publish
	// a denied skill MUST still select the blocking gate (else it fails open).
	t.Run("registry.json-only selects no-operator-leakage", func(t *testing.T) {
		ids := selectedIDs(t, []string{"registry.json"})
		assertHas(t, ids, "skill.no-operator-leakage")
	})

	t.Run("contract change", func(t *testing.T) {
		ids := selectedIDs(t, []string{"schemas/eval-outcomes.json"})
		assertHas(t, ids, "contract.cathedral-cut", "contract.finding-registry")
		assertHas(t, ids, alwaysIDs...)
		assertNot(t, ids, "go.build", "skill.schema")
	})

	t.Run("docs change", func(t *testing.T) {
		ids := selectedIDs(t, []string{"docs/how-it-works.md"})
		assertHas(t, ids, alwaysIDs...)
		assertNot(t, ids, "go.build", "skill.schema", "contract.skill-mesh", "claim.registry-drift")
	})

	t.Run("empty diff runs only always-checks", func(t *testing.T) {
		ids := selectedIDs(t, nil)
		assertHas(t, ids, alwaysIDs...)
		assertNot(t, ids, "go.build", "skill.schema", "contract.skill-mesh")
	})
}

// TestPredicateParity_InvalidationRunsAll: a go.mod (or gate-source) change must
// force every fast check regardless of routing.
func TestPredicateParity_InvalidationRunsAll(t *testing.T) {
	ids := selectedIDs(t, []string{"go.mod"})
	assertHas(t, ids, "go.build", "skill.schema", "contract.skill-mesh")
	assertHas(t, ids, alwaysIDs...)
}
