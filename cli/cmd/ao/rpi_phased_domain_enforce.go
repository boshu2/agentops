// practices: [agile-manifesto, dora-metrics, design-by-contract]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// domainEnforcementMode is the runtime-honesty classification of how a domain
// slice's read fence is being held for a given run. The three modes are
// mutually exclusive and the JSON evidence always carries exactly one:
//
//   - enforced:    a PreToolUse Read/Glob/Grep hook is installed AND the
//                  runtime executes the agent locally, so a denied-glob read
//                  is actually intercepted and blocked by the substrate.
//   - audited:     the runtime is hook-capable but no read-observing hook is
//                  installed, so the fence is checked against *visible*
//                  evidence (command outputs, generated-file manifests) only.
//                  This is the F3.3a behaviour.
//   - unavailable: the runtime cannot observe agent file reads at all (e.g.
//                  opaque Gas City sessions), so no enforcement claim is made.
//
// The cardinal rule: never report `enforced` unless a real observation
// substrate exists. A false enforcement claim is worse than an honest
// `unavailable`.
type domainEnforcementMode string

const (
	// domainEnforcementEnforced means a read-observing PreToolUse hook is
	// installed on a local-executor runtime: denied-glob reads hard-fail.
	domainEnforcementEnforced domainEnforcementMode = "enforced"
	// domainEnforcementAudited means visible-evidence scanning only (F3.3a).
	domainEnforcementAudited domainEnforcementMode = "audited"
	// domainEnforcementUnavailable means the runtime cannot observe reads.
	domainEnforcementUnavailable domainEnforcementMode = "unavailable"
)

// readObservingMatchers are the Claude Code tool matchers whose PreToolUse
// hooks can observe a file *read*. A manifest with a PreToolUse group matching
// any of these — and whose hook can block (decision:block / exit 2) — gives the
// substrate a real read-interception surface.
var readObservingMatchers = []string{"Read", "Glob", "Grep"}

// opaqueRuntimeModes are runtime modes whose executor sessions the phased
// orchestrator cannot instrument with PreToolUse hooks. Gas City sessions run
// out-of-process on a remote substrate that does not surface per-tool-call
// telemetry back to `ao`, so reads inside them are unobservable.
var opaqueRuntimeModes = map[string]bool{
	"gc": true,
}

// domainEnforcementDecision is the resolved enforcement posture for a run,
// including the reason so JSON evidence and stdout are self-explanatory.
type domainEnforcementDecision struct {
	// Mode is the resolved enforcement mode (always exactly one of the three).
	Mode domainEnforcementMode
	// Reason is a human-readable explanation of why this mode was chosen.
	Reason string
	// HookSource names the hooks manifest that supplied the read-observing
	// hook, when Mode is enforced. Empty otherwise.
	HookSource string
	// RuntimeMode is the normalized runtime mode the decision was made for.
	RuntimeMode string
}

