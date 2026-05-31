// Command witness-crosscheck is the hermetic helper behind
// scripts/witness-dolt-jsonl-crosscheck.sh (ag-lmdx.3). It re-derives the
// hash-chained JSONL witness from a Dolt-projection export and hash-compares it
// against the git-committed witness ledger, exiting non-zero on any divergence.
//
// It is a standalone helper main (like cmd/skill-frontmatter-json), NOT an `ao`
// subcommand, so it adds no surface to the ao CLI. The witness re-derivation
// logic lives in internal/drwitness; this is just the file/exit-code shell.
//
// Usage:
//
//	witness-crosscheck <dolt-rows.jsonl> <committed-witness.jsonl>
//
// Exit codes:
//
//	0  faithful: Dolt re-derives to the committed witness
//	1  diverged: Dolt is NOT a faithful projection (rewrite caught), or the
//	   committed witness chain is itself broken
//	2  usage / unreadable input
package main

import (
	"fmt"
	"os"

	"github.com/boshu2/agentops/cli/internal/drwitness"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: witness-crosscheck <dolt-rows.jsonl> <committed-witness.jsonl>")
		os.Exit(2)
	}
	doltPath, witnessPath := os.Args[1], os.Args[2]

	doltFile, err := os.Open(doltPath) // #nosec G304 -- CI gate path, not user input.
	if err != nil {
		fmt.Fprintf(os.Stderr, "open dolt rows %s: %v\n", doltPath, err)
		os.Exit(2)
	}
	defer func() { _ = doltFile.Close() }()

	rows, err := drwitness.ParseDoltRows(doltFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse dolt rows %s: %v\n", doltPath, err)
		os.Exit(2)
	}

	committed, err := os.ReadFile(witnessPath) // #nosec G304 -- CI gate path, not user input.
	if err != nil {
		fmt.Fprintf(os.Stderr, "read committed witness %s: %v\n", witnessPath, err)
		os.Exit(2)
	}

	res, err := drwitness.CrossCheck(rows, committed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cross-check: %v\n", err)
		os.Exit(2)
	}

	if !res.Match {
		fmt.Fprintf(os.Stderr, "WITNESS_CROSSCHECK: FAIL: %s\n", res.Detail)
		os.Exit(1)
	}
	fmt.Printf("WITNESS_CROSSCHECK: OK: %s\n", res.Detail)
}
