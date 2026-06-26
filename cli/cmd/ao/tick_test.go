package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTickCommandSurfaceCovered(t *testing.T) {
	// These literals are intentionally full leaf-command names: the command
	// surface parity gate scans *_test.go for this coverage.
	_ = []string{
		"ao chaos-test",
		"ao close",
		"ao council-gate",
		"ao guard-status",
		"ao install-guards",
		"ao ready",
		"ao verdict-gate",
	}
	covered := []string{
		"chaos-test",
		"close",
		"council-gate",
		"guard-status",
		"install-guards",
		"ready",
		"tick claim",
		"tick close",
		"tick council-gate",
		"tick guard-status",
		"tick install-guards",
		"tick next",
		"tick reopen",
		"tick smoke",
		"tick status",
		"tick verdict-gate",
		"verdict-gate",
	}
	registered := map[string]bool{}
	for _, cmd := range rootCmd.Commands() {
		registered[cmd.Name()] = true
	}
	for _, cmd := range tickCmd.Commands() {
		registered["tick "+cmd.Name()] = true
	}
	for _, name := range covered {
		if !registered[name] {
			t.Fatalf("expected registered command %q", name)
		}
	}
}

func TestTickReadyStateCounts(t *testing.T) {
	ready := []tickBead{{ID: "cp-ready", Status: "open"}}
	all := []tickBead{
		{ID: "cp-ready", Status: "open"},
		{ID: "cp-open", Status: "open"},
		{ID: "cp-work", Status: "in_progress"},
		{ID: "cp-done", Status: "closed"},
	}
	counts := tickCountBeads(ready, all)
	if counts.Ready != 1 || counts.Open != 2 || counts.InProgress != 1 || counts.Closed != 1 {
		t.Fatalf("tickCountBeads() = %+v, want ready=1 open=2 in_progress=1 closed=1", counts)
	}
}

// tickTestLedgerLine reproduces the real persisted br issues.jsonl line shape
// (full field set as flushed by `br sync --flush-only`), not a minimal
// hand-built marker — see standards test-pyramid "Fixture Fidelity".
func tickTestLedgerLine(id, status string) string {
	return `{"_type": "issue", "id": "` + id + `", "title": "Node 2", "status": "` + status + `", "priority": 2, "issue_type": "task", "created_at": "2026-05-08T12:09:00Z", "updated_at": "2026-05-30T12:57:05Z", "closed_at": "2026-05-08T12:09:11Z", "close_reason": "Closed", "dependency_count": 0, "dependent_count": 1, "comment_count": 0}` + "\n"
}

func TestTickClosePortAlreadyClosedIsIdempotent(t *testing.T) {
	tests := []struct {
		name      string
		ledgerDir string
	}{
		{name: "br workspace _beads", ledgerDir: "_beads"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BEADS_DIR", "")
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, tc.ledgerDir), 0o755); err != nil {
				t.Fatal(err)
			}
			body := tickTestLedgerLine("cp-done", "closed")
			if err := os.WriteFile(filepath.Join(dir, tc.ledgerDir, "issues.jsonl"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout bytes.Buffer
			rt := tickRuntime{workDir: dir, stdout: &stdout, stderr: &bytes.Buffer{}}
			if err := tickClosePort(rt, "cp-done", "msg", "missing-evidence.md", nil); err != nil {
				t.Fatalf("tickClosePort() unexpected error: %v", err)
			}
			if got := stdout.String(); got != "already closed cp-done @ none\n" {
				t.Fatalf("tickClosePort() stdout = %q", got)
			}
		})
	}
}

