package provenancegraph

import (
	"os"
	"path/filepath"
	"testing"
)

// writeGraph writes lines to a temp JSONL graph file and returns its path.
func writeGraph(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.jsonl")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write graph: %v", err)
	}
	return path
}

func TestFindOrphans_ArtifactWithNoInboundEdgeIsOrphan(t *testing.T) {
	recs := []GraphRecord{
		{Record: "node", ID: "gate:scenario-hash-stability", Type: "artifact", Path: ".github/workflows/validate.yml"},
		{Record: "node", ID: "d-gherkin-acceptance", Type: "directive", Path: "GOALS.md"},
		{Record: "edge", EdgeType: "directive_has_scenario", FromID: "d-gherkin-acceptance", ToID: "s-gherkin-acceptance-001"},
	}
	got := FindOrphans(recs)
	if len(got) != 1 {
		t.Fatalf("want 1 orphan, got %d: %+v", len(got), got)
	}
	want := OrphanFinding{
		Severity:         "error",
		Code:             "no_inbound_authored_edge",
		OrphanArtifactID: "gate:scenario-hash-stability",
		Path:             ".github/workflows/validate.yml",
		Message:          "artifact gate:scenario-hash-stability has no inbound authored/inferred provenance edge",
	}
	if got[0] != want {
		t.Fatalf("orphan finding mismatch:\n got  %+v\n want %+v", got[0], want)
	}
}

func TestFindOrphans_ArtifactWithInboundEdgeIsNotOrphan(t *testing.T) {
	recs := []GraphRecord{
		{Record: "node", ID: "gate:scenario-hash-stability", Type: "artifact", Path: ".github/workflows/validate.yml"},
		{Record: "node", ID: "d-gherkin-acceptance", Type: "directive", Path: "GOALS.md"},
		// An edge now points AT the artifact — it is wired, no longer orphaned.
		{Record: "edge", EdgeType: "bead_produced_artifact", FromID: "d-gherkin-acceptance", ToID: "gate:scenario-hash-stability"},
	}
	if got := FindOrphans(recs); len(got) != 0 {
		t.Fatalf("want 0 orphans once wired, got %d: %+v", len(got), got)
	}
}

func TestFindOrphans_NonArtifactNodesNeverOrphaned(t *testing.T) {
	// A directive with no inbound edge is NOT an orphan: the audit only flags
	// engineered artifact nodes.
	recs := []GraphRecord{
		{Record: "node", ID: "d-orphan-directive", Type: "directive", Path: "GOALS.md"},
		{Record: "node", ID: "s-lonely-scenario", Type: "scenario"},
	}
	if got := FindOrphans(recs); len(got) != 0 {
		t.Fatalf("want 0 orphans for non-artifact nodes, got %d: %+v", len(got), got)
	}
}

func TestFindOrphans_DeterministicSortByArtifactID(t *testing.T) {
	recs := []GraphRecord{
		{Record: "node", ID: "zzz", Type: "artifact"},
		{Record: "node", ID: "aaa", Type: "artifact"},
		{Record: "node", ID: "mmm", Type: "artifact"},
	}
	got := FindOrphans(recs)
	if len(got) != 3 {
		t.Fatalf("want 3 orphans, got %d", len(got))
	}
	wantOrder := []string{"aaa", "mmm", "zzz"}
	for i, w := range wantOrder {
		if got[i].OrphanArtifactID != w {
			t.Fatalf("orphan[%d] = %q, want %q (sort not deterministic)", i, got[i].OrphanArtifactID, w)
		}
	}
}

func TestReadGraphRecords_SkipsBlankAndParsesRecords(t *testing.T) {
	path := writeGraph(t,
		`{"record":"node","id":"a","type":"artifact","path":"p"}`,
		``,
		`{"record":"edge","edge_type":"directive_has_scenario","from_id":"d","to_id":"s"}`,
	)
	recs, err := ReadGraphRecords(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("want 2 records (blank skipped), got %d", len(recs))
	}
	if recs[0].Record != "node" || recs[0].ID != "a" || recs[0].Type != "artifact" {
		t.Fatalf("node record parse mismatch: %+v", recs[0])
	}
	if recs[1].Record != "edge" || recs[1].FromID != "d" || recs[1].ToID != "s" {
		t.Fatalf("edge record parse mismatch: %+v", recs[1])
	}
}

func TestReadGraphRecords_RejectsBadDiscriminator(t *testing.T) {
	path := writeGraph(t, `{"record":"bogus","id":"x"}`)
	if _, err := ReadGraphRecords(path); err == nil {
		t.Fatal("want error for unknown record discriminator, got nil")
	}
}

func TestReadGraphRecords_RejectsMalformedJSON(t *testing.T) {
	path := writeGraph(t, `{not json`)
	if _, err := ReadGraphRecords(path); err == nil {
		t.Fatal("want error for malformed JSON, got nil")
	}
}

func TestReadGraphRecords_MissingFileIsError(t *testing.T) {
	if _, err := ReadGraphRecords(filepath.Join(t.TempDir(), "absent.jsonl")); err == nil {
		t.Fatal("want error for missing graph file, got nil")
	}
}
