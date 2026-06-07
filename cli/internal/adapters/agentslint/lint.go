// practices: [wiki-knowledge-surface, design-by-contract]
package agentslint

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Options configures the .agents write-surface lint script adapter.
type Options struct {
	Script string
	JSON   bool
	Stdout io.Writer
	Stderr io.Writer
}

// Error is returned when the underlying lint script exits non-zero. The
// ExitCode field carries the script's exit code so callers can map it onto the
// host process exit status.
type Error struct {
	ExitCode int
	Script   string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s exited with code %d", e.Script, e.ExitCode)
}

// Run executes the configured lint script and preserves its stdout, stderr,
// and exit code contract.
func Run(opts Options) error {
	if _, err := os.Stat(opts.Script); err != nil {
		return fmt.Errorf("lint script not found at %s: %w", opts.Script, err)
	}

	cmdArgs := []string{}
	if opts.JSON {
		cmdArgs = append(cmdArgs, "--json")
	}
	c := exec.Command("bash", append([]string{opts.Script}, cmdArgs...)...) // #nosec G204 -- operator-configured repo script path.
	c.Stdout = opts.Stdout
	c.Stderr = opts.Stderr

	err := c.Run()
	if err == nil {
		return nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return &Error{ExitCode: ee.ExitCode(), Script: opts.Script}
	}
	return fmt.Errorf("running %s: %w", opts.Script, err)
}
