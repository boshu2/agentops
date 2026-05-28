// practices: [event-sourcing-cqrs, distributed-tracing]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// bridgeHandoffToLearnings writes decision entries from a handoff artifact
// into .agents/learnings/ as individual learning files with YAML frontmatter.
func bridgeHandoffToLearnings(cwd string, artifact *handoffArtifact) error {
	learningsDir := filepath.Join(cwd, ".agents", "learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		return fmt.Errorf("create learnings dir: %w", err)
	}
	date := time.Now().Format("2006-01-02")
	allResearchSources := []string{
		artifact.Goal,
		artifact.Summary,
		artifact.Continuation,
	}
	allResearchSources = append(allResearchSources, artifact.ArtifactsProduced...)
	allResearchSources = append(allResearchSources, artifact.DecisionsMade...)
	researchSources := renderResearchSourcesFrontmatter(gatherResearchSources(allResearchSources...))

	for i, decision := range artifact.DecisionsMade {
		slug := slugify(decision)
		if len(slug) > 40 {
			slug = slug[:40]
		}
		filename := fmt.Sprintf("%s-handoff-%s-%d.md", date, slug, i)
		content := fmt.Sprintf("---\ntype: learning\nsource: handoff-bridge\ndate: %s\nconfidence: medium\nsession_type: %s\nmaturity: provisional\nutility: 0.5\n%s---\n\n# Decision: %s\n\nCaptured from handoff artifact %s.\nGoal: %s\n", date, artifact.Type, researchSources, decision, artifact.ID, artifact.Goal)
		if err := os.WriteFile(filepath.Join(learningsDir, filename), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write learning %s: %w", filename, err)
		}
	}
	return nil
}
