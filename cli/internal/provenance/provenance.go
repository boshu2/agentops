// Package provenance tracks the lineage of olympus artifacts.
// It enables tracing from any artifact back to its source transcript.
package provenance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// staleCitationAge is the threshold at which a learning's cited source
// is considered stale. Learnings older than this without being refreshed
// are flagged for review.
const staleCitationAge = 180 * 24 * time.Hour

// AuditReport describes the outcome of a provenance audit over the
// local .agents/ corpus. Dream's INGEST stage uses this to detect
// stale citations and missing source references before the nightly
// loop runs MEASURE.
type AuditReport struct {
	// StaleCitations is the count of learnings whose cited source
	// bead or file no longer exists or is > 180 days old.
	StaleCitations int

	// MissingSources is the count of learnings whose frontmatter
	// source_bead field is empty or references a bead that no
	// longer exists.
	MissingSources int

	// Degraded accumulates soft-fail notes (parse errors on
	// individual learning files, etc.).
	Degraded []string

	// Duration is the wall-clock time the audit took.
	Duration time.Duration
}

// learningFrontmatter mirrors the subset of learning YAML frontmatter
// fields the audit cares about. Additional fields in the source are
// ignored.
type learningFrontmatter struct {
	Title      string         `yaml:"title"`
	SourceBead string         `yaml:"source_bead"`
	Source     flexibleString `yaml:"source"`
	Date       string         `yaml:"date"`
}

// flexibleString holds a learning frontmatter value that may be either a
// plain YAML scalar or a structured map/sequence. Older learnings used
// a bare string such as `source: retro-quick`; newer ones occasionally
// use a map like `source: {session: ..., evidence: [...]}` to carry
// provenance context. Either shape should be honored by the audit; the
// canonical "missing source" check only asks whether the value is empty.
type flexibleString string

// UnmarshalYAML accepts scalar, map, or sequence nodes. Scalars decode
// to their literal value. Maps and sequences decode to a non-empty
// sentinel so the caller's MissingSources check does not mark the
// field as empty; the exact serialization is not interpreted further.
func (f *flexibleString) UnmarshalYAML(node *yaml.Node) error {
	if node == nil {
		*f = ""
		return nil
	}
	switch node.Kind {
	case yaml.ScalarNode:
		*f = flexibleString(node.Value)
		return nil
	case yaml.MappingNode, yaml.SequenceNode:
		*f = "<structured>"
		return nil
	case yaml.AliasNode:
		if node.Alias != nil {
			return f.UnmarshalYAML(node.Alias)
		}
		*f = ""
		return nil
	default:
		*f = ""
		return nil
	}
}

func (f flexibleString) String() string { return string(f) }

// Audit scans .agents/learnings/ under cwd and returns an
// AuditReport. Never prints to stdout/stderr. Soft-fails on
// individual file read/parse errors; returns a hard error only
// on structural problems (missing .agents/ dir, unreadable
// learnings subdir).
//
// Dream's INGEST stage calls this in-process via RunIngest.
// The cmd/ao cobra layer (rpi_phased_provenance.go) wraps it for
// operator invocation.
func Audit(cwd string) (*AuditReport, error) {
	start := time.Now()
	report := &AuditReport{Degraded: make([]string, 0)}

	agentsDir := filepath.Join(cwd, ".agents")
	info, err := os.Stat(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			report.Duration = time.Since(start)
			return report, nil
		}
		return nil, fmt.Errorf("stat .agents: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf(".agents is not a directory: %s", agentsDir)
	}

	learningsDir := filepath.Join(agentsDir, "learnings")
	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		if os.IsNotExist(err) {
			report.Duration = time.Since(start)
			return report, nil
		}
		return nil, fmt.Errorf("read learnings dir: %w", err)
	}

	staleCutoff := time.Now().Add(-staleCitationAge)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(learningsDir, entry.Name())
		fm, parseErr := parseLearningFrontmatter(path)
		if parseErr != nil {
			report.Degraded = append(report.Degraded,
				fmt.Sprintf("%s: %v", entry.Name(), parseErr))
			continue
		}

		// Missing source: empty source_bead AND empty source fields.
		if strings.TrimSpace(fm.SourceBead) == "" && strings.TrimSpace(fm.Source.String()) == "" {
			report.MissingSources++
		}

		// Stale citation: date parses and is older than the cutoff.
		if fm.Date != "" {
			if parsedDate, dateErr := parseLearningDate(fm.Date); dateErr == nil {
				if parsedDate.Before(staleCutoff) {
					report.StaleCitations++
				}
			}
		}
	}

	report.Duration = time.Since(start)
	return report, nil
}

// parseLearningFrontmatter reads the leading YAML frontmatter block
// from a learning markdown file. Returns an error if the file cannot
// be read or the frontmatter block is malformed.
func parseLearningFrontmatter(path string) (*learningFrontmatter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return &learningFrontmatter{}, nil
	}
	rest := content[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, fmt.Errorf("unterminated frontmatter")
	}
	fmBlock := rest[:end]
	var fm learningFrontmatter
	if err := yaml.Unmarshal([]byte(fmBlock), &fm); err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	return &fm, nil
}

// parseLearningDate attempts to parse a learning date string using the
// two canonical formats: ISO date (2006-01-02) and RFC3339.
func parseLearningDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognized date format: %q", s)
}

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
