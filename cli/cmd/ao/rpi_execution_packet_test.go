// practices: [agile-manifesto, dora-metrics]
package main

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/boshu2/agentops/cli/internal/rpi"
)

func TestExecutionPacket_RoundTripWithCriteria(t *testing.T) {
	original := executionPacket{
		SchemaVersion:    1,
		Objective:        "round trip criteria",
		RunID:            "run-1",
		EpicID:           "soc-bcrn",
		ContractSurfaces: []string{"docs/contracts/repo-execution-profile.md"},
		TrackerMode:      "bd",
		EpicCriteria: []rpi.Criterion{
			{
				ID:               "ac-soc-bcrn.1",
				Description:      "executionPacket struct extends with criteria fields",
				CheckType:        "test_pass",
				CheckCommand:     "go test ./cmd/ao/...",
				EvidencePath:     ".agents/rpi/test-output.txt",
				EvidenceRequired: true,
				Weight:           1.0,
				Optional:         false,
			},
		},
		BeadCriteria: map[string][]rpi.Criterion{
			"soc-bcrn.1.1": {
				{
					ID:               "ac-soc-bcrn.1.1.1",
					Description:      "JSON schema for execution packet exists and parses",
					CheckType:        "file_exists",
					EvidencePath:     "schemas/execution-packet.schema.json",
					EvidenceRequired: true,
					Weight:           0.5,
					Optional:         false,
				},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded executionPacket
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("round-trip mismatch:\noriginal=%#v\ndecoded =%#v", original, decoded)
	}
}

func TestExecutionPacket_BackCompatV1NoCriteria(t *testing.T) {
	v1JSON := `{
		"schema_version": 1,
		"objective": "v1 packet",
		"run_id": "run-v1",
		"epic_id": "ag-100",
		"contract_surfaces": ["docs/contracts/repo-execution-profile.md"],
		"tracker_mode": "bd",
		"done_criteria": ["all tests pass", "coverage above threshold"]
	}`

	var packet executionPacket
	if err := json.Unmarshal([]byte(v1JSON), &packet); err != nil {
		t.Fatalf("v1 unmarshal: %v", err)
	}

	if packet.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d, want 1", packet.SchemaVersion)
	}
	if packet.Objective != "v1 packet" {
		t.Errorf("Objective = %q, want %q", packet.Objective, "v1 packet")
	}
	wantDone := []string{"all tests pass", "coverage above threshold"}
	if !reflect.DeepEqual(packet.DoneCriteria, wantDone) {
		t.Errorf("DoneCriteria = %v, want %v", packet.DoneCriteria, wantDone)
	}
	if packet.EpicCriteria != nil {
		t.Errorf("EpicCriteria = %v, want nil for v1 packet", packet.EpicCriteria)
	}
	if packet.BeadCriteria != nil {
		t.Errorf("BeadCriteria = %v, want nil for v1 packet", packet.BeadCriteria)
	}
}

func TestExecutionPacket_CustomRubricCriterion(t *testing.T) {
	original := rpi.Criterion{
		ID:               "ac-soc-bcrn.2.1",
		Description:      "council:vibe judges packet readiness",
		CheckType:        "custom_rubric",
		EvidenceRequired: false,
		Weight:           1.0,
		Optional:         false,
		AgentJudge:       "council:vibe",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded rpi.Criterion
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("custom_rubric round-trip mismatch:\noriginal=%#v\ndecoded =%#v", original, decoded)
	}
	if decoded.AgentJudge != "council:vibe" {
		t.Errorf("AgentJudge = %q, want %q", decoded.AgentJudge, "council:vibe")
	}
}

func TestExecutionPacket_OmitEmptyCriteria(t *testing.T) {
	packet := executionPacket{
		SchemaVersion:    1,
		Objective:        "omit empty",
		ContractSurfaces: []string{},
		TrackerMode:      "bd",
	}

	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if bytes.Contains(data, []byte(`"epic_criteria"`)) {
		t.Errorf("epic_criteria key should be omitted when empty; got: %s", data)
	}
	if bytes.Contains(data, []byte(`"bead_criteria"`)) {
		t.Errorf("bead_criteria key should be omitted when empty; got: %s", data)
	}
}
