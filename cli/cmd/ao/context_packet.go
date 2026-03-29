package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// contextPacketFlags holds the CLI flags for the context packet command.
var contextPacketFlags struct {
	goal  string
	epic  string
	repo  string
	limit int
	json  bool
}

func init() {
	packetCmd := &cobra.Command{
		Use:   "packet",
		Short: "Inspect the ranked stigmergic packet for a goal or epic",
		Long: `Show the ranked stigmergic packet assembled from knowledge findings,
compiled planning rules, pre-mortem checks, and matched next-work queue items.

The packet is built using the same ranking logic that RPI phases consume,
giving visibility into what knowledge the system would inject for a given
goal or epic context.`,
		RunE: runContextPacket,
	}

	packetCmd.Flags().StringVar(&contextPacketFlags.goal, "goal", "", "goal text to rank against")
	packetCmd.Flags().StringVar(&contextPacketFlags.epic, "epic", "", "active epic ID to filter by")
	packetCmd.Flags().StringVar(&contextPacketFlags.repo, "repo", "", "target repo name (default: auto-detect)")
	packetCmd.Flags().IntVar(&contextPacketFlags.limit, "limit", defaultStigmergicPacketLimit, "max items per section")
	packetCmd.Flags().BoolVar(&contextPacketFlags.json, "json", false, "output as JSON")

	contextCmd.AddCommand(packetCmd)
}

// runContextPacket assembles and displays the ranked stigmergic packet.
func runContextPacket(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	repo := contextPacketFlags.repo
	if repo == "" {
		repo = detectRepoName(cwd)
	}

	target := StigmergicTarget{
		GoalText:   contextPacketFlags.goal,
		ActiveEpic: contextPacketFlags.epic,
		Repo:       repo,
		Limit:      contextPacketFlags.limit,
	}

	packet, err := assembleStigmergicPacket(cwd, target)
	if err != nil {
		return fmt.Errorf("assembling stigmergic packet: %w", err)
	}

	if contextPacketFlags.json {
		return printPacketJSON(cmd, packet)
	}
	printPacketHuman(cmd, packet)
	return nil
}

// printPacketJSON renders the packet as indented JSON to stdout.
func printPacketJSON(cmd *cobra.Command, packet StigmergicPacket) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(packet); err != nil {
		return fmt.Errorf("encoding packet JSON: %w", err)
	}
	return nil
}

// printPacketHuman renders the packet in a human-readable format.
func printPacketHuman(cmd *cobra.Command, packet StigmergicPacket) {
	w := cmd.OutOrStdout()

	// Scorecard
	sc := packet.Scorecard
	fmt.Fprintln(w, "## Scorecard")
	fmt.Fprintf(w, "  Promoted findings:        %d\n", sc.PromotedFindings)
	fmt.Fprintf(w, "  Planning rules:           %d\n", sc.PlanningRules)
	fmt.Fprintf(w, "  Pre-mortem checks:        %d\n", sc.PreMortemChecks)
	fmt.Fprintf(w, "  Queue entries:            %d\n", sc.QueueEntries)
	fmt.Fprintf(w, "  Unconsumed batches:       %d\n", sc.UnconsumedBatches)
	fmt.Fprintf(w, "  Unconsumed items:         %d\n", sc.UnconsumedItems)
	fmt.Fprintf(w, "  High-severity unconsumed: %d\n", sc.HighSeverityUnconsumed)
	fmt.Fprintln(w)

	// Applied findings
	fmt.Fprintln(w, "## Applied Findings")
	if len(packet.AppliedFindings) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for i, id := range packet.AppliedFindings {
			fmt.Fprintf(w, "  %d. %s\n", i+1, id)
		}
	}
	fmt.Fprintln(w)

	// Planning rules
	fmt.Fprintln(w, "## Planning Rules")
	if len(packet.PlanningRules) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for _, rule := range packet.PlanningRules {
			fmt.Fprintf(w, "  - %s\n", strings.TrimSpace(rule))
		}
	}
	fmt.Fprintln(w)

	// Known risks
	fmt.Fprintln(w, "## Known Risks")
	if len(packet.KnownRisks) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for _, risk := range packet.KnownRisks {
			fmt.Fprintf(w, "  - %s\n", strings.TrimSpace(risk))
		}
	}
	fmt.Fprintln(w)

	// Matched next-work
	fmt.Fprintln(w, "## Matched Next-Work")
	if len(packet.PriorFindings) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for i, item := range packet.PriorFindings {
			sev := item.Severity
			if sev == "" {
				sev = "unset"
			}
			fmt.Fprintf(w, "  %d. [%s] %s (%s)\n", i+1, strings.ToUpper(sev), item.Title, item.Type)
			if item.Description != "" {
				fmt.Fprintf(w, "     %s\n", item.Description)
			}
		}
	}
}

// detectRepoName returns the base directory name as a repo identifier.
func detectRepoName(cwd string) string {
	// Walk up to find a .git directory and use that parent's name.
	dir := cwd
	for {
		if info, err := os.Stat(dir + "/.git"); err == nil && info.IsDir() {
			return fileBase(dir)
		}
		if info, err := os.Stat(dir + "/.git"); err == nil && !info.IsDir() {
			// worktree: .git is a file
			return fileBase(dir)
		}
		parent := dir[:max(strings.LastIndex(dir, "/"), 0)]
		if parent == "" || parent == dir {
			break
		}
		dir = parent
	}
	return fileBase(cwd)
}

// fileBase returns the last path component.
func fileBase(path string) string {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return path[idx+1:]
	}
	return path
}
