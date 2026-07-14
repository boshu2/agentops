package ratchet

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/trackerexec"
	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

// BdCLITimeout is the maximum duration to wait for bd CLI commands.
const BdCLITimeout = 5 * time.Second

// unknownValue is the fallback string for unrecognized steps or tiers.
const unknownValue = "unknown"

// ErrBdCLITimeout is returned when bd CLI command times out.
var ErrBdCLITimeout = fmt.Errorf("bd CLI timeout after %s", BdCLITimeout)

// gateCheckerFuncs maps each step to its gate-checking method.
var gateCheckerFuncs = map[Step]func(*GateChecker) (*GateResult, error){
	StepResearch:   (*GateChecker).checkResearchGate,
	StepPreMortem:  (*GateChecker).checkPreMortemGate,
	StepPlan:       (*GateChecker).checkPlanGate,
	StepImplement:  (*GateChecker).checkImplementGate,
	StepCrank:      (*GateChecker).checkCrankGate,
	StepVibe:       (*GateChecker).checkVibeGate,
	StepPostMortem: (*GateChecker).checkPostMortemGate,
}

// requiredInputs maps each step to its required input artifact description.
var requiredInputs = map[Step]string{
	StepResearch:   "",
	StepPreMortem:  ".agents/research/*.md",
	StepPlan:       ".agents/specs/*-v2.md OR .agents/synthesis/*.md",
	StepImplement:  "epic:<epic-id>",
	StepCrank:      "epic:<epic-id>",
	StepVibe:       "code changes (optional)",
	StepPostMortem: "closed epic (optional)",
}

// expectedOutputs maps each step to its expected output artifact description.
var expectedOutputs = map[Step]string{
	StepResearch:   ".agents/research/<topic>.md",
	StepPreMortem:  ".agents/specs/<topic>-v2.md",
	StepPlan:       "epic:<epic-id>",
	StepImplement:  "issue:<issue-id> (closed)",
	StepCrank:      "issue:<issue-id> (closed)",
	StepVibe:       "validation report",
	StepPostMortem: ".agents/learnings/<date>-<topic>.md",
}

// GateChecker validates that prerequisites are met before a step can proceed.
type GateChecker struct {
	locator *Locator
}

// TrackerStreams are the caller-owned streams supplied to tracker subprocesses.
// Tracker stdout remains reserved for the query result that GateChecker parses.
type TrackerStreams = trackerexec.Streams

// NewGateChecker creates a new gate checker.
func NewGateChecker(startDir string) (*GateChecker, error) {
	locator, err := NewLocator(startDir)
	if err != nil {
		return nil, err
	}
	return &GateChecker{locator: locator}, nil
}

// Check validates that the gate for a step is satisfied.
func (g *GateChecker) Check(step Step) (*GateResult, error) {
	if checker, ok := gateCheckerFuncs[step]; ok {
		return checker(g)
	}
	return &GateResult{
		Step:    step,
		Passed:  false,
		Message: fmt.Sprintf("Unknown step: %s", step),
	}, nil
}

// CheckContext preserves the caller's cancellation and tracker streams for
// gates that query br or bd. Check remains the source-compatible background
// wrapper for existing callers.
func (g *GateChecker) CheckContext(ctx context.Context, step Step, streams TrackerStreams) (*GateResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	switch step {
	case StepImplement:
		return g.checkImplementGateContext(ctx, streams)
	case StepCrank:
		return g.checkCrankGateContext(ctx, streams)
	case StepPostMortem:
		return g.checkPostMortemGateContext(ctx, streams)
	default:
		return g.Check(step)
	}
}

// checkResearchGate - No gate (chaos phase, always passes).
func (g *GateChecker) checkResearchGate() (*GateResult, error) {
	return &GateResult{
		Step:    StepResearch,
		Passed:  true,
		Message: "Research has no prerequisites (chaos phase)",
	}, nil
}

// checkPreMortemGate - Requires research artifact to exist.
func (g *GateChecker) checkPreMortemGate() (*GateResult, error) {
	// Look for any research artifact
	patterns := []string{
		"research/*.md",
		"research/**/*.md",
	}

	for _, pattern := range patterns {
		path, loc, err := g.locator.FindFirst(pattern)
		if err == nil {
			return &GateResult{
				Step:     StepPreMortem,
				Passed:   true,
				Message:  fmt.Sprintf("Research artifact found: %s", path),
				Input:    path,
				Location: string(loc),
			}, nil
		}
	}

	return &GateResult{
		Step:    StepPreMortem,
		Passed:  false,
		Message: "No research artifact found. Run /research first.",
	}, nil
}

// checkPlanGate - Requires synthesis or spec v2+ artifact.
func (g *GateChecker) checkPlanGate() (*GateResult, error) {
	// Look for synthesis or spec artifacts
	patterns := []string{
		"synthesis/*.md",
		"specs/*-v2.md",
		"specs/*-v*.md", // Any versioned spec
	}

	for _, pattern := range patterns {
		path, loc, err := g.locator.FindFirst(pattern)
		if err == nil {
			return &GateResult{
				Step:     StepPlan,
				Passed:   true,
				Message:  fmt.Sprintf("Spec/synthesis artifact found: %s", path),
				Input:    path,
				Location: string(loc),
			}, nil
		}
	}

	return &GateResult{
		Step:    StepPlan,
		Passed:  false,
		Message: "No spec or synthesis artifact found. Run /pre-mortem first.",
	}, nil
}

