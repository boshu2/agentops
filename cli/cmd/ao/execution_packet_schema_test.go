package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	packet "github.com/boshu2/agentops/cli/internal/domain/packet"
	cliRPI "github.com/boshu2/agentops/cli/internal/rpi"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestExecutionPacketSchemaValidatesTypedWorkPacketFields(t *testing.T) {
	schema := compileExecutionPacketSchemaForTest(t)
	packet := validExecutionPacketFixture()

	if err := validateExecutionPacketFixture(schema, packet); err != nil {
		t.Fatalf("valid execution packet failed schema validation: %v", err)
	}
}

func TestExecutionPacketDefaultVerdictResolvesFailClosed(t *testing.T) {
	schema := compileExecutionPacketSchemaForTest(t)

	packet := validExecutionPacketFixture()
	delete(packet, "default_verdict")
	if err := validateExecutionPacketFixture(schema, packet); err != nil {
		t.Fatalf("execution packet without default_verdict should remain valid: %v", err)
	}
	if got := unmarshalExecutionPacketForTest(t, packet).EffectiveVerdict(); got != cliRPI.ExecutionPacketVerdictFail {
		t.Fatalf("missing default_verdict resolved to %q, want %q", got, cliRPI.ExecutionPacketVerdictFail)
	}

	explicitPacket := validExecutionPacketFixture()
	explicitPacket["default_verdict"] = "PASS"
	if err := validateExecutionPacketFixture(schema, explicitPacket); err != nil {
		t.Fatalf("execution packet with explicit PASS default_verdict should validate: %v", err)
	}
	if got := unmarshalExecutionPacketForTest(t, explicitPacket).EffectiveVerdict(); got != cliRPI.ExecutionPacketVerdictPass {
		t.Fatalf("explicit default_verdict resolved to %q, want %q", got, cliRPI.ExecutionPacketVerdictPass)
	}

	legacyPacket := validExecutionPacketFixture()
	delete(legacyPacket, "routing")
	delete(legacyPacket, "default_verdict")
	delete(legacyPacket, "spec")
	if err := validateExecutionPacketFixture(schema, legacyPacket); err != nil {
		t.Fatalf("legacy execution packet without new typed fields should remain valid: %v", err)
	}
}

func TestExecutionPacketSchemaRejectsOffRosterRoutingFamily(t *testing.T) {
	schema := compileExecutionPacketSchemaForTest(t)
	packet := validExecutionPacketFixture()
	packet["routing"].(map[string]any)["agentops-dhk.1"].(map[string]any)["implementer"] = "llama"

	err := validateExecutionPacketFixture(schema, packet)
	if err == nil {
		t.Fatalf("schema accepted an off-roster routing family")
	}
	if !strings.Contains(err.Error(), "implementer") && !strings.Contains(err.Error(), "llama") {
		t.Fatalf("schema error = %q, want implementer/llama enum violation", err.Error())
	}
}

// TestExecutionPacketSchemaRejectsUnknownComplexity keeps complexity a closed
// enum (fast|standard|full) at the validation boundary. skills/rpi drives gate
// DEPTH off these exact values, so an out-of-enum complexity must not validate.
func TestExecutionPacketSchemaRejectsUnknownComplexity(t *testing.T) {
	schema := compileExecutionPacketSchemaForTest(t)
	packet := validExecutionPacketFixture()
	packet["complexity"] = "bogus"
	if err := validateExecutionPacketFixture(schema, packet); err == nil {
		t.Fatal("schema accepted an out-of-enum complexity; gate-depth selection must stay closed")
	}
}

