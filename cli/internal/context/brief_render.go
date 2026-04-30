package context

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BriefRenderInput is the pure input to [Render]. All fields the renderer needs
// are passed explicitly so the function has no hidden dependencies on globals,
// the filesystem, the clock, or environment variables. If a future caller has
// time-sensitive or env-derived data, they MUST resolve it before calling Render
// and pass it in via this struct.
//
// Fields:
//   - BeadID: identifier surfaced in the briefing header (e.g. "agentops-0zh").
//   - Task: the bead-derived task section (already-formatted markdown, will be
//     embedded as-is). The caller is responsible for trimming and shaping.
//   - Context: quest + repository context section. Same contract as Task.
//   - Learnings: relevant learnings section, pre-formatted by the caller.
//   - Navigation: pointer to repo AGENTS.md / discovery hints, pre-formatted.
//   - LearningIDs: stable IDs for the learnings included in [Learnings]; the
//     pure metadata output echoes these so persistence layers can store the
//     manifest without re-deriving it.
//   - CharsPerToken: divisor used for the rough token-count estimate. Defaults
//     to 4 (matching the upstream Hermes heuristic) if zero or negative.
type BriefRenderInput struct {
	BeadID        string
	Task          string
	Context       string
	Learnings     string
	Navigation    string
	LearningIDs   []string
	CharsPerToken int
}

// BriefRenderOutput is the pure output of [Render]: a fully-rendered markdown
// briefing plus the structural metadata a persistence layer would need. No
// timestamps, no IDs minted here — those are side-effectful concerns and belong
// to the thin shell that wraps this function.
//
// Fields:
//   - Markdown: the canonical rendered briefing. Persistence layers write this
//     verbatim; tests assert against it byte-for-byte.
//   - BeadID: echoed from the input for downstream convenience.
//   - LearningIDs: snapshot of the IDs embedded in the rendered markdown.
//   - TokenCount: rough estimate (len(Markdown) / CharsPerToken) used by
//     budget-aware callers; not authoritative.
type BriefRenderOutput struct {
	Markdown    string
	BeadID      string
	LearningIDs []string
	TokenCount  int
}

// Render assembles a four-section briefing markdown document from the supplied
// input and returns it together with structural metadata. It is a pure
// function: no I/O, no globals, no clock or environment access. Identical
// inputs produce identical outputs.
//
// This is a port of the post-Feb 24 olympus/hermes pattern (see
// olympus/hermes/brief/assembler.go::Assembler.Render and
// olympus/hermes/brief/template.go::RenderBriefing) which collapsed a 298 LOC
// briefing path down to ~79 LOC by separating pure rendering from persistence.
// The same split eliminates context divergence between heroEmbark and
// contextBuild paths in agentopsd.
//
// Render is a pure-function variant of context briefing assembly. Currently
// exposed for future agentopsd integration; integration TBD.
func Render(input BriefRenderInput) (BriefRenderOutput, error) {
	beadID := strings.TrimSpace(input.BeadID)
	if beadID == "" {
		return BriefRenderOutput{}, errors.New("brief render: BeadID is required")
	}

	charsPerToken := input.CharsPerToken
	if charsPerToken <= 0 {
		charsPerToken = 4
	}

	task := defaultIfBlank(input.Task, "No task content provided.")
	context := defaultIfBlank(input.Context, "No context provided.")
	learnings := defaultIfBlank(input.Learnings, "No relevant learnings found.")
	navigation := defaultIfBlank(input.Navigation, "No AGENTS.md files found in repository.")

	markdown := fmt.Sprintf(briefingTemplate, beadID, task, context, learnings, navigation)

	// Defensive copy of LearningIDs so callers can't mutate the output through
	// the input slice. Empty input -> nil output (not []string{}); tests assert
	// this distinction.
	var ids []string
	if len(input.LearningIDs) > 0 {
		ids = make([]string, len(input.LearningIDs))
		copy(ids, input.LearningIDs)
	}

	return BriefRenderOutput{
		Markdown:    markdown,
		BeadID:      beadID,
		LearningIDs: ids,
		TokenCount:  len(markdown) / charsPerToken,
	}, nil
}

// briefingTemplate is the canonical four-section briefing layout. Mirrors
// olympus/hermes/brief/template.go::RenderBriefing.
const briefingTemplate = `# Briefing: %s

## Task
%s

## Context
%s

## Learnings
%s

## Navigation
%s
`

// RenderAndPersist is the thin shell wrapper around [Render]: it computes the
// pure briefing, then writes the rendered markdown to dst, creating any
// missing parent directories. The returned [BriefRenderOutput] is exactly what
// [Render] produced — persistence does not mutate it.
//
// This is the persistence sibling to [Render], analogous to the Brief() shell
// in olympus/hermes/brief/assembler.go which calls Render() then atomically
// writes the result. We keep it deliberately thin: no extra metadata, no
// retries, no atomic-rename fanciness — callers who need those wrap this.
//
// dst must be a non-empty filesystem path. The file is written with mode 0644
// and parent directories are created with mode 0755.
//
// RenderAndPersist is currently exposed for future agentopsd integration;
// integration TBD.
func RenderAndPersist(input BriefRenderInput, dst string) (BriefRenderOutput, error) {
	out, err := Render(input)
	if err != nil {
		return BriefRenderOutput{}, fmt.Errorf("brief render: %w", err)
	}

	dst = strings.TrimSpace(dst)
	if dst == "" {
		return BriefRenderOutput{}, errors.New("brief render: dst path is required")
	}

	if parent := filepath.Dir(dst); parent != "" && parent != "." {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return BriefRenderOutput{}, fmt.Errorf("brief render: creating parent dir %q: %w", parent, err)
		}
	}

	if err := os.WriteFile(dst, []byte(out.Markdown), 0o644); err != nil {
		return BriefRenderOutput{}, fmt.Errorf("brief render: writing %q: %w", dst, err)
	}

	return out, nil
}

// defaultIfBlank returns fallback when value is empty or whitespace-only,
// otherwise returns value unchanged. Kept private; not part of the package
// surface.
func defaultIfBlank(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
