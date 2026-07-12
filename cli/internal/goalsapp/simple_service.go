package goalsapp

import (
	"context"

	goalmodel "github.com/boshu2/agentops/cli/internal/goals"
)

// SimpleService owns the direct goals use cases that already expose complete,
// writer-injected option records in the goals domain package.
type SimpleService struct{}

func (SimpleService) Validate(ctx context.Context, options goalmodel.ValidateOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return goalmodel.RunValidate(options)
}

func (SimpleService) History(ctx context.Context, options goalmodel.HistoryOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return goalmodel.RunHistory(options)
}

func (SimpleService) Export(ctx context.Context, options goalmodel.ExportOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return goalmodel.RunExport(options)
}

func (SimpleService) Drift(ctx context.Context, options goalmodel.DriftOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return goalmodel.RunDrift(options)
}

func (SimpleService) Meta(ctx context.Context, options goalmodel.MetaOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return goalmodel.RunMeta(options)
}