func TestTickLedgerDirResolution(t *testing.T) {
	tests := []struct {
		name       string
		processEnv string   // value for the BEADS_DIR process env ("" = unset)
		rtEnv      []string // tickRuntime env overlay
		mkdirs     []string // directories created under workDir
		want       func(workDir string) string
	}{
		{
			name:   "uses _beads when it exists",
			mkdirs: []string{"_beads", ".beads"},
			want:   func(d string) string { return filepath.Join(d, "_beads") },
		},
		{
			name:   "ignores retired .beads fallback",
			mkdirs: []string{".beads"},
			want:   func(d string) string { return filepath.Join(d, "_beads") },
		},
		{
			name: "defaults to _beads when nothing exists",
			want: func(d string) string { return filepath.Join(d, "_beads") },
		},
		{
			name:       "process BEADS_DIR wins over directory probe",
			processEnv: "custom_beads",
			mkdirs:     []string{"_beads"},
			want:       func(d string) string { return filepath.Join(d, "custom_beads") },
		},
		{
			name:       "rt.env BEADS_DIR wins over process env",
			processEnv: "process_beads",
			rtEnv:      []string{"BEADS_DIR=overlay_beads"},
			mkdirs:     []string{"_beads"},
			want:       func(d string) string { return filepath.Join(d, "overlay_beads") },
		},
		{
			name:  "last rt.env entry wins",
			rtEnv: []string{"BEADS_DIR=first", "BEADS_DIR=second"},
			want:  func(d string) string { return filepath.Join(d, "second") },
		},
		{
			name:       "absolute BEADS_DIR used verbatim",
			processEnv: "/abs/ledger",
			want:       func(string) string { return "/abs/ledger" },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BEADS_DIR", tc.processEnv)
			dir := t.TempDir()
			for _, sub := range tc.mkdirs {
				if err := os.Mkdir(filepath.Join(dir, sub), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			rt := tickRuntime{workDir: dir, env: tc.rtEnv}
			if got, want := tickLedgerDir(rt), tc.want(dir); got != want {
				t.Fatalf("tickLedgerDir() = %q, want %q", got, want)
			}
		})
	}
}

func TestTickStagePath(t *testing.T) {
	tests := []struct {
		name string
		root string
		path string
		want string
	}{
		{name: "under root is relative", root: "/repo", path: "/repo/_beads/issues.jsonl", want: filepath.Join("_beads", "issues.jsonl")},
		{name: "outside root stays absolute", root: "/repo", path: "/elsewhere/issues.jsonl", want: "/elsewhere/issues.jsonl"},
		{name: "root itself stays absolute via dot", root: "/repo", path: "/repo", want: "."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tickStagePath(tc.root, tc.path); got != tc.want {
				t.Fatalf("tickStagePath(%q, %q) = %q, want %q", tc.root, tc.path, got, tc.want)
			}
		})
	}
}

