package rpi

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestExecutionPacketLoopDensityTypesRoundTrip(t *testing.T) {
	original := struct {
		Density          ExecutionPacketDensity    `json:"density"`
		Artifacts        ExecutionPacketArtifacts  `json:"artifacts"`
		TestLevels       ExecutionPacketTestLevels `json:"test_levels"`
		RankedPacketPath string                    `json:"ranked_packet_path"`
	}{
		Density: ExecutionPacketDensity{
			Intent: "ship a dense handoff",
			Boundary: ExecutionPacketBoundary{
				BoundedContext: "agentops",
				NonGoals:       []string{"doctor workspace"},
				WriteScope:     []string{"schemas/execution-packet.schema.json"},
			},
			Evidence:   []string{"go test ./cmd/ao ./internal/rpi -run ExecutionPacket"},
			Decision:   "align schema and runtime packet fields",
			Constraint: []string{"keep raw artifacts out of the packet"},
			NextAction: "/crank .agents/rpi/execution-packet.json",
		},
		Artifacts: ExecutionPacketArtifacts{
			ResearchPath:     ".agents/research/topic.md",
			PlanPath:         ".agents/plans/topic.md",
			PreMortemPath:    ".agents/council/pre-mortem-topic.md",
			RankedPacketPath: ".agents/rpi/ranked-packet.json",
		},
		TestLevels: ExecutionPacketTestLevels{
			Required:    []string{"L0", "L1"},
			Recommended: []string{"L2"},
			Rationale:   "standard autonomous proof floor",
		},
		RankedPacketPath: ".agents/rpi/ranked-packet.json",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded struct {
		Density          ExecutionPacketDensity    `json:"density"`
		Artifacts        ExecutionPacketArtifacts  `json:"artifacts"`
		TestLevels       ExecutionPacketTestLevels `json:"test_levels"`
		RankedPacketPath string                    `json:"ranked_packet_path"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("round-trip mismatch:\noriginal=%#v\ndecoded =%#v", original, decoded)
	}
}

func TestExecutionPacketTypedWorkPacketFieldsRoundTrip(t *testing.T) {
	if DefaultExecutionPacketVerdict != ExecutionPacketVerdictFail {
		t.Fatalf("default verdict = %q, want %q", DefaultExecutionPacketVerdict, ExecutionPacketVerdictFail)
	}

	original := struct {
		Routing        map[string]ExecutionPacketRouting `json:"routing"`
		DefaultVerdict ExecutionPacketVerdict            `json:"default_verdict,omitempty"`
		Spec           ExecutionPacketSpec               `json:"spec"`
	}{
		Routing: map[string]ExecutionPacketRouting{
			"agentops-dhk.1": {
				Implementer: ExecutionPacketModelCodex,
				Reviewer:    ExecutionPacketModelGemini,
				Rationale:   "separate implementation and review families",
			},
		},
		DefaultVerdict: DefaultExecutionPacketVerdict,
		Spec: ExecutionPacketSpec{
			TestPath: "cli/cmd/ao/execution_packet_schema_test.go",
			RedTest:  "TestExecutionPacketSchemaValidatesTypedWorkPacketFields",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded struct {
		Routing        map[string]ExecutionPacketRouting `json:"routing"`
		DefaultVerdict ExecutionPacketVerdict            `json:"default_verdict,omitempty"`
		Spec           ExecutionPacketSpec               `json:"spec"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("round-trip mismatch:\noriginal=%#v\ndecoded =%#v", original, decoded)
	}
}
