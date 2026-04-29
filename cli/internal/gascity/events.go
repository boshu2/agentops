package gascity

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const lastEventIDHeader = "Last-Event-ID"

// ListEvents calls GET /v0/events at supervisor scope.
func (c *Client) ListEvents(
	ctx context.Context,
	params EventListParams,
) (TaggedEventListResponse, ResponseMeta, error) {
	var out TaggedEventListResponse
	meta, err := c.doJSONWithQuery(
		ctx,
		http.MethodGet,
		"/v0/events",
		supervisorEventListQuery(params),
		nil,
		&out,
	)
	if err != nil {
		return TaggedEventListResponse{}, meta, err
	}
	return out, meta, nil
}

// ListCityEvents calls GET /v0/city/{cityName}/events.
func (c *Client) ListCityEvents(
	ctx context.Context,
	cityName string,
	params EventListParams,
) (EventListResponse, ResponseMeta, error) {
	eventsPath, err := cityPath(cityName, "events")
	if err != nil {
		return EventListResponse{}, ResponseMeta{}, err
	}
	var out EventListResponse
	meta, err := c.doJSONWithQuery(
		ctx,
		http.MethodGet,
		eventsPath,
		cityEventListQuery(params),
		nil,
		&out,
	)
	if err != nil {
		return EventListResponse{}, meta, err
	}
	return out, meta, nil
}

// EmitCityEvent calls POST /v0/city/{cityName}/events.
func (c *Client) EmitCityEvent(
	ctx context.Context,
	cityName string,
	req EventEmitRequest,
) (EventEmitResponse, ResponseMeta, error) {
	eventsPath, err := cityPath(cityName, "events")
	if err != nil {
		return EventEmitResponse{}, ResponseMeta{}, err
	}
	var out EventEmitResponse
	meta, err := c.doJSON(ctx, http.MethodPost, eventsPath, req, &out)
	if err != nil {
		return EventEmitResponse{}, meta, err
	}
	return out, meta, nil
}

// StreamEvents opens GET /v0/events/stream at supervisor scope.
func (c *Client) StreamEvents(
	ctx context.Context,
	opts EventStreamOptions,
) (*EventStream, ResponseMeta, error) {
	query := url.Values{}
	if opts.AfterCursor != "" {
		query.Set("after_cursor", opts.AfterCursor)
	}
	return c.openEventStream(ctx, "/v0/events/stream", query, opts.LastEventID)
}

// StreamCityEvents opens GET /v0/city/{cityName}/events/stream.
func (c *Client) StreamCityEvents(
	ctx context.Context,
	cityName string,
	opts EventStreamOptions,
) (*EventStream, ResponseMeta, error) {
	eventsPath, err := cityPath(cityName, "events", "stream")
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	query := url.Values{}
	if opts.AfterSeq != "" {
		query.Set("after_seq", opts.AfterSeq)
	}
	return c.openEventStream(ctx, eventsPath, query, opts.LastEventID)
}

func (c *Client) openEventStream(
	ctx context.Context,
	requestPath string,
	query url.Values,
	lastEventID string,
) (*EventStream, ResponseMeta, error) {
	endpoint := *c.endpoint
	endpoint.Path = joinURLPath(endpoint.Path, requestPath)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, ResponseMeta{}, err
	}
	req.Header.Set("Accept", "text/event-stream")
	if lastEventID != "" {
		req.Header.Set(lastEventIDHeader, lastEventID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ResponseMeta{}, fmt.Errorf("gascity GET %s: %w", requestPath, err)
	}
	meta := ResponseMeta{
		StatusCode: resp.StatusCode,
		RequestID:  resp.Header.Get(RequestIDHeader),
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, meta, &APIError{
			Method:     http.MethodGet,
			Path:       requestPath,
			StatusCode: resp.StatusCode,
			RequestID:  meta.RequestID,
			Problem:    decodeProblemDetails(resp.Body, resp.StatusCode),
		}
	}
	return &EventStream{
		body:    resp.Body,
		decoder: NewSSEDecoder(resp.Body),
	}, meta, nil
}

// EventStream is an open GasCity SSE response.
type EventStream struct {
	body    io.Closer
	decoder *SSEDecoder
}

// Recv returns the next SSE frame, including heartbeat frames.
func (s *EventStream) Recv() (EventStreamFrame, error) {
	return s.decoder.Decode()
}

// NextEvent returns the next semantic event frame, skipping heartbeats.
func (s *EventStream) NextEvent() (EventStreamFrame, error) {
	for {
		frame, err := s.Recv()
		if err != nil {
			return EventStreamFrame{}, err
		}
		if frame.Heartbeat != nil {
			continue
		}
		return frame, nil
	}
}

