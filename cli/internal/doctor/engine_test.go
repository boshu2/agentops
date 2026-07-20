package doctor

import (
	"strings"
	"testing"
	"time"
)

// TestHealthLineReportsEverySeverityBucket guards novice edge 4b: the health
// one-liner once printed only P0 and P2, silently dropping P1 — the worst
// severity actually present. A persisted run whose only finding is the P1
// fm-skills-missing must surface every bucket (P0..P3) in the line, so the
// worst severity can never be omitted again. The fixture is the real
// persistence round-trip: Diagnose writes report.json, Health reads it back.
func TestHealthLineReportsEverySeverityBucket(t *testing.T) {
	repo, home := t.TempDir(), t.TempDir()
	rep, err := Diagnose(Options{
		RepoRoot: repo, CWD: repo, HomeDir: home, ToolVersion: "9.9.9",
		Only: []string{"fm-skills-missing"}, Now: time.Unix(1_700_000_000, 0),
	})
	if err != nil {
		t.Fatalf("Diagnose: %v", err)
	}
	if rep.Summary.BySeverity["P1"] != 1 || rep.Summary.TotalFindings != 1 {
		t.Fatalf("precondition: want exactly one P1 finding, got %+v", rep.Summary)
	}
	line, hr, err := Health(repo, "9.9.9")
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if hr.Findings != 1 || hr.Status != "findings" || hr.ExitCode != ExitFindings {
		t.Fatalf("HealthResult = %+v", hr)
	}
	if !strings.HasPrefix(line, "findings  ao=9.9.9 doctor="+DoctorVersion+" findings=1 ") {
		t.Fatalf("health line prefix wrong: %q", line)
	}
	for _, want := range []string{"P0=0", "P1=1", "P2=0", "P3=0"} {
		if !strings.Contains(line, want) {
			t.Fatalf("health line %q missing bucket %q", line, want)
		}
	}
}
