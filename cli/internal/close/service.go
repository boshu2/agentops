// Package close owns the close transaction policy independently of Cobra and
// concrete tracker, filesystem, and git effects.
package close

import (
	"context"
	"fmt"
)

const (
	BackendBR = "br"
	BackendBD = "bd"

	ExitRefused     = 3
	ExitPersistence = 4
	ExitTracker     = 10
)

type Mode uint8

const (
	ModeEnsure Mode = iota
	ModeStrict
)

type Snapshot struct {
	WorkDir string
	Env     []string
}

type Resolution struct {
	Backend   string
	Binary    string
	LedgerDir string
	RepoRoot  string
	WorkDir   string
	ChildEnv  []string
}

type Request struct {
	ID       string
	Message  string
	Evidence string
	Paths    []string
	Mode     Mode
}

type Result struct {
	ID            string
	Ref           string
	AlreadyClosed bool
}

type Runtime interface {
	Snapshot() (Snapshot, error)
}

type Tracker interface {
	Resolve(context.Context, Snapshot) (Resolution, error)
	Status(context.Context, Resolution, string) (bool, error)
	Close(context.Context, Resolution, string, string) error
	Sync(context.Context, Resolution) error
}

type Repository interface {
	Preflight(context.Context, Snapshot, Resolution, string, []string) error
	LedgerStatus(context.Context, Resolution, string) (bool, error)
	CommitLedger(context.Context, Snapshot, Resolution, string) (string, error)
	CommitPublic(context.Context, Snapshot, Resolution, string, []string) (string, error)
}

type Failure struct {
	Code    int
	Message string
	Cause   error
}

func (failure *Failure) Error() string {
	if failure == nil {
		return ""
	}
	if failure.Message != "" {
		return failure.Message
	}
	if failure.Cause != nil {
		return failure.Cause.Error()
	}
	return "close failed"
}

func (failure *Failure) Unwrap() error { return failure.Cause }
func (failure *Failure) ExitCode() int { return failure.Code }

type Service struct {
	runtime    Runtime
	tracker    Tracker
	repository Repository
}

func NewService(runtime Runtime, tracker Tracker, repository Repository) *Service {
	return &Service{runtime: runtime, tracker: tracker, repository: repository}
}

func (service *Service) Execute(ctx context.Context, request Request) (Result, error) {
	snapshot, err := service.runtime.Snapshot()
	if err != nil {
		return Result{}, &Failure{Code: 1, Message: err.Error(), Cause: err}
	}
	resolution, err := service.tracker.Resolve(ctx, snapshot)
	if err != nil {
		return Result{}, &Failure{Code: 1, Message: err.Error(), Cause: err}
	}
	if err := service.repository.Preflight(ctx, snapshot, resolution, request.Evidence, request.Paths); err != nil {
		return Result{}, &Failure{
			Code: ExitRefused, Message: fmt.Sprintf("REFUSED close %s: %s", request.ID, err), Cause: err,
		}
	}

	alreadyClosed := false
	if request.Mode == ModeEnsure {
		closed, statusErr := service.closed(ctx, resolution, request.ID)
		if statusErr != nil {
			return Result{}, &Failure{
				Code: ExitTracker, Message: fmt.Sprintf("FAILED close %s: %s status cannot be determined", request.ID, resolution.Backend), Cause: statusErr,
			}
		}
		alreadyClosed = closed
		if !closed {
			if err := service.close(ctx, resolution, request); err != nil {
				return Result{}, err
			}
		}
	} else if err := service.close(ctx, resolution, request); err != nil {
		return Result{}, err
	}

	ref := "none"
	if resolution.Backend == BackendBR {
		if err := service.tracker.Sync(ctx, resolution); err != nil {
			return Result{}, persistenceFailure("tracker sync failed", err)
		}
		closed, statusErr := service.repository.LedgerStatus(ctx, resolution, request.ID)
		if statusErr != nil || !closed {
			return Result{}, &Failure{
				Code: ExitTracker, Message: fmt.Sprintf("FAILED close %s: %s ledger does not prove the bead is closed", request.ID, resolution.Backend), Cause: statusErr,
			}
		}
		ref, err = service.repository.CommitLedger(ctx, snapshot, resolution, request.Message)
		if err != nil {
			return Result{}, persistenceFailure("ledger persistence failed", err)
		}
	} else {
		closed, statusErr := service.tracker.Status(ctx, resolution, request.ID)
		if statusErr != nil || !closed {
			return Result{}, &Failure{
				Code: ExitTracker, Message: fmt.Sprintf("FAILED close %s: %s does not prove the issue is closed", request.ID, resolution.Backend), Cause: statusErr,
			}
		}
	}

	publicRef, err := service.repository.CommitPublic(ctx, snapshot, resolution, request.Message, request.Paths)
	if err != nil {
		return Result{}, persistenceFailure("public persistence failed", err)
	}
	if resolution.Backend == BackendBD {
		ref = publicRef
	}
	if ref == "" {
		ref = "none"
	}
	return Result{ID: request.ID, Ref: ref, AlreadyClosed: alreadyClosed}, nil
}

func (service *Service) closed(ctx context.Context, resolution Resolution, id string) (bool, error) {
	if resolution.Backend == BackendBR {
		return service.repository.LedgerStatus(ctx, resolution, id)
	}
	return service.tracker.Status(ctx, resolution, id)
}

func (service *Service) close(ctx context.Context, resolution Resolution, request Request) error {
	if err := service.tracker.Close(ctx, resolution, request.ID, "evidence: "+request.Evidence); err != nil {
		return &Failure{
			Code: ExitTracker, Message: fmt.Sprintf("FAILED close %s: %s close failed or skipped; no files staged", request.ID, resolution.Backend), Cause: err,
		}
	}
	return nil
}

func persistenceFailure(message string, cause error) error {
	return &Failure{Code: ExitPersistence, Message: message + ": " + cause.Error(), Cause: cause}
}

func ShortRef(ref string) string {
	if len(ref) <= 7 {
		return ref
	}
	return ref[:7]
}
