package extract

import (
	"testing"
)

// fakeGen implements llm.Generator for client/extractor tests without a live
// model. It returns scripted responses in order (mirrors llm.fakeLLM).
type fakeGen struct {
	responses []string
	calls     int
	prompts   []string
}

func (f *fakeGen) Generate(prompt string) (string, error) {
	f.prompts = append(f.prompts, prompt)
	i := f.calls
	f.calls++
	if i < len(f.responses) {
		return f.responses[i], nil
	}
	return "", nil // out of scripted responses -> empty (filtered)
}

func (f *fakeGen) Digest() string     { return "sha256:fake" }
func (f *fakeGen) ContextBudget() int { return 8192 }
func (f *fakeGen) ModelName() string  { return "fake-extractor" }

func TestClient_Law0Backends(t *testing.T) {
	// The allowlist must be EXACTLY {codex, bushido-llama, agy}.
	allowed := AllowedBackends()
	want := map[Backend]bool{
		BackendCodex:        true,
		BackendBushidoLlama: true,
		BackendAGY:          true,
	}
	if len(allowed) != len(want) {
		t.Fatalf("AllowedBackends size: got %d (%v), want %d", len(allowed), allowed, len(want))
	}
	for _, b := range allowed {
		if !want[b] {
			t.Errorf("unexpected backend in allowlist: %q", b)
		}
		if !b.IsAllowed() {
			t.Errorf("allowed backend %q reports IsAllowed=false", b)
		}
	}

	// LAW 0: a "claude" backend id MUST be rejected.
	for _, bad := range []string{"claude", "claude -p", "claude --print", "anthropic", ""} {
		if _, err := ParseBackend(bad); err == nil {
			t.Errorf("ParseBackend(%q) should be rejected (LAW 0 / closed allowlist)", bad)
		}
		if Backend(bad).IsAllowed() {
			t.Errorf("Backend(%q).IsAllowed() should be false", bad)
		}
	}

	// Each allowed id parses back to itself.
	for _, ok := range []string{"codex", "bushido-llama", "agy"} {
		b, err := ParseBackend(ok)
		if err != nil {
			t.Errorf("ParseBackend(%q): unexpected error %v", ok, err)
		}
		if string(b) != ok {
			t.Errorf("ParseBackend(%q) = %q", ok, b)
		}
	}
}

func TestNewClientWithGenerator_RejectsBadBackend(t *testing.T) {
	if _, err := NewClientWithGenerator(Backend("claude"), &fakeGen{}); err == nil {
		t.Error("NewClientWithGenerator with claude backend should error (LAW 0)")
	}
	if _, err := NewClientWithGenerator(BackendCodex, nil); err == nil {
		t.Error("NewClientWithGenerator with nil generator should error")
	}
	c, err := NewClientWithGenerator(BackendBushidoLlama, &fakeGen{})
	if err != nil {
		t.Fatalf("NewClientWithGenerator(valid): %v", err)
	}
	if c.Backend() != BackendBushidoLlama {
		t.Errorf("Backend() = %q", c.Backend())
	}
}
