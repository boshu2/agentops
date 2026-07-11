package beads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

type ExecStreams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type TrackerExecutor interface {
	Execute(context.Context, []string, ExecStreams) error
}

type ExitError struct {
	Code int
}

func (err *ExitError) Error() string { return "" }
func (err *ExitError) ExitCode() int { return err.Code }

func ChildEnvironment(base []string, resolution TrackerResolution) []string {
	child := make([]string, 0, len(base)+1)
	for _, entry := range base {
		if !strings.HasPrefix(entry, "BEADS_DIR=") {
			child = append(child, entry)
		}
	}
	if resolution.Tracker == TrackerBR {
		child = append(child, "BEADS_DIR="+resolution.LedgerDir)
	}
	return child
}

func ChildDirectory(resolution TrackerResolution, callerDirectory string) string {
	if resolution.Tracker == TrackerBD {
		return filepath.Dir(resolution.LedgerDir)
	}
	return callerDirectory
}

func IsReadVerb(verb string) bool {
	switch verb {
	case "list", "ready", "show":
		return true
	default:
		return false
	}
}

func ArgsHaveJSONFlag(args []string) bool {
	for _, argument := range args {
		if argument == "--json" {
			return true
		}
	}
	return false
}

type brShowDependent struct {
	ID             string `json:"id"`
	DependencyType string `json:"dependency_type"`
}

type brShowIssue struct {
	Dependents []brShowDependent `json:"dependents"`
}

func BRChildren(raw []byte) ([]string, error) {
	var issues []brShowIssue
	if err := json.Unmarshal(raw, &issues); err != nil {
		return nil, err
	}
	var children []string
	for _, issue := range issues {
		for _, dependent := range issue.Dependents {
			if dependent.DependencyType == "parent-child" {
				children = append(children, dependent.ID)
			}
		}
	}
	return children, nil
}

var canonicalIssueKeys = []string{"id", "title", "description", "priority", "status"}

func CanonicalizeBDReadJSON(verb string, raw []byte) ([]byte, error) {
	elements, err := decodeIssueArray(raw)
	if err != nil {
		return nil, err
	}
	if elements == nil {
		elements = []map[string]json.RawMessage{}
	}
	isShow := verb == "show"
	for _, element := range elements {
		ensureCanonicalIssueKeys(element, isShow)
	}
	if verb == "list" {
		return json.Marshal(map[string]any{"issues": elements})
	}
	return json.Marshal(elements)
}

func decodeIssueArray(raw []byte) ([]map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty bd --json payload")
	}
	switch trimmed[0] {
	case '[':
		var array []map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &array); err != nil {
			return nil, err
		}
		return array, nil
	case '{':
		var envelope struct {
			Issues []map[string]json.RawMessage `json:"issues"`
		}
		if err := json.Unmarshal(trimmed, &envelope); err != nil {
			return nil, err
		}
		return envelope.Issues, nil
	default:
		return nil, fmt.Errorf("unexpected bd --json payload (not an array or object): %.32q", trimmed)
	}
}

func ensureCanonicalIssueKeys(element map[string]json.RawMessage, isShow bool) {
	if element == nil {
		return
	}
	for _, key := range canonicalIssueKeys {
		if _, exists := element[key]; !exists {
			element[key] = json.RawMessage("null")
		}
	}
	if isShow {
		if _, exists := element["dependents"]; !exists {
			element["dependents"] = json.RawMessage("[]")
		}
	}
}
