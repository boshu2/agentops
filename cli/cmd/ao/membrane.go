// practices: [hexagonal-architecture, escape-corpus-self-improvement]
package main

import (
	"context"
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
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/domainsignal"
	"github.com/boshu2/agentops/cli/internal/ports"
	"github.com/boshu2/agentops/cli/internal/search"
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
	membraneDeriveRun            string
	membraneDeriveDryRun         bool
	membraneDeriveForce          bool
	membraneRecallDomain         string
	membraneRecallJSON           bool
	membraneRecallIncludeCatches bool
	membraneRecallPaths          []string

	membraneCatchBead     string
	membraneCatchDomain   string
	membraneCatchReason   string
	membraneCatchClass    string
	membraneCatchPaths    []string
	membraneCatchDetector string
	membraneCatchGlobs    string
	membraneCatchKind     string
	membraneCatchMode     string
	membraneCatchHead     string
	membraneCatchRun      string
	membraneCatchEvidence string
	membraneCatchScope    string

	membraneTriageJSON bool
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

var membraneCatchCmd = &cobra.Command{
	Use:   "catch --bead <id> (--domain <bc> --reason <what> | --evidence <file>) [--scope head|staged|upstream] [--class <slug>] [--paths f1,f2] [--detector-pattern <re> --globs <g> --detector-kind <k>]",
	Short: "Record a membrane CATCH — a REFUTED defect, as a structured class the membrane remembers",
	Long: `Record a catch out-of-band: a REFUTED gate-verdict carrying the bounded
context (--domain), what was caught (--reason), and the affected files (--paths),
plus an optional mechanical detector. Unlike an escape (a confirmed-then-refuted
PAIR, structurally rare), a catch is the ABUNDANT signal — every real REFUTE is
one. The catch is keyed by a catch-native class_key (computed at emit from
domain+reason[+detector]) so DetectCatches/recall can group recurring classes,
and carries affected_paths so even a judgment-class catch is path-recallable.
This is the manual twin of the pawl-review REFUTED branch. (epic age-zpj5, S2)

With --evidence <file> the reason/domain/paths are DERIVED (age-ulab — the Go
port of pawl-review's emit_pawl_catch): the reason via the two-tier REFUTED
salvage (the last 'VERDICT: REFUTED <text>' sentinel, else the first
substantive 'REFUTED: <finding>' prose line multi-family reviews emit, else a
placeholder); the domain from the first changed file's top path component; the
affected paths from git by --scope (the --head commit, or the index for
staged). Any explicit --reason/--domain/--paths wins over extraction.`,
	RunE: runMembraneCatch,
}

var membraneTriageCmd = &cobra.Command{
	Use:   "triage",
	Short: "Triage the catch corpus: the HONEST recurrence + compilability read (today: INSUFFICIENT-DATA)",
	Long: `Read the CATCH corpus and report whether the compiler thesis has fuel — with a
PRE-REGISTERED NUMERIC rule, never a fabricated number. Two axes over the catch
class_keys: Axis-1 RECURRENCE (recurring/distinct) and Axis-2 COMPILABILITY
(assessed_compilable / ALL recurring, via an all-instances TP-replay of each
class's detector). The decision: INSUFFICIENT-DATA (below the 15-class power floor
OR any recurring class unassessed) | MEMORY-ONLY (axis1<0.20) | CURATED (axis2<0.33)
| GO. Reason-less REFUTEDs are an UNCLASSIFIED floor — counted, never synthesized
into a class. Today the corpus is far below the floor, so the honest verdict is
INSUFFICIENT-DATA — which is the PROVE-FIRST point: do not build the compiler on
faith. (epic age-zpj5, S4)`,
	RunE: runMembraneTriage,
}

// defaultMembraneCalibrateScript is the standing calibration harness (age-e508.2):
// it runs the current COLD membrane against the FROZEN weak-producer trap corpus
// and emits a dated evidence file with verbatim per-trap outcomes, aggregate
// catch/false-refute rates, and an honest trend vs the prior run for that adapter.
const defaultMembraneCalibrateScript = "scripts/membrane-calibrate.sh"

var membraneCalibrateCmd = &cobra.Command{
	Use:   "calibrate [--membrane-label <adapter>] [--membrane-cmd <c>] [--out-dir <dir>]",
	Short: "Calibrate the cold membrane's catch-rate against the frozen trap corpus (the standing ruler)",
	Long: `Run the standing membrane calibration harness (age-e508.2): measure the current
COLD membrane against the FROZEN weak-producer trap corpus (evals/membrane/frozen/)
and write a dated evidence file with verbatim per-trap outcomes, aggregate
catch-rate + false-refute-rate, and an honest trend vs the prior run.

The producer arm is frozen code (not a stochastic model), so a run is reproducible
byte-for-byte and any change is attributable to the MEMBRANE. The reviewer is
pluggable via --membrane-cmd/--membrane-label, and each label keeps its OWN trend
history — so this is ALSO the instrument that calibrates a FALLBACK reviewer family
(duel D3). On an unchanged corpus a catch-rate drop is flagged REGRESSION plainly.

HONESTY (ADR-0011): this CALIBRATES the proven membrane; it is NOT evidence that
the escape-corpus compounds. Scheduling is substrate-delegated (ADR-0009): wire a
cron line to this command, never an in-repo daemon. Forwards all flags verbatim to
` + defaultMembraneCalibrateScript + `.`,
	DisableFlagParsing: true,
	RunE:               runMembraneCalibrate,
}

// runMembraneCalibrate forwards `ao membrane calibrate [args]` to
// scripts/membrane-calibrate.sh verbatim (streaming its stdout/stderr), gated by
// the same repo-trust boundary as the pawl live-script path, and propagating the
// script's exit code so a REGRESSION-driven nonzero (if the harness ever adopts
// one) surfaces unchanged. The frozen corpus + evidence output live in the repo.
func runMembraneCalibrate(cmd *cobra.Command, args []string) error {
	// With DisableFlagParsing, cobra forwards --help/-h into RunE instead of printing
	// help itself. Intercept it BEFORE the repo-trust check so the command-surface doc
	// generator (which captures `--help`) records the STATIC help, not a path-dependent
	// RCE-guard error — that leak made cli/docs/COMMANDS.md non-deterministic across
	// worktrees and left derived.changed-scope unsatisfiable (age-e508.2 land).
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return cmd.Help()
		}
	}
	repoRoot, err := resolveAgentsRepoRoot()
	if err != nil {
		return err
	}
	if !repoScriptTrusted(repoRoot) {
		return untrustedRepoScriptError(repoRoot, defaultMembraneCalibrateScript)
	}
	script := filepath.Join(repoRoot, defaultMembraneCalibrateScript)
	if _, statErr := os.Stat(script); statErr != nil {
		return fmt.Errorf("membrane calibrate script not found at %s: %w", script, statErr)
	}
	c := exec.Command("bash", append([]string{script}, args...)...) // #nosec G204 -- fixed in-repo script (trust-gated) + operator-supplied calibration flags.
	c.Dir = repoRoot
	c.Stdin = cmd.InOrStdin()
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	runErr := c.Run()
	if runErr == nil {
		return nil
	}
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return &pawlReviewExitError{code: exitErr.ExitCode()}
	}
	return runErr
}

