package bridge

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// GCMinVersion is the minimum gc version required for bridge compatibility.
const GCMinVersion = "0.13.0"

// GCStatus represents the parsed output of `gc status --json`.
type GCStatus struct {
	City       string          `json:"city"`
	Controller GCController    `json:"controller"`
	Agents     []GCAgentInfo   `json:"agents"`
	Summary    GCStatusSummary `json:"summary"`
}

// GCController represents the controller state within GCStatus.
type GCController struct {
	Running bool   `json:"running"`
	PID     int    `json:"pid"`
	Mode    string `json:"mode"`
}

// GCAgentInfo represents a single agent entry within GCStatus.
type GCAgentInfo struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	Template string `json:"template"`
}

// GCStatusSummary holds aggregate agent counts.
type GCStatusSummary struct {
	Running int `json:"running"`
	Stopped int `json:"stopped"`
	Total   int `json:"total"`
}

// GCSession represents a session from `gc session list --json`.
type GCSession struct {
	ID       string `json:"id"`
	Alias    string `json:"alias"`
	State    string `json:"state"`
	Template string `json:"template"`
}

// ParseSemverParts extracts major, minor, patch integers from a version string.
func ParseSemverParts(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		num := strings.SplitN(parts[i], "-", 2)[0]
		result[i], _ = strconv.Atoi(num)
	}
	return result
}

// CompareSemver returns -1, 0, or 1 comparing two semver strings.
func CompareSemver(a, b string) int {
	aParts := ParseSemverParts(a)
	bParts := ParseSemverParts(b)
	for i := 0; i < 3; i++ {
		if aParts[i] < bParts[i] {
			return -1
		}
		if aParts[i] > bParts[i] {
			return 1
		}
	}
	return 0
}

// GCBridgeCompatible checks if the given version meets the minimum requirement.
func GCBridgeCompatible(version string) bool {
	return CompareSemver(version, GCMinVersion) >= 0
}

// ParseGCStatus parses the JSON output of `gc status --json`.
func ParseGCStatus(data []byte) (GCStatus, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return GCStatus{}, fmt.Errorf("parse gc status: %w", err)
	}
	for _, field := range []string{"controller", "agents", "summary"} {
		if missingJSONField(raw[field]) {
			return GCStatus{}, fmt.Errorf("parse gc status: missing required field %q", field)
		}
	}

	var status GCStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return GCStatus{}, fmt.Errorf("parse gc status: %w", err)
	}
	return status, nil
}

// ParseGCSessions parses the JSON output of `gc session list --json`.
func ParseGCSessions(data []byte) ([]GCSession, error) {
	var raw []map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse gc sessions: %w", err)
	}
	for i, entry := range raw {
		for _, field := range []string{"alias", "state"} {
			if missingJSONField(entry[field]) {
				return nil, fmt.Errorf("parse gc sessions: entry %d missing required field %q", i, field)
			}
		}
	}

	var sessions []GCSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("parse gc sessions: %w", err)
	}
	return sessions, nil
}

func missingJSONField(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null"
}

// GCNudgeArgs returns the command arguments for `gc session nudge`.
func GCNudgeArgs(agent, message string) []string {
	return []string{"session", "nudge", agent, message}
}

// GCPeekArgs returns the command arguments for `gc session peek`.
func GCPeekArgs(agent string, lines int) []string {
	return []string{"session", "peek", agent, "--lines", strconv.Itoa(lines)}
}

// GCEventEmitArgs returns the command arguments for `gc event emit`.
func GCEventEmitArgs(eventType, dataJSON string) []string {
	return []string{"event", "emit", eventType, "--payload", dataJSON}
}

// GCStatusArgs returns the command arguments for `gc status --json`, optionally scoped to a city.
func GCStatusArgs(cityPath string) []string {
	args := []string{"status", "--json"}
	if cityPath != "" {
		return append([]string{"--city", cityPath}, args...)
	}
	return args
}
