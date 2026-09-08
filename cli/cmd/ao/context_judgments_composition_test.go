package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContextJudgmentsComposedPreflight(t *testing.T) {
	bin := aoBinary(t)
	base, consumer := t.TempDir(), t.TempDir()
	configFile, profiles := filepath.Join(base, "config.yaml"), filepath.Join(base, "profiles.json")
	for path, contents := range map[string]string{
		configFile: "{}\n",
		profiles:   `{"profiles":[{"id":"required","runtime":"claude","model":"claude-test","family":"anthropic","effort":""}]}`,
	} {
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("AGENTOPS_CONFIG", configFile)
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		// ao config context must reject invalid output selection before BD runs.
		{"context-selection", []string{"config", "context", "--field", "invalid"}, "field must be bundle_root, evidence_root or staging_root"},
		// ao provenance verify-judgments must reject a denied provider before
		// reading nonexistent subject, acceptance or private candidate files.
		{"provider-denial", []string{"provenance", "verify-judgments", "--root", filepath.Join(base, "missing-subject"),
			"--manifest", filepath.Join(base, "missing-manifest"), "--intent", filepath.Join(base, "missing-intent"),
			"--evidence-root", base, "--author-context-id", "author", "--required-profiles", profiles,
			"--allowed-provider", "openai", "--verdict", filepath.Join(base, "missing-private-verdict")}, "provider_denied"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, test.args...)
			cmd.Dir = consumer
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "PATH=") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			cmd.Env = append(cmd.Env, "PATH=")
			out, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(string(out), test.want) {
				t.Fatalf("wanted exit 1 for %q, got %v\n%s", test.want, err, out)
			}
			entries, err := os.ReadDir(consumer)
			if err != nil || len(entries) != 0 {
				t.Fatalf("preflight wrote consumer state: %v %v", entries, err)
			}
		})
	}
}
