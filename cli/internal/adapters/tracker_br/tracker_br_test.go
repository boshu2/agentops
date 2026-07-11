package tracker_br

import (
	"context"
	"reflect"
	"testing"

	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

func TestTrackerCommandUsesResolvedWorktreeAndChildEnv(t *testing.T) {
	resolution := trackerresolve.Resolution{
		Tracker:  trackerresolve.BR,
		Binary:   "/fake/br",
		WorkDir:  "/repo/lane",
		ChildEnv: []string{"HOME=/home/test", "BEADS_DIR=/repo/_beads"},
	}
	adapter, err := New(resolution)
	if err != nil {
		t.Fatal(err)
	}
	command := adapter.CommandContext(context.Background(), "ready", "--json")
	if command.Path != "/fake/br" || command.Dir != "/repo/lane" {
		t.Fatalf("command path/dir = %q/%q", command.Path, command.Dir)
	}
	if !reflect.DeepEqual(command.Args[1:], []string{"ready", "--json"}) {
		t.Fatalf("command args = %v", command.Args)
	}
	if !reflect.DeepEqual(command.Env, resolution.ChildEnv) {
		t.Fatalf("command env = %v, want %v", command.Env, resolution.ChildEnv)
	}
}

func TestTrackerAdapterRejectsWrongBackend(t *testing.T) {
	_, err := New(trackerresolve.Resolution{Tracker: trackerresolve.BD, Binary: "bd"})
	if err == nil {
		t.Fatal("New() accepted bd resolution")
	}
}
