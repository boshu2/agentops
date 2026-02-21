package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Signal represents a detected outcome signal from a transcript.
type Signal struct {
	Name   string  `json:"name"`
	Value  bool    `json:"value"`
	Weight float64 `json:"weight"`
}

// SessionOutcome represents the analyzed outcome of a session.
type SessionOutcome struct {
	SessionID  string    `json:"session_id"`
	Reward     float64   `json:"reward"`
	Signals    []Signal  `json:"signals"`
	AnalyzedAt time.Time `json:"analyzed_at"`
	Transcript string    `json:"transcript,omitempty"`
	TotalLines int       `json:"total_lines,omitempty"`
}

// Signal weights (positive and negative)
const (
	// Positive signals
	weightTestsPass   = 0.30
	weightGitPush     = 0.20
	weightGitCommit   = 0.15
	weightBeadsClosed = 0.15
	weightRatchetLock = 0.10
	weightNoErrors    = 0.10

	// Penalty signals (negative)
	penaltyTestFailure = 0.20
	penaltyException   = 0.15
	penaltyNoCommit    = 0.10
)

// signalPatterns are pre-compiled regex patterns for signal detection.
// Compiled once at package init to avoid per-call overhead.
var signalPatterns = struct {
	testPass      *regexp.Regexp
	testFail      *regexp.Regexp
	gitPush       *regexp.Regexp
	gitCommit     *regexp.Regexp
	beadsClose    *regexp.Regexp
	ratchetRecord *regexp.Regexp
	pythonTrace   *regexp.Regexp
	goPanic       *regexp.Regexp
	exitZero      *regexp.Regexp
	exitNonZero   *regexp.Regexp
}{
	testPass:      regexp.MustCompile(`(?i)(PASSED|tests? passed|✓|ok$|All tests passed)`),
	testFail:      regexp.MustCompile(`(?i)(FAILED|FAILURE|tests? failed|✗|ERROR.*test)`),
	gitPush:       regexp.MustCompile(`(?i)(git push|pushed to|Enumerating objects:.*done.*Writing objects:)`),
	gitCommit:     regexp.MustCompile(`(?i)(git commit|\[.+\]\s+\d+ files? changed|\[\w+\s+\w+\])`),
	beadsClose:    regexp.MustCompile(`(?i)(bd close|issue.*closed|status.*closed)`),
	ratchetRecord: regexp.MustCompile(`(?i)(ol ratchet record|ratchet.*locked|Ratchet chain updated)`),
	pythonTrace:   regexp.MustCompile(`(?i)(Traceback \(most recent call|raise \w+Error|^\s+File ".+", line \d+)`),
	goPanic:       regexp.MustCompile(`(?i)(panic:|runtime error:|goroutine \d+ \[)`),
	exitZero:      regexp.MustCompile(`exit (code|status):?\s*0`),
	exitNonZero:   regexp.MustCompile(`exit (code|status):?\s*[1-9]\d*`),
}

// signalState holds detection state during transcript scanning.
type signalState struct {
	testsFound       bool
	testsPassed      bool
	testsFailed      bool
	gitPushFound     bool
	gitCommitFound   bool
	beadsClosedFound bool
	ratchetLockFound bool
	exceptionsFound  bool
}

// detectTestSignals checks for test-related signals in a line.
func (s *signalState) detectTestSignals(line string) {
	if signalPatterns.testPass.MatchString(line) || signalPatterns.exitZero.MatchString(line) {
		s.testsFound = true
		s.testsPassed = true
	}
	if signalPatterns.testFail.MatchString(line) || signalPatterns.exitNonZero.MatchString(line) {
		s.testsFound = true
		s.testsFailed = true
	}
}

// detectGitSignals checks for git-related signals.
func (s *signalState) detectGitSignals(line string) {
	if signalPatterns.gitPush.MatchString(line) {
		s.gitPushFound = true
	}
	if signalPatterns.gitCommit.MatchString(line) {
		s.gitCommitFound = true
	}
}

// detectWorkflowSignals checks for beads/ratchet signals.
func (s *signalState) detectWorkflowSignals(line string) {
	if signalPatterns.beadsClose.MatchString(line) {
		s.beadsClosedFound = true
	}
	if signalPatterns.ratchetRecord.MatchString(line) {
		s.ratchetLockFound = true
	}
}

