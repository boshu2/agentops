package ratchet

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
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
	// ctx is the base context from which per-command timeouts are derived.
	// Set once at the CLI entry via WithContext; never nil after
	// construction (NewGateChecker seeds it with context.Background()).
	ctx context.Context
}

// NewGateChecker creates a new gate checker. The base context defaults to
// context.Background(); callers at the CLI entry should override it via
// WithContext so cancellation propagates from the top of the stack.
func NewGateChecker(startDir string) (*GateChecker, error) {
	locator, err := NewLocator(startDir)
	if err != nil {
		return nil, err
	}
	return &GateChecker{locator: locator, ctx: context.Background()}, nil
}

// WithContext sets the base context used to derive per-command timeouts and
// returns the receiver for chaining. A nil ctx is ignored. Intended to be
// called once at the CLI entry layer (e.g. with cmd.Context()).
func (g *GateChecker) WithContext(ctx context.Context) *GateChecker {
	if ctx != nil {
		g.ctx = ctx
	}
	return g
}

// baseContext returns the configured base context, falling back to
// context.Background() if the checker was constructed without one (e.g. via
// a struct literal in tests).
func (g *GateChecker) baseContext() context.Context {
	if g.ctx == nil {
		return context.Background()
	}
	return g.ctx
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
	// Try to find an open epic
	epicID, err := g.findEpic("open")
	if err != nil || epicID == "" {
		// Also try in_progress
		epicID, _ = g.findEpic("in_progress")
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
	result, err := g.checkImplementGate()
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
	// Look for a closed epic
	epicID, err := g.findEpic("closed")
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
	// Derive a 5s timeout from the checker's base context so cancellation
	// propagates from the CLI entry rather than minting a fresh root here.
	ctx, cancel := context.WithTimeout(g.baseContext(), BdCLITimeout)
	defer cancel()

	// Call bd list --type epic --status <status>
	cmd := exec.CommandContext(ctx, "bd", "list", "--type", "epic", "--status", status)
	out, err := cmd.Output()
	if err != nil {
		// Check if the error was due to context timeout
		if ctx.Err() == context.DeadlineExceeded {
			return "", ErrBdCLITimeout
		}
		return "", err
	}

	if id := parseFirstEpicID(out); id != "" {
		return id, nil
	}

	return "", fmt.Errorf("no epic found with status %s", status)
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
