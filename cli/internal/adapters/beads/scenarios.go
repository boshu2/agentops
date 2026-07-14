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
	return repository.FetchScenarioBeadContext(context.Background(), id)
}

func (repository ScenarioRepository) FetchScenarioBeadContext(ctx context.Context, id string) (beadsapp.FetchedBead, error) {
	raw, err := repository.tracker.Output(ctx, "show", id, "--json")
	if err != nil {
		return beadsapp.FetchedBead{}, fmt.Errorf("bd show %s --json: %w", id, err)
	}
	return beadsapp.ParseScenarioBead(raw)
}

func (repository ScenarioRepository) UpdateDescription(id, description string) error {
	return repository.UpdateDescriptionContext(context.Background(), id, description)
}

func (repository ScenarioRepository) UpdateDescriptionContext(ctx context.Context, id, description string) error {
	_, err := repository.tracker.Output(ctx, "update", id, "--description", description)
	return err
}

var _ beadsapp.ScenarioRepository = ScenarioRepository{}
var _ beadsapp.ScenarioContextRepository = ScenarioRepository{}
