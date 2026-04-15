package goals

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/shellutil"
)

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
	GoalID     string
	Directives bool
	GoalsFile  string
	Timeout    time.Duration
	JSON       bool
	Verbose    bool
	SnapDir    string
	Stdout     io.Writer
	Stderr     io.Writer
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

	gf, err := LoadGoals(opts.GoalsFile)
	if err != nil {
		return fmt.Errorf("loading goals: %w", err)
	}

	if opts.Directives {
		if opts.GoalID != "" {
			return fmt.Errorf("--directives and --goal cannot be combined")
		}
		if gf.Format != "md" {
			return fmt.Errorf("--directives requires GOALS.md format. Run 'ao goals migrate --to-md' to convert")
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

	snap := Measure(gf, opts.Timeout)

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

// MigrateOptions configures the goals migrate command.
type MigrateOptions struct {
	ToMD      bool
	GoalsFile string
	Stdout    io.Writer
}

// RunMigrate migrates goals between formats.
func RunMigrate(opts MigrateOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}

	path := opts.GoalsFile
	gf, err := LoadGoals(path)
	if err != nil {
		return fmt.Errorf("load goals: %w", err)
	}

	if opts.ToMD {
		if gf.Format == "md" {
			fmt.Fprintln(opts.Stdout, "Already in GOALS.md format — no migration needed.")
			return nil
		}
		gf.Format = "md"
		gf.Version = 4
		if gf.Mission == "" {
			gf.Mission = "Project fitness goals"
		}
		if len(gf.Directives) == 0 {
			gf.Directives = DirectivesFromPillars(gf.Goals)
		}
		if len(gf.NorthStars) == 0 {
			gf.NorthStars = []string{
				"Every check passes before changes reach users",
				"Validation catches regressions automatically",
			}
		}
		if len(gf.AntiStars) == 0 {
			gf.AntiStars = []string{
				"Untested changes reaching main",
				"Goals that are trivially true or test implementation details",
			}
		}
		content := RenderGoalsMD(gf)
		mdPath := filepath.Join(filepath.Dir(path), "GOALS.md")
		if err := os.WriteFile(mdPath, []byte(content), 0o600); err != nil {
			return fmt.Errorf("writing GOALS.md: %w", err)
		}
		fmt.Fprintf(opts.Stdout, "Migrated %s → %s (GOALS.md format, version 4)\n", path, mdPath)
		fmt.Fprintln(opts.Stdout, "Original YAML file preserved. Delete it manually when ready.")
		return nil
	}

	if gf.Version >= 2 {
		fmt.Fprintf(opts.Stdout, "%s is already version %d — no migration needed.\n", path, gf.Version)
		return nil
	}

	backupPath := path + ".v1.bak"
	original, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read original for backup: %w", err)
	}
	if err := os.WriteFile(backupPath, original, 0o600); err != nil {
		return fmt.Errorf("write backup: %w", err)
	}
	fmt.Fprintf(opts.Stdout, "Backed up original to %s\n", backupPath)

	MigrateV1ToV2(gf)

	out, err := yaml.Marshal(gf)
	if err != nil {
		return fmt.Errorf("marshal migrated goals: %w", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		return fmt.Errorf("write migrated goals: %w", err)
	}

	fmt.Fprintf(opts.Stdout, "Migrated %s from version 1 to version 2.\n", path)
	return nil
}

// DirectivesFromPillars generates directives from existing goal pillar groupings.
func DirectivesFromPillars(gs []Goal) []Directive {
	seen := map[string]bool{}
	var pillars []string
	for _, g := range gs {
		if g.Pillar == "" {
			continue
		}
		if !seen[g.Pillar] {
			seen[g.Pillar] = true
			pillars = append(pillars, g.Pillar)
		}
	}
	if len(pillars) == 0 {
		return []Directive{
			{Number: 1, Title: "Improve project quality", Description: "Focus on the highest-impact improvements.", Steer: "increase"},
		}
	}
	dirs := make([]Directive, len(pillars))
	for i, p := range pillars {
		dirs[i] = Directive{
			Number:      i + 1,
			Title:       "Strengthen " + p,
			Description: fmt.Sprintf("Improve goals in the %s pillar.", p),
			Steer:       "increase",
		}
	}
	return dirs
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

// SteerAddOptions configures the goals steer add command.
type SteerAddOptions struct {
	Title       string
	Description string
	Steer       string
	GoalsFile   string
	JSON        bool
	DryRun      bool
	Stdout      io.Writer
}

// ValidSteers enumerates the allowed steer values.
var ValidSteers = map[string]bool{
	"increase": true,
	"decrease": true,
	"hold":     true,
	"explore":  true,
}

// RunSteerAdd adds a new directive to GOALS.md.
func RunSteerAdd(opts SteerAddOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if !ValidSteers[opts.Steer] {
		return fmt.Errorf("invalid steer value %q (valid: increase, decrease, hold, explore)", opts.Steer)
	}

	gf, resolvedPath, err := LoadMDGoals(opts.GoalsFile)
	if err != nil {
		return err
	}

	maxNum := 0
	for _, d := range gf.Directives {
		if d.Number > maxNum {
			maxNum = d.Number
		}
	}

	newDirective := Directive{Number: maxNum + 1, Title: opts.Title, Description: opts.Description, Steer: opts.Steer}
	gf.Directives = append(gf.Directives, newDirective)

	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(newDirective)
	}
	if opts.DryRun {
		fmt.Fprintf(opts.Stdout, "Would add directive #%d: %s\n", newDirective.Number, newDirective.Title)
		return nil
	}
	if err := WriteMDGoals(gf, resolvedPath); err != nil {
		return err
	}
	fmt.Fprintf(opts.Stdout, "Added directive #%d: %s (steer: %s)\n", newDirective.Number, newDirective.Title, newDirective.Steer)
	return nil
}

// SteerRemoveOptions configures the goals steer remove command.
type SteerRemoveOptions struct {
	Number    int
	GoalsFile string
	JSON      bool
	DryRun    bool
	Stdout    io.Writer
}

// RunSteerRemove removes a directive by number.
func RunSteerRemove(opts SteerRemoveOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	gf, resolvedPath, err := LoadMDGoals(opts.GoalsFile)
	if err != nil {
		return err
	}

	found := false
	var remaining []Directive
	for _, d := range gf.Directives {
		if d.Number == opts.Number {
			found = true
			continue
		}
		remaining = append(remaining, d)
	}
	if !found {
		return fmt.Errorf("directive #%d not found", opts.Number)
	}
	for i := range remaining {
		remaining[i].Number = i + 1
	}
	gf.Directives = remaining

	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(gf.Directives)
	}
	if opts.DryRun {
		fmt.Fprintf(opts.Stdout, "Would remove directive #%d and renumber %d remaining\n", opts.Number, len(remaining))
		return nil
	}
	if err := WriteMDGoals(gf, resolvedPath); err != nil {
		return err
	}
	fmt.Fprintf(opts.Stdout, "Removed directive #%d, renumbered %d remaining\n", opts.Number, len(remaining))
	return nil
}

