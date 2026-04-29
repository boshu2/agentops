package gascity

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClientEndpointValidation(t *testing.T) {
	if _, err := NewClient(Config{}); err == nil {
		t.Fatal("empty endpoint accepted")
	}
	if _, err := NewClient(Config{Endpoint: "localhost:1234"}); err == nil {
		t.Fatal("endpoint without scheme/host accepted")
	}
	client, err := NewClient(Config{Endpoint: "http://127.0.0.1:1234/"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := client.Endpoint(), "http://127.0.0.1:1234"; got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}

func TestClientReadinessProbes(t *testing.T) {
	requests := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests[r.URL.Path]++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			writeJSON(t, w, HealthResponse{OK: true, Status: "ok"})
		case "/v0/readiness":
			writeJSON(t, w, ReadinessResponse{Ready: true, Status: "ready"})
		case "/v0/provider-readiness":
			writeJSON(t, w, ReadinessResponse{Ready: false, Status: "degraded", Degraded: []string{"claude"}})
		case "/v0/city/agentops/readiness":
			writeJSON(t, w, ReadinessResponse{Ready: true, Status: "ready", Providers: []string{"codex"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	health, err := client.Health(ctx)
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if !health.OK || health.Status != "ok" {
		t.Fatalf("unexpected health: %#v", health)
	}
	ready, err := client.Readiness(ctx)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if !ready.Ready {
		t.Fatalf("readiness not ready: %#v", ready)
	}
	provider, err := client.ProviderReadiness(ctx)
	if err != nil {
		t.Fatalf("provider readiness: %v", err)
	}
	if provider.Ready || provider.Status != "degraded" || len(provider.Degraded) != 1 {
		t.Fatalf("provider degraded state not preserved: %#v", provider)
	}
	city, err := client.CityReadiness(ctx, "agentops")
	if err != nil {
		t.Fatalf("city readiness: %v", err)
	}
	if !city.Ready || len(city.Providers) != 1 {
		t.Fatalf("city readiness not preserved: %#v", city)
	}
	for _, path := range []string{"/health", "/v0/readiness", "/v0/provider-readiness", "/v0/city/agentops/readiness"} {
		if requests[path] != 1 {
			t.Fatalf("path %s called %d times", path, requests[path])
		}
	}
}

func TestClientReadinessFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"status":503}`, http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := NewClient(Config{Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Readiness(context.Background()); err == nil {
		t.Fatal("readiness status failure returned nil error")
	}
	if _, err := client.CityReadiness(context.Background(), " "); err == nil {
		t.Fatal("blank city accepted")
	}
}

func TestClientMutationHeadersAndRequestID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got, want := r.Header.Get(MutationHeader), "agentops"; got != want {
			t.Errorf("%s = %q, want %q", MutationHeader, got, want)
		}
		if got, want := r.Header.Get("Content-Type"), "application/json"; got != want {
			t.Errorf("Content-Type = %q, want %q", got, want)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		if got, want := body["name"], "agentops"; got != want {
			t.Errorf("body name = %q, want %q", got, want)
		}
		w.Header().Set(RequestIDHeader, "req-123")
		writeJSON(t, w, CityResponse{OK: true, Name: "agentops", Path: "/tmp/agentops"})
	}))
	defer server.Close()

	client, err := NewClient(Config{Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	var out CityResponse
	meta, err := client.doJSON(
		context.Background(),
		http.MethodPost,
		"/v0/city",
		map[string]string{"name": "agentops"},
		&out,
	)
	if err != nil {
		t.Fatalf("post city: %v", err)
	}
	if meta.RequestID != "req-123" {
		t.Fatalf("request ID = %q, want req-123", meta.RequestID)
	}
	if !out.OK || out.Name != "agentops" {
		t.Fatalf("city response not decoded: %#v", out)
	}
}

func TestClientProblemDetailsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(MutationHeader) == "" {
			t.Errorf("%s missing", MutationHeader)
		}
		w.Header().Set(RequestIDHeader, "req-denied")
		w.WriteHeader(http.StatusForbidden)
		writeJSON(t, w, ProblemDetails{
			Type:     "about:blank",
			Title:    "Forbidden",
			Status:   http.StatusForbidden,
			Detail:   "csrf: X-GC-Request header required on mutation endpoints",
			Instance: "/v0/city/agentops/sessions",
		})
	}))
	defer server.Close()

	client, err := NewClient(Config{Endpoint: server.URL, MutationToken: "test-agentops"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.doJSON(
		context.Background(),
		http.MethodPost,
		"/v0/city/agentops/sessions",
		SessionSubmitRequest{Message: "hello"},
		nil,
	)
	if err == nil {
		t.Fatal("problem details response returned nil error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error %T is not APIError: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusForbidden || apiErr.RequestID != "req-denied" {
		t.Fatalf("APIError metadata = %#v", apiErr)
	}
	if apiErr.Problem == nil || apiErr.Problem.Detail == "" {
		t.Fatalf("problem details not decoded: %#v", apiErr.Problem)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatal(err)
	}
}
