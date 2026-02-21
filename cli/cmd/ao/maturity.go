package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

var (
	maturityApply   bool
	maturityScan    bool
	maturityExpire  bool
	maturityArchive bool
	maturityEvict   bool
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
  provisional → candidate:    utility >= 0.7 AND reward_count >= 3
  candidate → established:    utility >= 0.7 AND reward_count >= 5 AND helpful > harmful
  any → anti-pattern:         utility <= 0.2 AND harmful_count >= 5
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
	rootCmd.AddCommand(maturityCmd)
	maturityCmd.Flags().BoolVar(&maturityApply, "apply", false, "Apply maturity transitions")
	maturityCmd.Flags().BoolVar(&maturityScan, "scan", false, "Scan all learnings for pending transitions")
	maturityCmd.Flags().BoolVar(&maturityExpire, "expire", false, "Scan for expired learnings")
	maturityCmd.Flags().BoolVar(&maturityArchive, "archive", false, "Move expired/evicted files to archive (requires --expire or --evict)")
	maturityCmd.Flags().BoolVar(&maturityEvict, "evict", false, "Identify eviction candidates (composite criteria)")
}

func runMaturity(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		fmt.Println("No learnings directory found.")
		return nil
	}

	// Evict mode: composite eviction criteria
	if maturityEvict {
		return runMaturityEvict(cmd)
	}

	// Expire mode: check for expired learnings
	if maturityExpire {
		return runMaturityExpire(cmd)
	}

	// Scan mode: check all learnings
	if maturityScan {
		return runMaturityScan(learningsDir)
	}

	// Single learning mode
	if len(args) == 0 {
		return fmt.Errorf("must provide learning-id or use --scan")
	}

	learningID := args[0]
	learningPath, err := findLearningFile(cwd, learningID)
	if err != nil {
		return fmt.Errorf("find learning: %w", err)
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would check maturity for: %s\n", learningID)
		return nil
	}

	var result *ratchet.MaturityTransitionResult
	if maturityApply {
		result, err = ratchet.ApplyMaturityTransition(learningPath)
	} else {
		result, err = ratchet.CheckMaturityTransition(learningPath)
	}

	if err != nil {
		return fmt.Errorf("check maturity: %w", err)
	}

	// Output results
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	displayMaturityResult(result, maturityApply)
	return nil
}

