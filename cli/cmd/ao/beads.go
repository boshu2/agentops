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
//	ao beads verify <id>     — stale-citation detector for a single bead
//	ao beads lint            — batch-verify every open bead
//	ao beads harvest <id>    — materialize a closed bead's reason into a learning
//
// The design goal is "pre-flight for inherited scope": catch bead
// descriptions that drift from HEAD before a new session acts on them. The
// planning rule at skills/plan/references/stale-scope-validation.md explains
// when to run these.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// execBD is the single entry point for shelling out to bd. Tests override
// this to avoid a hard dependency on the real binary. Production code calls
// `bd` via PATH; if absent, the caller emits a graceful warning and returns.
var execBD = func(args ...string) ([]byte, error) {
	cmd := exec.Command("bd", args...)
	return cmd.Output()
}

// bdAvailable reports whether the bd binary is reachable via PATH. Tests
// override this for deterministic behaviour.
var bdAvailable = func() bool {
	_, err := exec.LookPath("bd")
	return err == nil
}

// ------------------------------------------------------------------------
// Command wiring
// ------------------------------------------------------------------------

var beadsCmd = &cobra.Command{
	Use:   "beads",
	Short: "Complementary tooling for the bd (beads) issue tracker",
	Long: `Commands that help maintain the bd issue tracker alongside the main
bd CLI. These tools focus on catching stale descriptions before a new
session acts on them and harvesting closure reasons into durable learnings.

None of these commands replace bd itself — they complement it.`,
}

var (
	beadsVerifyJSON    bool
	beadsVerifyVerbose bool
)

var beadsVerifyCmd = &cobra.Command{
	Use:   "verify <bead-id>",
	Short: "Detect stale citations in a bead description (files, functions, symbols)",
	Long: `Reads a bead description via 'bd show <id>' and checks every file
path, function reference, and backticked symbol against HEAD. Reports each
citation as FRESH, STALE, or UNKNOWN with a per-citation reason.

Intended use: before acting on a deferred bead or handoff reference, run
'ao beads verify <id>' to catch drift. See the planning rule at
skills/plan/references/stale-scope-validation.md for when this applies.

Exit codes:
  0 — all citations fresh (or bd unavailable; graceful degradation)
  1 — at least one stale citation detected
  2 — error invoking bd or parsing output`,
	Args: cobra.ExactArgs(1),
	RunE: runBeadsVerify,
}

var (
	beadsLintStatus string
	beadsLintJSON   bool
)

var beadsLintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Batch-verify every open bead (or filtered set) against HEAD",
	Long: `Runs 'ao beads verify' on every bead matching a status filter and
aggregates the results. Useful as a weekly audit or pre-release gate.

Exit codes:
  0 — all beads have fresh citations (or bd unavailable)
  1 — at least one bead has stale citations
  2 — error invoking bd`,
	RunE: runBeadsLint,
}

var (
	beadsHarvestOutDir string
	beadsHarvestDryRun bool
)

var beadsHarvestCmd = &cobra.Command{
	Use:   "harvest <bead-id>",
	Short: "Materialize a closed bead's reason as a structured learning file",
	Long: `Reads a closed bead via 'bd show <id>' and writes its closure reason
to .agents/learnings/YYYY-MM-DD-<bead-id>-<slug>.md with frontmatter so the
learning can be picked up by the knowledge flywheel.

Only works on beads in CLOSED state. Use after 'bd close <id> --reason "..."'
to promote the reason into the learnings pool.

Exit codes:
  0 — learning written (or already exists, or bd unavailable)
  2 — bead is not closed / error`,
	Args: cobra.ExactArgs(1),
	RunE: runBeadsHarvest,
}

func init() {
	beadsCmd.GroupID = "knowledge"
	rootCmd.AddCommand(beadsCmd)
	beadsCmd.AddCommand(beadsVerifyCmd)
	beadsCmd.AddCommand(beadsLintCmd)
	beadsCmd.AddCommand(beadsHarvestCmd)

	beadsVerifyCmd.Flags().BoolVar(&beadsVerifyJSON, "json", false,
		"Emit verification report as JSON instead of human-readable text")
	beadsVerifyCmd.Flags().BoolVar(&beadsVerifyVerbose, "verbose", false,
		"Include FRESH citations in the output (default: stale only)")

	beadsLintCmd.Flags().StringVar(&beadsLintStatus, "status", "open",
		"bd status filter (open, closed, all)")
	beadsLintCmd.Flags().BoolVar(&beadsLintJSON, "json", false,
		"Emit lint report as JSON")

	beadsHarvestCmd.Flags().StringVar(&beadsHarvestOutDir, "out-dir", ".agents/learnings",
		"Directory to write the learning file into")
	beadsHarvestCmd.Flags().BoolVar(&beadsHarvestDryRun, "dry-run", false,
		"Print the learning content to stdout without writing a file")
}

