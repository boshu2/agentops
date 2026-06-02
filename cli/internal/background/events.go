// Package background contains the fixture-backed contracts for AgentOps
// background agents: NTM-supervised Claude/Codex sessions coordinated by
// mcp-agent-mail. It deliberately has no live NTM or bd side effects; callers
// use these pure helpers to normalize events, dedupe replays, summarize bead
// comments, and plan closure commands.
package background

import (
	"fmt"
	"sort"
	"strings"
)

const EventSchemaVersion = 1

type Event struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	Offset        int    `json:"offset"`
	Type          string `json:"type"`
	BeadID        string `json:"bead_id,omitempty"`
	Worker        string `json:"worker,omitempty"`
	Message       string `json:"message,omitempty"`
	ArtifactPath  string `json:"artifact_path,omitempty"`
	Verdict       string `json:"verdict,omitempty"`
}

func (e Event) Validate() error {
	if e.SchemaVersion != EventSchemaVersion {
		return fmt.Errorf("schema_version: want %d, got %d", EventSchemaVersion, e.SchemaVersion)
	}
	if strings.TrimSpace(e.SessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	if e.Offset < 0 {
		return fmt.Errorf("offset must be >= 0")
	}
	if strings.TrimSpace(e.Type) == "" {
		return fmt.Errorf("type is required")
	}
	return nil
}

func EventKey(e Event) string {
	return fmt.Sprintf("%s:%d", e.SessionID, e.Offset)
}

func Dedupe(events []Event, seen map[string]bool) ([]Event, error) {
	if seen == nil {
		seen = map[string]bool{}
	}
	out := make([]Event, 0, len(events))
	for _, e := range events {
		if err := e.Validate(); err != nil {
			return nil, err
		}
		key := EventKey(e)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, e)
	}
	return out, nil
}

func Summarize(events []Event) (string, error) {
	if len(events) == 0 {
		return "background-agent events: none", nil
	}
	workers := map[string]bool{}
	artifacts := map[string]bool{}
	last := events[len(events)-1]
	for _, e := range events {
		if err := e.Validate(); err != nil {
			return "", err
		}
		if e.Worker != "" {
			workers[e.Worker] = true
		}
		if e.ArtifactPath != "" {
			artifacts[e.ArtifactPath] = true
		}
	}
	parts := []string{
		fmt.Sprintf("background-agent session %s mirrored %d event(s)", last.SessionID, len(events)),
		fmt.Sprintf("last=%s", last.Type),
	}
	if len(workers) > 0 {
		parts = append(parts, "workers="+strings.Join(sortedKeys(workers), ","))
	}
	if len(artifacts) > 0 {
		parts = append(parts, "artifacts="+strings.Join(sortedKeys(artifacts), ","))
	}
	return strings.Join(parts, "; "), nil
}

type ClosurePlan struct {
	BeadID    string   `json:"bead_id"`
	Verdict   string   `json:"verdict"`
	Commands  []string `json:"commands"`
	LeaveOpen bool     `json:"leave_open"`
}

func PlanClosure(e Event) (ClosurePlan, error) {
	if err := e.Validate(); err != nil {
		return ClosurePlan{}, err
	}
	if e.Type != "session_end" {
		return ClosurePlan{}, fmt.Errorf("closure requires session_end event, got %q", e.Type)
	}
	if strings.TrimSpace(e.BeadID) == "" {
		return ClosurePlan{}, fmt.Errorf("bead_id is required for closure")
	}
	verdict := strings.ToUpper(strings.TrimSpace(e.Verdict))
	if verdict == "" {
		verdict = "WARN"
	}
	plan := ClosurePlan{BeadID: e.BeadID, Verdict: verdict}
	if verdict == "PASS" {
		plan.Commands = []string{
			"bd update " + e.BeadID + " --set-metadata verdict=PASS",
			"bd comment " + e.BeadID + " -m <background-agent lineage summary>",
			"bd dolt push # only if a real Dolt remote is configured",
		}
		return plan, nil
	}
	plan.LeaveOpen = true
	plan.Commands = []string{
		"bd comment " + e.BeadID + " -m <background-agent failure summary>",
		"bd update " + e.BeadID + " --status open",
	}
	return plan, nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
