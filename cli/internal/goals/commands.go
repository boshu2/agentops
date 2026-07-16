package goals

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/paths"
)

// withGoalFileCwd anchors the current process working directory to the git
// repo root that contains goalsFile, so relative-path goal checks like
// `bash scripts/foo.sh` resolve regardless of where `ao goals measure` was
// invoked from (soc-crzz). Returns a restore function the caller must defer.
// If goalsFile is not inside a git repo or cannot be resolved, returns a
// no-op restorer and leaves cwd untouched (preserves prior behavior).
func withGoalFileCwd(goalsFile string) func() {
	noop := func() {}
	if strings.TrimSpace(goalsFile) == "" {
		return noop
	}
	abs, err := filepath.Abs(goalsFile)
	if err != nil {
		return noop
	}
	resolved := paths.ResolveFromRoot(filepath.Dir(abs))
	if resolved == nil || resolved.RepoRoot == "" {
		return noop
	}
	prev, err := os.Getwd()
	if err != nil {
		return noop
	}
	if err := os.Chdir(resolved.RepoRoot); err != nil {
		return noop
	}
	return func() { _ = os.Chdir(prev) }
}

// HistoryOptions configures the goals history command.
type HistoryOptions struct {
	GoalID      string
	Since       string
	JSON        bool
	HistoryPath string
	Stdout      io.Writer
}

// RunHistory loads and displays goal measurement history.
func RunHistory(opts HistoryOptions) error {
	if opts.HistoryPath == "" {
		opts.HistoryPath = ".agents/ao/goals/history.jsonl"
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}

	entries, err := LoadHistory(opts.HistoryPath)
	if err != nil {
		return fmt.Errorf("loading history: %w", err)
	}

	if len(entries) == 0 {
		fmt.Fprintln(opts.Stdout, "No history entries found. Run 'ao goals measure' first.")
		return nil
	}

	if opts.Since != "" || opts.GoalID != "" {
		var since time.Time
		if opts.Since != "" {
			var parseErr error
			since, parseErr = time.Parse("2006-01-02", opts.Since)
			if parseErr != nil {
				return fmt.Errorf("invalid --since date: %w", parseErr)
			}
		}
		entries = QueryHistory(entries, opts.GoalID, since)
	}

	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	fmt.Fprintf(opts.Stdout, "%-20s  %4s  %5s  %7s  %s\n", "TIMESTAMP", "PASS", "TOTAL", "SCORE", "GIT SHA")
	for _, e := range entries {
		fmt.Fprintf(opts.Stdout, "%-20s  %4d  %5d  %6.1f%%  %s\n",
			e.Timestamp, e.GoalsPassing, e.GoalsTotal, e.Score, e.GitSHA)
	}
	return nil
}

// MeasureOptions configures the goals measure command.
type MeasureOptions struct {
	GoalID       string
	ExcludeTag   string // Filter out goals whose Tags include this value (e.g. "long-cycle").
	Directives   bool
	GoalsFile    string
	Timeout      time.Duration
	TotalTimeout time.Duration
	JSON         bool
	Verbose      bool
	SnapDir      string
	Stdout       io.Writer
	Stderr       io.Writer
}

