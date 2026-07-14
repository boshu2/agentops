package tracker_br

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boshu2/agentops/cli/internal/trackerexec"
	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

type trackerBRCommandContext interface {
	CommandContext(context.Context, []string, trackerexec.Streams) *trackerexec.ResolvedCommand
}

func TestTrackerBRDelegatesProcessConstructionToTrackerexec(t *testing.T) {
	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	if err := os.Mkdir(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	workDir, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "br")
	contents := "#!/bin/sh\n" +
		"IFS= read -r input\n" +
		"printf 'cwd=%s\\nmarker=%s\\nargc=%s\\narg1=%s\\narg2=%s\\nstdin=%s\\n' \"$PWD\" \"$MARKER\" \"$#\" \"$1\" \"$2\" \"$input\"\n" +
		"printf 'stderr=%s\\n' \"$ERR_MARKER\" >&2\n" +
		"exit 23\n"
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}

	resolution := trackerresolve.Resolution{
		Tracker:  trackerresolve.BR,
		Binary:   script,
		WorkDir:  workDir,
		ChildEnv: []string{"MARKER=canonical", "ERR_MARKER=canonical-error"},
	}
	adapter, err := New(resolution)
	if err != nil {
		t.Fatal(err)
	}
	commandContext, ok := any(adapter).(trackerBRCommandContext)
	if !ok {
		t.Fatal("Adapter.CommandContext does not expose trackerexec's argument and stream contract")
	}
	var stdout, stderr bytes.Buffer
	command := commandContext.CommandContext(
		context.Background(),
		[]string{"ready", "--json"},
		trackerexec.Streams{Stdin: strings.NewReader("input-value\n"), Stdout: &stdout, Stderr: &stderr},
	)
	resolution.ChildEnv[0] = "MARKER=mutated-after-construction"
	err = command.Run()
	wantOutput := "cwd=" + workDir + "\nmarker=canonical\nargc=2\narg1=ready\narg2=--json\nstdin=input-value\n"
	if stdout.String() != wantOutput {
		t.Fatalf("tracker_br command stdout = %q, want %q", stdout.String(), wantOutput)
	}
	if stderr.String() != "stderr=canonical-error\n" {
		t.Fatalf("tracker_br command stderr = %q, want %q", stderr.String(), "stderr=canonical-error\n")
	}
	var exit *trackerexec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 {
		t.Fatalf("tracker_br command exit mapping = %T %v, want *trackerexec.ExitError(23)", err, err)
	}

	marker := filepath.Join(root, "canceled-command-ran")
	cancelScript := filepath.Join(root, "cancel-br")
	if err := os.WriteFile(cancelScript, []byte("#!/bin/sh\nprintf ran > \"$1\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cancelAdapter, err := New(trackerresolve.Resolution{
		Tracker:  trackerresolve.BR,
		Binary:   cancelScript,
		WorkDir:  workDir,
		ChildEnv: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	cancelCommandContext, ok := any(cancelAdapter).(trackerBRCommandContext)
	if !ok {
		t.Fatal("Adapter.CommandContext does not expose trackerexec's argument and stream contract")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := cancelCommandContext.CommandContext(canceled, []string{marker}, trackerexec.Streams{}).Run(); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled tracker_br command error = %T %v, want context.Canceled", err, err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("canceled tracker_br command launched child: stat error %v", err)
	}
}
func TestTrackerAdapterRejectsWrongBackend(t *testing.T) {
	_, err := New(trackerresolve.Resolution{Tracker: trackerresolve.BD, Binary: "bd"})
	if err == nil {
		t.Fatal("New() accepted bd resolution")
	}
}
