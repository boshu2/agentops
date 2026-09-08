package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectCommentErrorsAreNotHiddenByShow(t *testing.T) {
	root := t.TempDir()
	bd := filepath.Join(root, "bd")
	// This adapter-only fixture makes show succeed while the direct comments read
	// fails, reproducing the native failure the service must never swallow.
	script := "#!/bin/sh\ncase \" $* \" in\n*' show '*) printf '[{\"id\":\"fixture-anchor\"}]';;\n*' comments '*) exit 17;;\n*) exit 19;;\nesac\n"
	if err := os.WriteFile(bd, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root)
	_, err := (Gateway{}).AnchorComments(context.Background(), root, "fixture-anchor")
	if err == nil || !strings.Contains(err.Error(), "exit status 17") {
		t.Fatalf("direct read failure swallowed: %v", err)
	}
}
func TestNativeResponseLimitRejectsRatherThanTruncates(t *testing.T) {
	var out limitedContextOutput
	payload := make([]byte, (16<<20)+1)
	if n, err := out.Write(payload); err != nil || n != len(payload) {
		t.Fatalf("bounded sink: %d %v", n, err)
	}
	if !out.overflow || out.Len() != 16<<20 {
		t.Fatalf("overflow not retained as failure: %+v", out.overflow)
	}
	if _, err := out.Write([]byte("trailing")); err != nil || out.Len() != 16<<20 {
		t.Fatal("overflow sink grew")
	}
}
