package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/config"
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

type olConstraint struct {
	Pattern    string  `json:"pattern"`
	Detection  string  `json:"detection"`
	Source     string  `json:"source,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
	Status     string  `json:"status,omitempty"`
}

type injectedKnowledge struct {
	Predecessor   *predecessorContext `json:"predecessor,omitempty"`
	Learnings     []learning          `json:"learnings,omitempty"`
	Patterns      []pattern           `json:"patterns,omitempty"`
	Sessions      []session           `json:"sessions,omitempty"`
	OLConstraints []olConstraint      `json:"ol_constraints,omitempty"`
	Timestamp     time.Time           `json:"timestamp"`
	Query         string              `json:"query,omitempty"`
	BeadID        string              `json:"bead_id,omitempty"`
}

type learning struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Summary        string  `json:"summary"`
	Source         string  `json:"source,omitempty"`
	SourceBead     string  `json:"source_bead,omitempty"`  // Bead ID that produced this learning
	SourcePhase    string  `json:"source_phase,omitempty"` // RPI phase (research|plan|implement|validate)
	FreshnessScore float64 `json:"freshness_score,omitempty"`
	AgeWeeks       float64 `json:"age_weeks,omitempty"`
	Utility        float64 `json:"utility,omitempty"`         // MemRL utility value
	CompositeScore float64 `json:"composite_score,omitempty"` // Two-Phase ranking score
	Maturity       string  `json:"maturity,omitempty"`        // CASS maturity level
	SessionType    string  `json:"session_type,omitempty"`    // career, research, debug, implement, brainstorm
	BodyText       string  `json:"-"`                         // Full body text for search (populated on demand)
	Stability      string  `json:"-"`                         // "experimental" | "stable", default "stable"
	Superseded     bool    `json:"-"`                         // Internal flag - not serialized
	Global         bool    `json:"-"`                         // Internal flag: from global dir
}

type pattern struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	FilePath       string  `json:"file_path,omitempty"`
	FreshnessScore float64 `json:"freshness_score,omitempty"`
	AgeWeeks       float64 `json:"age_weeks,omitempty"`
	Utility        float64 `json:"utility,omitempty"`
	CompositeScore float64 `json:"composite_score,omitempty"`
	Global         bool    `json:"-"` // Internal flag: from global dir
}

type knowledgeFinding struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	Summary             string   `json:"summary"`
	Source              string   `json:"source,omitempty"`
	SourceSkill         string   `json:"source_skill,omitempty"`
	Severity            string   `json:"severity,omitempty"`
	Detectability       string   `json:"detectability,omitempty"`
	Status              string   `json:"status,omitempty"`
	CompilerTargets     []string `json:"compiler_targets,omitempty"`
	ScopeTags           []string `json:"scope_tags,omitempty"`
	ApplicableWhen      []string `json:"applicable_when,omitempty"`
	ApplicableLanguages []string `json:"applicable_languages,omitempty"`
	HitCount            int      `json:"hit_count,omitempty"`
	LastCited           string   `json:"last_cited,omitempty"`
	RetiredBy           string   `json:"retired_by,omitempty"`
	FreshnessScore      float64  `json:"freshness_score,omitempty"`
	AgeWeeks            float64  `json:"age_weeks,omitempty"`
	Utility             float64  `json:"utility,omitempty"`
	CompositeScore      float64  `json:"composite_score,omitempty"`
	Global              bool     `json:"-"`
}

type session struct {
	Date    string `json:"date"`
	Summary string `json:"summary"`
	Path    string `json:"path,omitempty"`
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

func runInject(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(os.Stderr, "NOTICE: ao inject is deprecated (removal target: v3.0.0). Use 'ao lookup' for learnings or see .agents/AGENTS.md for navigation.")
	query := resolveInjectQuery(args)

	if GetDryRun() {
		printInjectDryRun(query)
		return nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	maybeQuarantineFlagged(cwd)

	cfg := loadInjectConfig()
	beadCtx := resolveInjectBeadContext(cwd)

	sessionID := resolveSessionID(injectSessionID)
	knowledge := gatherKnowledge(cwd, query, sessionID, cfg)
	knowledge.BeadID = injectBead

	// Dedup: skip learnings whose title already appears in MEMORY.md
	knowledge.Learnings = filterMemoryDuplicates(cwd, knowledge.Learnings)

	if err := applyInjectModifiers(cwd, knowledge, beadCtx); err != nil {
		return err
	}

	output, err := renderInjectOutput(cwd, knowledge)
	if err != nil {
		return err
	}

	fmt.Println(output)
	return nil
}

func maybeQuarantineFlagged(cwd string) {
	if !injectQuarantineFlagged {
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

func resolveInjectBeadContext(cwd string) *BeadContext {
	if injectBead == "" {
		return nil
	}
	beadCtx := resolveBeadContext(injectBead, cwd)
	VerbosePrintf("Bead context: id=%s title=%q labels=%v\n", injectBead, beadCtx.Title, beadCtx.Labels)
	return beadCtx
}

func applyInjectModifiers(cwd string, knowledge *injectedKnowledge, beadCtx *BeadContext) error {
	if beadCtx != nil {
		for i := range knowledge.Learnings {
			applyBeadBoost(&knowledge.Learnings[i], beadCtx)
		}
		resortLearnings(knowledge.Learnings)
	}

	if injectSessionType != "" {
		for i := range knowledge.Learnings {
			knowledge.Learnings[i].CompositeScore *= sessionTypeBoost(knowledge.Learnings[i], injectSessionType)
		}
		resortLearnings(knowledge.Learnings)
	}

	if injectPredecessor != "" {
		knowledge.Predecessor = parsePredecessorFile(injectPredecessor)
	}

	if injectForSkill == "" {
		return nil
	}

	decl, err := parseContextDeclaration(injectForSkill)
	if err != nil {
		return fmt.Errorf("parse context declaration for %s: %w", injectForSkill, err)
	}
	if decl != nil {
		filtered := applyContextFilter(knowledge, decl)
		*knowledge = *filtered
	}

	runID := os.Getenv("RPI_RUN_ID")
	ctxDir, ctxErr := ensureContextDir(cwd, runID)
	if ctxErr != nil {
		fmt.Fprintf(os.Stderr, "WARN: context dir: %v\n", ctxErr)
		return nil
	}

	VerbosePrintf("context-dir: %s\n", ctxDir)
	return nil
}

func renderInjectOutput(cwd string, knowledge *injectedKnowledge) (string, error) {
	output := renderKnowledgeIndex(knowledge)
	if !injectIndexOnly {
		rendered, err := renderKnowledge(knowledge, injectFormat)
		if err != nil {
			return "", err
		}
		output = rendered
	}

	charBudget := injectMaxTokens * InjectCharsPerToken
	if len(output) > charBudget {
		if injectFormat == "json" {
			output = trimJSONToCharBudget(knowledge, charBudget)
		} else {
			output = trimToCharBudget(output, charBudget)
		}
	}

	if !injectProfile {
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
	reportPath := filepath.Join(cwd, ".agents", "defrag", "quality-report.json")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no quality report found at %s", reportPath)
		}
		return fmt.Errorf("read quality report: %w", err)
	}

	var report struct {
		FlaggedPaths []string `json:"flagged_paths"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("parse quality report: %w", err)
	}

	quarantined := 0
	for _, p := range report.FlaggedPaths {
		absPath := p
		if !filepath.IsAbs(p) {
			absPath = filepath.Join(cwd, p)
		}
		if err := quarantineLearning(absPath, "flagged by quality report"); err != nil {
			VerbosePrintf("Warning: quarantine %s: %v\n", p, err)
			continue
		}
		quarantined++
	}
	if quarantined > 0 {
		fmt.Printf("Quarantined %d flagged learnings\n", quarantined)
	}
	return nil
}