// SteerPrioritizeOptions configures the goals steer prioritize command.
type SteerPrioritizeOptions struct {
	Number      int
	NewPosition int
	GoalsFile   string
	JSON        bool
	DryRun      bool
	Stdout      io.Writer
}

// RunSteerPrioritize moves a directive to a new position.
func RunSteerPrioritize(opts SteerPrioritizeOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	gf, resolvedPath, err := LoadMDGoals(opts.GoalsFile)
	if err != nil {
		return err
	}
	if len(gf.Directives) == 0 {
		return fmt.Errorf("no directives to prioritize")
	}
	if opts.NewPosition < 1 || opts.NewPosition > len(gf.Directives) {
		return fmt.Errorf("new position must be between 1 and %d", len(gf.Directives))
	}

	srcIdx := -1
	for i, d := range gf.Directives {
		if d.Number == opts.Number {
			srcIdx = i
			break
		}
	}
	if srcIdx < 0 {
		return fmt.Errorf("directive #%d not found", opts.Number)
	}

	moving := gf.Directives[srcIdx]
	directives := make([]Directive, 0, len(gf.Directives))
	directives = append(directives, gf.Directives[:srcIdx]...)
	directives = append(directives, gf.Directives[srcIdx+1:]...)

	insertIdx := opts.NewPosition - 1
	if insertIdx > len(directives) {
		insertIdx = len(directives)
	}
	result := make([]Directive, 0, len(gf.Directives))
	result = append(result, directives[:insertIdx]...)
	result = append(result, moving)
	result = append(result, directives[insertIdx:]...)
	for i := range result {
		result[i].Number = i + 1
	}
	gf.Directives = result

	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(gf.Directives)
	}
	if opts.DryRun {
		fmt.Fprintf(opts.Stdout, "Would move directive %q to position %d\n", moving.Title, opts.NewPosition)
		return nil
	}
	if err := WriteMDGoals(gf, resolvedPath); err != nil {
		return err
	}
	fmt.Fprintf(opts.Stdout, "Moved directive %q to position %d\n", moving.Title, opts.NewPosition)
	return nil
}

