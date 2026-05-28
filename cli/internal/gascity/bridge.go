package gascity

import "github.com/boshu2/agentops/cli/internal/posture"

// BridgeContract declares which layers a project imports from and exports to
// Gas City. It is the typed seam between this distribution (agentops) and
// the substrate (gascity). The cross-project invariant is checked in
// bridge_test.go: OwnedLayers (from posture.AgentOps) unioned with
// ImportedLayers (from AgentOpsBridge) and ExternalLayers must equal
// posture.AllLayers.
//
// Mirrors (in role, not language idiom) mt-olympus's
// crates/mt-olympus-core/src/gascity_event_bridge.rs — currently the only
// other typed cross-project Layer-5 import seam in the wild. See
// .agents/research/2026-05-26-three-project-stack-pattern.md §4.
type BridgeContract struct {
	ImportedLayers posture.LayerSet
	ExportedLayers posture.LayerSet
}

// AgentOpsBridge is the canonical contract for this distribution. AgentOps
// imports Layer 5 (Loop) from Gas City — the loop primitive that drives one
// research→plan→implement→validate→learn tick — and exports nothing back
// (Distribution-owned layers are realized in this repo, not handed to the
// substrate). Source: synthesis §3-§4.
var AgentOpsBridge = BridgeContract{
	ImportedLayers: posture.NewLayerSet(posture.LayerLoop),
	ExportedLayers: 0,
}
