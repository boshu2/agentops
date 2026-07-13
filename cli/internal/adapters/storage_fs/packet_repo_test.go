package storage_fs

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/boshu2/agentops/cli/internal/domain/packet"
)

func mortemCompatibilityFixtureDir(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("MORTEM_COMPAT_FIXTURES_DIR"); root != "" {
		return root
	}
	return filepath.Join("..", "..", "..", "..", "tests", "fixtures", "mortem-compatibility")
}

func validPacket() packet.ExecutionPacket {
	return packet.ExecutionPacket{
		SchemaVersion:    2,
		Objective:        "persist the canonical execution packet",
		RunID:            "run-001",
		EpicID:           "EPIC-1",
		PlanPath:         ".agents/plans/x.md",
		ContractSurfaces: []string{"schemas/execution-packet.schema.json"},
		TrackerMode:      "beads",
		Complexity:       packet.ComplexityStandard,
		DefaultVerdict:   packet.ExecutionPacketVerdictFail,
		TestLevels: &packet.ExecutionPacketTestLevels{
			Required:    []packet.TestLevel{packet.L1, packet.L2},
			Recommended: []packet.TestLevel{packet.L3},
			Rationale:   "standard autonomous proof floor",
		},
		Source:      "discovery",
		GeneratedAt: "2026-05-12T00:00:00Z",
	}
}

func TestRepo_RoundTripPersistsAndLoads(t *testing.T) {
	tmp := t.TempDir()
	r := &Repo{Root: tmp}
	ctx := context.Background()
	runID := "run-001"
	p := validPacket()
	p.SchemaVersion = packet.CurrentExecutionPacketSchemaVersion

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

func TestRepo_LoadConsumesLegacyMortemReadbackFixtures(t *testing.T) {
	for _, fixture := range []string{"v1-old-only.json", "v2-old-only.json"} {
		t.Run(fixture, func(t *testing.T) {
			fragment, err := os.ReadFile(filepath.Join(mortemCompatibilityFixtureDir(t), "legacy-readback", fixture))
			if err != nil {
				t.Fatalf("read legacy readback fixture: %v", err)
			}
			var fixtureFields map[string]json.RawMessage
			if err := json.Unmarshal(fragment, &fixtureFields); err != nil {
				t.Fatalf("parse legacy readback fixture: %v", err)
			}

			base, err := json.Marshal(validPacket())
			if err != nil {
				t.Fatal(err)
			}
			var persisted map[string]json.RawMessage
			if err := json.Unmarshal(base, &persisted); err != nil {
				t.Fatal(err)
			}
			for key, value := range fixtureFields {
				if key != "required" {
					persisted[key] = value
				}
			}
			data, err := json.Marshal(persisted)
			if err != nil {
				t.Fatal(err)
			}

			root := t.TempDir()
			runID := "legacy-mortem-readback"
			archive := filepath.Join(root, ".agents", "rpi", "runs", runID, "execution-packet.json")
			if err := os.MkdirAll(filepath.Dir(archive), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(archive, data, 0o644); err != nil {
				t.Fatal(err)
			}
			loaded, err := (&Repo{Root: root}).Load(context.Background(), runID)
			if err != nil {
				t.Fatalf("production Repo.Load rejected %s bytes: %v", fixture, err)
			}
			if loaded.PreMortemVerdict != packet.ExecutionPacketVerdictPass {
				t.Fatalf("PreMortemVerdict = %q, want PASS from %s", loaded.PreMortemVerdict, fixture)
			}
		})
	}
}

func TestRepo_SaveRejectsInvalidPacket(t *testing.T) {
	tmp := t.TempDir()
	r := &Repo{Root: tmp}
	ctx := context.Background()

	bad := validPacket()
	// contract_surfaces is optional by design now (age-55qz.2 reconciled the schema
	// to the live emitter), so use a field that is still genuinely schema-invalid:
	bad.SchemaVersion = 0 // violates schema_version minimum:1

	err := r.Save(ctx, "run-bad", bad)
	if !errors.Is(err, packet.ErrSchemaViolation) {
		t.Fatalf("Save: got %v, want errors.Is(err, ErrSchemaViolation) == true", err)
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

func TestRepo_LoadRejectsRichSchemaViolation(t *testing.T) {
	tmp := t.TempDir()
	r := &Repo{Root: tmp}
	ctx := context.Background()
	runID := "run-invalid-rich"
	dir := filepath.Join(tmp, ".agents/rpi/runs", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "execution-packet.json")
	data := []byte(`{
		"schema_version": 2,
		"objective": "reject invalid persisted packet",
		"contract_surfaces": ["schemas/execution-packet.schema.json"],
		"tracker_mode": "beads",
		"default_verdict": "MAYBE"
	}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write packet: %v", err)
	}

	_, err := r.Load(ctx, runID)
	if !errors.Is(err, packet.ErrSchemaViolation) {
		t.Fatalf("Load: got %v, want errors.Is(err, ErrSchemaViolation)", err)
	}
}

func TestRepo_LoadResolvesMissingDefaultVerdictFailClosed(t *testing.T) {
	tmp := t.TempDir()
	r := &Repo{Root: tmp}
	ctx := context.Background()
	runID := "run-fail-closed"
	dir := filepath.Join(tmp, ".agents/rpi/runs", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "execution-packet.json")
	data := []byte(`{
		"schema_version": 2,
		"objective": "load path resolves absent default verdict",
		"contract_surfaces": ["schemas/execution-packet.schema.json"],
		"tracker_mode": "beads"
	}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write packet: %v", err)
	}

	loaded, err := r.Load(ctx, runID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := loaded.DefaultVerdict; got != packet.ExecutionPacketVerdictFail {
		t.Fatalf("loaded DefaultVerdict = %q, want fail-closed %q", got, packet.ExecutionPacketVerdictFail)
	}
}

func TestRepo_LoadMigratesLegacySlimPacket(t *testing.T) {
	tmp := t.TempDir()
	r := &Repo{Root: tmp}
	ctx := context.Background()
	runID := "run-legacy"
	dir := filepath.Join(tmp, ".agents/rpi/runs", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "execution-packet.json")
	data := []byte(`{
		"plan_path": ".agents/plans/legacy.md",
		"epic_id": "EPIC-LEGACY",
		"complexity": "standard",
		"test_levels": ["L1", "L2"],
		"ranked_packet_path": ".agents/rpi/ranked-packet.json",
		"provenance": {
			"created_at": "2026-05-12T00:00:00Z",
			"source": "discovery",
			"run_id": "run-legacy"
		}
	}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write packet: %v", err)
	}

	loaded, err := r.Load(ctx, runID)
	if err != nil {
		t.Fatalf("Load legacy slim packet: %v", err)
	}
	if loaded.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want migrated legacy schema_version 1", loaded.SchemaVersion)
	}
	if loaded.PlanPath != ".agents/plans/legacy.md" {
		t.Fatalf("PlanPath = %q, want legacy plan path", loaded.PlanPath)
	}
	if loaded.TestLevels == nil || !reflect.DeepEqual(loaded.TestLevels.Required, []packet.TestLevel{packet.L1, packet.L2}) {
		t.Fatalf("TestLevels = %#v, want migrated required legacy levels", loaded.TestLevels)
	}
	if loaded.DefaultVerdict != packet.ExecutionPacketVerdictFail {
		t.Fatalf("DefaultVerdict = %q, want fail-closed FAIL", loaded.DefaultVerdict)
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
