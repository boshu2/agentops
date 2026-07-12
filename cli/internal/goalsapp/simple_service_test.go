package goalsapp

import (
	"context"
	"errors"
	"testing"

	goalmodel "github.com/boshu2/agentops/cli/internal/goals"
)

func TestSimpleServiceHonorsCancelledContextBeforeEffects(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := SimpleService{}
	tests := []struct {
		name string
		run  func() error
	}{
		{"validate", func() error { return service.Validate(ctx, goalmodel.ValidateOptions{}) }},
		{"history", func() error { return service.History(ctx, goalmodel.HistoryOptions{}) }},
		{"export", func() error { return service.Export(ctx, goalmodel.ExportOptions{}) }},
		{"drift", func() error { return service.Drift(ctx, goalmodel.DriftOptions{}) }},
		{"meta", func() error { return service.Meta(ctx, goalmodel.MetaOptions{}) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
		})
	}
}
