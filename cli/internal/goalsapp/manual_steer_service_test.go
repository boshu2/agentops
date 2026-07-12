package goalsapp

import (
	"context"
	"errors"
	"testing"

	goalmodel "github.com/boshu2/agentops/cli/internal/goals"
)

func TestManualSteerServiceHonorsCancelledContextBeforeEffects(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := ManualSteerService{}
	tests := []struct {
		name string
		run  func() error
	}{
		{"add", func() error { return service.Add(ctx, goalmodel.SteerAddOptions{}) }},
		{"remove", func() error { return service.Remove(ctx, goalmodel.SteerRemoveOptions{}) }},
		{"prioritize", func() error { return service.Prioritize(ctx, goalmodel.SteerPrioritizeOptions{}) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
		})
	}
}
