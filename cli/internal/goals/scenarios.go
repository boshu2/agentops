package goals

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Link-health classifications for a directive→scenario link.
const (
	LinkHealthOK       = "ok"       // scenario file resolved and consistent
	LinkHealthMissing  = "missing"  // directive references a scenario with no file
	LinkHealthConflict = "conflict" // scenario's directive_id disagrees with the directive
	LinkHealthError    = "error"    // scenario file present but unreadable/malformed
)

// ScenarioMeta is the minimal scenario-file metadata the executable-spec layer
// reads to report link health. The full scenario JSON has many more fields;
// only these participate in directive↔scenario listing.
type ScenarioMeta struct {
	ID                    string
	Status                string
	SatisfactionThreshold float64
	DirectiveID           string // the scenario file's own directive_id, if any
	Path                  string // resolved file path
}

// ScenarioResolver locates a scenario file by ID. A nil meta with a nil error
// means "not found" (a missing link, not a hard failure).
type ScenarioResolver func(id string) (*ScenarioMeta, error)

// ScenarioLink is one directive→scenario link with resolved health.
type ScenarioLink struct {
	ScenarioID            string  `json:"scenario_id"`
	Status                string  `json:"status,omitempty"`
	SatisfactionThreshold float64 `json:"satisfaction_threshold,omitempty"`
	Path                  string  `json:"path,omitempty"`
	LinkHealth            string  `json:"link_health"`
	Message               string  `json:"message,omitempty"`
}

// DirectiveScenarios is the scenario membership of one directive.
type DirectiveScenarios struct {
	DirectiveNumber int            `json:"directive_number"`
	DirectiveID     string         `json:"directive_id,omitempty"`
	Title           string         `json:"title"`
	Scenarios       []ScenarioLink `json:"scenarios"`
}

// ScenariosReport is the full directive→scenarios listing.
type ScenariosReport struct {
	Directives []DirectiveScenarios `json:"directives"`
}

// ScenariosOptions configures RunScenarios.
type ScenariosOptions struct {
	GoalsFile    string
	DirectiveNum int    // 0 = no display-number filter
	DirectiveID  string // "" = no stable-ID filter
	JSON         bool
	Stdout       io.Writer
	Stderr       io.Writer
	// SpecDirs is the ordered scenario-file search path. Empty uses the
	// default (spec/scenarios, then .agents/holdout — see docs/adr/ADR-0003).
	SpecDirs []string
}

// DefaultScenarioDirs is the ordered search path for scenario files: promoted
// spec scenarios first, then ad hoc holdout scenarios (docs/adr/ADR-0003).
func DefaultScenarioDirs() []string {
	return []string{
		filepath.Join("spec", "scenarios"),
		filepath.Join(".agents", "holdout"),
	}
}

// FileScenarioResolver returns a resolver that reads scenario JSON from the
// given directories in order, returning the first match.
func FileScenarioResolver(dirs []string) ScenarioResolver {
	return func(id string) (*ScenarioMeta, error) {
		for _, dir := range dirs {
			path := filepath.Join(dir, id+".json")
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			var raw struct {
				Status                string  `json:"status"`
				SatisfactionThreshold float64 `json:"satisfaction_threshold"`
				DirectiveID           string  `json:"directive_id"`
			}
			if err := json.Unmarshal(data, &raw); err != nil {
				return nil, fmt.Errorf("%s: invalid JSON: %w", path, err)
			}
			return &ScenarioMeta{
				ID:                    id,
				Status:                raw.Status,
				SatisfactionThreshold: raw.SatisfactionThreshold,
				DirectiveID:           raw.DirectiveID,
				Path:                  path,
			}, nil
		}
		return nil, nil
	}
}

// FilterDirectives narrows a directive slice by display number and/or stable
// ID. A zero number and empty ID return the slice unchanged.
func FilterDirectives(directives []ParsedDirective, num int, id string) []ParsedDirective {
	if num == 0 && id == "" {
		return directives
	}
	out := make([]ParsedDirective, 0, len(directives))
	for _, d := range directives {
		if num != 0 && d.Number != num {
			continue
		}
		if id != "" && d.StableID != id {
			continue
		}
		out = append(out, d)
	}
	return out
}

