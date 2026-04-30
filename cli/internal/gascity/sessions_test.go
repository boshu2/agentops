package gascity

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCityAndSessionMethods(t *testing.T) {
	seen := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.Method+" "+r.URL.Path]++
		w.Header().Set(RequestIDHeader, "req-"+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v0/city":
			assertMutationHeader(t, r)
			var req CityCreateRequest
			decodeRequest(t, r, &req)
			if req.Dir != "/tmp/agentops-city" || req.Provider != "codex" {
				t.Fatalf("create city request = %#v", req)
			}
			writeJSON(t, w, CityResponse{OK: true, Name: "agentops", Path: req.Dir})
		case r.Method == http.MethodGet && r.URL.Path == "/v0/cities":
			writeJSON(t, w, CityListResponse{
				Items: []CityInfo{{Name: "agentops", Path: "/tmp/agentops-city", Running: true}},
				Total: 1,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v0/city/agentops":
			writeJSON(t, w, CityGetResponse{
				Name:            "agentops",
				Path:            "/tmp/agentops-city",
				AgentCount:      2,
				RigCount:        1,
				Provider:        "codex",
				SessionTemplate: "agentops/codex",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v0/city/agentops/sessions":
			assertMutationHeader(t, r)
			var req SessionCreateRequest
			decodeRequest(t, r, &req)
			if req.Kind != "agent" || req.Name != "agentops/codex" || req.Alias != "rpi" {
				t.Fatalf("create session request = %#v", req)
			}
			writeJSON(t, w, testSession())
		case r.Method == http.MethodGet && r.URL.Path == "/v0/city/agentops/sessions":
			if got, want := r.URL.Query().Get("state"), "active"; got != want {
				t.Fatalf("state query = %q, want %q", got, want)
			}
			if got, want := r.URL.Query().Get("template"), "agentops/codex"; got != want {
				t.Fatalf("template query = %q, want %q", got, want)
			}
			if got, want := r.URL.Query().Get("cursor"), "cursor-1"; got != want {
				t.Fatalf("cursor query = %q, want %q", got, want)
			}
			if got, want := r.URL.Query().Get("limit"), "2"; got != want {
				t.Fatalf("limit query = %q, want %q", got, want)
			}
			if got, want := r.URL.Query().Get("peek"), "true"; got != want {
				t.Fatalf("peek query = %q, want %q", got, want)
			}
			writeJSON(t, w, SessionListResponse{
				Items:      []Session{testSession()},
				Total:      1,
				NextCursor: "cursor-2",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v0/city/agentops/session/sess_123":
			if got, want := r.URL.Query().Get("peek"), "true"; got != want {
				t.Fatalf("peek query = %q, want %q", got, want)
			}
			writeJSON(t, w, testSession())
		case r.Method == http.MethodPost && r.URL.Path == "/v0/city/agentops/session/sess_123/submit":
			assertMutationHeader(t, r)
			var req SessionSubmitRequest
			decodeRequest(t, r, &req)
			if req.Message != "continue" || req.Intent != "follow_up" {
				t.Fatalf("submit request = %#v", req)
			}
			writeJSON(t, w, SessionSubmitResponse{
				Status: "accepted",
				ID:     "sess_123",
				Queued: true,
				Intent: "follow_up",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/v0/city/agentops/session/sess_123/close":
			assertMutationHeader(t, r)
			writeJSON(t, w, map[string]any{"ok": true})
		case r.Method == http.MethodGet && r.URL.Path == "/v0/city/agentops/session/sess_123/transcript":
			if got, want := r.URL.Query().Get("format"), "conversation"; got != want {
				t.Fatalf("format query = %q, want %q", got, want)
			}
			if got, want := r.URL.Query().Get("tail"), "0"; got != want {
				t.Fatalf("tail query = %q, want %q", got, want)
			}
			writeJSON(t, w, TranscriptResponse{
				ID:       "sess_123",
				Template: "agentops/codex",
				Provider: "codex",
				Format:   "conversation",
				Turns:    []TranscriptEntry{{Role: "assistant", Text: "done"}},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
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

	cities, _, err := client.ListCities(ctx)
	if err != nil {
		t.Fatalf("ListCities: %v", err)
	}
	if cities.Total != 1 || cities.Items[0].Name != "agentops" {
		t.Fatalf("cities not decoded: %#v", cities)
	}

	cityStatus, _, err := client.GetCity(ctx, "agentops")
	if err != nil {
		t.Fatalf("GetCity: %v", err)
	}
	if cityStatus.SessionTemplate != "agentops/codex" {
		t.Fatalf("city status not decoded: %#v", cityStatus)
	}

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

	got, _, err := client.GetSession(ctx, "agentops", "sess_123", SessionGetOptions{Peek: true})
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Provider != "codex" {
		t.Fatalf("session get not decoded: %#v", got)
	}

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
	if _, err := client.CloseSession(ctx, "agentops", "sess_123"); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

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

	for _, key := range []string{
		"POST /v0/city",
		"GET /v0/cities",
		"GET /v0/city/agentops",
		"POST /v0/city/agentops/sessions",
		"GET /v0/city/agentops/sessions",
		"GET /v0/city/agentops/session/sess_123",
		"POST /v0/city/agentops/session/sess_123/submit",
		"POST /v0/city/agentops/session/sess_123/close",
		"GET /v0/city/agentops/session/sess_123/transcript",
	} {
		if seen[key] != 1 {
			t.Fatalf("%s seen %d times", key, seen[key])
		}
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

func decodeRequest(t *testing.T, r *http.Request, v any) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		t.Fatalf("decode request: %v", err)
	}
}
