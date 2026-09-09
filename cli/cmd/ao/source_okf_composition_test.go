package main

import (
	"bytes"
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
)

func TestSourceReadAndOKFComposedBoundary(t *testing.T) {
	bin := aoBinary(t)
	base, consumer := t.TempDir(), t.TempDir()
	page := "---\ntype: observation\ntitle: PRIVATE_INPUT_CANARY\ndescription: A synthetic observation.\nstatus: draft\nsources:\n  - resource: synthetic:episode\nknowledge_use: maintained-reference\napplicability: One synthetic episode.\nclaim: The fixture has complete metadata.\nlimitations: No real-world claim.\nconsumer: Profile checker test.\nretirement_condition: When the profile changes.\n---\n"
	valid, missingStatus := filepath.Join(base, "valid.md"), filepath.Join(base, "missing-status.md")
	for name, contents := range map[string]string{valid: page, missingStatus: strings.Replace(page, "status: draft\n", "", 1)} {
		if err := os.WriteFile(name, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name       string
		args       []string
		exit       int
		stderr     string
		structured bool
		valid      bool
	}{
		// ao session read-source must deny missing policy before opening a source or invoking BD.
		{"source-denied", []string{"session", "read-source", "--file", filepath.Join(base, "missing-private-source"), "--start-byte", "0", "--max-bytes", "1"}, 1, "explicit access policy is required", false, false},
		{"source-output-profile", []string{"-o", "yaml", "session", "read-source", "--file", valid, "--start-byte", "0", "--max-bytes", "1"}, 1, "supports JSON only", false, false},
		// ao provenance check-okf checks structure without echoing or admitting candidate content.
		{"okf-valid", []string{"provenance", "check-okf", "--file", valid}, 0, "", true, true},
		{"okf-missing-status", []string{"provenance", "check-okf", "--file", missingStatus, "--json"}, 1, "structural findings", true, false},
		{"okf-unsupported-profile", []string{"provenance", "check-okf", "--file", filepath.Join(base, "missing-private.md"), "--profile", "unknown"}, 1, "unsupported profile", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, tc.args...)
			cmd.Dir = consumer
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "PATH=") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			cmd.Env = append(cmd.Env, "PATH=")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			code := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			if code != tc.exit || !strings.Contains(stderr.String(), tc.stderr) {
				t.Fatalf("exit %d, want %d; stderr=%s", code, tc.exit, &stderr)
			}
			if strings.Contains(stdout.String()+stderr.String(), "PRIVATE_INPUT_CANARY") {
				t.Fatal("candidate content echoed by structural or denied-read path")
			}
			if tc.structured {
				var report struct {
					Valid     bool   `json:"structurally_valid"`
					Digest    string `json:"content_sha256"`
					Assurance string `json:"assurance"`
				}
				if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
					t.Fatal(err)
				}
				if report.Valid != tc.valid || report.Assurance == "" || len(report.Digest) != 64 {
					t.Fatalf("incomplete structural report: %+v", report)
				}
				if tc.valid && report.Digest != fmt.Sprintf("%x", sha256.Sum256([]byte(page))) {
					t.Fatal("report is not bound to exact input bytes")
				}
			} else if stdout.Len() != 0 {
				t.Fatalf("preflight emitted content: %s", &stdout)
			}
			entries, err := os.ReadDir(consumer)
			if err != nil || len(entries) != 0 {
				t.Fatalf("consumer mutated: %v %v", entries, err)
			}
		})
	}
}

func TestSourceReadAndOKFCapabilities(t *testing.T) {
	for _, tc := range []struct{ path, id, args, output, effects string }{
		{"ao session", "ao.session", "arbitrary", "text", "filesystem,process,environment"},
		{"ao session read-source", "ao.session.read-source", "none", "structured", "filesystem,process,environment"},
		{"ao provenance check-okf", "ao.provenance.check-okf", "no-args", "structured", "filesystem"},
	} {
		entry := capabilityEntry(t, tc.path)
		if entry.ID != tc.id || entry.Args != tc.args || entry.Output != tc.output || entry.Effects != tc.effects || entry.ExitCodes["0"] != "success" || entry.ExitCodes["1"] != "failure" {
			t.Errorf("%s: %+v", tc.path, entry)
		}
	}
}
