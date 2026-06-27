package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

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
