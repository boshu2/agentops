package packet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mortemCompatibilityFixture struct {
	SchemaVersion int                     `json:"schema_version"`
	Required      bool                    `json:"required"`
	LegacyVerdict *ExecutionPacketVerdict `json:"pre_mortem_verdict,omitempty"`
	Verdict       *ExecutionPacketVerdict `json:"premortem_verdict,omitempty"`
}

func mortemFixtureDir(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("MORTEM_COMPAT_FIXTURES_DIR"); root != "" {
		return root
	}
	return filepath.Join("..", "..", "..", "..", "tests", "fixtures", "mortem-compatibility")
}

func loadMortemCompatibilityFixture(t *testing.T, name string) mortemCompatibilityFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(mortemFixtureDir(t), name))
	if err != nil {
		t.Fatalf("read mortem fixture %s: %v", name, err)
	}
	var fixture mortemCompatibilityFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("parse mortem fixture %s: %v", name, err)
	}
	if fixture.SchemaVersion == 0 {
		t.Fatalf("mortem fixture %s omitted schema_version", name)
	}
	return fixture
}

func packetJSONFromMortemFixture(t *testing.T, fixture mortemCompatibilityFixture) []byte {
	t.Helper()
	base, err := json.Marshal(validBase())
	if err != nil {
		t.Fatalf("marshal valid packet: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(base, &raw); err != nil {
		t.Fatalf("decode valid packet: %v", err)
	}
	raw["schema_version"] = fixture.SchemaVersion
	delete(raw, "pre_mortem_verdict")
	delete(raw, "premortem_verdict")
	if fixture.LegacyVerdict != nil {
		raw["pre_mortem_verdict"] = *fixture.LegacyVerdict
	}
	if fixture.Verdict != nil {
		raw["premortem_verdict"] = *fixture.Verdict
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal fixture packet: %v", err)
	}
	return data
}

func TestExecutionPacketDecodeJSON_MortemSchemaOwnership(t *testing.T) {
	tests := []struct {
		name      string
		fixture   string
		want      ExecutionPacketVerdict
		wantError []string
	}{
		{name: "v1 accepts legacy", fixture: "v1-old-only.json", want: ExecutionPacketVerdictPass},
		{name: "v2 accepts legacy", fixture: "v2-old-only.json", want: ExecutionPacketVerdictWarn},
		{name: "v3 accepts canonical", fixture: "v3-new-only.json", want: ExecutionPacketVerdictPass},
		{name: "v1 rejects canonical", fixture: "v1-new-only-invalid.json", wantError: []string{"schema_version 1", "premortem_verdict"}},
		{name: "v2 rejects canonical", fixture: "v2-new-only-invalid.json", wantError: []string{"schema_version 2", "premortem_verdict"}},
		{name: "v3 rejects legacy", fixture: "v3-old-only-invalid.json", wantError: []string{"schema_version 3", "pre_mortem_verdict"}},
		{name: "unknown version fails closed", fixture: "unknown-version.json", wantError: []string{"schema_version 4"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := loadMortemCompatibilityFixture(t, tt.fixture)
			p, err := DecodeJSON(packetJSONFromMortemFixture(t, fixture))
			if len(tt.wantError) > 0 {
				if err == nil {
					t.Fatalf("DecodeJSON accepted %s", tt.fixture)
				}
				for _, fragment := range tt.wantError {
					if !strings.Contains(err.Error(), fragment) {
						t.Errorf("DecodeJSON error %q does not contain %q", err, fragment)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodeJSON rejected %s: %v", tt.fixture, err)
			}
			if p.PreMortemVerdict != tt.want {
				t.Fatalf("PreMortemVerdict = %q, want %q", p.PreMortemVerdict, tt.want)
			}
		})
	}
}

func TestExecutionPacketPublishedSchema_MortemOwnership(t *testing.T) {
	schema, err := schemaForExecutionPacket()
	if err != nil {
		t.Fatalf("compile published execution-packet schema: %v", err)
	}
	base, err := json.Marshal(validBase())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		fixture   string
		version   int
		artifacts map[string]any
		wantValid bool
	}{
		{name: "v2 rejects canonical-only verdict", fixture: "v2-new-only-invalid.json"},
		{name: "v3 rejects legacy-only verdict", fixture: "v3-old-only-invalid.json"},
		{name: "v3 accepts equal transition verdicts", fixture: "both-equal.json", wantValid: true},
		{name: "v3 rejects conflicting transition verdicts", fixture: "both-conflicting.json"},
		{name: "v2 rejects canonical-only artifact path", version: 2, artifacts: map[string]any{"premortem_path": "reports/wrong.md"}},
		{name: "v3 rejects legacy-only artifact path", version: 3, artifacts: map[string]any{"pre_mortem_path": "reports/wrong.md"}},
		{name: "v3 rejects equal dual artifact paths", version: 3, artifacts: map[string]any{"pre_mortem_path": "reports/equal.md", "premortem_path": "reports/equal.md"}},
		{name: "v3 rejects conflicting dual artifact paths", version: 3, artifacts: map[string]any{"pre_mortem_path": "reports/legacy.md", "premortem_path": "reports/canonical.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data []byte
			if tt.fixture != "" {
				data = packetJSONFromMortemFixture(t, loadMortemCompatibilityFixture(t, tt.fixture))
			} else {
				var raw map[string]any
				if err := json.Unmarshal(base, &raw); err != nil {
					t.Fatal(err)
				}
				raw["schema_version"] = tt.version
				raw["artifacts"] = tt.artifacts
				data, err = json.Marshal(raw)
				if err != nil {
					t.Fatal(err)
				}
			}
			var instance any
			if err := json.Unmarshal(data, &instance); err != nil {
				t.Fatal(err)
			}
			err := schema.Validate(instance)
			if tt.wantValid && err != nil {
				t.Fatalf("published schema rejected valid transition representation: %v", err)
			}
			if !tt.wantValid && err == nil {
				t.Fatal("published schema accepted a wrong-version mortem field without its version-owned counterpart")
			}
		})
	}
}

func TestExecutionPacketDecodeJSON_MortemArtifactPathOwnershipMatchesSchemaVersion(t *testing.T) {
	base, err := json.Marshal(validBase())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		version   int
		artifacts map[string]any
		want      string
		wantErr   string
	}{
		{name: "v1 legacy path", version: 1, artifacts: map[string]any{"pre_mortem_path": "reports/v1.md"}, want: "reports/v1.md"},
		{name: "v2 legacy path", version: 2, artifacts: map[string]any{"pre_mortem_path": "reports/v2.md"}, want: "reports/v2.md"},
		{name: "v3 canonical path", version: 3, artifacts: map[string]any{"premortem_path": "reports/v3.md"}, want: "reports/v3.md"},
		{name: "v2 rejects canonical path", version: 2, artifacts: map[string]any{"premortem_path": "reports/wrong.md"}, wantErr: "artifacts.premortem_path"},
		{name: "v3 rejects legacy path", version: 3, artifacts: map[string]any{"pre_mortem_path": "reports/wrong.md"}, wantErr: "artifacts.pre_mortem_path"},
		{name: "v3 rejects equal dual paths", version: 3, artifacts: map[string]any{"pre_mortem_path": "reports/equal.md", "premortem_path": "reports/equal.md"}, wantErr: "artifacts.pre_mortem_path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw map[string]any
			if err := json.Unmarshal(base, &raw); err != nil {
				t.Fatal(err)
			}
			raw["schema_version"] = tt.version
			raw["artifacts"] = tt.artifacts
			data, err := json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			p, err := DecodeJSON(data)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("DecodeJSON error = %v, want error naming %s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.Artifacts == nil || p.Artifacts.PreMortemPath != tt.want {
				t.Fatalf("Artifacts = %+v, want path %q", p.Artifacts, tt.want)
			}
		})
	}
}

