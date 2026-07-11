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
// candidate "## Scenarios" block to stdout. By default it is a dry-run: the
// bead is never modified. With --write it appends the block to the bead's
// description via `bd update`, but only after an operator y/N confirmation.
// validate parses an authored "## Scenarios" block and exits non-zero when it
// is missing or malformed. An LLM fallback for unparseable acceptance is
// tracked as a later slice of the parent feature (ag-dwq).
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/scenarios"
)

var (
	beadsScenariosJSON         bool
	beadsScenariosForce        bool
	beadsScenariosWrite        bool
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
	out, err := beadsTrackerOutput("show", id, "--json")
	if err != nil {
		return fetchedBead{}, fmt.Errorf("bd show %s --json: %w", id, err)
	}
	return parseBeadFromBDJSON(out)
}

func executeBeadsScenariosExtract(cmd *cobra.Command, args []string) error {
	id := args[0]

	if !beadsTrackerAvailable() {
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

	if beadsScenariosWrite {
		return writeExtractedScenarios(cmd, id, bead, extracted)
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

// writeExtractedScenarios prints the candidate block, asks the operator for a
// y/N confirmation on stdin, and — only when confirmed — appends the block to
// the bead's description via `bd update`. Declining leaves the bead unchanged.
// The block is appended (not replacing the description) so the original
// acceptance prose is preserved.
func writeExtractedScenarios(cmd *cobra.Command, id string, bead fetchedBead, extracted []scenarios.Scenario) error {
	rendered := scenarios.Render(extracted)
	fmt.Fprintf(cmd.ErrOrStderr(),
		"About to append this block to %s's description:\n\n%s\nProceed? [y/N]: ", id, rendered)

	if !confirmScenarioWrite(cmd.InOrStdin()) {
		fmt.Fprintf(cmd.ErrOrStderr(), "aborted; %s left unchanged\n", id)
		return nil
	}

	newDesc := composeDescriptionWithScenarios(bead.Description, rendered)
	if _, err := beadsTrackerOutput("update", id, "--description", newDesc); err != nil {
		return fmt.Errorf("write '## Scenarios' into %s: %w (inspect with 'bd show %s')", id, err, id)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s updated with %d scenario(s)\n", id, len(extracted))
	return nil
}

// confirmScenarioWrite reads a single line and treats "y"/"yes"
// (case-insensitive) as confirmation. Any other input — including EOF or an
// empty line — is a decline, so the default is the safe no-op.
func confirmScenarioWrite(r io.Reader) bool {
	line, _ := bufio.NewReader(r).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// composeDescriptionWithScenarios appends the rendered block to the existing
// description with a blank-line separator, preserving the original text. When
// the description is empty the block stands alone.
func composeDescriptionWithScenarios(desc, rendered string) string {
	block := strings.TrimRight(rendered, "\n")
	base := strings.TrimRight(desc, "\n")
	if base == "" {
		return block + "\n"
	}
	return base + "\n\n" + block + "\n"
}

func executeBeadsScenariosValidate(cmd *cobra.Command, args []string) error {
	id := args[0]

	if !beadsTrackerAvailable() {
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
