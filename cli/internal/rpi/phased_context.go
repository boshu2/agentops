package rpi

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"
)

// Phase represents an RPI phase with its index and name.
type Phase struct {
	Num  int
	Name string
	Step string // ratchet step name
}

// Phases defines the consolidated 3-phase RPI lifecycle.
var Phases = []Phase{
	{1, "discovery", "research"},
	{2, "implementation", "implement"},
	{3, "validation", "validate"},
}

// RetryContext holds context for retrying a failed gate.
type RetryContext struct {
	Attempt  int
	Findings []Finding
	Verdict  string
}

// PhaseContextBudgets provides phase-specific context guidance.
var PhaseContextBudgets = map[int]string{
	1: "BUDGET: This session runs research + plan + pre-mortem. Research: limit to ~15 file reads, write findings to .agents/research/. Plan: write to .agents/plans/, focus on issue creation. Pre-mortem: invoke /council, read the verdict, done. If pre-mortem FAILs, re-plan and re-run pre-mortem within this session (max 3 attempts).",
	2: "BUDGET (CRITICAL): Crank is the highest-risk phase for context. /crank spawns workers internally. Do NOT re-read worker output into your context. Trust /crank to manage its waves. Read only the completion status.",
	3: "BUDGET: This session runs vibe + post-mortem. Vibe: invoke /council on recent changes, read the verdict. Post-mortem: invoke /council + /retro, read output files, write summary. Minimal context for both.",
}

// PhaseSummaryInstruction is prepended to each phase prompt so Claude writes a rich summary.
const PhaseSummaryInstruction = `PHASE SUMMARY CONTRACT: Before finishing this session, write a concise summary (max 500 tokens) to .agents/rpi/phase-{{.PhaseNum}}-summary.md covering key insights, tradeoffs considered, and risks for subsequent phases. This file is read by the next phase.

`

// ContextDisciplineInstruction is prepended to every phase prompt to prevent compaction.
const ContextDisciplineInstruction = `CONTEXT DISCIPLINE: You are running inside ao rpi phased (phase {{.PhaseNum}} of 3). Each phase gets a FRESH context window. Stay disciplined:
- Do NOT accumulate large file contents in context. Read files with the Read tool JIT and extract only what you need.
- Do NOT explore broadly when narrow exploration suffices. Be surgical.
- Write findings, plans, and results to DISK (files in .agents/), not just in conversation.
- If you are delegating to workers or spawning agents, do NOT accumulate their full output. Read their result files from disk.
- If you notice context degradation (forgetting earlier instructions, repeating yourself, losing track of the goal), IMMEDIATELY write a handoff to .agents/rpi/phase-{{.PhaseNum}}-handoff.md with: (1) what you accomplished, (2) what remains, (3) key context. Then finish cleanly.
{{.ContextBudget}}
`

// AutodevProgramInstruction is the template for autodev program contract injection.
const AutodevProgramInstruction = `{{if .ProgramPath}}AUTODEV PROGRAM CONTRACT: Read {{.ProgramPath}} before any other repo exploration. Treat it as the repo-local operational contract. Stay within its mutable scope, respect immutable scope, run its validation commands before claiming success, use its decision policy for keep vs revert, escalate out-of-scope work, and obey its stop conditions.

{{end}}`

// RetryContextDisciplineInstruction is appended to retry prompts to prevent
// re-doing work that already succeeded in prior phases or prior attempts.
const RetryContextDisciplineInstruction = `Before retrying, summarize what was accomplished in prior phases and what specific issue caused the retry. Do not repeat work that already succeeded.`

// RetryPhaseSummaryInstruction is appended to retry prompts so the model
// includes prior phase context when constructing the retry attempt.
const RetryPhaseSummaryInstruction = `Include a brief summary of prior phase outcomes when constructing the retry context. This helps the model avoid re-doing completed work and focus on the specific failure.`

