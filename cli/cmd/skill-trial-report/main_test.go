package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandRequiresExplicitInputsAndWritesJSON(t *testing.T) {
	var out, stderr bytes.Buffer
	if err := run(nil, &out, &stderr); err == nil {
		t.Fatal("missing input accepted")
	}
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"session_meta","payload":{"id":"native"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--session", path}, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"id": "native"`) || !strings.Contains(out.String(), `"usage": null`) {
		t.Fatalf("output: %s", out.String())
	}
}
