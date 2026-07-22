package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNativeObservePRClassifiesExactAbsentConflictingAndAmbiguous(t *testing.T) {
	intent := nativePRIntentForTest()
	exact := nativePRRecord{
		NodeID: "PR_exact", Number: 7, URL: "https://example.invalid/pull/7", State: "OPEN",
		HeadRefName: intent.Branch, BaseRefName: intent.BaseRef, HeadRefOID: intent.ExpectedHead,
		BaseRefOID: intent.BaseOID, Body: nativePRBody(intent),
	}
	for name, scenario := range map[string]struct {
		records []nativePRRecord
		state   string
	}{
		"absent":       {state: "absent"},
		"exact":        {records: []nativePRRecord{exact}, state: "open"},
		"closed":       {records: []nativePRRecord{withNativePRState(exact, "CLOSED")}, state: "closed"},
		"wrong_marker": {records: []nativePRRecord{withNativePRBody(exact, "ordinary PR")}, state: "conflicting"},
		"wrong_base":   {records: []nativePRRecord{withNativePRBase(exact, "release")}, state: "conflicting"},
		"two_exact":    {records: []nativePRRecord{exact, withNativePRNumber(exact, 8)}, state: "ambiguous"},
		"mixed":        {records: []nativePRRecord{exact, withNativePRBody(withNativePRNumber(exact, 8), "ordinary PR")}, state: "ambiguous"},
	} {
		t.Run(name, func(t *testing.T) {
			provider := &NativeProviders{}
			provider.ghRun = listOnlyGH(t, scenario.records)
			observation, err := provider.ObservePR(context.Background(), intent)
			if err != nil || observation.State != scenario.state {
				t.Fatalf("ObservePR = %#v, %v; want state %q", observation, err, scenario.state)
			}
			if scenario.state == "open" && (!matchesPRObservation(observation, intent) || observation.PR.NodeID != "PR_exact" || observation.PR.Number != "7") {
				t.Fatalf("exact observation = %#v", observation)
			}
		})
	}
}

