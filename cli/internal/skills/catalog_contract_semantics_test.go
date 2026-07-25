package skills

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCatalogV4RejectsFrozenSemanticHostiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		want   string
		mutate func(entry, contract map[string]any)
	}{
		{
			name: "forbidden verdict authority",
			want: "only validate may write_verdict",
			mutate: func(_ map[string]any, contract map[string]any) {
				contract["authority"] = append(stringSlice(contract["authority"]), "write_verdict")
			},
		},
		{
			name: "mutation without authority",
			want: "mutating effect requires mutate_subject",
			mutate: func(_ map[string]any, contract map[string]any) {
				contract["authority"] = []any{"read_evidence"}
			},
		},
		{
			name: "missing required receipt",
			want: "requires receipt=required",
			mutate: func(_ map[string]any, contract map[string]any) {
				contractEffects(contract)[1]["receipt"] = "optional"
			},
		},
		{
			name: "unvalidated binding output",
			want: "binding output",
			mutate: func(_ map[string]any, contract map[string]any) {
				contractArtifacts(contract, "produces")[0]["validator"] = nil
			},
		},
		{
			name: "normalized trigger collision",
			want: "normalized trigger text",
			mutate: func(_ map[string]any, contract map[string]any) {
				contractTriggerCases(contract, "negative")[0]["prompt"] =
					"  CREATE\tA metadata-complete skill source package.  "
			},
		},
		{
			name: "alias to another skill",
			want: "canonical_skill must equal owning skill",
			mutate: func(_ map[string]any, contract map[string]any) {
				contractTriggerCases(contract, "aliases")[0]["canonical_skill"] = "workflow-builder"
			},
		},
		{
			name: "self neighbor",
			want: "nearest neighbor must differ from owning skill",
			mutate: func(entry, contract map[string]any) {
				contractTriggerCases(contract, "nearest_neighbors")[0]["skill"] = entry["name"]
			},
		},
		{
			name: "non-RPI hard dependency",
			want: "may not declare hard skill dependencies",
			mutate: func(entry, _ map[string]any) {
				entry["dependencies"] = []any{"plan"}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			body := mutateV4Fixture(t, test.mutate)
			err := loadCatalogBody(t, body)
			if err == nil {
				t.Fatal("LoadCatalog succeeded; want semantic rejection")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error %q does not contain %q", err, test.want)
			}
		})
	}
}

func TestLoadCatalogV4RejectsRemainingCompilerSemanticMismatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		want   string
		mutate func(entry, contract map[string]any)
	}{
		{
			name: "refine intent outside plan",
			want: "only plan may refine_intent",
			mutate: func(_ map[string]any, contract map[string]any) {
				contract["authority"] = append(stringSlice(contract["authority"]), "refine_intent")
			},
		},
		{
			name: "dispatch outside rpi",
			want: "only rpi may dispatch_phase",
			mutate: func(_ map[string]any, contract map[string]any) {
				contract["authority"] = append(stringSlice(contract["authority"]), "dispatch_phase")
			},
		},
		{
			name: "transport outside runtime",
			want: "transport requires runtime primary_layer",
			mutate: func(_ map[string]any, contract map[string]any) {
				contract["authority"] = append(stringSlice(contract["authority"]), "transport")
			},
		},
		{
			name: "rpi without dispatch",
			want: "rpi must declare dispatch_phase",
			mutate: func(entry, contract map[string]any) {
				entry["name"] = "rpi"
				entry["dependencies"] = []any{"plan", "implement", "validate"}
				contractTriggerCases(contract, "aliases")[0]["canonical_skill"] = "rpi"
			},
		},
		{
			name: "process cleanup required",
			want: "requires cleanup=required",
			mutate: func(_ map[string]any, contract map[string]any) {
				contractEffects(contract)[3]["cleanup"] = "best_effort"
			},
		},
		{
			name: "mutate authority needs authorized effect",
			want: "mutate_subject requires",
			mutate: func(_ map[string]any, contract map[string]any) {
				for _, effect := range contractEffects(contract) {
					if effect["kind"] == "filesystem.write" {
						effect["authorization"] = "validate"
					}
				}
			},
		},
		{
			name: "binding output needs schema",
			want: "binding output",
			mutate: func(_ map[string]any, contract map[string]any) {
				contractArtifacts(contract, "produces")[0]["schema_ref"] = nil
			},
		},
		{
			name: "normalized alias prompt collision",
			want: "normalized trigger text",
			mutate: func(_ map[string]any, contract map[string]any) {
				contractTriggerCases(contract, "aliases")[0]["alias"] =
					" create  a metadata-complete SKILL source package. "
			},
		},
		{
			name: "normalized alias collision",
			want: "normalized trigger text",
			mutate: func(_ map[string]any, contract map[string]any) {
				aliases := contract["triggers"].(map[string]any)["aliases"].([]any)
				contract["triggers"].(map[string]any)["aliases"] = append(aliases, map[string]any{
					"id":              "skill-factory-alias-two",
					"alias":           "  SKILL\tFACTORY ",
					"canonical_skill": "skill-builder",
				})
			},
		},
		{
			name: "unicode case-fold collision",
			want: "normalized trigger text",
			mutate: func(_ map[string]any, contract map[string]any) {
				contractTriggerCases(contract, "positive")[0]["prompt"] = "Straße"
				contractTriggerCases(contract, "aliases")[0]["alias"] = "STRASSE"
			},
		},
		{
			name: "rpi incomplete dependencies",
			want: "rpi hard dependencies must be exactly",
			mutate: func(entry, contract map[string]any) {
				entry["name"] = "rpi"
				entry["dependencies"] = []any{"plan", "implement"}
				contract["authority"] = append(stringSlice(contract["authority"]), "dispatch_phase")
				contractTriggerCases(contract, "aliases")[0]["canonical_skill"] = "rpi"
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			body := mutateV4Fixture(t, test.mutate)
			err := loadCatalogBody(t, body)
			if err == nil {
				t.Fatal("LoadCatalog succeeded; want semantic rejection")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error %q does not contain %q", err, test.want)
			}
		})
	}
}

