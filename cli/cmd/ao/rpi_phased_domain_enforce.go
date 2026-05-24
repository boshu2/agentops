// practices: [agile-manifesto, dora-metrics, design-by-contract]
package main

import (
	"fmt"
)

// domainEnforcementMode is the runtime-honesty classification of how a domain
// slice's read fence is being held for a given run. The modes are mutually
// exclusive and the JSON evidence always carries exactly one:
//
//   - audited:     the runtime is observable but no substrate intercepts agent
//     file reads, so the fence is checked against *visible*
//     evidence (command outputs, generated-file manifests) only.
//     This is the default for every observable runtime under the
//     hookless model (AgentOps no longer ships read-observing
//     interception).
//   - unavailable: the runtime cannot observe agent file reads at all (e.g.
//     opaque Gas City sessions), so no enforcement claim is made.
//
// The cardinal rule: never report an enforcement claim stronger than the
// substrate can back. Under the hookless model no runtime hard-intercepts
// reads, so the resolver only ever returns `audited` or `unavailable`.
type domainEnforcementMode string

const (
	// domainEnforcementEnforced is retained for backward-compatible JSON
	// readers/tests; the hookless resolver never returns it (no substrate
	// intercepts reads).
	domainEnforcementEnforced domainEnforcementMode = "enforced"
	// domainEnforcementAudited means visible-evidence scanning only (F3.3a).
	domainEnforcementAudited domainEnforcementMode = "audited"
	// domainEnforcementUnavailable means the runtime cannot observe reads.
	domainEnforcementUnavailable domainEnforcementMode = "unavailable"
)

// opaqueRuntimeModes are runtime modes whose executor sessions the phased
// orchestrator cannot instrument at all. Gas City sessions run out-of-process
// on a remote substrate that does not surface per-tool-call telemetry back to
// `ao`, so reads inside them are unobservable.
var opaqueRuntimeModes = map[string]bool{
	"gc": true,
}

// domainEnforcementDecision is the resolved enforcement posture for a run,
// including the reason so JSON evidence and stdout are self-explanatory.
type domainEnforcementDecision struct {
	// Mode is the resolved enforcement mode.
	Mode domainEnforcementMode
	// Reason is a human-readable explanation of why this mode was chosen.
	Reason string
	// HookSource is retained for backward-compatible JSON output; it is always
	// empty under the hookless model.
	HookSource string
	// RuntimeMode is the normalized runtime mode the decision was made for.
	RuntimeMode string
}

// resolveDomainEnforcement decides the enforcement posture for a domain-scoped
// run. Under the hookless model the decision is honest by construction:
//
//   - An opaque runtime (Gas City) is `unavailable`: there is no substrate to
//     observe reads, full stop.
//   - Any observable runtime is `audited`: AgentOps ships no read-observing
//     interception substrate, so the fence is checked against visible evidence
//     only (F3.3a behaviour).
func resolveDomainEnforcement(_ string, state *phasedState) domainEnforcementDecision {
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
	return domainEnforcementDecision{
		Mode: domainEnforcementAudited,
		Reason: "no read-interception substrate is installed (hookless model); the read " +
			"fence is checked against visible evidence only (command outputs, generated-file manifests)",
		RuntimeMode: runtimeMode,
	}
}

// gateFailedFromEnforcement reports whether the slice gate must hard-fail given
// the resolved enforcement mode and the out-of-domain references found in
// visible evidence.
//
// Under the hookless model the resolver never returns `enforced`, so the gate
// never hard-fails on the fence — `audited` and `unavailable` modes make no
// enforcement claim, they only report. The `enforced` branch is retained for
// backward-compatible callers passing an explicit mode.
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
