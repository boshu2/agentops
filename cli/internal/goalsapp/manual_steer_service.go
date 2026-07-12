package goalsapp

import (
	"context"

	goalmodel "github.com/boshu2/agentops/cli/internal/goals"
)

// ManualSteerService owns operator-directed directive mutations.
type ManualSteerService struct{}

func (ManualSteerService) Add(ctx context.Context, options goalmodel.SteerAddOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return goalmodel.RunSteerAdd(options)
}

func (ManualSteerService) Remove(ctx context.Context, options goalmodel.SteerRemoveOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return goalmodel.RunSteerRemove(options)
}

func (ManualSteerService) Prioritize(ctx context.Context, options goalmodel.SteerPrioritizeOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return goalmodel.RunSteerPrioritize(options)
}
