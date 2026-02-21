package ratchet

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/boshu2/agentops/cli/internal/types"
)

// Validator performs quality validation on artifacts before locking.
type Validator struct {
	locator *Locator
	mu      sync.RWMutex
	metrics *ValidationMetrics
}

// ValidationMetrics tracks lenient vs strict validations.
type ValidationMetrics struct {
	LenientCount     int64             `json:"lenient_count"`
	StrictCount      int64             `json:"strict_count"`
	LenientArtifacts map[string]string `json:"lenient_artifacts"` // path -> expiry_date
}

// NewValidator creates a new validator.
func NewValidator(startDir string) (*Validator, error) {
	locator, err := NewLocator(startDir)
	if err != nil {
		return nil, err
	}
	return &Validator{
		locator: locator,
		metrics: &ValidationMetrics{
			LenientArtifacts: make(map[string]string),
		},
	}, nil
}

// lenientExpiryDefaultDays is the default expiry for lenient mode.
const lenientExpiryDefaultDays = 90

// lenientExpiryWarningDays is when to warn about upcoming expiry.
const lenientExpiryWarningDays = 30

// getLenientExpiry returns the expiry date, applying defaults if needed.
// Returns nil if not in lenient mode.
func getLenientExpiry(opts *ValidateOptions) *time.Time {
	if !opts.Lenient {
		return nil
	}
	if opts.LenientExpiryDate != nil {
		return opts.LenientExpiryDate
	}
	defaultExpiry := time.Now().AddDate(0, 0, lenientExpiryDefaultDays)
	return &defaultExpiry
}

// checkLenientExpiry validates expiry date and populates warnings/errors.
// Returns true if validation should continue, false if expired.
func checkLenientExpiry(expiryDate *time.Time, result *ValidationResult) bool {
	if expiryDate == nil {
		return true
	}

	expiryStr := expiryDate.Format(time.RFC3339)
	result.LenientExpiryDate = &expiryStr

	daysUntilExpiry := time.Until(*expiryDate).Hours() / 24

	switch {
	case daysUntilExpiry <= 0:
		// Expired - fail validation
		result.Valid = false
		result.Issues = append(result.Issues,
			"Lenient validation expired on "+expiryDate.Format("2006-01-02")+
				" - artifacts must be migrated")
		result.Lenient = false
		return false
	case daysUntilExpiry <= lenientExpiryWarningDays:
		// Expiring soon - warn
		result.LenientExpiringSoon = true
		result.Warnings = append(result.Warnings,
			"Lenient validation expires in "+
				formatDays(int(daysUntilExpiry))+
				" - artifacts must be migrated before then")
	}
	return true
}

// checkSchemaVersion validates schema_version field presence.
// Returns true if validation should continue, false if missing and strict.
func (v *Validator) checkSchemaVersion(artifactPath string, opts *ValidateOptions, result *ValidationResult) bool {
	hasSchemaVersion := v.hasSchemaVersion(artifactPath)

	if hasSchemaVersion {
		return true
	}

	if !opts.Lenient {
		result.Valid = false
		result.Issues = append(result.Issues,
			"Missing schema_version field - artifact not compatible with current schema. Use --lenient to bypass (temporary legacy support)")
		return false
	}

	// Lenient mode - warn but continue
	result.Warnings = append(result.Warnings,
		"Missing schema_version field - using lenient legacy bypass")
	return true
}

// Validate checks that an artifact meets quality requirements for its step.
// This defaults to STRICT mode (lenient = false).
func (v *Validator) Validate(step Step, artifactPath string) (*ValidationResult, error) {
	return v.ValidateWithOptions(step, artifactPath, &ValidateOptions{})
}

