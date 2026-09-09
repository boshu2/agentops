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
	if err := os.Chmod(state, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("/bin/chmod", "+a", "user:nobody deny read", state).CombinedOutput(); err != nil {
		t.Fatalf("set restrictive ACL: %v: %s", err, out)
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
	acl, err := exec.Command("/bin/ls", "-le", state).CombinedOutput()
	if err != nil || !strings.Contains(string(acl), "deny read") {
		t.Errorf("restrictive ACL lost: %v: %s", err, acl)
	}
}

func TestMineSession_CheckpointInheritedACLRejected(t *testing.T) {
	dir := t.TempDir()
	state := filepath.Join(dir, "state.json")
	session := writeMineSession(t, dir, "session.jsonl", "{\"type\":\"tool_use\",\"tool_name\":\"Read\",\"tool_input\":{}}\n")
	opts := MineOptions{File: session, State: state}
	if _, err := mine(t, opts); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("/bin/chmod", "+a", "user:nobody allow read,file_inherit", dir).CombinedOutput(); err != nil {
		t.Fatalf("set inheritable ACL: %v: %s", err, out)
	}
	if _, err := mine(t, opts); err == nil || !strings.Contains(err.Error(), "permissions") {
		t.Errorf("update with different inherited permissions: %v", err)
	}
	after, err := os.ReadFile(state)
	if err != nil || !bytes.Equal(before, after) {
		t.Errorf("checkpoint changed: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Errorf("permission probe left files: %v, %v", entries, err)
	}
}
