package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

var (
	maturityApply       bool
	maturityScan        bool
	maturityCurate      bool
	maturityExpire      bool
	maturityArchive     bool
	maturityEvict       bool
	maturityGlobal      bool
	maturityMigrateMd   bool
	maturityRecalibrate bool
	maturityUncitedDays int
)

var maturityCmd = &cobra.Command{
	Use:   "maturity [learning-id]",
	Short: "Check and manage learning maturity levels",
	Long: `Check and manage CASS (Contextual Agent Session Search) maturity levels.

Learnings progress through maturity stages based on feedback:
  provisional  → Initial stage, needs positive feedback
  candidate    → Received positive feedback, being validated
  established  → Proven value through consistent positive feedback
  anti-pattern → Consistently harmful, surfaced as what NOT to do

Transition Rules:
  provisional → candidate:    utility >= 0.55 AND reward_count >= 3
  candidate → established:    utility >= 0.55 AND reward_count >= 5 AND helpful > harmful
  any → anti-pattern:         utility <= 0.2 AND harmful_count >= 3
  established → candidate:    utility < 0.5 (demotion)
  candidate → provisional:    utility < 0.3 (demotion)

Examples:
  ao maturity L001                    # Check maturity status of a learning
  ao maturity L001 --apply            # Check and apply transition if needed
  ao maturity --scan                  # Scan all learnings for pending transitions
  ao maturity --scan --apply          # Apply all pending transitions`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMaturity,
}

func init() {
	maturityCmd.GroupID = "core"
	rootCmd.AddCommand(maturityCmd)
	maturityCmd.Flags().BoolVar(&maturityApply, "apply", false, "Apply maturity transitions")
	maturityCmd.Flags().BoolVar(&maturityScan, "scan", false, "Scan all learnings for pending transitions")
	maturityCmd.Flags().BoolVar(&maturityCurate, "curate", false, "Normalize metadata and identify low-signal or uncited stale learnings")
	maturityCmd.Flags().BoolVar(&maturityExpire, "expire", false, "Scan for expired learnings")
	maturityCmd.Flags().BoolVar(&maturityArchive, "archive", false, "Move expired/evicted/curated files to archive (requires --expire, --evict, or --curate)")
	maturityCmd.Flags().BoolVar(&maturityEvict, "evict", false, "Identify eviction candidates (composite criteria)")
	maturityCmd.Flags().BoolVar(&maturityGlobal, "global", false, "Operate on ~/.agents/learnings instead of the local workspace learnings")
	maturityCmd.Flags().BoolVar(&maturityMigrateMd, "migrate-md", false, "Add default frontmatter to .md learnings missing utility field")
	maturityCmd.Flags().BoolVar(&maturityRecalibrate, "recalibrate", false, "Reset utility to 0.5 for all learnings")
	maturityCmd.Flags().IntVar(&maturityUncitedDays, "uncited-days", 60, "Archive provisional/candidate learnings with zero citations older than this many days when used with --curate")
}

func runMaturitySingle(cwd string, learningID string) error {
	learningPath, err := findLearningFile(cwd, learningID)
	if err != nil {
		return fmt.Errorf("find learning: %w", err)
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would check maturity for: %s\n", learningID)
		return nil
	}

	result, err := checkOrApplyMaturity(learningPath)
	if err != nil {
		return fmt.Errorf("check maturity: %w", err)
	}

	return outputSingleMaturityResult(result)
}

func checkOrApplyMaturity(learningPath string) (*ratchet.MaturityTransitionResult, error) {
	if maturityApply {
		return ratchet.ApplyMaturityTransition(learningPath)
	}
	return ratchet.CheckMaturityTransition(learningPath)
}

func outputSingleMaturityResult(result *ratchet.MaturityTransitionResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	displayMaturityResult(result, maturityApply)
	return nil
}

