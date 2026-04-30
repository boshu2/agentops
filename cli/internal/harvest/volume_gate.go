package harvest

import (
	"fmt"
	"io"
)

// DefaultMaxPromotions is the default volume-gate threshold for promotions
// per `ao harvest` invocation. Estimated from a 30-day median + 3σ on the
// home-vault hub: a steady-state pass writes well under this; values above
// signal either a legitimate backlog drain (e.g., the 2,638-promotion
// post-cleanup run) or a feedback-loop regression. The gate is advisory:
// it surfaces the count, never blocks.
const DefaultMaxPromotions = 500

// EmitVolumeGateWarning is the advisory volume gate from soc-f2q4 (4-B).
// When len(catalog.Promoted) > threshold (strictly greater), it writes a
// single WARN line to w describing the count, the threshold, and the
// override paths (--max-promotions=N flag, AO_MAX_PROMOTIONS env). It
// returns true iff the warning was emitted.
//
// Crucially this is WARN-only: callers MUST NOT change exit code based on
// the return value. The 2,638-promotion drain from the soc-ujls cleanup
// was a legitimate operation; a hard gate would have falsely blocked it.
// Operators tune via the override paths or set AO_MAX_PROMOTIONS=0 (any
// non-positive threshold disables the gate) to silence it entirely.
//
// Nil-safe: a nil catalog or nil writer is a no-op.
func EmitVolumeGateWarning(catalog *Catalog, threshold int, w io.Writer) bool {
	if catalog == nil || w == nil {
		return false
	}
	if threshold <= 0 {
		return false
	}
	count := len(catalog.Promoted)
	if count <= threshold {
		return false
	}
	fmt.Fprintf(w,
		"WARN: %d promotions exceeded threshold %d (override: --max-promotions=N or AO_MAX_PROMOTIONS=N)\n",
		count, threshold)
	return true
}