// resolveInjectQuery returns the query from positional args or the --context flag.
func resolveInjectQuery(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return injectContext
}

// printInjectDryRun prints the dry-run message for inject.
func printInjectDryRun(query string) {
	fmt.Printf("[dry-run] Would inject knowledge")
	if query != "" {
		fmt.Printf(" filtered by: %s", query)
	}
	fmt.Printf(" (max %d tokens)\n", injectMaxTokens)
}

// gatherKnowledge collects all knowledge sources and records citations.
func gatherKnowledge(cwd, query, sessionID string, cfg *config.Config) *injectedKnowledge {
	knowledge := &injectedKnowledge{
		Timestamp: time.Now(),
		Query:     query,
	}

	globalLearningsDir := ""
	globalPatternsDir := ""
	globalWeight := 0.8
	if cfg != nil {
		globalLearningsDir = cfg.Paths.GlobalLearningsDir
		globalPatternsDir = cfg.Paths.GlobalPatternsDir
		globalWeight = cfg.Paths.GlobalWeight
	}

	knowledge.Learnings = gatherLearnings(cwd, query, sessionID, globalLearningsDir, globalWeight)
	knowledge.Patterns = gatherPatterns(cwd, query, sessionID, globalPatternsDir, globalWeight)

	// Non-verbose quality gate summary (stderr — does not pollute stdout inject output)
	if len(knowledge.Learnings) > 0 || len(knowledge.Patterns) > 0 {
		fmt.Fprintf(os.Stderr, "Injected %d learnings, %d patterns\n",
			len(knowledge.Learnings), len(knowledge.Patterns))
	}
	knowledge.Sessions = gatherSessions(cwd, query)
	knowledge.OLConstraints = gatherOLConstraints(cwd, query)

	return knowledge
}