func runMaturity(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	patternsDir := filepath.Join(cwd, ".agents", "patterns")

	if !dirExists(learningsDir) && !dirExists(patternsDir) {
		fmt.Println("No learnings or patterns directory found.")
		return nil
	}

	switch {
	case maturityMigrateMd:
		return runMaturityMigrateMd(learningsDir)
	case maturityRecalibrate:
		return runMaturityRecalibrate(learningsDir)
	case maturityCurate:
		return runMaturityCurate(cmd)
	case maturityEvict:
		return runMaturityEvict(cmd)
	case maturityExpire:
		return runMaturityExpire(cmd)
	case maturityScan:
		return runMaturityScanAll(learningsDir, patternsDir)
	case len(args) == 0:
		return fmt.Errorf("must provide learning-id or use --scan")
	default:
		return runMaturitySingle(cwd, args[0])
	}
}

// runMaturityMigrateMd adds default frontmatter to .md learnings that lack a utility field.
func runMaturityMigrateMd(learningsDir string) error {
	files, err := ratchet.GlobLearningFiles(learningsDir)
	if err != nil {
		return fmt.Errorf("glob learnings: %w", err)
	}

	// Filter to .md files only
	var mdFiles []string
	for _, f := range files {
		if strings.HasSuffix(f, ".md") {
			mdFiles = append(mdFiles, f)
		}
	}

	migrated := 0
	for _, file := range mdFiles {
		changed, normalizeErr := normalizeLearningMetadata(file, false)
		if normalizeErr != nil {
			VerbosePrintf("Warning: could not normalize %s: %v\n", filepath.Base(file), normalizeErr)
			continue
		}
		if changed {
			migrated++
		}
	}

	fmt.Printf("Migrated %d of %d .md learnings\n", migrated, len(mdFiles))
	return nil
}

func defaultLearningMetadataFields() map[string]string {
	return lifecycle.DefaultLearningMetadataFields()
}

func normalizeLearningMetadata(file string, dryRun bool) (bool, error) {
	if strings.HasSuffix(file, ".jsonl") {
		return normalizeLearningJSONLMetadata(file, dryRun)
	}
	return normalizeLearningMarkdownMetadata(file, dryRun)
}

func normalizeLearningMarkdownMetadata(file string, dryRun bool) (bool, error) {
	fields, err := parseFrontmatterFields(file, lifecycle.LearningMetadataFieldOrder()...)
	if err != nil {
		return false, err
	}
	missing := lifecycle.MissingMetadataFields(fields)
	if len(missing) == 0 {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return false, err
	}
	text := string(content)
	if lifecycle.HasYAMLFrontmatter(text) {
		lines := strings.Split(text, "\n")
		endIdx := lifecycle.FindFrontmatterEnd(lines)
		if endIdx == -1 {
			return false, fmt.Errorf("malformed frontmatter")
		}
		updatedFM := updateFrontMatterFields(lines[1:endIdx], missing)
		rebuilt := rebuildWithFrontMatter(updatedFM, lines[endIdx+1:])
		return true, atomicWriteFile(file, []byte(rebuilt), 0o600)
	}
	return true, atomicWriteFile(file, []byte(lifecycle.BuildMarkdownFrontmatterPrefix()+text), 0o600)
}

func normalizeLearningJSONLMetadata(file string, dryRun bool) (bool, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return false, nil
	}
	newLine, changed, err := lifecycle.NormalizeJSONLLine(lines[0])
	if err != nil || !changed {
		return false, err
	}
	if dryRun {
		return true, nil
	}
	lines[0] = newLine
	return true, atomicWriteFile(file, []byte(strings.Join(lines, "\n")), 0o600)
}

// runMaturityRecalibrate resets utility to InitialUtility (0.5) for all learnings.
func runMaturityRecalibrate(learningsDir string) error {
	files, err := ratchet.GlobLearningFiles(learningsDir)
	if err != nil {
		return fmt.Errorf("glob learnings: %w", err)
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would recalibrate %d learnings (utility reset to %.1f)\n", len(files), types.InitialUtility)
		return nil
	}

	recalibrated := 0
	for _, file := range files {
		// updateLearningUtility with alpha=1.0 and reward=InitialUtility
		// yields: new = (1-1.0)*old + 1.0*0.5 = 0.5
		_, _, err := updateLearningUtility(file, types.InitialUtility, 1.0)
		if err != nil {
			VerbosePrintf("Warning: could not recalibrate %s: %v\n", filepath.Base(file), err)
			continue
		}
		recalibrated++
	}

	fmt.Printf("Recalibrated %d learnings (utility reset to %.1f)\n", recalibrated, types.InitialUtility)
	return nil
}

