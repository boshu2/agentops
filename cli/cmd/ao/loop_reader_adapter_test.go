// practices: [hexagonal-architecture, tdd]
package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Sibling pattern: cycle 83's citation_port_adapter_test.go (this same
// dir) — temp dir + planted fixture + adapter call + assert.

func writeLoopFixture(t *testing.T, dir string, lines ...string) string {
	t.Helper()
	path := filepath.Join(dir, "cycle-history.jsonl")
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestProductionLoopReader_LatestPicksHighestNumber(t *testing.T) {
	dir := t.TempDir()
	path := writeLoopFixture(t, dir,
		`{"cycle":1,"mode":"a","result":"improved","commit":"sha1"}`,
		`{"cycle":2,"mode":"b","result":"improved","commit":"sha2"}`,
		`{"cycle":3,"mode":"c","result":"unchanged"}`,
	)
	r := newProductionLoopReader(path)
	v, err := r.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.Number != 3 {
		t.Fatalf("Number = %d, want 3", v.Number)
	}
	if v.Result != "unchanged" {
		t.Fatalf("Result = %q, want 'unchanged'", v.Result)
	}
}

func TestProductionLoopReader_LatestMissingFileReturnsZeroValue(t *testing.T) {
	r := newProductionLoopReader(filepath.Join(t.TempDir(), "does-not-exist.jsonl"))
	v, err := r.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.Number != 0 || v.Mode != "" || v.Result != "" {
		t.Fatalf("missing-file Latest should return zero-value; got %+v", v)
	}
}

func TestProductionLoopReader_RangeFiltersInclusive(t *testing.T) {
	dir := t.TempDir()
	path := writeLoopFixture(t, dir,
		`{"cycle":1,"mode":"a"}`,
		`{"cycle":2,"mode":"b"}`,
		`{"cycle":3,"mode":"c"}`,
		`{"cycle":4,"mode":"d"}`,
	)
	r := newProductionLoopReader(path)
	got, err := r.Range(context.Background(), 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Number != 2 || got[1].Number != 3 {
		t.Fatalf("numbers = [%d, %d], want [2, 3]", got[0].Number, got[1].Number)
	}
}

func TestProductionLoopReader_IdleStreakCountsTrailing(t *testing.T) {
	dir := t.TempDir()
	path := writeLoopFixture(t, dir,
		`{"cycle":1,"result":"improved"}`,
		`{"cycle":2,"result":"improved"}`,
		`{"cycle":3,"result":"idle"}`,
		`{"cycle":4,"result":"unchanged"}`,
	)
	r := newProductionLoopReader(path)
	got, err := r.IdleStreak(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != 2 {
		t.Fatalf("IdleStreak = %d, want 2", got)
	}
}

func TestProductionLoopReader_MalformedLinesSkipped(t *testing.T) {
	dir := t.TempDir()
	path := writeLoopFixture(t, dir,
		`{"cycle":1,"result":"improved"}`,
		`this is not json`,
		`{"cycle":2,"result":"improved"}`,
		`{"broken json:`,
		`{"cycle":3,"result":"unchanged"}`,
	)
	r := newProductionLoopReader(path)
	v, err := r.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.Number != 3 {
		t.Fatalf("Latest after malformed-skip = %d, want 3", v.Number)
	}
}

func TestProductionLoopReader_EmptyPathSafe(t *testing.T) {
	r := newProductionLoopReader("")
	v, err := r.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v.Number != 0 {
		t.Fatalf("empty path Latest should be zero-value; got %+v", v)
	}
	got, err := r.Range(context.Background(), 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("empty path Range len = %d, want 0", len(got))
	}
	streak, err := r.IdleStreak(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if streak != 0 {
		t.Fatalf("empty path IdleStreak = %d, want 0", streak)
	}
}

// Cycle 161 (soc-ckc4): CycleEntry was widened to project StartedAt
// and Title fields after the cycle-157 narrowness post-mortem
// (docs/learnings/2026-05-13-bc-ports-narrowness-postmortem.md).
// This test pins the projection so a future field rename or accidental
// drop is caught.
func TestProductionLoopReader_ProjectsStartedAtAndTitle(t *testing.T) {
	dir := t.TempDir()
	path := writeLoopFixture(t, dir,
		`{"cycle":7,"mode":"cleanup","result":"improved","commit":"abc1234","milestone":"ms","started_at":"2026-05-13T07:00:00-04:00","title":"sweep dead code"}`,
	)
	r := newProductionLoopReader(path)
	got, err := r.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.StartedAt != "2026-05-13T07:00:00-04:00" {
		t.Errorf("StartedAt = %q, want 2026-05-13T07:00:00-04:00", got.StartedAt)
	}
	if got.Title != "sweep dead code" {
		t.Errorf("Title = %q, want \"sweep dead code\"", got.Title)
	}
	// Sanity: existing fields still project.
	if got.Number != 7 || got.Result != "improved" || got.Milestone != "ms" {
		t.Errorf("existing-field regression: %+v", got)
	}
}

func TestProductionLoopReader_HonorsContextCancellation(t *testing.T) {
	dir := t.TempDir()
	path := writeLoopFixture(t, dir, `{"cycle":1,"result":"improved"}`)
	r := newProductionLoopReader(path)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, fn := range []struct {
		name string
		call func() error
	}{
		{"Latest", func() error { _, err := r.Latest(ctx); return err }},
		{"Range", func() error { _, err := r.Range(ctx, 0, 10); return err }},
		{"IdleStreak", func() error { _, err := r.IdleStreak(ctx); return err }},
	} {
		err := fn.call()
		if err == nil {
			t.Fatalf("%s: expected cancellation error, got nil", fn.name)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%s error = %v, want context.Canceled", fn.name, err)
		}
	}
}