// goalHasTag reports whether g.Tags contains the given tag (case-insensitive).
func goalHasTag(g Goal, tag string) bool {
	for _, t := range g.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// applyMeasureFilters narrows gf.Goals according to MeasureOptions:
// --goal restricts to a single ID (returns "not found" if absent), then
// --exclude-tag drops any remaining goal whose Tags include that value.
// Mutates gf in place.
func applyMeasureFilters(gf *GoalFile, opts MeasureOptions) error {
	if opts.GoalID != "" {
		var filtered []Goal
		for _, g := range gf.Goals {
			if g.ID == opts.GoalID {
				filtered = append(filtered, g)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("goal %q not found", opts.GoalID)
		}
		gf.Goals = filtered
	}
	if opts.ExcludeTag != "" {
		var filtered []Goal
		for _, g := range gf.Goals {
			if !goalHasTag(g, opts.ExcludeTag) {
				filtered = append(filtered, g)
			}
		}
		gf.Goals = filtered
	}
	return nil
}

// RunMeasure runs goal checks and produces a snapshot.
func RunMeasure(opts MeasureOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.SnapDir == "" {
		opts.SnapDir = ".agents/ao/goals/baselines"
	}

	restore := withGoalFileCwd(opts.GoalsFile)
	defer restore()

	gf, err := LoadGoals(opts.GoalsFile)
	if err != nil {
		return fmt.Errorf("loading goals: %w", err)
	}

	if opts.Directives {
		if opts.GoalID != "" {
			return fmt.Errorf("--directives and --goal cannot be combined")
		}
		if gf.Format != "md" {
			return fmt.Errorf("--directives requires GOALS.md format")
		}
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(gf.Directives)
	}

	if errs := ValidateGoals(gf); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(opts.Stderr, "validation: %s\n", e)
		}
		return fmt.Errorf("%d validation errors", len(errs))
	}

	if err := applyMeasureFilters(gf, opts); err != nil {
		return err
	}

	snap := MeasureWithTotalTimeout(gf, opts.Timeout, opts.TotalTimeout)

	path, err := SaveSnapshot(snap, opts.SnapDir)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "warning: could not save snapshot: %v\n", err)
	} else if opts.Verbose {
		fmt.Fprintf(opts.Stderr, "Snapshot saved: %s\n", path)
	}

	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(snap)
	}

	fmt.Fprintf(opts.Stdout, "%-30s  %-8s  %8s  %6s\n", "GOAL", "RESULT", "DURATION", "WEIGHT")
	fmt.Fprintf(opts.Stdout, "%-30s  %-8s  %8s  %6s\n", "------------------------------", "--------", "--------", "------")
	for _, m := range snap.Goals {
		fmt.Fprintf(opts.Stdout, "%-30s  %-8s  %7.1fs  %6d\n",
			m.GoalID, m.Result, m.Duration, m.Weight)
	}
	fmt.Fprintln(opts.Stdout)
	fmt.Fprintf(opts.Stdout, "Score: %.1f%% (%d/%d passing, %d skipped)\n",
		snap.Summary.Score, snap.Summary.Passing, snap.Summary.Total, snap.Summary.Skipped)

	return nil
}

// ValidateOptions configures the goals validate command.
type ValidateOptions struct {
	GoalsFile string
	JSON      bool
	Stdout    io.Writer
}