func init() {
	membraneCmd.GroupID = "knowledge"
	rootCmd.AddCommand(membraneCmd)
	membraneCmd.AddCommand(membraneDeriveCmd)
	membraneCmd.AddCommand(membraneRecallCmd)
	membraneCmd.AddCommand(membraneCatchCmd)
	membraneCmd.AddCommand(membraneTriageCmd)
	membraneCmd.AddCommand(membraneCalibrateCmd)

	membraneDeriveCmd.Flags().StringVar(&membraneDeriveRun, "run", "", "Run id to scan for escapes (required)")
	membraneDeriveCmd.Flags().BoolVar(&membraneDeriveDryRun, "dry-run", false, "Report what would be derived without writing files")
	membraneDeriveCmd.Flags().BoolVar(&membraneDeriveForce, "force", false, "Overwrite existing derived artifacts")

	membraneRecallCmd.Flags().StringVar(&membraneRecallDomain, "domain", "", "Bounded-context / work-class tag to recall escapes for (required)")
	membraneRecallCmd.Flags().BoolVar(&membraneRecallJSON, "json", false, "Emit the recalled escapes as JSON")
	membraneRecallCmd.Flags().BoolVar(&membraneRecallIncludeCatches, "include-catches", false, "Also surface CATCH classes in the domain (the abundant memory; escapes are rare)")
	membraneRecallCmd.Flags().StringSliceVar(&membraneRecallPaths, "paths", nil, "With --include-catches, narrow to catches whose affected_paths overlap these files")

	membraneCatchCmd.Flags().StringVar(&membraneCatchBead, "bead", "", "Bead id the catch was found on (required)")
	membraneCatchCmd.Flags().StringVar(&membraneCatchDomain, "domain", "", "Bounded-context / work-class tag (required)")
	membraneCatchCmd.Flags().StringVar(&membraneCatchReason, "reason", "", "What was caught — the defect (required; the class reason when no --class given)")
	membraneCatchCmd.Flags().StringVar(&membraneCatchClass, "class", "", "Optional SEMANTIC class slug (e.g. stale-retired-surface). When set it keys the class CROSS-BEAD (the same label on different beads is ONE class), instead of the bead-drifting reason. Slug shape: lowercase [a-z0-9] words joined by '-'")
	membraneCatchCmd.Flags().StringSliceVar(&membraneCatchPaths, "paths", nil, "Concrete repo-relative file paths the catch touches (comma-separated or repeated)")
	membraneCatchCmd.Flags().StringVar(&membraneCatchDetector, "detector-pattern", "", "Optional regex that mechanically detects this class (makes it a compile candidate)")
	membraneCatchCmd.Flags().StringVar(&membraneCatchGlobs, "globs", "", "Optional path globs scoping the detector pattern")
	membraneCatchCmd.Flags().StringVar(&membraneCatchKind, "detector-kind", "", "Optional detector kind (e.g. regex)")
	membraneCatchCmd.Flags().StringVar(&membraneCatchMode, "mode", "", "Pawl diversity mode: fresh-context (default) | multi-model | deterministic")
	membraneCatchCmd.Flags().StringVar(&membraneCatchHead, "head", "", "Commit sha the catch was found at (default: git HEAD)")
	membraneCatchCmd.Flags().StringVar(&membraneCatchRun, "run", "", "Run id (default: membrane-catch)")
	membraneCatchCmd.Flags().StringVar(&membraneCatchEvidence, "evidence", "", "Pawl-review evidence file: derive --reason (two-tier REFUTED salvage), --domain (first changed file's top dir) and --paths (changed files, first 20); explicit flags win")
	membraneCatchCmd.Flags().StringVar(&membraneCatchScope, "scope", "head", "With --evidence: changed-file scope — head (the --head commit), staged (the index), or upstream (configured-upstream merge-base through --head)")

	membraneTriageCmd.Flags().BoolVar(&membraneTriageJSON, "json", false, "Emit the triage result as JSON")
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

// recallCatchesByDomain is the testable core of CATCH-keyed recall (epic age-zpj5,
// S3): load the ledger, DetectCatches, and return the catch classes in `domain` —
// optionally narrowed to catches whose affected_paths OVERLAP `paths` (set
// intersection). This is the abundant consumption side of membrane memory (escapes
// are structurally rare; catches are not): before reviewing a change in a bounded
// context and touching some files, recall the catches already made HERE so the
// reviewer does not re-derive them. ADVISORY only — never a gate input.
func recallCatchesByDomain(root, domain string, paths []string) ([]yieldledger.Catch, error) {
	ledger, err := yieldledger.Load(root)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, p := range paths {
		if p = strings.TrimSpace(p); p != "" {
			want[p] = true
		}
	}
	var out []yieldledger.Catch
	for _, c := range yieldledger.DetectCatches(ledger) {
		if c.Domain != domain {
			continue
		}
		if len(want) > 0 {
			overlap := false
			for _, p := range c.AffectedPaths {
				if want[p] {
					overlap = true
					break
				}
			}
			if !overlap {
				continue
			}
		}
		out = append(out, c)
	}
	return out, nil
}

// buildCatchInput assembles the GateVerdictInput for a structured catch: a REFUTED
// gate-verdict carrying domain+reason+affected_paths (+ optional detector). class_key
// is computed by the writer at emit; affected_paths/refuter_families are sanitized
// there. Pure (given head, ts, mode) so it is unit-testable without git/clock. (S2)
func buildCatchInput(bead, domain, reason string, paths []string, detector, globs, kind, mode, head, run string, ts time.Time) yieldledger.GateVerdictInput {
	if strings.TrimSpace(mode) == "" {
		mode = yieldledger.ModeFreshContext
	}
	if strings.TrimSpace(run) == "" {
		run = "membrane-catch"
	}
	return yieldledger.GateVerdictInput{
		BeadID:              bead,
		RunID:               run,
		TS:                  ts,
		Difficulty:          1,
		PawlVerdictRef:      yieldledger.PawlVerdictRef{BeadID: bead, HeadSHA: head},
		Disposition:         yieldledger.DispositionRefuted,
		HeadSHA:             head,
		Attempt:             1,
		Mode:                mode,
		AuthorContextID:     "ao-membrane-catch",
		AuthorFamily:        "manual",
		AuthorNeReviewer:    true,
		EvidencePresent:     true,
		Domain:              domain,
		Reason:              reason,
		DetectorPattern:     detector,
		ConstraintPathGlobs: globs,
		DetectorKind:        kind,
		AffectedPaths:       paths,
	}
}

// classSlugRe is the accepted shape for a --class semantic slug: one or more
// lowercase alphanumeric words joined by single '-' (no leading/trailing/double
// dash, no spaces or uppercase). It matches slugify's output so a validated slug
// survives ClassKeyFor's slugify unchanged — a class the operator can read back
// verbatim in triage. (age-jjt8)
var classSlugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// The two-tier REFUTED reason salvage (age-ulab — Go port of emit_pawl_catch's
// grep/sed in scripts/pawl-review.sh). Tier 1 keys on the single-reviewer
// `VERDICT: REFUTED <text>` sentinel; tier 2 (age-9931 parity) salvages the
// substantive `REFUTED: <finding>` prose line multi-family reviews emit,
// excluding the routed `PAWL <nonce> REFUTED` sentinel (which carries no
// reason) and any VERDICT line.
var (
	catchVerdictSentinelRe = regexp.MustCompile(`(?i)^[ \t]*VERDICT:[ \t]*REFUTED`)
	catchVerdictStripRe    = regexp.MustCompile(`(?i)^[ \t]*VERDICT:[ \t]*REFUTED[\s:—-]*`)
	catchRoutedSentinelRe  = regexp.MustCompile(`(?i)PAWL\s+r?[0-9a-fx]+\s+REFUTED`)
	catchProseRe           = regexp.MustCompile(`(?i)REFUTED:`)
	catchVerdictAnyRe      = regexp.MustCompile(`(?i)VERDICT:`)
	catchProseStripRe      = regexp.MustCompile(`(?i)^.*REFUTED[:\s]+`)
)

// catchReasonCap is the reason length cap (the bash `cut -c1-200`).
const catchReasonCap = 200

// extractRefutedReason applies the two-tier salvage to a pawl-review evidence
// body and returns the extracted reason, capped at catchReasonCap runes —
// or "" when neither tier hits (the caller applies the placeholder). Tier 1:
// the LAST `VERDICT: REFUTED <text>` sentinel line, prefix stripped. Tier 2
// (only when tier 1 yields no text — routed multi-family REFUTEs emit a bare
// sentinel): the FIRST `REFUTED: <finding>` prose line that is neither a
// routed `PAWL <nonce> REFUTED` sentinel nor a VERDICT line, stripped through
// its last REFUTED marker (bash sed greedy parity). (age-ulab)
func extractRefutedReason(evidence string) string {
	lines := strings.Split(evidence, "\n")
	// Tier 1: the last sentinel line only (bash `grep | tail -1`).
	for i := len(lines) - 1; i >= 0; i-- {
		if !catchVerdictSentinelRe.MatchString(lines[i]) {
			continue
		}
		if r := capReason(strings.TrimSpace(catchVerdictStripRe.ReplaceAllString(lines[i], ""))); r != "" {
			return r
		}
		break // bare sentinel — fall through to the tier-2 prose salvage
	}
	// Tier 2: the first substantive prose finding (bash `grep | grep -v | head -1`).
	for _, line := range lines {
		if !catchProseRe.MatchString(line) || catchRoutedSentinelRe.MatchString(line) || catchVerdictAnyRe.MatchString(line) {
			continue
		}
		return capReason(strings.TrimSpace(catchProseStripRe.ReplaceAllString(line, "")))
	}
	return ""
}

// capReason truncates s to at most catchReasonCap runes.
func capReason(s string) string {
	if utf8.RuneCountInString(s) <= catchReasonCap {
		return s
	}
	return string([]rune(s)[:catchReasonCap])
}

// changedFilesForCatch lists the changed files a catch covers, computed from
// git by scope exactly as pawl-review's emit_pawl_catch does: the files of the
// --head commit for scope=head, the index for scope=staged, or configured-
// upstream merge-base through --head for scope=upstream. BEST-EFFORT — nil
// on any git error (the caller falls back to the pawl-review domain): the
// catch is observability, not a gate. (age-ulab)
func changedFilesForCatch(root, scope, head string) []string {
	var args []string
	switch scope {
	case "staged":
		args = []string{"-c", "core.fsmonitor=", "-C", root, "diff", "--cached", "--no-ext-diff", "--no-textconv", "--name-only", "--no-color"}
	case "upstream":
		ref := strings.TrimSpace(head)
		if ref == "" {
			ref = "HEAD"
		}
		upstream, err := exec.Command("git", "-C", root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}").Output()
		if err != nil {
			return nil
		}
		base, err := exec.Command("git", "-C", root, "merge-base", strings.TrimSpace(string(upstream)), ref).Output()
		if err != nil || strings.TrimSpace(string(base)) == "" {
			return nil
		}
		args = []string{"-c", "core.fsmonitor=", "-C", root, "diff", strings.TrimSpace(string(base)) + ".." + ref, "--no-ext-diff", "--no-textconv", "--name-only", "--no-color"}
	default:
		ref := strings.TrimSpace(head)
		if ref == "" {
			ref = "HEAD"
		}
		args = []string{"-c", "core.fsmonitor=", "-C", root, "show", ref, "--no-ext-diff", "--no-textconv", "--name-only", "--format=", "--no-color"}
	}
	out, err := exec.Command("git", args...).Output() // #nosec G204 -- fixed git binary; ref is a commit sha from the local review.
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) != "" {
			files = append(files, line)
		}
	}
	return files
}

