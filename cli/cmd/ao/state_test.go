package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestStateCommandsRegistered(t *testing.T) {
	for _, path := range [][]string{
		{"state"},
		{"state", "validate"},
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
	if err := runStateValidate(cmd, []string{filepath.Join(root, "schemas", "fixtures", "state-memory", "valid-finding.json")}); err != nil {
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
	err = runStateValidate(cmd, []string{filepath.Join(root, "schemas", "fixtures", "state-memory", "bad-finding-extra-field.json")})
	if err == nil {
		t.Fatal("runStateValidate accepted bad fixture")
	}
}

func TestStateAdmitCommandDefaultsDestination(t *testing.T) {
	root, err := repoRootOrCwd()
	if err != nil {
		t.Fatal(err)
	}
	oldFinding := stateAdmitFinding
	oldDestination := stateAdmitDestination
	oldMaxAgeDays := stateAdmitMaxAgeDays
	oldDryRun := dryRun
	t.Cleanup(func() {
		stateAdmitFinding = oldFinding
		stateAdmitDestination = oldDestination
		stateAdmitMaxAgeDays = oldMaxAgeDays
		dryRun = oldDryRun
	})
	stateAdmitFinding = filepath.Join(root, "schemas", "fixtures", "state-memory", "valid-finding.json")
	stateAdmitDestination = ""
	stateAdmitMaxAgeDays = 30
	dryRun = true

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := runStateAdmit(cmd, nil); err != nil {
		t.Fatalf("runStateAdmit(default destination): %v", err)
	}
	if !strings.Contains(out.String(), "Would admit finding-age-membrane-valid -> .agents/state/findings/finding-age-membrane-valid.json") {
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