// PhasePrompts defines Go templates for each phase's Claude invocation.
var PhasePrompts = map[int]string{
	1: `{{if .SwarmFirst}}SWARM-FIRST EXECUTION CONTRACT:
- Default to /swarm for each step in this phase (research, plan, pre-mortem) using a lead + worker team pattern.
- If /swarm runtime is unavailable, execute the direct commands below in this same session.
- Keep worker outputs on disk and consume thin summaries only.

{{end}}Run these skills IN SEQUENCE. Do not skip any step.
NONINTERACTIVE FALLBACK: If this runtime cannot execute slash commands in print mode, perform the equivalent work directly. Still write research findings to .agents/research/, a markdown plan to .agents/plans/, a pre-mortem council report at .agents/council/rpi-pre-mortem.md containing a line exactly like "## Council Verdict: PASS" or "## Council Verdict: WARN", and the required phase summary file.

STEP 1 — Research:
{{if .SwarmFirst}}Prefer: execute this step via /swarm with research-focused workers.
Fallback direct command:
{{end}}/research "{{.Goal}}"{{if not .Interactive}} --auto{{end}}

STEP 2 — Plan:
After research completes, run:
{{if .SwarmFirst}}Prefer: execute this step via /swarm with planning/decomposition workers.
Fallback direct command:
{{end}}/plan "{{.Goal}}"{{if not .Interactive}} --auto{{end}}

STEP 3 — Pre-mortem:
After plan completes, run:
{{if .SwarmFirst}}Prefer: execute this step via /swarm (including council/critique workers when available).
Fallback direct command:
{{end}}/pre-mortem{{if .FastPath}} --quick{{end}}

If pre-mortem returns FAIL, re-run /plan with the findings and then /pre-mortem again. Max 3 total attempts. If still FAIL after 3 attempts, stop and report.
	If pre-mortem returns PASS or WARN, proceed.`,

	2: `{{if .SwarmFirst}}SWARM-FIRST EXECUTION CONTRACT:
- Run implementation with swarm-managed waves by default (lead + worker teams).
- Prefer crank paths that delegate to /swarm for wave execution.

{{end}}NONINTERACTIVE FALLBACK: If this runtime cannot execute /crank in print mode, implement the requested change directly from the referenced issue, plan, or execution packet. Run focused tests, write implementation notes to .agents/rpi/phase-2-summary.md, and keep edits scoped.

{{if .TasklistMode}}TASKLIST MODE: Tracker is unavailable or unhealthy. Use .agents/rpi/execution-packet.json as the objective spine instead of bd issue queries.
/crank .agents/rpi/execution-packet.json{{if .TestFirst}} --test-first{{end}}{{else if .PlanFileMode}}PLAN-FILE MODE: No beads epic exists. Use TaskList for issue tracking.
/crank {{.PlanFilePath}}{{if .TestFirst}} --test-first{{end}}{{else}}/crank {{.EpicID}}{{if .TestFirst}} --test-first{{end}}{{end}}`,

	3: `{{if .SwarmFirst}}SWARM-FIRST EXECUTION CONTRACT:
- Use swarm/team execution for validation and retrospective steps where available.
- Keep validator and implementer contexts isolated; do not reuse implementation worker context.

{{end}}Run these skills IN SEQUENCE. Do not skip any step.
NONINTERACTIVE FALLBACK: If this runtime cannot execute slash commands in print mode, perform equivalent validation and post-mortem work directly. Run the relevant tests, write a vibe council report at .agents/council/rpi-vibe.md containing a line exactly like "## Council Verdict: PASS" or "## Council Verdict: WARN", write post-mortem notes under .agents/retros/, and write the required phase summary file.

STEP 1 — Vibe:
{{if .SwarmFirst}}Prefer: execute vibe using /swarm-driven validation workers.
Fallback direct command:
{{end}}/vibe{{if .FastPath}} --quick{{end}} recent

If vibe returns FAIL, STOP and report the findings. Do NOT proceed to post-mortem.
If vibe returns PASS or WARN, proceed.

STEP 2 — Post-mortem:
{{if .SwarmFirst}}Prefer: execute post-mortem using /swarm-driven retro workers.
Fallback direct command:
{{end}}{{if or .TasklistMode .PlanFileMode}}/post-mortem --quick recent{{else}}/post-mortem{{if .FastPath}} --quick{{end}} {{.EpicID}}{{end}}`,
}

