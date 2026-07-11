package beads

import (
	"strings"
	"testing"
)

type acceptanceRepositoryFunc func([]string) ([]byte, error)

func (function acceptanceRepositoryFunc) ShowAcceptance(ids []string) ([]byte, error) {
	return function(ids)
}

func TestAcceptanceServiceRejectsPartialTrackerResponse(t *testing.T) {
	service := AcceptanceService{Repository: acceptanceRepositoryFunc(func([]string) ([]byte, error) {
		return []byte(`[{"id":"age-a","issue_type":"task","description":"## Acceptance\ncomplete"}]`), nil
	})}
	_, _, err := service.VerifyAcceptance([]string{"age-a", "age-missing"})
	if err == nil || !strings.Contains(err.Error(), "age-missing") {
		t.Fatalf("error = %v", err)
	}
}
