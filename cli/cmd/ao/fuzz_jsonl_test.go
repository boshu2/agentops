// practices: [property-based-testing, llm-eval-harness]
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzParseLearningJSONL fuzzes the JSONL learning parser with arbitrary data.
// parseLearningJSONL reads from a file path, so we write fuzz data to a temp file.
func FuzzParseLearningJSONL(f *testing.F) {
	// Seed corpus with realistic learning formats
	f.Add(`{"summary":"test learning","confidence":0.8,"tags":["test"]}`)
	f.Add(`{}`)
	f.Add(``)
	f.Add(`{"summary":""}`)
	f.Add(`{"summary":"multi-line\nlearning","confidence":1.0,"tags":["a","b"],"utility":0.9}`)
	f.Add(`{"superseded_by":"other-id"}`)
	f.Add(`{"summary":"has source","source_bead":"bd-123","source_phase":"research","maturity":"validated"}`)
	f.Add(`not json at all`)
	f.Add(`{"summary":"` + string(make([]byte, 1024)) + `"}`)

	f.Fuzz(func(t *testing.T, data string) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			return
		}
		// Must never panic — errors are acceptable
		_, _ = parseLearningJSONL(path)
	})
}

func TestFuzzParseLearningJSONL_SeedCorrectness(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		summary string
		hasErr  bool
	}{
		{"valid learning", `{"summary":"test learning","confidence":0.8}`, "test learning", false},
		{"with source", `{"summary":"has source","source_bead":"bd-123","source_phase":"research"}`, "has source", false},
		{"empty json", `{}`, "", false},
		{"not json", `not json at all`, "", false},
		{"empty file", ``, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.jsonl")
			if err := os.WriteFile(path, []byte(tt.input), 0644); err != nil {
				t.Fatal(err)
			}
			l, err := parseLearningJSONL(path)
			if tt.hasErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.hasErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.hasErr && tt.summary != "" && l.Summary != tt.summary {
				t.Fatalf("expected summary=%q, got %q", tt.summary, l.Summary)
			}
		})
	}
}

// FuzzParseJSONLFirstLine fuzzes the first-line JSONL parser used by feedback.
func FuzzParseJSONLFirstLine(f *testing.F) {
	// Seed corpus with realistic JSONL first lines
	f.Add(`{"utility":0.5,"reward_count":3}`)
	f.Add(`{}`)
	f.Add(``)
	f.Add(`{"key":"value","nested":{"a":1}}`)
	f.Add("not json\n{\"second\":\"line\"}")
	f.Add(`{"utility":0.5}` + "\n" + `{"second":"line"}`)
	f.Add(`null`)
	f.Add(`[]`)

	f.Fuzz(func(t *testing.T, data string) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			return
		}
		// Must never panic — errors are acceptable
		_, _, _ = parseJSONLFirstLine(path)
	})
}

func TestFuzzParseJSONLFirstLine_SeedCorrectness(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		hasErr    bool
		wantLines int // expected number of lines returned
	}{
		{"single valid line", `{"utility":0.5,"reward_count":3}`, false, 1},
		{"two lines returns first", `{"utility":0.5}` + "\n" + `{"second":"line"}`, false, 2},
		{"empty file", ``, true, 0},
		{"empty json object", `{}`, false, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.jsonl")
			if err := os.WriteFile(path, []byte(tt.input), 0644); err != nil {
				t.Fatal(err)
			}
			lines, parsed, err := parseJSONLFirstLine(path)
			if tt.hasErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.hasErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.hasErr {
				if len(lines) == 0 {
					t.Fatal("expected non-empty lines slice")
				}
				if parsed == nil {
					t.Fatal("expected non-nil parsed map")
				}
				if tt.wantLines > 0 && len(lines) != tt.wantLines {
					t.Fatalf("expected %d lines, got %d", tt.wantLines, len(lines))
				}
			}
		})
	}
}

// FuzzParseJSONLSessionSummary fuzzes the session summary JSONL parser.
func FuzzParseJSONLSessionSummary(f *testing.F) {
	// Seed corpus with realistic session formats
	f.Add(`{"summary":"Session about testing fuzz targets"}`)
	f.Add(`{}`)
	f.Add(``)
	f.Add(`{"summary":""}`)
	f.Add(`{"summary":"a]very long summary that exceeds the typical truncation length and should be handled gracefully by the parser without any panics or crashes regardless of content"}`)
	f.Add(`{"no_summary_field":true}`)
	f.Add(`not json`)
	f.Add(`{"summary":123}`)

	f.Fuzz(func(t *testing.T, data string) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.jsonl")
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			return
		}
		// Must never panic — errors are acceptable
		_, _ = parseJSONLSessionSummary(path)
	})
}

func TestFuzzParseJSONLSessionSummary_SeedCorrectness(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		summary string
		hasErr  bool
	}{
		{"valid summary", `{"summary":"Session about testing"}`, "Session about testing", false},
		{"empty summary", `{"summary":""}`, "", false},
		{"no summary field", `{"no_summary_field":true}`, "", false},
		{"not json", `not json`, "", false},
		{"empty file", ``, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.jsonl")
			if err := os.WriteFile(path, []byte(tt.input), 0644); err != nil {
				t.Fatal(err)
			}
			summary, err := parseJSONLSessionSummary(path)
			if tt.hasErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.hasErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.hasErr && tt.summary != "" && summary != tt.summary {
				t.Fatalf("expected summary=%q, got %q", tt.summary, summary)
			}
		})
	}
}
