package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/config"
)

var (
	dreamCouncilRunnerTimeout = 90 * time.Second
	dreamRunCodexCouncilFn    = dreamRunCodexCouncil
	dreamRunClaudeCouncilFn   = dreamRunClaudeCouncil
)

type overnightCouncilRunnerReport struct {
	Runner                 string   `json:"runner" yaml:"runner"`
	Headline               string   `json:"headline" yaml:"headline"`
	RecommendedKind        string   `json:"recommended_kind" yaml:"recommended_kind"`
	RecommendedFirstAction string   `json:"recommended_first_action" yaml:"recommended_first_action"`
	Risks                  []string `json:"risks" yaml:"risks"`
	Opportunities          []string `json:"opportunities" yaml:"opportunities"`
	Confidence             string   `json:"confidence" yaml:"confidence"`
	WildcardIdea           string   `json:"wildcard_idea" yaml:"wildcard_idea"`
}

type overnightCouncilSummary struct {
	RequestedRunners       []string                       `json:"requested_runners" yaml:"requested_runners"`
	CompletedRunners       []string                       `json:"completed_runners,omitempty" yaml:"completed_runners,omitempty"`
	FailedRunners          []string                       `json:"failed_runners,omitempty" yaml:"failed_runners,omitempty"`
	ConsensusPolicy        string                         `json:"consensus_policy" yaml:"consensus_policy"`
	ConsensusKind          string                         `json:"consensus_kind,omitempty" yaml:"consensus_kind,omitempty"`
	RecommendedFirstAction string                         `json:"recommended_first_action,omitempty" yaml:"recommended_first_action,omitempty"`
	Reports                []overnightCouncilRunnerReport `json:"reports,omitempty" yaml:"reports,omitempty"`
	Disagreements          []string                       `json:"disagreements,omitempty" yaml:"disagreements,omitempty"`
	WildcardIdeas          []string                       `json:"wildcard_ideas,omitempty" yaml:"wildcard_ideas,omitempty"`
}

type overnightDreamscape struct {
	Weather    string `json:"weather" yaml:"weather"`
	Visibility string `json:"visibility" yaml:"visibility"`
	Council    string `json:"council" yaml:"council"`
	Tension    string `json:"tension,omitempty" yaml:"tension,omitempty"`
	FirstMove  string `json:"first_move" yaml:"first_move"`
}

type dreamCouncilPacket struct {
	RunID           string         `json:"run_id"`
	Goal            string         `json:"goal,omitempty"`
	RepoRoot        string         `json:"repo_root"`
	ConsensusPolicy string         `json:"consensus_policy"`
	CreativeLane    bool           `json:"creative_lane"`
	CloseLoop       map[string]any `json:"close_loop,omitempty"`
	MetricsHealth   map[string]any `json:"metrics_health,omitempty"`
	RetrievalLive   map[string]any `json:"retrieval_live,omitempty"`
	Briefing        map[string]any `json:"briefing,omitempty"`
	NextActionHint  string         `json:"next_action_hint,omitempty"`
}

func resolveDreamRunRunners(dcfg config.DreamConfig) []string {
	selected := normalizeDreamRunnerList(dcfg.Runners)
	if len(overnightRunners) > 0 {
		selected = normalizeDreamRunnerList(overnightRunners)
	}
	if value := strings.TrimSpace(overnightModels); value != "" {
		selected = normalizeDreamRunnerList([]string{value})
	}
	out := make([]string, 0, len(selected))
	for _, name := range selected {
		switch name {
		case "codex", "claude":
			out = append(out, name)
		}
	}
	return out
}

func resolveDreamRunnerModels(cfg *config.Config) map[string]string {
	tier := cfg.ResolveTier("council")
	tierCfg, ok := cfg.Models.Tiers[tier]
	if !ok {
		return map[string]string{}
	}
	return map[string]string{
		"claude": strings.TrimSpace(tierCfg.Claude),
		"codex":  strings.TrimSpace(tierCfg.Codex),
	}
}