func TestLoadCatalogV4EnforcesExactEffectObligationSets(t *testing.T) {
	t.Parallel()

	mutatingKinds := []string{
		"filesystem.write",
		"network.write",
		"environment.write",
		"credential.switch",
		"external.mutate",
		"runtime.session",
		"host.configure",
	}
	receiptKinds := append(append([]string(nil), mutatingKinds...), "process.start")
	cleanupKinds := []string{"process.start", "credential.switch", "runtime.session"}

	for _, kind := range receiptKinds {
		t.Run("receipt/"+kind, func(t *testing.T) {
			t.Parallel()
			body := mutateV4Fixture(t, func(_ map[string]any, contract map[string]any) {
				contract["effects"] = []any{semanticEffect(kind, "required", "optional")}
			})
			assertCatalogRejectionContains(t, body, "requires receipt=required")
		})
	}
	for _, kind := range cleanupKinds {
		t.Run("cleanup/"+kind, func(t *testing.T) {
			t.Parallel()
			body := mutateV4Fixture(t, func(_ map[string]any, contract map[string]any) {
				contract["effects"] = []any{semanticEffect(kind, "best_effort", "required")}
			})
			assertCatalogRejectionContains(t, body, "requires cleanup=required")
		})
	}
	for _, kind := range mutatingKinds {
		t.Run("mutate-authority/"+kind, func(t *testing.T) {
			t.Parallel()
			body := mutateV4Fixture(t, func(_ map[string]any, contract map[string]any) {
				contract["authority"] = []any{"read_evidence"}
				contract["effects"] = []any{semanticEffect(
					kind,
					requiredCleanup(kind),
					"required",
				)}
			})
			assertCatalogRejectionContains(t, body, "requires mutate_subject authority")
		})
	}
}

func TestLoadCatalogV4AcceptsExactRPIHardDependencySet(t *testing.T) {
	t.Parallel()

	body := mutateV4Fixture(t, func(entry, contract map[string]any) {
		entry["name"] = "rpi"
		entry["dependencies"] = []any{"validate", "plan", "implement"}
		contract["authority"] = append(stringSlice(contract["authority"]), "dispatch_phase")
		contractTriggerCases(contract, "aliases")[0]["canonical_skill"] = "rpi"
	})
	if err := loadCatalogBody(t, body); err != nil {
		t.Fatalf("LoadCatalog rejected exact unordered rpi dependency set: %v", err)
	}
}

func TestLoadCatalogV4AcceptsOwnerScopedAuthorities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		skillName    string
		primaryLayer string
		authority    string
	}{
		{name: "plan refines intent", skillName: "plan", authority: "refine_intent"},
		{name: "validate writes verdict", skillName: "validate", authority: "write_verdict"},
		{
			name:         "runtime transports",
			skillName:    "runtime-adapter",
			primaryLayer: "runtime",
			authority:    "transport",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			body := mutateV4Fixture(t, func(entry, contract map[string]any) {
				entry["name"] = test.skillName
				contractTriggerCases(contract, "aliases")[0]["canonical_skill"] = test.skillName
				contract["authority"] = append(stringSlice(contract["authority"]), test.authority)
				if test.primaryLayer != "" {
					contract["primary_layer"] = test.primaryLayer
				}
			})
			if err := loadCatalogBody(t, body); err != nil {
				t.Fatalf("LoadCatalog rejected valid scoped authority: %v", err)
			}
		})
	}
}

