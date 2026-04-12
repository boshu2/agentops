package llm

import (
	"errors"
	"strings"
	"testing"
)

// fakeLLM implements Generator for tests without touching a real ollama
// daemon. Each test mounts its own scripted responses.
type fakeLLM struct {
	responses []string
	errs      []error
	calls     int
	prompts   []string
}

func (f *fakeLLM) Generate(prompt string) (string, error) {
	f.prompts = append(f.prompts, prompt)
	i := f.calls
	f.calls++
	if i < len(f.errs) && f.errs[i] != nil {
		return "", f.errs[i]
	}
	if i < len(f.responses) {
		return f.responses[i], nil
	}
	return "", errors.New("fakeLLM: out of responses")
}

func (f *fakeLLM) Digest() string     { return "sha256:fake-digest-1234" }
func (f *fakeLLM) ContextBudget() int { return 8192 }
func (f *fakeLLM) ModelName() string  { return "gemma4:e4b" }

// validJSONOutput matches what the proven JS worker prompt produces.
const validJSONOutput = `{
  "title": "Worktree isolation prevents parallel session file loss",
  "summary": "The session discovered that parallel Claude Code sessions in a shared worktree destroy untracked files during branch switches. Git worktree add was the fix.",
  "entities": ["cli/internal/llm/chunker.go", "feat/ao-forge-tiered"],
  "concepts": ["worktree isolation", "parallel session hazard"],
  "decisions": ["chose git worktree over single-branch commits because parallel session kept flipping HEAD"],
  "open_questions": ["Does the parallel session have a branch-switch hook?"],
  "work_phase": "implement"
}`

// validSpikeOutput is the legacy strict-markdown format (gemma2:9b fallback).
const validSpikeOutput = `### Intent
Run vibe command

### Summary
The assistant will run the vibe command in quick mode to analyze recent changes.

### Entities
- [[file:/FIXTURE/.claude/plugins/cache/agentops-marketplace/agentops/2.27.0/skills/vibe]]

### Assistant condensed
The assistant will execute the vibe command with the --quick flag.`

func TestSummarizer_SummarizeChunk_ParsesJSON(t *testing.T) {
	f := &fakeLLM{responses: []string{validJSONOutput}}
	s := NewSummarizer(f)
	chunk := TurnChunk{Index: 0, UserText: "u", AssistantText: "a"}
	note, err := s.SummarizeChunk(chunk)
	if err != nil {
		t.Fatalf("SummarizeChunk: %v", err)
	}
	if !strings.Contains(note.Intent, "Worktree isolation") {
		t.Errorf("Intent: got %q", note.Intent)
	}
	if !strings.Contains(note.Summary, "parallel Claude Code sessions") {
		t.Errorf("Summary: got %q", note.Summary)
	}
	if len(note.Entities) != 2 {
		t.Errorf("Entities: want 2, got %d: %v", len(note.Entities), note.Entities)
	}
	if len(note.Concepts) != 2 {
		t.Errorf("Concepts: want 2, got %v", note.Concepts)
	}
	if len(note.Decisions) != 1 {
		t.Errorf("Decisions: want 1, got %v", note.Decisions)
	}
	if note.WorkPhase != "implement" {
		t.Errorf("WorkPhase: got %q", note.WorkPhase)
	}
	if note.Skipped {
		t.Errorf("should not be Skipped")
	}
}

func TestSummarizer_SummarizeChunk_FallbackMarkdown(t *testing.T) {
	f := &fakeLLM{responses: []string{validSpikeOutput}}
	s := NewSummarizer(f)
	note, err := s.SummarizeChunk(TurnChunk{})
	if err != nil {
		t.Fatalf("SummarizeChunk: %v", err)
	}
	if note.Intent != "Run vibe command" {
		t.Errorf("Intent: got %q", note.Intent)
	}
}

func TestSummarizer_SummarizeChunk_SkipOutput(t *testing.T) {
	f := &fakeLLM{responses: []string{"SKIP"}}
	s := NewSummarizer(f)
	note, err := s.SummarizeChunk(TurnChunk{})
	if err != nil {
		t.Fatalf("SummarizeChunk: %v", err)
	}
	if !note.Skipped {
		t.Errorf("SKIP output should set Skipped=true")
	}
}

func TestSummarizer_SummarizeChunk_EmptyJSONSkip(t *testing.T) {
	f := &fakeLLM{responses: []string{`{"title":"","summary":""}`}}
	s := NewSummarizer(f)
	note, err := s.SummarizeChunk(TurnChunk{})
	if err != nil {
		t.Fatalf("SummarizeChunk: %v", err)
	}
	if !note.Skipped {
		t.Errorf("empty title+summary should be Skipped")
	}
}

func TestSummarizer_SummarizeChunk_JSONSkipObject(t *testing.T) {
	f := &fakeLLM{responses: []string{`{"skip":true}`}}
	s := NewSummarizer(f)
	note, err := s.SummarizeChunk(TurnChunk{})
	if err != nil {
		t.Fatalf("SummarizeChunk: %v", err)
	}
	if !note.Skipped {
		t.Errorf("{skip:true} should be Skipped")
	}
}

func TestSummarizer_SummarizeChunk_LLMErrorPropagates(t *testing.T) {
	f := &fakeLLM{errs: []error{errors.New("timeout")}}
	s := NewSummarizer(f)
	_, err := s.SummarizeChunk(TurnChunk{})
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Errorf("want timeout error propagated, got %v", err)
	}
}

func TestSummarizer_PromptContainsTranscriptDelimiters(t *testing.T) {
	f := &fakeLLM{responses: []string{validJSONOutput}}
	s := NewSummarizer(f)
	chunk := TurnChunk{UserText: "hello", AssistantText: "world"}
	_, _ = s.SummarizeChunk(chunk)
	if len(f.prompts) != 1 {
		t.Fatalf("want 1 prompt, got %d", len(f.prompts))
	}
	p := f.prompts[0]
	for _, want := range []string{
		"=== BEGIN TRANSCRIPT ===",
		"=== END TRANSCRIPT ===",
		"Return ONLY a JSON object",
		"USER:\nhello",
		"ASSISTANT:\nworld",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestParseEntityList_HandlesWikilinks(t *testing.T) {
	body := `- [[file:/foo/bar.go]]
- [[bead:na-123]]
- [[concept:noise-filter]]`
	ents := parseEntityList(body)
	if len(ents) != 3 {
		t.Fatalf("want 3 entities, got %d: %+v", len(ents), ents)
	}
	if ents[0] != "file:/foo/bar.go" {
		t.Errorf("first entity: got %q", ents[0])
	}
}
