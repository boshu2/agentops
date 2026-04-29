// Package wikiworker validates structured wiki extraction payloads produced by
// AgentWorker sessions before they are persisted into the knowledge corpus.
package wikiworker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/boshu2/agentops/cli/internal/agentworker"
)

const ExtractionSchemaVersion = 1

var (
	ErrEmptyExtraction   = errors.New("empty wiki extraction")
	ErrInvalidJSON       = errors.New("invalid wiki extraction JSON")
	ErrInvalidSchema     = errors.New("invalid wiki extraction schema")
	ErrExtractionRefusal = errors.New("wiki extraction refusal")
)

// Extraction is the strict schema for a wiki/forge extraction payload.
type Extraction struct {
	SchemaVersion int                    `json:"schema_version"`
	Title         string                 `json:"title"`
	Summary       string                 `json:"summary"`
	Entities      []string               `json:"entities"`
	Concepts      []string               `json:"concepts"`
	Decisions     []string               `json:"decisions"`
	OpenQuestions []string               `json:"open_questions"`
	WorkPhase     string                 `json:"work_phase"`
	Artifacts     []agentworker.Artifact `json:"artifacts,omitempty"`
	Refusal       string                 `json:"refusal,omitempty"`
}

// ParseExtraction decodes and validates a worker-produced wiki extraction.
func ParseExtraction(raw []byte) (Extraction, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return Extraction{}, ErrEmptyExtraction
	}
	var out Extraction
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return Extraction{}, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	var extra struct{}
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return Extraction{}, fmt.Errorf("%w: trailing JSON tokens", ErrInvalidJSON)
	}
	if err := ValidateExtraction(out); err != nil {
		return Extraction{}, err
	}
	return out, nil
}

// ValidateExtraction enforces the strict wiki extraction schema.
func ValidateExtraction(out Extraction) error {
	if strings.TrimSpace(out.Refusal) != "" {
		return fmt.Errorf("%w: %s", ErrExtractionRefusal, out.Refusal)
	}
	if out.SchemaVersion != ExtractionSchemaVersion {
		return fmt.Errorf("%w: schema_version must be %d", ErrInvalidSchema, ExtractionSchemaVersion)
	}
	if strings.TrimSpace(out.Title) == "" {
		return fmt.Errorf("%w: title is required", ErrInvalidSchema)
	}
	if strings.TrimSpace(out.Summary) == "" {
		return fmt.Errorf("%w: summary is required", ErrInvalidSchema)
	}
	if !allowedWorkPhase(out.WorkPhase) {
		return fmt.Errorf("%w: invalid work_phase %q", ErrInvalidSchema, out.WorkPhase)
	}
	if err := validateStringList("entities", out.Entities); err != nil {
		return err
	}
	if err := validateStringList("concepts", out.Concepts); err != nil {
		return err
	}
	if err := validateStringList("decisions", out.Decisions); err != nil {
		return err
	}
	if err := validateStringList("open_questions", out.OpenQuestions); err != nil {
		return err
	}
	if err := agentworker.ValidateArtifacts(out.Artifacts); err != nil {
		return err
	}
	return nil
}

func allowedWorkPhase(phase string) bool {
	switch phase {
	case "research", "plan", "implement", "verify", "post-mortem", "other":
		return true
	default:
		return false
	}
}

func validateStringList(field string, values []string) error {
	if values == nil {
		return fmt.Errorf("%w: %s must be an array", ErrInvalidSchema, field)
	}
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: %s[%d] is empty", ErrInvalidSchema, field, i)
		}
	}
	return nil
}
