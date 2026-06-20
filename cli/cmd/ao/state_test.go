package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/aostate"
	"github.com/spf13/cobra"
)

func TestStateCommandsRegistered(t *testing.T) {
	for _, path := range [][]string{
		{"state"},
		{"state", "validate"},
		{"state", "candidate"},
		{"state", "candidate", "validate"},
		{"state", "review-request"},
		{"state", "admit"},
		{"state", "verify"},
		{"state", "doctor"},
	} {
		cmd, _, err := rootCmd.Find(path)
		if err != nil {
			t.Fatalf("find %v: %v", path, err)
		}
		if cmd == nil {
			t.Fatalf("command %v not registered", path)
		}
	}
}

func TestStateValidateCommandAcceptsValidFixture(t *testing.T) {
	root, err := repoRootOrCwd()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runStateValidate(cmd, []string{filepath.Join(root, "schemas", "fixtures", "ao-state", "valid-candidate.json")}); err != nil {
		t.Fatalf("runStateValidate(valid): %v", err)
	}
	if !strings.Contains(out.String(), "Validated 1 state file") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestStateValidateCommandRejectsBadFixture(t *testing.T) {
	root, err := repoRootOrCwd()
	if err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	err = runStateValidate(cmd, []string{filepath.Join(root, "schemas", "fixtures", "ao-state", "bad-candidate-extra-field.json")})
	if err == nil {
		t.Fatal("runStateValidate accepted bad fixture")
	}
}

func TestAoStateCandidateValidateEmitsDigest(t *testing.T) {
	root, err := repoRootOrCwd()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runStateCandidateValidate(cmd, []string{filepath.Join(root, "schemas", "fixtures", "ao-state", "valid-candidate.json")}); err != nil {
		t.Fatalf("runStateCandidateValidate(valid): %v", err)
	}
	if !strings.Contains(out.String(), "Candidate valid:") || !strings.Contains(out.String(), "sha256:") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestAoStateAdmitCommandDefaultsDestination(t *testing.T) {
	root, err := repoRootOrCwd()
	if err != nil {
		t.Fatal(err)
	}
	candidatePath, verdictPath := writeCommandAdmissionFiles(t, root)
	oldFinding := stateAdmitFinding
	oldCandidate := stateAdmitCandidate
	oldVerdict := stateAdmitVerdict
	oldDestination := stateAdmitDestination
	oldMaxAgeDays := stateAdmitMaxAgeDays
	oldDryRun := dryRun
	t.Cleanup(func() {
		stateAdmitFinding = oldFinding
		stateAdmitCandidate = oldCandidate
		stateAdmitVerdict = oldVerdict
		stateAdmitDestination = oldDestination
		stateAdmitMaxAgeDays = oldMaxAgeDays
		dryRun = oldDryRun
	})
	stateAdmitCandidate = candidatePath
	stateAdmitVerdict = verdictPath
	stateAdmitFinding = ""
	stateAdmitDestination = ""
	stateAdmitMaxAgeDays = 30
	dryRun = true

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runStateAdmit(cmd, nil); err != nil {
		t.Fatalf("runStateAdmit(default destination): %v", err)
	}
	if !strings.Contains(out.String(), "Would admit finding-age-membrane-valid -> .ao/accepted/findings/finding-age-membrane-valid.json") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestStateVerifyCommandPassesCheckedInContracts(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetContext(context.Background())
	if err := runStateVerify(cmd, nil); err != nil {
		t.Fatalf("runStateVerify: %v\noutput:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "State verify: PASS") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func writeCommandAdmissionFiles(t *testing.T, root string) (string, string) {
	t.Helper()
	candidateBytes, err := os.ReadFile(filepath.Join(root, "schemas", "fixtures", "ao-state", "valid-candidate.json"))
	if err != nil {
		t.Fatal(err)
	}
	digest, err := aostate.CanonicalDigest(candidateBytes)
	if err != nil {
		t.Fatal(err)
	}
	verdictBytes, err := os.ReadFile(filepath.Join(root, "schemas", "fixtures", "ao-state", "valid-verdict.json"))
	if err != nil {
		t.Fatal(err)
	}
	var verdict aostate.AdmissionVerdict
	if err := json.Unmarshal(verdictBytes, &verdict); err != nil {
		t.Fatal(err)
	}
	verdict.CandidateDigest = digest
	verdictBytes, err = json.MarshalIndent(verdict, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	verdictBytes = append(verdictBytes, '\n')

	candidateRel := filepath.ToSlash(filepath.Join(".ao", "candidates", "findings", "finding-age-membrane-valid.json"))
	verdictRel := filepath.ToSlash(filepath.Join(".ao", "reviews", "finding-age-membrane-valid.verdict.json"))
	writeRepoTempFile(t, root, candidateRel, candidateBytes)
	writeRepoTempFile(t, root, verdictRel, verdictBytes)
	return filepath.Join(root, filepath.FromSlash(candidateRel)), filepath.Join(root, filepath.FromSlash(verdictRel))
}

func writeRepoTempFile(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Remove(path)
		dir := filepath.Dir(path)
		for {
			if dir == root || filepath.Base(dir) == ".ao" {
				_ = os.Remove(dir)
				return
			}
			_ = os.Remove(dir)
			dir = filepath.Dir(dir)
		}
	})
}