func displayMaturityDistribution(dist *ratchet.MaturityDistribution) {
	fmt.Println("=== Maturity Distribution ===")
	fmt.Printf("  Provisional:  %d\n", dist.Provisional)
	fmt.Printf("  Candidate:    %d\n", dist.Candidate)
	fmt.Printf("  Established:  %d\n", dist.Established)
	fmt.Printf("  Anti-Pattern: %d\n", dist.AntiPattern)
	fmt.Printf("  Total:        %d\n", dist.Total)
	fmt.Println()
}

func displayPendingTransitions(results []*ratchet.MaturityTransitionResult) error {
	fmt.Printf("=== Pending Transitions (%d) ===\n", len(results))
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}
	for _, r := range results {
		displayMaturityResult(r, false)
		fmt.Println()
	}
	return nil
}

func applyScannedTransitions(artifactDir string, results []*ratchet.MaturityTransitionResult) {
	fmt.Println("=== Applying Transitions ===")
	applied := 0
	// Resolve from the parent (.agents/) so the resolver can find files in both learnings/ and patterns/
	baseDir := filepath.Dir(artifactDir)
	for _, r := range results {
		learningPath, err := findLearningFile(baseDir, r.LearningID)
		if err != nil {
			VerbosePrintf("Warning: could not find %s: %v\n", r.LearningID, err)
			continue
		}
		result, err := ratchet.ApplyMaturityTransition(learningPath)
		if err != nil {
			VerbosePrintf("Warning: could not apply transition for %s: %v\n", r.LearningID, err)
			continue
		}
		if result.Transitioned {
			fmt.Printf("✓ %s: %s → %s\n", result.LearningID, result.OldMaturity, result.NewMaturity)
			applied++
		}
	}
	fmt.Printf("\nApplied %d transitions.\n", applied)
}

// runMaturityScanAll scans both learnings/ and patterns/ for maturity transitions.
func runMaturityScanAll(learningsDir, patternsDir string) error {
	var dirs []string
	if dirExists(learningsDir) {
		dirs = append(dirs, learningsDir)
	}
	if dirExists(patternsDir) {
		dirs = append(dirs, patternsDir)
	}
	if len(dirs) == 0 {
		fmt.Println("No learnings or patterns directory found.")
		return nil
	}

	if GetDryRun() {
		for _, d := range dirs {
			fmt.Printf("[dry-run] Would scan: %s\n", d)
		}
		return nil
	}

	totalDist := &ratchet.MaturityDistribution{}
	var allResults []*ratchet.MaturityTransitionResult

	for _, dir := range dirs {
		dist, err := ratchet.GetMaturityDistribution(dir)
		if err != nil {
			return fmt.Errorf("get distribution from %s: %w", dir, err)
		}
		totalDist.Provisional += dist.Provisional
		totalDist.Candidate += dist.Candidate
		totalDist.Established += dist.Established
		totalDist.AntiPattern += dist.AntiPattern
		totalDist.Unknown += dist.Unknown
		totalDist.Total += dist.Total

		results, err := ratchet.ScanForMaturityTransitions(dir)
		if err != nil {
			return fmt.Errorf("scan transitions in %s: %w", dir, err)
		}
		allResults = append(allResults, results...)
	}

	displayMaturityDistribution(totalDist)

	if len(allResults) == 0 {
		fmt.Println("No pending maturity transitions found.")
		return nil
	}

	if err := displayPendingTransitions(allResults); err != nil {
		return err
	}

	if maturityApply {
		applyScannedTransitions(dirs[0], allResults)
	}

	return nil
}

