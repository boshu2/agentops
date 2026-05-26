package gascity

import (
	"testing"

	"github.com/boshu2/agentops/cli/internal/posture"
)

// TestAgentOpsStanceCoversAllLayers asserts the central invariant of the
// six-layer posture contract: every layer is either owned by agentops,
// imported via the GC bridge, or realized by a named external dependency
// (currently just `bd` for LayerBead — see posture.ExternalLayers). If a
// future PR adds a seventh layer to posture.AllLayers without extending one
// of the three sources, this test fails — surfacing the posture violation
// before merge.
//
// Source: .agents/research/2026-05-26-three-project-stack-pattern.md §3-§5.
func TestAgentOpsStanceCoversAllLayers(t *testing.T) {
	covered := posture.AgentOps.OwnedLayers.
		Union(AgentOpsBridge.ImportedLayers).
		Union(posture.ExternalLayers)
	if covered != posture.AllLayers {
		t.Fatalf("posture coverage gap:\n"+
			"  owned    = %08b (%s)\n"+
			"  import   = %08b (Layer 5 from GC)\n"+
			"  external = %08b (Layer 2 from `bd`)\n"+
			"  covered  = %08b\n"+
			"  all      = %08b\n\n"+
			"Fix: extend posture.AgentOps.OwnedLayers (cli/internal/posture/posture.go),\n"+
			"     AgentOpsBridge.ImportedLayers (cli/internal/gascity/bridge.go),\n"+
			"  or posture.ExternalLayers (if the new layer is realized by an external dep).\n"+
			"Reference: .agents/research/2026-05-26-three-project-stack-pattern.md",
			uint8(posture.AgentOps.OwnedLayers),
			posture.AgentOps.Kind,
			uint8(AgentOpsBridge.ImportedLayers),
			uint8(posture.ExternalLayers),
			uint8(covered),
			uint8(posture.AllLayers))
	}
}

// TestImportedLayersSubsetOfL5 asserts AgentOps doesn't claim to import more
// from Gas City than the Layer-5 (Loop) seam it actually consumes. Per the
// synthesis, gascity is the substrate (owns Loop+Distribution); agentops as
// a Distribution imports Loop. Importing more would mean re-implementing
// Distribution-layer logic from inside Gas City, which violates the posture.
func TestImportedLayersSubsetOfL5(t *testing.T) {
	expected := posture.NewLayerSet(posture.LayerLoop)
	if AgentOpsBridge.ImportedLayers != expected {
		t.Fatalf("bridge imports more than Layer 5:\n"+
			"  got      = %08b\n"+
			"  expected = %08b\n\n"+
			"If you intend to import additional layers from GC, update the doctrine "+
			"first (docs/architecture/operating-loop.md) before extending this bridge.",
			uint8(AgentOpsBridge.ImportedLayers),
			uint8(expected))
	}
}

// TestNoSeventhLayer asserts the layer enum stays at six. A seventh layer
// would land either as (a) a new Layer constant in cli/internal/posture/posture.go
// without updating posture.AllLayers — caught by TestAgentOpsStanceCoversAllLayers
// — or (b) updating posture.AllLayers without updating the canonical list here —
// caught by this test. Either failure surfaces the policy violation.
func TestNoSeventhLayer(t *testing.T) {
	canonical := posture.NewLayerSet(
		posture.LayerRole,
		posture.LayerBead,
		posture.LayerSkill,
		posture.LayerLedger,
		posture.LayerLoop,
		posture.LayerDistribution,
	)
	if canonical != posture.AllLayers {
		t.Fatalf("AllLayers drift: canonical=%08b AllLayers=%08b — a seventh layer "+
			"was likely added without updating the canonical list. The six-layer "+
			"model is invariant per synthesis §1; adding a layer requires updating "+
			"the synthesis and the doctrine first.",
			uint8(canonical), uint8(posture.AllLayers))
	}
}

// TestAgentOpsBridgeExportsNothing asserts the Distribution posture: a
// Distribution realizes its owned layers in its own repo; it does not hand
// those realizations back to the substrate. ExportedLayers must be empty.
func TestAgentOpsBridgeExportsNothing(t *testing.T) {
	if AgentOpsBridge.ExportedLayers != 0 {
		t.Fatalf("bridge exports layers back to GC: %08b — a Distribution posture "+
			"realizes owned layers in-repo, not via export to the substrate. "+
			"If you're trying to publish a layer to GC, you may be moving toward "+
			"the Sovereign or Substrate posture; reread synthesis §3 first.",
			uint8(AgentOpsBridge.ExportedLayers))
	}
}
