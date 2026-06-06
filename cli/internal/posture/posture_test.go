package posture

import "testing"

// TestLayer_String asserts every canonical layer renders its exact name and
// that out-of-range values degrade to a sentinel rather than panicking on the
// layerNames index.
func TestLayer_String(t *testing.T) {
	cases := []struct {
		layer Layer
		want  string
	}{
		{LayerRole, "Role"},
		{LayerBead, "Bead"},
		{LayerSkill, "Skill"},
		{LayerLedger, "Ledger"},
		{LayerLoop, "Loop"},
		{LayerDistribution, "Distribution"},
		{Layer(0), ""},             // index 0 is the empty sentinel
		{Layer(7), "UnknownLayer"}, // first index past the table
		{Layer(255), "UnknownLayer"},
	}
	for _, c := range cases {
		if got := c.layer.String(); got != c.want {
			t.Errorf("Layer(%d).String() = %q, want %q", c.layer, got, c.want)
		}
	}
}

// TestLayer_Values pins the bit positions. Layer values double as shift
// amounts in LayerSet, so reordering them silently corrupts every persisted
// or compared set — this guards the contract.
func TestLayer_Values(t *testing.T) {
	want := map[Layer]uint8{
		LayerRole:         1,
		LayerBead:         2,
		LayerSkill:        3,
		LayerLedger:       4,
		LayerLoop:         5,
		LayerDistribution: 6,
	}
	for layer, val := range want {
		if uint8(layer) != val {
			t.Errorf("%s has value %d, want %d", layer, uint8(layer), val)
		}
	}
}

// TestNewLayerSet_AndHas verifies set construction and membership, including
// that absent layers report false and that the empty set contains nothing.
func TestNewLayerSet_AndHas(t *testing.T) {
	s := NewLayerSet(LayerRole, LayerSkill)
	if !s.Has(LayerRole) {
		t.Error("set built with LayerRole should Has(LayerRole)")
	}
	if !s.Has(LayerSkill) {
		t.Error("set built with LayerSkill should Has(LayerSkill)")
	}
	if s.Has(LayerLedger) {
		t.Error("set without LayerLedger should not Has(LayerLedger)")
	}
	if s.Has(LayerDistribution) {
		t.Error("set without LayerDistribution should not Has(LayerDistribution)")
	}

	var empty LayerSet
	for _, l := range []Layer{LayerRole, LayerBead, LayerSkill, LayerLedger, LayerLoop, LayerDistribution} {
		if empty.Has(l) {
			t.Errorf("zero LayerSet should not Has(%s)", l)
		}
	}
}

// TestNewLayerSet_Idempotent confirms repeating a layer does not change the
// set (bit-OR is idempotent).
func TestNewLayerSet_Idempotent(t *testing.T) {
	once := NewLayerSet(LayerLoop)
	twice := NewLayerSet(LayerLoop, LayerLoop)
	if once != twice {
		t.Errorf("NewLayerSet(Loop) = %d, NewLayerSet(Loop, Loop) = %d; want equal", once, twice)
	}
}

// TestLayerSet_Union checks union is the set-theoretic OR: it contains every
// member of both operands and is commutative.
func TestLayerSet_Union(t *testing.T) {
	a := NewLayerSet(LayerRole, LayerSkill)
	b := NewLayerSet(LayerSkill, LayerLedger)
	u := a.Union(b)

	for _, l := range []Layer{LayerRole, LayerSkill, LayerLedger} {
		if !u.Has(l) {
			t.Errorf("union should Has(%s)", l)
		}
	}
	if u.Has(LayerLoop) {
		t.Error("union should not invent LayerLoop")
	}
	if a.Union(b) != b.Union(a) {
		t.Error("Union should be commutative")
	}
	// Union with the empty set is identity.
	var empty LayerSet
	if a.Union(empty) != a {
		t.Error("Union with empty set should equal the original set")
	}
}

