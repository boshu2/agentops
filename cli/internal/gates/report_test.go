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
