package main

import "errors"

// ErrArtifactNotFound is the sentinel a by-ID knowledge lookup returns when
// EVERY source cleanly missed. It is deliberately distinct from a hard probe
// failure (I/O error, corrupt file, permission): a hard failure surfaces the
// underlying error instead. An unreachable source must never masquerade as
// absence — "unreachable is not absent" (age-gascity-port-slate-irye.5).
var ErrArtifactNotFound = errors.New("no artifact found matching ID")

// federatedProbe is one source in a multi-source point read. It returns
// (found, err):
//   - found == true  → the probe located (and emitted) the artifact; err carries
//     any error from emitting it and is propagated as-is.
//   - found == false, err == nil → a CLEAN miss (the source was readable and the
//     artifact simply was not there).
//   - found == false, err != nil → a HARD failure (the source was unreachable,
//     corrupt, or unreadable); this must never be flattened into a clean miss.
type federatedProbe func() (found bool, err error)

// resolveFederated runs probes in order and enforces the read-federation
// invariant, mirroring gascity internal/storeref/storeref.go Resolve's
// first-hard-error discipline:
//
//   - Returns (true, err) as soon as a probe reports found — err is whatever that
//     probe returned when emitting the hit (nil on success).
//   - Otherwise returns (false, firstHardErr): the FIRST hard (non-nil) probe
//     error seen, preserved across the whole scan. A later clean miss never
//     overwrites an earlier hard failure.
//   - Returns (false, nil) ONLY when every probe was a clean miss.
//
// A hard failure means an authoritative source was unavailable, which must never
// look identical to a genuinely absent artifact — that is the silent correctness
// hole (a wrong close/skip) this guards against.
func resolveFederated(probes ...federatedProbe) (found bool, err error) {
	var firstErr error
	for _, probe := range probes {
		hit, perr := probe()
		if hit {
			// A hit short-circuits and propagates its own (emit) error, not the
			// earlier recorded hard error: a located artifact is authoritative.
			return true, perr
		}
		if perr != nil && firstErr == nil {
			firstErr = perr
		}
	}
	return false, firstErr
}