func TestExecutionPacketDecodeJSON_MortemEqualAndConflictRules(t *testing.T) {
	equal := loadMortemCompatibilityFixture(t, "both-equal.json")
	p, err := DecodeJSON(packetJSONFromMortemFixture(t, equal))
	if err != nil {
		t.Fatalf("DecodeJSON rejected both-equal fixture: %v", err)
	}
	if p.PreMortemVerdict != ExecutionPacketVerdictPass {
		t.Fatalf("both-equal verdict = %q, want PASS", p.PreMortemVerdict)
	}

	conflicting := loadMortemCompatibilityFixture(t, "both-conflicting.json")
	_, err = DecodeJSON(packetJSONFromMortemFixture(t, conflicting))
	if err == nil {
		t.Fatal("DecodeJSON accepted conflicting mortem fields")
	}
	for _, key := range []string{"pre_mortem_verdict", "premortem_verdict"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("conflict error %q does not name %s", err, key)
		}
	}
}

func TestMortemCompatibilityFixtures_NeitherPresentFollowsRequiredFlag(t *testing.T) {
	optional := loadMortemCompatibilityFixture(t, "neither-optional.json")
	optionalData := packetJSONFromMortemFixture(t, optional)
	if _, err := DecodeJSON(optionalData); err != nil {
		t.Fatalf("optional neither-present fixture failed: %v", err)
	}

	required := loadMortemCompatibilityFixture(t, "neither-required.json")
	requiredData := packetJSONFromMortemFixture(t, required)
	_, err := DecodeJSONWithRequirements(requiredData, DecodeRequirements{PreMortemVerdict: true})
	if err == nil {
		t.Fatal("DecodeJSONWithRequirements accepted a packet missing its required mortem verdict")
	}
	for _, key := range []string{"pre_mortem_verdict", "premortem_verdict"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("required mortem error %q does not name %s", err, key)
		}
	}
}