// ------------------------------------------------------------------------
// verify
// ------------------------------------------------------------------------

// CitationStatus is the three-valued verdict for a single citation extracted
// from a bead description.
type CitationStatus string

const (
	CitationFresh   CitationStatus = "FRESH"
	CitationStale   CitationStatus = "STALE"
	CitationUnknown CitationStatus = "UNKNOWN"
)

// Citation is a single verifiable reference pulled from a bead description.
type Citation struct {
	Kind     string         `json:"kind"`     // "file", "function", "symbol"
	Raw      string         `json:"raw"`      // verbatim text from description
	Status   CitationStatus `json:"status"`   // FRESH / STALE / UNKNOWN
	Reason   string         `json:"reason"`   // human-readable explanation
	Resolved string         `json:"resolved"` // HEAD location if resolved differently
}

// VerifyReport is the structured result of `ao beads verify`.
type VerifyReport struct {
	BeadID      string     `json:"bead_id"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	Citations   []Citation `json:"citations"`
	StaleCount  int        `json:"stale_count"`
	FreshCount  int        `json:"fresh_count"`
	TotalCount  int        `json:"total_count"`
	BDAvailable bool       `json:"bd_available"`
}

func runBeadsVerify(cmd *cobra.Command, args []string) error {
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
		os.Exit(1)
	}
	return nil
}

// verifyBead shells out to bd, parses the description, extracts citations,
// and verifies each against HEAD. Returns a report regardless of verdict;
// callers decide what to do with StaleCount.
func verifyBead(beadID string) (*VerifyReport, error) {
	if !bdAvailable() {
		return &VerifyReport{BeadID: beadID, BDAvailable: false}, nil
	}
	raw, err := execBD("show", beadID)
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
type bdShowParsed struct {
	ID          string
	Title       string
	Status      string
	Description string // DESCRIPTION-heading body (open beads, empty on closed)
	CloseReason string // "Close reason:" line (closed beads only)
}

// Body returns the non-empty body to use for citation extraction / harvest.
// Prefers CloseReason (closed beads) and falls back to Description.
func (p *bdShowParsed) Body() string {
	if p.CloseReason != "" {
		return p.CloseReason
	}
	return p.Description
}

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
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty bd show output")
	}
	out := &bdShowParsed{}
	lines := strings.Split(raw, "\n")
	headerRe := regexp.MustCompile(`^[○●✓]?\s*(\S+)\s*·\s*(.*?)\s*\[([^\[\]]*)\]\s*$`)
	for i, line := range lines {
		if out.ID == "" {
			if m := headerRe.FindStringSubmatch(line); m != nil {
				out.ID = strings.TrimSpace(m[1])
				out.Title = strings.TrimSpace(m[2])
				out.Status = strings.TrimSpace(m[3])
				continue
			}
		}
		if strings.HasPrefix(line, "Close reason:") {
			out.CloseReason = strings.TrimSpace(strings.TrimPrefix(line, "Close reason:"))
			continue
		}
		if strings.HasPrefix(line, "DESCRIPTION") {
			// Everything until EOF or [rerun: ...] sentinel.
			tail := strings.Join(lines[i+1:], "\n")
			if idx := strings.LastIndex(tail, "\n[rerun:"); idx >= 0 {
				tail = tail[:idx]
			}
			// Also strip a leading [rerun:] if the description body is empty
			// (closed beads often have just this marker).
			tail = strings.TrimSpace(tail)
			if strings.HasPrefix(tail, "[rerun:") {
				tail = ""
			}
			out.Description = tail
			break
		}
	}
	if out.ID == "" && out.Description == "" && out.CloseReason == "" {
		return nil, fmt.Errorf("could not parse bd show output: %q", beadTruncate(raw, 80))
	}
	return out, nil
}

// extractCitations pulls verifiable references out of a description body.
// Three kinds are recognised:
//   - File paths (with optional :line suffix)
//   - Go function references (`func Name(` or `type.Method(`)
//   - Backticked symbols that look like identifiers
func extractCitations(desc string) []Citation {
	var out []Citation
	seen := make(map[string]bool)

	// File paths. Accept common source/doc extensions. Allow an optional
	// ":line" suffix. Lead-dot paths (.agents/, .github/) are captured by
	// requiring the character before the match to be non-ident, then
	// including the dot in the path prefix.
	fileRe := regexp.MustCompile(`(?:^|[^\w.])([.\w][\w./-]*\.(?:go|py|sh|md|yaml|yml|json|ts|tsx|js|jsx|rs|toml))(?::(\d+))?`)
	for _, m := range fileRe.FindAllStringSubmatch(desc, -1) {
		path := m[1]
		line := m[2]
		raw := path
		if line != "" {
			raw = path + ":" + line
		}
		key := "file:" + raw
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Citation{Kind: "file", Raw: raw})
	}

	// Go function citations: `func Name` or `func (r *T) Name`.
	funcRe := regexp.MustCompile(`\bfunc\s+(?:\([^)]*\)\s*)?([A-Za-z_]\w*)`)
	for _, m := range funcRe.FindAllStringSubmatch(desc, -1) {
		raw := "func " + m[1]
		key := "func:" + m[1]
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Citation{Kind: "function", Raw: raw})
	}

	// Backticked symbols that look like identifiers (not arbitrary code blocks).
	// Require at least one alpha char and only ident-safe chars.
	backtickRe := regexp.MustCompile("`([A-Za-z_][\\w.]{2,})`")
	for _, m := range backtickRe.FindAllStringSubmatch(desc, -1) {
		sym := m[1]
		// Skip things that look like file paths (already handled) or numbers.
		if strings.ContainsAny(sym, "/") {
			continue
		}
		key := "sym:" + sym
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Citation{Kind: "symbol", Raw: "`" + sym + "`"})
	}

	// Deterministic ordering for test stability.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Raw < out[j].Raw
	})
	return out
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
type LintReport struct {
	StatusFilter string         `json:"status_filter"`
	TotalBeads   int            `json:"total_beads"`
	CleanBeads   int            `json:"clean_beads"`
	StaleBeads   int            `json:"stale_beads"`
	ErrorBeads   int            `json:"error_beads"`
	PerBead      []VerifyReport `json:"per_bead"`
}

func runBeadsLint(cmd *cobra.Command, args []string) error {
	if !bdAvailable() {
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
		os.Exit(1)
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
	raw, err := execBD("list", "--status", status)
	if err != nil {
		return nil, fmt.Errorf("bd list: %w", err)
	}
	// Bead ID grammar: 2-4 letter rig, dash, then ident chars. Examples:
	// na-h61, na-348.1, ocpcm2-abc.
	idRe := regexp.MustCompile(`\b([a-z]{2,6}-[0-9a-z][\w.]*)\b`)
	seen := make(map[string]bool)
	var ids []string
	for _, line := range strings.Split(string(raw), "\n") {
		// Skip header/footer lines that might match incidentally — we want
		// only lines that start with a bullet or tree char.
		trimmed := strings.TrimLeft(line, " \t├─└│")
		if trimmed == "" {
			continue
		}
		firstRune := []rune(trimmed)[0]
		if firstRune != '○' && firstRune != '●' && firstRune != '✓' {
			continue
		}
		if m := idRe.FindStringSubmatch(line); m != nil {
			id := m[1]
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids, nil
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
type LearningFrontmatter struct {
	Title      string   `json:"title" yaml:"title"`
	BeadID     string   `json:"bead_id" yaml:"bead_id"`
	Source     string   `json:"source" yaml:"source"` // "bd-close"
	Date       string   `json:"date" yaml:"date"`
	Tags       []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Maturity   string   `json:"maturity" yaml:"maturity"` // "provisional" — fresh harvest
	Provenance string   `json:"provenance" yaml:"provenance"`
}

func runBeadsHarvest(cmd *cobra.Command, args []string) error {
	beadID := args[0]
	if !bdAvailable() {
		fmt.Fprintln(os.Stderr, "WARN: bd not on PATH — skipping harvest (graceful degradation)")
		return nil
	}
	raw, err := execBD("show", beadID)
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
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", fm.Title)
	fmt.Fprintf(&b, "bead_id: %s\n", fm.BeadID)
	fmt.Fprintf(&b, "source: %s\n", fm.Source)
	fmt.Fprintf(&b, "date: %s\n", fm.Date)
	fmt.Fprintf(&b, "maturity: %s\n", fm.Maturity)
	fmt.Fprintf(&b, "provenance: %q\n", fm.Provenance)
	b.WriteString("tags:\n")
	for _, t := range fm.Tags {
		fmt.Fprintf(&b, "  - %s\n", t)
	}
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", fm.Title)
	fmt.Fprintf(&b, "Harvested from closed bead [%s] on %s.\n\n", fm.BeadID, fm.Date)
	b.WriteString("## Closure reason\n\n")
	b.WriteString(parsed.Body())
	b.WriteString("\n")
	return b.String()
}

// isClosedStatus is tolerant of the various ways bd might render a closed
// state. Uses substring matching because the real status field often
// includes priority and type tokens (e.g., "● P2 · CLOSED" or "CLOSED P1 task").
func isClosedStatus(status string) bool {
	s := strings.ToUpper(strings.TrimSpace(status))
	for _, tok := range []string{"CLOSED", "DONE", "RESOLVED"} {
		if strings.Contains(s, tok) {
			return true
		}
	}
	return false
}

// beadSlugify converts a free-text title into a filesystem-safe kebab-case slug
// capped at maxLen characters.
func beadSlugify(title string, maxLen int) string {
	s := strings.ToLower(title)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if len(out) > maxLen {
		out = strings.TrimRight(out[:maxLen], "-")
	}
	if out == "" {
		out = "untitled"
	}
	return out
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
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
