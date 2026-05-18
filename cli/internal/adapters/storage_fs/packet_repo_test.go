package storage_fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/boshu2/agentops/cli/internal/domain/packet"
)

func validPacket() packet.ExecutionPacket {
	return packet.ExecutionPacket{
		PlanPath:   ".agents/plans/x.md",
		EpicID:     "EPIC-1",
		Complexity: packet.ComplexityStandard,
		TestLevels: []packet.TestLevel{packet.L1, packet.L2},
		Provenance: packet.Provenance{
			CreatedAt: "2026-05-12T00:00:00Z",
			Source:    "discovery",
			RunID:     "run-001",
		},
	}
}

func TestRepo_RoundTripPersistsAndLoads(t *testing.T) {
	tmp := t.TempDir()
	r := &Repo{Root: tmp}
	ctx := context.Background()
	runID := "run-001"
	p := validPacket()

	if err := r.Save(ctx, runID, p); err != nil {
		t.Fatalf("Save returned unexpected error: %v", err)
	}

	loaded, err := r.Load(ctx, runID)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if !reflect.DeepEqual(loaded, p) {
		t.Fatalf("Load: got %+v, want %+v", loaded, p)
	}

	latest, err := r.LoadLatest(ctx)
	if err != nil {
		t.Fatalf("LoadLatest returned unexpected error: %v", err)
	}
	if !reflect.DeepEqual(latest, p) {
		t.Fatalf("LoadLatest: got %+v, want %+v", latest, p)
	}
}

func TestRepo_SaveRejectsInvalidPacket(t *testing.T) {
	tmp := t.TempDir()
	r := &Repo{Root: tmp}
	ctx := context.Background()

	bad := validPacket()
	bad.PlanPath = "" // violates I1

	err := r.Save(ctx, "run-bad", bad)
	if !errors.Is(err, packet.ErrPlanPathEmpty) {
		t.Fatalf("Save: got %v, want errors.Is(err, ErrPlanPathEmpty) == true", err)
	}

	// Verify no files were written.
	latestPath := filepath.Join(tmp, ".agents/rpi/execution-packet.json")
	if _, statErr := os.Stat(latestPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected latest file to not exist, stat err = %v", statErr)
	}

	archivePath := filepath.Join(tmp, ".agents/rpi/runs/run-bad/execution-packet.json")
	if _, statErr := os.Stat(archivePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected archive file to not exist, stat err = %v", statErr)
	}
}

func TestRepo_LoadLatestReturnsErrNotExistWhenAbsent(t *testing.T) {
	tmp := t.TempDir()
	r := &Repo{Root: tmp}
	ctx := context.Background()

	p, err := r.LoadLatest(ctx)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadLatest: got err=%v, want errors.Is(err, os.ErrNotExist) == true", err)
	}
	if !reflect.DeepEqual(p, packet.ExecutionPacket{}) {
		t.Fatalf("LoadLatest: got packet=%+v, want zero packet", p)
	}
}

func TestRepo_LoadByRunIDReturnsErrNotExistWhenAbsent(t *testing.T) {
	tmp := t.TempDir()
	r := &Repo{Root: tmp}
	ctx := context.Background()

	_, err := r.Load(ctx, "missing-run")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load: got err=%v, want errors.Is(err, os.ErrNotExist) == true", err)
	}
}

func TestRepo_SaveRejectsUnsafeRunID(t *testing.T) {
	// Defense-in-depth (soc-odp0): runID flows directly into filepath.Join.
	// Any path-traversal token must be rejected before any filesystem write.
	cases := []struct {
		name  string
		runID string
	}{
		{"empty", ""},
		{"dot-dot", ".."},
		{"dot-dot-traversal", "../escape"},
		{"forward-slash", "run/sub"},
		{"backslash", "run\\sub"},
		{"absolute-unix", "/etc/passwd"},
		{"absolute-windows", "C:\\windows\\system32"},
		{"leading-dot", ".hidden"},
		{"nested-dot-dot", "ok/../escape"},
		{"nul-byte", "run\x00id"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			r := &Repo{Root: tmp}
			ctx := context.Background()

			err := r.Save(ctx, tc.runID, validPacket())
			if !errors.Is(err, ErrInvalidRunID) {
				t.Fatalf("Save(%q): got err=%v, want errors.Is(err, ErrInvalidRunID)", tc.runID, err)
			}

			// Confirm no file landed outside tmp.
			latestPath := filepath.Join(tmp, ".agents/rpi/execution-packet.json")
			if _, statErr := os.Stat(latestPath); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("expected no latest file written, stat err = %v", statErr)
			}
		})
	}
}

func TestRepo_LoadRejectsUnsafeRunID(t *testing.T) {
	tmp := t.TempDir()
	r := &Repo{Root: tmp}
	ctx := context.Background()

	_, err := r.Load(ctx, "../escape")
	if !errors.Is(err, ErrInvalidRunID) {
		t.Fatalf("Load: got err=%v, want errors.Is(err, ErrInvalidRunID)", err)
	}
}

func TestRepo_SaveIsAtomicForLatestPointer(t *testing.T) {
	// soc-odp0 item 6: the latest pointer must never reference a packet whose
	// archive doesn't exist. Verify by inspecting the *.tmp absence after Save
	// and confirming no half-written latest is left if archive write would
	// fail (we cannot easily simulate disk-full here, so we assert the
	// no-tmp-leftover invariant, which is the observable atomic-write
	// contract).
	tmp := t.TempDir()
	r := &Repo{Root: tmp}
	ctx := context.Background()
	runID := "run-atomic"

	if err := r.Save(ctx, runID, validPacket()); err != nil {
		t.Fatalf("Save unexpected err: %v", err)
	}

	latestTmp := filepath.Join(tmp, ".agents/rpi/execution-packet.json.tmp")
	if _, statErr := os.Stat(latestTmp); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected no temp file at %s after Save; got stat err=%v", latestTmp, statErr)
	}

	archiveTmp := filepath.Join(tmp, ".agents/rpi/runs", runID, "execution-packet.json.tmp")
	if _, statErr := os.Stat(archiveTmp); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected no temp file at %s after Save; got stat err=%v", archiveTmp, statErr)
	}
}

func TestRepo_SaveWritesArchiveBeforeLatest(t *testing.T) {
	// soc-odp0 item 6: archive must exist for any runID referenced by latest.
	// After a successful Save, both files must be present and identical.
	tmp := t.TempDir()
	r := &Repo{Root: tmp}
	ctx := context.Background()
	runID := "run-archive-first"
	p := validPacket()

	if err := r.Save(ctx, runID, p); err != nil {
		t.Fatalf("Save unexpected err: %v", err)
	}

	archivePath := filepath.Join(tmp, ".agents/rpi/runs", runID, "execution-packet.json")
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("expected archive at %s; stat err=%v", archivePath, err)
	}

	latestPath := filepath.Join(tmp, ".agents/rpi/execution-packet.json")
	if _, err := os.Stat(latestPath); err != nil {
		t.Fatalf("expected latest at %s; stat err=%v", latestPath, err)
	}

	// Byte-equality: both files serialize the same packet.
	archiveBytes, _ := os.ReadFile(archivePath)
	latestBytes, _ := os.ReadFile(latestPath)
	if !reflect.DeepEqual(archiveBytes, latestBytes) {
		t.Fatalf("archive and latest content differ; atomic Save invariant violated")
	}
}
