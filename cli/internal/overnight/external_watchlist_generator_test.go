package overnight

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestExternalWatchlist_MissingFileSoftSuccess verifies a missing watchlist
// file is treated as a successful run with zero candidates — operators may
// not have curated a watchlist yet.
func TestExternalWatchlist_MissingFileSoftSuccess(t *testing.T) {
	cwd := t.TempDir()
	opts := newTestOpts(cwd)
	missing := filepath.Join(cwd, ".agents", "dream", "does-not-exist.yaml")
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	got := runExternalWatchlistGeneratorAt(context.Background(), opts, missing, now, now)
	if got.Status != "completed" {
		t.Errorf("Status = %q, want completed", got.Status)
	}
	if got.CandidateCount != 0 {
		t.Errorf("CandidateCount = %d, want 0", got.CandidateCount)
	}
	if got.Error != "" {
		t.Errorf("Error = %q, want empty", got.Error)
	}
}

// TestExternalWatchlist_MalformedYamlSoftFails verifies invalid YAML produces
// a soft-fail sidecar with a populated Error rather than a panic.
func TestExternalWatchlist_MalformedYamlSoftFails(t *testing.T) {
	cwd := t.TempDir()
	opts := newTestOpts(cwd)
	path := filepath.Join(cwd, "watchlist.yaml")
	if err := os.WriteFile(path, []byte("entries: [not-a-list-of-objects\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	got := runExternalWatchlistGeneratorAt(context.Background(), opts, path, now, now)
	if got.Status != "soft-fail" {
		t.Errorf("Status = %q, want soft-fail", got.Status)
	}
	if got.Error == "" {
		t.Errorf("expected non-empty Error on malformed YAML")
	}
}

// TestExternalWatchlist_StalenessGate verifies entries within their
// stale_after window emit nothing and entries past the window emit one
// candidate each.
func TestExternalWatchlist_StalenessGate(t *testing.T) {
	cwd := t.TempDir()
	opts := newTestOpts(cwd)
	path := filepath.Join(cwd, "watchlist.yaml")
	contents := `entries:
  - id: stale-one
    title: stale entry one
    type: task
    severity: medium
    source: src://stale-one
    last_seen_at: 2026-04-01T00:00:00Z
  - id: fresh
    title: fresh entry
    type: task
    severity: low
    source: src://fresh
    last_seen_at: 2026-04-25T00:00:00Z
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	got := runExternalWatchlistGeneratorAt(context.Background(), opts, path, now, now)
	if got.Status != "completed" {
		t.Fatalf("Status = %q, want completed", got.Status)
	}
	if got.CandidateCount != 1 {
		t.Fatalf("CandidateCount = %d, want 1\ncandidates=%+v", got.CandidateCount, got.Candidates)
	}
	c := got.Candidates[0]
	if c.ID != "stale-one" {
		t.Errorf("emitted candidate ID = %q, want stale-one", c.ID)
	}
	if c.Status != "proposed" {
		t.Errorf("Status = %q, want proposed", c.Status)
	}
	if len(c.Requires) != 1 || c.Requires[0] != "human-review" {
		t.Errorf("Requires = %v", c.Requires)
	}
	if !strings.HasPrefix(c.DedupKey, "external-watchlist|") {
		t.Errorf("DedupKey = %q, want external-watchlist| prefix", c.DedupKey)
	}
}

// TestExternalWatchlist_DefaultStaleAfter verifies the default 168h
// threshold applies when stale_after is omitted on an entry.
func TestExternalWatchlist_DefaultStaleAfter(t *testing.T) {
	cwd := t.TempDir()
	opts := newTestOpts(cwd)
	path := filepath.Join(cwd, "watchlist.yaml")
	contents := `entries:
  - id: just-stale
    title: just past 168h
    type: task
    severity: low
    source: src://just-stale
    last_seen_at: 2026-04-18T00:00:00Z
  - id: just-fresh
    title: just within 168h
    type: task
    severity: low
    source: src://just-fresh
    last_seen_at: 2026-04-20T00:00:00Z
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	// Pin "now" exactly 168h+1m past just-stale.
	now := time.Date(2026, 4, 25, 0, 1, 0, 0, time.UTC)

	got := runExternalWatchlistGeneratorAt(context.Background(), opts, path, now, now)
	if got.CandidateCount != 1 {
		t.Fatalf("CandidateCount = %d, want 1\ncandidates=%+v", got.CandidateCount, got.Candidates)
	}
	if got.Candidates[0].ID != "just-stale" {
		t.Errorf("emitted = %q, want just-stale", got.Candidates[0].ID)
	}
}

// TestExternalWatchlist_NeverSeededIsHeld verifies entries with zero
// last_seen_at never emit — operators must seed the cursor first.
func TestExternalWatchlist_NeverSeededIsHeld(t *testing.T) {
	cwd := t.TempDir()
	opts := newTestOpts(cwd)
	path := filepath.Join(cwd, "watchlist.yaml")
	contents := `entries:
  - id: unseeded
    title: never reviewed
    type: task
    severity: low
    source: src://unseeded
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	got := runExternalWatchlistGeneratorAt(context.Background(), opts, path, now, now)
	if got.CandidateCount != 0 {
		t.Errorf("CandidateCount = %d, want 0 for unseeded entry", got.CandidateCount)
	}
}

// TestExternalWatchlist_FixtureRun is the L2 fixture-driven test. It uses
// the canonical fixture under testdata/ and verifies that the four sample
// entries produce two stale candidates with the expected fields.
func TestExternalWatchlist_FixtureRun(t *testing.T) {
	cwd := t.TempDir()
	opts := newTestOpts(cwd)
	fixturePath := filepath.Join("testdata", "external-watchlist-fixture.yaml")
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	got := runExternalWatchlistGeneratorAt(context.Background(), opts, fixturePath, now, now)
	if got.Status != "completed" {
		t.Fatalf("Status = %q, want completed", got.Status)
	}
	// Fixture has 4 entries: stale-week (>168h), fresh-day (<168h),
	// stale-custom-window (>48h), never-seeded (zero last_seen).
	// Expected: 2 candidates emitted (stale-week, stale-custom-window).
	if got.CandidateCount != 2 {
		t.Fatalf("CandidateCount = %d, want 2\ncandidates=%+v", got.CandidateCount, got.Candidates)
	}
	emittedIDs := make(map[string]bool, len(got.Candidates))
	for _, c := range got.Candidates {
		emittedIDs[c.ID] = true
		if c.Status != "proposed" {
			t.Errorf("candidate %s Status = %q, want proposed", c.ID, c.Status)
		}
		if len(c.Requires) != 1 || c.Requires[0] != "human-review" {
			t.Errorf("candidate %s Requires = %v", c.ID, c.Requires)
		}
		if !strings.HasPrefix(c.DedupKey, "external-watchlist|") {
			t.Errorf("candidate %s DedupKey = %q, want external-watchlist| prefix", c.ID, c.DedupKey)
		}
	}
	if !emittedIDs["stale-week"] {
		t.Errorf("expected stale-week to emit")
	}
	if !emittedIDs["stale-custom-window"] {
		t.Errorf("expected stale-custom-window to emit (48h override)")
	}
	if emittedIDs["fresh-day"] {
		t.Errorf("fresh-day should not emit")
	}
	if emittedIDs["never-seeded"] {
		t.Errorf("never-seeded should not emit")
	}
}

// TestExternalWatchlist_ContextCancelSoftFails verifies a pre-cancelled
// context yields a soft-fail sidecar instead of crashing or producing
// partial output.
func TestExternalWatchlist_ContextCancelSoftFails(t *testing.T) {
	cwd := t.TempDir()
	opts := newTestOpts(cwd)
	path := filepath.Join(cwd, "watchlist.yaml")
	if err := os.WriteFile(path, []byte("entries: []\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	now := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)

	got := runExternalWatchlistGeneratorAt(ctx, opts, path, now, now)
	if got.Status != "soft-fail" {
		t.Errorf("Status = %q, want soft-fail", got.Status)
	}
	if got.Error == "" {
		t.Errorf("expected Error to mention cancellation")
	}
}
