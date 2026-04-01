// Package storage provides interfaces and implementations for persisting
// olympus session data, indexes, and provenance records.
package storage

import (
	"io"
	"time"
)

// Session represents extracted knowledge from a transcript session.
type Session struct {
	// ID is the unique session identifier (from transcript).
	ID string `json:"session_id"`

	// Date is when the session occurred.
	Date time.Time `json:"date"`

	// Summary is a brief description of the session.
	Summary string `json:"summary,omitempty"`

	// Decisions lists architectural choices made.
	Decisions []string `json:"decisions,omitempty"`

	// Knowledge contains insights and learnings.
	Knowledge []string `json:"knowledge,omitempty"`

	// FilesChanged lists files modified in the session.
	FilesChanged []string `json:"files_changed,omitempty"`

	// Issues lists issue IDs referenced or created.
	Issues []string `json:"issues,omitempty"`

	// ToolCalls counts tool invocations by type.
	ToolCalls map[string]int `json:"tool_calls,omitempty"`

	// Tokens tracks token usage.
	Tokens TokenUsage `json:"tokens,omitempty"`

	// TranscriptPath is the source transcript file.
	TranscriptPath string `json:"transcript_path,omitempty"`

	// SessionType is the detected session type (career, research, debug, implement, brainstorm, general).
	SessionType string `json:"session_type,omitempty"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	Input     int  `json:"input"`
	Output    int  `json:"output"`
	Total     int  `json:"total"`
	Estimated bool `json:"estimated,omitempty"`
}

// IndexEntry represents a single entry in the session index.
type IndexEntry struct {
	// SessionID links to the full session.
	SessionID string `json:"session_id"`

	// Date for quick filtering.
	Date time.Time `json:"date"`

	// SessionPath is the path to the session file.
	SessionPath string `json:"session_path"`

	// Summary for search/preview.
	Summary string `json:"summary,omitempty"`

	// Tags for categorization.
	Tags []string `json:"tags,omitempty"`
}

// ProvenanceRecord tracks the origin of an artifact.
type ProvenanceRecord struct {
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

// Storage is the interface for persisting olympus data.
type Storage interface {
	// WriteSession writes a session to storage.
	// Returns the path where the session was written.
	WriteSession(session *Session) (string, error)

	// WriteIndex appends an entry to the session index.
	WriteIndex(entry *IndexEntry) error

	// WriteProvenance records provenance information.
	WriteProvenance(record *ProvenanceRecord) error

	// ReadSession retrieves a session by ID.
	ReadSession(sessionID string) (*Session, error)

	// ListSessions returns all session index entries.
	ListSessions() ([]IndexEntry, error)

	// QueryProvenance finds provenance records for an artifact.
	QueryProvenance(artifactPath string) ([]ProvenanceRecord, error)

	// Init creates the required directory structure.
	Init() error

	// Close releases any resources.
	Close() error
}

// Formatter transforms sessions into specific output formats.
type Formatter interface {
	// Format writes the session to the given writer.
	Format(w io.Writer, session *Session) error

	// Extension returns the file extension for this format.
	Extension() string
}
