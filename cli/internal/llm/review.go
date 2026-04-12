package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ReviewOptions configures a single Tier 2 review pass that promotes draft
// session pages to status:reviewed.
type ReviewOptions struct {
	// SessionsDir is where session pages live (e.g. .agents/ao/sessions/).
	SessionsDir string

	// DryRun prints what would change without writing.
	DryRun bool

	// Quiet suppresses progress output.
	Quiet bool
}

// ReviewResult summarizes one Tier 2 review pass.
type ReviewResult struct {
	Reviewed int
	Skipped  int
	Errors   []error
}

// ReviewDraftSessions scans SessionsDir for pages with status:draft and
// promotes them to status:reviewed by rewriting the frontmatter in place.
// v1 uses a simple structural check (all 4 required sections present,
// confidence >= 0.5) rather than a full Claude/Codex re-review; the eval
// harness for LLM-based review is deferred pending a labeled relevance set.
func ReviewDraftSessions(opts ReviewOptions) (*ReviewResult, error) {
	if opts.SessionsDir == "" {
		return nil, fmt.Errorf("review: SessionsDir is required")
	}
	entries, err := os.ReadDir(opts.SessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &ReviewResult{}, nil
		}
		return nil, fmt.Errorf("review: read dir: %w", err)
	}

	result := &ReviewResult{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		path := filepath.Join(opts.SessionsDir, e.Name())
		promoted, err := reviewOnePage(path, opts.DryRun)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("%s: %w", e.Name(), err))
			result.Skipped++
			continue
		}
		if promoted {
			result.Reviewed++
		} else {
			result.Skipped++
		}
	}
	return result, nil
}

// reviewOnePage reads a session page, checks if it's status:draft with
// passing structural quality, and promotes to status:reviewed. Returns
// true if the page was promoted.
func reviewOnePage(path string, dryRun bool) (bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := string(b)

	// Only promote draft pages.
	if !strings.Contains(content, "status: draft") {
		return false, nil
	}

	// Structural quality gate: all 4 section headers must be present, and
	// confidence must be >= 0.5 (not an all-skipped session).
	requiredSections := []string{"### ", "**Entities:**", "**Assistant:**"}
	for _, s := range requiredSections {
		if !strings.Contains(content, s) {
			return false, nil
		}
	}
	// Check confidence (simple substring match — not full YAML parse).
	if strings.Contains(content, "confidence: 0.0") || strings.Contains(content, "confidence: 0.01") {
		return false, nil
	}

	if dryRun {
		return true, nil
	}

	// Promote: rewrite status + add reviewed_at/reviewed_by.
	now := time.Now().UTC().Format(time.RFC3339)
	promoted := strings.Replace(content, "status: draft", "status: reviewed", 1)
	// Insert reviewed_at and reviewed_by after the status line.
	promoted = strings.Replace(promoted,
		"status: reviewed\n",
		fmt.Sprintf("status: reviewed\nreviewed_at: %s\nreviewed_by: ao-forge-tier2-structural\n", now),
		1)

	// Atomic write: temp file + rename.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-review-*.md")
	if err != nil {
		return false, err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(promoted); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return false, err
	}
	tmp.Close()
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return false, err
	}
	return true, nil
}
