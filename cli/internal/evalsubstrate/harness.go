package evalsubstrate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type HarnessLockEntry struct {
	Path   string `json:"path,omitempty"`
	Target string `json:"target,omitempty"`
	SHA256 string `json:"sha256"`
	Role   string `json:"role,omitempty"`
}

type HarnessLock struct {
	SchemaVersion int                `json:"schema_version"`
	HarnessID     string             `json:"harness_id"`
	ContentHash   string             `json:"content_hash"`
	Files         []HarnessLockEntry `json:"files,omitempty"`
	Imports       []HarnessLockEntry `json:"imports,omitempty"`
	CapturedAt    string             `json:"captured_at"`
	CapturedBy    string             `json:"captured_by"`
}

// SnapshotHarness walks a Harness source dir, canonicalizes every file by
// suffix, computes the per-file sha256, then aggregates into a directory
// content_hash per §7.
func SnapshotHarness(srcDir, harnessID, capturedBy string) (*Harness, *HarnessLock, error) {
	srcDir = filepath.Clean(srcDir)
	info, err := os.Stat(srcDir)
	if err != nil {
		return nil, nil, fmt.Errorf("SnapshotHarness: stat: %w", err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("SnapshotHarness: %q is not a directory", srcDir)
	}

	var entries []HarnessLockEntry
	err = filepath.Walk(srcDir, func(path string, fi os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if fi.IsDir() {
			return nil
		}
		if filepath.Base(path) == "harness.lock.json" {
			return nil
		}
		rel, rerr := filepath.Rel(srcDir, path)
		if rerr != nil {
			return rerr
		}
		h, herr := ContentHashFile(path)
		if herr != nil {
			return fmt.Errorf("hash %q: %w", path, herr)
		}
		entries = append(entries, HarnessLockEntry{
			Path:   filepath.ToSlash(rel),
			SHA256: strings.TrimPrefix(h, "sha256:"),
			Role:   inferRole(rel),
		})
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("SnapshotHarness: walk: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	dirHash := aggregateDirHash(entries)

	canonicalSource, _ := CanonicalizePath(srcDir)
	h := &Harness{
		SchemaVersion: SchemaVersion,
		ID:            harnessID,
		ContentHash:   dirHash,
		Source: HarnessSource{
			Kind: "directory",
			Path: canonicalSource,
		},
		Files:      lockToFiles(entries),
		LockFile:   "./harness.lock.json",
		CapturedAt: timeNow().UTC().Format(time.RFC3339),
		CapturedBy: capturedBy,
	}
	lock := &HarnessLock{
		SchemaVersion: SchemaVersion,
		HarnessID:     harnessID,
		ContentHash:   dirHash,
		Files:         entries,
		CapturedAt:    h.CapturedAt,
		CapturedBy:    capturedBy,
	}
	return h, lock, nil
}

// VerifyHarnessLock recomputes the directory content_hash and compares it
// to the lock's recorded content_hash. Returns true on match — gate #8 fires
// when this returns false.
func VerifyHarnessLock(srcDir string, lock *HarnessLock) (bool, string, error) {
	var entries []HarnessLockEntry
	err := filepath.Walk(srcDir, func(path string, fi os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if fi.IsDir() {
			return nil
		}
		if filepath.Base(path) == "harness.lock.json" {
			return nil
		}
		rel, _ := filepath.Rel(srcDir, path)
		h, herr := ContentHashFile(path)
		if herr != nil {
			return herr
		}
		entries = append(entries, HarnessLockEntry{
			Path:   filepath.ToSlash(rel),
			SHA256: strings.TrimPrefix(h, "sha256:"),
		})
		return nil
	})
	if err != nil {
		return false, "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	got := aggregateDirHash(entries)
	return got == lock.ContentHash, got, nil
}

func WriteHarnessLock(srcDir string, lock *HarnessLock) error {
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return fmt.Errorf("WriteHarnessLock: marshal: %w", err)
	}
	data = append(data, '\n')
	return WriteAtomic(filepath.Join(srcDir, "harness.lock.json"), data)
}

func LoadHarnessLock(srcDir string) (*HarnessLock, error) {
	raw, err := os.ReadFile(filepath.Join(srcDir, "harness.lock.json"))
	if err != nil {
		return nil, fmt.Errorf("LoadHarnessLock: %w", err)
	}
	var lock HarnessLock
	if err := json.Unmarshal(raw, &lock); err != nil {
		return nil, fmt.Errorf("LoadHarnessLock: parse: %w", err)
	}
	return &lock, nil
}

func LoadHarnessYAML(path string) (*Harness, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("LoadHarnessYAML: %w", err)
	}
	var h Harness
	if err := yaml.Unmarshal(raw, &h); err != nil {
		return nil, fmt.Errorf("LoadHarnessYAML: parse: %w", err)
	}
	return &h, nil
}

func aggregateDirHash(entries []HarnessLockEntry) string {
	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e.Path))
		h.Write([]byte{0})
		h.Write([]byte(e.SHA256))
		h.Write([]byte{'\n'})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func lockToFiles(entries []HarnessLockEntry) []HarnessFile {
	out := make([]HarnessFile, 0, len(entries))
	for _, e := range entries {
		out = append(out, HarnessFile{
			Path:   e.Path,
			SHA256: e.SHA256,
			Role:   e.Role,
		})
	}
	return out
}

func inferRole(rel string) string {
	rel = filepath.ToSlash(rel)
	switch {
	case strings.HasSuffix(rel, "SKILL.md"):
		return "prompt"
	case strings.HasPrefix(rel, "references/"):
		return "reference"
	case strings.HasSuffix(rel, ".md"):
		return "doc"
	case strings.HasSuffix(rel, ".yaml") || strings.HasSuffix(rel, ".yml"):
		return "config"
	}
	return ""
}
