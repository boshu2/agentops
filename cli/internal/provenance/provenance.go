// Package provenance tracks the lineage of olympus artifacts.
// It enables tracing from any artifact back to its source transcript.
package provenance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// absPathFunc is a variable so tests can override it.
var absPathFunc = filepath.Abs

// Record represents a single provenance entry.
// It links an artifact to its source.
type Record struct {
	// ID is the unique record identifier.
	ID string `json:"id"`

	// ArtifactPath is the file that was produced.
	ArtifactPath string `json:"artifact_path"`

	// WorkspacePath is the workspace root that owns the artifact.
	WorkspacePath string `json:"workspace_path,omitempty"`

	// ArtifactType classifies the output (session, index, etc).
	ArtifactType string `json:"artifact_type"`

	// SourcePath is the input file.
	SourcePath string `json:"source_path"`

	// SourceType classifies the input (transcript).
	SourceType string `json:"source_type"`

	// SessionID links to the conversation.
	SessionID string `json:"session_id,omitempty"`

	// CreatedAt is when the record was created.
	CreatedAt time.Time `json:"created_at"`

	// Metadata holds additional provenance data.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Graph manages provenance records and enables querying.
type Graph struct {
	// Path is the location of the provenance JSONL file.
	Path string

	// Records are loaded records for querying.
	Records []Record
}

// NewGraph creates a graph from a provenance file.
func NewGraph(path string) (*Graph, error) {
	g := &Graph{Path: path}
	if err := g.load(); err != nil {
		return nil, err
	}
	return g, nil
}

// load reads all records from the provenance file.
func (g *Graph) load() error {
	f, err := os.Open(g.Path)
	if os.IsNotExist(err) {
		g.Records = nil
		return nil
	}
	if err != nil {
		return fmt.Errorf("open provenance file: %w", err)
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // read-only, errors non-critical
	}()

	g.Records = nil
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue // Skip malformed lines
		}
		g.Records = append(g.Records, record)
	}

	return scanner.Err()
}

// TraceResult contains the provenance chain for an artifact.
type TraceResult struct {
	// Artifact is the path being traced.
	Artifact string `json:"artifact"`

	// Chain is the provenance path from artifact to source.
	Chain []Record `json:"chain"`

	// Sources are the original transcript sources.
	Sources []string `json:"sources"`
}

// Trace finds the provenance chain for an artifact.
func (g *Graph) Trace(artifactPath string) (*TraceResult, error) {
	result := &TraceResult{
		Artifact: artifactPath,
		Chain:    make([]Record, 0),
		Sources:  make([]string, 0),
	}

	absPath, err := absPathFunc(artifactPath)
	if err != nil {
		absPath = artifactPath
	}

	g.matchByAbsPath(absPath, artifactPath, result)
	if len(result.Chain) == 0 {
		g.matchByBasename(filepath.Base(artifactPath), result)
	}

	return result, nil
}

// matchByAbsPath finds records matching the full or absolute artifact path.
func (g *Graph) matchByAbsPath(absPath, artifactPath string, result *TraceResult) {
	for _, record := range g.Records {
		recordAbs, _ := filepath.Abs(record.ArtifactPath)
		if recordAbs == absPath || record.ArtifactPath == artifactPath {
			appendTraceRecord(result, record)
		}
	}
}

// matchByBasename finds records matching only the filename component.
func (g *Graph) matchByBasename(baseName string, result *TraceResult) {
	for _, record := range g.Records {
		if filepath.Base(record.ArtifactPath) == baseName {
			appendTraceRecord(result, record)
		}
	}
}

// appendTraceRecord adds a record to the trace result and tracks transcript sources.
func appendTraceRecord(result *TraceResult, record Record) {
	result.Chain = append(result.Chain, record)
	if record.SourceType == "transcript" {
		result.Sources = append(result.Sources, record.SourcePath)
	}
}

// FindBySession finds all provenance records for a session ID.
func (g *Graph) FindBySession(sessionID string) []Record {
	var results []Record
	for _, record := range g.Records {
		if record.SessionID == sessionID {
			results = append(results, record)
		}
	}
	return results
}

// FindBySource finds all artifacts derived from a source.
func (g *Graph) FindBySource(sourcePath string) []Record {
	var results []Record
	absSource, _ := filepath.Abs(sourcePath)

	for _, record := range g.Records {
		recordSource, _ := filepath.Abs(record.SourcePath)
		if recordSource == absSource || record.SourcePath == sourcePath {
			results = append(results, record)
		}
	}
	return results
}

// Stats returns statistics about the provenance graph.
type Stats struct {
	TotalRecords     int            `json:"total_records"`
	ArtifactTypes    map[string]int `json:"artifact_types"`
	SourceTypes      map[string]int `json:"source_types"`
	UniqueSessions   int            `json:"unique_sessions"`
	UniqueWorkspaces int            `json:"unique_workspaces"`
}

// GetStats returns statistics about the graph.
func (g *Graph) GetStats() *Stats {
	stats := &Stats{
		TotalRecords:  len(g.Records),
		ArtifactTypes: make(map[string]int),
		SourceTypes:   make(map[string]int),
	}

	sessions := make(map[string]bool)
	workspaces := make(map[string]bool)
	for _, record := range g.Records {
		stats.ArtifactTypes[record.ArtifactType]++
		stats.SourceTypes[record.SourceType]++
		if record.SessionID != "" {
			sessions[record.SessionID] = true
		}
		if record.WorkspacePath != "" {
			workspaces[record.WorkspacePath] = true
		}
	}

	stats.UniqueSessions = len(sessions)
	stats.UniqueWorkspaces = len(workspaces)
	return stats
}
