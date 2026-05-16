// practices: [hexagonal-architecture, ddd-bounded-context]
package ports

import (
	"context"
	"errors"
)

// InMemoryCloseout is a CloseoutPort that records requests and echoes
// the requested actions as completed.
type InMemoryCloseout struct {
	Requests  []CloseoutRequest
	Artifacts []string
	Warnings  []string
}

func NewInMemoryCloseout() *InMemoryCloseout {
	return &InMemoryCloseout{Requests: []CloseoutRequest{}, Artifacts: []string{}, Warnings: []string{}}
}

func (c *InMemoryCloseout) Close(ctx context.Context, req CloseoutRequest) (CloseoutResult, error) {
	if err := ctx.Err(); err != nil {
		return CloseoutResult{}, err
	}
	if len(req.Actions) == 0 {
		return CloseoutResult{}, errors.New("ports: CloseoutPort.Close actions required")
	}
	c.Requests = append(c.Requests, cloneCloseoutRequest(req))
	return CloseoutResult{
		Completed: append([]CloseoutAction(nil), req.Actions...),
		Artifacts: append([]string(nil), c.Artifacts...),
		Warnings:  append([]string(nil), c.Warnings...),
	}, nil
}

func cloneCloseoutRequest(req CloseoutRequest) CloseoutRequest {
	metadata := make(map[string]string, len(req.Metadata))
	for key, value := range req.Metadata {
		metadata[key] = value
	}
	req.Metadata = metadata
	req.Actions = append([]CloseoutAction(nil), req.Actions...)
	return req
}

var _ CloseoutPort = (*InMemoryCloseout)(nil)
