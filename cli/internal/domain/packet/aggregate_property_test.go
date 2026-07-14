package packet

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validBase() ExecutionPacket {
	return ExecutionPacket{
		SchemaVersion:    2,
		Objective:        "harden the execution packet workpacket contract",
		ContractSurfaces: []string{"schemas/execution-packet.schema.json", "cli/internal/domain/packet/invariants.go"},
		TrackerMode:      "beads",
		Complexity:       ComplexityStandard,
		TestLevels: &ExecutionPacketTestLevels{
			Required:    []TestLevel{L1, L2},
			Recommended: []TestLevel{L3},
			Rationale:   "standard autonomous proof floor",
		},
		Routing: map[string]ExecutionPacketRouting{
			"agentops-dhk.2": {
				Implementer: ExecutionPacketModelCodex,
				Reviewer:    ExecutionPacketModelGemini,
				Rationale:   "separate implementation and review families",
			},
		},
		Spec: &ExecutionPacketSpec{
			TestPath: "cli/internal/domain/packet/aggregate_property_test.go",
			RedTest:  "TestExecutionPacket_ValidateRejectsRichSchemaViolation",
		},
	}
}

func TestExecutionPacket_ValidateAcceptsRichSchemaPacket(t *testing.T) {
	p := validBase()
	p.PlanPath = ""

	if err := p.Validate(); err != nil {
		t.Fatalf("Validate() rejected valid rich packet: %v", err)
	}
}

func TestExecutionPacket_EmbeddedSchemaMatchesRootSchema(t *testing.T) {
	rootSchemaPath := filepath.Join("..", "..", "..", "..", "schemas", "execution-packet.schema.json")
	rootSchema, err := os.ReadFile(rootSchemaPath)
	if err != nil {
		t.Fatalf("read root execution packet schema: %v", err)
	}
	if !bytes.Equal(executionPacketSchemaBytes, rootSchema) {
		t.Fatalf("embedded execution packet schema differs from %s", rootSchemaPath)
	}
}

func TestExecutionPacket_ValidateRejectsRichSchemaViolation(t *testing.T) {
	p := validBase()
	p.Routing["agentops-dhk.2"] = ExecutionPacketRouting{
		Implementer: ExecutionPacketModelFamily("llama"),
		Reviewer:    ExecutionPacketModelGemini,
		Rationale:   "not on the schema roster",
	}

	err := p.Validate()
	if !errors.Is(err, ErrSchemaViolation) {
		t.Fatalf("Validate() error = %v, want errors.Is ErrSchemaViolation", err)
	}
	if !strings.Contains(err.Error(), "implementer") && !strings.Contains(err.Error(), "llama") {
		t.Fatalf("Validate() error = %q, want implementer/llama enum detail", err.Error())
	}
}

func TestExecutionPacket_ValidateJSONRejectsAdditionalProperties(t *testing.T) {
	data := []byte(`{
		"schema_version": 2,
		"objective": "reject unknown rich packet fields",
		"contract_surfaces": ["schemas/execution-packet.schema.json"],
		"tracker_mode": "beads",
		"unexpected": true
	}`)

	err := ValidateJSON(data)
	if !errors.Is(err, ErrSchemaViolation) {
		t.Fatalf("ValidateJSON() error = %v, want errors.Is ErrSchemaViolation", err)
	}
}
