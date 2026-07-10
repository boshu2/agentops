package agentmail_cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/boshu2/agentops/cli/internal/ports"
)

type Runner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type Adapter struct {
	runner                  Runner
	mu                      sync.Mutex
	discovered              bool
	reserveSupportsJSONFlag bool
}

func New(runner Runner) *Adapter { return &Adapter{runner: runner} }

func (a *Adapter) discover(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.discovered {
		return nil
	}
	raw, err := a.runner.Run(ctx, "am", "capabilities", "--json")
	if err != nil {
		return fmt.Errorf("discover Agent Mail capabilities: %w", err)
	}
	var capability struct {
		SchemaVersion string `json:"schema_version"`
		Commands      []struct {
			Name             string `json:"name"`
			SupportsJSONFlag bool   `json:"supports_json_flag"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(raw, &capability); err != nil {
		return fmt.Errorf("decode Agent Mail capabilities: %w", err)
	}
	if capability.SchemaVersion != "am.capabilities.v1" {
		return fmt.Errorf("unsupported Agent Mail capability schema %q", capability.SchemaVersion)
	}
	for _, command := range capability.Commands {
		if command.Name == "file_reservations reserve" {
			a.reserveSupportsJSONFlag = command.SupportsJSONFlag
			break
		}
	}
	a.discovered = true
	return nil
}

func (a *Adapter) EnsureIdentity(ctx context.Context, req ports.AgentMailIdentityRequest) (ports.AgentMailIdentity, error) {
	if err := req.Validate(); err != nil {
		return ports.AgentMailIdentity{}, err
	}
	if err := a.discover(ctx); err != nil {
		return ports.AgentMailIdentity{}, err
	}
	args := []string{"agents", "register", "--project", req.Project, "--program", req.Program, "--model", req.Model, "--name", req.Agent}
	if req.Task != "" {
		args = append(args, "--task", req.Task)
	}
	args = append(args, "--json")
	raw, err := a.runner.Run(ctx, "am", args...)
	if err != nil {
		return ports.AgentMailIdentity{}, err
	}
	var response struct {
		Agent struct {
			Name string `json:"name"`
		} `json:"agent"`
		Project struct {
			HumanKey string `json:"human_key"`
		} `json:"project"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return ports.AgentMailIdentity{}, err
	}
	return ports.AgentMailIdentity{Project: response.Project.HumanKey, Agent: response.Agent.Name}, nil
}

func (a *Adapter) Reserve(ctx context.Context, req ports.AgentMailReservationRequest) (ports.AgentMailReservation, error) {
	if err := req.Validate(); err != nil {
		return ports.AgentMailReservation{}, err
	}
	if err := a.discover(ctx); err != nil {
		return ports.AgentMailReservation{}, err
	}
	args := []string{"file_reservations", "reserve", req.Project, req.Agent}
	args = append(args, req.Paths...)
	args = append(args, "--ttl", strconv.Itoa(req.TTLSeconds))
	if req.Exclusive {
		args = append(args, "--exclusive")
	} else {
		args = append(args, "--shared")
	}
	if req.Reason != "" {
		args = append(args, "--reason", req.Reason)
	}
	// Agent Mail 0.3.10 emits JSON for reserve by default and advertises
	// supports_json_flag=false; passing --json to that surface is a hard CLI
	// error. Request the flag only when capability discovery explicitly allows it.
	if a.reserveSupportsJSONFlag {
		args = append(args, "--json")
	}
	raw, err := a.runner.Run(ctx, "am", args...)
	if err != nil {
		return ports.AgentMailReservation{}, err
	}
	var response struct {
		IDs []int `json:"reservation_ids"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return ports.AgentMailReservation{}, err
	}
	return ports.AgentMailReservation{IDs: response.IDs}, nil
}

func (a *Adapter) Send(ctx context.Context, req ports.AgentMailMessageRequest) (ports.AgentMailMessage, error) {
	if err := req.Validate(); err != nil {
		return ports.AgentMailMessage{}, err
	}
	if err := a.discover(ctx); err != nil {
		return ports.AgentMailMessage{}, err
	}
	args := []string{"mail", "send", "--project", req.Project, "--from", req.Sender, "--to", strings.Join(req.Recipients, ","), "--subject", req.Subject, "--body", req.Body}
	if req.ThreadID != "" {
		args = append(args, "--thread-id", req.ThreadID)
	}
	if req.AckRequired {
		args = append(args, "--ack-required")
	}
	args = append(args, "--json")
	raw, err := a.runner.Run(ctx, "am", args...)
	if err != nil {
		return ports.AgentMailMessage{}, err
	}
	var response struct {
		Message struct {
			ID int `json:"id"`
		} `json:"message"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return ports.AgentMailMessage{}, err
	}
	return ports.AgentMailMessage{ID: response.Message.ID}, nil
}

func (a *Adapter) Acknowledge(ctx context.Context, project, agent string, messageID int) error {
	if err := a.discover(ctx); err != nil {
		return err
	}
	if project == "" || agent == "" || messageID <= 0 {
		return fmt.Errorf("project, agent, and positive message id are required")
	}
	_, err := a.runner.Run(ctx, "am", "mail", "ack", "--project", project, "--agent", agent, strconv.Itoa(messageID))
	return err
}

func (a *Adapter) Release(ctx context.Context, project, agent string, reservationIDs []int) error {
	if err := a.discover(ctx); err != nil {
		return err
	}
	if project == "" || agent == "" || len(reservationIDs) == 0 {
		return fmt.Errorf("project, agent, and reservation ids are required")
	}
	ids := make([]string, len(reservationIDs))
	for i, id := range reservationIDs {
		ids[i] = strconv.Itoa(id)
	}
	_, err := a.runner.Run(ctx, "am", "file_reservations", "release", project, agent, "--ids", strings.Join(ids, ","))
	return err
}
