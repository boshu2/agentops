// practices: [wiki-knowledge-surface, team-knowledge-sharing]
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/warmind"
	"github.com/spf13/cobra"
)

var (
	warmindAuto   bool
	warmindQuiet  bool
	warmindSimple bool // Use simple sync (v1 behavior)
)

var warmindCmd = &cobra.Command{
	Use:   "warmind",
	Short: "Team knowledge sharing via warmind",
	Long: `Warmind enables team-wide knowledge sharing by syncing learnings to a
git-tracked directory (.warmind/) that all team members can access.

V2 Features:
  - Pool-based staging with quality scoring
  - Citation-gated promotion (requires citations from OTHER engineers)
  - Team maturity lifecycle with decay
  - Contradiction detection between learnings

Subcommands:
  sync         Copy local learnings to warmind pool for team sharing
  status       Show warmind health metrics
  pool         Manage the warmind pool
  promote      Promote a staged candidate
  contradict   Scan for contradictions
  close-loop   Run full flywheel closure`,
}

var warmindSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync local learnings to team warmind",
	Long: `Copy learnings from .agents/learnings/ to .warmind/pool/.

In V2 mode (default), learnings go through:
  1. Pool staging (pending)
  2. Quality scoring (novelty, specificity, actionability, confidence)
  3. Tier assignment (gold/silver/bronze/discard)
  4. Citation-gated promotion to .warmind/learnings/

Use --simple for V1 behavior (direct copy to .warmind/learnings/).

Examples:
  ao warmind sync              # Full V2 pipeline
  ao warmind sync --simple     # V1 direct copy
  ao warmind sync --auto       # Auto mode for hooks`,
	RunE: runWarmindSync,
}

var warmindStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show warmind health metrics",
	Long: `Display warmind knowledge health metrics:
  - Pool status (pending, staged, rejected)
  - Learning corpus size and maturity distribution
  - Citation statistics
  - Pending contradictions`,
	RunE: runWarmindStatus,
}

var warmindPoolCmd = &cobra.Command{
	Use:   "pool",
	Short: "Manage the warmind pool",
}

var warmindPoolListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pool entries",
	Long: `List candidates in the warmind pool.

Examples:
  ao warmind pool list                # All entries
  ao warmind pool list --staged       # Only staged
  ao warmind pool list --pending      # Only pending`,
	RunE: runWarmindPoolList,
}

var warmindPromoteCmd = &cobra.Command{
	Use:   "promote <candidate-id>",
	Short: "Promote a staged candidate",
	Long: `Manually promote a staged candidate to team learnings.

Promotion requires meeting citation thresholds:
  - Gold: auto-promote after 24h
  - Silver: 1 citation from another engineer
  - Bronze: 3 citations from other engineers

Use --force to bypass citation requirements (admin override).

Examples:
  ao warmind promote learn-2026-05-27-abc123
  ao warmind promote learn-2026-05-27-abc123 --force`,
	Args: cobra.ExactArgs(1),
	RunE: runWarmindPromote,
}

var warmindContradictCmd = &cobra.Command{
	Use:   "contradict",
	Short: "Scan for contradictions",
	Long: `Scan team learnings for potential contradictions.

Detects opposing recommendations between learnings, such as:
  - "always use X" vs "never use X"
  - "do Y" vs "don't do Y"

Contradictions require manual review and resolution.

Examples:
  ao warmind contradict              # Scan for new contradictions
  ao warmind contradict --list       # List pending contradictions`,
	RunE: runWarmindContradict,
}

var warmindCloseLoopCmd = &cobra.Command{
	Use:   "close-loop",
	Short: "Run full flywheel closure",
	Long: `Run the complete warmind flywheel closure:
  1. Check staged candidates for auto-promotion (gold tier + age)
  2. Check citation counts for promotion eligibility
  3. Run maturity maintenance (decay, archival)
  4. Scan for contradictions

Examples:
  ao warmind close-loop
  ao warmind close-loop --quiet`,
	RunE: runWarmindCloseLoop,
}

var (
	warmindPoolStaged   bool
	warmindPoolPending  bool
	warmindPromoteForce bool
	warmindContradictList bool
)

