package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// DefaultContextBudget is the fallback context window when /api/show does not
// expose the model's context length. Conservative value; most local 8-9B
// gemma models advertise 8192.
const DefaultContextBudget = 4096

// DefaultRequestTimeout caps a single /api/generate call. Warm gemma2:9b
// returns turn summaries in ~3-4s per the W0-6 spike; 120s accommodates cold
// starts (~70s) without hanging the whole pipeline.
const DefaultRequestTimeout = 120 * time.Second

// Sentinel errors returned by the ollama client so callers can branch on
// specific failure modes (offline vs model-missing vs generic).
var (
	ErrOllamaOffline     = errors.New("ollama: daemon unreachable")
	ErrModelNotInstalled = errors.New("ollama: model not installed")
)

// OllamaOptions configures a new client.
type OllamaOptions struct {
	// Endpoint is the base URL, e.g. http://localhost:11434. No trailing
	// slash. If empty, ResolveDefaultEndpoint is used.
	Endpoint string

	// Model is the ollama model tag, e.g. "gemma2:9b".
	Model string

	// Timeout is the initialization-phase HTTP timeout (applied to /api/tags
	// and /api/show). Generate requests use RequestTimeout, defaulting to
	// DefaultRequestTimeout and overridable via SetRequestTimeout.
	Timeout time.Duration

	// MaxRetries is the retry budget for 5xx responses on /api/generate. 0
	// disables retries; the initial attempt always runs.
	MaxRetries int
}

// OllamaClient is a minimal HTTP client for the ollama /api/generate endpoint.
// It validates the model is installed at construction time, records the
// model digest for frontmatter stamping, and looks up the context budget
// via /api/show (falling back to DefaultContextBudget on any failure).
type OllamaClient struct {
	endpoint       string
	model          string
	digest         string
	contextBudget  int
	httpClient     *http.Client
	requestTimeout time.Duration
	maxRetries     int
}

type tagsResponse struct {
	Models []struct {
		Name   string `json:"name"`
		Digest string `json:"digest"`
	} `json:"models"`
}

type showResponse struct {
	ModelInfo map[string]any `json:"model_info"`
}

type generateRequest struct {
	Model     string                 `json:"model"`
	Prompt    string                 `json:"prompt"`
	Stream    bool                   `json:"stream"`
	Format    string                 `json:"format,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty"`
	KeepAlive string                 `json:"keep_alive,omitempty"`
}

type generateResponse struct {
	Response string `json:"response"`
}

