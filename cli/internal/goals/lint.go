package goals

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Lint-finding severities.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// Lint-finding codes.
const (
	CodeMissingScenario       = "missing-scenario"
	CodeDirectiveIDConflict   = "directive-id-conflict"
	CodeMalformedScenario     = "malformed-scenario"
	CodeZeroScenarioDirective = "zero-scenario-directive"
	CodeOrphanScenario        = "orphan-scenario"
)

// LintFinding is one executable-spec link-lint finding.
type LintFinding struct {
	Severity    string `json:"severity"`
	Code        string `json:"code"`
	DirectiveID string `json:"directive_id,omitempty"`
	ScenarioID  string `json:"scenario_id,omitempty"`
	Path        string `json:"path,omitempty"`
	Message     string `json:"message"`
}

// LintReport is the full executable-spec link-lint result.
type LintReport struct {
	Findings []LintFinding `json:"findings"`
	Errors   int           `json:"errors"`
	Warnings int           `json:"warnings"`
}

func (r *LintReport) add(severity, code, directiveID, scenarioID, path, msg string) {
	r.Findings = append(r.Findings, LintFinding{
		Severity: severity, Code: code, DirectiveID: directiveID,
		ScenarioID: scenarioID, Path: path, Message: msg,
	})
	if severity == SeverityError {
		r.Errors++
	} else {
		r.Warnings++
	}
}

// LintOptions configures RunLint.
type LintOptions struct {
	GoalsFile string
	Strict    bool
	JSON      bool
	// SpecDirs is the ordered scenario search path. The first entry is treated
	// as the tracked promoted-spec directory. Empty uses the default.
	SpecDirs []string
	Stdout   io.Writer
}

// LintScenarios checks the executable-spec link graph: every directive→scenario
// link and every scenario file. It is a pure function over an injectable
// resolver and file list, so it is unit-testable without file IO.
//
// Errors: a directive references a missing scenario file; a scenario's
// directive_id conflicts with the directive that lists it; a linked scenario
// file is malformed. Warnings: a directive has no linked active scenario; a
// scenario file is referenced by no directive (orphan). A holdout scenario
// with no directive_id is a legitimate ad hoc scenario and is not flagged.
func LintScenarios(directives []ParsedDirective, resolve ScenarioResolver, allFiles []ScenarioMeta, specDir string) LintReport {
	var rep LintReport
	linked := map[string]bool{}
	for _, d := range directives {
		active := 0
		for _, sid := range d.Scenarios {
			linked[sid] = true
			link := resolveLink(d.StableID, sid, resolve)
			switch link.LinkHealth {
			case LinkHealthMissing:
				rep.add(SeverityError, CodeMissingScenario, d.StableID, sid, "",
					fmt.Sprintf("directive #%d references scenario %s but no scenario file exists", d.Number, sid))
			case LinkHealthConflict:
				rep.add(SeverityError, CodeDirectiveIDConflict, d.StableID, sid, link.Path, link.Message)
			case LinkHealthError:
				rep.add(SeverityError, CodeMalformedScenario, d.StableID, sid, link.Path, link.Message)
			case LinkHealthOK:
				if link.Status == "active" {
					active++
				}
			}
		}
		if active == 0 {
			rep.add(SeverityWarning, CodeZeroScenarioDirective, d.StableID, "", "",
				fmt.Sprintf("directive #%d has no linked active scenario", d.Number))
		}
	}
	for _, f := range allFiles {
		if linked[f.ID] {
			continue
		}
		promoted := filepath.Dir(f.Path) == specDir
		if !promoted && f.DirectiveID == "" {
			continue // legitimate ad hoc holdout scenario — advisory only (ADR-0003)
		}
		msg := "scenario file is referenced by no directive"
		if f.DirectiveID != "" {
			msg = fmt.Sprintf("scenario claims directive %q but no directive lists it (one-sided link)", f.DirectiveID)
		}
		rep.add(SeverityWarning, CodeOrphanScenario, f.DirectiveID, f.ID, f.Path, msg)
	}
	return rep
}

// scanScenarioDir returns the scenario-file metadata for every *.json file in
// dir. A missing directory yields no files and no error.
func scanScenarioDir(dir string) ([]ScenarioMeta, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	var metas []ScenarioMeta
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-len(".json")]
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		var raw struct {
			ID                    string  `json:"id"`
			Status                string  `json:"status"`
			SatisfactionThreshold float64 `json:"satisfaction_threshold"`
			DirectiveID           string  `json:"directive_id"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			metas = append(metas, ScenarioMeta{ID: id, Path: path})
			continue
		}
		if raw.ID != "" {
			id = raw.ID
		}
		metas = append(metas, ScenarioMeta{
			ID: id, Status: raw.Status, SatisfactionThreshold: raw.SatisfactionThreshold,
			DirectiveID: raw.DirectiveID, Path: path,
		})
	}
	return metas, nil
}

// RunLint runs executable-spec link lint over GOALS.md and the scenario files.
// It exits non-zero when there is at least one error, or — under Strict — at
// least one warning.
func RunLint(opts LintOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	dirs := opts.SpecDirs
	if len(dirs) == 0 {
		dirs = DefaultScenarioDirs()
	}

	patcher, _, err := LoadGoalsPatcher(opts.GoalsFile)
	if err != nil {
		return fmt.Errorf("loading goals: %w", err)
	}
	var allFiles []ScenarioMeta
	for _, dir := range dirs {
		metas, err := scanScenarioDir(dir)
		if err != nil {
			return err
		}
		allFiles = append(allFiles, metas...)
	}

	report := LintScenarios(patcher.Directives(), FileScenarioResolver(dirs), allFiles, dirs[0])
	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		writeLintText(opts.Stdout, report)
	}

	if report.Errors > 0 {
		return fmt.Errorf("%d executable-spec link error(s) — fix the scenario links above", report.Errors)
	}
	if opts.Strict && report.Warnings > 0 {
		return fmt.Errorf("%d executable-spec link warning(s) under --strict", report.Warnings)
	}
	return nil
}

// writeLintText renders the lint report in a compact human format.
func writeLintText(w io.Writer, report LintReport) {
	if len(report.Findings) == 0 {
		fmt.Fprintln(w, "No executable-spec link defects found.")
		return
	}
	for _, f := range report.Findings {
		fmt.Fprintf(w, "[%s] %s", f.Severity, f.Code)
		if f.DirectiveID != "" {
			fmt.Fprintf(w, " directive=%s", f.DirectiveID)
		}
		if f.ScenarioID != "" {
			fmt.Fprintf(w, " scenario=%s", f.ScenarioID)
		}
		fmt.Fprintf(w, " — %s\n", f.Message)
	}
	fmt.Fprintf(w, "%d error(s), %d warning(s)\n", report.Errors, report.Warnings)
}
