// practices: [bdd-gherkin, llm-eval-harness]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/boshu2/agentops/cli/internal/goalstrace"
	"github.com/spf13/cobra"
)

var (
	traceFrom    string
	traceOrphans bool
	traceStrict  bool
)

var goalsTraceCmd = &cobra.Command{
	Use:     "trace",
	Short:   "Trace the directive→scenario→bead→artifact→learning executable-spec chain",
	GroupID: "analysis",
	Args:    cobra.NoArgs,
	Long: `Walk the executable-spec trace chain defined in docs/adr/ADR-0005.

Two modes:

  ao goals trace --from <id>     Render the lineage rooted at a directive,
                                 scenario, or bead ID as a tree.
  ao goals trace --orphans       Audit the whole chain for gaps: broken
                                 references (errors) and missing yields
                                 (warnings).

  ao goals trace --from d-fitness-gate-bdd        tree from a directive
  ao goals trace --from s-2026-05-17-001 -o json  graph as line-delimited JSON
  ao goals trace --orphans                        whole-chain gap audit
  ao goals trace --orphans --strict               warnings fail too (exit 1)

--strict escalates warning-class defects to a non-zero exit (ADR-0005 §4.2);
errors always exit non-zero regardless of --strict.

Relationship to "ao goals scenarios --lint": lint is an edit-time check of a
single directive↔scenario edge; trace --orphans is a review-time audit of the
whole directive→scenario→bead→artifact→learning chain.`,
	RunE: runGoalsTrace,
}

// runGoalsTrace dispatches between --from (tree/graph render) and --orphans
// (chain-gap audit). Exactly one mode must be selected.
func runGoalsTrace(cmd *cobra.Command, _ []string) error {
	if traceFrom == "" && !traceOrphans {
		return fmt.Errorf("ao goals trace requires either --from <id> or --orphans")
	}
	if traceFrom != "" && traceOrphans {
		return fmt.Errorf("ao goals trace: --from and --orphans are mutually exclusive")
	}
	root := traceProjectRoot()
	graph, err := goalstrace.Walk(goalstrace.Options{ProjectRoot: root})
	if err != nil {
		return fmt.Errorf("building trace graph: %w", err)
	}
	if traceOrphans {
		return runTraceOrphans(cmd, graph)
	}
	return runTraceFrom(cmd, graph)
}

// traceProjectRoot returns the project root the walker resolves against. The
// walker resolves GOALS.md, scenarios, and artifacts relative to it; the
// current working directory is the project root for CLI invocations.
func traceProjectRoot() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

// runTraceFrom renders the lineage rooted at traceFrom. Human output is a
// tree; -o json emits the rooted subgraph per ADR-0005 §5.
func runTraceFrom(cmd *cobra.Command, graph goalstrace.Graph) error {
	sub, ok := rootedSubgraph(graph, traceFrom)
	if !ok {
		return fmt.Errorf("ao goals trace --from: no directive, scenario, or bead node %q in the trace graph", traceFrom)
	}
	if goalsJSONOutput() {
		return emitGraphJSON(cmd, sub)
	}
	renderTree(cmd, sub, traceFrom)
	return nil
}

// rootedSubgraph extracts the connected subgraph reachable from rootID by
// forward edge traversal. The second return is false when rootID names no
// node in the graph.
func rootedSubgraph(g goalstrace.Graph, rootID string) (goalstrace.Graph, bool) {
	nodeByID := map[string]goalstrace.Node{}
	for _, n := range g.Nodes {
		nodeByID[n.ID] = n
	}
	if _, ok := nodeByID[rootID]; !ok {
		return goalstrace.Graph{}, false
	}
	reached := map[string]bool{rootID: true}
	expandReachable(g, reached)

	sub := goalstrace.Graph{Diagnostics: g.Diagnostics}
	for _, n := range g.Nodes {
		if reached[n.ID] {
			sub.Nodes = append(sub.Nodes, n)
		}
	}
	for _, e := range g.Edges {
		if reached[e.FromID] && reached[e.ToID] {
			sub.Edges = append(sub.Edges, e)
		}
	}
	sub.Sort()
	return sub, true
}

// expandReachable grows the reached set to a fixed point by following every
// forward edge whose source is already reached.
func expandReachable(g goalstrace.Graph, reached map[string]bool) {
	for {
		grew := false
		for _, e := range g.Edges {
			if reached[e.FromID] && e.ToID != "" && !reached[e.ToID] {
				reached[e.ToID] = true
				grew = true
			}
		}
		if !grew {
			return
		}
	}
}

