package beads

import (
	"strings"
	"testing"
)

func TestAcceptanceRepositoryRejectsBDSelection(t *testing.T) {
	tracker := NewTrackerWith(
		func() (string, error) { return t.TempDir(), nil },
		func() []string { return []string{"AGENTOPS_TRACKER=bd"} },
		func(name string) (string, error) { return "/fake/" + name, nil },
	)
	_, err := NewAcceptanceRepository(tracker).ShowAcceptance([]string{"age-x"})
	if err == nil || !strings.Contains(err.Error(), "requires the BR acceptance wire") {
		t.Fatalf("error = %v", err)
	}
}
