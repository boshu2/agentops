package close

import (
	"bytes"
	"context"
	"errors"
	"testing"

	closeapp "github.com/boshu2/agentops/cli/internal/close"
)

type fakeUseCases struct {
	request closeapp.Request
	result  closeapp.Result
	err     error
}

func (useCases *fakeUseCases) Execute(_ context.Context, request closeapp.Request) (closeapp.Result, error) {
	useCases.request = request
	return useCases.result, useCases.err
}

func TestModuleCommandParsesDelegatesAndRenders(t *testing.T) {
	useCases := &fakeUseCases{result: closeapp.Result{ID: "age-1", Ref: "123456789", AlreadyClosed: true}}
	command := NewModule(useCases).Command()
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"age-1", "finish", "proof.md", "docs/result.md"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if useCases.request.Mode != closeapp.ModeEnsure || len(useCases.request.Paths) != 1 {
		t.Fatalf("request = %+v", useCases.request)
	}
	if got, want := stdout.String(), "already closed age-1 @ 1234567\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestModuleFailureRendersOnceAndPreservesExitClass(t *testing.T) {
	useCases := &fakeUseCases{err: &closeapp.Failure{Code: closeapp.ExitPersistence, Message: "public persistence failed"}}
	command := NewModule(useCases).Command()
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"age-1", "finish", "proof.md"})
	err := command.Execute()
	var exit interface{ ExitCode() int }
	if !errors.As(err, &exit) || exit.ExitCode() != closeapp.ExitPersistence {
		t.Fatalf("error = %#v", err)
	}
	if got, want := stderr.String(), "public persistence failed\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestModuleContractPublishesStableCloseExits(t *testing.T) {
	contract := (Module{}).Contract()
	for _, code := range []int{0, 1, closeapp.ExitRefused, closeapp.ExitPersistence, closeapp.ExitTracker} {
		if contract.ExitClasses[code] == "" {
			t.Fatalf("missing exit class %d", code)
		}
	}
}
