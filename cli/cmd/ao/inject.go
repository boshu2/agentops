package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/config"
	"github.com/boshu2/agentops/cli/internal/search"
)

const (
	// DefaultInjectMaxTokens is the default token budget for injection (~1500 tokens ≈ 6KB)
	DefaultInjectMaxTokens = 1500

	// InjectCharsPerToken is the approximate characters per token (conservative estimate)
	InjectCharsPerToken = search.InjectCharsPerToken

	// MaxLearningsToInject is the maximum number of learnings to include
	MaxLearningsToInject = 10

	// MaxPatternsToInject is the maximum number of patterns to include
	MaxPatternsToInject = 5

	// MaxSessionsToInject is the maximum number of recent sessions to summarize
	MaxSessionsToInject = 5

	// Knowledge section directory names under .agents/.
	SectionLearnings = "learnings"
	SectionFindings  = "findings"
	SectionPatterns  = "patterns"
	SectionResearch  = "research"
	SectionSessions  = "sessions"
)

var (
	injectMaxTokens         int
	injectContext           string
	injectFormat            string
	injectSessionID         string
	injectNoCite            bool
	injectApplyDecay        bool
	injectBead              string
	injectPredecessor       string
	injectIndexOnly         bool
	injectQuarantineFlagged bool
	injectForSkill          string
	injectSessionType       string
	injectProfile           bool
)

// Type aliases — canonical definitions live in internal/search/types.go.
type injectedKnowledge = search.InjectedKnowledge
type learning = search.Learning
type pattern = search.Pattern
type knowledgeFinding = search.KnowledgeFinding
type session = search.Session

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
  ao inject --apply-decay       # Apply confidence decay before ranking
  ao inject --bead ag-7abc      # Work-scoped injection for bead
  ao inject --predecessor /path/to/handoff.md  # Include predecessor context
  ao inject --for=research "authentication"    # Filtered by skill's context contract

Environment variables:
  RPI_RUN_ID    When set, --for uses this as the context artifact directory name
                instead of generating an adhoc-<timestamp> ID. Set by /rpi orchestrator.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInject,
}

func init() {
	injectCmd.GroupID = "knowledge"
	rootCmd.AddCommand(injectCmd)
	injectCmd.Flags().IntVar(&injectMaxTokens, "max-tokens", DefaultInjectMaxTokens, "Maximum tokens to output")
	injectCmd.Flags().StringVar(&injectContext, "context", "", "Context query for filtering (alternative to positional arg)")
	injectCmd.Flags().StringVar(&injectFormat, "format", "markdown", "Output format: markdown, json")
	injectCmd.Flags().StringVar(&injectSessionID, "session", "", "Session ID for citation tracking (auto-generated if empty)")
	injectCmd.Flags().BoolVar(&injectNoCite, "no-cite", false, "Disable citation recording")
	injectCmd.Flags().BoolVar(&injectApplyDecay, "apply-decay", false, "Apply confidence decay before ranking")
	injectCmd.Flags().StringVar(&injectBead, "bead", "", "Bead ID for work-scoped knowledge injection")
	injectCmd.Flags().StringVar(&injectPredecessor, "predecessor", "", "Path to predecessor handoff file for context injection")
	injectCmd.Flags().BoolVar(&injectIndexOnly, "index-only", false, "Output compact knowledge index table instead of full content")
	injectCmd.Flags().BoolVar(&injectQuarantineFlagged, "quarantine-flagged", false, "Quarantine flagged learnings from quality report")
	injectCmd.Flags().StringVar(&injectForSkill, "for", "", "Skill name — assembles context per skill's context declaration")
	injectCmd.Flags().StringVar(&injectSessionType, "session-type", "", "Session type for scoring boost (career, research, debug, implement, brainstorm)")
	injectCmd.Flags().BoolVar(&injectProfile, "profile", false, "Include .agents/profile.md identity artifact in output")
}

