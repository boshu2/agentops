package main

import (
	"context"
	"encoding/json"
	"os"

	beadsadapter "github.com/boshu2/agentops/cli/internal/adapters/beads"
)

// beadsVerdictError remains only for command families and white-box tests
// that have not yet adopted the shared command exit type.
type beadsVerdictError struct {
	code int
}

func (err *beadsVerdictError) Error() string { return "" }
func (err *beadsVerdictError) ExitCode() int { return err.code }

var beadsTrackerOutput = func(args ...string) ([]byte, error) {
	return currentBeadsTracker().Output(context.Background(), args...)
}

var beadsTrackerAvailable = func() bool { return currentBeadsTracker().Available() }

func currentBeadsTracker() *beadsadapter.Tracker {
	return beadsadapter.NewTrackerWith(os.Getwd, os.Environ, func(name string) (string, error) {
		return trackerLookPath(name)
	})
}

func beadMinInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func emitJSON(file *os.File, value any) error {
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
