package agentworker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const OutputSchemaVersion = 1

var (
	ErrEmptyOutput       = errors.New("empty worker output")
	ErrInvalidJSON       = errors.New("invalid worker output JSON")
	ErrInvalidSchema     = errors.New("invalid worker output schema")
	ErrWorkerRefusal     = errors.New("worker output refusal")
	ErrMalformedArtifact = errors.New("malformed worker artifact")
)

// OutputEnvelope is the strict outer schema for structured worker results.
// Runtime-specific payloads remain inside Text or Payload; session identity,
// terminal status, refusals, and artifacts stay uniform across providers.
type OutputEnvelope struct {
	SchemaVersion int             `json:"schema_version"`
	Session       SessionRef      `json:"session"`
	Status        SessionStatus   `json:"status"`
	Text          string          `json:"text,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	Refusal       string          `json:"refusal,omitempty"`
	Artifacts     []Artifact      `json:"artifacts,omitempty"`
}

// ParseOutputEnvelope decodes and validates a structured worker output.
func ParseOutputEnvelope(raw []byte) (OutputEnvelope, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return OutputEnvelope{}, ErrEmptyOutput
	}
	var out OutputEnvelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		return OutputEnvelope{}, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}
	var extra struct{}
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return OutputEnvelope{}, fmt.Errorf("%w: trailing JSON tokens", ErrInvalidJSON)
	}
	if err := ValidateOutputEnvelope(out); err != nil {
		return OutputEnvelope{}, err
	}
	return out, nil
}

// ValidateOutputEnvelope enforces the strict worker-output contract.
func ValidateOutputEnvelope(out OutputEnvelope) error {
	if strings.TrimSpace(out.Refusal) != "" {
		return fmt.Errorf("%w: %s", ErrWorkerRefusal, out.Refusal)
	}
	if out.SchemaVersion != OutputSchemaVersion {
		return fmt.Errorf("%w: schema_version must be %d", ErrInvalidSchema, OutputSchemaVersion)
	}
	if err := out.Session.Validate(); err != nil {
		return fmt.Errorf("%w: session: %v", ErrInvalidSchema, err)
	}
	if out.Session.Status == "" {
		return fmt.Errorf("%w: session.status is required", ErrInvalidSchema)
	}
	if out.Status == "" {
		return fmt.Errorf("%w: status is required", ErrInvalidSchema)
	}
	if out.Session.Status != out.Status {
		return fmt.Errorf("%w: session.status %s does not match status %s", ErrInvalidSchema, out.Session.Status, out.Status)
	}
	if !out.Status.Terminal() {
		return fmt.Errorf("%w: status must be terminal, got %s", ErrInvalidSchema, out.Status)
	}
	if out.Status != StatusCompleted {
		return fmt.Errorf("%w: non-success terminal status %s", ErrInvalidSchema, out.Status)
	}
	if strings.TrimSpace(out.Text) == "" && len(bytes.TrimSpace(out.Payload)) == 0 && len(out.Artifacts) == 0 {
		return ErrEmptyOutput
	}
	if err := ValidateArtifacts(out.Artifacts); err != nil {
		return err
	}
	return nil
}

// ValidateArtifacts checks worker artifacts before consumers trust or persist
// them.
func ValidateArtifacts(artifacts []Artifact) error {
	for i, artifact := range artifacts {
		if strings.TrimSpace(artifact.Kind) == "" {
			return fmt.Errorf("%w: artifacts[%d].kind is required", ErrMalformedArtifact, i)
		}
		if strings.TrimSpace(artifact.Path) == "" && strings.TrimSpace(artifact.URI) == "" {
			return fmt.Errorf("%w: artifacts[%d] requires path or uri", ErrMalformedArtifact, i)
		}
		switch artifact.ValidationStatus {
		case "", "pending", "valid":
		default:
			return fmt.Errorf("%w: artifacts[%d].validation_status=%q", ErrMalformedArtifact, i, artifact.ValidationStatus)
		}
	}
	return nil
}
