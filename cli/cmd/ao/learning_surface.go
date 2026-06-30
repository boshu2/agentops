// practices: [wiki-knowledge-surface, ai-assisted-dev]
package main

import (
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/search"
)

// Learning/knowledge type aliases + the small helper surface that SURVIVING
// commands (session bootstrap, maturity, lookup, canon, context_*) depend on.
//
// These were relocated here from inject.go / inject_learnings.go (age-oovc,
// memory-moat removal Phase 0 / KEYSTONE) so the redundant retrieval command
// files can be deleted in Phase 1 without breaking the universal init or other
// survivors. Canonical implementations live in internal/search; these are the
// thin package-main aliases/wrappers the rest of cmd/ao references by name.

// Type aliases — canonical definitions live in internal/search/types.go.
type injectedKnowledge = search.InjectedKnowledge
type learning = search.Learning
type pattern = search.Pattern
type knowledgeFinding = search.KnowledgeFinding
type session = search.Session

// canonLearningsDir is the repo-relative path to the team canon tier, derived
// from the canon.go constants (canonDir/canonLearnings) so the two never drift.
var canonLearningsDir = filepath.Join(canonDir, canonLearnings)

// globLearningFiles discovers .md and .jsonl learning files under dir.
func globLearningFiles(dir string) []string {
	return walkKnowledgeFiles(dir, ".md", ".jsonl")
}

// parseLearningFile parses a learning file into a Learning. Thin wrapper over
// the canonical internal/search parser.
func parseLearningFile(path string) (learning, error) { return search.ParseLearningFile(path) }

// applyFreshnessScore applies the freshness-decay score to a learning. Thin
// wrapper over the canonical internal/search scorer.
func applyFreshnessScore(l *learning, file string, now time.Time) {
	search.ApplyFreshnessToLearning(l, file, now)
}