// resolveLink classifies a single directive→scenario link.
func resolveLink(directiveStableID, scenarioID string, resolve ScenarioResolver) ScenarioLink {
	link := ScenarioLink{ScenarioID: scenarioID, LinkHealth: LinkHealthMissing}
	meta, err := resolve(scenarioID)
	switch {
	case err != nil:
		link.LinkHealth = LinkHealthError
		link.Message = err.Error()
	case meta == nil:
		link.Message = "no scenario file found on the search path"
	default:
		link.Status = meta.Status
		link.SatisfactionThreshold = meta.SatisfactionThreshold
		link.Path = meta.Path
		link.LinkHealth = LinkHealthOK
		if meta.DirectiveID != "" && directiveStableID != "" && meta.DirectiveID != directiveStableID {
			link.LinkHealth = LinkHealthConflict
			link.Message = fmt.Sprintf("scenario directive_id %q does not match directive %q",
				meta.DirectiveID, directiveStableID)
		}
	}
	return link
}

// BuildScenariosReport assembles the directive→scenario listing, resolving each
// linked scenario through the supplied resolver. It performs no file IO of its
// own, so it is fully unit-testable with a fake resolver.
func BuildScenariosReport(directives []ParsedDirective, resolve ScenarioResolver) ScenariosReport {
	rep := ScenariosReport{Directives: make([]DirectiveScenarios, 0, len(directives))}
	for _, d := range directives {
		ds := DirectiveScenarios{
			DirectiveNumber: d.Number,
			DirectiveID:     d.StableID,
			Title:           d.Title,
			Scenarios:       make([]ScenarioLink, 0, len(d.Scenarios)),
		}
		for _, sid := range d.Scenarios {
			ds.Scenarios = append(ds.Scenarios, resolveLink(d.StableID, sid, resolve))
		}
		rep.Directives = append(rep.Directives, ds)
	}
	return rep
}

// RunScenarios lists the holdout scenarios linked to each GOALS.md directive.
// It is read-only: it never mutates GOALS.md or any scenario file.
func RunScenarios(opts ScenariosOptions) error {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	dirs := opts.SpecDirs
	if len(dirs) == 0 {
		dirs = DefaultScenarioDirs()
	}

	patcher, _, err := LoadGoalsPatcher(opts.GoalsFile)
	if err != nil {
		return fmt.Errorf("loading goals: %w", err)
	}
	directives := FilterDirectives(patcher.Directives(), opts.DirectiveNum, opts.DirectiveID)
	if len(directives) == 0 && (opts.DirectiveNum != 0 || opts.DirectiveID != "") {
		return fmt.Errorf("no directive matches the filter (run 'ao goals scenarios' with no flags to list every directive)")
	}

	report := BuildScenariosReport(directives, FileScenarioResolver(dirs))
	if opts.JSON {
		enc := json.NewEncoder(opts.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	writeScenariosText(opts.Stdout, report)
	return nil
}

// writeScenariosText renders the report in a compact human format.
func writeScenariosText(w io.Writer, report ScenariosReport) {
	for _, d := range report.Directives {
		id := d.DirectiveID
		if id == "" {
			id = "(no stable id)"
		}
		fmt.Fprintf(w, "Directive %d [%s]: %s\n", d.DirectiveNumber, id, d.Title)
		if len(d.Scenarios) == 0 {
			fmt.Fprintln(w, "  (no linked scenarios)")
			continue
		}
		for _, s := range d.Scenarios {
			fmt.Fprintf(w, "  %-20s %-8s %s\n", s.ScenarioID, s.LinkHealth, scenarioDetail(s))
		}
	}
}

// scenarioDetail formats the status/threshold/message tail of a scenario line.
func scenarioDetail(s ScenarioLink) string {
	if s.LinkHealth == LinkHealthOK || s.LinkHealth == LinkHealthConflict {
		detail := fmt.Sprintf("status=%s threshold=%.2f", s.Status, s.SatisfactionThreshold)
		if s.Message != "" {
			detail += " — " + s.Message
		}
		return detail
	}
	return s.Message
}
