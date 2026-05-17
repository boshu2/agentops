// practices: [agile-manifesto, dora-metrics]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/domainslice"
)

// TestRPIPhasedScaffoldDomain_CreatesManifest verifies --scaffold-domain writes
// a manifest at the expected tracked path.
func TestRPIPhasedScaffoldDomain_CreatesManifest(t *testing.T) {
	cwd := t.TempDir()
	result, err := scaffoldDomainManifest(cwd, "payments", false)
	if err != nil {
		t.Fatalf("scaffoldDomainManifest: %v", err)
	}
	wantRel := filepath.Join("docs", "domains", "payments", "manifest.yaml")
	if result.ManifestPath != wantRel {
		t.Errorf("ManifestPath = %q, want %q", result.ManifestPath, wantRel)
	}
	if !result.Created || result.Overwritten {
		t.Errorf("expected Created=true Overwritten=false, got %+v", result)
	}
	if _, err := os.Stat(filepath.Join(cwd, wantRel)); err != nil {
		t.Errorf("manifest not on disk: %v", err)
	}
}

// TestRPIPhasedScaffoldDomain_RoundTrips verifies the scaffolded manifest
// validates against the F3.1 schema/loader — domainslice.Load must accept it.
func TestRPIPhasedScaffoldDomain_RoundTrips(t *testing.T) {
	cwd := t.TempDir()
	if _, err := scaffoldDomainManifest(cwd, "auth", false); err != nil {
		t.Fatalf("scaffoldDomainManifest: %v", err)
	}
	abs := filepath.Join(cwd, "docs", "domains", "auth", "manifest.yaml")

	manifest, err := domainslice.Load(abs)
	if err != nil {
		t.Fatalf("scaffolded manifest did not round-trip through domainslice.Load: %v", err)
	}
	if manifest.Domain != "auth" {
		t.Errorf("loaded Domain = %q, want auth", manifest.Domain)
	}
	if manifest.SchemaVersion != 1 {
		t.Errorf("loaded SchemaVersion = %d, want 1", manifest.SchemaVersion)
	}
	if manifest.Version != "0.1.0" {
		t.Errorf("loaded Version = %q, want 0.1.0", manifest.Version)
	}
	if len(manifest.ContextRoots) == 0 {
		t.Error("loaded ContextRoots is empty; schema requires minItems:1")
	}
	if manifest.Owner == "" {
		t.Error("loaded Owner is empty; schema requires it")
	}
}

// TestRPIPhasedScaffoldDomain_AcceptedByDomainFlag verifies the scaffolded slice
// is accepted by `ao rpi phased --domain` (resolveDomainEvidence loads it).
func TestRPIPhasedScaffoldDomain_AcceptedByDomainFlag(t *testing.T) {
	cwd := t.TempDir()
	if _, err := scaffoldDomainManifest(cwd, "shipping", false); err != nil {
		t.Fatalf("scaffoldDomainManifest: %v", err)
	}
	evidence, err := resolveDomainEvidence(cwd, phasedEngineOptions{Domain: "shipping"})
	if err != nil {
		t.Fatalf("resolveDomainEvidence on scaffolded slice: %v", err)
	}
	if evidence == nil {
		t.Fatal("expected non-nil evidence for scaffolded --domain slice")
	}
	if evidence.Domain != "shipping" {
		t.Errorf("evidence.Domain = %q, want shipping", evidence.Domain)
	}
	if evidence.ManifestPath != filepath.Join("docs", "domains", "shipping", "manifest.yaml") {
		t.Errorf("evidence.ManifestPath = %q, unexpected", evidence.ManifestPath)
	}
}

