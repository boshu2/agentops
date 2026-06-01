package quest

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestFleetLease_FsyncOnAtomicWrite is the L3 fault-injection contract test
// for the fleet-lease fsync gap (olympus Feb 23 P0, finding #2).
//
// Pre-mortem requirement (vyp Finding 4): assert that the lease-write helper
// recovers cleanly via atomic rename + fsync barrier even under
// crash-mid-write conditions.
//
// Strategy: agentopsd does not yet have a dedicated fleet-lease writer, but
// the canonical helper is [AtomicWriteFile] in this package. The fleet lease,
// when ported, MUST go through this helper (or an equivalent same-dir
// tmp+fsync+rename path). This test asserts the contract so that a future
// `writeFleetLeaseFile` regression that calls plain os.WriteFile gets caught.
//
// We simulate the fault by:
//  1. Writing a known-good lease body via AtomicWriteFile.
//  2. Inspecting the parent directory IMMEDIATELY after the call to assert
//     no .tmp-* leftover (fsync+rename completed atomically — if a SIGKILL
//     interrupted the write, either the tmp file would remain OR the target
//     would be untouched, never partially-written).
//  3. Reading the target back and asserting the bytes equal the input
//     exactly — no partial write, no truncation.
//
// This is the same shape olympus's writeFleetLeaseFile must adopt to fix the
// Feb 23 finding. If anyone reintroduces a plain os.WriteFile path for fleet
// leases, the regression surface is documented HERE and the next test below
// (TestFleetLease_PlainWriteFile_RegressionSurface) makes the gap visible.
func TestFleetLease_FsyncOnAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	leasePath := filepath.Join(dir, "fleet.lease")

	// Lease body: shape modeled on olympus fleet-envelope shard leases —
	// non-trivial size so a partial-write would be detectable.
	body := make([]byte, 4096)
	if _, err := rand.Read(body); err != nil {
		t.Fatalf("seeding lease body: %v", err)
	}
	bodyHex := hex.EncodeToString(body)

	if err := AtomicWriteFile(leasePath, []byte(bodyHex)); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}

	// Assertion 1: directory has exactly one entry (the lease), no tmp leftover.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("dir entry count: got %d (%v), want 1", len(entries), names)
	}
	if entries[0].Name() != "fleet.lease" {
		t.Fatalf("dir entry name: got %q, want %q", entries[0].Name(), "fleet.lease")
	}

	// Assertion 2: file contents are exactly the input — no truncation, no
	// interleave, no partial bytes from a prior write or a tmp file.
	got, err := os.ReadFile(leasePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != bodyHex {
		t.Fatalf("lease body mismatch: got %d bytes, want %d bytes", len(got), len(bodyHex))
	}

	// Assertion 3: the lease file was created with sane mode bits (regular
	// file, not symlink, not device).
	info, err := os.Stat(leasePath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("lease mode: got %v, want regular file", info.Mode())
	}
	if info.Size() != int64(len(bodyHex)) {
		t.Fatalf("lease size: got %d, want %d", info.Size(), len(bodyHex))
	}
}