func resolveDreamConsensusPolicy(dcfg config.DreamConfig) string {
	policy := strings.TrimSpace(dcfg.ConsensusPolicy)
	if policy == "" {
		policy = "majority"
	}
	return policy
}

func resolveDreamCreativeLane(dcfg config.DreamConfig) bool {
	enabled := false
	if dcfg.CreativeLane != nil {
		enabled = *dcfg.CreativeLane
	}
	if overnightCreative {
		enabled = true
	}
	return enabled
}

func appendDreamCouncilPlan(summary *overnightSummary, settings overnightSettings) {
	if len(settings.Runners) == 0 {
		return
	}
	councilDir := filepath.Join(summary.OutputDir, "council")
	summary.Artifacts["council_packet"] = filepath.Join(councilDir, "packet.json")
	summary.Artifacts["council_synthesis"] = filepath.Join(councilDir, "synthesis.json")
	summary.Steps = append(summary.Steps, overnightStepSummary{
		Name:     "council-packet",
		Status:   "pending",
		Artifact: summary.Artifacts["council_packet"],
		Note:     strings.Join(settings.Runners, ","),
	})
	for _, runner := range settings.Runners {
		key := "council_" + runner
		summary.Artifacts[key] = filepath.Join(councilDir, runner+".json")
		summary.Steps = append(summary.Steps, overnightStepSummary{
			Name:     "council-" + runner,
			Status:   "pending",
			Artifact: summary.Artifacts[key],
		})
	}
	summary.Steps = append(summary.Steps, overnightStepSummary{
		Name:     "council-synthesis",
		Status:   "pending",
		Artifact: summary.Artifacts["council_synthesis"],
	})
	summary.Council = &overnightCouncilSummary{
		RequestedRunners: append([]string{}, settings.Runners...),
		ConsensusPolicy:  settings.Consensus,
	}
}

func runDreamCouncil(ctx context.Context, cwd string, log io.Writer, summary *overnightSummary, settings overnightSettings) error {
	if len(settings.Runners) == 0 {
		return nil
	}
	packet := dreamCouncilPacket{
		RunID:           summary.RunID,
		Goal:            summary.Goal,
		RepoRoot:        summary.RepoRoot,
		ConsensusPolicy: settings.Consensus,
		CreativeLane:    settings.CreativeLane,
		NextActionHint:  deriveDreamNextAction(*summary),
	}
	if data, err := loadJSONMap(summary.Artifacts["close_loop"]); err == nil {
		packet.CloseLoop = data
	}
	if data, err := loadJSONMap(summary.Artifacts["metrics_health"]); err == nil {
		packet.MetricsHealth = data
	}
	if data, err := loadJSONMap(summary.Artifacts["retrieval_live"]); err == nil {
		packet.RetrievalLive = data
	}
	if path := summary.Artifacts["briefing"]; path != "" {
		if data, err := loadJSONMap(path); err == nil {
			packet.Briefing = data
		}
	}

	if err := writeJSONFile(summary.Artifacts["council_packet"], packet); err != nil {
		return fmt.Errorf("write dream council packet: %w", err)
	}
	setOvernightStepStatus(summary, "council-packet", "done", summary.Artifacts["council_packet"], "")

	schemaPath := filepath.Join(summary.OutputDir, "council", "report-schema.json")
	if err := writeDreamCouncilSchema(schemaPath); err != nil {
		return fmt.Errorf("write dream council schema: %w", err)
	}

	reports := make([]overnightCouncilRunnerReport, 0, len(settings.Runners))
	failed := []string{}
	for _, runner := range settings.Runners {
		artifactKey := "council_" + runner
		artifactPath := summary.Artifacts[artifactKey]
		report, err := runDreamCouncilRunner(ctx, cwd, log, runner, settings.RunnerModels[runner], schemaPath, packet, artifactPath, settings.CreativeLane)
		if err != nil {
			setOvernightStepStatus(summary, "council-"+runner, "soft-fail", artifactPath, err.Error())
			summary.Degraded = append(summary.Degraded, fmt.Sprintf("%s council run failed: %v", runner, err))
			failed = append(failed, runner)
			continue
		}
		setOvernightStepStatus(summary, "council-"+runner, "done", artifactPath, "")
		reports = append(reports, report)
	}

	if len(reports) == 0 {
		setOvernightStepStatus(summary, "council-synthesis", "soft-fail", summary.Artifacts["council_synthesis"], "no Dream Council runner completed")
		if summary.Council == nil {
			summary.Council = &overnightCouncilSummary{}
		}
		summary.Council.FailedRunners = failed
		return nil
	}

	synthesis := synthesizeDreamCouncil(settings.Runners, failed, settings.Consensus, reports)
	if err := writeJSONFile(summary.Artifacts["council_synthesis"], synthesis); err != nil {
		return fmt.Errorf("write dream council synthesis: %w", err)
	}
	setOvernightStepStatus(summary, "council-synthesis", "done", summary.Artifacts["council_synthesis"], "")
	summary.Council = &synthesis
	return nil
}

