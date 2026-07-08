// Package epicstatus implements a deterministic group-terminality predicate:
// given the members of an epic/wave, it decides whether the group is actually
// "done" (terminal) without trusting agent self-report.
//
// This is the membrane's "no verdict = not done" applied at group granularity.
// It steals the three production-paid guards from gas city's convoy model — a
// design-steal, NOT a dependency:
//
//  1. an unresolved/missing member resolves to an unknown-status placeholder
//     that NEVER counts as done;
//  2. a group with a deliberately-open descendant (a human-gate/checkpoint
//     bead) is NOT complete;
//  3. a zero-descendant, still-materializing group is skipped, not done.
//
// The predicate is pure: no I/O, no clock, no process exec. Callers resolve the
// ledger and build the member set; this package renders the verdict.
package epicstatus

import (
	"fmt"
	"sort"
	"strings"
)

// Verdict is the terminal-ness classification of a group (epic/wave).
type Verdict string

const (
	// Terminal means every live descendant is present and closed — done.
	Terminal Verdict = "terminal"
	// NotTerminal means at least one descendant blocks completion.
	NotTerminal Verdict = "not-terminal"
	// Skipped means the group has zero live descendants — still materializing,
	// so it is deliberately NOT reported as done.
	Skipped Verdict = "skipped"
)

// Machine-readable reason codes. Exactly one is set on a Result.
const (
	// ReasonAllTerminal — happy path: all live descendants closed.
	ReasonAllTerminal = "all-terminal"
	// ReasonNoDescendants — guard 3: zero live descendants (materializing).
	ReasonNoDescendants = "no-descendants"
	// ReasonUnknownMember — guard 1: a member could not be resolved.
	ReasonUnknownMember = "unknown-member"
	// ReasonOpenCheckpoint — guard 2: a deliberately-open human-gate/checkpoint
	// descendant is still open.
	ReasonOpenCheckpoint = "open-checkpoint"
	// ReasonOpenMember — a plain (non-checkpoint) descendant is still open.
	ReasonOpenMember = "open-member"
)

// UnknownStatus is the placeholder status reported for a member that could not
// be resolved in the ledger (guard 1). It never counts as done.
const UnknownStatus = "unknown-status"

// Member is one resolved-or-unresolved descendant of a group. The caller builds
// these from the ledger; Present=false marks a missing/unresolved member.
type Member struct {
	// ID is the bead id of the descendant.
	ID string
	// Present is false when the member id could not be resolved to a ledger
	// record (a dangling reference) — it becomes an unknown-status placeholder.
	Present bool
	// Status is the raw ledger status (only meaningful when Present).
	Status string
	// IssueType is the raw ledger issue_type (used to classify checkpoints).
	IssueType string
	// Labels are the raw ledger labels (used to classify checkpoints).
	Labels []string
}

// MemberRoll is the per-member roll-up in a Result.
type MemberRoll struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Present bool   `json:"present"`
	Done    bool   `json:"done"`
	// Class is one of: "done", ReasonUnknownMember, ReasonOpenCheckpoint,
	// ReasonOpenMember.
	Class string `json:"class"`
}

// Result is the deterministic terminality verdict for a group.
type Result struct {
	Group    string       `json:"group"`
	Verdict  Verdict      `json:"verdict"`
	Terminal bool         `json:"terminal"`
	Code     string       `json:"code"`
	Reason   string       `json:"reason"`
	Total    int          `json:"total"`
	Done     int          `json:"done"`
	Blocking int          `json:"blocking"`
	Members  []MemberRoll `json:"members"`
	Blockers []MemberRoll `json:"blockers"`
}

// terminalStatuses are the ledger statuses that count as "done".
var terminalStatuses = map[string]bool{
	"closed": true, "done": true, "resolved": true, "completed": true,
}

// deletedStatuses mark a bead as removed — such a member is excluded from the
// live descendant set entirely (neither counted nor a blocker).
var deletedStatuses = map[string]bool{
	"tombstone": true, "deleted": true,
}

// checkpointStatuses / Types / Labels mark a deliberately-open (human-gate)
// descendant — guard 2. Any of these signals classifies an open member as a
// checkpoint.
var checkpointStatuses = map[string]bool{"deferred": true}

var checkpointTypes = map[string]bool{"checkpoint": true, "gate": true}

