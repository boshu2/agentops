package goalsapp

import (
	"context"

	goalmodel "github.com/boshu2/agentops/cli/internal/goals"
)

// ManagementService owns the mutating goal-file lifecycle use cases.
type ManagementService struct{}

func (ManagementService) Add(ctx context.Context, options goalmodel.AddOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return goalmodel.RunAdd(ctx, options)
}

func (ManagementService) Init(ctx context.Context, options goalmodel.InitOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return goalmodel.RunInit(options)
}

func (ManagementService) Migrate(ctx context.Context, options goalmodel.MigrateOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return goalmodel.RunMigrate(options)
}

func (ManagementService) Prune(ctx context.Context, options goalmodel.PruneOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return goalmodel.RunPrune(options)
}
