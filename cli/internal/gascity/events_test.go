package gascity

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientEventListAndEmitMethods(t *testing.T) {
	client, fixture, cleanup := newGasCityEventsFixture(t)
	defer cleanup()
	ctx := context.Background()

	assertListEvents(t, ctx, client)
	assertListCityEvents(t, ctx, client)
	assertEmitCityEvent(t, ctx, client)
	fixture.assertSeen(
		"GET /v0/events",
		"GET /v0/city/agentops/events",
		"POST /v0/city/agentops/events",
	)
}

func TestClientEventStreamMethods(t *testing.T) {
	client, fixture, cleanup := newGasCityEventsFixture(t)
	defer cleanup()
	ctx := context.Background()

	assertStreamEvents(t, ctx, client)
	assertStreamCityEvents(t, ctx, client)
	fixture.assertSeen("GET /v0/events/stream", "GET /v0/city/agentops/events/stream")
}

type gasCityEventsFixture struct {
	t      *testing.T
	seen   map[string]int
	routes map[string]http.HandlerFunc
}

func newGasCityEventsFixture(t *testing.T) (*Client, *gasCityEventsFixture, func()) {
	t.Helper()
	fixture := &gasCityEventsFixture{
		t:    t,
		seen: make(map[string]int),
	}
	fixture.routes = map[string]http.HandlerFunc{
		"GET /v0/events":                      fixture.handleListEvents,
		"GET /v0/city/agentops/events":        fixture.handleListCityEvents,
		"POST /v0/city/agentops/events":       fixture.handleEmitCityEvent,
		"GET /v0/events/stream":               fixture.handleStreamEvents,
		"GET /v0/city/agentops/events/stream": fixture.handleStreamCityEvents,
	}

	server := httptest.NewServer(fixture)
	client, err := NewClient(Config{Endpoint: server.URL})
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	return client, fixture, server.Close
}

func (f *gasCityEventsFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Method + " " + r.URL.Path
	f.seen[key]++
	w.Header().Set(RequestIDHeader, "req-events")

	handler, ok := f.routes[key]
	if !ok {
		f.t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
	}
	handler(w, r)
}

func (f *gasCityEventsFixture) assertSeen(keys ...string) {
	f.t.Helper()
	for _, key := range keys {
		if f.seen[key] != 1 {
			f.t.Fatalf("%s seen %d times", key, f.seen[key])
		}
	}
}

func (f *gasCityEventsFixture) handleListEvents(w http.ResponseWriter, r *http.Request) {
	assertQueryParam(f.t, r, "type", "session.woke")
	writeJSON(f.t, w, TaggedEventListResponse{
		Items: []TaggedWireEvent{{
			City: "agentops",
			WireEvent: WireEvent{
				Seq:     4,
				Type:    "session.woke",
				Subject: "sess_123",
			},
		}},
		Total: 1,
	})
}

func (f *gasCityEventsFixture) handleListCityEvents(w http.ResponseWriter, r *http.Request) {
	assertQueryParam(f.t, r, "cursor", "cursor-1")
	assertQueryParam(f.t, r, "index", "3")
	writeJSON(f.t, w, EventListResponse{
		Items: []WireEvent{{
			Seq:     5,
			Type:    "session.completed",
			Subject: "sess_123",
		}},
		Total:      1,
		NextCursor: "cursor-2",
	})
}