// injectOptionsFromFlags builds an InjectOptions from the cobra flag vars + positional args.
func injectOptionsFromFlags(args []string) *search.InjectOptions {
	opts := &search.InjectOptions{
		MaxTokens:         injectMaxTokens,
		Context:           injectContext,
		Format:            injectFormat,
		SessionID:         injectSessionID,
		NoCite:            injectNoCite,
		ApplyDecay:        injectApplyDecay,
		Bead:              injectBead,
		Predecessor:       injectPredecessor,
		IndexOnly:         injectIndexOnly,
		QuarantineFlagged: injectQuarantineFlagged,
		ForSkill:          injectForSkill,
		SessionType:       injectSessionType,
		Profile:           injectProfile,
	}
	if len(args) > 0 {
		opts.Query = args[0]
	} else {
		opts.Query = injectContext
	}
	return opts
}

func runInject(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(os.Stderr, "NOTICE: ao inject is deprecated (removal target: v3.0.0). Use 'ao lookup' for learnings or see .agents/AGENTS.md for navigation.")
	opts := injectOptionsFromFlags(args)

	if GetDryRun() {
		printInjectDryRun(opts)
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	maybeQuarantineFlagged(cwd, opts)

	cfg := loadInjectConfig()
	beadCtx := resolveInjectBeadContext(cwd, opts)

	sessionID := resolveSessionID(opts.SessionID)
	knowledge := gatherKnowledge(cwd, opts, sessionID, cfg)
	knowledge.BeadID = opts.Bead

	// Dedup: skip learnings whose title already appears in MEMORY.md
	knowledge.Learnings = search.FilterMemoryDuplicates(cwd, knowledge.Learnings)

	if err := applyInjectModifiers(cwd, opts, knowledge, beadCtx); err != nil {
		return err
	}

	output, err := renderInjectOutput(cwd, opts, knowledge)
	if err != nil {
		return err
	}

	fmt.Println(output)
	return nil
}

func maybeQuarantineFlagged(cwd string, opts *search.InjectOptions) {
	if !opts.QuarantineFlagged {
		return
	}
	if qErr := runQuarantineFlagged(cwd); qErr != nil {
		VerbosePrintf("Warning: quarantine-flagged: %v\n", qErr)
	}
}

func loadInjectConfig() *config.Config {
	cfg, cfgErr := config.Load(nil)
	if cfgErr != nil {
		VerbosePrintf("Warning: config load: %v (using defaults)\n", cfgErr)
	}
	return cfg
}

func resolveInjectBeadContext(cwd string, opts *search.InjectOptions) *BeadContext {
	if opts.Bead == "" {
		return nil
	}
	beadCtx := resolveBeadContext(opts.Bead, cwd)
	VerbosePrintf("Bead context: id=%s title=%q labels=%v\n", opts.Bead, beadCtx.Title, beadCtx.Labels)
	return beadCtx
}

func applyInjectModifiers(cwd string, opts *search.InjectOptions, knowledge *injectedKnowledge, beadCtx *BeadContext) error {
	if beadCtx != nil {
		for i := range knowledge.Learnings {
			applyBeadBoost(&knowledge.Learnings[i], beadCtx)
		}
		search.ResortLearnings(knowledge.Learnings)
	}

	if opts.SessionType != "" {
		for i := range knowledge.Learnings {
			knowledge.Learnings[i].CompositeScore *= sessionTypeBoost(knowledge.Learnings[i], opts.SessionType)
		}
		search.ResortLearnings(knowledge.Learnings)
	}

	if opts.Predecessor != "" {
		knowledge.Predecessor = parsePredecessorFile(opts.Predecessor)
	}

	if opts.ForSkill == "" {
		return nil
	}

	decl, err := parseContextDeclaration(opts.ForSkill)
	if err != nil {
		return fmt.Errorf("parse context declaration for %s: %w", opts.ForSkill, err)
	}
	if decl != nil {
		filtered := applyContextFilter(knowledge, decl)
		*knowledge = *filtered
	}

	runID := os.Getenv("RPI_RUN_ID")
	ctxDir, ctxErr := ensureContextDir(cwd, runID, nil)
	if ctxErr != nil {
		fmt.Fprintf(os.Stderr, "WARN: context dir: %v\n", ctxErr)
		return nil
	}

	VerbosePrintf("context-dir: %s\n", ctxDir)
	return nil
}

func renderInjectOutput(cwd string, opts *search.InjectOptions, knowledge *injectedKnowledge) (string, error) {
	output := renderKnowledgeIndex(knowledge)
	if !opts.IndexOnly {
		rendered, err := renderKnowledge(knowledge, opts.Format)
		if err != nil {
			return "", err
		}
		output = rendered
	}

	charBudget := opts.MaxTokens * InjectCharsPerToken
	if len(output) > charBudget {
		if opts.Format == "json" {
			output = search.TrimJSONToCharBudget(knowledge, charBudget)
		} else {
			output = search.TrimToCharBudget(output, charBudget)
		}
	}

	if !opts.Profile {
		return output, nil
	}

	profilePath := filepath.Join(cwd, ".agents", "profile.md")
	if data, readErr := os.ReadFile(profilePath); readErr == nil {
		output = "## Identity\n\n" + string(data) + "\n\n" + output
	}
	return output, nil
}

// runQuarantineFlagged reads .agents/defrag/quality-report.json and quarantines flagged learnings.
func runQuarantineFlagged(cwd string) error {
	paths, err := search.ReadFlaggedQualityPaths(cwd)
	if err != nil {
		return err
	}
	quarantined := 0
	for _, absPath := range paths {
		if err := quarantineLearning(absPath, "flagged by quality report"); err != nil {
			VerbosePrintf("Warning: quarantine %s: %v\n", absPath, err)
			continue
		}
		quarantined++
	}
	if quarantined > 0 {
		fmt.Printf("Quarantined %d flagged learnings\n", quarantined)
	}
	return nil
}

// printInjectDryRun prints the dry-run message for inject.
func printInjectDryRun(opts *search.InjectOptions) {
	fmt.Printf("[dry-run] Would inject knowledge")
	if opts.Query != "" {
		fmt.Printf(" filtered by: %s", opts.Query)
	}
	fmt.Printf(" (max %d tokens)\n", opts.MaxTokens)
}

// gatherKnowledge collects all knowledge sources and records citations.
func gatherKnowledge(cwd string, opts *search.InjectOptions, sessionID string, cfg *config.Config) *injectedKnowledge {
	knowledge := &injectedKnowledge{
		Timestamp: time.Now(),
		Query:     opts.Query,
	}

	globalLearningsDir := ""
	globalPatternsDir := ""
	globalWeight := 0.8
	if cfg != nil {
		globalLearningsDir = cfg.Paths.GlobalLearningsDir
		globalPatternsDir = cfg.Paths.GlobalPatternsDir
		globalWeight = cfg.Paths.GlobalWeight
	}

	knowledge.Learnings = gatherLearnings(cwd, opts, sessionID, globalLearningsDir, globalWeight)
	knowledge.Patterns = gatherPatterns(cwd, opts, sessionID, globalPatternsDir, globalWeight)

	// Non-verbose quality gate summary (stderr — does not pollute stdout inject output)
	if len(knowledge.Learnings) > 0 || len(knowledge.Patterns) > 0 {
		fmt.Fprintf(os.Stderr, "Injected %d learnings, %d patterns\n",
			len(knowledge.Learnings), len(knowledge.Patterns))
	}
	knowledge.Sessions = gatherSessions(cwd, opts.Query)

	return knowledge
}

// gatherLearnings collects learnings and records their citations.
func gatherLearnings(cwd string, opts *search.InjectOptions, sessionID, globalDir string, globalWeight float64) []learning {
	learnings, err := collectLearnings(cwd, opts.Query, MaxLearningsToInject, globalDir, globalWeight)
	if err != nil {
		VerbosePrintf("Warning: failed to collect learnings: %v\n", err)
	}

	if !opts.NoCite && len(learnings) > 0 {
		if err := recordCitations(cwd, learnings, sessionID, opts.Query); err != nil {
			VerbosePrintf("Warning: failed to record citations: %v\n", err)
		} else {
			VerbosePrintf("Recorded %d citations for session %s\n", len(learnings), sessionID)
		}
	}

	return learnings
}

// gatherPatterns collects patterns and records their citations.
func gatherPatterns(cwd string, opts *search.InjectOptions, sessionID, globalDir string, globalWeight float64) []pattern {
	patterns, err := collectPatterns(cwd, opts.Query, MaxPatternsToInject, globalDir, globalWeight)
	if err != nil {
		VerbosePrintf("Warning: failed to collect patterns: %v\n", err)
	}

	if !opts.NoCite && len(patterns) > 0 {
		if err := recordPatternCitations(cwd, patterns, sessionID, opts.Query); err != nil {
			VerbosePrintf("Warning: failed to record pattern citations: %v\n", err)
		} else {
			VerbosePrintf("Recorded %d pattern citations for session %s\n", len(patterns), sessionID)
		}
	}

	return patterns
}

// gatherSessions collects recent session summaries.
func gatherSessions(cwd, query string) []session {
	sessions, err := collectRecentSessions(cwd, query, MaxSessionsToInject)
	if err != nil {
		VerbosePrintf("Warning: failed to collect sessions: %v\n", err)
	}
	return sessions
}

// Thin wrappers — canonical definitions in internal/search/inject_run.go.
// These exist for test call sites and for readable use within this file.
func renderKnowledge(k *injectedKnowledge, format string) (string, error) {
	return search.RenderKnowledge(k, format, compactText)
}
func findAgentsSubdir(startDir, subdir string) string {
	return search.FindAgentsSubdir(startDir, subdir)
}
func resortLearnings(ls []learning)                  { search.ResortLearnings(ls) }
func trimToCharBudget(out string, budget int) string { return search.TrimToCharBudget(out, budget) }
func trimJSONToCharBudget(k *injectedKnowledge, b int) string {
	return search.TrimJSONToCharBudget(k, b)
}
func filterMemoryDuplicates(cwd string, ls []learning) []learning {
	return search.FilterMemoryDuplicates(cwd, ls)
}
func formatKnowledgeMarkdown(k *injectedKnowledge) string {
	return search.FormatKnowledgeMarkdown(k, compactText)
}

// writePredecessorSection — thin wrapper for tests.
func writePredecessorSection(sb *strings.Builder, pred *predecessorContext) {
	search.WritePredecessorSection(sb, pred)
}

// Thin wrappers — canonical definitions in internal/search/util.go.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	return search.AtomicWriteFile(path, data, perm)
}
func truncateText(s string, maxLen int) string { return search.TruncateText(s, maxLen) }

// recordCitations records citation events for retrieved learnings.
// This is critical for closing the MemRL feedback loop (Phase 0).
// Citations link: session → learning → feedback → utility update.
func recordCitations(baseDir string, learnings []learning, sessionID, query string) error {
	return recordCitationsInNamespace(baseDir, learnings, sessionID, query, defaultCitationMetricNamespace())
}

func recordCitationsInNamespace(baseDir string, learnings []learning, sessionID, query, namespace string) error {
	return search.RecordLearningCitations(
		baseDir, learnings,
		canonicalSessionID(sessionID), query, canonicalMetricNamespace(namespace),
		canonicalArtifactPath,
	)
}

// recordPatternCitations records citation events for retrieved patterns.
func recordPatternCitations(baseDir string, patterns []pattern, sessionID, query string) error {
	return recordPatternCitationsInNamespace(baseDir, patterns, sessionID, query, defaultCitationMetricNamespace())
}

func recordPatternCitationsInNamespace(baseDir string, patterns []pattern, sessionID, query, namespace string) error {
	return search.RecordPatternCitations(
		baseDir, patterns,
		canonicalSessionID(sessionID), query, canonicalMetricNamespace(namespace),
		canonicalArtifactPath,
	)
}
