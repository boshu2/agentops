// practices: [wiki-knowledge-surface, pragmatic-programmer]
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type knowledgeContextPlaybook struct {
	TopicID string
	Title   string
	Path    string
	Summary string
}

func knowledgeAgentsRoot(cwd string) string {
	root := findGitRoot(cwd)
	if root == "" {
		root = cwd
	}
	return filepath.Join(root, ".agents")
}

func loadKnowledgeBeliefsForContext(cwd, query string, limit int) []string {
	agentsRoot := knowledgeAgentsRoot(cwd)
	beliefBookPath := filepath.Join(agentsRoot, "knowledge", "book-of-beliefs.md")

	var beliefs []string
	if data, err := os.ReadFile(beliefBookPath); err == nil {
		text := string(data)
		beliefs = append(beliefs, extractKnowledgeListItems(text, "## Core Beliefs")...)
		beliefs = append(beliefs, extractKnowledgeListItems(text, "## Operating Principles")...)
	}

	if len(beliefs) == 0 {
		topics := loadKnowledgeTopicDetails(agentsRoot)
		if len(topics) == 0 {
			return nil
		}
		coreBeliefs, operatingPrinciples := collectKnowledgeBeliefSections(topics, agentsRoot)
		beliefs = append(beliefs, coreBeliefs...)
		beliefs = append(beliefs, operatingPrinciples...)
	}

	return rankKnowledgeContextLines(query, dedupeKnowledgeStrings(beliefs), limit)
}

func loadKnowledgePlaybooksForContext(cwd, query string, limit int) []knowledgeContextPlaybook {
	agentsRoot := knowledgeAgentsRoot(cwd)
	topics := loadKnowledgeTopicDetails(agentsRoot)
	if len(topics) == 0 {
		return nil
	}

	candidateLimit := limit
	if candidateLimit < 4 {
		candidateLimit = 4
	}
	selected := selectRelevantKnowledgeTopics(query, topics, agentsRoot, candidateLimit)
	playbooks := make([]knowledgeContextPlaybook, 0, limit)
	for _, topic := range selected {
		if limit > 0 && len(playbooks) >= limit {
			break
		}
		if topic.Health != "healthy" {
			continue
		}

		promotedPath := filepath.Join(agentsRoot, "packets", "promoted", topic.ID+".md")
		playbookPath := filepath.Join(agentsRoot, "playbooks", topic.ID+".md")
		if !knowledgePathExists(promotedPath) || !knowledgePathExists(playbookPath) {
			continue
		}

		data, err := os.ReadFile(playbookPath)
		if err != nil {
			continue
		}
		text := string(data)
		playbooks = append(playbooks, knowledgeContextPlaybook{
			TopicID: topic.ID,
			Title:   topic.Title,
			Path:    playbookPath,
			Summary: firstNonEmptyTrimmed(
				knowledgeSectionText(text, "## When To Use"),
				knowledgeSectionText(text, "## Summary"),
				topic.Summary,
			),
		})
	}
	return playbooks
}

func extractKnowledgeListItems(text, heading string) []string {
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
		switch {
		case strings.HasPrefix(trimmed, "- "):
			items = append(items, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		case knowledgeOrderedItem(trimmed) != "":
			items = append(items, knowledgeOrderedItem(trimmed))
		}
	}
	return dedupeKnowledgeStrings(items)
}

func knowledgeOrderedItem(line string) string {
	dot := strings.Index(line, ". ")
	if dot <= 0 {
		return ""
	}
	for _, r := range line[:dot] {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return strings.TrimSpace(line[dot+2:])
}

func rankKnowledgeContextLines(query string, items []string, limit int) []string {
	if len(items) == 0 {
		return nil
	}

	queryTokens := knowledgeTokens(query)
	type scoredLine struct {
		text  string
		score int
		index int
	}

	scored := make([]scoredLine, 0, len(items))
	hasScore := false
	for idx, item := range items {
		score := 0
		lower := strings.ToLower(item)
		for _, token := range queryTokens {
			if strings.Contains(lower, token) {
				score += 2
			}
		}
		if score > 0 {
			hasScore = true
		}
		scored = append(scored, scoredLine{text: item, score: score, index: idx})
	}

	if hasScore {
		sort.SliceStable(scored, func(i, j int) bool {
			if scored[i].score != scored[j].score {
				return scored[i].score > scored[j].score
			}
			return scored[i].index < scored[j].index
		})
	}

	ranked := make([]string, 0, len(scored))
	for _, item := range scored {
		trimmed := strings.Join(strings.Fields(strings.TrimSpace(item.text)), " ")
		if trimmed == "" {
			continue
		}
		ranked = append(ranked, trimmed)
		if limit > 0 && len(ranked) >= limit {
			break
		}
	}
	return ranked
}

func displayKnowledgeContextPath(cwd, path string) string {
	if path == "" {
		return ""
	}
	if rel, err := filepath.Rel(cwd, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	root := findGitRoot(cwd)
	if root != "" {
		if rel, err := filepath.Rel(root, path); err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}
	return path
}
