package goalsapp

import (
	"context"
	"errors"
	"testing"

	goalmodel "github.com/boshu2/agentops/cli/internal/goals"
)

func TestManagementServiceHonorsCancelledContextBeforeEffects(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := ManagementService{}
	tests := []struct {
		name string
		run  func() error
	}{
		{"add", func() error { return service.Add(ctx, goalmodel.AddOptions{}) }},
		{"init", func() error { return service.Init(ctx, goalmodel.InitOptions{}) }},
		{"migrate", func() error { return service.Migrate(ctx, goalmodel.MigrateOptions{}) }},
		{"prune", func() error { return service.Prune(ctx, goalmodel.PruneOptions{}) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
		})
	}
}