// checkImplementGate - Requires open or in_progress epic via bd CLI.
func (g *GateChecker) checkImplementGate() (*GateResult, error) {
	return g.checkImplementGateContext(context.Background(), TrackerStreams{})
}

func (g *GateChecker) checkImplementGateContext(ctx context.Context, streams TrackerStreams) (*GateResult, error) {
	// Try to find an open epic
	epicID, err := g.findEpicContext(ctx, "open", streams)
	if isCallerContextError(err) {
		return nil, err
	}
	if err != nil || epicID == "" {
		// Also try in_progress
		epicID, err = g.findEpicContext(ctx, "in_progress", streams)
		if isCallerContextError(err) {
			return nil, err
		}
	}

	if epicID != "" {
		return &GateResult{
			Step:     StepImplement,
			Passed:   true,
			Message:  fmt.Sprintf("Epic %s exists", epicID),
			Input:    epicID,
			Location: "beads",
		}, nil
	}

	return &GateResult{
		Step:    StepImplement,
		Passed:  false,
		Message: "No open epic found. Run /plan first.",
	}, nil
}

// checkCrankGate uses the implement gate requirements but preserves crank identity.
func (g *GateChecker) checkCrankGate() (*GateResult, error) {
	return g.checkCrankGateContext(context.Background(), TrackerStreams{})
}

func (g *GateChecker) checkCrankGateContext(ctx context.Context, streams TrackerStreams) (*GateResult, error) {
	result, err := g.checkImplementGateContext(ctx, streams)
	if result != nil {
		result.Step = StepCrank
	}
	return result, err
}

// checkVibeGate - Soft gate (always passes, but warns if no code).
func (g *GateChecker) checkVibeGate() (*GateResult, error) {
	// This is a soft gate - always passes
	// But we check for code changes to provide a meaningful message
	hasChanges := g.checkGitChanges()

	if hasChanges {
		return &GateResult{
			Step:    StepVibe,
			Passed:  true,
			Message: "Code changes detected, ready for validation",
		}, nil
	}

	return &GateResult{
		Step:    StepVibe,
		Passed:  true,
		Message: "Soft gate: always passes (no code changes detected)",
	}, nil
}

// checkPostMortemGate - Requires recently closed epic.
func (g *GateChecker) checkPostMortemGate() (*GateResult, error) {
	return g.checkPostMortemGateContext(context.Background(), TrackerStreams{})
}

func (g *GateChecker) checkPostMortemGateContext(ctx context.Context, streams TrackerStreams) (*GateResult, error) {
	// Look for a closed epic
	epicID, err := g.findEpicContext(ctx, "closed", streams)
	if isCallerContextError(err) {
		return nil, err
	}
	if err == nil && epicID != "" {
		return &GateResult{
			Step:     StepPostMortem,
			Passed:   true,
			Message:  fmt.Sprintf("Closed epic %s found", epicID),
			Input:    epicID,
			Location: "beads",
		}, nil
	}

	// Also pass if there are completed entries in the chain
	// (user may be running post-mortem on informal work)
	return &GateResult{
		Step:    StepPostMortem,
		Passed:  true,
		Message: "Soft gate: always passes (no closed epic found, informal review OK)",
	}, nil
}

// parseFirstEpicID extracts the first epic ID from bd CLI output.
// Returns "" if no valid non-comment line is found.
func parseFirstEpicID(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if fields := strings.Fields(line); len(fields) > 0 {
			return fields[0]
		}
	}
	return ""
}

// findEpic uses bd CLI to find an epic with the given status.
func (g *GateChecker) findEpic(status string) (string, error) {
	return g.findEpicContext(context.Background(), status, TrackerStreams{})
}

func (g *GateChecker) findEpicContext(ctx context.Context, status string, streams TrackerStreams) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	// Create context with 5s timeout
	commandCtx, cancel := context.WithTimeout(ctx, BdCLITimeout)
	defer cancel()

	// Call bd list --type epic --status <status>
	resolution, resolveErr := trackerresolve.Resolve(g.locator.startDir, os.Environ())
	if resolveErr != nil {
		return "", resolveErr
	}
	// The query result is parsed here, so child stdout cannot be caller-owned.
	streams.Stdout = nil
	cmd := (trackerexec.Factory{}).Command(
		commandCtx,
		resolution,
		[]string{"list", "--type", "epic", "--status", status},
		streams,
	)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if commandCtx.Err() == context.DeadlineExceeded {
			return "", ErrBdCLITimeout
		}
		return "", err
	}

	if id := parseFirstEpicID(out); id != "" {
		return id, nil
	}

	return "", fmt.Errorf("no epic found with status %s", status)
}

func isCallerContextError(err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded
}

// checkGitChanges returns true if there are uncommitted changes.
func (g *GateChecker) checkGitChanges() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	if g.locator != nil && g.locator.startDir != "" {
		cmd.Dir = g.locator.startDir
	}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// GetRequiredInput returns the expected input artifact for a step.
func GetRequiredInput(step Step) string {
	if input, ok := requiredInputs[step]; ok {
		return input
	}
	return unknownValue
}

// GetExpectedOutput returns the expected output artifact for a step.
func GetExpectedOutput(step Step) string {
	if output, ok := expectedOutputs[step]; ok {
		return output
	}
	return unknownValue
}
