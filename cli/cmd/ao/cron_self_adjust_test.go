// practices: [dora-metrics, lean-startup]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/evolve"
)

// minimalCronTemplate returns a self-contained template with a single
// VERBATIM-PRESERVE marker whose SHA is computed inline. Used by tests that
// don't need the real production template.
func minimalCronTemplate(t *testing.T) string {
	t.Helper()
	inner := "\nload-bearing\n"
	sha := evolve.ComputeMarkerSHA(inner)
	return strings.Join([]string{
		"---",
		"template_version: 1",
		"verbatim_markers:",
		"  test-marker: " + sha,
		"---",
		"",
		"# Cycle {{.CronSelfAdjustCounter}}",
		"",
		"Shipped: {{range .ShippedCommits}}{{.Sha}}:{{.Bead}}{{if .Scenario}}#{{.Scenario}}{{end}} {{end}}",
		"Next: {{.NextRecommendedBead}}",
		"Sub-beads: {{range .SubBeadsFiledThisCycle}}- {{.}} {{end}}",
		"Tests: {{.TestsDelta}}",
		"",
		"<!-- VERBATIM-PRESERVE:start name=\"test-marker\" -->" + inner + "<!-- VERBATIM-PRESERVE:end -->",
	}, "\n")
}

// withFixedCronClock pins the clock for deterministic timestamps.
func withFixedCronClock(t *testing.T, ts time.Time) {
	t.Helper()
	prev := cronSelfAdjustClock
	cronSelfAdjustClock = func() time.Time { return ts }
	t.Cleanup(func() { cronSelfAdjustClock = prev })
}

// TestCronSelfAdjust_RoundTrip exercises the happy path: render a template
// with shipped/next/sub-beads, verify stdout JSON and history row.
func TestCronSelfAdjust_RoundTrip(t *testing.T) {
	dir := chdirTemp(t)
	withFixedCronClock(t, time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC))

	templatePath := filepath.Join(dir, ".agents/evolve/cron-template.md")
	if err := os.MkdirAll(filepath.Dir(templatePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(templatePath, []byte(minimalCronTemplate(t)), 0o644); err != nil {
		t.Fatalf("seed template: %v", err)
	}

	out, err := executeCommand(
		"cron", "self-adjust",
		"--on", "cycle-close",
		"--template", ".agents/evolve/cron-template.md",
		"--shipped", "abc123:soc-x,def456:soc-y#scen",
		"--next", "soc-z",
		"--sub-beads", "soc-q,soc-r",
		"--tests-delta", "+3 passing",
	)
	if err != nil {
		t.Fatalf("err: %v\nout=%s", err, out)
	}
	jsonStart := strings.Index(out, "{")
	if jsonStart < 0 {
		t.Fatalf("no JSON in output: %q", out)
	}
	var spec cronSelfAdjustSpec
	if err := json.Unmarshal([]byte(out[jsonStart:]), &spec); err != nil {
		t.Fatalf("decode spec: %v\nout=%s", err, out)
	}
	if !strings.Contains(spec.NewCronPrompt, "Cycle 1") {
		t.Errorf("prompt missing cycle counter: %q", spec.NewCronPrompt)
	}
	if !strings.Contains(spec.NewCronPrompt, "abc123:soc-x") {
		t.Errorf("prompt missing shipped commit: %q", spec.NewCronPrompt)
	}
	if !strings.Contains(spec.NewCronPrompt, "soc-y#scen") {
		t.Errorf("prompt missing scenario: %q", spec.NewCronPrompt)
	}
	if !strings.Contains(spec.NewCronPrompt, "Next: soc-z") {
		t.Errorf("prompt missing next: %q", spec.NewCronPrompt)
	}
	if !strings.Contains(spec.NewCronPrompt, "- soc-q") || !strings.Contains(spec.NewCronPrompt, "- soc-r") {
		t.Errorf("prompt missing sub-beads: %q", spec.NewCronPrompt)
	}
	if !strings.Contains(spec.NewCronPrompt, "+3 passing") {
		t.Errorf("prompt missing tests delta: %q", spec.NewCronPrompt)
	}
	if spec.ScheduleHint != "cycle-close" {
		t.Errorf("schedule hint = %q, want cycle-close", spec.ScheduleHint)
	}

	rows, err := cronHistoryReadRows(filepath.Join(dir, cronSelfAdjustHistoryRel))
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("history rows = %d, want 1", len(rows))
	}
	got := rows[0]
	want := cronSelfAdjustHistoryRow{
		Timestamp:        "2026-05-21T12:00:00Z",
		CronIDBefore:     "",
		CronIDAfter:      "",
		Shipped:          []string{"abc123:soc-x", "def456:soc-y#scen"},
		Next:             "soc-z",
		SubBeadsFiled:    []string{"soc-q", "soc-r"},
		TestsDelta:       "+3 passing",
		RenderedTemplate: templatePath,
	}
	if got.Timestamp != want.Timestamp ||
		got.Next != want.Next ||
		got.TestsDelta != want.TestsDelta ||
		got.RenderedTemplate != want.RenderedTemplate {
		t.Errorf("history scalars mismatch:\n got=%+v\nwant=%+v", got, want)
	}
	if strings.Join(got.Shipped, "|") != strings.Join(want.Shipped, "|") {
		t.Errorf("shipped mismatch: got=%v want=%v", got.Shipped, want.Shipped)
	}
	if strings.Join(got.SubBeadsFiled, "|") != strings.Join(want.SubBeadsFiled, "|") {
		t.Errorf("sub-beads mismatch: got=%v want=%v", got.SubBeadsFiled, want.SubBeadsFiled)
	}
}

