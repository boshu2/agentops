package gcmaintainer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// checkServiceBinary verifies, on macOS, that the Gas City supervisor
// LaunchAgent resolves to the same executable as the selected gc binary.
// Absent LaunchAgent means nothing to check; AGENTOPS_GC_SKIP_SERVICE_CHECK=1
// skips it (used by tests and non-service installs).
func (o *ops) checkServiceBinary() error {
	if os.Getenv("AGENTOPS_GC_SKIP_SERVICE_CHECK") == "1" || runtime.GOOS != "darwin" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("HOME is required to inspect the supervisor LaunchAgent: %w", err)
	}
	plist := filepath.Join(home, "Library", "LaunchAgents", "com.gascity.supervisor.plist")
	if !isRegularFile(plist) {
		return nil
	}
	out, err := exec.Command("/usr/libexec/PlistBuddy", "-c", "Print :ProgramArguments:0", plist).Output()
	program := strings.TrimSpace(string(out))
	if err != nil || program == "" {
		return fmt.Errorf("gc supervisor LaunchAgent has no program path")
	}
	if !isExecutableFile(program) {
		return fmt.Errorf("gc supervisor LaunchAgent points to a missing binary: %s", program)
	}
	resolved, err := canonical(program)
	if err != nil {
		return fmt.Errorf("gc supervisor LaunchAgent points to a missing binary: %s", program)
	}
	if resolved != o.gcBin {
		return fmt.Errorf("gc supervisor LaunchAgent binary differs from --gc-bin: %s", resolved)
	}
	return nil
}

type gcSession struct {
	Name        string `json:"name"`
	SessionName string `json:"session_name"`
	Alias       string `json:"alias"`
	State       string `json:"state"`
}

type gcSessionList struct {
	Sessions []gcSession `json:"sessions"`
}

// checkGCHealth requires a clean gc doctor and surfaces observability
// disagreements (partial status, roster-vs-session liveness) as warnings only.
func (o *ops) checkGCHealth() error {
	var doctor struct {
		OK             bool `json:"ok"`
		BlockingFailed int  `json:"blocking_failed"`
		Failed         int  `json:"failed"`
		Results        []struct {
			Status string `json:"status"`
		} `json:"results"`
	}
	if err := o.gcJSON(&doctor, "gc doctor failed", "--city", o.city, "doctor", "--json"); err != nil {
		return err
	}
	if !doctor.OK || doctor.BlockingFailed != 0 || doctor.Failed != 0 {
		return fmt.Errorf("gc doctor reports failures")
	}
	warnings := 0
	for _, result := range doctor.Results {
		if result.Status == "warning" {
			warnings++
		}
	}
	if warnings > 0 {
		fmt.Fprintf(o.stderr, "warning: gc doctor reports %d upstream/config warning(s)\n", warnings)
	}

	var status struct {
		Partial bool `json:"partial"`
		Health  struct {
			Signals []string `json:"signals"`
		} `json:"health"`
	}
	if err := o.gcJSON(&status, "gc status failed", "--city", o.city, "status", "--json"); err != nil {
		return err
	}
	var sessions gcSessionList
	if err := o.gcJSON(&sessions, "gc session list failed", "--city", o.city, "session", "list", "--json"); err != nil {
		return err
	}
	if status.Partial {
		fmt.Fprintln(o.stderr, "warning: gc status returned a partial snapshot; use session, pane, bead, and Doctor evidence")
	}
	if slices.Contains(status.Health.Signals, "no_agents_running") && anySessionLive(sessions, "active", "creating") {
		fmt.Fprintln(o.stderr, "warning: gc status says no agents while session state has a live session; roster is authoritative for liveness only")
	}
	return nil
}

func anySessionLive(sessions gcSessionList, states ...string) bool {
	for _, session := range sessions.Sessions {
		if slices.Contains(states, session.State) {
			return true
		}
	}
	return false
}