func runMaturityScan(learningsDir string) error {
	if GetDryRun() {
		fmt.Printf("[dry-run] Would scan learnings in: %s\n", learningsDir)
		return nil
	}

	// Get distribution first
	dist, err := ratchet.GetMaturityDistribution(learningsDir)
	if err != nil {
		return fmt.Errorf("get distribution: %w", err)
	}

	fmt.Println("=== Maturity Distribution ===")
	fmt.Printf("  Provisional:  %d\n", dist.Provisional)
	fmt.Printf("  Candidate:    %d\n", dist.Candidate)
	fmt.Printf("  Established:  %d\n", dist.Established)
	fmt.Printf("  Anti-Pattern: %d\n", dist.AntiPattern)
	fmt.Printf("  Total:        %d\n", dist.Total)
	fmt.Println()

	// Scan for pending transitions
	results, err := ratchet.ScanForMaturityTransitions(learningsDir)
	if err != nil {
		return fmt.Errorf("scan transitions: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No pending maturity transitions found.")
		return nil
	}

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

	// Apply transitions if requested
	if maturityApply {
		fmt.Println("=== Applying Transitions ===")
		applied := 0
		for _, r := range results {
			learningPath, err := findLearningFile(filepath.Dir(learningsDir), r.LearningID)
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
// Returns a map of field name to value for the requested fields.
func parseFrontmatterFields(path string, fields ...string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(f)
	inFrontmatter := false
	dashCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" {
			dashCount++
			if dashCount == 1 {
				inFrontmatter = true
				continue
			}
			if dashCount == 2 {
				break
			}
		}

		if inFrontmatter {
			for _, field := range fields {
				prefix := field + ":"
				if strings.HasPrefix(trimmed, prefix) {
					val := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
					// Strip surrounding quotes if present
					val = strings.Trim(val, "\"'")
					result[field] = val
				}
			}
		}
	}

	return result, scanner.Err()
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

		path := filepath.Join(learningsDir, entry.Name())
		fields, err := parseFrontmatterFields(path, "valid_until", "expiry_status")
		if err != nil {
			VerbosePrintf("Warning: could not read %s: %v\n", entry.Name(), err)
			cats.neverExpiring = append(cats.neverExpiring, entry.Name())
			continue
		}

		// Check if already archived
		if fields["expiry_status"] == "archived" {
			cats.alreadyArchived = append(cats.alreadyArchived, entry.Name())
			continue
		}

		validUntil, hasExpiry := fields["valid_until"]
		if !hasExpiry || validUntil == "" {
			cats.neverExpiring = append(cats.neverExpiring, entry.Name())
			continue
		}

		// Parse date
		expiry, parseErr := time.Parse("2006-01-02", validUntil)
		if parseErr != nil {
			expiry, parseErr = time.Parse(time.RFC3339, validUntil)
		}
		if parseErr != nil {
			VerbosePrintf("Warning: malformed valid_until in %s: %s\n", entry.Name(), validUntil)
			cats.neverExpiring = append(cats.neverExpiring, entry.Name())
			continue
		}

		if time.Now().After(expiry) {
			cats.newlyExpired = append(cats.newlyExpired, entry.Name())
		} else {
			cats.active = append(cats.active, entry.Name())
		}
	}

	total := len(cats.active) + len(cats.neverExpiring) + len(cats.newlyExpired) + len(cats.alreadyArchived)

	fmt.Println("=== Expiry Scan ===")
	fmt.Printf("  Active:           %d\n", len(cats.active))
	fmt.Printf("  Never-expiring:   %d (no valid_until field)\n", len(cats.neverExpiring))
	fmt.Printf("  Newly expired:    %d\n", len(cats.newlyExpired))
	fmt.Printf("  Already archived: %d\n", len(cats.alreadyArchived))
	fmt.Printf("  Total:            %d\n", total)

	// Archive expired files if requested
	if maturityArchive && len(cats.newlyExpired) > 0 {
		archiveDir := filepath.Join(cwd, ".agents", "archive", "learnings")

		if GetDryRun() {
			fmt.Println()
			for _, name := range cats.newlyExpired {
				fmt.Printf("[dry-run] Would archive: %s -> .agents/archive/learnings/%s\n", name, name)
			}
			return nil
		}

		if err := os.MkdirAll(archiveDir, 0o755); err != nil {
			return fmt.Errorf("create archive directory: %w", err)
		}

		fmt.Println()
		for _, name := range cats.newlyExpired {
			src := filepath.Join(learningsDir, name)
			dst := filepath.Join(archiveDir, name)
			if err := os.Rename(src, dst); err != nil {
				fmt.Fprintf(os.Stderr, "Error moving %s: %v\n", name, err)
				continue
			}
			fmt.Printf("Archived: %s -> .agents/archive/learnings/%s\n", name, name)
		}
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
	files, err := filepath.Glob(filepath.Join(learningsDir, "*.jsonl"))
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
	data, ok := readLearningJSONLData(file)
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

func readLearningJSONLData(file string) (map[string]interface{}, bool) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, false
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, false
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &data); err != nil {
		return nil, false
	}

	return data, true
}

func isEvictionEligible(utility, confidence float64, maturity string) bool {
	if maturity == "established" {
		return false
	}
	if utility >= 0.3 {
		return false
	}
	return confidence < 0.2
}