func TestMortemCompatibilityFixtureLoader_ConsumesAndRejectsInvalidContent(t *testing.T) {
	data := []byte(`{"schema_version":"not-an-integer"}`)
	var fixture mortemCompatibilityFixture
	if err := json.Unmarshal(data, &fixture); err == nil {
		t.Fatal("invalid fixture content parsed successfully; fixture bytes are not enforced")
	}
}

func TestExecutionPacketMarshal_ExplicitLegacyV2KeepsLegacyWireNames(t *testing.T) {
	fixtureData, err := os.ReadFile(filepath.Join(mortemFixtureDir(t), "writer-legacy-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var writerFixture struct {
		SchemaVersion int                               `json:"schema_version"`
		PacketFields  map[string]ExecutionPacketVerdict `json:"packet_fields"`
	}
	if err := json.Unmarshal(fixtureData, &writerFixture); err != nil {
		t.Fatalf("parse writer fixture: %v", err)
	}
	p := validBase()
	p.SchemaVersion = writerFixture.SchemaVersion
	p.PreMortemVerdict = writerFixture.PacketFields["pre_mortem_verdict"]
	p.Artifacts = &ExecutionPacketArtifacts{PreMortemPath: "reports/legacy.md"}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal execution packet: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("decode marshaled execution packet: %v", err)
	}
	var version int
	if err := json.Unmarshal(raw["schema_version"], &version); err != nil || version != 2 {
		t.Fatalf("schema_version = %d (err=%v), want explicit legacy v2 representation", version, err)
	}
	if _, ok := raw["pre_mortem_verdict"]; !ok {
		t.Error("explicit v2 representation omitted legacy pre_mortem_verdict")
	}
	if _, ok := raw["premortem_verdict"]; ok {
		t.Error("explicit v2 representation emitted v3 premortem_verdict")
	}
	var artifacts map[string]json.RawMessage
	if err := json.Unmarshal(raw["artifacts"], &artifacts); err != nil {
		t.Fatalf("decode marshaled artifacts: %v", err)
	}
	if _, ok := artifacts["pre_mortem_path"]; !ok {
		t.Error("explicit v2 representation omitted artifacts.pre_mortem_path")
	}
	if _, ok := artifacts["premortem_path"]; ok {
		t.Error("explicit v2 representation emitted artifacts.premortem_path")
	}
}

func TestExecutionPacketMarshal_MortemWriterIsCanonicalV3(t *testing.T) {
	fixtureData, err := os.ReadFile(filepath.Join(mortemFixtureDir(t), "writer-canonical-v3.json"))
	if err != nil {
		t.Fatal(err)
	}
	var writerFixture struct {
		SchemaVersion int                               `json:"schema_version"`
		PacketFields  map[string]ExecutionPacketVerdict `json:"packet_fields"`
		RuntimePaths  []string                          `json:"runtime_paths"`
	}
	if err := json.Unmarshal(fixtureData, &writerFixture); err != nil {
		t.Fatalf("parse canonical writer fixture: %v", err)
	}
	p := validBase()
	p.SchemaVersion = CurrentExecutionPacketSchemaVersion
	p.PreMortemVerdict = writerFixture.PacketFields["premortem_verdict"]
	p.Artifacts = &ExecutionPacketArtifacts{PreMortemPath: writerFixture.RuntimePaths[0]}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal execution packet: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := json.Unmarshal(raw["schema_version"], &version); err != nil || version != 3 {
		t.Fatalf("schema_version = %d (err=%v), want canonical v3", version, err)
	}
	if _, ok := raw["premortem_verdict"]; !ok {
		t.Error("canonical writer omitted premortem_verdict")
	}
	if _, ok := raw["pre_mortem_verdict"]; ok {
		t.Error("canonical writer emitted legacy pre_mortem_verdict")
	}
	var artifacts map[string]json.RawMessage
	if err := json.Unmarshal(raw["artifacts"], &artifacts); err != nil {
		t.Fatal(err)
	}
	if _, ok := artifacts["premortem_path"]; !ok {
		t.Error("canonical writer omitted artifacts.premortem_path")
	}
	if _, ok := artifacts["pre_mortem_path"]; ok {
		t.Error("canonical writer emitted legacy artifacts.pre_mortem_path")
	}
	if err := ValidateJSON(data); err != nil {
		t.Fatalf("canonical writer emitted invalid v3 packet: %v", err)
	}
}
