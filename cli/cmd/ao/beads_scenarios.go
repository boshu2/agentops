// `ao beads scenarios` — convert a bead's free-text acceptance criteria into
// Gherkin scenarios.
//
// This slice ships a single read-only subcommand:
//
//	ao beads scenarios extract <bead-id>
//
// It fetches the bead via `bd show <id> --json`, deterministically converts
// the acceptance bullets into Given/When/Then triples, and prints a candidate
// "## Scenarios" block to stdout. It is a dry-run: the bead is never modified.
// Operator-confirmed write-back, idempotent guards, a validate subcommand, and
// an LLM fallback are tracked as later slices of the parent feature (ag-dwq).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/scenarios"
)

var (
	beadsScenariosJSON  bool
	beadsScenariosForce bool
)

// fetchedBead is the subset of a bead the scenarios command needs: the
// acceptance text it extracts from, and the description it guards against. A
// bead whose description already carries a "## Scenarios" block is not
// re-extracted over unless --force is passed.
type fetchedBead struct {
	Acceptance  string
	Description string
}

// beadsScenariosFetch fetches a bead's acceptance and description text. It is a
// package var so tests can inject a fake without shelling out to bd.
var beadsScenariosFetch = func(id string) (fetchedBead, error) {
	out, err := execBD("show", id, "--json")
	if err != nil {
		return fetchedBead{}, fmt.Errorf("bd show %s --json: %w", id, err)
	}
	return parseBeadFromBDJSON(out)
}

var beadsScenariosCmd = &cobra.Command{
	Use:   "scenarios",
	Short: "Convert bead acceptance criteria into Gherkin scenarios",
	Args:  cobra.NoArgs,
	Long: `Turn a bead's free-text acceptance criteria into structured Gherkin
Given/When/Then scenarios.

The 'extract' subcommand is a dry-run: it prints a candidate '## Scenarios'
block to stdout for review and never modifies the bead.`,
}

var beadsScenariosExtractCmd = &cobra.Command{
	Use:   "extract <bead-id>",
	Short: "Print a candidate Gherkin '## Scenarios' block from a bead's acceptance (dry-run)",
	Args:  cobra.ExactArgs(1),
	Long: `Read a bead's acceptance criteria via 'bd show <id> --json', convert the
free-text bullets into Given/When/Then scenarios using deterministic rules,
and print a candidate '## Scenarios' block to stdout.

This is a dry-run — the bead is never modified. Review the output and author
it into the bead manually. With --json the scenarios are emitted as structured
data on stdout instead of a Gherkin block.

If the bead already carries a '## Scenarios' block, extract refuses (nothing to
do) unless --force is passed, to avoid generating noise over already-shaped
acceptance.`,
	RunE: runBeadsScenariosExtract,
}

func init() {
	beadsCmd.AddCommand(beadsScenariosCmd)
	beadsScenariosCmd.AddCommand(beadsScenariosExtractCmd)
	beadsScenariosExtractCmd.Flags().BoolVar(&beadsScenariosJSON, "json", false,
		"Emit extracted scenarios as JSON (data on stdout) instead of a Gherkin block")
	beadsScenariosExtractCmd.Flags().BoolVar(&beadsScenariosForce, "force", false,
		"Extract even when the bead already has a '## Scenarios' block")
}

func runBeadsScenariosExtract(cmd *cobra.Command, args []string) error {
	id := args[0]

	if !bdAvailable() {
		fmt.Fprintln(cmd.ErrOrStderr(),
			"warning: bd not found on PATH; cannot fetch bead. Install bd or author scenarios manually.")
		return nil
	}

	bead, err := beadsScenariosFetch(id)
	if err != nil {
		return fmt.Errorf("fetch acceptance for %s: %w (inspect with 'bd show %s --json')", id, err, id)
	}

	if !beadsScenariosForce &&
		(scenarios.HasScenariosBlock(bead.Description) || scenarios.HasScenariosBlock(bead.Acceptance)) {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"bead %s already has a '## Scenarios' block; nothing to extract. Re-run with --force to extract anyway.\n", id)
		return nil
	}

	extracted, err := scenarios.Extract(bead.Acceptance)
	if err != nil {
		return fmt.Errorf(
			"extract scenarios from %s: %w; author a '## Scenarios' block manually (see CLAUDE.md acceptance doctrine)",
			id, err)
	}

	if beadsScenariosJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			BeadID    string               `json:"bead_id"`
			Scenarios []scenarios.Scenario `json:"scenarios"`
		}{BeadID: id, Scenarios: extracted})
	}

	fmt.Fprint(cmd.OutOrStdout(), scenarios.Render(extracted))
	return nil
}

// parseBeadFromBDJSON extracts the acceptance and description text from the
// output of `bd show <id> --json`. bd emits a JSON array of bead objects; older
// versions may emit a single object, so both shapes are handled. The
// acceptance field used for extraction prefers acceptance_criteria, falling
// back to the description when it is empty; the raw description is returned
// separately so the caller can guard against an already-present scenarios
// block.
func parseBeadFromBDJSON(out []byte) (fetchedBead, error) {
	type bead struct {
		AcceptanceCriteria string `json:"acceptance_criteria"`
		Description        string `json:"description"`
	}

	trimmed := bytes.TrimSpace(out)
	var b bead
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var arr []bead
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return fetchedBead{}, fmt.Errorf("parse bd json array: %w", err)
		}
		if len(arr) == 0 {
			return fetchedBead{}, fmt.Errorf("bead not found")
		}
		b = arr[0]
	} else if err := json.Unmarshal(trimmed, &b); err != nil {
		return fetchedBead{}, fmt.Errorf("parse bd json: %w", err)
	}

	desc := strings.TrimSpace(b.Description)
	acc := strings.TrimSpace(b.AcceptanceCriteria)
	if acc == "" {
		acc = desc
	}
	if acc == "" {
		return fetchedBead{}, fmt.Errorf("bead has no acceptance_criteria or description text")
	}
	return fetchedBead{Acceptance: acc, Description: desc}, nil
}
