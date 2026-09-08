package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// Exercise the assembled Cobra root, including its output/configuration seams.
// Direct package tests cannot detect a different default in the root host.
func TestProvenanceEvidenceComposedWorkflow(t *testing.T) {
	bin := aoBinary(t)
	base := t.TempDir()
	consumer, evidenceRoot := filepath.Join(base, "consumer"), filepath.Join(base, "evidence")
	for _, dir := range []string{consumer, evidenceRoot} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, value string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	configFile := filepath.Join(base, "config.yaml")
	write(configFile, "{}\n")
	t.Setenv("AGENTOPS_CONFIG", configFile)
	t.Setenv("AGENTOPS_OUTPUT", "table")
	for _, key := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_WORK_TREE", "GIT_INDEX_FILE"} {
		value, set := os.LookupEnv(key)
		t.Cleanup(func() {
			if set {
				_ = os.Setenv(key, value)
			} else {
				_ = os.Unsetenv(key)
			}
		})
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	execute := func(args ...string) (string, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Dir = consumer
		// Each invocation has real process defaults; the in-process flag-reset
		// harness cannot reset StringArray defaults faithfully. Empty PATH also
		// proves that these installed commands need neither Git nor Python.
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "PATH=") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "PATH=")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	run := func(path string, args ...string) string {
		t.Helper()
		out, err := execute(append(strings.Fields(path), args...)...)
		if err != nil {
			t.Fatalf("%s: %v\n%s", path, err, out)
		}
		return out
	}
	field := func(out, name string) string {
		t.Helper()
		var value map[string]any
		if err := json.Unmarshal([]byte(out), &value); err != nil {
			t.Fatalf("invalid JSON output: %v\n%s", err, out)
		}
		text, ok := value[name].(string)
		if !ok || text == "" {
			t.Fatalf("missing %s: %s", name, out)
		}
		return text
	}
	intent, subject := filepath.Join(base, "intent.md"), filepath.Join(consumer, "a[1].md")
	write(intent, "The exact subject must satisfy the fixture criterion.\n")
	write(subject, "fixture subject\n")
	snapshot := run("provenance snapshot-intent", "--source", intent, "--evidence-root", evidenceRoot)
	intentRef := field(snapshot, "intent_ref")
	// The store returns a canonical path; macOS temp roots may use /var's
	// symlink spelling. Compare the same filesystem identity on both sides.
	canonicalEvidenceRoot, err := filepath.EvalSymlinks(evidenceRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(intentRef, canonicalEvidenceRoot+string(os.PathSeparator)) {
		t.Fatalf("snapshot escaped evidence root: %s", intentRef)
	}
	manifest := filepath.Join(evidenceRoot, "manifest.json")
	manifestArgs := []string{"--root", consumer, "--include", "a[1].md"}
	for _, flag := range []string{"--output", "-o"} {
		for _, format := range []string{"json", "yaml"} {
			out, err := execute(append([]string{flag, format, "provenance", "manifest"}, manifestArgs...)...)
			var value map[string]any
			if err != nil {
				t.Errorf("manifest %s %s: %v\n%s", flag, format, err, out)
				continue
			}
			if format == "json" {
				err = json.Unmarshal([]byte(out), &value)
			} else {
				if json.Valid([]byte(out)) {
					t.Errorf("manifest %s yaml emitted JSON", flag)
				}
				err = yaml.Unmarshal([]byte(out), &value)
			}
			if err != nil || value["schema_version"] != "subject-manifest.v1" {
				t.Errorf("manifest %s %s has wrong schema or format: %v\n%s", flag, format, err, out)
			}
		}
	}
	run("provenance manifest", "--root", consumer, "--include", "a[1].md", "--out", "manifest.json", "--evidence-root", evidenceRoot, "--output", "json")
	run("provenance verify-manifest", "--root", consumer, "--manifest", manifest)
	input := filepath.Join(base, "digest.json")
	write(input, `{"b":2,"a":1}`)
	wantDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(`{"a":1,"b":2}`)))
	if got := strings.TrimSpace(run("provenance digest", input)); got != wantDigest {
		t.Errorf("default digest output = %q, want plain %q", got, wantDigest)
	}
	if got := field(run("provenance digest", input, "--json"), "digest"); got != wantDigest {
		t.Errorf("JSON digest = %q, want %q", got, wantDigest)
	}
	draft := filepath.Join(base, "draft.json")
	write(draft, `{"verdict":"PASS","criteria":[{"id":"fixture","result":"PASS","evidence_refs":["fixture-check"]}],"findings":[],"evidence_refs":["fixture-check"],"checked":["a[1].md"],"not_checked":[],"validated_at":"2026-01-01T00:00:00Z"}`)
	storeArgs := []string{"--root", consumer, "--draft", draft, "--intent-source", intent, "--subject-manifest", manifest,
		"--author-context-id", "fixture-author", "--validator-context-id", "fixture-judge",
		"--freshness-source", "runtime", "--freshness-attester-id", "fixture-runtime", "--scope-result", "PASS"}
	for _, leaf := range []struct {
		name string
		args []string
	}{
		{"snapshot-intent", []string{"--source", intent}},
		{"manifest", append(append([]string(nil), manifestArgs...), "--out", "manifest.json")},
		{"store-verdict", storeArgs},
	} {
		for _, guard := range []struct {
			name string
			args []string
			want string
		}{
			{"dry-run", []string{"--dry-run"}, "evidence writes are disabled"},
			{"version", []string{"--helper-version", "unsupported"}, "incompatible evidence helper version"},
			{"yaml-conflict", []string{"--output", "yaml", "--json"}, "conflicting output formats"},
			{"table-conflict", []string{"-o", "table", "--json"}, "conflicting output formats"},
		} {
			dest := filepath.Join(base, leaf.name+"-"+guard.name)
			if err := os.Mkdir(dest, 0700); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"provenance", leaf.name, "--evidence-root", dest}, leaf.args...)
			out, err := execute(append(args, guard.args...)...)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(out, guard.want) {
				t.Errorf("%s %s: wanted exit 1 for %q, got %v\n%s", leaf.name, guard.name, guard.want, err, out)
			}
			entries, err := os.ReadDir(dest)
			if err != nil || len(entries) != 0 {
				t.Errorf("%s %s wrote evidence before rejection: %v %v", leaf.name, guard.name, entries, err)
			}
		}
	}
	stored := run("provenance store-verdict", "--root", consumer, "--evidence-root", evidenceRoot, "--draft", draft,
		"--intent-source", intent, "--subject-manifest", manifest, "--author-context-id", "fixture-author",
		"--validator-context-id", "fixture-judge", "--freshness-source", "runtime", "--freshness-attester-id", "fixture-runtime", "--scope-result", "PASS")
	verdict := field(stored, "path")
	run("provenance verify-verdict", "--verdict", verdict)
	verify := []string{"provenance", "verify-subject", "--root", consumer, "--manifest", manifest, "--verdict", verdict, "--intent", intent}
	run("provenance verify-subject", verify[2:]...)
	otherIntent := filepath.Join(base, "other-intent.md")
	write(otherIntent, "Different acceptance.\n")
	wrong := append([]string(nil), verify...)
	wrong[len(wrong)-1] = otherIntent
	if _, err := execute(wrong...); err == nil {
		t.Error("root command admitted a different expected intent")
	}
	write(subject, "changed after judgment\n")
	if _, err := execute(verify...); err == nil {
		t.Error("root command admitted a changed subject")
	}
	files, err := os.ReadDir(consumer)
	if err != nil || len(files) != 1 || files[0].Name() != "a[1].md" {
		t.Fatalf("evidence leaked into consumer: %v %v", files, err)
	}
}
