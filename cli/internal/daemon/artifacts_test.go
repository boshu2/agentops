package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArtifactRef(t *testing.T) {
	ref := ArtifactRef{
		Path:      ".agents/handoffs/sha256/aa/bb/" + strings.Repeat("a", 64),
		SHA256:    strings.Repeat("a", 64),
		Size:      12,
		WrittenAt: projectionTestTime(t, 0).Format(time.RFC3339Nano),
	}
	if err := ref.Validate(); err != nil {
		t.Fatalf("valid ref rejected: %v", err)
	}
	for name, bad := range map[string]ArtifactRef{
		"missing path": {SHA256: ref.SHA256, Size: 1, WrittenAt: ref.WrittenAt},
		"bad hash":     {Path: ref.Path, SHA256: "not-sha", Size: 1, WrittenAt: ref.WrittenAt},
		"bad size":     {Path: ref.Path, SHA256: ref.SHA256, Size: -1, WrittenAt: ref.WrittenAt},
		"bad time":     {Path: ref.Path, SHA256: ref.SHA256, Size: 1, WrittenAt: "yesterday"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := bad.Validate(); err == nil {
				t.Fatal("invalid ref accepted")
			}
		})
	}
}

func TestContentAddressedStore(t *testing.T) {
	root := t.TempDir()
	now := projectionTestTime(t, 3)
	store := NewContentAddressedArtifactStore(root, ArtifactStoreOptions{Now: func() time.Time { return now }})
	ref, err := store.PutBytes([]byte("content-addressed artifact\n"))
	if err != nil {
		t.Fatalf("PutBytes: %v", err)
	}
	if err := ref.Validate(); err != nil {
		t.Fatalf("ref invalid: %v", err)
	}
	if !strings.HasPrefix(ref.Path, ".agents/handoffs/sha256/") {
		t.Fatalf("path = %q, want content-addressed handoff path", ref.Path)
	}
	parts := strings.Split(ref.Path, "/")
	if len(parts) < 6 || parts[len(parts)-3] != ref.SHA256[:2] || parts[len(parts)-2] != ref.SHA256[2:4] || parts[len(parts)-1] != ref.SHA256 {
		t.Fatalf("path = %q, want aa/bb/full-sha layout for %s", ref.Path, ref.SHA256)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ref.Path)))
	if err != nil {
		t.Fatalf("read stored artifact: %v", err)
	}
	if string(data) != "content-addressed artifact\n" {
		t.Fatalf("stored data = %q", string(data))
	}
	again, err := store.PutBytes([]byte("content-addressed artifact\n"))
	if err != nil {
		t.Fatalf("PutBytes duplicate: %v", err)
	}
	if again.Path != ref.Path || again.SHA256 != ref.SHA256 || again.Size != ref.Size {
		t.Fatalf("duplicate ref = %#v, want same path/hash/size as %#v", again, ref)
	}
}
