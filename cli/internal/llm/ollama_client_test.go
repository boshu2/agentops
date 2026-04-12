package llm

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestServer wraps httptest.NewServer with a handler registry so each test
// can mount /api/tags, /api/show, /api/generate behavior independently.
func newTestServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, h := range handlers {
		mux.HandleFunc(path, h)
	}
	return httptest.NewServer(mux)
}

// tagsResponseOK returns a /api/tags body advertising the named model + digest.
func tagsResponseOK(model, digest string) string {
	body := map[string]any{
		"models": []map[string]any{
			{
				"name":        model,
				"modified_at": "2026-04-11T00:00:00Z",
				"size":        5400000000,
				"digest":      digest,
			},
		},
	}
	b, _ := json.Marshal(body)
	return string(b)
}

func TestOllamaClient_Generate_SendsStreamFalse(t *testing.T) {
	var captured map[string]any
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/api/tags": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, tagsResponseOK("gemma2:9b", "sha256:ff02c3"))
		},
		"/api/generate": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &captured)
			_, _ = io.WriteString(w, `{"response":"### Intent\nok\n### Summary\nfine\n### Entities\n\n### Assistant condensed\ntest"}`)
		},
	})
	defer srv.Close()

	c, err := NewOllamaClient(OllamaOptions{
		Endpoint: srv.URL,
		Model:    "gemma2:9b",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}
	_, err = c.Generate("hello prompt")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	// CRITICAL: stream MUST be false (default ollama behavior is stream:true
	// which would break our single-shot JSON decode).
	stream, ok := captured["stream"]
	if !ok {
		t.Fatalf("generate request body missing 'stream' field; body=%+v", captured)
	}
	if streamBool, _ := stream.(bool); streamBool {
		t.Errorf("stream: want false, got %v", stream)
	}
	if model, _ := captured["model"].(string); model != "gemma2:9b" {
		t.Errorf("model: want gemma2:9b, got %v", captured["model"])
	}
	if format, _ := captured["format"].(string); format != "json" {
		t.Errorf("format: want json, got %v", captured["format"])
	}
	if keepAlive, _ := captured["keep_alive"].(string); keepAlive != "30m" {
		t.Errorf("keep_alive: want 30m, got %v", captured["keep_alive"])
	}
	options, ok := captured["options"].(map[string]any)
	if !ok {
		t.Fatalf("options: got %T", captured["options"])
	}
	if temperature, _ := options["temperature"].(float64); temperature != 0.2 {
		t.Errorf("temperature: want 0.2, got %v", options["temperature"])
	}
	if numPredict, _ := options["num_predict"].(float64); numPredict != 800 {
		t.Errorf("num_predict: want 800, got %v", options["num_predict"])
	}
	if numCtx, _ := options["num_ctx"].(float64); numCtx != 4096 {
		t.Errorf("num_ctx: want 4096, got %v", options["num_ctx"])
	}
}

func TestOllamaClient_Init_ErrModelNotInstalled(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/api/tags": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, tagsResponseOK("llama3:8b", "sha256:other"))
		},
	})
	defer srv.Close()

	_, err := NewOllamaClient(OllamaOptions{
		Endpoint: srv.URL,
		Model:    "gemma2:9b",
		Timeout:  5 * time.Second,
	})
	if !errors.Is(err, ErrModelNotInstalled) {
		t.Errorf("want ErrModelNotInstalled, got %v", err)
	}
}

func TestOllamaClient_Init_OfflineReturnsErrOllamaOffline(t *testing.T) {
	// 127.0.0.1:1 is guaranteed to refuse connection.
	_, err := NewOllamaClient(OllamaOptions{
		Endpoint: "http://127.0.0.1:1",
		Model:    "gemma2:9b",
		Timeout:  500 * time.Millisecond,
	})
	if !errors.Is(err, ErrOllamaOffline) {
		t.Errorf("want ErrOllamaOffline, got %v", err)
	}
}

func TestOllamaClient_Init_RecordsDigest(t *testing.T) {
	wantDigest := "sha256:ff02c3702f32aaaaaa"
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/api/tags": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, tagsResponseOK("gemma2:9b", wantDigest))
		},
	})
	defer srv.Close()

	c, err := NewOllamaClient(OllamaOptions{
		Endpoint: srv.URL,
		Model:    "gemma2:9b",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}
	if c.Digest() != wantDigest {
		t.Errorf("digest: want %q, got %q", wantDigest, c.Digest())
	}
}

