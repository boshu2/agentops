package gascity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCityMethods(t *testing.T) {
	client, fixture, cleanup := newGasCitySessionFixture(t)
	defer cleanup()
	ctx := context.Background()

	assertCreateCity(t, ctx, client)
	assertListCities(t, ctx, client)
	assertGetCity(t, ctx, client)
	fixture.assertSeen("POST /v0/city", "GET /v0/cities", "GET /v0/city/agentops")
}

func TestClientSessionLifecycleMethods(t *testing.T) {
	client, fixture, cleanup := newGasCitySessionFixture(t)
	defer cleanup()
	ctx := context.Background()

	assertCreateSession(t, ctx, client)
	assertListSessions(t, ctx, client)
	assertGetSession(t, ctx, client)
	assertSubmitSession(t, ctx, client)
	assertCloseSession(t, ctx, client)
	fixture.assertSeen(
		"POST /v0/city/agentops/sessions",
		"GET /v0/city/agentops/sessions",
		"GET /v0/city/agentops/session/sess_123",
		"POST /v0/city/agentops/session/sess_123/submit",
		"POST /v0/city/agentops/session/sess_123/close",
	)
}

func TestClientSessionTranscriptMethod(t *testing.T) {
	client, fixture, cleanup := newGasCitySessionFixture(t)
	defer cleanup()
	ctx := context.Background()

	assertSessionTranscript(t, ctx, client)
	fixture.assertSeen("GET /v0/city/agentops/session/sess_123/transcript")
}

type gasCitySessionFixture struct {
	t      *testing.T
	seen   map[string]int
	routes map[string]http.HandlerFunc
}

func newGasCitySessionFixture(t *testing.T) (*Client, *gasCitySessionFixture, func()) {
	t.Helper()
	fixture := &gasCitySessionFixture{
		t:    t,
		seen: make(map[string]int),
	}
	fixture.routes = map[string]http.HandlerFunc{
		"POST /v0/city":                                     fixture.handleCreateCity,
		"GET /v0/cities":                                    fixture.handleListCities,
		"GET /v0/city/agentops":                             fixture.handleGetCity,
		"POST /v0/city/agentops/sessions":                   fixture.handleCreateSession,
		"GET /v0/city/agentops/sessions":                    fixture.handleListSessions,
		"GET /v0/city/agentops/session/sess_123":            fixture.handleGetSession,
		"POST /v0/city/agentops/session/sess_123/submit":    fixture.handleSubmitSession,
		"POST /v0/city/agentops/session/sess_123/close":     fixture.handleCloseSession,
		"GET /v0/city/agentops/session/sess_123/transcript": fixture.handleSessionTranscript,
	}

	server := httptest.NewServer(fixture)
	client, err := NewClient(Config{Endpoint: server.URL})
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	return client, fixture, server.Close
}

func (f *gasCitySessionFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Method + " " + r.URL.Path
	f.seen[key]++
	w.Header().Set(RequestIDHeader, "req-"+r.URL.Path)
	w.Header().Set("Content-Type", "application/json")

	handler, ok := f.routes[key]
	if !ok {
		f.t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
	}
	handler(w, r)
}

func (f *gasCitySessionFixture) assertSeen(keys ...string) {
	f.t.Helper()
	for _, key := range keys {
		if f.seen[key] != 1 {
			f.t.Fatalf("%s seen %d times", key, f.seen[key])
		}
	}
}

func (f *gasCitySessionFixture) handleCreateCity(w http.ResponseWriter, r *http.Request) {
	assertMutationHeader(f.t, r)
	var req CityCreateRequest
	decodeRequest(f.t, r, &req)
	if req.Dir != "/tmp/agentops-city" || req.Provider != "codex" {
		f.t.Fatalf("create city request = %#v", req)
	}
	writeJSON(f.t, w, CityResponse{OK: true, Name: "agentops", Path: req.Dir})
}

func (f *gasCitySessionFixture) handleListCities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(f.t, w, CityListResponse{
		Items: []CityInfo{{Name: "agentops", Path: "/tmp/agentops-city", Running: true}},
		Total: 1,
	})
}

func (f *gasCitySessionFixture) handleGetCity(w http.ResponseWriter, _ *http.Request) {
	writeJSON(f.t, w, CityGetResponse{
		Name:            "agentops",
		Path:            "/tmp/agentops-city",
		AgentCount:      2,
		RigCount:        1,
		Provider:        "codex",
		SessionTemplate: "agentops/codex",
	})
}

func (f *gasCitySessionFixture) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	assertMutationHeader(f.t, r)
	var req SessionCreateRequest
	decodeRequest(f.t, r, &req)
	if req.Kind != "agent" || req.Name != "agentops/codex" || req.Alias != "rpi" {
		f.t.Fatalf("create session request = %#v", req)
	}
	writeJSON(f.t, w, testSession())
}

func (f *gasCitySessionFixture) handleListSessions(w http.ResponseWriter, r *http.Request) {
	assertQueryParam(f.t, r, "state", "active")
	assertQueryParam(f.t, r, "template", "agentops/codex")
	assertQueryParam(f.t, r, "cursor", "cursor-1")
	assertQueryParam(f.t, r, "limit", "2")
	assertQueryParam(f.t, r, "peek", "true")
	writeJSON(f.t, w, SessionListResponse{
		Items:      []Session{testSession()},
		Total:      1,
		NextCursor: "cursor-2",
	})
}

func (f *gasCitySessionFixture) handleGetSession(w http.ResponseWriter, r *http.Request) {
	assertQueryParam(f.t, r, "peek", "true")
	writeJSON(f.t, w, testSession())
}