// detectErrorSignals checks for exceptions/panics.
func (s *signalState) detectErrorSignals(line string) {
	if signalPatterns.pythonTrace.MatchString(line) || signalPatterns.goPanic.MatchString(line) {
		s.exceptionsFound = true
	}
}

// addSignal appends a signal and updates reward.
func addSignal(signals *[]Signal, reward *float64, name string, found bool, weight float64) {
	if found {
		*signals = append(*signals, Signal{Name: name, Value: true, Weight: weight})
		*reward += weight
	} else {
		*signals = append(*signals, Signal{Name: name, Value: false, Weight: 0})
	}
}

// addPenalty appends a penalty signal and updates reward.
func addPenalty(signals *[]Signal, reward *float64, name string, condition bool, penalty float64) {
	if condition {
		*signals = append(*signals, Signal{Name: name, Value: true, Weight: -penalty})
		*reward -= penalty
	}
}

// clampReward constrains reward to [0, 1].
func clampReward(reward float64) float64 {
	if reward < 0 {
		return 0
	}
	if reward > 1 {
		return 1
	}
	return reward
}

// buildSignals converts detection state to reward signals.
func (s *signalState) buildSignals() ([]Signal, float64) {
	var signals []Signal
	reward := 0.0

	// Test signal has special logic
	testsPassed := s.testsFound && s.testsPassed && !s.testsFailed
	if testsPassed {
		addSignal(&signals, &reward, "tests_pass", true, weightTestsPass)
	} else if s.testsFound {
		signals = append(signals, Signal{Name: "tests_pass", Value: false, Weight: 0})
	}

	// Positive signals
	addSignal(&signals, &reward, "git_push", s.gitPushFound, weightGitPush)
	addSignal(&signals, &reward, "git_commit", s.gitCommitFound, weightGitCommit)
	addSignal(&signals, &reward, "beads_closed", s.beadsClosedFound, weightBeadsClosed)
	addSignal(&signals, &reward, "ratchet_lock", s.ratchetLockFound, weightRatchetLock)
	addSignal(&signals, &reward, "no_errors", !s.exceptionsFound, weightNoErrors)

	// Penalties
	addPenalty(&signals, &reward, "test_failure", s.testsFound && s.testsFailed, penaltyTestFailure)
	addPenalty(&signals, &reward, "exceptions", s.exceptionsFound, penaltyException)
	addPenalty(&signals, &reward, "no_commit", !s.gitCommitFound, penaltyNoCommit)

	return signals, clampReward(reward)
}