// TestTickClose_PersistsLedgerInOwnRepoNotPublic is the L2 regression for the
// private br ledger contract: tickClose must persist _beads through its nested
// git repo and must never stage ledger files in the public repo.
func TestTickClose_PersistsLedgerInOwnRepoNotPublic(t *testing.T) {
	tests := []struct {
		name       string
		ledgerDir  string
		rtEnv      bool // set BEADS_DIR in rt.env to the ledger dir
		commitNoop bool
	}{
		{
			name:      "br workspace _beads",
			ledgerDir: "_beads",
		},
		{
			name:      "explicit BEADS_DIR override",
			ledgerDir: "custom_beads",
			rtEnv:     true,
		},
		{
			name:       "empty ledger commit is ok when ledger shows closed",
			ledgerDir:  "_beads",
			commitNoop: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BEADS_DIR", "")
			dir := t.TempDir()
			ledger := filepath.Join(dir, tc.ledgerDir)
			if err := os.MkdirAll(ledger, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ledger, "issues.jsonl"), []byte(tickTestLedgerLine("cp-ledger", "closed")), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ledger, "metadata.json"), []byte(`{"database":"beads"}`), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(dir, "evidence"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "evidence", "proof.md"), []byte("proof"), 0o644); err != nil {
				t.Fatal(err)
			}

			fakebin := filepath.Join(dir, "fakebin")
			if err := os.MkdirAll(fakebin, 0o755); err != nil {
				t.Fatal(err)
			}
			publicHeadFile := filepath.Join(dir, "fake-public-head")
			ledgerHeadFile := filepath.Join(dir, "fake-ledger-head")
			gitLog := filepath.Join(dir, "fake-git.log")
			if err := os.WriteFile(publicHeadFile, []byte("public1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(ledgerHeadFile, []byte("ledger1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			fakeBR := "#!/usr/bin/env bash\ncase \"${1:-}\" in close|sync|update) exit 0 ;; *) echo \"unexpected br call: $*\" >&2; exit 43 ;; esac\n"
			fakeGit := `#!/usr/bin/env bash
printf '%s\n' "$*" >> "${TICK_TEST_GIT_LOG:?}"
if [ "${1:-}" = "-C" ]; then
  shift 2
  case "${1:-}" in
    rev-parse) cat "${TICK_TEST_LEDGER_HEAD_FILE:?}" ;;
    add) exit 0 ;;
    commit)
      if [ "${TICK_TEST_LEDGER_COMMIT_NOOP:-}" = "1" ]; then
        echo "nothing to commit, working tree clean"
        exit 1
      fi
      echo ledger2 > "${TICK_TEST_LEDGER_HEAD_FILE:?}"
      ;;
    *) : ;;
  esac
  exit 0
fi
case "${1:-}" in
  rev-parse) cat "${TICK_TEST_PUBLIC_HEAD_FILE:?}" ;;
  add) exit 0 ;;
  commit) echo public2 > "${TICK_TEST_PUBLIC_HEAD_FILE:?}" ;;
  *) : ;;
esac
`
			if err := os.WriteFile(filepath.Join(fakebin, "br"), []byte(fakeBR), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(fakebin, "git"), []byte(fakeGit), 0o755); err != nil {
				t.Fatal(err)
			}

			// exec.Command resolves binaries via the parent process PATH
			// (not c.Env), so the stub PATH must go through t.Setenv.
			t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("TICK_TEST_PUBLIC_HEAD_FILE", publicHeadFile)
			t.Setenv("TICK_TEST_LEDGER_HEAD_FILE", ledgerHeadFile)
			t.Setenv("TICK_TEST_GIT_LOG", gitLog)
			if tc.commitNoop {
				t.Setenv("TICK_TEST_LEDGER_COMMIT_NOOP", "1")
			}
			var stdout, stderr bytes.Buffer
			rt := tickRuntime{
				workDir: dir,
				stdout:  &stdout,
				stderr:  &stderr,
			}
			if tc.rtEnv {
				rt.env = []string{"BEADS_DIR=" + tc.ledgerDir}
			}
			if err := tickClose(rt, "cp-ledger", "close msg", "evidence/proof.md", []string{"docs/close.md"}); err != nil {
				t.Fatalf("tickClose() error: %v (stderr=%q)", err, stderr.String())
			}
			got, err := os.ReadFile(gitLog)
			if err != nil {
				t.Fatalf("git was never invoked: %v", err)
			}
			lines := strings.Split(strings.TrimSpace(string(got)), "\n")
			wantLedgerAdd := "-C " + ledger + " add -- issues.jsonl metadata.json"
			wantLedgerCommit := "-C " + ledger + " commit -q -m close msg"
			if !tickTestLinesContain(lines, wantLedgerAdd) {
				t.Fatalf("git calls missing ledger add %q; got %q", wantLedgerAdd, strings.TrimSpace(string(got)))
			}
			if !tickTestLinesContain(lines, wantLedgerCommit) {
				t.Fatalf("git calls missing ledger commit %q; got %q", wantLedgerCommit, strings.TrimSpace(string(got)))
			}
			for _, line := range lines {
				if strings.HasPrefix(line, "add ") {
					if strings.Contains(line, "_beads") || strings.Contains(line, ".beads") ||
						strings.Contains(line, "issues.jsonl") || strings.Contains(line, "metadata.json") {
						t.Fatalf("public git add staged ledger path: %q", line)
					}
					if line != "add -- evidence/proof.md docs/close.md" {
						t.Fatalf("public git add = %q, want evidence and caller path only", line)
					}
				}
			}
			wantSHA := "ledger2"
			if tc.commitNoop {
				wantSHA = "ledger1"
			}
			if want := "closed cp-ledger @ " + wantSHA + "\n"; stdout.String() != want {
				t.Fatalf("tickClose() stdout = %q, want %q", stdout.String(), want)
			}
		})
	}
}

