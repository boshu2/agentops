// practices: [fail-closed-safety, wiki-knowledge-surface]
package corpus

import "strings"

// The two promote-gate fields S3 (epic ag-k7tq9) adds to every learning's
// frontmatter. They are the CEILING the publish pipeline (S5/S6) reads — NOT a
// capture property. Per the unanimous cross-family council verdict
// (.agents/council/2026-06-15-corpus-private-public-seam-verdict.md): the corpus
// is lossless and private-by-default; sensitivity is decided at promote time.
//
// Allowlist, fail-closed: only sensitivity==SensitivityPublic AND
// publishable==true may cross the seam into docs/wiki. Default excludes.
const (
	// SensitivityField is the frontmatter key for the publish-gate ceiling.
	SensitivityField = "sensitivity"
	// PublishableField is the frontmatter key for the promotion allowlist flag.
	PublishableField = "publishable"

	// SensitivityDefault is the safe default for an un-triaged learning: nothing
	// is publishable until it is affirmatively reviewed and cleared.
	SensitivityDefault = "unknown"
	// PublishableDefault is the safe default: inclusion is earned, never assumed.
	PublishableDefault = "false"
)

// metaFilenames are corpus files that are NOT learning records and must never
// be annotated (policy/readme/index docs that live alongside the learnings).
var metaFilenames = map[string]bool{
	"CORPUS-POLICY.md": true,
	"README.md":        true,
	"MEMORY.md":        true,
	"INDEX.md":         true,
}

// IsLearningFile reports whether a corpus file (identified by its base name)
// is a learning record eligible for classification. Meta/policy/index docs are
// excluded.
func IsLearningFile(base string) bool {
	return !metaFilenames[base]
}

// AnnotateLearning ensures a learning's YAML frontmatter carries the two
// promote-gate fields with safe defaults, returning the (possibly) rewritten
// content and whether anything changed.
//
// It is deliberately MALFORMED-TOLERANT: it operates on the `---` frontmatter
// fence textually and never parses the (possibly broken) YAML body. The corpus
// holds hand-edited and machine-extracted records with inconsistent frontmatter
// (the migration must not crash on a single junk record — council risk note).
//
// Behavior:
//   - File opens with a `---` fence: any of the two keys that is absent from the
//     frontmatter region is inserted just before the closing fence (or, if no
//     closing fence exists, right after the opening fence). An already-present
//     key (any value) is left untouched — defaults never clobber a real decision.
//   - File has no opening fence: a minimal frontmatter block carrying both
//     defaults is prepended.
//
// Key order is stable (sensitivity, then publishable) for deterministic diffs.
func AnnotateLearning(content string) (string, bool) {
	defaults := []struct{ key, val string }{
		{SensitivityField, SensitivityDefault},
		{PublishableField, PublishableDefault},
	}

	// Normalize on \n for line work; the corpus is unix-newline.
	lines := strings.Split(content, "\n")

	if len(lines) == 0 || lines[0] != "---" {
		// No frontmatter — prepend a minimal block with both defaults.
		var b strings.Builder
		b.WriteString("---\n")
		for _, d := range defaults {
			b.WriteString(d.key + ": " + d.val + "\n")
		}
		b.WriteString("---\n\n")
		b.WriteString(content)
		return b.String(), true
	}

	// Find the closing fence (first bare `---` after line 0).
	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			closeIdx = i
			break
		}
	}

	// The frontmatter region is lines[1:closeIdx] (or lines[1:] if no close).
	regionEnd := closeIdx
	if regionEnd == -1 {
		regionEnd = len(lines)
	}
	present := map[string]bool{}
	for i := 1; i < regionEnd; i++ {
		if k, ok := frontmatterKey(lines[i]); ok {
			present[k] = true
		}
	}

	var missing []string
	for _, d := range defaults {
		if !present[d.key] {
			missing = append(missing, d.key+": "+d.val)
		}
	}
	if len(missing) == 0 {
		return content, false
	}

	// Insert before the closing fence, or right after the opening fence when the
	// file has no closing fence (malformed — keep the keys inside the header zone).
	insertAt := closeIdx
	if insertAt == -1 {
		insertAt = 1
	}
	out := make([]string, 0, len(lines)+len(missing))
	out = append(out, lines[:insertAt]...)
	out = append(out, missing...)
	out = append(out, lines[insertAt:]...)
	return strings.Join(out, "\n"), true
}

// frontmatterKey extracts the bare YAML key from a frontmatter line, if the line
// is a top-level `key: ...` (or `key:`) mapping entry. Indented lines (block
// values, list items) return ok=false so a nested `sensitivity:` never counts.
func frontmatterKey(line string) (string, bool) {
	if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' || line[0] == '-' {
		return "", false
	}
	idx := strings.IndexByte(line, ':')
	if idx <= 0 {
		return "", false
	}
	return line[:idx], true
}
