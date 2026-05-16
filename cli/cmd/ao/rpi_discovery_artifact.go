// Package main - rpi discovery-artifact helper.
//
// Implements --discovery-artifact=<path> for `ao rpi phased`. When the caller
// has already produced a validated discovery artifact (e.g. from
// /council --evidence --commit-ready), this flag lets them skip Phase 1 and
// write a minimally-structured execution packet directly from the artifact.
//
// See skills/rpi/references/discovery-artifact-mode.md for the behavioural
// spec. This file is intentionally standalone so the core phased runner stays
// unchanged when the flag is not set.
// practices: [agile-manifesto, dora-metrics]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// discoveryArtifact is the in-memory representation of a parsed discovery
// artifact (markdown with optional YAML frontmatter). Fields map 1:1 to the
// execution packet shape expected by /crank.
type discoveryArtifact struct {
	Goal        string   `json:"goal"`
	InScope     []string `json:"in_scope"`
	OutOfScope  []string `json:"out_of_scope"`
	LocEstimate string   `json:"loc_estimate,omitempty"`
	AbortGates  []string `json:"abort_gates"`
	TDDMatrix   []string `json:"tdd_matrix"`
	Risks       []string `json:"risks"`
	SourcePath  string   `json:"source_path"`
}

// loadDiscoveryArtifact reads and parses a discovery artifact from path.
// It validates that the file exists and returns a populated discoveryArtifact.
// Parsing is best-effort: missing sections degrade gracefully to empty slices.
func loadDiscoveryArtifact(path string) (*discoveryArtifact, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("discovery-artifact path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve discovery-artifact path %q: %w", path, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("discovery-artifact not found: %s", abs)
		}
		return nil, fmt.Errorf("stat discovery-artifact %q: %w", abs, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("discovery-artifact %q is a directory, not a file", abs)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read discovery-artifact %q: %w", abs, err)
	}
	art := parseDiscoveryArtifact(string(raw))
	art.SourcePath = abs
	return art, nil
}

// preloadDiscoveryArtifact loads a caller-provided discovery artifact before
// phased state initialization. If no explicit goal was passed, the artifact goal
// becomes the fallback goal for --from=implementation runs.
func preloadDiscoveryArtifact(path string, args []string) (*discoveryArtifact, []string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, args, nil
	}
	art, err := loadDiscoveryArtifact(path)
	if err != nil {
		return nil, args, err
	}
	if len(args) == 0 && strings.TrimSpace(art.Goal) != "" {
		args = []string{art.Goal}
	}
	return art, args, nil
}

// applyDiscoveryArtifactToPacket rewrites the execution packet from the
// artifact when the run starts at implementation. Discovery-start runs keep the
// normal discovery behavior and only warn that the flag is ignored.
func applyDiscoveryArtifactToPacket(cwd string, art *discoveryArtifact, startPhase int, goal string) error {
	if art == nil {
		return nil
	}
	if startPhase < 2 {
		fmt.Printf("Warning: --discovery-artifact is only honored with --from=implementation; ignoring.\n")
		return nil
	}
	packetPath, err := writeExecutionPacketFromArtifact(cwd, art, goal)
	if err != nil {
		return fmt.Errorf("apply discovery-artifact: %w", err)
	}
	fmt.Printf("Phase 1 (discovery) complete — artifact: %s\n", art.SourcePath)
	VerbosePrintf("Execution packet written from discovery artifact: %s\n", packetPath)
	return nil
}

// parseDiscoveryArtifact parses markdown body (with optional YAML frontmatter)
// into a discoveryArtifact. It is deliberately tolerant: missing sections leave
// their fields empty. This function is pure so tests can exercise it directly.
func parseDiscoveryArtifact(content string) *discoveryArtifact {
	art := &discoveryArtifact{}
	body, fm := splitFrontmatter(content)

	// Extract goal from frontmatter, then from first H1 or first non-empty line.
	if g := frontmatterString(fm, "goal"); g != "" {
		art.Goal = g
	} else {
		art.Goal = extractGoalFromBody(body)
	}
	if g := frontmatterString(fm, "loc_estimate"); g != "" {
		art.LocEstimate = g
	}

	// Scope sections - look for "in scope", "out of scope", "abort", "tdd",
	// "test matrix", "risks". Matching is case-insensitive and tolerates
	// variants like "In-Scope", "Abort Gates", "TDD Matrix".
	art.InScope = extractListSection(body, []string{"in scope", "in-scope", "file manifest", "what ships"})
	art.OutOfScope = extractListSection(body, []string{"out of scope", "out-of-scope", "not shipping"})
	art.AbortGates = extractListSection(body, []string{"abort gates", "abort gate", "abort if", "stop if"})
	art.TDDMatrix = extractListSection(body, []string{"tdd matrix", "test matrix", "tests", "test assertions"})
	art.Risks = extractListSection(body, []string{"risks", "unknowns", "risk matrix"})

	return art
}

