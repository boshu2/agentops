// Package goalstrace implements a read-only graph walker that builds the
// executable-spec trace chain defined in docs/adr/ADR-0005-trace-link-convention.md:
//
//	directive -> linked scenarios -> beads claiming them -> verdicts/results -> learnings.
//
// The walker performs NO mutation anywhere. It reads GOALS.md (via the F1.0
// patcher), scenario JSON files, the scenario-results artifact, beads (via the
// `bd` CLI), and learning files, then emits a connected, deterministically
// ordered graph of typed edges.
//
// Confidence rule (ADR-0005 §3): edges discovered via exact ID matches are
// ConfidenceHigh and count as closure proof; edges discovered via heuristic
// free-text matches are ConfidenceLow and NEVER count as closure proof.
//
// This is the F4.1 walker. The F4.2/F4.3 CLI commands (`ao goals trace`,
// `ao goals render`) consume the Graph produced here.
package goalstrace

import "sort"

// NodeType enumerates the five node kinds in the trace graph (ADR-0005 §Context).
type NodeType string

const (
	// NodeDirective is a GOALS.md directive block with a stable d- ID.
	NodeDirective NodeType = "directive"
	// NodeScenario is a scenario JSON file (promoted spec or ad hoc holdout).
	NodeScenario NodeType = "scenario"
	// NodeBead is a bd issue.
	NodeBead NodeType = "bead"
	// NodeArtifact is an RPI run artifact (verdict file, scenario-result, etc.).
	NodeArtifact NodeType = "artifact"
	// NodeLearning is a docs/learnings/<date>-<slug>.md file.
	NodeLearning NodeType = "learning"
)

// EdgeType enumerates exactly the six edge types from ADR-0005 §1.
type EdgeType string

const (
	// EdgeDirectiveHasScenario links a directive to a linked scenario.
	EdgeDirectiveHasScenario EdgeType = "directive_has_scenario"
	// EdgeScenarioResult links a scenario to a result/verdict artifact.
	EdgeScenarioResult EdgeType = "scenario_result"
	// EdgeScenarioClaimedByBead links a scenario to a bead claiming it.
	EdgeScenarioClaimedByBead EdgeType = "scenario_claimed_by_bead"
	// EdgeBeadProducedArtifact links a bead to an artifact it produced.
	EdgeBeadProducedArtifact EdgeType = "bead_produced_artifact"
	// EdgeArtifactCitedByLearning links an artifact to a learning citing it.
	EdgeArtifactCitedByLearning EdgeType = "artifact_cited_by_learning"
	// EdgeDirectiveHasLearning links a directive to a learning recording it.
	EdgeDirectiveHasLearning EdgeType = "directive_has_learning"
)

// EdgeTypes returns all six ADR-0005 edge types in canonical order.
func EdgeTypes() []EdgeType {
	return []EdgeType{
		EdgeDirectiveHasScenario,
		EdgeScenarioResult,
		EdgeScenarioClaimedByBead,
		EdgeBeadProducedArtifact,
		EdgeArtifactCitedByLearning,
		EdgeDirectiveHasLearning,
	}
}

// Confidence classifies how an edge was discovered (ADR-0005 §3).
type Confidence string

const (
	// ConfidenceHigh marks an exact-ID match: counts as closure proof.
	ConfidenceHigh Confidence = "high"
	// ConfidenceLow marks a heuristic free-text match: NEVER closure proof.
	ConfidenceLow Confidence = "low"
)

// Severity classifies a link defect (ADR-0005 §4).
type Severity string

const (
	// SeverityError marks a broken explicit link (always surfaced).
	SeverityError Severity = "error"
	// SeverityWarning marks an absent optional yield (surfaced by default,
	// escalated to an error under --strict for some codes).
	SeverityWarning Severity = "warning"
)

// Defect codes from ADR-0005 §4.1 (errors) and §4.2 (warnings).
const (
	DefectBrokenScenarioRef       = "broken_scenario_ref"
	DefectBrokenDirectiveBackref  = "broken_directive_backref"
	DefectBrokenBeadScenarioClaim = "broken_bead_scenario_claim"
	DefectBrokenArtifactBeadRef   = "broken_artifact_bead_ref"
	DefectBrokenLearningDirRef    = "broken_learning_directive_ref"

	DefectScenarioNotPromoted = "scenario_not_promoted"
	DefectDirectiveNoScenario = "directive_no_scenarios"
	DefectScenarioNoBeadClaim = "scenario_no_bead_claim"
	DefectScenarioNoResult    = "scenario_no_result"
	DefectBeadNoArtifact      = "bead_no_artifact"
	DefectNoLearningYield     = "no_learning_yield"
	DefectLowConfidenceOnly   = "low_confidence_only"
)

