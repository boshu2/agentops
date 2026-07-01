// practices: [wiki-knowledge-surface]
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/storage"
)

// writeJSONAtomic marshals value to indented JSON and writes it to path durably
// and atomically via storage.AtomicWriteFile — the canonical repo writer, which
// fsyncs the temp file and the parent directory so a crash cannot leave a partial
// or lost write, and a concurrent reader sees either the old or the complete new
// content. Replaces the earlier fsync-less temp-file + rename body extracted from
// the retired overnight curator (soc-2rtm0) for the KEEP forge queue path.
func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return storage.AtomicWriteFile(path, data, 0o644)
}

// curatorJob is a single Dream-curator worker-queue job written by the KEEP
// forge enqueue path. Extracted from the retired overnight curator (soc-2rtm0);
// the JSON shape is the on-disk contract the external curator worker consumes.
type curatorJob struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	CreatedAt  string            `json:"created_at"`
	MaxRetries int               `json:"max_retries"`
	Source     *curatorJobSource `json:"source,omitempty"`
}

// curatorJobSource identifies the session file a curator job ingests, with an
// optional chunk range. Extracted from the retired overnight curator (soc-2rtm0).
type curatorJobSource struct {
	Path       string `json:"path"`
	ChunkStart int    `json:"chunk_start"`
	ChunkEnd   int    `json:"chunk_end"`
}

// buildCuratorID returns a sortable, unique id of the form
// <UTC-timestamp>-<kind>-<8hex>. Extracted from the retired overnight curator
// (soc-2rtm0) for the KEEP forge queue path.
func buildCuratorID(kind string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	suffix := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s", time.Now().UTC().Format("20060102T150405Z"), strings.ReplaceAll(kind, ":", "-"), suffix)
}
