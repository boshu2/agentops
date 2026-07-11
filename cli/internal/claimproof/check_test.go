package claimproof

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckChangedClaimRendersProofCard(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "PRODUCT.md", "# Product\n\n<!-- agentops:claim:AOP-CLAIM-PRODUCT-VALUE -->\n\nA changed claim.\n")
	writeFile(t, root, "docs/evidence/value.md", "# Evidence\n")
	writeFile(t, root, "docs/contracts/claim-registry.yaml", `version: "1"
tiers:
  PROVEN:
    cite_allowed: ["**"]
claims:
  AOP-CLAIM-PRODUCT-VALUE:
    tier: PROVEN
    surfaces:
      - PRODUCT.md
    evidence:
      - docs/evidence/value.md
`)

	report, err := Check(context.Background(), Options{
		RepoRoot:    root,
		Base:        "origin/main",
		ChangedOnly: true,
		Workspace: testWorkspace{run: fakeGit(map[string]fakeGitResult{
			"diff --name-only origin/main...HEAD":                {out: "PRODUCT.md\n"},
			"diff --name-only HEAD":                              {},
			"ls-files --others --exclude-standard":               {},
			"ls-files --error-unmatch -- docs/evidence/value.md": {out: "docs/evidence/value.md\n"},
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Claims != 1 || report.Summary.ChangedSurfaces != 1 {
		t.Fatalf("summary = %+v", report.Summary)
	}
	card := report.Cards[0]
	if card.ClaimID != "AOP-CLAIM-PRODUCT-VALUE" || card.Surface != "PRODUCT.md" {
		t.Fatalf("wrong card identity: %+v", card)
	}
	if card.Tier != "PROVEN" || card.Verdict != "supported" {
		t.Fatalf("wrong proof status: %+v", card)
	}
	if !card.CitationOK {
		t.Fatalf("citation should be allowed: %+v", card)
	}
	if len(card.Evidence) != 1 || card.Evidence[0].Status != "tracked" {
		t.Fatalf("evidence not classified as tracked: %+v", card.Evidence)
	}
	if !strings.Contains(card.NextAction, "ao gate check") {
		t.Fatalf("next action should include validation gate, got %q", card.NextAction)
	}
}

func TestCheckAgentsOnlyEvidenceIsNotCitable(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "# Readme\n\n<!-- agentops:claim:AOP-CLAIM-README-FAST -->\n\nClaim.\n")
	writeFile(t, root, ".agents/findings/fast.md", "# Local finding\n")
	writeFile(t, root, "docs/contracts/claim-registry.yaml", `version: "1"
tiers:
  PILOT:
    cite_allowed: ["**"]
claims:
  AOP-CLAIM-README-FAST:
    tier: PILOT
    surfaces:
      - README.md
    evidence:
      - .agents/findings/fast.md
`)

	report, err := Check(context.Background(), Options{
		RepoRoot:    root,
		ChangedOnly: true,
		Workspace: testWorkspace{run: fakeGit(map[string]fakeGitResult{
			"diff --name-only origin/main...HEAD":  {out: "README.md\n"},
			"diff --name-only HEAD":                {},
			"ls-files --others --exclude-standard": {},
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Cards) != 1 {
		t.Fatalf("cards = %d, want 1", len(report.Cards))
	}
	card := report.Cards[0]
	if card.Verdict != "not_citable" {
		t.Fatalf("verdict = %q, want not_citable: %+v", card.Verdict, card)
	}
	if len(card.Evidence) != 1 || card.Evidence[0].Status != "not_citable" {
		t.Fatalf("evidence status = %+v", card.Evidence)
	}
	if !strings.Contains(card.NextAction, "export .agents evidence") {
		t.Fatalf("next action does not explain export: %q", card.NextAction)
	}
}

func TestCheckFlagsCitationCeilingBeforeEvidenceStatus(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "PRODUCT.md", "# Product\n\n<!-- agentops:claim:AOP-CLAIM-PRODUCT-VALUE -->\n\nA changed claim.\n")
	writeFile(t, root, "docs/evidence/value.md", "# Evidence\n")
	writeFile(t, root, "docs/contracts/claim-registry.yaml", `version: "1"
tiers:
  UNPROVEN:
    cite_allowed: ["docs/comparisons/**", "docs/wiki-for-agents.md"]
claims:
  AOP-CLAIM-PRODUCT-VALUE:
    tier: UNPROVEN
    surfaces:
      - PRODUCT.md
    evidence:
      - docs/evidence/value.md
`)

	report, err := Check(context.Background(), Options{
		RepoRoot:    root,
		Base:        "origin/main",
		ChangedOnly: true,
		Workspace: testWorkspace{run: fakeGit(map[string]fakeGitResult{
			"diff --name-only origin/main...HEAD":                {out: "PRODUCT.md\n"},
			"diff --name-only HEAD":                              {},
			"ls-files --others --exclude-standard":               {},
			"ls-files --error-unmatch -- docs/evidence/value.md": {out: "docs/evidence/value.md\n"},
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Cards[0].Verdict; got != "citation_ceiling" {
		t.Fatalf("verdict = %q, want citation_ceiling: %+v", got, report.Cards[0])
	}
	if report.Cards[0].CitationOK {
		t.Fatalf("citation should not be allowed: %+v", report.Cards[0])
	}
}

func TestCheckNoChangedClaimsIsCleanNoop(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "cli/cmd/ao/foo.go", "package main\n")
	writeFile(t, root, "docs/contracts/claim-registry.yaml", `version: "1"
tiers: {}
claims: {}
`)

	report, err := Check(context.Background(), Options{
		RepoRoot:    root,
		ChangedOnly: true,
		Workspace: testWorkspace{run: fakeGit(map[string]fakeGitResult{
			"diff --name-only origin/main...HEAD":  {out: "cli/cmd/ao/foo.go\n"},
			"diff --name-only HEAD":                {},
			"ls-files --others --exclude-standard": {},
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Cards) != 0 {
		t.Fatalf("cards = %+v, want empty", report.Cards)
	}
	if report.Summary.Claims != 0 || report.Summary.ChangedSurfaces != 0 {
		t.Fatalf("summary = %+v, want clean no-op", report.Summary)
	}
}

type fakeGitResult struct {
	out string
	err error
}

type testGitRunner func(context.Context, string, ...string) (string, error)

type testWorkspace struct{ run testGitRunner }

func (testWorkspace) WorkingDirectory() (string, error)    { return os.Getwd() }
func (testWorkspace) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (testWorkspace) Stat(path string) error {
	_, err := os.Stat(path)
	return err
}
func (testWorkspace) IsNotExist(err error) bool { return os.IsNotExist(err) }
func (workspace testWorkspace) Git(ctx context.Context, root string, args ...string) (string, error) {
	return workspace.run(ctx, root, args...)
}

func fakeGit(results map[string]fakeGitResult) testGitRunner {
	return func(_ context.Context, _ string, args ...string) (string, error) {
		key := strings.Join(args, " ")
		if result, ok := results[key]; ok {
			return result.out, result.err
		}
		return "", errors.New("unexpected git command: " + key)
	}
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