func TestOllamaClient_ContextBudget_FromShow(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/api/tags": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, tagsResponseOK("gemma2:9b", "sha256:ff02c3"))
		},
		"/api/show": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"model_info":{"gemma2.context_length":8192}}`)
		},
	})
	defer srv.Close()

	c, err := NewOllamaClient(OllamaOptions{
		Endpoint: srv.URL,
		Model:    "gemma2:9b",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}
	if c.ContextBudget() != 8192 {
		t.Errorf("context budget: want 8192, got %d", c.ContextBudget())
	}
}

func TestOllamaClient_ContextBudget_FallbackOnMissing(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/api/tags": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, tagsResponseOK("gemma2:9b", "sha256:ff02c3"))
		},
		"/api/show": func(w http.ResponseWriter, r *http.Request) {
			// No model_info — simulate unknown model shape.
			_, _ = io.WriteString(w, `{}`)
		},
	})
	defer srv.Close()

	c, err := NewOllamaClient(OllamaOptions{
		Endpoint: srv.URL,
		Model:    "gemma2:9b",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}
	if c.ContextBudget() != DefaultContextBudget {
		t.Errorf("fallback context budget: want %d, got %d", DefaultContextBudget, c.ContextBudget())
	}
}

func TestOllamaClient_Generate_RetriesOn5xx(t *testing.T) {
	var calls int
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/api/tags": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, tagsResponseOK("gemma2:9b", "sha256:ff02c3"))
		},
		"/api/generate": func(w http.ResponseWriter, r *http.Request) {
			calls++
			if calls < 2 {
				w.WriteHeader(503)
				return
			}
			_, _ = io.WriteString(w, `{"response":"ok"}`)
		},
	})
	defer srv.Close()

	c, err := NewOllamaClient(OllamaOptions{
		Endpoint:   srv.URL,
		Model:      "gemma2:9b",
		Timeout:    5 * time.Second,
		MaxRetries: 3,
	})
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}
	resp, err := c.Generate("hi")
	if err != nil {
		t.Fatalf("Generate after retry: %v", err)
	}
	if resp != "ok" {
		t.Errorf("response: want 'ok', got %q", resp)
	}
	if calls != 2 {
		t.Errorf("want 2 calls (1 retry), got %d", calls)
	}
}

func TestOllamaClient_Generate_MalformedJSON(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/api/tags": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, tagsResponseOK("gemma2:9b", "sha256:ff02c3"))
		},
		"/api/generate": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `not json{{`)
		},
	})
	defer srv.Close()

	c, err := NewOllamaClient(OllamaOptions{
		Endpoint: srv.URL,
		Model:    "gemma2:9b",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}
	_, err = c.Generate("hi")
	if err == nil {
		t.Fatal("want error on malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "decode") && !strings.Contains(err.Error(), "JSON") && !strings.Contains(err.Error(), "json") {
		t.Errorf("error should mention decode/json, got %v", err)
	}
}

func TestOllamaClient_Generate_Timeout(t *testing.T) {
	srv := newTestServer(t, map[string]http.HandlerFunc{
		"/api/tags": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, tagsResponseOK("gemma2:9b", "sha256:ff02c3"))
		},
		"/api/generate": func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			_, _ = io.WriteString(w, `{"response":"too late"}`)
		},
	})
	defer srv.Close()

	c, err := NewOllamaClient(OllamaOptions{
		Endpoint: srv.URL,
		Model:    "gemma2:9b",
		Timeout:  5 * time.Second, // init-phase timeout
	})
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}
	// Use a per-request timeout smaller than the server's sleep.
	c.SetRequestTimeout(50 * time.Millisecond)
	_, err = c.Generate("hi")
	if err == nil {
		t.Fatal("want timeout error, got nil")
	}
}

func TestDefaultEndpoint_ResolvesFromEnv(t *testing.T) {
	t.Setenv("AGENTOPS_LLM_ENDPOINT", "http://example.invalid:9999")
	got := ResolveDefaultEndpoint()
	if got != "http://example.invalid:9999" {
		t.Errorf("env override: got %q", got)
	}

	t.Setenv("AGENTOPS_LLM_ENDPOINT", "")
	got = ResolveDefaultEndpoint()
	if got != "http://localhost:11434" {
		t.Errorf("fallback: want http://localhost:11434, got %q", got)
	}
}
