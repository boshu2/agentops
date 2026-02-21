package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ratchet"
	"github.com/boshu2/agentops/cli/internal/types"
)

const (
	// DefaultInjectMaxTokens is the default token budget for injection (~1500 tokens ≈ 6KB)
	DefaultInjectMaxTokens = 1500

	// InjectCharsPerToken is the approximate characters per token (conservative estimate)
	InjectCharsPerToken = 4

	// MaxLearningsToInject is the maximum number of learnings to include
	MaxLearningsToInject = 10

	// MaxPatternsToInject is the maximum number of patterns to include
	MaxPatternsToInject = 5

	// MaxSessionsToInject is the maximum number of recent sessions to summarize
	MaxSessionsToInject = 5

	// quarantineRelPath is the path to OL quarantine constraints relative to the .ol/ directory.
	quarantineRelPath = "constraints/quarantine.json"
)

var (
	injectMaxTokens  int
	injectContext    string
	injectFormat     string
	injectSessionID  string
	injectNoCite     bool
	injectApplyDecay bool
)

type olConstraint struct {
	Pattern    string  `json:"pattern"`
	Detection  string  `json:"detection"`
	Source     string  `json:"source,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Status     string  `json:"status,omitempty"`
}

type injectedKnowledge struct {
	Learnings     []learning     `json:"learnings,omitempty"`
	Patterns      []pattern      `json:"patterns,omitempty"`
	Sessions      []session      `json:"sessions,omitempty"`
	OLConstraints []olConstraint `json:"ol_constraints,omitempty"`
	Timestamp     time.Time      `json:"timestamp"`
	Query         string         `json:"query,omitempty"`
}

type learning struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Summary        string  `json:"summary"`
	Source         string  `json:"source,omitempty"`
	FreshnessScore float64 `json:"freshness_score,omitempty"`
	AgeWeeks       float64 `json:"age_weeks,omitempty"`
	Utility        float64 `json:"utility,omitempty"`         // MemRL utility value
	CompositeScore float64 `json:"composite_score,omitempty"` // Two-Phase ranking score
	Superseded     bool    `json:"-"`                         // Internal flag - not serialized
}

type pattern struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	FilePath       string  `json:"file_path,omitempty"`
	FreshnessScore float64 `json:"freshness_score,omitempty"`
	AgeWeeks       float64 `json:"age_weeks,omitempty"`
	Utility        float64 `json:"utility,omitempty"`
	CompositeScore float64 `json:"composite_score,omitempty"`
}

type session struct {
	Date    string `json:"date"`
	Summary string `json:"summary"`
}

var injectCmd = &cobra.Command{
	Use:   "inject [context]",
	Short: "Output relevant knowledge for session injection",
	Long: `Inject searches and outputs relevant knowledge for session startup.

This command is designed to be called from a SessionStart hook to
inject prior learnings, patterns, and context into new sessions.

Searches:
  1. Recent learnings (.agents/learnings/*.md)
  2. Active patterns (.agents/patterns/*.md)
  3. Recent session summaries (.agents/ao/sessions/)

Uses file-based search with Two-Phase retrieval (freshness + utility scoring).
CASS integration adds maturity weighting and confidence decay.

Examples:
  ao inject                     # Inject general knowledge
  ao inject "authentication"    # Inject knowledge about auth
  ao inject --max-tokens 2000   # Larger budget
  ao inject --format json       # JSON output
  ao inject --no-cite           # Skip citation recording
  ao inject --apply-decay       # Apply confidence decay before ranking`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInject,
}

func init() {
	rootCmd.AddCommand(injectCmd)
	injectCmd.Flags().IntVar(&injectMaxTokens, "max-tokens", DefaultInjectMaxTokens, "Maximum tokens to output")
	injectCmd.Flags().StringVar(&injectContext, "context", "", "Context query for filtering (alternative to positional arg)")
	injectCmd.Flags().StringVar(&injectFormat, "format", "markdown", "Output format: markdown, json")
	injectCmd.Flags().StringVar(&injectSessionID, "session", "", "Session ID for citation tracking (auto-generated if empty)")
	injectCmd.Flags().BoolVar(&injectNoCite, "no-cite", false, "Disable citation recording")
	injectCmd.Flags().BoolVar(&injectApplyDecay, "apply-decay", false, "Apply confidence decay before ranking")
}

func runInject(cmd *cobra.Command, args []string) error {
	// Get context query
	query := injectContext
	if len(args) > 0 {
		query = args[0]
	}

	if GetDryRun() {
		fmt.Printf("[dry-run] Would inject knowledge")
		if query != "" {
			fmt.Printf(" filtered by: %s", query)
		}
		fmt.Printf(" (max %d tokens)\n", injectMaxTokens)
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Calculate character budget from tokens
	charBudget := injectMaxTokens * InjectCharsPerToken

	// Get or generate session ID for citation tracking
	sessionID := canonicalSessionID(injectSessionID)

	// Collect knowledge
	knowledge := &injectedKnowledge{
		Timestamp: time.Now(),
		Query:     query,
	}

	// Search learnings
	learnings, err := collectLearnings(cwd, query, MaxLearningsToInject)
	if err != nil {
		VerbosePrintf("Warning: failed to collect learnings: %v\n", err)
	}
	knowledge.Learnings = learnings

	// Record citations for retrieved learnings (Phase 0: Critical for MemRL feedback loop)
	if !injectNoCite && len(learnings) > 0 {
		if err := recordCitations(cwd, learnings, sessionID, query); err != nil {
			VerbosePrintf("Warning: failed to record citations: %v\n", err)
		} else {
			VerbosePrintf("Recorded %d citations for session %s\n", len(learnings), sessionID)
		}
	}

	// Search patterns
	patterns, err := collectPatterns(cwd, query, MaxPatternsToInject)
	if err != nil {
		VerbosePrintf("Warning: failed to collect patterns: %v\n", err)
	}
	knowledge.Patterns = patterns

	// Record citations for retrieved patterns (closes σ gap: patterns were retrieved but never cited)
	if !injectNoCite && len(patterns) > 0 {
		if err := recordPatternCitations(cwd, patterns, sessionID, query); err != nil {
			VerbosePrintf("Warning: failed to record pattern citations: %v\n", err)
		} else {
			VerbosePrintf("Recorded %d pattern citations for session %s\n", len(patterns), sessionID)
		}
	}

	// Search recent sessions
	sessions, err := collectRecentSessions(cwd, query, MaxSessionsToInject)
	if err != nil {
		VerbosePrintf("Warning: failed to collect sessions: %v\n", err)
	}
	knowledge.Sessions = sessions

	// Discover OL constraints (no-op if .ol/ doesn't exist)
	olConstraints, err := collectOLConstraints(cwd, query)
	if err != nil {
		VerbosePrintf("Warning: failed to collect OL constraints: %v\n", err)
	}
	knowledge.OLConstraints = olConstraints

	// Format output
	var output string
	if injectFormat == "json" {
		data, err := json.MarshalIndent(knowledge, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		output = string(data)
	} else {
		output = formatKnowledgeMarkdown(knowledge)
	}

	// Trim to budget if needed
	if len(output) > charBudget {
		output = trimToCharBudget(output, charBudget)
	}

	fmt.Println(output)
	return nil
}

// findAgentsSubdir looks for .agents/{subdir}/ walking up to rig root
func findAgentsSubdir(startDir, subdir string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, ".agents", subdir)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		// Check if we're at rig root (has .beads, crew, or polecats)
		markers := []string{".beads", "crew", "polecats"}
		atRigRoot := false
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				atRigRoot = true
				break
			}
		}
		if atRigRoot {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// formatKnowledgeMarkdown formats knowledge as markdown
func formatKnowledgeMarkdown(k *injectedKnowledge) string {
	var sb strings.Builder

	sb.WriteString("## Injected Knowledge (ol inject)\n\n")

	if len(k.Learnings) > 0 {
		sb.WriteString("### Recent Learnings\n")
		for _, l := range k.Learnings {
			if l.Summary != "" {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", l.ID, l.Summary))
			} else {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", l.ID, l.Title))
			}
		}
		sb.WriteString("\n")
	}

	if len(k.Patterns) > 0 {
		sb.WriteString("### Active Patterns\n")
		for _, p := range k.Patterns {
			if p.Description != "" {
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", p.Name, p.Description))
			} else {
				sb.WriteString(fmt.Sprintf("- **%s**\n", p.Name))
			}
		}
		sb.WriteString("\n")
	}

	if len(k.Sessions) > 0 {
		sb.WriteString("### Recent Sessions\n")
		for _, s := range k.Sessions {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", s.Date, s.Summary))
		}
		sb.WriteString("\n")
	}

	if len(k.OLConstraints) > 0 {
		sb.WriteString("### Olympus Constraints\n")
		for _, c := range k.OLConstraints {
			sb.WriteString(fmt.Sprintf("- **[olympus constraint]** %s: %s\n", c.Pattern, c.Detection))
		}
		sb.WriteString("\n")
	}

	if len(k.Learnings) == 0 && len(k.Patterns) == 0 && len(k.Sessions) == 0 && len(k.OLConstraints) == 0 {
		sb.WriteString("*No prior knowledge found.*\n\n")
	}

	sb.WriteString(fmt.Sprintf("*Last injection: %s*\n", k.Timestamp.Format(time.RFC3339)))

	return sb.String()
}

// trimToCharBudget truncates output to fit character budget
func trimToCharBudget(output string, budget int) string {
	if len(output) <= budget {
		return output
	}

	// Try to truncate at a section boundary
	lines := strings.Split(output, "\n")
	var result strings.Builder
	for _, line := range lines {
		if result.Len()+len(line)+1 > budget-50 { // Leave room for truncation marker
			break
		}
		result.WriteString(line)
		result.WriteString("\n")
	}

	result.WriteString("\n*[truncated to fit token budget]*\n")
	return result.String()
}

// truncateText truncates a string to max length with ellipsis
func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// collectOLConstraints reads constraints from .ol/constraints/quarantine.json.
// Returns nil (no-op) if .ol/ directory doesn't exist.
func collectOLConstraints(cwd, query string) ([]olConstraint, error) {
	olDir := filepath.Join(cwd, ".ol")
	if _, err := os.Stat(olDir); os.IsNotExist(err) {
		return nil, nil // Not an Olympus project
	}

	quarantinePath := filepath.Join(olDir, quarantineRelPath)
	data, err := os.ReadFile(quarantinePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No quarantine file
		}
		return nil, fmt.Errorf("read quarantine.json: %w", err)
	}

	var raw []olConstraint
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse quarantine.json: %w", err)
	}

	// Filter by query if provided
	if query == "" {
		return raw, nil
	}

	queryLower := strings.ToLower(query)
	var filtered []olConstraint
	for _, c := range raw {
		content := strings.ToLower(c.Pattern + " " + c.Detection)
		if strings.Contains(content, queryLower) {
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}

// recordCitations records citation events for retrieved learnings.
// This is critical for closing the MemRL feedback loop (Phase 0).
// Citations link: session → learning → feedback → utility update.
func recordCitations(baseDir string, learnings []learning, sessionID, query string) error {
	canonicalSession := canonicalSessionID(sessionID)
	for _, l := range learnings {
		event := types.CitationEvent{
			ArtifactPath: canonicalArtifactPath(baseDir, l.Source),
			SessionID:    canonicalSession,
			CitedAt:      time.Now(),
			CitationType: "retrieved", // Will be upgraded to "applied" if session succeeds
			Query:        query,
		}

		if err := ratchet.RecordCitation(baseDir, event); err != nil {
			return fmt.Errorf("record citation for %s: %w", l.ID, err)
		}
	}
	return nil
}

// recordPatternCitations records citation events for retrieved patterns.
func recordPatternCitations(baseDir string, patterns []pattern, sessionID, query string) error {
	canonicalSession := canonicalSessionID(sessionID)
	for _, p := range patterns {
		if p.FilePath == "" {
			continue
		}
		event := types.CitationEvent{
			ArtifactPath: canonicalArtifactPath(baseDir, p.FilePath),
			SessionID:    canonicalSession,
			CitedAt:      time.Now(),
			CitationType: "retrieved",
			Query:        query,
		}
		if err := ratchet.RecordCitation(baseDir, event); err != nil {
			return fmt.Errorf("record citation for pattern %s: %w", p.Name, err)
		}
	}
	return nil
}
