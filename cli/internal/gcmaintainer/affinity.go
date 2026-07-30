package gcmaintainer

import (
	"encoding/json"
	"fmt"
)

// affinityBead is one ready formula bead as reported by `gc bd ready --json`.
type affinityBead struct {
	ID       string                     `json:"id"`
	Assignee string                     `json:"assignee"`
	Metadata map[string]json.RawMessage `json:"metadata"`
}

// metaString returns the string value of a metadata key, or "" when the key is
// absent or not a string (matching jq's `// ""` fallback in the shell port).
func (b affinityBead) metaString(key string) string {
	raw, ok := b.Metadata[key]
	if !ok {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return value
}

// recoverAffinity finds ready formula beads whose gc.session_affinity=require
// assignment points at a session that is no longer live, and clears exactly
// those assignees when --apply is set. The default is a read-only dry run; it
// never slings, retries, closes, restarts, or selects work.
func (o *ops) recoverAffinity() error {
	var sessions gcSessionList
	if err := o.gcJSON(&sessions, "cannot list sessions", "--city", o.city, "session", "list", "--json"); err != nil {
		return err
	}
	var ready []affinityBead
	if err := o.gcJSON(&ready, "cannot list ready rig work", "--city", o.city, "--rig", o.rigName, "bd", "ready", "--json"); err != nil {
		return err
	}

	var stale []affinityBead
	for _, bead := range ready {
		if bead.Assignee == "" || bead.metaString("gc.session_affinity") != "require" || bead.metaString("gc.routed_to") == "" {
			continue
		}
		if !assigneeLive(sessions, bead.Assignee) {
			stale = append(stale, bead)
		}
	}
	if len(stale) == 0 {
		fmt.Fprintln(o.stdout, "No stale required session-affinity assignments found.")
		return nil
	}
	for _, bead := range stale {
		if !o.apply {
			fmt.Fprintf(o.stdout, "would clear %s assignee=%s routed_to=%s\n", bead.ID, bead.Assignee, bead.metaString("gc.routed_to"))
			continue
		}
		description := fmt.Sprintf("cannot clear assignee on %s", bead.ID)
		if _, err := o.gcOutput(description, "--city", o.city, "--rig", o.rigName, "bd", "update", bead.ID, "--assignee", ""); err != nil {
			return err
		}
		fmt.Fprintf(o.stdout, "cleared %s assignee=%s routed_to=%s\n", bead.ID, bead.Assignee, bead.metaString("gc.routed_to"))
	}
	if !o.apply {
		fmt.Fprintln(o.stdout, "Dry run only; pass --apply to clear exactly these ready stale assignments.")
	}
	return nil
}

// assigneeLive reports whether any session claims the assignee's identity in a
// live state (active, creating, starting, or waking).
func assigneeLive(sessions gcSessionList, assignee string) bool {
	for _, session := range sessions.Sessions {
		if session.Name != assignee && session.SessionName != assignee && session.Alias != assignee {
			continue
		}
		switch session.State {
		case "active", "creating", "starting", "waking":
			return true
		}
	}
	return false
}