// hooksManifestForEnforce is a minimal projection of hooks.json sufficient to
// detect a read-observing PreToolUse hook. It deliberately does not depend on
// the full bridge.HooksConfig parse so a malformed unrelated section cannot
// derail enforcement detection.
type hooksManifestForEnforce struct {
	Hooks struct {
		PreToolUse []struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"PreToolUse"`
	} `json:"hooks"`
}

// hooksManifestPathsFn resolves the ordered set of hooks.json locations to
// probe for a read-observing PreToolUse hook. It is a package var so tests can
// pin it to a deterministic fixture set instead of the host's installed hooks
// (the default reaches into the real home directory).
var hooksManifestPathsFn = defaultHooksManifestPaths

// defaultHooksManifestPaths returns the ordered set of hooks.json locations to
// probe, repo manifest first then the installed Claude/AgentOps manifests.
func defaultHooksManifestPaths(cwd string) []string {
	paths := []string{filepath.Join(cwd, "hooks", "hooks.json")}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths,
			filepath.Join(home, ".agentops", "hooks.json"),
			filepath.Join(home, ".claude", "hooks.json"),
		)
	}
	return paths
}

// matcherCoversReadObservation reports whether a PreToolUse matcher string
// covers at least one read-observing tool. Claude Code matchers may be a
// single tool name or a `|`-separated alternation (e.g. "Edit|Write|Bash").
func matcherCoversReadObservation(matcher string) bool {
	for _, tok := range strings.Split(matcher, "|") {
		tok = strings.TrimSpace(tok)
		for _, m := range readObservingMatchers {
			if tok == m {
				return true
			}
		}
	}
	return false
}

// manifestHasReadObservingHook reports whether a parsed manifest declares at
// least one PreToolUse group that matches a read-observing tool and carries a
// runnable command hook.
func manifestHasReadObservingHook(m *hooksManifestForEnforce) bool {
	for _, group := range m.Hooks.PreToolUse {
		if !matcherCoversReadObservation(group.Matcher) {
			continue
		}
		for _, h := range group.Hooks {
			if h.Type == "command" && strings.TrimSpace(h.Command) != "" {
				return true
			}
		}
	}
	return false
}

// detectReadObservingHook scans the candidate hooks manifests and returns the
// path of the first manifest that declares a read-observing PreToolUse hook.
// Returns "" when no such hook is installed anywhere. Malformed or missing
// manifests are skipped silently — absence of a hook is a valid (audited)
// state, not an error.
func detectReadObservingHook(cwd string) string {
	for _, path := range hooksManifestPathsFn(cwd) {
		raw, err := os.ReadFile(path) //nolint:gosec // path is a fixed manifest location
		if err != nil {
			continue
		}
		var m hooksManifestForEnforce
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if manifestHasReadObservingHook(&m) {
			return path
		}
	}
	return ""
}

// resolveDomainEnforcement decides the enforcement posture for a domain-scoped
// run. The decision is honest by construction:
//
//   - An opaque runtime (Gas City) is always `unavailable`: there is no
//     substrate to observe reads, full stop.
//   - A hook-capable runtime with a read-observing PreToolUse hook installed
//     is `enforced`: a denied-glob read is intercepted and blocked.
//   - A hook-capable runtime with no read-observing hook is `audited`: the
//     fence is checked against visible evidence only (F3.3a behaviour).
func resolveDomainEnforcement(cwd string, state *phasedState) domainEnforcementDecision {
	runtimeMode := normalizeRuntimeMode(state.Opts.RuntimeMode)
	if opaqueRuntimeModes[runtimeMode] {
		return domainEnforcementDecision{
			Mode: domainEnforcementUnavailable,
			Reason: fmt.Sprintf(
				"runtime %q runs agent sessions out-of-process; the orchestrator cannot "+
					"observe their file reads, so the read fence cannot be hard-enforced",
				runtimeMode),
			RuntimeMode: runtimeMode,
		}
	}
	hookSource := detectReadObservingHook(cwd)
	if hookSource == "" {
		return domainEnforcementDecision{
			Mode: domainEnforcementAudited,
			Reason: "no PreToolUse Read/Glob/Grep hook installed; the read fence is " +
				"checked against visible evidence only (command outputs, generated-file manifests)",
			RuntimeMode: runtimeMode,
		}
	}
	return domainEnforcementDecision{
		Mode: domainEnforcementEnforced,
		Reason: fmt.Sprintf(
			"runtime %q executes agents locally and a read-observing PreToolUse hook "+
				"is installed; denied-glob reads/writes are intercepted and blocked",
			runtimeMode),
		HookSource:  hookSource,
		RuntimeMode: runtimeMode,
	}
}

// gateFailedFromEnforcement reports whether the slice gate must hard-fail given
// the resolved enforcement mode and the out-of-domain references found in
// visible evidence.
//
// Under `enforced`, a denied-glob reference that surfaced in visible evidence
// means the read fence was crossed despite a live interception hook (the
// reference reached an output/artifact), so the gate fails hard. Under
// `audited` and `unavailable` the gate never hard-fails on the fence — those
// modes make no enforcement claim — it only reports.
func gateFailedFromEnforcement(mode domainEnforcementMode, refs []outOfDomainRef) bool {
	if mode != domainEnforcementEnforced {
		return false
	}
	for _, ref := range refs {
		if ref.Reason == "matched denied glob" {
			return true
		}
	}
	return false
}
