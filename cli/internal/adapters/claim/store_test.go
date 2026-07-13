package claim

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestEvidenceStoreHistoryIdempotenceAndDowngrade(t *testing.T) {
	root := t.TempDir()
	store := NewEvidenceStore(func() (string, error) { return root, nil })
	ctx := context.Background()
	pg2 := ports.EvidenceBinding{Claim: "AOP-X", Path: "p.md", Level: ports.EvidenceLevelPG2}
	if err := store.Bind(ctx, pg2); err != nil {
		t.Fatal(err)
	}
	if err := store.Bind(ctx, pg2); err != nil {
		t.Fatal(err)
	}
	pg3 := ports.EvidenceBinding{Claim: "AOP-X", Path: "p.md", Level: ports.EvidenceLevelPG3, AuthorID: "author", JudgeID: "judge"}
	if err := store.Bind(ctx, pg3); err != nil {
		t.Fatal(err)
	}
	if err := store.Bind(ctx, pg2); err == nil {
		t.Fatal("downgrade succeeded")
	}
	bindings, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 2 || bindings[0].Level != ports.EvidenceLevelPG3 || bindings[1].Level != ports.EvidenceLevelPG2 {
		t.Fatalf("physical newest-first history = %+v", bindings)
	}
}

func TestEvidenceStoreSkipsMalformedRowsAndUsesRepoRoot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".agents", "findings", "evidence-bindings.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-json\n{\"claim\":\"A\",\"path\":\"p\",\"level\":\"PG1\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewEvidenceStore(func() (string, error) { return root, nil })
	bindings, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 || bindings[0].Claim != "A" {
		t.Fatalf("bindings = %+v", bindings)
	}
}
