package provenanceapp

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMineSession_CheckpointACLRejected(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "state.json")
	session := writeMineSession(t, dir, "session.jsonl", "{\"type\":\"tool_use\",\"tool_name\":\"Read\",\"tool_input\":{}}\n")
	opts := MineOptions{File: session, State: state}
	if _, err := mine(t, opts); err != nil {
		t.Fatal(err)
	}
	// Builtin Guests SID avoids relying on localized account names.
	if out, err := exec.Command("icacls", state, "/deny", "*S-1-5-32-546:(R)").CombinedOutput(); err != nil {
		t.Fatalf("set restrictive ACL: %v: %s", err, out)
	}
	info, err := os.Stat(state)
	if err != nil {
		t.Fatal(err)
	}
	beforeACL, err := checkpointSecurity(state, info)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mine(t, opts); err == nil || !strings.Contains(err.Error(), "permissions") {
		t.Errorf("update with non-preservable permissions: %v", err)
	}
	after, err := os.ReadFile(state)
	if err != nil || !bytes.Equal(before, after) {
		t.Errorf("checkpoint changed: %v", err)
	}
	afterACL, err := checkpointSecurity(state, info)
	if err != nil || !bytes.Equal(beforeACL, afterACL) {
		t.Errorf("restrictive ACL changed: %v", err)
	}
}
