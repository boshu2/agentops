package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/bridge"
)

// gcExecFn is the type for exec.Command-compatible functions.
type gcExecFn func(name string, arg ...string) *exec.Cmd

// gcLookFn is the type for exec.LookPath-compatible functions.
type gcLookFn func(file string) (string, error)

// gcDefaultExec returns exec.Command if fn is nil.
func gcDefaultExec(fn gcExecFn) gcExecFn {
	if fn != nil {
		return fn
	}
	return exec.Command
}

// gcDefaultLook returns exec.LookPath if fn is nil.
func gcDefaultLook(fn gcLookFn) gcLookFn {
	if fn != nil {
		return fn
	}
	return exec.LookPath
}

// gcMinVersion is the minimum gc version required for bridge compatibility.
const gcMinVersion = bridge.GCMinVersion

// GCStatus is an alias for bridge.GCStatus.
type GCStatus = bridge.GCStatus

// GCController is an alias for bridge.GCController.
type GCController = bridge.GCController

// GCAgentInfo is an alias for bridge.GCAgentInfo.
type GCAgentInfo = bridge.GCAgentInfo

// GCStatusSummary is an alias for bridge.GCStatusSummary.
type GCStatusSummary = bridge.GCStatusSummary

// GCSession is an alias for bridge.GCSession.
type GCSession = bridge.GCSession

// gcBridgeAvailable returns true if the gc binary is on PATH.
func gcBridgeAvailable(lookPath gcLookFn) bool {
	_, err := gcDefaultLook(lookPath)("gc")
	return err == nil
}

// gcBridgeVersion returns the gc version string.
func gcBridgeVersion(execCommand gcExecFn) (string, error) {
	out, err := gcDefaultExec(execCommand)("gc", "version").Output()
	if err != nil {
		return "", fmt.Errorf("gc version: %w", err)
	}
	version := strings.TrimSpace(string(out))
	if version == "" {
		return "", fmt.Errorf("gc version: empty output")
	}
	return version, nil
}

// gcBridgeCompatible checks if the given version meets the minimum requirement.
func gcBridgeCompatible(version string) bool {
	return bridge.GCBridgeCompatible(version)
}

// compareSemver returns -1, 0, or 1 comparing two semver strings.
func compareSemver(a, b string) int {
	return bridge.CompareSemver(a, b)
}

// parseSemverParts extracts major, minor, patch integers from a version string.
func parseSemverParts(v string) [3]int {
	return bridge.ParseSemverParts(v)
}

// gcBridgeReady checks both binary availability AND controller running.
// Returns (ready, reason).
func gcBridgeReady(cityPath string, execCommand gcExecFn, lookPath gcLookFn) (bool, string) {
	if !gcBridgeAvailable(lookPath) {
		return false, "gc binary not found on PATH"
	}
	v, err := gcBridgeVersion(execCommand)
	if err != nil {
		return false, fmt.Sprintf("gc version check failed: %v", err)
	}
	if !gcBridgeCompatible(v) {
		return false, fmt.Sprintf("gc version %s below minimum %s", v, gcMinVersion)
	}
	execFn := gcDefaultExec(execCommand)
	args := bridge.GCStatusArgs(cityPath)
	out, err := execFn("gc", args...).Output()
	if err != nil {
		return false, fmt.Sprintf("gc controller not running: %v", err)
	}
	status, err := parseGCStatus(out)
	if err != nil {
		return false, fmt.Sprintf("gc status parse error: %v", err)
	}
	if !status.Controller.Running {
		return false, "gc controller not running"
	}
	return true, "gc bridge ready"
}

// parseGCStatus parses the JSON output of `gc status --json`.
func parseGCStatus(data []byte) (GCStatus, error) {
	return bridge.ParseGCStatus(data)
}

// parseGCSessions parses the JSON output of `gc session list --json`.
func parseGCSessions(data []byte) ([]GCSession, error) {
	return bridge.ParseGCSessions(data)
}

// gcNudgeArgs returns the command arguments for `gc session nudge`.
func gcNudgeArgs(agent, message string) []string {
	return bridge.GCNudgeArgs(agent, message)
}

// gcPeekArgs returns the command arguments for `gc session peek`.
func gcPeekArgs(agent string, lines int) []string {
	return bridge.GCPeekArgs(agent, lines)
}

// gcEventEmitArgs returns the command arguments for `gc event emit`.
func gcEventEmitArgs(eventType, dataJSON string) []string {
	return bridge.GCEventEmitArgs(eventType, dataJSON)
}

// gcBridgeCityPath walks up from cwd looking for city.toml.
// Returns the directory containing city.toml, or empty string if not found.
func gcBridgeCityPath(cwd string) string {
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "city.toml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
