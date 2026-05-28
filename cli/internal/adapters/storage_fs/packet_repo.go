// Package storage_fs is a filesystem adapter for the PacketRepository port.
// Storage layout: <root>/.agents/rpi/execution-packet.json (latest)
//
//	<root>/.agents/rpi/runs/<runID>/execution-packet.json (archive)
package storage_fs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/domain/packet"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// ErrInvalidRunID is returned when a runID contains path-traversal tokens or
// is otherwise unsafe to interpolate into a filesystem path. Callers should
// surface it directly; do not wrap and hide.
var ErrInvalidRunID = errors.New("storage_fs: invalid runID")

type Repo struct{ Root string }

// Compile-time interface check.
var _ ports.PacketRepository = (*Repo)(nil)

// validateRunID rejects runIDs that would let a caller escape the per-run
// archive directory under r.Root/.agents/rpi/runs/. Defense-in-depth: today's
// v1 callers are all domain-side, but the trust boundary moves outward over
// time and a missing check here would let a future caller write outside the
// repo. Rule set:
//   - non-empty
//   - no path separators ("/", "\")
//   - no parent-directory traversal (".." anywhere as a path segment or substring)
//   - no leading dot (avoids hidden-dir confusion and edge cases like "." itself)
//   - no NUL byte (defensive against unexpected input encodings)
func validateRunID(runID string) error {
	if runID == "" {
		return fmt.Errorf("%w: empty", ErrInvalidRunID)
	}
	if strings.ContainsAny(runID, "/\\") {
		return fmt.Errorf("%w: contains path separator: %q", ErrInvalidRunID, runID)
	}
	if strings.Contains(runID, "..") {
		return fmt.Errorf("%w: contains parent-directory token: %q", ErrInvalidRunID, runID)
	}
	if strings.HasPrefix(runID, ".") {
		return fmt.Errorf("%w: starts with '.': %q", ErrInvalidRunID, runID)
	}
	if strings.ContainsRune(runID, 0) {
		return fmt.Errorf("%w: contains NUL byte", ErrInvalidRunID)
	}
	return nil
}

func (r *Repo) Save(_ context.Context, runID string, p packet.ExecutionPacket) error {
	if err := validateRunID(runID); err != nil {
		return err
	}
	if err := p.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(r.Root, ".agents/rpi/runs", runID), 0o755); err != nil {
		return err
	}
	// Write the per-run archive first so 'latest' never advertises a packet
	// without a matching archive copy. Both writes are atomic (write-temp +
	// rename) so partial-file states are not observable.
	archivePath := filepath.Join(r.Root, ".agents/rpi/runs", runID, "execution-packet.json")
	if err := writeJSONAtomic(archivePath, p); err != nil {
		return err
	}
	return writeJSONAtomic(filepath.Join(r.Root, ".agents/rpi/execution-packet.json"), p)
}

func (r *Repo) Load(_ context.Context, runID string) (packet.ExecutionPacket, error) {
	if err := validateRunID(runID); err != nil {
		return packet.ExecutionPacket{}, err
	}
	return readJSON(filepath.Join(r.Root, ".agents/rpi/runs", runID, "execution-packet.json"))
}

func (r *Repo) LoadLatest(_ context.Context) (packet.ExecutionPacket, error) {
	p, err := readJSON(filepath.Join(r.Root, ".agents/rpi/execution-packet.json"))
	if errors.Is(err, os.ErrNotExist) {
		return packet.ExecutionPacket{}, err
	}
	return p, err
}

// writeJSONAtomic serializes p and replaces the file at path atomically:
// write to <path>.tmp in the same directory, then os.Rename onto path.
// os.Rename is atomic on POSIX filesystems for same-directory targets.
// On failure, the temp file is removed; the destination is left intact.
func writeJSONAtomic(path string, p packet.ExecutionPacket) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		// Best-effort cleanup; ignore secondary error.
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func readJSON(path string) (packet.ExecutionPacket, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return packet.ExecutionPacket{}, err
	}
	var p packet.ExecutionPacket
	return p, json.Unmarshal(b, &p)
}
