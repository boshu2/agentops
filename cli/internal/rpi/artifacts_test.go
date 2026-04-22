package rpi

import (
	"testing"
)

func TestPathClean(t *testing.T) {
	cases := map[string]string{
		"  foo/bar  ":    "foo/bar",
		"foo//bar":       "foo/bar",
		"foo/./bar":      "foo/bar",
		"foo/bar/../baz": "foo/baz",
	}
	for in, want := range cases {
		if got := PathClean(in); got != want {
			t.Errorf("PathClean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsSafeArtifactRelPath(t *testing.T) {
	safe := []string{
		"a/b.json",
		"plans/plan.md",
		"x.txt",
	}
	for _, p := range safe {
		if !IsSafeArtifactRelPath(p) {
			t.Errorf("%q should be safe", p)
		}
	}
	unsafe := []string{
		"",
		".",
		"..",
		"../x",
		"/absolute",
		"/etc/passwd",
	}
	for _, p := range unsafe {
		if IsSafeArtifactRelPath(p) {
			t.Errorf("%q should be unsafe", p)
		}
	}
}

func TestClassifyRPIArtifact(t *testing.T) {
	cases := []struct {
		rel       string
		wantKind  string
		wantPhase int
	}{
		{".agents/rpi/runs/x/execution-packet.json", "execution_packet", 0},
		{".agents/rpi/state.json", "phased_state", 0},
		{".agents/rpi/runs/x/c2-events.jsonl", "run_events", 0},
		{".agents/rpi/runs/x/heartbeat.txt", "run_heartbeat", 0},
		{".agents/rpi/runs/x/phase-2-result.json", "phase_result", 2},
		{".agents/rpi/runs/x/phase-1-handoff.json", "phase_handoff", 1},
		{".agents/rpi/runs/x/phase-3-summary.md", "phase_summary", 3},
		{".agents/rpi/runs/x/phase-2-evaluator.json", "phase_evaluator", 2},
		{".agents/rpi/plans/foo.md", "plan", 0},
		{".agents/rpi/research/x.md", "research", 0},
		{".agents/rpi/council/pre-mortem-x.md", "council_pre_mortem", 0},
		{".agents/rpi/council/post-mortem-y.md", "council_post_mortem", 0},
		{".agents/rpi/council/vibe-check.md", "council_vibe", 0},
		{"random/file.txt", "artifact", 0},
	}
	for _, tc := range cases {
		t.Run(tc.rel, func(t *testing.T) {
			kind, _, phase := ClassifyRPIArtifact(tc.rel, "state.json", "c2-events.jsonl")
			if kind != tc.wantKind || phase != tc.wantPhase {
				t.Errorf("got (%q, %d), want (%q, %d)", kind, phase, tc.wantKind, tc.wantPhase)
			}
		})
	}
}

func TestArtifactPhaseNumber(t *testing.T) {
	cases := map[string]int{
		"phase-1-result.json":  1,
		"phase-42-handoff.md":  42,
		"no-phase-here.txt":    0,
		"phase-notanumber.txt": 0,
	}
	for in, want := range cases {
		if got := ArtifactPhaseNumber(in); got != want {
			t.Errorf("ArtifactPhaseNumber(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestArtifactContentType(t *testing.T) {
	cases := map[string]string{
		"foo.json":  "application/json",
		"foo.jsonl": "application/json",
		"foo.md":    "text/markdown",
		"foo.mdx":   "text/markdown",
		"foo.txt":   "text/plain",
		"foo":       "text/plain",
		"foo.JSON":  "application/json", // case-insensitive
	}
	for in, want := range cases {
		if got := ArtifactContentType(in); got != want {
			t.Errorf("ArtifactContentType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSortArtifactRefs(t *testing.T) {
	refs := []ArtifactRef{
		{Path: "b.json", UpdatedAt: "2026-04-20T10:00:00Z"},
		{Path: "a.json", UpdatedAt: "2026-04-22T10:00:00Z"},
		{Path: "c.json", UpdatedAt: "2026-04-22T10:00:00Z"},
	}
	SortArtifactRefs(refs)
	// Newest first; within same date, alphabetical
	if refs[0].Path != "a.json" {
		t.Errorf("[0] = %q", refs[0].Path)
	}
	if refs[1].Path != "c.json" {
		t.Errorf("[1] = %q", refs[1].Path)
	}
	if refs[2].Path != "b.json" {
		t.Errorf("[2] = %q", refs[2].Path)
	}
}
