package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var contextExplainFlags struct {
	task  string
	phase string
	limit int
}

type contextExplainResult struct {
	Query      string                       `json:"query"`
	Phase      string                       `json:"phase"`
	Repo       string                       `json:"repo"`
	Payload    contextExplainPayloadHealth  `json:"payload"`
	Health     []contextExplainFamilyHealth `json:"health"`
	Selected   []contextExplainSelection    `json:"selected"`
	Suppressed []contextExplainSuppression  `json:"suppressed"`
	Scorecard  stigmergicScorecard          `json:"scorecard"`
}

type contextExplainPayloadHealth struct {
	Status        string `json:"status"`
	SelectedCount int    `json:"selected_count"`
	Reason        string `json:"reason"`
}

type contextExplainFamilyHealth struct {
	Family string `json:"family"`
	Count  int    `json:"count"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type contextExplainSelection struct {
	Class      string `json:"class"`
	Title      string `json:"title"`
	Reason     string `json:"reason"`
	SourcePath string `json:"source_path,omitempty"`
}

type contextExplainSuppression struct {
	Class  string `json:"class"`
	Reason string `json:"reason"`
	Count  int    `json:"count,omitempty"`
}

func init() {
	explainCmd := &cobra.Command{
		Use:   "explain",
		Short: "Explain why context artifacts were selected or suppressed",
		Long: `Show the runtime relevance explanation for a task or phase.

This command uses the same ranked context assembly path as startup and
briefing assembly, then exposes:
  - which artifacts were selected
  - which classes were suppressed by policy
  - packet-family health and thinness diagnostics`,
		RunE: runContextExplain,
	}

	explainCmd.Flags().StringVar(&contextExplainFlags.task, "task", "", "task or query to explain")
	explainCmd.Flags().StringVar(&contextExplainFlags.phase, "phase", "task", "Context phase: task, startup, planning, pre-mortem, validation")
	explainCmd.Flags().IntVar(&contextExplainFlags.limit, "limit", defaultStigmergicPacketLimit, "max items per class")
	contextCmd.AddCommand(explainCmd)
}

func runContextExplain(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	query := strings.TrimSpace(contextExplainFlags.task)
	phase := normalizeAssemblePhase(contextExplainFlags.phase)
	repo := detectRepoName(cwd)
	bundle := collectRankedContextBundle(cwd, query, contextExplainFlags.limit)
	result := buildContextExplainResult(cwd, repo, query, phase, bundle)

	if GetOutput() == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	printContextExplainHuman(cmd, result)
	return nil
}

func buildContextExplainResult(cwd, repo, query, phase string, bundle rankedContextBundle) contextExplainResult {
	bundle = rerankContextBundleForPhase(cwd, query, phase, bundle)
	selected := collectContextExplainSelections(bundle, phase)
	return contextExplainResult{
		Query:      query,
		Phase:      phase,
		Repo:       repo,
		Payload:    contextExplainPayload(selected),
		Health:     collectContextExplainHealth(cwd, bundle),
		Selected:   selected,
		Suppressed: collectContextExplainSuppressions(cwd, bundle, phase),
		Scorecard:  bundle.Packet.Scorecard,
	}
}

func contextExplainPayload(selected []contextExplainSelection) contextExplainPayloadHealth {
	count := len(selected)
	switch {
	case count == 0:
		return contextExplainPayloadHealth{Status: "empty", SelectedCount: 0, Reason: "No ranked artifacts matched the current query and phase."}
	case count < 4:
		return contextExplainPayloadHealth{Status: "thin", SelectedCount: count, Reason: "Payload is present but thin; manual review recommended before trusting it as the only runtime context."}
	default:
		return contextExplainPayloadHealth{Status: "healthy", SelectedCount: count, Reason: "Payload has enough ranked coverage to explain current startup or briefing context."}
	}
}

func collectContextExplainSelections(bundle rankedContextBundle, phase string) []contextExplainSelection {
	var selections []contextExplainSelection

	for _, rule := range bundle.Packet.PlanningRules {
		selections = append(selections, contextExplainSelection{
			Class:  "planning-rule",
			Title:  truncateText(compactText(rule), 140),
			Reason: "Compiled from matched findings in the stigmergic packet.",
		})
	}
	for _, risk := range bundle.Packet.KnownRisks {
		selections = append(selections, contextExplainSelection{
			Class:  "known-risk",
			Title:  truncateText(compactText(risk), 140),
			Reason: "Compiled from matched pre-mortem checks for the current task.",
		})
	}
	for _, finding := range bundle.Findings {
		reason := "Ranked by lexical overlap, applicability, and finding score."
		if stringSliceContainsFold(bundle.Packet.AppliedFindings, finding.ID) {
			reason = "Selected directly by the stigmergic packet as a matched finding."
		}
		selections = append(selections, contextExplainSelection{
			Class:      "finding",
			Title:      firstNonEmpty(finding.Title, finding.ID),
			Reason:     reason,
			SourcePath: finding.Source,
		})
	}
	for _, item := range bundle.Learnings {
		selections = append(selections, contextExplainSelection{
			Class:      "learning",
			Title:      firstNonEmpty(item.Title, item.ID),
			Reason:     "Matched the query and passed runtime retrieval quality gates.",
			SourcePath: item.Source,
		})
	}
	for _, item := range bundle.Patterns {
		selections = append(selections, contextExplainSelection{
			Class:      "pattern",
			Title:      item.Name,
			Reason:     "Matched the query and survived pattern ranking.",
			SourcePath: item.FilePath,
		})
	}
	for _, item := range bundle.NextWork {
		selections = append(selections, contextExplainSelection{
			Class:  "next-work",
			Title:  item.Title,
			Reason: "Selected from the backlog by repo affinity, severity, and query overlap.",
		})
	}
	for _, item := range bundle.RecentSessions {
		selections = append(selections, contextExplainSelection{
			Class:  "recent-session",
			Title:  fmt.Sprintf("%s: %s", item.Date, truncateText(compactText(item.Summary), 100)),
			Reason: "Recent session summary remained relevant to the current query.",
		})
	}
	for _, item := range bundle.Research {
		selections = append(selections, contextExplainSelection{
			Class:      "research",
			Title:      item.Title,
			Reason:     "Recent research artifact matched the current query.",
			SourcePath: item.Path,
		})
	}
	for _, item := range bundle.LegacyIntel {
		selections = append(selections, contextExplainSelection{
			Class:      "legacy-signal",
			Title:      item.title,
			Reason:     fmt.Sprintf("Legacy %s artifact remained query-relevant after ranked signals were assembled.", item.kind),
			SourcePath: item.sourcePath,
		})
	}

	return selections
}

func collectContextExplainHealth(cwd string, bundle rankedContextBundle) []contextExplainFamilyHealth {
	return []contextExplainFamilyHealth{
		describeContextFamily("findings", countMatchingFiles(filepath.Join(cwd, ".agents", SectionFindings), "*.md"), false),
		describeContextFamily("planning-rules", countMatchingFiles(filepath.Join(cwd, ".agents", "planning-rules"), "*.md"), false),
		describeContextFamily("pre-mortem-checks", countMatchingFiles(filepath.Join(cwd, ".agents", "pre-mortem-checks"), "*.md"), false),
		describeContextFamily("next-work", bundle.Packet.Scorecard.UnconsumedItems, false),
		describeContextFamily("topic-packets", countKnowledgeArtifacts(filepath.Join(cwd, ".agents", "topics")), true),
		describeContextFamily("source-manifests", countKnowledgeArtifacts(filepath.Join(cwd, ".agents", "packets", "source-manifests")), true),
		describeContextFamily("promoted-packets", countKnowledgeArtifacts(filepath.Join(cwd, ".agents", "packets", "promoted")), true),
	}
}

func describeContextFamily(name string, count int, experimental bool) contextExplainFamilyHealth {
	status := "healthy"
	reason := "Family has enough artifacts to participate without additional warnings."

	switch {
	case count == 0:
		status = "missing"
		reason = "No artifacts are available from this family in the current workspace."
	case count < 3:
		status = "thin"
		reason = "Coverage is thin; manual review is recommended before treating this family as strong runtime context."
	}

	if experimental {
		if count == 0 {
			return contextExplainFamilyHealth{Family: name, Count: count, Status: "missing", Reason: "Experimental family has no artifacts in this workspace."}
		}
		if count < 3 {
			return contextExplainFamilyHealth{Family: name, Count: count, Status: "manual_review", Reason: "Experimental family is thin and stays suppressed from default startup payloads."}
		}
		return contextExplainFamilyHealth{Family: name, Count: count, Status: "experimental", Reason: "Experimental family is available but remains out of default startup injection until health gates harden."}
	}

	return contextExplainFamilyHealth{Family: name, Count: count, Status: status, Reason: reason}
}

func countKnowledgeArtifacts(dir string) int {
	files := walkKnowledgeFiles(dir, ".md", ".json", ".jsonl")
	count := 0
	for _, file := range files {
		base := strings.ToLower(filepath.Base(file))
		if base == "readme.md" || base == "index.md" {
			continue
		}
		count++
	}
	return count
}

func collectContextExplainSuppressions(cwd string, bundle rankedContextBundle, phase string) []contextExplainSuppression {
	suppressed := []contextExplainSuppression{}
	for _, class := range []string{"discovery-notes", "pending-knowledge", "raw-transcripts"} {
		policy, ok := sessionIntelligencePolicyFor(class)
		if !ok {
			continue
		}
		item := contextExplainSuppression{Class: class, Reason: policy.SuppressionReason}
		if class == "pending-knowledge" {
			item.Count = countKnowledgeArtifacts(filepath.Join(cwd, ".agents", "knowledge", "pending"))
		}
		suppressed = append(suppressed, item)
	}

	for _, health := range collectContextExplainHealth(cwd, bundle) {
		if !strings.Contains(health.Family, "packets") && !strings.Contains(health.Family, "manifests") {
			continue
		}
		reason := health.Reason
		if policy, ok := sessionIntelligencePolicyFor(health.Family); ok {
			reason = policy.SuppressionReason
		}
		suppressed = append(suppressed, contextExplainSuppression{
			Class:  health.Family,
			Reason: reason,
			Count:  health.Count,
		})
	}

	if len(bundle.Learnings) == 0 {
		suppressed = append(suppressed, contextExplainSuppression{Class: "learning", Reason: fmt.Sprintf("No learnings ranked into the %s payload for this query.", phase)})
	}
	if len(bundle.Patterns) == 0 {
		suppressed = append(suppressed, contextExplainSuppression{Class: "pattern", Reason: fmt.Sprintf("No patterns ranked into the %s payload for this query.", phase)})
	}
	if len(bundle.Findings) == 0 {
		suppressed = append(suppressed, contextExplainSuppression{Class: "finding", Reason: fmt.Sprintf("No findings ranked into the %s payload for this query.", phase)})
	}

	return suppressed
}

func printContextExplainHuman(cmd *cobra.Command, result contextExplainResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "## Context Explain")
	fmt.Fprintf(w, "- Query: %s\n", firstNonEmpty(result.Query, "(none)"))
	fmt.Fprintf(w, "- Phase: %s\n", result.Phase)
	fmt.Fprintf(w, "- Repo: %s\n", result.Repo)
	fmt.Fprintf(w, "- Payload: %s (%d selected)\n", strings.ToUpper(result.Payload.Status), result.Payload.SelectedCount)
	fmt.Fprintf(w, "  %s\n\n", result.Payload.Reason)

	fmt.Fprintln(w, "## Packet Health")
	for _, item := range result.Health {
		fmt.Fprintf(w, "- %s: %s (%d) - %s\n", item.Family, strings.ToUpper(item.Status), item.Count, item.Reason)
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## Selected")
	if len(result.Selected) == 0 {
		fmt.Fprintln(w, "- None")
	} else {
		for _, item := range result.Selected {
			fmt.Fprintf(w, "- [%s] %s - %s\n", item.Class, item.Title, item.Reason)
			if item.SourcePath != "" {
				fmt.Fprintf(w, "  source: %s\n", item.SourcePath)
			}
		}
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "## Suppressed")
	for _, item := range result.Suppressed {
		if item.Count > 0 {
			fmt.Fprintf(w, "- [%s] %s (%d)\n", item.Class, item.Reason, item.Count)
			continue
		}
		fmt.Fprintf(w, "- [%s] %s\n", item.Class, item.Reason)
	}
}