// NewOllamaClient constructs a client, probes /api/tags to validate the model
// is installed, records its digest, and queries /api/show for the context
// budget. Returns ErrOllamaOffline or ErrModelNotInstalled for specific
// construction failures; other errors are wrapped.
func NewOllamaClient(opts OllamaOptions) (*OllamaClient, error) {
	if opts.Endpoint == "" {
		opts.Endpoint = ResolveDefaultEndpoint()
	}
	if opts.Model == "" {
		return nil, errors.New("ollama: model is required")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	// Strip trailing slash.
	opts.Endpoint = strings.TrimRight(opts.Endpoint, "/")

	c := &OllamaClient{
		endpoint:       opts.Endpoint,
		model:          opts.Model,
		contextBudget:  DefaultContextBudget,
		httpClient:     &http.Client{Timeout: opts.Timeout},
		requestTimeout: DefaultRequestTimeout,
		maxRetries:     opts.MaxRetries,
	}

	// Step 1: /api/tags validates reachability and model presence.
	tags, err := c.fetchTags()
	if err != nil {
		if isConnRefused(err) {
			return nil, fmt.Errorf("%w: %v", ErrOllamaOffline, err)
		}
		return nil, fmt.Errorf("ollama: /api/tags: %w", err)
	}
	var digest string
	for _, m := range tags.Models {
		if m.Name == opts.Model {
			digest = m.Digest
			break
		}
	}
	if digest == "" {
		return nil, fmt.Errorf("%w: %s", ErrModelNotInstalled, opts.Model)
	}
	c.digest = digest

	// Step 2: /api/show for context budget. Best-effort; fall back on any
	// failure so the pipeline degrades honestly.
	if budget, ok := c.fetchContextBudget(); ok {
		c.contextBudget = budget
	}

	return c, nil
}

// Digest returns the model digest recorded at init. Used for frontmatter
// (model_digest field) and version-pinning in session pages.
func (c *OllamaClient) Digest() string { return c.digest }

// ContextBudget returns the advertised context window size in tokens.
func (c *OllamaClient) ContextBudget() int { return c.contextBudget }

// SetRequestTimeout overrides the per-request timeout for subsequent
// Generate calls. Useful for tests.
func (c *OllamaClient) SetRequestTimeout(d time.Duration) {
	c.requestTimeout = d
}

// Generate posts a single prompt to /api/generate with stream:false and
// returns the response body string. Retries up to MaxRetries times on 5xx.
func (c *OllamaClient) Generate(prompt string) (string, error) {
	body := generateRequest{
		Model:  c.model,
		Prompt: prompt,
		// CRITICAL: stream:false. Ollama's default is true which would break
		// single-shot JSON decode. Asserted by TestOllamaClient_Generate_SendsStreamFalse.
		Stream: false,
		// format:json forces ollama's structured output mode. gemma4:e4b
		// returns empty responses without this; the JS worker on bushido
		// uses it for all 9K+ successful ingests.
		Format:    "json",
		KeepAlive: "30m",
		Options: map[string]interface{}{
			"temperature": 0.2,
			"num_predict": 800,
			"num_ctx":     4096,
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("ollama: marshal generate request: %w", err)
	}

	var lastErr error
	attempts := c.maxRetries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		text, retry, err := c.generateOnce(raw)
		if err == nil {
			return text, nil
		}
		lastErr = err
		if !retry {
			return "", err
		}
	}
	return "", fmt.Errorf("ollama: generate after %d attempts: %w", attempts, lastErr)
}

// generateOnce runs one HTTP round-trip. Returns (response, retryable, err).
func (c *OllamaClient) generateOnce(body []byte) (string, bool, error) {
	client := &http.Client{Timeout: c.requestTimeout}
	resp, err := client.Post(c.endpoint+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		// Network errors (including timeouts) are terminal for this request
		// — we don't retry timeouts because the whole session should move on.
		return "", false, fmt.Errorf("ollama: POST /api/generate: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return "", true, fmt.Errorf("ollama: /api/generate returned %d", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", false, fmt.Errorf("ollama: /api/generate returned %d: %s", resp.StatusCode, truncForErr(b))
	}
	var out generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", false, fmt.Errorf("ollama: decode /api/generate json response: %w", err)
	}
	return out.Response, false, nil
}

func (c *OllamaClient) fetchTags() (*tagsResponse, error) {
	resp, err := c.httpClient.Get(c.endpoint + "/api/tags")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var out tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// fetchContextBudget calls /api/show?model=<model> and tries to extract a
// context_length field. Best-effort: returns (0, false) on any failure.
func (c *OllamaClient) fetchContextBudget() (int, bool) {
	body, err := json.Marshal(map[string]string{"name": c.model})
	if err != nil {
		return 0, false
	}
	resp, err := c.httpClient.Post(c.endpoint+"/api/show", "application/json", bytes.NewReader(body))
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		return 0, false
	}
	defer resp.Body.Close()
	var out showResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, false
	}
	if out.ModelInfo == nil {
		return 0, false
	}
	// Try common key patterns. Ollama exposes keys like
	// "gemma2.context_length", "llama.context_length", etc.
	for k, v := range out.ModelInfo {
		if !strings.HasSuffix(k, ".context_length") && k != "context_length" {
			continue
		}
		switch n := v.(type) {
		case float64:
			return int(n), true
		case int:
			return n, true
		}
	}
	return 0, false
}

// ResolveDefaultEndpoint returns the default ollama endpoint from
// $AGENTOPS_LLM_ENDPOINT or http://localhost:11434. Not hardcoded to bushido
// per pre-mortem F6.
func ResolveDefaultEndpoint() string {
	if v := os.Getenv("AGENTOPS_LLM_ENDPOINT"); v != "" {
		return v
	}
	return "http://localhost:11434"
}

func isConnRefused(err error) bool {
	if err == nil {
		return false
	}
	// Both net.OpError{Op:"dial"} and url.Error wrapping it count as offline.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		msg := urlErr.Error()
		return strings.Contains(msg, "connection refused") ||
			strings.Contains(msg, "dial") ||
			strings.Contains(msg, "no such host")
	}
	return false
}

func truncForErr(b []byte) string {
	if len(b) > 200 {
		return string(b[:200]) + "…"
	}
	return string(b)
}
