package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SessionFrontmatter is the 17-field frontmatter schema for session pages,
// enumerated in the jsonl-to-llm-wiki plan (council S6 / ARCH-3).
//
// Required fields are marshaled unconditionally; optional fields
// (ParentSession, ReviewedAt, ReviewedBy) are omitted when empty.
type SessionFrontmatter struct {
	// Required
	Type         string    // always "session"
	SessionID    string    // source .jsonl session ID
	SourceJSONL  string    // absolute path to source file
	Workspace    string    // cwd captured from session
	StartedAt    time.Time // first message timestamp
	EndedAt      time.Time // last message timestamp
	Turns        int       // number of user→assistant pairs
	TokensIn     int       // sum of input tokens across assistant messages
	TokensOut    int       // sum of output tokens
	ModelPrimary string    // claude model used in the source session
	Status       string    // "draft" | "reviewed" | "promoted"
	Tier         int       // 1 = local LLM, 2 = Claude/Codex, 3 = human
	Model        string    // LLM that produced these notes (e.g. gemma2:9b)
	ModelDigest  string    // ollama digest of that model
	Confidence   float64   // 0.0-1.0
	IngestedAt   time.Time // when this page was written
	IngestedBy   string    // "ao-forge-tier1"
	FirstSeen    time.Time // same as IngestedAt on first pass

	// Optional
	ParentSession string    // set when this page supersedes another
	ReviewedAt    time.Time // set when Tier 2/3 reviewer touches it
	ReviewedBy    string    // reviewer identity
}

// SessionPage is a rendered session markdown file: frontmatter + body notes.
type SessionPage struct {
	Frontmatter SessionFrontmatter
	Notes       []ChunkNote
}

// SessionMeta is the subset of per-session metadata that BuildPage needs;
// the caller fills it from the parser output and runtime context.
type SessionMeta struct {
	SessionID    string
	SourceJSONL  string
	Workspace    string
	StartedAt    time.Time
	EndedAt      time.Time
	Turns        int
	TokensIn     int
	TokensOut    int
	ModelPrimary string
	Model        string
	ModelDigest  string
	IngestedBy   string
}

// BuildPage assembles a SessionPage from a SessionMeta + slice of ChunkNotes,
// filling defaults (status=draft, tier=1, ingested_at=now) and computing a
// confidence score based on the skip ratio.
func BuildPage(meta SessionMeta, notes []ChunkNote) SessionPage {
	now := time.Now().UTC()
	skipped := 0
	for _, n := range notes {
		if n.Skipped {
			skipped++
		}
	}
	var confidence float64
	if len(notes) > 0 {
		confidence = float64(len(notes)-skipped) / float64(len(notes))
	}
	if confidence == 0 && len(notes) > 0 {
		// All-skipped sessions still get a minimum nonzero confidence so the
		// page is not silently discarded downstream.
		confidence = 0.01
	}
	if len(notes) == 0 {
		confidence = 0.01
	}
	return SessionPage{
		Frontmatter: SessionFrontmatter{
			Type:         "session",
			SessionID:    meta.SessionID,
			SourceJSONL:  meta.SourceJSONL,
			Workspace:    meta.Workspace,
			StartedAt:    meta.StartedAt,
			EndedAt:      meta.EndedAt,
			Turns:        meta.Turns,
			TokensIn:     meta.TokensIn,
			TokensOut:    meta.TokensOut,
			ModelPrimary: meta.ModelPrimary,
			Status:       "draft",
			Tier:         1,
			Model:        meta.Model,
			ModelDigest:  meta.ModelDigest,
			Confidence:   confidence,
			IngestedAt:   now,
			IngestedBy:   meta.IngestedBy,
			FirstSeen:    now,
		},
		Notes: notes,
	}
}