// ValidateWithOptions checks artifacts with lenient mode support.
func (v *Validator) ValidateWithOptions(step Step, artifactPath string, opts *ValidateOptions) (*ValidationResult, error) {
	if opts == nil {
		opts = &ValidateOptions{}
	}

	result := &ValidationResult{
		Step:     step,
		Valid:    true,
		Issues:   []string{},
		Warnings: []string{},
		Lenient:  opts.Lenient,
	}

	// Check if artifact exists
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		result.Valid = false
		result.Issues = append(result.Issues, "Artifact not found: "+artifactPath)
		return result, nil
	}

	// Check lenient expiry (includes setting defaults)
	expiryDate := getLenientExpiry(opts)
	if !checkLenientExpiry(expiryDate, result) {
		return result, nil // Expired - don't continue
	}

	// Check schema version
	if !v.checkSchemaVersion(artifactPath, opts, result) {
		return result, nil // Missing and strict - don't continue
	}

	// Run step-specific validation
	v.validateStep(step, artifactPath, result)

	// Assess tier based on validation results
	tier := v.assessTier(result)
	result.Tier = &tier

	// Track metrics
	v.trackMetrics(opts.Lenient, artifactPath, result.LenientExpiryDate)

	return result, nil
}

// validateStep runs step-specific validation rules.
func (v *Validator) validateStep(step Step, artifactPath string, result *ValidationResult) {
	switch step {
	case StepResearch:
		v.validateResearch(artifactPath, result)
	case StepPreMortem:
		v.validatePreMortem(artifactPath, result)
	case StepPlan:
		v.validatePlan(artifactPath, result)
	case StepPostMortem:
		v.validatePostMortem(artifactPath, result)
	default:
		result.Warnings = append(result.Warnings,
			"No validation rules for step: "+string(step))
	}
}

// validateResearch checks research artifact quality.
func (v *Validator) validateResearch(path string, result *ValidationResult) {
	content, err := os.ReadFile(path)
	if err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, "Cannot read file: "+err.Error())
		return
	}

	text := string(content)

	// Check for schema_version field (lenient mode: warning only)
	if !v.hasFrontmatterField(text, "schema_version") {
		result.Warnings = append(result.Warnings,
			"Missing schema_version field in frontmatter - new artifacts should include schema_version: 1")
	}

	// Check for required sections
	requiredSections := []string{
		"## Summary",
		"## Key Findings",
		"## Recommendations",
	}

	for _, section := range requiredSections {
		if !strings.Contains(text, section) {
			result.Warnings = append(result.Warnings,
				"Missing recommended section: "+section)
		}
	}

	// Check minimum length
	wordCount := len(strings.Fields(text))
	if wordCount < 100 {
		result.Warnings = append(result.Warnings,
			"Research seems short ("+string(rune(wordCount))+" words), consider adding more detail")
	}

	// Check for citations/sources
	if !strings.Contains(text, "Source") && !strings.Contains(text, "Reference") && !strings.Contains(text, "http") {
		result.Warnings = append(result.Warnings,
			"No sources or references found")
	}
}

// validatePreMortem checks pre-mortem/spec artifact quality.
func (v *Validator) validatePreMortem(path string, result *ValidationResult) {
	content, err := os.ReadFile(path)
	if err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, "Cannot read file: "+err.Error())
		return
	}

	text := string(content)

	// Check for schema_version field (lenient mode: warning only)
	if !v.hasFrontmatterField(text, "schema_version") {
		result.Warnings = append(result.Warnings,
			"Missing schema_version field in frontmatter - new artifacts should include schema_version: 1")
	}

	// Check for pre-mortem findings table
	if !strings.Contains(text, "Finding") && !strings.Contains(text, "| ID |") {
		result.Warnings = append(result.Warnings,
			"Missing findings table - pre-mortem should identify failure modes")
	}

	// Check for mitigations
	if !strings.Contains(text, "Mitigation") && !strings.Contains(text, "Fix") {
		result.Warnings = append(result.Warnings,
			"Missing mitigations - each finding should have a fix")
	}

	// Check version indicator
	versionPattern := regexp.MustCompile(`-v\d+\.md$`)
	if !versionPattern.MatchString(filepath.Base(path)) {
		result.Warnings = append(result.Warnings,
			"Filename should include version suffix (e.g., -v2.md)")
	}
}

