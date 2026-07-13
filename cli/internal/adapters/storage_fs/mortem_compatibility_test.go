package storage_fs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/domain/packet"
)

func TestRepo_MortemWriterPersistsCanonicalV3AndDefaultReaderRoundTrips(t *testing.T) {
	root := t.TempDir()
	repo := &Repo{Root: root}
	p := validPacket()
	p.PreMortemVerdict = packet.ExecutionPacketVerdictPass
	p.Artifacts = &packet.ExecutionPacketArtifacts{PreMortemPath: ".agents/premortem-checks/current.md"}

	if err := repo.Save(context.Background(), "mortem-v1-writer", p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := filepath.Join(root, ".agents", "rpi", "runs", "mortem-v1-writer", "execution-packet.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := json.Unmarshal(raw["schema_version"], &version); err != nil || version != 3 {
		t.Fatalf("stored schema_version = %d (err=%v), want canonical v3", version, err)
	}
	if _, ok := raw["premortem_verdict"]; !ok {
		t.Fatal("storage writer omitted canonical premortem_verdict")
	}
	if _, ok := raw["pre_mortem_verdict"]; ok {
		t.Fatal("storage writer emitted legacy pre_mortem_verdict")
	}
	var artifacts map[string]json.RawMessage
	if err := json.Unmarshal(raw["artifacts"], &artifacts); err != nil {
		t.Fatal(err)
	}
	if _, ok := artifacts["premortem_path"]; !ok {
		t.Fatal("storage writer omitted canonical artifacts.premortem_path")
	}
	if _, ok := artifacts["pre_mortem_path"]; ok {
		t.Fatal("storage writer emitted legacy artifacts.pre_mortem_path")
	}

	loaded, err := repo.Load(context.Background(), "mortem-v1-writer")
	if err != nil {
		t.Fatalf("Load must read canonical schema-v3 evidence by default: %v", err)
	}
	if loaded.PreMortemVerdict != packet.ExecutionPacketVerdictPass {
		t.Fatalf("loaded mortem verdict = %q, want PASS", loaded.PreMortemVerdict)
	}
}
