// Package main — `ao beads` subcommand group.
//
// Provides tools for the bd (beads) issue tracker that complement — but do
// not replace — the bd CLI itself. All commands in this group degrade
// gracefully when bd is not on PATH: they emit a warning and exit 0 rather
// than break environments that don't have bd installed.
//
// Subcommands:
//
//	ao beads audit           — backlog hygiene audit for open/in-progress beads
//	ao beads cluster         — consolidation suggestions for overlapping open beads
//	ao beads dir             — print the resolved live br ledger directory
//	ao beads verify <id>     — stale-citation detector for a single bead
//	ao beads lint            — batch-verify every open bead
//	ao beads harvest <id>    — materialize a closed bead's reason into a learning
//
// The design goal is "pre-flight for inherited scope": catch bead
// descriptions that drift from HEAD before a new session acts on them. The
// planning rule at skills/plan/references/stale-scope-validation.md explains
// when to run these.
// practices: [dora-metrics, lean-startup]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	beadsadapter "github.com/boshu2/agentops/cli/internal/adapters/beads"
	beadsapp "github.com/boshu2/agentops/cli/internal/beads"
)

// beadsVerdictError carries a verdict exit code out through cobra's RunE so
// Execute() can map it to os.Exit() WITHOUT calling os.Exit() mid-command —
// which would skip deferred cleanup (temp files, lock release) and kill the
// test binary, making these paths untestable.
//
// The `ao beads verify|lint|audit` commands use exit-code-as-verdict: exit 1
// means "stale citations / flagged beads found", a normal diagnostic outcome,
// not an internal failure. The verdict text already went to stdout, so the
// error carries no message (Error() == ""); the returning RunE sets
// cmd.SilenceErrors so cobra emits no spurious "Error:" line, while genuine
// errors returned elsewhere in those commands still print normally. Mirrors
// gateExitError (validate.go) and doctorExitError (doctor_surface.go).
type beadsVerdictError struct {
	code int
}

func (e *beadsVerdictError) Error() string { return "" }

// ExitCode returns the process exit code this verdict maps to.
func (e *beadsVerdictError) ExitCode() int { return e.code }

// beadsTrackerOutput is the single entry point for shelling out to bd. Tests override
// this to avoid a hard dependency on the real binary. Production code calls
// `bd` via PATH; if absent, the caller emits a graceful warning and returns.
var beadsTrackerOutput = func(args ...string) ([]byte, error) {
	return currentBeadsTracker().Output(context.Background(), args...)
}

// beadsTrackerAvailable reports whether the bd binary is reachable via PATH. Tests
// override this for deterministic behaviour.
var beadsTrackerAvailable = func() bool {
	return currentBeadsTracker().Available()
}

func currentBeadsTracker() *beadsadapter.Tracker {
	return beadsadapter.NewTrackerWith(os.Getwd, os.Environ, func(name string) (string, error) {
		return trackerLookPath(name)
	})
}

var (
	beadsDirJSON       bool
	beadsDirRequire    bool
	beadsTrackerJSON   bool
	beadsVerifyJSON    bool
	beadsVerifyVerbose bool
	beadsLintStatus    string
	beadsLintJSON      bool
	beadsHarvestOutDir string
	beadsHarvestDryRun bool
)

// ------------------------------------------------------------------------
// verify
// ------------------------------------------------------------------------

// CitationStatus is the three-valued verdict for a single citation extracted
// from a bead description.
type CitationStatus = beadsapp.CitationStatus

const (
	CitationFresh   = beadsapp.CitationFresh
	CitationStale   = beadsapp.CitationStale
	CitationUnknown = beadsapp.CitationUnknown
)

// Citation is a single verifiable reference pulled from a bead description.
type Citation = beadsapp.Citation

// VerifyReport is the structured result of `ao beads verify`.
type VerifyReport = beadsapp.VerifyReport

