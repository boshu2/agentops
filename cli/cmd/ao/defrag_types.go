package main

import "github.com/boshu2/agentops/cli/internal/lifecycle"

// Defrag result type aliases live in this UNTAGGED file (extracted from
// defrag.go, which is archived behind //go:build flywheel per ADR-0012 /
// age-nzwo) because spine code still references the types: goals_prune.go uses
// PruneResult and uat_smoke_test.go uses DefragReport. Keeping the aliases
// spine-resident lets the `ao defrag` command archive without breaking the
// default build. The aliases forward to the canonical internal/lifecycle types.
type (
	DefragReport      = lifecycle.DefragReport
	PruneResult       = lifecycle.PruneResult
	DefragDedupResult = lifecycle.DefragDedupResult
)
