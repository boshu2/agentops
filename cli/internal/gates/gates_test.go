package gates

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// sourceOnlyPathNeedles are substrings that only exist inside an agentops
// SOURCE checkout. A repair hint is read by whoever ran `ao gate check`, and
// most of them installed the CLI (brew, npx, plugin) and have no checkout, so a
// hint naming one of these is an instruction they cannot follow. Observed live:
// a fresh install's failing shell gate said "inspect native gate
// shell.shellcheck-changed in cli/internal/gates".
var sourceOnlyPathNeedles = []string{"cli/internal/", "cli/cmd/"}

// TestCheck_EffectiveRepairHintNamesNoSourceCheckoutPath is the acceptance for
// the derived native-check hint: with no explicit RepairHint it must name the
// gate ID and the published docs, never a Go source path.
func TestCheck_EffectiveRepairHintNamesNoSourceCheckoutPath(t *testing.T) {
	native := Check{
		ID:    "shell.shellcheck-changed",
		Tiers: Fast,
		Run:   func(context.Context, RunContext) (ports.GateVerdict, error) { return ports.GateVerdict{}, nil },
	}
	hint := native.EffectiveRepairHint()
	for _, needle := range sourceOnlyPathNeedles {
		if strings.Contains(hint, needle) {
			t.Fatalf("derived native hint %q names source-checkout path %q", hint, needle)
		}
	}
	if !strings.Contains(hint, native.ID) {
		t.Errorf("derived native hint %q does not name the gate ID %q", hint, native.ID)
	}
	if !strings.Contains(hint, GateDocsURL) {
		t.Errorf("derived native hint %q does not point at %s", hint, GateDocsURL)
	}
}

// TestCheck_EffectiveRepairHintPrefersExplicitHint pins the precedence: an
// explicit, plain-language remedy always wins over the derived fallback.
func TestCheck_EffectiveRepairHintPrefersExplicitHint(t *testing.T) {
	want := "run 'shellcheck -S warning <file>' and fix the warnings"
	native := Check{
		ID:         "shell.shellcheck-changed",
		Tiers:      Fast,
		RepairHint: want,
		Run:        func(context.Context, RunContext) (ports.GateVerdict, error) { return ports.GateVerdict{}, nil },
	}
	if got := native.EffectiveRepairHint(); got != want {
		t.Fatalf("EffectiveRepairHint() = %q, want %q", got, want)
	}
}

// TestOrchestrator_UnbornHeadSurfacesFriendlyError proves the friendly
// unborn-HEAD message survives the orchestrator's wrap and reaches the surface
// the user reads: `ao gate check` in a zero-commit repo previously printed
// "gates: detect changed files: git show --name-only --pretty=format: HEAD:
// exit status 128".
func TestOrchestrator_UnbornHeadSurfacesFriendlyError(t *testing.T) {
	root := initRepoNoCommits(t, false)
	reg := NewRegistry()
	if err := reg.Add(Check{ID: "shell", Tiers: Fast, Match: []string{"**/*.sh"}, Backing: "noop.sh"}); err != nil {
		t.Fatal(err)
	}
	o := NewOrchestrator(reg, nil, NewGitChangedFiles(root), root)

	_, err := o.Run(context.Background(), RunOptions{Mode: Fast, Scope: ScopeHead})
	if err == nil {
		t.Fatal("Run on a zero-commit repo returned no error")
	}
	if !errors.Is(err, ErrUnbornHead) {
		t.Fatalf("Run err = %v, want errors.Is ErrUnbornHead", err)
	}
	if strings.Contains(err.Error(), "exit status") {
		t.Errorf("orchestrator error leaks a git exit status: %v", err)
	}
	if !strings.Contains(err.Error(), "no commits yet") {
		t.Errorf("orchestrator error missing the friendly cause: %v", err)
	}
}