func executeBeadsVerify(cmd *cobra.Command, args []string) error {
	beadID := args[0]
	report, err := verifyBead(beadID)
	if err != nil {
		return err
	}
	if !report.BDAvailable {
		fmt.Fprintln(os.Stderr, "WARN: bd not on PATH — skipping verify (graceful degradation)")
		return nil
	}
	if beadsVerifyJSON {
		return emitJSON(os.Stdout, report)
	}
	emitVerifyHuman(os.Stdout, report, beadsVerifyVerbose)
	if report.StaleCount > 0 {
		if cmd != nil {
			cmd.SilenceErrors = true
		}
		return &beadsVerdictError{code: 1}
	}
	return nil
}

// verifyBead shells out to bd, parses the description, extracts citations,
// and verifies each against HEAD. Returns a report regardless of verdict;
// callers decide what to do with StaleCount.
func verifyBead(beadID string) (*VerifyReport, error) {
	if !beadsTrackerAvailable() {
		return &VerifyReport{BeadID: beadID, BDAvailable: false}, nil
	}
	raw, err := beadsTrackerOutput("show", beadID)
	if err != nil {
		return nil, fmt.Errorf("bd show %s: %w", beadID, err)
	}
	parsed, err := parseBDShow(string(raw))
	if err != nil {
		return nil, err
	}
	citations := extractCitations(parsed.Body())
	cwd, _ := os.Getwd()
	for i := range citations {
		verifyCitationInPlace(&citations[i], cwd)
	}
	report := &VerifyReport{
		BeadID:      beadID,
		Title:       parsed.Title,
		Status:      parsed.Status,
		Citations:   citations,
		TotalCount:  len(citations),
		BDAvailable: true,
	}
	for _, c := range citations {
		switch c.Status {
		case CitationFresh:
			report.FreshCount++
		case CitationStale:
			report.StaleCount++
		}
	}
	return report, nil
}

// bdShowParsed captures the shape of `bd show <id>` output that we care
// about. Intentionally tolerant — missing fields become empty strings.
//
// Note on Description vs CloseReason: for OPEN beads, the original filed
// description lives under the `DESCRIPTION` heading. For CLOSED beads, bd
// typically hides the original description and surfaces the operator's
// `Close reason:` line instead. `harvest` wants the close reason; `verify`
// wants whichever is present. The Body accessor returns the first non-empty.
type bdShowParsed = beadsapp.ParsedBead

// parseBDShow parses the human-readable `bd show <id>` output. Observed
// formats (2026-04-11):
//
//	Open bead:
//	  ○ na-h61 · TITLE   [● P2 · OPEN]
//	  Owner: ... · Type: ... · Created: ... · Updated: ...
//	  DESCRIPTION
//	  <body until blank line or [rerun: ...]>
//
//	Closed bead:
//	  ✓ na-h61 · TITLE   [● P2 · CLOSED]
//	  Owner: ... · Type: ... · Created: ... · Updated: ...
//	  Close reason: <body>
//	  DESCRIPTION
//	  (empty or [rerun: ...])
//
// We accept ○, ●, ✓, or no bullet marker. The Close reason: line is
// captured into CloseReason. The DESCRIPTION body is captured into
// Description.
func parseBDShow(raw string) (*bdShowParsed, error) {
	return beadsapp.ParseBDShow(raw)
}

// extractCitations pulls verifiable references out of a description body.
// Three kinds are recognised:
//   - File paths (with optional :line suffix)
//   - Go function references (`func Name(` or `type.Method(`)
//   - Backticked symbols that look like identifiers
func extractCitations(desc string) []Citation {
	return beadsapp.ExtractCitations(desc)
}

// verifyCitationInPlace checks a single citation against the repo state at
// cwd and mutates its Status/Reason/Resolved fields accordingly.
func verifyCitationInPlace(c *Citation, cwd string) {
	switch c.Kind {
	case "file":
		verifyFileCitation(c, cwd)
	case "function":
		verifyFunctionCitation(c, cwd)
	case "symbol":
		verifySymbolCitation(c, cwd)
	default:
		c.Status = CitationUnknown
		c.Reason = "unrecognized citation kind"
	}
}

