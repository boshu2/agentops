// practices: [design-by-contract, in-toto-provenance]
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	doneadapter "github.com/boshu2/agentops/cli/internal/adapters/done"
	doneapp "github.com/boshu2/agentops/cli/internal/done"
	"github.com/boshu2/agentops/cli/internal/provenancegraph"
)

// Fixture shas: full 40-char commit OIDs with distinct 7-char prefixes.
const (
	doneSHAConfirmed = "1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a1a"
	doneSHARefuted   = "2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b"
	doneSHANoVerdict = "3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c"
)

func toDoneEdges(edges []provenancegraph.Edge) []doneapp.Edge {
	result := make([]doneapp.Edge, 0, len(edges))
	for _, edge := range edges {
		result = append(result, doneapp.Edge{FromID: edge.FromID, FromType: edge.FromType, ToID: edge.ToID,
			ToType: edge.ToType, Relation: edge.Relation, EvidenceRef: edge.EvidenceRef})
	}
	return result
}

// resetDoneFlags snapshots and restores the package-global done cobra flags
// (test-isolation rule: package-global flag vars must not leak across tests).
func resetDoneFlags(t *testing.T) {
	t.Helper()
	prevSHA, prevReason := doneSHA, doneReason
	prevForce, prevJSON := doneForceNoVerdct, doneJSON
	t.Cleanup(func() {
		doneSHA, doneReason = prevSHA, prevReason
		doneForceNoVerdct, doneJSON = prevForce, prevJSON
	})
	doneSHA, doneReason = "", "Done"
	doneForceNoVerdct, doneJSON = false, false
}

// stubBr installs a fake `br` binary at the FRONT of PATH that records its
// argv (one arg per line) and exits with the code in exit-code (default 0).
// BEADS_DIR is pinned to a throwaway tempdir so no test can ever touch a real
// ledger even if a real br leaked onto PATH. Returns the argv capture path.
func stubBr(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "br-argv")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > '" + argvFile + "'\n" +
		"if [ -f '" + filepath.Join(dir, "exit-code") + "' ]; then exit \"$(cat '" + filepath.Join(dir, "exit-code") + "')\"; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "br"), []byte(script), 0o755); err != nil {
		t.Fatalf("write br stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BEADS_DIR", t.TempDir())
	return argvFile
}

