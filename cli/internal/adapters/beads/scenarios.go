package beads

import (
	"context"
	"fmt"

	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
)

type ScenarioRepository struct {
	tracker *Tracker
}

func NewScenarioRepository(tracker *Tracker) ScenarioRepository {
	return ScenarioRepository{tracker: tracker}
}

func (repository ScenarioRepository) Available() bool {
	return repository.tracker != nil && repository.tracker.Available()
}

func (repository ScenarioRepository) FetchScenarioBead(id string) (beadsapp.FetchedBead, error) {
	raw, err := repository.tracker.Output(context.Background(), "show", id, "--json")
	if err != nil {
		return beadsapp.FetchedBead{}, fmt.Errorf("bd show %s --json: %w", id, err)
	}
	return beadsapp.ParseScenarioBead(raw)
}

func (repository ScenarioRepository) UpdateDescription(id, description string) error {
	_, err := repository.tracker.Output(context.Background(), "update", id, "--description", description)
	return err
}

var _ beadsapp.ScenarioRepository = ScenarioRepository{}
