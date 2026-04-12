package goals

import (
	"context"
	"errors"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestMeasureOne_Pass(t *testing.T) {
	goal := Goal{
		ID:     "test-pass",
		Check:  "exit 0",
		Weight: 5,
	}
	m := MeasureOne(goal, 5*time.Second)
	if m.Result != "pass" {
		t.Errorf("expected pass, got %q", m.Result)
	}
	if m.GoalID != "test-pass" {
		t.Errorf("GoalID = %q, want %q", m.GoalID, "test-pass")
	}
	if m.Weight != 5 {
		t.Errorf("Weight = %d, want 5", m.Weight)
	}
	if m.Duration < 0 {
		t.Errorf("Duration should be >= 0, got %f", m.Duration)
	}
}

func TestMeasureOne_Fail(t *testing.T) {
	goal := Goal{
		ID:     "test-fail",
		Check:  "exit 1",
		Weight: 3,
	}
	m := MeasureOne(goal, 5*time.Second)
	if m.Result != "fail" {
		t.Errorf("expected fail, got %q", m.Result)
	}
}

func TestMeasureOne_Timeout(t *testing.T) {
	goal := Goal{
		ID:     "test-timeout",
		Check:  "sleep 10",
		Weight: 1,
	}
	m := MeasureOne(goal, 100*time.Millisecond)
	if m.Result != "skip" {
		t.Errorf("expected skip on timeout, got %q", m.Result)
	}
}

func TestMeasureOne_OutputTruncated(t *testing.T) {
	goal := Goal{
		ID:     "test-truncate",
		Check:  "printf '%600s' | tr ' ' 'A'",
		Weight: 1,
	}
	m := MeasureOne(goal, 5*time.Second)
	if len([]rune(m.Output)) > 500 {
		t.Errorf("output should be truncated to 500 runes, got %d", len([]rune(m.Output)))
	}
}

func TestMeasureOne_ContinuousMetric_ParsesValue(t *testing.T) {
	threshold := 0.5
	goal := Goal{
		ID:     "test-continuous",
		Check:  "echo 0.75",
		Weight: 2,
		Continuous: &ContinuousMetric{
			Metric:    "my_metric",
			Threshold: threshold,
		},
	}
	m := MeasureOne(goal, 5*time.Second)
	if m.Value == nil {
		t.Fatal("expected Value to be set for continuous metric")
	}
	if *m.Value != 0.75 {
		t.Errorf("Value = %f, want 0.75", *m.Value)
	}
	if m.Threshold == nil {
		t.Fatal("expected Threshold to be set for continuous metric")
	}
	if *m.Threshold != threshold {
		t.Errorf("Threshold = %f, want %f", *m.Threshold, threshold)
	}
}

func TestMeasureOne_ContinuousMetric_NonNumericOutput(t *testing.T) {
	goal := Goal{
		ID:     "test-nonnumeric",
		Check:  "echo hello",
		Weight: 1,
		Continuous: &ContinuousMetric{
			Metric:    "my_metric",
			Threshold: 0.5,
		},
	}
	m := MeasureOne(goal, 5*time.Second)
	if m.Value != nil {
		t.Errorf("expected Value to be nil for non-numeric output, got %f", *m.Value)
	}
}

func TestClassifyResult(t *testing.T) {
	tests := []struct {
		name   string
		ctxErr error
		cmdErr error
		want   string
	}{
		{name: "pass", want: resultPass},
		{name: "fail", cmdErr: errors.New("boom"), want: resultFail},
		{name: "timeout", ctxErr: context.DeadlineExceeded, want: resultSkip},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyResult(tt.ctxErr, tt.cmdErr); got != tt.want {
				t.Fatalf("classifyResult(%v, %v) = %q, want %q", tt.ctxErr, tt.cmdErr, got, tt.want)
			}
		})
	}
}

func TestTruncateOutput_TrimWithoutTruncation(t *testing.T) {
	got := truncateOutput([]byte("  ok  \n"))
	if got != "ok" {
		t.Fatalf("truncateOutput returned %q, want %q", got, "ok")
	}
}

func TestApplyContinuousMetric(t *testing.T) {
	goal := Goal{
		Continuous: &ContinuousMetric{
			Metric:    "coverage",
			Threshold: 90.0,
		},
	}

	m := &Measurement{Output: "91.5"}
	applyContinuousMetric(m, goal)
	if m.Value == nil || *m.Value != 91.5 {
		t.Fatalf("Value = %v, want 91.5", m.Value)
	}
	if m.Threshold == nil || *m.Threshold != 90.0 {
		t.Fatalf("Threshold = %v, want 90.0", m.Threshold)
	}

	blank := &Measurement{Output: ""}
	applyContinuousMetric(blank, goal)
	if blank.Value != nil || blank.Threshold != nil {
		t.Fatal("blank output should not set continuous metric values")
	}
}