// readStubArgv reads the argv the br stub recorded, one arg per slice entry.
func readStubArgv(t *testing.T, argvFile string) []string {
	t.Helper()
	data, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("br stub never invoked (no argv file): %v", err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

// seedDoneLedger writes verdict fixtures THROUGH the production writer
// (buildVerdictCommitEdge + Store.Append — fixture-fidelity rule): one
// CONFIRMED verdict on sha-1a, one REFUTED-only verdict on sha-2b, and a
// landed-but-unreviewed edge on sha-3c.
func seedDoneLedger(t *testing.T) {
	t.Helper()
	store := provenancegraph.NewStore(resolveLedgerPath())
	edges := []provenancegraph.Edge{
		buildBeadCommitEdge("ag-done.1", doneSHAConfirmed),
		buildVerdictCommitEdge(pawlVerdict{
			BeadID: "ag-done.1", HeadSHA: doneSHAConfirmed, Disposition: "CONFIRMED"}),
		buildBeadCommitEdge("ag-done.2", doneSHARefuted),
		buildVerdictCommitEdge(pawlVerdict{
			BeadID: "ag-done.2", HeadSHA: doneSHARefuted, Disposition: "REFUTED"}),
		buildBeadCommitEdge("ag-done.3", doneSHANoVerdict),
	}
	for i, e := range edges {
		e.TS = "2026-07-01T00:00:0" + string(rune('0'+i)) + "Z"
		if _, err := store.Append(e); err != nil {
			t.Fatalf("seed edge %d: %v", i, err)
		}
	}
}

// makeDoneGitRepo builds a throwaway git repo in a tempdir whose HEAD commit
// touches exactly the given files (on top of a base commit — a root commit
// shows no files to plain diff-tree and would fail closed, matching the pawl
// gate's semantics), chdirs into it, and returns the HEAD sha.
func makeDoneGitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, ".init"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	for path, content := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	run("add", "-A")
	run("commit", "-q", "-m", "fixture")
	t.Chdir(dir)
	return run("rev-parse", "HEAD")
}

// TestDone_ConfirmedVerdictCloses: a CONFIRMED verdict bound to --sha closes
// the bead via br with the exact stamped close reason.
func TestDone_ConfirmedVerdictCloses(t *testing.T) {
	chdirRepoFixture(t)
	seedDoneLedger(t)
	resetDoneFlags(t)
	argvFile := stubBr(t)
	doneSHA = doneSHAConfirmed

	c, out := provTestCmd()
	if err := runDone(c, []string{"ag-done.1"}); err != nil {
		t.Fatalf("ao done with CONFIRMED verdict: %v", err)
	}

	wantArgv := []string{"close", "ag-done.1", "-r", "Done [verdict:1a1a1a1:CONFIRMED]"}
	got := readStubArgv(t, argvFile)
	if len(got) != len(wantArgv) {
		t.Fatalf("br argv = %q, want %q", got, wantArgv)
	}
	for i := range wantArgv {
		if got[i] != wantArgv[i] {
			t.Fatalf("br argv[%d] = %q, want %q", i, got[i], wantArgv[i])
		}
	}
	if !strings.Contains(out.String(), "closed ag-done.1 at 1a1a1a1 [verdict:1a1a1a1:CONFIRMED]") {
		t.Errorf("human output missing close line:\n%s", out.String())
	}
}

// TestDone_ShaPrefixResolves: a 7-char --sha prefix binds to the same verdict
// (the provenance_show prefix convention).
func TestDone_ShaPrefixResolves(t *testing.T) {
	chdirRepoFixture(t)
	seedDoneLedger(t)
	resetDoneFlags(t)
	argvFile := stubBr(t)
	doneSHA = doneSHAConfirmed[:7]

	c, _ := provTestCmd()
	if err := runDone(c, []string{"ag-done.1"}); err != nil {
		t.Fatalf("ao done with 7-char prefix: %v", err)
	}
	argv := readStubArgv(t, argvFile)
	if argv[3] != "Done [verdict:1a1a1a1:CONFIRMED]" {
		t.Fatalf("close reason = %q, want CONFIRMED stamp", argv[3])
	}
}

// TestDone_RefusalPaths: no verdict (and not waiver-eligible) or a
// non-CONFIRMED-only verdict must refuse with a corrective error naming
// ao verify, and br must never be invoked.
func TestDone_RefusalPaths(t *testing.T) {
	cases := []struct {
		name        string
		sha         string
		wantInError []string
	}{
		{
			name: "no verdict refuses naming ao verify",
			sha:  doneSHANoVerdict,
			wantInError: []string{
				"no verdict recorded for commit 3c3c3c3",
				"no verdict = not done",
				"ao verify ag-done.3",
				"ao pawl review ag-done.3",
				"--force-no-verdict",
			},
		},
		{
			name: "REFUTED-only verdict refuses naming the disposition",
			sha:  doneSHARefuted,
			wantInError: []string{
				"none CONFIRMED",
				"REFUTED",
				"ao verify",
			},
		},
		{
			name:        "short sha prefix is a usage error",
			sha:         doneSHANoVerdict[:6],
			wantInError: []string{"at least 7"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chdirRepoFixture(t)
			seedDoneLedger(t)
			resetDoneFlags(t)
			argvFile := stubBr(t)
			doneSHA = tc.sha

			c, _ := provTestCmd()
			beadID := "ag-done.3"
			if tc.sha == doneSHARefuted {
				beadID = "ag-done.2"
			}
			err := runDone(c, []string{beadID})
			if err == nil {
				t.Fatal("want refusal error, got nil")
			}
			for _, want := range tc.wantInError {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal error missing %q:\n%s", want, err.Error())
				}
			}
			if _, statErr := os.Stat(argvFile); statErr == nil {
				t.Error("br was invoked on a refused close — refusal must not close")
			}
		})
	}
}

// TestDone_ForceNoVerdictStampsUnverified: the escape hatch closes with an
// explicit, greppable UNVERIFIED stamp instead of blocking.
func TestDone_ForceNoVerdictStampsUnverified(t *testing.T) {
	chdirRepoFixture(t)
	seedDoneLedger(t)
	resetDoneFlags(t)
	argvFile := stubBr(t)
	doneSHA = doneSHANoVerdict
	doneForceNoVerdct = true
	doneReason = "Shipped anyway"

	c, out := provTestCmd()
	if err := runDone(c, []string{"ag-done.3"}); err != nil {
		t.Fatalf("ao done --force-no-verdict: %v", err)
	}
	argv := readStubArgv(t, argvFile)
	if argv[3] != "Shipped anyway [verdict:3c3c3c3:UNVERIFIED]" {
		t.Fatalf("close reason = %q, want UNVERIFIED stamp", argv[3])
	}
	if !strings.Contains(out.String(), "UNVERIFIED close") {
		t.Errorf("human output must flag the UNVERIFIED close:\n%s", out.String())
	}
}

