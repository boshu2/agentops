// Package claim owns application policy for claiming tracker work, binding
// evidence, and checking changed public claims.
package claim

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/boshu2/agentops/cli/internal/claimproof"
	"github.com/boshu2/agentops/cli/internal/ports"
)

type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type TrackerClaimer interface {
	Claim(context.Context, string, Streams) error
}

type ProofChecker interface {
	Check(context.Context, string, bool) (claimproof.Report, error)
}

type BindRequest struct {
	Claim    string
	Path     string
	Level    string
	Anchors  []string
	AuthorID string
	JudgeID  string
}

type Service struct {
	tracker TrackerClaimer
	binder  ports.ClaimEvidenceBinderPort
	checker ProofChecker
}

func NewService(tracker TrackerClaimer, binder ports.ClaimEvidenceBinderPort, checker ProofChecker) Service {
	return Service{tracker: tracker, binder: binder, checker: checker}
}

func (service Service) Claim(ctx context.Context, id string, streams Streams) error {
	if service.tracker == nil {
		return errors.New("claim tracker is not configured")
	}
	return service.tracker.Claim(ctx, id, streams)
}

func (service Service) Bind(ctx context.Context, request BindRequest) error {
	if request.Claim == "" {
		return errors.New("claim bind: --claim required\n  Example: ao claim bind --claim AOP-CLAIM-1 --path internal/foo.go")
	}
	if request.Path == "" {
		return errors.New("claim bind: --path required (evidence file, relative to repo root)\n  Example: ao claim bind --claim AOP-CLAIM-1 --path internal/foo.go")
	}
	level := strings.ToUpper(request.Level)
	switch level {
	case "", "PG1", "PG2", "PG3", "PG4":
	default:
		return fmt.Errorf("claim bind: invalid --level %q (want PG1|PG2|PG3|PG4)", request.Level)
	}
	if service.binder == nil {
		return errors.New("claim bind: evidence binder is not configured")
	}
	if err := service.binder.Bind(ctx, ports.EvidenceBinding{
		Claim: ports.ClaimID(request.Claim), Path: request.Path,
		Level: ports.EvidenceLevel(level), Anchors: request.Anchors,
		AuthorID: request.AuthorID, JudgeID: request.JudgeID,
	}); err != nil {
		return fmt.Errorf("claim bind: %w", err)
	}
	return nil
}

func (service Service) List(ctx context.Context) ([]ports.EvidenceBinding, error) {
	if service.binder == nil {
		return nil, errors.New("claim list: evidence binder is not configured")
	}
	bindings, err := service.binder.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("claim list: %w", err)
	}
	return bindings, nil
}

func (service Service) Check(ctx context.Context, base string, changedOnly bool) (claimproof.Report, error) {
	if !changedOnly {
		return claimproof.Report{}, errors.New("claim check: --changed is required for this read-only MVP")
	}
	if service.checker == nil {
		return claimproof.Report{}, errors.New("claim check: proof checker is not configured")
	}
	report, err := service.checker.Check(ctx, base, changedOnly)
	if err != nil {
		return claimproof.Report{}, fmt.Errorf("claim check: %w", err)
	}
	return report, nil
}
