package persistence

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEvalCancellationPreservesRecoverablePayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.json")
	old := []byte(`{"offset":42,"pending":["job-a","job-b"]}`)
	if err := (Store{}).Save(path, old); err != nil {
		t.Fatal(err)
	}
	stop := errors.New("checkpoint cancelled")
	calls := 0
	store := Store{BeforeCommit: func() error {
		calls++
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, old) {
			t.Fatalf("old checkpoint changed before commit: %q, %v", got, err)
		}
		return stop
	}}
	if err := store.Save(path, []byte(`{"offset":43,"pending":[]}`)); !errors.Is(err, stop) {
		t.Fatalf("cancellation error = %v", err)
	}
	got, err := Load(path)
	if err != nil || !bytes.Equal(got, old) || calls != 1 {
		t.Fatalf("restart after cancellation = %q, %v, hook calls=%d", got, err, calls)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("cancel left temporary state: %v, %v", entries, err)
	}
	if err := (Store{}).Save(path, []byte("replacement\x00payload")); err != nil {
		t.Fatal(err)
	}
	got, err = Load(path)
	if err != nil || string(got) != "replacement\x00payload" {
		t.Fatalf("committed payload missing after reopen: %q, %v", got, err)
	}
}

func TestEvalCancellationDoesNotCreateCheckpoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.json")
	stop := errors.New("stop")
	err := (Store{BeforeCommit: func() error { return stop }}).Save(path, []byte("new"))
	if !errors.Is(err, stop) {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled new checkpoint exists: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cancelled creation leaked state: %v %v", entries, err)
	}
}
