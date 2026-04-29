package agentworker

import (
	"errors"
	"testing"
)

func TestWorkerOutputSchemaAcceptsValidEnvelope(t *testing.T) {
	out, err := ParseOutputEnvelope([]byte(`{
		"schema_version": 1,
		"session": {
			"worker_kind": "codex",
			"provider": "fake",
			"job_id": "wiki.forge:1",
			"attempt_id": "attempt-1",
			"session_id": "session-1",
			"status": "completed"
		},
		"status": "completed",
		"text": "structured wiki extraction",
		"artifacts": [
			{"kind": "wiki-note", "path": ".agents/wiki/sources/session-1.md", "validation_status": "valid"}
		]
	}`))
	if err != nil {
		t.Fatalf("ParseOutputEnvelope: %v", err)
	}
	if out.Session.SessionID != "session-1" {
		t.Fatalf("session id: %q", out.Session.SessionID)
	}
}

func TestWorkerOutputSchemaRejectsInvalidJSON(t *testing.T) {
	_, err := ParseOutputEnvelope([]byte(`{"schema_version":`))
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("want ErrInvalidJSON, got %v", err)
	}
}

func TestWorkerOutputSchemaRejectsEmptyOutput(t *testing.T) {
	_, err := ParseOutputEnvelope([]byte("   \n"))
	if !errors.Is(err, ErrEmptyOutput) {
		t.Fatalf("want ErrEmptyOutput, got %v", err)
	}
}

func TestWorkerOutputSchemaRejectsRefusal(t *testing.T) {
	_, err := ParseOutputEnvelope([]byte(`{
		"schema_version": 1,
		"session": {"worker_kind": "claude", "provider": "fake", "session_id": "session-1"},
		"status": "completed",
		"refusal": "I cannot inspect private transcripts"
	}`))
	if !errors.Is(err, ErrWorkerRefusal) {
		t.Fatalf("want ErrWorkerRefusal, got %v", err)
	}
}

func TestWorkerOutputSchemaRejectsMalformedArtifact(t *testing.T) {
	_, err := ParseOutputEnvelope([]byte(`{
		"schema_version": 1,
		"session": {"worker_kind": "codex", "provider": "fake", "session_id": "session-1", "status": "completed"},
		"status": "completed",
		"text": "structured wiki extraction",
		"artifacts": [{"kind": "wiki-note"}]
	}`))
	if !errors.Is(err, ErrMalformedArtifact) {
		t.Fatalf("want ErrMalformedArtifact, got %v", err)
	}
}
