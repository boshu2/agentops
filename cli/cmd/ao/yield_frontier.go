// practices: [dora-metrics, trunk-based-development]
//
// The VERIFIED FRONTIER — the async-membrane governance line in
// `ao yield report` (age-fdae, R1 of epic age-xnet).
//
// With async verification, commits land on origin/main BEFORE their verdicts
// bind. The frontier is the last-known-good sha: the highest origin/main
// commit whose walked ancestors ALL satisfy RESOLVED under the arms available
// today —
//
//  1. a CONFIRMED verdict→commit edge in docs/provenance/ledger.jsonl, or
//  2. the #trivial provenance-only waiver (the exact pawl_trivial_waiver
//     semantics from scripts/lib/trivial-waiver.sh, ported — never a parallel
//     re-derivation), DOMINATED by any REFUTED verdict on the commit.
//
// Every commit above the frontier is the PENDING WINDOW: landed, awaiting its
// verdict (short sha, bead id from the subject, age). Read-only computation
// over the provenance ledger + git ancestry; the walk is bounded at
// frontierMaxWalk commits (CAUTION, age-fdae) — commits older than the
// horizon are assumed resolved. Compensation arms arrive in R3a/R4: extend
// resolveCommitToday, nothing else.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

const (
	// frontierRef is the published ref the frontier walks — the async
	// membrane verifies what LANDED, so the remote-tracking main, never a
	// local branch.
	frontierRef = "origin/main"
	// frontierMaxWalk bounds the walk back from origin/main (CAUTION,
	// age-fdae): a fixed-cost read, not an unbounded ancestry scan. Commits
	// older than the horizon are assumed resolved.
	frontierMaxWalk = 200
	// provenanceLedgerRelPath is the repo-rooted provenance ledger holding
	// the verdict→commit edges the CONFIRMED arm reads.
	provenanceLedgerRelPath = "docs/provenance/ledger.jsonl"
)

// frontierCommit is one origin/main commit in the bounded walk.
type frontierCommit struct {
	SHA     string
	Subject string
	Body    string
	When    time.Time
}

// commitVerdicts is the verdict evidence the provenance ledger binds to one
// commit sha (a commit can carry both after a REFUTED→CONFIRMED re-review).
type commitVerdicts struct {
	Confirmed bool
	Refuted   bool
}

// frontierArm names which RESOLVED arm satisfied a commit ("" = unresolved).
type frontierArm string

const (
	frontierArmNone      frontierArm = ""
	frontierArmConfirmed frontierArm = "confirmed-verdict"
	frontierArmWaiver    frontierArm = "trivial-waiver"
)

// yieldReportPendingCommit is one origin/main commit above the verified
// frontier — landed, awaiting its verdict.
type yieldReportPendingCommit struct {
	SHA     string `json:"sha"`
	Short   string `json:"short_sha"`
	Bead    string `json:"bead,omitempty"`
	Subject string `json:"subject"`
	Age     string `json:"age"`
	TS      string `json:"ts,omitempty"`
}

// resolveCommitToday evaluates the RESOLVED predicate for one origin/main
// commit under the arms available TODAY (the R3a extensibility seam — future
// compensation arms extend THIS chain; callers depend only on "some arm
// resolved it"):
//
//  1. CONFIRMED verdict edge — a cross-family verdict bound to the commit in
//     the provenance ledger. An earlier REFUTED on the same commit does not
//     undo a later CONFIRMED (re-review supersedes).
//  2. #trivial provenance-only waiver — an author assertion, so any REFUTED
//     verdict on the commit DOMINATES it: refuted evidence beats asserted
//     triviality.
func resolveCommitToday(c frontierCommit, v commitVerdicts, waived func(frontierCommit) bool) frontierArm {
	if v.Confirmed {
		return frontierArmConfirmed
	}
	if !v.Refuted && waived(c) {
		return frontierArmWaiver
	}
	return frontierArmNone
}

