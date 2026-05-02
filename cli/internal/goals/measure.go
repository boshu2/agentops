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
	Tags      []string `json:"tags,omitempty"`
}

// classifyResult maps command exit status to a result string.
func classifyResult(ctxErr, cmdErr error) string {
	switch {
	case errors.Is(ctxErr, context.DeadlineExceeded), errors.Is(ctxErr, context.Canceled):
		return resultSkip
	case cmdErr != nil:
		return resultFail
	default:
		return resultPass
	}
}

// truncateOutput caps output at 500 runes by keeping the first 200 and last
// 200 runes joined by a truncation marker, then trims whitespace.
// Diagnostic gate output (e.g. check-flywheel-compounding.sh) often puts the
// operator hint at the END of stdout; head-only truncation cuts that hint.
// The head+tail shape preserves both the failure label and the trailing fix.
const (
	truncateLimit  = 500
	truncateHead   = 200
	truncateTail   = 200
	truncateMarker = "\n…[truncated]…\n"
)

func truncateOutput(raw []byte) string {
	s := string(raw)
	// Fast path: byte length is an upper bound on rune count, so any output
	// whose byte length is already within the cap needs no rune counting or
	// []rune allocation. Hot paths (successful gates with short stdout)
	// always take this branch.
	if len(s) > truncateLimit && utf8.RuneCountInString(s) > truncateLimit {
		runes := []rune(s)
		head := string(runes[:truncateHead])
		tail := string(runes[len(runes)-truncateTail:])
		s = head + truncateMarker + tail
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
	return MeasureOneContext(context.Background(), goal, timeout)
}

// MeasureOneContext runs a single goal's check command under both a caller
// context and a per-goal timeout.
func MeasureOneContext(parent context.Context, goal Goal, timeout time.Duration) Measurement {
	m := Measurement{GoalID: goal.ID, Weight: goal.Weight, Tags: goal.Tags}
	if parent == nil {
		parent = context.Background()
	}
	if err := parent.Err(); err != nil {
		return skippedMeasurement(goal, err)
	}

	ctx, cancel := context.WithTimeout(parent, timeout)
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
		m.Output = err.Error()
		m.Result = classifyResult(ctx.Err(), err)
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
	return measureWithContext(context.Background(), gf, timeout)
}

// MeasureWithTotalTimeout runs all goals under a whole-measurement timeout.
// A non-positive totalTimeout keeps the historical unbounded whole-run behavior.
func MeasureWithTotalTimeout(gf *GoalFile, timeout, totalTimeout time.Duration) *Snapshot {
	ctx := context.Background()
	var cancel context.CancelFunc
	if totalTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, totalTimeout)
		defer cancel()
	}
	return measureWithContext(ctx, gf, timeout)
}

func measureWithContext(ctx context.Context, gf *GoalFile, timeout time.Duration) *Snapshot {
	measurements := runGoalsWithContext(ctx, gf.Goals, timeout)
	return &Snapshot{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		GitSHA:    gitSHA(),
		Goals:     measurements,
		Summary:   computeSummary(measurements),
	}
}

func skippedMeasurement(goal Goal, err error) Measurement {
	output := "measurement skipped"
	if err != nil {
		output += ": " + err.Error()
	}
	return Measurement{
		GoalID: goal.ID,
		Result: resultSkip,
		Output: output,
		Weight: goal.Weight,
		Tags:   goal.Tags,
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
	return runGoalsWithContext(context.Background(), allGoals, timeout)
}

func runGoalsWithContext(ctx context.Context, allGoals []Goal, timeout time.Duration) []Measurement {
	if ctx == nil {
		ctx = context.Background()
	}
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
			if err := ctx.Err(); err != nil {
				measurements = append(measurements, skippedMeasurement(g, err))
				continue
			}
			measurements = append(measurements, MeasureOneContext(ctx, g, timeout))
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
				if !acquireGoalSlot(ctx, sem) {
					results[idx] = skippedMeasurement(goal, ctx.Err())
					return
				}
				defer func() { <-sem }()
				exclusive.Lock()
				defer exclusive.Unlock()
				if err := ctx.Err(); err != nil {
					results[idx] = skippedMeasurement(goal, err)
					return
				}
				results[idx] = MeasureOneContext(ctx, goal, timeout)
				return
			}

			if !acquireGoalSlot(ctx, sem) {
				results[idx] = skippedMeasurement(goal, ctx.Err())
				return
			}
			defer func() { <-sem }()
			exclusive.RLock()
			defer exclusive.RUnlock()
			if err := ctx.Err(); err != nil {
				results[idx] = skippedMeasurement(goal, err)
				return
			}
			results[idx] = MeasureOneContext(ctx, goal, timeout)
		}(i, g)
	}
	wg.Wait()

	return append(measurements, results...)
}

func acquireGoalSlot(ctx context.Context, sem chan struct{}) bool {
	select {
	case sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
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