// ValidateResult holds the outcome of a goals validation.
type ValidateResult struct {
	Valid      bool     `json:"valid"`
	Errors     []string `json:"errors,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
	GoalCount  int      `json:"goal_count"`
	Version    int      `json:"version"`
	Format     string   `json:"format"`
	Directives int      `json:"directives"`
}

// RunValidate validates goals structure and wiring.
func RunValidate(opts ValidateOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}

	result := ValidateResult{}

	gf, err := LoadGoals(opts.GoalsFile)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("load: %v", err))
		return OutputValidateResult(opts.Stdout, opts.JSON, result)
	}

	result.Version = gf.Version
	result.GoalCount = len(gf.Goals)
	result.Format = gf.Format
	result.Directives = len(gf.Directives)

	appendGoalDirectiveWarnings(&result, gf)
	appendGoalValidationErrors(&result, gf)
	appendUnwiredScriptWarnings(&result, gf)
	appendMissingGoalScriptErrors(&result, gf)

	result.Valid = len(result.Errors) == 0
	return OutputValidateResult(opts.Stdout, opts.JSON, result)
}

func appendGoalDirectiveWarnings(result *ValidateResult, gf *GoalFile) {
	if gf.Format == "md" && gf.Mission == "" {
		result.Warnings = append(result.Warnings, "empty mission")
	}
	if gf.Format == "md" && len(gf.Directives) == 0 {
		result.Warnings = append(result.Warnings, "no directives defined")
	}
	for _, d := range gf.Directives {
		if d.Steer == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("directive %d %q: missing steer", d.Number, d.Title))
		}
	}
}

func appendGoalValidationErrors(result *ValidateResult, gf *GoalFile) {
	for _, err := range ValidateGoals(gf) {
		result.Errors = append(result.Errors, err.Error())
	}
}

func appendUnwiredScriptWarnings(result *ValidateResult, gf *GoalFile) {
	scriptFiles, _ := filepath.Glob("scripts/check-*.sh")
	for _, sf := range scriptFiles {
		base := filepath.Base(sf)
		if !goalsReferenceScript(gf.Goals, base) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("script %s not wired to any goal", base))
		}
	}
}

func goalsReferenceScript(goals []Goal, scriptBase string) bool {
	for _, g := range goals {
		if strings.Contains(g.Check, scriptBase) {
			return true
		}
	}
	return false
}

func appendMissingGoalScriptErrors(result *ValidateResult, gf *GoalFile) {
	for _, g := range gf.Goals {
		scriptPath, ok := goalScriptPath(g.Check)
		if !ok {
			continue
		}
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			result.Errors = append(result.Errors, fmt.Sprintf("goal %s: script %s does not exist", g.ID, scriptPath))
		}
	}
}

func goalScriptPath(check string) (string, bool) {
	if !strings.HasPrefix(check, "scripts/") {
		return "", false
	}
	parts := strings.Fields(check)
	if len(parts) == 0 {
		return "", false
	}
	return parts[0], true
}

// OutputValidateResult formats and writes a ValidateResult.
func OutputValidateResult(w io.Writer, asJSON bool, result ValidateResult) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if result.Valid {
		fmt.Fprintf(w, "VALID: %d goals, version %d, format %s\n", result.GoalCount, result.Version, result.Format)
		if result.Directives > 0 {
			fmt.Fprintf(w, "  Directives: %d\n", result.Directives)
		}
	} else {
		fmt.Fprintf(w, "INVALID: %d errors\n", len(result.Errors))
	}

	for _, e := range result.Errors {
		fmt.Fprintf(w, "  ERROR: %s\n", e)
	}
	for _, wn := range result.Warnings {
		fmt.Fprintf(w, "  WARN: %s\n", wn)
	}

	if !result.Valid {
		return fmt.Errorf("validation failed")
	}
	return nil
}

// ExportOptions configures the goals export command.
type ExportOptions struct {
	GoalsFile string
	Timeout   time.Duration
	SnapDir   string
	Stdout    io.Writer
	Stderr    io.Writer
}

// RunExport exports the latest snapshot as JSON.
func RunExport(opts ExportOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.SnapDir == "" {
		opts.SnapDir = ".agents/ao/goals/baselines"
	}

	snap, err := LoadLatestSnapshot(opts.SnapDir)
	if err != nil {
		gf, loadErr := LoadGoals(opts.GoalsFile)
		if loadErr != nil {
			return fmt.Errorf("loading goals: %w", loadErr)
		}
		snap = Measure(gf, opts.Timeout)
		if _, saveErr := SaveSnapshot(snap, opts.SnapDir); saveErr != nil {
			fmt.Fprintf(opts.Stderr, "warning: could not save snapshot: %v\n", saveErr)
		}
	}

	enc := json.NewEncoder(opts.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(snap)
}

// MetaOptions configures the goals meta command.
type MetaOptions struct {
	GoalsFile string
	Timeout   time.Duration
	JSON      bool
	Stdout    io.Writer
}

// RunMeta runs and reports meta-goals only.
func RunMeta(opts MetaOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}

	gf, err := LoadGoals(opts.GoalsFile)
	if err != nil {
		return fmt.Errorf("loading goals: %w", err)
	}

	var metaGoals []Goal
	for _, g := range gf.Goals {
		if g.Type == GoalTypeMeta {
			metaGoals = append(metaGoals, g)
		}
	}

	if len(metaGoals) == 0 {
		fmt.Fprintln(opts.Stdout, "No meta-goals found (type: meta)")
		return nil
	}

	metaGF := &GoalFile{Version: gf.Version, Mission: gf.Mission, Goals: metaGoals}
	snap := Measure(metaGF, opts.Timeout)

	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(snap)
	}

	fmt.Fprintf(opts.Stdout, "Meta-Goals: %d total\n\n", len(metaGoals))
	fmt.Fprintf(opts.Stdout, "%-30s  %-8s  %8s\n", "GOAL", "RESULT", "DURATION")
	for _, m := range snap.Goals {
		fmt.Fprintf(opts.Stdout, "%-30s  %-8s  %7.1fs\n", m.GoalID, m.Result, m.Duration)
	}
	fmt.Fprintln(opts.Stdout)

	if snap.Summary.Failing > 0 {
		fmt.Fprintf(opts.Stdout, "META-HEALTH: DEGRADED (%d/%d failing)\n", snap.Summary.Failing, snap.Summary.Total)
		return fmt.Errorf("meta-goal failures detected")
	}

	fmt.Fprintf(opts.Stdout, "META-HEALTH: OK (%d/%d passing)\n", snap.Summary.Passing, snap.Summary.Total)
	return nil
}

// DriftOptions configures the goals drift command.
type DriftOptions struct {
	GoalsFile string
	Timeout   time.Duration
	JSON      bool
	SnapDir   string
	Stdout    io.Writer
	Stderr    io.Writer
}

// RunDrift compares snapshots for regressions.
func RunDrift(opts DriftOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.SnapDir == "" {
		opts.SnapDir = ".agents/ao/goals/baselines"
	}

	gf, err := LoadGoals(opts.GoalsFile)
	if err != nil {
		return fmt.Errorf("loading goals: %w", err)
	}

	latest, err := LoadLatestSnapshot(opts.SnapDir)
	if err != nil {
		snap := Measure(gf, opts.Timeout)
		if _, saveErr := SaveSnapshot(snap, opts.SnapDir); saveErr != nil {
			fmt.Fprintf(opts.Stderr, "warning: could not save snapshot: %v\n", saveErr)
		}
		fmt.Fprintln(opts.Stdout, "No baseline snapshot found. Created initial snapshot.")
		fmt.Fprintf(opts.Stdout, "Score: %.1f%% (%d/%d passing)\n", snap.Summary.Score, snap.Summary.Passing, snap.Summary.Total)
		return nil
	}

	current := Measure(gf, opts.Timeout)
	if _, saveErr := SaveSnapshot(current, opts.SnapDir); saveErr != nil {
		fmt.Fprintf(opts.Stderr, "warning: could not save snapshot: %v\n", saveErr)
	}

	drifts := ComputeDrift(latest, current)

	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(drifts)
	}

	regressions := 0
	improvements := 0
	for _, d := range drifts {
		switch d.Delta {
		case "regressed":
			regressions++
		case "improved":
			improvements++
		}
	}

	fmt.Fprintf(opts.Stdout, "Drift: %d regressions, %d improvements, %d unchanged\n\n",
		regressions, improvements, len(drifts)-regressions-improvements)

	if regressions > 0 || improvements > 0 {
		fmt.Fprintf(opts.Stdout, "%-30s  %-10s  %-8s  %s\n", "GOAL", "DELTA", "BEFORE", "AFTER")
		for _, d := range drifts {
			if d.Delta == "unchanged" {
				continue
			}
			fmt.Fprintf(opts.Stdout, "%-30s  %-10s  %-8s  -> %s\n", d.GoalID, d.Delta, d.Before, d.After)
		}
		fmt.Fprintln(opts.Stdout)
	}

	fmt.Fprintf(opts.Stdout, "Baseline: %.1f%% -> Current: %.1f%%\n", latest.Summary.Score, current.Summary.Score)
	return nil
}
