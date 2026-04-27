package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	aocontext "github.com/boshu2/agentops/cli/internal/context"
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

type contextExplainPayloadHealth = aocontext.ExplainPayloadHealth
type contextExplainFamilyHealth = aocontext.ExplainFamilyHealth
type contextExplainSelection = aocontext.ExplainSelection
type contextExplainSuppression = aocontext.ExplainSuppression

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
	_ = explainCmd.RegisterFlagCompletionFunc("phase", staticCompletionFunc("task", "startup", "planning", "pre-mortem", "validation"))
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
	return aocontext.ExplainPayload(selected)
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
		reason := nextWorkExplainReason(bundle.CWD, item)
		selections = append(selections, contextExplainSelection{
			Class:  "next-work",
			Title:  item.Title,
			Reason: reason,
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
	return aocontext.DescribeContextFamily(name, count, experimental)
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
	suppressed = append(suppressed, collectContextExplainNextWorkProofSuppressions(cwd)...)

	return suppressed
}

func nextWorkExplainReason(cwd string, item nextWorkItem) string {
	if proof := classifyNextWorkCompletionProof(cwd, "", item); proof.Complete {
		return proofBackedNextWorkReason(proof)
	}
	return "Selected from the backlog by repo affinity, severity, and query overlap."
}

func proofBackedNextWorkReason(proof nextWorkProofDecision) string {
	return aocontext.ProofBackedNextWorkReason(proof.Source, proof.Detail)
}

func collectContextExplainNextWorkProofSuppressions(cwd string) []contextExplainSuppression {
	queuePath := filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl")
	data, err := os.ReadFile(queuePath)
	if err != nil || len(data) == 0 {
		return nil
	}

	type aggregate struct {
		count  int
		detail string
	}

	aggregates := map[string]*aggregate{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entry, err := parseNextWorkEntryLine(line)
		if err != nil {
			continue
		}
		for _, item := range entry.Items {
			proof := classifyNextWorkCompletionProof(cwd, entry.SourceEpic, item)
			if !proof.Complete {
				continue
			}
			agg, ok := aggregates[proof.Source]
			if !ok {
				agg = &aggregate{}
				aggregates[proof.Source] = agg
			}
			agg.count++
			if agg.detail == "" {
				agg.detail = proof.Detail
			}
		}
	}

	order := []string{"completed_run", "evidence_only_closure", "execution_packet"}
	suppressed := make([]contextExplainSuppression, 0, len(aggregates))
	for _, source := range order {
		agg, ok := aggregates[source]
		if !ok || agg.count == 0 {
			continue
		}
		suppressed = append(suppressed, contextExplainSuppression{
			Class:  "next-work",
			Count:  agg.count,
			Reason: proofBackedNextWorkSuppressionReason(source, agg.count, agg.detail),
		})
	}
	if len(aggregates) == 0 {
		return nil
	}
	return suppressed
}

func proofBackedNextWorkSuppressionReason(source string, count int, detail string) string {
	return aocontext.ProofBackedNextWorkSuppressionReason(source, count, detail)
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
