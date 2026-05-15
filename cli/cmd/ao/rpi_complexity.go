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

// classifyComplexity analyzes a goal description and returns the appropriate ComplexityLevel.
func classifyComplexity(goal string) ComplexityLevel {
	return rpi.ClassifyComplexity(goal)
}

// scoreGoal computes a complexityScore from the goal string using whole-word matching.
func scoreGoal(goal string) complexityScore {
	return rpi.ScoreGoal(goal)
}
