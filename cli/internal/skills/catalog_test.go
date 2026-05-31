package skills

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// fixture builds a small, deterministic catalog for query tests.
func fixture() []CatalogEntry {
	return []CatalogEntry{
		{
			Name:          "evolve",
			Description:   "Run autonomous improvement loops.",
			HexagonalRole: "driving-adapter",
			Consumes:      []string{"rpi", "goals"},
			Produces:      []string{"verdict-ledger"},
			Practices:     []string{"tdd", "dora-metrics"},
			UserInvocable: true,
		},
		{
			Name:          "rpi",
			Description:   "Run discovery, crank, validation.",
			HexagonalRole: "domain",
			Consumes:      []string{"discovery"},
			Produces:      []string{"verdict-ledger", "handoff"},
			Practices:     []string{"tdd"},
			UserInvocable: true,
		},
		{
			Name:          "goals",
			Description:   "Maintain AgentOps goals.",
			HexagonalRole: "supporting",
			Consumes:      []string{},
			Produces:      []string{"goals-doc"},
			Practices:     []string{"bdd-gherkin"},
			UserInvocable: false,
		},
	}
}

func TestList_FilterByRole(t *testing.T) {
	got := List(fixture(), ListFilter{Role: "domain"})
	if len(got) != 1 {
		t.Fatalf("role=domain: want 1 entry, got %d", len(got))
	}
	if got[0].Name != "rpi" {
		t.Errorf("role=domain: want rpi, got %q", got[0].Name)
	}
}

func TestList_FilterByRoleCaseInsensitive(t *testing.T) {
	got := List(fixture(), ListFilter{Role: "DOMAIN"})
	if len(got) != 1 || got[0].Name != "rpi" {
		t.Errorf("role=DOMAIN: want [rpi], got %v", names(got))
	}
}

func TestList_FilterByProduces(t *testing.T) {
	got := names(List(fixture(), ListFilter{Produces: "verdict-ledger"}))
	want := []string{"evolve", "rpi"} // sorted by name
	if !reflect.DeepEqual(got, want) {
		t.Errorf("produces=verdict-ledger: want %v, got %v", want, got)
	}
}

func TestList_FilterByConsumes(t *testing.T) {
	got := names(List(fixture(), ListFilter{Consumes: "rpi"}))
	want := []string{"evolve"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("consumes=rpi: want %v, got %v", want, got)
	}
}

func TestList_FilterByPractice(t *testing.T) {
	got := names(List(fixture(), ListFilter{Practice: "tdd"}))
	want := []string{"evolve", "rpi"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("practice=tdd: want %v, got %v", want, got)
	}
}

func TestList_FilterByUserInvocable(t *testing.T) {
	yes := true
	got := names(List(fixture(), ListFilter{UserInvocable: &yes}))
	want := []string{"evolve", "rpi"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("user_invocable=true: want %v, got %v", want, got)
	}

	no := false
	got = names(List(fixture(), ListFilter{UserInvocable: &no}))
	want = []string{"goals"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("user_invocable=false: want %v, got %v", want, got)
	}
}

func TestList_CombinedFilters(t *testing.T) {
	got := names(List(fixture(), ListFilter{Produces: "verdict-ledger", Role: "domain"}))
	want := []string{"rpi"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("produces+role: want %v, got %v", want, got)
	}
}

func TestList_NoFilterReturnsAllSortedNonNil(t *testing.T) {
	got := List(fixture(), ListFilter{})
	want := []string{"evolve", "goals", "rpi"}
	if !reflect.DeepEqual(names(got), want) {
		t.Errorf("no filter: want %v, got %v", want, names(got))
	}
}

func TestList_NoMatchReturnsEmptyNotNil(t *testing.T) {
	got := List(fixture(), ListFilter{Role: "nonexistent"})
	if got == nil {
		t.Fatal("no-match List returned nil; want non-nil empty slice for JSON []")
	}
	if len(got) != 0 {
		t.Errorf("no-match: want 0 entries, got %d", len(got))
	}
}

func TestConsumers_Exact(t *testing.T) {
	got := Consumers(fixture(), "rpi")
	want := []string{"evolve"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("consumers(rpi): want %v, got %v", want, got)
	}
}

func TestConsumers_DiscoveryHasOne(t *testing.T) {
	got := Consumers(fixture(), "discovery")
	want := []string{"rpi"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("consumers(discovery): want %v, got %v", want, got)
	}
}

func TestConsumers_NoneReturnsEmptyNotNil(t *testing.T) {
	// "evolve" is consumed by nobody in the fixture.
	got := Consumers(fixture(), "evolve")
	if got == nil {
		t.Fatal("consumers(evolve) returned nil; want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("consumers(evolve): want 0, got %v", got)
	}
}

func TestProducers_VerdictLedger(t *testing.T) {
	got := Producers(fixture(), "verdict-ledger")
	want := []string{"evolve", "rpi"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("producers(verdict-ledger): want %v, got %v", want, got)
	}
}

func TestProducers_Unique(t *testing.T) {
	got := Producers(fixture(), "goals-doc")
	want := []string{"goals"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("producers(goals-doc): want %v, got %v", want, got)
	}
}

func TestMermaid_Deterministic(t *testing.T) {
	want := "graph LR\n" +
		"  s_evolve[evolve]\n" +
		"  s_goals[goals]\n" +
		"  s_rpi[rpi]\n" +
		"  s_evolve --> s_goals\n" +
		"  s_evolve --> s_rpi\n"
	got := Mermaid(fixture())
	if got != want {
		t.Errorf("Mermaid mismatch:\nwant:\n%q\ngot:\n%q", want, got)
	}
}

func TestMermaid_OmitsUnknownEdges(t *testing.T) {
	// rpi consumes "discovery", which is not a node in the fixture, so no
	// edge from rpi should appear.
	got := Mermaid(fixture())
	if contains(got, "discovery") {
		t.Errorf("Mermaid drew edge to unknown skill 'discovery':\n%s", got)
	}
}

func TestLoadCatalog_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	const body = `{
  "schema_version": "1",
  "generated_at": "2026-05-31T00:00:00Z",
  "skill_count": 1,
  "skills": [
    {
      "name": "evolve",
      "description": "loops",
      "hexagonal_role": "driving-adapter",
      "consumes": ["rpi"],
      "produces": ["verdict-ledger"],
      "context_rel": [],
      "practices": ["tdd"],
      "user_invocable": true,
      "codex_override_present": false,
      "references_count": 2
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, err := LoadCatalog(dir)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if cat.SchemaVersion != "1" {
		t.Errorf("schema_version: want 1, got %q", cat.SchemaVersion)
	}
	if cat.SkillCount != 1 || len(cat.Skills) != 1 {
		t.Fatalf("skill_count: want 1/1, got %d/%d", cat.SkillCount, len(cat.Skills))
	}
	e := cat.Skills[0]
	if e.Name != "evolve" || e.HexagonalRole != "driving-adapter" || e.ReferencesCount != 2 {
		t.Errorf("entry mismatch: %+v", e)
	}
	if !reflect.DeepEqual(e.Consumes, []string{"rpi"}) {
		t.Errorf("consumes: want [rpi], got %v", e.Consumes)
	}
}

func TestLoadCatalog_MissingFileErrors(t *testing.T) {
	_, err := LoadCatalog(t.TempDir())
	if err == nil {
		t.Fatal("LoadCatalog on missing file: want error, got nil")
	}
}

// names projects entries to a sorted-by-construction slice of names.
func names(entries []CatalogEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