func (f *gasCityEventsFixture) handleEmitCityEvent(w http.ResponseWriter, r *http.Request) {
	assertMutationHeader(f.t, r)
	var req EventEmitRequest
	decodeRequest(f.t, r, &req)
	if req.Type != "agentops.test" || req.Actor != "codex" {
		f.t.Fatalf("emit request = %#v", req)
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(f.t, w, EventEmitResponse{Status: "recorded"})
}

func (f *gasCityEventsFixture) handleStreamEvents(w http.ResponseWriter, r *http.Request) {
	assertHeader(f.t, r, lastEventIDHeader, "alpha:4")
	assertQueryParam(f.t, r, "after_cursor", "alpha:3")
	writeSSE(f.t, w, ""+
		"id: alpha:4\n"+
		"event: heartbeat\n"+
		"data: {\"timestamp\":\"2026-04-28T20:45:10Z\"}\n\n"+
		"id: alpha:5\n"+
		"event: tagged_event\n"+
		"data: {\"city\":\"agentops\",\"seq\":5,\"type\":\"session.completed\",\"subject\":\"sess_123\"}\n\n")
}

func (f *gasCityEventsFixture) handleStreamCityEvents(w http.ResponseWriter, r *http.Request) {
	assertHeader(f.t, r, lastEventIDHeader, "5")
	assertQueryParam(f.t, r, "after_seq", "4")
	writeSSE(f.t, w, ""+
		"id: 6\n"+
		"event: event\n"+
		"data: {\"seq\":6,\"type\":\"session.woke\",\"subject\":\"sess_123\"}\n\n")
}

func assertListEvents(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	supervisorEvents, _, err := client.ListEvents(ctx, EventListParams{
		Type:  "session.woke",
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if supervisorEvents.Total != 1 || supervisorEvents.Items[0].City != "agentops" {
		t.Fatalf("supervisor events not decoded: %#v", supervisorEvents)
	}
}

func assertListCityEvents(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	cityEvents, _, err := client.ListCityEvents(ctx, "agentops", EventListParams{
		Type:   "session.completed",
		Cursor: "cursor-1",
		Index:  "3",
		Wait:   "1s",
		Limit:  1,
	})
	if err != nil {
		t.Fatalf("ListCityEvents: %v", err)
	}
	if cityEvents.NextCursor != "cursor-2" || cityEvents.Items[0].Seq != 5 {
		t.Fatalf("city events not decoded: %#v", cityEvents)
	}
}

func assertEmitCityEvent(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	emit, _, err := client.EmitCityEvent(ctx, "agentops", EventEmitRequest{
		Type:    "agentops.test",
		Actor:   "codex",
		Subject: "sess_123",
		Message: "ok",
	})
	if err != nil {
		t.Fatalf("EmitCityEvent: %v", err)
	}
	if emit.Status != "recorded" {
		t.Fatalf("emit not decoded: %#v", emit)
	}
}

func assertStreamEvents(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	stream, _, err := client.StreamEvents(ctx, EventStreamOptions{
		LastEventID: "alpha:4",
		AfterCursor: "alpha:3",
	})
	if err != nil {
		t.Fatalf("StreamEvents: %v", err)
	}
	defer stream.Close()
	heartbeat, err := stream.Recv()
	if err != nil {
		t.Fatalf("stream heartbeat: %v", err)
	}
	if heartbeat.Heartbeat == nil || heartbeat.Heartbeat.Timestamp == "" {
		t.Fatalf("heartbeat not decoded: %#v", heartbeat)
	}
	event, err := stream.NextEvent()
	if err != nil {
		t.Fatalf("stream event: %v", err)
	}
	if event.TaggedEvent == nil || event.TaggedEvent.City != "agentops" {
		t.Fatalf("tagged event not decoded: %#v", event)
	}
}

func assertStreamCityEvents(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	cityStream, _, err := client.StreamCityEvents(ctx, "agentops", EventStreamOptions{
		LastEventID: "5",
		AfterSeq:    "4",
	})
	if err != nil {
		t.Fatalf("StreamCityEvents: %v", err)
	}
	defer cityStream.Close()
	cityEvent, err := cityStream.NextEvent()
	if err != nil {
		t.Fatalf("city stream event: %v", err)
	}
	if cityEvent.CityEvent == nil || cityEvent.CityEvent.Seq != 6 {
		t.Fatalf("city event not decoded: %#v", cityEvent)
	}
}

func assertHeader(t *testing.T, r *http.Request, key, want string) {
	t.Helper()
	if got := r.Header.Get(key); got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}

func TestSSEDecoderAndJSONLines(t *testing.T) {
	decoder := NewSSEDecoder(strings.NewReader("" +
		": keepalive comment\n" +
		"id: 10\n" +
		"event: heartbeat\n" +
		"retry: 1500\n" +
		"data: {\"timestamp\":\"2026-04-28T20:45:10Z\"}\n\n" +
		"id: 11\n" +
		"event: event\n" +
		"data: {\"seq\":11,\"type\":\"session.woke\"}\n\n"))

	heartbeat, err := decoder.Decode()
	if err != nil {
		t.Fatalf("decode heartbeat: %v", err)
	}
	if heartbeat.ID != "10" || heartbeat.Retry != 1500 || heartbeat.Heartbeat == nil {
		t.Fatalf("heartbeat frame mismatch: %#v", heartbeat)
	}
	event, err := decoder.Decode()
	if err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if event.CityEvent == nil || event.CityEvent.Seq != 11 {
		t.Fatalf("event frame mismatch: %#v", event)
	}
	if _, err := decoder.Decode(); err != io.EOF {
		t.Fatalf("final decode err = %v, want EOF", err)
	}

	cityLines := strings.NewReader("{\"seq\":1,\"type\":\"city.ready\"}\n\n{\"seq\":2,\"type\":\"session.woke\"}\n")
	cityEvents, err := DecodeWireEventJSONLines(cityLines)
	if err != nil {
		t.Fatalf("DecodeWireEventJSONLines: %v", err)
	}
	if len(cityEvents) != 2 || cityEvents[1].Type != "session.woke" {
		t.Fatalf("city JSONL mismatch: %#v", cityEvents)
	}

	taggedLines := strings.NewReader("{\"city\":\"agentops\",\"seq\":3,\"type\":\"session.completed\"}\n")
	taggedEvents, err := DecodeTaggedWireEventJSONLines(taggedLines)
	if err != nil {
		t.Fatalf("DecodeTaggedWireEventJSONLines: %v", err)
	}
	if len(taggedEvents) != 1 || taggedEvents[0].City != "agentops" {
		t.Fatalf("tagged JSONL mismatch: %#v", taggedEvents)
	}
}

func TestReconnectCursorHelpers(t *testing.T) {
	cityFrame := EventStreamFrame{
		ID:        "42",
		CityEvent: &EventStreamEnvelope{Seq: 42, Type: "session.woke"},
	}
	if got, want := CursorFromFrame(cityFrame), "42"; got != want {
		t.Fatalf("city cursor = %q, want %q", got, want)
	}
	fallbackFrame := EventStreamFrame{CityEvent: &EventStreamEnvelope{Seq: 43}}
	if got, want := CursorFromFrame(fallbackFrame), "43"; got != want {
		t.Fatalf("fallback cursor = %q, want %q", got, want)
	}

	cityOpts := ReconnectOptions(EventStreamScopeCity, "43")
	if cityOpts.LastEventID != "43" || cityOpts.AfterSeq != "43" || cityOpts.AfterCursor != "" {
		t.Fatalf("city reconnect options = %#v", cityOpts)
	}
	supervisorOpts := ReconnectOptions(EventStreamScopeSupervisor, "alpha:4,beta:9")
	if supervisorOpts.LastEventID != "alpha:4,beta:9" ||
		supervisorOpts.AfterCursor != "alpha:4,beta:9" ||
		supervisorOpts.AfterSeq != "" {
		t.Fatalf("supervisor reconnect options = %#v", supervisorOpts)
	}
}

func TestTerminalStateClassifier(t *testing.T) {
	tests := []struct {
		name  string
		input TerminalStateInput
		want  TerminalClassification
	}{
		{
			name: "terminal event with transcript completes",
			input: TerminalStateInput{
				EventType:           "session.completed",
				TranscriptRequired:  true,
				TranscriptAvailable: true,
			},
			want: TerminalClassification{
				Status:   TerminalStatusCompleted,
				Terminal: true,
			},
		},
		{
			name: "terminal event without transcript is degraded",
			input: TerminalStateInput{
				EventType:          "session.completed",
				TranscriptRequired: true,
			},
			want: TerminalClassification{
				Status:   TerminalStatusTerminalWithoutTranscript,
				Terminal: true,
				Degraded: true,
				Reason:   "terminal state observed without transcript evidence",
			},
		},
		{
			name: "failure event maps to failed",
			input: TerminalStateInput{
				EventType: "session.failed",
			},
			want: TerminalClassification{
				Status:   TerminalStatusFailed,
				Terminal: true,
			},
		},
		{
			name: "payload status overrides generic event",
			input: TerminalStateInput{
				EventType:    "session.closed",
				EventPayload: map[string]any{"status": "failed"},
			},
			want: TerminalClassification{
				Status:   TerminalStatusFailed,
				Terminal: true,
			},
		},
		{
			name: "cancel event maps to cancelled",
			input: TerminalStateInput{
				EventType: "session.killed",
			},
			want: TerminalClassification{
				Status:   TerminalStatusCancelled,
				Terminal: true,
			},
		},
		{
			name: "session state closed maps to completed",
			input: TerminalStateInput{
				SessionState: "closed",
			},
			want: TerminalClassification{
				Status:   TerminalStatusCompleted,
				Terminal: true,
			},
		},
		{
			name: "missing accepted session is lost",
			input: TerminalStateInput{
				SessionMissing: true,
			},
			want: TerminalClassification{
				Status:   TerminalStatusLost,
				Terminal: true,
				Degraded: true,
				Reason:   "session missing after acceptance",
			},
		},
		{
			name: "provider unreachable is degraded",
			input: TerminalStateInput{
				ProviderUnreachable: true,
			},
			want: TerminalClassification{
				Status:   TerminalStatusProviderUnreachable,
				Degraded: true,
				Reason:   "provider readiness unavailable before terminal state",
			},
		},
		{
			name: "event stream unavailable requires reconciliation",
			input: TerminalStateInput{
				EventStreamUnavailable: true,
			},
			want: TerminalClassification{
				Status:   TerminalStatusEventStreamUnavailable,
				Degraded: true,
				Reason:   "event stream unavailable; REST reconciliation required",
			},
		},
		{
			name: "running by default",
			input: TerminalStateInput{
				SessionState: "active",
			},
			want: TerminalClassification{Status: TerminalStatusRunning},
		},
		{
			name: "terminal evidence beats provider unreachable",
			input: TerminalStateInput{
				EventType:           "session.completed",
				ProviderUnreachable: true,
				TranscriptRequired:  true,
				TranscriptAvailable: true,
			},
			want: TerminalClassification{
				Status:   TerminalStatusCompleted,
				Terminal: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyTerminalState(tc.input)
			if got != tc.want {
				t.Fatalf("classification = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func writeSSE(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "text/event-stream")
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatalf("write SSE: %v", err)
	}
}