// LoadMDGoals loads goals and validates the format is markdown.
func LoadMDGoals(goalsFile string) (*GoalFile, string, error) {
	resolvedPath := ResolveGoalsPath(goalsFile)
	gf, err := LoadGoals(goalsFile)
	if err != nil {
		return nil, "", fmt.Errorf("loading goals: %w", err)
	}
	if gf.Format != "md" {
		return nil, "", fmt.Errorf("directives require GOALS.md format; run 'ao goals migrate --to-md'")
	}
	return gf, resolvedPath, nil
}

// WriteMDGoals renders and writes a GoalFile back to disk as GOALS.md.
func WriteMDGoals(gf *GoalFile, path string) error {
	content := RenderGoalsMD(gf)
	if strings.ToLower(filepath.Ext(path)) != ".md" {
		path = filepath.Join(filepath.Dir(path), "GOALS.md")
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing goals file: %w", err)
	}
	return nil
}

// PruneOptions configures the goals prune command.
type PruneOptions struct {
	GoalsFile string
	DryRun    bool
	JSON      bool
	Stdout    io.Writer
}

// PruneResult holds the outcome of a goals prune operation.
type PruneResult struct {
	StaleGoals []StaleGoal `json:"stale_goals"`
	Removed    int         `json:"removed"`
	DryRun     bool        `json:"dry_run"`
}

// StaleGoal identifies a goal referencing a nonexistent file.
type StaleGoal struct {
	ID    string `json:"id"`
	Check string `json:"check"`
	Path  string `json:"missing_path"`
}

// FindMissingPath checks if a goal's check command references a missing file.
func FindMissingPath(check string) string {
	parts := strings.Fields(check)
	for _, part := range parts {
		if strings.HasPrefix(part, "scripts/") || strings.HasPrefix(part, "./scripts/") ||
			strings.HasPrefix(part, "tests/") || strings.HasPrefix(part, "./tests/") ||
			strings.HasPrefix(part, "hooks/") || strings.HasPrefix(part, "./hooks/") {
			cleanPath := strings.TrimRight(part, ";|&")
			if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
				return cleanPath
			}
		}
		if strings.Contains(part, "/") && filepath.Ext(part) != "" {
			cleanPath := strings.TrimRight(part, ";|&")
			if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
				return cleanPath
			}
		}
	}
	return ""
}

// AddOptions configures the goals add command.
type AddOptions struct {
	ID, Check, Type, Description, GoalsFile string
	Weight                                  int
	Timeout                                 time.Duration
	DryRun                                  bool
	Stdout                                  io.Writer
}