func TestNativeObservePRRejectsIncompleteUnknownAndTruncatedResponses(t *testing.T) {
	intent := nativePRIntentForTest()
	exact := nativePRRecord{NodeID: "PR_exact", Number: 7, URL: "https://example.invalid/pull/7", State: "OPEN", HeadRefName: intent.Branch, BaseRefName: intent.BaseRef, HeadRefOID: intent.ExpectedHead, BaseRefOID: intent.BaseOID, Body: nativePRBody(intent)}
	for name, records := range map[string][]nativePRRecord{
		"incomplete": {{State: "OPEN", HeadRefName: intent.Branch, BaseRefName: intent.BaseRef, Body: nativePRBody(intent)}},
		"unknown":    {withNativePRState(exact, "UNKNOWN")},
		"truncated":  repeatNativePR(exact, nativePRListLimit),
	} {
		t.Run(name, func(t *testing.T) {
			provider := &NativeProviders{ghRun: listOnlyGH(t, records)}
			observation, err := provider.ObservePR(context.Background(), intent)
			if name == "truncated" {
				if err != nil || observation.State != "ambiguous" {
					t.Fatalf("truncated observation = %#v, %v", observation, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("%s response was accepted: %#v", name, observation)
			}
		})
	}
}

func TestNativeCreatePRIsCreateOnlyAndColdObservationAdoptsExactActual(t *testing.T) {
	fixture := newNativeEpochFixture(t, "target.txt")
	intent := nativePRIntentForTest()
	intent.BaseOID = fixture.base
	intent.ExpectedHead = fixture.candidate
	fixture.provider.context.Repository = intent.Repository

	var records []nativePRRecord
	createCount := 0
	fixture.provider.ghRun = func(_ context.Context, args ...string) ([]byte, error) {
		if reflect.DeepEqual(args[:2], []string{"pr", "list"}) {
			return json.Marshal(records)
		}
		if !reflect.DeepEqual(args[:2], []string{"pr", "create"}) {
			return nil, errors.New("unexpected gh command")
		}
		createCount++
		if got := flagValue(args, "--repo"); got != intent.Repository {
			t.Fatalf("create repo = %q", got)
		}
		if got := flagValue(args, "--base"); got != intent.BaseRef {
			t.Fatalf("create base = %q", got)
		}
		if got := flagValue(args, "--head"); got != intent.Branch {
			t.Fatalf("create head = %q", got)
		}
		if got := flagValue(args, "--body"); got != nativePRBody(intent) {
			t.Fatalf("create body = %q", got)
		}
		records = []nativePRRecord{{
			NodeID: "PR_created", Number: 21, URL: "https://example.invalid/pull/21", State: "OPEN",
			HeadRefName: intent.Branch, BaseRefName: intent.BaseRef, HeadRefOID: intent.ExpectedHead,
			BaseRefOID: intent.BaseOID, Body: nativePRBody(intent),
		}}
		return []byte("https://example.invalid/pull/21\n"), nil
	}

	created, err := fixture.provider.CreatePR(context.Background(), intent)
	if err != nil || !matchesPRObservation(created, intent) || created.PR.NodeID != "PR_created" {
		t.Fatalf("CreatePR = %#v, %v", created, err)
	}
	if createCount != 1 {
		t.Fatalf("create effects = %d, want 1", createCount)
	}
	cold, err := fixture.provider.CreatePR(context.Background(), intent)
	if err != nil || !matchesPRObservation(cold, intent) || createCount != 1 {
		t.Fatalf("cold exact adoption = %#v, %v; creates=%d", cold, err, createCount)
	}
}

func TestNativeCreatePRRefusesMovedBaseKnownActualAndConflictingState(t *testing.T) {
	fixture := newNativeEpochFixture(t, "target.txt")
	intent := nativePRIntentForTest()
	intent.ExpectedHead = fixture.candidate
	fixture.provider.context.Repository = intent.Repository
	for name, configure := range map[string]func(*PRIntent, *NativeProviders){
		"moved_base": func(intent *PRIntent, provider *NativeProviders) {
			intent.BaseOID = fixture.parent
			provider.ghRun = func(context.Context, ...string) ([]byte, error) {
				t.Fatal("gh called after moved base")
				return nil, nil
			}
		},
		"known_actual": func(intent *PRIntent, provider *NativeProviders) {
			intent.BaseOID, intent.NodeID, intent.Number, intent.URL = fixture.base, "PR_known", "3", "https://example.invalid/pull/3"
			provider.ghRun = func(context.Context, ...string) ([]byte, error) {
				t.Fatal("gh called for known actual")
				return nil, nil
			}
		},
		"conflicting": func(intent *PRIntent, provider *NativeProviders) {
			intent.BaseOID = fixture.base
			record := nativePRRecord{NodeID: "PR_other", Number: 4, URL: "https://example.invalid/pull/4", State: "OPEN", HeadRefName: intent.Branch, BaseRefName: intent.BaseRef, HeadRefOID: intent.ExpectedHead, BaseRefOID: intent.BaseOID, Body: "ordinary PR"}
			provider.ghRun = listOnlyGH(t, []nativePRRecord{record})
		},
	} {
		t.Run(name, func(t *testing.T) {
			trial := intent
			provider := *fixture.provider
			configure(&trial, &provider)
			if observation, err := provider.CreatePR(context.Background(), trial); err == nil {
				t.Fatalf("%s create advanced as %#v", name, observation)
			}
		})
	}
}

func nativePRIntentForTest() PRIntent {
	return PRIntent{
		SchemaVersion: "pr-intent.v1", HandoffID: strings.Repeat("h", 64), Epoch: 1,
		Repository: "boshu2/agentops", BaseRef: "main", BaseOID: strings.Repeat("a", 40),
		Branch: "gc/delivery/handoff", ExpectedHead: strings.Repeat("b", 40),
		PRID: "pr-stable", EffectID: strings.Repeat("c", 64),
	}
}

func listOnlyGH(t *testing.T, records []nativePRRecord) func(context.Context, ...string) ([]byte, error) {
	t.Helper()
	return func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) < 2 || args[0] != "pr" || args[1] != "list" {
			t.Fatalf("unexpected gh argv: %#v", args)
		}
		return json.Marshal(records)
	}
}

func withNativePRState(value nativePRRecord, state string) nativePRRecord {
	value.State = state
	return value
}
func withNativePRBody(value nativePRRecord, body string) nativePRRecord {
	value.Body = body
	return value
}
func withNativePRBase(value nativePRRecord, base string) nativePRRecord {
	value.BaseRefName = base
	return value
}
func withNativePRNumber(value nativePRRecord, number int) nativePRRecord {
	value.Number = number
	value.NodeID += "_other"
	value.URL += "-other"
	return value
}
func repeatNativePR(value nativePRRecord, count int) []nativePRRecord {
	records := make([]nativePRRecord, count)
	for i := range records {
		records[i] = withNativePRNumber(value, i+1)
	}
	return records
}

func flagValue(args []string, flag string) string {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == flag {
			return args[index+1]
		}
	}
	return ""
}