// catchDomainFromFiles resolves the catch domain: the first path component of
// the first changed file, falling back to "pawl-review" when nothing resolved
// (bash `head -1 | cut -d/ -f1` parity). (age-ulab)
func catchDomainFromFiles(files []string) string {
	if len(files) == 0 {
		return "pawl-review"
	}
	d := files[0]
	if i := strings.IndexByte(d, '/'); i >= 0 {
		d = d[:i]
	}
	if d == "" {
		return "pawl-review"
	}
	return d
}

// runMembraneCatch records a catch via the production Writer. With --evidence
// it additionally derives reason/domain/paths (age-ulab — the Go absorption of
// pawl-review.sh's emit_pawl_catch): reason via extractRefutedReason over the
// evidence file, domain + affected paths from git by --scope. Explicit
// --reason/--domain/--paths always win over extraction. (epic age-zpj5, S2)
func runMembraneCatch(cmd *cobra.Command, _ []string) error {
	evidencePath := strings.TrimSpace(membraneCatchEvidence)
	if strings.TrimSpace(membraneCatchBead) == "" {
		return fmt.Errorf("ao membrane catch: --bead is required")
	}
	if evidencePath == "" && (strings.TrimSpace(membraneCatchDomain) == "" || strings.TrimSpace(membraneCatchReason) == "") {
		return fmt.Errorf("ao membrane catch: --bead, --domain, and --reason are required (or pass --evidence to derive reason/domain/paths)")
	}
	scope := strings.TrimSpace(membraneCatchScope)
	if scope == "" {
		scope = "head"
	}
	if scope != "head" && scope != "staged" && scope != "upstream" {
		return fmt.Errorf("ao membrane catch: --scope must be head, staged, or upstream, got %q", scope)
	}
	class := strings.TrimSpace(membraneCatchClass)
	if class != "" && !classSlugRe.MatchString(class) {
		return fmt.Errorf("ao membrane catch: --class %q is not a valid slug (want lowercase [a-z0-9] words joined by '-', e.g. stale-retired-surface)", class)
	}
	// repoRootOrCwd (NOT resolveProjectDir) so a catch emitted from a repo subdir —
	// or by pawl-review.sh running from any cwd — lands in the REPO-ROOT yield ledger
	// where recall reads it, not a fragmented <cwd>/.agents/yield (age-6sg.1 class).
	root, err := repoRootOrCwd()
	if err != nil {
		return err
	}
	head := strings.TrimSpace(membraneCatchHead)
	if head == "" {
		head = gitHeadSHA(root)
	}
	if utf8.RuneCountInString(head) < 7 {
		return fmt.Errorf("ao membrane catch: need a >=7-char --head (commit sha); none resolved (pass --head or run inside a git repo)")
	}
	reason := strings.TrimSpace(membraneCatchReason)
	domain := strings.TrimSpace(membraneCatchDomain)
	paths := membraneCatchPaths
	if evidencePath != "" {
		if reason == "" {
			content, readErr := os.ReadFile(evidencePath) // #nosec G304 -- operator-supplied evidence path; read-only.
			if readErr != nil {
				// Fail-safe (emit_pawl_catch parity): an unreadable evidence file
				// must not LOSE the catch — warn and fall to the placeholder.
				fmt.Fprintf(cmd.ErrOrStderr(), "membrane: warning: cannot read --evidence %s: %v\n", evidencePath, readErr)
			}
			reason = extractRefutedReason(string(content))
			if reason == "" {
				reason = "pawl-review REFUTED (see evidence)"
			}
		}
		if domain == "" || len(paths) == 0 {
			files := changedFilesForCatch(root, scope, head)
			if domain == "" {
				domain = catchDomainFromFiles(files)
			}
			if len(paths) == 0 {
				if len(files) > 20 {
					files = files[:20] // bash `head -20` parity
				}
				paths = files
			}
		}
	}
	in := buildCatchInput(membraneCatchBead, domain, reason, paths,
		membraneCatchDetector, membraneCatchGlobs, membraneCatchKind, membraneCatchMode, head, membraneCatchRun, time.Now().UTC())
	in.Class = class
	w := yieldledger.Writer{}
	if _, err := w.AppendGateVerdict(root, in); err != nil {
		return fmt.Errorf("ao membrane catch: emit: %w", err)
	}
	ck := yieldledger.ClassKeyFor(domain, reason, membraneCatchDetector, class)
	fmt.Fprintf(cmd.OutOrStdout(), "membrane: recorded catch for %s@%s — class %s (domain=%s)\n", membraneCatchBead, head[:7], ck, domain)
	return nil
}