// frontmatterRegex captures YAML-style frontmatter delimited by leading '---'.
var frontmatterRegex = regexp.MustCompile(`(?s)\A---\s*\n(.*?)\n---\s*\n?`)

// splitFrontmatter separates optional YAML-style frontmatter from the body.
// Returns (body, frontmatterMap). If no frontmatter is present, returns the
// original content and an empty map.
func splitFrontmatter(content string) (string, map[string]string) {
	fm := map[string]string{}
	m := frontmatterRegex.FindStringSubmatchIndex(content)
	if m == nil {
		return content, fm
	}
	fmText := content[m[2]:m[3]]
	body := content[m[1]:]
	for _, line := range strings.Split(fmText, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if idx := strings.Index(trimmed, ":"); idx > 0 {
			key := strings.TrimSpace(trimmed[:idx])
			val := strings.TrimSpace(trimmed[idx+1:])
			val = strings.Trim(val, `"'`)
			if key != "" {
				fm[key] = val
			}
		}
	}
	return body, fm
}

// frontmatterString returns the string value for key, or "" if absent.
func frontmatterString(fm map[string]string, key string) string {
	return strings.TrimSpace(fm[key])
}

// extractGoalFromBody returns the first H1 title or first non-empty paragraph.
func extractGoalFromBody(body string) string {
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
		// First non-empty, non-heading line as fallback.
		if !strings.HasPrefix(trimmed, "#") {
			return trimmed
		}
	}
	return ""
}

// headingLineRegex matches markdown ATX headings (## Title, ### Title).
var headingLineRegex = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)

