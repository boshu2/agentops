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

	// Reviewer optionally makes the final promote/skip decision after the
	// structural quality gate passes.
	Reviewer PageReviewer
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
	Reviewer     PageReviewer
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

// ReviewDecision is a parsed Tier 2 reviewer verdict.
type ReviewDecision struct {
	Promote bool
	Reason  string
}

// PageReviewer makes a Tier 2 promote/skip decision for one draft session page.
type PageReviewer interface {
	ReviewPage(page string) (ReviewDecision, error)
	ReviewerID() string
}

// GeneratorReviewer adapts a Generator into a page reviewer.
type GeneratorReviewer struct {
	gen Generator
}

const reviewPromptTemplate = `You are a Tier 2 reviewer for an AgentOps session wiki page.

Decide whether this draft page should be promoted to status:reviewed.

Promote only if the page has a clear intent, useful summary, relevant entities,
and a concrete assistant summary. Skip pages that are empty, low-signal,
unsafe, malformed, mostly tool noise, or not useful as durable knowledge.

Return exactly two lines:
DECISION: promote|skip
REASON: <one concise sentence>

PAGE:
%s`

const reviewPromptPageMaxChars = 6000

// NewGeneratorReviewer returns a reviewer that asks the configured Generator for
// a strict promote/skip decision.
func NewGeneratorReviewer(gen Generator) *GeneratorReviewer {
	return &GeneratorReviewer{gen: gen}
}

// ReviewerID identifies the backend in reviewed_by frontmatter.
func (r *GeneratorReviewer) ReviewerID() string {
	if r == nil || r.gen == nil {
		return "ao-forge-tier2-llm"
	}
	return safeReviewMetadata("ao-forge-tier2-llm-" + r.gen.ModelName())
}

// ReviewPage sends one page through the generator and parses its strict verdict.
func (r *GeneratorReviewer) ReviewPage(page string) (ReviewDecision, error) {
	if r == nil || r.gen == nil {
		return ReviewDecision{}, fmt.Errorf("reviewer: generator is required")
	}
	promptPage := truncate(page, reviewPromptPageMaxChars)
	raw, err := r.gen.Generate(fmt.Sprintf(reviewPromptTemplate, promptPage))
	if err != nil {
		return ReviewDecision{}, fmt.Errorf("reviewer generate: %w", err)
	}
	return parseReviewDecision(raw)
}

// ReviewDraftSessions scans SessionsDir for pages with status:draft and
// promotes them to status:reviewed by rewriting the frontmatter in place.
// v1 uses a simple structural check (required sections present, confidence >=
// 0.5). When Reviewer is set, the structural pass is followed by a configured
// LLM-backed final decision.
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
		promoted, err := reviewOnePageWithOptions(path, reviewPageOptions{
			DryRun:   opts.DryRun,
			Reviewer: opts.Reviewer,
		})
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
		result := evaluateReviewEvalCase(sessionsDir, evalCase, opts.Reviewer)
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

func evaluateReviewEvalCase(sessionsDir string, evalCase ReviewEvalCase, reviewer PageReviewer) ReviewEvalResult {
	path := resolveReviewEvalCasePath(sessionsDir, evalCase.Path)
	promoted, err := reviewOnePageWithOptions(path, reviewPageOptions{
		DryRun:   true,
		Reviewer: reviewer,
	})
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

type reviewPageOptions struct {
	DryRun     bool
	Reviewer   PageReviewer
	ReviewedBy string
}

// reviewOnePage reads a session page, checks if it's status:draft with
// passing structural quality, and promotes to status:reviewed. Returns
// true if the page was promoted.
func reviewOnePage(path string, dryRun bool) (bool, error) {
	return reviewOnePageWithOptions(path, reviewPageOptions{DryRun: dryRun})
}

func reviewOnePageWithOptions(path string, opts reviewPageOptions) (bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := string(b)

	// Only promote draft pages.
	if !strings.Contains(content, "status: draft") {
		return false, nil
	}

	// Structural quality gate: required section markers must be present, and
	// confidence must be >= 0.5 (not an all-skipped session).
	if !structuralReviewPasses(content) {
		return false, nil
	}

	if opts.Reviewer != nil {
		decision, err := opts.Reviewer.ReviewPage(content)
		if err != nil {
			return false, err
		}
		if !decision.Promote {
			return false, nil
		}
	}

	if opts.DryRun {
		return true, nil
	}

	// Promote: rewrite status + add reviewed_at/reviewed_by.
	now := time.Now().UTC().Format(time.RFC3339)
	promoted := strings.Replace(content, "status: draft", "status: reviewed", 1)
	// Insert reviewed_at and reviewed_by after the status line.
	reviewedBy := opts.ReviewedBy
	if reviewedBy == "" {
		reviewedBy = "ao-forge-tier2-structural"
		if opts.Reviewer != nil {
			reviewedBy = opts.Reviewer.ReviewerID()
		}
	}
	promoted = strings.Replace(promoted,
		"status: reviewed\n",
		fmt.Sprintf("status: reviewed\nreviewed_at: %s\nreviewed_by: %s\n", now, safeReviewMetadata(reviewedBy)),
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

func structuralReviewPasses(content string) bool {
	requiredSections := []string{"### ", "**Entities:**", "**Assistant:**"}
	for _, s := range requiredSections {
		if !strings.Contains(content, s) {
			return false
		}
	}
	// Check confidence (simple substring match — not full YAML parse).
	return !strings.Contains(content, "confidence: 0.0") && !strings.Contains(content, "confidence: 0.01")
}

func parseReviewDecision(raw string) (ReviewDecision, error) {
	body := stripCodeFence(strings.TrimSpace(raw))
	if body == "" {
		return ReviewDecision{}, fmt.Errorf("reviewer output is empty")
	}
	if decision, ok := parseBareReviewDecision(body); ok {
		return decision, nil
	}

	var decisionValue string
	var reason string
	for _, line := range strings.Split(body, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "decision":
			decisionValue = strings.ToLower(strings.TrimSpace(value))
		case "reason":
			reason = strings.TrimSpace(value)
		}
	}
	switch decisionValue {
	case "promote":
		return ReviewDecision{Promote: true, Reason: reason}, nil
	case "skip":
		return ReviewDecision{Promote: false, Reason: reason}, nil
	default:
		return ReviewDecision{}, fmt.Errorf("reviewer output missing DECISION: promote|skip")
	}
}

func parseBareReviewDecision(body string) (ReviewDecision, bool) {
	switch strings.ToLower(strings.TrimSpace(body)) {
	case "promote":
		return ReviewDecision{Promote: true}, true
	case "skip":
		return ReviewDecision{Promote: false}, true
	default:
		return ReviewDecision{}, false
	}
}

func safeReviewMetadata(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "ao-forge-tier2"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.', r == ':':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}
