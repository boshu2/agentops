package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadCatalogVersionedFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version string
		name    string
	}{
		{version: "1", name: "legacy-v1"},
		{version: "2", name: "legacy-v2"},
		{version: "3", name: "legacy-v3"},
		{version: "4", name: "shadow-v4"},
	}
	for _, tt := range tests {
		t.Run("v"+tt.version, func(t *testing.T) {
			t.Parallel()
			cat := loadCatalogFixture(t, "catalog-v"+tt.version+".json")
			if cat.SchemaVersion != tt.version {
				t.Fatalf("schema_version = %q, want %q", cat.SchemaVersion, tt.version)
			}
			if cat.SkillCount != 1 || len(cat.Skills) != 1 || cat.Skills[0].Name != tt.name {
				t.Fatalf("unexpected catalog: %#v", cat)
			}
		})
	}
}

func TestLoadCatalogV3PreservesLiveFields(t *testing.T) {
	t.Parallel()

	entry := loadCatalogFixture(t, "catalog-v3.json").Skills[0]
	if len(entry.Capabilities) != 1 || entry.Capabilities[0] != "judge" {
		t.Fatalf("capabilities were not preserved: %#v", entry.Capabilities)
	}
	if len(entry.Effects) != 1 || entry.Effects[0] != "read_workspace" {
		t.Fatalf("effects were not preserved: %#v", entry.Effects)
	}
	if entry.CanonicalStatus != "canonical" || entry.Disposition != "keep" || entry.Tier != "execution" {
		t.Fatalf("v3 classification fields were not preserved: %#v", entry)
	}
}

func TestLoadCatalogV4PreservesTypedContract(t *testing.T) {
	t.Parallel()

	contract := loadCatalogFixture(t, "catalog-v4.json").Skills[0].ContractV3
	if contract == nil {
		t.Fatal("contract_v3 was dropped")
	}
	if contract.SchemaVersion != "skill-contract.v3" || contract.PrimaryLayer != "support" || len(contract.LifecycleSeams) != 2 {
		t.Fatalf("layer/seams were not preserved: %#v", contract)
	}
	if len(contract.Effects) != 1 || contract.Effects[0].ID != "publish-owned-projection" ||
		contract.Effects[0].Receipt != "required" {
		t.Fatalf("structured effects were not preserved: %#v", contract.Effects)
	}
	if len(contract.Artifacts.Consumes) != 1 || contract.Artifacts.Consumes[0].SchemaRef == nil ||
		*contract.Artifacts.Consumes[0].SchemaRef != "skill-contract.v3" ||
		contract.Artifacts.Consumes[0].Validator != nil {
		t.Fatalf("typed artifacts were not preserved: %#v", contract.Artifacts)
	}
	if len(contract.Triggers.Negative) != 1 || contract.Triggers.Negative[0].Expected != "do_not_route" ||
		len(contract.Triggers.NearestNeighbors) != 1 {
		t.Fatalf("trigger families were not preserved: %#v", contract.Triggers)
	}
	if contract.Failure.PartialMutation.Action != "rollback_then_stop" {
		t.Fatalf("failure semantics were not preserved: %#v", contract.Failure)
	}
	if contract.Proof.Command == "" || len(contract.Proof.FixtureRefs) != 1 {
		t.Fatalf("proof declaration was not preserved: %#v", contract.Proof)
	}
	assertV4ContractRoundTrip(t, contract)
}

func TestLoadCatalogAcceptsLiveV3Catalog(t *testing.T) {
	t.Parallel()

	skillsDir := repoSkillsDir(t)
	if skillsDir == "" {
		t.Skip("skills/ not found relative to test working dir")
	}
	cat, err := LoadCatalog(skillsDir)
	if err != nil {
		t.Fatalf("LoadCatalog(live v3): %v", err)
	}
	if cat.SchemaVersion != "3" || cat.SkillCount == 0 || cat.SkillCount != len(cat.Skills) {
		t.Fatalf("unexpected live catalog envelope: version=%q count=%d len=%d", cat.SchemaVersion, cat.SkillCount, len(cat.Skills))
	}
}

