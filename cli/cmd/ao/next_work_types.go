package main

import cliRPI "github.com/boshu2/agentops/cli/internal/rpi"

// Next-work corpus type aliases used by the keeper context/codex commands
// (codex.go, context_*.go, stigmergic_packet.go). Canonical definitions live in
// internal/rpi. Relocated here from rpi_loop.go (age-uco1 layer 3) so they
// survive the deletion of the ao rpi command surface (cmd/ao/rpi_*.go) in
// layer 4 / age-3pdt.
type (
	nextWorkEntry         = cliRPI.NextWorkEntry
	nextWorkItem          = cliRPI.NextWorkItem
	nextWorkProofRef      = cliRPI.NextWorkProofRef
	nextWorkProofDecision = cliRPI.NextWorkProofDecision
)