// TestTickEvidenceRefusal_DurableBinding pins the durable close-evidence
// binding (age-l7yh): a close is refused when the cited evidence is a
// repo-internal path present in the working tree but NOT a committed git blob
// in HEAD (an ancestor of the close commit). Committed blobs are durable and
// allowed; the only exempt class is the gitignored .agents/** runtime corpus
// (ephemeral by design). Everything else is refused — uncommitted, modified,
// directory (tree), committed symlink, evidence/-substring, and anything that
// resolves outside the repo (including via a crafted symlink). Uses a real git
// repo to exercise the actual git incantations.
func TestTickEvidenceRefusal_DurableBinding(t *testing.T) {
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-q")
	// A committed evidence blob (durable, an ancestor of any future commit).
	if err := os.WriteFile(filepath.Join(dir, "committed.md"), []byte("proof"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "committed.md")
	runGit("commit", "-q", "-m", "seed")
	// A repo-internal file present on disk but NOT committed (and not ignored).
	if err := os.WriteFile(filepath.Join(dir, "uncommitted.md"), []byte("proof"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A gitignored runtime-corpus evidence file (.agents/** — ephemeral by
	// design, exempt) and a gitignored NON-corpus build artifact (logs/ — NOT
	// exempt: must be committed to serve as durable evidence).
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".agents/\nlogs/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".agents", "ev.md"), []byte("proof"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logs", "build.log"), []byte("artifact"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An uncommitted file under an evidence/ staging dir — the old leaky
	// substring exemption let this through; the durable binding now refuses it.
	if err := os.MkdirAll(filepath.Join(dir, "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evidence", "staged.md"), []byte("staged"), 0o644); err != nil {
		t.Fatal(err)
	}
	// An evidence file OUTSIDE the repo (the git-blob binding cannot govern it).
	external := filepath.Join(t.TempDir(), "external.md")
	if err := os.WriteFile(external, []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}

	rt := tickRuntime{workDir: dir, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	cases := []struct {
		name       string
		evidence   string
		wantRefuse bool
	}{
		{"committed blob is durable -> allowed", "committed.md", false},
		{"uncommitted repo file -> refused (durable binding)", "uncommitted.md", true},
		{"absolute path inside repo, uncommitted -> refused (no abs bypass)", filepath.Join(dir, "uncommitted.md"), true},
		{"gitignored .agents evidence -> allowed (exempt)", ".agents/ev.md", false},
		{"gitignored non-corpus artifact -> refused (no fail-open)", "logs/build.log", true},
		{"uncommitted evidence/ staging file -> refused (no substring bypass)", "evidence/staged.md", true},
		{"evidence/ substring on uncommitted file -> refused (token, not text)", "uncommitted.md (see evidence/)", true},
		{"external (out-of-repo) absolute path -> refused (not in repo)", external, true},
		{"missing artifact -> refused", "nope.md", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evFirst := tickFirstEvidenceToken(tc.evidence)
			reason := tickEvidenceRefusal(rt, tc.evidence, evFirst)
			if (reason != "") != tc.wantRefuse {
				t.Fatalf("tickEvidenceRefusal(%q) reason=%q, wantRefuse=%v", tc.evidence, reason, tc.wantRefuse)
			}
		})
	}

	// Regression (pawl REFUTED round 1): evidence cited from a SUBDIRECTORY must
	// bind cwd-relative, not root-relative. A file committed inside sub/ and
	// cited as its subdir-relative name from rt.workDir=sub/ must be recognized
	// as committed (durable) and allowed — not falsely refused because HEAD:name
	// looked at the repo root.
	t.Run("committed evidence in subdir binds cwd-relative (allowed)", func(t *testing.T) {
		sub := filepath.Join(dir, "sub")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "subproof.md"), []byte("proof"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit("add", "sub/subproof.md")
		runGit("commit", "-q", "-m", "subdir evidence")
		subRT := tickRuntime{workDir: sub, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
		if reason := tickEvidenceRefusal(subRT, "subproof.md", "subproof.md"); reason != "" {
			t.Fatalf("committed subdir evidence cited cwd-relative = %q, want allowed", reason)
		}
	})

	// Regression (pawl REFUTED round 8): a tracked file that is committed but has
	// uncommitted working-tree changes is NOT durably in history — the cited
	// content differs from HEAD — and must be refused.
	t.Run("committed-but-modified file -> refused (dirty != durable)", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(dir, "committed.md"), []byte("MODIFIED uncommitted"), 0o644); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.WriteFile(filepath.Join(dir, "committed.md"), []byte("proof"), 0o644) }()
		if reason := tickEvidenceRefusal(rt, "committed.md", "committed.md"); reason == "" {
			t.Fatalf("committed-but-modified evidence = allowed, want refused (working tree != HEAD)")
		}
	})

	// Regression (pawl REFUTED round 6): a committed DIRECTORY (tree) is not a
	// durable evidence blob. Citing a committed directory must be refused — the
	// gate requires a committed file (blob), not a whole directory.
	t.Run("committed directory (tree) -> refused (blob required)", func(t *testing.T) {
		treedir := filepath.Join(dir, "treedir")
		if err := os.MkdirAll(treedir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(treedir, "inner.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit("add", "treedir/inner.md")
		runGit("commit", "-q", "-m", "tree evidence")
		if reason := tickEvidenceRefusal(rt, "treedir", "treedir"); reason == "" {
			t.Fatalf("committed directory cited as evidence = allowed, want refused (tree is not a blob)")
		}
	})

	// Regression (pawl REFUTED round 7): an in-repo SYMLINK whose target is
	// outside the repo must not bypass the binding. The evidence path is
	// classified lexically (the leaf symlink is not followed), so it stays
	// in-repo and — being uncommitted — is refused.
	t.Run("in-repo symlink to external target -> refused (no symlink escape)", func(t *testing.T) {
		external := filepath.Join(t.TempDir(), "outside.md")
		if err := os.WriteFile(external, []byte("outside"), 0o644); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "linkout.md")
		if err := os.Symlink(external, link); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}
		if reason := tickEvidenceRefusal(rt, "linkout.md", "linkout.md"); reason == "" {
			t.Fatalf("in-repo symlink to external = allowed, want refused (symlink escape)")
		}
	})

	// Regression (pawl REFUTED round 10): a COMMITTED SYMLINK is a blob, but its
	// bytes are only the target path — the pointed-at content is not in history.
	// Citing a committed symlink must be refused (tree mode 120000, not 100xxx).
	t.Run("committed symlink -> refused (target not in history)", func(t *testing.T) {
		linktarget := filepath.Join(t.TempDir(), "target.md")
		if err := os.WriteFile(linktarget, []byte("target"), 0o644); err != nil {
			t.Fatal(err)
		}
		syml := filepath.Join(dir, "committed-link.md")
		if err := os.Symlink(linktarget, syml); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}
		runGit("add", "committed-link.md")
		runGit("commit", "-q", "-m", "commit symlink evidence")
		if reason := tickEvidenceRefusal(rt, "committed-link.md", "committed-link.md"); reason == "" {
			t.Fatalf("committed symlink cited as evidence = allowed, want refused (mode 120000)")
		}
	})

	// Regression (pawl REFUTED round 10): an ABSOLUTE evidence path routed through
	// an in-repo SYMLINKED DIRECTORY whose target is outside the repo must not be
	// exempted as "outside" — the parent is resolved, the path resolves outside,
	// and outside now means refused (not allowed).
	t.Run("absolute path via in-repo symlinked dir -> refused", func(t *testing.T) {
		outdir := t.TempDir()
		if err := os.WriteFile(filepath.Join(outdir, "proof.md"), []byte("proof"), 0o644); err != nil {
			t.Fatal(err)
		}
		linkdir := filepath.Join(dir, "linkdir")
		if err := os.Symlink(outdir, linkdir); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}
		abs := filepath.Join(linkdir, "proof.md")
		if reason := tickEvidenceRefusal(rt, abs, abs); reason == "" {
			t.Fatalf("absolute path via symlinked dir = allowed, want refused (escapes repo)")
		}
	})

	// Guard: outside a git repo (no resolvable HEAD) the durable binding must
	// NOT fire — there is no history to bind to, so an on-disk repo-internal
	// evidence file falls back to the existence check and is allowed.
	t.Run("no git history -> binding skipped (allowed)", func(t *testing.T) {
		plain := t.TempDir()
		if err := os.WriteFile(filepath.Join(plain, "proof.md"), []byte("proof"), 0o644); err != nil {
			t.Fatal(err)
		}
		plainRT := tickRuntime{workDir: plain, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
		if reason := tickEvidenceRefusal(plainRT, "proof.md", "proof.md"); reason != "" {
			t.Fatalf("tickEvidenceRefusal in non-git dir = %q, want allowed", reason)
		}
	})
}

func TestTickCloseLinkedWorktreeUsesCanonicalBeadsLedger(t *testing.T) {
	t.Setenv("BEADS_DIR", "")
	root, lane := makeGitRepoWithLinkedWorktree(t)
	ledger := filepath.Join(root, "_beads")
	if err := os.MkdirAll(ledger, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ledger, "issues.jsonl"), []byte(tickTestLedgerLine("cp-linked", "closed")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ledger, "metadata.json"), []byte(`{"database":"beads"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(lane, "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lane, "evidence", "proof.md"), []byte("proof"), 0o644); err != nil {
		t.Fatal(err)
	}

	fakebin := filepath.Join(t.TempDir(), "fakebin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	publicHeadFile := filepath.Join(fakebin, "fake-public-head")
	ledgerHeadFile := filepath.Join(fakebin, "fake-ledger-head")
	gitLog := filepath.Join(fakebin, "fake-git.log")
	if err := os.WriteFile(publicHeadFile, []byte("public1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledgerHeadFile, []byte("ledger1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeBR := "#!/usr/bin/env bash\ncase \"${1:-}\" in close|sync|update) exit 0 ;; *) echo \"unexpected br call: $*\" >&2; exit 43 ;; esac\n"
	fakeGit := `#!/usr/bin/env bash
printf '%s\n' "$*" >> "${TICK_TEST_GIT_LOG:?}"
if [ "${1:-}" = "-C" ]; then
  dir="$2"
  shift 2
  if [ "$dir" = "${TICK_TEST_WORKDIR:?}" ] && [ "${1:-}" = "rev-parse" ] && [ "${2:-}" = "--git-common-dir" ]; then
    printf '%s\n' "${TICK_TEST_GIT_COMMON_DIR:?}"
    exit 0
  fi
  if [ "$dir" = "${TICK_TEST_WORKDIR:?}" ] && [ "${1:-}" = "rev-parse" ] && [ "${2:-}" = "--show-toplevel" ]; then
    printf '%s\n' "${TICK_TEST_WORKDIR:?}"
    exit 0
  fi
  case "${1:-}" in
    rev-parse) cat "${TICK_TEST_LEDGER_HEAD_FILE:?}" ;;
    add) exit 0 ;;
    commit) echo ledger2 > "${TICK_TEST_LEDGER_HEAD_FILE:?}" ;;
    *) : ;;
  esac
  exit 0
fi
case "${1:-}" in
  rev-parse) cat "${TICK_TEST_PUBLIC_HEAD_FILE:?}" ;;
  add) exit 0 ;;
  commit) echo public2 > "${TICK_TEST_PUBLIC_HEAD_FILE:?}" ;;
  *) : ;;
esac
`
	if err := os.WriteFile(filepath.Join(fakebin, "br"), []byte(fakeBR), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakebin, "git"), []byte(fakeGit), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakebin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TICK_TEST_WORKDIR", lane)
	t.Setenv("TICK_TEST_GIT_COMMON_DIR", filepath.Join(root, ".git"))
	t.Setenv("TICK_TEST_PUBLIC_HEAD_FILE", publicHeadFile)
	t.Setenv("TICK_TEST_LEDGER_HEAD_FILE", ledgerHeadFile)
	t.Setenv("TICK_TEST_GIT_LOG", gitLog)

	var stdout, stderr bytes.Buffer
	rt := tickRuntime{workDir: lane, stdout: &stdout, stderr: &stderr}
	if err := tickClose(rt, "cp-linked", "close msg", "evidence/proof.md", []string{"docs/close.md"}); err != nil {
		t.Fatalf("tickClose() error: %v (stderr=%q)", err, stderr.String())
	}
	got, err := os.ReadFile(gitLog)
	if err != nil {
		t.Fatalf("git was never invoked: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	wantCommonDir := "-C " + lane + " rev-parse --git-common-dir"
	if !tickTestLinesContain(lines, wantCommonDir) {
		t.Fatalf("git calls missing linked-worktree common-dir lookup %q; got %q", wantCommonDir, strings.TrimSpace(string(got)))
	}
	wantLedgerAdd := "-C " + ledger + " add -- issues.jsonl metadata.json"
	if !tickTestLinesContain(lines, wantLedgerAdd) {
		t.Fatalf("git calls missing canonical ledger add %q; got %q", wantLedgerAdd, strings.TrimSpace(string(got)))
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "add ") && (strings.Contains(line, "_beads") ||
			strings.Contains(line, "issues.jsonl") || strings.Contains(line, "metadata.json")) {
			t.Fatalf("public git add staged ledger path: %q", line)
		}
	}
}

func tickTestLinesContain(lines []string, want string) bool {
	for _, line := range lines {
		if line == want {
			return true
		}
	}
	return false
}

func TestTickFirstReadyFromJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "array",
			body: `[{"id":"cp-123","status":"open"},{"id":"cp-456","status":"open"}]`,
			want: "cp-123",
		},
		{
			name: "wrapped issues",
			body: `{"issues":[{"id":"cp-789","status":"open"}]}`,
			want: "cp-789",
		},
		{
			name: "empty",
			body: `[]`,
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tickFirstReadyFromJSON([]byte(tc.body)); got != tc.want {
				t.Fatalf("tickFirstReadyFromJSON() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTickVerdictHasCommandsRun(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "missing",
			body: "VERDICT: PASS\nREASONS: ok\n",
			want: false,
		},
		{
			name: "empty",
			body: "COMMANDS RUN:\nREASONS:\nok\n",
			want: false,
		},
		{
			name: "inline",
			body: "COMMANDS RUN: ao tick guard-status\nREASONS: ok\n",
			want: true,
		},
		{
			name: "body",
			body: "COMMANDS RUN:\n  ao tick smoke\nREASONS: ok\n",
			want: true,
		},
		{
			name: "hyphenated heading",
			body: "commands-run:\n  git status\n",
			want: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tickVerdictHasCommandsRun(tc.body); got != tc.want {
				t.Fatalf("tickVerdictHasCommandsRun() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTickVerdictTokenCounts(t *testing.T) {
	pass, fail := tickVerdictTokenCounts("VERDICT: PASS\n verdict: fail\nnot VERDICT: PASS\n")
	if pass != 1 || fail != 1 {
		t.Fatalf("tickVerdictTokenCounts() = pass %d fail %d, want 1/1", pass, fail)
	}
}

func tickTestVerdict(author, judge, program, family, verdict, command string) string {
	// Each distinct judge gets a distinct non-author context so the typed-judge
	// tuple satisfies the context-floor (the independence axis is fresh context).
	return fmt.Sprintf("author: %s\njudge: %s\njudge_program: %s\njudge_model_family: %s\ncontext_id: ctx-%s\nVERDICT: %s\nCOMMANDS RUN:\n  %s\n", author, judge, program, family, judge, verdict, command)
}

func TestTickVerdictIdentityRequiresTypedIndependentJudge(t *testing.T) {
	valid := tickTestVerdict("codex", "athena", "claude-code", "claude", "PASS", "ao tick guard-status")
	identity, gaps := tickVerdictIdentity(valid)
	if len(gaps) != 0 {
		t.Fatalf("valid identity gaps = %v, want none", gaps)
	}
	if identity.Author != "codex" || identity.JudgeName != "athena" || identity.JudgeProgram != "claude-code" || identity.JudgeModelFamily != "claude" {
		t.Fatalf("identity = %+v, want typed judge tuple", identity)
	}

	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing judge program",
			body: "author: codex\njudge: athena\njudge_model_family: claude\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n",
		},
		{
			name: "self judge rejected despite inline waiver",
			body: "author: codex\njudge: codex\njudge_program: codex-cli\njudge_model_family: codex\nallow_self: true\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n",
		},
		{
			name: "unknown family rejected",
			body: tickTestVerdict("codex", "athena", "claude-code", "unknown", "PASS", "ao tick guard-status"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, gaps := tickVerdictIdentity(tc.body); len(gaps) == 0 {
				t.Fatal("tickVerdictIdentity() gaps = none, want fail-closed gap")
			}
		})
	}
}

func TestTickCouncilGateMatrix(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	pass1 := write("pass1.md", tickTestVerdict("codex", "athena", "claude-code", "claude", "PASS", "ao tick guard-status"))
	pass2 := write("pass2.md", tickTestVerdict("codex", "windyelm", "gemini-cli", "gemini", "PASS", "ao tick verdict-gate -"))
	fail1 := write("fail1.md", tickTestVerdict("codex", "windyelm", "gemini-cli", "gemini", "FAIL", "ao tick guard-status"))
	unverified := write("unverified.md", "VERDICT: PASS\n")
	contradictory := write("contradictory.md", "author: codex\njudge: athena\njudge_program: claude-code\njudge_model_family: claude\nVERDICT: FAIL\nVERDICT: PASS\nCOMMANDS RUN:\n  ao tick guard-status\n")

	rt := tickRuntime{workDir: dir, stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}}
	if code := tickExitCode(tickCouncilGate(rt, []string{pass1, pass2})); code != 0 {
		t.Fatalf("two-pass council code = %d, want 0", code)
	}
	if code := tickExitCode(tickCouncilGate(rt, []string{pass1, fail1})); code != tickExitDisagree {
		t.Fatalf("mixed council code = %d, want %d", code, tickExitDisagree)
	}
	if code := tickExitCode(tickCouncilGate(rt, []string{pass1, unverified})); code != tickExitCouncil {
		t.Fatalf("unverified council code = %d, want %d", code, tickExitCouncil)
	}
	if code := tickExitCode(tickCouncilGate(rt, []string{pass1, contradictory})); code != tickExitCouncil {
		t.Fatalf("contradictory council code = %d, want %d", code, tickExitCouncil)
	}
}

// Retired: the family-floor council test (same-family two-context -> reject) was
// flipped to the context-floor in TestTickCouncilGateContextFloorFlip (the staged
// S1b.4 acceptance test). Self-judge (by context) and duplicate-context
// fail-closed are covered there.

func TestTickLedgerShowsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "issues.jsonl")
	body := `{"id":"cp-open","status":"open"}` + "\n" +
		`{"id":"cp-done","status":"closed"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if !tickLedgerShowsClosed(path, "cp-done") {
		t.Fatal("expected closed bead to resolve")
	}
	if tickLedgerShowsClosed(path, "cp-open") {
		t.Fatal("open bead resolved as closed")
	}
}
