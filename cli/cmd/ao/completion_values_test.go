package main

import (
	"reflect"
	"sort"
	"testing"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/spf13/cobra"
)

func TestStaticCompletionFunc_SortsAndSuppressesFileCompletion(t *testing.T) {
	fn := staticCompletionFunc("zebra", "apple", "mango")
	got, directive := fn(nil, nil, "")

	want := []string{"apple", "mango", "zebra"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("values = %v, want %v", got, want)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
	}
}

func TestStaticCompletionFunc_DoesNotMutateCallerSlice(t *testing.T) {
	values := []string{"zebra", "apple", "mango"}
	snapshot := append([]string(nil), values...)
	_ = staticCompletionFunc(values...)

	if !reflect.DeepEqual(values, snapshot) {
		t.Errorf("caller slice mutated: %v vs %v", values, snapshot)
	}
}

func TestTemplateCompletionValues_MatchesLifecycleValidTemplates(t *testing.T) {
	got := templateCompletionValues()

	want := make([]string, 0, len(lifecycle.ValidTemplates))
	for name, enabled := range lifecycle.ValidTemplates {
		if enabled {
			want = append(want, name)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %d, want %d (%v vs %v)", len(got), len(want), got, want)
	}

	gotSet := make(map[string]bool, len(got))
	for _, n := range got {
		gotSet[n] = true
	}
	for _, n := range want {
		if !gotSet[n] {
			t.Errorf("completion values missing %q (got %v)", n, got)
		}
	}
}

func TestTemplateCompletionValues_IsSorted(t *testing.T) {
	got := templateCompletionValues()
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Errorf("values not sorted at index %d: %v", i, got)
		}
	}
}

func sortedCompletionValues(values ...string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func mustFindCompletionCommand(t *testing.T, path ...string) *cobra.Command {
	t.Helper()
	cmd := rootCmd
	for _, name := range path {
		var next *cobra.Command
		for _, child := range cmd.Commands() {
			if child.Name() == name {
				next = child
				break
			}
		}
		if next == nil {
			t.Fatalf("command path %v not found at %q", path, name)
		}
		cmd = next
	}
	return cmd
}

// TestFlagCompletions_Registered verifies that every enumerated flag we care
// about has a completion function registered with the expected value set.
// This is an L2 integration test: it exercises the real cobra command tree
// built by init() side-effects in this package.
func TestFlagCompletions_Registered(t *testing.T) {
	tmplValues := templateCompletionValues()
	phaseValues := sortedCompletionValues("task", "startup", "planning", "pre-mortem", "validation")
	tierValues := sortedCompletionValues("gold", "silver", "bronze")
	planStatusValues := sortedCompletionValues("active", "completed", "abandoned", "superseded")

	cases := []struct {
		name string
		cmd  *cobra.Command
		flag string
		want []string
	}{
		{"root --output", rootCmd, "output", []string{"json", "table", "yaml"}},
		{"seed --template", seedCmd, "template", tmplValues},
		{"goals init --template", goalsInitCmd, "template", tmplValues},
		{"inject --format", injectCmd, "format", []string{"json", "markdown"}},
		{"inject --session-type", injectCmd, "session-type",
			[]string{"brainstorm", "career", "debug", "implement", "research"}},
		{"config models --set-tier", configModelsCmd, "set-tier",
			sortedCompletionValues("quality", "balanced", "budget")},
		{"goals add --type", goalsAddCmd, "type",
			sortedCompletionValues("health", "architecture", "quality", "meta")},
		{"goals steer add --steer", goalsSteerAddCmd, "steer",
			sortedCompletionValues("increase", "decrease", "hold", "explore")},
		{"gate bulk-approve --tier", gateBulkApproveCmd, "tier", tierValues},
		{"pool list --tier", poolListCmd, "tier", tierValues},
		{"pool list --status", poolListCmd, "status",
			sortedCompletionValues("pending", "staged", "promoted", "rejected")},
		{"pool stage --min-tier", poolStageCmd, "min-tier", tierValues},
		{"context assemble --phase", mustFindCompletionCommand(t, "context", "assemble"), "phase", phaseValues},
		{"context explain --phase", mustFindCompletionCommand(t, "context", "explain"), "phase", phaseValues},
		{"context packet-status --phase", mustFindCompletionCommand(t, "context", "packet-status"), "phase", phaseValues},
		{"rpi phased --from", mustFindCompletionCommand(t, "rpi", "phased"), "from",
			sortedCompletionValues("discovery", "implementation", "validation", "research", "plan", "pre-mortem", "crank", "vibe", "post-mortem")},
		{"rpi phased --runtime", mustFindCompletionCommand(t, "rpi", "phased"), "runtime",
			sortedCompletionValues("auto", "direct", "stream", "tmux")},
		{"overnight curator enqueue --kind", overnightCuratorEnqueueCmd, "kind",
			sortedCompletionValues("ingest-claude-session", "lint-wiki", "dream-seed")},
		{"overnight curator event --severity", overnightCuratorEventCmd, "severity",
			sortedCompletionValues("info", "warn", "high", "critical")},
		{"feedback-loop --citation-type", feedbackLoopCmd, "citation-type",
			sortedCompletionValues("retrieved", "applied", "all")},
		{"plans list --status", plansListCmd, "status", planStatusValues},
		{"plans update --status", plansUpdateCmd, "status", planStatusValues},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn, exists := tc.cmd.GetFlagCompletionFunc(tc.flag)
			if !exists {
				t.Fatalf("no completion registered for %s %s", tc.cmd.Name(), tc.flag)
			}
			got, directive := fn(tc.cmd, nil, "")
			if directive != cobra.ShellCompDirectiveNoFileComp {
				t.Errorf("directive = %v, want ShellCompDirectiveNoFileComp", directive)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("completion values = %v, want %v", got, tc.want)
			}
		})
	}
}
