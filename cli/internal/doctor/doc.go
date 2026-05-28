// Package doctor implements the shared engine for `ao doctor`'s diagnose-and-repair
// surface: the mutate() chokepoint, run-artifact management, advisory locking,
// the capabilities contract, and the detector/fixer registry.
//
// Every disk write performed under `ao doctor fix` or `ao doctor undo` flows
// through exactly one function — Mutate — which backs up verbatim, records a
// SHA-256-witnessed line into actions.jsonl, and writes atomically via
// temp-file + rename. Detectors are pure (no writes). This single chokepoint
// is what makes reversibility, idempotence, and crash-recovery provable.
//
// This package is the FOUNDATION wave: it ships zero detectors and zero
// fixers. Later per-subsystem waves register their own Detector and Fixer
// implementations from init() functions in new files, without editing any
// shared file.
package doctor
