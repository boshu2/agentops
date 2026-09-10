package checks

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestEvalLearningReadError(t *testing.T) {
	for _, prefix := range []string{".agents/ao/learnings/", ".agents/learnings/"} {
		t.Run(prefix, func(t *testing.T) {
			root := t.TempDir()
			path := prefix + "unreadable.md"
			if err := os.MkdirAll(filepath.Join(root, path), 0755); err != nil {
				t.Fatal(err)
			}
			v, err := runLearningCoherence(context.Background(), gates.RunContext{RepoRoot: root, ChangedFiles: []string{path}})
			report := gates.Report{Results: []gates.CheckResult{{
				Check: gates.Check{ID: "learning.coherence", Blocking: true}, Verdict: v, Err: err,
			}}}
			reason := v.Reason
			if err != nil {
				reason += err.Error()
			}
			if report.ExitCode() != 1 || !strings.Contains(reason, path) {
				t.Fatalf("unreadable learning must block and name path: %+v, %v", v, err)
			}
		})
	}
}

func TestEvalLearningControls(t *testing.T) {
	for _, path := range []string{".agents/ao/learnings/deleted.md", ".agents/learnings/deleted.md", "other/unreadable.md", ".agents/learnings/ignored.txt"} {
		root := t.TempDir()
		if !strings.HasSuffix(path, "deleted.md") {
			if err := os.MkdirAll(filepath.Join(root, path), 0755); err != nil {
				t.Fatal(err)
			}
		}
		v, err := runLearningCoherence(context.Background(), gates.RunContext{RepoRoot: root, ChangedFiles: []string{path}})
		if err != nil || v.Status != ports.GateStatusPass {
			t.Fatalf("control %s: %+v, %v", path, v, err)
		}
	}
}