func verifyFileCitation(c *Citation, cwd string) {
	// Strip optional :line suffix for the stat check.
	path := c.Raw
	if idx := strings.LastIndex(path, ":"); idx >= 0 {
		if _, err := fmt.Sscanf(path[idx+1:], "%d", new(int)); err == nil {
			path = path[:idx]
		}
	}

	// First, try the exact path as-is.
	abs := filepath.Join(cwd, path)
	if _, err := os.Stat(abs); err == nil {
		c.Status = CitationFresh
		c.Reason = "file exists at HEAD"
		return
	}

	// If the path contains a slash, the citation is specific enough that
	// a miss is a real STALE. No fallback.
	if strings.Contains(path, "/") {
		c.Status = CitationStale
		c.Reason = fmt.Sprintf("file %s not found at HEAD", path)
		return
	}

	// Bare filename (e.g., "loop.go", "types.go"). Search by basename
	// across the common source roots to decide FRESH / UNKNOWN / STALE.
	matches := findFilesByBasename(cwd, path)
	switch len(matches) {
	case 0:
		c.Status = CitationStale
		c.Reason = fmt.Sprintf("bare filename %q has zero matches at HEAD", path)
	case 1:
		c.Status = CitationFresh
		c.Reason = "bare filename resolves uniquely"
		c.Resolved = matches[0]
	default:
		c.Status = CitationUnknown
		c.Reason = fmt.Sprintf("bare filename %q is ambiguous (%d matches) — cite the full path", path, len(matches))
		c.Resolved = strings.Join(matches[:beadMinInt(3, len(matches))], ", ")
	}
}

// findFilesByBasename walks cli/, skills/, docs/, scripts/, and .agents/
// looking for files whose basename matches name. Returns up to 10 relative
// paths. Used to resolve bare-filename citations.
func findFilesByBasename(cwd, name string) []string {
	var matches []string
	roots := []string{"cli", "skills", "docs", "scripts", ".agents"}
	for _, root := range roots {
		rootAbs := filepath.Join(cwd, root)
		if _, err := os.Stat(rootAbs); err != nil {
			continue
		}
		_ = filepath.Walk(rootAbs, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				// Skip hidden dirs except .agents (already rooted).
				base := filepath.Base(path)
				if strings.HasPrefix(base, ".") && path != rootAbs {
					return filepath.SkipDir
				}
				// Skip test-heavy / generated paths.
				if base == "node_modules" || base == "vendor" || base == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Base(path) != name {
				return nil
			}
			rel, relErr := filepath.Rel(cwd, path)
			if relErr == nil {
				matches = append(matches, rel)
			}
			if len(matches) >= 10 {
				return filepath.SkipDir
			}
			return nil
		})
		if len(matches) >= 10 {
			break
		}
	}
	return matches
}

func beadMinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func verifyFunctionCitation(c *Citation, cwd string) {
	// c.Raw is "func Name". Grep for it across cli/ and skills/.
	name := strings.TrimPrefix(c.Raw, "func ")
	matches := grepSymbol(cwd, name)
	if len(matches) == 0 {
		c.Status = CitationStale
		c.Reason = fmt.Sprintf("function %q has zero definitions at HEAD", name)
		return
	}
	c.Status = CitationFresh
	c.Reason = fmt.Sprintf("function defined at %d location(s)", len(matches))
	if len(matches) == 1 {
		c.Resolved = matches[0]
	}
}

func verifySymbolCitation(c *Citation, cwd string) {
	sym := strings.Trim(c.Raw, "`")
	matches := grepSymbol(cwd, sym)
	if len(matches) == 0 {
		c.Status = CitationStale
		c.Reason = fmt.Sprintf("symbol %q has zero references at HEAD", sym)
		return
	}
	c.Status = CitationFresh
	c.Reason = fmt.Sprintf("symbol found at %d location(s)", len(matches))
}