// TestDone_WaivedTrivialProvenanceOnlyCommit: with no verdict, a commit whose
// changed files are ALL under docs/provenance/ closes with the waived-trivial
// stamp — resolved from HEAD (no --sha), covering the default-sha path.
func TestDone_WaivedTrivialProvenanceOnlyCommit(t *testing.T) {
	head := makeDoneGitRepo(t, map[string]string{
		"docs/provenance/ledger.jsonl": "",
	})
	resetDoneFlags(t)
	argvFile := stubBr(t)

	c, _ := provTestCmd()
	if err := runDone(c, []string{"ag-done.4"}); err != nil {
		t.Fatalf("ao done on provenance-only commit: %v", err)
	}
	argv := readStubArgv(t, argvFile)
	want := "Done [verdict:" + head[:7] + ":waived-trivial]"
	if argv[3] != want {
		t.Fatalf("close reason = %q, want %q", argv[3], want)
	}
}

// TestDone_NonProvenanceCommitNotWaived: a verdict-less commit touching any
// path outside docs/provenance/ must NOT waive — it refuses.
func TestDone_NonProvenanceCommitNotWaived(t *testing.T) {
	makeDoneGitRepo(t, map[string]string{
		"docs/provenance/ledger.jsonl": "",
		"README.md":                    "code change\n",
	})
	resetDoneFlags(t)
	argvFile := stubBr(t)

	c, _ := provTestCmd()
	err := runDone(c, []string{"ag-done.5"})
	if err == nil {
		t.Fatal("want refusal on mixed commit, got nil")
	}
	if !strings.Contains(err.Error(), "ao verify ag-done.5") {
		t.Errorf("refusal must name ao verify:\n%s", err.Error())
	}
	if _, statErr := os.Stat(argvFile); statErr == nil {
		t.Error("br was invoked on a refused close")
	}
}

// TestDone_JSONShape: --json emits the exact done result contract.
func TestDone_JSONShape(t *testing.T) {
	chdirRepoFixture(t)
	seedDoneLedger(t)
	resetDoneFlags(t)
	stubBr(t)
	doneSHA = doneSHAConfirmed
	doneJSON = true

	c, out := provTestCmd()
	if err := runDone(c, []string{"ag-done.1"}); err != nil {
		t.Fatalf("ao done --json: %v", err)
	}
	var r doneapp.Result
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("--json output not a done result: %v\n%s", err, out.String())
	}
	want := doneapp.Result{
		BeadID:      "ag-done.1",
		CommitSHA:   doneSHAConfirmed,
		Disposition: "CONFIRMED",
		Stamp:       "[verdict:1a1a1a1:CONFIRMED]",
		CloseReason: "Done [verdict:1a1a1a1:CONFIRMED]",
		Closed:      true,
	}
	if r != want {
		t.Fatalf("done result = %+v, want %+v", r, want)
	}
}

// TestDone_BrFailurePropagates: a non-zero br exit surfaces as an error
// carrying br's output, never a silent success.
func TestDone_BrFailurePropagates(t *testing.T) {
	chdirRepoFixture(t)
	seedDoneLedger(t)
	resetDoneFlags(t)
	argvFile := stubBr(t)
	if err := os.WriteFile(filepath.Join(filepath.Dir(argvFile), "exit-code"), []byte("3"), 0o644); err != nil {
		t.Fatalf("write exit-code: %v", err)
	}
	doneSHA = doneSHAConfirmed

	c, _ := provTestCmd()
	err := runDone(c, []string{"ag-done.1"})
	if err == nil {
		t.Fatal("want br failure error, got nil")
	}
	if !strings.Contains(err.Error(), "br close ag-done.1") {
		t.Errorf("error must name the br close invocation: %s", err.Error())
	}
}

