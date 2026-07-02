package gates

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestReportGitHubAnnotations(t *testing.T) {
	report := &Report{
		Mode:      Full,
		Scope:     ScopeHead,
		StartedAt: time.Unix(0, 0),
		Results: []CheckResult{
			{
				Check: Check{ID: "blocking.fail", Blocking: true},
				Verdict: ports.GateVerdict{
					Status:  ports.GateStatusFail,
					Reason:  "failed 100%",
					LogTail: "line one\nline two",
				},
			},
			{
				Check:   Check{ID: "advisory.warn", Blocking: false},
				Verdict: ports.GateVerdict{Status: ports.GateStatusWarn, Reason: "warned"},
			},
			{
				Check:   Check{ID: "passing", Blocking: true},
				Verdict: ports.GateVerdict{Status: ports.GateStatusPass, Reason: "ok"},
			},
		},
	}

	var out bytes.Buffer
	report.GitHubAnnotations(&out)
	got := out.String()
	if !strings.Contains(got, "::error title=blocking.fail::failed 100%25%0Aline one%0Aline two") {
		t.Fatalf("missing escaped error annotation: %q", got)
	}
	if !strings.Contains(got, "::warning title=advisory.warn::warned") {
		t.Fatalf("missing warning annotation: %q", got)
	}
	if strings.Contains(got, "passing") {
		t.Fatalf("PASS checks should not emit annotations: %q", got)
	}
}

// gatesFromJSON parses r.JSON() into a name->log_tail map plus the status, so
// tests can assert the emitted detail exactly.
func gatesFromJSON(t *testing.T, r *Report) map[string]struct{ Status, LogTail string } {
	t.Helper()
	raw, err := r.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var parsed struct {
		Gates []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			LogTail string `json:"log_tail"`
		} `json:"gates"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	out := map[string]struct{ Status, LogTail string }{}
	for _, g := range parsed.Gates {
		out[g.Name] = struct{ Status, LogTail string }{g.Status, g.LogTail}
	}
	return out
}

// TestReportJSON_FailLogTail is the SLICE 2 acceptance: a FAILing gate's JSON
// log_tail is never empty. Native-Go checks (and silently-failing backing
// scripts) capture no output, so the field falls back to the reason; a captured
// tail longer than the window is bounded to its last lines; passing gates keep
// an empty (omitted) log_tail.
func TestReportJSON_FailLogTail(t *testing.T) {
	// A captured tail with more than maxLogTailLines lines.
	var big strings.Builder
	for i := 0; i < 40; i++ {
		big.WriteString("noise line ")
		big.WriteByte(byte('0' + i%10))
		big.WriteByte('\n')
	}
	big.WriteString("FAIL: the real reason is on the last line")
	bigTail := big.String()

	report := &Report{
		Mode:      Full,
		Scope:     ScopeHead,
		StartedAt: time.Unix(0, 0),
		Results: []CheckResult{
			// FAIL, native check, no captured output -> fall back to reason.
			{
				Check:   Check{ID: "native.fail", Blocking: true},
				Verdict: ports.GateVerdict{Status: ports.GateStatusFail, Reason: "CHANGELOG.md != docs/CHANGELOG.md"},
			},
			// FAIL, backing script with output -> bounded to last 15 lines.
			{
				Check:   Check{ID: "backing.fail", Blocking: true},
				Verdict: ports.GateVerdict{Status: ports.GateStatusFail, Reason: "exit 1", LogTail: bigTail},
			},
			// Evaluation error, no output -> fall back to the eval-error reason.
			{
				Check:   Check{ID: "eval.error", Blocking: true},
				Verdict: ports.GateVerdict{Status: ports.GateStatusUnknown},
				Err:     errFake("index malformed"),
			},
			// PASS -> log_tail stays empty (omitted).
			{
				Check:   Check{ID: "ok.pass", Blocking: true},
				Verdict: ports.GateVerdict{Status: ports.GateStatusPass, Reason: "ok"},
			},
		},
	}

	got := gatesFromJSON(t, report)

	if g := got["native.fail"]; g.LogTail != "CHANGELOG.md != docs/CHANGELOG.md" {
		t.Fatalf("native.fail log_tail = %q, want reason fallback", g.LogTail)
	}

	backing := got["backing.fail"].LogTail
	if strings.Count(backing, "\n") != maxLogTailLines-1 {
		t.Fatalf("backing.fail log_tail = %d lines, want %d\n%q",
			strings.Count(backing, "\n")+1, maxLogTailLines, backing)
	}
	if !strings.HasSuffix(backing, "FAIL: the real reason is on the last line") {
		t.Fatalf("backing.fail log_tail should end with the real reason line, got %q", backing)
	}
	if len(backing) >= len(bigTail) {
		t.Fatalf("backing.fail log_tail (%d bytes) should be shorter than the 41-line input (%d bytes)", len(backing), len(bigTail))
	}

	if g := got["eval.error"]; g.LogTail != "evaluation error: index malformed" {
		t.Fatalf("eval.error log_tail = %q, want the eval-error reason", g.LogTail)
	}

	if g := got["ok.pass"]; g.LogTail != "" {
		t.Fatalf("ok.pass log_tail = %q, want empty (omitted)", g.LogTail)
	}
}

// TestLastLines exercises the line-bounding helper directly.
func TestLastLines(t *testing.T) {
	cases := []struct {
		name, in string
		n        int
		want     string
	}{
		{"empty", "", 15, ""},
		{"fewer than n", "a\nb", 15, "a\nb"},
		{"exactly n", "a\nb\nc", 3, "a\nb\nc"},
		{"more than n keeps tail", "a\nb\nc\nd", 2, "c\nd"},
		{"trailing newlines trimmed", "a\nb\n\n\n", 15, "a\nb"},
		{"single line", "only", 15, "only"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := lastLines(tc.in, tc.n); got != tc.want {
				t.Fatalf("lastLines(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
			}
		})
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }

func TestGateReportHumanExplainability(t *testing.T) {
	report := &Report{
		Mode:      Fast,
		Scope:     ScopeHead,
		StartedAt: time.Unix(0, 0),
		Results: []CheckResult{
			{
				Check: Check{
					ID:         "derived.changed-scope",
					Tiers:      Fast,
					Blocking:   true,
					Backing:    "regen-changed-scope.sh",
					Args:       []string{"--check", "--scope", "head"},
					RepairHint: "bash scripts/regen-changed-scope.sh --scope head",
				},
				SelectedReason: `selected: changed file "skills/x/SKILL.md" matched "skills/**"`,
				Verdict:        ports.GateVerdict{Status: ports.GateStatusPass, Reason: "ok"},
			},
		},
		Skipped: []SkippedCheck{
			{
				Check:  Check{ID: "always.regen-all", Tiers: Full, Blocking: true, Backing: "regen-all.sh", RepairHint: "bash scripts/regen-all.sh"},
				Reason: "skipped: check tiers full do not include active mode fast",
			},
		},
	}

	var out bytes.Buffer
	report.Human(&out)
	got := out.String()
	for _, want := range []string{
		`selected: changed file "skills/x/SKILL.md" matched "skills/**"`,
		"backing: bash scripts/regen-changed-scope.sh --check --scope head",
		"artifact: scripts/regen-changed-scope.sh",
		"repair: bash scripts/regen-changed-scope.sh --scope head",
		"skipped gates:",
		"skipped: check tiers full do not include active mode fast",
		"not run: 1 gates",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("human output missing %q:\n%s", want, got)
		}
	}
}
