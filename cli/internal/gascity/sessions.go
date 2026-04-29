package gascity

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// CreateCity calls POST /v0/city.
func (c *Client) CreateCity(ctx context.Context, req CityCreateRequest) (CityResponse, ResponseMeta, error) {
	var out CityResponse
	meta, err := c.doJSON(ctx, http.MethodPost, "/v0/city", req, &out)
	if err != nil {
		return CityResponse{}, meta, err
	}
	return out, meta, nil
}

// ListCities calls GET /v0/cities.
func (c *Client) ListCities(ctx context.Context) (CityListResponse, ResponseMeta, error) {
	var out CityListResponse
	meta, err := c.doJSON(ctx, http.MethodGet, "/v0/cities", nil, &out)
	if err != nil {
		return CityListResponse{}, meta, err
	}
	return out, meta, nil
}

// GetCity calls GET /v0/city/{cityName}.
func (c *Client) GetCity(ctx context.Context, cityName string) (CityGetResponse, ResponseMeta, error) {
	cityPath, err := cityBasePath(cityName)
	if err != nil {
		return CityGetResponse{}, ResponseMeta{}, err
	}
	var out CityGetResponse
	meta, err := c.doJSON(ctx, http.MethodGet, cityPath, nil, &out)
	if err != nil {
		return CityGetResponse{}, meta, err
	}
	return out, meta, nil
}

// CreateSession calls POST /v0/city/{cityName}/sessions.
func (c *Client) CreateSession(
	ctx context.Context,
	cityName string,
	req SessionCreateRequest,
) (Session, ResponseMeta, error) {
	sessionPath, err := cityPath(cityName, "sessions")
	if err != nil {
		return Session{}, ResponseMeta{}, err
	}
	var out Session
	meta, err := c.doJSON(ctx, http.MethodPost, sessionPath, req, &out)
	if err != nil {
		return Session{}, meta, err
	}
	return out, meta, nil
}

// ListSessions calls GET /v0/city/{cityName}/sessions.
func (c *Client) ListSessions(
	ctx context.Context,
	cityName string,
	params SessionListParams,
) (SessionListResponse, ResponseMeta, error) {
	sessionPath, err := cityPath(cityName, "sessions")
	if err != nil {
		return SessionListResponse{}, ResponseMeta{}, err
	}
	var out SessionListResponse
	meta, err := c.doJSONWithQuery(
		ctx,
		http.MethodGet,
		sessionPath,
		sessionListQuery(params),
		nil,
		&out,
	)
	if err != nil {
		return SessionListResponse{}, meta, err
	}
	return out, meta, nil
}

// GetSession calls GET /v0/city/{cityName}/session/{id}.
func (c *Client) GetSession(
	ctx context.Context,
	cityName string,
	id string,
	opts SessionGetOptions,
) (Session, ResponseMeta, error) {
	sessionPath, err := cityPath(cityName, "session", id)
	if err != nil {
		return Session{}, ResponseMeta{}, err
	}
	query := url.Values{}
	if opts.Peek {
		query.Set("peek", "true")
	}
	var out Session
	meta, err := c.doJSONWithQuery(ctx, http.MethodGet, sessionPath, query, nil, &out)
	if err != nil {
		return Session{}, meta, err
	}
	return out, meta, nil
}

// SubmitSession calls POST /v0/city/{cityName}/session/{id}/submit.
func (c *Client) SubmitSession(
	ctx context.Context,
	cityName string,
	id string,
	req SessionSubmitRequest,
) (SessionSubmitResponse, ResponseMeta, error) {
	sessionPath, err := cityPath(cityName, "session", id, "submit")
	if err != nil {
		return SessionSubmitResponse{}, ResponseMeta{}, err
	}
	var out SessionSubmitResponse
	meta, err := c.doJSON(ctx, http.MethodPost, sessionPath, req, &out)
	if err != nil {
		return SessionSubmitResponse{}, meta, err
	}
	return out, meta, nil
}

// SessionTranscript calls GET /v0/city/{cityName}/session/{id}/transcript.
func (c *Client) SessionTranscript(
	ctx context.Context,
	cityName string,
	id string,
	opts TranscriptOptions,
) (TranscriptResponse, ResponseMeta, error) {
	sessionPath, err := cityPath(cityName, "session", id, "transcript")
	if err != nil {
		return TranscriptResponse{}, ResponseMeta{}, err
	}
	query := url.Values{}
	if opts.Format != "" {
		query.Set("format", opts.Format)
	}
	if opts.Tail != nil {
		if *opts.Tail < 0 {
			return TranscriptResponse{}, ResponseMeta{}, fmt.Errorf("tail must be non-negative")
		}
		query.Set("tail", strconv.Itoa(*opts.Tail))
	}
	if opts.Before != "" {
		query.Set("before", opts.Before)
	}
	var out TranscriptResponse
	meta, err := c.doJSONWithQuery(ctx, http.MethodGet, sessionPath, query, nil, &out)
	if err != nil {
		return TranscriptResponse{}, meta, err
	}
	return out, meta, nil
}

func sessionListQuery(params SessionListParams) url.Values {
	query := url.Values{}
	if params.State != "" {
		query.Set("state", params.State)
	}
	if params.Template != "" {
		query.Set("template", params.Template)
	}
	if params.Cursor != "" {
		query.Set("cursor", params.Cursor)
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Peek {
		query.Set("peek", "true")
	}
	return query
}

func cityBasePath(cityName string) (string, error) {
	return cityPath(cityName)
}

func cityPath(cityName string, elems ...string) (string, error) {
	cityName = strings.TrimSpace(cityName)
	if cityName == "" {
		return "", fmt.Errorf("city name is required")
	}
	parts := []string{"/v0/city", url.PathEscape(cityName)}
	for _, elem := range elems {
		elem = strings.TrimSpace(elem)
		if elem == "" {
			return "", fmt.Errorf("path element is required")
		}
		parts = append(parts, url.PathEscape(elem))
	}
	return strings.Join(parts, "/"), nil
}
