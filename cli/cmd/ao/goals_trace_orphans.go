// practices: [bdd-gherkin, llm-eval-harness]
package main

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/boshu2/agentops/cli/internal/goalstrace"
	"github.com/spf13/cobra"
)

// orphanFinding is one chain-gap defect reported by `ao goals trace --orphans`.
// It carries the ADR-0005 §5 orphan fields.
type orphanFinding struct {
	Severity    string `json:"severity"`
	Code        string `json:"code"`
	DirectiveID string `json:"directive_id,omitempty"`
	ScenarioID  string `json:"scenario_id,omitempty"`
	Path        string `json:"path,omitempty"`
	Message     string `json:"message"`
}

// runTraceOrphans audits the whole trace graph for gaps, reports them, and
// returns a non-zero exit when errors exist (always) or warnings exist under
// --strict (ADR-0005 §4.2).
func runTraceOrphans(cmd *cobra.Command, graph goalstrace.Graph) error {
	findings := collectOrphans(graph)
	sortOrphans(findings)
	if goalsJSONOutput() {
		if err := emitOrphansJSON(cmd, findings); err != nil {
			return err
		}
	} else {
		renderOrphans(cmd, findings)
	}
	return orphanExit(findings)
}

// collectOrphans gathers every chain-gap finding from the graph: explicit
// broken-link errors carried on edge defects, plus missing-yield warnings
// derived from node coverage.
func collectOrphans(g goalstrace.Graph) []orphanFinding {
	var out []orphanFinding
	out = append(out, edgeDefectOrphans(g)...)
	out = append(out, missingYieldOrphans(g)...)
	return out
}

// edgeDefectOrphans converts every defect the walker attached to an edge into
// an orphanFinding, preserving the walker's error/warning severity.
func edgeDefectOrphans(g goalstrace.Graph) []orphanFinding {
	var out []orphanFinding
	for _, e := range g.Edges {
		for _, d := range e.Defects {
			out = append(out, orphanFinding{
				Severity:    string(d.Severity),
				Code:        d.Code,
				DirectiveID: orphanDirectiveID(e),
				ScenarioID:  orphanScenarioID(e),
				Path:        e.Evidence,
				Message:     orphanMessage(e, d),
			})
		}
	}
	return out
}

// orphanDirectiveID returns the directive endpoint of an edge, when the edge
// has one (directive_has_scenario / directive_has_learning start at a
// directive).
func orphanDirectiveID(e goalstrace.Edge) string {
	switch e.Type {
	case goalstrace.EdgeDirectiveHasScenario, goalstrace.EdgeDirectiveHasLearning:
		return e.FromID
	default:
		return ""
	}
}

// orphanScenarioID returns the scenario endpoint of an edge, when the edge has
// one.
func orphanScenarioID(e goalstrace.Edge) string {
	switch e.Type {
	case goalstrace.EdgeDirectiveHasScenario, goalstrace.EdgeScenarioResult:
		return e.ToID
	case goalstrace.EdgeScenarioClaimedByBead:
		return e.FromID
	default:
		return ""
	}
}

// orphanMessage formats a human-readable message for an edge defect, falling
// back to the defect's own detail when present.
func orphanMessage(e goalstrace.Edge, d goalstrace.Defect) string {
	if d.Detail != "" {
		return d.Detail
	}
	return fmt.Sprintf("%s defect on %s edge %s->%s", d.Code, e.Type, e.FromID, e.ToID)
}

