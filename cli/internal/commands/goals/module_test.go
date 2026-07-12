package goals

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	goalapp "github.com/boshu2/agentops/cli/internal/goals"
	"github.com/spf13/cobra"
)

type simpleUseCasesSpy struct {
	called   string
	validate goalapp.ValidateOptions
	history  goalapp.HistoryOptions
	export   goalapp.ExportOptions
	drift    goalapp.DriftOptions
	meta     goalapp.MetaOptions
}

func (spy *simpleUseCasesSpy) Validate(_ context.Context, options goalapp.ValidateOptions) error {
	spy.called, spy.validate = "validate", options
	return nil
}

func (spy *simpleUseCasesSpy) History(_ context.Context, options goalapp.HistoryOptions) error {
	spy.called, spy.history = "history", options
	return nil
}

func (spy *simpleUseCasesSpy) Export(_ context.Context, options goalapp.ExportOptions) error {
	spy.called, spy.export = "export", options
	return nil
}

func (spy *simpleUseCasesSpy) Drift(_ context.Context, options goalapp.DriftOptions) error {
	spy.called, spy.drift = "drift", options
	return nil
}

func (spy *simpleUseCasesSpy) Meta(_ context.Context, options goalapp.MetaOptions) error {
	spy.called, spy.meta = "meta", options
	return nil
}

func TestModuleOwnsExactGoalsCommandTree(t *testing.T) {
	command := NewModule(UseCases{}, HostOptions{}).Command()
	want := []string{
		"goals add", "goals drift", "goals export", "goals history", "goals init",
		"goals measure", "goals meta", "goals migrate", "goals prune", "goals render",
		"goals scenarios", "goals steer", "goals steer add", "goals steer apply",
		"goals steer prioritize", "goals steer recommend", "goals steer remove",
		"goals trace", "goals validate",
	}
	var got []string
	var walk func(*cobra.Command)
	walk = func(node *cobra.Command) {
		for _, child := range node.Commands() {
			got = append(got, child.CommandPath())
			walk(child)
		}
	}
	walk(command)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command paths = %#v, want %#v", got, want)
	}
	if got := command.PersistentFlags().Lookup("timeout").DefValue; got != "240" {
		t.Fatalf("--timeout default = %q, want 240", got)
	}
	if command.PersistentFlags().Lookup("file") == nil {
		t.Fatal("missing persistent --file")
	}
}

func TestSimpleCommandsDelegateResolvedRequests(t *testing.T) {
	spy := &simpleUseCasesSpy{}
	module := NewModule(UseCases{Simple: spy}, HostOptions{
		OutputMode:       func() string { return "json" },
		ResolveGoalsPath: func(path string) string { return "resolved:" + path },
	})

	tests := []struct {
		args   []string
		called string
		check  func(*testing.T, *simpleUseCasesSpy)
	}{
		{[]string{"--file", "custom.md", "--timeout", "17", "validate"}, "validate", func(t *testing.T, spy *simpleUseCasesSpy) {
			if spy.validate.GoalsFile != "resolved:custom.md" || !spy.validate.JSON || spy.validate.Stdout == nil {
				t.Fatalf("validate options = %+v", spy.validate)
			}
		}},
		{[]string{"history", "--goal", "g-1", "--since", "2026-07-01"}, "history", func(t *testing.T, spy *simpleUseCasesSpy) {
			if spy.history.GoalID != "g-1" || spy.history.Since != "2026-07-01" || !spy.history.JSON || spy.history.Stdout == nil {
				t.Fatalf("history options = %+v", spy.history)
			}
		}},
		{[]string{"--timeout", "17", "export"}, "export", func(t *testing.T, spy *simpleUseCasesSpy) {
			if spy.export.GoalsFile != "resolved:" || spy.export.Timeout != 17*time.Second || spy.export.Stdout == nil || spy.export.Stderr == nil {
				t.Fatalf("export options = %+v", spy.export)
			}
		}},
		{[]string{"--timeout", "17", "drift"}, "drift", func(t *testing.T, spy *simpleUseCasesSpy) {
			if spy.drift.Timeout != 17*time.Second || !spy.drift.JSON || spy.drift.Stdout == nil || spy.drift.Stderr == nil {
				t.Fatalf("drift options = %+v", spy.drift)
			}
		}},
		{[]string{"--timeout", "17", "meta"}, "meta", func(t *testing.T, spy *simpleUseCasesSpy) {
			if spy.meta.Timeout != 17*time.Second || !spy.meta.JSON || spy.meta.Stdout == nil {
				t.Fatalf("meta options = %+v", spy.meta)
			}
		}},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			command := module.Command()
			command.SetArgs(test.args)
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if spy.called != test.called {
				t.Fatalf("called %q, want %q", spy.called, test.called)
			}
			test.check(t, spy)
		})
	}
}
