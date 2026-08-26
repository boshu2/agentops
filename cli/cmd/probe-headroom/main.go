// Command probe-headroom is the hermetic helper behind
// scripts/check-skill-probe-headroom.sh (gate skill.probe-headroom). It reads
// agentops-skill-probe.* scorecards and reports whether the scenario left any
// HEADROOM to measure in — the question a probe verdict cannot answer, because
// INERT covers both an honest null and a scenario the control arm already aced.
//
// It is a standalone helper main (like cmd/witness-crosscheck and
// cmd/skill-frontmatter-json), NOT an `ao` subcommand, so it adds no surface to
// the ao CLI. The rule lives in internal/probeheadroom; this is the
// file/exit-code shell.
//
// Usage:
//
//	probe-headroom <scorecard.json> [<scorecard.json> ...]   # one probe group
//	probe-headroom --scan <dir>                              # advisory sweep
//
// Exit codes (single-group mode, preserved from the prior-art shell rule):
//
//	0  SEPARATED  — the control arm left room; the verdict reflects the skill
//	3  SATURATED  — the control arm aced it at two effort levels; row is void
//	4  FLOOR      — the treatment arm never acted; scenario or discriminator
//	5  UNMEASURED — no usable treatment reps; the run did not happen
//	2  usage / unreadable input
//
// --scan is advisory: it always exits 0 (or 2 on a read error) and prints one
// line per probe group so a non-blocking gate can name saturated groups
// without failing the build.
package main

import (
	"fmt"
	"os"

	"github.com/boshu2/agentops/cli/internal/probeheadroom"
)

const usage = "usage: probe-headroom <scorecard.json> [...] | probe-headroom --scan <dir>"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}

	if args[0] == "--scan" {
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(2)
		}
		os.Exit(scan(args[1]))
	}

	for _, arg := range args {
		if len(arg) > 1 && arg[0] == '-' {
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(2)
		}
	}

	cards, err := probeheadroom.LoadFiles(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe-headroom: %v\n", err)
		os.Exit(2)
	}
	res, err := probeheadroom.Classify(cards)
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe-headroom: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf("probe: %s  skill: %s\n", res.Probe, res.Skill)
	fmt.Printf("%-18s%-16s%-18s%-18s%s\n", "model", "effort", "control", "treatment", "verdict")
	for _, card := range cards {
		fmt.Printf("%-18s%-16s%-18s%-18s%s\n",
			orUnknown(card.Producer.Model),
			card.EffortLabel(),
			armCell(card.Control),
			armCell(card.Treatment),
			orUnknown(card.Verdict))
	}
	fmt.Printf("PROBE_HEADROOM: %s: %s\n", res.Class, res.Detail)
	os.Exit(res.Class.ExitCode())
}

// scan classifies every probe group under dir and reports each one. It never
// fails on a classification: the gate that calls it is advisory, and a
// saturated historical group is a true finding about the ledger, not a
// regression introduced by the change under test.
func scan(dir string) int {
	groups, probes, err := probeheadroom.LoadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe-headroom: %v\n", err)
		return 2
	}
	if len(probes) == 0 {
		fmt.Printf("PROBE_HEADROOM_SCAN: no scorecards under %s\n", dir)
		return 0
	}
	saturated := 0
	for _, probe := range probes {
		res, classifyErr := probeheadroom.Classify(groups[probe])
		if classifyErr != nil {
			fmt.Fprintf(os.Stderr, "probe-headroom: %v\n", classifyErr)
			return 2
		}
		if res.Class == probeheadroom.Saturated {
			saturated++
			fmt.Fprintf(os.Stderr, "::warning::probe group '%s' (skill %s) is SATURATED: %s\n", res.Probe, res.Skill, res.Detail)
		}
		fmt.Printf("  %-40s %-12s %s\n", res.Probe, res.Class, res.Detail)
	}
	fmt.Printf("PROBE_HEADROOM_SCAN: %d probe group(s) under %s, %d SATURATED\n", len(probes), dir, saturated)
	return 0
}

func armCell(arm probeheadroom.Arm) string {
	if arm.Rate == nil {
		return fmt.Sprintf("n/a (n=%d)", arm.Usable)
	}
	return fmt.Sprintf("%.2f (n=%d)", *arm.Rate, arm.Usable)
}

func orUnknown(s string) string {
	if s == "" {
		return "?"
	}
	return s
}
