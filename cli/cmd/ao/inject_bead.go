package main

import (
	"strings"

	"github.com/boshu2/agentops/cli/internal/search"
)

// Constants — canonical values live in internal/search.
const (
	BeadScoreMatchDirect   = search.BeadScoreMatchDirect
	BeadScoreMatchLabel    = search.BeadScoreMatchLabel
	BeadScoreMatchCategory = search.BeadScoreMatchCategory
	beadContextCacheFile   = search.BeadContextCacheFile
)

// Type alias — canonical type lives in internal/search.
type BeadContext = search.BeadContext

// Thin wrappers — delegate to search package, kept for test compatibility.
func resolveBeadContext(beadID, cwd string) *BeadContext {
	return search.ResolveBeadContext(beadID, cwd)
}
func readBeadCache(cwd, beadID string) *BeadContext { return search.ReadBeadCache(cwd, beadID) }
func splitLabels(s string) []string                 { return search.SplitLabels(s) }
func buildKeywords(ctx *BeadContext) []string       { return search.BuildKeywords(ctx) }

// applyBeadBoost adjusts a learning's composite score based on bead context.
// Stays in cmd/ao because it depends on the learning type.
func applyBeadBoost(l *learning, bead *BeadContext) {
	if bead == nil {
		return
	}

	if l.SourceBead != "" && l.SourceBead == bead.ID {
		l.CompositeScore *= BeadScoreMatchDirect
		return
	}

	if len(bead.Labels) > 0 {
		titleLower := strings.ToLower(l.Title + " " + l.Summary)
		for _, label := range bead.Labels {
			if strings.Contains(titleLower, strings.ToLower(label)) {
				l.CompositeScore *= BeadScoreMatchLabel
				break
			}
		}
	}

	if len(bead.Keywords) > 0 {
		titleLower := strings.ToLower(l.Title + " " + l.Summary)
		for _, kw := range bead.Keywords {
			if strings.Contains(titleLower, kw) {
				l.CompositeScore *= BeadScoreMatchCategory
				break
			}
		}
	}
}