func runMaturityScan(learningsDir string) error {
	if GetDryRun() {
		fmt.Printf("[dry-run] Would scan learnings in: %s\n", learningsDir)
		return nil
	}

	dist, err := ratchet.GetMaturityDistribution(learningsDir)
	if err != nil {
		return fmt.Errorf("get distribution: %w", err)
	}
	displayMaturityDistribution(dist)

	results, err := ratchet.ScanForMaturityTransitions(learningsDir)
	if err != nil {
		return fmt.Errorf("scan transitions: %w", err)
	}
	if len(results) == 0 {
		fmt.Println("No pending maturity transitions found.")
		return nil
	}

	if err := displayPendingTransitions(results); err != nil {
		return err
	}

	if maturityApply {
		applyScannedTransitions(learningsDir, results)
	}

	return nil
}

func displayMaturityResult(r *ratchet.MaturityTransitionResult, applied bool) {
	fmt.Printf("Learning: %s\n", r.LearningID)
	fmt.Printf("  Maturity:  %s", r.OldMaturity)
	if r.Transitioned {
		action := "→"
		if applied {
			action = "→✓"
		}
		fmt.Printf(" %s %s", action, r.NewMaturity)
	}
	fmt.Println()
	fmt.Printf("  Utility:   %.3f\n", r.Utility)
	fmt.Printf("  Confidence: %.3f\n", r.Confidence)
	fmt.Printf("  Feedback:  %d total (helpful: %d, harmful: %d)\n",
		r.RewardCount, r.HelpfulCount, r.HarmfulCount)
	fmt.Printf("  Reason:    %s\n", r.Reason)
}

// expiryCategory tracks how a learning file is categorized for expiry.
type expiryCategory struct {
	active          []string
	neverExpiring   []string
	newlyExpired    []string
	alreadyArchived []string
}

// parseFrontmatterFields extracts specific fields from YAML frontmatter in a markdown file.
func parseFrontmatterFields(path string, fields ...string) (map[string]string, error) {
	return lifecycle.ParseFrontmatterFields(path, fields...)
}

func runMaturityExpire(cmd *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		fmt.Println("No learnings directory found.")
		return nil
	}

	cats := expiryCategory{}

	entries, err := os.ReadDir(learningsDir)
	if err != nil {
		return fmt.Errorf("read learnings directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		classifyExpiryEntry(entry, learningsDir, &cats)
	}

	total := len(cats.active) + len(cats.neverExpiring) + len(cats.newlyExpired) + len(cats.alreadyArchived)

	fmt.Println("=== Expiry Scan ===")
	fmt.Printf("  Active:           %d\n", len(cats.active))
	fmt.Printf("  Never-expiring:   %d (no valid_until field)\n", len(cats.neverExpiring))
	fmt.Printf("  Newly expired:    %d\n", len(cats.newlyExpired))
	fmt.Printf("  Already archived: %d\n", len(cats.alreadyArchived))
	fmt.Printf("  Total:            %d\n", total)

	if maturityArchive && len(cats.newlyExpired) > 0 {
		return archiveExpiredLearnings(cwd, learningsDir, cats.newlyExpired)
	}

	return nil
}

// classifyExpiryEntry reads frontmatter from one learning file and appends to the appropriate category.
func classifyExpiryEntry(entry os.DirEntry, learningsDir string, cats *expiryCategory) {
	path := filepath.Join(learningsDir, entry.Name())
	fields, err := parseFrontmatterFields(path, "valid_until", "expiry_status")
	if err != nil {
		VerbosePrintf("Warning: could not read %s: %v\n", entry.Name(), err)
		cats.neverExpiring = append(cats.neverExpiring, entry.Name())
		return
	}
	switch lifecycle.ClassifyExpiryFields(fields, time.Now()) {
	case lifecycle.ExpiryAlreadyArchived:
		cats.alreadyArchived = append(cats.alreadyArchived, entry.Name())
	case lifecycle.ExpiryNewlyExpired:
		cats.newlyExpired = append(cats.newlyExpired, entry.Name())
	case lifecycle.ExpiryActive:
		cats.active = append(cats.active, entry.Name())
	default:
		cats.neverExpiring = append(cats.neverExpiring, entry.Name())
	}
}