// TestAllLayers asserts the universe contains exactly the six canonical
// layers and nothing outside them.
func TestAllLayers(t *testing.T) {
	for _, l := range []Layer{LayerRole, LayerBead, LayerSkill, LayerLedger, LayerLoop, LayerDistribution} {
		if !AllLayers.Has(l) {
			t.Errorf("AllLayers should Has(%s)", l)
		}
	}
	if AllLayers.Has(Layer(0)) {
		t.Error("AllLayers should not contain the empty sentinel layer 0")
	}
	if AllLayers.Has(Layer(7)) {
		t.Error("AllLayers should not contain an out-of-universe layer")
	}
}

// TestExternalLayers locks the "Bead is universal" synthesis claim: Bead — and
// only Bead — is realized outside the agentops/gascity/mt-olympus trio.
func TestExternalLayers(t *testing.T) {
	if !ExternalLayers.Has(LayerBead) {
		t.Error("ExternalLayers must contain LayerBead (provided by bd/beads_rust)")
	}
	for _, l := range []Layer{LayerRole, LayerSkill, LayerLedger, LayerLoop, LayerDistribution} {
		if ExternalLayers.Has(l) {
			t.Errorf("ExternalLayers should not contain %s; only Bead is external", l)
		}
	}
}

// TestKind_String asserts every posture family renders its exact name and
// out-of-range kinds degrade safely.
func TestKind_String(t *testing.T) {
	cases := []struct {
		kind Kind
		want string
	}{
		{KindSubstrate, "Substrate"},
		{KindDistribution, "Distribution"},
		{KindSovereign, "Sovereign"},
		{Kind(0), ""},
		{Kind(4), "UnknownKind"},
		{Kind(255), "UnknownKind"},
	}
	for _, c := range cases {
		if got := c.kind.String(); got != c.want {
			t.Errorf("Kind(%d).String() = %q, want %q", c.kind, got, c.want)
		}
	}
}

// TestAgentOpsStance is the regression guard that internal/gascity/bridge_test.go
// used to provide before that package was removed (ag-hfc, 3.1 teardown). The
// posture.go doc comment states the AgentOps claim "must either update this
// value or fail the integration test" — this test IS that enforcement: it pins
// what this repo declares about itself so a silent edit to the stance is caught.
func TestAgentOpsStance(t *testing.T) {
	if AgentOps.Kind != KindDistribution {
		t.Errorf("AgentOps.Kind = %s, want Distribution", AgentOps.Kind)
	}

	// AgentOps owns Role, Skill, Ledger, Distribution.
	for _, l := range []Layer{LayerRole, LayerSkill, LayerLedger, LayerDistribution} {
		if !AgentOps.OwnedLayers.Has(l) {
			t.Errorf("AgentOps should own %s", l)
		}
	}

	// It does NOT own Loop (imported from the substrate) or Bead (external dep).
	if AgentOps.OwnedLayers.Has(LayerLoop) {
		t.Error("AgentOps must not own Loop — it imports Loop from the substrate")
	}
	if AgentOps.OwnedLayers.Has(LayerBead) {
		t.Error("AgentOps must not own Bead — Bead is provided externally by bd")
	}

	// Exact-set assertion: owned == {Role, Skill, Ledger, Distribution}, no more.
	want := NewLayerSet(LayerRole, LayerSkill, LayerLedger, LayerDistribution)
	if AgentOps.OwnedLayers != want {
		t.Errorf("AgentOps.OwnedLayers = %d, want %d (Role|Skill|Ledger|Distribution)", AgentOps.OwnedLayers, want)
	}
}

// TestAgentOpsOwnedSubsetOfAll asserts the structural invariant that owned
// layers are always a subset of the recognized universe.
func TestAgentOpsOwnedSubsetOfAll(t *testing.T) {
	if AgentOps.OwnedLayers.Union(AllLayers) != AllLayers {
		t.Errorf("AgentOps.OwnedLayers (%d) is not a subset of AllLayers (%d)", AgentOps.OwnedLayers, AllLayers)
	}
}
