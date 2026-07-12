package done

import (
	"context"
	"errors"
	"strings"
	"testing"

	doneapp "github.com/boshu2/agentops/cli/internal/done"
	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

func TestRepositoryMapsGitAndFailsClosed(t *testing.T) {
	repository := Repository{
		WorkingDirFunc: func() (string, error) { return "/repo", nil },
		GitOutput: func(_ context.Context, cwd string, args ...string) ([]byte, error) {
			if cwd != "/repo" {
				t.Fatalf("cwd = %q", cwd)
			}
			switch args[0] {
			case "rev-parse":
				return []byte("abcdef0123456789\n"), nil
			case "diff-tree":
				return []byte("docs/provenance/ledger.jsonl\x00"), nil
			case "show":
				edge := provenancegraph.Edge{FromID: "age-1@abcdef0", FromType: "verdict", ToID: "abcdef0123456789", ToType: "commit", Relation: "wasDerivedFrom", EvidenceRef: "disposition=CONFIRMED"}
				return []byte(mustEncodeEdge(t, edge)), nil
			default:
				return nil, errors.New("unexpected")
			}
		},
	}
	if cwd, _ := repository.WorkingDir(); cwd != "/repo" {
		t.Fatalf("cwd = %q", cwd)
	}
	if sha, err := repository.ResolveHead(context.Background(), "/repo"); err != nil || sha != "abcdef0123456789" {
		t.Fatalf("sha=%q err=%v", sha, err)
	}
	if !repository.CommitProvenanceOnly(context.Background(), "/repo", "abcdef0") {
		t.Fatal("provenance-only commit not recognized")
	}
	edges, ok := repository.OriginEdges(context.Background(), "/repo")
	if !ok || len(edges) != 1 || edges[0].FromID != "age-1@abcdef0" {
		t.Fatalf("origin edges = %+v ok=%v", edges, ok)
	}

	repository.GitOutput = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("git failed") }
	if repository.CommitProvenanceOnly(context.Background(), "/repo", "abcdef0") {
		t.Fatal("git failure must fail closed")
	}
	if _, ok := repository.OriginEdges(context.Background(), "/repo"); ok {
		t.Fatal("origin read failure reported success")
	}
}

func TestLedgerAndTrackerMapPorts(t *testing.T) {
	ledger := Ledger{ReadEdges: func() ([]provenancegraph.Edge, error) {
		return []provenancegraph.Edge{{FromID: "v", Relation: "wasDerivedFrom"}}, nil
	}}
	edges, err := ledger.Read(context.Background())
	if err != nil || len(edges) != 1 || edges[0].FromID != "v" {
		t.Fatalf("edges=%+v err=%v", edges, err)
	}
	var got []string
	tracker := Tracker{Run: func(_ context.Context, args ...string) ([]byte, error) {
		got = append([]string(nil), args...)
		return []byte("closed\n"), nil
	}}
	output, err := tracker.Close(context.Background(), "age-1", "Done [verdict:abcdef0:CONFIRMED]")
	if err != nil || output != "closed\n" || strings.Join(got, " ") != "close age-1 -r Done [verdict:abcdef0:CONFIRMED]" {
		t.Fatalf("output=%q args=%v err=%v", output, got, err)
	}
	var _ doneapp.RepositoryPort = Repository{}
	var _ doneapp.LedgerPort = Ledger{}
	var _ doneapp.TrackerPort = Tracker{}
}

func mustEncodeEdge(t *testing.T, edge provenancegraph.Edge) string {
	t.Helper()
	return `{"from_id":"` + edge.FromID + `","from_type":"` + edge.FromType + `","to_id":"` + edge.ToID + `","to_type":"` + edge.ToType + `","relation":"` + edge.Relation + `","evidence_ref":"` + edge.EvidenceRef + `"}` + "\n"
}
