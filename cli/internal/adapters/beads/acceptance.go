package beads

import (
	"context"
	"fmt"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
)

type AcceptanceRepository struct {
	tracker *Tracker
}

func NewAcceptanceRepository(tracker *Tracker) AcceptanceRepository {
	return AcceptanceRepository{tracker: tracker}
}

func (repository AcceptanceRepository) ShowAcceptance(ids []string) ([]byte, error) {
	if repository.tracker == nil {
		return nil, fmt.Errorf("beads acceptance tracker is not configured")
	}
	resolved, err := repository.tracker.Resolve()
	if err != nil {
		return nil, err
	}
	if resolved.Tracker != beadsapp.TrackerBR {
		return nil, fmt.Errorf("verify-acceptance requires the BR acceptance wire; selected tracker is %s", resolved.Tracker)
	}
	args := append([]string{"show", "--format", "json"}, ids...)
	return repository.tracker.Output(context.Background(), args...)
}

var _ beadsapp.AcceptanceRepository = AcceptanceRepository{}