// trivialMarkerSubjectRe / trivialMarkerBodyRe port the age-w2ny marker
// grammar from scripts/lib/trivial-waiver.sh verbatim: #trivial is a marker
// only as a TRAILING tag at the end of the subject or a standalone body line.
// A prose mention (mid-subject or in-body) never waives — that was the
// historical fail-open where any commit could bypass the pawl by naming
// #trivial.
var (
	trivialMarkerSubjectRe = regexp.MustCompile(`(?i)(^|[ \t])#trivial[ \t]*$`)
	trivialMarkerBodyRe    = regexp.MustCompile(`(?im)^[ \t]*#trivial[ \t]*$`)
)

// hasTrivialMarker reports whether the commit message carries an explicit
// #trivial marker per the age-w2ny grammar.
func hasTrivialMarker(subject, body string) bool {
	return trivialMarkerSubjectRe.MatchString(subject) || trivialMarkerBodyRe.MatchString(body)
}

// trivialWaiverDiffOK verifies the age-u43w arm of the waiver: the commit's
// diff touches ONLY docs/provenance/ paths. Fail-closed: a failed diff-tree
// or an empty changed-file list cannot prove triviality, so it does not
// waive. --no-renames forces a rename INTO docs/provenance/ to expose its
// non-provenance source path.
func trivialWaiverDiffOK(root, sha string) bool {
	cmd := exec.Command("git", "-C", root, "diff-tree", // #nosec G204 nosemgrep -- root from resolveProjectDir, sha from git rev-list output; fixed read-only git query.
		"--no-commit-id", "--no-renames", "--name-only", "-r", sha)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	changed := false
	for _, f := range strings.Split(string(out), "\n") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		changed = true
		if !strings.HasPrefix(f, "docs/provenance/") {
			return false
		}
	}
	return changed
}

// commitTriviallyWaived is the production waiver predicate: explicit marker
// AND provenance-only diff, both required.
func commitTriviallyWaived(root string) func(frontierCommit) bool {
	return func(c frontierCommit) bool {
		return hasTrivialMarker(c.Subject, c.Body) && trivialWaiverDiffOK(root, c.SHA)
	}
}

// provenanceVerdictEdge is the subset of one provenance-ledger line the
// frontier reads (real shape: from_id "<bead>@<sha7>", to_id full commit sha,
// disposition inside evidence_ref — see docs/provenance/ledger.jsonl).
type provenanceVerdictEdge struct {
	FromType    string `json:"from_type"`
	ToID        string `json:"to_id"`
	ToType      string `json:"to_type"`
	EvidenceRef string `json:"evidence_ref"`
}

// frontierDispositionRe extracts the verdict disposition from an
// evidence_ref like "pawl-verdict age-mv67 disposition=CONFIRMED".
var frontierDispositionRe = regexp.MustCompile(`disposition=([A-Z]+)`)

// loadCommitVerdicts indexes the provenance ledger's verdict→commit edges by
// commit sha. A missing ledger is an empty index, not an error (a fresh repo
// has no provenance yet); malformed lines are skipped — this is a read-only
// report, not a ledger validator.
func loadCommitVerdicts(root string) map[string]commitVerdicts {
	idx := map[string]commitVerdicts{}
	// The frontier describes origin/main, so its verdict evidence must come from
	// origin/main's COMMITTED ledger — never the worktree file, where an
	// uncommitted/unlanded CONFIRMED edge could certify a published commit the
	// published ref carries no evidence for (age-fdae refute-fix). Fail-closed:
	// if the ref-scoped read fails, no verdicts resolve and the frontier holds.
	cmd := exec.Command("git", "-C", root, "show", frontierRef+":"+provenanceLedgerRelPath) // #nosec G204 -- fixed argv over a well-known repo-relative path.
	out, err := cmd.Output()
	if err != nil {
		return idx
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e provenanceVerdictEdge
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		if e.FromType != "verdict" || e.ToType != "commit" {
			continue
		}
		m := frontierDispositionRe.FindStringSubmatch(e.EvidenceRef)
		if m == nil {
			continue
		}
		key := frontierSHAKey(e.ToID)
		if key == "" {
			continue
		}
		v := idx[key]
		switch m[1] {
		case "CONFIRMED":
			v.Confirmed = true
		case "REFUTED":
			v.Refuted = true
		}
		idx[key] = v
	}
	return idx
}

