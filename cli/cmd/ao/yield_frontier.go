// practices: [dora-metrics, trunk-based-development]
//
// The VERIFIED FRONTIER — the async-membrane governance line in
// `ao yield report` (age-fdae R1; resolution delegated to the frontier
// package in R3a, epic age-xnet).
//
// With async verification, commits land on origin/main BEFORE their verdicts
// bind. The frontier is the last-known-good sha: the highest first-parent
// (mainline) origin/main commit whose walked ancestry is RESOLVED. RESOLVED
// itself has exactly ONE implementation — cli/internal/frontier's
// uniform-precedence evaluator (CONFIRMED pawl verdict ∨ #trivial
// provenance-only waiver ∨ verified-by-compensation ∨ resolved-by-compensator,
// with REFUTED dominating every non-resolution arm). This file owns only the
// REPORT surface: snapshotting origin/main's COMMITTED ledger (never the
// worktree file — the age-fdae refute-fix), delegating to frontier.Compute,
// and rendering the pending window (short sha, bead id from the subject,
// age). The walk stays bounded at frontierMaxWalk commits (CAUTION,
// age-fdae); commits older than the horizon are assumed resolved.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/boshu2/agentops/cli/internal/frontier"
	"github.com/boshu2/agentops/cli/internal/provenancegraph"
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
	// the verdict→commit edges the frontier evaluator reads.
	provenanceLedgerRelPath = "docs/provenance/ledger.jsonl"
)

// frontierCommit is one origin/main mainline commit in the bounded walk — the
// display row source for the pending window (message-body reads live in the
// frontier evaluator, which reads the repo itself).
type frontierCommit struct {
	SHA     string
	Subject string
	When    time.Time
}

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

// loadOriginLedgerEdges snapshots the provenance ledger AS COMMITTED on
// origin/main (`git show <ref>:<path>`) and decodes it with the production
// reader. The frontier describes origin/main, so its verdict evidence must
// come from origin/main's COMMITTED ledger — never the worktree file, where
// an uncommitted CONFIRMED edge could certify a published commit the
// published ref carries no evidence for (the age-fdae refute-fix, preserved
// across the R3a delegation). A ref without the ledger (fresh repo) is an
// empty snapshot, not an error — fail-closed: no evidence, nothing resolves.
// A committed ledger that does not decode IS an error the report surfaces as
// a degraded frontier: the audit authority never silently drops a corrupt
// record (provenancegraph.Store.Read discipline).
func loadOriginLedgerEdges(root string) ([]provenancegraph.Edge, error) {
	cmd := exec.Command("git", "-C", root, "show", frontierRef+":"+provenanceLedgerRelPath) // #nosec G204 -- fixed argv over a well-known repo-relative path.
	out, err := cmd.Output()
	if err != nil {
		return nil, nil // no committed ledger on the ref: empty snapshot, frontier holds
	}
	edges, err := provenancegraph.DecodeEdges(bytes.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("committed ledger %s:%s: %w", frontierRef, provenanceLedgerRelPath, err)
	}
	return edges, nil
}

// listFrontierCommits walks origin/main newest-first, bounded at
// frontierMaxWalk, in ONE git call (%x1f field / %x1e record separators).
// --first-parent keeps the walk on the mainline — the frontier vouches for
// what landed on main, not for interior branch topology, and
// frontier.Compute's candidates share the same first-parent lineage.
func listFrontierCommits(root string) ([]frontierCommit, error) {
	cmd := exec.Command("git", "-C", root, "log", "--first-parent", // #nosec G204 nosemgrep -- root from resolveProjectDir; fixed read-only git query.
		"-n", strconv.Itoa(frontierMaxWalk), "--format=%H%x1f%ct%x1f%s%x1e", frontierRef)
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
		parts := strings.SplitN(rec, "\x1f", 3)
		if len(parts) != 3 {
			continue
		}
		var when time.Time
		if ct, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
			when = time.Unix(ct, 0).UTC()
		}
		out = append(out, frontierCommit{
			SHA:     parts[0],
			When:    when,
			Subject: strings.TrimRight(parts[2], "\n"),
		})
	}
	return out, nil
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

// buildFrontierSection computes the VERIFIED FRONTIER over origin/main by
// delegating RESOLVED to frontier.Compute — the single evaluator the close
// gate and the land lane share — then rendering the pending window as the
// mainline commits above the frontier sha. Read-only; a git failure (no repo,
// no origin/main) degrades to an error the report prints, never fatal. The
// git root is re-derived from the project dir so the section is correct from
// a subdirectory (mirroring repoRootOrCwd).
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
	edges, err := loadOriginLedgerEdges(gitRoot)
	if err != nil {
		return "", pending, err
	}
	res, err := frontier.Compute(gitRoot, edges, frontierRef, frontierMaxWalk)
	if err != nil {
		return "", pending, err
	}
	for _, c := range commits {
		if c.SHA == res.SHA {
			break
		}
		pending = append(pending, yieldReportPendingCommit{
			SHA:     c.SHA,
			Short:   shortSHA(c.SHA),
			Bead:    beadIDFromSubject(c.Subject),
			Subject: c.Subject,
			Age:     fmtReportAge(now.Sub(c.When)),
			TS:      rfc3339OrEmpty(c.When),
		})
	}
	return res.SHA, pending, nil
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