// archiveExpiredLearnings moves newly expired learnings to the archive directory.
func archiveExpiredLearnings(cwd, learningsDir string, expired []string) error {
	archiveDir := filepath.Join(cwd, ".agents", "archive", "learnings")

	if GetDryRun() {
		fmt.Println()
		for _, name := range expired {
			fmt.Printf("[dry-run] Would archive: %s -> .agents/archive/learnings/%s\n", name, name)
		}
		return nil
	}

	if err := os.MkdirAll(archiveDir, 0o750); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}

	fmt.Println()
	for _, name := range expired {
		src := filepath.Join(learningsDir, name)
		dst := filepath.Join(archiveDir, name)
		if err := os.Rename(src, dst); err != nil {
			fmt.Fprintf(os.Stderr, "Error moving %s: %v\n", name, err)
			continue
		}
		fmt.Printf("Archived: %s -> .agents/archive/learnings/%s\n", name, name)
	}
	return nil
}

// evictionCandidate holds metadata about a learning eligible for eviction.
type evictionCandidate struct {
	Path       string  `json:"path"`
	Name       string  `json:"name"`
	Utility    float64 `json:"utility"`
	Confidence float64 `json:"confidence"`
	Maturity   string  `json:"maturity"`
	LastCited  string  `json:"last_cited,omitempty"`
}

type curationCandidate struct {
	Path          string   `json:"path"`
	Name          string   `json:"name"`
	Utility       float64  `json:"utility"`
	Confidence    float64  `json:"confidence"`
	Maturity      string   `json:"maturity"`
	LastCited     string   `json:"last_cited,omitempty"`
	BodyChars     int      `json:"body_chars"`
	Reasons       []string `json:"reasons"`
	Normalized    bool     `json:"normalized,omitempty"`
	WorkspaceRoot string   `json:"workspace_root,omitempty"`
}

func resolveMaturityRoot(cwd string) (string, string, error) {
	if !maturityGlobal {
		return cwd, filepath.Join(cwd, ".agents", "learnings"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve home directory: %w", err)
	}
	return home, filepath.Join(home, ".agents", "learnings"), nil
}

// buildCitationMap returns a map of canonical artifact path to latest cited_at.
func buildCitationMap(baseDir string) map[string]time.Time {
	result := make(map[string]time.Time)

	citations, err := ratchet.LoadCitations(baseDir)
	if err != nil {
		return result
	}

	for _, entry := range citations {
		key := canonicalArtifactPath(baseDir, entry.ArtifactPath)
		if key == "" {
			continue
		}
		if existing, ok := result[key]; !ok || entry.CitedAt.After(existing) {
			result[key] = entry.CitedAt
		}
	}

	return result
}

func runMaturityCurate(cmd *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	baseDir, learningsDir, err := resolveMaturityRoot(cwd)
	if err != nil {
		return err
	}
	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		fmt.Println("No learnings directory found.")
		return nil
	}

	files, err := ratchet.GlobLearningFiles(learningsDir)
	if err != nil {
		return fmt.Errorf("glob learnings: %w", err)
	}
	lastCited := buildCitationMap(baseDir)
	cutoff := time.Now().AddDate(0, 0, -maturityUncitedDays)

	candidates := make([]curationCandidate, 0, len(files))
	normalized := 0
	for _, file := range files {
		candidate, ok, changed, curateErr := buildCurationCandidate(baseDir, file, lastCited, cutoff)
		if curateErr != nil {
			VerbosePrintf("Warning: curate %s: %v\n", filepath.Base(file), curateErr)
			continue
		}
		if changed {
			normalized++
		}
		if ok {
			candidates = append(candidates, candidate)
		}
	}

	shouldArchive, err := reportCurationCandidates(files, normalized, candidates)
	if err != nil {
		return err
	}
	if !shouldArchive || !maturityArchive {
		return nil
	}

	return archiveCurationCandidates(baseDir, candidates)
}