func evictionCitationStatus(file string, lastCited map[string]time.Time, cutoff time.Time) (string, bool) {
	citedAt, ok := lastCited[file]
	if !ok {
		return "never", true
	}
	if citedAt.After(cutoff) {
		return "", false
	}
	return citedAt.Format("2006-01-02"), true
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

	fmt.Println("Candidates (utility < 0.3, confidence < 0.2, not cited in 90d, not established):")
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

	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
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

func floatValueFromData(data map[string]interface{}, key string, defaultValue float64) float64 {
	value, ok := data[key].(float64)
	if !ok {
		return defaultValue
	}
	return value
}

func nonEmptyStringFromData(data map[string]interface{}, key, defaultValue string) string {
	value, ok := data[key].(string)
	if !ok || value == "" {
		return defaultValue
	}
	return value
}

// antiPatternCmd lists and manages anti-patterns.
var antiPatternCmd = &cobra.Command{
	Use:   "anti-patterns",
	Short: "List learnings marked as anti-patterns",
	Long: `List learnings that have been marked as anti-patterns.

Anti-patterns are learnings that have received consistent harmful feedback
(utility <= 0.2 and harmful_count >= 5). They are surfaced to agents as
examples of what NOT to do.

Examples:
  ao anti-patterns                    # List all anti-patterns
  ao anti-patterns --format json      # Output as JSON`,
	RunE: runAntiPatterns,
}

func init() {
	rootCmd.AddCommand(antiPatternCmd)
}

func runAntiPatterns(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		fmt.Println("No learnings directory found.")
		return nil
	}

	antiPatterns, err := ratchet.GetAntiPatterns(learningsDir)
	if err != nil {
		return fmt.Errorf("get anti-patterns: %w", err)
	}

	if len(antiPatterns) == 0 {
		fmt.Println("No anti-patterns found.")
		return nil
	}

	if GetOutput() == "json" {
		data, _ := json.MarshalIndent(antiPatterns, "", "  ")
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

// promoteAntiPatternsCmd explicitly promotes learnings to anti-pattern status.
var promoteAntiPatternsCmd = &cobra.Command{
	Use:   "promote-anti-patterns",
	Short: "Promote harmful learnings to anti-pattern status",
	Long: `Scan learnings and promote those meeting anti-pattern criteria.

A learning becomes an anti-pattern when:
  - utility <= 0.2 (consistently not helpful)
  - harmful_count >= 5 (multiple negative feedback events)

This is useful for batch processing to identify and mark anti-patterns
that should be surfaced as "what NOT to do".

Examples:
  ao promote-anti-patterns            # Scan and promote
  ao promote-anti-patterns --dry-run  # Preview without changing`,
	RunE: runPromoteAntiPatterns,
}

func init() {
	rootCmd.AddCommand(promoteAntiPatternsCmd)
}

func runPromoteAntiPatterns(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		fmt.Println("No learnings directory found.")
		return nil
	}

	// Scan for all transitions
	results, err := ratchet.ScanForMaturityTransitions(learningsDir)
	if err != nil {
		return fmt.Errorf("scan transitions: %w", err)
	}

	// Filter for anti-pattern promotions only
	var antiPatternPromotions []*ratchet.MaturityTransitionResult
	for _, r := range results {
		if r.NewMaturity == "anti-pattern" {
			antiPatternPromotions = append(antiPatternPromotions, r)
		}
	}

	if len(antiPatternPromotions) == 0 {
		fmt.Println("No learnings eligible for anti-pattern promotion.")
		return nil
	}

	fmt.Printf("Found %d learning(s) eligible for anti-pattern promotion:\n\n", len(antiPatternPromotions))

	for _, r := range antiPatternPromotions {
		fmt.Printf("  • %s (utility: %.3f, harmful: %d)\n",
			r.LearningID, r.Utility, r.HarmfulCount)
	}

	if GetDryRun() {
		fmt.Println("\n[dry-run] Would promote the above learnings to anti-pattern status.")
		return nil
	}

	fmt.Println("\nPromoting to anti-pattern status...")
	promoted := 0
	for _, r := range antiPatternPromotions {
		learningPath, err := findLearningFile(filepath.Dir(learningsDir), r.LearningID)
		if err != nil {
			VerbosePrintf("Warning: could not find %s: %v\n", r.LearningID, err)
			continue
		}

		result, err := ratchet.ApplyMaturityTransition(learningPath)
		if err != nil {
			VerbosePrintf("Warning: could not apply transition for %s: %v\n", r.LearningID, err)
			continue
		}

		if result.Transitioned && result.NewMaturity == "anti-pattern" {
			fmt.Printf("  ✓ %s → anti-pattern\n", result.LearningID)
			promoted++
		}
	}

	fmt.Printf("\nPromoted %d learning(s) to anti-pattern status.\n", promoted)
	return nil
}
