package extract

// This file defines the LLM-client abstraction the extractor calls, with a
// CLOSED backend allowlist. LAW 0: the Claude print/headless mode is NEVER a
// selectable backend — it bills the Anthropic API / burns the Claude Max quota.
// The only permitted backends are:
//
//	codex         — `codex exec` (Codex Pro sub), structured output via --output-schema
//	bushido-llama — the local llama.cpp / ollama OpenAI-compat endpoint
//	agy           — AGY (the sanctioned Gemini/Pro sub-backed path)
//
// The allowlist is a typed enum + set so any other id (notably "claude") is
// rejected with an error at selection time, not silently honored. This file
// reuses the llm.Generator mechanics (see cli/internal/llm) rather than
// reimplementing HTTP; the codex path mirrors runCodexExec's --output-schema
// structured-output handling.

import (
	"context"
	"fmt"
	"os"

	"github.com/boshu2/agentops/cli/internal/llm"
)

// Backend is a typed identifier for an allowed LLM backend. The zero value is
// invalid; use ParseBackend or the exported constants.
type Backend string

const (
	// BackendCodex is the `codex exec` structured-output path (LAW-0 compliant).
	BackendCodex Backend = "codex"
	// BackendBushidoLlama is the local llama.cpp / ollama OpenAI-compat endpoint.
	BackendBushidoLlama Backend = "bushido-llama"
	// BackendAGY is AGY — the sanctioned Gemini/Pro sub-backed path.
	BackendAGY Backend = "agy"
)

// allowedBackends is the CLOSED allowlist. It is exactly {codex, bushido-llama,
// agy}. Notably "claude" is absent (LAW 0). This is the single source of truth
// for backend admission.
var allowedBackends = map[Backend]bool{
	BackendCodex:        true,
	BackendBushidoLlama: true,
	BackendAGY:          true,
}

// AllowedBackends returns the closed allowlist as a slice, for tests and
// diagnostics. It deliberately excludes any Claude/print backend (LAW 0).
func AllowedBackends() []Backend {
	return []Backend{BackendCodex, BackendBushidoLlama, BackendAGY}
}

// IsAllowed reports whether b is in the closed allowlist.
func (b Backend) IsAllowed() bool { return allowedBackends[b] }

// ParseBackend validates a backend id against the closed allowlist and returns
// the typed Backend, or an error naming the rejection. Any Claude/print-mode
// id is rejected here — LAW 0 has no exception.
func ParseBackend(id string) (Backend, error) {
	b := Backend(id)
	if !b.IsAllowed() {
		return "", fmt.Errorf("backend %q is not an allowed extractor backend: must be one of {codex, bushido-llama, agy} (LAW 0: a claude print-mode backend is forbidden)", id)
	}
	return b, nil
}

// Client is the small LLM-client abstraction the extractor uses. It wraps a
// chosen backend plus the schema-constrained call mechanics. Construct one with
// NewClient (validates the backend) or NewClientWithGenerator (test injection).
type Client struct {
	backend Backend
	// gen is the model backend. For BackendBushidoLlama it is an llm.Generator
	// (the ollama OpenAI-compat client or a test fake). For BackendCodex it may
	// be a codex-exec-backed Generator; tests inject a fake either way.
	gen llm.Generator
	// codexExec, when set, is the schema-constrained codex turn function
	// (signature mirrors cli/cmd/ao/eval_scenario_ab.go runCodexExec). It is
	// optional; the Generator path is the default and what tests exercise.
	codexExec func(ctx context.Context, prompt, outputSchemaPath string) (string, int, error)
}

// NewCodexClient constructs a production Client on the BackendCodex backend,
// wired to a raw schema-constrained codex turn function (the signature mirrors
// cli/cmd/ao/eval_scenario_ab.go runCodexExec). LAW 0: BackendCodex is on the
// closed allowlist; a Claude print-mode backend is never selectable here.
//
// The raw function expects a FILE PATH to a JSON Schema, but Client.Generate
// hands its codexExec the schema BYTES (it does not own a temp file). This
// constructor bridges that mismatch: the stored closure writes the schema bytes
// to a temp file, passes that temp file's PATH to raw, and removes the temp file
// afterward. A nil raw function is a programming error and is rejected.
func NewCodexClient(raw func(ctx context.Context, prompt, schemaPath string) (string, int, error)) *Client {
	if raw == nil {
		// A nil raw would make Generate fall through to the no-callable-generator
		// error; surface the misconfiguration as an inert client whose Generate
		// reports it, rather than panicking at call time.
		return &Client{backend: BackendCodex}
	}
	return &Client{
		backend: BackendCodex,
		codexExec: func(ctx context.Context, prompt, schemaBytes string) (string, int, error) {
			// Client.Generate passes the schema BYTES here; raw wants a PATH.
			// Write the bytes to a temp file and hand raw the path.
			f, err := os.CreateTemp("", "extract-codex-schema-*.json")
			if err != nil {
				return "", 0, fmt.Errorf("codex schema temp: %w", err)
			}
			tmpPath := f.Name()
			defer func() { _ = os.Remove(tmpPath) }()
			if _, err := f.WriteString(schemaBytes); err != nil {
				_ = f.Close()
				return "", 0, fmt.Errorf("write codex schema temp: %w", err)
			}
			if err := f.Close(); err != nil {
				return "", 0, fmt.Errorf("close codex schema temp: %w", err)
			}
			return raw(ctx, prompt, tmpPath)
		},
	}
}

// NewClientWithGenerator constructs a Client bound to an explicit Generator. It
// still enforces the closed allowlist on the backend id. This is the seam tests
// use to inject a fake Generator (no live model).
func NewClientWithGenerator(backend Backend, gen llm.Generator) (*Client, error) {
	if !backend.IsAllowed() {
		return nil, fmt.Errorf("backend %q is not an allowed extractor backend (LAW 0)", string(backend))
	}
	if gen == nil {
		return nil, fmt.Errorf("nil generator for backend %q", string(backend))
	}
	return &Client{backend: backend, gen: gen}, nil
}

// Backend returns the client's selected backend.
func (c *Client) Backend() Backend { return c.backend }

// Generate calls the underlying backend with the prompt. The compiled schema is
// passed so a structured-output backend (codex) can constrain its final message;
// Generator backends (bushido-llama) instead rely on the prompt + JSON mode and
// ignore the schema bytes here (the extractor still parses against the template).
func (c *Client) Generate(ctx context.Context, prompt string, schema []byte) (string, error) {
	if c.gen != nil {
		return c.gen.Generate(prompt)
	}
	if c.codexExec != nil {
		out, _, err := c.codexExec(ctx, prompt, string(schema))
		return out, err
	}
	return "", fmt.Errorf("client backend %q has no callable generator", string(c.backend))
}
