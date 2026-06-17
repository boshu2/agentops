// practices: [wiki-knowledge-surface, lean-startup]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/extract"
	"github.com/boshu2/agentops/cli/internal/provenancegraph"
	"github.com/boshu2/agentops/cli/internal/storage"
)

// Typed-extraction opt-in for forge (age-2jf, VALUE-GATED).
//
// This is the explicit opt-in path that routes forge knowledge through the
// native typed extraction engine (cli/internal/extract) to emit
// learning.v1-schema-valid records, instead of the heuristic regex
// parser.Extractor path that writePendingLearnings consumes.
//
// IT IS OFF BY DEFAULT and MUST STAY OFF until the value gate
// (age-kf-s1-close-loop-0ly.4) measures a positive A/B delta. The default
// forge output is byte-for-byte unchanged: when the opt-in is not set, none of
// this code runs and writePendingLearnings (the existing heuristic writer) is
// used verbatim. See TestForge_DefaultUnchangedUntilGate.
//
// The opt-in is exposed two ways (either enables it):
//   - the --typed flag on `ao forge transcript`
//   - the AGENTOPS_FORGE_TYPED=1 environment variable
//
// Fallback: if the typed path is off OR the typed extraction fails (e.g. the
// LLM backend is unavailable / a hard transport error), the heuristic
// parser.Extractor path is retained — callers fall back to
// writePendingLearnings.

// forgeTyped is bound to the --typed flag on `ao forge transcript`.
var forgeTyped bool

// forgeTypedEnv is the environment-variable opt-in name.
const forgeTypedEnv = "AGENTOPS_FORGE_TYPED"

// forgeTypedClient is the seam that constructs the typed extraction client.
// It is a package var so tests can inject a fake-Generator-backed client (no
// live model). In PRODUCTION it returns a real LAW-0 codex-backed client
// (extract.NewCodexClient(runCodexExec)) — BackendCodex is on the closed
// allowlist; a Claude print-mode backend is never wired here.
var forgeTypedClient = func() *extract.Client { return extract.NewCodexClient(runCodexExec) }

// learningIDSlugRe strips a node id down to the [a-z0-9]+ tail the
// learning.v1 learning_id pattern (^learn-YYYY-MM-DD-[a-z0-9]+$) permits.
var learningIDSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// typedExtractionEnabled reports whether the typed opt-in is active. It is the
// SINGLE gate: false unless the --typed flag was set OR AGENTOPS_FORGE_TYPED is
// a truthy value. Default (neither set) is OFF — the value gate working as
// designed.
func typedExtractionEnabled() bool {
	if forgeTyped {
		return true
	}
	switch strings.TrimSpace(os.Getenv(forgeTypedEnv)) {
	case "1", "true", "TRUE", "yes", "on":
		return true
	default:
		return false
	}
}

// typedLearningRecord is the learning.v1-schema-valid record the typed opt-in
// path emits. Field tags match schemas/learning.v1.schema.json exactly so a
// json.Marshal of this struct validates against that schema.
type typedLearningRecord struct {
	LearningID    string  `json:"learning_id"`
	Content       string  `json:"content"`
	Category      string  `json:"category"`
	Confidence    float64 `json:"confidence"`
	UtilityScore  int     `json:"utility_score"`
	BriefingCount int     `json:"briefing_count"`
	SourceSession string  `json:"source_session"`
	SourceBead    *string `json:"source_bead,omitempty"`
	CreatedAt     string  `json:"created_at"`
	Tombstoned    bool    `json:"tombstoned"`
	SchemaVersion int     `json:"schema_version"`
}

// nodeTypeToCategory maps a provenance template node_type onto the closed
// learning.v1 category enum {architecture, debugging, process, testing,
// security}. The mapping is intentionally conservative — anything that is not
// clearly architecture falls back to "process" (the broadest bucket) rather
// than emitting a value outside the enum.
func nodeTypeToCategory(nodeType string) string {
	switch strings.ToLower(strings.TrimSpace(nodeType)) {
	case "artifact":
		return "architecture"
	default:
		// decision, finding, learning, bead, and anything unrecognized.
		return "process"
	}
}

