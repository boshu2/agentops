package verdictcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type corpusCase struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Expected       string          `json:"expected"`
	SchemaLenient  bool            `json:"schema_lenient,omitempty"`
	FilenameDigest string          `json:"filename_digest"`
	Raw            string          `json:"raw,omitempty"`
	Artifact       json.RawMessage `json:"artifact,omitempty"`
}

// TestGoldenCorpus runs the shared cross-language verdict-contract corpus.
// The same cases run through the Python validator and the JSON schema
// (scripts/check-verdict-contract-corpus.sh); a divergence between the
// implementations fails CI on whichever side drifted.
func TestGoldenCorpus(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "tests", "fixtures", "verdict-contract", "cases")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corpus dir: %v", err)
	}
	if len(entries) < 10 {
		t.Fatalf("suspiciously small corpus (%d cases) — corpus missing?", len(entries))
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		payload, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var c corpusCase
		if err := json.Unmarshal(payload, &c); err != nil {
			t.Fatalf("%s: parse case: %v", entry.Name(), err)
		}
		t.Run(c.Name, func(t *testing.T) {
			body := []byte(c.Raw)
			if c.Raw == "" {
				body = c.Artifact
			}
			verifyErr := VerifyArtifact(body, c.FilenameDigest)
			switch c.Expected {
			case "valid":
				if verifyErr != nil {
					t.Fatalf("expected valid, got: %v\n%s", verifyErr, c.Description)
				}
			case "invalid":
				if verifyErr == nil {
					t.Fatalf("expected invalid, but verification passed\n%s", c.Description)
				}
			default:
				t.Fatalf("unknown expected value %q", c.Expected)
			}
		})
	}
}
