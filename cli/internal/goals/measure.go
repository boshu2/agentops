package goals

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/boshu2/agentops/cli/internal/shellutil"
)

// Measurement captures the result of running a single goal's check command.
type Measurement struct {
	GoalID    string   `json:"goal_id"`
	Result    string   `json:"result"` // "pass", "fail", "skip"
	Value     *float64 `json:"value,omitempty"`
	Threshold *float64 `json:"threshold,omitempty"`
	Duration  float64  `json:"duration_s"`
	Output    string   `json:"output,omitempty"`
	Weight    int      `json:"weight"`
}

// classifyResult maps command exit status to a result string.
func classifyResult(ctxErr, cmdErr error) string {
	switch {
	case errors.Is(ctxErr, context.DeadlineExceeded):
		return resultSkip
	case cmdErr != nil:
		return resultFail
	default:
		return resultPass
	}
}

// truncateOutput limits output to 500 runes and trims whitespace.
// Uses rune-aware truncation to avoid splitting multi-byte UTF-8 characters.
// Note: ASCII-only strings could use a len(s) fast-path, but rune safety is preferred.
func truncateOutput(raw []byte) string {
	s := string(raw)
	if utf8.RuneCountInString(s) > 500 {
		runes := []rune(s)
		s = string(runes[:500])
	}
	return strings.TrimSpace(s)
}

// applyContinuousMetric parses a numeric value from output for continuous goals.
func applyContinuousMetric(m *Measurement, goal Goal) {
	if goal.Continuous == nil || m.Output == "" {
		return
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(m.Output), 64); err == nil {
		m.Value = &v
		t := goal.Continuous.Threshold
		m.Threshold = &t
	}
}

// childGroups tracks process group IDs of running gate commands so they can
// be killed if the parent process receives a signal.
var childGroups struct {
	mu   sync.Mutex
	pids map[int]struct{}
}

func init() { childGroups.pids = make(map[int]struct{}) }

func trackChild(pid int) {
	childGroups.mu.Lock()
	defer childGroups.mu.Unlock()
	childGroups.pids[pid] = struct{}{}
}

func untrackChild(pid int) {
	childGroups.mu.Lock()
	defer childGroups.mu.Unlock()
	delete(childGroups.pids, pid)
}

// killAllChildren is implemented in measure_unix.go and measure_windows.go
// using platform-specific process termination (POSIX signals vs taskkill).

// MeasureOne runs a single goal's check command and returns a Measurement.
// Exit 0 = pass, non-zero = fail, context deadline exceeded = skip.
// Uses process groups so child processes are killed on timeout.
func MeasureOne(goal Goal, timeout time.Duration) Measurement {
	m := Measurement{GoalID: goal.ID, Weight: goal.Weight}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	// SanitizedBashCommand bypasses ~/.bashrc and BASH_ENV so user shell
	// aliases cannot silently change the meaning of goal check strings.
	cmd := shellutil.SanitizedBashCommand(ctx, goal.Check)
	configureProcGroup(cmd)
	cmd.WaitDelay = 3 * time.Second

	// Capture combined stdout+stderr via buffer so we can track the PID
	// between Start and Wait for signal-based cleanup.
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		m.Duration = time.Since(start).Seconds()
		m.Result = resultFail
		m.Output = err.Error()
		return m
	}

	trackChild(cmd.Process.Pid)
	err := cmd.Wait()
	untrackChild(cmd.Process.Pid)

	m.Duration = time.Since(start).Seconds()
	m.Output = truncateOutput(buf.Bytes())
	m.Result = classifyResult(ctx.Err(), err)
	applyContinuousMetric(&m, goal)
	return m
}

// Measure runs all goals and returns a Snapshot. Meta-goals run first, then all others.
func Measure(gf *GoalFile, timeout time.Duration) *Snapshot {
	measurements := runGoals(gf.Goals, timeout)
	return &Snapshot{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		GitSHA:    gitSHA(),
		Goals:     measurements,
		Summary:   computeSummary(measurements),
	}
}