func init() {
	warmindCmd.AddCommand(warmindSyncCmd)
	warmindCmd.AddCommand(warmindStatusCmd)
	warmindCmd.AddCommand(warmindPoolCmd)
	warmindCmd.AddCommand(warmindPromoteCmd)
	warmindCmd.AddCommand(warmindContradictCmd)
	warmindCmd.AddCommand(warmindCloseLoopCmd)

	warmindPoolCmd.AddCommand(warmindPoolListCmd)

	warmindSyncCmd.Flags().BoolVar(&warmindAuto, "auto", false, "Auto mode for hooks (quiet, skips if nothing new)")
	warmindSyncCmd.Flags().BoolVar(&warmindQuiet, "quiet", false, "Suppress output")
	warmindSyncCmd.Flags().BoolVar(&warmindSimple, "simple", false, "Use simple V1 sync (direct copy)")

	warmindPoolListCmd.Flags().BoolVar(&warmindPoolStaged, "staged", false, "Show only staged entries")
	warmindPoolListCmd.Flags().BoolVar(&warmindPoolPending, "pending", false, "Show only pending entries")

	warmindPromoteCmd.Flags().BoolVar(&warmindPromoteForce, "force", false, "Force promotion (bypass citation requirements)")

	warmindContradictCmd.Flags().BoolVar(&warmindContradictList, "list", false, "List pending contradictions")

	warmindCloseLoopCmd.Flags().BoolVar(&warmindQuiet, "quiet", false, "Suppress output")

	warmindCmd.GroupID = "knowledge"
	rootCmd.AddCommand(warmindCmd)
}

func runWarmindSync(cmd *cobra.Command, args []string) error {
	if warmindSimple {
		return runWarmindSyncSimple()
	}
	return runWarmindSyncV2()
}

// runWarmindSyncV2 runs the full V2 pipeline with pool staging and scoring.
func runWarmindSyncV2() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg := warmind.DefaultConfig()
	srcDir := filepath.Join(cwd, ".agents", "learnings")

	// Check if source exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		if !warmindQuiet && !warmindAuto {
			fmt.Println("No local learnings found (.agents/learnings/)")
		}
		return nil
	}

	// Initialize pool and scorer
	pool := warmind.NewPool(cwd, cfg)
	if err := pool.Init(); err != nil {
		return fmt.Errorf("init pool: %w", err)
	}

	scorer := warmind.NewScorer(cfg.Scoring, filepath.Join(cwd, cfg.LearningsDir))

	// Get author info
	author, authorEmail := warmind.GetCurrentGitUser()

	// Read local learnings
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read learnings directory: %w", err)
	}

	added := 0
	scored := 0
	skipped := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		srcPath := filepath.Join(srcDir, entry.Name())
		content, err := os.ReadFile(srcPath)
		if err != nil {
			continue
		}

		// Parse metadata from frontmatter
		title, category, tags, confidence, createdAt := parseLearningFrontmatter(string(content))
		if title == "" {
			title = strings.TrimSuffix(entry.Name(), ".md")
		}

		// Create candidate
		candidate := warmind.Candidate{
			ID:          generateCandidateID(entry.Name()),
			Title:       title,
			Content:     string(content),
			Author:      author,
			AuthorEmail: authorEmail,
			SourcePath:  srcPath,
			Category:    category,
			Tags:        tags,
			Confidence:  confidence,
			CreatedAt:   createdAt,
			ContentHash: warmind.ContentHash(string(content)),
		}

		// Add to pool
		if err := pool.Add(candidate); err != nil {
			if strings.Contains(err.Error(), "duplicate") {
				skipped++
			}
			continue
		}
		added++

		// Score and stage
		scoring := scorer.Score(candidate)
		if err := pool.Score(candidate.ID, scoring); err != nil {
			if !warmindQuiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to score %s: %v\n", candidate.ID, err)
			}
			continue
		}
		scored++

		// Move local learning to .synced/ to prevent shadowing warmind version
		// This ensures inject will find the warmind version and record proper citations
		syncedDir := filepath.Join(srcDir, ".synced")
		if err := os.MkdirAll(syncedDir, 0755); err == nil {
			syncedPath := filepath.Join(syncedDir, entry.Name())
			if err := os.Rename(srcPath, syncedPath); err != nil && !warmindQuiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to move %s to .synced/: %v\n", entry.Name(), err)
			}
		}
	}

	if !warmindQuiet && !warmindAuto {
		fmt.Printf("Warmind sync: %d added to pool, %d scored, %d skipped (duplicates)\n", added, scored, skipped)

		// Show staged candidates
		staged, _ := pool.ListStaged()
		if len(staged) > 0 {
			fmt.Printf("\nStaged candidates awaiting citations:\n")
			for _, s := range staged {
				fmt.Printf("  [%s] %s (citations: %d, tier: %s)\n",
					s.Candidate.ID[:12], s.Candidate.Title[:minInt(40, len(s.Candidate.Title))],
					s.CitationCount, s.Scoring.Tier)
			}
			fmt.Printf("\nRun 'git add .warmind && git commit' to share pool with team\n")
		}
	}

	return nil
}

