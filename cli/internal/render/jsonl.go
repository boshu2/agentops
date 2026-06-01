package render

import (
	"encoding/json"
	"io"

	"github.com/boshu2/agentops/cli/internal/sessionstore"
)

// JSONLFormatter outputs sessions as JSON Lines format.
// Each session is a single JSON object on one line.
type JSONLFormatter struct {
	// Pretty enables indented JSON (not recommended for JSONL).
	Pretty bool
}

// NewJSONLFormatter creates a new JSONL formatter.
func NewJSONLFormatter() *JSONLFormatter {
	return &JSONLFormatter{
		Pretty: false,
	}
}

// Format writes the session as a JSON line.
func (jf *JSONLFormatter) Format(w io.Writer, session *sessionstore.Session) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false) // Don't escape < > & in content

	if jf.Pretty {
		encoder.SetIndent("", "  ")
	}

	// Create the output structure
	output := jf.buildOutput(session)

	return encoder.Encode(output)
}

// Extension returns the file extension for JSONL.
func (jf *JSONLFormatter) Extension() string {
	return ".jsonl"
}

// jsonlOutput is the structure written to JSONL files.
type jsonlOutput struct {
	SessionID      string         `json:"session_id"`
	Date           string         `json:"date"`
	Summary        string         `json:"summary,omitempty"`
	Decisions      []string       `json:"decisions,omitempty"`
	Knowledge      []string       `json:"knowledge,omitempty"`
	FilesChanged   []string       `json:"files_changed,omitempty"`
	Issues         []string       `json:"issues,omitempty"`
	ToolCalls      map[string]int `json:"tool_calls,omitempty"`
	Tokens         *tokenOutput   `json:"tokens,omitempty"`
	TranscriptPath string         `json:"transcript_path,omitempty"`
}

type tokenOutput struct {
	Input  int `json:"input"`
	Output int `json:"output"`
	Total  int `json:"total"`
}

// buildOutput creates the JSON output structure.
func (jf *JSONLFormatter) buildOutput(session *sessionstore.Session) *jsonlOutput {
	output := &jsonlOutput{
		SessionID:      session.ID,
		Date:           session.Date.Format("2006-01-02"),
		Summary:        session.Summary,
		Decisions:      session.Decisions,
		Knowledge:      session.Knowledge,
		FilesChanged:   session.FilesChanged,
		Issues:         session.Issues,
		ToolCalls:      session.ToolCalls,
		TranscriptPath: session.TranscriptPath,
	}

	// Only include tokens if there's data
	if session.Tokens.Total > 0 {
		output.Tokens = &tokenOutput{
			Input:  session.Tokens.Input,
			Output: session.Tokens.Output,
			Total:  session.Tokens.Total,
		}
	}

	return output
}
