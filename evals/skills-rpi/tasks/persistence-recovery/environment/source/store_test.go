package persistence

import (
	"path/filepath"
	"testing"
)

func TestSaveThenReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	if err := (Store{}).Save(path, []byte(`{"offset":4}`)); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || string(got) != `{"offset":4}` {
		t.Fatalf("reopened = %s, %v", got, err)
	}
}
