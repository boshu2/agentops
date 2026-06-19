// practices: [hexagonal-architecture, escape-corpus-self-improvement]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/yieldledger"
)

// The membrane command family closes the self-improving loop (age-zqc, epic
// age-cwo): an ESCAPE — a gate-verdict that CONFIRMED a bead a later attempt
// REFUTED — is the label that makes the membrane harder to fool. derive-checks
// turns each escape into a finding (the check that would have caught it) and
// compiles it into a pre-mortem membrane check via the existing
// FindingCompilerPort. The escape is the label; the compiled check is the
// membrane getting harder to fool.

var (
	membraneDeriveRun    string
	membraneDeriveDryRun bool
	membraneDeriveForce  bool
	membraneRecallDomain string
	membraneRecallJSON   bool
)

var membraneCmd = &cobra.Command{
	Use:   "membrane",
	Short: "Self-improving membrane: turn escapes into membrane checks",
	Long: `Operate the self-improving membrane (epic age-cwo).

An escape is a membrane miss: a gate-verdict that CONFIRMED a bead which a later,
higher-attempt gate-verdict then REFUTED. derive-checks reads the yield ledger,
detects escapes, derives a finding for each (the check that would have caught
it), and compiles it into a pre-mortem membrane check under .agents/. Re-running
is idempotent — the artifact id is derived deterministically from the escape.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var membraneDeriveCmd = &cobra.Command{
	Use:   "derive-checks --run <id>",
	Short: "Derive membrane checks from escapes in a run",
	RunE:  runMembraneDeriveChecks,
}

var membraneRecallCmd = &cobra.Command{
	Use:   "recall --domain <domain>",
	Short: "Recall past escapes in a domain — the membrane's 'look out for this here' memory",
	Long: `Recall the membrane's accumulated memory for one bounded context: every
escape (a confirmed-then-refuted false-done) recorded in --domain, across all
runs in the yield ledger. This is the consumption side of the self-improving
membrane — before working in a domain, recall what has slipped past the gate
here before so the same class of miss is caught one altitude earlier.`,
	RunE: runMembraneRecall,
}

func init() {
	membraneCmd.GroupID = "knowledge"
	rootCmd.AddCommand(membraneCmd)
	membraneCmd.AddCommand(membraneDeriveCmd)
	membraneCmd.AddCommand(membraneRecallCmd)

	membraneDeriveCmd.Flags().StringVar(&membraneDeriveRun, "run", "", "Run id to scan for escapes (required)")
	membraneDeriveCmd.Flags().BoolVar(&membraneDeriveDryRun, "dry-run", false, "Report what would be derived without writing files")
	membraneDeriveCmd.Flags().BoolVar(&membraneDeriveForce, "force", false, "Overwrite existing derived artifacts")

	membraneRecallCmd.Flags().StringVar(&membraneRecallDomain, "domain", "", "Bounded-context / work-class tag to recall escapes for (required)")
	membraneRecallCmd.Flags().BoolVar(&membraneRecallJSON, "json", false, "Emit the recalled escapes as JSON")
}

// recallByDomain is the testable core of `ao membrane recall`: load the yield
// ledger and return every escape recorded in domain, across all runs — the
// membrane's "what has escaped here before" memory for one bounded context.
// (age-membrane-memory-j9c6.4)
func recallByDomain(root, domain string) ([]yieldledger.Escape, error) {
	ledger, err := yieldledger.Load(root)
	if err != nil {
		return nil, err
	}
	return yieldledger.EscapesByDomain(ledger, domain), nil
}

func runMembraneRecall(cmd *cobra.Command, args []string) error {
	domain := strings.TrimSpace(membraneRecallDomain)
	if domain == "" {
		return fmt.Errorf("--domain is required")
	}
	// repoRootOrCwd (not resolveProjectDir) so recall from a subdirectory (cli/)
	// reads the repo's real .agents/yield, not an empty one — a raw-cwd read
	// would fail OPEN (silently "no escapes here"). (cross-family refute fix)
	root, err := repoRootOrCwd()
	if err != nil {
		return err
	}
	escapes, err := recallByDomain(root, domain)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if membraneRecallJSON {
		if escapes == nil {
			escapes = []yieldledger.Escape{} // emit [] not null for a clean no-match
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(escapes)
	}
	if len(escapes) == 0 {
		fmt.Fprintf(out, "No past escapes recorded in domain %q — clean here (or no data yet).\n", domain)
		return nil
	}
	fmt.Fprintf(out, "Membrane recall — %d past escape(s) in domain %q (look out for these here):\n\n", len(escapes), domain)
	for _, e := range escapes {
		missed := e.Missed
		if missed == "" {
			missed = "(no recorded reason)"
		}
		fmt.Fprintf(out, "- %s (run %s, refuted by %s): %s\n",
			e.BeadID, e.RunID, strings.Join(e.RefuterFamilies, ","), missed)
	}
	return nil
}

// derivedCheck is one escape→finding→check result for reporting.
type derivedCheck struct {
	Escape       yieldledger.Escape `json:"escape"`
	FindingID    string             `json:"finding_id"`
	FindingPath  string             `json:"finding_path"`
	CheckPath    string             `json:"check_path"`
	Wrote        bool               `json:"wrote"`
	SkippedExist bool               `json:"skipped_exists"`
}

type membraneDeriveReport struct {
	Run      string         `json:"run"`
	DryRun   bool           `json:"dry_run"`
	Escapes  int            `json:"escapes"`
	Derived  []derivedCheck `json:"derived"`
	Compiler string         `json:"compiler_target"`
}

func runMembraneDeriveChecks(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(membraneDeriveRun) == "" {
		return fmt.Errorf("--run is required")
	}
	root, err := resolveProjectDir()
	if err != nil {
		return err
	}

	ledger, err := yieldledger.Load(root)
	if err != nil {
		return err
	}
	escapes := yieldledger.DetectEscapes(ledger, membraneDeriveRun)

	compiler := newProductionFindingCompiler()
	report := membraneDeriveReport{
		Run:      membraneDeriveRun,
		DryRun:   membraneDeriveDryRun,
		Escapes:  len(escapes),
		Compiler: string(ports.CompiledOutputPreMortemCheck),
	}

	for _, e := range escapes {
		artifact := deriveFindingFromEscape(e)
		outputs, err := compiler.Compile(context.Background(), artifact)
		if err != nil {
			return fmt.Errorf("compile finding %s: %w", artifact.ID, err)
		}

		dc := derivedCheck{
			Escape:      e,
			FindingID:   artifact.ID,
			FindingPath: filepath.Join(".agents", "findings", artifact.ID+".md"),
		}
		for _, out := range outputs {
			if out.Kind == ports.CompiledOutputPreMortemCheck {
				dc.CheckPath = out.Path
			}
		}

		if !membraneDeriveDryRun {
			wrote, err := writeDerivedArtifacts(root, artifact, outputs, membraneDeriveForce)
			if err != nil {
				return err
			}
			dc.Wrote = wrote
			dc.SkippedExist = !wrote
		}
		report.Derived = append(report.Derived, dc)
	}

	return writeMembraneDeriveReport(cmd.OutOrStdout(), report)
}

// deriveFindingFromEscape turns an escape into the finding that would have
// caught it: a pre-mortem-targeted finding asking whether a unit like this one
// was re-verified by a fresh-context refuter before the membrane confirmed it.
// The id is deterministic (escape bead + confirmed head sha) so re-running is
// idempotent.
func deriveFindingFromEscape(e yieldledger.Escape) ports.FindingArtifact {
	id := deriveEscapeFindingID(e)

	var b strings.Builder
	fmt.Fprintf(&b, "Bead `%s` was CONFIRMED by the membrane at attempt %d (`%s`) ",
		e.BeadID, e.ConfirmedAttempt, shortSHA(e.ConfirmedHeadSHA))
	fmt.Fprintf(&b, "but a later attempt-%d review REFUTED it (`%s`)",
		e.RefutedAttempt, shortSHA(e.RefutedHeadSHA))
	if len(e.RefuterFamilies) > 0 {
		fmt.Fprintf(&b, " — caught by %s", strings.Join(e.RefuterFamilies, ", "))
	}
	b.WriteString(". The membrane let a false-done through.\n\n")
	if e.Domain != "" {
		fmt.Fprintf(&b, "**Domain:** %s — look out for this class of miss when working here.\n\n", e.Domain)
	}
	if e.Missed != "" {
		fmt.Fprintf(&b, "**What was missed:** %s\n\n", e.Missed)
	}
	b.WriteString("**Detection question:** before CONFIRMING a unit like this, has its acceptance ")
	b.WriteString("been re-verified by a fresh-context refuter that does NOT trust the prior ")
	b.WriteString("CONFIRMED verdict — re-running the deterministic acceptance check on the claimed ")
	b.WriteString("head, not the verdict text?\n")

	art := ports.FindingArtifact{
		ID: id,
		Frontmatter: map[string]string{
			"id":                   id,
			"type":                 "finding",
			"source":               "escape",
			"source_skill":         "membrane",
			"status":               "active",
			"severity":             "significant",
			"detectability":        "advisory",
			"compiler_targets":     string(ports.CompiledOutputPreMortemCheck),
			"escape_bead_id":       e.BeadID,
			"escape_run_id":        e.RunID,
			"escape_confirmed_sha": e.ConfirmedHeadSHA,
			"escape_refuted_sha":   e.RefutedHeadSHA,
			"title":                fmt.Sprintf("Escape: %s confirmed then refuted", e.BeadID),
		},
		Body: b.String(),
	}
	// The domain dimension + what-was-missed are what make the gold layer
	// queryable: a finding tagged with its domain is recallable as "look out for
	// this here." (age-membrane-memory-j9c6.3)
	if e.Domain != "" {
		art.Frontmatter["escape_domain"] = e.Domain
	}
	if e.Missed != "" {
		art.Frontmatter["escape_missed"] = e.Missed
	}
	return art
}

// deriveEscapeFindingID returns the deterministic finding id for an escape. The
// id is keyed on the full escape identity — run + bead + the confirmed AND
// refuted head shas — because escapes are run-scoped: the same bead confirmed at
// the same head in two distinct runs (or refuted by different reviewers) is two
// distinct escapes and must not collapse to one artifact, or the escape corpus
// silently under-counts (the corpus is the compounding asset).
func deriveEscapeFindingID(e yieldledger.Escape) string {
	return "escape-" + sanitizeIDPart(e.BeadID) +
		"-" + sanitizeIDPart(e.RunID) +
		"-" + shortSHA(e.ConfirmedHeadSHA) +
		"-" + shortSHA(e.RefutedHeadSHA)
}

func sanitizeIDPart(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}

// writeDerivedArtifacts persists the finding (.agents/findings/<id>.md) and each
// compiled output at its contract path. Each target is written independently: an
// existing file is left untouched (idempotent) unless force is set, but a
// MISSING target is always written even when its siblings already exist — so a
// finding present with its compiled check deleted is REPAIRED, not silently
// reported as already-done. Returns true if any file was written.
func writeDerivedArtifacts(root string, artifact ports.FindingArtifact, outputs []ports.CompiledOutput, force bool) (bool, error) {
	type target struct {
		abs   string
		body  []byte
		label string
	}
	targets := []target{{
		abs:   filepath.Join(root, ".agents", "findings", artifact.ID+".md"),
		body:  renderFindingArtifact(artifact),
		label: "finding " + artifact.ID,
	}}
	for _, out := range outputs {
		targets = append(targets, target{
			abs:   filepath.Join(root, filepath.FromSlash(out.Path)),
			body:  out.Body,
			label: "compiled " + out.Path,
		})
	}

	wroteAny := false
	for _, t := range targets {
		if !force {
			if _, err := os.Stat(t.abs); err == nil {
				continue // present — leave it (idempotent); only missing targets are (re)written
			}
		}
		if err := writeFindingFileAtomic(t.abs, t.body, 0o644); err != nil {
			return wroteAny, fmt.Errorf("write %s: %w", t.label, err)
		}
		wroteAny = true
	}
	return wroteAny, nil
}

// renderFindingArtifact serializes a FindingArtifact to the on-disk finding
// shape: sorted YAML frontmatter then the body.
func renderFindingArtifact(artifact ports.FindingArtifact) []byte {
	var b strings.Builder
	keys := make([]string, 0, len(artifact.Frontmatter))
	for k := range artifact.Frontmatter {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteString("---\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s: %q\n", k, artifact.Frontmatter[k])
	}
	b.WriteString("---\n\n")
	b.WriteString(artifact.Body)
	if !strings.HasSuffix(artifact.Body, "\n") {
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func writeMembraneDeriveReport(out interface{ Write([]byte) (int, error) }, report membraneDeriveReport) error {
	if GetOutput() == "json" {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	if report.Escapes == 0 {
		fmt.Fprintf(out, "No escapes in run %s — membrane has no misses to learn from.\n", report.Run)
		return nil
	}
	verb := "Derived"
	if report.DryRun {
		verb = "Would derive"
	}
	fmt.Fprintf(out, "%s %d membrane check(s) from %d escape(s) in run %s:\n", verb, len(report.Derived), report.Escapes, report.Run)
	for _, dc := range report.Derived {
		status := ""
		if !report.DryRun {
			if dc.Wrote {
				status = " [written]"
			} else if dc.SkippedExist {
				status = " [exists, skipped — use --force]"
			}
		}
		fmt.Fprintf(out, "  %s → %s%s\n", dc.Escape.BeadID, dc.CheckPath, status)
	}
	return nil
}