// validatePlan checks plan/epic artifact quality.
func (v *Validator) validatePlan(path string, result *ValidationResult) {
	// Plans might be beads issues, not files
	if strings.HasPrefix(path, "epic:") {
		// This is an epic ID, validate via beads
		v.validateEpicIssue(strings.TrimPrefix(path, "epic:"), result)
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, "Cannot read file: "+err.Error())
		return
	}

	text := string(content)

	// Handle TOML formula files separately
	if strings.HasSuffix(path, ".toml") {
		v.validateFormulaToml(text, result)
		return
	}

	// Check for schema_version field (lenient mode: warning only)
	if !v.hasFrontmatterField(text, "schema_version") {
		result.Warnings = append(result.Warnings,
			"Missing schema_version field in frontmatter - new artifacts should include schema_version: 1")
	}

	// Check for required plan elements
	if !strings.Contains(text, "## Objective") && !strings.Contains(text, "## Goal") {
		result.Warnings = append(result.Warnings,
			"Missing objective/goal section")
	}

	if !strings.Contains(text, "## Tasks") && !strings.Contains(text, "## Issues") {
		result.Warnings = append(result.Warnings,
			"Missing tasks/issues breakdown")
	}

	if !strings.Contains(text, "## Success Criteria") && !strings.Contains(text, "## Acceptance") {
		result.Warnings = append(result.Warnings,
			"Missing success criteria")
	}
}

// validatePostMortem checks post-mortem/retro artifact quality.
func (v *Validator) validatePostMortem(path string, result *ValidationResult) {
	content, err := os.ReadFile(path)
	if err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, "Cannot read file: "+err.Error())
		return
	}

	text := string(content)

	// Check for schema_version field (lenient mode: warning only)
	if !v.hasFrontmatterField(text, "schema_version") {
		result.Warnings = append(result.Warnings,
			"Missing schema_version field in frontmatter - new artifacts should include schema_version: 1")
	}

	// Check for learnings section
	if !strings.Contains(text, "## Learnings") && !strings.Contains(text, "## Key Learnings") {
		result.Warnings = append(result.Warnings,
			"Missing learnings section - retros should capture what was learned")
	}

	// Check for patterns section
	if !strings.Contains(text, "## Patterns") && !strings.Contains(text, "## Reusable Patterns") {
		result.Warnings = append(result.Warnings,
			"Consider adding patterns section for reusable workflows")
	}

	// Check for next steps
	if !strings.Contains(text, "## Next") && !strings.Contains(text, "## Follow-up") {
		result.Warnings = append(result.Warnings,
			"Missing next steps/follow-up section")
	}
}

// validateEpicIssue validates an epic via beads CLI.
func (v *Validator) validateEpicIssue(epicID string, result *ValidationResult) {
	// This would call bd show <epicID> and parse the output
	// For now, we just check that it looks like a valid ID
	if epicID == "" {
		result.Valid = false
		result.Issues = append(result.Issues, "Empty epic ID")
		return
	}

	// Basic ID format check
	if !strings.Contains(epicID, "-") {
		result.Warnings = append(result.Warnings,
			"Epic ID should have prefix (e.g., ol-0001)")
	}
}

// assessTier determines the quality tier based on validation results.
func (v *Validator) assessTier(result *ValidationResult) Tier {
	if !result.Valid {
		return TierObservation // Invalid = lowest tier
	}

	issueCount := len(result.Issues)
	warningCount := len(result.Warnings)

	if issueCount == 0 && warningCount == 0 {
		return TierPattern // No issues = ready for patterns
	}

	if issueCount == 0 && warningCount <= 2 {
		return TierLearning // Minor warnings = learning tier
	}

	return TierObservation // Has issues = observation tier
}

