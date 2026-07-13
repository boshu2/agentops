package main

import (
	"os"

	beadsadapter "github.com/boshu2/agentops/cli/internal/adapters/beads"
)

func currentBeadsTracker() *beadsadapter.Tracker {
	return beadsadapter.NewTrackerWith(os.Getwd, os.Environ, func(name string) (string, error) {
		return trackerLookPath(name)
	})
}
