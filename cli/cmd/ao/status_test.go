// practices: [dora-metrics, sre]
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTruncateStatus(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncated", "hello world this is long", 10, "hello w..."},
		{"empty string", "", 10, ""},
		{"with newline", "first line\nsecond line", 60, "first line"},
		{"newline only", "\nsecond line", 60, ""},
		{"maxLen 4", "hello", 4, "h..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStatus(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateStatus(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestFirstLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single line", "hello", "hello"},
		{"multi line", "first\nsecond\nthird", "first"},
		{"empty string", "", ""},
		{"starts with newline", "\nfirst", ""},
		{"trailing newline", "hello\n", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstLine(tt.input)
			if got != tt.want {
				t.Errorf("firstLine(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFindLastForgeTime(t *testing.T) {
	t.Run("finds most recent file", func(t *testing.T) {
		tmp := t.TempDir()
		retrosDir := filepath.Join(tmp, ".agents", "retros")
		learningsDir := filepath.Join(tmp, ".agents", "learnings")
		if err := os.MkdirAll(retrosDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(learningsDir, 0755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(retrosDir, "retro-1.md"), []byte("retro"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(learningsDir, "L1.md"), []byte("learning"), 0644); err != nil {
			t.Fatal(err)
		}

		result := findLastForgeTime(tmp)
		if result.IsZero() {
			t.Error("expected non-zero time")
		}
		// Should be very recent (within last minute)
		if time.Since(result) > time.Minute {
			t.Errorf("last forge time too old: %v", result)
		}
	})

	t.Run("empty dirs return zero", func(t *testing.T) {
		tmp := t.TempDir()
		if err := os.MkdirAll(filepath.Join(tmp, ".agents", "retros"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(tmp, ".agents", "learnings"), 0755); err != nil {
			t.Fatal(err)
		}

		result := findLastForgeTime(tmp)
		if !result.IsZero() {
			t.Errorf("expected zero time, got %v", result)
		}
	})

	t.Run("nonexistent dirs return zero", func(t *testing.T) {
		tmp := t.TempDir()
		result := findLastForgeTime(tmp)
		if !result.IsZero() {
			t.Errorf("expected zero time, got %v", result)
		}
	})

	t.Run("ignores subdirectories", func(t *testing.T) {
		tmp := t.TempDir()
		retrosDir := filepath.Join(tmp, ".agents", "retros")
		if err := os.MkdirAll(filepath.Join(retrosDir, "subdir"), 0755); err != nil {
			t.Fatal(err)
		}

		result := findLastForgeTime(tmp)
		if !result.IsZero() {
			t.Errorf("expected zero time (dirs should be ignored), got %v", result)
		}
	})
}

func TestFormatDurationBrief(t *testing.T) {
	tests := []struct {
		name  string
		input time.Duration
		want  string
	}{
		{"sub-minute", 30 * time.Second, "<1m"},
		{"minutes", 45 * time.Minute, "45m"},
		{"hours", 5 * time.Hour, "5h"},
		{"days", 3 * 24 * time.Hour, "3d"},
		{"weeks", 45 * 24 * time.Hour, "6w"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDurationBrief(tt.input)
			if got != tt.want {
				t.Errorf("formatDurationBrief(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadFlywheelBrief_IncludesStigmergicScorecard(t *testing.T) {
	tmp := t.TempDir()
	for _, rel := range []string{
		filepath.Join(".agents", "findings"),
		filepath.Join(".agents", "planning-rules"),
		filepath.Join(".agents", "pre-mortem-checks"),
		filepath.Join(".agents", "rpi"),
	} {
		if err := os.MkdirAll(filepath.Join(tmp, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "findings", "f-1.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "planning-rules", "f-1.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "pre-mortem-checks", "f-1.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	queue := `{"source_epic":"ag-h83","timestamp":"2026-03-11T17:00:00Z","items":[{"title":"High one","type":"task","severity":"high","source":"council-finding","description":"d1","target_repo":"agentops","consumed":false}],"consumed":false,"claim_status":"available","claimed_by":null,"claimed_at":null,"consumed_by":null,"consumed_at":null}
`
	if err := os.WriteFile(filepath.Join(tmp, ".agents", "rpi", "next-work.jsonl"), []byte(queue), 0o644); err != nil {
		t.Fatal(err)
	}

	brief := loadFlywheelBrief(tmp)
	if brief == nil {
		t.Fatal("expected flywheel brief")
	}
	if brief.PromotedFindings != 1 || brief.PlanningRules != 1 || brief.PreMortemChecks != 1 {
		t.Fatalf("brief signal counts = %+v, want 1/1/1", brief)
	}
	if brief.UnconsumedItems != 1 || brief.HighSeverityUnconsumed != 1 {
		t.Fatalf("brief backlog counts = %+v, want 1/1", brief)
	}
}

func TestPrintFlywheelHealth_IncludesBacklogLine(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	printFlywheelHealth(&flywheelBrief{
		Status:                 "COMPOUNDING",
		TotalArtifacts:         10,
		Velocity:               1.2,
		NewArtifacts:           3,
		StaleArtifacts:         1,
		PromotedFindings:       2,
		PlanningRules:          2,
		PreMortemChecks:        1,
		UnconsumedItems:        7,
		HighSeverityUnconsumed: 3,
	})

	_ = w.Close()
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "Backlog:") {
		t.Fatalf("expected backlog line, got: %q", got)
	}
}

func TestLoadQualitySignals_ReturnsRecentValidEntries(t *testing.T) {
	tmp := t.TempDir()
	signalsDir := filepath.Join(tmp, ".agents", "signals")
	if err := os.MkdirAll(signalsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonl := strings.Join([]string{
		`{"timestamp":"2026-04-13T01:00:00Z","signal_type":"repeated_prompt","detail":"first","session_id":"s1"}`,
		`not-json`,
		`{"timestamp":"2026-04-13T01:01:00Z","signal_type":"correction","detail":"second","session_id":"s2"}`,
		`{"timestamp":"2026-04-13T01:02:00Z","signal_type":"repeated_prompt","detail":"third","session_id":"s3"}`,
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(signalsDir, "session-quality.jsonl"), []byte(jsonl), 0o644); err != nil {
		t.Fatal(err)
	}

	got := loadQualitySignals(filepath.Join(tmp, ".agents"), 2)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(got), got)
	}
	if got[0].Detail != "second" || got[1].Detail != "third" {
		t.Fatalf("details = %q, %q; want second, third", got[0].Detail, got[1].Detail)
	}
	if got[1].SessionID != "s3" || got[1].SignalType != "repeated_prompt" {
		t.Fatalf("last signal = %+v, want session s3 repeated_prompt", got[1])
	}
}

func TestLoadQualitySignals_MissingFileReturnsNil(t *testing.T) {
	got := loadQualitySignals(filepath.Join(t.TempDir(), ".agents"), 10)
	if got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}

func TestRunStatus_LoadsQualitySignalsFromAgentsRoot(t *testing.T) {
	resetCommandState(t)

	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".agents", "ao"), 0o755); err != nil {
		t.Fatal(err)
	}
	signalsDir := filepath.Join(tmp, ".agents", "signals")
	if err := os.MkdirAll(signalsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonl := `{"timestamp":"2026-04-13T01:02:00Z","signal_type":"correction","detail":"status should read agents root","session_id":"s1"}`
	if err := os.WriteFile(filepath.Join(signalsDir, "session-quality.jsonl"), []byte(jsonl+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWd)
	})

	got, err := captureStdout(t, func() error {
		return runStatus(statusCmd, nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Session Quality Signals") {
		t.Fatalf("expected quality signal section, got: %q", got)
	}
	if !strings.Contains(got, "status should read agents root") {
		t.Fatalf("expected signal from .agents/signals, got: %q", got)
	}
}

func TestLoadFlywheelBrief_TimesOut(t *testing.T) {
	oldTimeout := statusFlywheelTimeout
	oldCompute := statusComputeFlywheelBrief
	t.Cleanup(func() {
		statusFlywheelTimeout = oldTimeout
		statusComputeFlywheelBrief = oldCompute
	})

	statusFlywheelTimeout = 5 * time.Millisecond
	cancelObserved := make(chan struct{})
	statusComputeFlywheelBrief = func(ctx context.Context, _ string) *flywheelBrief {
		<-ctx.Done()
		close(cancelObserved)
		return &flywheelBrief{Status: "SHOULD_NOT_BLOCK"}
	}

	start := time.Now()
	got := loadFlywheelBrief(t.TempDir())
	elapsed := time.Since(start)

	if got != nil {
		t.Fatalf("got %+v, want nil after timeout", got)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("loadFlywheelBrief took %s, want bounded timeout", elapsed)
	}
	select {
	case <-cancelObserved:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("compute goroutine did not observe timeout cancellation")
	}
}

func TestOutputStatus_IncludesQualitySignalsHuman(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	err = outputStatus(&statusOutput{
		Initialized:  true,
		BaseDir:      filepath.Join(t.TempDir(), ".agents"),
		SessionCount: 0,
		QualitySignals: []qualitySignalInfo{{
			Timestamp:  "2026-04-13T01:02:00Z",
			SignalType: "correction",
			Detail:     "Prompt starts with correction pattern",
			SessionID:  "s1",
		}},
	})
	_ = w.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "Session Quality Signals") {
		t.Fatalf("expected quality signal section, got: %q", got)
	}
	if !strings.Contains(got, "correction") || !strings.Contains(got, "Prompt starts with correction pattern") {
		t.Fatalf("expected rendered quality signal, got: %q", got)
	}
}

// sampleNTMStatusJSON is a representative `ntm --robot-status` payload: a
// background-agent session with a claude pane plus a non-agent shell pane, and
// an unrelated session that must be ignored.
func sampleNTMStatusJSON(claudeLastOutput string) string {
	return `{
  "success": true,
  "sessions": [
    {
      "name": "agentops-bg",
      "panes": 2,
      "agents": [
        {"type":"unknown","pane_idx":1,"is_active":false,"process_state":"S","process_state_name":"sleeping","last_output_ts":"` + claudeLastOutput + `"},
        {"type":"claude","pane_idx":2,"is_active":true,"process_state":"S","process_state_name":"sleeping","last_output_ts":"` + claudeLastOutput + `","context_model":"claude-opus-4-5-20251101"}
      ]
    },
    {
      "name": "scratch",
      "panes": 1,
      "agents": [
        {"type":"codex","pane_idx":1,"is_active":true,"process_state":"S","process_state_name":"running","last_output_ts":"` + claudeLastOutput + `"}
      ]
    }
  ]
}`
}

func TestBackgroundAgents_ParseSampleNTMStatus(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-30 * time.Second).Format(time.RFC3339)
	agents := parseBackgroundAgents([]byte(sampleNTMStatusJSON(recent)), now)

	if len(agents) != 1 {
		t.Fatalf("len = %d, want 1 (only the claude pane in agentops-bg; shell pane + scratch session excluded)", len(agents))
	}
	a := agents[0]
	if a.Session != "agentops-bg" {
		t.Errorf("Session = %q, want agentops-bg", a.Session)
	}
	if a.Runtime != "claude" {
		t.Errorf("Runtime = %q, want claude", a.Runtime)
	}
	if a.Mailbox != "agentops-claude-ntm-worker" {
		t.Errorf("Mailbox = %q, want agentops-claude-ntm-worker", a.Mailbox)
	}
	if a.PaneIdx != 2 {
		t.Errorf("PaneIdx = %d, want 2", a.PaneIdx)
	}
	if !a.Active {
		t.Error("Active = false, want true for the claude pane")
	}
	if a.Health != "ok" {
		t.Errorf("Health = %q, want ok", a.Health)
	}
	if a.ContextModel != "claude-opus-4-5-20251101" {
		t.Errorf("ContextModel = %q, want claude-opus-4-5-20251101", a.ContextModel)
	}
	if a.LastCheckIn == "" {
		t.Error("LastCheckIn empty, want a derived '<dur> ago' string")
	}
	// Bead/branch are not carried by NTM; they must stay empty, not fabricated.
	if a.Bead != "" || a.Branch != "" {
		t.Errorf("Bead=%q Branch=%q, want both empty (not derivable from NTM)", a.Bead, a.Branch)
	}
}

func TestBackgroundAgents_NoNTMFallback(t *testing.T) {
	prev := statusNTMRunner
	t.Cleanup(func() { statusNTMRunner = prev })

	// ntm binary missing / errors -> fail-open to nil.
	statusNTMRunner = func(context.Context) ([]byte, error) {
		return nil, errors.New("exec: \"ntm\": executable file not found in $PATH")
	}
	if got := loadBackgroundAgents(context.Background(), time.Now()); got != nil {
		t.Fatalf("no-NTM fallback = %+v, want nil", got)
	}

	// Garbage JSON also fails open rather than erroring the whole status command.
	statusNTMRunner = func(context.Context) ([]byte, error) {
		return []byte("not json"), nil
	}
	if got := loadBackgroundAgents(context.Background(), time.Now()); got != nil {
		t.Fatalf("invalid-JSON fallback = %+v, want nil", got)
	}
}

func TestBackgroundAgents_LoadFromInjectedNTM(t *testing.T) {
	prev := statusNTMRunner
	t.Cleanup(func() { statusNTMRunner = prev })
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-10 * time.Second).Format(time.RFC3339)
	statusNTMRunner = func(context.Context) ([]byte, error) {
		return []byte(sampleNTMStatusJSON(recent)), nil
	}
	got := loadBackgroundAgents(context.Background(), now)
	if len(got) != 1 || got[0].Runtime != "claude" {
		t.Fatalf("loadBackgroundAgents = %+v, want one claude agent", got)
	}
}

func TestBackgroundAgents_HealthStuckAndError(t *testing.T) {
	now := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)

	ok := bgNTMAgent{ProcessState: "S", ProcessStateName: "sleeping", LastOutputTS: now.Add(-1 * time.Minute).Format(time.RFC3339)}
	if got := classifyBackgroundHealth(ok, now); got != "ok" {
		t.Errorf("recent pane health = %q, want ok", got)
	}

	stuck := bgNTMAgent{ProcessState: "S", ProcessStateName: "sleeping", LastOutputTS: now.Add(-30 * time.Minute).Format(time.RFC3339)}
	if got := classifyBackgroundHealth(stuck, now); got != "stuck" {
		t.Errorf("silent pane health = %q, want stuck", got)
	}

	zombie := bgNTMAgent{ProcessState: "Z", ProcessStateName: "zombie", LastOutputTS: now.Format(time.RFC3339)}
	if got := classifyBackgroundHealth(zombie, now); got != "error" {
		t.Errorf("zombie pane health = %q, want error", got)
	}
}

func TestBackgroundAgents_OutputStatusHumanSection(t *testing.T) {
	got, err := captureStdout(t, func() error {
		return outputStatus(&statusOutput{
			Initialized: true,
			BaseDir:     filepath.Join(t.TempDir(), ".agents"),
			BackgroundAgents: []backgroundAgentInfo{{
				Session:      "agentops-bg",
				Runtime:      "claude",
				Mailbox:      "agentops-claude-ntm-worker",
				PaneIdx:      2,
				Active:       true,
				Health:       "ok",
				LastCheckIn:  "<1m ago",
				ContextModel: "claude-opus-4-5-20251101",
			}},
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Background Agents (NTM)", "agentops-bg", "runtime=claude", "mailbox=agentops-claude-ntm-worker", "bead=-", "branch=-"} {
		if !strings.Contains(got, want) {
			t.Fatalf("background-agent section missing %q:\n%s", want, got)
		}
	}
}