// TestCronSelfAdjust_RefusesOnMarkerDrift confirms VerifyMarkers gating wires
// through; tampering with marker content trips an exit-1 error.
func TestCronSelfAdjust_RefusesOnMarkerDrift(t *testing.T) {
	dir := chdirTemp(t)

	templatePath := filepath.Join(dir, ".agents/evolve/cron-template.md")
	if err := os.MkdirAll(filepath.Dir(templatePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Use the minimal template, but tamper with the marker's inner content
	// after generating its SHA — drift should be detected.
	tpl := minimalCronTemplate(t)
	tampered := strings.Replace(tpl, "load-bearing", "tampered-content", 1)
	if err := os.WriteFile(templatePath, []byte(tampered), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	out, err := executeCommand(
		"cron", "self-adjust",
		"--template", ".agents/evolve/cron-template.md",
		"--shipped", "abc:soc-x",
	)
	if err == nil {
		t.Fatalf("expected drift error\nout=%s", out)
	}
	if !strings.Contains(err.Error(), "drift") {
		t.Errorf("error message: %v", err)
	}
}

// TestCronSelfAdjust_EmptyShippedTolerated covers the corner case where the
// cycle shipped nothing — render should still succeed and emit a valid spec.
func TestCronSelfAdjust_EmptyShippedTolerated(t *testing.T) {
	dir := chdirTemp(t)
	withFixedCronClock(t, time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC))

	templatePath := filepath.Join(dir, ".agents/evolve/cron-template.md")
	if err := os.MkdirAll(filepath.Dir(templatePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(templatePath, []byte(minimalCronTemplate(t)), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	out, err := executeCommand(
		"cron", "self-adjust",
		"--template", ".agents/evolve/cron-template.md",
	)
	if err != nil {
		t.Fatalf("err: %v\nout=%s", err, out)
	}
	jsonStart := strings.Index(out, "{")
	if jsonStart < 0 {
		t.Fatalf("no JSON: %q", out)
	}
	var spec cronSelfAdjustSpec
	if err := json.Unmarshal([]byte(out[jsonStart:]), &spec); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if spec.NewCronPrompt == "" {
		t.Errorf("expected non-empty prompt")
	}
}

// TestCronSelfAdjust_CounterIncrementsAcrossInvocations verifies cycle counter
// advances each call.
func TestCronSelfAdjust_CounterIncrementsAcrossInvocations(t *testing.T) {
	dir := chdirTemp(t)
	withFixedCronClock(t, time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC))

	templatePath := filepath.Join(dir, ".agents/evolve/cron-template.md")
	if err := os.MkdirAll(filepath.Dir(templatePath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(templatePath, []byte(minimalCronTemplate(t)), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// First call → counter 1.
	out, err := executeCommand("cron", "self-adjust", "--template", ".agents/evolve/cron-template.md")
	if err != nil {
		t.Fatalf("call 1: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "Cycle 1") {
		t.Errorf("call 1 counter: %q", out)
	}

	// Second call → counter 2.
	out, err = executeCommand("cron", "self-adjust", "--template", ".agents/evolve/cron-template.md")
	if err != nil {
		t.Fatalf("call 2: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "Cycle 2") {
		t.Errorf("call 2 counter: %q", out)
	}
}

// TestParseShippedCommits covers the comma-separated --shipped parser.
func TestParseShippedCommits(t *testing.T) {
	cases := []struct {
		in   string
		want []evolve.ShippedCommit
	}{
		{"", nil},
		{"abc:soc-x", []evolve.ShippedCommit{{Sha: "abc", Bead: "soc-x"}}},
		{"abc:soc-x,def:soc-y", []evolve.ShippedCommit{
			{Sha: "abc", Bead: "soc-x"},
			{Sha: "def", Bead: "soc-y"},
		}},
		{"abc:soc-x#scen", []evolve.ShippedCommit{{Sha: "abc", Bead: "soc-x", Scenario: "scen"}}},
		{"soc-only", []evolve.ShippedCommit{{Bead: "soc-only"}}},
	}
	for _, tc := range cases {
		got := parseShippedCommits(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("parseShippedCommits(%q) len=%d, want %d", tc.in, len(got), len(tc.want))
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseShippedCommits(%q)[%d] = %+v, want %+v", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

// TestCronSelfAdjust_RegisteredOnCron confirms the subcommand is reachable
// via `ao cron self-adjust`.
func TestCronSelfAdjust_RegisteredOnCron(t *testing.T) {
	var found bool
	for _, sub := range cronCmd.Commands() {
		if sub.Name() == "self-adjust" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("cron self-adjust should be registered on cronCmd")
	}
}

// TestCron_RegisteredOnRoot confirms `ao cron` is reachable.
func TestCron_RegisteredOnRoot(t *testing.T) {
	var found bool
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "cron" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("cron should be registered on rootCmd")
	}
}