// runWarmindSyncSimple is the V1 simple sync (direct copy).
func runWarmindSyncSimple() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg, _ := config.Load(nil)
	srcDir := filepath.Join(cwd, ".agents", "learnings")
	dstDir := filepath.Join(cwd, cfg.Paths.WarmindLearningsDir)

	// Check if source exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		if !warmindQuiet && !warmindAuto {
			fmt.Println("No local learnings found (.agents/learnings/)")
		}
		return nil
	}

	// Create destination directory
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("create warmind directory: %w", err)
	}

	// Get author name
	author := resolveAuthor()

	// Collect existing warmind content hashes for dedup
	existingHashes := collectWarmindHashes(dstDir)

	// Sync learnings
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read learnings directory: %w", err)
	}

	synced := 0
	skipped := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		srcPath := filepath.Join(srcDir, entry.Name())
		content, err := os.ReadFile(srcPath)
		if err != nil {
			continue
		}

		// Check for duplicate by content hash
		hash := contentHashBytes(content)
		if existingHashes[hash] {
			skipped++
			continue
		}

		// Add author attribution if not present
		contentStr := string(content)
		if !strings.Contains(contentStr, "author:") {
			contentStr = addAuthorToFrontmatter(contentStr, author)
		}

		// Write to warmind
		dstPath := filepath.Join(dstDir, entry.Name())
		if err := os.WriteFile(dstPath, []byte(contentStr), 0644); err != nil {
			if !warmindQuiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to write %s: %v\n", entry.Name(), err)
			}
			continue
		}

		existingHashes[hash] = true
		synced++
	}

	if !warmindQuiet && !warmindAuto {
		fmt.Printf("Warmind sync: %d synced, %d skipped (duplicates)\n", synced, skipped)
		if synced > 0 {
			fmt.Printf("Run 'git add %s && git commit' to share with team\n", cfg.Paths.WarmindLearningsDir)
		}
	}

	return nil
}

func runWarmindStatus(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg := warmind.DefaultConfig()
	pool := warmind.NewPool(cwd, cfg)
	tracker := warmind.NewCitationTracker(cwd, cfg)
	mm := warmind.NewMaturityManager(cwd, cfg, tracker)
	cd := warmind.NewContradictionDetector(cwd, cfg, mm)

	// Pool stats
	pending, _ := pool.ListPending()
	staged, _ := pool.ListStaged()

	// Learning stats
	learnings, _ := mm.ScanLearnings()
	var provisional, established int
	for _, l := range learnings {
		switch l.Maturity {
		case warmind.MaturityProvisional:
			provisional++
		case warmind.MaturityEstablished:
			established++
		}
	}

	// Citation stats
	citationStats, _ := tracker.GetCitationStats()

	// Contradiction stats
	contradictions, _ := cd.GetPending()

	fmt.Println("Warmind Status")
	fmt.Println("==============")
	fmt.Println()
	fmt.Println("Pool:")
	fmt.Printf("  Pending:  %d\n", len(pending))
	fmt.Printf("  Staged:   %d (awaiting citations)\n", len(staged))
	fmt.Println()
	fmt.Println("Learnings:")
	fmt.Printf("  Total:        %d\n", len(learnings))
	fmt.Printf("  Provisional:  %d\n", provisional)
	fmt.Printf("  Established:  %d\n", established)
	fmt.Println()
	if citationStats != nil {
		fmt.Println("Citations:")
		fmt.Printf("  Total:  %d\n", citationStats.Total)
		fmt.Printf("  Self:   %d\n", citationStats.SelfCount)
		fmt.Printf("  Other:  %d\n", citationStats.OtherCount)
		fmt.Println()
	}
	fmt.Println("Contradictions:")
	fmt.Printf("  Pending review: %d\n", len(contradictions))

	return nil
}

