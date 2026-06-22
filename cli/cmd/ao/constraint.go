// practices: [design-by-contract, code-complete]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/formatter"
	"github.com/boshu2/agentops/cli/internal/search"
	"github.com/spf13/cobra"
)

type (
	constraintIndex     = search.ConstraintIndex
	constraintEntry     = search.ConstraintEntry
	constraintAppliesTo = search.ConstraintAppliesTo
	constraintDetector  = search.ConstraintDetector
)

var constraintCmd = &cobra.Command{
	Use:   "constraint",
	Short: "Manage compiled constraints",
	Args:  cobra.NoArgs,
	Long: `Manage constraints compiled from promoted findings.

Constraints live in .agents/constraints/index.json. This command manages their
lifecycle.

Subcommands:
  activate  Change a constraint from draft to active
  retire    Change a constraint from active to retired
  review    List constraints needing review (>90 days without citation)
  list      List all constraints with status`,
}

func init() {
	constraintCmd.GroupID = "core"
	rootCmd.AddCommand(constraintCmd)
	constraintCmd.AddCommand(constraintActivateCmd)
	constraintCmd.AddCommand(constraintRetireCmd)
	constraintCmd.AddCommand(constraintReviewCmd)
	constraintCmd.AddCommand(constraintListCmd)
	constraintCmd.AddCommand(constraintPublishCmd)
}

func constraintIndexPath() string { return search.ConstraintIndexPath() }
func constraintLockPath() string  { return search.ConstraintLockPath() }
func loadConstraintIndex() (*constraintIndex, error) {
	return search.LoadConstraintIndex()
}
func withConstraintLock(fn func() error) error { return search.WithConstraintLock(fn) }
func saveConstraintIndexUnlocked(idx *constraintIndex) error {
	return search.SaveConstraintIndexUnlocked(idx)
}
func saveConstraintIndex(idx *constraintIndex) error { return search.SaveConstraintIndex(idx) }
func findConstraint(idx *constraintIndex, id string) *constraintEntry {
	return search.FindConstraint(idx, id)
}

// printConstraintTable renders a slice of constraintEntry as a formatted table
// using formatter.Table with columns ID, STATUS, COMPILED, TITLE.
func printConstraintTable(entries []constraintEntry) {
	tbl := formatter.NewTable(os.Stdout, "ID", "STATUS", "COMPILED", "TITLE")
	tbl.SetMaxWidth(0, 30) // ID
	tbl.SetMaxWidth(2, 20) // COMPILED
	tbl.SetMaxWidth(3, 50) // TITLE
	for _, c := range entries {
		tbl.AddRow(c.ID, c.Status, c.CompiledAt, c.Title)
	}
	_ = tbl.Render()
}

// ----- activate -----

var constraintActivateCmd = &cobra.Command{
	Use:   "activate <id>",
	Short: "Change constraint status from draft to active",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		var activated *constraintEntry
		if err := withConstraintLock(func() error {
			idx, err := loadConstraintIndex()
			if err != nil {
				return err
			}
			c := findConstraint(idx, id)
			if c == nil {
				return fmt.Errorf("constraint %q not found\n  List available constraints with: ao constraint list", id)
			}
			if c.Status != "draft" {
				return fmt.Errorf("constraint %q is %q, can only activate from draft", id, c.Status)
			}
			c.Status = "active"
			clone := *c
			activated = &clone
			return saveConstraintIndexUnlocked(idx)
		}); err != nil {
			return err
		}
		if GetOutput() == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(activated)
		}
		fmt.Printf("Constraint %q activated\n", id)
		return nil
	},
}

// ----- retire -----

var constraintRetireCmd = &cobra.Command{
	Use:   "retire <id>",
	Short: "Change constraint status from active to retired",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		var retired *constraintEntry
		if err := withConstraintLock(func() error {
			idx, err := loadConstraintIndex()
			if err != nil {
				return err
			}
			c := findConstraint(idx, id)
			if c == nil {
				return fmt.Errorf("constraint %q not found\n  List available constraints with: ao constraint list", id)
			}
			if c.Status != "active" {
				return fmt.Errorf("constraint %q is %q, can only retire from active", id, c.Status)
			}
			c.Status = "retired"
			clone := *c
			retired = &clone
			return saveConstraintIndexUnlocked(idx)
		}); err != nil {
			return err
		}
		if GetOutput() == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(retired)
		}
		fmt.Printf("Constraint %q retired\n", id)
		return nil
	},
}

// ----- publish -----

var constraintPublishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish active constraints to the tracked surface so CI / clean clones enforce them",
	Long: `Export the ACTIVE constraints to docs/constraints/published.json (tracked + committed),
carrying ONLY the enforceable detector surface — finding ids and the .agents/ artifact/review paths
are stripped, so no private findings or evidence leak.

This is a DELIBERATE act, not auto-on-activate: a derived rule that hardens the whole repo for
everyone should be a conscious, reviewable, committed change (mirroring the draft->activate human
gate). ao gate check unions the published set with the local .agents/ index, so a clean CI checkout
(which has no .agents/) enforces exactly what you publish. Commit the file to make it travel.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// A MISSING local index means there are simply no constraints to publish —
		// publish is then a graceful NO-OP that writes an EMPTY tracked set, never an
		// error (a CI publish step on a fresh repo, or "un-publish everything", must
		// not fail). Only a present-but-unreadable index is a real error.
		schemaVersion := 1
		var active []constraintEntry
		if _, statErr := os.Stat(constraintIndexPath()); statErr == nil {
			idx, err := loadConstraintIndex()
			if err != nil {
				return err
			}
			if idx.SchemaVersion != 0 {
				schemaVersion = idx.SchemaVersion
			}
			for _, c := range idx.Constraints {
				if c.Status == "active" {
					active = append(active, search.SanitizeForPublish(c))
				}
			}
		}
		path := search.PublishedConstraintIndexRelPath()
		// Merge-preserve: a hand-authored STANDARDS rule (source != "finding") already
		// in the published surface must SURVIVE a publish — publish only refreshes the
		// escape-derived (source="finding") entries it owns. Without this, publishing
		// would wipe every manually-seeded repo rule. (age-az6n)
		preserved, err := preservedStandardsConstraints(path)
		if err != nil {
			return err
		}
		seen := make(map[string]bool, len(active)+len(preserved))
		merged := make([]constraintEntry, 0, len(active)+len(preserved))
		for _, c := range active { // the fresh escape-derived set owns its ids
			merged = append(merged, c)
			seen[c.ID] = true
		}
		for _, c := range preserved {
			if !seen[c.ID] {
				merged = append(merged, c)
			}
		}
		published := &search.ConstraintIndex{SchemaVersion: schemaVersion, Constraints: merged}
		// Defense in depth behind the allowlist: refuse to write rather than leak a
		// residual private .agents/ path that rode along in a kept field.
		if leaked := search.PublishedLeaks(published); len(leaked) > 0 {
			return fmt.Errorf("refusing to publish: constraint(s) %v still carry a private .agents/ path "+
				"(fix the constraint's title/message/globs before publishing)", leaked)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("create published constraints dir: %w", err)
		}
		data, err := json.MarshalIndent(published, "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
		if err := os.WriteFile(path, data, 0o644); err != nil { // #nosec G306 -- tracked, committed surface; world-readable is intended.
			return fmt.Errorf("write published constraints: %w", err)
		}
		if GetOutput() == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(published)
		}
		fmt.Printf("Published %d constraint(s) to %s (%d escape-derived, %d standards preserved) — commit it so CI / clean clones enforce them.\n",
			len(merged), path, len(active), len(merged)-len(active))
		return nil
	},
}

// preservedStandardsConstraints reads the STANDARDS (source != "finding") entries
// from the existing published surface so `ao constraint publish` refreshes only its
// own escape-derived entries and never wipes a hand-authored repo rule. A missing
// file means nothing to preserve; a present-but-CORRUPT file is an error (fail
// closed — never silently drop hand-authored rules). (age-az6n)
func preservedStandardsConstraints(path string) ([]constraintEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading published constraints: %w", err)
	}
	var idx constraintIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("existing published constraints malformed (fix or remove %s before publishing): %w", path, err)
	}
	var out []constraintEntry
	for _, c := range idx.Constraints {
		if c.Source != "finding" {
			out = append(out, c)
		}
	}
	return out, nil
}

// ----- review -----

var constraintReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "List constraints compiled >90 days ago without recent citation",
	RunE: func(cmd *cobra.Command, args []string) error {
		idx, err := loadConstraintIndex()
		if err != nil {
			return err
		}

		cutoff := time.Now().AddDate(0, 0, -90)
		stale := search.FilterStaleConstraints(idx.Constraints, cutoff)

		if GetOutput() == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(stale)
		}

		if len(stale) == 0 {
			fmt.Println("No constraints need review.")
			return nil
		}

		printConstraintTable(stale)
		fmt.Printf("\n%d constraint(s) need review (>90 days old)\n", len(stale))
		return nil
	},
}

// ----- list -----

var constraintListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all constraints with status",
	RunE: func(cmd *cobra.Command, args []string) error {
		idx, err := loadConstraintIndex()
		if err != nil {
			return err
		}

		if GetOutput() == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(idx.Constraints)
		}

		if len(idx.Constraints) == 0 {
			fmt.Println("No constraints found. Promote findings via the flywheel to populate .agents/constraints/index.json.")
			return nil
		}

		printConstraintTable(idx.Constraints)
		fmt.Printf("\n%d constraint(s) total\n", len(idx.Constraints))
		return nil
	},
}