// recordString returns the string value of a Record field, or "" when absent
// or non-string.
func typedRecordString(r extract.Record, field string) string {
	v, ok := r[field]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

// buildTypedLearnings converts a typed extraction Result's entity records into
// learning.v1-schema-valid records for the given session/date. Records whose
// content (summary, then id) is empty are skipped — content has minLength 1.
func buildTypedLearnings(res *extract.Result, sessionID string, date time.Time) []typedLearningRecord {
	if res == nil {
		return nil
	}
	dateStr := date.Format("2006-01-02")
	createdAt := date.Format(time.RFC3339)
	if date.IsZero() {
		now := time.Now().UTC()
		dateStr = now.Format("2006-01-02")
		createdAt = now.Format(time.RFC3339)
	}

	out := make([]typedLearningRecord, 0, len(res.Entities))
	for i, ent := range res.Entities {
		summary := typedRecordString(ent, "summary")
		id := typedRecordString(ent, "id")
		content := summary
		if content == "" {
			content = id
		}
		if content == "" {
			// minLength 1 — nothing to record.
			continue
		}
		slug := learningIDSlugRe.ReplaceAllString(strings.ToLower(id), "")
		if slug == "" {
			slug = fmt.Sprintf("e%d", i+1)
		}
		out = append(out, typedLearningRecord{
			LearningID:    fmt.Sprintf("learn-%s-%s", dateStr, slug),
			Content:       content,
			Category:      nodeTypeToCategory(typedRecordString(ent, "node_type")),
			Confidence:    0.5,
			UtilityScore:  0,
			BriefingCount: 0,
			SourceSession: sessionID,
			CreatedAt:     createdAt,
			Tombstoned:    false,
			SchemaVersion: 1,
		})
	}
	return out
}

// sessionKnowledgeText joins a session's knowledge and decisions into a single
// text blob for the extractor to read.
func sessionKnowledgeText(session *storage.Session) string {
	var b strings.Builder
	for _, d := range session.Decisions {
		if s := strings.TrimSpace(d); s != "" {
			b.WriteString(s)
			b.WriteString("\n\n")
		}
	}
	for _, k := range session.Knowledge {
		if s := strings.TrimSpace(k); s != "" {
			b.WriteString(s)
			b.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(b.String())
}

// writeTypedPendingLearnings is the typed opt-in counterpart to
// writePendingLearnings. It runs the session's accumulated knowledge through
// the native extraction engine (provenance template + injected client) and
// writes one learning.v1-schema-valid JSON record per surviving entity to
// .agents/knowledge/pending/<id>.learning.json.
//
// It returns the number of records written. A non-nil error means the
// extraction itself failed hard (e.g. nil client, schema compile, or a
// transport error) — callers treat that as "typed unavailable" and fall back
// to the heuristic writePendingLearnings path.
func writeTypedPendingLearnings(session *storage.Session, baseDir string, client *extract.Client) (int, error) {
	if session == nil {
		return 0, nil
	}
	if client == nil {
		return 0, fmt.Errorf("typed extraction: nil client (backend unavailable)")
	}

	text := sessionKnowledgeText(session)
	if text == "" {
		return 0, nil
	}

	tmpl, err := extract.LoadProvenanceTemplate()
	if err != nil {
		return 0, fmt.Errorf("typed extraction: load template: %w", err)
	}

	res, err := extract.Extract(context.Background(), text, tmpl, client)
	if err != nil {
		return 0, fmt.Errorf("typed extraction: %w", err)
	}

	// Seal each extracted relation as a PROV-O edge in the provenance ledger.
	// The graph is no longer faked: relations -> provenancegraph edges via
	// extract.AppendRelation, reusing the store's idempotency + sealing as-is.
	// Individual bad edges are skipped+logged; one bad edge never fails the
	// whole forge run (callers still get the learning records).
	sealTypedRelations(res, baseDir)

	records := buildTypedLearnings(res, session.ID, session.Date)
	if len(records) == 0 {
		return 0, nil
	}

	pendingDir := filepath.Join(baseDir, ".agents", "knowledge", "pending")
	if err := os.MkdirAll(pendingDir, 0700); err != nil {
		return 0, fmt.Errorf("create pending dir: %w", err)
	}

	written := 0
	for _, rec := range records {
		data, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			return written, fmt.Errorf("marshal typed learning %s: %w", rec.LearningID, err)
		}
		filename := sanitizePathComponent(rec.LearningID) + ".learning.json"
		path := filepath.Join(pendingDir, filename)
		if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
			return written, fmt.Errorf("write %s: %w", filename, err)
		}
		written++
	}
	return written, nil
}

// sealTypedRelations seals each extracted relation as a PROV-O edge in the
// committed provenance ledger (docs/provenance/ledger.jsonl, resolved under
// baseDir). It builds a nodeTypes map (entity id -> node_type) from the
// extraction's entities so the edge adapter can resolve endpoint node types,
// then appends each relation via extract.AppendRelation with the "mined" trust
// tier. A nil result, no relations, or a nil store is a no-op. Individual
// relation errors (e.g. a non-PROV-O verb, missing endpoint id) are skipped and
// logged — one bad edge never aborts the forge run.
func sealTypedRelations(res *extract.Result, baseDir string) {
	if res == nil || len(res.Relations) == 0 {
		return
	}

	nodeTypes := make(map[string]string, len(res.Entities))
	for _, ent := range res.Entities {
		id := typedRecordString(ent, "id")
		if id == "" {
			continue
		}
		nodeTypes[id] = typedRecordString(ent, "node_type")
	}

	store := provenancegraph.NewStore(filepath.Join(baseDir, provenancegraph.LedgerRelativePath))
	for _, rel := range res.Relations {
		if _, err := extract.AppendRelation(store, rel, nodeTypes, "mined"); err != nil {
			fmt.Fprintf(os.Stderr, "ao forge: skip relation edge: %v\n", err)
		}
	}
}