func buildCurationCandidate(baseDir, file string, lastCited map[string]time.Time, cutoff time.Time) (curationCandidate, bool, bool, error) {
	info, statErr := os.Stat(file)
	if statErr != nil {
		return curationCandidate{}, false, false, statErr
	}
	originalModTime := info.ModTime()

	normalized, err := normalizeLearningMetadata(file, GetDryRun())
	if err != nil {
		return curationCandidate{}, false, false, err
	}

	l, err := parseLearningFile(file)
	if err != nil {
		return curationCandidate{}, false, normalized, err
	}
	if l.Superseded {
		return curationCandidate{}, false, normalized, nil
	}

	maturity := strings.TrimSpace(l.Maturity)
	if maturity == "" {
		maturity = "provisional"
	}
	body := strings.TrimSpace(stripLearningHeading(l.BodyText))
	reasons := make([]string, 0, 2)
	if isLowSignalLearningBody(body) {
		reasons = append(reasons, "low-signal-body")
	}

	lastCitedStr := ""
	if shouldArchiveUncitedLearning(baseDir, file, maturity, originalModTime, lastCited, cutoff) {
		reasons = append(reasons, fmt.Sprintf("uncited-%dd", maturityUncitedDays))
		if citedAt, ok := lastCited[canonicalArtifactPath(baseDir, file)]; ok {
			lastCitedStr = citedAt.Format("2006-01-02")
		} else {
			lastCitedStr = "never"
		}
	}
	if len(reasons) == 0 {
		return curationCandidate{}, false, normalized, nil
	}

	data, ok := readLearningData(file)
	if !ok {
		data = map[string]any{}
	}
	if lastCitedStr == "" {
		if citedAt, ok := lastCited[canonicalArtifactPath(baseDir, file)]; ok {
			lastCitedStr = citedAt.Format("2006-01-02")
		} else {
			lastCitedStr = "never"
		}
	}

	return curationCandidate{
		Path:          file,
		Name:          filepath.Base(file),
		Utility:       floatValueFromData(data, "utility", types.InitialUtility),
		Confidence:    floatValueFromData(data, "confidence", 0.0),
		Maturity:      maturity,
		LastCited:     lastCitedStr,
		BodyChars:     len(body),
		Reasons:       reasons,
		Normalized:    normalized,
		WorkspaceRoot: baseDir,
	}, true, normalized, nil
}

func shouldArchiveUncitedLearning(baseDir, file, maturity string, modTime time.Time, lastCited map[string]time.Time, cutoff time.Time) bool {
	_, cited := lastCited[canonicalArtifactPath(baseDir, file)]
	return lifecycle.ShouldArchiveUncitedLearning(maturity, modTime, cited, cutoff)
}

func isLowSignalLearningBody(body string) bool {
	return lifecycle.IsLowSignalLearningBody(body)
}

func stripLearningHeading(content string) string {
	return lifecycle.StripLearningHeading(content)
}

func reportCurationCandidates(files []string, normalized int, candidates []curationCandidate) (bool, error) {
	fmt.Printf("=== Corpus Curation ===\n")
	fmt.Printf("  Learnings scanned:  %d\n", len(files))
	fmt.Printf("  Metadata normalized: %d\n", normalized)
	fmt.Printf("  Archive candidates: %d\n", len(candidates))
	fmt.Println()

	if len(candidates) == 0 {
		fmt.Println("No curation candidates found.")
		return false, nil
	}

	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return false, enc.Encode(candidates)
	}

	fmt.Printf("Candidates (low-signal body or uncited for %d days):\n", maturityUncitedDays)
	for _, c := range candidates {
		fmt.Printf("  %s  reasons=%s  body=%d  utility=%.3f  confidence=%.3f  maturity=%s  last_cited=%s\n",
			c.Name,
			strings.Join(c.Reasons, ","),
			c.BodyChars,
			c.Utility,
			c.Confidence,
			c.Maturity,
			c.LastCited,
		)
	}

	return true, nil
}

