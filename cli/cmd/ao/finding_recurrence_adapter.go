// practices: [hexagonal-architecture, ddd-bounded-context]
package main

import (
	"context"

	"github.com/boshu2/agentops/cli/internal/ports"
)

// productionFindingRecurrenceReducer is the CLI-side adapter that reconciles
// validation findings before the bookkeeper projects producer candidates.
type productionFindingRecurrenceReducer struct{}

func newProductionFindingRecurrenceReducer() *productionFindingRecurrenceReducer {
	return &productionFindingRecurrenceReducer{}
}

func (r *productionFindingRecurrenceReducer) Reduce(ctx context.Context, observations []ports.FindingObservation) ([]ports.ProducerRuleCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return ports.ReduceFindingRecurrence(observations)
}

var _ ports.FindingRecurrenceReducerPort = (*productionFindingRecurrenceReducer)(nil)