// RunAdd adds a new goal.
func RunAdd(ctx context.Context, opts AddOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if !KebabRe.MatchString(opts.ID) {
		return fmt.Errorf("goal ID must be kebab-case: %q", opts.ID)
	}

	gf, err := LoadGoals(opts.GoalsFile)
	if err != nil {
		return fmt.Errorf("loading goals: %w", err)
	}
	for _, g := range gf.Goals {
		if g.ID == opts.ID {
			return fmt.Errorf("goal %q already exists", opts.ID)
		}
	}

	if !opts.DryRun {
		checkCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
		// SanitizedBashCommand bypasses ~/.bashrc and BASH_ENV so user shell
		// aliases cannot silently change the meaning of new goal check strings.
		testCmd := shellutil.SanitizedBashCommand(checkCtx, opts.Check)
		if out, err := testCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("check command failed (exit non-zero):\n%s", string(out))
		}
	}

	goalType := GoalType(opts.Type)
	if opts.Type != "" && !ValidTypes[goalType] {
		return fmt.Errorf("invalid type %q (valid: health, architecture, quality, meta)", opts.Type)
	}
	if opts.Type == "" {
		goalType = GoalTypeHealth
	}
	desc := opts.Description
	if desc == "" {
		desc = opts.ID
	}

	newGoal := Goal{ID: opts.ID, Description: desc, Check: opts.Check, Weight: opts.Weight, Type: goalType}
	gf.Goals = append(gf.Goals, newGoal)

	if gf.Format == "md" {
		content := RenderGoalsMD(gf)
		if err := os.WriteFile(ResolveGoalsPath(opts.GoalsFile), []byte(content), 0o600); err != nil {
			return fmt.Errorf("writing goals: %w", err)
		}
	} else {
		data, err := yaml.Marshal(gf)
		if err != nil {
			return fmt.Errorf("marshaling goals: %w", err)
		}
		if err := os.WriteFile(opts.GoalsFile, data, 0o600); err != nil {
			return fmt.Errorf("writing goals: %w", err)
		}
	}

	fmt.Fprintf(opts.Stdout, "Added goal %q (type: %s, weight: %d)\n", opts.ID, goalType, opts.Weight)
	return nil
}

// InitOptions configures the goals init command.
type InitOptions struct {
	NonInteractive bool
	Template       string
	GoalsFile      string
	JSON           bool
	DryRun         bool
	Stdin          io.Reader
	Stdout         io.Writer
	TemplatesFS    fs.ReadFileFS
}

// GoalTemplate is the YAML structure of an embedded template file.
type GoalTemplate struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description"`
	Directives  []string           `yaml:"directives"`
	Gates       []GoalTemplateGate `yaml:"gates"`
}

// GoalTemplateGate mirrors a single gate entry in a template YAML file.
type GoalTemplateGate struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
	Check       string `yaml:"check"`
	Weight      int    `yaml:"weight"`
	Type        string `yaml:"type"`
}

// ValidTemplateNames lists the recognised --template values.
var ValidTemplateNames = []string{"go-cli", "python-lib", "web-app", "rust-cli", "generic"}

// BuildDefaultGoalFile creates a GoalFile with sensible defaults.
func BuildDefaultGoalFile() *GoalFile {
	dir, err := os.Getwd()
	if err != nil {
		dir = "project"
	}
	dirName := filepath.Base(dir)

	return &GoalFile{
		Version: 4, Format: "md",
		Mission:    fmt.Sprintf("Fitness goals for %s", dirName),
		NorthStars: []string{"All checks pass on every commit"},
		AntiStars:  []string{"Untested changes reaching main"},
		Directives: []Directive{{Number: 1, Title: "Establish baseline", Description: "Get all gates passing and maintain a green baseline.", Steer: "increase"}},
	}
}

// BuildInteractiveGoalFile prompts the user for goal file fields.
func BuildInteractiveGoalFile(r io.Reader) (*GoalFile, error) {
	scanner := bufio.NewScanner(r)

	mission, err := promptLine(scanner, "Mission (one sentence): ")
	if err != nil {
		return nil, err
	}
	if mission == "" {
		dir, _ := os.Getwd()
		if dir == "" {
			dir = "project"
		}
		mission = fmt.Sprintf("Fitness goals for %s", filepath.Base(dir))
	}

	northRaw, _ := promptLine(scanner, "North stars (comma-separated): ")
	northStars := SplitCommaSeparated(northRaw)
	if len(northStars) == 0 {
		northStars = []string{"All checks pass on every commit"}
	}

	antiRaw, _ := promptLine(scanner, "Anti stars (comma-separated): ")
	antiStars := SplitCommaSeparated(antiRaw)
	if len(antiStars) == 0 {
		antiStars = []string{"Untested changes reaching main"}
	}

	dirTitle, _ := promptLine(scanner, "First directive title: ")
	if dirTitle == "" {
		dirTitle = "Establish baseline"
	}

	dirDesc, _ := promptLine(scanner, "First directive description: ")
	if dirDesc == "" {
		dirDesc = "Get all gates passing and maintain a green baseline."
	}

	return &GoalFile{
		Version: 4, Format: "md", Mission: mission,
		NorthStars: northStars, AntiStars: antiStars,
		Directives: []Directive{{Number: 1, Title: dirTitle, Description: dirDesc, Steer: "increase"}},
	}, nil
}

