// `ao beads scenarios` — convert and check a bead's acceptance criteria as
// Gherkin scenarios.
//
// This surface ships two read-only subcommands:
//
//	ao beads scenarios extract <bead-id>    convert acceptance -> candidate block
//	ao beads scenarios validate <bead-id>   check an authored block is well-formed
//
// extract fetches the bead via `bd show <id> --json`, deterministically
// converts the acceptance bullets into Given/When/Then triples, and prints a
// candidate "## Scenarios" block to stdout. It is a dry-run: the bead is never
// modified. validate parses an authored "## Scenarios" block and exits non-zero
// when it is missing or malformed. Operator-confirmed write-back and an LLM
// fallback are tracked as later slices of the parent feature (ag-dwq).
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
	beadsScenariosJSON         bool
	beadsScenariosForce        bool
	beadsScenariosValidateJSON bool
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
block to stdout for review and never modifies the bead. The 'validate'
subcommand checks that an authored '## Scenarios' block is well-formed Gherkin
and exits non-zero (naming the parse error) when it is not.`,
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

var beadsScenariosValidateCmd = &cobra.Command{
	Use:   "validate <bead-id>",
	Short: "Check that a bead's authored '## Scenarios' block is well-formed Gherkin",
	Args:  cobra.ExactArgs(1),
	Long: `Read a bead via 'bd show <id> --json' and validate its authored
'## Scenarios' block. Each scenario must declare a name and a Given/When/Then
step (in that order) with a non-empty body; And/But lines are accepted as
continuations.

Exits 0 when the block is well-formed. Exits non-zero, naming the parse error,
when the block is missing or malformed. With --json a verdict object is emitted
on stdout ({"bead_id","valid","scenarios"} on success, or
{"bead_id","valid":false,"error"} on failure) while diagnostics go to stderr.`,
	RunE: runBeadsScenariosValidate,
}

func init() {
	beadsCmd.AddCommand(beadsScenariosCmd)
	beadsScenariosCmd.AddCommand(beadsScenariosExtractCmd)
	beadsScenariosCmd.AddCommand(beadsScenariosValidateCmd)
	beadsScenariosExtractCmd.Flags().BoolVar(&beadsScenariosJSON, "json", false,
		"Emit extracted scenarios as JSON (data on stdout) instead of a Gherkin block")
	beadsScenariosExtractCmd.Flags().BoolVar(&beadsScenariosForce, "force", false,
		"Extract even when the bead already has a '## Scenarios' block")
	beadsScenariosValidateCmd.Flags().BoolVar(&beadsScenariosValidateJSON, "json", false,
		"Emit a structured validation verdict as JSON on stdout")
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

func runBeadsScenariosValidate(cmd *cobra.Command, args []string) error {
	id := args[0]

	if !bdAvailable() {
		fmt.Fprintln(cmd.ErrOrStderr(),
			"warning: bd not found on PATH; cannot fetch bead. Install bd or author scenarios manually.")
		return nil
	}

	bead, err := beadsScenariosFetch(id)
	if err != nil {
		return fmt.Errorf("fetch bead %s: %w (inspect with 'bd show %s --json')", id, err, id)
	}

	// The authored '## Scenarios' block lives in the description; fall back to
	// the acceptance field when the description carries no block.
	text := bead.Description
	if !scenarios.HasScenariosBlock(text) && scenarios.HasScenariosBlock(bead.Acceptance) {
		text = bead.Acceptance
	}

	parsed, vErr := scenarios.ParseBlock(text)
	if vErr != nil {
		if beadsScenariosValidateJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			_ = enc.Encode(struct {
				BeadID string `json:"bead_id"`
				Valid  bool   `json:"valid"`
				Error  string `json:"error"`
			}{BeadID: id, Valid: false, Error: vErr.Error()})
		}
		return fmt.Errorf(
			"validate scenarios for %s: %w; fix the block or regenerate it with 'ao beads scenarios extract %s --force'",
			id, vErr, id)
	}

	if beadsScenariosValidateJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			BeadID    string               `json:"bead_id"`
			Valid     bool                 `json:"valid"`
			Scenarios []scenarios.Scenario `json:"scenarios"`
		}{BeadID: id, Valid: true, Scenarios: parsed})
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s: %d scenario(s) well-formed\n", id, len(parsed))
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