// TestExecutionPacketSchemaAcceptsLiveEmittedSlimPacket pins the enforced schema
// to what `ao rpi` actually emits. A real on-disk execution packet (the slim shape
// with issues/done_when/likely_blocker/ignore_today, write_scope_first_slice, and
// null epic_id/ranked_packet_path) MUST validate — the absence of this test is what
// let the schema silently drift stricter than the emitter (age-55qz.2).
func TestExecutionPacketSchemaAcceptsLiveEmittedSlimPacket(t *testing.T) {
	schema := compileExecutionPacketSchemaForTest(t)
	data, err := os.ReadFile("testdata/live-execution-packet.json")
	if err != nil {
		t.Fatalf("read live execution packet testdata: %v", err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse live execution packet testdata: %v", err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("a real ao-rpi-emitted execution packet must validate against the enforced schema (schema drifted from the emitter): %v", err)
	}
}

// TestExecutionPacketLiveFixtureRoundTripsLossless guards the load->save round
// trip for the real ao-rpi packet. The schema accepting the slim shape is not
// enough: the struct must MODEL issues/done_when/ignore_today/write_scope_first_slice
// (else they drop on load) and nil optional slices must omit (not marshal as the
// schema-invalid `null` that contract_surfaces/write_scope used to). Reproduces
// the cross-family pawl's REFUTED defect on age-55qz.2.
func TestExecutionPacketLiveFixtureRoundTripsLossless(t *testing.T) {
	data, err := os.ReadFile("testdata/live-execution-packet.json")
	if err != nil {
		t.Fatalf("read live execution packet testdata: %v", err)
	}
	pkt, err := packet.DecodeJSON(data)
	if err != nil {
		t.Fatalf("DecodeJSON rejected the live ao-rpi packet: %v", err)
	}
	if len(pkt.Issues) == 0 {
		t.Fatal("issues[] dropped on load — struct does not model the live slim shape")
	}
	if len(pkt.DoneWhen) == 0 {
		t.Fatal("done_when dropped on load")
	}
	if len(pkt.IgnoreToday) == 0 {
		t.Fatal("ignore_today dropped on load")
	}
	if pkt.Density == nil || len(pkt.Density.Boundary.WriteScopeFirstSlice) == 0 {
		t.Fatal("boundary.write_scope_first_slice dropped on load")
	}
	out, err := json.Marshal(pkt)
	if err != nil {
		t.Fatalf("marshal loaded packet: %v", err)
	}
	if err := packet.ValidateJSON(out); err != nil {
		t.Fatalf("round-tripped packet no longer satisfies the schema (nil slice marshaled as null?): %v", err)
	}
	// Preservation, not just revalidation: the PERSISTED FORM must be a fixpoint.
	// The first load normalizes once (json null -> absent for nullable fields,
	// empty arrays omitted, default_verdict resolved fail-closed); every save
	// thereafter must be byte-identical, so repeated Load->Save can never
	// progressively drift a packet on disk.
	pkt2, err := packet.DecodeJSON(out)
	if err != nil {
		t.Fatalf("re-decode of the saved packet failed: %v", err)
	}
	out2, err := json.Marshal(pkt2)
	if err != nil {
		t.Fatalf("re-marshal of the saved packet failed: %v", err)
	}
	if !bytes.Equal(out, out2) {
		t.Fatalf("Load->Save is not a fixpoint — repeated saves drift the persisted bytes:\n first=%s\n second=%s", out, out2)
	}
}

// TestExecutionPacketEffectiveVerdictFailsClosedOnMalformed closes the verdict
// resolver's malformed-value path in the cmd/ao layer (previously only covered in
// internal/rpi). A junk default_verdict must resolve fail-closed to FAIL, not pass.
func TestExecutionPacketEffectiveVerdictFailsClosedOnMalformed(t *testing.T) {
	for _, bad := range []string{"MAYBE", "pass", " FAIL", "ok", ""} {
		packet := validExecutionPacketFixture()
		packet["default_verdict"] = bad
		if got := unmarshalExecutionPacketForTest(t, packet).EffectiveVerdict(); got != cliRPI.ExecutionPacketVerdictFail {
			t.Fatalf("malformed default_verdict %q resolved to %q, want fail-closed %q", bad, got, cliRPI.ExecutionPacketVerdictFail)
		}
	}
}

// TestExecutionPacketSchemaRejectsUnknownValidationLaneField completes .2's
// additionalProperties:false hardening for the one object field it had left
// unguarded: a validation_lane carrying a field the ValidationLane struct does
// not model would otherwise validate and then drop on Load->Save.
func TestExecutionPacketSchemaRejectsUnknownValidationLaneField(t *testing.T) {
	schema := compileExecutionPacketSchemaForTest(t)
	p := validExecutionPacketFixture()
	p["validation_lanes"] = []any{map[string]any{"name": "x", "future": "unmodeled"}}
	if err := validateExecutionPacketFixture(schema, p); err == nil {
		t.Fatal("schema accepted an unmodeled validation_lane field; lanes must round-trip losslessly")
	}
}

// TestExecutionPacketSchemaRejectsUnknownAutodevProgramField closes the
// autodev_program accepted-but-dropped hole: the schema was an open object while
// the struct models a fixed field set, so documented PROGRAM.md fields (e.g.
// objective/escalation) validated then dropped on Load->Save. autodev_program is
// now additionalProperties:false, so an unmodeled field is rejected loudly at the
// validation boundary rather than silently lost. Reproduces the cross-family
// pawl's second REFUTED defect on age-55qz.2.
func TestExecutionPacketSchemaRejectsUnknownAutodevProgramField(t *testing.T) {
	schema := compileExecutionPacketSchemaForTest(t)
	p := validExecutionPacketFixture()
	p["autodev_program"] = map[string]any{"path": "PROGRAM.md", "objective": "unmodeled-field"}
	if err := validateExecutionPacketFixture(schema, p); err == nil {
		t.Fatal("schema accepted an unmodeled autodev_program field; program metadata must round-trip losslessly")
	}
}

// TestExecutionPacketSchemaAcceptsModeledAutodevProgram keeps the modeled
// ExecutionPacketProgram fields valid after the additionalProperties:false close.
func TestExecutionPacketSchemaAcceptsModeledAutodevProgram(t *testing.T) {
	schema := compileExecutionPacketSchemaForTest(t)
	p := validExecutionPacketFixture()
	p["autodev_program"] = map[string]any{
		"path":            "PROGRAM.md",
		"mutable_scope":   []any{"cli/"},
		"experiment_unit": "bead",
		"stop_conditions": []any{"gate red"},
	}
	if err := validateExecutionPacketFixture(schema, p); err != nil {
		t.Fatalf("schema rejected a fully-modeled autodev_program: %v", err)
	}
}

// TestExecutionPacketValidationLaneExplicitFalseRoundTrips closes the lane
// round-trip presence gap: read_only/writes_artifacts/isolated_agents_home/
// release_only were omitempty, so a schema-valid lane carrying explicit false
// re-marshaled as {}, dropping the execution-policy bits. They now always
// serialize so DecodeJSON->Marshal is byte-faithful for explicit false.
// Reproduces the cross-family pawl's first REFUTED defect on age-55qz.2.
func TestExecutionPacketValidationLaneExplicitFalseRoundTrips(t *testing.T) {
	in := []byte(`{"name":"unit","command":"go test","read_only":false,"writes_artifacts":false,"isolated_agents_home":false,"release_only":false}`)
	var lane packet.ValidationLane
	if err := json.Unmarshal(in, &lane); err != nil {
		t.Fatalf("unmarshal lane: %v", err)
	}
	out, err := json.Marshal(lane)
	if err != nil {
		t.Fatalf("marshal lane: %v", err)
	}
	for _, key := range []string{"read_only", "writes_artifacts", "isolated_agents_home", "release_only"} {
		if !bytes.Contains(out, []byte(`"`+key+`"`)) {
			t.Fatalf("lane re-marshal dropped %q (explicit false not preserved): %s", key, out)
		}
	}
}

// TestExecutionPacketSchemaRejectsEmptyObjective keeps objective non-empty at the
// validation boundary. objective was string-only (empty allowed), so a legacy
// packet with no objective migrated to a rich packet with objective:"" and
// persisted as a meaningless packet. objective now has minLength:1.
func TestExecutionPacketSchemaRejectsEmptyObjective(t *testing.T) {
	schema := compileExecutionPacketSchemaForTest(t)
	p := validExecutionPacketFixture()
	p["objective"] = ""
	if err := validateExecutionPacketFixture(schema, p); err == nil {
		t.Fatal("schema accepted an empty objective; the migration fail-closed claim requires a non-empty objective")
	}
}

// TestDecodeJSONFailsClosedOnEmptyObjectiveLegacyMigration is the end-to-end
// guard for the migration fail-open the cross-family pawl found on age-55qz.2: a
// legacy-looking object with a VALID complexity but no objective must NOT migrate
// into a persisted empty-objective packet.
func TestDecodeJSONFailsClosedOnEmptyObjectiveLegacyMigration(t *testing.T) {
	if _, err := packet.DecodeJSON([]byte(`{"test_levels":["L1"],"complexity":"standard"}`)); err == nil {
		t.Fatal("DecodeJSON accepted a legacy packet that migrates to an empty-objective packet (fail-open migration hole)")
	}
}

// TestExecutionPacketBoundaryRequiresAWriteScope enforces the Boundary handoff
// contract the schema documented ("at least one should be present") but did not
// enforce: a density boundary with neither write_scope nor write_scope_first_slice
// leaves /crank with no write boundary. anyOf now requires one. Reproduces the
// cross-family pawl's Boundary defect on age-55qz.2.
func TestExecutionPacketBoundaryRequiresAWriteScope(t *testing.T) {
	schema := compileExecutionPacketSchemaForTest(t)
	p := validExecutionPacketFixture()
	p["density"] = map[string]any{
		"intent":      "x",
		"decision":    "y",
		"constraint":  []any{"c"},
		"evidence":    []any{"e"},
		"next_action": "n",
		"boundary": map[string]any{
			"bounded_context": "bc",
			"non_goals":       []any{"ng"},
		},
	}
	if err := validateExecutionPacketFixture(schema, p); err == nil {
		t.Fatal("schema accepted a boundary with no write_scope or write_scope_first_slice; /crank would have no write boundary")
	}
	// An EMPTY write-scope array is not a write boundary either (and omitempty
	// would drop it on re-marshal, destabilizing Load->Save).
	p["density"].(map[string]any)["boundary"].(map[string]any)["write_scope_first_slice"] = []any{}
	if err := validateExecutionPacketFixture(schema, p); err == nil {
		t.Fatal("schema accepted an empty write_scope_first_slice as a write boundary")
	}
	// With a write boundary present it must validate.
	p["density"].(map[string]any)["boundary"].(map[string]any)["write_scope_first_slice"] = []any{"cli/"}
	if err := validateExecutionPacketFixture(schema, p); err != nil {
		t.Fatalf("schema rejected a boundary that declares write_scope_first_slice: %v", err)
	}
}

// TestExecutionPacketLegacyMigrationDerivesObjective pins the INTENDED lenient
// legacy-read behavior (not a bug): a minimal legacy slim packet migrates with
// objective derived from epic_id->plan_path, while a legacy packet from which no
// objective can be derived fails closed against the rich schema's required+
// non-empty objective. Documents the migration contract the cross-family pawl
// probed in round 3.
func TestExecutionPacketLegacyMigrationDerivesObjective(t *testing.T) {
	// Derivable objective (from plan_path): migrates.
	if _, err := packet.DecodeJSON([]byte(`{"plan_path":"docs/plan.md","test_levels":["L1"]}`)); err != nil {
		t.Fatalf("legacy packet with a derivable objective (plan_path) must migrate: %v", err)
	}
	// No derivable objective: fails closed.
	if _, err := packet.DecodeJSON([]byte(`{"test_levels":["L1"],"complexity":"standard"}`)); err == nil {
		t.Fatal("legacy packet with no derivable objective must fail closed (empty objective)")
	}
}

// TestExecutionPacketSchemaAcceptsNullMutationEscapeHatch keeps the schema
// authoritative over the REAL repo-execution-profile contract: lanes encode the
// no-escape-hatch state as JSON null (matching the *string field and
// docs/contracts/repo-execution-profile.schema.json), so the execution-packet
// schema must accept null, not reject the repo's own validation-lane contract.
func TestExecutionPacketSchemaAcceptsNullMutationEscapeHatch(t *testing.T) {
	schema := compileExecutionPacketSchemaForTest(t)
	p := validExecutionPacketFixture()
	p["validation_lanes"] = []any{map[string]any{"name": "unit", "command": "go test", "mutation_escape_hatch": nil}}
	if err := validateExecutionPacketFixture(schema, p); err != nil {
		t.Fatalf("schema rejected a lane with null mutation_escape_hatch (the repo profile's no-hatch state): %v", err)
	}
}

// TestDecodeJSONFailsClosedOnLegacyMigrationHole guards the legacy-migration
// path against fail-open: a legacy-looking object that migrates to a packet the
// rich schema would reject (here an out-of-enum complexity) must error, not
// slip through unvalidated.
func TestDecodeJSONFailsClosedOnLegacyMigrationHole(t *testing.T) {
	if _, err := packet.DecodeJSON([]byte(`{"test_levels":["L1"],"complexity":"bogus"}`)); err == nil {
		t.Fatal("DecodeJSON accepted a legacy packet that migrates to a schema-invalid packet (fail-open migration hole)")
	}
}

func compileExecutionPacketSchemaForTest(t *testing.T) *jsonschema.Schema {
	t.Helper()
	path := findRepoFileForTest(t, "schemas", "execution-packet.schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read execution packet schema: %v", err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse execution packet schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(path, doc); err != nil {
		t.Fatalf("add execution packet schema resource: %v", err)
	}
	schema, err := compiler.Compile(path)
	if err != nil {
		t.Fatalf("compile execution packet schema: %v", err)
	}
	return schema
}

func validateExecutionPacketFixture(schema *jsonschema.Schema, packet map[string]any) error {
	return schema.Validate(packet)
}

func unmarshalExecutionPacketForTest(t *testing.T, packet map[string]any) cliRPI.ExecutionPacket {
	t.Helper()
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("marshal execution packet fixture: %v", err)
	}
	var loaded cliRPI.ExecutionPacket
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal execution packet fixture: %v", err)
	}
	return loaded
}

func validExecutionPacketFixture() map[string]any {
	return map[string]any{
		"schema_version":    2,
		"objective":         "harden the execution packet workpacket contract",
		"contract_surfaces": []any{"schemas/execution-packet.schema.json", "cli/internal/rpi/execution_packet.go"},
		"tracker_mode":      "beads",
		"bead_criteria": map[string]any{
			"agentops-dhk.1": []any{
				map[string]any{
					"id":                "ac-agentops-dhk.1",
					"description":       "Execution packets carry typed workpacket routing, default verdict, and red-test spec metadata.",
					"check_type":        "test_pass",
					"check_command":     "go test ./cmd/ao -run TestExecutionPacketSchema -count=1",
					"evidence_required": true,
					"weight":            1,
					"optional":          false,
				},
			},
		},
		"routing": map[string]any{
			"agentops-dhk.1": map[string]any{
				"implementer": "codex",
				"reviewer":    "gemini",
				"rationale":   "Keep implementation and review on distinct rostered model families.",
			},
		},
		"default_verdict": "FAIL",
		"spec": map[string]any{
			"test_path": "cli/cmd/ao/execution_packet_schema_test.go",
			"red_test":  "TestExecutionPacketSchemaValidatesTypedWorkPacketFields",
		},
	}
}