var sessionOutcomeCmd = &cobra.Command{
	Use:   "session-outcome [transcript-path]",
	Short: "Analyze session transcript to derive reward signal",
	Long: `Parse a Claude Code session transcript and derive a composite reward signal.

The reward signal (0.0 - 1.0) is computed from detected success/failure indicators:

Positive signals:
  - Tests pass (+0.30): "PASSED", "OK", exit code 0
  - Git push (+0.20): successful push to remote
  - Git commit (+0.15): successful commit
  - Beads closed (+0.15): bd close succeeded
  - Ratchet lock (+0.10): ol ratchet record succeeded
  - No errors (+0.10): absence of exceptions/panics

Penalties:
  - Test failure (-0.20): "FAILED", non-zero exit
  - Exceptions (-0.15): Python tracebacks, Go panics
  - No commits (-0.10): session ended without git activity

Examples:
  ao session-outcome ~/.claude/projects/*/transcript.jsonl
  ao session-outcome --session abc123
  ao session-outcome --output json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSessionOutcome,
}

var (
	sessionOutcomeSessionID string
	sessionOutcomeOutput    string
)

func init() {
	rootCmd.AddCommand(sessionOutcomeCmd)
	sessionOutcomeCmd.Flags().StringVar(&sessionOutcomeSessionID, "session", "", "Session ID (extracted from transcript if not provided)")
	sessionOutcomeCmd.Flags().StringVar(&sessionOutcomeOutput, "output", "text", "Output format: text, json")
}

func runSessionOutcome(cmd *cobra.Command, args []string) error {
	var transcriptPath string

	if len(args) > 0 {
		transcriptPath = args[0]
	} else {
		// Find most recent transcript
		homeDir, _ := os.UserHomeDir()
		transcriptsDir := filepath.Join(homeDir, ".claude", "projects")
		transcriptPath = findMostRecentTranscript(transcriptsDir)
		if transcriptPath == "" {
			return fmt.Errorf("no transcript found; specify path as argument")
		}
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would analyze transcript: %s\n", transcriptPath)
		return nil
	}

	// Parse and analyze transcript
	outcome, err := analyzeTranscript(transcriptPath, sessionOutcomeSessionID)
	if err != nil {
		return fmt.Errorf("analyze transcript: %w", err)
	}

	// Output result
	switch sessionOutcomeOutput {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(outcome)

	default:
		fmt.Printf("Session Outcome Analysis\n")
		fmt.Printf("========================\n")
		fmt.Printf("Session ID:  %s\n", outcome.SessionID)
		fmt.Printf("Reward:      %.2f\n", outcome.Reward)
		fmt.Printf("Lines:       %d\n", outcome.TotalLines)
		fmt.Printf("\nSignals detected:\n")
		for _, s := range outcome.Signals {
			status := "✗"
			if s.Value {
				status = "✓"
			}
			fmt.Printf("  %s %-20s (weight: %+.2f)\n", status, s.Name, s.Weight)
		}
	}

	return nil
}

// analyzeTranscript parses a transcript and computes reward signal.
func analyzeTranscript(path string, sessionID string) (*SessionOutcome, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open transcript: %w", err)
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // read-only transcript analysis, close error non-fatal
	}()

	outcome := &SessionOutcome{
		SessionID:  canonicalSessionID(sessionID),
		Transcript: path,
		AnalyzedAt: time.Now(),
		Signals:    []Signal{},
	}

	state := &signalState{}
	totalLines := scanTranscript(f, outcome, state)
	outcome.TotalLines = totalLines

	// Generate session ID if still empty
	if outcome.SessionID == "" {
		outcome.SessionID = canonicalSessionID("")
	}

	// Build signals and compute reward
	outcome.Signals, outcome.Reward = state.buildSignals()
	return outcome, nil
}

// scanTranscript reads lines from the file and detects signals.
func scanTranscript(f *os.File, outcome *SessionOutcome, state *signalState) int {
	scanner := bufio.NewScanner(f)
	// Use larger buffer for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	totalLines := 0
	for scanner.Scan() {
		totalLines++
		line := scanner.Text()

		// Try to extract session ID if not provided
		if outcome.SessionID == "" {
			outcome.SessionID = extractSessionID(line)
		}

		// Detect signals using focused helpers
		state.detectTestSignals(line)
		state.detectGitSignals(line)
		state.detectWorkflowSignals(line)
		state.detectErrorSignals(line)
	}

	return totalLines
}

// extractSessionID tries to extract session ID from a JSON line.
func extractSessionID(line string) string {
	// Try to parse as JSON and extract sessionId
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return ""
	}

	if sessionID, ok := data["sessionId"].(string); ok {
		return canonicalSessionID(sessionID)
	}
	if sessionID, ok := data["session_id"].(string); ok {
		return canonicalSessionID(sessionID)
	}
	return ""
}

// findMostRecentTranscript finds the most recently modified transcript in the directory.
func findMostRecentTranscript(baseDir string) string {
	var mostRecent string
	var mostRecentTime time.Time

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if info.ModTime().After(mostRecentTime) {
			mostRecentTime = info.ModTime()
			mostRecent = path
		}
		return nil
	})

	if err != nil {
		return ""
	}
	return mostRecent
}

// findTranscriptForSession finds the newest transcript that contains the target session ID.
func findTranscriptForSession(baseDir, sessionID string) string {
	if strings.TrimSpace(sessionID) == "" {
		return ""
	}

	var bestPath string
	var bestModTime time.Time
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if !transcriptContainsSessionID(path, sessionID) {
			return nil
		}
		if info.ModTime().After(bestModTime) {
			bestModTime = info.ModTime()
			bestPath = path
		}
		return nil
	})
	if err != nil {
		return ""
	}
	return bestPath
}

func transcriptContainsSessionID(path, sessionID string) bool {
	targetAliases := make(map[string]bool)
	for _, alias := range sessionIDAliases(sessionID) {
		targetAliases[alias] = true
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // read-only check, close error non-fatal
	}()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		extracted := extractSessionID(line)
		if extracted != "" && targetAliases[extracted] {
			return true
		}
		if strings.Contains(line, sessionID) {
			return true
		}
		lineCount++
		if lineCount >= 5000 {
			break
		}
	}
	return false
}
