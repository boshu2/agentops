package okfprofile

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckFileBoundedAndReadOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "observation.md")
	payload := []byte(observation)
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	result, err := CheckFile(path, Profile)
	if err != nil || !result.StructurallyValid {
		t.Fatalf("check: %+v %v", result, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(payload, after) {
		t.Fatal("checker altered input")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("checker created output: %v %v", entries, err)
	}

	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckFile(link, Profile); err == nil {
		t.Fatal("file symlink accepted")
	}
	if _, err := CheckFile(filepath.Join(dir, "missing.md"), "future/2"); err == nil || !strings.Contains(err.Error(), "unsupported profile") {
		t.Fatalf("version not checked before input: %v", err)
	}

	for _, name := range []string{"index.md", "log.md", "page.txt"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, payload, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := CheckFile(path, Profile); err == nil {
			t.Fatalf("non-concept %s accepted", name)
		}
	}
	if err := os.Truncate(path, MaxPageBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckFile(path, Profile); err == nil || !strings.Contains(err.Error(), "read bound") {
		t.Fatalf("oversized input: %v", err)
	}
}

func TestCheckFileErrorDoesNotEchoLocator(t *testing.T) {
	_, err := CheckFile(filepath.Join(t.TempDir(), "private-owner-canary", "page.md"), Profile)
	if err == nil || strings.Contains(err.Error(), "private-owner-canary") {
		t.Fatalf("locator echoed: %v", err)
	}
}