// RetryPrompts defines templates for retry invocations with feedback context.
var RetryPrompts = map[int]string{
	3: `{{if .TasklistMode}}/crank .agents/rpi/execution-packet.json{{if .TestFirst}} --test-first{{end}}{{else if .PlanFileMode}}/crank {{.PlanFilePath}}{{if .TestFirst}} --test-first{{end}}{{else}}/crank {{.EpicID}}{{if .TestFirst}} --test-first{{end}}{{end}}` + "\n\n" +
		`Vibe FAIL (attempt {{.RetryAttempt}}/{{.MaxRetries}}). Address these findings:` + "\n" +
		`{{range .Findings}}FINDING: {{.Description}} | FIX: {{.Fix}} | REF: {{.Ref}}` + "\n" + `{{end}}`,
}

// PhaseNameToNum converts a phase name to a consolidated phase number (1-3).
func PhaseNameToNum(name string) int {
	normalized := strings.ToLower(strings.TrimSpace(name))
	aliases := map[string]int{
		"discovery":      1,
		"implementation": 2,
		"validation":     3,
		"research":       1,
		"plan":           1,
		"pre-mortem":     1,
		"premortem":      1,
		"pre_mortem":     1,
		"crank":          2,
		"implement":      2,
		"vibe":           3,
		"validate":       3,
		"post-mortem":    3,
		"postmortem":     3,
		"post_mortem":    3,
	}
	return aliases[normalized]
}

// ParsePhaseBudgetSpec parses --budget=<phase:seconds,...> into per-phase durations.
func ParsePhaseBudgetSpec(spec string) (map[int]time.Duration, error) {
	budgets := make(map[int]time.Duration)
	trimmed := strings.TrimSpace(spec)
	if trimmed == "" {
		return budgets, nil
	}

	entries := strings.Split(trimmed, ",")
	for _, entry := range entries {
		token := strings.TrimSpace(entry)
		if token == "" {
			return nil, fmt.Errorf("invalid budget spec %q (empty entry)", spec)
		}

		parts := strings.SplitN(token, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid budget entry %q (expected <phase>:<seconds>)", token)
		}

		phaseName := strings.TrimSpace(parts[0])
		phaseNum := PhaseNameToNum(phaseName)
		if phaseNum == 0 {
			return nil, fmt.Errorf("unknown budget phase %q (valid: discovery|implementation|validation and aliases)", phaseName)
		}

		secondsRaw := strings.TrimSpace(parts[1])
		seconds, err := strconv.Atoi(secondsRaw)
		if err != nil || seconds <= 0 {
			return nil, fmt.Errorf("invalid budget seconds %q for phase %q (must be positive integer)", secondsRaw, phaseName)
		}
		budgets[phaseNum] += time.Duration(seconds) * time.Second
	}
	return budgets, nil
}

// DefaultPhaseBudgetForComplexity returns the default budget for a consolidated phase.
func DefaultPhaseBudgetForComplexity(complexity ComplexityLevel, phaseNum int) time.Duration {
	switch phaseNum {
	case 1:
		switch complexity {
		case ComplexityFast:
			return 6 * time.Minute
		case ComplexityFull:
			return 25 * time.Minute
		default:
			return 13 * time.Minute
		}
	case 2:
		return 0
	case 3:
		switch complexity {
		case ComplexityFast:
			return 0
		case ComplexityFull:
			return 10 * time.Minute
		default:
			return 5 * time.Minute
		}
	default:
		return 0
	}
}