func TestLoadCatalogRejectsHostileEnvelopes(t *testing.T) {
	t.Parallel()

	valid := mustReadFixture(t, "catalog-v3.json")
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "duplicate top-level key",
			body: `{"schema_version":"3","schema_version":"3","skill_count":0,"skills":[]}`,
			want: "duplicate object key",
		},
		{
			name: "duplicate nested key",
			body: `{"schema_version":"3","skill_count":1,"skills":[{"name":"x","name":"x"}]}`,
			want: "duplicate object key",
		},
		{
			name: "unknown top-level field",
			body: strings.Replace(valid, `"skill_count": 1,`, `"skill_count": 1, "surprise": true,`, 1),
			want: "unknown field",
		},
		{
			name: "unknown entry field",
			body: strings.Replace(valid, `"name": "legacy-v3",`, `"name": "legacy-v3", "surprise": true,`, 1),
			want: "unknown field",
		},
		{
			name: "unknown nested field",
			body: strings.Replace(valid, `"context_rel": [],`, `"context_rel": [{"kind":"customer-of","with":"x","surprise":true}],`, 1),
			want: "unknown field",
		},
		{
			name: "trailing json",
			body: valid + "\n{}",
			want: "trailing JSON",
		},
		{
			name: "numeric version",
			body: strings.Replace(valid, `"schema_version": "3"`, `"schema_version": 3`, 1),
			want: "schema_version",
		},
		{
			name: "unsupported version",
			body: strings.Replace(valid, `"schema_version": "3"`, `"schema_version": "5"`, 1),
			want: "unsupported",
		},
		{
			name: "version envelope mismatch",
			body: strings.Replace(valid, `"schema_version": "3"`, `"schema_version": "2"`, 1),
			want: "unknown field",
		},
		{
			name: "count mismatch",
			body: strings.Replace(valid, `"skill_count": 1`, `"skill_count": 2`, 1),
			want: "skill_count",
		},
		{
			name: "negative count",
			body: strings.Replace(valid, `"skill_count": 1`, `"skill_count": -1`, 1),
			want: "skill_count",
		},
		{
			name: "duplicate skill names",
			body: strings.Replace(valid, `"skill_count": 1`, `"skill_count": 2`, 1),
			want: "duplicate skill name",
		},
	}
	tests[len(tests)-1].body = strings.Replace(
		tests[len(tests)-1].body,
		"\n  ]\n}",
		",\n"+extractFirstEntry(t, valid)+"\n  ]\n}",
		1,
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := loadCatalogBody(t, tt.body)
			if err == nil {
				t.Fatal("LoadCatalog succeeded; want rejection")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}

func TestLoadCatalogRejectsMalformedVersionSpecificEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fixture string
		old     string
		new     string
		want    string
	}{
		{name: "v1 missing required field", fixture: "catalog-v1.json", old: `"consumes": [],`, new: "", want: "consumes"},
		{name: "v1 invalid role", fixture: "catalog-v1.json", old: `"hexagonal_role": "supporting"`, new: `"hexagonal_role": "operator"`, want: "hexagonal_role"},
		{name: "v1 invalid timestamp", fixture: "catalog-v1.json", old: `"generated_at": "2026-05-20T22:21:21Z"`, new: `"generated_at": "yesterday"`, want: "generated_at"},
		{name: "v2 missing dependency field", fixture: "catalog-v2.json", old: `"dependencies": ["legacy-v1"],`, new: "", want: "dependencies"},
		{name: "v2 duplicate dependency", fixture: "catalog-v2.json", old: `"dependencies": ["legacy-v1"]`, new: `"dependencies": ["legacy-v1", "legacy-v1"]`, want: "dependencies"},
		{name: "v3 missing capabilities", fixture: "catalog-v3.json", old: `"capabilities": ["judge"],`, new: "", want: "capabilities"},
		{name: "v3 invalid canonical status", fixture: "catalog-v3.json", old: `"canonical_status": "canonical"`, new: `"canonical_status": "draft"`, want: "canonical_status"},
		{name: "v3 invalid disposition", fixture: "catalog-v3.json", old: `"disposition": "keep"`, new: `"disposition": "discard"`, want: "disposition"},
		{name: "v4 missing contract", fixture: "catalog-v4.json", old: contractV3Fragment(t), new: "", want: "contract_v3"},
		{name: "v4 missing contract schema version", fixture: "catalog-v4.json", old: `"schema_version": "skill-contract.v3",`, new: "", want: "schema_version"},
		{name: "v4 invalid contract schema version", fixture: "catalog-v4.json", old: `"schema_version": "skill-contract.v3"`, new: `"schema_version": "skill-contract.v2"`, want: "schema_version"},
		{name: "v4 invalid layer", fixture: "catalog-v4.json", old: `"primary_layer": "support"`, new: `"primary_layer": "core"`, want: "primary_layer"},
		{name: "v4 null layer", fixture: "catalog-v4.json", old: `"primary_layer": "support"`, new: `"primary_layer": null`, want: "null value"},
		{name: "v4 invalid effect id", fixture: "catalog-v4.json", old: `"id": "publish-owned-projection"`, new: `"id": "Publish Projection"`, want: "invalid identifier"},
		{name: "v4 invalid effect kind", fixture: "catalog-v4.json", old: `"kind": "filesystem.write"`, new: `"kind": "filesystem.delete"`, want: "kind"},
		{name: "v4 incomplete triggers", fixture: "catalog-v4.json", old: `"negative": [`, new: `"omitted_negative": [`, want: "unknown field"},
		{name: "v4 wrong trigger expectation", fixture: "catalog-v4.json", old: `"expected": "route"`, new: `"expected": "clarify"`, want: "expected"},
		{name: "v4 duplicate cross-family trigger id", fixture: "catalog-v4.json", old: `"id": "run-experiment"`, new: `"id": "compile-contract"`, want: "duplicate trigger id"},
		{name: "v4 incomplete failure semantics", fixture: "catalog-v4.json", old: `"cleanup": {`, new: `"other": {`, want: "unknown field"},
		{name: "v4 invalid failure action", fixture: "catalog-v4.json", old: `"action": "stop"`, new: `"action": "retry"`, want: "action"},
		{name: "v4 malformed effect", fixture: "catalog-v4.json", old: `"receipt": "required"`, new: `"receipt": 7`, want: "receipt"},
		{name: "v4 unknown nested effect field", fixture: "catalog-v4.json", old: `"scope": "owned-projections",`, new: `"scope": "owned-projections", "surprise": true,`, want: "unknown field"},
		{name: "v4 invalid artifact kind", fixture: "catalog-v4.json", old: `"kind": "source"`, new: `"kind": "blob"`, want: "kind"},
		{name: "v4 empty artifact schema ref", fixture: "catalog-v4.json", old: `"schema_ref": "skill-contract.v3"`, new: `"schema_ref": ""`, want: "must not be an empty string"},
		{name: "v4 invalid proof class", fixture: "catalog-v4.json", old: `"class": "deterministic"`, new: `"class": "phrase_grep"`, want: "class"},
		{name: "v4 empty proof fixture", fixture: "catalog-v4.json", old: `"fixture_refs": ["testdata/catalog-v4.json"]`, new: `"fixture_refs": [""]`, want: "fixture_refs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := strings.Replace(mustReadFixture(t, tt.fixture), tt.old, tt.new, 1)
			if body == mustReadFixture(t, tt.fixture) {
				t.Fatalf("test mutation did not apply: %q", tt.old)
			}
			err := loadCatalogBody(t, body)
			if err == nil {
				t.Fatal("LoadCatalog succeeded; want rejection")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}

func TestLoadCatalogStrictnessAcrossVersions(t *testing.T) {
	t.Parallel()

	nextVersion := map[string]string{"1": "2", "2": "1", "3": "2", "4": "3"}
	for _, version := range []string{"1", "2", "3", "4"} {
		t.Run("v"+version, func(t *testing.T) {
			t.Parallel()
			valid := mustReadFixture(t, "catalog-v"+version+".json")
			mutations := []struct {
				name string
				body string
				want string
			}{
				{
					name: "duplicate object key",
					body: strings.Replace(
						valid,
						`"schema_version": "`+version+`"`,
						`"schema_version": "`+version+`", "schema_version": "`+version+`"`,
						1,
					),
					want: "duplicate object key",
				},
				{
					name: "unknown entry field",
					body: strings.Replace(valid, `"name":`, `"unknown_entry_field": true, "name":`, 1),
					want: "unknown field",
				},
				{name: "trailing JSON", body: valid + "\n[]", want: "trailing JSON"},
				{
					name: "numeric version",
					body: strings.Replace(valid, `"schema_version": "`+version+`"`, `"schema_version": `+version, 1),
					want: "schema_version must be a string",
				},
				{
					name: "version envelope mismatch",
					body: strings.Replace(
						valid,
						`"schema_version": "`+version+`"`,
						`"schema_version": "`+nextVersion[version]+`"`,
						1,
					),
					want: "catalog v",
				},
				{
					name: "count mismatch",
					body: strings.Replace(valid, `"skill_count": 1`, `"skill_count": 2`, 1),
					want: "skill_count",
				},
				{name: "duplicate skill names", body: duplicateFixtureEntry(t, valid), want: "duplicate skill name"},
				{
					name: "missing entry name",
					body: strings.Replace(valid, `"name":`, `"omitted_name":`, 1),
					want: "unknown field",
				},
			}
			for _, mutation := range mutations {
				t.Run(mutation.name, func(t *testing.T) {
					t.Parallel()
					err := loadCatalogBody(t, mutation.body)
					if err == nil {
						t.Fatal("LoadCatalog succeeded; want rejection")
					}
					if !strings.Contains(err.Error(), mutation.want) {
						t.Fatalf("error %q does not contain %q", err, mutation.want)
					}
				})
			}
		})
	}
}

func loadCatalogFixture(t *testing.T, name string) *Catalog {
	t.Helper()
	dir := t.TempDir()
	body := mustReadFixture(t, name)
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cat, err := LoadCatalog(dir)
	if err != nil {
		t.Fatalf("LoadCatalog(%s): %v", name, err)
	}
	return cat
}

func loadCatalogBody(t *testing.T, body string) error {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCatalog(dir)
	return err
}

func mustReadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func duplicateFixtureEntry(t *testing.T, body string) string {
	t.Helper()
	body = strings.Replace(body, `"skill_count": 1`, `"skill_count": 2`, 1)
	duplicated := strings.Replace(
		body,
		"\n  ]\n}",
		",\n"+extractFirstEntry(t, body)+"\n  ]\n}",
		1,
	)
	if duplicated == body {
		t.Fatal("could not duplicate fixture entry")
	}
	return duplicated
}

func extractFirstEntry(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, "    {")
	end := strings.LastIndex(body, "    }")
	if start < 0 || end < start {
		t.Fatal("fixture entry not found")
	}
	return body[start : end+5]
}

func assertV4ContractRoundTrip(t *testing.T, contract *ContractV3) {
	t.Helper()
	var fixture struct {
		Skills []struct {
			ContractV3 json.RawMessage `json:"contract_v3"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(mustReadFixture(t, "catalog-v4.json")), &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Skills) != 1 {
		t.Fatalf("v4 fixture skill count = %d, want 1", len(fixture.Skills))
	}
	gotRaw, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(fixture.Skills[0].ContractV3, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(gotRaw, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("v4 typed contract changed on round trip:\nwant: %s\ngot:  %s", fixture.Skills[0].ContractV3, gotRaw)
	}
}

func contractV3Fragment(t *testing.T) string {
	t.Helper()
	body := mustReadFixture(t, "catalog-v4.json")
	start := strings.Index(body, `      "contract_v3": {`)
	end := strings.LastIndex(body, "\n      }")
	if start < 0 || end < start {
		t.Fatal("contract_v3 fragment not found")
	}
	if start < 2 || body[start-2:start] != ",\n" {
		t.Fatal("contract_v3 fragment is not preceded by an entry-field separator")
	}
	start -= 2
	return body[start : end+8]
}