func runWarmindPoolList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg := warmind.DefaultConfig()
	pool := warmind.NewPool(cwd, cfg)

	var status warmind.Status
	if warmindPoolStaged {
		status = warmind.StatusStaged
	} else if warmindPoolPending {
		status = warmind.StatusPending
	}

	entries, err := pool.List(warmind.ListOptions{Status: status})
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No pool entries found")
		return nil
	}

	fmt.Printf("%-14s %-8s %-6s %-5s %-40s\n", "ID", "STATUS", "TIER", "CITES", "TITLE")
	fmt.Println(strings.Repeat("-", 80))

	for _, e := range entries {
		title := e.Candidate.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		fmt.Printf("%-14s %-8s %-6s %-5d %-40s\n",
			e.Candidate.ID[:12]+"...",
			e.Status,
			e.Scoring.Tier,
			e.CitationCount,
			title)
	}

	return nil
}

func runWarmindPromote(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg := warmind.DefaultConfig()
	pool := warmind.NewPool(cwd, cfg)
	candidateID := args[0]

	if warmindPromoteForce {
		// Force promotion - bypass checks by updating entry
		if err := pool.SetCitationCount(candidateID, 100); err != nil {
			return fmt.Errorf("force: failed to set citation count: %w", err)
		}
	}

	artifactPath, err := pool.Promote(candidateID)
	if err != nil {
		return fmt.Errorf("promote failed: %w", err)
	}

	fmt.Printf("Promoted %s to %s\n", candidateID, artifactPath)
	return nil
}

func runWarmindContradict(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg := warmind.DefaultConfig()
	tracker := warmind.NewCitationTracker(cwd, cfg)
	mm := warmind.NewMaturityManager(cwd, cfg, tracker)
	cd := warmind.NewContradictionDetector(cwd, cfg, mm)

	if warmindContradictList {
		// List pending contradictions
		pending, err := cd.GetPending()
		if err != nil {
			return err
		}

		if len(pending) == 0 {
			fmt.Println("No pending contradictions")
			return nil
		}

		fmt.Printf("Pending Contradictions (%d):\n\n", len(pending))
		for _, c := range pending {
			fmt.Printf("ID: %s\n", c.ID)
			fmt.Printf("  Learning A: %s (%s)\n", filepath.Base(c.LearningA), c.LearningAAuthor)
			fmt.Printf("  Learning B: %s (%s)\n", filepath.Base(c.LearningB), c.LearningBAuthor)
			fmt.Printf("  Summary: %s\n", c.Summary)
			fmt.Println()
		}
		return nil
	}

	// Scan for new contradictions
	report, err := cd.Scan()
	if err != nil {
		return err
	}

	fmt.Printf("Contradiction scan: %d learnings scanned, %d new contradictions detected\n",
		report.Scanned, report.NewDetected)

	if report.NewDetected > 0 {
		fmt.Println("\nNew contradictions:")
		for _, c := range report.Contradictions {
			fmt.Printf("  [%s] %s vs %s\n", c.ID[:12], filepath.Base(c.LearningA), filepath.Base(c.LearningB))
			fmt.Printf("    %s\n", c.Summary)
		}
	}

	return nil
}

