package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// beadIDPattern matches a tracker bead id: a lowercase alpha prefix, a dash,
// then an alphanumeric token that may carry dotted child suffixes
// (ag-62jrm, ag-tixgy, soc-y8b.5, psite-agu.1). Anchored only inside the
// recognized citation contexts below — never matched against free prose — so
// ordinary hyphenated words (cross-context, follow-up) cannot become edges.
var beadIDPattern = `[a-z]{2,}-[a-z0-9]+(?:\.[0-9]+)*`

var (
	// "Closes-scenario: <id>#slug" or "Closes: <id>" trailers.
	closesTrailerRe = regexp.MustCompile(`(?mi)^\s*Closes(?:-scenario)?:\s*(` + beadIDPattern + `)`)
	// PR-title / subject parens convention: "(<id> #slug)" or bare "(<id>)".
	// The "(?:^|\s)" guard requires the open-paren to be at the start or after
	// whitespace, so a conventional-commit scope (`feat(cc-hooks):` — paren glued
	// to the type word) is NOT mistaken for a bead reference.
	titleParensRe = regexp.MustCompile(`(?:^|\s)\((` + beadIDPattern + `)(?:\s+#[^)]*)?\)`)
)

// extractLandedBeadIDs returns the bead ids a commit message cites as landed,
// in true first-seen (text-position) order, de-duplicated. It reads only the
// recognized citation contexts (Closes/Closes-scenario trailers and the
// parenthesized title convention) so prose that merely resembles an id is never
// captured. Pure: no I/O, so it is the unit under test.
func extractLandedBeadIDs(msg string) []string {
	type hit struct {
		pos int
		id  string
	}
	var hits []hit
	collect := func(re *regexp.Regexp) {
		for _, m := range re.FindAllStringSubmatchIndex(msg, -1) {
			// m[2],m[3] are the start/end of capture group 1 (the id).
			hits = append(hits, hit{pos: m[2], id: msg[m[2]:m[3]]})
		}
	}
	collect(closesTrailerRe)
	collect(titleParensRe)
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].pos < hits[j].pos })

	seen := map[string]bool{}
	var ids []string
	for _, h := range hits {
		if !seen[h.id] {
			seen[h.id] = true
			ids = append(ids, h.id)
		}
	}
	return ids
}

// buildBeadCommitEdge constructs the slice-1 SDLC edge: the bead was generated
// by the landing commit. trust_tier is "inferred" — the link is a deterministic
// observation of the commit, never a self-graded claim.
func buildBeadCommitEdge(beadID, commitSHA string) provenancegraph.Edge {
	return provenancegraph.Edge{
		FromID:      beadID,
		FromType:    "bead",
		ToID:        commitSHA,
		ToType:      "commit",
		Relation:    "wasGeneratedBy",
		TrustTier:   "inferred",
		EvidenceRef: "commit " + commitSHA,
	}
}

var (
	provEmitCommit string
	provEmitRange  string
	provEmitJSON   bool
	provEmitDryRun bool
)

var provenanceEmitLandedCmd = &cobra.Command{
	Use:   "emit-landed",
	Short: "Auto-emit bead→commit provenance edges for landed commits",
	Long: `Read the commit(s) being landed and append a schema-valid, hash-chained
provenance edge (bead --wasGeneratedBy--> commit) for every bead the commit
message cites via a Closes / Closes-scenario trailer or the parenthesized PR-title
convention. This is the milestone-1 SENSOR emitter (ag-62jrm): the ledger floor
was poured by ag-8jf97 but nothing fed it. Designed to run unattended from a
landing hook (e.g. the on-main CI job), so the navigator's position signal
accumulates without a human in the loop.

Idempotent: re-emitting the same bead/commit edge is a no-op. Honest trust tier:
edges are 'inferred' (a deterministic read of the commit), never self-graded.

Examples:
  ao provenance emit-landed --commit HEAD
  ao provenance emit-landed --commit "$GITHUB_SHA" --json
  ao provenance emit-landed --range origin/main..HEAD`,
	Args: cobra.NoArgs,
	RunE: runProvenanceEmitLanded,
}

