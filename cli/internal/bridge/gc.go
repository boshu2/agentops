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
	Name          string `json:"name"`
	QualifiedName string `json:"qualified_name"`
	Running       bool   `json:"running"`
	State         string `json:"state"`
	Template      string `json:"template"`
}

// GCStatusSummary holds aggregate agent counts.
type GCStatusSummary struct {
	Running int `json:"running"`
	Stopped int `json:"stopped"`
	Total   int `json:"total"`
}

// GCSession represents a session from `gc session list --json`.
type GCSession struct {
	ID       string
	Alias    string
	State    string
	Template string
	Closed   bool
}

// UnmarshalJSON accepts both the early lowercase session-list shape and the
// v1.0.0 exported struct shape with capitalized keys.
func (s *GCSession) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID            string `json:"id"`
		Alias         string `json:"alias"`
		State         string `json:"state"`
		Template      string `json:"template"`
		Closed        bool   `json:"closed"`
		UpperID       string `json:"ID"`
		UpperAlias    string `json:"Alias"`
		UpperState    string `json:"State"`
		UpperTemplate string `json:"Template"`
		UpperClosed   bool   `json:"Closed"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.ID = firstNonEmpty(raw.ID, raw.UpperID)
	s.Alias = firstNonEmpty(raw.Alias, raw.UpperAlias)
	s.State = firstNonEmpty(raw.State, raw.UpperState)
	s.Template = firstNonEmpty(raw.Template, raw.UpperTemplate)
	s.Closed = raw.Closed || raw.UpperClosed
	return nil
}

// UnmarshalJSON accepts both the early summary shape and the v1.0.0
// total_agents/running_agents shape.
func (s *GCStatusSummary) UnmarshalJSON(data []byte) error {
	var raw struct {
		Running       int `json:"running"`
		Stopped       int `json:"stopped"`
		Total         int `json:"total"`
		RunningAgents int `json:"running_agents"`
		StoppedAgents int `json:"stopped_agents"`
		TotalAgents   int `json:"total_agents"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.Running = firstNonZero(raw.Running, raw.RunningAgents)
	s.Stopped = firstNonZero(raw.Stopped, raw.StoppedAgents)
	s.Total = firstNonZero(raw.Total, raw.TotalAgents)
	return nil
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
			if missingJSONField(jsonField(entry, field)) {
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

func jsonField(entry map[string]json.RawMessage, lowerName string) json.RawMessage {
	if raw := entry[lowerName]; !missingJSONField(raw) {
		return raw
	}
	titleName := strings.ToUpper(lowerName[:1]) + lowerName[1:]
	return entry[titleName]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

// GCSessionNewArgs returns the command arguments for `gc session new`.
func GCSessionNewArgs(template, alias string) []string {
	args := []string{"session", "new", template}
	if alias != "" {
		args = append(args, "--alias", alias)
	}
	return append(args, "--no-attach")
}

// GCNudgeArgs returns the command arguments for `gc session nudge`.
func GCNudgeArgs(agent, message string) []string {
	return []string{"session", "nudge", agent, "--delivery", "immediate", message}
}

// GCPeekArgs returns the command arguments for `gc session peek`.
func GCPeekArgs(agent string, lines int) []string {
	return []string{"session", "peek", agent, "--lines", strconv.Itoa(lines)}
}

// GCEventEmitArgs returns the command arguments for `gc event emit`.
func GCEventEmitArgs(eventType, dataJSON string) []string {
	return GCEventEmitArgsWithFields(eventType, "", "", "", dataJSON)
}

// GCEventEmitArgsWithFields returns the command arguments for `gc event emit`
// using only non-empty optional fields.
func GCEventEmitArgsWithFields(eventType, actor, subject, message, dataJSON string) []string {
	args := []string{"event", "emit", eventType}
	if actor != "" {
		args = append(args, "--actor", actor)
	}
	if subject != "" {
		args = append(args, "--subject", subject)
	}
	if message != "" {
		args = append(args, "--message", message)
	}
	if dataJSON != "" {
		args = append(args, "--payload", dataJSON)
	}
	return args
}

// GCEventsArgsConfig controls `gc events` fallback argument construction.
type GCEventsArgsConfig struct {
	Type        string
	Since       string
	After       string
	AfterCursor string
	Watch       bool
	Follow      bool
}

// GCEventsArgs returns command arguments for `gc events`.
func GCEventsArgs(cfg GCEventsArgsConfig) []string {
	args := []string{"events"}
	if cfg.Type != "" {
		args = append(args, "--type", cfg.Type)
	}
	if cfg.Since != "" {
		args = append(args, "--since", cfg.Since)
	}
	if cfg.After != "" {
		args = append(args, "--after", cfg.After)
	}
	if cfg.AfterCursor != "" {
		args = append(args, "--after-cursor", cfg.AfterCursor)
	}
	if cfg.Watch {
		args = append(args, "--watch")
	}
	if cfg.Follow {
		args = append(args, "--follow")
	}
	return args
}

// GCSessionListArgs returns command arguments for `gc session list --json`.
func GCSessionListArgs() []string {
	return []string{"session", "list", "--json"}
}

// GCStatusArgs returns the command arguments for `gc status --json`, optionally scoped to a city.
func GCStatusArgs(cityPath string) []string {
	args := []string{"status", "--json"}
	if cityPath != "" {
		return append([]string{"--city", cityPath}, args...)
	}
	return args
}