// maxParallelGoals limits concurrent goal checks to avoid resource contention.
// Keep low — heavy gates (go test, go build) compete for CPU.
const maxParallelGoals = 2

// requiresExclusiveExecution marks test-heavy gates that should not overlap
// with other goal checks because they contend on the same module/worktree.
func requiresExclusiveExecution(goal Goal) bool {
	check := strings.ToLower(goal.Check)
	return strings.Contains(check, "go test") ||
		strings.Contains(check, "check-cmdao-coverage-floor.sh")
}

// osExitFn is the exit function called on signal. Override in tests to
// avoid terminating the test process.
var osExitFn = os.Exit

// runGoals executes meta-goals first (sequential), then non-meta goals (parallel).
// Installs a signal handler to kill all child process groups on SIGINT/SIGTERM.
func runGoals(allGoals []Goal, timeout time.Duration) []Measurement {
	// Install signal handler to kill children on interrupt.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case <-sigCh:
			killAllChildren()
			osExitFn(130) // 128 + SIGINT(2)
		case <-done:
			return
		}
	}()
	defer func() {
		signal.Stop(sigCh)
		close(done)
	}()

	// Phase 1: meta-goals run sequentially (they may affect non-meta goals).
	var measurements []Measurement
	for _, g := range allGoals {
		if g.Type == GoalTypeMeta {
			measurements = append(measurements, MeasureOne(g, timeout))
		}
	}

	// Phase 2: non-meta goals run concurrently with a semaphore.
	var nonMeta []Goal
	for _, g := range allGoals {
		if g.Type != GoalTypeMeta {
			nonMeta = append(nonMeta, g)
		}
	}
	if len(nonMeta) == 0 {
		return measurements
	}

	results := make([]Measurement, len(nonMeta))
	sem := make(chan struct{}, maxParallelGoals)
	var exclusive sync.RWMutex
	var wg sync.WaitGroup
	for i, g := range nonMeta {
		wg.Add(1)
		go func(idx int, goal Goal) {
			defer wg.Done()

			if requiresExclusiveExecution(goal) {
				// Acquire semaphore BEFORE exclusive lock to prevent deadlock:
				// if Lock() is acquired first, readers holding sem slots and
				// waiting for RLock would block the writer from getting a slot.
				sem <- struct{}{}
				defer func() { <-sem }()
				exclusive.Lock()
				defer exclusive.Unlock()
				results[idx] = MeasureOne(goal, timeout)
				return
			}

			sem <- struct{}{}
			defer func() { <-sem }()
			exclusive.RLock()
			defer exclusive.RUnlock()
			results[idx] = MeasureOne(goal, timeout)
		}(i, g)
	}
	wg.Wait()

	return append(measurements, results...)
}

// computeSummary aggregates pass/fail/skip counts and weighted score.
func computeSummary(measurements []Measurement) SnapshotSummary {
	var summary SnapshotSummary
	summary.Total = len(measurements)
	var weightedPass, weightedTotal int
	for _, m := range measurements {
		switch m.Result {
		case resultPass:
			summary.Passing++
			weightedPass += m.Weight
			weightedTotal += m.Weight
		case resultFail:
			summary.Failing++
			weightedTotal += m.Weight
		case resultSkip:
			summary.Skipped++
		}
	}
	if weightedTotal > 0 {
		summary.Score = float64(weightedPass) / float64(weightedTotal) * 100
	}
	return summary
}

const gitSHATimeout = 2 * time.Second

// gitSHA returns the short git SHA of HEAD, or "" on error.
func gitSHA() string {
	return gitSHAWithTimeout(gitSHATimeout)
}

func gitSHAWithTimeout(timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	// Bound pipe-drain waits after cancellation so wrapper scripts cannot stall timeout handling.
	cmd.WaitDelay = timeout

	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
