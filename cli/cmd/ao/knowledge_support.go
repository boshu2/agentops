package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/boshu2/agentops/cli/internal/bridge"
	"github.com/boshu2/agentops/cli/internal/forge"
	"github.com/boshu2/agentops/cli/internal/pool"
	"github.com/boshu2/agentops/cli/internal/search"
	"github.com/boshu2/agentops/cli/internal/types"
	"github.com/boshu2/agentops/cli/internal/types/quest"
)

const (
	InjectCharsPerToken  = search.InjectCharsPerToken
	MaxLearningsToInject = 10
	MaxPatternsToInject  = 5
	SectionLearnings     = "learnings"
	SectionFindings      = "findings"
	SectionPatterns      = "patterns"
	SectionResearch      = "research"
	SectionSessions      = "sessions"
)

var injectApplyDecay bool

func contextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), timeout)
}

func findAgentsSubdir(startDir, subdir string) string {
	return search.FindAgentsSubdir(startDir, subdir)
}

func compactText(input string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
}

func truncateText(input string, maxLength int) string {
	return search.TruncateText(input, maxLength)
}

func trimJSONToCharBudget(knowledge *injectedKnowledge, budget int) string {
	return search.TrimJSONToCharBudget(knowledge, budget)
}

func trimToCharBudget(value string, budget int) string {
	return search.TrimToCharBudget(value, budget)
}

func extractFrontmatter(content string) (string, error) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", nil
	}
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			return strings.Join(lines[1:index], "\n"), nil
		}
	}
	return "", nil
}

func estimateTokens(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	n := len(text) / InjectCharsPerToken
	if n < 1 {
		return 1
	}
	return n
}

func slugify(value string) string { return pool.Slugify(value) }

func shortHash7(value string) string {
	if len(value) > 7 {
		return value[:7]
	}
	return value
}

const highConfidenceCitationThreshold = 0.7

func citationConfidenceScore(citationType string) float64 {
	switch canonicalCitationType(citationType) {
	case types.CitationTypeHelpful:
		return 1.0
	case types.CitationTypeUsedInFinalArtifact, types.CitationTypeApplied:
		return 0.9
	case types.CitationTypeReference:
		return 0.7
	case types.CitationTypeRetrieved:
		return 0.5
	default:
		return 0
	}
}

func citationEventIsHighConfidence(citation types.CitationEvent) bool {
	if canonicalCitationType(citation.CitationType) == types.CitationTypeHarmful ||
		canonicalCitationType(citation.CitationType) == types.CitationTypeRefuted {
		return false
	}
	if citation.MatchConfidence > 0 {
		return normalizeCitationMatchConfidence(citation.MatchConfidence) >= highConfidenceCitationThreshold
	}
	return citationConfidenceScore(citation.CitationType) >= highConfidenceCitationThreshold
}

func gitChangedFiles(cwd string, limit int) []string {
	out := runCommand(cwd, 1200*time.Millisecond, "git", "diff", "--name-only", "HEAD")
	if strings.TrimSpace(out) == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if limit > 0 && len(lines) > limit {
		lines = lines[:limit]
	}
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}

func runCommand(cwd string, timeout time.Duration, name string, args ...string) string {
	ctx, cancel := contextWithTimeout(timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func firstNonEmptyTrimmed(values ...string) string {
	return bridge.FirstNonEmptyTrimmed(values...)
}

func splitMarkdownSections(content string) []string {
	return forge.SplitMarkdownSections(content)
}

func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	return quest.AtomicWriteFileWithPerm(path, data, perm)
}

func gatherResearchSources(texts ...string) []string {
	seen := make(map[string]bool)
	var refs []string
	for _, value := range texts {
		for _, ref := range extractResearchRefsFromText(value) {
			if !seen[ref] {
				seen[ref] = true
				refs = append(refs, ref)
			}
		}
	}
	sort.Strings(refs)
	return refs
}

func renderResearchSourcesFrontmatter(refs []string) string {
	if len(refs) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("research_sources:\n")
	for _, ref := range refs {
		fmt.Fprintf(&builder, "  - %q\n", ref)
	}
	return builder.String()
}