// Close closes the underlying SSE response body.
func (s *EventStream) Close() error {
	return s.body.Close()
}

// ReconnectOptions returns both header and query reconnect cursors for the
// selected stream scope. GasCity accepts either; setting both makes reconnect
// behavior explicit across browser and non-browser clients.
func ReconnectOptions(scope EventStreamScope, cursor string) EventStreamOptions {
	opts := EventStreamOptions{LastEventID: cursor}
	switch scope {
	case EventStreamScopeCity:
		opts.AfterSeq = cursor
	case EventStreamScopeSupervisor:
		opts.AfterCursor = cursor
	}
	return opts
}

// CursorFromFrame returns the durable reconnect cursor for a parsed SSE frame.
func CursorFromFrame(frame EventStreamFrame) string {
	if frame.ID != "" {
		return frame.ID
	}
	if frame.CityEvent != nil && frame.CityEvent.Seq > 0 {
		return strconv.FormatInt(frame.CityEvent.Seq, 10)
	}
	if frame.TaggedEvent != nil && frame.TaggedEvent.Seq > 0 {
		return strconv.FormatInt(frame.TaggedEvent.Seq, 10)
	}
	return ""
}

// ClassifyTerminalState maps GasCity event/session evidence to AgentOps' worker
// terminal/degraded states.
func ClassifyTerminalState(input TerminalStateInput) TerminalClassification {
	if status := terminalStatusFromEvent(input.EventType, input.EventPayload); status != TerminalStatusUnknown {
		return classifyTerminalWithTranscript(status, input)
	}
	if status := terminalStatusFromSession(input.SessionState, input.SessionStatus); status != TerminalStatusUnknown {
		return classifyTerminalWithTranscript(status, input)
	}
	if input.SessionMissing {
		return TerminalClassification{
			Status:   TerminalStatusLost,
			Terminal: true,
			Degraded: true,
			Reason:   "session missing after acceptance",
		}
	}
	if input.ProviderUnreachable {
		return TerminalClassification{
			Status:   TerminalStatusProviderUnreachable,
			Degraded: true,
			Reason:   "provider readiness unavailable before terminal state",
		}
	}
	if input.EventStreamUnavailable {
		return TerminalClassification{
			Status:   TerminalStatusEventStreamUnavailable,
			Degraded: true,
			Reason:   "event stream unavailable; REST reconciliation required",
		}
	}
	return TerminalClassification{Status: TerminalStatusRunning}
}

func classifyTerminalWithTranscript(status string, input TerminalStateInput) TerminalClassification {
	if input.TranscriptUnavailable || (input.TranscriptRequired && !input.TranscriptAvailable) {
		return TerminalClassification{
			Status:   TerminalStatusTerminalWithoutTranscript,
			Terminal: true,
			Degraded: true,
			Reason:   "terminal state observed without transcript evidence",
		}
	}
	return TerminalClassification{
		Status:   status,
		Terminal: true,
	}
}

func terminalStatusFromEvent(eventType string, payload map[string]any) string {
	if status := terminalStatusFromString(payloadString(payload, "status")); status != TerminalStatusUnknown {
		return status
	}
	eventType = strings.ToLower(strings.TrimSpace(eventType))
	switch {
	case strings.Contains(eventType, ".completed"),
		strings.Contains(eventType, ".succeeded"),
		strings.HasSuffix(eventType, ".ready"):
		return TerminalStatusCompleted
	case strings.Contains(eventType, ".failed"),
		strings.Contains(eventType, ".error"),
		strings.Contains(eventType, ".init_failed"):
		return TerminalStatusFailed
	case strings.Contains(eventType, ".cancelled"),
		strings.Contains(eventType, ".canceled"),
		strings.Contains(eventType, ".killed"):
		return TerminalStatusCancelled
	case strings.Contains(eventType, ".closed"),
		strings.Contains(eventType, ".drained"),
		strings.Contains(eventType, ".archived"):
		return TerminalStatusCompleted
	default:
		return TerminalStatusUnknown
	}
}

func terminalStatusFromSession(state, status string) string {
	if mapped := terminalStatusFromString(status); mapped != TerminalStatusUnknown {
		return mapped
	}
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "closed", "drained", "archived":
		return TerminalStatusCompleted
	default:
		return TerminalStatusUnknown
	}
}

func terminalStatusFromString(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "completed", "complete", "succeeded", "success", "ok":
		return TerminalStatusCompleted
	case "failed", "failure", "error":
		return TerminalStatusFailed
	case "cancelled", "canceled", "cancel", "killed":
		return TerminalStatusCancelled
	default:
		return TerminalStatusUnknown
	}
}