// grepSymbol greps for a symbol across the common source roots (cli/,
// skills/, docs/, scripts/) and returns a list of "path:line" matches.
// Limited to 10 results for speed.
func grepSymbol(cwd, sym string) []string {
	if sym == "" {
		return nil
	}
	// Escape regex special characters in the symbol.
	safe := regexp.QuoteMeta(sym)
	cmd := exec.Command("grep", "-rn", "-l", "--include=*.go", "--include=*.md", "--include=*.py",
		"--include=*.sh", "--include=*.yaml", "--include=*.yml", "--include=*.json",
		safe, filepath.Join(cwd, "cli"), filepath.Join(cwd, "skills"), filepath.Join(cwd, "scripts"))
	out, _ := cmd.Output()
	var matches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches = append(matches, line)
		if len(matches) >= 10 {
			break
		}
	}
	return matches
}

func emitVerifyHuman(w *os.File, r *VerifyReport, verbose bool) {
	fmt.Fprintf(w, "bead %s: %s  [%s]\n", r.BeadID, r.Title, r.Status)
	fmt.Fprintf(w, "  citations: %d total, %d fresh, %d stale\n",
		r.TotalCount, r.FreshCount, r.StaleCount)
	for _, c := range r.Citations {
		if c.Status == CitationFresh && !verbose {
			continue
		}
		marker := "  "
		switch c.Status {
		case CitationStale:
			marker = "[STALE]"
		case CitationFresh:
			marker = "[FRESH]"
		case CitationUnknown:
			marker = "[?????]"
		}
		fmt.Fprintf(w, "  %s %s — %s\n", marker, c.Raw, c.Reason)
		if c.Resolved != "" {
			fmt.Fprintf(w, "          → %s\n", c.Resolved)
		}
	}
}

// ------------------------------------------------------------------------
// lint
// ------------------------------------------------------------------------

// LintReport is the aggregate result of `ao beads lint`.
type LintReport = beadsapp.LintReport

func executeBeadsLint(cmd *cobra.Command, args []string) error {
	if !beadsTrackerAvailable() {
		fmt.Fprintln(os.Stderr, "WARN: bd not on PATH — skipping lint (graceful degradation)")
		return nil
	}
	ids, err := listBeadIDs(beadsLintStatus)
	if err != nil {
		return err
	}
	report := &LintReport{StatusFilter: beadsLintStatus, TotalBeads: len(ids)}
	for _, id := range ids {
		vr, err := verifyBead(id)
		if err != nil {
			report.ErrorBeads++
			continue
		}
		report.PerBead = append(report.PerBead, *vr)
		if vr.StaleCount > 0 {
			report.StaleBeads++
		} else {
			report.CleanBeads++
		}
	}
	if beadsLintJSON {
		if err := emitJSON(os.Stdout, report); err != nil {
			return err
		}
	} else {
		emitLintHuman(os.Stdout, report)
	}
	if report.StaleBeads > 0 {
		if cmd != nil {
			cmd.SilenceErrors = true
		}
		return &beadsVerdictError{code: 1}
	}
	return nil
}

// listBeadIDs extracts a list of bead IDs from `bd list --status=<filter>`.
//
// The bd list output uses several shapes depending on bead state and
// hierarchy. Examples (observed 2026-04-11):
//
//	○ na-h61 · TITLE    [OPEN ...]                 // bd show style
//	✓ na-0g5 ● P1 task Integrate behavioral...     // bd list flat
//	├── ✓ na-348.1 ● P1 task Retro ...             // bd list tree child
//	└── ✓ na-348.2 ● P1 task Nightly ...           // bd list tree child
//
// We match against a permissive rig-id pattern (`<rig>-<suffix>`) anywhere
// on the line after optional tree chars + bullet — this is robust to
// future bd output tweaks.
func listBeadIDs(status string) ([]string, error) {
	raw, err := beadsTrackerOutput("list", "--status", status)
	if err != nil {
		return nil, fmt.Errorf("bd list: %w", err)
	}
	return beadsapp.ParseBeadIDs(raw), nil
}