// missingYieldOrphans derives warning-class findings the walker does not emit
// as edge defects: scenario nodes with no claiming bead, and directives with
// no learning yield.
func missingYieldOrphans(g goalstrace.Graph) []orphanFinding {
	cov := indexCoverage(g)
	var out []orphanFinding
	for _, n := range g.Nodes {
		switch n.Type {
		case goalstrace.NodeScenario:
			if !cov.scenarioClaimed[n.ID] {
				out = append(out, orphanFinding{
					Severity: string(goalstrace.SeverityWarning), Code: goalstrace.DefectScenarioNoBeadClaim,
					ScenarioID: n.ID, Path: n.Path,
					Message: "scenario " + n.ID + " has no claiming bead",
				})
			}
		case goalstrace.NodeDirective:
			if !cov.directiveHasLearning[n.ID] {
				out = append(out, orphanFinding{
					Severity: string(goalstrace.SeverityWarning), Code: goalstrace.DefectNoLearningYield,
					DirectiveID: n.ID, Path: n.Path,
					Message: "directive " + n.ID + " has no learning yield",
				})
			}
		}
	}
	return out
}

// coverageIndex records which scenarios are claimed and which directives have
// a learning, so the missing-yield pass is a single scan.
type coverageIndex struct {
	scenarioClaimed      map[string]bool
	directiveHasLearning map[string]bool
}

// indexCoverage builds the coverageIndex from the graph's edges.
func indexCoverage(g goalstrace.Graph) coverageIndex {
	cov := coverageIndex{
		scenarioClaimed:      map[string]bool{},
		directiveHasLearning: map[string]bool{},
	}
	for _, e := range g.Edges {
		switch e.Type {
		case goalstrace.EdgeScenarioClaimedByBead:
			cov.scenarioClaimed[e.FromID] = true
		case goalstrace.EdgeDirectiveHasLearning:
			if e.ToID != "" {
				cov.directiveHasLearning[e.FromID] = true
			}
		}
	}
	return cov
}

// sortOrphans orders findings deterministically: errors before warnings, then
// by code, directive, scenario, and path.
func sortOrphans(f []orphanFinding) {
	sort.SliceStable(f, func(i, j int) bool {
		a, b := f[i], f[j]
		if a.Severity != b.Severity {
			return a.Severity < b.Severity // "error" < "warning"
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.DirectiveID != b.DirectiveID {
			return a.DirectiveID < b.DirectiveID
		}
		if a.ScenarioID != b.ScenarioID {
			return a.ScenarioID < b.ScenarioID
		}
		return a.Path < b.Path
	})
}

// emitOrphansJSON writes each finding as one JSON object per line.
func emitOrphansJSON(cmd *cobra.Command, findings []orphanFinding) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	for _, f := range findings {
		if err := enc.Encode(f); err != nil {
			return fmt.Errorf("encoding orphan finding: %w", err)
		}
	}
	return nil
}

// renderOrphans prints the findings as a human-readable report.
func renderOrphans(cmd *cobra.Command, findings []orphanFinding) {
	w := cmd.OutOrStdout()
	if len(findings) == 0 {
		fmt.Fprintln(w, "No trace-chain orphans found.")
		return
	}
	errCount, warnCount := 0, 0
	for _, f := range findings {
		marker := "WARN "
		if f.Severity == string(goalstrace.SeverityError) {
			marker = "ERROR"
			errCount++
		} else {
			warnCount++
		}
		fmt.Fprintf(w, "%s  %-26s %s\n", marker, f.Code, f.Message)
	}
	fmt.Fprintf(w, "\n%d error(s), %d warning(s)\n", errCount, warnCount)
	if warnCount > 0 && !traceStrict {
		fmt.Fprintln(w, "(warnings do not fail the command; pass --strict to escalate)")
	}
}

// orphanExit returns a non-zero exit error when the findings include any error,
// or any warning while --strict is set (ADR-0005 §4.2).
func orphanExit(findings []orphanFinding) error {
	errCount, warnCount := 0, 0
	for _, f := range findings {
		if f.Severity == string(goalstrace.SeverityError) {
			errCount++
		} else {
			warnCount++
		}
	}
	if errCount > 0 {
		return fmt.Errorf("trace chain has %d error(s)", errCount)
	}
	if warnCount > 0 && traceStrict {
		return fmt.Errorf("trace chain has %d warning(s) (--strict)", warnCount)
	}
	return nil
}
