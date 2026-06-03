package background

import (
	"context"
	"fmt"
	"strings"
)

const (
	DefaultAssignmentSession = "agentops-bg"
	defaultAssignmentTTL     = "2h"
)

// AssignmentRequest describes one background-agent assignment and the file
// reservation manifest that must be secured before the message is sent.
type AssignmentRequest struct {
	Bead       string   `json:"bead"`
	To         []string `json:"to"`
	Branch     string   `json:"branch,omitempty"`
	Files      []string `json:"files"`
	Skills     []string `json:"skills,omitempty"`
	Validation string   `json:"validation,omitempty"`
	Session    string   `json:"session,omitempty"`
	Topic      string   `json:"topic,omitempty"`
	DryRun     bool     `json:"dry_run,omitempty"`
	Message    string   `json:"message,omitempty"`
}

type AssignmentReservationEvidence struct {
	Required bool     `json:"required"`
	Granted  bool     `json:"granted,omitempty"`
	Paths    []string `json:"paths,omitempty"`
	Raw      string   `json:"raw,omitempty"`
}

type AssignmentMessageEvidence struct {
	MessageID string `json:"message_id,omitempty"`
	ThreadID  string `json:"thread_id,omitempty"`
	Subject   string `json:"subject,omitempty"`
	Raw       string `json:"raw,omitempty"`
}

type AssignmentCopyPasteEvidence struct {
	Message        string `json:"message,omitempty"`
	ReserveCommand string `json:"reserve_command,omitempty"`
	SendCommand    string `json:"send_command,omitempty"`
}

type AssignmentEvidence struct {
	Bead        string                        `json:"bead"`
	Topic       string                        `json:"topic"`
	To          []string                      `json:"to"`
	Files       []string                      `json:"files"`
	Session     string                        `json:"session,omitempty"`
	Transport   string                        `json:"transport"`
	DryRun      bool                          `json:"dry_run"`
	Sent        bool                          `json:"sent"`
	Reservation AssignmentReservationEvidence `json:"reservation"`
	Message     *AssignmentMessageEvidence    `json:"message,omitempty"`
	CopyPaste   *AssignmentCopyPasteEvidence  `json:"copy_paste,omitempty"`
}

