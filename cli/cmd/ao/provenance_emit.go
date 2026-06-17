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

// beadIDPattern matches an AgentOps tracker bead id: the literal `ag-` prefix,
// then an alphanumeric token that may carry dotted child suffixes
// (ag-62jrm, ag-tixgy, ag-agu.1). STRICT BY PREFIX, FAIL-CLOSED: the prior
// `[a-z]{2,}-…` form matched any hyphenated word, so prose inside the
// title-parens convention — "(council-ratified)", "(pre-mortem cleanup)" —
// emitted FALSE edges (bead_id="council-ratified"; 3 stray "pre-" edges already
// in the ledger). The agentops tracker is `ag-` only (265/265 ids); a foreign
// prefix (soc-, psite-) is another repo's bead, never an agentops landing, so
// it is deliberately NOT emitted here. Tightening the prefix is the fix
// (ag-62jrm / ag-5qltf): English compounds can no longer be mistaken for ids.
var beadIDPattern = `ag-[a-z0-9]+(?:\.[0-9]+)*`

var (
	// "Closes-scenario: <id>#slug" or "Closes: <id>" trailers — explicit,
	// machine-intended citations, recognized ANYWHERE in the message.
	closesTrailerRe = regexp.MustCompile(`(?mi)^\s*Closes(?:-scenario)?:\s*(` + beadIDPattern + `)`)
	// A parenthesized group opening at start-or-after-whitespace. Applied ONLY to
	// the SUBJECT line (see extractLandedBeadIDs) — body parens are prose and must
	// never become edges. The "(?:^|\s)" guard also excludes the conventional-
	// commit scope (`feat(cc-hooks):` — paren glued to the type word).
	subjectParenGroupRe = regexp.MustCompile(`(?:^|\s)\(([^)]*)\)`)
	// Every bead id inside a subject-paren group — supports comma-lists like
	// "(ag-7hdg0, ag-56cru)" and the "(<id> #slug)" form.
	beadIDRe = regexp.MustCompile(beadIDPattern)
)

// extractLandedBeadIDs returns the AgentOps bead ids a commit cites as landed,
// in true first-seen (text-position) order, de-duplicated. It reads only two
// STRUCTURED contexts and FAILS CLOSED everywhere else: (1) Closes/Closes-
// scenario trailers anywhere, and (2) the parenthesized convention on the
// SUBJECT LINE ONLY. Body prose — e.g. "(council-ratified)" or "(pre-mortem
// cleanup)" — is never read, which (with the strict ag- namespace) is the fix
// for the false-edge bug (ag-62jrm / ag-5qltf). Pure: no I/O, so it is the unit
// under test. Namespace validation is by the deliberate ag- prefix, NOT by any
// observed ledger data (the ledger is the contaminated surface here).
func extractLandedBeadIDs(msg string) []string {
	type hit struct {
		pos int
		id  string
	}
	var hits []hit
	// 1) Closes / Closes-scenario trailers — anywhere in the message.
	for _, m := range closesTrailerRe.FindAllStringSubmatchIndex(msg, -1) {
		hits = append(hits, hit{pos: m[2], id: msg[m[2]:m[3]]})
	}
	// 2) Subject-line parenthetical convention ONLY. Within each subject-paren
	// group, extract EVERY bead id so a comma-list yields all of them.
	subject := msg
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		subject = msg[:i]
	}
	for _, pm := range subjectParenGroupRe.FindAllStringSubmatchIndex(subject, -1) {
		innerStart, innerEnd := pm[2], pm[3] // capture group 1 = inner paren text
		inner := subject[innerStart:innerEnd]
		for _, im := range beadIDRe.FindAllStringIndex(inner, -1) {
			hits = append(hits, hit{pos: innerStart + im[0], id: inner[im[0]:im[1]]})
		}
	}
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
//
// It also stamps the additive (bead_id, merge_sha) mesh join keys (ag-5qltf,
// epic ag-w0wr2): bead_id is the universal yield↔provenance join key and
// merge_sha anchors this bead→commit hop (push-to-main: the full landed trunk
// OID). They denormalize from_id/to_id into first-class fields so the mesh join
// resolves without decoding edge endpoints — and are non-payload (see Edge), so
// stamping them does not perturb the hash chain.
func buildBeadCommitEdge(beadID, commitSHA string) provenancegraph.Edge {
	return provenancegraph.Edge{
		FromID:      beadID,
		FromType:    "bead",
		ToID:        commitSHA,
		ToType:      "commit",
		Relation:    "wasGeneratedBy",
		TrustTier:   "inferred",
		EvidenceRef: "commit " + commitSHA,
		BeadID:      beadID,
		MergeSHA:    commitSHA,
	}
}

var (
	provEmitCommit   string
	provEmitRange    string
	provEmitTrunkRef string
	provEmitJSON     bool
	provEmitDryRun   bool
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
	provenanceEmitLandedCmd.Flags().StringVar(&provEmitTrunkRef, "trunk-ref", "", "Require each commit to be an ancestor of this ref (e.g. origin/main) before emitting; merge_sha uses the resolved full OID")
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
	if strings.TrimSpace(provEmitTrunkRef) != "" {
		commits, err = filterCommitsOnTrunk(commits, provEmitTrunkRef)
		if err != nil {
			return err
		}
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

// filterCommitsOnTrunk drops commits that are not ancestors of trunkRef so
// merge_sha/to_id only record OIDs that actually exist on the shared trunk
// (age-0tn: pre-push local HEAD can diverge from origin/main after rewrites).
func filterCommitsOnTrunk(commits []landedCommit, trunkRef string) ([]landedCommit, error) {
	trunkRef = strings.TrimSpace(trunkRef)
	if trunkRef == "" {
		return commits, nil
	}
	if _, err := gitOutput(".", "rev-parse", "--verify", trunkRef); err != nil {
		return nil, fmt.Errorf("trunk-ref %q not resolvable: %w", trunkRef, err)
	}
	var kept []landedCommit
	for _, c := range commits {
		if _, err := gitOutput(".", "merge-base", "--is-ancestor", c.sha, trunkRef); err != nil {
			continue
		}
		kept = append(kept, c)
	}
	return kept, nil
}

// shortHash7 abbreviates a sha/hash to 7 chars for display.
func shortHash7(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