func (f *gasCitySessionFixture) handleSubmitSession(w http.ResponseWriter, r *http.Request) {
	assertMutationHeader(f.t, r)
	var req SessionSubmitRequest
	decodeRequest(f.t, r, &req)
	if req.Message != "continue" || req.Intent != "follow_up" {
		f.t.Fatalf("submit request = %#v", req)
	}
	writeJSON(f.t, w, SessionSubmitResponse{
		Status: "accepted",
		ID:     "sess_123",
		Queued: true,
		Intent: "follow_up",
	})
}

func (f *gasCitySessionFixture) handleCloseSession(w http.ResponseWriter, r *http.Request) {
	assertMutationHeader(f.t, r)
	writeJSON(f.t, w, map[string]any{"ok": true})
}

func (f *gasCitySessionFixture) handleSessionTranscript(w http.ResponseWriter, r *http.Request) {
	assertQueryParam(f.t, r, "format", "conversation")
	assertQueryParam(f.t, r, "tail", "0")
	writeJSON(f.t, w, TranscriptResponse{
		ID:       "sess_123",
		Template: "agentops/codex",
		Provider: "codex",
		Format:   "conversation",
		Turns:    []TranscriptEntry{{Role: "assistant", Text: "done"}},
	})
}

func assertCreateCity(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	city, meta, err := client.CreateCity(ctx, CityCreateRequest{
		Dir:      "/tmp/agentops-city",
		Provider: "codex",
	})
	if err != nil {
		t.Fatalf("CreateCity: %v", err)
	}
	if !city.OK || meta.RequestID == "" {
		t.Fatalf("city/meta not decoded: city=%#v meta=%#v", city, meta)
	}
}

func assertListCities(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	cities, _, err := client.ListCities(ctx)
	if err != nil {
		t.Fatalf("ListCities: %v", err)
	}
	if cities.Total != 1 || cities.Items[0].Name != "agentops" {
		t.Fatalf("cities not decoded: %#v", cities)
	}
}

func assertGetCity(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	cityStatus, _, err := client.GetCity(ctx, "agentops")
	if err != nil {
		t.Fatalf("GetCity: %v", err)
	}
	if cityStatus.SessionTemplate != "agentops/codex" {
		t.Fatalf("city status not decoded: %#v", cityStatus)
	}
}

func assertCreateSession(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	session, _, err := client.CreateSession(ctx, "agentops", SessionCreateRequest{
		Kind:  "agent",
		Name:  "agentops/codex",
		Alias: "rpi",
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if session.ID != "sess_123" || !session.Running {
		t.Fatalf("session not decoded: %#v", session)
	}
}

func assertListSessions(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	list, _, err := client.ListSessions(ctx, "agentops", SessionListParams{
		State:    "active",
		Template: "agentops/codex",
		Cursor:   "cursor-1",
		Limit:    2,
		Peek:     true,
	})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if list.Total != 1 || list.NextCursor != "cursor-2" {
		t.Fatalf("session list not decoded: %#v", list)
	}
}

func assertGetSession(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	got, _, err := client.GetSession(ctx, "agentops", "sess_123", SessionGetOptions{Peek: true})
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Provider != "codex" {
		t.Fatalf("session get not decoded: %#v", got)
	}
}

func assertSubmitSession(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	submit, _, err := client.SubmitSession(ctx, "agentops", "sess_123", SessionSubmitRequest{
		Message: "continue",
		Intent:  "follow_up",
	})
	if err != nil {
		t.Fatalf("SubmitSession: %v", err)
	}
	if !submit.Queued || submit.Intent != "follow_up" {
		t.Fatalf("submit not decoded: %#v", submit)
	}
}

func assertCloseSession(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	if _, err := client.CloseSession(ctx, "agentops", "sess_123"); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
}

func assertSessionTranscript(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	tail := 0
	transcript, _, err := client.SessionTranscript(ctx, "agentops", "sess_123", TranscriptOptions{
		Format: "conversation",
		Tail:   &tail,
	})
	if err != nil {
		t.Fatalf("SessionTranscript: %v", err)
	}
	if len(transcript.Turns) != 1 || transcript.Turns[0].Text != "done" {
		t.Fatalf("transcript not decoded: %#v", transcript)
	}
}

func TestClientSessionPathValidation(t *testing.T) {
	client, err := NewClient(Config{Endpoint: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, _, err := client.ListSessions(ctx, " ", SessionListParams{}); err == nil {
		t.Fatal("blank city accepted")
	}
	if _, _, err := client.GetSession(ctx, "agentops", " ", SessionGetOptions{}); err == nil {
		t.Fatal("blank session ID accepted")
	}
	tail := -1
	if _, _, err := client.SessionTranscript(ctx, "agentops", "sess_123", TranscriptOptions{Tail: &tail}); err == nil {
		t.Fatal("negative transcript tail accepted")
	}
}

func testSession() Session {
	return Session{
		ID:          "sess_123",
		Kind:        "agent",
		Template:    "agentops/codex",
		State:       "active",
		Title:       "RPI phase 1",
		Alias:       "rpi",
		Provider:    "codex",
		SessionName: "agentops-codex-sess-123",
		CreatedAt:   "2026-04-28T20:15:00Z",
		Running:     true,
	}
}

func assertMutationHeader(t *testing.T, r *http.Request) {
	t.Helper()
	if got, want := r.Header.Get(MutationHeader), "agentops"; got != want {
		t.Fatalf("%s = %q, want %q", MutationHeader, got, want)
	}
}

func assertQueryParam(t *testing.T, r *http.Request, key, want string) {
	t.Helper()
	if got := r.URL.Query().Get(key); got != want {
		t.Fatalf("%s query = %q, want %q", key, got, want)
	}
}

func decodeRequest(t *testing.T, r *http.Request, v any) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		t.Fatalf("decode request: %v", err)
	}
}