type AssignmentTransport interface {
	Name() string
	ReserveFiles(context.Context, AssignmentRequest) (AssignmentReservationEvidence, error)
	SendMessage(context.Context, AssignmentRequest) (AssignmentMessageEvidence, error)
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type NTMAssignmentTransport struct {
	runner CommandRunner
	ttl    string
}

func NewNTMAssignmentTransport(runner CommandRunner, ttl string) *NTMAssignmentTransport {
	if strings.TrimSpace(ttl) == "" {
		ttl = defaultAssignmentTTL
	}
	return &NTMAssignmentTransport{
		runner: runner,
		ttl:    ttl,
	}
}

func (t *NTMAssignmentTransport) Name() string { return "ntm-agent-mail" }

func (t *NTMAssignmentTransport) ReserveFiles(ctx context.Context, req AssignmentRequest) (AssignmentReservationEvidence, error) {
	req = normalizeAssignmentRequest(req)
	if t.runner == nil {
		return AssignmentReservationEvidence{}, fmt.Errorf("ntm command runner is required")
	}
	args := []string{"lock", req.Session}
	args = append(args, req.Files...)
	args = append(args, "--reason", "Assignment "+req.Bead, "--ttl", t.ttl, "--json")
	raw, err := t.runner.Run(ctx, "ntm", args...)
	if err != nil {
		return AssignmentReservationEvidence{}, commandError("ntm lock", raw, err)
	}
	return AssignmentReservationEvidence{
		Required: true,
		Granted:  true,
		Paths:    req.Files,
		Raw:      strings.TrimSpace(string(raw)),
	}, nil
}

func (t *NTMAssignmentTransport) SendMessage(ctx context.Context, req AssignmentRequest) (AssignmentMessageEvidence, error) {
	req = normalizeAssignmentRequest(req)
	if t.runner == nil {
		return AssignmentMessageEvidence{}, fmt.Errorf("ntm command runner is required")
	}
	subject := "Assignment: " + req.Bead
	args := []string{"mail", "send", req.Session}
	for _, recipient := range req.To {
		args = append(args, "--to", recipient)
	}
	args = append(args, "--subject", subject, "--thread", req.Topic, "--json", assignmentMessage(req))
	raw, err := t.runner.Run(ctx, "ntm", args...)
	if err != nil {
		return AssignmentMessageEvidence{}, commandError("ntm mail send", raw, err)
	}
	return AssignmentMessageEvidence{
		ThreadID: req.Topic,
		Subject:  subject,
		Raw:      strings.TrimSpace(string(raw)),
	}, nil
}

func AssignBackgroundAgent(ctx context.Context, req AssignmentRequest, transport AssignmentTransport) (AssignmentEvidence, error) {
	req = normalizeAssignmentRequest(req)
	if err := validateAssignmentRequest(req); err != nil {
		return AssignmentEvidence{}, err
	}
	evidence := AssignmentEvidence{
		Bead:      req.Bead,
		Topic:     req.Topic,
		To:        req.To,
		Files:     req.Files,
		Session:   req.Session,
		DryRun:    req.DryRun,
		Sent:      false,
		Transport: "copy-paste",
		Reservation: AssignmentReservationEvidence{
			Required: false,
			Paths:    req.Files,
		},
		CopyPaste: &AssignmentCopyPasteEvidence{
			Message:        assignmentMessage(req),
			ReserveCommand: ntmReserveCommand(req, defaultAssignmentTTL),
			SendCommand:    ntmSendCommand(req),
		},
	}
	if req.DryRun || transport == nil {
		return evidence, nil
	}
	evidence.Transport = transport.Name()
	evidence.CopyPaste = nil
	reservation, err := transport.ReserveFiles(ctx, req)
	if err != nil {
		return AssignmentEvidence{}, fmt.Errorf("reserve assignment files: %w", err)
	}
	evidence.Reservation = reservation
	message, err := transport.SendMessage(ctx, req)
	if err != nil {
		return AssignmentEvidence{}, fmt.Errorf("send assignment message: %w", err)
	}
	evidence.Message = &message
	evidence.Sent = true
	return evidence, nil
}

func BuildAssignmentMessage(req AssignmentRequest) string {
	return assignmentMessage(normalizeAssignmentRequest(req))
}

func normalizeAssignmentRequest(req AssignmentRequest) AssignmentRequest {
	req.Bead = strings.TrimSpace(req.Bead)
	req.Branch = strings.TrimSpace(req.Branch)
	req.Validation = strings.TrimSpace(req.Validation)
	req.Session = strings.TrimSpace(req.Session)
	req.Topic = strings.TrimSpace(req.Topic)
	req.Message = strings.TrimSpace(req.Message)
	req.To = compactStrings(req.To)
	req.Files = compactStrings(req.Files)
	req.Skills = compactStrings(req.Skills)
	if req.Session == "" {
		req.Session = DefaultAssignmentSession
	}
	if req.Topic == "" {
		req.Topic = req.Bead
	}
	return req
}

func validateAssignmentRequest(req AssignmentRequest) error {
	if req.Bead == "" {
		return fmt.Errorf("bead is required")
	}
	if len(req.To) == 0 {
		return fmt.Errorf("recipient is required")
	}
	if len(req.Files) == 0 {
		return fmt.Errorf("files are required so assignment reservations are explicit")
	}
	return nil
}

func assignmentMessage(req AssignmentRequest) string {
	if req.Message != "" {
		return req.Message
	}
	branch := req.Branch
	if branch == "" {
		branch = "cursor/<bead>-<slug>-<session>"
	}
	skills := req.Skills
	if len(skills) == 0 {
		skills = []string{"research", "implement", "validation", "provenance"}
	}
	validation := req.Validation
	if validation == "" {
		validation = "run the smallest relevant tests plus `scripts/pre-push-gate.sh --fast` when code/docs changed"
	}
	var sb strings.Builder
	sb.WriteString("BACKGROUND AGENT ASSIGNMENT\n\n")
	sb.WriteString("Bead: ")
	sb.WriteString(req.Bead)
	sb.WriteString("\n")
	sb.WriteString("To: ")
	sb.WriteString(strings.Join(req.To, ", "))
	sb.WriteString("\n")
	sb.WriteString("Branch/worktree: ")
	sb.WriteString(branch)
	sb.WriteString("\n")
	sb.WriteString("Skills: ")
	sb.WriteString(strings.Join(skills, ", "))
	sb.WriteString("\n")
	sb.WriteString("Validation: ")
	sb.WriteString(validation)
	sb.WriteString("\n\n")
	sb.WriteString("Working-directory note: file paths are repo-root relative. Go CLI validation commands that reference `./cmd/ao` should run from `cli/` (for example `cd cli && go test ./cmd/ao -run Agent`).\n\n")
	sb.WriteString("Before editing:\n")
	sb.WriteString("1. Confirm this assignment in the mcp-agent-mail thread.\n")
	sb.WriteString("2. Reserve these file paths/globs through mcp-agent-mail:\n")
	for _, file := range req.Files {
		sb.WriteString("   - ")
		sb.WriteString(file)
		sb.WriteString("\n")
	}
	sb.WriteString("3. Create/use one worktree for this bead; do not edit the shared checkout.\n")
	sb.WriteString("4. Use skills as the execution contract; do not run deprecated `ao rpi` / `ao evolve` wrappers.\n\n")
	sb.WriteString("Closeout:\n")
	sb.WriteString("- Reply with branch, commits, tests, provenance/evidence paths, and any scope escapes.\n")
	sb.WriteString("- Do not self-merge.\n")
	return sb.String()
}

func ntmReserveCommand(req AssignmentRequest, ttl string) string {
	if strings.TrimSpace(ttl) == "" {
		ttl = defaultAssignmentTTL
	}
	parts := []string{"ntm", "lock", req.Session}
	parts = append(parts, req.Files...)
	parts = append(parts, "--reason", "Assignment "+req.Bead, "--ttl", ttl, "--json")
	return shellCommand(parts)
}

func ntmSendCommand(req AssignmentRequest) string {
	parts := []string{"ntm", "mail", "send", req.Session}
	for _, recipient := range req.To {
		parts = append(parts, "--to", recipient)
	}
	parts = append(parts, "--subject", "Assignment: "+req.Bead, "--thread", req.Topic, "--json", assignmentMessage(req))
	return shellCommand(parts)
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func commandError(label string, raw []byte, err error) error {
	msg := strings.TrimSpace(string(raw))
	if msg != "" {
		return fmt.Errorf("%s: %s: %w", label, msg, err)
	}
	return fmt.Errorf("%s: %w", label, err)
}

func shellCommand(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		return !(r == '-' || r == '_' || r == '/' || r == '.' || r == ':' || r == ',' ||
			(r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'))
	}) == -1 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