// DetectGates checks for common project files and returns matching gate goals.
func DetectGates(projectRoot string) []Goal {
	var detected []Goal
	stat := func(rel string) bool {
		_, err := os.Stat(filepath.Join(projectRoot, rel))
		return err == nil
	}

	switch {
	case stat("cli/go.mod"):
		detected = append(detected,
			Goal{ID: "go-build", Description: "Go project builds cleanly", Check: "cd cli && go build ./...", Weight: 5, Type: GoalTypeHealth},
			Goal{ID: "go-test", Description: "Go tests pass", Check: "cd cli && go test ./...", Weight: 5, Type: GoalTypeHealth})
	case stat("go.mod"):
		detected = append(detected,
			Goal{ID: "go-build", Description: "Go project builds cleanly", Check: "go build ./...", Weight: 5, Type: GoalTypeHealth},
			Goal{ID: "go-test", Description: "Go tests pass", Check: "go test ./...", Weight: 5, Type: GoalTypeHealth})
	}
	if stat("package.json") {
		detected = append(detected, Goal{ID: "npm-test", Description: "npm tests pass", Check: "npm test", Weight: 5, Type: GoalTypeHealth})
	}
	if stat("Cargo.toml") {
		detected = append(detected, Goal{ID: "cargo-test", Description: "Cargo tests pass", Check: "cargo test", Weight: 5, Type: GoalTypeHealth})
	}
	if stat("pyproject.toml") {
		detected = append(detected, Goal{ID: "python-test", Description: "Python tests pass", Check: "pytest", Weight: 5, Type: GoalTypeHealth})
	}
	if stat("Makefile") {
		detected = append(detected, Goal{ID: "make-build", Description: "Make build succeeds", Check: "make build", Weight: 5, Type: GoalTypeHealth})
	}
	return detected
}

// LoadTemplate reads a named template from a filesystem.
func LoadTemplate(fsys fs.ReadFileFS, name string) (*GoalTemplate, error) {
	data, err := fs.ReadFile(fsys, filepath.Join("templates", name+".yaml"))
	if err != nil {
		return nil, fmt.Errorf("template %q not found: %w", name, err)
	}
	var tmpl GoalTemplate
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("parsing template %q: %w", name, err)
	}
	return &tmpl, nil
}

// TemplateGatesToGoals converts template gates into Goal values.
func TemplateGatesToGoals(tmpl *GoalTemplate) []Goal {
	out := make([]Goal, 0, len(tmpl.Gates))
	for _, g := range tmpl.Gates {
		out = append(out, Goal{ID: g.ID, Description: g.Description, Check: g.Check, Weight: g.Weight, Type: GoalType(g.Type)})
	}
	return out
}

// AutoDetectTemplate chooses a template name based on project marker files.
func AutoDetectTemplate(projectRoot string) string {
	stat := func(rel string) bool {
		_, err := os.Stat(filepath.Join(projectRoot, rel))
		return err == nil
	}
	switch {
	case stat("go.mod") || stat("cli/go.mod"):
		return "go-cli"
	case stat("Cargo.toml"):
		return "rust-cli"
	case stat("pyproject.toml") || stat("setup.py"):
		return "python-lib"
	case stat("package.json"):
		return "web-app"
	default:
		return ""
	}
}