func runWarmindCloseLoop(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	cfg := warmind.DefaultConfig()
	pool := warmind.NewPool(cwd, cfg)
	tracker := warmind.NewCitationTracker(cwd, cfg)
	mm := warmind.NewMaturityManager(cwd, cfg, tracker)
	cd := warmind.NewContradictionDetector(cwd, cfg, mm)

	if !warmindQuiet {
		fmt.Println("Running warmind close-loop...")
	}

	// 1. Check for auto-promotions
	promoted := 0
	staged, _ := pool.ListStaged()
	for _, entry := range staged {
		eligible, _ := pool.CheckPromotion(entry.Candidate.ID)
		if eligible {
			if _, err := pool.Promote(entry.Candidate.ID); err == nil {
				promoted++
				if !warmindQuiet {
					fmt.Printf("  Promoted: %s\n", entry.Candidate.ID)
				}
			}
		}
	}

	// 2. Run maturity maintenance
	maturityReport, err := mm.RunMaintenance()
	if err != nil && !warmindQuiet {
		fmt.Fprintf(os.Stderr, "Warning: maturity maintenance failed: %v\n", err)
	}

	// 3. Scan for contradictions
	contradictReport, _ := cd.Scan()

	// 4. Prune old citations
	pruned, _ := tracker.Prune(90 * 24 * time.Hour)

	if !warmindQuiet {
		fmt.Println()
		fmt.Println("Close-loop results:")
		fmt.Printf("  Promoted:       %d\n", promoted)
		if maturityReport != nil {
			fmt.Printf("  Maturity:       %d transitions (%d promoted, %d archived)\n",
				len(maturityReport.Transitions), maturityReport.Promoted, maturityReport.Archived)
		}
		if contradictReport != nil {
			fmt.Printf("  Contradictions: %d new\n", contradictReport.NewDetected)
		}
		fmt.Printf("  Citations:      %d pruned\n", pruned)
	}

	return nil
}

// --- Helper functions ---

func resolveAuthor() string {
	if name := getGitConfigValue("user.name"); name != "" {
		return name
	}
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return "unknown"
}

func getGitConfigValue(key string) string {
	out, err := exec.Command("git", "config", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func collectWarmindHashes(dir string) map[string]bool {
	hashes := make(map[string]bool)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return hashes
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		hashes[contentHashBytes(content)] = true
	}
	return hashes
}

func contentHashBytes(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

func addAuthorToFrontmatter(content, author string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return fmt.Sprintf("---\nauthor: %s\nsynced_at: %s\n---\n%s",
			author, time.Now().Format(time.RFC3339), content)
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			newLines := make([]string, 0, len(lines)+2)
			newLines = append(newLines, lines[:i]...)
			newLines = append(newLines, fmt.Sprintf("author: %s", author))
			newLines = append(newLines, fmt.Sprintf("synced_at: %s", time.Now().Format(time.RFC3339)))
			newLines = append(newLines, lines[i:]...)
			return strings.Join(newLines, "\n")
		}
	}

	return content
}

func generateCandidateID(filename string) string {
	base := strings.TrimSuffix(filename, ".md")
	hash := warmind.ContentHash(base + time.Now().String())
	return "learn-" + base + "-" + hash[:8]
}

func parseLearningFrontmatter(content string) (title, category string, tags []string, confidence float64, createdAt time.Time) {
	confidence = 0.7 // Default
	createdAt = time.Now()

	lines := strings.Split(content, "\n")
	if len(lines) < 2 || strings.TrimSpace(lines[0]) != "---" {
		return
	}

	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			break
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"'")

		switch key {
		case "title":
			title = value
		case "category":
			category = value
		case "tags":
			// Parse [tag1, tag2] format
			value = strings.Trim(value, "[]")
			for _, t := range strings.Split(value, ",") {
				if t = strings.TrimSpace(t); t != "" {
					tags = append(tags, t)
				}
			}
		case "confidence":
			if f, err := fmt.Sscanf(value, "%f", &confidence); err == nil && f > 0 {
				// parsed
			}
		case "created_at":
			if t, err := time.Parse(time.RFC3339, value); err == nil {
				createdAt = t
			}
		}
	}

	// If no title, extract from first heading
	if title == "" {
		for _, line := range lines {
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimPrefix(line, "# ")
				break
			}
		}
	}

	return
}