// frontierSHAKey normalizes a commit sha to the 12-hex-prefix key the verdict
// index uses, tolerating abbreviated shas in ledger to_id fields.
func frontierSHAKey(sha string) string {
	sha = strings.ToLower(strings.TrimSpace(sha))
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// listFrontierCommits walks origin/main newest-first, bounded at
// frontierMaxWalk, in ONE git call (%x1f field / %x1e record separators so
// multi-line bodies survive). --first-parent keeps the walk on the mainline —
// the frontier vouches for what landed on main, not for interior branch
// topology.
func listFrontierCommits(root string) ([]frontierCommit, error) {
	cmd := exec.Command("git", "-C", root, "log", "--first-parent", // #nosec G204 nosemgrep -- root from resolveProjectDir; fixed read-only git query.
		"-n", strconv.Itoa(frontierMaxWalk), "--format=%H%x1f%ct%x1f%s%x1f%b%x1e", frontierRef)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			if i := strings.IndexByte(msg, '\n'); i >= 0 {
				msg = msg[:i]
			}
			return nil, fmt.Errorf("git log %s: %s", frontierRef, msg)
		}
		return nil, fmt.Errorf("git log %s: %w", frontierRef, err)
	}
	records := strings.Split(stdout.String(), "\x1e")
	out := make([]frontierCommit, 0, len(records))
	for _, rec := range records {
		rec = strings.TrimLeft(rec, "\n")
		if strings.TrimSpace(rec) == "" {
			continue
		}
		parts := strings.SplitN(rec, "\x1f", 4)
		if len(parts) != 4 {
			continue
		}
		var when time.Time
		if ct, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			when = time.Unix(ct, 0).UTC()
		}
		out = append(out, frontierCommit{
			SHA:     parts[0],
			When:    when,
			Subject: parts[2],
			Body:    strings.TrimRight(parts[3], "\n"),
		})
	}
	return out, nil
}

// computeFrontier finds the LKG frontier within the bounded walk: the highest
// (newest) commit at-or-below which EVERY walked commit is RESOLVED. Input is
// newest-first (git log order). Returns the frontier sha ("" when even the
// oldest walked commit is unresolved) and the pending window: every commit
// above the frontier, newest first — including already-resolved commits above
// an unresolved ancestor, whose LKG status is not yet reachable.
func computeFrontier(commits []frontierCommit, resolve func(frontierCommit) frontierArm) (string, []frontierCommit) {
	oldestUnresolved := -1
	for i := len(commits) - 1; i >= 0; i-- {
		if resolve(commits[i]) == frontierArmNone {
			oldestUnresolved = i
			break
		}
	}
	switch {
	case len(commits) == 0:
		return "", nil
	case oldestUnresolved == -1:
		return commits[0].SHA, nil
	case oldestUnresolved+1 < len(commits):
		return commits[oldestUnresolved+1].SHA, commits[:oldestUnresolved+1]
	default:
		return "", commits
	}
}

// frontierBeadParenRe / frontierBeadForRe extract the bead id from a commit
// subject: the conventional trailing "(age-xxxx)" group, else the
// "for <bead>" form bind commits use. An id is lowercase alnum segments
// joined by hyphens with an optional dotted-child suffix — at least one
// hyphen, so a conventional-commit scope like "(yield)" never matches.
var (
	frontierBeadParenRe = regexp.MustCompile(`\(([a-z][a-z0-9]*(?:-[a-z0-9]+)+(?:\.[0-9]+)*)\)`)
	frontierBeadForRe   = regexp.MustCompile(`\bfor[ \t]+([a-z][a-z0-9]*(?:-[a-z0-9]+)+(?:\.[0-9]+)*)`)
)

