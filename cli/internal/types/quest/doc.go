// Package quest provides atomic file-write utilities used across the agentops CLI.
//
// This package was originally created (2026-04-29) as part of the Olympus
// extraction (epic agentops-tqc) to host Olympus's domain types. Those types
// (Quest, Bead, Learning, Briefing, Verdict, Finding, Sections + ULID helpers)
// were deleted on the same day by epic agentops-3ga (bead 3ga.2) after
// investigation confirmed they had zero real consumers anywhere in cli/internal/.
//
// What remains is the atomic file-write helper (AtomicWriteFile,
// AtomicWriteYAML, AtomicWriteFileWithPerm), which serves as the canonical
// destination for the writeFileAtomic dedup work in bead agentops-3ga.4.
//
// The package name "quest" is preserved for now to avoid disrupting in-flight
// imports; a future cleanup may rename it to something more descriptive
// (cli/internal/atomicfile/ or similar).
package quest
