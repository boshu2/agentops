package domainsignal

import (
	"reflect"
	"testing"
)

func TestClassifyPathToBC(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"cli/internal/gates/escape.go", BC2Validation},
		{"cli/internal/gates/run.go", BC2Validation},
		{"scripts/custom-review.sh", ""},              // prefix-only: scripts/ is a mixed bag -> no signal
		{"scripts/check-go-command-test-pair.sh", ""}, // unclassified -> no signal
		{"_beads/history.jsonl", BC3Loop},             // historical tracker artifact
		{"tests/scripts/foo.bats", BC2Validation},     // clean prefix
		{".agents/yield/yield-ledger.jsonl", BC1Corpus},
		{"cli/internal/wiki/locator.go", BC1Corpus},
		{"_beads/issues.jsonl", BC3Loop},
		{"_beads/handoff.jsonl", BC3Loop},
		{"skills/discovery/SKILL.md", BC4Factory},
		{"cli/internal/orchestration/shape.go", BC6Orchestration},
		{"cli/internal/swarm/x.go", BC6Orchestration},
		{"cli/cmd/ao/membrane.go", BC5Runtime}, // runtime adapter (broad, last)
		{"cli/internal/runtimecmd/sibling.go", BC5Runtime},
		{"README.md", ""},                // unclassified -> no signal
		{"docs/architecture/foo.md", ""}, // unclassified
	}
	for _, c := range cases {
		if got := ClassifyPathToBC(c.path); got != c.want {
			t.Errorf("ClassifyPathToBC(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// Regression: a bare "gate" substring rule
// misclassified Go files that merely CONTAIN the substring — "aggreGATE",
// "naviGATE", "planPAWL" outside its package. The contains rules are now scoped to
// scripts/, so these classify by their real prefix (or not at all), never BC2.
func TestClassifyPathToBC_NoUnanchoredSubstringFalsePositives(t *testing.T) {
	cases := []struct{ path, want string }{
		{"cli/internal/search/aggregate.go", BC1Corpus}, // "aggreGATE" -> BC1 via search prefix, NOT BC2
		{"_beads/navigate.jsonl", BC3Loop},              // historical BC3 prefix, NOT BC2
		{"cli/internal/wiki/propagate.go", BC1Corpus},   // "propaGATE" -> BC1, NOT BC2
		{"cli/cmd/ao/delegate.go", BC5Runtime},          // "deleGATE" -> BC5, NOT BC2
		{"docs/mitigate-plan.md", ""},                   // "mitiGATE" in an unclassified path -> still ""
		// prefix-only means even a scripts/ path containing "gate"/"pawl" is NOT
		// force-classified — no substring matching exists to create a false signal:
		{"scripts/run-gate.sh", ""},
		{"scripts/aggregate-data.sh", ""}, // "gate" substring, but no clean prefix -> no false BC2
		{"scripts/custom-review.sh", ""},
	}
	for _, c := range cases {
		if got := ClassifyPathToBC(c.path); got != c.want {
			t.Errorf("ClassifyPathToBC(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestClassifyPathToBC_SpecificBeforeBroadRuntime(t *testing.T) {
	// A validation package under cli/internal must NOT be swallowed by the broad
	// cli/cmd/ao runtime rule (which is intentionally last). gates is BC2.
	if got := ClassifyPathToBC("cli/internal/gates/writer.go"); got != BC2Validation {
		t.Fatalf("gates must classify BC2 (specific before broad), got %q", got)
	}
}

func TestChangedFilesDomains_DistinctSetInBCOrder(t *testing.T) {
	got := ChangedFilesDomains([]string{
		"cli/cmd/ao/membrane.go",       // BC5
		"cli/internal/gates/escape.go", // BC2
		"cli/internal/gates/writer.go", // BC2 (dup)
		"README.md",                    // unclassified -> dropped
	})
	want := []string{BC2Validation, BC5Runtime} // sorted BC2 < BC5, distinct
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ChangedFilesDomains = %v, want %v (distinct, BC-ordered, unclassified dropped)", got, want)
	}
}

func TestChangedFilesDomains_DoesNotCollapseToDominant(t *testing.T) {
	// The whole point: preserve the cross-domain spread. Two BCs in, two BCs out —
	// even when one BC has more files (dominant-only would hide the minority).
	got := ChangedFilesDomains([]string{
		"cli/internal/gates/a.go", "cli/internal/gates/b.go", "cli/internal/safety/c.go", // 3x BC2
		"cli/internal/swarm/d.go", // 1x BC6 (the minority signal that must survive)
	})
	want := []string{BC2Validation, BC6Orchestration}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("minority BC must survive (no dominant collapse): got %v, want %v", got, want)
	}
}

func TestBuild_MismatchWhenIntentNotInChanged(t *testing.T) {
	// Intended BC2, but the code changed in BC5/BC6 -> the work crossed contexts.
	r := Build(BC2Validation, []string{"cli/cmd/ao/x.go", "cli/internal/swarm/y.go"}, "concurrency")
	if !r.Mismatch {
		t.Fatalf("intent BC2 not among changed {BC5,BC6} must be a mismatch; got %+v", r)
	}
	if r.IntentDomain != BC2Validation || r.EscapeDomain != "concurrency" {
		t.Fatalf("raw signals must be preserved: %+v", r)
	}
	if !reflect.DeepEqual(r.ChangedFileDomains, []string{BC5Runtime, BC6Orchestration}) {
		t.Fatalf("changed domains: %v", r.ChangedFileDomains)
	}
}

func TestBuild_NoMismatchWhenIntentAmongChanged(t *testing.T) {
	// Intended BC2 and the code DID change in BC2 (plus BC5) -> intent is satisfied,
	// no mismatch even though the change also spilled into BC5.
	r := Build(BC2Validation, []string{"cli/internal/gates/x.go", "cli/cmd/ao/y.go"}, BC2Validation)
	if r.Mismatch {
		t.Fatalf("intent BC2 IS among changed -> no mismatch; got %+v", r)
	}
}

func TestBuild_NoMismatchWhenSignalsMissing(t *testing.T) {
	// Missing intent or no classifiable changes -> cannot assert a mismatch.
	if Build("", []string{"cli/cmd/ao/x.go"}, "d").Mismatch {
		t.Error("no intent -> no mismatch")
	}
	if Build(BC2Validation, []string{"README.md"}, "d").Mismatch {
		t.Error("no classifiable changed files -> no mismatch")
	}
	if Build(BC2Validation, nil, "d").Mismatch {
		t.Error("no changed files -> no mismatch")
	}
}