// Defect is one link defect attached to an edge or node.
type Defect struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Detail   string   `json:"detail,omitempty"`
}

// Node is one node in the trace graph.
type Node struct {
	ID    string   `json:"id"`
	Type  NodeType `json:"type"`
	Label string   `json:"label,omitempty"`
	// Path is the evidence file for this node (GOALS.md, scenario JSON,
	// artifact, learning). Empty for beads, which have no local file.
	Path string `json:"path,omitempty"`
}

// Edge is one typed link between two nodes (ADR-0005 §5 output contract).
type Edge struct {
	Type       EdgeType   `json:"edge_type"`
	FromID     string     `json:"from_id"`
	ToID       string     `json:"to_id"`
	Confidence Confidence `json:"confidence"`
	// Evidence is the file path that justifies the edge.
	Evidence string   `json:"evidence,omitempty"`
	Defects  []Defect `json:"defects,omitempty"`
}

// Graph is the connected executable-spec trace graph for one or more
// directives. Nodes and Edges are deterministically ordered (see Sort).
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
	// Diagnostics records non-fatal degradation messages (e.g. bd absent,
	// learnings dir absent).
	Diagnostics []string `json:"diagnostics,omitempty"`
}

// Summary is the aggregate record emitted last in the walker JSON stream
// (ADR-0005 §5).
type Summary struct {
	Summary            bool `json:"summary"`
	ErrorCount         int  `json:"error_count"`
	WarningCount       int  `json:"warning_count"`
	LowConfidenceEdges int  `json:"low_confidence_edges"`
}

// builder accumulates nodes and edges during a walk, de-duplicating nodes by ID.
type builder struct {
	nodeIdx map[string]int
	nodes   []Node
	edges   []Edge
	diags   []string
}

func newBuilder() *builder {
	return &builder{nodeIdx: map[string]int{}}
}

// addNode inserts a node, keeping the first label/path seen for a given ID.
func (b *builder) addNode(n Node) {
	if i, ok := b.nodeIdx[n.ID]; ok {
		if b.nodes[i].Label == "" {
			b.nodes[i].Label = n.Label
		}
		if b.nodes[i].Path == "" {
			b.nodes[i].Path = n.Path
		}
		return
	}
	b.nodeIdx[n.ID] = len(b.nodes)
	b.nodes = append(b.nodes, n)
}

// addEdge appends an edge to the builder.
func (b *builder) addEdge(e Edge) {
	b.edges = append(b.edges, e)
}

// addDiag records a degradation diagnostic.
func (b *builder) addDiag(msg string) {
	b.diags = append(b.diags, msg)
}

// graph finalizes the builder into a deterministically ordered Graph.
func (b *builder) graph() Graph {
	g := Graph{Nodes: b.nodes, Edges: b.edges, Diagnostics: b.diags}
	g.Sort()
	return g
}

// Sort orders nodes and edges deterministically so JSON output is stable
// across runs (ADR-0005 acceptance: "JSON graph ordering deterministic").
func (g *Graph) Sort() {
	sort.SliceStable(g.Nodes, func(i, j int) bool {
		if g.Nodes[i].Type != g.Nodes[j].Type {
			return g.Nodes[i].Type < g.Nodes[j].Type
		}
		return g.Nodes[i].ID < g.Nodes[j].ID
	})
	sort.SliceStable(g.Edges, func(i, j int) bool {
		a, b := g.Edges[i], g.Edges[j]
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		if a.FromID != b.FromID {
			return a.FromID < b.FromID
		}
		if a.ToID != b.ToID {
			return a.ToID < b.ToID
		}
		return a.Evidence < b.Evidence
	})
}

// Summary computes the aggregate error/warning/low-confidence counts.
func (g *Graph) Summarize() Summary {
	s := Summary{Summary: true}
	for _, e := range g.Edges {
		if e.Confidence == ConfidenceLow {
			s.LowConfidenceEdges++
		}
		for _, d := range e.Defects {
			switch d.Severity {
			case SeverityError:
				s.ErrorCount++
			case SeverityWarning:
				s.WarningCount++
			}
		}
	}
	return s
}