func TestLoadCatalogV4DoesNotInventRepositoryFilesystemSemantics(t *testing.T) {
	t.Parallel()

	body := mutateV4Fixture(t, func(_ map[string]any, contract map[string]any) {
		binding := contractArtifacts(contract, "produces")[0]
		binding["schema_ref"] = "future/schema.json"
		binding["validator"] = "future/validator"
		proof := contract["proof"].(map[string]any)
		proof["command"] = "future/proof-runner"
		proof["harness_refs"] = []any{"future/proof-runner"}
		proof["fixture_refs"] = []any{"future/fixture.json"}
		contractTriggerCases(contract, "nearest_neighbors")[0]["skill"] = "future-skill"
	})
	if err := loadCatalogBody(t, body); err != nil {
		t.Fatalf("standalone reader performed repository existence validation: %v", err)
	}
}

func TestLoadCatalogV4RejectsInvalidHarnessRefs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(proof map[string]any)
	}{
		{
			name: "missing",
			mutate: func(proof map[string]any) {
				delete(proof, "harness_refs")
			},
		},
		{
			name: "empty set",
			mutate: func(proof map[string]any) {
				proof["harness_refs"] = []any{}
			},
		},
		{
			name: "empty ref",
			mutate: func(proof map[string]any) {
				proof["harness_refs"] = []any{""}
			},
		},
		{
			name: "duplicate ref",
			mutate: func(proof map[string]any) {
				proof["harness_refs"] = []any{
					"skills/skill-builder/scripts/test-contract-v3.sh",
					"skills/skill-builder/scripts/test-contract-v3.sh",
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			body := mutateV4Fixture(t, func(_ map[string]any, contract map[string]any) {
				test.mutate(contract["proof"].(map[string]any))
			})
			err := loadCatalogBody(t, body)
			if err == nil {
				t.Fatal("LoadCatalog succeeded; want harness_refs rejection")
			}
			if !strings.Contains(err.Error(), "harness_refs") {
				t.Fatalf("error %q does not identify harness_refs", err)
			}
		})
	}
}

func semanticEffect(kind, cleanup, receipt string) map[string]any {
	return map[string]any{
		"id":            strings.ReplaceAll(kind, ".", "-"),
		"kind":          kind,
		"scope":         "one explicit semantic test scope",
		"authorization": "caller",
		"cleanup":       cleanup,
		"receipt":       receipt,
	}
}

func requiredCleanup(kind string) string {
	switch kind {
	case "credential.switch", "runtime.session":
		return "required"
	default:
		return "none"
	}
}

func assertCatalogRejectionContains(t *testing.T, body, want string) {
	t.Helper()
	err := loadCatalogBody(t, body)
	if err == nil {
		t.Fatal("LoadCatalog succeeded; want semantic rejection")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err, want)
	}
}

func mutateV4Fixture(
	t *testing.T,
	mutate func(entry, contract map[string]any),
) string {
	t.Helper()
	document := decodeV4FixtureDocument(t)
	entry := firstV4FixtureEntry(t, document)
	contract := objectValue(t, entry, "contract_v3")
	mutate(entry, contract)
	body, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func decodeV4FixtureDocument(t *testing.T) map[string]any {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(mustReadFixture(t, "catalog-v4.json")), &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func firstV4FixtureEntry(t *testing.T, document map[string]any) map[string]any {
	t.Helper()
	skills, ok := document["skills"].([]any)
	if !ok || len(skills) != 1 {
		t.Fatalf("v4 fixture skills are invalid: %#v", document["skills"])
	}
	entry, ok := skills[0].(map[string]any)
	if !ok {
		t.Fatalf("v4 fixture entry is invalid: %#v", skills[0])
	}
	return entry
}

func contractEffects(contract map[string]any) []map[string]any {
	return objectSlice(contract["effects"])
}

func contractArtifacts(contract map[string]any, family string) []map[string]any {
	return objectSlice(contract["artifacts"].(map[string]any)[family])
}

func contractTriggerCases(contract map[string]any, family string) []map[string]any {
	return objectSlice(contract["triggers"].(map[string]any)[family])
}

func objectSlice(value any) []map[string]any {
	values := value.([]any)
	objects := make([]map[string]any, 0, len(values))
	for _, value := range values {
		objects = append(objects, value.(map[string]any))
	}
	return objects
}

func objectValue(t *testing.T, object map[string]any, name string) map[string]any {
	t.Helper()
	value, ok := object[name].(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object: %#v", name, object[name])
	}
	return value
}

func stringSlice(value any) []any {
	values := value.([]any)
	return append([]any(nil), values...)
}

func repoRootFromSkillsDir(t *testing.T) string {
	t.Helper()
	skillsDir := repoSkillsDir(t)
	if skillsDir == "" {
		t.Fatal("repository skills directory not found")
	}
	return filepath.Dir(skillsDir)
}
