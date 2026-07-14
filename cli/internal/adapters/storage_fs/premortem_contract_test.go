package storage_fs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/domain/packet"
)

func TestRepo_PremortemContractPersistsOneCanonicalBinaryVerdict(t *testing.T) {
	root := t.TempDir()
	repo := &Repo{Root: root}
	p := validPacket()
	p.PremortemVerdict = packet.ExecutionPacketVerdictPass
	p.Artifacts = &packet.ExecutionPacketArtifacts{PremortemPath: ".agents/council/premortem.json"}

	if err := repo.Save(context.Background(), "premortem-contract", p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	path := filepath.Join(root, ".agents", "rpi", "runs", "premortem-contract", "execution-packet.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["premortem_verdict"]; !ok {
		t.Fatal("storage writer omitted canonical premortem_verdict")
	}
	if _, ok := raw["pre_mortem_verdict"]; ok {
		t.Fatal("storage writer emitted removed pre_mortem_verdict")
	}
	var artifacts map[string]json.RawMessage
	if err := json.Unmarshal(raw["artifacts"], &artifacts); err != nil {
		t.Fatal(err)
	}
	if _, ok := artifacts["premortem_path"]; !ok {
		t.Fatal("storage writer omitted canonical artifacts.premortem_path")
	}
	if _, ok := artifacts["pre_mortem_path"]; ok {
		t.Fatal("storage writer emitted removed artifacts.pre_mortem_path")
	}

	loaded, err := repo.Load(context.Background(), "premortem-contract")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.PremortemVerdict != packet.ExecutionPacketVerdictPass {
		t.Fatalf("loaded verdict = %q, want PASS", loaded.PremortemVerdict)
	}
	if loaded.Artifacts == nil || loaded.Artifacts.PremortemPath != ".agents/council/premortem.json" {
		t.Fatalf("loaded artifacts = %+v", loaded.Artifacts)
	}
}
