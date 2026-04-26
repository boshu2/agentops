package main

import (
	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ratchet"
)

var ratchetCmd = &cobra.Command{
	Use:   "ratchet",
	Short: "Brownian Ratchet workflow tracking",
	Long: `Track progress through the phased RPI workflow.

The Brownian Ratchet ensures progress can't be lost:
  Chaos × Filter → Ratchet = Progress

Inspection:
  status (s)    Show current ratchet chain state
  check (c)     Check if a step's gate is met
  next (n)      Show next pending RPI step
  spec          Get current spec path
  validate      Validate step requirements

Progression:
  record        Record step completion
  promote (p)   Record tier promotion
  skip          Record intentional skip

Search & Trace:
  find          Search for artifacts across locations
  trace         Trace provenance backward

Management:
  migrate            Migrate legacy chain format
  migrate-artifacts  Add schema_version to artifacts

The ratchet chain is stored in .agents/ao/chain.jsonl`,
}

// Ratchet command flags (shared across subcommands)
var (
	ratchetEpicID      string
	ratchetChainID     string
	ratchetInput       string
	ratchetOutput      string
	ratchetReason      string
	ratchetTier        int
	ratchetLock        bool
	ratchetFiles       []string
	ratchetLenient     bool
	ratchetLenientDays int
	ratchetCycle       int
	ratchetParentEpic  string
)

// ratchetStepInfo holds step information for status output.
type ratchetStepInfo struct {
	Step       ratchet.Step       `json:"step"`
	Status     ratchet.StepStatus `json:"status"`
	Output     string             `json:"output,omitempty"`
	Input      string             `json:"input,omitempty"`
	Time       string             `json:"time,omitempty"`
	Location   string             `json:"location,omitempty"`
	Cycle      int                `json:"cycle,omitempty"`
	ParentEpic string             `json:"parent_epic,omitempty"`
}

// ratchetStatusOutput holds the full status output structure.
type ratchetStatusOutput struct {
	ChainID string            `json:"chain_id"`
	Started string            `json:"started"`
	EpicID  string            `json:"epic_id,omitempty"`
	Steps   []ratchetStepInfo `json:"steps"`
	Path    string            `json:"path"`
}

func init() {
	ratchetCmd.AddGroup(
		&cobra.Group{ID: "inspection", Title: "Inspection:"},
		&cobra.Group{ID: "progression", Title: "Progression:"},
		&cobra.Group{ID: "search", Title: "Search & Trace:"},
		&cobra.Group{ID: "management", Title: "Management:"},
	)
	ratchetCmd.GroupID = "workflow"
	rootCmd.AddCommand(ratchetCmd)
}

func statusIcon(status ratchet.StepStatus) string {
	switch status {
	case ratchet.StatusLocked:
		return "✓"
	case ratchet.StatusSkipped:
		return "⊘"
	case ratchet.StatusInProgress:
		return "◐"
	case ratchet.StatusPending:
		return "○"
	default:
		return "○"
	}
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return "..."[:max]
	}
	return string(runes[:max-3]) + "..."
}