func TestMeasure_MetaGoalsRunFirst(t *testing.T) {
	gf := &GoalFile{
		Version: 2,
		Goals: []Goal{
			{ID: "non-meta-1", Check: "exit 0", Weight: 1, Type: GoalTypeHealth},
			{ID: "meta-1", Check: "exit 0", Weight: 1, Type: GoalTypeMeta},
			{ID: "non-meta-2", Check: "exit 0", Weight: 1, Type: GoalTypeQuality},
		},
	}
	snap := Measure(gf, 5*time.Second)
	if len(snap.Goals) != 3 {
		t.Fatalf("expected 3 measurements, got %d", len(snap.Goals))
	}
	if snap.Goals[0].GoalID != "meta-1" {
		t.Errorf("expected meta-1 first, got %q", snap.Goals[0].GoalID)
	}
}

func TestMeasure_SummaryCorrect(t *testing.T) {
	gf := &GoalFile{
		Version: 2,
		Goals: []Goal{
			{ID: "pass-1", Check: "exit 0", Weight: 5, Type: GoalTypeHealth},
			{ID: "pass-2", Check: "exit 0", Weight: 3, Type: GoalTypeHealth},
			{ID: "fail-1", Check: "exit 1", Weight: 2, Type: GoalTypeHealth},
		},
	}
	snap := Measure(gf, 5*time.Second)
	if snap.Summary.Total != 3 {
		t.Errorf("Total = %d, want 3", snap.Summary.Total)
	}
	if snap.Summary.Passing != 2 {
		t.Errorf("Passing = %d, want 2", snap.Summary.Passing)
	}
	if snap.Summary.Failing != 1 {
		t.Errorf("Failing = %d, want 1", snap.Summary.Failing)
	}
	if snap.Summary.Score != 80.0 {
		t.Errorf("Score = %f, want 80.0", snap.Summary.Score)
	}
}

func TestMeasure_SkippedGoalsExcludedFromScore(t *testing.T) {
	gf := &GoalFile{
		Version: 2,
		Goals: []Goal{
			{ID: "pass-1", Check: "exit 0", Weight: 5, Type: GoalTypeHealth},
			{ID: "skip-1", Check: "sleep 5", Weight: 10, Type: GoalTypeHealth},
		},
	}
	snap := Measure(gf, 2*time.Second)
	if snap.Summary.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", snap.Summary.Skipped)
	}
	if snap.Summary.Score != 100.0 {
		t.Errorf("Score = %f, want 100.0 (skipped excluded)", snap.Summary.Score)
	}
}

func TestMeasure_EmptyGoals(t *testing.T) {
	gf := &GoalFile{Version: 2, Goals: []Goal{}}
	snap := Measure(gf, 5*time.Second)
	if snap.Summary.Total != 0 {
		t.Errorf("Total = %d, want 0", snap.Summary.Total)
	}
	if snap.Summary.Score != 0 {
		t.Errorf("Score = %f, want 0 for empty goals", snap.Summary.Score)
	}
	if snap.Timestamp == "" {
		t.Error("Timestamp should not be empty")
	}
}

func TestRequiresExclusiveExecution(t *testing.T) {
	if !requiresExclusiveExecution(Goal{Check: "go test ./..."}) {
		t.Fatal("go test checks should require exclusive execution")
	}
	if !requiresExclusiveExecution(Goal{Check: "./scripts/check-cmdao-coverage-floor.sh"}) {
		t.Fatal("coverage floor check should require exclusive execution")
	}
	if requiresExclusiveExecution(Goal{Check: "echo ok"}) {
		t.Fatal("simple commands should not require exclusive execution")
	}
}

func TestComputeSummary_AllOutcomes(t *testing.T) {
	summary := computeSummary([]Measurement{
		{Result: resultPass, Weight: 3},
		{Result: resultFail, Weight: 1},
		{Result: resultSkip, Weight: 9},
	})
	if summary.Total != 3 || summary.Passing != 1 || summary.Failing != 1 || summary.Skipped != 1 {
		t.Fatalf("unexpected summary counts: %+v", summary)
	}
	if summary.Score != 75.0 {
		t.Fatalf("Score = %f, want 75.0", summary.Score)
	}
}

func TestTruncateOutput_MultiByteRunes(t *testing.T) {
	runes := make([]rune, 501)
	for i := range runes {
		runes[i] = '世'
	}
	input := []byte(string(runes))

	result := truncateOutput(input)
	runeCount := len([]rune(result))
	if runeCount > 500 {
		t.Errorf("expected <=500 runes, got %d", runeCount)
	}
	if runeCount < 500 {
		t.Errorf("expected exactly 500 runes (not fewer), got %d", runeCount)
	}
	for i, r := range result {
		if r == '\uFFFD' {
			t.Errorf("invalid UTF-8 at byte %d (replacement character found)", i)
			break
		}
	}
}

