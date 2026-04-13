package llm

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/parser"
	"github.com/boshu2/agentops/cli/internal/types"
)

// KillSwitchEnv is the environment variable name that short-circuits the
// Tier 1 path when set to "1" (e.g. AGENTOPS_FORGE_TIER1_DISABLE=1).
const KillSwitchEnv = "AGENTOPS_FORGE_TIER1_DISABLE"

// Tier1Options configures a single RunForgeTier1 invocation. The zero value
// is NOT valid; callers must set SourcePaths, Model, Endpoint, and OutputDir
// at minimum.
type Tier1Options struct {
	// SourcePaths is the list of .jsonl files to process.
	SourcePaths []string

	// OutputDir is where session markdown pages land (typically
	// .agents/ao/sessions/ under the project root).
	OutputDir string

	// Model is the ollama model tag, e.g. "gemma2:9b".
	Model string

	// Endpoint is the ollama HTTP endpoint. Empty falls back to
	// ResolveDefaultEndpoint().
	Endpoint string

	// MaxChars is the per-chunk char budget. Zero falls back to DefaultMaxChars.
	MaxChars int

	// Quiet suppresses per-file progress output.
	Quiet bool

	// Writer is where progress lines are printed when !Quiet. Defaults to
	// os.Stdout when nil.
	Writer io.Writer

	// Workspace is the cwd to stamp into frontmatter. Defaults to the
	// process cwd when empty.
	Workspace string

	// IngestedBy is the name stamped into the ingested_by field. Defaults to
	// "ao-forge-tier1".
	IngestedBy string

	// clientFactory is an injection hook for tests that want to substitute a
	// Generator instead of hitting a real ollama endpoint. Unexported — use
	// SetGeneratorFactory.
	clientFactory func(opts OllamaOptions) (Generator, error)
}

// SetGeneratorFactory overrides the LLM backend factory on the options. Used
// by tests to inject a fake Generator; production callers should leave it
// unset so the default ollama client is constructed.
func (o *Tier1Options) SetGeneratorFactory(f func(opts OllamaOptions) (Generator, error)) {
	o.clientFactory = f
}

// Tier1Result summarizes one RunForgeTier1 invocation.
type Tier1Result struct {
	FilesProcessed int
	FilesSkipped   int
	SessionsWrote  []string
	Errors         []error
}

// RunForgeTier1 is the entry point that cli/cmd/ao/forge.go dispatches into
// when --tier=1 is passed. It chains:
//
//	parser (existing) → Redact → ChunkTurns → Summarizer.SummarizeChunk →
//	BuildPage → WriteSessionPage
//
// Redaction runs BEFORE chunking (critical per pre-mortem F3). The kill
// switch (KillSwitchEnv=1) short-circuits the whole path at the very top so
// operators can disable Tier 1 instantly without a code change.
func RunForgeTier1(opts Tier1Options) (*Tier1Result, error) {
	if os.Getenv(KillSwitchEnv) == "1" {
		return nil, fmt.Errorf("%s=1 set: tier 1 forge is disabled", KillSwitchEnv)
	}
	if len(opts.SourcePaths) == 0 {
		return nil, fmt.Errorf("tier1: no source paths supplied")
	}
	if opts.OutputDir == "" {
		return nil, fmt.Errorf("tier1: OutputDir is required")
	}
	if opts.Model == "" {
		return nil, fmt.Errorf("tier1: Model is required")
	}
	if opts.Endpoint == "" {
		opts.Endpoint = ResolveDefaultEndpoint()
	}
	if opts.MaxChars <= 0 {
		opts.MaxChars = DefaultMaxChars
	}
	if opts.Writer == nil {
		opts.Writer = os.Stdout
	}
	if opts.Workspace == "" {
		if wd, err := os.Getwd(); err == nil {
			opts.Workspace = wd
		}
	}
	if opts.IngestedBy == "" {
		opts.IngestedBy = "ao-forge-tier1"
	}

	// Build the LLM client once per invocation (init-phase /api/tags probe
	// catches missing model early).
	gen, err := buildGenerator(opts)
	if err != nil {
		return nil, fmt.Errorf("tier1: build LLM client: %w", err)
	}
	summarizer := NewSummarizer(gen)

	p := parser.NewParser()
	p.MaxContentLength = 0

	result := &Tier1Result{}

	for _, path := range opts.SourcePaths {
		sessionPath, err := processOneSession(path, opts, p, summarizer, gen)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", path, err))
			result.FilesSkipped++
			if !opts.Quiet {
				fmt.Fprintf(opts.Writer, "  ✗ %s: %v\n", filepath.Base(path), err)
			}
			continue
		}
		result.FilesProcessed++
		result.SessionsWrote = append(result.SessionsWrote, sessionPath)
		if !opts.Quiet {
			fmt.Fprintf(opts.Writer, "  ✓ %s → %s\n", filepath.Base(path), filepath.Base(sessionPath))
		}
	}
	return result, nil
}

