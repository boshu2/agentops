package done

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	doneapp "github.com/boshu2/agentops/cli/internal/done"
)

type fakeService struct {
	request doneapp.Request
	result  doneapp.Result
	err     error
}

func (service *fakeService) Execute(_ context.Context, request doneapp.Request) (doneapp.Result, error) {
	service.request = request
	return service.result, service.err
}

func TestCommandOwnsFlagsAndDelegatesRequest(t *testing.T) {
	service := &fakeService{result: doneapp.Result{BeadID: "age-1", CommitSHA: "abcdef0123456789", Disposition: doneapp.DispositionConfirmed,
		Stamp: "[verdict:abcdef0:CONFIRMED]", Closed: true}}
	command := NewModule(service).Command()
	command.SetArgs([]string{"age-1", "--sha", "abcdef0", "--reason", "Shipped", "--force-no-verdict"})
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	want := doneapp.Request{BeadID: "age-1", SHA: "abcdef0", Reason: "Shipped", ForceNoVerdict: true}
	if service.request != want {
		t.Fatalf("request = %+v, want %+v", service.request, want)
	}
	if !strings.Contains(stdout.String(), "closed age-1 at abcdef0 [verdict:abcdef0:CONFIRMED]") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	for _, flag := range []string{"sha", "reason", "force-no-verdict", "json"} {
		if command.Flags().Lookup(flag) == nil {
			t.Errorf("missing local flag %q", flag)
		}
	}
}

func TestCommandJSONIsStructuredAndTrackerOutputIsNotData(t *testing.T) {
	service := &fakeService{result: doneapp.Result{BeadID: "age-1", CommitSHA: "abcdef0123456789", Disposition: doneapp.DispositionUnverified,
		Stamp: "[verdict:abcdef0:UNVERIFIED]", CloseReason: "Done [verdict:abcdef0:UNVERIFIED]", Closed: true, TrackerOutput: "tracker chatter"}}
	command := NewModule(service).Command()
	command.SetArgs([]string{"age-1", "--json"})
	var stdout bytes.Buffer
	command.SetOut(&stdout)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var result doneapp.Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if result.BeadID != "age-1" || strings.Contains(stdout.String(), "tracker chatter") {
		t.Fatalf("result=%+v stdout=%q", result, stdout.String())
	}
}

func TestCommandRejectsWrongArityWithoutCallingService(t *testing.T) {
	for _, args := range [][]string{nil, {"age-1", "extra"}} {
		service := &fakeService{}
		command := NewModule(service).Command()
		command.SetArgs(args)
		if err := command.Execute(); err == nil {
			t.Fatalf("args %v accepted", args)
		}
		if service.request.BeadID != "" {
			t.Fatalf("args %v called service", args)
		}
	}
}
