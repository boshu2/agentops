// practices: [ai-assisted-dev, wiki-knowledge-surface]
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFakePython3(t *testing.T) string {
	t.Helper()

	binDir := t.TempDir()
	pythonPath := filepath.Join(binDir, "python3")
	script := `#!/bin/sh
set -eu
if [ -n "${MIND_TEST_ARGS_FILE:-}" ]; then
  : > "$MIND_TEST_ARGS_FILE"
  for arg in "$@"; do
    printf '%s\n' "$arg" >> "$MIND_TEST_ARGS_FILE"
  done
fi
exit "${MIND_TEST_EXIT_CODE:-0}"
`
	if err := os.WriteFile(pythonPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake python3: %v", err)
	}
	return binDir
}

func readArgsFile(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read args file: %v", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func TestMindRunFunc(t *testing.T) {
	t.Run("dry-run inversion", func(t *testing.T) {
		tests := []struct {
			name      string
			dryRun    bool
			wantWrite bool
		}{
			{name: "dry-run false adds --write", dryRun: false, wantWrite: true},
			{name: "dry-run true omits --write", dryRun: true, wantWrite: false},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				origDryRun := dryRun
				t.Cleanup(func() { dryRun = origDryRun })

				_ = chdirTemp(t)
				argsFile := filepath.Join(t.TempDir(), "mind-args.txt")
				binDir := writeFakePython3(t)

				t.Setenv("MIND_TEST_ARGS_FILE", argsFile)
				t.Setenv("MIND_TEST_EXIT_CODE", "0")
				t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

				dryRun = tc.dryRun
				if err := mindRunFunc("all")(mindCmd, nil); err != nil {
					t.Fatalf("mindRunFunc returned error: %v", err)
				}

				args := readArgsFile(t, argsFile)
				gotWrite := hasArg(args, "--write")
				if gotWrite != tc.wantWrite {
					t.Fatalf("args=%v, has --write=%v, want %v", args, gotWrite, tc.wantWrite)
				}
			})
		}
	})

	t.Run("builds --vault from cwd", func(t *testing.T) {
		origDryRun := dryRun
		t.Cleanup(func() { dryRun = origDryRun })

		cwd := chdirTemp(t)
		argsFile := filepath.Join(t.TempDir(), "mind-args.txt")
		binDir := writeFakePython3(t)

		t.Setenv("MIND_TEST_ARGS_FILE", argsFile)
		t.Setenv("MIND_TEST_EXIT_CODE", "0")
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		dryRun = true
		if err := mindRunFunc("graph")(mindCmd, nil); err != nil {
			t.Fatalf("mindRunFunc returned error: %v", err)
		}

		args := readArgsFile(t, argsFile)
		vaultIdx := -1
		for i, arg := range args {
			if arg == "--vault" {
				vaultIdx = i
				break
			}
		}
		if vaultIdx == -1 {
			t.Fatalf("expected --vault in args, got %v", args)
		}
		if vaultIdx+1 >= len(args) {
			t.Fatalf("expected vault path after --vault, got %v", args)
		}
		gotVault := args[vaultIdx+1]
		gotInfo, err := os.Stat(gotVault)
		if err != nil {
			t.Fatalf("stat vault path %q: %v", gotVault, err)
		}
		cwdInfo, err := os.Stat(cwd)
		if err != nil {
			t.Fatalf("stat cwd %q: %v", cwd, err)
		}
		if !os.SameFile(gotInfo, cwdInfo) {
			t.Fatalf("vault path = %q, want path to cwd %q", gotVault, cwd)
		}
	})

	t.Run("returns error when python3 is missing", func(t *testing.T) {
		origDryRun := dryRun
		t.Cleanup(func() { dryRun = origDryRun })

		dryRun = false
		t.Setenv("PATH", t.TempDir())

		err := mindRunFunc("scan")(mindCmd, nil)
		if err == nil {
			t.Fatal("expected error when python3 is not on PATH")
		}
		if !strings.Contains(err.Error(), "python3 not found") {
			t.Fatalf("expected python3-not-found error, got: %v", err)
		}
	})

	t.Run("wraps command failures", func(t *testing.T) {
		origDryRun := dryRun
		t.Cleanup(func() { dryRun = origDryRun })

		_ = chdirTemp(t)
		argsFile := filepath.Join(t.TempDir(), "mind-args.txt")
		binDir := writeFakePython3(t)

		t.Setenv("MIND_TEST_ARGS_FILE", argsFile)
		t.Setenv("MIND_TEST_EXIT_CODE", "7")
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		dryRun = false
		err := mindRunFunc("normalize")(mindCmd, nil)
		if err == nil {
			t.Fatal("expected error from failing python3 wrapper")
		}
		if !strings.Contains(err.Error(), "mind normalize failed") {
			t.Fatalf("expected wrapped error, got: %v", err)
		}
	})
}