var checkpointLabels = map[string]bool{
	"checkpoint": true, "human-gate": true, "gate": true,
	"manual-gate": true, "needs-human": true, "human-review": true,
}

func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// isCheckpoint reports whether a member is a deliberately-open human-gate /
// checkpoint bead, inferred from its status, issue_type, or labels.
func isCheckpoint(m Member) bool {
	if checkpointStatuses[norm(m.Status)] {
		return true
	}
	if checkpointTypes[norm(m.IssueType)] {
		return true
	}
	for _, l := range m.Labels {
		if checkpointLabels[norm(l)] {
			return true
		}
	}
	return false
}

// classify turns one raw member into its roll-up (present/missing, done, and
// blocking class). Deleted members return ok=false and are dropped by the
// caller.
func classify(m Member) (roll MemberRoll, ok bool) {
	if !m.Present {
		return MemberRoll{ID: m.ID, Status: UnknownStatus, Present: false, Done: false, Class: ReasonUnknownMember}, true
	}
	st := norm(m.Status)
	if deletedStatuses[st] {
		return MemberRoll{}, false // not a live member
	}
	if terminalStatuses[st] {
		return MemberRoll{ID: m.ID, Status: m.Status, Present: true, Done: true, Class: "done"}, true
	}
	class := ReasonOpenMember
	if isCheckpoint(m) {
		class = ReasonOpenCheckpoint
	}
	return MemberRoll{ID: m.ID, Status: m.Status, Present: true, Done: false, Class: class}, true
}

// topLineCode picks the machine-readable reason code for a set of blockers by
// precedence: an unresolvable member outranks an open checkpoint, which
// outranks a plain open member.
func topLineCode(blockers []MemberRoll) string {
	seen := map[string]bool{}
	for _, b := range blockers {
		seen[b.Class] = true
	}
	switch {
	case seen[ReasonUnknownMember]:
		return ReasonUnknownMember
	case seen[ReasonOpenCheckpoint]:
		return ReasonOpenCheckpoint
	default:
		return ReasonOpenMember
	}
}

func reasonFor(group string, code string, total, done, blocking int) string {
	switch code {
	case ReasonNoDescendants:
		return fmt.Sprintf("%s has zero live descendants — still materializing; skipped, not done", group)
	case ReasonAllTerminal:
		return fmt.Sprintf("%s terminal: all %d descendant(s) closed", group, total)
	case ReasonUnknownMember:
		return fmt.Sprintf("%s not done: %d/%d descendant(s) done, %d blocking (an unresolved/missing member counts as unknown-status)", group, done, total, blocking)
	case ReasonOpenCheckpoint:
		return fmt.Sprintf("%s not done: %d/%d descendant(s) done, %d blocking (a deliberately-open checkpoint/human-gate descendant)", group, done, total, blocking)
	default:
		return fmt.Sprintf("%s not done: %d/%d descendant(s) done, %d blocking (an open descendant)", group, done, total, blocking)
	}
}

// Evaluate applies the three group-terminality guards to a group's members and
// returns a deterministic verdict. Pure: no I/O, no clock.
func Evaluate(group string, members []Member) Result {
	rolls := make([]MemberRoll, 0, len(members))
	for _, m := range members {
		if roll, ok := classify(m); ok {
			rolls = append(rolls, roll)
		}
	}
	sort.SliceStable(rolls, func(i, j int) bool { return rolls[i].ID < rolls[j].ID })

	var blockers []MemberRoll
	done := 0
	for _, r := range rolls {
		if r.Done {
			done++
		} else {
			blockers = append(blockers, r)
		}
	}

	res := Result{
		Group:    group,
		Total:    len(rolls),
		Done:     done,
		Blocking: len(blockers),
		Members:  rolls,
		Blockers: blockers,
	}

	switch {
	case len(rolls) == 0:
		// Guard 3: zero live descendants — still materializing.
		res.Verdict = Skipped
		res.Terminal = false
		res.Code = ReasonNoDescendants
	case len(blockers) == 0:
		// Happy path: every live descendant closed.
		res.Verdict = Terminal
		res.Terminal = true
		res.Code = ReasonAllTerminal
	default:
		// Guards 1 & 2 (and plain open members): at least one blocker.
		res.Verdict = NotTerminal
		res.Terminal = false
		res.Code = topLineCode(blockers)
	}
	res.Reason = reasonFor(group, res.Code, res.Total, res.Done, res.Blocking)
	return res
}
