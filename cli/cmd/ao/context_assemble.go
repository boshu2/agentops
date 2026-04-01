package main

import (
	"bufio"
	"cmp"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

// --- constants and defaults ---

const (
	defaultAssembleMaxChars = 28000
	defaultAssembleOutput   = ".agents/rpi/briefing-current.md"

	// Section char budgets (approximate targets).
	budgetGoals    = 2000
	budgetHistory  = 8000
	budgetIntel    = 12000
	budgetTask     = 4000
	budgetProtocol = 2000

	// Maximum history entries to read from cycle-history.jsonl.
	maxHistoryEntries = 5
)

// Redaction patterns.
var (
	envVarLineRe = regexp.MustCompile(`(?i).*(KEY|TOKEN|SECRET|PASSWORD|API).*`)
	envAssignRe  = regexp.MustCompile(`[A-Z_]+=\S+`)
	jwtRe        = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+`)
)

// Static protocol template embedded in the binary.
const protocolTemplate = `## PROTOCOL

### Execution Contract
1. **Verify before claiming** — run the command, read the output, confirm success.
2. **Edit, don't rewrite** — prefer targeted edits over full-file rewrites.
3. **Follow existing patterns** — match the codebase's conventions.
4. **Commit with context** — reference issue IDs in commit messages.
5. **Record in ratchet** — log completion via ao ratchet record.

### Communication Rules
- Workers report to lead only (no peer-to-peer).
- Use filesystem for artifacts, messages for signals only.
- Fresh context per execution unit (Ralph Wiggum pattern).

### Quality Gates
- All existing tests must pass after changes.
- No new linter warnings.
- Build verification for CLI repos (go build + go vet).
`

// --- flag variables ---

var (
	assembleTask     string
	assemblePhase    string
	assembleMaxChars int
	assembleOutput   string
)

// --- cobra command registration ---

func init() {
	assembleCmd := &cobra.Command{
		Use:   "assemble",
		Short: "Build a 5-section context packet briefing for a task",
		Long: `Assemble gathers GOALS, HISTORY, INTEL, TASK, and PROTOCOL into a
single Markdown briefing document. Secrets are redacted before output.

Examples:
  ao context assemble --task="Implement auth middleware"
  ao context assemble --task="Fix rate limiter" --max-chars=20000
  ao context assemble --task="Add tests" --output-file=briefing.md`,
		RunE: runContextAssemble,
	}
	assembleCmd.Flags().StringVar(&assembleTask, "task", "", "Task description (required)")
	assembleCmd.Flags().StringVar(&assemblePhase, "phase", "task", "Context phase: task, startup, planning, pre-mortem, validation")
	assembleCmd.Flags().IntVar(&assembleMaxChars, "max-chars", defaultAssembleMaxChars, "Total character budget")
	assembleCmd.Flags().StringVar(&assembleOutput, "output-file", defaultAssembleOutput, "Output path for briefing")
	_ = assembleCmd.MarkFlagRequired("task")

	// Register under contextCmd (package-level var in context.go).
	contextCmd.AddCommand(assembleCmd)
}

// --- main entrypoint ---

func runContextAssemble(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	// Build sections.
	sections := assembleSectionsForPhase(cwd, assembleTask, assemblePhase, assembleMaxChars)

	// Compose markdown.
	md := composeBriefingMarkdown(sections)

	// Ensure output directory exists.
	outPath := assembleOutput
	if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(cwd, outPath)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return fmt.Errorf("mkdir output dir: %w", err)
	}
	if err := os.WriteFile(outPath, []byte(md), 0o600); err != nil {
		return fmt.Errorf("write briefing: %w", err)
	}

	// Write provenance manifest.
	if err := writeProvenanceManifest(cwd, outPath, sections); err != nil {
		// Non-fatal: log but don't fail.
		fmt.Fprintf(os.Stderr, "warning: provenance manifest: %v\n", err)
	}

	// Output.
	if GetOutput() == "json" {
		return outputAssembleJSON(cmd, outPath, sections)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Briefing written to %s (%d chars)\n", outPath, len(md))
	return nil
}

// --- section assembly ---

type assembledSection struct {
	Name       string `json:"name"`
	CharCount  int    `json:"char_count"`
	Redactions int    `json:"redactions"`
	Content    string `json:"-"`
}

func assembleSections(cwd, task string, maxChars int) []assembledSection {
	return assembleSectionsForPhase(cwd, task, "task", maxChars)
}

func assembleSectionsForPhase(cwd, task, phase string, maxChars int) []assembledSection {
	// Scale budgets proportionally if maxChars differs from default.
	scale := float64(maxChars) / float64(defaultAssembleMaxChars)
	bGoals := int(float64(budgetGoals) * scale)
	bHistory := int(float64(budgetHistory) * scale)
	bIntel := int(float64(budgetIntel) * scale)
	bTask := int(float64(budgetTask) * scale)
	bProtocol := int(float64(budgetProtocol) * scale)

	var sections []assembledSection

	// 1. GOALS
	goalsContent := gatherGoals(cwd, bGoals)
	goalsContent, goalsRedactions := redactContent(goalsContent, cwd)
	sections = append(sections, assembledSection{
		Name:       "GOALS",
		CharCount:  len(goalsContent),
		Redactions: goalsRedactions,
		Content:    goalsContent,
	})

	// 2. HISTORY
	historyContent := gatherHistory(cwd, bHistory)
	historyContent, historyRedactions := redactContent(historyContent, cwd)
	sections = append(sections, assembledSection{
		Name:       sectionHistory,
		CharCount:  len(historyContent),
		Redactions: historyRedactions,
		Content:    historyContent,
	})

	// 3. INTEL
	intelContent := gatherIntel(cwd, task, phase, bIntel)
	intelContent, intelRedactions := redactContent(intelContent, cwd)
	sections = append(sections, assembledSection{
		Name:       sectionIntel,
		CharCount:  len(intelContent),
		Redactions: intelRedactions,
		Content:    intelContent,
	})

	// 4. TASK
	taskContent := formatTaskSection(task, bTask)
	taskContent, taskRedactions := redactContent(taskContent, cwd)
	sections = append(sections, assembledSection{
		Name:       sectionTask,
		CharCount:  len(taskContent),
		Redactions: taskRedactions,
		Content:    taskContent,
	})

	// 5. PROTOCOL
	protocolContent := truncateToCharBudget(protocolTemplate, bProtocol)
	protocolContent, protocolRedactions := redactContent(protocolContent, cwd)
	sections = append(sections, assembledSection{
		Name:       "PROTOCOL",
		CharCount:  len(protocolContent),
		Redactions: protocolRedactions,
		Content:    protocolContent,
	})

	return sections
}

// --- section gatherers ---

func gatherGoals(cwd string, budget int) string {
	var sb strings.Builder
	sb.WriteString("## GOALS\n\n")

	// Try to load and measure goals.
	goalsPath := filepath.Join(cwd, "GOALS.md")
	if _, err := os.Stat(goalsPath); err != nil {
		goalsPath = filepath.Join(cwd, "GOALS.yaml")
	}

	gf, err := goals.LoadGoals(goalsPath)
	if err != nil {
		// Gracefully handle missing GOALS file.
		sb.WriteString("_No GOALS file found._\n")
		return truncateToCharBudget(sb.String(), budget)
	}

	// Format goal summaries — failing gates first for priority.
	sb.WriteString(fmt.Sprintf("Mission: %s\n\n", gf.Mission))

	for _, g := range gf.Goals {
		line := fmt.Sprintf("- **%s** (w:%d, type:%s): %s\n  Check: `%s`\n",
			g.ID, g.Weight, g.Type, g.Description, g.Check)
		if sb.Len()+len(line) > budget {
			break
		}
		sb.WriteString(line)
	}

	return truncateToCharBudget(sb.String(), budget)
}

func gatherHistory(cwd string, budget int) string {
	var sb strings.Builder
	sb.WriteString("## HISTORY\n\n")

	historyPath := filepath.Join(cwd, ".agents", "evolve", "cycle-history.jsonl")
	f, err := os.Open(historyPath)
	if err != nil {
		sb.WriteString("_No cycle history found._\n")
		return truncateToCharBudget(sb.String(), budget)
	}
	defer f.Close()

	// Read all lines, keep last N.
	var allLines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			allLines = append(allLines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		VerbosePrintf("Warning: reading cycle history: %v\n", err)
	}

	// Keep last maxHistoryEntries entries.
	start := 0
	if len(allLines) > maxHistoryEntries {
		start = len(allLines) - maxHistoryEntries
	}
	entries := allLines[start:]

	if len(entries) == 0 {
		sb.WriteString("_No cycle history entries._\n")
		return truncateToCharBudget(sb.String(), budget)
	}

	for i, line := range entries {
		// Try to parse as JSON for pretty display.
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			formatted := formatHistoryEntry(entry, i+1)
			if sb.Len()+len(formatted) > budget {
				break
			}
			sb.WriteString(formatted)
		} else {
			// Raw line fallback.
			if sb.Len()+len(line)+2 > budget {
				break
			}
			sb.WriteString(line + "\n")
		}
	}

	return truncateToCharBudget(sb.String(), budget)
}

func formatHistoryEntry(entry map[string]interface{}, index int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### Entry %d\n", index))

	type historyField struct {
		label   string
		primary string
		aliases []string
	}

	fields := []historyField{
		{label: "timestamp", primary: "timestamp"},
		{label: "cycle", primary: "cycle"},
		{label: "target", primary: "target", aliases: []string{"goal_id"}},
		{label: "goal_ids", primary: "goal_ids"},
		{label: "result", primary: "result", aliases: []string{"status"}},
		{label: "sha", primary: "sha"},
		{label: "canonical_sha", primary: "canonical_sha"},
		{label: "log_sha", primary: "log_sha"},
		{label: "goals_passing", primary: "goals_passing"},
		{label: "goals_total", primary: "goals_total"},
		{label: "summary", primary: "summary"},
		{label: "error", primary: "error"},
	}

	for _, field := range fields {
		if v, ok := lookupHistoryField(entry, field.primary, field.aliases...); ok && v != nil {
			sb.WriteString(fmt.Sprintf("- **%s**: %v\n", field.label, formatHistoryValue(v)))
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func lookupHistoryField(entry map[string]interface{}, primary string, aliases ...string) (interface{}, bool) {
	if v, ok := entry[primary]; ok && v != nil {
		return v, true
	}
	for _, alias := range aliases {
		if v, ok := entry[alias]; ok && v != nil {
			return v, true
		}
	}
	return nil, false
}

func formatHistoryValue(value interface{}) interface{} {
	switch v := value.(type) {
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, ", ")
	case []string:
		return strings.Join(v, ", ")
	default:
		return value
	}
}

func gatherIntel(cwd, task, phase string, budget int) string {
	var sb strings.Builder
	sb.WriteString("## INTEL\n\n")
	sb.WriteString(renderRankedIntelSection(cwd, task, phase, budget))
	return truncateToCharBudget(sb.String(), budget)
}

type intelEntry struct {
	title      string
	content    string
	kind       string // "learning" or "pattern"
	sourcePath string
}

func collectIntelEntries(cwd string) []intelEntry {
	dirs := []struct {
		path string
		kind string
	}{
		{path: filepath.Join(cwd, ".agents", "learnings"), kind: "learning"},
		{path: filepath.Join(cwd, ".agents", "patterns"), kind: "pattern"},
	}

	combined := make([]intelEntry, 0, 64)
	for _, spec := range dirs {
		combined = append(combined, readIntelDir(spec.path, spec.kind)...)
	}
	slices.SortFunc(combined, func(a, b intelEntry) int {
		return cmp.Compare(a.title, b.title)
	})
	return combined
}

func readIntelDir(dir, kind string) []intelEntry {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var result []intelEntry
	for _, e := range entries {
		name := e.Name()
		lowerName := strings.ToLower(name)
		if e.IsDir() || !(strings.HasSuffix(lowerName, ".md") || strings.HasSuffix(lowerName, ".json")) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			VerbosePrintf("Warning: read intel %s: %v\n", name, err)
			continue
		}

		content := strings.TrimSpace(string(data))
		if strings.HasSuffix(lowerName, ".json") {
			content = extractIntelJSONContent(data)
		}

		title := strings.TrimSuffix(name, filepath.Ext(name))
		result = append(result, intelEntry{
			title:      title,
			content:    content,
			kind:       kind,
			sourcePath: filepath.Join(dir, name),
		})
	}
	return result
}

func extractIntelJSONContent(data []byte) string {
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return strings.TrimSpace(string(data))
	}

	for _, key := range []string{"content", "pattern", "summary", "description", "title"} {
		if v, ok := decoded[key]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprintf("%v", v))
			if s != "" {
				return s
			}
		}
	}
	return strings.TrimSpace(string(data))
}

func formatTaskSection(task string, budget int) string {
	var sb strings.Builder
	sb.WriteString("## TASK\n\n")
	sb.WriteString(task)
	sb.WriteString("\n")
	return truncateToCharBudget(sb.String(), budget)
}

// --- char budget enforcement ---

func truncateToCharBudget(content string, budget int) string {
	if budget <= 0 {
		return ""
	}
	runes := []rune(content)
	if len(runes) <= budget {
		return content
	}
	// Truncate at budget, try to break at a newline.
	truncated := string(runes[:budget])
	lastNL := strings.LastIndex(truncated, "\n")
	if lastNL > budget/2 {
		truncated = truncated[:lastNL+1]
	}
	return truncated + "\n... [truncated to fit budget]\n"
}

// --- redaction ---

func redactContent(content, cwd string) (string, int) {
	redactionCount := 0

	// 1. Env var assignments on sensitive lines.
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if envVarLineRe.MatchString(line) {
			if locs := envAssignRe.FindAllStringIndex(line, -1); len(locs) > 0 {
				for j := len(locs) - 1; j >= 0; j-- {
					line = line[:locs[j][0]] + "[REDACTED: env-var]" + line[locs[j][1]:]
					redactionCount++
				}
				lines[i] = line
			}
		}
	}
	content = strings.Join(lines, "\n")

	// 2. JWT tokens.
	jwtMatches := jwtRe.FindAllStringIndex(content, -1)
	for i := len(jwtMatches) - 1; i >= 0; i-- {
		content = content[:jwtMatches[i][0]] + "[REDACTED: jwt-token]" + content[jwtMatches[i][1]:]
		redactionCount++
	}

	// 3. High-entropy strings (>30 chars, >4.5 bits/char).
	content, entropyRedactions := redactHighEntropy(content)
	redactionCount += entropyRedactions

	// Log redactions.
	if redactionCount > 0 {
		logRedactions(cwd, redactionCount)
	}

	return content, redactionCount
}

func redactHighEntropy(content string) (string, int) {
	redactions := 0
	// Find words/tokens > 30 chars that look like secrets.
	wordRe := regexp.MustCompile(`\S{31,}`)
	content = wordRe.ReplaceAllStringFunc(content, func(match string) string {
		if shannonEntropy(match) > 4.5 {
			redactions++
			return "[REDACTED: high-entropy]"
		}
		return match
	})
	return content, redactions
}

func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]int)
	for _, r := range s {
		freq[r]++
	}
	length := float64(len([]rune(s)))
	entropy := 0.0
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

func logRedactions(cwd string, count int) {
	logDir := filepath.Join(cwd, ".agents", "ao")
	_ = os.MkdirAll(logDir, 0o750)
	logPath := filepath.Join(logDir, "redaction.log")

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()

	entry := fmt.Sprintf("%s: redacted %d item(s) during context assemble\n",
		time.Now().UTC().Format(time.RFC3339), count)
	_, _ = f.WriteString(entry)
}

// --- markdown composition ---

func composeBriefingMarkdown(sections []assembledSection) string {
	var sb strings.Builder
	sb.WriteString("# Context Briefing\n\n")
	sb.WriteString(fmt.Sprintf("_Generated: %s_\n\n", time.Now().UTC().Format(time.RFC3339)))

	for _, s := range sections {
		sb.WriteString(s.Content)
		sb.WriteString("\n")
	}

	return sb.String()
}

// --- provenance manifest ---

type provenanceManifest struct {
	Timestamp  string             `json:"timestamp"`
	OutputPath string             `json:"output_path"`
	Task       string             `json:"task"`
	Phase      string             `json:"phase"`
	MaxChars   int                `json:"max_chars"`
	Sections   []assembledSection `json:"sections"`
}

func writeProvenanceManifest(cwd, outPath string, sections []assembledSection) error {
	manifestDir := filepath.Join(cwd, ".agents", "ao", "injections")
	if err := os.MkdirAll(manifestDir, 0o750); err != nil {
		return err
	}

	ts := time.Now().UTC().Format("20060102-150405")
	manifestPath := filepath.Join(manifestDir, ts+".json")

	manifest := provenanceManifest{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		OutputPath: outPath,
		Task:       assembleTask,
		Phase:      normalizeAssemblePhase(assemblePhase),
		MaxChars:   assembleMaxChars,
		Sections:   sections,
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, data, 0o600)
}

// --- JSON output ---

type assembleJSONOutput struct {
	OutputPath    string             `json:"output_path"`
	TotalChars    int                `json:"total_chars"`
	Sections      []assembledSection `json:"sections"`
	TotalRedacted int                `json:"total_redacted"`
	Timestamp     string             `json:"timestamp"`
}

func outputAssembleJSON(cmd *cobra.Command, outPath string, sections []assembledSection) error {
	totalChars := 0
	totalRedacted := 0
	for _, s := range sections {
		totalChars += s.CharCount
		totalRedacted += s.Redactions
	}

	out := assembleJSONOutput{
		OutputPath:    outPath,
		TotalChars:    totalChars,
		Sections:      sections,
		TotalRedacted: totalRedacted,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return nil
}