// TestRPIPhasedScaffoldDomain_NoOverwriteWithoutForce verifies an existing
// manifest is NOT overwritten unless --force is passed.
func TestRPIPhasedScaffoldDomain_NoOverwriteWithoutForce(t *testing.T) {
	cwd := t.TempDir()
	if _, err := scaffoldDomainManifest(cwd, "payments", false); err != nil {
		t.Fatalf("first scaffold: %v", err)
	}
	abs := filepath.Join(cwd, "docs", "domains", "payments", "manifest.yaml")
	marker := "# operator edit -- keep me\n"
	original, _ := os.ReadFile(abs)
	if err := os.WriteFile(abs, append([]byte(marker), original...), 0o644); err != nil {
		t.Fatalf("mark file: %v", err)
	}

	_, err := scaffoldDomainManifest(cwd, "payments", false)
	if err == nil {
		t.Fatal("expected error scaffolding over an existing manifest without --force")
	}
	if !strings.Contains(err.Error(), "already exists") || !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should mention 'already exists' and '--force', got: %v", err)
	}
	after, _ := os.ReadFile(abs)
	if !strings.Contains(string(after), marker) {
		t.Error("operator edit was clobbered by a no-force scaffold")
	}
}

// TestRPIPhasedScaffoldDomain_ForceOverwrites verifies --force replaces an
// existing manifest and the result records Overwritten.
func TestRPIPhasedScaffoldDomain_ForceOverwrites(t *testing.T) {
	cwd := t.TempDir()
	if _, err := scaffoldDomainManifest(cwd, "payments", false); err != nil {
		t.Fatalf("first scaffold: %v", err)
	}
	abs := filepath.Join(cwd, "docs", "domains", "payments", "manifest.yaml")
	if err := os.WriteFile(abs, []byte("# stale content\n"), 0o644); err != nil {
		t.Fatalf("stale write: %v", err)
	}

	result, err := scaffoldDomainManifest(cwd, "payments", true)
	if err != nil {
		t.Fatalf("scaffold with --force: %v", err)
	}
	if !result.Overwritten || result.Created {
		t.Errorf("expected Overwritten=true Created=false, got %+v", result)
	}
	if _, err := domainslice.Load(abs); err != nil {
		t.Errorf("force-overwritten manifest invalid: %v", err)
	}
}

// TestRPIPhasedScaffoldDomain_RejectsBadName verifies invalid domain names are
// rejected before any file is written.
func TestRPIPhasedScaffoldDomain_RejectsBadName(t *testing.T) {
	cwd := t.TempDir()
	for _, bad := range []string{"", "Payments", "1auth", "has space", "has_underscore"} {
		_, err := scaffoldDomainManifest(cwd, bad, false)
		if err == nil {
			t.Errorf("expected rejection for invalid domain name %q", bad)
		}
	}
	if _, err := os.Stat(filepath.Join(cwd, "docs", "domains")); !os.IsNotExist(err) {
		t.Error("invalid name should not create docs/domains/")
	}
}

// TestRPIPhasedScaffoldDomain_OutputNamesNextCommands verifies the scaffold
// summary names the follow-up commands (attach via --domain, dry-run, lint).
func TestRPIPhasedScaffoldDomain_OutputNamesNextCommands(t *testing.T) {
	cwd := t.TempDir()
	out, err := captureStdout(t, func() error {
		return runScaffoldDomain(cwd, "payments", false)
	})
	if err != nil {
		t.Fatalf("runScaffoldDomain: %v", err)
	}
	for _, want := range []string{
		"Created domain-slice manifest:",
		"docs/domains/payments/manifest.yaml",
		"ao rpi phased --domain payments",
		"--dry-run",
		"ao goals scenarios --lint",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("scaffold output missing %q\noutput:\n%s", want, out)
		}
	}
}

// TestRPIPhasedScaffoldDomain_DoesNotRunRPI verifies that --scaffold-domain
// only writes the manifest and exits -- it does NOT start an RPI run. The proof
// is that no RPI run artifacts (.agents/rpi/) are created.
func TestRPIPhasedScaffoldDomain_DoesNotRunRPI(t *testing.T) {
	cwd := t.TempDir()
	if err := runScaffoldDomain(cwd, "payments", false); err != nil {
		t.Fatalf("runScaffoldDomain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, "docs", "domains", "payments", "manifest.yaml")); err != nil {
		t.Errorf("manifest not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".agents", "rpi")); !os.IsNotExist(err) {
		t.Error("--scaffold-domain must not start an RPI run (.agents/rpi/ was created)")
	}
}
