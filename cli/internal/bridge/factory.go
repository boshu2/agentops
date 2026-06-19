package bridge

import "fmt"

// FactoryRecommendedCommands returns the recommended next-step commands for the factory lane.
func FactoryRecommendedCommands(goal string) []string {
	if goal == "" {
		return []string{
			"Set a concrete goal, then run `ao factory start --goal \"your goal\"` for a briefing-first startup.",
			"Run `/rpi \"your goal\"` for the skill-first delivery lane, or use NTM/Agent Mail for out-of-session execution.",
			"Use `ao orchestrate status` to inspect orchestration readiness.",
			"Run `ao codex stop` when the session ends so the flywheel closes explicitly.",
		}
	}

	quotedGoal := fmt.Sprintf("%q", goal)
	return []string{
		fmt.Sprintf("Run `/rpi %s` for the skill-first software-factory lane.", quotedGoal),
		"Use NTM/Agent Mail for out-of-session execution when this must outlive the current session.",
		"Use `ao orchestrate status` to inspect orchestration readiness.",
		"Run `ao codex stop` when the session ends so the flywheel closes explicitly.",
	}
}