// gatherLearnings collects learnings and records their citations.
func gatherLearnings(cwd, query, sessionID, globalDir string, globalWeight float64) []learning {
	learnings, err := collectLearnings(cwd, query, MaxLearningsToInject, globalDir, globalWeight)
	if err != nil {
		VerbosePrintf("Warning: failed to collect learnings: %v\n", err)
	}

	// Record citations for retrieved learnings (Phase 0: Critical for MemRL feedback loop)
	if !injectNoCite && len(learnings) > 0 {
		if err := recordCitations(cwd, learnings, sessionID, query); err != nil {
			VerbosePrintf("Warning: failed to record citations: %v\n", err)
		} else {
			VerbosePrintf("Recorded %d citations for session %s\n", len(learnings), sessionID)
		}
	}

	return learnings
}

// gatherPatterns collects patterns and records their citations.
func gatherPatterns(cwd, query, sessionID, globalDir string, globalWeight float64) []pattern {
	patterns, err := collectPatterns(cwd, query, MaxPatternsToInject, globalDir, globalWeight)
	if err != nil {
		VerbosePrintf("Warning: failed to collect patterns: %v\n", err)
	}

	// Record citations for retrieved patterns (closes σ gap: patterns were retrieved but never cited)
	if !injectNoCite && len(patterns) > 0 {
		if err := recordPatternCitations(cwd, patterns, sessionID, query); err != nil {
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

// gatherOLConstraints collects Olympus constraints (no-op if .ol/ doesn't exist).
func gatherOLConstraints(cwd, query string) []olConstraint {
	olConstraints, err := collectOLConstraints(cwd, query)
	if err != nil {
		VerbosePrintf("Warning: failed to collect OL constraints: %v\n", err)
	}
	return olConstraints
}

// renderKnowledge formats the knowledge struct into the requested output format.
func renderKnowledge(knowledge *injectedKnowledge, format string) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(knowledge, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal json: %w", err)
		}
		return string(data), nil
	}
	return formatKnowledgeMarkdown(knowledge), nil
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

func writeLearningsSection(sb *strings.Builder, learnings []learning) {
	if len(learnings) == 0 {
		return
	}
	sb.WriteString("### Recent Learnings\n")
	for _, l := range learnings {
		text := l.Title
		if l.Summary != "" {
			text = l.Summary
		}
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", l.ID, text))
	}
	sb.WriteString("\n")
}

func writePatternsSection(sb *strings.Builder, patterns []pattern) {
	if len(patterns) == 0 {
		return
	}
	sb.WriteString("### Active Patterns\n")
	for _, p := range patterns {
		if p.Description != "" {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", p.Name, p.Description))
		} else {
			sb.WriteString(fmt.Sprintf("- **%s**\n", p.Name))
		}
	}
	sb.WriteString("\n")
}

func writeSessionsSection(sb *strings.Builder, sessions []session) {
	if len(sessions) == 0 {
		return
	}
	sb.WriteString("### Recent Sessions\n")
	for _, s := range sessions {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", s.Date, s.Summary))
	}
	sb.WriteString("\n")
}

func writeConstraintsSection(sb *strings.Builder, constraints []olConstraint) {
	if len(constraints) == 0 {
		return
	}
	sb.WriteString("### Olympus Constraints\n")
	for _, c := range constraints {
		sb.WriteString(fmt.Sprintf("- **[olympus constraint]** %s: %s\n", c.Pattern, c.Detection))
	}
	sb.WriteString("\n")
}

// resortLearnings re-sorts learnings by CompositeScore after bead boosting.
func resortLearnings(learnings []learning) {
	slices.SortFunc(learnings, func(a, b learning) int {
		if a.CompositeScore > b.CompositeScore {
			return -1
		}
		if a.CompositeScore < b.CompositeScore {
			return 1
		}
		return 0
	})
}

// filterMemoryDuplicates removes learnings whose Title or ID already appears in MEMORY.md.
// Uses exact title/ID match to avoid false positives.
func filterMemoryDuplicates(cwd string, learnings []learning) []learning {
	memoryFile, err := findMemoryFile(cwd)
	if err != nil {
		return learnings // no MEMORY.md = no dedup
	}
	content, err := os.ReadFile(memoryFile)
	if err != nil {
		return learnings
	}
	memoryText := string(content)

	filtered := make([]learning, 0, len(learnings))
	for _, l := range learnings {
		// Skip if learning ID appears verbatim in MEMORY.md
		if l.ID != "" && strings.Contains(memoryText, l.ID) {
			continue
		}
		// Skip if learning Title appears verbatim in MEMORY.md
		if l.Title != "" && strings.Contains(memoryText, l.Title) {
			continue
		}
		filtered = append(filtered, l)
	}
	return filtered
}

// formatKnowledgeMarkdown formats knowledge as markdown
func formatKnowledgeMarkdown(k *injectedKnowledge) string {
	var sb strings.Builder
	sb.WriteString("## Injected Knowledge (ao inject)\n\n")
	writePredecessorSection(&sb, k.Predecessor)
	writeLearningsSection(&sb, k.Learnings)
	writePatternsSection(&sb, k.Patterns)
	writeSessionsSection(&sb, k.Sessions)
	writeConstraintsSection(&sb, k.OLConstraints)
	if k.Predecessor == nil && len(k.Learnings) == 0 && len(k.Patterns) == 0 && len(k.Sessions) == 0 && len(k.OLConstraints) == 0 {
		sb.WriteString("*No prior knowledge found.*\n\n")
	}
	sb.WriteString(fmt.Sprintf("*Last injection: %s*\n", k.Timestamp.Format(time.RFC3339)))
	return sb.String()
}

// writePredecessorSection writes the predecessor context block.
func writePredecessorSection(sb *strings.Builder, pred *predecessorContext) {
	if pred == nil {
		return
	}
	sb.WriteString("### Predecessor Context")
	if pred.SessionAge != "" {
		sb.WriteString(fmt.Sprintf(" (%s ago)", pred.SessionAge))
	}
	sb.WriteString("\n")
	if pred.WorkingOn != "" {
		sb.WriteString(fmt.Sprintf("- **Working on:** %s\n", pred.WorkingOn))
	}
	if pred.Progress != "" {
		sb.WriteString(fmt.Sprintf("- **Progress:** %s\n", pred.Progress))
	}
	if pred.Blocker != "" {
		sb.WriteString(fmt.Sprintf("- **Blocker:** %s\n", pred.Blocker))
	}
	if pred.NextStep != "" {
		sb.WriteString(fmt.Sprintf("- **Next step:** %s\n", pred.NextStep))
	}
	if pred.RawSummary != "" && pred.Progress == "" {
		sb.WriteString(fmt.Sprintf("- %s\n", pred.RawSummary))
	}
	sb.WriteString("\n")
}

// trimJSONToCharBudget truncates JSON output by progressively removing items
// from the knowledge struct until it fits the budget, then adds a "truncated" field.
func trimJSONToCharBudget(knowledge *injectedKnowledge, budget int) string {
	// Progressively trim: sessions first, then patterns, then learnings
	trimmed := *knowledge
	trimmed.Learnings = append([]learning(nil), knowledge.Learnings...)
	trimmed.Patterns = append([]pattern(nil), knowledge.Patterns...)
	trimmed.Sessions = append([]session(nil), knowledge.Sessions...)
	trimmed.OLConstraints = append([]olConstraint(nil), knowledge.OLConstraints...)

	type truncatedKnowledge struct {
		injectedKnowledge
		Truncated bool `json:"truncated"`
	}

	tryMarshal := func() string {
		tk := truncatedKnowledge{injectedKnowledge: trimmed, Truncated: true}
		data, err := json.MarshalIndent(tk, "", "  ")
		if err != nil {
			return "{\"truncated\": true}"
		}
		return string(data)
	}

	// Remove sessions first
	for len(trimmed.Sessions) > 0 {
		if out := tryMarshal(); len(out) <= budget {
			return out
		}
		trimmed.Sessions = trimmed.Sessions[:len(trimmed.Sessions)-1]
	}
	// Remove OL constraints
	for len(trimmed.OLConstraints) > 0 {
		if out := tryMarshal(); len(out) <= budget {
			return out
		}
		trimmed.OLConstraints = trimmed.OLConstraints[:len(trimmed.OLConstraints)-1]
	}
	// Remove patterns
	for len(trimmed.Patterns) > 0 {
		if out := tryMarshal(); len(out) <= budget {
			return out
		}
		trimmed.Patterns = trimmed.Patterns[:len(trimmed.Patterns)-1]
	}
	// Remove learnings
	for len(trimmed.Learnings) > 0 {
		if out := tryMarshal(); len(out) <= budget {
			return out
		}
		trimmed.Learnings = trimmed.Learnings[:len(trimmed.Learnings)-1]
	}

	return tryMarshal()
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

// atomicWriteFile writes data to a temp file then renames into place,
// preventing corruption from crashes or concurrent writes.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ao-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// truncateText truncates a string to max length with ellipsis.
// Uses rune-safe slicing to avoid breaking multi-byte UTF-8 characters.
func truncateText(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."[:maxLen]
	}
	return string(runes[:maxLen-3]) + "..."
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
