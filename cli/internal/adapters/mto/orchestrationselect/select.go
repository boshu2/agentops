// practices: [hexagonal-architecture, safe-degradation]
package orchestrationselect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/boshu2/agentops/cli/internal/orchestration"
	"github.com/boshu2/agentops/cli/internal/ports"
)

// execCommandRunner is the production CommandRunner adapter: it shells out via
// os/exec so ProbeNTM actually invokes `ntm --robot-capabilities`.
type execCommandRunner struct{}

// Run executes name with args and returns the combined output. A non-zero exit
// or missing binary surfaces as the canonical degradation signal.
func (execCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

var _ orchestration.CommandRunner = execCommandRunner{}

// WorkSpecFromFlags maps command flag values onto the orchestration port shape.
func WorkSpecFromFlags(pin string, optOut bool) ports.WorkSpec {
	return ports.WorkSpec{
		OptOut: optOut,
		Pin:    ports.Backend(strings.TrimSpace(pin)),
	}
}

// Select resolves the orchestration backend through the production selector.
func Select(ctx context.Context, pin string, optOut bool) (ports.SelectionTrace, error) {
	selector := orchestration.NewSelector(execCommandRunner{})
	return selector.Select(ctx, WorkSpecFromFlags(pin, optOut))
}

// RenderSelectionTrace writes a SelectionTrace as JSON or as the stable human
// summary used by `ao orchestrate select`.
func RenderSelectionTrace(out io.Writer, trace ports.SelectionTrace, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(trace)
	}

	fmt.Fprintf(out, "Backend: %s\n", trace.Chosen)
	fmt.Fprintf(out, "Reason:  %s\n", trace.Reason)
	considered := make([]string, 0, len(trace.Considered))
	for _, b := range trace.Considered {
		considered = append(considered, string(b))
	}
	fmt.Fprintf(out, "Ladder:  %s\n", strings.Join(considered, " -> "))
	return nil
}
