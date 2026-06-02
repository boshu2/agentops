// Package posture types a project's stance toward the six canonical layers
// of the agent stack. It answers ONE question: how does this project relate
// to the substrate (NTM + MCP + managed-agents) and to the cross-project layer ownership map?
//
// It deliberately does NOT model the operational flow of context through a
// loop tick — that is the context-compiler thesis (the AgentOps 3.0 core),
// which lives in skills/domain/references/context-compiler.md and will land
// as Go types under cli/internal/compile/ per bead ag-mfh. Posture is the
// structural axis; compile is the operational axis. They are siblings, not
// substitutes.
//
// Domain vocabulary: a project's "posture" is its declared ownership stance
// over the six layers (Role, Bead, Skill, Ledger, Loop, Distribution). Three
// kinds of stance are recognized: Substrate (NTM+MCP+managed-agents), Distribution
// (agentops), Sovereign (mt-olympus). Source:
// .agents/research/2026-05-26-three-project-stack-pattern.md §1, §3.
//
// MINIMAL by design — additions require citation in
// docs/architecture/operating-loop.md or the synthesis above.
package posture

// Layer names one of the six canonical layers in the agent stack. Values
// double as bit positions in LayerSet, which fixes the universe at six.
type Layer uint8

const (
	LayerRole         Layer = 1 // identity definition (TOML/MD persona)
	LayerBead         Layer = 2 // atomic, dep-aware work unit
	LayerSkill        Layer = 3 // portable knowledge artifact (SKILL.md)
	LayerLedger       Layer = 4 // append-only JSONL evidence
	LayerLoop         Layer = 5 // one research→plan→implement→validate tick
	LayerDistribution Layer = 6 // named role identity importable into a runtime
)

var layerNames = [...]string{"", "Role", "Bead", "Skill", "Ledger", "Loop", "Distribution"}

func (l Layer) String() string {
	if int(l) < len(layerNames) {
		return layerNames[l]
	}
	return "UnknownLayer"
}

// LayerSet is a compact bit-set over the six layers. Cross-project posture
// invariant: OwnedLayers ∪ ImportedLayers ∪ ExternalLayers == AllLayers.
type LayerSet uint8

// NewLayerSet builds a LayerSet from zero or more layers.
func NewLayerSet(layers ...Layer) LayerSet {
	var s LayerSet
	for _, l := range layers {
		s |= 1 << l
	}
	return s
}

// Has reports whether the set contains the given layer.
func (s LayerSet) Has(l Layer) bool { return s&(1<<l) != 0 }

// Union returns the set-theoretic union of s and other.
func (s LayerSet) Union(other LayerSet) LayerSet { return s | other }

// AllLayers is the universe of recognized layers.
var AllLayers = NewLayerSet(LayerRole, LayerBead, LayerSkill, LayerLedger, LayerLoop, LayerDistribution)

// ExternalLayers names layers realized by projects outside the
// agentops (Distribution) + the substrate + mt-olympus (Sovereign) trio. Layer 2 (Bead) is provided by the `bd`
// (beads_rust) project, on which all three postures depend. Synthesis §5
// notes this as "Bead is universal."
var ExternalLayers = NewLayerSet(LayerBead)

// Kind names the family a posture belongs to. Synthesis §3 derives these
// from observed practice in agentops (Distribution), the substrate (Substrate),
// and mt-olympus (Sovereign).
type Kind uint8

const (
	KindSubstrate    Kind = 1 // owns Loop+Distribution; provides registry
	KindDistribution Kind = 2 // owns Role+Skill+Ledger+Distribution; imports Loop
	KindSovereign    Kind = 3 // owns all six; adapts substrate at seam-of-choice
)

var kindNames = [...]string{"", "Substrate", "Distribution", "Sovereign"}

func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "UnknownKind"
}

// Stance is a project's declared ownership stance over the six layers. A
// Stance has a Kind (its family) and an OwnedLayers set (the specific
// layers this project realizes in-repo, as opposed to importing or
// receiving from an external dep).
type Stance struct {
	Kind        Kind
	OwnedLayers LayerSet
}

// AgentOps is what this repo claims about itself. Anything that contradicts
// this claim must either update this value or fail the integration test in
// the posture synthesis. Source: synthesis §3.
var AgentOps = Stance{
	Kind:        KindDistribution,
	OwnedLayers: NewLayerSet(LayerRole, LayerSkill, LayerLedger, LayerDistribution),
}
