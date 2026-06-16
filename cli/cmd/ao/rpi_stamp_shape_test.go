// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/orchestration"
)

func TestParseAMActiveCount(t *testing.T) {
	// Real shape from `am robot agents --active --json`.
	out := []byte(`{"_meta":{"command":"robot agents"},"count":3,"agents":[]}`)
	if got := parseAMActiveCount(out); got != 3 {
		t.Fatalf("count = %d, want 3", got)
	}
	if got := parseAMActiveCount([]byte("not json")); got != 0 {
		t.Fatalf("malformed count = %d, want 0 floor", got)
	}
	if got := parseAMActiveCount([]byte(`{"count":-5}`)); got != 0 {
		t.Fatalf("negative count = %d, want 0 floor", got)
	}
}

func TestParseAMReservationWriteSets_GroupsByAgent(t *testing.T) {
	// Two lanes, one shared path (BlueLake + GreenCastle both hold dag.md).
	out := []byte(`{"all_active":[
		{"agent":"BlueLake","path":"skills/discovery/references/dag.md","exclusive":true},
		{"agent":"BlueLake","path":"cli/internal/orchestration/shape.go","exclusive":true},
		{"agent":"GreenCastle","path":"skills/discovery/references/dag.md","exclusive":true},
		{"agent":"","path":"ignored.go"},
		{"agent":"RedIsland","path":""}
	]}`)
	sets := parseAMReservationWriteSets(out)
	if len(sets) != 2 {
		t.Fatalf("lanes = %d, want 2 (empty agent/path skipped): %v", len(sets), sets)
	}
	// Sorted by agent: BlueLake first, GreenCastle second.
	if len(sets[0]) != 2 || sets[1][0] != "skills/discovery/references/dag.md" {
		t.Fatalf("unexpected grouping: %v", sets)
	}
	// The grouped sets must actually drive a contention verdict.
	v := orchestration.ValidateShape(orchestration.ShapeInputs{LiveWriters: 2, WriteSets: sets})
	if v.Shape != orchestration.ShapeAMOnly {
		t.Fatalf("overlapping lanes → shape = %s, want am-only", v.Shape)
	}
}

func TestParseAMReservationWriteSets_Empty(t *testing.T) {
	if got := parseAMReservationWriteSets([]byte(`{"all_active":[]}`)); got != nil {
		t.Fatalf("empty reservations = %v, want nil", got)
	}
	if got := parseAMReservationWriteSets([]byte("garbage")); got != nil {
		t.Fatalf("malformed reservations = %v, want nil", got)
	}
}

// writePacket writes a minimal hand-compiled packet like the live /discovery
// path produces (no orchestration_decision yet), plus a sentinel field that
// must survive the stamp.
func writeLivePacket(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "execution-packet.json")
	body := map[string]any{
		"schema_version":    1,
		"objective":         "wire the live discovery shape",
		"epic_id":           "age-gud",
		"contract_surfaces": []string{"skills/discovery/references/dag.md"},
		"tracker_mode":      "br",
	}
	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestStampShapeOnPacket_RoundTrip_PreservesFields(t *testing.T) {
	dir := t.TempDir()
	path := writeLivePacket(t, dir)

	verdict := orchestration.ShapeVerdict{
		Shape:           orchestration.ShapeATMOnly,
		PredicatesFired: []string{"durability:unattended"},
		Rationale:       "unattended fired",
	}
	if err := stampShapeOnPacket(path, verdict, "2026-06-16T00:00:00Z"); err != nil {
		t.Fatalf("stamp: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	// Sentinel fields preserved.
	if got["objective"] != "wire the live discovery shape" || got["epic_id"] != "age-gud" {
		t.Fatalf("stamp dropped existing fields: %v", got)
	}
	dec, ok := got["orchestration_decision"].(map[string]any)
	if !ok {
		t.Fatalf("orchestration_decision missing or wrong type: %v", got["orchestration_decision"])
	}
	if dec["chosen_shape"] != "atm-only" {
		t.Fatalf("chosen_shape = %v, want atm-only", dec["chosen_shape"])
	}
	fired, ok := dec["predicates_fired"].([]any)
	if !ok || len(fired) != 1 || fired[0] != "durability:unattended" {
		t.Fatalf("predicates_fired = %v, want [durability:unattended]", dec["predicates_fired"])
	}
	if dec["ts"] != "2026-06-16T00:00:00Z" {
		t.Fatalf("ts = %v", dec["ts"])
	}
}

func TestStampShapeOnPacket_PredicatesFiredAlwaysPresent(t *testing.T) {
	dir := t.TempDir()
	path := writeLivePacket(t, dir)
	// Single-agent verdict: no predicates fired, but the field must still be
	// present as an empty array (honest record).
	verdict := orchestration.ShapeVerdict{Shape: orchestration.ShapeSingleAgent, Rationale: "default"}
	if err := stampShapeOnPacket(path, verdict, "2026-06-16T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	dec := got["orchestration_decision"].(map[string]any)
	fired, ok := dec["predicates_fired"].([]any)
	if !ok || len(fired) != 0 {
		t.Fatalf("predicates_fired = %v, want present empty array", dec["predicates_fired"])
	}
}

// fakeAM returns canned am stdout per (robot, <subcommand>, ...).
func fakeAM(active, reservations string) amRunner {
	return func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[1] == "agents" {
			return []byte(active), nil
		}
		if len(args) >= 2 && args[1] == "reservations" {
			return []byte(reservations), nil
		}
		return []byte("{}"), nil
	}
}

func TestGatherShapeInputs_LiveContentionOverridesProposal(t *testing.T) {
	// The live path's worst case: model proposes single-agent ("disjoint lanes")
	// but ground truth shows 2 writers on the same path → override to am-only.
	active := `{"count":2}`
	reservations := `{"all_active":[
		{"agent":"A","path":"skills/discovery/references/dag.md"},
		{"agent":"B","path":"skills/discovery/references/dag.md"}
	]}`
	in := gatherShapeInputs("/repo", "single-agent", false, fakeAM(active, reservations))
	v := orchestration.ValidateShape(in)
	if v.Shape != orchestration.ShapeAMOnly {
		t.Fatalf("shape = %s, want am-only (contention override)", v.Shape)
	}
	if !v.Overridden {
		t.Fatalf("expected proposal overridden")
	}
	foundContention := false
	for _, p := range v.PredicatesFired {
		if p == "contention:live-writers-overlap" {
			foundContention = true
		}
	}
	if !foundContention {
		t.Fatalf("predicates_fired missing contention: %v", v.PredicatesFired)
	}
}

func TestGatherShapeInputs_NilRunnerIsSingleAgentFloor(t *testing.T) {
	in := gatherShapeInputs("/repo", "", false, nil)
	if in.LiveWriters != 0 || in.WriteSets != nil {
		t.Fatalf("nil runner should yield empty inputs: %+v", in)
	}
	if v := orchestration.ValidateShape(in); v.Shape != orchestration.ShapeSingleAgent {
		t.Fatalf("nil-runner shape = %s, want single-agent", v.Shape)
	}
}

func TestGatherShapeInputs_UnattendedDrivesATM(t *testing.T) {
	in := gatherShapeInputs("/repo", "", true, fakeAM(`{"count":0}`, `{"all_active":[]}`))
	if v := orchestration.ValidateShape(in); v.Shape != orchestration.ShapeATMOnly {
		t.Fatalf("unattended shape = %s, want atm-only", v.Shape)
	}
}