func runDreamCouncilRunner(
	ctx context.Context,
	cwd string,
	log io.Writer,
	runner string,
	model string,
	schemaPath string,
	packet dreamCouncilPacket,
	outputPath string,
	creative bool,
) (overnightCouncilRunnerReport, error) {
	promptBytes, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return overnightCouncilRunnerReport{}, fmt.Errorf("marshal council packet: %w", err)
	}
	prompt := buildDreamCouncilPrompt(runner, string(promptBytes), creative)
	runnerCtx, cancel, runnerTimeout := withDreamCouncilRunnerTimeout(ctx)
	defer cancel()
	tempOutputPath, cleanup, err := newDreamCouncilOutputTempFile(outputPath)
	if err != nil {
		return overnightCouncilRunnerReport{}, fmt.Errorf("prepare %s output path: %w", runner, err)
	}
	defer cleanup()
	switch runner {
	case "codex":
		if err := dreamRunCodexCouncilFn(runnerCtx, cwd, model, schemaPath, prompt, tempOutputPath, log); err != nil {
			return overnightCouncilRunnerReport{}, normalizeDreamCouncilRunnerError(runner, runnerTimeout, runnerCtx, err)
		}
	case "claude":
		if err := dreamRunClaudeCouncilFn(runnerCtx, cwd, model, schemaPath, prompt, tempOutputPath, log); err != nil {
			return overnightCouncilRunnerReport{}, normalizeDreamCouncilRunnerError(runner, runnerTimeout, runnerCtx, err)
		}
	default:
		return overnightCouncilRunnerReport{}, fmt.Errorf("unsupported Dream Council runner %q", runner)
	}
	report, data, err := loadDreamCouncilRunnerReport(tempOutputPath, runner)
	if err != nil {
		return overnightCouncilRunnerReport{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return overnightCouncilRunnerReport{}, fmt.Errorf("prepare %s artifact dir: %w", runner, err)
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return overnightCouncilRunnerReport{}, fmt.Errorf("write %s output: %w", runner, err)
	}
	return report, nil
}

func withDreamCouncilRunnerTimeout(parent context.Context) (context.Context, context.CancelFunc, time.Duration) {
	timeout := dreamCouncilRunnerTimeout
	if deadline, ok := parent.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	if timeout <= 0 {
		timeout = time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	return ctx, cancel, timeout
}

func newDreamCouncilOutputTempFile(outputPath string) (string, func(), error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return "", func() {}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(outputPath), filepath.Base(outputPath)+".*.tmp")
	if err != nil {
		return "", func() {}, err
	}
	path := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() {}, err
	}
	cleanup := func() { _ = os.Remove(path) }
	return path, cleanup, nil
}

func normalizeDreamCouncilRunnerError(runner string, timeout time.Duration, runnerCtx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(runnerCtx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%s council timed out after %s", runner, timeout.Round(time.Second))
	}
	if errors.Is(err, context.Canceled) || errors.Is(runnerCtx.Err(), context.Canceled) {
		return fmt.Errorf("%s council canceled", runner)
	}
	return err
}