// ValidateForPromotion checks if an artifact is ready for tier promotion.
func (v *Validator) ValidateForPromotion(artifactPath string, targetTier Tier) (*ValidationResult, error) {
	result := &ValidationResult{
		Step:     Step("promotion"),
		Valid:    true,
		Issues:   []string{},
		Warnings: []string{},
	}

	// Check artifact exists
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		result.Valid = false
		result.Issues = append(result.Issues, "Artifact not found: "+artifactPath)
		return result, nil
	}

	// Tier-specific promotion requirements
	switch targetTier {
	case TierLearning:
		// Promotion to T1 requires: 2+ citations
		citations := v.countCitations(artifactPath)
		if citations < 2 {
			result.Valid = false
			result.Issues = append(result.Issues,
				"Promotion to learning tier requires 2+ citations (found: %d)")
		}

	case TierPattern:
		// Promotion to T2 requires: 3+ sessions
		sessions := v.countSessionRefs(artifactPath)
		if sessions < 3 {
			result.Valid = false
			result.Issues = append(result.Issues,
				"Promotion to pattern tier requires references in 3+ sessions")
		}

	case TierSkill:
		// Promotion to T3 requires: SKILL.md format
		if !v.hasSkillFormat(artifactPath) {
			result.Valid = false
			result.Issues = append(result.Issues,
				"Promotion to skill tier requires SKILL.md format")
		}

	case TierCore:
		// Promotion to T4 requires: 10+ uses across sessions
		result.Warnings = append(result.Warnings,
			"Core tier promotion requires manual review (10+ documented uses)")
	}

	currentTier := v.assessTier(result)
	result.Tier = &currentTier

	return result, nil
}

// countCitations counts how many times an artifact is referenced.
func (v *Validator) countCitations(artifactPath string) int {
	// This would search for references to this artifact in other files
	// Simplified implementation: count backlinks in the same directory
	baseName := filepath.Base(artifactPath)
	dir := filepath.Dir(artifactPath)

	count := 0
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || path == artifactPath {
			return nil
		}
		if strings.HasSuffix(path, ".md") {
			if v.fileContains(path, baseName) {
				count++
			}
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to walk %s: %v\n", dir, err)
	}

	return count
}

// countSessionRefs counts sessions that reference this artifact.
// Searches .agents/ao/sessions/ for JSONL and markdown files
// that mention the artifact by name.
func (v *Validator) countSessionRefs(artifactPath string) int {
	baseName := filepath.Base(artifactPath)

	// Build list of session directories to search
	var sessionsDirs []string

	// Check current directory's sessions
	localSessions := filepath.Join(v.locator.startDir, ".agents", "ao", "sessions")
	if _, err := os.Stat(localSessions); err == nil {
		sessionsDirs = append(sessionsDirs, localSessions)
	}

	// Check rig root sessions (if different from local)
	if rigDir := v.locator.findRigRoot(); rigDir != "" && rigDir != v.locator.startDir {
		rigSessions := filepath.Join(rigDir, ".agents", "ao", "sessions")
		if _, err := os.Stat(rigSessions); err == nil {
			sessionsDirs = append(sessionsDirs, rigSessions)
		}
	}

	// Check town-level sessions
	townSessions := filepath.Join(v.locator.townDir, ".agents", "ao", "sessions")
	if _, err := os.Stat(townSessions); err == nil {
		sessionsDirs = append(sessionsDirs, townSessions)
	}

	if len(sessionsDirs) == 0 {
		return 0
	}

	count := 0
	seen := make(map[string]bool) // Dedupe by session file

	for _, sessionsDir := range sessionsDirs {
		if err := filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}

			// Only search JSONL and markdown files
			ext := filepath.Ext(path)
			if ext != ".jsonl" && ext != ".md" {
				return nil
			}

			// Skip if already counted this file
			if seen[path] {
				return nil
			}

			if v.fileContains(path, baseName) {
				seen[path] = true
				count++
			}
			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to walk %s: %v\n", sessionsDir, err)
		}
	}

	return count
}

// hasSkillFormat checks if a file has valid SKILL.md format.
func (v *Validator) hasSkillFormat(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	text := string(content)

	// Required skill sections
	required := []string{
		"## Description",
		"## Triggers",
		"## Instructions",
	}

	for _, section := range required {
		if !strings.Contains(text, section) {
			return false
		}
	}

	return true
}

