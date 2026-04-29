package gascity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"

	"github.com/boshu2/agentops/cli/internal/bridge"
)

var ErrFallbackDisabled = errors.New("gascity CLI fallback disabled")

// CommandRunner runs a command and returns its combined output.
type CommandRunner interface {
	RunCommand(ctx context.Context, name string, args ...string) ([]byte, error)
}

// CommandRunnerFunc adapts a function into a CommandRunner.
type CommandRunnerFunc func(ctx context.Context, name string, args ...string) ([]byte, error)

func (f CommandRunnerFunc) RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return f(ctx, name, args...)
}

// FallbackConfig makes CLI fallback explicit. Disabled fallback never runs gc.
type FallbackConfig struct {
	Enabled  bool
	Command  string
	CityPath string
	Runner   CommandRunner
}

// AdapterConfig combines the API client with an explicit CLI fallback.
type AdapterConfig struct {
	Client   *Client
	Fallback FallbackConfig
}

// Adapter prefers the public API and only falls back to gc CLI when configured
// and the API is unavailable.
type Adapter struct {
	client   *Client
	fallback *Fallback
}

// NewAdapter creates an API-first adapter with optional explicit fallback.
func NewAdapter(cfg AdapterConfig) *Adapter {
	return &Adapter{
		client:   cfg.Client,
		fallback: NewFallback(cfg.Fallback),
	}
}

// Fallback is an explicit gc CLI fallback adapter.
type Fallback struct {
	enabled  bool
	command  string
	cityPath string
	runner   CommandRunner
}

// NewFallback creates a fallback adapter. It may be disabled; disabled methods
// return ErrFallbackDisabled.
func NewFallback(cfg FallbackConfig) *Fallback {
	command := strings.TrimSpace(cfg.Command)
	if command == "" {
		command = "gc"
	}
	runner := cfg.Runner
	if runner == nil {
		runner = execCommandRunner{}
	}
	return &Fallback{
		enabled:  cfg.Enabled,
		command:  command,
		cityPath: strings.TrimSpace(cfg.CityPath),
		runner:   runner,
	}
}

// ListCityEvents returns city-scoped events using gc events JSONL output.
func (f *Fallback) ListCityEvents(ctx context.Context, params EventListParams) (EventListResponse, error) {
	if err := f.requireEnabled(); err != nil {
		return EventListResponse{}, err
	}
	args := withCityPath(f.cityPath, bridge.GCEventsArgs(bridge.GCEventsArgsConfig{
		Type:  params.Type,
		Since: params.Since,
		After: params.Index,
	})...)
	out, err := f.runner.RunCommand(ctx, f.command, args...)
	if err != nil {
		return EventListResponse{}, fmt.Errorf("gc events fallback: %w", err)
	}
	events, err := DecodeWireEventJSONLines(strings.NewReader(string(out)))
	if err != nil {
		return EventListResponse{}, err
	}
	return EventListResponse{Items: events, Total: len(events)}, nil
}

// ListEvents returns supervisor-scoped events using gc events JSONL output.
func (f *Fallback) ListEvents(ctx context.Context, params EventListParams) (TaggedEventListResponse, error) {
	if err := f.requireEnabled(); err != nil {
		return TaggedEventListResponse{}, err
	}
	args := bridge.GCEventsArgs(bridge.GCEventsArgsConfig{
		Type:        params.Type,
		Since:       params.Since,
		AfterCursor: params.Cursor,
	})
	out, err := f.runner.RunCommand(ctx, f.command, args...)
	if err != nil {
		return TaggedEventListResponse{}, fmt.Errorf("gc events fallback: %w", err)
	}
	events, err := DecodeTaggedWireEventJSONLines(strings.NewReader(string(out)))
	if err != nil {
		return TaggedEventListResponse{}, err
	}
	return TaggedEventListResponse{Items: events, Total: len(events)}, nil
}

// ListSessions returns sessions using gc session list --json.
func (f *Fallback) ListSessions(ctx context.Context) (SessionListResponse, error) {
	if err := f.requireEnabled(); err != nil {
		return SessionListResponse{}, err
	}
	args := withCityPath(f.cityPath, bridge.GCSessionListArgs()...)
	out, err := f.runner.RunCommand(ctx, f.command, args...)
	if err != nil {
		return SessionListResponse{}, fmt.Errorf("gc session list fallback: %w", err)
	}
	gcSessions, err := bridge.ParseGCSessions(out)
	if err != nil {
		return SessionListResponse{}, err
	}
	sessions := make([]Session, len(gcSessions))
	for i, session := range gcSessions {
		sessions[i] = Session{
			ID:       session.ID,
			Alias:    session.Alias,
			State:    session.State,
			Template: session.Template,
			Closed:   session.Closed,
		}
	}
	return SessionListResponse{Items: sessions, Total: len(sessions)}, nil
}

// EmitCityEvent records an event using gc event emit.
func (f *Fallback) EmitCityEvent(ctx context.Context, req EventEmitRequest) (EventEmitResponse, error) {
	if err := f.requireEnabled(); err != nil {
		return EventEmitResponse{}, err
	}
	payload, err := json.Marshal(map[string]string{})
	if err != nil {
		return EventEmitResponse{}, err
	}
	args := withCityPath(
		f.cityPath,
		bridge.GCEventEmitArgsWithFields(req.Type, req.Actor, req.Subject, req.Message, string(payload))...,
	)
	if _, err := f.runner.RunCommand(ctx, f.command, args...); err != nil {
		return EventEmitResponse{}, fmt.Errorf("gc event emit fallback: %w", err)
	}
	return EventEmitResponse{Status: "recorded"}, nil
}

// ListCityEvents prefers API and falls back to gc CLI only on API-unavailable errors.
func (a *Adapter) ListCityEvents(
	ctx context.Context,
	cityName string,
	params EventListParams,
) (EventListResponse, ResponseMeta, error) {
	if a.client != nil {
		out, meta, err := a.client.ListCityEvents(ctx, cityName, params)
		if err == nil || !IsAPIUnavailable(err) {
			return out, meta, err
		}
		fallbackOut, fallbackErr := a.fallback.ListCityEvents(ctx, params)
		if fallbackErr == nil {
			return fallbackOut, ResponseMeta{}, nil
		}
		return EventListResponse{}, meta, fmt.Errorf("%w; fallback failed: %v", err, fallbackErr)
	}
	fallbackOut, err := a.fallback.ListCityEvents(ctx, params)
	return fallbackOut, ResponseMeta{}, err
}

// IsAPIUnavailable returns true for transport or 5xx GasCity API failures.
func IsAPIUnavailable(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusBadGateway ||
			apiErr.StatusCode == http.StatusServiceUnavailable ||
			apiErr.StatusCode == http.StatusGatewayTimeout
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

func (f *Fallback) requireEnabled() error {
	if f == nil || !f.enabled {
		return ErrFallbackDisabled
	}
	return nil
}

func withCityPath(cityPath string, args ...string) []string {
	if strings.TrimSpace(cityPath) == "" {
		return args
	}
	return append([]string{"--city", cityPath}, args...)
}

type execCommandRunner struct{}

func (execCommandRunner) RunCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		if text != "" {
			return out, fmt.Errorf("%w: %s", err, text)
		}
	}
	return out, err
}
