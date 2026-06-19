package rpi

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyNextWorkCompletionProof_ExecutionPacketPath(t *testing.T) {
	cwd := t.TempDir()
	packetPath := filepath.Join(cwd, "packet.json")
	if err := os.WriteFile(packetPath, []byte(`{"objective":"do the thing"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	item := NextWorkItem{
		Title: "do the thing",
		ProofRef: &NextWorkProofRef{
			Kind: "execution_packet",
			Path: "packet.json", // relative — resolved against cwd
		},
	}
	got := ClassifyNextWorkCompletionProof(cwd, "", item)
	if !got.Complete || got.Source != "execution_packet" {
		t.Fatalf("expected complete execution_packet proof, got %+v", got)
	}
}

func TestClassifyNextWorkCompletionProof_ExecutionPacketMissingFileNotComplete(t *testing.T) {
	cwd := t.TempDir()
	item := NextWorkItem{
		Title: "no packet on disk",
		ProofRef: &NextWorkProofRef{
			Kind: "execution_packet",
			Path: "absent.json",
		},
	}
	if got := ClassifyNextWorkCompletionProof(cwd, "", item); got.Complete {
		t.Fatalf("a missing packet with no registry run must not be complete, got %+v", got)
	}
}

func TestClassifyNextWorkCompletionProof_NoProofRefNoRunsNotComplete(t *testing.T) {
	cwd := t.TempDir()
	item := NextWorkItem{Title: "unstarted"}
	if got := ClassifyNextWorkCompletionProof(cwd, "age-none", item); got.Complete {
		t.Fatalf("an item with no proof and no matching run must not be complete, got %+v", got)
	}
}
