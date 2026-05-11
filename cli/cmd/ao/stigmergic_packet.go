// practices: [ai-assisted-dev, team-topologies]
package main

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const defaultStigmergicPacketLimit = 5

type StigmergicTarget struct {
	GoalText   string
	IssueType  string
	Files      []string
	ActiveEpic string
	Repo       string
	Limit      int
}

type StigmergicPacket struct {
	AppliedFindings []string       `json:"applied_findings,omitempty"`
	PlanningRules   []string       `json:"planning_rules,omitempty"`
	KnownRisks      []string       `json:"known_risks,omitempty"`
	PriorFindings   []nextWorkItem `json:"prior_findings,omitempty"`
	Scorecard       stigmergicScorecard
}

type stigmergicScorecard struct {
	PromotedFindings       int `json:"promoted_findings"`
	PlanningRules          int `json:"planning_rules"`
	PreMortemChecks        int `json:"pre_mortem_checks"`
	QueueEntries           int `json:"queue_entries"`
	UnconsumedBatches      int `json:"unconsumed_batches"`
	UnconsumedItems        int `json:"unconsumed_items"`
	HighSeverityUnconsumed int `json:"high_severity_unconsumed"`
}

type stigmergicFindingCandidate struct {
	finding knowledgeFinding
	score   int
}

type stigmergicQueueCandidate struct {
	item      nextWorkItem
	score     int
	severity  int
	affinity  int
	freshness int
	typeRank  int
}

func assembleStigmergicPacket(cwd string, target StigmergicTarget) (StigmergicPacket, error) {
	scorecard, err := loadStigmergicScorecard(cwd)
	if err != nil {
		return StigmergicPacket{}, err
	}

	findings, err := rankStigmergicFindings(cwd, target)
	if err != nil {
		return StigmergicPacket{}, err
	}
	findingIDs := make([]string, 0, len(findings))
	for _, finding := range findings {
		findingIDs = append(findingIDs, finding.ID)
	}

	priorFindings, err := rankPriorFindings(cwd, target)
	if err != nil {
		return StigmergicPacket{}, err
	}

	return StigmergicPacket{
		AppliedFindings: uniqueStringsPreserveOrder(findingIDs),
		PlanningRules:   compiledSummariesForFindings(cwd, "planning-rules", findingIDs),
		KnownRisks:      compiledSummariesForFindings(cwd, "pre-mortem-checks", findingIDs),
		PriorFindings:   priorFindings,
		Scorecard:       scorecard,
	}, nil
}

func loadVisibleNextWorkEntries(cwd, repoFilter string) ([]nextWorkEntry, error) {
	entries, err := readQueueEntries(filepath.Join(cwd, ".agents", "rpi", "next-work.jsonl"))
	if err != nil {
		return nil, err
	}

	visible := make([]nextWorkEntry, 0, len(entries))
	for _, entry := range entries {
		entryVisible := entry
		entryVisible.Items = nil
		for _, item := range entry.Items {
			if !isQueueItemSelectable(item) {
				continue
			}
			if repoFilter != "" && item.TargetRepo != "" && item.TargetRepo != "*" && item.TargetRepo != repoFilter {
				continue
			}
			if classifyNextWorkCompletionProof(cwd, entry.SourceEpic, item).Complete {
				continue
			}
			entryVisible.Items = append(entryVisible.Items, item)
		}
		if len(entryVisible.Items) > 0 {
			visible = append(visible, entryVisible)
		}
	}
	return visible, nil
}

func flattenNextWorkEntries(entries []nextWorkEntry) []nextWorkItem {
	items := make([]nextWorkItem, 0)
	for _, entry := range entries {
		items = append(items, entry.Items...)
	}
	return items
}

