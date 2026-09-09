package provenanceapp

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
	// Named UID 65534 has no access even though the mask/other bits allow read.
	acl := binary.LittleEndian.AppendUint32(nil, 2) // POSIX_ACL_XATTR_VERSION
	for _, entry := range []struct {
		tag, perm uint16
		id        uint32
	}{{1, 6, ^uint32(0)}, {2, 0, 65534}, {4, 4, ^uint32(0)}, {16, 4, ^uint32(0)}, {32, 4, ^uint32(0)}} {
		acl = binary.LittleEndian.AppendUint16(acl, entry.tag)
		acl = binary.LittleEndian.AppendUint16(acl, entry.perm)
		acl = binary.LittleEndian.AppendUint32(acl, entry.id)
	}
	const key = "system.posix_acl_access"
	if err := syscall.Setxattr(state, key, acl, 0); err != nil {
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
	actual := make([]byte, len(acl))
	n, err := syscall.Getxattr(state, key, actual)
	if err != nil || n != len(acl) || !bytes.Equal(acl, actual) {
		t.Errorf("restrictive ACL changed: %v", err)
	}
}