// fileContains checks if a file contains a string.
func (v *Validator) fileContains(path, needle string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // read-only search, close error non-fatal
	}()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), needle) {
			return true
		}
	}
	return false
}

// trackMetrics records validation mode usage and artifact expiry dates.
func (v *Validator) trackMetrics(lenient bool, artifactPath string, expiryDate *string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if lenient {
		v.metrics.LenientCount++
		if expiryDate != nil {
			v.metrics.LenientArtifacts[artifactPath] = *expiryDate
		}
	} else {
		v.metrics.StrictCount++
	}
}

// GetMetrics returns the current validation metrics.
func (v *Validator) GetMetrics() *ValidationMetrics {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Return a copy to prevent external mutation
	metrics := &ValidationMetrics{
		LenientCount:     v.metrics.LenientCount,
		StrictCount:      v.metrics.StrictCount,
		LenientArtifacts: make(map[string]string),
	}

	for k, v := range v.metrics.LenientArtifacts {
		metrics.LenientArtifacts[k] = v
	}

	return metrics
}

// formatDays formats the number of days as a human-readable string.
func formatDays(days int) string {
	switch {
	case days == 0:
		return "today"
	case days == 1:
		return "1 day"
	default:
		return fmt.Sprintf("%d days", days)
	}
}

// hasSchemaVersion checks if an artifact has a schema_version field.
// Looks for schema_version in YAML frontmatter or as a markdown field.
func (v *Validator) hasSchemaVersion(path string) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	text := string(content)

	// Check for schema_version field in various formats:
	// 1. YAML frontmatter: schema_version: ...
	// 2. Markdown field: **schema_version**:
	// 3. JSON field: "schema_version":
	patterns := []string{
		"schema_version:",    // YAML/Markdown format
		"\"schema_version\"", // JSON format
		"schema-version:",    // Hyphenated format
		"\"schema-version\"", // JSON hyphenated
	}

	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}

	return false
}

// hasFrontmatterField checks if a markdown file has a field in its YAML frontmatter.
// Looks for lines like "field_name: value" or "field_name: 1" between --- delimiters.
func (v *Validator) hasFrontmatterField(text, fieldName string) bool {
	lines := strings.Split(text, "\n")

	inFrontmatter := false
	fieldPattern := regexp.MustCompile(`^` + regexp.QuoteMeta(fieldName) + `\s*:`)

	for _, line := range lines {
		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			} else {
				// End of frontmatter
				break
			}
		}

		if inFrontmatter && fieldPattern.MatchString(line) {
			return true
		}
	}

	return false
}

// validateFormulaToml checks formula artifact quality in TOML format.
func (v *Validator) validateFormulaToml(text string, result *ValidationResult) {
	// Check for required top-level TOML fields
	requiredFields := []string{
		"formula",
		"description",
		"version",
		"type",
	}

	for _, field := range requiredFields {
		pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(field) + `\s*=`)
		if !pattern.MatchString(text) {
			result.Warnings = append(result.Warnings,
				"Missing required TOML field: "+field)
		}
	}

	// Check for schema_version field (lenient mode: warning only)
	if !strings.Contains(text, "schema_version") {
		result.Warnings = append(result.Warnings,
			"Missing schema_version field in TOML - new artifacts should include schema_version = 1")
	}

	// Check for steps array
	if !strings.Contains(text, "[[steps]]") {
		result.Warnings = append(result.Warnings,
			"Missing [[steps]] array - formula should define work items")
	}
}

// --- Path Validation (ol-a46.1.8) ---

// ValidateArtifactPath checks if a path is absolute (required for close_reason references).
// Returns nil if valid, error with message if invalid.
func ValidateArtifactPath(path string) error {
	if path == "" {
		return nil // Empty is valid (no path referenced)
	}

	// Must be absolute (start with /)
	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute, got: %s", path)
	}

	// Must not contain tilde (shell-dependent)
	if strings.HasPrefix(path, "~") {
		return fmt.Errorf("path must not use tilde (~), got: %s", path)
	}

	return nil
}

