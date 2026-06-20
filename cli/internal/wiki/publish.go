// This file implements the dry-run half of `ao wiki publish` — the membrane
// seam over the gold compiler (age-port-openkb-into-agentops-go-5qw.9, first
// slice). A publish candidate is the gold tree compiled fresh, its leak-scan,
// and a stable content digest. The verdict-gated REAL publish (a CONFIRMED
// verdict for the digest before anything goes public) is a separate slice with
// an open design fork — see bead age-xf9r.
package wiki

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// publishDigestEpoch is the fixed clock the publish compiler runs against. The
// gold render bakes a compile-time date into any entry lacking a `date` field
// (gold.go: timestamp defaults to now()), so running against the real clock
// would make the publish digest drift day-to-day. Pinning the clock makes the
// digest purely CONTENT-derived — the stable identity the verdict gate binds to.
var publishDigestEpoch = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// PublishCandidate is the result of compiling a dry-run publish: the
// deterministic content digest of the candidate gold tree, the compile stats,
// and the (temp) directory it was compiled into. The caller leak-scans OutDir
// and is responsible for removing it (see Cleanup).
type PublishCandidate struct {
	// Digest is the sha256 (hex) over the SORTED (relpath, bytes) of every
	// emitted file — stable across runs and machines, so the same corpus always
	// yields the same publish identity (the key the verdict gate will bind to).
	Digest string
	// Stats is the gold compiler's promotion/redaction report.
	Stats GoldStats
	// OutDir is the temp directory the candidate was compiled into.
	OutDir string
}

// Cleanup removes the candidate's temp output tree. Safe to call on a zero
// value (no-op when OutDir is empty).
func (c PublishCandidate) Cleanup() {
	if c.OutDir != "" {
		_ = os.RemoveAll(c.OutDir)
	}
}

// CompilePublishCandidate compiles the corpus at agentsDir into a fresh temp
// gold tree and returns its content digest + stats, WITHOUT touching the real
// .ao/wiki output. The caller scans OutDir for leaks (fail-closed) and calls
// Cleanup. confidenceFloor of 0 uses the compiler's default.
func CompilePublishCandidate(agentsDir string, confidenceFloor float64) (PublishCandidate, error) {
	tmp, err := os.MkdirTemp("", "ao-wiki-publish-*")
	if err != nil {
		return PublishCandidate{}, fmt.Errorf("create temp output: %w", err)
	}
	gc := &GoldCompiler{
		AgentsDir:       agentsDir,
		OutDir:          tmp,
		ConfidenceFloor: confidenceFloor,
		// Pin the clock so the digest is content-derived, not date-of-run.
		Now: func() time.Time { return publishDigestEpoch },
	}
	stats, err := gc.Compile(false)
	if err != nil {
		_ = os.RemoveAll(tmp)
		return PublishCandidate{}, fmt.Errorf("compile gold: %w", err)
	}
	digest, err := digestTree(tmp)
	if err != nil {
		_ = os.RemoveAll(tmp)
		return PublishCandidate{}, fmt.Errorf("digest gold tree: %w", err)
	}
	return PublishCandidate{Digest: digest, Stats: stats, OutDir: tmp}, nil
}

// digestTree returns a stable sha256 (hex) over the sorted (relpath, content)
// of every file under root. Directory order from the filesystem is not stable,
// so paths are sorted; a NUL separator after each blob prevents content/path
// boundary ambiguity.
func digestTree(root string) (string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)

	h := sha256.New()
	for _, p := range files {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return "", err
		}
		b, err := os.ReadFile(p) //nolint:gosec // compiler-owned temp tree
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\n", filepath.ToSlash(rel))
		_, _ = h.Write(b)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
