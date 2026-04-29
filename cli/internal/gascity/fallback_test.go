package gascity

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestFallbackDisabledIsExplicit(t *testing.T) {
	fallback := NewFallback(FallbackConfig{})
	_, err := fallback.ListCityEvents(context.Background(), EventListParams{})
	if !errors.Is(err, ErrFallbackDisabled) {
		t.Fatalf("ListCityEvents err = %v, want ErrFallbackDisabled", err)
	}
}

func TestFallbackCLIAdapter(t *testing.T) {
	runner := &recordingRunner{
		outputs: map[string]string{
			"gc --city /tmp/agentops events --type session.completed --after 7":                                           "{\"seq\":8,\"type\":\"session.completed\",\"subject\":\"sess_123\"}\n",
			"gc events --type session.woke --after-cursor alpha:4":                                                        "{\"city\":\"agentops\",\"seq\":5,\"type\":\"session.woke\"}\n",
			"gc --city /tmp/agentops session list --json":                                                                 `[{"id":"sess_123","alias":"rpi","state":"active","template":"agentops/codex"}]`,
			"gc --city /tmp/agentops event emit agentops.test --actor codex --subject sess_123 --message ok --payload {}": "",
		},
	}
	fallback := NewFallback(FallbackConfig{
		Enabled:  true,
		Command:  "gc",
		CityPath: "/tmp/agentops",
		Runner:   runner,
	})

	cityEvents, err := fallback.ListCityEvents(context.Background(), EventListParams{
		Type:  "session.completed",
		Index: "7",
	})
	if err != nil {
		t.Fatalf("ListCityEvents: %v", err)
	}
	if cityEvents.Total != 1 || cityEvents.Items[0].Seq != 8 {
		t.Fatalf("city events = %#v", cityEvents)
	}

	tagged, err := fallback.ListEvents(context.Background(), EventListParams{
		Type:   "session.woke",
		Cursor: "alpha:4",
	})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if tagged.Total != 1 || tagged.Items[0].City != "agentops" {
		t.Fatalf("tagged events = %#v", tagged)
	}

	sessions, err := fallback.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if sessions.Total != 1 || sessions.Items[0].ID != "sess_123" {
		t.Fatalf("sessions = %#v", sessions)
	}

	emit, err := fallback.EmitCityEvent(context.Background(), EventEmitRequest{
		Type:    "agentops.test",
		Actor:   "codex",
		Subject: "sess_123",
		Message: "ok",
	})
	if err != nil {
		t.Fatalf("EmitCityEvent: %v", err)
	}
	if emit.Status != "recorded" {
		t.Fatalf("emit = %#v", emit)
	}
}

func TestAdapterFallbackOnlyWhenEnabledAndAPIUnavailable(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(RequestIDHeader, "req-api-down")
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(t, w, ProblemDetails{
			Status: http.StatusServiceUnavailable,
			Detail: "no_providers: supervisor unavailable",
		})
	}))
	defer api.Close()

	client, err := NewClient(Config{Endpoint: api.URL})
	if err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{
		outputs: map[string]string{
			"gc --city /tmp/agentops events --type session.completed": "{\"seq\":9,\"type\":\"session.completed\"}\n",
		},
	}

	disabled := NewAdapter(AdapterConfig{Client: client})
	if _, _, err := disabled.ListCityEvents(context.Background(), "agentops", EventListParams{Type: "session.completed"}); err == nil {
		t.Fatal("disabled fallback returned nil error")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("disabled fallback unexpectedly ran: %#v", runner.calls)
	}

	enabled := NewAdapter(AdapterConfig{
		Client: client,
		Fallback: FallbackConfig{
			Enabled:  true,
			CityPath: "/tmp/agentops",
			Runner:   runner,
		},
	})
	events, _, err := enabled.ListCityEvents(context.Background(), "agentops", EventListParams{Type: "session.completed"})
	if err != nil {
		t.Fatalf("enabled fallback ListCityEvents: %v", err)
	}
	if events.Total != 1 || events.Items[0].Seq != 9 {
		t.Fatalf("fallback events = %#v", events)
	}
	if want := []string{"gc --city /tmp/agentops events --type session.completed"}; !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("runner calls = %#v, want %#v", runner.calls, want)
	}
}

func TestAdapterDoesNotFallbackForClientContractErrors(t *testing.T) {
	runner := &recordingRunner{}
	client, err := NewClient(Config{Endpoint: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewAdapter(AdapterConfig{
		Client: client,
		Fallback: FallbackConfig{
			Enabled: true,
			Runner:  runner,
		},
	})
	if _, _, err := adapter.ListCityEvents(context.Background(), "", EventListParams{}); err == nil {
		t.Fatal("blank city returned nil error")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("fallback ran for client validation error: %#v", runner.calls)
	}
}

type recordingRunner struct {
	outputs map[string]string
	calls   []string
}

func (r *recordingRunner) RunCommand(_ context.Context, name string, args ...string) ([]byte, error) {
	call := strings.Join(append([]string{name}, args...), " ")
	r.calls = append(r.calls, call)
	output, ok := r.outputs[call]
	if !ok {
		return nil, errors.New("unexpected command: " + call)
	}
	return []byte(output), nil
}
