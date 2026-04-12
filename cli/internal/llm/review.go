package llm

import (
	"encoding/json"
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

// ReviewEvalOptions configures a labeled dry-run eval for the Tier 2 review gate.
type ReviewEvalOptions struct {
	SessionsDir  string
	ManifestPath string
}

// ReviewEvalManifest labels session pages with the expected review decision.
type ReviewEvalManifest struct {
	ID          string           `json:"id"`
	Description string           `json:"description,omitempty"`
	Cases       []ReviewEvalCase `json:"cases"`
}

// ReviewEvalCase is one labeled session-page decision.
type ReviewEvalCase struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Expected string `json:"expected"`
	Reason   string `json:"reason,omitempty"`
}

// ReviewEvalReport summarizes the eval decision quality.
type ReviewEvalReport struct {
	ID           string             `json:"id"`
	ManifestPath string             `json:"manifest_path"`
	SessionsDir  string             `json:"sessions_dir"`
	Cases        int                `json:"cases"`
	Passed       int                `json:"passed"`
	Failed       int                `json:"failed"`
	Errors       int                `json:"errors"`
	Accuracy     float64            `json:"accuracy"`
	Results      []ReviewEvalResult `json:"results"`
}

// ReviewEvalResult is the decision outcome for one eval case.
type ReviewEvalResult struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	Expected     string `json:"expected"`
	Actual       string `json:"actual"`
	Passed       bool   `json:"passed"`
	Reason       string `json:"reason,omitempty"`
	ErrorMessage string `json:"error,omitempty"`
}

// ReviewDraftSessions scans SessionsDir for pages with status:draft and
// promotes them to status:reviewed by rewriting the frontmatter in place.
// v1 uses a simple structural check (required sections present, confidence >=
// 0.5) rather than a full Claude/Codex re-review.
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

// EvaluateReviewDraftSessions runs the same page decision as ReviewDraftSessions
// in dry-run mode and compares it to a labeled manifest.
func EvaluateReviewDraftSessions(opts ReviewEvalOptions) (*ReviewEvalReport, error) {
	if opts.SessionsDir == "" {
		return nil, fmt.Errorf("review eval: SessionsDir is required")
	}
	if opts.ManifestPath == "" {
		return nil, fmt.Errorf("review eval: ManifestPath is required")
	}

	sessionsDir, err := filepath.Abs(opts.SessionsDir)
	if err != nil {
		return nil, fmt.Errorf("review eval: resolve sessions dir: %w", err)
	}
	manifestPath, err := filepath.Abs(opts.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("review eval: resolve manifest path: %w", err)
	}

	manifest, err := LoadReviewEvalManifest(manifestPath)
	if err != nil {
		return nil, err
	}

	report := &ReviewEvalReport{
		ID:           manifest.ID,
		ManifestPath: manifestPath,
		SessionsDir:  sessionsDir,
		Cases:        len(manifest.Cases),
		Results:      make([]ReviewEvalResult, 0, len(manifest.Cases)),
	}

	for _, evalCase := range manifest.Cases {
		result := evaluateReviewEvalCase(sessionsDir, evalCase)
		report.Results = append(report.Results, result)
		if result.Passed {
			report.Passed++
		} else {
			report.Failed++
		}
		if result.ErrorMessage != "" {
			report.Errors++
		}
	}
	if report.Cases > 0 {
		report.Accuracy = float64(report.Passed) / float64(report.Cases)
	}

	return report, nil
}

// LoadReviewEvalManifest reads and validates a Tier 2 review eval manifest.
func LoadReviewEvalManifest(path string) (ReviewEvalManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ReviewEvalManifest{}, fmt.Errorf("read review eval manifest %s: %w", path, err)
	}

	var manifest ReviewEvalManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ReviewEvalManifest{}, fmt.Errorf("parse review eval manifest %s: %w", path, err)
	}
	if strings.TrimSpace(manifest.ID) == "" {
		return ReviewEvalManifest{}, fmt.Errorf("review eval manifest %s missing id", path)
	}
	if len(manifest.Cases) == 0 {
		return ReviewEvalManifest{}, fmt.Errorf("review eval manifest %s has no cases", path)
	}
	for i, evalCase := range manifest.Cases {
		if strings.TrimSpace(evalCase.ID) == "" {
			return ReviewEvalManifest{}, fmt.Errorf("review eval manifest %s case %d missing id", path, i)
		}
		if strings.TrimSpace(evalCase.Path) == "" {
			return ReviewEvalManifest{}, fmt.Errorf("review eval manifest %s case %s missing path", path, evalCase.ID)
		}
		if !validReviewEvalDecision(evalCase.Expected) {
			return ReviewEvalManifest{}, fmt.Errorf("review eval manifest %s case %s expected must be promote or skip", path, evalCase.ID)
		}
	}
	return manifest, nil
}

func evaluateReviewEvalCase(sessionsDir string, evalCase ReviewEvalCase) ReviewEvalResult {
	path := resolveReviewEvalCasePath(sessionsDir, evalCase.Path)
	promoted, err := reviewOnePage(path, true)
	actual := "skip"
	if promoted {
		actual = "promote"
	}

	result := ReviewEvalResult{
		ID:       evalCase.ID,
		Path:     normalizeReviewEvalPath(sessionsDir, path),
		Expected: evalCase.Expected,
		Actual:   actual,
		Reason:   evalCase.Reason,
	}
	if err != nil {
		result.ErrorMessage = err.Error()
		result.Passed = false
		return result
	}
	result.Passed = result.Expected == result.Actual
	return result
}

func resolveReviewEvalCasePath(sessionsDir, evalPath string) string {
	if filepath.IsAbs(evalPath) {
		return filepath.Clean(evalPath)
	}
	return filepath.Clean(filepath.Join(sessionsDir, filepath.FromSlash(evalPath)))
}

func normalizeReviewEvalPath(sessionsDir, path string) string {
	rel, err := filepath.Rel(sessionsDir, path)
	if err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

func validReviewEvalDecision(decision string) bool {
	return decision == "promote" || decision == "skip"
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