// extractListSection scans body for a heading whose normalized text contains
// any of the labels, then harvests the bullet items under that heading until
// the next heading. The scan is case-insensitive and tolerant of punctuation.
func extractListSection(body string, labels []string) []string {
	var out []string
	lines := strings.Split(body, "\n")
	inSection := false
	var sectionLevel int
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		m := headingLineRegex.FindStringSubmatch(trimmed)
		if m != nil {
			level := len(m[1])
			title := strings.ToLower(strings.TrimSpace(m[2]))
			// Stop when we hit a heading at the same or shallower level than the matched section.
			if inSection && level <= sectionLevel {
				inSection = false
			}
			if !inSection && headingMatchesAnyLabel(title, labels) {
				inSection = true
				sectionLevel = level
				continue
			}
			continue
		}
		if !inSection {
			continue
		}
		if item := extractBulletItem(line); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// headingMatchesAnyLabel reports whether title (lowercased) contains any label.
func headingMatchesAnyLabel(title string, labels []string) bool {
	clean := strings.Map(func(r rune) rune {
		if r == ':' || r == '\u2014' || r == '-' {
			return ' '
		}
		return r
	}, title)
	for _, label := range labels {
		if strings.Contains(clean, label) {
			return true
		}
	}
	return false
}

// bulletRegex matches markdown bullet markers: -, *, +, or numbered "1." / "1)".
var bulletRegex = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+(.*)$`)

// extractBulletItem returns the text of a markdown bullet item, stripping
// inline code and trailing whitespace. Returns "" if line is not a bullet.
func extractBulletItem(line string) string {
	m := bulletRegex.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	item := strings.TrimSpace(m[1])
	// Strip leading "**bold:** " labels to keep the semantic value.
	item = strings.TrimSpace(item)
	if item == "" {
		return ""
	}
	return item
}

// writeExecutionPacketFromArtifact materializes .agents/rpi/execution-packet.json
// from the parsed artifact. The packet shape is a canonical-superset: it
// populates the same fields that writeExecutionPacketSeed would and adds the
// discovery-artifact-specific scope/abort/tdd fields so /crank and downstream
// tooling can consume the packet without a second /discovery run.
//
// goalOverride (if non-empty) wins over the artifact's own goal field.
func writeExecutionPacketFromArtifact(cwd string, art *discoveryArtifact, goalOverride string) (string, error) {
	if art == nil {
		return "", fmt.Errorf("nil discovery artifact")
	}
	goal := strings.TrimSpace(goalOverride)
	if goal == "" {
		goal = strings.TrimSpace(art.Goal)
	}

	packetDir := filepath.Join(cwd, ".agents", "rpi")
	if err := os.MkdirAll(packetDir, 0o750); err != nil {
		return "", fmt.Errorf("create %s: %w", packetDir, err)
	}

	// Build done_criteria from abort_gates + tdd_matrix so /crank's existing
	// "done when all criteria met" logic does not require any new code path.
	doneCriteria := append([]string{}, orEmptyStringSlice(art.TDDMatrix)...)
	doneCriteria = append(doneCriteria, orEmptyStringSlice(art.AbortGates)...)

	// ContractSurfaces carries the artifact path first so downstream tools
	// can cite provenance, then any in_scope entries that look like paths.
	contractSurfaces := []string{art.SourcePath}
	for _, entry := range art.InScope {
		if looksLikePath(entry) {
			contractSurfaces = append(contractSurfaces, entry)
		}
	}
	profile, err := loadRepoExecutionProfile(cwd)
	if err != nil {
		return "", err
	}
	generatedAt := time.Now().UTC().Format(time.RFC3339)

	packet := map[string]any{
		// Canonical executionPacket fields (kept in-sync with rpi_execution_packet.go).
		"schema_version": 1,
		"objective":      goal,
		"density": map[string]any{
			"intent": goal,
			"boundary": map[string]any{
				"bounded_context": executionPacketBoundedContext(cwd),
				"non_goals":       orEmptyStringSlice(art.OutOfScope),
				"write_scope":     orEmptyStringSlice(art.InScope),
			},
			"evidence":    append(orEmptyStringSlice(art.TDDMatrix), orEmptyStringSlice(art.AbortGates)...),
			"decision":    "Discovery artifact supplied by operator; implementation starts from the artifact-backed execution packet.",
			"constraint":  orEmptyStringSlice(art.AbortGates),
			"next_action": "/crank .agents/rpi/execution-packet.json",
		},
		"artifacts": map[string]any{
			"research_path":      art.SourcePath,
			"ranked_packet_path": executionPacketRankedPacketPath,
		},
		"contract_surfaces":   contractSurfaces,
		"validation_commands": orEmptyStringSlice(profile.ValidationCommands),
		"validation_lanes":    profile.ValidationLanes,
		"done_criteria":       doneCriteria,
		"tracker_mode":        "discovery-artifact",
		"test_levels": map[string]any{
			"required":    []string{"L0", "L1"},
			"recommended": []string{"L2"},
			"rationale":   "Discovery-artifact handoff preserves the standard autonomous proof floor unless the artifact narrows it with explicit criteria.",
		},
		"ranked_packet_path":  executionPacketRankedPacketPath,
		"discovery_timestamp": generatedAt,

		// Discovery-artifact-mode extensions (documented in
		// skills/rpi/references/discovery-artifact-mode.md).
		"phase":               "implementation",
		"source":              "discovery-artifact",
		"discovery_artifacts": []string{art.SourcePath},
		"scope": map[string]any{
			"in_scope":     orEmptyStringSlice(art.InScope),
			"out_of_scope": orEmptyStringSlice(art.OutOfScope),
			"loc_estimate": art.LocEstimate,
		},
		"abort_gates":  orEmptyStringSlice(art.AbortGates),
		"tdd_matrix":   orEmptyStringSlice(art.TDDMatrix),
		"risks":        orEmptyStringSlice(art.Risks),
		"generated_at": generatedAt,
	}

	packetPath := filepath.Join(packetDir, "execution-packet.json")
	buf, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal execution packet: %w", err)
	}
	buf = append(buf, '\n')
	if err := os.WriteFile(packetPath, buf, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", packetPath, err)
	}
	return packetPath, nil
}

// looksLikePath is a tiny heuristic for picking path-like entries out of an
// in_scope bullet list. It avoids bringing in a full markdown parser.
func looksLikePath(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	// Strip inline code fences.
	s = strings.Trim(s, "`")
	if strings.ContainsAny(s, " \t") {
		// Paths with spaces are rare in this codebase; most bullets with a
		// space are prose, not paths.
		return false
	}
	return strings.Contains(s, "/") || strings.Contains(s, ".")
}

// orEmptyStringSlice returns s if non-nil, else an empty []string. This keeps
// JSON output stable (`[]`) instead of `null` for readability.
func orEmptyStringSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