func loadDreamCouncilRunnerReport(path, runner string) (overnightCouncilRunnerReport, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return overnightCouncilRunnerReport{}, nil, fmt.Errorf("read %s output: %w", runner, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return overnightCouncilRunnerReport{}, nil, fmt.Errorf("%s output was empty", runner)
	}
	var report overnightCouncilRunnerReport
	if err := json.Unmarshal(data, &report); err != nil {
		return overnightCouncilRunnerReport{}, nil, fmt.Errorf("parse %s output: %w", runner, err)
	}
	if err := validateDreamCouncilRunnerReport(runner, report); err != nil {
		return overnightCouncilRunnerReport{}, nil, err
	}
	report.Runner = runner
	normalized, err := json.Marshal(report)
	if err != nil {
		return overnightCouncilRunnerReport{}, nil, fmt.Errorf("rewrite %s output: %w", runner, err)
	}
	return report, normalized, nil
}

func validateDreamCouncilRunnerReport(expectedRunner string, report overnightCouncilRunnerReport) error {
	switch {
	case strings.TrimSpace(report.Runner) == "":
		return fmt.Errorf("%s output missing runner field", expectedRunner)
	case !dreamCouncilRunnerMatches(expectedRunner, report.Runner):
		return fmt.Errorf("%s output runner mismatch: got %q", expectedRunner, report.Runner)
	case strings.TrimSpace(report.Headline) == "":
		return fmt.Errorf("%s output missing headline", expectedRunner)
	case strings.TrimSpace(report.RecommendedKind) == "":
		return fmt.Errorf("%s output missing recommended_kind", expectedRunner)
	case strings.TrimSpace(report.RecommendedFirstAction) == "":
		return fmt.Errorf("%s output missing recommended_first_action", expectedRunner)
	case strings.TrimSpace(report.Confidence) == "":
		return fmt.Errorf("%s output missing confidence", expectedRunner)
	default:
		return nil
	}
}

func dreamCouncilRunnerMatches(expectedRunner, actualRunner string) bool {
	expected := normalizeDreamCouncilRunnerLabel(expectedRunner)
	actual := normalizeDreamCouncilRunnerLabel(actualRunner)
	return actual == expected || actual == expected+"-dream-council"
}

func normalizeDreamCouncilRunnerLabel(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("_", "-", " ", "-")
	normalized = replacer.Replace(normalized)
	return strings.Trim(normalized, "-")
}

func buildDreamCouncilPrompt(runner, packet string, creative bool) string {
	wildcard := "Return wildcard_idea as an empty string when you do not have a useful wildcard idea."
	if creative {
		wildcard = "Include one bounded wildcard idea in wildcard_idea when you see a genuinely useful creative branch; otherwise return wildcard_idea as an empty string."
	}
	return fmt.Sprintf(`You are the %s Dream Council runner.

Analyze the bedtime packet below and return JSON that matches the provided schema.

Rules:
- do not use tools
- stay grounded in the packet
- set runner exactly to "%s"
- choose one recommended_kind from the schema enum
- make recommended_first_action concrete and immediately actionable
- keep risks/opportunities short
- always include wildcard_idea as a string
- %s

Bedtime packet:
%s
`, runner, runner, wildcard, packet)
}

func writeDreamCouncilSchema(path string) error {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"runner":   map[string]any{"type": "string"},
			"headline": map[string]any{"type": "string"},
			"recommended_kind": map[string]any{
				"type": "string",
				"enum": []string{"research", "implement", "validate", "repair", "promote", "document"},
			},
			"recommended_first_action": map[string]any{"type": "string"},
			"risks":                    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"opportunities":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"confidence":               map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}},
			"wildcard_idea":            map[string]any{"type": "string"},
		},
		"required": []string{
			"runner",
			"headline",
			"recommended_kind",
			"recommended_first_action",
			"risks",
			"opportunities",
			"confidence",
			"wildcard_idea",
		},
	}
	return writeJSONFile(path, schema)
}

func dreamRunCodexCouncil(ctx context.Context, cwd, model, schemaPath, prompt, outputPath string, log io.Writer) error {
	args := []string{"exec", "--skip-git-repo-check", "-C", cwd, "--output-schema", schemaPath, "-o", outputPath}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, prompt)
	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Stdout = log
	cmd.Stderr = log
	return cmd.Run()
}

