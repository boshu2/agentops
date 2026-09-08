package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boshu2/agentops/cli/internal/evidence"
)

// The real root process runs with an empty PATH. This leaf cannot fall back to
// the old shell/Python scanner, and its advisory and incomplete exit codes must
// survive root composition rather than only succeeding inside the module.
func TestProvenanceEvidenceOrphansNative(t *testing.T) {
	bin := aoBinary(t)
	base := t.TempDir()
	root := filepath.Join(base, "consumer")
	home := filepath.Join(base, "home")
	for _, dir := range []string{filepath.Join(root, "docs/evals/scorecards"), home} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	artifact := filepath.Join(root, "docs/evals/scorecards/sc.json")
	payload := []byte(`{"evaluator":{"h":{"path":"missing","sha256":"old"}}}`)
	if err := os.WriteFile(artifact, payload, 0600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(base, "config.yaml")
	if err := os.WriteFile(config, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	execute := func(args ...string) ([]byte, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Dir = root
		cmd.Env = []string{"PATH=", "HOME=" + home, "TMPDIR=" + base, "AGENTOPS_CONFIG=" + config, "AGENTOPS_OUTPUT=table"}
		return cmd.CombinedOutput()
	}
	args := []string{"provenance", "evidence-orphans", "--root", root, "--changed", "missing", "--changed", "missing"}
	out, err := execute(args...)
	if err != nil {
		t.Fatalf("native empty-PATH scan: %v\n%s", err, out)
	}
	var result evidence.OrphanReceipt
	if err = json.Unmarshal(out, &result); err != nil {
		t.Fatalf("JSON: %v\n%s", err, out)
	}
	if result.BindingCount != 1 || result.ArtifactCount != 1 || result.Orphaned[0].Cause != "both" || !reflect.DeepEqual(result.Changed, []string{"missing", "missing"}) {
		t.Fatalf("wrong complete receipt: %+v", result)
	}
	for _, format := range []string{"table", "yaml"} {
		got, err := execute(append(args, "--output", format)...)
		var exit *exec.ExitError
		ok := errors.As(err, &exit)
		if !ok || exit.ExitCode() != 2 || !strings.Contains(string(got), "JSON is the only") {
			t.Fatalf("%s: wanted exit2 got %v %s", format, err, got)
		}
	}
	for _, bad := range [][]string{{"provenance", "evidence-orphans"}, append(append([]string(nil), args...), "--text")} {
		got, err := execute(bad...)
		var exit *exec.ExitError
		ok := errors.As(err, &exit)
		if !ok || exit.ExitCode() != 2 {
			t.Fatalf("misuse: wanted exit2 got %v %s", err, got)
		}
	}
	after, err := os.ReadFile(artifact)
	if err != nil || string(after) != string(payload) {
		t.Fatalf("reader changed artifact: %v", err)
	}
	if err := os.WriteFile(artifact, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err = execute(args...)
	var exit *exec.ExitError
	ok := errors.As(err, &exit)
	if !ok || exit.ExitCode() != 2 || !strings.Contains(string(out), "malformed JSON") || strings.Contains(string(out), `"binding_count"`) {
		t.Fatalf("incomplete scan emitted a receipt or wrong exit: %v %s", err, out)
	}
	var unexpected []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && path != artifact {
			unexpected = append(unexpected, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(unexpected) != 0 {
		t.Fatalf("reader wrote consumer files: %v", unexpected)
	}
}
