// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("boom")
}

func TestContextArtifactDir_WithRunID(t *testing.T) {
	got := contextArtifactDir("run-abc123", nil)
	want := filepath.Join(".agents", "context", "run-abc123")
	if got != want {
		t.Errorf("contextArtifactDir(\"run-abc123\") = %q, want %q", got, want)
	}
}

func TestContextArtifactDir_Empty(t *testing.T) {
	got := contextArtifactDir("", nil)
	prefix := filepath.Join(".agents", "context", "adhoc-")
	if !strings.HasPrefix(got, prefix) {
		t.Errorf("contextArtifactDir(\"\") = %q, want prefix %q", got, prefix)
	}
	// Verify the suffix after "adhoc-" matches <timestamp>-<4hex>
	suffix := strings.TrimPrefix(got, prefix)
	if suffix == "" {
		t.Errorf("contextArtifactDir(\"\") suffix is empty, expected timestamp-hex")
	}
	parts := strings.SplitN(suffix, "-", 2)
	if len(parts) != 2 {
		t.Errorf("contextArtifactDir(\"\") suffix %q expected format <timestamp>-<hex>, got %d parts", suffix, len(parts))
	} else {
		for _, c := range parts[0] {
			if c < '0' || c > '9' {
				t.Errorf("contextArtifactDir(\"\") timestamp part %q contains non-numeric character %q", parts[0], string(c))
				break
			}
		}
		if len(parts[1]) != 4 {
			t.Errorf("contextArtifactDir(\"\") hex suffix %q expected 4 characters", parts[1])
		}
	}
}

func TestNewAdhocContextRunID_UsesCryptoSuffix(t *testing.T) {
	got := newAdhocContextRunID(time.Unix(1234, 0), strings.NewReader("\xab\xcd"))
	if got != "adhoc-1234-abcd" {
		t.Fatalf("newAdhocContextRunID() = %q, want %q", got, "adhoc-1234-abcd")
	}
}

func TestNewAdhocContextRunID_FallsBackToTimeBits(t *testing.T) {
	now := time.Unix(1234, 0).Add(0x1234)
	got := newAdhocContextRunID(now, errReader{})
	want := "adhoc-1234-c634"
	if got != want {
		t.Fatalf("newAdhocContextRunID() fallback = %q, want %q", got, want)
	}
}

// TestEnsureContextDir_IdempotentOnAdhocCollision pins the documented
// collision behavior: when two adhoc IDs share the same timestamp AND
// the same 16-bit random suffix (~1/65 536 within the same second),
// the second ensureContextDir call MUST reuse the directory rather than
// erroring. This is the behavior that lets concurrent adhoc-id callers
// stay safe without coordination.
func TestEnsureContextDir_IdempotentOnAdhocCollision(t *testing.T) {
	tmpDir := t.TempDir()

	first, err := ensureContextDir(tmpDir, "adhoc-1234-abcd", nil)
	if err != nil {
		t.Fatalf("first ensureContextDir error: %v", err)
	}
	// Drop a marker so we can prove the second call reused the same dir.
	marker := filepath.Join(first, "marker.txt")
	if err := os.WriteFile(marker, []byte("from-first"), 0644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	second, err := ensureContextDir(tmpDir, "adhoc-1234-abcd", nil)
	if err != nil {
		t.Fatalf("collision ensureContextDir error: %v", err)
	}
	if first != second {
		t.Errorf("collision returned different paths: first=%q second=%q", first, second)
	}
	got, err := os.ReadFile(filepath.Join(second, "marker.txt"))
	if err != nil {
		t.Fatalf("read marker after collision: %v", err)
	}
	if string(got) != "from-first" {
		t.Errorf("marker = %q, want %q (second call clobbered the dir instead of reusing it)", got, "from-first")
	}
}

// TestNewAdhocContextRunID_DistinctSuffixesInSameSecond pins that two adhoc
// IDs minted in the same second with different entropy reads produce
// different run IDs. This is the protection that makes 1-second timestamp
// granularity acceptable in practice.
func TestNewAdhocContextRunID_DistinctSuffixesInSameSecond(t *testing.T) {
	now := time.Unix(2000, 0)
	a := newAdhocContextRunID(now, strings.NewReader("\x00\x01"))
	b := newAdhocContextRunID(now, strings.NewReader("\xff\xfe"))
	if a == b {
		t.Fatalf("expected different IDs, got %q == %q", a, b)
	}
	if a != "adhoc-2000-0001" || b != "adhoc-2000-fffe" {
		t.Errorf("ids = (%q, %q), want (adhoc-2000-0001, adhoc-2000-fffe)", a, b)
	}
}

func TestEnsureContextDir_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	got, err := ensureContextDir(tmpDir, "test-run", nil)
	if err != nil {
		t.Fatalf("ensureContextDir(%q, \"test-run\") error: %v", tmpDir, err)
	}
	wantSuffix := filepath.Join(".agents", "context", "test-run")
	if !strings.HasSuffix(got, wantSuffix) {
		t.Errorf("ensureContextDir returned %q, want suffix %q", got, wantSuffix)
	}
	info, err := os.Stat(got)
	if err != nil {
		t.Fatalf("os.Stat(%q) error: %v", got, err)
	}
	if !info.IsDir() {
		t.Errorf("%q is not a directory", got)
	}
}
