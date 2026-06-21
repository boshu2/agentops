package wiki

// Deterministic, SAFE --fix repairs for the OpenKB structural health checks
// (age-port-openkb-into-agentops-go-5qw.4).
//
// FixWiki performs ONLY the narrow, lossless repair the bead sanctions:
// stripping a dangling [[dir/slug]] wikilink whose target page does not exist.
// It NEVER deletes a page, never touches a valid link, never rewrites
// frontmatter, and is idempotent (a second run is a no-op). Any defect that
// cannot be safely auto-fixed (invalid frontmatter, missing fields, orphans) is
// reported by CheckWiki but left untouched here — repair is read-additive only
// in the sense of removing a single broken reference, never lossy.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FixResult summarizes what FixWiki repaired.
type FixResult struct {
	// Workspace is the resolved workspace root.
	Workspace string `json:"workspace"`
	// LinksStripped is the total number of dangling wikilinks removed.
	LinksStripped int `json:"links_stripped"`
	// PagesModified is the number of page files rewritten.
	PagesModified int `json:"pages_modified"`
	// Repairs is the per-page detail: which page lost which broken target.
	Repairs []LinkRepair `json:"repairs,omitempty"`
}

// LinkRepair records one stripped dangling link.
type LinkRepair struct {
	// Page is the page key the broken link was removed FROM.
	Page string `json:"page"`
	// Target is the dangling wikilink target that was removed.
	Target string `json:"target"`
}

// FixWiki applies the safe deterministic repairs to the workspace's wiki tree
// and returns a summary. It is idempotent: only dangling links are stripped, so
// a clean (or already-repaired) wiki is a no-op.
func FixWiki(workspace string) (FixResult, error) {
	pages, err := loadWikiPages(workspace)
	if err != nil {
		return FixResult{}, err
	}
	existing, err := linkTargetSet(workspace)
	if err != nil {
		return FixResult{}, err
	}
	result := FixResult{Workspace: workspace}

	for _, p := range pages {
		data, err := os.ReadFile(p.path) //nolint:gosec // path is wiki-tree-bounded
		if err != nil {
			return FixResult{}, fmt.Errorf("read page %s: %w", p.key, err)
		}
		fixed, stripped := stripBrokenLinks(string(data), existing)
		if len(stripped) == 0 {
			continue
		}
		if err := atomicRewrite(p.path, fixed); err != nil {
			return FixResult{}, fmt.Errorf("rewrite page %s: %w", p.key, err)
		}
		result.PagesModified++
		result.LinksStripped += len(stripped)
		for _, target := range stripped {
			result.Repairs = append(result.Repairs, LinkRepair{Page: p.key, Target: target})
		}
	}
	return result, nil
}

// bulletOnlyLinkPattern matches a list-item line whose ONLY content is a single
// wikilink (with optional trailing punctuation): "- [[entities/x]]". Such a line
// is removed whole when the link is broken, so no empty bullet is left behind.
var bulletOnlyLinkPattern = regexp.MustCompile(`^\s*[-*]\s+(\[\[[A-Za-z0-9][A-Za-z0-9_/.-]*\]\])\s*[.,;:]?\s*$`)

// stripBrokenLinks removes every dangling [[dir/slug]] reference from text and
// returns the cleaned text plus the list of stripped targets (in document
// order, with duplicates per occurrence). A wikilink is dangling iff its target
// key is absent from existing.
//
// Two cases, in this order:
//
//  1. A bullet line whose sole content is the broken link → the whole line is
//     dropped (else an empty "- " bullet would remain).
//  2. Otherwise → only the [[...]] token is removed, and the resulting double
//     space (if any) is collapsed so the prose stays clean.
func stripBrokenLinks(text string, existing map[string]bool) (string, []string) {
	var stripped []string

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		// Case 1: bullet-only broken link → drop the line.
		if m := bulletOnlyLinkPattern.FindStringSubmatch(line); m != nil {
			target := wikilinkTarget(m[1])
			if !existing[target] {
				stripped = append(stripped, target)
				continue // drop the whole bullet line
			}
		}
		// Case 2: strip dangling inline tokens, keep the line. The match captures
		// the token plus AT MOST ONE adjacent space on each side, so the gap left
		// by removal is closed LOCALLY — any other whitespace on the line
		// (intentional alignment, code spacing) is never touched. A global
		// whitespace collapse here was the data-safety bug: it mangled spacing far
		// from the removed link.
		cleaned := inlineLinkSpan.ReplaceAllStringFunc(line, func(m string) string {
			sub := inlineLinkSpan.FindStringSubmatch(m)
			lead, tok, trail := sub[1], sub[2], sub[3]
			if existing[wikilinkTarget(tok)] {
				return m // valid link — returned verbatim, spaces preserved
			}
			stripped = append(stripped, wikilinkTarget(tok))
			// Remove the token plus exactly ONE of its OWN adjacent spaces (leading
			// if present, else trailing), keeping the other. This closes the gap
			// the link occupied — "a [[broken]] b" → "a b", "a [[broken]]" → "a" —
			// without touching any other whitespace on the line (no global trim).
			if lead != "" {
				return trail
			}
			return ""
		})
		out = append(out, cleaned)
	}
	return strings.Join(out, "\n"), stripped
}

// wikilinkTarget extracts the "dir/slug" target from a "[[dir/slug]]" token.
func wikilinkTarget(tok string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(tok, "[["), "]]"))
}

// inlineLinkSpan matches a wikilink token plus AT MOST ONE immediately adjacent
// space on each side. Removing a broken link via this span closes the gap it
// occupied without disturbing any other whitespace on the line — the data-safety
// property a global collapse violated.
var inlineLinkSpan = regexp.MustCompile(`( ?)(\[\[[A-Za-z0-9][A-Za-z0-9_/.-]*\]\])( ?)`)

// atomicRewrite writes contents to path via a temp file + rename so a crash
// never leaves a torn page. The destination directory is assumed to exist (we
// only ever rewrite files we just read).
func atomicRewrite(path, contents string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".wiki-fix-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op after a successful rename
	if _, err := tmp.WriteString(contents); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil { //nolint:gosec // wiki pages are world-readable like the compile stage's 0644 artifacts
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp into place: %w", err)
	}
	return nil
}
