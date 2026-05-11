// practices: [agile-manifesto, dora-metrics]
package main

import (
	"github.com/boshu2/agentops/cli/internal/rpi"
)

// ComplexityLevel classifies the ceremony complexity of an RPI goal.
// It determines how many gates and council validations are required.
type ComplexityLevel = rpi.ComplexityLevel

const (
	ComplexityFast     = rpi.ComplexityFast
	ComplexityStandard = rpi.ComplexityStandard
	ComplexityFull     = rpi.ComplexityFull
)

// complexityScore holds intermediate scoring data used to classify a goal.
type complexityScore = rpi.ComplexityScore

// complexScopeKeywords are words that suggest the goal spans multiple files or systems.
var complexScopeKeywords = rpi.ComplexScopeKeywords

// complexOperationKeywords are verbs or nouns that indicate significant engineering work.
var complexOperationKeywords = rpi.ComplexOperationKeywords

// simpleOperationKeywords are verbs that indicate small, focused changes.
var simpleOperationKeywords = rpi.SimpleOperationKeywords

// containsWholeWord returns true if text contains kw as a whole word (word-boundary match).
func containsWholeWord(text, kw string) bool {
	return rpi.ContainsWholeWord(text, kw)
}

// classifyComplexity analyzes a goal description and returns the appropriate ComplexityLevel.
func classifyComplexity(goal string) ComplexityLevel {
	return rpi.ClassifyComplexity(goal)
}

// scoreGoal computes a complexityScore from the goal string using whole-word matching.
func scoreGoal(goal string) complexityScore {
	return rpi.ScoreGoal(goal)
}

// levelFromScore converts a complexityScore into a ComplexityLevel.
func levelFromScore(s complexityScore) ComplexityLevel {
	return rpi.LevelFromScore(s)
}
