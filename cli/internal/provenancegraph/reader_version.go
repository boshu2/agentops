// practices: [design-by-contract]
package provenancegraph

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
const LedgerReaderVersion = 1
