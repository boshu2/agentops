package gates

import (
	"bytes"
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