// TestMeasureOne_StartError exercises the cmd.Start() error path in MeasureOne
// (lines 109-113). When bash itself cannot start, the function should return
// a fail result with the error message in Output.
func TestMeasureOne_StartError(t *testing.T) {
	// Save current PATH and set it to empty to make "bash" unresolvable.
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	defer os.Setenv("PATH", origPath)

	goal := Goal{
		ID:     "start-error",
		Check:  "echo hello",
		Weight: 4,
	}
	m := MeasureOne(goal, 5*time.Second)
	if m.Result != "fail" {
		t.Errorf("Result = %q, want %q for start error", m.Result, "fail")
	}
	if m.GoalID != "start-error" {
		t.Errorf("GoalID = %q, want %q", m.GoalID, "start-error")
	}
	if m.Weight != 4 {
		t.Errorf("Weight = %d, want 4", m.Weight)
	}
	if m.Output == "" {
		t.Error("Output should contain the start error message")
	}
	if m.Duration < 0 {
		t.Errorf("Duration should be >= 0, got %f", m.Duration)
	}
}

// TestRunGoals_OnlyMetaGoals_EarlyReturn exercises the early return at line 186
// when all goals are meta-type and the nonMeta slice is empty.
func TestRunGoals_OnlyMetaGoals_EarlyReturn(t *testing.T) {
	goals := []Goal{
		{ID: "meta-a", Check: "echo a", Weight: 2, Type: GoalTypeMeta},
		{ID: "meta-b", Check: "echo b", Weight: 3, Type: GoalTypeMeta},
	}
	measurements := runGoals(goals, 5*time.Second)
	if len(measurements) != 2 {
		t.Fatalf("got %d measurements, want 2", len(measurements))
	}
	if measurements[0].GoalID != "meta-a" {
		t.Errorf("measurements[0].GoalID = %q, want %q", measurements[0].GoalID, "meta-a")
	}
	if measurements[1].GoalID != "meta-b" {
		t.Errorf("measurements[1].GoalID = %q, want %q", measurements[1].GoalID, "meta-b")
	}
	for i, m := range measurements {
		if m.Result != "pass" {
			t.Errorf("measurements[%d].Result = %q, want %q", i, m.Result, "pass")
		}
	}
}

// TestRunGoals_EmptyGoals exercises runGoals with no goals at all.
func TestRunGoals_EmptyGoals(t *testing.T) {
	measurements := runGoals([]Goal{}, 5*time.Second)
	if len(measurements) != 0 {
		t.Errorf("got %d measurements, want 0 for empty goals", len(measurements))
	}
}

func TestGitSHA_OutsideGitRepo(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(origDir) //nolint:errcheck
	}()

	sha := gitSHA()
	if sha != "" {
		t.Errorf("expected empty SHA outside git repo, got %q", sha)
	}
}

func TestChildGroupsInitialized(t *testing.T) {
	// Bug #7: childGroups.pids should be non-nil at package init time.
	// Before the fix, it starts nil and relies on lazy init in trackChild.
	if childGroups.pids == nil {
		t.Fatal("childGroups.pids is nil at package init; expected eager initialization")
	}
}

func TestRunGoals_SignalHandlerCallsExit(t *testing.T) {
	// Exercise the signal handler branch in runGoals: when a SIGINT arrives
	// during goal execution, the handler calls killAllChildren() and osExitFn(130).
	// We override osExitFn to capture the exit code instead of terminating.
	var exitCode int
	exitCalled := make(chan struct{})
	origExit := osExitFn
	osExitFn = func(code int) {
		exitCode = code
		close(exitCalled)
		// Block forever so the goroutine doesn't return and cause races.
		select {}
	}
	defer func() { osExitFn = origExit }()

	// Use a goal that sleeps long enough for us to send a signal.
	goals := []Goal{
		{ID: "slow", Check: "sleep 30", Weight: 1, Type: GoalTypeHealth},
	}

	done := make(chan struct{})
	go func() {
		runGoals(goals, 30*time.Second)
		close(done)
	}()

	// Give the goroutine time to start and install the signal handler,
	// then send SIGINT to ourselves.
	time.Sleep(100 * time.Millisecond)
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("FindProcess: %v", err)
	}
	if err := proc.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("sending SIGINT: %v", err)
	}

	select {
	case <-exitCalled:
		if exitCode != 130 {
			t.Errorf("exit code = %d, want 130", exitCode)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for signal handler to call osExitFn")
	}
}

func TestTrackChild_ConcurrentAccess(t *testing.T) {
	// Bug #7: Verify trackChild/untrackChild are safe under concurrent access.
	// Must pass with -race flag.
	const goroutines = 10

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		pid := 10000 + i
		go func(p int) {
			defer wg.Done()
			trackChild(p)
		}(pid)
		go func(p int) {
			defer wg.Done()
			untrackChild(p)
		}(pid)
	}
	wg.Wait()

	// Clean up: remove any leftover tracked pids from this test.
	childGroups.mu.Lock()
	for pid := 10000; pid < 10000+goroutines; pid++ {
		delete(childGroups.pids, pid)
	}
	childGroups.mu.Unlock()
}
