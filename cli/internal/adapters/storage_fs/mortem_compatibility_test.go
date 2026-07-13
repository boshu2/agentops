package storage_fs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/domain/packet"
)

func TestRepo_MortemWriterStaysLegacyV2AndDefaultReaderRoundTrips(t *testing.T) {
	root := t.TempDir()
	repo := &Repo{Root: root}
	p := validPacket()
	p.PreMortemVerdict = packet.ExecutionPacketVerdictPass
	p.Artifacts = &packet.ExecutionPacketArtifacts{PreMortemPath: "reports/legacy.md"}

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
	if err := json.Unmarshal(raw["schema_version"], &version); err != nil || version != 2 {
		t.Fatalf("stored schema_version = %d (err=%v), want 2 through S7", version, err)
	}
	if _, ok := raw["pre_mortem_verdict"]; !ok {
		t.Fatal("storage writer omitted legacy pre_mortem_verdict before S8")
	}
	if _, ok := raw["premortem_verdict"]; ok {
		t.Fatal("storage writer emitted v2 premortem_verdict before S8")
	}

	loaded, err := repo.Load(context.Background(), "mortem-v1-writer")
	if err != nil {
		t.Fatalf("Load must read ordinary schema-v2 legacy-key evidence by default: %v", err)
	}
	if loaded.PreMortemVerdict != packet.ExecutionPacketVerdictPass {
		t.Fatalf("loaded mortem verdict = %q, want PASS", loaded.PreMortemVerdict)
	}
}
