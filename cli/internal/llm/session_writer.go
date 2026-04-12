package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SessionMeta is the pipeline input used to build a session markdown page.
type SessionMeta struct {
	SessionID     string
	SourceJSONL   string
	Workspace     string
	StartedAt     time.Time
	EndedAt       time.Time
	Turns         int
	TokensIn      int
	TokensOut     int
	ModelPrimary  string
	Status        string
	Tier          int
	Model         string
	ModelDigest   string
	Confidence    float64
	IngestedAt    time.Time
	IngestedBy    string
	ParentSession string
	FirstSeen     time.Time
	ReviewedAt    time.Time
	ReviewedBy    string
}

// SessionFrontmatter is the YAML contract for .agents/ao/sessions/*.md files.
// Required fields are marshaled unconditionally; optional fields are omitted
// when empty.
type SessionFrontmatter struct {
	Type          string
	SessionID     string
	SourceJSONL   string
	Workspace     string
	StartedAt     time.Time
	EndedAt       time.Time
	Turns         int
	TokensIn      int
	TokensOut     int
	ModelPrimary  string
	Status        string
	Tier          int
	Model         string
	ModelDigest   string
	Confidence    float64
	IngestedAt    time.Time
	IngestedBy    string
	ParentSession string
	FirstSeen     time.Time
	ReviewedAt    time.Time
	ReviewedBy    string
}

// SessionPage is the rendered markdown artifact for one summarized session.
type SessionPage struct {
	Frontmatter SessionFrontmatter
	Notes       []ChunkNote
}

// BuildPage applies v1 defaults and packages notes into a markdown page.
func BuildPage(meta SessionMeta, notes []ChunkNote) SessionPage {
	now := time.Now().UTC()
	if meta.Status == "" {
		meta.Status = "draft"
	}
	if meta.Tier == 0 {
		meta.Tier = 1
	}
	if meta.IngestedAt.IsZero() {
		meta.IngestedAt = now
	}
	if meta.FirstSeen.IsZero() {
		meta.FirstSeen = meta.IngestedAt
	}
	if meta.Confidence <= 0 {
		meta.Confidence = noteConfidence(notes)
	}

	return SessionPage{
		Frontmatter: SessionFrontmatter{
			Type:          "session",
			SessionID:     meta.SessionID,
			SourceJSONL:   meta.SourceJSONL,
			Workspace:     meta.Workspace,
			StartedAt:     meta.StartedAt,
			EndedAt:       meta.EndedAt,
			Turns:         meta.Turns,
			TokensIn:      meta.TokensIn,
			TokensOut:     meta.TokensOut,
			ModelPrimary:  meta.ModelPrimary,
			Status:        meta.Status,
			Tier:          meta.Tier,
			Model:         meta.Model,
			ModelDigest:   meta.ModelDigest,
			Confidence:    meta.Confidence,
			IngestedAt:    meta.IngestedAt,
			IngestedBy:    meta.IngestedBy,
			ParentSession: meta.ParentSession,
			FirstSeen:     meta.FirstSeen,
			ReviewedAt:    meta.ReviewedAt,
			ReviewedBy:    meta.ReviewedBy,
		},
		Notes: notes,
	}
}

// Render returns the full markdown page with YAML frontmatter and wikilinked notes.
func (p SessionPage) Render() string {
	var b strings.Builder
	p.renderFrontmatter(&b)
	fmt.Fprintf(&b, "# Session %s\n\n", p.Frontmatter.SessionID)
	for _, note := range p.Notes {
		p.renderNote(&b, note)
	}
	return b.String()
}

