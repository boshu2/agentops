// practices: [llm-eval-harness, ai-assisted-dev]
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/boshu2/agentops/cli/internal/lifecycle"
	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

var (
	temperMinMaturity string
	temperMinUtility  float64
	temperMinFeedback int
	temperForce       bool
	temperRecursive   bool
)

var temperCmd = &cobra.Command{
	Use:   "temper",
	Short: "TEMPER phase - validate and lock artifacts",
	Long: `The TEMPER phase validates and locks knowledge artifacts.

In the metallurgical metaphor:
  FORGE  → Extract raw knowledge from transcripts
  TEMPER → Validate, harden, and lock for storage
  STORE  → Index for retrieval and search

The temper command ensures artifacts meet quality requirements before
being permanently stored in the knowledge base.

Commands:
  validate   Check artifact structure and MemRL requirements
  lock       Lock validated artifacts (engage ratchet)
  status     Show tempered vs pending artifacts`,
}

// TemperResult holds validation results for an artifact.
type TemperResult struct {
	Path          string         `json:"path"`
	Valid         bool           `json:"valid"`
	Tempered      bool           `json:"tempered,omitempty"`
	Issues        []string       `json:"issues,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
	Maturity      types.Maturity `json:"maturity,omitempty"`
	Utility       float64        `json:"utility,omitempty"`
	Confidence    float64        `json:"confidence,omitempty"`
	FeedbackCount int            `json:"feedback_count,omitempty"`
	ValidatedAt   time.Time      `json:"validated_at,omitempty"`
}

// TemperStatus holds overall status of tempered artifacts.
type TemperStatus struct {
	Tempered    int            `json:"tempered"`
	Pending     int            `json:"pending"`
	ByMaturity  map[string]int `json:"by_maturity"`
	MeanUtility float64        `json:"mean_utility"`
	Artifacts   []TemperResult `json:"artifacts,omitempty"`
}

func init() {
	temperCmd.Hidden = true
	temperCmd.GroupID = "knowledge"
	rootCmd.AddCommand(temperCmd)

	// validate subcommand
	validateCmd := &cobra.Command{
		Use:   "validate <files...>",
		Short: "Validate artifact structure",
		Long: `Check that artifacts meet quality requirements for TEMPER.

Validates:
  - Required fields (ID, date, schema version)
  - MemRL maturity thresholds
  - Utility and confidence requirements
  - Minimum feedback count

Examples:
  ao temper validate .agents/learnings/*.md
  ao temper validate .agents/patterns/error-handling.md
  ao temper validate --min-maturity=candidate --min-utility=0.5`,
		Args: cobra.MinimumNArgs(1),
		RunE: runTemperValidate,
	}
	validateCmd.Flags().StringVar(&temperMinMaturity, "min-maturity", "provisional", "Minimum maturity level (provisional, candidate, established)")
	validateCmd.Flags().Float64Var(&temperMinUtility, "min-utility", 0.5, "Minimum utility threshold")
	validateCmd.Flags().IntVar(&temperMinFeedback, "min-feedback", 1, "Minimum feedback count")
	temperCmd.AddCommand(validateCmd)

	// lock subcommand
	lockCmd := &cobra.Command{
		Use:   "lock <files...>",
		Short: "Lock validated artifacts",
		Long: `Lock artifacts that have passed validation.

This engages the ratchet - locked artifacts cannot be modified without
explicit unlock. Ensures knowledge stability.

Examples:
  ao temper lock .agents/learnings/mutex-pattern.md
  ao temper lock .agents/patterns/*.md --force
  ao temper lock --recursive .agents/learnings/`,
		Args: cobra.MinimumNArgs(1),
		RunE: runTemperLock,
	}
	lockCmd.Flags().BoolVar(&temperForce, "force", false, "Lock without validation check")
	lockCmd.Flags().BoolVar(&temperRecursive, "recursive", false, "Process directories recursively")
	temperCmd.AddCommand(lockCmd)

	// status subcommand
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show tempered vs pending artifacts",
		Long: `Display status of artifacts in the TEMPER pipeline.

Shows:
  - Count of tempered (locked) vs pending artifacts
  - Breakdown by maturity level
  - Mean utility across artifacts
  - List of artifacts needing attention

Examples:
  ao temper status
  ao temper status --json
  ao temper status --verbose`,
		RunE: runTemperStatus,
	}
	temperCmd.AddCommand(statusCmd)
}

// validateTemperFiles runs validation on each file and returns (results, allValid).
func validateTemperFiles(files []string) ([]TemperResult, bool) {
	var results []TemperResult
	allValid := true
	for _, path := range files {
		result := validateArtifact(path, temperMinMaturity, temperMinUtility, temperMinFeedback)
		results = append(results, result)
		if !result.Valid {
			allValid = false
		}
	}
	return results, allValid
}

func runTemperValidate(cmd *cobra.Command, args []string) error {
	w := cmd.OutOrStdout()

	if GetDryRun() {
		fmt.Fprintf(w, "[dry-run] Would validate %d file(s)\n", len(args))
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	files, err := expandFilePatterns(cwd, args)
	if err != nil {
		return fmt.Errorf("expand patterns: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found matching patterns")
	}

	results, allValid := validateTemperFiles(files)

	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	case "yaml":
		enc := yaml.NewEncoder(w)
		return enc.Encode(results)
	default:
		printValidationResults(results)
	}

	if !allValid {
		return fmt.Errorf("validation failed: one or more artifacts are invalid")
	}

	return nil
}

// tryValidateAndLock attempts to validate and lock a single artifact.
// Returns true if locked successfully, false if skipped.
func tryValidateAndLock(baseDir, path string) bool {
	if !temperForce {
		result := validateArtifact(path, temperMinMaturity, temperMinUtility, temperMinFeedback)
		if !result.Valid {
			fmt.Fprintf(os.Stderr, "Skipping %s: validation failed\n", filepath.Base(path))
			for _, issue := range result.Issues {
				fmt.Fprintf(os.Stderr, "  - %s\n", issue)
			}
			return false
		}
	}

	if err := lockArtifact(baseDir, path); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to lock %s: %v\n", filepath.Base(path), err)
		return false
	}

	VerbosePrintf("Locked: %s\n", filepath.Base(path))
	return true
}

func runTemperLock(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	files, err := expandFilePatterns(cwd, args)
	if err != nil {
		return fmt.Errorf("expand patterns: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found matching patterns")
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would lock %d file(s)\n", len(files))
		for _, f := range files {
			fmt.Printf("  - %s\n", f)
		}
		return nil
	}

	locked, skipped := 0, 0
	for _, path := range files {
		if tryValidateAndLock(cwd, path) {
			locked++
		} else {
			skipped++
		}
	}

	fmt.Printf("\nTempered %d artifact(s)", locked)
	if skipped > 0 {
		fmt.Printf(", %d skipped", skipped)
	}
	fmt.Println()

	return nil
}

func runTemperStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	status, err := computeTemperStatus(cwd)
	if err != nil {
		return fmt.Errorf("compute status: %w", err)
	}

	switch GetOutput() {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)

	case "yaml":
		enc := yaml.NewEncoder(os.Stdout)
		return enc.Encode(status)

	default:
		printTemperStatus(status)
	}

	return nil
}

// validateArtifact checks if an artifact meets TEMPER requirements.
func validateArtifact(path, minMaturity string, minUtility float64, minFeedback int) TemperResult {
	result := TemperResult{
		Path:        path,
		Valid:       true,
		ValidatedAt: time.Now(),
	}

	// Check file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		result.Valid = false
		result.Issues = append(result.Issues, "file not found")
		return result
	}

	// Parse artifact for metadata
	meta, err := parseArtifactMetadata(path)
	if err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("parse error: %v", err))
		return result
	}

	result.Maturity = meta.Maturity
	result.Utility = meta.Utility
	result.Confidence = meta.Confidence
	result.FeedbackCount = meta.FeedbackCount
	result.Tempered = meta.Tempered

	// Delegate pure validation to lifecycle package
	lm := lifecycle.ArtifactMeta{
		ID: meta.ID, Maturity: string(meta.Maturity), Utility: meta.Utility,
		Confidence: meta.Confidence, FeedbackCount: meta.FeedbackCount,
		SchemaVersion: meta.SchemaVersion, Tempered: meta.Tempered,
	}
	issues, warnings := lifecycle.ValidateArtifactMeta(lm, minMaturity, minUtility, minFeedback)
	result.Issues = append(result.Issues, issues...)
	result.Warnings = append(result.Warnings, warnings...)
	if len(issues) > 0 {
		result.Valid = false
	}

	return result
}

// artifactMetadata holds parsed metadata from an artifact.
type artifactMetadata struct {
	ID            string
	Maturity      types.Maturity
	Utility       float64
	Confidence    float64
	FeedbackCount int
	SchemaVersion int
	Tempered      bool
}

// parseFrontmatterMetadata extracts metadata from YAML frontmatter on markdown artifacts.
func parseFrontmatterMetadata(path string, meta *artifactMetadata) {
	fields, err := parseFrontmatterFields(
		path,
		"id",
		"maturity",
		"utility",
		"confidence",
		"reward_count",
		"schema_version",
		"status",
	)
	if err != nil {
		return
	}

	if fields["id"] != "" {
		meta.ID = fields["id"]
	}
	if fields["maturity"] != "" {
		meta.Maturity = types.Maturity(strings.ToLower(fields["maturity"]))
	}
	if fields["utility"] != "" {
		//nolint:errcheck // parsing optional metadata, zero value is acceptable default
		fmt.Sscanf(fields["utility"], "%f", &meta.Utility) // #nosec G104
	}
	if fields["confidence"] != "" {
		//nolint:errcheck // parsing optional metadata, zero value is acceptable default
		fmt.Sscanf(fields["confidence"], "%f", &meta.Confidence) // #nosec G104
	}
	if fields["reward_count"] != "" {
		//nolint:errcheck // parsing optional metadata, zero value is acceptable default
		fmt.Sscanf(fields["reward_count"], "%d", &meta.FeedbackCount) // #nosec G104
	}
	if fields["schema_version"] != "" {
		//nolint:errcheck // parsing optional metadata, zero value is acceptable default
		fmt.Sscanf(fields["schema_version"], "%d", &meta.SchemaVersion) // #nosec G104
	}
	if status := strings.ToLower(fields["status"]); status == "tempered" || status == "locked" {
		meta.Tempered = true
	}
}

// parseJSONLMetadata extracts metadata from a JSONL artifact file.
func parseJSONLMetadata(path string, meta *artifactMetadata) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // read-only metadata parse, close error non-fatal
	}()

	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		var data types.Candidate
		if err := json.Unmarshal(scanner.Bytes(), &data); err == nil {
			meta.ID = data.ID
			meta.Maturity = data.Maturity
			meta.Utility = data.Utility
			meta.Confidence = data.Confidence
			meta.FeedbackCount = data.RewardCount
		}
	}
	return nil
}

// parseMarkdownField extracts a value for a field from a markdown line.
func parseMarkdownField(line, field string) (string, bool) {
	return lifecycle.ParseMarkdownField(line, field)
}

// applyMarkdownLine applies a single parsed markdown line to the metadata.
func applyMarkdownLine(line string, meta *artifactMetadata) {
	lm := lifecycle.ArtifactMeta{
		ID: meta.ID, Maturity: string(meta.Maturity), Utility: meta.Utility,
		Confidence: meta.Confidence, SchemaVersion: meta.SchemaVersion, Tempered: meta.Tempered,
	}
	lifecycle.ApplyMarkdownLine(line, &lm)
	meta.ID = lm.ID
	meta.Maturity = types.Maturity(lm.Maturity)
	meta.Utility = lm.Utility
	meta.Confidence = lm.Confidence
	meta.SchemaVersion = lm.SchemaVersion
	meta.Tempered = lm.Tempered
}

// parseMarkdownMetadata extracts metadata from markdown content.
func parseMarkdownMetadata(content string, meta *artifactMetadata) {
	lm := lifecycle.ParseMarkdownMeta(content)
	if lm.ID != "" {
		meta.ID = lm.ID
	}
	if lm.Maturity != "" {
		meta.Maturity = types.Maturity(lm.Maturity)
	}
	if lm.Utility != 0 {
		meta.Utility = lm.Utility
	}
	if lm.Confidence != 0 {
		meta.Confidence = lm.Confidence
	}
	if lm.SchemaVersion != 0 {
		meta.SchemaVersion = lm.SchemaVersion
	}
	if lm.Tempered {
		meta.Tempered = true
	}
}

// parseArtifactMetadata extracts metadata from an artifact file.
func parseArtifactMetadata(path string) (*artifactMetadata, error) {
	meta := &artifactMetadata{
		Maturity:   types.MaturityProvisional,
		Utility:    types.InitialUtility,
		Confidence: 0.5,
	}

	if strings.HasSuffix(path, ".jsonl") {
		if err := parseJSONLMetadata(path, meta); err != nil {
			return nil, err
		}
		if meta.ID != "" {
			return meta, nil
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	parseFrontmatterMetadata(path, meta)
	parseMarkdownMetadata(string(content), meta)

	if meta.ID == "" {
		base := filepath.Base(path)
		meta.ID = strings.TrimSuffix(strings.TrimSuffix(base, ".md"), ".jsonl")
	}

	return meta, nil
}

// lockArtifact locks an artifact via the ratchet system.
func lockArtifact(baseDir, path string) error {
	chain, err := ratchet.LoadChain(baseDir)
	if err != nil {
		return fmt.Errorf("load chain: %w", err)
	}

	entry := ratchet.ChainEntry{
		Step:      ratchet.Step("temper"),
		Timestamp: time.Now(),
		Output:    path,
		Locked:    true,
	}

	return chain.Append(entry)
}

// countPoolPending adds pending pool entries to status.
func countPoolPending(baseDir string, status *TemperStatus) {
	p := pool.NewPool(baseDir)
	entries, err := p.List(pool.ListOptions{})
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Status == types.PoolStatusPending || e.Status == types.PoolStatusStaged {
			status.Pending++
			status.ByMaturity[string(e.Candidate.Maturity)]++
		}
	}
}

// scanArtifactDir scans a directory and updates status with artifact counts.
func scanArtifactDir(dir string, status *TemperStatus, totalUtility *float64, utilityCount *int) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !isArtifactFile(info.Name()) {
			return nil
		}

		meta, err := parseArtifactMetadata(path)
		if err != nil {
			return nil
		}

		status.Tempered++
		status.ByMaturity[string(meta.Maturity)]++

		if meta.Utility > 0 {
			*totalUtility += meta.Utility
			*utilityCount++
		}

		if GetVerbose() {
			status.Artifacts = append(status.Artifacts, TemperResult{
				Path:       path,
				Tempered:   true,
				Maturity:   meta.Maturity,
				Utility:    meta.Utility,
				Confidence: meta.Confidence,
			})
		}
		return nil
	})
	if err != nil {
		VerbosePrintf("Warning: scan %s: %v\n", dir, err)
	}
}

// computeTemperStatus calculates overall status of artifacts.
func computeTemperStatus(baseDir string) (*TemperStatus, error) {
	status := &TemperStatus{
		ByMaturity: make(map[string]int),
	}

	countPoolPending(baseDir, status)

	dirs := []string{
		filepath.Join(baseDir, ".agents", "learnings"),
		filepath.Join(baseDir, ".agents", "patterns"),
	}

	var totalUtility float64
	var utilityCount int

	for _, dir := range dirs {
		scanArtifactDir(dir, status, &totalUtility, &utilityCount)
	}

	if utilityCount > 0 {
		status.MeanUtility = totalUtility / float64(utilityCount)
	}

	return status, nil
}

// isContainedPath checks if path is contained within baseDir.
func isContainedPath(baseDir, path string) bool {
	return lifecycle.IsContainedPath(baseDir, path)
}

// isArtifactFile checks if a filename is a valid artifact file type.
func isArtifactFile(name string) bool {
	return lifecycle.IsArtifactFile(name)
}

func expandDirectoryRecursive(baseDir, dir string) ([]string, error) {
	return lifecycle.ExpandDirectoryRecursive(baseDir, dir)
}

func expandDirectoryFlat(dir string) []string { return lifecycle.ExpandDirectoryFlat(dir) }

func expandGlobPattern(baseDir, pattern string) ([]string, error) {
	return lifecycle.ExpandGlobPattern(baseDir, pattern)
}

func expandDirectory(baseDir, dir string) ([]string, error) {
	if temperRecursive {
		return expandDirectoryRecursive(baseDir, dir)
	}
	return expandDirectoryFlat(dir), nil
}

func expandSinglePattern(baseDir, pattern string) ([]string, error) {
	return lifecycle.ExpandSinglePattern(baseDir, pattern, temperRecursive)
}

// expandFilePatterns expands glob patterns and handles recursive flag.
func expandFilePatterns(baseDir string, patterns []string) ([]string, error) {
	return lifecycle.ExpandFilePatterns(baseDir, patterns, temperRecursive)
}

// printValidationResults prints validation results in table format.
func printValidationResults(results []TemperResult) {
	fmt.Println()
	fmt.Println("TEMPER Validation Results")
	fmt.Println("=========================")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	//nolint:errcheck // CLI tabwriter output to stdout, errors unlikely and non-recoverable
	fmt.Fprintln(w, "FILE\tSTATUS\tMATURITY\tUTILITY")
	//nolint:errcheck // CLI tabwriter output to stdout
	fmt.Fprintln(w, "----\t------\t--------\t-------")

	for _, r := range results {
		status := "VALID"
		if !r.Valid {
			status = "INVALID"
		}
		//nolint:errcheck // CLI tabwriter output to stdout
		fmt.Fprintf(w, "%s\t%s\t%s\t%.2f\n",
			filepath.Base(r.Path),
			status,
			r.Maturity,
			r.Utility,
		)
	}
	_ = w.Flush()

	// Print issues
	hasIssues := false
	for _, r := range results {
		if len(r.Issues) > 0 || len(r.Warnings) > 0 {
			if !hasIssues {
				fmt.Println("\nDetails:")
				hasIssues = true
			}
			fmt.Printf("\n%s:\n", filepath.Base(r.Path))
			for _, issue := range r.Issues {
				fmt.Printf("  ERROR: %s\n", issue)
			}
			for _, warn := range r.Warnings {
				fmt.Printf("  WARN: %s\n", warn)
			}
		}
	}
}

// printTemperStatus prints status in table format.
func printTemperStatus(status *TemperStatus) {
	fmt.Println()
	fmt.Println("TEMPER Pipeline Status")
	fmt.Println("======================")
	fmt.Println()

	fmt.Printf("Tempered (locked): %d\n", status.Tempered)
	fmt.Printf("Pending review:    %d\n", status.Pending)
	fmt.Printf("Mean utility:      %.2f\n", status.MeanUtility)
	fmt.Println()

	if len(status.ByMaturity) > 0 {
		fmt.Println("By Maturity:")
		maturities := []string{"provisional", "candidate", "established", "anti-pattern"}
		for _, m := range maturities {
			if count, ok := status.ByMaturity[m]; ok && count > 0 {
				fmt.Printf("  %-15s: %d\n", m, count)
			}
		}
	}

	if len(status.Artifacts) > 0 {
		fmt.Println("\nArtifacts:")
		for _, a := range status.Artifacts {
			status := "TEMPERED"
			if !a.Tempered {
				status = "pending"
			}
			fmt.Printf("  %s [%s] utility=%.2f maturity=%s\n",
				filepath.Base(a.Path), status, a.Utility, a.Maturity)
		}
	}
}
