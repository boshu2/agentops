// Package orchestration holds capability-detection probes for the
// external multi-agent runtimes AgentOps can drive (NTM tmux swarms,
// and future siblings).
//
// The guiding rule, learned from the NTM detection spike: external
// runtimes MUST be detected by CAPABILITY, not by `command -v`. A
// binary being on PATH says nothing about whether it can actually run
// a swarm — the hard dependencies (tmux, git, a persistent host, the
// agent CLIs) may still be missing. So each probe asks the tool to
// report its own capabilities and degrades gracefully when the tool is
// absent: absence is a normal degradation signal, not an error.
//
// Probes accept a small injectable CommandRunner interface so callers
// can be tested against in-memory fakes instead of shelling out to a
// real subprocess, and return plain structs describing what was found.
package orchestration