func init() {
	provenanceCmd.AddCommand(provenanceEmitLandedCmd)
	provenanceEmitLandedCmd.Flags().StringVar(&provEmitCommit, "commit", "", "Single commit-ish to emit edges for (default HEAD when no --range)")
	provenanceEmitLandedCmd.Flags().StringVar(&provEmitRange, "range", "", "Git revision range (e.g. origin/main..HEAD); emits for every commit in it")
	provenanceEmitLandedCmd.Flags().BoolVar(&provEmitJSON, "json", false, "Emit appended edges as JSON")
	provenanceEmitLandedCmd.Flags().BoolVar(&provEmitDryRun, "dry-run", false, "Resolve and print edges without writing the ledger")
}

// landedCommit pairs a resolved commit sha with its message.
type landedCommit struct {
	sha string
	msg string
}

func runProvenanceEmitLanded(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	out := cmd.OutOrStdout()

	commits, err := resolveLandedCommits(provEmitRange, provEmitCommit)
	if err != nil {
		return err
	}

	store := provenancegraph.NewStore(resolveLedgerPath())
	appended := 0
	skipped := 0
	for _, c := range commits {
		for _, beadID := range extractLandedBeadIDs(c.msg) {
			edge := buildBeadCommitEdge(beadID, c.sha)
			edge.TS = time.Now().UTC().Format(time.RFC3339)
			if provEmitDryRun {
				fmt.Fprintf(out, "would emit %s --%s--> %s\n", edge.FromID, edge.Relation, shortHash7(edge.ToID))
				continue
			}
			res, appendErr := store.Append(edge)
			if appendErr != nil {
				return fmt.Errorf("emit edge for %s: %w", beadID, appendErr)
			}
			if res.Skipped {
				skipped++
				continue
			}
			appended++
			if !provEmitJSON {
				fmt.Fprintf(out, "emitted %s --%s--> %s\n", res.Edge.FromID, res.Edge.Relation, shortHash7(res.Edge.ToID))
			}
		}
	}
	if !provEmitDryRun && !provEmitJSON {
		fmt.Fprintf(out, "provenance emit-landed: %d appended, %d already present\n", appended, skipped)
	}
	return nil
}

// resolveLandedCommits returns the (sha, message) pairs to emit for. A --range
// wins; otherwise the single --commit (default HEAD) is used. Full 40-char
// shas are used as commit node ids so edges are stable and verifiable.
func resolveLandedCommits(rangeSpec, commit string) ([]landedCommit, error) {
	if strings.TrimSpace(rangeSpec) != "" {
		shas, err := gitOutput(".", "rev-list", "--reverse", rangeSpec)
		if err != nil {
			return nil, fmt.Errorf("rev-list %s: %w", rangeSpec, err)
		}
		var commits []landedCommit
		for _, sha := range strings.Fields(shas) {
			msg, msgErr := gitOutput(".", "show", "-s", "--format=%B", sha)
			if msgErr != nil {
				return nil, fmt.Errorf("read commit %s: %w", sha, msgErr)
			}
			commits = append(commits, landedCommit{sha: sha, msg: msg})
		}
		return commits, nil
	}

	ref := strings.TrimSpace(commit)
	if ref == "" {
		ref = "HEAD"
	}
	sha, err := gitOutput(".", "rev-parse", ref)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", ref, err)
	}
	sha = strings.TrimSpace(sha)
	msg, err := gitOutput(".", "show", "-s", "--format=%B", sha)
	if err != nil {
		return nil, fmt.Errorf("read commit %s: %w", sha, err)
	}
	return []landedCommit{{sha: sha, msg: msg}}, nil
}

// shortHash7 abbreviates a sha/hash to 7 chars for display.
func shortHash7(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
