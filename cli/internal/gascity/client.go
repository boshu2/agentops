package gascity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

const defaultMutationToken = "agentops"

// Config controls the GasCity public API client.
type Config struct {
	Endpoint      string
	HTTPClient    *http.Client
	MutationToken string
}

// Client is a narrow handwritten client for GasCity public API operations.
type Client struct {
	endpoint      *url.URL
	httpClient    *http.Client
	mutationToken string
}

// ResponseMeta carries correlation metadata from a GasCity response.
type ResponseMeta struct {
	StatusCode int
	RequestID  string
}

// NewClient validates config and returns a GasCity API client.
func NewClient(cfg Config) (*Client, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("gascity endpoint is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse gascity endpoint: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("gascity endpoint must include scheme and host")
	}
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	mutationToken := strings.TrimSpace(cfg.MutationToken)
	if mutationToken == "" {
		mutationToken = defaultMutationToken
	}
	return &Client{
		endpoint:      parsed,
		httpClient:    client,
		mutationToken: mutationToken,
	}, nil
}

// Endpoint returns the normalized base endpoint string.
func (c *Client) Endpoint() string {
	return strings.TrimRight(c.endpoint.String(), "/")
}

// Health reads GET /health.
func (c *Client) Health(ctx context.Context) (HealthResponse, error) {
	var out HealthResponse
	if err := c.getJSON(ctx, "/health", &out); err != nil {
		return HealthResponse{}, err
	}
	return out, nil
}

// Readiness reads GET /v0/readiness.
func (c *Client) Readiness(ctx context.Context) (ReadinessResponse, error) {
	return c.readiness(ctx, "/v0/readiness")
}

// ProviderReadiness reads GET /v0/provider-readiness.
func (c *Client) ProviderReadiness(ctx context.Context) (ReadinessResponse, error) {
	return c.readiness(ctx, "/v0/provider-readiness")
}

// CityReadiness reads GET /v0/city/{cityName}/readiness.
func (c *Client) CityReadiness(ctx context.Context, cityName string) (ReadinessResponse, error) {
	cityName = strings.TrimSpace(cityName)
	if cityName == "" {
		return ReadinessResponse{}, fmt.Errorf("city name is required")
	}
	return c.readiness(ctx, "/v0/city/"+url.PathEscape(cityName)+"/readiness")
}

func (c *Client) readiness(ctx context.Context, path string) (ReadinessResponse, error) {
	var out ReadinessResponse
	if err := c.getJSON(ctx, path, &out); err != nil {
		return ReadinessResponse{}, err
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, requestPath string, out any) error {
	_, err := c.doJSONWithQuery(ctx, http.MethodGet, requestPath, nil, nil, out)
	return err
}

func (c *Client) doJSON(
	ctx context.Context,
	method string,
	requestPath string,
	body any,
	out any,
) (ResponseMeta, error) {
	return c.doJSONWithQuery(ctx, method, requestPath, nil, body, out)
}

func (c *Client) doJSONWithQuery(
	ctx context.Context,
	method string,
	requestPath string,
	query url.Values,
	body any,
	out any,
) (ResponseMeta, error) {
	endpoint := *c.endpoint
	endpoint.Path = joinURLPath(endpoint.Path, requestPath)
	endpoint.RawQuery = query.Encode()

	var reqBody io.Reader
	if body != nil {
		buf := &bytes.Buffer{}
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return ResponseMeta{}, fmt.Errorf(
				"encode gascity %s %s: %w",
				method,
				requestPath,
				err,
			)
		}
		reqBody = buf
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reqBody)
	if err != nil {
		return ResponseMeta{}, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if isMutationMethod(method) {
		req.Header.Set(MutationHeader, c.mutationToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ResponseMeta{}, fmt.Errorf("gascity %s %s: %w", method, requestPath, err)
	}
	defer func() { _ = resp.Body.Close() }()

	meta := ResponseMeta{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get(RequestIDHeader),
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return meta, &APIError{
			Method:     method,
			Path:       requestPath,
			StatusCode: resp.StatusCode,
			RequestID:  meta.RequestID,
			Problem:    decodeProblemDetails(resp.Body, resp.StatusCode),
		}
	}
	if out == nil {
		return meta, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return meta, fmt.Errorf("decode gascity %s %s: %w", method, requestPath, err)
	}
	return meta, nil
}

func decodeProblemDetails(body io.Reader, statusCode int) *ProblemDetails {
	var problem ProblemDetails
	if err := json.NewDecoder(body).Decode(&problem); err != nil {
		return nil
	}
	if problem.Type == "" &&
		problem.Title == "" &&
		problem.Status == 0 &&
		problem.Detail == "" &&
		problem.Instance == "" &&
		len(problem.Errors) == 0 {
		return nil
	}
	if problem.Status == 0 {
		problem.Status = statusCode
	}
	return &problem
}

func isMutationMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func joinURLPath(basePath, requestPath string) string {
	joined := path.Join("/", basePath, requestPath)
	if strings.HasSuffix(requestPath, "/") && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	return joined
}