func archiveCurationCandidates(baseDir string, candidates []curationCandidate) error {
	archiveDir := filepath.Join(baseDir, ".agents", "archive", "learnings")
	if GetDryRun() {
		fmt.Println()
		for _, c := range candidates {
			fmt.Printf("[dry-run] Would archive: %s -> .agents/archive/learnings/%s\n", c.Name, c.Name)
		}
		return nil
	}
	if err := os.MkdirAll(archiveDir, 0o750); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}

	fmt.Println()
	archived := 0
	for _, c := range candidates {
		dst := filepath.Join(archiveDir, c.Name)
		if err := os.Rename(c.Path, dst); err != nil {
			fmt.Fprintf(os.Stderr, "Error moving %s: %v\n", c.Name, err)
			continue
		}
		fmt.Printf("Archived: %s -> .agents/archive/learnings/%s\n", c.Name, c.Name)
		archived++
	}
	fmt.Printf("\nArchived %d learning(s).\n", archived)
	return nil
}

func runMaturityEvict(cmd *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		fmt.Println("No learnings directory found.")
		return nil
	}

	lastCited := buildCitationMap(cwd)
	files, err := ratchet.GlobLearningFiles(learningsDir)
	if err != nil {
		return fmt.Errorf("glob learnings: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -90)
	candidates := collectEvictionCandidates(cwd, files, lastCited, cutoff)

	shouldArchive, err := reportEvictionCandidates(files, candidates)
	if err != nil {
		return err
	}
	if !shouldArchive || !maturityArchive {
		return nil
	}

	return archiveEvictionCandidates(cwd, candidates)
}

func collectEvictionCandidates(baseDir string, files []string, lastCited map[string]time.Time, cutoff time.Time) []evictionCandidate {
	candidates := make([]evictionCandidate, 0, len(files))
	for _, file := range files {
		candidate, ok := buildEvictionCandidate(baseDir, file, lastCited, cutoff)
		if ok {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func buildEvictionCandidate(baseDir, file string, lastCited map[string]time.Time, cutoff time.Time) (evictionCandidate, bool) {
	data, ok := readLearningData(file)
	if !ok {
		return evictionCandidate{}, false
	}

	utility := floatValueFromData(data, "utility", 0.5)
	confidence := floatValueFromData(data, "confidence", 0.5)
	maturity := nonEmptyStringFromData(data, "maturity", "provisional")
	if !isEvictionEligible(utility, confidence, maturity) {
		return evictionCandidate{}, false
	}

	lastCitedStr, ok := evictionCitationStatus(canonicalArtifactPath(baseDir, file), lastCited, cutoff)
	if !ok {
		return evictionCandidate{}, false
	}

	return evictionCandidate{
		Path:       file,
		Name:       filepath.Base(file),
		Utility:    utility,
		Confidence: confidence,
		Maturity:   maturity,
		LastCited:  lastCitedStr,
	}, true
}

func readLearningJSONLData(file string) (map[string]any, bool) {
	return lifecycle.ReadLearningJSONLData(file)
}

// readLearningData dispatches to the appropriate reader based on file extension.
// Returns metadata map and true if data was successfully read.
func readLearningData(file string) (map[string]any, bool) {
	if strings.HasSuffix(file, ".jsonl") {
		return readLearningJSONLData(file)
	}
	// For .md: parse frontmatter fields
	fields, err := parseFrontmatterFields(file, "utility", "confidence", "maturity",
		"helpful_count", "harmful_count", "reward_count")
	if err != nil {
		return nil, false
	}
	data := lifecycle.ParseFrontmatterFloats(fields)
	return data, len(data) > 0
}

func isEvictionEligible(utility, confidence float64, maturity string) bool {
	return lifecycle.IsEvictionEligible(utility, confidence, maturity)
}

func evictionCitationStatus(file string, lastCited map[string]time.Time, cutoff time.Time) (string, bool) {
	return lifecycle.EvictionCitationStatus(file, lastCited, cutoff)
}

func reportEvictionCandidates(files []string, candidates []evictionCandidate) (bool, error) {
	fmt.Printf("=== Eviction Scan ===\n")
	fmt.Printf("  Learnings scanned: %d\n", len(files))
	fmt.Printf("  Eviction candidates: %d\n", len(candidates))
	fmt.Println()

	if len(candidates) == 0 {
		fmt.Println("No eviction candidates found.")
		return false, nil
	}

	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return false, enc.Encode(candidates)
	}

	fmt.Println("Candidates (utility < 0.3, confidence < 0.3, not cited in 90d, not established):")
	for _, c := range candidates {
		fmt.Printf("  %s  utility=%.3f  confidence=%.3f  maturity=%s  last_cited=%s\n",
			c.Name, c.Utility, c.Confidence, c.Maturity, c.LastCited)
	}

	return true, nil
}

func archiveEvictionCandidates(cwd string, candidates []evictionCandidate) error {
	archiveDir := filepath.Join(cwd, ".agents", "archive", "learnings")

	if GetDryRun() {
		fmt.Println()
		for _, c := range candidates {
			fmt.Printf("[dry-run] Would archive: %s -> .agents/archive/learnings/%s\n", c.Name, c.Name)
		}
		return nil
	}

	if err := os.MkdirAll(archiveDir, 0o750); err != nil {
		return fmt.Errorf("create archive directory: %w", err)
	}

	fmt.Println()
	archived := 0
	for _, c := range candidates {
		dst := filepath.Join(archiveDir, c.Name)
		if err := os.Rename(c.Path, dst); err != nil {
			fmt.Fprintf(os.Stderr, "Error moving %s: %v\n", c.Name, err)
			continue
		}
		fmt.Printf("Archived: %s -> .agents/archive/learnings/%s\n", c.Name, c.Name)
		archived++
	}
	fmt.Printf("\nArchived %d learning(s).\n", archived)
	return nil
}

func floatValueFromData(data map[string]any, key string, defaultValue float64) float64 {
	return lifecycle.FloatValueFromData(data, key, defaultValue)
}

func nonEmptyStringFromData(data map[string]any, key, defaultValue string) string {
	return lifecycle.NonEmptyStringFromData(data, key, defaultValue)
}

// antiPatternCmd lists and manages anti-patterns.
var antiPatternCmd = &cobra.Command{
	Use:   "anti-patterns",
	Short: "List learnings marked as anti-patterns",
	Long: `List learnings that have been marked as anti-patterns.

Anti-patterns are learnings that have received consistent harmful feedback
(utility <= 0.2 and harmful_count >= 3). They are surfaced to agents as
examples of what NOT to do.

Examples:
  ao anti-patterns                    # List all anti-patterns
  ao anti-patterns --format json      # Output as JSON`,
	RunE: runAntiPatterns,
}

func init() {
	antiPatternCmd.GroupID = "core"
	rootCmd.AddCommand(antiPatternCmd)
}

func runAntiPatterns(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		if GetOutput() == "json" {
			fmt.Println("[]")
			return nil
		}
		fmt.Println("No learnings directory found.")
		return nil
	}

	antiPatterns, err := ratchet.GetAntiPatterns(learningsDir)
	if err != nil {
		return fmt.Errorf("get anti-patterns: %w", err)
	}

	if len(antiPatterns) == 0 {
		if GetOutput() == "json" {
			fmt.Println("[]")
			return nil
		}
		fmt.Println("No anti-patterns found.")
		return nil
	}

	if GetOutput() == "json" {
		data, err := json.MarshalIndent(antiPatterns, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal anti-patterns: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Found %d anti-pattern(s):\n\n", len(antiPatterns))
	for _, path := range antiPatterns {
		// Read summary from the file
		result, err := ratchet.CheckMaturityTransition(path)
		if err != nil {
			fmt.Printf("  • %s\n", filepath.Base(path))
			continue
		}

		fmt.Printf("  • %s\n", result.LearningID)
		fmt.Printf("    Utility: %.3f, Harmful: %d, Reason: %s\n",
			result.Utility, result.HarmfulCount, result.Reason)
	}

	return nil
}
