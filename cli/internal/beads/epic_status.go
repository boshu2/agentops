package beads

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/boshu2/agentops/cli/internal/epicstatus"
)

type LedgerReader interface {
	ReadFile(string) ([]byte, error)
}

// LedgerBead is the subset of an issues.jsonl record needed to derive epic
// membership.
type LedgerBead struct {
	ID           string      `json:"id"`
	Status       string      `json:"status"`
	IssueType    string      `json:"issue_type"`
	Labels       []string    `json:"labels"`
	Dependencies []LedgerDep `json:"dependencies"`
}

type LedgerDep struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
}

// ParseLedger parses newline-delimited issues and fails closed on any malformed
// non-blank line.
func ParseLedger(raw []byte) ([]LedgerBead, error) {
	var records []LedgerBead
	for index, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var record LedgerBead
		if err := json.Unmarshal([]byte(trimmed), &record); err != nil {
			return nil, fmt.Errorf("line %d: %w", index+1, err)
		}
		records = append(records, record)
	}
	return records, nil
}

// BuildMembers derives the deterministic union of prefix children and
// parent-child edges, including unresolved in-family references.
func BuildMembers(epic string, records []LedgerBead) (members []epicstatus.Member, epicPresent bool) {
	prefix := epic + "."
	byID := make(map[string]LedgerBead, len(records))
	for _, record := range records {
		byID[record.ID] = record
		if record.ID == epic {
			epicPresent = true
		}
	}
	if !epicPresent {
		return nil, false
	}

	memberIDs := map[string]bool{}
	for _, record := range records {
		if record.ID == epic {
			continue
		}
		if strings.HasPrefix(record.ID, prefix) || hasParentChildEdgeTo(record, epic) {
			memberIDs[record.ID] = true
		}
	}

	missing := map[string]bool{}
	scan := func(record LedgerBead) {
		for _, dependency := range record.Dependencies {
			for _, reference := range []string{dependency.DependsOnID, dependency.IssueID} {
				if reference == "" || reference == epic || memberIDs[reference] {
					continue
				}
				if strings.HasPrefix(reference, prefix) {
					if _, exists := byID[reference]; !exists {
						missing[reference] = true
					}
				}
			}
		}
	}
	scan(byID[epic])
	for id := range memberIDs {
		scan(byID[id])
	}

	for id := range memberIDs {
		record := byID[id]
		members = append(members, epicstatus.Member{
			ID:        record.ID,
			Present:   true,
			Status:    record.Status,
			IssueType: record.IssueType,
			Labels:    record.Labels,
		})
	}
	for id := range missing {
		members = append(members, epicstatus.Member{ID: id, Present: false})
	}
	sort.SliceStable(members, func(left, right int) bool { return members[left].ID < members[right].ID })
	return members, true
}

func hasParentChildEdgeTo(record LedgerBead, parentID string) bool {
	for _, dependency := range record.Dependencies {
		if dependency.Type == "parent-child" && dependency.DependsOnID == parentID {
			return true
		}
	}
	return false
}