// TestLookupDoneVerdicts: the pure ledger scan is exact about binding,
// confirmation, and disposition collection.
// TestLookupDoneVerdicts_ForeignBeadNeverCertifies pins the pawl catch on this
// bead's own landing: a CONFIRMED verdict recorded for a DIFFERENT bead on the
// same commit must never certify the bead being closed (wrong-object
// certification) — it lands in ForeignBeads and Confirmed stays false.
// TestDoneProvenanceOnly_LeadingSpacePathNotWaived pins the pawl catch: a file
// whose name begins with a space is OUTSIDE docs/provenance/ and must not be
// trimmed into the waiver allowlist.
func TestDoneProvenanceOnly_LeadingSpacePathNotWaived(t *testing.T) {
	repo := t.TempDir()
	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	gitRun("init", "-q")
	gitRun("commit", "-q", "--allow-empty", "-m", "base")
	if err := os.MkdirAll(filepath.Join(repo, " docs", "provenance"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, " docs", "provenance", "ledger.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun("add", "--", " docs")
	gitRun("commit", "-q", "-m", "sneaky")
	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = repo
	revOut, err := revCmd.Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	sha := strings.TrimSpace(string(revOut))
	if doneadapter.SystemRepository().CommitProvenanceOnly(context.Background(), repo, sha) {
		t.Fatal("a leading-space path (' docs/provenance/...') must NOT be waived as provenance-only")
	}
}

func TestLookupDoneVerdicts_ForeignBeadNeverCertifies(t *testing.T) {
	edges := []provenancegraph.Edge{
		buildVerdictCommitEdge(pawlVerdict{BeadID: "ag-done-1", HeadSHA: doneSHAConfirmed, Disposition: "CONFIRMED"}),
	}
	got := doneapp.LookupVerdicts(toDoneEdges(edges), "ag-other", doneSHAConfirmed)
	if got.Confirmed {
		t.Fatal("a CONFIRMED verdict for ag-done-1 must not certify ag-other")
	}
	if len(got.Dispositions) != 0 {
		t.Fatalf("foreign verdicts must not enter Dispositions, got %v", got.Dispositions)
	}
	if len(got.ForeignBeads) != 1 || !strings.HasPrefix(got.ForeignBeads[0], "ag-done-1@") {
		t.Fatalf("ForeignBeads = %v, want the ag-done-1 verdict surfaced", got.ForeignBeads)
	}
	// Same commit, RIGHT bead still certifies.
	right := doneapp.LookupVerdicts(toDoneEdges(edges), "ag-done-1", doneSHAConfirmed)
	if !right.Confirmed {
		t.Fatal("the owning bead must still certify")
	}
}

func TestLookupDoneVerdicts(t *testing.T) {
	edges := []provenancegraph.Edge{
		buildBeadCommitEdge("ag-x", doneSHAConfirmed),
		buildVerdictCommitEdge(pawlVerdict{BeadID: "ag-x", HeadSHA: doneSHAConfirmed, Disposition: "REFUTED"}),
		buildVerdictCommitEdge(pawlVerdict{BeadID: "ag-x", HeadSHA: doneSHAConfirmed, Disposition: "CONFIRMED"}),
		buildVerdictCommitEdge(pawlVerdict{BeadID: "ag-y", HeadSHA: doneSHARefuted, Disposition: "REFUTED"}),
	}

	got := doneapp.LookupVerdicts(toDoneEdges(edges), "ag-x", doneSHAConfirmed)
	if !got.Confirmed {
		t.Fatal("sha with a CONFIRMED verdict must report Confirmed")
	}
	if got.VerdictID != "ag-x@1a1a1a1" {
		t.Fatalf("VerdictID = %q, want ag-x@1a1a1a1", got.VerdictID)
	}
	if len(got.Dispositions) != 2 || got.Dispositions[0] != "REFUTED" || got.Dispositions[1] != "CONFIRMED" {
		t.Fatalf("Dispositions = %v, want [REFUTED CONFIRMED]", got.Dispositions)
	}

	refuted := doneapp.LookupVerdicts(toDoneEdges(edges), "ag-y", doneSHARefuted)
	if refuted.Confirmed {
		t.Fatal("REFUTED-only sha must not report Confirmed")
	}
	if len(refuted.Dispositions) != 1 || refuted.Dispositions[0] != "REFUTED" {
		t.Fatalf("Dispositions = %v, want [REFUTED]", refuted.Dispositions)
	}

	none := doneapp.LookupVerdicts(toDoneEdges(edges), "ag-x", doneSHANoVerdict)
	if none.Confirmed || len(none.Dispositions) != 0 {
		t.Fatalf("unbound sha lookup = %+v, want empty", none)
	}
}

// TestShaBindsCommit: prefix binding in both directions, 7-char floor, hex-only.
func TestShaBindsCommit(t *testing.T) {
	cases := []struct {
		query, commit string
		want          bool
	}{
		{doneSHAConfirmed, doneSHAConfirmed, true},
		{doneSHAConfirmed[:7], doneSHAConfirmed, true},
		{doneSHAConfirmed, doneSHAConfirmed[:7], true},
		{strings.ToUpper(doneSHAConfirmed[:8]), doneSHAConfirmed, true},
		{doneSHAConfirmed[:6], doneSHAConfirmed, false},
		{doneSHARefuted, doneSHAConfirmed, false},
		{"ag-done.1", doneSHAConfirmed, false},
	}
	for _, tc := range cases {
		if got := doneapp.SHABindsCommit(tc.query, tc.commit); got != tc.want {
			t.Errorf("doneapp.SHABindsCommit(%q, %q) = %v, want %v", tc.query, tc.commit, got, tc.want)
		}
	}
}

// makeDoneOriginRepo builds a bare "origin" whose main branch carries a
// docs/provenance/ledger.jsonl seeded with originEdges, plus a local checkout
// whose WORKING-TREE ledger carries localEdges — a stale checkout that lags
// origin: its ledger is written but never committed, and it only *fetches*
// origin so origin/main is a resolvable ref without fast-forwarding the working
// tree. Edges are written THROUGH the production writer (Store.Append) for
// fixture fidelity — the same on-disk hash-chained shape `git show` returns.
// chdirs into the local checkout and returns nothing (the caller drives ao done
// from cwd).
func makeDoneOriginRepo(t *testing.T, localEdges, originEdges []provenancegraph.Edge) {
	t.Helper()
	root := t.TempDir()
	bare := filepath.Join(root, "origin.git")
	builder := filepath.Join(root, "builder")
	local := filepath.Join(root, "local")

	gitEnv := append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	git := func(dir string, args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = gitEnv
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git -C %s %v: %v\n%s", dir, args, err, out)
		}
	}
	writeLedger := func(dir string, edges []provenancegraph.Edge) {
		t.Helper()
		if len(edges) == 0 {
			return
		}
		store := provenancegraph.NewStore(filepath.Join(dir, provenancegraph.LedgerRelativePath))
		for i, e := range edges {
			e.TS = "2026-07-02T00:00:0" + string(rune('0'+i)) + "Z"
			if _, err := store.Append(e); err != nil {
				t.Fatalf("append fixture edge %d in %s: %v", i, dir, err)
			}
		}
	}

	for _, d := range []string{bare, builder, local} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	git(bare, "init", "-q", "--bare")

	// Builder seeds origin/main with the origin ledger and pushes it.
	git(builder, "init", "-q")
	writeLedger(builder, originEdges)
	git(builder, "add", "--", provenancegraph.LedgerRelativePath)
	git(builder, "commit", "-q", "-m", "origin ledger")
	git(builder, "remote", "add", "origin", bare)
	git(builder, "push", "-q", "origin", "HEAD:main")

	// Local is the operator's stale checkout: its working-tree ledger lags
	// origin (localEdges), and it only fetches origin so origin/main resolves —
	// mirroring the incident where the local ledger predated a pushed verdict.
	git(local, "init", "-q")
	writeLedger(local, localEdges)
	git(local, "remote", "add", "origin", bare)
	git(local, "fetch", "-q", "origin")
	t.Chdir(local)
}

// TestDone_OriginLedgerFallbackConfirms: the incident — a CONFIRMED verdict is
// already on origin/main but the local working-tree ledger predates it. ao done
// must consult origin/main and close exactly as if the verdict were local (same
// stamp), with NO manual `git merge --ff-only`.
func TestDone_OriginLedgerFallbackConfirms(t *testing.T) {
	makeDoneOriginRepo(t,
		nil, // stale local: no verdict for the commit
		[]provenancegraph.Edge{
			buildBeadCommitEdge("ag-orig.1", doneSHAConfirmed),
			buildVerdictCommitEdge(pawlVerdict{
				BeadID: "ag-orig.1", HeadSHA: doneSHAConfirmed, Disposition: "CONFIRMED"}),
		})
	resetDoneFlags(t)
	argvFile := stubBr(t)
	doneSHA = doneSHAConfirmed

	c, out := provTestCmd()
	if err := runDone(c, []string{"ag-orig.1"}); err != nil {
		t.Fatalf("ao done with CONFIRMED verdict only on origin/main: %v", err)
	}

	wantArgv := []string{"close", "ag-orig.1", "-r", "Done [verdict:1a1a1a1:CONFIRMED]"}
	got := readStubArgv(t, argvFile)
	if len(got) != len(wantArgv) {
		t.Fatalf("br argv = %q, want %q", got, wantArgv)
	}
	for i := range wantArgv {
		if got[i] != wantArgv[i] {
			t.Fatalf("br argv[%d] = %q, want %q", i, got[i], wantArgv[i])
		}
	}
	if !strings.Contains(out.String(), "closed ag-orig.1 at 1a1a1a1 [verdict:1a1a1a1:CONFIRMED]") {
		t.Errorf("human output missing close line:\n%s", out.String())
	}
}

// TestDone_OriginLedgerFallbackMissRefusesWithFetchHint: the verdict is in
// NEITHER the local ledger nor origin/main (origin holds only an unrelated
// verdict). ao done must still refuse — fail-closed — and the refusal must
// extend with the git-fetch hint so the operator knows a just-pushed verdict
// may not have reached this checkout's origin ref yet.
func TestDone_OriginLedgerFallbackMissRefusesWithFetchHint(t *testing.T) {
	makeDoneOriginRepo(t,
		nil, // local: nothing
		[]provenancegraph.Edge{
			// origin has a verdict, but for a DIFFERENT commit than we query.
			buildBeadCommitEdge("ag-orig.2", doneSHARefuted),
			buildVerdictCommitEdge(pawlVerdict{
				BeadID: "ag-orig.2", HeadSHA: doneSHARefuted, Disposition: "REFUTED"}),
		})
	resetDoneFlags(t)
	argvFile := stubBr(t)
	doneSHA = doneSHANoVerdict // certified by neither local nor origin

	c, _ := provTestCmd()
	err := runDone(c, []string{"ag-orig.2"})
	if err == nil {
		t.Fatal("want refusal when the verdict is in neither ledger, got nil")
	}
	for _, want := range []string{
		"no verdict recorded for commit 3c3c3c3",
		"no verdict = not done",
		"git fetch origin && git merge --ff-only origin/main",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal error missing %q:\n%s", want, err.Error())
		}
	}
	if _, statErr := os.Stat(argvFile); statErr == nil {
		t.Error("br was invoked on a refused close — refusal must not close")
	}
}

