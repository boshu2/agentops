package packet

import (
	"encoding/json"
	"strings"
	"testing"
)

func packetJSONWithPremortem(t *testing.T, version int, verdict ExecutionPacketVerdict) []byte {
	t.Helper()
	p := validBase()
	p.SchemaVersion = version
	p.PremortemVerdict = verdict
	p.Artifacts = &ExecutionPacketArtifacts{PremortemPath: "reports/premortem.json"}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal execution packet: %v", err)
	}
	return data
}

func TestExecutionPacketPremortemContract_AcceptsOneCanonicalBinaryVerdict(t *testing.T) {
	for _, version := range []int{1, 2, 3} {
		for _, verdict := range []ExecutionPacketVerdict{ExecutionPacketVerdictPass, ExecutionPacketVerdictFail} {
			t.Run(strings.Join([]string{string(rune('0' + version)), string(verdict)}, "-"), func(t *testing.T) {
				data := packetJSONWithPremortem(t, version, verdict)
				if err := ValidateJSON(data); err != nil {
					t.Fatalf("ValidateJSON rejected canonical packet: %v", err)
				}
				decoded, err := DecodeJSON(data)
				if err != nil {
					t.Fatalf("DecodeJSON rejected canonical packet: %v", err)
				}
				if decoded.PremortemVerdict != verdict {
					t.Fatalf("PremortemVerdict = %q, want %q", decoded.PremortemVerdict, verdict)
				}
				if decoded.Artifacts == nil || decoded.Artifacts.PremortemPath != "reports/premortem.json" {
					t.Fatalf("Artifacts = %+v, want canonical premortem path", decoded.Artifacts)
				}
			})
		}
	}
}

func TestExecutionPacketPremortemContract_RejectsAlternateReadinessFields(t *testing.T) {
	base := packetJSONWithPremortem(t, CurrentExecutionPacketSchemaVersion, ExecutionPacketVerdictPass)
	var packet map[string]any
	if err := json.Unmarshal(base, &packet); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "legacy verdict", mutate: func(p map[string]any) { p["pre_mortem_verdict"] = "PASS" }},
		{name: "WARN readiness", mutate: func(p map[string]any) { p["premortem_verdict"] = "WARN" }},
		{name: "legacy path", mutate: func(p map[string]any) { p["artifacts"].(map[string]any)["pre_mortem_path"] = "reports/legacy.md" }},
		{name: "perspective plans", mutate: func(p map[string]any) { p["artifacts"].(map[string]any)["perspective_plan_paths"] = []string{"one.md"} }},
		{name: "synthesis", mutate: func(p map[string]any) { p["artifacts"].(map[string]any)["synthesis_packet_path"] = "synthesis.yaml" }},
		{name: "Fable", mutate: func(p map[string]any) { p["artifacts"].(map[string]any)["fable_approval_path"] = "fable.yaml" }},
		{name: "ApprovalEdge", mutate: func(p map[string]any) { p["artifacts"].(map[string]any)["approval_edge_path"] = "approval.yaml" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var candidate map[string]any
			bytes, err := json.Marshal(packet)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(bytes, &candidate); err != nil {
				t.Fatal(err)
			}
			tt.mutate(candidate)
			bytes, err = json.Marshal(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateJSON(bytes); err == nil {
				t.Fatal("ValidateJSON accepted alternate readiness state")
			}
		})
	}
}

func TestExecutionPacketPremortemContract_RequiredMeansCanonicalField(t *testing.T) {
	p := validBase()
	p.SchemaVersion = CurrentExecutionPacketSchemaVersion
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	_, err = DecodeJSONWithRequirements(data, DecodeRequirements{PremortemVerdict: true})
	if err == nil || !strings.Contains(err.Error(), "premortem_verdict") {
		t.Fatalf("missing canonical verdict error = %v", err)
	}
}