// BudgetComplexityLevel returns the effective complexity level for budget resolution.
func BudgetComplexityLevel(fastPath bool, complexity ComplexityLevel) ComplexityLevel {
	if fastPath {
		return ComplexityFast
	}
	if complexity == "" {
		return ComplexityStandard
	}
	return complexity
}

// ResolvePhaseBudget returns the effective per-phase runtime budget.
// Returns hasBudget=false when no budget applies to the phase.
func ResolvePhaseBudget(noBudget bool, budgetSpec string, fastPath bool, complexity ComplexityLevel, phaseNum int) (budget time.Duration, hasBudget bool, err error) {
	if noBudget {
		return 0, false, nil
	}

	overrides, err := ParsePhaseBudgetSpec(budgetSpec)
	if err != nil {
		return 0, false, err
	}
	if override, ok := overrides[phaseNum]; ok {
		return override, true, nil
	}

	defaultBudget := DefaultPhaseBudgetForComplexity(BudgetComplexityLevel(fastPath, complexity), phaseNum)
	if defaultBudget <= 0 {
		return 0, false, nil
	}
	return defaultBudget, true, nil
}

// ReadPhaseSummaries reads all phase summary files prior to the given phase.
func ReadPhaseSummaries(cwd string, currentPhase int) string {
	var summaries []string
	rpiDir := filepath.Join(cwd, ".agents", "rpi")

	for i := 1; i < currentPhase; i++ {
		path := filepath.Join(rpiDir, fmt.Sprintf("phase-%d-summary.md", i))
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		if len(content) > 2000 {
			content = content[:2000] + "..."
		}
		phaseName := "unknown"
		if i > 0 && i <= len(Phases) {
			phaseName = Phases[i-1].Name
		}
		summaries = append(summaries, fmt.Sprintf("[Phase %d: %s]\n%s", i, phaseName, content))
	}

	if len(summaries) == 0 {
		return ""
	}
	return strings.Join(summaries, "\n\n")
}

// BuildPhaseContext constructs a context block from goal, verdicts, and prior phase summaries.
func BuildPhaseContext(cwd, goal string, verdicts map[string]string, phaseNum int) string {
	var parts []string

	if goal != "" {
		parts = append(parts, fmt.Sprintf("Goal: %s", goal))
	}

	for key, verdict := range verdicts {
		parts = append(parts, fmt.Sprintf("%s verdict: %s", strings.ReplaceAll(key, "_", "-"), verdict))
	}

	if cwd != "" {
		summaries := ReadPhaseSummaries(cwd, phaseNum)
		if summaries != "" {
			parts = append(parts, summaries)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	return "--- RPI Context (from prior phases) ---\n" + strings.Join(parts, "\n")
}

// RenderPreambleInstructions renders the context-discipline and summary-contract
// instruction templates into the prompt builder.
func RenderPreambleInstructions(prompt *strings.Builder, data any, warnFn func(string, ...any)) {
	if warnFn == nil {
		warnFn = func(string, ...any) {}
	}
	disciplineTmpl, err := template.New("discipline").Parse(ContextDisciplineInstruction)
	if err == nil {
		if execErr := disciplineTmpl.Execute(prompt, data); execErr != nil {
			warnFn("Warning: could not render context discipline instruction: %v\n", execErr)
		}
	}
	programTmpl, err := template.New("program").Parse(AutodevProgramInstruction)
	if err == nil {
		if execErr := programTmpl.Execute(prompt, data); execErr != nil {
			warnFn("Warning: could not render autodev program instruction: %v\n", execErr)
		}
	}
	summaryTmpl, err := template.New("summary").Parse(PhaseSummaryInstruction)
	if err == nil {
		if execErr := summaryTmpl.Execute(prompt, data); execErr != nil {
			warnFn("Warning: could not render summary instruction: %v\n", execErr)
		}
	}
}
