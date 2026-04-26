package ratchet

import (
	"bufio"
	"strings"
	"testing"
)

// FuzzParseChainLines fuzzes the JSONL chain parser with arbitrary input.
// parseChainLines expects line 1 as chain metadata and subsequent lines as entries.
func FuzzParseChainLines(f *testing.F) {
	// Seed corpus with realistic chain formats (JSONL: line 1 = metadata, rest = entries)
	f.Add(`{"id":"chain-001","started":"2026-01-01T00:00:00Z","chain":[]}` + "\n" +
		`{"step":"research","timestamp":"2026-01-01T00:01:00Z","output":"plan.md","locked":true}`)
	f.Add(`{"id":"chain-002","started":"2026-01-01T00:00:00Z"}`)
	f.Add(``)
	f.Add(`not json at all`)
	f.Add(`{"id":""}` + "\n" + `{"step":"bad","timestamp":"not-a-time"}`)
	f.Add(`{"id":"chain-003"}` + "\n" + `malformed entry` + "\n" + `{"step":"plan","timestamp":"2026-01-01T00:00:00Z","output":"out.md","locked":false}`)
	f.Add(`{}`)
	f.Add("\n\n\n")
	f.Add(`{"id":"chain-004","started":"2026-01-01T00:00:00Z","epic_id":"epic-1"}` + "\n" +
		`{"step":"research","timestamp":"2026-01-01T00:00:00Z","input":"in.md","output":"out.md","locked":true}` + "\n" +
		`{"step":"plan","timestamp":"2026-01-01T00:01:00Z","input":"out.md","output":"plan.md","locked":false}`)

	f.Fuzz(func(t *testing.T, data string) {
		scanner := bufio.NewScanner(strings.NewReader(data))
		chain := &Chain{}
		// Must never panic — errors are acceptable
		_ = parseChainLines(scanner, chain)
	})
}

// TestFuzzParseChainLines_SeedCorrectness pins behavioral expectations for
// each seed input the fuzzer adds. Fuzz targets prove no-panic; this companion
// asserts the parser actually populates Chain fields correctly for the
// realistic inputs we seed it with. Without this, a regression that makes
// parseChainLines silently drop entries would still pass the fuzz target.
func TestFuzzParseChainLines_SeedCorrectness(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantID       string
		wantEpic     string
		wantEntries  int
		wantFirstStep string
		wantErr      bool
	}{
		{
			name:          "metadata_plus_one_entry",
			input:         `{"id":"chain-001","started":"2026-01-01T00:00:00Z","chain":[]}` + "\n" + `{"step":"research","timestamp":"2026-01-01T00:01:00Z","output":"plan.md","locked":true}`,
			wantID:        "chain-001",
			wantEntries:   1,
			wantFirstStep: "research",
		},
		{
			name:        "metadata_only",
			input:       `{"id":"chain-002","started":"2026-01-01T00:00:00Z"}`,
			wantID:      "chain-002",
			wantEntries: 0,
		},
		{
			name:        "empty_input",
			input:       ``,
			wantID:      "",
			wantEntries: 0,
		},
		{
			name:    "non_json_first_line",
			input:   `not json at all`,
			wantErr: true,
		},
		{
			name:        "malformed_entry_skipped_among_valid",
			input:       `{"id":"chain-003"}` + "\n" + `malformed entry` + "\n" + `{"step":"plan","timestamp":"2026-01-01T00:00:00Z","output":"out.md","locked":false}`,
			wantID:      "chain-003",
			wantEntries: 1,
			wantFirstStep: "plan",
		},
		{
			name:        "empty_metadata_object",
			input:       `{}`,
			wantID:      "",
			wantEntries: 0,
		},
		{
			name:          "epic_id_with_two_entries",
			input:         `{"id":"chain-004","started":"2026-01-01T00:00:00Z","epic_id":"epic-1"}` + "\n" + `{"step":"research","timestamp":"2026-01-01T00:00:00Z","input":"in.md","output":"out.md","locked":true}` + "\n" + `{"step":"plan","timestamp":"2026-01-01T00:01:00Z","input":"out.md","output":"plan.md","locked":false}`,
			wantID:        "chain-004",
			wantEpic:      "epic-1",
			wantEntries:   2,
			wantFirstStep: "research",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := bufio.NewScanner(strings.NewReader(tt.input))
			chain := &Chain{}
			err := parseChainLines(scanner, chain)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected parse error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			if chain.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", chain.ID, tt.wantID)
			}
			if chain.EpicID != tt.wantEpic {
				t.Errorf("EpicID = %q, want %q", chain.EpicID, tt.wantEpic)
			}
			if len(chain.Entries) != tt.wantEntries {
				t.Fatalf("Entries len = %d, want %d", len(chain.Entries), tt.wantEntries)
			}
			if tt.wantEntries > 0 && tt.wantFirstStep != "" {
				if string(chain.Entries[0].Step) != tt.wantFirstStep {
					t.Errorf("Entries[0].Step = %q, want %q", chain.Entries[0].Step, tt.wantFirstStep)
				}
			}
		})
	}
}