// emitGraphJSON writes the graph as a line-delimited JSON stream per ADR-0005
// §5: one edge object per line, then a final summary record.
func emitGraphJSON(cmd *cobra.Command, g goalstrace.Graph) error {
	w := cmd.OutOrStdout()
	enc := json.NewEncoder(w)
	for _, e := range g.Edges {
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("encoding edge: %w", err)
		}
	}
	if err := enc.Encode(g.Summarize()); err != nil {
		return fmt.Errorf("encoding summary: %w", err)
	}
	return nil
}

// renderTree prints the rooted subgraph as an indented human-readable tree.
func renderTree(cmd *cobra.Command, g goalstrace.Graph, rootID string) {
	w := cmd.OutOrStdout()
	nodeByID := map[string]goalstrace.Node{}
	for _, n := range g.Nodes {
		nodeByID[n.ID] = n
	}
	children := childIndex(g)
	fmt.Fprintf(w, "%s\n", nodeLine(nodeByID[rootID], rootID))
	visited := map[string]bool{rootID: true}
	printChildren(w, rootID, children, nodeByID, visited, "")
	s := g.Summarize()
	fmt.Fprintf(w, "\n%d node(s), %d edge(s); %d error(s), %d warning(s), %d low-confidence edge(s)\n",
		len(g.Nodes), len(g.Edges), s.ErrorCount, s.WarningCount, s.LowConfidenceEdges)
}

// childIndex groups edges by their source node ID, preserving the graph's
// deterministic edge order.
func childIndex(g goalstrace.Graph) map[string][]goalstrace.Edge {
	out := map[string][]goalstrace.Edge{}
	for _, e := range g.Edges {
		if e.ToID == "" {
			continue
		}
		out[e.FromID] = append(out[e.FromID], e)
	}
	return out
}

// printChildren recursively prints the child edges of fromID. visited guards
// against cycles so a malformed graph cannot loop forever.
func printChildren(w writer, fromID string, children map[string][]goalstrace.Edge,
	nodeByID map[string]goalstrace.Node, visited map[string]bool, indent string) {
	edges := children[fromID]
	for i, e := range edges {
		branch, nextIndent := treeGlyph(indent, i == len(edges)-1)
		fmt.Fprintf(w, "%s%s %s\n", branch, edgeLabel(e), nodeLine(nodeByID[e.ToID], e.ToID))
		if visited[e.ToID] {
			continue
		}
		visited[e.ToID] = true
		printChildren(w, e.ToID, children, nodeByID, visited, nextIndent)
	}
}

// writer is the minimal interface renderTree helpers need.
type writer interface{ Write([]byte) (int, error) }

// treeGlyph returns the branch prefix for a tree row and the indent to use for
// that row's own children.
func treeGlyph(indent string, last bool) (branch, nextIndent string) {
	if last {
		return indent + "└── ", indent + "    "
	}
	return indent + "├── ", indent + "│   "
}

// edgeLabel renders an edge as a compact "[edge_type:confidence]" tag, marking
// any attached defects.
func edgeLabel(e goalstrace.Edge) string {
	label := fmt.Sprintf("[%s:%s]", e.Type, e.Confidence)
	if len(e.Defects) > 0 {
		codes := make([]string, 0, len(e.Defects))
		for _, d := range e.Defects {
			codes = append(codes, string(d.Severity)+":"+d.Code)
		}
		sort.Strings(codes)
		label += " (" + strings.Join(codes, ", ") + ")"
	}
	return label
}

// nodeLine renders a node as "type id — label", tolerating an unknown node.
func nodeLine(n goalstrace.Node, fallbackID string) string {
	if n.ID == "" {
		return "(unresolved) " + fallbackID
	}
	if n.Label != "" {
		return fmt.Sprintf("%s %s — %s", n.Type, n.ID, n.Label)
	}
	return fmt.Sprintf("%s %s", n.Type, n.ID)
}

func init() {
	goalsTraceCmd.Flags().StringVar(&traceFrom, "from", "", "Render the trace lineage rooted at this directive, scenario, or bead ID")
	goalsTraceCmd.Flags().BoolVar(&traceOrphans, "orphans", false, "Audit the whole chain for broken references (errors) and missing yields (warnings)")
	goalsTraceCmd.Flags().BoolVar(&traceStrict, "strict", false, "Escalate warning-class defects to a non-zero exit (ADR-0005 §4.2)")
	goalsCmd.AddCommand(goalsTraceCmd)
}
