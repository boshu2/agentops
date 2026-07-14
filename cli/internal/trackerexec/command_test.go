package trackerexec

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolvedCommandAppliesContextWorkDirChildEnvAndStreams(t *testing.T) {
	if _, err := os.Stat("command.go"); err != nil {
		t.Fatalf("trackerexec core behavior is missing: %v", err)
	}

	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	// Park under internal/testdata so go run stays inside the module, but
	// the quest json-marshal AST walk skips testdata (and ENOENT mid-walk).
	testdata := filepath.Join(moduleRoot, "internal", "testdata")
	if err := os.MkdirAll(testdata, 0o755); err != nil {
		t.Fatal(err)
	}
	helperDir, err := os.MkdirTemp(testdata, "trackerexec-contract-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(helperDir) })

	helper := `package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boshu2/agentops/cli/internal/trackerexec"
	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

func main() {
	root, err := os.MkdirTemp("", "trackerexec-behavior-")
	must(err)
	defer os.RemoveAll(root)
	workDir := filepath.Join(root, "work")
	must(os.Mkdir(workDir, 0o755))
	workDir, err = filepath.EvalSymlinks(workDir)
	must(err)
	script := filepath.Join(root, "tracker")
	must(os.WriteFile(script, []byte("#!/bin/sh\nIFS= read -r input\nprintf 'cwd=%s\\nmarker=%s\\nargc=%s\\narg1=%s\\narg2=%s\\nstdin=%s\\nleak=%s\\n' \"$PWD\" \"$MARKER\" \"$#\" \"$1\" \"$2\" \"$input\" \"${SHOULD_NOT_INHERIT-unset}\"\nprintf 'stderr=%s\\n' \"$ERR_MARKER\" >&2\nexit 23\n"), 0o755))

	resolution := trackerresolve.Resolution{
		Tracker: trackerresolve.BR,
		Binary: script,
		WorkDir: workDir,
		ChildEnv: []string{"MARKER=child", "ERR_MARKER=child-error"},
	}
	var stdout, stderr bytes.Buffer
	command := (trackerexec.Factory{}).Command(
		context.Background(),
		resolution,
		[]string{"arg one", "arg-two"},
		trackerexec.Streams{Stdin: strings.NewReader("input-value\n"), Stdout: &stdout, Stderr: &stderr},
	)
	resolution.ChildEnv[0] = "MARKER=mutated-after-construction"
	err = command.Run()
	var exit *trackerexec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 {
		fail("typed exit = %T %v, want *trackerexec.ExitError(23)", err, err)
	}
	wantOut := fmt.Sprintf("cwd=%s\nmarker=child\nargc=2\narg1=arg one\narg2=arg-two\nstdin=input-value\nleak=unset\n", workDir)
	if stdout.String() != wantOut {
		fail("stdout = %q, want %q", stdout.String(), wantOut)
	}
	if stderr.String() != "stderr=child-error\n" {
		fail("stderr = %q", stderr.String())
	}

	emptyEnvScript := filepath.Join(root, "empty-env-tracker")
	must(os.WriteFile(emptyEnvScript, []byte("#!/bin/sh\nif [ \"${SHOULD_NOT_INHERIT-unset}\" != unset ]; then exit 41; fi\n"), 0o755))
	isolatedEmptyEnv := make([]string, 0, 4)
	command = (trackerexec.Factory{}).Command(
		context.Background(),
		trackerresolve.Resolution{Tracker: trackerresolve.BD, Binary: emptyEnvScript, WorkDir: workDir, ChildEnv: isolatedEmptyEnv},
		nil,
		trackerexec.Streams{},
	)
	if err := command.Run(); err != nil {
		fail("non-nil empty ChildEnv inherited parent environment: %T %v", err, err)
	}

	marker := filepath.Join(root, "should-not-run")
	cancelScript := filepath.Join(root, "canceled-tracker")
	must(os.WriteFile(cancelScript, []byte("#!/bin/sh\nprintf ran > \"$1\"\n"), 0o755))
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	command = (trackerexec.Factory{}).Command(
		canceled,
		trackerresolve.Resolution{Tracker: trackerresolve.BD, Binary: cancelScript, WorkDir: workDir, ChildEnv: []string{}},
		[]string{marker},
		trackerexec.Streams{},
	)
	if err := command.Run(); !errors.Is(err, context.Canceled) {
		fail("canceled command error = %T %v, want context.Canceled", err, err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		fail("canceled caller context launched child: stat error %v", err)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func fail(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
`
	if err := os.WriteFile(filepath.Join(helperDir, "main.go"), []byte(helper), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "run", helperDir)
	command.Dir = moduleRoot
	command.Env = append(os.Environ(), "SHOULD_NOT_INHERIT=parent")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("trackerexec behavior contract failed: %v\n%s", err, output)
	}
}