// TestDone_LocalVerdictClosesUnchangedWithOrigin: existing behavior is
// untouched — when the LOCAL ledger already certifies the commit, ao done
// closes from it and never depends on origin/main (here origin carries no
// certifying verdict at all).
func TestDone_LocalVerdictClosesUnchangedWithOrigin(t *testing.T) {
	makeDoneOriginRepo(t,
		[]provenancegraph.Edge{ // local already certifies
			buildBeadCommitEdge("ag-loc.3", doneSHAConfirmed),
			buildVerdictCommitEdge(pawlVerdict{
				BeadID: "ag-loc.3", HeadSHA: doneSHAConfirmed, Disposition: "CONFIRMED"}),
		},
		[]provenancegraph.Edge{ // origin: only a bead edge, no verdict
			buildBeadCommitEdge("ag-loc.3", doneSHAConfirmed),
		})
	resetDoneFlags(t)
	argvFile := stubBr(t)
	doneSHA = doneSHAConfirmed

	c, out := provTestCmd()
	if err := runDone(c, []string{"ag-loc.3"}); err != nil {
		t.Fatalf("ao done with a local CONFIRMED verdict: %v", err)
	}
	argv := readStubArgv(t, argvFile)
	if argv[3] != "Done [verdict:1a1a1a1:CONFIRMED]" {
		t.Fatalf("close reason = %q, want the local CONFIRMED stamp", argv[3])
	}
	if !strings.Contains(out.String(), "closed ag-loc.3 at 1a1a1a1 [verdict:1a1a1a1:CONFIRMED]") {
		t.Errorf("human output missing close line:\n%s", out.String())
	}
}
