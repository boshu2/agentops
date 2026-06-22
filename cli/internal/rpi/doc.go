// Package rpi is a LEGACY package name. It once implemented the rpi
// (Research-Plan-Implement) orchestration ENGINE — that engine has been DELETED
// (age-tlj6, executing ADR-0009): the `ao rpi` command, the loop/dispatcher/
// session-spawner, and the dashboard daemon are gone, and no skill references it.
//
// What remains here is the live UTILITY code the engine left behind, still consumed
// across cmd/ao, internal/eval, and the mto adapter:
//
//   - next-work schema + ranking (next-work entries, queue read/rewrite, ranking)
//   - run discovery / registry / run-info (RPIRunInfo, run scanning, toolchain)
//   - the execution packet, the run-state file, and a few markdown/handoff helpers
//
// The cluster is intentionally NOT renamed or split: it is entangled (next-work
// proof -> run discovery -> run-info) and used by ~16 importers, so a rename would be
// broad mechanical churn for cosmetic value (cross-family council decision, age-tlj6).
// Read `rpi` as a historical name for "next-work + run-tracking utilities," not an
// engine.
package rpi
