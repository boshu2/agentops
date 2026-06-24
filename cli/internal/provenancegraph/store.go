package provenancegraph

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Store reads and appends provenance edges to a JSONL ledger file. The ledger
// is the committed audit authority; Store performs append-only writes and
// never rewrites prior records.
type Store struct {
	// Path is the location of the JSONL ledger file.
	Path string
}

// NewStore returns a Store bound to path. The file need not exist yet; the
// first append creates it (and any missing parent directories).
func NewStore(path string) *Store {
	return &Store{Path: path}
}

// Read loads every non-blank line of the ledger as an Edge, in file order.
// A missing file is treated as an empty ledger (no error), so the genesis
// append works on a fresh clone. Malformed lines are a hard error so the audit
// authority never silently drops a corrupt record.
func (s *Store) Read() ([]Edge, error) {
	f, err := os.Open(s.Path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open ledger: %w", err)
	}
	defer func() { _ = f.Close() }()

	var edges []Edge
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Bytes()
		if len(trimSpace(raw)) == 0 {
			continue
		}
		var e Edge
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("ledger line %d: invalid JSON: %w", line, err)
		}
		edges = append(edges, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan ledger: %w", err)
	}
	return edges, nil
}

// trimSpace trims ASCII whitespace without allocating a string.
func trimSpace(b []byte) []byte {
	start := 0
	for start < len(b) && (b[start] == ' ' || b[start] == '\t' || b[start] == '\r' || b[start] == '\n') {
		start++
	}
	end := len(b)
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\r' || b[end-1] == '\n') {
		end--
	}
	return b[start:end]
}

// LastHash returns the hash of the final record in the ledger, or the empty
// string for an empty/absent ledger (the genesis prev_hash).
func (s *Store) LastHash() (string, error) {
	edges, err := s.Read()
	if err != nil {
		return "", err
	}
	if len(edges) == 0 {
		return "", nil
	}
	return edges[len(edges)-1].Hash, nil
}

// AppendResult reports the outcome of an Append.
type AppendResult struct {
	// Edge is the sealed edge as written (or the pre-existing duplicate when
	// Skipped is true).
	Edge Edge
	// Skipped is true when an identical edge already existed and the append
	// was idempotently a no-op.
	Skipped bool
}

// Append seals a new edge onto the current chain tip and appends it to the
// ledger as one JSON line. It is idempotent: if an edge with the same identity
// (EdgeIdentity) already exists, no write occurs and the existing edge is
// returned with Skipped=true. The schema_version is forced and the edge is
// validated before any write, so a malformed edge never reaches disk.
func (s *Store) Append(e Edge) (AppendResult, error) {
	var result AppendResult
	// The whole read-seal-write is one critical section under a cross-process
	// advisory lock: the seal links onto the CURRENT chain tip, so two
	// concurrent appenders that each read the same tip would both seal onto it
	// and FORK the hash chain. The lock serializes appenders across goroutines
	// AND separate `ao` processes (each opens its own lock fd; flock is keyed on
	// the open file description, so it is mutually exclusive either way).
	err := s.withAppendLock(func() error {
		existing, err := s.Read()
		if err != nil {
			return err
		}

		// Idempotency: skip if an identical edge already exists.
		identity := EdgeIdentity(Edge{
			SchemaVersion: SchemaVersion,
			FromID:        e.FromID, FromType: e.FromType,
			ToID: e.ToID, ToType: e.ToType,
			Relation: e.Relation, EvidenceRef: e.EvidenceRef, TrustTier: e.TrustTier,
		})
		for _, ex := range existing {
			if EdgeIdentity(ex) == identity {
				result = AppendResult{Edge: ex, Skipped: true}
				return nil
			}
		}

		prevHash := ""
		if len(existing) > 0 {
			prevHash = existing[len(existing)-1].Hash
		}

		sealed, err := Seal(e, prevHash)
		if err != nil {
			return err
		}

		if err := s.writeLine(sealed); err != nil {
			return err
		}
		result = AppendResult{Edge: sealed}
		return nil
	})
	if err != nil {
		return AppendResult{}, err
	}
	return result, nil
}

// withAppendLock runs fn while holding an exclusive cross-process advisory lock
// on a sidecar lock file beside the ledger. Acquiring the lock serializes the
// read-seal-write critical section so concurrent appends cannot fork the
// hash chain. The lock file is created on first append and is never written
// to (it is only a lock token); it is gitignored alongside the ledger.
func (s *Store) withAppendLock(fn func() error) (err error) {
	lockPath := s.Path + ".lock"
	if dir := filepath.Dir(lockPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create ledger dir: %w", err)
		}
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open ledger lock: %w", err)
	}
	locked := false
	// Deferred cleanup: unlock (if locked) then close. The first error is
	// propagated only when fn would otherwise return nil, so the caller always
	// sees the most significant error.
	defer func() {
		if locked {
			if unlockErr := unlockFile(f); unlockErr != nil && err == nil {
				err = fmt.Errorf("unlock ledger: %w", unlockErr)
			}
		}
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close ledger lock: %w", closeErr)
		}
	}()

	if err := lockFile(f); err != nil {
		return fmt.Errorf("lock ledger: %w", err)
	}
	locked = true

	return fn()
}

// writeLine appends one edge as a compact JSON line, creating parent dirs and
// the file on first write.
func (s *Store) writeLine(e Edge) error {
	if dir := filepath.Dir(s.Path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create ledger dir: %w", err)
		}
	}
	f, err := os.OpenFile(s.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open ledger for append: %w", err)
	}
	defer func() { _ = f.Close() }()

	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal edge: %w", err)
	}
	b = append(b, '\n')
	if _, err := f.Write(b); err != nil {
		return fmt.Errorf("write edge: %w", err)
	}
	return nil
}