// SplitCommaSeparated splits a comma-separated string, trimming whitespace.
func SplitCommaSeparated(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// RunPrune removes goals referencing nonexistent files.
func RunPrune(opts PruneOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}

	resolved := opts.GoalsFile
	resolvedPath := ResolveGoalsPath(resolved)

	gf, err := LoadGoals(resolved)
	if err != nil {
		return fmt.Errorf("loading goals: %w", err)
	}

	var stale []StaleGoal
	staleIDs := make(map[string]bool)

	for _, g := range gf.Goals {
		missingPath := FindMissingPath(g.Check)
		if missingPath != "" {
			stale = append(stale, StaleGoal{ID: g.ID, Check: g.Check, Path: missingPath})
			staleIDs[g.ID] = true
		}
	}

	result := PruneResult{StaleGoals: stale, DryRun: opts.DryRun}

	if opts.DryRun || len(stale) == 0 {
		if opts.JSON {
			enc := json.NewEncoder(opts.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}
		if len(stale) == 0 {
			fmt.Fprintln(opts.Stdout, "No stale goals found.")
			return nil
		}
		fmt.Fprintf(opts.Stdout, "Found %d stale goal(s):\n", len(stale))
		for _, s := range stale {
			fmt.Fprintf(opts.Stdout, "  %s: %s (missing: %s)\n", s.ID, s.Check, s.Path)
		}
		fmt.Fprintln(opts.Stdout, "\nRun without --dry-run to remove them.")
		return nil
	}

	var kept []Goal
	for _, g := range gf.Goals {
		if !staleIDs[g.ID] {
			kept = append(kept, g)
		}
	}
	gf.Goals = kept
	result.Removed = len(stale)

	if gf.Format == "md" {
		if err := WriteMDGoals(gf, resolvedPath); err != nil {
			return err
		}
	} else {
		data, err := yaml.Marshal(gf)
		if err != nil {
			return fmt.Errorf("marshaling goals: %w", err)
		}
		if err := os.WriteFile(resolvedPath, data, 0o600); err != nil {
			return fmt.Errorf("writing goals file: %w", err)
		}
	}

	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Fprintf(opts.Stdout, "Pruned %d stale goal(s) from %s\n", result.Removed, resolvedPath)
	for _, s := range stale {
		fmt.Fprintf(opts.Stdout, "  removed: %s (missing: %s)\n", s.ID, s.Path)
	}
	return nil
}

// RunInit bootstraps a new GOALS.md file.
func RunInit(opts InitOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}

	resolvedPath := ResolveGoalsPath(opts.GoalsFile)

	if _, err := os.Stat(resolvedPath); err == nil {
		return fmt.Errorf("goals file already exists: %s", resolvedPath)
	}
	if resolvedPath != opts.GoalsFile {
		if _, err := os.Stat(opts.GoalsFile); err == nil {
			return fmt.Errorf("goals file already exists: %s", opts.GoalsFile)
		}
	}

	projectRoot := filepath.Dir(resolvedPath)
	tmplName := opts.Template
	if tmplName == "" {
		tmplName = AutoDetectTemplate(projectRoot)
	}

	var tmpl *GoalTemplate
	if tmplName != "" && opts.TemplatesFS != nil {
		var err error
		tmpl, err = LoadTemplate(opts.TemplatesFS, tmplName)
		if err != nil {
			return fmt.Errorf("loading template %q: %w", tmplName, err)
		}
	}

	var gf *GoalFile
	if opts.NonInteractive {
		gf = BuildDefaultGoalFile()
	} else {
		var err error
		gf, err = BuildInteractiveGoalFile(opts.Stdin)
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
	}

	if tmpl != nil {
		gf.Goals = append(gf.Goals, TemplateGatesToGoals(tmpl)...)
	} else {
		gf.Goals = append(gf.Goals, DetectGates(projectRoot)...)
	}

	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(gf)
	}

	content := RenderGoalsMD(gf)

	outPath := resolvedPath
	if strings.ToLower(filepath.Ext(outPath)) != ".md" {
		outPath = filepath.Join(filepath.Dir(outPath), "GOALS.md")
	}

	if opts.DryRun {
		fmt.Fprintf(opts.Stdout, "Would write %s:\n\n%s", outPath, content)
		return nil
	}

	if err := os.WriteFile(outPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing goals file: %w", err)
	}

	fmt.Fprintf(opts.Stdout, "Created %s with %d gates\n", outPath, len(gf.Goals))
	return nil
}

func promptLine(scanner *bufio.Scanner, msg string) (string, error) {
	fmt.Print(msg)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}