func dreamRunClaudeCouncil(ctx context.Context, cwd, model, schemaPath, prompt, outputPath string, log io.Writer) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() { _ = outFile.Close() }()

	args := []string{"-p", "--json-schema", schemaPath}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, prompt)
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = cwd
	cmd.Stdout = outFile
	cmd.Stderr = log
	return cmd.Run()
}

func synthesizeDreamCouncil(requested, failed []string, policy string, reports []overnightCouncilRunnerReport) overnightCouncilSummary {
	summary := overnightCouncilSummary{
		RequestedRunners: append([]string{}, requested...),
		FailedRunners:    append([]string{}, failed...),
		ConsensusPolicy:  policy,
		Reports:          append([]overnightCouncilRunnerReport{}, reports...),
	}
	counts := map[string]int{}
	actions := map[string]string{}
	for _, report := range reports {
		summary.CompletedRunners = append(summary.CompletedRunners, report.Runner)
		counts[report.RecommendedKind]++
		if _, ok := actions[report.RecommendedKind]; !ok {
			actions[report.RecommendedKind] = report.RecommendedFirstAction
		}
		if report.WildcardIdea != "" {
			summary.WildcardIdeas = append(summary.WildcardIdeas, fmt.Sprintf("%s: %s", report.Runner, report.WildcardIdea))
		}
	}
	bestKind := ""
	bestCount := 0
	for kind, count := range counts {
		if count > bestCount || (count == bestCount && kind < bestKind) {
			bestKind = kind
			bestCount = count
		}
	}
	summary.ConsensusKind = bestKind
	summary.RecommendedFirstAction = actions[bestKind]
	for _, report := range reports {
		if report.RecommendedKind != bestKind || report.RecommendedFirstAction != summary.RecommendedFirstAction {
			summary.Disagreements = append(summary.Disagreements, fmt.Sprintf("%s prefers %s: %s", report.Runner, report.RecommendedKind, report.RecommendedFirstAction))
		}
	}
	sort.Strings(summary.CompletedRunners)
	sort.Strings(summary.FailedRunners)
	sort.Strings(summary.Disagreements)
	sort.Strings(summary.WildcardIdeas)
	return summary
}

func ensureOvernightDerivedViews(summary *overnightSummary) {
	if summary.Council == nil {
		if path := summary.Artifacts["council_synthesis"]; path != "" {
			data, err := os.ReadFile(path)
			if err == nil {
				var council overnightCouncilSummary
				if json.Unmarshal(data, &council) == nil {
					summary.Council = &council
				}
			}
		}
	}
	scape := buildDreamscape(*summary)
	summary.Dreamscape = &scape
}

func buildDreamscape(summary overnightSummary) overnightDreamscape {
	scape := overnightDreamscape{
		Weather:    "steady",
		Visibility: "clear",
		Council:    "single-voice",
		FirstMove:  deriveDreamNextAction(summary),
	}
	if coverage, ok := lookupFloat(summary.RetrievalLive, "coverage"); ok && coverage < 0.50 {
		scape.Weather = "fog"
		scape.Visibility = "limited"
	}
	if escape, ok := lookupBool(summary.MetricsHealth, "escape_velocity"); ok && !escape {
		scape.Weather = "storm-front"
	}
	if summary.Council != nil {
		switch {
		case len(summary.Council.CompletedRunners) > 1 && len(summary.Council.Disagreements) == 0:
			scape.Council = "aligned"
		case len(summary.Council.Disagreements) > 0:
			scape.Council = "mixed"
			scape.Tension = summary.Council.Disagreements[0]
		case len(summary.Council.CompletedRunners) > 0:
			scape.Council = "single-voice"
		}
		if summary.Council.RecommendedFirstAction != "" {
			scape.FirstMove = summary.Council.RecommendedFirstAction
		}
	}
	if len(summary.Degraded) > 0 && scape.Tension == "" {
		scape.Tension = summary.Degraded[0]
	}
	return scape
}

func writeJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
