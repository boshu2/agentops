package wikiworker

import (
	"errors"
	"testing"

	"github.com/boshu2/agentops/cli/internal/agentworker"
)

func TestWikiExtractionSchemaAcceptsValidExtraction(t *testing.T) {
	out, err := ParseExtraction([]byte(`{
		"schema_version": 1,
		"title": "Daemon ledger owns status",
		"summary": "The daemon event ledger is authoritative and projections are derived from it.",
		"entities": ["agentopsd", ".agents/rpi"],
		"concepts": ["event sourcing", "daemon projections"],
		"decisions": ["Use one run/job ledger because status files can be replayed"],
		"open_questions": [],
		"work_phase": "plan",
		"artifacts": [
			{"kind": "wiki-note", "path": ".agents/wiki/sources/daemon-ledger.md", "validation_status": "valid"}
		]
	}`))
	if err != nil {
		t.Fatalf("ParseExtraction: %v", err)
	}
	if out.Title != "Daemon ledger owns status" {
		t.Fatalf("title: %q", out.Title)
	}
}

func TestWikiExtractionSchemaRejectsInvalidJSON(t *testing.T) {
	_, err := ParseExtraction([]byte(`{"schema_version":`))
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("want ErrInvalidJSON, got %v", err)
	}
}

func TestWikiExtractionSchemaRejectsEmptyOutput(t *testing.T) {
	_, err := ParseExtraction([]byte("\n\t"))
	if !errors.Is(err, ErrEmptyExtraction) {
		t.Fatalf("want ErrEmptyExtraction, got %v", err)
	}
}

func TestWikiExtractionSchemaRejectsRefusal(t *testing.T) {
	_, err := ParseExtraction([]byte(`{
		"schema_version": 1,
		"title": "Refusal",
		"summary": "The worker refused.",
		"entities": [],
		"concepts": [],
		"decisions": [],
		"open_questions": [],
		"work_phase": "other",
		"refusal": "I cannot extract this"
	}`))
	if !errors.Is(err, ErrExtractionRefusal) {
		t.Fatalf("want ErrExtractionRefusal, got %v", err)
	}
}

func TestWikiExtractionSchemaRejectsMalformedArtifacts(t *testing.T) {
	_, err := ParseExtraction([]byte(`{
		"schema_version": 1,
		"title": "Malformed artifact",
		"summary": "The artifact does not identify a path or URI.",
		"entities": [],
		"concepts": [],
		"decisions": [],
		"open_questions": [],
		"work_phase": "other",
		"artifacts": [{"kind": "wiki-note"}]
	}`))
	if !errors.Is(err, agentworker.ErrMalformedArtifact) {
		t.Fatalf("want ErrMalformedArtifact, got %v", err)
	}
}

func TestWikiExtractionSchemaRejectsNilArrays(t *testing.T) {
	_, err := ParseExtraction([]byte(`{
		"schema_version": 1,
		"title": "Missing arrays",
		"summary": "Array fields must be explicit for deterministic downstream writes.",
		"work_phase": "other"
	}`))
	if !errors.Is(err, ErrInvalidSchema) {
		t.Fatalf("want ErrInvalidSchema, got %v", err)
	}
}