func processOneSession(path string, opts Tier1Options, p *parser.Parser, s *Summarizer, gen Generator) (string, error) {
	parsed, err := p.ParseFile(path)
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	if parsed == nil || len(parsed.Messages) == 0 {
		return "", fmt.Errorf("empty transcript")
	}

	// Step 1 — REDACT BEFORE CHUNKING (pre-mortem F3 critical).
	redacted := make([]types.TranscriptMessage, len(parsed.Messages))
	for i, m := range parsed.Messages {
		m.Content = Redact(m.Content)
		redacted[i] = m
	}

	// Step 2 — chunk into turn-sized budgets.
	chunks := ChunkTurns(redacted, opts.MaxChars)
	if len(chunks) == 0 {
		return "", fmt.Errorf("no usable turn chunks after redaction + filter")
	}

	// Step 3 — summarize each chunk. Errors on individual chunks are
	// collected but do not abort the session (degrades honestly).
	notes := make([]ChunkNote, 0, len(chunks))
	for _, chunk := range chunks {
		note, err := s.SummarizeChunk(chunk)
		if err != nil {
			// Record as a skipped chunk so the confidence score reflects
			// the loss. This lets the pipeline ship a partial page rather
			// than failing the whole session.
			notes = append(notes, ChunkNote{Index: chunk.Index, Skipped: true, Raw: err.Error()})
			continue
		}
		notes = append(notes, note)
	}

	// Step 4 — assemble session meta + page.
	meta := deriveSessionMeta(path, parsed, opts, gen)
	page := BuildPage(meta, notes)

	// Step 5 — atomic write to .agents/ao/sessions/<session-id>.md.
	outPath := filepath.Join(opts.OutputDir, sanitizeSessionID(meta.SessionID)+".md")
	if err := WriteSessionPage(outPath, page); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}

	// Append to LOG.md and INDEX.md (non-fatal — don't fail the session on log errors).
	agentsDir := filepath.Dir(filepath.Dir(opts.OutputDir)) // wiki/sources → .agents/
	title := meta.SessionID
	if len(notes) > 0 && !notes[0].Skipped && notes[0].Intent != "" {
		title = notes[0].Intent
	}
	relPath := filepath.Join("wiki", "sources", sanitizeSessionID(meta.SessionID))
	_ = AppendToLog(agentsDir, opts.IngestedBy, "INGEST", title, relPath)
	_ = AppendToIndex(agentsDir, "Wiki Sources", relPath, title)

	return outPath, nil
}

func deriveSessionMeta(path string, parsed *parser.ParseResult, opts Tier1Options, gen Generator) SessionMeta {
	var started, ended time.Time
	if len(parsed.Messages) > 0 {
		started = parsed.Messages[0].Timestamp
		ended = parsed.Messages[len(parsed.Messages)-1].Timestamp
	}
	if started.IsZero() {
		started = time.Now().UTC()
	}
	if ended.IsZero() {
		ended = started
	}

	// Prefer the SessionID embedded in the parser output; fall back to
	// the filename stem.
	sid := ""
	for _, m := range parsed.Messages {
		if m.SessionID != "" {
			sid = m.SessionID
			break
		}
	}
	if sid == "" {
		base := filepath.Base(path)
		sid = strings.TrimSuffix(base, filepath.Ext(base))
	}

	turns := 0
	for _, m := range parsed.Messages {
		if m.Type == "user" {
			turns++
		}
	}

	return SessionMeta{
		SessionID:    sid,
		SourceJSONL:  path,
		Workspace:    opts.Workspace,
		StartedAt:    started,
		EndedAt:      ended,
		Turns:        turns,
		TokensIn:     0, // not tracked in v1 (requires usage block parsing)
		TokensOut:    0,
		ModelPrimary: "",
		Model:        opts.Model,
		ModelDigest:  gen.Digest(),
		IngestedBy:   opts.IngestedBy,
	}
}

// sanitizeSessionID returns a filename-safe version of a session ID.
func sanitizeSessionID(s string) string {
	if s == "" {
		return "session"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}

// buildGenerator constructs the LLM client — either via the injected factory
// (tests) or via NewOllamaClient (production).
func buildGenerator(opts Tier1Options) (Generator, error) {
	ollamaOpts := OllamaOptions{
		Endpoint:   opts.Endpoint,
		Model:      opts.Model,
		Timeout:    10 * time.Second,
		MaxRetries: 2,
	}
	if opts.clientFactory != nil {
		return opts.clientFactory(ollamaOpts)
	}
	return NewOllamaClient(ollamaOpts)
}
