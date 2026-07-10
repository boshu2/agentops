package ports

import (
	"context"
	"fmt"
	"strings"
)

type AgentMailIdentityRequest struct {
	Project string
	Agent   string
	Program string
	Model   string
	Task    string
}

func (r AgentMailIdentityRequest) Validate() error {
	if strings.TrimSpace(r.Project) == "" || strings.TrimSpace(r.Agent) == "" {
		return fmt.Errorf("project and agent are required")
	}
	if strings.TrimSpace(r.Program) == "" || strings.TrimSpace(r.Model) == "" {
		return fmt.Errorf("program and model are required")
	}
	return nil
}

type AgentMailIdentity struct{ Project, Agent string }

type AgentMailReservationRequest struct {
	Project    string
	Agent      string
	Paths      []string
	TTLSeconds int
	Exclusive  bool
	Reason     string
}

func (r AgentMailReservationRequest) Validate() error {
	if strings.TrimSpace(r.Project) == "" || strings.TrimSpace(r.Agent) == "" {
		return fmt.Errorf("project and agent are required")
	}
	if len(r.Paths) == 0 {
		return fmt.Errorf("at least one reservation path is required")
	}
	for _, path := range r.Paths {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("reservation paths must be nonempty")
		}
	}
	if r.TTLSeconds <= 0 {
		return fmt.Errorf("ttl_seconds must be positive")
	}
	return nil
}

type AgentMailReservation struct{ IDs []int }

type AgentMailMessageRequest struct {
	Project     string
	Sender      string
	Recipients  []string
	Subject     string
	Body        string
	ThreadID    string
	AckRequired bool
}

func (r AgentMailMessageRequest) Validate() error {
	if strings.TrimSpace(r.Project) == "" || strings.TrimSpace(r.Sender) == "" {
		return fmt.Errorf("project and sender are required")
	}
	if len(r.Recipients) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	if strings.TrimSpace(r.Subject) == "" || strings.TrimSpace(r.Body) == "" {
		return fmt.Errorf("subject and body are required")
	}
	return nil
}

type AgentMailMessage struct{ ID int }

type AgentMailPort interface {
	EnsureIdentity(context.Context, AgentMailIdentityRequest) (AgentMailIdentity, error)
	Reserve(context.Context, AgentMailReservationRequest) (AgentMailReservation, error)
	Send(context.Context, AgentMailMessageRequest) (AgentMailMessage, error)
	Acknowledge(ctx context.Context, project, agent string, messageID int) error
	Release(ctx context.Context, project, agent string, reservationIDs []int) error
}
