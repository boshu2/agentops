// practices: [hexagonal-architecture, capability-detection]
package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// NTMHardDeps are the robot-command capabilities an AgentOps NTM
// background-agent swarm cannot run without.
//
// A missing entry means the host cannot drive background Claude/Codex skill
// sessions even if the ntm binary is on PATH, which is exactly why detection is
// capability-based rather than `command -v`. `mail` is load-bearing: it is the
// NTM robot surface backed by mcp-agent-mail for assignment, reservation,
// check-in, and handoff coordination.
var NTMHardDeps = []string{
	"activity",
	"dashboard",
	"mail",
	"send",
	"spawn",
}

// NTMCapabilities is the result of probing the NTM runtime. Available
// reports whether ntm is present and responded to the capabilities
// query. Capabilities is the set of capability tokens ntm reported.
// MissingDeps lists any NTMHardDeps not covered by the reported
// capabilities; it is empty when every hard dep is satisfied and is
// always nil/empty when Available is false (an absent runtime has no
// meaningful per-dep breakdown).
type NTMCapabilities struct {
	Available    bool
	Capabilities []string
	MissingDeps  []string
}

// CommandRunner abstracts running an external command so probes can be
// tested against an in-memory fake. Run executes name with args and
// returns the combined output, or an error if the command could not be
// started or exited non-zero. A non-nil error is the canonical signal
// that the tool is absent or unusable.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// robotCapabilitiesPayload is the JSON shape ntm emits for
// `ntm --robot-capabilities`. Only the fields the probe consumes are
// declared; unknown fields are ignored by encoding/json.
type robotCapabilitiesPayload struct {
	Capabilities []string `json:"capabilities"`
	Commands     []struct {
		Name string `json:"name"`
	} `json:"commands"`
}

// ProbeNTM detects the NTM runtime by capability. It invokes
// `ntm --robot-capabilities` via runner and parses the result.
//
// Degradation contract: if ntm is absent — signalled by runner
// returning an error — ProbeNTM returns NTMCapabilities{Available:
// false} and a nil error. Absence is a normal degradation signal, not
// a failure, so callers can branch on Available without handling an
// error path for the common "ntm not installed" case. A hard error is
// returned only when ntm IS present but its output cannot be parsed,
// since that is a genuine contract violation worth surfacing.
func ProbeNTM(ctx context.Context, runner CommandRunner) (NTMCapabilities, error) {
	out, err := runner.Run(ctx, "ntm", "--robot-capabilities")
	if err != nil {
		// ntm is absent or unusable: degrade gracefully.
		return NTMCapabilities{Available: false}, nil
	}

	var payload robotCapabilitiesPayload
	if err := json.Unmarshal(out, &payload); err != nil {
		return NTMCapabilities{}, fmt.Errorf("parsing ntm --robot-capabilities output: %w", err)
	}

	caps := normalizeCapabilities(payload.Capabilities)
	if len(caps) == 0 && len(payload.Commands) > 0 {
		names := make([]string, 0, len(payload.Commands))
		for _, cmd := range payload.Commands {
			names = append(names, cmd.Name)
		}
		caps = normalizeCapabilities(names)
	}
	return NTMCapabilities{
		Available:    true,
		Capabilities: caps,
		MissingDeps:  missingHardDeps(caps),
	}, nil
}

// normalizeCapabilities trims, drops blanks, and sorts the reported
// capability tokens so callers get a stable, comparable slice.
func normalizeCapabilities(raw []string) []string {
	caps := make([]string, 0, len(raw))
	for _, c := range raw {
		c = strings.TrimSpace(c)
		if c != "" {
			caps = append(caps, c)
		}
	}
	sort.Strings(caps)
	return caps
}

// missingHardDeps returns the NTMHardDeps not present in caps. The
// result is nil when every hard dep is satisfied.
func missingHardDeps(caps []string) []string {
	present := make(map[string]struct{}, len(caps))
	for _, c := range caps {
		present[c] = struct{}{}
	}

	var missing []string
	for _, dep := range NTMHardDeps {
		if _, ok := present[dep]; !ok {
			missing = append(missing, dep)
		}
	}
	return missing
}