func loadStigmergicScorecard(cwd string) (stigmergicScorecard, error) {
	scorecard := stigmergicScorecard{
		PromotedFindings: countMatchingFiles(filepath.Join(cwd, ".agents", SectionFindings), "*.md"),
		PlanningRules:    countMatchingFiles(filepath.Join(cwd, ".agents", "planning-rules"), "*.md"),
		PreMortemChecks:  countMatchingFiles(filepath.Join(cwd, ".agents", "pre-mortem-checks"), "*.md"),
	}

	entries, err := loadVisibleNextWorkEntries(cwd, "")
	if err != nil {
		return stigmergicScorecard{}, err
	}
	for _, entry := range entries {
		scorecard.QueueEntries++
		scorecard.UnconsumedBatches++
		for _, item := range entry.Items {
			if !isQueueItemSelectable(item) {
				continue
			}
			scorecard.UnconsumedItems++
			if strings.EqualFold(strings.TrimSpace(item.Severity), "high") {
				scorecard.HighSeverityUnconsumed++
			}
		}
	}
	return scorecard, nil
}

func rankStigmergicFindings(cwd string, target StigmergicTarget) ([]knowledgeFinding, error) {
	findingsDir := filepath.Join(cwd, ".agents", SectionFindings)
	if _, err := os.Stat(findingsDir); os.IsNotExist(err) {
		findingsDir = findAgentsSubdir(cwd, SectionFindings)
	}

	files, err := filepath.Glob(filepath.Join(findingsDir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("glob findings: %w", err)
	}

	now := time.Now()
	needles := stigmergicNeedles(target)
	candidates := make([]stigmergicFindingCandidate, 0, len(files))
	for _, file := range files {
		finding, err := parseFindingFile(file)
		if err != nil {
			continue
		}
		applyFindingFreshness(&finding, file, now)
		if !findingStatusActiveForRetrieval(finding.Status) {
			continue
		}
		score := scoreFindingCandidate(finding, needles, target)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, stigmergicFindingCandidate{finding: finding, score: score})
	}

	slices.SortFunc(candidates, func(a, b stigmergicFindingCandidate) int {
		if diff := cmp.Compare(b.score, a.score); diff != 0 {
			return diff
		}
		if diff := cmp.Compare(b.finding.CompositeScore, a.finding.CompositeScore); diff != 0 {
			return diff
		}
		return cmp.Compare(a.finding.ID, b.finding.ID)
	})

	limit := target.packetLimit()
	ranked := make([]knowledgeFinding, 0, stigmergicMinInt(limit, len(candidates)))
	for _, candidate := range candidates[:stigmergicMinInt(limit, len(candidates))] {
		ranked = append(ranked, candidate.finding)
	}
	return ranked, nil
}

func rankPriorFindings(cwd string, target StigmergicTarget) ([]nextWorkItem, error) {
	entries, err := loadVisibleNextWorkEntries(cwd, target.Repo)
	if err != nil {
		return nil, err
	}

	needles := stigmergicNeedles(target)
	candidates := make([]stigmergicQueueCandidate, 0)
	for _, entry := range entries {
		for _, item := range entry.Items {
			affinity := repoAffinityRank(item, target.Repo)
			score := scoreQueueCandidate(item, needles, target, affinity)
			if score <= 0 {
				continue
			}
			candidates = append(candidates, stigmergicQueueCandidate{
				item:      item,
				score:     score,
				severity:  severityRank(item.Severity),
				affinity:  affinity,
				freshness: freshnessRank(item),
				typeRank:  workTypeRank(item),
			})
		}
	}

	slices.SortFunc(candidates, func(a, b stigmergicQueueCandidate) int {
		if diff := cmp.Compare(b.score, a.score); diff != 0 {
			return diff
		}
		if diff := cmp.Compare(b.affinity, a.affinity); diff != 0 {
			return diff
		}
		if diff := cmp.Compare(b.severity, a.severity); diff != 0 {
			return diff
		}
		if diff := cmp.Compare(b.freshness, a.freshness); diff != 0 {
			return diff
		}
		if diff := cmp.Compare(b.typeRank, a.typeRank); diff != 0 {
			return diff
		}
		return cmp.Compare(a.item.Title, b.item.Title)
	})

	limit := target.packetLimit()
	ranked := make([]nextWorkItem, 0, stigmergicMinInt(limit, len(candidates)))
	for _, candidate := range candidates[:stigmergicMinInt(limit, len(candidates))] {
		ranked = append(ranked, candidate.item)
	}
	return ranked, nil
}

