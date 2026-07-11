// practices: [tdd]
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/claimproof"
	"github.com/boshu2/agentops/cli/internal/ports"
)

func TestClaimBind_EmptyClaimRejected(t *testing.T) {
	err := claimBindRun(context.Background(), claimOptions{path: "p", level: "PG2"})
	if err == nil {
		t.Fatal("expected error on empty claim")
	}
}

func TestClaimBind_EmptyPathRejected(t *testing.T) {
	err := claimBindRun(context.Background(), claimOptions{claim: "X", level: "PG2"})
	if err == nil {
		t.Fatal("expected error on empty path")
	}
}

func TestClaimBind_InvalidLevelRejected(t *testing.T) {
	err := claimBindRun(context.Background(), claimOptions{claim: "X", path: "p", level: "PG99"})
	if err == nil {
		t.Fatal("expected error on bogus level")
	}
	if !strings.Contains(err.Error(), "invalid --level") {
		t.Fatalf("error not informative: %v", err)
	}
}

func TestClaimBind_StubCalledWithBinding(t *testing.T) {
	called := false
	var gotOpts claimOptions
	stub := func(_ context.Context, opts claimOptions) error {
		called = true
		gotOpts = opts
		return nil
	}
	var buf bytes.Buffer
	err := claimBindRun(context.Background(), claimOptions{
		claim:    "AOP-CLAIM-X",
		path:     ".agents/findings/x.md",
		level:    "PG3",
		anchors:  []string{"L10", "L20"},
		authorID: "agent-1",
		judgeID:  "agent-2",
		writer:   &buf,
		bindFn:   stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("bindFn not invoked")
	}
	if gotOpts.claim != "AOP-CLAIM-X" || gotOpts.level != "PG3" {
		t.Fatalf("opts mismatch: %+v", gotOpts)
	}
	if gotOpts.authorID != "agent-1" || gotOpts.judgeID != "agent-2" {
		t.Fatalf("reviewer metadata mismatch: %+v", gotOpts)
	}
	if !strings.Contains(buf.String(), `level=PG3`) {
		t.Fatalf("confirmation missing level: %q", buf.String())
	}
}

func TestClaimBind_DefaultLevelIsPG1(t *testing.T) {
	// Note: level default is set by cobra; in the test path we pass it explicitly.
	// This checks the validator accepts PG1 default value.
	stub := func(_ context.Context, _ claimOptions) error { return nil }
	err := claimBindRun(context.Background(), claimOptions{
		claim:  "X",
		path:   "p",
		level:  "PG1",
		bindFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClaimBind_StubErrorWrapped(t *testing.T) {
	stub := func(_ context.Context, _ claimOptions) error {
		return errors.New("downgrade rejected")
	}
	err := claimBindRun(context.Background(), claimOptions{
		claim:  "X",
		path:   "p",
		level:  "PG1",
		bindFn: stub,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "claim bind:") {
		t.Fatalf("error not wrapped: %v", err)
	}
}

func TestClaimList_StubReturnsBindings(t *testing.T) {
	stub := func(_ context.Context, _ claimOptions) ([]ports.EvidenceBinding, error) {
		return []ports.EvidenceBinding{
			{Claim: "AOP-A", Path: "a.md", Level: ports.EvidenceLevelPG2},
			{Claim: "AOP-B", Path: "b.md", Level: ports.EvidenceLevelPG4},
		}, nil
	}
	var buf bytes.Buffer
	err := claimListRun(context.Background(), claimOptions{
		writer: &buf,
		listFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("len = %d, want 2", len(lines))
	}
	if !strings.Contains(lines[0], `"Level":"PG2"`) || !strings.Contains(lines[1], `"Level":"PG4"`) {
		t.Fatalf("levels missing: %s", buf.String())
	}
}

func TestClaimList_EmptyBindings(t *testing.T) {
	stub := func(_ context.Context, _ claimOptions) ([]ports.EvidenceBinding, error) {
		return []ports.EvidenceBinding{}, nil
	}
	var buf bytes.Buffer
	err := claimListRun(context.Background(), claimOptions{
		writer: &buf,
		listFn: stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Fatalf("empty should be 0 bytes, got %q", buf.String())
	}
}

func TestClaimCheckRequiresChanged(t *testing.T) {
	err := claimCheckRun(context.Background(), claimCheckOptions{})
	if err == nil {
		t.Fatal("expected --changed requirement")
	}
	if !strings.Contains(err.Error(), "--changed") {
		t.Fatalf("error not helpful: %v", err)
	}
}

func TestClaimCheckCommandRegistered(t *testing.T) {
	out, err := executeCommand("claim", "check", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Report read-only proof cards for changed public claim markers.") {
		t.Fatalf("help missing claim check summary:\n%s", out)
	}
	if !strings.Contains(out, "--changed") || !strings.Contains(out, "--base") {
		t.Fatalf("help missing expected flags:\n%s", out)
	}
}

func TestClaimCheckAcceptsNoPositionals(t *testing.T) {
	if claimCheckCmd.Args == nil {
		t.Fatal("claim check must declare an explicit positional-argument policy")
	}
	if err := claimCheckCmd.Args(claimCheckCmd, nil); err != nil {
		t.Fatalf("claim check rejected zero positional arguments: %v", err)
	}
	if err := claimCheckCmd.Args(claimCheckCmd, []string{"unexpected"}); err == nil {
		t.Fatal("claim check accepted an unexpected positional argument")
	}
}

func TestClaimBindingsPathUsesRepositoryRoot(t *testing.T) {
	repo := t.TempDir()
	initMinimalGitRepo(t, repo)
	subdir := filepath.Join(repo, "nested")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := testProjectDir
	testProjectDir = subdir
	t.Cleanup(func() { testProjectDir = old })

	path, err := claimBindingsPath()
	if err != nil {
		t.Fatal(err)
	}
	root, err := resolveRepoRoot(subdir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".agents", "findings", "evidence-bindings.jsonl")
	if path != want {
		t.Fatalf("claimBindingsPath() = %q, want repo-rooted %q", path, want)
	}
}

func TestClaimCheckJSONOutput(t *testing.T) {
	var got claimproof.Options
	stub := func(_ context.Context, opts claimproof.Options) (claimproof.Report, error) {
		got = opts
		return claimproof.Report{
			Summary: claimproof.Summary{
				Mode:            "changed",
				Base:            "origin/main",
				ChangedSurfaces: 1,
				Claims:          1,
				Verdicts:        map[string]int{"supported": 1},
			},
			Cards: []claimproof.Card{{
				ClaimID:       "AOP-CLAIM-X",
				Surface:       "PRODUCT.md",
				Tier:          "PILOT",
				RegistryFound: true,
				Evidence: []claimproof.Evidence{{
					Path:   "docs/evidence/x.md",
					Status: "tracked",
				}},
				Verdict:    "supported",
				NextAction: "run validation",
			}},
		}, nil
	}

	var buf bytes.Buffer
	err := claimCheckRun(context.Background(), claimCheckOptions{
		repoRoot:    "/repo",
		base:        "origin/main",
		changedOnly: true,
		jsonOutput:  true,
		writer:      &buf,
		checkFn:     stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.RepoRoot != "/repo" || got.Base != "origin/main" || !got.ChangedOnly {
		t.Fatalf("opts not forwarded: %+v", got)
	}
	var report claimproof.Report
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(report.Cards) != 1 || report.Cards[0].ClaimID != "AOP-CLAIM-X" {
		t.Fatalf("wrong report: %+v", report)
	}
}

func TestClaimCheckHumanNoop(t *testing.T) {
	stub := func(_ context.Context, _ claimproof.Options) (claimproof.Report, error) {
		return claimproof.Report{
			Summary: claimproof.Summary{
				Mode:     "changed",
				Base:     "origin/main",
				Verdicts: map[string]int{},
			},
		}, nil
	}

	var buf bytes.Buffer
	err := claimCheckRun(context.Background(), claimCheckOptions{
		base:        "origin/main",
		changedOnly: true,
		writer:      &buf,
		checkFn:     stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No changed claim markers found.") {
		t.Fatalf("missing no-op output: %q", buf.String())
	}
}

func TestClaimCheckHumanShowsCitationPolicy(t *testing.T) {
	stub := func(_ context.Context, _ claimproof.Options) (claimproof.Report, error) {
		return claimproof.Report{
			Summary: claimproof.Summary{
				Mode:            "changed",
				Base:            "origin/main",
				ChangedSurfaces: 1,
				Claims:          1,
				Verdicts:        map[string]int{"citation_ceiling": 1},
			},
			Cards: []claimproof.Card{{
				ClaimID:       "AOP-CLAIM-X",
				Surface:       "PRODUCT.md",
				Tier:          "UNPROVEN",
				CitationOK:    false,
				CiteAllowed:   []string{"docs/comparisons/**", "docs/wiki-for-agents.md"},
				RegistryFound: true,
				Verdict:       "citation_ceiling",
				NextAction:    "promote the claim tier",
			}},
		}, nil
	}

	var buf bytes.Buffer
	err := claimCheckRun(context.Background(), claimCheckOptions{
		base:        "origin/main",
		changedOnly: true,
		writer:      &buf,
		checkFn:     stub,
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "citation_ok: false") || !strings.Contains(out, "docs/comparisons/**") {
		t.Fatalf("missing citation policy in human output: %q", out)
	}
}