func (p SessionPage) renderFrontmatter(b *strings.Builder) {
	fm := p.Frontmatter
	b.WriteString("---\n")
	requiredStringField(b, "type", fm.Type)
	requiredStringField(b, "session_id", fm.SessionID)
	requiredStringField(b, "source_jsonl", fm.SourceJSONL)
	requiredStringField(b, "workspace", fm.Workspace)
	requiredTimeField(b, "started_at", fm.StartedAt)
	requiredTimeField(b, "ended_at", fm.EndedAt)
	requiredIntField(b, "turns", fm.Turns)
	requiredIntField(b, "tokens_in", fm.TokensIn)
	requiredIntField(b, "tokens_out", fm.TokensOut)
	requiredStringField(b, "model_primary", fm.ModelPrimary)
	requiredStringField(b, "status", fm.Status)
	requiredIntField(b, "tier", fm.Tier)
	requiredStringField(b, "model", fm.Model)
	requiredStringField(b, "model_digest", fm.ModelDigest)
	requiredFloatField(b, "confidence", fm.Confidence)
	requiredTimeField(b, "ingested_at", fm.IngestedAt)
	requiredStringField(b, "ingested_by", fm.IngestedBy)
	optionalStringField(b, "parent_session", fm.ParentSession)
	requiredTimeField(b, "first_seen", fm.FirstSeen)
	optionalTimeField(b, "reviewed_at", fm.ReviewedAt)
	optionalStringField(b, "reviewed_by", fm.ReviewedBy)
	b.WriteString("---\n\n")
}

func (p SessionPage) renderNote(b *strings.Builder, note ChunkNote) {
	if note.Skipped {
		fmt.Fprintf(b, "### Turn %d skipped\n\n", note.Index)
		return
	}
	heading := strings.TrimSpace(note.Intent)
	if heading == "" {
		heading = fmt.Sprintf("Turn %d", note.Index)
	}
	fmt.Fprintf(b, "### %s\n\n", heading)
	if summary := strings.TrimSpace(note.Summary); summary != "" {
		b.WriteString(summary)
		b.WriteString("\n\n")
	}
	if assistant := strings.TrimSpace(note.AssistantCondensed); assistant != "" {
		b.WriteString("**Assistant condensed:** ")
		b.WriteString(assistant)
		b.WriteString("\n\n")
	}
	if len(note.Entities) > 0 {
		b.WriteString("**Entities:**\n")
		for _, entity := range note.Entities {
			entity = strings.TrimSpace(entity)
			if entity == "" {
				continue
			}
			fmt.Fprintf(b, "- [[%s]]\n", entity)
		}
		b.WriteString("\n")
	}
}

// WriteSessionPage writes a page through a temp file in the destination
// directory, then renames it into place.
func WriteSessionPage(path string, page SessionPage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-session-*.md")
	if err != nil {
		return fmt.Errorf("create tempfile: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.WriteString(page.Render()); err != nil {
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
		return fmt.Errorf("atomic rename %s -> %s: %w", tmpPath, path, err)
	}
	return nil
}

func requiredStringField(b *strings.Builder, key, value string) {
	fmt.Fprintf(b, "%s: %s\n", key, cleanScalar(strings.TrimSpace(value)))
}

func optionalStringField(b *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	fmt.Fprintf(b, "%s: %s\n", key, cleanScalar(value))
}

func requiredTimeField(b *strings.Builder, key string, value time.Time) {
	fmt.Fprintf(b, "%s: %s\n", key, value.UTC().Format(time.RFC3339))
}

func optionalTimeField(b *strings.Builder, key string, value time.Time) {
	if value.IsZero() {
		return
	}
	fmt.Fprintf(b, "%s: %s\n", key, value.UTC().Format(time.RFC3339))
}

func requiredIntField(b *strings.Builder, key string, value int) {
	fmt.Fprintf(b, "%s: %d\n", key, value)
}

func requiredFloatField(b *strings.Builder, key string, value float64) {
	fmt.Fprintf(b, "%s: %g\n", key, value)
}

func cleanScalar(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func noteConfidence(notes []ChunkNote) float64 {
	if len(notes) == 0 {
		return 0.01
	}
	var kept int
	for _, note := range notes {
		if !note.Skipped {
			kept++
		}
	}
	if kept == 0 {
		return 0.01
	}
	return float64(kept) / float64(len(notes))
}
