package checks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/gates"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// Each fixture contains exactly one active rule and one existing in-scope file,
// so an unrelated violation or a missing file cannot mask a detector error.
func runConstraintDetectorFixture(t *testing.T, detector, content string) ports.GateVerdict {
	t.Helper()
	index := fmt.Sprintf(`{"schema_version":1,"constraints":[{
		"id":"f-detector","title":"detector contract","status":"active",
		"compiled_at":"2026-06-21T00:00:00Z",
		"applies_to":{"path_globs":["cli/**"]},"detector":%s
	}]}`, detector)
	const rel = "cli/internal/detector.go"
	rc := writeConstraintFixture(t, index, map[string]string{rel: content}, []string{rel})
	actual, err := os.ReadFile(filepath.Join(rc.RepoRoot, rel))
	if err != nil {
		t.Fatalf("read back detector fixture: %v", err)
	}
	if string(actual) != content {
		t.Fatalf("detector fixture content = %q, want %q", actual, content)
	}
	check, ok := gates.Default.Get("constraints.enforce")
	if !ok || check.Run == nil {
		t.Fatal("constraints.enforce must have a registered native gate route")
	}
	verdict, err := check.Run(context.Background(), rc)
	if err != nil {
		t.Fatalf("constraints.enforce returned an error instead of a verdict: %v", err)
	}
	return verdict
}

func TestConstraintGate_DetectorEvaluationDiagnostics(t *testing.T) {
	cases := []struct {
		name     string
		detector string
		logTail  string
	}{
		{
			name:     "invalid exclusion regex",
			detector: `{"kind":"regex","mode":"match","pattern":"panic\\(","exclude":"("}`,
			logTail:  "f-detector (detector contract): constraint could not be evaluated: bad exclude regex \"(\": error parsing regexp: missing closing ): `(`",
		},
		{
			name:     "empty required pattern",
			detector: `{"kind":"regex","mode":"required","pattern":""}`,
			logTail:  "f-detector (detector contract): constraint could not be evaluated: regex detector has empty pattern",
		},
		{
			name:     "unsupported detector mode",
			detector: `{"kind":"regex","mode":"bogus","pattern":"panic\\("}`,
			logTail:  "f-detector (detector contract): constraint could not be evaluated: unknown detector mode \"bogus\"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runConstraintDetectorFixture(t, tc.detector, "package example\nfunc run() { panic(\"boom\") }\n")
			want := ports.GateVerdict{
				Status:  ports.GateStatusFail,
				Reason:  "1 active constraint(s) could not be evaluated — failing closed",
				LogTail: tc.logTail,
			}
			if got != want {
				t.Fatalf("detector evaluation verdict = %#v, want %#v", got, want)
			}
		})
	}
}

func TestConstraintGate_DetectorExclusionDiagnostics(t *testing.T) {
	const detector = `{"kind":"regex","mode":"match","pattern":"panic\\(","exclude":"//nolint","message":"no panic() in cli"}`
	cases := []struct {
		name    string
		content string
		want    ports.GateVerdict
	}{
		{
			name:    "only unexcluded match is reported",
			content: "package example\n\nfunc run() {\n\tpanic(\"excluded\") //nolint\n\tpanic(\"blocked\")\n}\n",
			want: ports.GateVerdict{
				Status:  ports.GateStatusFail,
				Reason:  "1 constraint violation(s) in changed files",
				LogTail: "cli/internal/detector.go:5 [f-detector] no panic() in cli",
			},
		},
		{
			name:    "excluded match is clean control",
			content: "package example\n\nfunc run() {\n\tpanic(\"excluded\") //nolint\n}\n",
			want: ports.GateVerdict{
				Status: ports.GateStatusPass,
				Reason: "1 active constraint(s) evaluated, no violations",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runConstraintDetectorFixture(t, detector, tc.content)
			if got != tc.want {
				t.Fatalf("exclusion verdict = %#v, want %#v", got, tc.want)
			}
		})
	}
}
