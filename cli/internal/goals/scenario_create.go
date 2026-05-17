package goals

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/scenario"
)

// ScenarioCreateOptions configures RunScenarioCreate.
type ScenarioCreateOptions struct {
	GoalsFile    string
	DirectiveNum int
	Goal         string
	Threshold    float64
	Status       string
	Source       string
	// SpecDir is the directory the promoted spec scenario is written to.
	// Empty defaults to spec/scenarios (docs/adr/ADR-0003).
	SpecDir string
	Now     func() time.Time
	JSON    bool
	Stdout  io.Writer
}

// ScenarioCreateResult is the machine-readable outcome of --create.
type ScenarioCreateResult struct {
	ScenarioID   string `json:"scenario_id"`
	ScenarioPath string `json:"scenario_path"`
	DirectiveID  string `json:"directive_id"`
	DirectiveNum int    `json:"directive_number"`
	Linked       bool   `json:"linked"`
}

// RunScenarioCreate scaffolds a promoted spec scenario and links it
// bidirectionally to a GOALS.md directive: the scenario JSON carries the
// directive's stable ID, and the directive's "**Scenarios:**" line gains the
// scenario ID via the non-lossy patcher.
//
// Failure ordering preserves the invariant in soc-58nt.1.3: if scenario
// creation fails, GOALS.md is never written; if the GOALS.md write fails, the
// created scenario path is reported and the link is marked incomplete.
func RunScenarioCreate(opts ScenarioCreateOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	specDir := opts.SpecDir
	if specDir == "" {
		specDir = filepath.Join("spec", "scenarios")
	}

	patcher, goalsPath, err := LoadGoalsPatcher(opts.GoalsFile)
	if err != nil {
		return fmt.Errorf("loading goals: %w", err)
	}
	directive, ok := patcher.DirectiveByNumber(opts.DirectiveNum)
	if !ok {
		return fmt.Errorf("directive #%d not found (run 'ao goals scenarios' to list directives)", opts.DirectiveNum)
	}

	// Ensure the target directive has a stable ID. The patcher edit is
	// in-memory only; GOALS.md is not written until the link step, so a
	// scenario-creation failure below leaves GOALS.md on disk untouched.
	stableID := directive.StableID
	if stableID == "" {
		stableID = uniqueID(SlugifyDirectiveID(directive.Title), patcherStableIDs(patcher))
		if err := patcher.SetAttribute(opts.DirectiveNum, AttrDirectiveID, stableID); err != nil {
			return fmt.Errorf("assigning directive ID: %w", err)
		}
	}

	res, err := scenario.Create(scenario.CreateOptions{
		Goal:        opts.Goal,
		Threshold:   opts.Threshold,
		Status:      opts.Status,
		Source:      opts.Source,
		DirectiveID: stableID,
		Dir:         specDir,
		Now:         opts.Now,
	})
	if err != nil {
		return fmt.Errorf("creating scenario: %w", err)
	}

	current, _ := patcher.DirectiveByNumber(opts.DirectiveNum)
	linkErr := linkScenarioAndWrite(patcher, goalsPath, opts.DirectiveNum, current.Scenarios, res.Scenario.ID)
	result := ScenarioCreateResult{
		ScenarioID:   res.Scenario.ID,
		ScenarioPath: res.Path,
		DirectiveID:  stableID,
		DirectiveNum: opts.DirectiveNum,
		Linked:       linkErr == nil,
	}
	writeScenarioCreateResult(opts, result)
	if linkErr != nil {
		return fmt.Errorf("scenario created at %s but GOALS.md was not linked: %w; repair: re-run 'ao goals scenarios --create' or add %s to directive #%d's Scenarios line by hand",
			res.Path, linkErr, res.Scenario.ID, opts.DirectiveNum)
	}
	return nil
}

// patcherStableIDs collects every stable Directive ID already in the file.
func patcherStableIDs(p *GoalsPatcher) map[string]bool {
	used := map[string]bool{}
	for _, d := range p.Directives() {
		if d.StableID != "" {
			used[d.StableID] = true
		}
	}
	return used
}

// linkScenarioAndWrite appends scenarioID to the directive's Scenarios line
// (if not already present) and writes GOALS.md back through the patcher.
func linkScenarioAndWrite(p *GoalsPatcher, goalsPath string, directiveNum int, existing []string, scenarioID string) error {
	for _, s := range existing {
		if s == scenarioID {
			return p.WriteFile(goalsPath) // already linked; still flush the stable-ID edit
		}
	}
	linked := append(append([]string{}, existing...), scenarioID)
	if err := p.SetAttribute(directiveNum, AttrScenarios, strings.Join(linked, ", ")); err != nil {
		return err
	}
	return p.WriteFile(goalsPath)
}

// writeScenarioCreateResult renders the create outcome to stdout.
func writeScenarioCreateResult(opts ScenarioCreateOptions, result ScenarioCreateResult) {
	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return
	}
	if result.Linked {
		fmt.Fprintf(opts.Stdout, "Created scenario %s at %s and linked it to directive #%d [%s]\n",
			result.ScenarioID, result.ScenarioPath, result.DirectiveNum, result.DirectiveID)
		return
	}
	fmt.Fprintf(opts.Stdout, "Created scenario %s at %s (NOT linked — directive #%d unchanged)\n",
		result.ScenarioID, result.ScenarioPath, result.DirectiveNum)
}
