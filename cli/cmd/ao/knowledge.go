package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const knowledgeBuilderTimeout = 20 * time.Minute

var (
	knowledgeActivateGoal         string
	knowledgeBriefGoal            string
	knowledgePlaybooksIncludeThin bool
)

type knowledgeBuilderInvocation struct {
	Step   string   `json:"step"`
	Script string   `json:"script"`
	Args   []string `json:"args,omitempty"`
}

type knowledgeBuilderRun struct {
	knowledgeBuilderInvocation
	Path     string            `json:"path"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Output   string            `json:"output,omitempty"`
}

type knowledgeBuilderResult struct {
	Workspace  string              `json:"workspace"`
	AgentsRoot string              `json:"agents_root"`
	Step       knowledgeBuilderRun `json:"step"`
	OutputPath string              `json:"output_path,omitempty"`
}

type knowledgeTopicGap struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Health   string   `json:"health"`
	Path     string   `json:"path"`
	OpenGaps []string `json:"open_gaps,omitempty"`
}

type knowledgePromotionGap struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Path    string   `json:"path"`
	Missing []string `json:"missing"`
}

type knowledgeWeakClaim struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type knowledgeGapSummary struct {
	Workspace           string                  `json:"workspace"`
	AgentsRoot          string                  `json:"agents_root"`
	ThinTopics          []knowledgeTopicGap     `json:"thin_topics,omitempty"`
	PromotionGaps       []knowledgePromotionGap `json:"promotion_gaps,omitempty"`
	WeakClaims          []knowledgeWeakClaim    `json:"weak_claims,omitempty"`
	NextRecommendedWork []string                `json:"next_recommended_work,omitempty"`
}

type knowledgeActivateResult struct {
	Workspace      string                `json:"workspace"`
	AgentsRoot     string                `json:"agents_root"`
	BeliefBook     string                `json:"belief_book,omitempty"`
	PlaybooksIndex string                `json:"playbooks_index,omitempty"`
	Briefing       string                `json:"briefing,omitempty"`
	Steps          []knowledgeBuilderRun `json:"steps"`
	Gaps           knowledgeGapSummary   `json:"gaps"`
}

type knowledgeTopicState struct {
	ID       string
	Title    string
	Health   string
	Path     string
	OpenGaps []string
}

var knowledgeCmd = &cobra.Command{
	Use:   "knowledge",
	Short: "Operationalize a mature .agents corpus into reusable operator surfaces",
	Long: `Knowledge turns a mature .agents corpus into operator-ready surfaces.

Subcommands:
  activate   Run the full outer loop over workspace-local builders
  beliefs    Refresh the belief book
  playbooks  Refresh candidate playbooks
  brief      Compile a goal-time briefing
  gaps       Report thin topics, promotion gaps, and next mining work`,
}

func init() {
	knowledgeCmd.GroupID = "knowledge"
	rootCmd.AddCommand(knowledgeCmd)

	activateCmd := &cobra.Command{
		Use:   "activate",
		Short: "Run the full knowledge activation outer loop",
		Args:  cobra.NoArgs,
		RunE:  runKnowledgeActivate,
	}
	activateCmd.Flags().StringVar(&knowledgeActivateGoal, "goal", "", "Optional goal for briefing compilation during activation")

	beliefsCmd := &cobra.Command{
		Use:   "beliefs",
		Short: "Refresh the belief book from promoted evidence",
		Args:  cobra.NoArgs,
		RunE:  runKnowledgeBeliefs,
	}

	playbooksCmd := &cobra.Command{
		Use:   "playbooks",
		Short: "Refresh playbook candidates from healthy topics",
		Args:  cobra.NoArgs,
		RunE:  runKnowledgePlaybooks,
	}
	playbooksCmd.Flags().BoolVar(&knowledgePlaybooksIncludeThin, "include-thin", false, "Include thin topics when building playbook candidates")

	briefCmd := &cobra.Command{
		Use:   "brief",
		Short: "Compile a goal-time briefing",
		Args:  cobra.NoArgs,
		RunE:  runKnowledgeBrief,
	}
	briefCmd.Flags().StringVar(&knowledgeBriefGoal, "goal", "", "Goal to compile into a briefing")
	_ = briefCmd.MarkFlagRequired("goal")

	gapsCmd := &cobra.Command{
		Use:   "gaps",
		Short: "Report thin topics, promotion gaps, and next mining work",
		Args:  cobra.NoArgs,
		RunE:  runKnowledgeGaps,
	}

	knowledgeCmd.AddCommand(activateCmd, beliefsCmd, playbooksCmd, briefCmd, gapsCmd)
}

func runKnowledgeActivate(cmd *cobra.Command, args []string) error {
	workspace, agentsRoot, scriptsRoot, err := resolveKnowledgeWorkspace()
	if err != nil {
		return err
	}

	steps := []knowledgeBuilderInvocation{
		{Step: "source-manifests", Script: "source_manifest_build.py", Args: []string{"--all"}},
		{Step: "topic-packets", Script: "topic_packet_build.py", Args: []string{"--all"}},
		{Step: "promoted-packets", Script: "corpus_packet_promote.py", Args: []string{"--all"}},
		{Step: "chunk-bundles", Script: "knowledge_chunk_build.py", Args: []string{"--all"}},
		{Step: "belief-book", Script: "book_of_beliefs_build.py", Args: []string{"--all"}},
		{Step: "playbooks", Script: "playbook_build.py", Args: []string{"--all"}},
	}
	if strings.TrimSpace(knowledgeActivateGoal) != "" {
		steps = append(steps, knowledgeBuilderInvocation{
			Step:   "briefing",
			Script: "briefing_build.py",
			Args:   []string{"--goal", strings.TrimSpace(knowledgeActivateGoal)},
		})
	}
	if err := requireKnowledgeScripts(scriptsRoot, steps); err != nil {
		return err
	}

	runs := make([]knowledgeBuilderRun, 0, len(steps))
	for _, step := range steps {
		run, runErr := runKnowledgeBuilder(scriptsRoot, step)
		if runErr != nil {
			return runErr
		}
		runs = append(runs, run)
	}

	briefingPath := ""
	if len(runs) > 0 && runs[len(runs)-1].Step == "briefing" {
		briefingPath = firstNonEmptyTrimmed(runs[len(runs)-1].Metadata["briefing"], latestKnowledgeBriefing(agentsRoot))
	}

	result := knowledgeActivateResult{
		Workspace:      workspace,
		AgentsRoot:     agentsRoot,
		BeliefBook:     filepath.Join(agentsRoot, "knowledge", "book-of-beliefs.md"),
		PlaybooksIndex: filepath.Join(agentsRoot, "playbooks", "index.md"),
		Briefing:       briefingPath,
		Steps:          runs,
		Gaps:           collectKnowledgeGapSummary(workspace),
	}

	if !GetDryRun() {
		for _, expected := range []string{result.BeliefBook, result.PlaybooksIndex} {
			if !knowledgePathExists(expected) {
				return fmt.Errorf("knowledge activate succeeded but expected output is missing: %s", expected)
			}
		}
		if strings.TrimSpace(knowledgeActivateGoal) != "" && result.Briefing == "" {
			return fmt.Errorf("knowledge activate succeeded but no briefing output was detected")
		}
	}

	return outputKnowledgeActivateResult(result)
}

func runKnowledgeBeliefs(cmd *cobra.Command, args []string) error {
	workspace, agentsRoot, scriptsRoot, err := resolveKnowledgeWorkspace()
	if err != nil {
		return err
	}
	step := knowledgeBuilderInvocation{Step: "belief-book", Script: "book_of_beliefs_build.py", Args: []string{"--all"}}
	if err := requireKnowledgeScripts(scriptsRoot, []knowledgeBuilderInvocation{step}); err != nil {
		return err
	}
	run, err := runKnowledgeBuilder(scriptsRoot, step)
	if err != nil {
		return err
	}

	result := knowledgeBuilderResult{
		Workspace:  workspace,
		AgentsRoot: agentsRoot,
		Step:       run,
		OutputPath: filepath.Join(agentsRoot, "knowledge", "book-of-beliefs.md"),
	}
	if !GetDryRun() && !knowledgePathExists(result.OutputPath) {
		return fmt.Errorf("belief builder completed but output is missing: %s", result.OutputPath)
	}
	return outputKnowledgeBuilderResult(result)
}

func runKnowledgePlaybooks(cmd *cobra.Command, args []string) error {
	workspace, agentsRoot, scriptsRoot, err := resolveKnowledgeWorkspace()
	if err != nil {
		return err
	}
	step := knowledgeBuilderInvocation{Step: "playbooks", Script: "playbook_build.py", Args: []string{"--all"}}
	if knowledgePlaybooksIncludeThin {
		step.Args = append(step.Args, "--include-thin")
	}
	if err := requireKnowledgeScripts(scriptsRoot, []knowledgeBuilderInvocation{step}); err != nil {
		return err
	}
	run, err := runKnowledgeBuilder(scriptsRoot, step)
	if err != nil {
		return err
	}

	result := knowledgeBuilderResult{
		Workspace:  workspace,
		AgentsRoot: agentsRoot,
		Step:       run,
		OutputPath: filepath.Join(agentsRoot, "playbooks", "index.md"),
	}
	if !GetDryRun() && !knowledgePathExists(result.OutputPath) {
		return fmt.Errorf("playbook builder completed but output is missing: %s", result.OutputPath)
	}
	return outputKnowledgeBuilderResult(result)
}

func runKnowledgeBrief(cmd *cobra.Command, args []string) error {
	workspace, agentsRoot, scriptsRoot, err := resolveKnowledgeWorkspace()
	if err != nil {
		return err
	}
	step := knowledgeBuilderInvocation{
		Step:   "briefing",
		Script: "briefing_build.py",
		Args:   []string{"--goal", strings.TrimSpace(knowledgeBriefGoal)},
	}
	if err := requireKnowledgeScripts(scriptsRoot, []knowledgeBuilderInvocation{step}); err != nil {
		return err
	}
	run, err := runKnowledgeBuilder(scriptsRoot, step)
	if err != nil {
		return err
	}
	result := knowledgeBuilderResult{
		Workspace:  workspace,
		AgentsRoot: agentsRoot,
		Step:       run,
		OutputPath: firstNonEmptyTrimmed(run.Metadata["briefing"], latestKnowledgeBriefing(agentsRoot)),
	}
	if !GetDryRun() && result.OutputPath == "" {
		return fmt.Errorf("briefing builder completed but no briefing output was detected")
	}
	return outputKnowledgeBuilderResult(result)
}

func runKnowledgeGaps(cmd *cobra.Command, args []string) error {
	workspace, _, _, err := resolveKnowledgeWorkspace()
	if err != nil {
		return err
	}
	return outputKnowledgeGapSummary(collectKnowledgeGapSummary(workspace))
}

func resolveKnowledgeWorkspace() (string, string, string, error) {
	workspace, err := resolveProjectDir()
	if err != nil {
		return "", "", "", err
	}
	agentsRoot := filepath.Join(workspace, ".agents")
	if !knowledgePathExists(agentsRoot) {
		return "", "", "", fmt.Errorf("knowledge activation requires %s", agentsRoot)
	}
	return workspace, agentsRoot, filepath.Join(agentsRoot, "scripts"), nil
}

func requireKnowledgeScripts(scriptsRoot string, steps []knowledgeBuilderInvocation) error {
	var missing []string
	for _, step := range steps {
		path := filepath.Join(scriptsRoot, step.Script)
		if !knowledgePathExists(path) {
			missing = append(missing, path)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("knowledge activation requires workspace-local builder scripts:\n- %s", strings.Join(missing, "\n- "))
}

func runKnowledgeBuilder(scriptsRoot string, step knowledgeBuilderInvocation) (knowledgeBuilderRun, error) {
	run := knowledgeBuilderRun{
		knowledgeBuilderInvocation: step,
		Path:                       filepath.Join(scriptsRoot, step.Script),
	}
	if GetDryRun() {
		return run, nil
	}

	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		return run, fmt.Errorf("python3 not found: install Python 3 to use ao knowledge")
	}

	ctx, cancel := contextWithTimeout(knowledgeBuilderTimeout)
	defer cancel()

	builderArgs := append([]string{run.Path}, step.Args...)
	cmd := exec.CommandContext(ctx, pythonPath, builderArgs...)
	cmd.Dir = scriptsRoot
	output, err := cmd.CombinedOutput()
	run.Output = strings.TrimSpace(string(output))
	run.Metadata = parseKnowledgeBuilderMetadata(run.Output)

	if ctx.Err() == context.DeadlineExceeded {
		return run, fmt.Errorf("%s timed out after %s", step.Script, knowledgeBuilderTimeout)
	}
	if err != nil {
		if run.Output == "" {
			return run, fmt.Errorf("%s failed: %w", step.Script, err)
		}
		return run, fmt.Errorf("%s failed: %w\n%s", step.Script, err, run.Output)
	}

	return run, nil
}

func parseKnowledgeBuilderMetadata(output string) map[string]string {
	metadata := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key != "" && value != "" {
			metadata[key] = value
		}
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func outputKnowledgeActivateResult(result knowledgeActivateResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Knowledge activation target: %s\n", result.Workspace)
	for _, step := range result.Steps {
		fmt.Printf("- %s: %s\n", step.Step, filepath.Base(step.Path))
	}
	if result.BeliefBook != "" {
		fmt.Printf("Belief book: %s\n", result.BeliefBook)
	}
	if result.PlaybooksIndex != "" {
		fmt.Printf("Playbooks index: %s\n", result.PlaybooksIndex)
	}
	if result.Briefing != "" {
		fmt.Printf("Briefing: %s\n", result.Briefing)
	}
	fmt.Printf("Thin topics: %d | Promotion gaps: %d | Weak claims: %d\n",
		len(result.Gaps.ThinTopics), len(result.Gaps.PromotionGaps), len(result.Gaps.WeakClaims))
	fmt.Println("Use `ao knowledge gaps` for the full gap report.")
	return nil
}

func outputKnowledgeBuilderResult(result knowledgeBuilderResult) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	fmt.Printf("Knowledge builder: %s\n", result.Step.Step)
	fmt.Printf("Workspace: %s\n", result.Workspace)
	if result.OutputPath != "" {
		fmt.Printf("Output: %s\n", result.OutputPath)
	}
	if result.Step.Output != "" {
		fmt.Printf("Builder output:\n%s\n", result.Step.Output)
	}
	return nil
}

func outputKnowledgeGapSummary(summary knowledgeGapSummary) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}

	fmt.Printf("Knowledge gaps for %s\n", summary.Workspace)
	fmt.Println()
	fmt.Println("Thin topics:")
	if len(summary.ThinTopics) == 0 {
		fmt.Println("- None surfaced")
	} else {
		for _, topic := range summary.ThinTopics {
			reason := topic.Health
			if len(topic.OpenGaps) > 0 {
				reason = topic.OpenGaps[0]
			}
			fmt.Printf("- %s: %s\n", topic.Title, reason)
		}
	}

	fmt.Println()
	fmt.Println("Promotion gaps:")
	if len(summary.PromotionGaps) == 0 {
		fmt.Println("- None surfaced")
	} else {
		for _, gap := range summary.PromotionGaps {
			fmt.Printf("- %s: missing %s\n", gap.Title, strings.Join(gap.Missing, ", "))
		}
	}

	fmt.Println()
	fmt.Println("Weak claims needing review:")
	if len(summary.WeakClaims) == 0 {
		fmt.Println("- None surfaced")
	} else {
		for _, claim := range summary.WeakClaims {
			fmt.Printf("- %s: %s\n", claim.Title, claim.Reason)
		}
	}

	fmt.Println()
	fmt.Println("Next recommended work:")
	if len(summary.NextRecommendedWork) == 0 {
		fmt.Println("- No follow-up work surfaced")
	} else {
		for _, item := range summary.NextRecommendedWork {
			fmt.Printf("- %s\n", item)
		}
	}
	return nil
}

func collectKnowledgeGapSummary(workspace string) knowledgeGapSummary {
	agentsRoot := filepath.Join(workspace, ".agents")
	summary := knowledgeGapSummary{
		Workspace:  workspace,
		AgentsRoot: agentsRoot,
	}

	topics := loadKnowledgeTopics(agentsRoot)
	if len(topics) == 0 {
		summary.NextRecommendedWork = []string{"Generate topic packets before assessing activation gaps."}
		return summary
	}

	hasBeliefBook := knowledgePathExists(filepath.Join(agentsRoot, "knowledge", "book-of-beliefs.md"))
	hasPlaybooksIndex := knowledgePathExists(filepath.Join(agentsRoot, "playbooks", "index.md"))

	for _, topic := range topics {
		if topic.Health != "healthy" {
			summary.ThinTopics = append(summary.ThinTopics, knowledgeTopicGap{
				ID:       topic.ID,
				Title:    topic.Title,
				Health:   topic.Health,
				Path:     topic.Path,
				OpenGaps: topic.OpenGaps,
			})
			if len(topic.OpenGaps) > 0 {
				for _, gap := range topic.OpenGaps {
					summary.WeakClaims = append(summary.WeakClaims, knowledgeWeakClaim{
						ID:     topic.ID,
						Title:  topic.Title,
						Path:   topic.Path,
						Reason: gap,
					})
				}
			} else {
				summary.WeakClaims = append(summary.WeakClaims, knowledgeWeakClaim{
					ID:     topic.ID,
					Title:  topic.Title,
					Path:   topic.Path,
					Reason: fmt.Sprintf("topic health is %s", topic.Health),
				})
			}
			continue
		}

		var missing []string
		if !knowledgePathExists(filepath.Join(agentsRoot, "packets", "promoted", topic.ID+".md")) {
			missing = append(missing, "promoted-packet")
		}
		if !knowledgePathExists(filepath.Join(agentsRoot, "playbooks", topic.ID+".md")) {
			missing = append(missing, "playbook")
		}
		if len(missing) > 0 {
			summary.PromotionGaps = append(summary.PromotionGaps, knowledgePromotionGap{
				ID:      topic.ID,
				Title:   topic.Title,
				Path:    topic.Path,
				Missing: missing,
			})
		}
	}

	nextWork := make([]string, 0, 6)
	if !hasBeliefBook {
		nextWork = append(nextWork, "Refresh the belief book with `ao knowledge beliefs`.")
	}
	if !hasPlaybooksIndex {
		nextWork = append(nextWork, "Refresh playbook candidates with `ao knowledge playbooks`.")
	}
	for _, topic := range summary.ThinTopics {
		nextWork = append(nextWork, fmt.Sprintf("Mine or review more evidence for %s before promotion.", topic.Title))
		if len(nextWork) >= 6 {
			break
		}
	}
	for _, gap := range summary.PromotionGaps {
		if len(nextWork) >= 6 {
			break
		}
		nextWork = append(nextWork, fmt.Sprintf("Promote %s into %s.", gap.Title, strings.Join(gap.Missing, " + ")))
	}
	summary.NextRecommendedWork = dedupeKnowledgeStrings(nextWork)
	return summary
}

func loadKnowledgeTopics(agentsRoot string) []knowledgeTopicState {
	topicsDir := filepath.Join(agentsRoot, "topics")
	entries, err := os.ReadDir(topicsDir)
	if err != nil {
		return nil
	}

	topics := make([]knowledgeTopicState, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		if entry.Name() == "index.md" || entry.Name() == "README.md" {
			continue
		}
		path := filepath.Join(topicsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		frontmatter := parseKnowledgeFrontmatter(text)
		state := knowledgeTopicState{
			ID:     knowledgeFrontmatterString(frontmatter, "topic_id", strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))),
			Title:  knowledgeFrontmatterString(frontmatter, "title", strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))),
			Health: knowledgeFrontmatterString(frontmatter, "health_state", "thin"),
			Path:   path,
		}
		state.OpenGaps = filterKnowledgeOpenGaps(extractKnowledgeBullets(text, "## Open Gaps"))
		topics = append(topics, state)
	}
	return topics
}

func parseKnowledgeFrontmatter(text string) map[string]any {
	if !strings.HasPrefix(text, "---\n") {
		return nil
	}
	rest := strings.TrimPrefix(text, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil
	}
	var frontmatter map[string]any
	if err := yaml.Unmarshal([]byte(rest[:end]), &frontmatter); err != nil {
		return nil
	}
	return frontmatter
}

func knowledgeFrontmatterString(frontmatter map[string]any, key, fallback string) string {
	if frontmatter == nil {
		return fallback
	}
	if value, ok := frontmatter[key]; ok {
		if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
			return text
		}
	}
	return fallback
}

func extractKnowledgeBullets(text, heading string) []string {
	lines := strings.Split(text, "\n")
	items := make([]string, 0)
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == heading {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			break
		}
		if strings.HasPrefix(trimmed, "- ") {
			items = append(items, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		}
	}
	return items
}

func filterKnowledgeOpenGaps(items []string) []string {
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), "No open gaps recorded.") {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func latestKnowledgeBriefing(agentsRoot string) string {
	briefingsDir := filepath.Join(agentsRoot, "briefings")
	entries, err := os.ReadDir(briefingsDir)
	if err != nil {
		return ""
	}

	var latestPath string
	var latestTime time.Time
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if latestPath == "" || info.ModTime().After(latestTime) {
			latestPath = filepath.Join(briefingsDir, entry.Name())
			latestTime = info.ModTime()
		}
	}
	return latestPath
}

func dedupeKnowledgeStrings(items []string) []string {
	seen := make(map[string]bool, len(items))
	deduped := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		deduped = append(deduped, trimmed)
	}
	return deduped
}

func knowledgePathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
