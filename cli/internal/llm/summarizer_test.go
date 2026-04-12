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
func (f *fakeLLM) ModelName() string  { return "gemma2:9b" }

// A realistic gemma output that the spike validated on real chunks.
const validSpikeOutput = `### Intent
Run vibe command

### Summary
The assistant will run the vibe command in quick mode to analyze recent changes.

### Entities
- [[file:/FIXTURE/.claude/plugins/cache/agentops-marketplace/agentops/2.27.0/skills/vibe]]

### Assistant condensed
The assistant will execute the vibe command with the --quick flag.`

func TestSummarizer_SummarizeChunk_ParsesStrictMarkdown(t *testing.T) {
	f := &fakeLLM{responses: []string{validSpikeOutput}}
	s := NewSummarizer(f)
	chunk := TurnChunk{Index: 0, UserText: "u", AssistantText: "a"}
	note, err := s.SummarizeChunk(chunk)
	if err != nil {
		t.Fatalf("SummarizeChunk: %v", err)
	}
	if note.Intent != "Run vibe command" {
		t.Errorf("Intent: got %q", note.Intent)
	}
	if !strings.Contains(note.Summary, "vibe command") {
		t.Errorf("Summary: got %q", note.Summary)
	}
	if len(note.Entities) == 0 {
		t.Errorf("Entities: want at least 1, got 0")
	}
	if !strings.Contains(note.AssistantCondensed, "--quick") {
		t.Errorf("AssistantCondensed: got %q", note.AssistantCondensed)
	}
	if note.Skipped {
		t.Errorf("should not be Skipped")
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

func TestSummarizer_SummarizeChunk_CodeFenceTolerance(t *testing.T) {
	// Tolerate ```markdown fences wrapping the whole output (per spike scoring).
	f := &fakeLLM{responses: []string{"```markdown\n" + validSpikeOutput + "\n```"}}
	s := NewSummarizer(f)
	note, err := s.SummarizeChunk(TurnChunk{})
	if err != nil {
		t.Fatalf("SummarizeChunk with fence: %v", err)
	}
	if note.Intent == "" {
		t.Errorf("code-fence tolerance failed, Intent empty")
	}
}

func TestSummarizer_SummarizeChunk_MissingSectionErrors(t *testing.T) {
	f := &fakeLLM{responses: []string{"### Intent\nfoo\n### Summary\nbar"}}
	s := NewSummarizer(f)
	_, err := s.SummarizeChunk(TurnChunk{})
	if err == nil {
		t.Fatal("want error on missing Entities/Assistant condensed")
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

func TestSummarizer_PromptContainsSpikeTemplate(t *testing.T) {
	f := &fakeLLM{responses: []string{validSpikeOutput}}
	s := NewSummarizer(f)
	chunk := TurnChunk{UserText: "hello", AssistantText: "world"}
	_, _ = s.SummarizeChunk(chunk)
	if len(f.prompts) != 1 {
		t.Fatalf("want 1 prompt, got %d", len(f.prompts))
	}
	p := f.prompts[0]
	// Verbatim tags from the W0-6 spike PROMPT_TEMPLATE.
	for _, want := range []string{
		"strict markdown, no JSON",
		"### Intent",
		"### Summary",
		"### Entities",
		"### Assistant condensed",
		`If the turn has no substantive content, output exactly "SKIP"`,
		"USER:\nhello",
		"ASSISTANT:\nworld",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q\nfull prompt:\n%s", want, p)
		}
	}
}

func TestRenderPrompt_AllowsLiteralPercentSigns(t *testing.T) {
	template := "Progress: 100% complete\n\nINPUT:\n%s"
	input := "USER:\ncoverage is 99.5%\nASSISTANT:\nship it"
	got := renderPrompt(template, input)

	for _, want := range []string{
		"Progress: 100% complete",
		"coverage is 99.5%",
		"ASSISTANT:\nship it",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered prompt missing %q\nfull prompt:\n%s", want, got)
		}
	}
	if strings.Contains(got, "%!") {
		t.Fatalf("rendered prompt contains fmt interpolation artifact: %s", got)
	}
}

func TestParseEntities_HandlesWikilinks(t *testing.T) {
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