// ExtractArtifactPaths finds all artifact path references in a close_reason string.
// Looks for patterns like "Artifact: /path" or "See /path".
func ExtractArtifactPaths(closeReason string) []string {
	if closeReason == "" {
		return nil
	}

	var paths []string

	// Pattern: "Artifact: /path" or "artifact: /path"
	artifactPattern := regexp.MustCompile(`(?i)artifact:\s*(/[^\s]+)`)
	for _, match := range artifactPattern.FindAllStringSubmatch(closeReason, -1) {
		if len(match) > 1 {
			paths = append(paths, match[1])
		}
	}

	// Pattern: "See /path" or "see /path"
	seePattern := regexp.MustCompile(`(?i)see\s+(/[^\s]+)`)
	for _, match := range seePattern.FindAllStringSubmatch(closeReason, -1) {
		if len(match) > 1 {
			paths = append(paths, match[1])
		}
	}

	return paths
}

// ValidateCloseReason checks that all artifact paths in a close_reason are absolute.
// Returns list of issues found.
func ValidateCloseReason(closeReason string) []string {
	var issues []string

	paths := ExtractArtifactPaths(closeReason)
	for _, path := range paths {
		if err := ValidateArtifactPath(path); err != nil {
			issues = append(issues, err.Error())
		}
	}

	// Check for common relative path patterns that aren't caught by extract
	relativePatterns := []string{
		`\.\/`,   // ./
		`\.\.\/`, // ../
		`~\/`,    // ~/
	}

	for _, pattern := range relativePatterns {
		if matched, _ := regexp.MatchString(pattern, closeReason); matched {
			issues = append(issues, "close_reason may contain relative path: "+closeReason)
			break
		}
	}

	return issues
}

// --- Citation Tracking (ol-a46 Phase 0) ---

// CitationsFilePath is the relative path to the citations JSONL file.
const CitationsFilePath = ".agents/ao/citations.jsonl"

// CanonicalArtifactPath resolves citation artifact paths to a stable absolute form.
func CanonicalArtifactPath(baseDir, artifactPath string) string {
	p := strings.TrimSpace(artifactPath)
	if p == "" {
		return ""
	}
	p = filepath.Clean(p)
	if !filepath.IsAbs(p) {
		if strings.TrimSpace(baseDir) == "" {
			baseDir = "."
		}
		p = filepath.Join(baseDir, p)
	}
	abs, err := filepath.Abs(p)
	if err == nil {
		p = abs
	}
	return filepath.Clean(p)
}

// RecordCitation appends a citation event to the citations log.
// Creates the file and parent directories if they don't exist.
func RecordCitation(baseDir string, event types.CitationEvent) error {
	// Ensure citation has timestamp
	if event.CitedAt.IsZero() {
		event.CitedAt = time.Now()
	}
	event.ArtifactPath = CanonicalArtifactPath(baseDir, event.ArtifactPath)

	// Build full path
	citationsPath := filepath.Join(baseDir, CitationsFilePath)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(citationsPath), 0700); err != nil {
		return fmt.Errorf("create citations directory: %w", err)
	}

	// Open file for append (create if doesn't exist)
	f, err := os.OpenFile(citationsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open citations file: %w", err)
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // write already complete, close best-effort
	}()

	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal citation: %w", err)
	}

	// Write JSONL line
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write citation: %w", err)
	}

	return nil
}