func payloadString(payload map[string]any, key string) string {
	if len(payload) == 0 {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

// SSEDecoder parses text/event-stream frames.
type SSEDecoder struct {
	reader *bufio.Reader
}

// NewSSEDecoder returns a decoder for a GasCity text/event-stream body.
func NewSSEDecoder(r io.Reader) *SSEDecoder {
	return &SSEDecoder{reader: bufio.NewReader(r)}
}

// Decode reads one SSE frame.
func (d *SSEDecoder) Decode() (EventStreamFrame, error) {
	raw, err := d.readRawFrame()
	if err != nil {
		return EventStreamFrame{}, err
	}
	frame := EventStreamFrame{
		ID:      raw.id,
		Event:   raw.event,
		Retry:   raw.retry,
		RawData: raw.data,
	}
	switch raw.event {
	case "heartbeat":
		var heartbeat HeartbeatEvent
		if err := json.Unmarshal(raw.data, &heartbeat); err != nil {
			return EventStreamFrame{}, fmt.Errorf("decode heartbeat SSE frame: %w", err)
		}
		frame.Heartbeat = &heartbeat
	case "event":
		var event EventStreamEnvelope
		if err := json.Unmarshal(raw.data, &event); err != nil {
			return EventStreamFrame{}, fmt.Errorf("decode event SSE frame: %w", err)
		}
		frame.CityEvent = &event
	case "tagged_event":
		var event TaggedEventStreamEnvelope
		if err := json.Unmarshal(raw.data, &event); err != nil {
			return EventStreamFrame{}, fmt.Errorf("decode tagged_event SSE frame: %w", err)
		}
		frame.TaggedEvent = &event
	}
	return frame, nil
}

type rawSSEFrame struct {
	id    string
	event string
	retry int
	data  []byte
}

func (d *SSEDecoder) readRawFrame() (rawSSEFrame, error) {
	var frame rawSSEFrame
	var data []string
	for {
		line, err := d.reader.ReadString('\n')
		if err != nil && !(err == io.EOF && line != "") {
			if err == io.EOF && frame.id == "" && frame.event == "" && len(data) == 0 {
				return rawSSEFrame{}, io.EOF
			}
			return rawSSEFrame{}, err
		}

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if frame.id == "" && frame.event == "" && len(data) == 0 {
				if err == io.EOF {
					return rawSSEFrame{}, io.EOF
				}
				continue
			}
			frame.data = []byte(strings.Join(data, "\n"))
			return frame, nil
		}

		if strings.HasPrefix(line, ":") {
			if err == io.EOF {
				return rawSSEFrame{}, io.EOF
			}
			continue
		}

		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "id":
			frame.id = value
		case "event":
			frame.event = value
		case "retry":
			retry, parseErr := strconv.Atoi(value)
			if parseErr != nil {
				return rawSSEFrame{}, fmt.Errorf("parse SSE retry %q: %w", value, parseErr)
			}
			frame.retry = retry
		case "data":
			data = append(data, value)
		}

		if err == io.EOF {
			frame.data = []byte(strings.Join(data, "\n"))
			return frame, nil
		}
	}
}

// DecodeWireEventJSONLines parses JSONL emitted by city-scoped gc events list mode.
func DecodeWireEventJSONLines(r io.Reader) ([]WireEvent, error) {
	var events []WireEvent
	err := scanJSONLines(r, func(line []byte) error {
		var event WireEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return err
		}
		events = append(events, event)
		return nil
	})
	return events, err
}

// DecodeTaggedWireEventJSONLines parses JSONL emitted by supervisor gc events list mode.
func DecodeTaggedWireEventJSONLines(r io.Reader) ([]TaggedWireEvent, error) {
	var events []TaggedWireEvent
	err := scanJSONLines(r, func(line []byte) error {
		var event TaggedWireEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return err
		}
		events = append(events, event)
		return nil
	})
	return events, err
}

func scanJSONLines(r io.Reader, decode func([]byte) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := decode([]byte(line)); err != nil {
			return fmt.Errorf("decode JSONL line %d: %w", lineNo, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan JSONL: %w", err)
	}
	return nil
}

func supervisorEventListQuery(params EventListParams) url.Values {
	query := eventFilterQuery(params)
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	return query
}

func cityEventListQuery(params EventListParams) url.Values {
	query := eventFilterQuery(params)
	if params.Index != "" {
		query.Set("index", params.Index)
	}
	if params.Wait != "" {
		query.Set("wait", params.Wait)
	}
	if params.Cursor != "" {
		query.Set("cursor", params.Cursor)
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	return query
}

func eventFilterQuery(params EventListParams) url.Values {
	query := url.Values{}
	if params.Type != "" {
		query.Set("type", params.Type)
	}
	if params.Actor != "" {
		query.Set("actor", params.Actor)
	}
	if params.Since != "" {
		query.Set("since", params.Since)
	}
	return query
}
