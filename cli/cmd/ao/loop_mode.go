//go:build legacy

package main

import "fmt"

// Loop-execution-mode contract, re-homed from the deleted `ao evolve` CLI (ag-llni).
// The /evolve loop now runs in-session via the skill; these leaf mode constants +
// validation moved to the kept `ao loop` surface, shared by the loop subcommands
// (next-work, blocked, write-stop-marker).
const (
	loopModeBurst = "burst"
	loopModeLoop  = "loop"
)

// validateLoopMode rejects any --mode other than burst|loop.
func validateLoopMode(mode string) error {
	switch mode {
	case loopModeBurst, loopModeLoop:
		return nil
	default:
		return fmt.Errorf("--mode must be 'burst' or 'loop'")
	}
}
