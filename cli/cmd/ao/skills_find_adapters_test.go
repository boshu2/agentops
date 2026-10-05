// practices: [design-by-contract, code-complete]
package main

import (
	"encoding/json"
	"testing"
)

// TestSkillsFind_SeparatesHeadlessAdapters runs `ao skills find` through the
// root command against the shipped catalog.
//
// Codex Exec and Claude Exec describe the same job (one prompt, headless,
// one-shot) for two different CLIs, and each skill carries that CLI's own
// flags. The runtime named in the request is the only thing that separates
// them, so a request that names Claude must rank Claude Exec first and a
// request that names Codex must rank Codex Exec first. Routing either request
// to the sibling hands the agent the wrong tool's sandbox and timeout flags.
func TestSkillsFind_SeparatesHeadlessAdapters(t *testing.T) {
	cases := []struct {
		query string
		want  string
	}{
		{query: "run one prompt through headless claude", want: "claude-exec"},
		{query: "run one prompt through headless codex", want: "codex-exec"},
	}
	// executeCommand leaves --json and --limit set on the shared `skills find`
	// command; put every flag back to its default so no later test inherits them.
	t.Cleanup(func() { resetFlagChangesRecursive(rootCmd) })
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			out, err := executeCommand("skills", "find", "--json", "--limit", "1", tc.query)
			if err != nil {
				t.Fatalf("ao skills find %q: %v\noutput: %s", tc.query, err, out)
			}
			var got []struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("ao skills find --json did not print a JSON array: %v\noutput: %s", err, out)
			}
			if len(got) != 1 {
				t.Fatalf("ao skills find --limit 1 %q returned %d matches, want 1: %s", tc.query, len(got), out)
			}
			if got[0].Name != tc.want {
				t.Errorf("ao skills find %q ranked %q first, want %q", tc.query, got[0].Name, tc.want)
			}
		})
	}
}
