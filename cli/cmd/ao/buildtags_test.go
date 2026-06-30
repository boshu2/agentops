package main

import (
	"bytes"
	"testing"
)

// TestBuildtags_DefaultBuildIsSpine asserts that the default build (no archive
// build tags) reports the spine: archiveBuildTags is empty and `ao buildtags`
// prints the spine line. The flywheel/legacy variants are exercised by the
// build-tag verification script (scripts/verify-buildtags.sh), which actually
// compiles with -tags; here we lock the default-build contract.
func TestBuildtags_DefaultBuildIsSpine(t *testing.T) {
	// This asserts the DEFAULT (spine) build contract. Under -tags flywheel/legacy
	// the binary is intentionally a restored superset, so archiveBuildTags is
	// non-empty there by design — that path is covered by verify-buildtags.sh.
	if len(archiveBuildTags) != 0 {
		t.Skipf("restored build (archive tags %v); spine contract is asserted only in the default build", archiveBuildTags)
	}

	var out bytes.Buffer
	buildtagsCmd.SetOut(&out)
	t.Cleanup(func() { buildtagsCmd.SetOut(nil) })

	if err := buildtagsCmd.RunE(buildtagsCmd, nil); err != nil {
		t.Fatalf("buildtags RunE: %v", err)
	}
	got := out.String()
	if !bytes.Contains([]byte(got), []byte("spine")) {
		t.Fatalf("default build buildtags output = %q, want it to mention 'spine'", got)
	}
}

// TestBuildtags_HiddenFromDefaultSurface keeps the introspection command off the
// public command surface so adding the mechanism is no user-facing behavior
// change.
func TestBuildtags_HiddenFromDefaultSurface(t *testing.T) {
	if !buildtagsCmd.Hidden {
		t.Fatal("buildtags must stay Hidden so the default command surface is unchanged")
	}
}