// LoadCitations reads all citation events from the citations log.
func LoadCitations(baseDir string) ([]types.CitationEvent, error) {
	citationsPath := filepath.Join(baseDir, CitationsFilePath)

	f, err := os.Open(citationsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No citations yet
		}
		return nil, fmt.Errorf("open citations file: %w", err)
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // read-only, close error non-fatal
	}()

	var citations []types.CitationEvent
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event types.CitationEvent
		if err := json.Unmarshal(line, &event); err != nil {
			// Skip malformed lines
			continue
		}
		event.ArtifactPath = CanonicalArtifactPath(baseDir, event.ArtifactPath)
		citations = append(citations, event)
	}

	if err := scanner.Err(); err != nil {
		return citations, fmt.Errorf("scan citations: %w", err)
	}

	return citations, nil
}

// CountCitationsForArtifact returns the number of times an artifact has been cited.
func CountCitationsForArtifact(baseDir, artifactPath string) (int, error) {
	citations, err := LoadCitations(baseDir)
	if err != nil {
		return 0, err
	}

	target := CanonicalArtifactPath(baseDir, artifactPath)
	count := 0
	for _, c := range citations {
		if CanonicalArtifactPath(baseDir, c.ArtifactPath) == target {
			count++
		}
	}
	return count, nil
}

// GetCitationsSince returns citations after a given time.
func GetCitationsSince(baseDir string, since time.Time) ([]types.CitationEvent, error) {
	allCitations, err := LoadCitations(baseDir)
	if err != nil {
		return nil, err
	}

	var filtered []types.CitationEvent
	for _, c := range allCitations {
		if c.CitedAt.After(since) {
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}

// GetUniqueCitedArtifacts returns unique artifact paths that were cited in a period.
func GetUniqueCitedArtifacts(baseDir string, since, until time.Time) ([]string, error) {
	allCitations, err := LoadCitations(baseDir)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var unique []string

	for _, c := range allCitations {
		if c.CitedAt.After(since) && c.CitedAt.Before(until) {
			path := CanonicalArtifactPath(baseDir, c.ArtifactPath)
			if !seen[path] {
				seen[path] = true
				unique = append(unique, path)
			}
		}
	}
	return unique, nil
}

// --- MemRL Tier-to-Reward Conversion (Phase 4) ---

// TierToReward converts a ratchet quality tier to a reward value for MemRL.
// This enables validation results to feed into the feedback loop.
// Higher tiers = higher rewards = more surface priority.
func TierToReward(tier Tier) float64 {
	switch tier {
	case TierCore:
		return 1.0 // T4 - Proven knowledge in CLAUDE.md (10+ uses)
	case TierSkill:
		return 0.9 // T3 - Tested workflow in skills
	case TierPattern:
		return 0.75 // T2 - Recognized pattern (3+ sessions)
	case TierLearning:
		return 0.5 // T1 - Validated learning (2+ citations)
	case TierObservation:
		return 0.25 // T0 - Raw observation
	default:
		return 0.0
	}
}

// RewardToTier converts a reward value back to the nearest quality tier.
// Used for tier-based promotion decisions.
func RewardToTier(reward float64) Tier {
	switch {
	case reward >= 0.95:
		return TierCore
	case reward >= 0.8:
		return TierSkill
	case reward >= 0.6:
		return TierPattern
	case reward >= 0.35:
		return TierLearning
	default:
		return TierObservation
	}
}

// TierFromValidation derives a tier from validation results.
// Combines issues, warnings, and explicit tier assignment.
func TierFromValidation(result *ValidationResult) Tier {
	// If explicit tier assigned, use it
	if result.Tier != nil {
		return *result.Tier
	}

	// Derive from validation quality
	if !result.Valid {
		return TierObservation
	}

	issueCount := len(result.Issues)
	warningCount := len(result.Warnings)

	switch {
	case issueCount == 0 && warningCount == 0:
		return TierPattern
	case issueCount == 0 && warningCount <= 2:
		return TierLearning
	default:
		return TierObservation
	}
}

// GetCitationsForSession returns citations for a specific session.
func GetCitationsForSession(baseDir, sessionID string) ([]types.CitationEvent, error) {
	allCitations, err := LoadCitations(baseDir)
	if err != nil {
		return nil, err
	}

	var filtered []types.CitationEvent
	for _, c := range allCitations {
		if c.SessionID == sessionID {
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}