func scoreFindingCandidate(finding knowledgeFinding, needles []string, target StigmergicTarget) int {
	score := overlapScore(strings.Join([]string{
		finding.ID,
		finding.Title,
		finding.Summary,
		finding.SourceSkill,
		strings.Join(finding.ScopeTags, " "),
		strings.Join(finding.ApplicableWhen, " "),
		strings.Join(finding.ApplicableLanguages, " "),
		strings.Join(finding.CompilerTargets, " "),
	}, " "), needles)
	score += changedFileOverlapScore(target.Files, finding.Title, finding.Summary, strings.Join(finding.ScopeTags, " "), strings.Join(finding.CompilerTargets, " "))

	if target.IssueType != "" && stringSliceContainsFold(finding.ApplicableWhen, target.IssueType) {
		score += 4
	}
	if target.ActiveEpic != "" && strings.Contains(strings.ToLower(finding.Summary), strings.ToLower(target.ActiveEpic)) {
		score += 2
	}
	if score > 0 && strings.EqualFold(strings.TrimSpace(finding.Status), "active") {
		score++
	}
	return score
}

func scoreQueueCandidate(item nextWorkItem, needles []string, target StigmergicTarget, affinity int) int {
	score := overlapScore(strings.Join([]string{
		item.Title,
		item.Type,
		item.Severity,
		item.Source,
		item.Description,
		item.Evidence,
		item.TargetRepo,
	}, " "), needles)
	score += changedFileOverlapScore(target.Files, item.Title, item.Description, item.Evidence)
	score += affinity * 3
	score += severityRank(item.Severity)
	if target.IssueType != "" && strings.EqualFold(strings.TrimSpace(item.Type), target.IssueType) {
		score += 2
	}
	return score
}

func changedFileOverlapScore(files []string, fields ...string) int {
	if len(files) == 0 {
		return 0
	}

	haystack := strings.ToLower(strings.Join(fields, " "))
	score := 0
	for _, file := range files {
		for _, token := range tokenizeStigmergicPath(file) {
			if strings.Contains(haystack, token) {
				score += 2
			}
		}
	}
	return score
}

func overlapScore(haystack string, needles []string) int {
	lower := strings.ToLower(haystack)
	score := 0
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			score++
		}
	}
	return score
}

func stigmergicNeedles(target StigmergicTarget) []string {
	tokens := tokenizeStigmergicText(target.GoalText)
	tokens = append(tokens, tokenizeStigmergicText(target.IssueType)...)
	tokens = append(tokens, tokenizeStigmergicText(target.ActiveEpic)...)
	for _, file := range target.Files {
		tokens = append(tokens, tokenizeStigmergicPath(file)...)
	}
	return uniqueStringsPreserveOrder(tokens)
}

func tokenizeStigmergicText(input string) []string {
	replacer := strings.NewReplacer(
		"/", " ",
		"_", " ",
		"-", " ",
		".", " ",
		",", " ",
		":", " ",
		";", " ",
		"(", " ",
		")", " ",
	)
	clean := strings.ToLower(replacer.Replace(input))
	parts := strings.Fields(clean)
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) < 3 {
			continue
		}
		switch part {
		case "and", "for", "the", "with", "into", "from", "that", "this":
			continue
		default:
			tokens = append(tokens, part)
		}
	}
	return tokens
}

func tokenizeStigmergicPath(path string) []string {
	clean := strings.ReplaceAll(filepath.ToSlash(path), "/", " ")
	return tokenizeStigmergicText(clean)
}

func stringSliceContainsFold(items []string, needle string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(needle)) {
			return true
		}
	}
	return false
}

func countMatchingFiles(dir, pattern string) int {
	files, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return 0
	}
	return len(files)
}

func (target StigmergicTarget) packetLimit() int {
	if target.Limit > 0 {
		return target.Limit
	}
	return defaultStigmergicPacketLimit
}

func stigmergicMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