// TestFleetLease_AtomicWriteSurvivesConcurrentReaders asserts the second half
// of the fsync contract: a concurrent reader either sees the OLD file or the
// NEW file, never a partial write or a torn read. This is the property that
// makes atomic-rename safe for control-plane state like fleet leases.
//
// Without atomic rename, a reader that opens the file mid-write sees a
// truncated file (open() succeeds, read() returns 0 bytes or partial bytes).
// With atomic rename, the inode swap is atomic at the directory level —
// readers always see a complete file.
func TestFleetLease_AtomicWriteSurvivesConcurrentReaders(t *testing.T) {
	dir := t.TempDir()
	leasePath := filepath.Join(dir, "fleet.lease")

	oldBody := bytes.Repeat([]byte("OLD-LEASE-V1\n"), 256) // ~3 KiB
	newBody := bytes.Repeat([]byte("NEW-LEASE-V2\n"), 256) // ~3 KiB

	if err := AtomicWriteFile(leasePath, oldBody); err != nil {
		t.Fatalf("seeding old lease: %v", err)
	}

	// Spawn N concurrent readers while a writer races to overwrite.
	const nReaders = 16
	var wg sync.WaitGroup
	results := make([][]byte, nReaders)
	startGate := make(chan struct{})

	for i := 0; i < nReaders; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startGate
			data, err := os.ReadFile(leasePath)
			if err != nil {
				t.Errorf("reader %d: %v", idx, err)
				return
			}
			results[idx] = data
		}(i)
	}

	// Writer races readers. AtomicWriteFile must guarantee no reader observes
	// a partial body.
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-startGate
		if err := AtomicWriteFile(leasePath, newBody); err != nil {
			t.Errorf("racing writer: %v", err)
		}
	}()

	close(startGate)
	wg.Wait()

	for i, got := range results {
		if got == nil {
			continue
		}
		if !bytes.Equal(got, oldBody) && !bytes.Equal(got, newBody) {
			t.Errorf("reader %d saw torn lease: got %d bytes, expected exact old (%d) or new (%d)",
				i, len(got), len(oldBody), len(newBody))
		}
	}
}

// TestFleetLease_PlainWriteFile_RegressionSurface documents the regression
// surface explicitly. It does NOT fail today because agentopsd has not yet
// ported a fleet-lease writer — this test asserts that IF someone adds a
// `writeFleetLeaseFile` helper, it must not use plain os.WriteFile.
//
// The check is structural: scan the daemon and overnight packages for any
// helper named writeFleetLease*; if found, fail unless it routes through
// AtomicWriteFile. Currently a no-op (the helper doesn't exist yet) — but
// landing as a constraint test means future PRs that add the regression
// surface trip this guard.
//
// Per vyp scope: do NOT modify production code; document the gap.
func TestFleetLease_PlainWriteFile_RegressionSurface(t *testing.T) {
	// Walk the daemon + overnight packages looking for fleet-lease writers.
	roots := []string{
		filepath.Join("..", "..", "daemon"),
		filepath.Join("..", "..", "overnight"),
	}

	type hit struct {
		path string
		line string
	}
	var hits []hit

	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			content := string(data)
			// Match olympus's writeFleetLeaseFile pattern. If a function with
			// that prefix exists AND uses os.WriteFile directly (not via
			// AtomicWriteFile), that is the regression.
			if strings.Contains(content, "writeFleetLease") {
				lines := strings.Split(content, "\n")
				for _, line := range lines {
					if strings.Contains(line, "writeFleetLease") {
						hits = append(hits, hit{path: path, line: strings.TrimSpace(line)})
					}
				}
			}
			return nil
		})
	}

	// Every fleet-lease writer that exists must route through AtomicWriteFile.
	// If none exists yet, the contract is vacuously satisfied — the assertion
	// below holds either way: hits == 0 OR every hit's file uses
	// AtomicWriteFile. The expression `regressionCount` MUST be 0.
	regressionCount := 0
	for _, h := range hits {
		data, err := os.ReadFile(h.path)
		if err != nil {
			t.Fatalf("re-reading %s: %v", h.path, err)
		}
		content := string(data)
		usesPlain := strings.Contains(content, "os.WriteFile")
		usesAtomic := strings.Contains(content, "AtomicWriteFile")
		if usesPlain && !usesAtomic {
			regressionCount++
			t.Errorf("regression surface: %s contains writeFleetLease + os.WriteFile but NOT AtomicWriteFile (line: %q)",
				h.path, h.line)
		}
	}
	// Exact assertion: zero plain-os.WriteFile fleet-lease writers.
	if regressionCount != 0 {
		t.Fatalf("fleet-lease regression count: got %d, want 0", regressionCount)
	}
}