// gitHeadSHA returns the HEAD commit sha of the repo at root, or "" if unavailable.
func gitHeadSHA(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runMembraneTriage computes + prints the honest catch-corpus triage. (epic age-zpj5, S4)
func runMembraneTriage(cmd *cobra.Command, _ []string) error {
	root, err := repoRootOrCwd()
	if err != nil {
		return err
	}
	ledger, err := yieldledger.Load(root)
	if err != nil {
		return err
	}
	res := yieldledger.TriageCorpus(ledger, gitAssessCompilability(root))
	out := cmd.OutOrStdout()
	if membraneTriageJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	fmt.Fprintf(out, "Membrane triage — DECISION: %s\n", res.Decision)
	fmt.Fprintf(out, "  classes: %d distinct (%d with a stored reason), %d recurring; unclassified floor: %d\n",
		res.DistinctClasses, res.ClassesWithStoredReason, res.RecurringClasses, res.UnclassifiedFloor)
	fmt.Fprintf(out, "  axis-1 recurrence:    %.2f (recurring/distinct)\n", res.Axis1Recurrence)
	fmt.Fprintf(out, "  axis-2 compilability: %.2f (compilable/recurring); coverage %.0f%% (%d compilable, %d not, %d unassessed)\n",
		res.Axis2Compilable, res.Axis2Coverage*100, res.AssessedCompilable, res.AssessedNotCompilable, res.Unassessed)
	switch res.Decision {
	case yieldledger.DecisionInsufficientData:
		fmt.Fprintf(out, "  -> need >=%d stored-reason classes AND 100%% assessment coverage before a GO/NO-GO; do NOT build the compiler on faith.\n", yieldledger.TriagePowerFloor)
	case yieldledger.DecisionMemoryOnly:
		fmt.Fprintln(out, "  -> catches are one-off-dominated; keep them as recall-only MEMORY, no compiler.")
	case yieldledger.DecisionCurated:
		fmt.Fprintln(out, "  -> recurrence is real but mostly judgment-class; CURATED manual mechanical-only compiler behind shadow-bake.")
	case yieldledger.DecisionGo:
		fmt.Fprintln(out, "  -> enough mechanical recurrence to justify the auto-compiler tier (still shadow-baked).")
	}
	return nil
}

// gitAssessCompilability returns the all-instances TP-replay assessment closure: for a
// recurring class with a detector, replay it against the file content at EVERY stored bad
// instance (must hit all) AND the clean HEAD (must not hit). No detector -> not_compilable;
// any un-replayable instance -> unassessed (conservative: never a false GO). (epic age-zpj5, S4)
func gitAssessCompilability(root string) func(yieldledger.Catch) string {
	return func(c yieldledger.Catch) string {
		if strings.TrimSpace(c.DetectorPattern) == "" {
			return yieldledger.AssessNotCompilable
		}
		bad := make([]string, 0, len(c.Instances))
		for _, inst := range c.Instances {
			content, ok := gitShowFiles(root, inst.HeadSHA, inst.AffectedPaths)
			if !ok {
				return yieldledger.AssessUnassessed // un-replayable instance -> conservative
			}
			bad = append(bad, content)
		}
		clean, cleanOK := gitShowFiles(root, "HEAD", c.AffectedPaths)
		if !cleanOK {
			// Can't read clean HEAD (paths deleted/renamed) -> can't PROVE zero-FP ->
			// unassessed, never a vacuously-passing false compilable (codex S4 refute).
			return yieldledger.AssessUnassessed
		}
		return yieldledger.AssessCompilability(c.DetectorPattern, bad, clean)
	}
}

// gitShowFiles concatenates the content of paths at commit ref. Returns false when none of
// the paths can be read at ref (an un-replayable instance).
func gitShowFiles(root, ref string, paths []string) (string, bool) {
	if strings.TrimSpace(ref) == "" || len(paths) == 0 {
		return "", false
	}
	var b strings.Builder
	readAny := false
	for _, p := range paths {
		out, err := exec.Command("git", "-C", root, "show", ref+":"+p).Output()
		if err != nil {
			continue // a path absent at this ref is tolerated; need at least one readable
		}
		b.Write(out)
		b.WriteByte('\n')
		readAny = true
	}
	if !readAny {
		return "", false
	}
	return b.String(), true
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
	var catches []yieldledger.Catch
	if membraneRecallIncludeCatches {
		if catches, err = recallCatchesByDomain(root, domain, membraneRecallPaths); err != nil {
			return err
		}
	}
	out := cmd.OutOrStdout()
	if membraneRecallJSON {
		if escapes == nil {
			escapes = []yieldledger.Escape{} // emit [] not null for a clean no-match
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if membraneRecallIncludeCatches {
			if catches == nil {
				catches = []yieldledger.Catch{}
			}
			return enc.Encode(map[string]any{"escapes": escapes, "catches": catches})
		}
		return enc.Encode(escapes)
	}
	// Escapes (the existing surface — unchanged when --include-catches is off).
	if len(escapes) == 0 {
		fmt.Fprintf(out, "No past escapes recorded in domain %q — clean here (or no data yet).\n", domain)
	} else {
		fmt.Fprintf(out, "Membrane recall — %d past escape(s) in domain %q (look out for these here):\n\n", len(escapes), domain)
		for _, e := range escapes {
			missed := e.Missed
			if missed == "" {
				missed = "(no recorded reason)"
			}
			fmt.Fprintf(out, "- %s (run %s, refuted by %s): %s\n",
				e.BeadID, e.RunID, strings.Join(e.RefuterFamilies, ","), missed)
		}
	}
	// Catches (the abundant memory) — additive, only with --include-catches.
	if membraneRecallIncludeCatches {
		if len(catches) == 0 {
			fmt.Fprintf(out, "\nNo past catches recorded in domain %q.\n", domain)
		} else {
			fmt.Fprintf(out, "\nMembrane recall — %d past catch class(es) in domain %q (verify these do not recur):\n\n", len(catches), domain)
			for _, c := range catches {
				fmt.Fprintf(out, "- [%s] ×%d (%s) paths=%s: %s\n",
					c.ClassKey, c.HitCount, strings.Join(c.Beads, ","), strings.Join(c.AffectedPaths, ","), c.Reason)
			}
		}
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

	// cmd.Context() is nil when a test invokes the RunE directly (no SetContext);
	// exec.CommandContext panics on a nil context, so default to Background.
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	for _, e := range escapes {
		artifact := deriveFindingFromEscape(e, buildDomainRecord(ctx, root, e))
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
func deriveFindingFromEscape(e yieldledger.Escape, dom domainsignal.Record) ports.FindingArtifact {
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
	if e.Domain == yieldledger.DomainUnclassified {
		// EM.2.1: an UNCLASSIFIED escape is visible debt, never a real domain — do
		// not present it as a routable "look out for this here" signal; flag it for
		// classification so the derived check can eventually route by domain.
		b.WriteString("**Domain:** UNCLASSIFIED — ⚠ this escape was never classified; classify it (set --domain on the overturning REFUTED) so the derived check can route by domain.\n\n")
	} else if e.Domain != "" {
		fmt.Fprintf(&b, "**Domain:** %s — look out for this class of miss when working here.\n\n", e.Domain)
	}
	if e.Missed == yieldledger.ReasonUnspecified {
		// EM.2.1: an unspecified reason is visible debt, not a usable "what was
		// missed" — flag it for classification rather than rendering the placeholder.
		b.WriteString("**What was missed:** ⚠ unspecified — this escape's reason was never recorded; set --reason on the overturning REFUTED.\n\n")
	} else if e.Missed != "" {
		fmt.Fprintf(&b, "**What was missed:** %s\n\n", e.Missed)
	}
	// EM.2.2: the three-signal domain record. intent_domain (where the work was
	// meant to be) + changed_file_domains (where the code actually changed) are
	// PRESERVED alongside escape_domain so a cross-context escape is visible.
	if dom.IntentDomain != "" || len(dom.ChangedFileDomains) > 0 {
		b.WriteString("**Domain signals:** ")
		if dom.IntentDomain != "" {
			fmt.Fprintf(&b, "intended `%s`", dom.IntentDomain)
		}
		if len(dom.ChangedFileDomains) > 0 {
			fmt.Fprintf(&b, ", code changed in `%s`", strings.Join(dom.ChangedFileDomains, "`, `"))
		}
		b.WriteString(".\n\n")
		if dom.Mismatch {
			fmt.Fprintf(&b, "**⚠ DOMAIN MISMATCH:** the work was intended for `%s` but the code changed outside it (%s) — this escape crossed bounded contexts; weight the cross-domain blast radius.\n\n",
				dom.IntentDomain, strings.Join(dom.ChangedFileDomains, ", "))
		}
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
	// EM.2.2: the other two domain signals, queryable for cross-context analysis.
	if dom.IntentDomain != "" {
		art.Frontmatter["intent_domain"] = dom.IntentDomain
	}
	if len(dom.ChangedFileDomains) > 0 {
		art.Frontmatter["changed_file_domains"] = strings.Join(dom.ChangedFileDomains, ", ")
	}
	if dom.Mismatch {
		art.Frontmatter["domain_mismatch"] = "true"
	}
	// EM.2.10 — THE CUT WIRE, reconnected. When the escape carries a mechanical
	// detector (a re-introducible pattern + the paths it applies to), upgrade the
	// finding from advisory to MECHANICAL and add the constraint compile target, so
	// productionFindingCompiler -> search.BuildConstraintEntry emits a real draft
	// constraint into .agents/constraints/index.json that the gate enforces. Until
	// this, every escape was hardcoded advisory and the index stayed empty — the
	// membrane remembered escapes but never BLOCKED their re-introduction. A
	// process-gap escape (no detector) keeps the advisory pre-mortem path above.
	if e.DetectorPattern != "" {
		kind := e.DetectorKind
		if kind == "" {
			kind = "regex"
		}
		compiledAt := e.RefutedTS
		if compiledAt == "" {
			compiledAt = e.ConfirmedTS
		}
		art.Frontmatter["detectability"] = "mechanical"
		art.Frontmatter["detector_pattern"] = e.DetectorPattern
		art.Frontmatter["detector_kind"] = kind
		art.Frontmatter["constraint_path_globs"] = e.ConstraintPathGlobs
		art.Frontmatter["compiled_at"] = compiledAt
		art.Frontmatter["compiler_targets"] = string(ports.CompiledOutputPreMortemCheck) + "," + string(ports.CompiledOutputConstraint)
	}
	return art
}

// buildDomainRecord assembles the three-signal domain record for an escape at
// DERIVE time (EM.2.2, council Option C — no emit-time burden, no schema change).
// Both reads are BEST-EFFORT and degrade to "": a GC'd SHA yields no changed-file
// domains, and a bead without an intent tag yields no intent_domain. escape_domain
// comes from the escape itself (EM.2.1).
func buildDomainRecord(ctx context.Context, root string, e yieldledger.Escape) domainsignal.Record {
	return domainsignal.Build(
		beadIntentDomain(ctx, root, e.BeadID),
		escapeChangedFiles(ctx, root, e),
		e.Domain,
	)
}

// escapeChangedFiles returns the repo-relative paths that changed between the
// escape's confirmed and refuted heads. Empty on any error (e.g. a GC'd SHA).
func escapeChangedFiles(ctx context.Context, root string, e yieldledger.Escape) []string {
	if e.ConfirmedHeadSHA == "" || e.RefutedHeadSHA == "" {
		return nil
	}
	// #nosec G204 -- fixed git binary; refs are SHAs from the local yield ledger.
	out, err := exec.CommandContext(ctx, "git", "-C", root, "diff", "--name-only",
		e.ConfirmedHeadSHA+".."+e.RefutedHeadSHA).Output()
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths
}

// beadIntentDomain reads agent_context.intent_domain for a bead via `br show
// --json` (the agent_context value is itself a JSON string). Empty on any error
// or when the bead carries no intent tag.
func beadIntentDomain(ctx context.Context, root, beadID string) string {
	if beadID == "" {
		return ""
	}
	out, err := beadsTrackerCommandContextInDir(ctx, root, "show", beadID, "--json").Output()
	if err != nil {
		return ""
	}
	var rows []struct {
		AgentContext string `json:"agent_context"`
	}
	// br show --json may return an object or a single-element array; try both.
	var one struct {
		AgentContext string `json:"agent_context"`
	}
	ac := ""
	if json.Unmarshal(out, &rows) == nil && len(rows) > 0 {
		ac = rows[0].AgentContext
	} else if json.Unmarshal(out, &one) == nil {
		ac = one.AgentContext
	}
	if ac == "" {
		return ""
	}
	var parsed struct {
		IntentDomain string `json:"intent_domain"`
	}
	if json.Unmarshal([]byte(ac), &parsed) != nil {
		return ""
	}
	return parsed.IntentDomain
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
	wroteAny := false
	findingAbs := filepath.Join(root, ".agents", "findings", artifact.ID+".md")
	if wrote, err := writeFileTargetIdempotent(findingAbs, renderFindingArtifact(artifact), force); err != nil {
		return wroteAny, fmt.Errorf("write finding %s: %w", artifact.ID, err)
	} else if wrote {
		wroteAny = true
	}

	for _, out := range outputs {
		// A constraint output is a structured ConstraintEntry MERGED into the
		// shared .agents/constraints/index.json (the gate's executable surface),
		// not a standalone file — upsert under lock instead of file-replace.
		if out.Kind == ports.CompiledOutputConstraint {
			var entry search.ConstraintEntry
			if err := json.Unmarshal(out.Body, &entry); err != nil {
				return wroteAny, fmt.Errorf("decode compiled constraint for %s: %w", artifact.ID, err)
			}
			wrote, err := search.UpsertConstraintAt(root, entry, force)
			if err != nil {
				return wroteAny, fmt.Errorf("merge constraint %s: %w", entry.ID, err)
			}
			if wrote {
				wroteAny = true
			}
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(out.Path))
		if wrote, err := writeFileTargetIdempotent(abs, out.Body, force); err != nil {
			return wroteAny, fmt.Errorf("write compiled %s: %w", out.Path, err)
		} else if wrote {
			wroteAny = true
		}
	}
	return wroteAny, nil
}

// writeFileTargetIdempotent writes body to abs atomically, leaving an existing
// file untouched unless force is set (so a present artifact with a deleted
// sibling is repaired, not silently skipped). Returns whether it wrote.
func writeFileTargetIdempotent(abs string, body []byte, force bool) (bool, error) {
	if !force {
		if _, err := os.Stat(abs); err == nil {
			return false, nil
		}
	}
	if err := writeFindingFileAtomic(abs, body, 0o644); err != nil {
		return false, err
	}
	return true, nil
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
