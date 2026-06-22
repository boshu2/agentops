// practices: [design-by-contract, ai-assisted-dev]
package worktree

import (
	"strings"
	"testing"
)

func TestValidateWorktreeSibling(t *testing.T) {
	tests := []struct {
		name      string
		repoRoot  string
		worktree  string
		wantErr   bool
		errSubstr string
	}{
		{"valid sibling", "/dev/agentops", "/dev/agentops-wt-1", false, ""},
		{"valid sibling with trailing slash", "/dev/agentops/", "/dev/agentops-wt-1/", false, ""},
		{"not a sibling (different parent)", "/dev/agentops", "/tmp/elsewhere", true, "not a sibling"},
		{"worktree IS the repo root", "/dev/agentops", "/dev/agentops", true, "is the repo root"},
		{"nested under repo (not sibling)", "/dev/agentops", "/dev/agentops/sub", true, "not a sibling"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorktreeSibling(tt.repoRoot, tt.worktree)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateWorktreeSibling(%q,%q) = nil, want error", tt.repoRoot, tt.worktree)
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
			} else if err != nil {
				t.Errorf("ValidateWorktreeSibling(%q,%q) = %v, want nil", tt.repoRoot, tt.worktree, err)
			}
		})
	}
}
