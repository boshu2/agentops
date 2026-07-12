package done

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const testSHA = "1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a"

type fakeRepository struct {
	cwd, head string
	trivial   bool
	origin    []Edge
	originOK  bool
}

func (repo fakeRepository) WorkingDir() (string, error) { return repo.cwd, nil }
func (repo fakeRepository) ResolveHead(context.Context, string) (string, error) {
	return repo.head, nil
}
func (repo fakeRepository) CommitProvenanceOnly(context.Context, string, string) bool {
	return repo.trivial
}
func (repo fakeRepository) OriginEdges(context.Context, string) ([]Edge, bool) {
	return repo.origin, repo.originOK
}

type fakeLedger struct {
	edges []Edge
	err   error
}

func (ledger fakeLedger) Read(context.Context) ([]Edge, error) { return ledger.edges, ledger.err }

type fakeTracker struct {
	id, reason string
	output     string
	err        error
}

func (tracker *fakeTracker) Close(_ context.Context, id, reason string) (string, error) {
	tracker.id, tracker.reason = id, reason
	return tracker.output, tracker.err
}

func verdictEdge(bead, sha, disposition string) Edge {
	return Edge{FromID: bead + "@" + sha[:7], FromType: "verdict", ToID: sha, ToType: "commit",
		Relation: "wasDerivedFrom", EvidenceRef: "pawl-verdict " + bead + " disposition=" + disposition}
}

func TestServiceConfirmedVerdictClosesWithProofStamp(t *testing.T) {
	tracker := &fakeTracker{output: "closed by tracker"}
	service := NewService(fakeRepository{cwd: "/repo"}, fakeLedger{edges: []Edge{verdictEdge("age-1", testSHA, DispositionConfirmed)}}, tracker)
	result, err := service.Execute(context.Background(), Request{BeadID: "age-1", SHA: testSHA, Reason: "Shipped"})
	if err != nil {
		t.Fatal(err)
	}
	if tracker.id != "age-1" || tracker.reason != "Shipped [verdict:1a1a1a1:CONFIRMED]" {
		t.Fatalf("tracker close = %q %q", tracker.id, tracker.reason)
	}
	if !result.Closed || result.Disposition != DispositionConfirmed || result.TrackerOutput != "closed by tracker" {
		t.Fatalf("result = %+v", result)
	}
}

func TestServiceRefusesMissingOrRefutedVerdict(t *testing.T) {
	for _, edges := range [][]Edge{nil, {verdictEdge("age-1", testSHA, "REFUTED")}} {
		tracker := &fakeTracker{}
		service := NewService(fakeRepository{cwd: "/repo"}, fakeLedger{edges: edges}, tracker)
		_, err := service.Execute(context.Background(), Request{BeadID: "age-1", SHA: testSHA})
		if err == nil || !strings.Contains(err.Error(), "no verdict = not done") || !strings.Contains(err.Error(), "ao verify age-1") {
			t.Fatalf("refusal = %v", err)
		}
		if tracker.id != "" {
			t.Fatal("refusal mutated tracker")
		}
	}
}

func TestServiceOriginFallbackTrivialWaiverAndForcedEscape(t *testing.T) {
	for _, test := range []struct {
		name, disposition string
		repo              fakeRepository
		force             bool
	}{
		{name: "origin", disposition: DispositionConfirmed, repo: fakeRepository{cwd: "/repo", originOK: true, origin: []Edge{verdictEdge("age-1", testSHA, DispositionConfirmed)}}},
		{name: "trivial", disposition: DispositionWaived, repo: fakeRepository{cwd: "/repo", trivial: true}},
		{name: "forced", disposition: DispositionUnverified, repo: fakeRepository{cwd: "/repo"}, force: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			tracker := &fakeTracker{}
			service := NewService(test.repo, fakeLedger{}, tracker)
			result, err := service.Execute(context.Background(), Request{BeadID: "age-1", SHA: testSHA, ForceNoVerdict: test.force})
			if err != nil {
				t.Fatal(err)
			}
			if result.Disposition != test.disposition {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestServicePropagatesLedgerAndTrackerFailures(t *testing.T) {
	tracker := &fakeTracker{err: errors.New("close failed")}
	service := NewService(fakeRepository{cwd: "/repo"}, fakeLedger{err: errors.New("ledger failed")}, tracker)
	if _, err := service.Execute(context.Background(), Request{BeadID: "age-1", SHA: testSHA}); err == nil || !strings.Contains(err.Error(), "read provenance ledger") {
		t.Fatalf("ledger error = %v", err)
	}
	service = NewService(fakeRepository{cwd: "/repo", trivial: true}, fakeLedger{}, tracker)
	if _, err := service.Execute(context.Background(), Request{BeadID: "age-1", SHA: testSHA}); err == nil || !strings.Contains(err.Error(), "close age-1") {
		t.Fatalf("tracker error = %v", err)
	}
}

func TestProvenanceOnlyChangedFilesIsExactAndFailClosed(t *testing.T) {
	for _, test := range []struct {
		input string
		want  bool
	}{
		{"docs/provenance/ledger.jsonl\x00", true},
		{"docs/provenance/a\x00docs/provenance/b\x00", true},
		{" docs/provenance/a\x00", false},
		{"docs/provenance-evil/a\x00", false},
		{"", false},
	} {
		if got := ProvenanceOnlyChangedFiles(test.input); got != test.want {
			t.Errorf("ProvenanceOnlyChangedFiles(%q) = %v, want %v", test.input, got, test.want)
		}
	}
}
