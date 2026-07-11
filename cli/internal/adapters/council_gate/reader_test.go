package council_gate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestReaderResolvesWorkingDirectoryAtReadTime(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for directory, body := range map[string]string{first: "first", second: "second"} {
		if err := os.WriteFile(filepath.Join(directory, "verdict.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	current := first
	reader := Reader{WorkingDirectory: func() (string, error) { return current, nil }}
	if body, err := reader.Read(context.Background(), "verdict.md", io.Reader(nil)); err != nil || body != "first" {
		t.Fatalf("first read body=%q err=%v", body, err)
	}
	current = second
	if body, err := reader.Read(context.Background(), "verdict.md", io.Reader(nil)); err != nil || body != "second" {
		t.Fatalf("second read body=%q err=%v", body, err)
	}
}