// Render returns the markdown serialization of a SessionPage: YAML frontmatter
// followed by one H3 section per non-skipped chunk note.
func (p SessionPage) Render() string {
	var b strings.Builder
	b.WriteString("---\n")
	writeKV(&b, "type", p.Frontmatter.Type)
	writeKV(&b, "session_id", p.Frontmatter.SessionID)
	writeKV(&b, "source_jsonl", p.Frontmatter.SourceJSONL)
	writeKV(&b, "workspace", p.Frontmatter.Workspace)
	writeKV(&b, "started_at", p.Frontmatter.StartedAt.UTC().Format(time.RFC3339))
	writeKV(&b, "ended_at", p.Frontmatter.EndedAt.UTC().Format(time.RFC3339))
	writeKV(&b, "turns", fmt.Sprintf("%d", p.Frontmatter.Turns))
	writeKV(&b, "tokens_in", fmt.Sprintf("%d", p.Frontmatter.TokensIn))
	writeKV(&b, "tokens_out", fmt.Sprintf("%d", p.Frontmatter.TokensOut))
	writeKV(&b, "model_primary", p.Frontmatter.ModelPrimary)
	writeKV(&b, "status", p.Frontmatter.Status)
	writeKV(&b, "tier", fmt.Sprintf("%d", p.Frontmatter.Tier))
	writeKV(&b, "model", p.Frontmatter.Model)
	writeKV(&b, "model_digest", p.Frontmatter.ModelDigest)
	writeKV(&b, "confidence", fmt.Sprintf("%.2f", p.Frontmatter.Confidence))
	writeKV(&b, "ingested_at", p.Frontmatter.IngestedAt.UTC().Format(time.RFC3339))
	writeKV(&b, "ingested_by", p.Frontmatter.IngestedBy)
	writeKV(&b, "first_seen", p.Frontmatter.FirstSeen.UTC().Format(time.RFC3339))
	// Optional fields — omit when empty.
	if p.Frontmatter.ParentSession != "" {
		writeKV(&b, "parent_session", p.Frontmatter.ParentSession)
	}
	if !p.Frontmatter.ReviewedAt.IsZero() {
		writeKV(&b, "reviewed_at", p.Frontmatter.ReviewedAt.UTC().Format(time.RFC3339))
	}
	if p.Frontmatter.ReviewedBy != "" {
		writeKV(&b, "reviewed_by", p.Frontmatter.ReviewedBy)
	}
	b.WriteString("---\n\n")

	// Body: one H3 per chunk note. Skipped notes render as a compact line.
	b.WriteString("# Session notes\n\n")
	for _, note := range p.Notes {
		if note.Skipped {
			fmt.Fprintf(&b, "### Chunk %d — SKIP\n\n", note.Index)
			continue
		}
		fmt.Fprintf(&b, "### %s\n\n", note.Intent)
		if note.Summary != "" {
			b.WriteString(note.Summary)
			b.WriteString("\n\n")
		}
		if len(note.Entities) > 0 {
			b.WriteString("**Entities:**\n")
			for _, e := range note.Entities {
				fmt.Fprintf(&b, "- [[%s]]\n", e)
			}
			b.WriteString("\n")
		}
		if note.AssistantCondensed != "" {
			b.WriteString("**Assistant:** ")
			b.WriteString(note.AssistantCondensed)
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

// WriteSessionPage writes a rendered page to path atomically: the content is
// written to a sibling tempfile and os.Rename'd into place. Never leaves a
// partial file on failure (per pre-mortem F2).
func WriteSessionPage(path string, page SessionPage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	content := page.Render()
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-session-*.md")
	if err != nil {
		return fmt.Errorf("create tempfile: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("write tempfile: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return fmt.Errorf("sync tempfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close tempfile: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("atomic rename %s → %s: %w", tmpPath, path, err)
	}
	return nil
}

func writeKV(b *strings.Builder, key, value string) {
	// Simple scalar serialization: no quoting unless the value contains a
	// character that YAML would interpret. For our fields (paths, IDs,
	// timestamps) plain scalars are safe.
	fmt.Fprintf(b, "%s: %s\n", key, value)
}