// beadIDFromSubject extracts the bead id from a commit subject, or "" when
// none is recognizable (rendered as "-" in the pending table).
func beadIDFromSubject(subject string) string {
	if ms := frontierBeadParenRe.FindAllStringSubmatch(subject, -1); len(ms) > 0 {
		return ms[len(ms)-1][1]
	}
	if m := frontierBeadForRe.FindStringSubmatch(subject); m != nil {
		return m[1]
	}
	return ""
}

// buildFrontierSection computes the VERIFIED FRONTIER over origin/main:
// frontier sha + pending-window rows. Read-only; a git failure (no repo, no
// origin/main) degrades to an error the report prints, never fatal. The git
// root is re-derived from the project dir so the section is correct from a
// subdirectory (mirroring repoRootOrCwd).
func buildFrontierSection(root string, now time.Time) (string, []yieldReportPendingCommit, error) {
	pending := []yieldReportPendingCommit{}
	gitRoot := root
	if top, err := resolveRepoRoot(root); err == nil && top != "" {
		gitRoot = top
	}
	commits, err := listFrontierCommits(gitRoot)
	if err != nil {
		return "", pending, err
	}
	verdicts := loadCommitVerdicts(gitRoot)
	waived := commitTriviallyWaived(gitRoot)
	resolve := func(c frontierCommit) frontierArm {
		return resolveCommitToday(c, verdicts[frontierSHAKey(c.SHA)], waived)
	}
	frontierSHA, pendingCommits := computeFrontier(commits, resolve)
	for _, c := range pendingCommits {
		pending = append(pending, yieldReportPendingCommit{
			SHA:     c.SHA,
			Short:   shortSHA(c.SHA),
			Bead:    beadIDFromSubject(c.Subject),
			Subject: c.Subject,
			Age:     fmtReportAge(now.Sub(c.When)),
			TS:      rfc3339OrEmpty(c.When),
		})
	}
	return frontierSHA, pending, nil
}

// renderFrontierText writes the VERIFIED FRONTIER section of the text report:
// the ✓ line when the frontier IS origin/main, else the frontier sha plus one
// pending row per commit awaiting its verdict.
func renderFrontierText(out io.Writer, doc yieldReportDoc) error {
	fmt.Fprintf(out, "\nVERIFIED FRONTIER — last-known-good %s\n", frontierRef)
	switch {
	case doc.FrontierError != "":
		fmt.Fprintf(out, "  ⚠ frontier unavailable: %s\n", doc.FrontierError)
		return nil
	case len(doc.Pending) == 0 && doc.FrontierSHA != "":
		fmt.Fprintf(out, "  frontier == origin/main ✓ (%s)\n", shortSHA(doc.FrontierSHA))
		return nil
	}
	if doc.FrontierSHA == "" {
		fmt.Fprintf(out, "  frontier: none — no fully-resolved commit within the last %d walked commits\n", frontierMaxWalk)
	} else {
		fmt.Fprintf(out, "  frontier: %s\n", shortSHA(doc.FrontierSHA))
	}
	fmt.Fprintf(out, "  pending: %d commit(s) above the frontier awaiting verdicts\n", len(doc.Pending))
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "    SHA\tBEAD\tAGE\tSUBJECT")
	for _, p := range doc.Pending {
		bead := p.Bead
		if bead == "" {
			bead = "-"
		}
		fmt.Fprintf(tw, "    %s\t%s\t%s\t%s\n", p.Short, bead, p.Age, truncateReportText(p.Subject, 60))
	}
	return tw.Flush()
}
