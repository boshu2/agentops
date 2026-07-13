package packet

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func fourUmbrellaPacket() ExecutionPacket {
	p := validBase()
	p.SkillsLoaded = []ExecutionPacketSkillLoad{
		{Name: "rpi", Reason: "orchestrator"},
		{Name: "discovery", Reason: "shape intent"},
		{Name: "crank", Reason: "implement slices"},
		{Name: "validate", Reason: "prove acceptance"},
		{Name: "learn", Reason: "capture post-verdict observations"},
	}
	p.PhaseReceipts = []ExecutionPacketPhaseReceipt{
		{Phase: "discovery", Skill: "discovery", Status: "DONE", Artifact: ".agents/rpi/phase-1-summary.md"},
		{Phase: "crank", Skill: "crank", Status: "DONE", Artifact: ".agents/rpi/phase-2-summary.md"},
		{Phase: "validate", Skill: "validate", Status: "PASS", Artifact: ".agents/rpi/phase-3-summary.md"},
		{Phase: "learn", Skill: "learn", Status: "DONE", Artifact: ".agents/rpi/phase-4-summary.md"},
	}
	return p
}

func TestExecutionPacketLifecycleReceipts_RequiresFourOrderedUmbrellas(t *testing.T) {
	p := fourUmbrellaPacket()
	if err := p.ValidateLifecycleReceipts(); err != nil {
		t.Fatalf("ValidateLifecycleReceipts rejected four ordered umbrellas: %v", err)
	}

	missingLearn := fourUmbrellaPacket()
	missingLearn.PhaseReceipts = missingLearn.PhaseReceipts[:3]
	if err := missingLearn.ValidateLifecycleReceipts(); err == nil || !strings.Contains(err.Error(), "learn") {
		t.Fatalf("missing Learn error = %v, want diagnostic naming learn", err)
	}

	outOfOrder := fourUmbrellaPacket()
	outOfOrder.PhaseReceipts[2], outOfOrder.PhaseReceipts[3] = outOfOrder.PhaseReceipts[3], outOfOrder.PhaseReceipts[2]
	if err := outOfOrder.ValidateLifecycleReceipts(); err == nil || !strings.Contains(err.Error(), "phase_receipts[2]") {
		t.Fatalf("out-of-order error = %v, want diagnostic naming phase_receipts[2]", err)
	}
}

func TestExecutionPacketLifecycleReceipts_RoundTripWithoutExtensionStripping(t *testing.T) {
	want := fourUmbrellaPacket()
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeJSON(data)
	if err != nil {
		t.Fatalf("DecodeJSON rejected four-umbrella packet: %v", err)
	}
	if !reflect.DeepEqual(got.SkillsLoaded, want.SkillsLoaded) {
		t.Fatalf("SkillsLoaded = %#v, want %#v", got.SkillsLoaded, want.SkillsLoaded)
	}
	if !reflect.DeepEqual(got.PhaseReceipts, want.PhaseReceipts) {
		t.Fatalf("PhaseReceipts = %#v, want %#v", got.PhaseReceipts, want.PhaseReceipts)
	}
}
