// practices: [ddd-bounded-context, design-by-contract]
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestScopeCommand_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"scope"})
	if err != nil {
		t.Fatalf("scope command not found on rootCmd: %v", err)
	}
	if cmd.Name() != "scope" {
		t.Fatalf("want scope, got %q", cmd.Name())
	}
	wantSubs := map[string]bool{"freeze": false, "unfreeze": false, "status": false}
	for _, sub := range cmd.Commands() {
		if _, ok := wantSubs[sub.Name()]; ok {
			wantSubs[sub.Name()] = true
		}
	}
	for name, present := range wantSubs {
		if !present {
			t.Errorf("scope subcommand %q missing", name)
		}
	}
}

func TestScopeStatusJSON_EmptyLock(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "scope.lock")
	t.Setenv("AO_SCOPE_LOCK", lock)
	scopeJSON = true
	defer func() { scopeJSON = false }()

	cmd, _, err := rootCmd.Find([]string{"scope", "status"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("not JSON: %v / %q", err, buf.String())
	}
	if v, ok := out["schema_version"].(float64); !ok || v != 1 {
		t.Fatalf("schema_version: %v", out["schema_version"])
	}
}

func TestScopeFreezeThenStatus_NonJSON(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "scope.lock")
	t.Setenv("AO_SCOPE_LOCK", lock)
	scopeJSON = false

	freezeCmd, _, err := rootCmd.Find([]string{"scope", "freeze"})
	if err != nil {
		t.Fatalf("freeze find: %v", err)
	}
	buf := &bytes.Buffer{}
	freezeCmd.SetOut(buf)
	if err := freezeCmd.RunE(freezeCmd, []string{"cli/cmd/ao/"}); err != nil {
		t.Fatalf("freeze RunE: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("cli/cmd/ao")) {
		t.Fatalf("output missing dir: %q", buf.String())
	}

	if _, err := os.Stat(lock); err != nil {
		t.Fatalf("lock not created: %v", err)
	}

	statusCmd, _, err := rootCmd.Find([]string{"scope", "status"})
	if err != nil {
		t.Fatalf("status find: %v", err)
	}
	buf2 := &bytes.Buffer{}
	statusCmd.SetOut(buf2)
	if err := statusCmd.RunE(statusCmd, nil); err != nil {
		t.Fatalf("status RunE: %v", err)
	}
	if !bytes.Contains(buf2.Bytes(), []byte("1 frozen")) {
		t.Fatalf("status output missing count: %q", buf2.String())
	}
}

func TestScopeUnfreezeAll(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "scope.lock")
	t.Setenv("AO_SCOPE_LOCK", lock)
	scopeJSON = false

	freezeCmd, _, _ := rootCmd.Find([]string{"scope", "freeze"})
	if err := freezeCmd.RunE(freezeCmd, []string{"a/", "b/"}); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	unfreezeCmd, _, _ := rootCmd.Find([]string{"scope", "unfreeze"})
	buf := &bytes.Buffer{}
	unfreezeCmd.SetOut(buf)
	if err := unfreezeCmd.RunE(unfreezeCmd, nil); err != nil {
		t.Fatalf("unfreeze RunE: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("no frozen")) {
		t.Fatalf("unfreeze output missing 'no frozen': %q", buf.String())
	}
}