func emitLintHuman(w *os.File, r *LintReport) {
	fmt.Fprintf(w, "ao beads lint (status=%s): %d beads\n", r.StatusFilter, r.TotalBeads)
	fmt.Fprintf(w, "  %d clean, %d stale, %d errors\n", r.CleanBeads, r.StaleBeads, r.ErrorBeads)
	for _, vr := range r.PerBead {
		if vr.StaleCount == 0 {
			continue
		}
		fmt.Fprintf(w, "\n  [STALE] %s: %s\n", vr.BeadID, vr.Title)
		for _, c := range vr.Citations {
			if c.Status != CitationStale {
				continue
			}
			fmt.Fprintf(w, "    - %s: %s\n", c.Raw, c.Reason)
		}
	}
}

// ------------------------------------------------------------------------
// harvest
// ------------------------------------------------------------------------

// LearningFrontmatter is the yaml frontmatter block written to the top of
// each materialised learning file. Intentionally minimal — downstream
// reducers handle enrichment.
type LearningFrontmatter = beadsapp.LearningFrontmatter

func executeBeadsHarvest(cmd *cobra.Command, args []string) error {
	beadID := args[0]
	if !beadsTrackerAvailable() {
		fmt.Fprintln(os.Stderr, "WARN: bd not on PATH — skipping harvest (graceful degradation)")
		return nil
	}
	raw, err := beadsTrackerOutput("show", beadID)
	if err != nil {
		return fmt.Errorf("bd show %s: %w", beadID, err)
	}
	parsed, err := parseBDShow(string(raw))
	if err != nil {
		return err
	}
	if !isClosedStatus(parsed.Status) {
		return fmt.Errorf("bead %s is not CLOSED (status=%q) — harvest only materialises closed beads", beadID, parsed.Status)
	}

	fm := LearningFrontmatter{
		Title:      parsed.Title,
		BeadID:     beadID,
		Source:     "bd-close",
		Date:       time.Now().UTC().Format("2006-01-02"),
		Tags:       []string{"bead-closure", "auto-harvested"},
		Maturity:   "provisional",
		Provenance: fmt.Sprintf("bd show %s (harvested via `ao beads harvest`)", beadID),
	}

	body := renderLearningBody(fm, parsed)

	if beadsHarvestDryRun {
		fmt.Println(body)
		return nil
	}

	slug := beadSlugify(parsed.Title, 40)
	fname := fmt.Sprintf("%s-%s-%s.md", fm.Date, beadID, slug)
	target := filepath.Join(beadsHarvestOutDir, fname)

	if err := os.MkdirAll(beadsHarvestOutDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", beadsHarvestOutDir, err)
	}
	if _, err := os.Stat(target); err == nil {
		fmt.Fprintf(os.Stderr, "learning already exists at %s — not overwriting\n", target)
		return nil
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}
	fmt.Printf("harvested bead %s → %s\n", beadID, target)
	return nil
}

// renderLearningBody composes the markdown body for a harvested bead,
// including YAML frontmatter and the closure reason as the primary learning
// content.
func renderLearningBody(fm LearningFrontmatter, parsed *bdShowParsed) string {
	return beadsapp.RenderLearningBody(fm, parsed)
}

// isClosedStatus is tolerant of the various ways bd might render a closed
// state. Uses substring matching because the real status field often
// includes priority and type tokens (e.g., "● P2 · CLOSED" or "CLOSED P1 task").
func isClosedStatus(status string) bool {
	return beadsapp.IsClosedStatus(status)
}

// beadSlugify converts a free-text title into a filesystem-safe kebab-case slug
// capped at maxLen characters.
func beadSlugify(title string, maxLen int) string {
	return beadsapp.Slugify(title, maxLen)
}

// ------------------------------------------------------------------------
// shared helpers
// ------------------------------------------------------------------------

func emitJSON(w *os.File, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func beadTruncate(s string, n int) string {
	return beadsapp.Truncate(s, n)
}
