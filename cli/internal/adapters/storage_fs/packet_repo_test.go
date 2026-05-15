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
