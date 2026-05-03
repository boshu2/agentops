package safety

import (
	"sort"
	"strconv"
	"time"
)

// TeamSandboxRule identifies a specific contract rule from the team-runner
// sandbox contract documented in backend-claude-teams.md.
type TeamSandboxRule string

const (
	RuleTeamBeforeTask  TeamSandboxRule = "team_before_task"
	RulePreAssignTasks  TeamSandboxRule = "pre_assign_tasks"
	RuleLeadOnlyCommits TeamSandboxRule = "lead_only_commits"
	RuleThinMessages    TeamSandboxRule = "thin_messages"
	RuleNewTeamPerWave  TeamSandboxRule = "new_team_per_wave"
	RuleAlwaysCleanup   TeamSandboxRule = "always_cleanup"
	RuleUnknownAction   TeamSandboxRule = "unknown_action"
)

// teamState tracks the lifecycle state of a team.
type teamState int

const (
	teamNeverSeen teamState = iota
	teamActive
	teamDeleted
)

// ContractViolation records a sandbox contract breach.
type ContractViolation struct {
	Rule       TeamSandboxRule
	Detail     string
	AgentID    string
	TeamName   string
	Timestamp  time.Time
	EventIndex int
}

// TeamLifecycleEvent represents a team operation for audit trail analysis.
type TeamLifecycleEvent struct {
	Action    string // "create", "task", "delete"
	TeamName  string
	Timestamp time.Time
	AgentID   string
}

// ValidateMessageSize checks rule 4 (thin messages): messages must be under
// maxTokens. Uses ceiling(chars/4) as a rough token approximation since exact
// tokenization requires a model-specific tokenizer. Non-empty messages always
// estimate at least 1 token.
func ValidateMessageSize(message string, maxTokens int) *ContractViolation {
	if len(message) == 0 {
		return nil
	}
	estimatedTokens := (len(message) + 3) / 4 // ceiling division
	if estimatedTokens > maxTokens {
		return &ContractViolation{
			Rule:   RuleThinMessages,
			Detail: "message exceeds token limit: estimated " + strconv.Itoa(estimatedTokens) + " tokens (max " + strconv.Itoa(maxTokens) + ")",
		}
	}
	return nil
}

// ValidateTeamLifecycle checks rules 1, 5, and 6:
//   - Rule 1: TeamCreate must precede any Task for that team
//   - Rule 5: A second create for the same team requires an intervening delete
//   - Rule 6: Every created team must be deleted
//
// Events are sorted by timestamp before validation. Unknown actions emit a
// violation. Empty team names are rejected.
//
// Note: The current API is batch-oriented (pass all events, get all violations).
// A future streaming integration would need a stateful Observe(event) + Finalize()
// API; the batch approach is correct for offline audit.
// indexedEvent pairs a TeamLifecycleEvent with its original input slice index
// so that EventIndex in violations refers to the caller's position, not the
// post-sort position.
type indexedEvent struct {
	event   TeamLifecycleEvent
	origIdx int
}

// teamMeta tracks lifecycle state and last-seen provenance per team.
type teamMeta struct {
	state     teamState
	agentID   string
	timestamp time.Time
	origIdx   int
}

func ValidateTeamLifecycle(events []TeamLifecycleEvent) []ContractViolation {
	var violations []ContractViolation

	// Wrap events to preserve original index, then sort by timestamp.
	indexed := make([]indexedEvent, len(events))
	for i, ev := range events {
		indexed[i] = indexedEvent{event: ev, origIdx: i}
	}
	sort.SliceStable(indexed, func(i, j int) bool {
		return indexed[i].event.Timestamp.Before(indexed[j].event.Timestamp)
	})

	// Track team lifecycle state with provenance metadata.
	teams := make(map[string]*teamMeta)

	for _, ie := range indexed {
		ev := ie.event

		// Reject empty team names.
		if ev.TeamName == "" {
			violations = append(violations, ContractViolation{
				Rule:       RuleTeamBeforeTask,
				Detail:     "event has empty team name",
				AgentID:    ev.AgentID,
				TeamName:   "",
				Timestamp:  ev.Timestamp,
				EventIndex: ie.origIdx,
			})
			continue
		}

		meta := teams[ev.TeamName]
		if meta == nil {
			meta = &teamMeta{state: teamNeverSeen}
			teams[ev.TeamName] = meta
		}

		switch ev.Action {
		case "create":
			if meta.state == teamActive {
				violations = append(violations, ContractViolation{
					Rule:       RuleNewTeamPerWave,
					Detail:     "team " + ev.TeamName + " created without deleting previous instance",
					AgentID:    ev.AgentID,
					TeamName:   ev.TeamName,
					Timestamp:  ev.Timestamp,
					EventIndex: ie.origIdx,
				})
			}
			meta.state = teamActive
			meta.agentID = ev.AgentID
			meta.timestamp = ev.Timestamp
			meta.origIdx = ie.origIdx

		case "task":
			if meta.state != teamActive {
				violations = append(violations, ContractViolation{
					Rule:       RuleTeamBeforeTask,
					Detail:     "task dispatched to team " + ev.TeamName + " before TeamCreate",
					AgentID:    ev.AgentID,
					TeamName:   ev.TeamName,
					Timestamp:  ev.Timestamp,
					EventIndex: ie.origIdx,
				})
			}
			// Update last-seen provenance.
			meta.agentID = ev.AgentID
			meta.timestamp = ev.Timestamp
			meta.origIdx = ie.origIdx

		case "delete":
			if meta.state != teamActive {
				violations = append(violations, ContractViolation{
					Rule:       RuleAlwaysCleanup,
					Detail:     "team " + ev.TeamName + " deleted without being active",
					AgentID:    ev.AgentID,
					TeamName:   ev.TeamName,
					Timestamp:  ev.Timestamp,
					EventIndex: ie.origIdx,
				})
			}
			meta.state = teamDeleted
			meta.agentID = ev.AgentID
			meta.timestamp = ev.Timestamp
			meta.origIdx = ie.origIdx

		default:
			violations = append(violations, ContractViolation{
				Rule:       RuleUnknownAction,
				Detail:     "unknown action: " + ev.Action,
				AgentID:    ev.AgentID,
				TeamName:   ev.TeamName,
				Timestamp:  ev.Timestamp,
				EventIndex: ie.origIdx,
			})
		}
	}

	// Rule 6: any team still active at the end was not cleaned up.
	// Sort team names for deterministic violation ordering.
	var uncleaned []string
	for name, meta := range teams {
		if meta.state == teamActive {
			uncleaned = append(uncleaned, name)
		}
	}
	sort.Strings(uncleaned)
	for _, name := range uncleaned {
		meta := teams[name]
		violations = append(violations, ContractViolation{
			Rule:       RuleAlwaysCleanup,
			Detail:     "team " + name + " was never deleted",
			TeamName:   name,
			AgentID:    meta.agentID,
			Timestamp:  meta.timestamp,
			EventIndex: meta.origIdx,
		})
	}

	return violations
}
