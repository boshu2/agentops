// practices: [design-by-contract]
package provenancegraph

import (
	"reflect"
	"sort"
	"strings"
)

// LedgerReaderVersion is the monotonic CAPABILITY level of THIS binary's ledger
// reader — "which shapes of committed record can I verify without a false
// break?". It is the durable contract the installed pre-push hook (age-rk3r.6,
// `ao verify init`) probes as its ao-version FLOOR before it trusts a chain
// verify.
//
// Why an integer capability and not `ao --version`: the compatibility boundary
// is a READER capability, not a release tag. A pre-age-rk3r.3 reader unmarshals
// a v1.1 verdict record into a struct that DROPS the additive fields
// (reviewer_family, degraded, rounds, duration_s, evidence_path), recomputes the
// payload WITHOUT them, and so reports a SPURIOUS payload_hash mismatch — a false
// "broken chain" — on every v1.1 record (see the Edge "COMPATIBILITY BOUNDARY"
// note in edge.go). The floor must therefore be expressed as "does this reader
// understand the current record shapes", which a monotonic capability integer
// states precisely and a fragile semver parse of a dev/build version string does
// not.
//
// Levels (bump this AND the installed-hook floor together whenever a reader
// change becomes REQUIRED to correctly verify a newer record shape):
//
//	1  understands the age-rk3r.3 v1.1 additive verdict fields, so it never
//	   false-breaks a v1.1 record's hash chain. (age-rk3r.6)
//
// The mere EXISTENCE of the `ao provenance ledger-reader-version` subcommand that
// prints this is itself the pre-age-rk3r.6 floor: a binary old enough to predate
// the v1.1 fields also predates this subcommand, so the hook's probe fails on it
// and refuses with an upgrade message rather than trusting a false-broken chain.
//
// BUMP RULE (verification-surface-honesty S4): bump this level on ANY change to
// the payload-hash fieldset (edgePayload) — additive or not — because a reader
// whose fieldset lags the writer's recomputes the payload differently and
// reports a spurious payload_hash mismatch. The binding is enforced by test:
// reader_version_test.go freezes payloadHashFieldset() per level and fails any
// fieldset change that arrives without a version bump.
const LedgerReaderVersion = 1

// payloadHashSkewHint is the shared payload_hash-mismatch error surface. From
// a reader's seat the mismatch is indistinguishable between real tampering and
// reader-version/hashing skew (a stale binary whose edgePayload fieldset lags
// the writer's — live on 2026-07-10 an installed ao false-flagged the real
// ledger as BROKEN at line 423 while a fresh build verified all 441 records).
// Every mismatch site (VerifyFile, VerifyChain) must emit THIS text so the
// operator rules out skew before treating the ledger as tampered.
const payloadHashSkewHint = "payload_hash mismatch — record content was altered, OR this ao's ledger reader is stale (reader-version/hashing skew): rebuild ao from source (cd cli && make build) and re-verify before treating the ledger as tampered"

// payloadHashFieldset returns the sorted JSON field names of edgePayload — the
// exact fieldset that feeds payload_hash. reader_version_test.go freezes this
// per LedgerReaderVersion so any fieldset change forces a level bump.
func payloadHashFieldset() []string {
	typ := reflect.TypeOf(edgePayload{})
	fields := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		if name, _, _ := strings.Cut(tag, ","); name != "" && name != "-" {
			fields = append(fields, name)
		}
	}
	sort.Strings(fields)
	return fields
}
