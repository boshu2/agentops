package beads

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/boshu2/agentops/cli/internal/epicstatus"
)

type DirectoryResult struct {
	Path    string
	Source  string
	Tracker string
}

type DirectoryUseCases interface {
	ResolveDirectory(bool) (DirectoryResult, error)
}

type DirectoryService struct {
	Resolver  TrackerResolver
	Inspector LedgerInspector
}

func (service DirectoryService) ResolveDirectory(require bool) (DirectoryResult, error) {
	if service.Resolver == nil || service.Inspector == nil {
		return DirectoryResult{}, fmt.Errorf("beads directory ports are not configured")
	}
	if !service.Resolver.BeadsDirOverride() {
		if resolved, err := service.Resolver.Resolve(); err == nil && resolved.Tracker == TrackerBD {
			if require {
				if reason := LedgerMissing(TrackerBD, service.Inspector.InspectLedger(resolved.LedgerDir)); reason != "" {
					return DirectoryResult{}, fmt.Errorf("beads dir --require: %s (resolved %s for tracker bd via %s); refusing to print a path a bd write could silently fall back from", reason, resolved.LedgerDir, resolved.Source)
				}
			}
			return DirectoryResult{Path: resolved.LedgerDir, Source: resolved.Source, Tracker: TrackerBD}, nil
		}
	}
	resolved, err := service.Resolver.BRLedger()
	if err != nil {
		return DirectoryResult{}, err
	}
	if require {
		if reason := LedgerMissing(TrackerBR, service.Inspector.InspectLedger(resolved.Path)); reason != "" {
			return DirectoryResult{}, fmt.Errorf("beads dir --require: %s (resolved %s via %s); refusing to print a path a br write could silently fall back from", reason, resolved.Path, resolved.Source)
		}
	}
	return DirectoryResult{Path: resolved.Path, Source: resolved.Source, Tracker: TrackerBR}, nil
}

type ResumeOptions struct {
	Agent  string
	Ledger string
}

type ResumeResult struct {
	Event      TransferredEvent
	PriorAgent string
}

type RecoveryUseCases interface {
	StaleClaims(context.Context, float64) ([]StaleEvent, error)
	Resume(context.Context, string, ResumeOptions) (ResumeResult, error)
	EpicStatus(string) (epicstatus.Result, error)
}

type RecoveryService struct {
	StaleSource StaleSource
	Claims      ClaimStore
	Runtime     ResumeRuntime
	Resolver    TrackerResolver
	Reader      LedgerReader
}

func (service RecoveryService) StaleClaims(ctx context.Context, thresholdHours float64) ([]StaleEvent, error) {
	if service.StaleSource == nil || service.Runtime == nil {
		return nil, fmt.Errorf("beads stale-claims ports are not configured")
	}
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	events, err := DetectStale(bounded, service.StaleSource, service.Runtime.Now(), thresholdHours)
	if err != nil {
		return nil, fmt.Errorf("br list: %w", err)
	}
	return events, nil
}

func (service RecoveryService) Resume(ctx context.Context, beadID string, options ResumeOptions) (ResumeResult, error) {
	if service.Claims == nil || service.Runtime == nil {
		return ResumeResult{}, fmt.Errorf("beads resume ports are not configured")
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	prior, err := service.Claims.Show(bounded, beadID)
	if err != nil {
		return ResumeResult{}, fmt.Errorf("fetch prior state: %w", err)
	}
	if prior.Status != "in_progress" {
		return ResumeResult{}, fmt.Errorf("bead %s is %q, not in_progress — resume only handles in_progress claims", beadID, prior.Status)
	}
	now := service.Runtime.Now().UTC()
	agent := options.Agent
	if agent == "" {
		agent = service.Runtime.Actor()
	}
	if agent == "" {
		agent = "ao-beads-resume"
	}
	if err := service.Claims.Claim(bounded, beadID, agent); err != nil {
		return ResumeResult{}, fmt.Errorf("claim transfer: %w", err)
	}
	posterior, err := service.Claims.Show(bounded, beadID)
	if err != nil {
		posterior = StaleBeadRecord{ID: beadID, Status: "in_progress", Assignee: agent, UpdatedAt: now.Format(time.RFC3339)}
	}
	event := BuildTransferredEvent(beadID, agent, prior, posterior, now)
	ledger, err := service.Runtime.ResolveRepoPath(options.Ledger)
	if err != nil {
		return ResumeResult{}, err
	}
	if err := service.Runtime.AppendEvent(ledger, event); err != nil {
		return ResumeResult{}, fmt.Errorf("append ledger (claim already transferred): %w", err)
	}
	priorAgent := prior.Assignee
	if priorAgent == "" {
		priorAgent = "unknown"
	}
	return ResumeResult{Event: event, PriorAgent: priorAgent}, nil
}

func (service RecoveryService) EpicStatus(epic string) (epicstatus.Result, error) {
	if service.Resolver == nil || service.Reader == nil {
		return epicstatus.Result{}, fmt.Errorf("beads epic-status ports are not configured")
	}
	ledger, err := service.Resolver.BRLedger()
	if err != nil {
		return epicstatus.Result{}, err
	}
	path := filepath.Join(ledger.Path, "issues.jsonl")
	raw, err := service.Reader.ReadFile(path)
	if err != nil {
		return epicstatus.Result{}, fmt.Errorf("read ledger %s: %w", path, err)
	}
	records, err := ParseLedger(raw)
	if err != nil {
		return epicstatus.Result{}, fmt.Errorf("parse ledger: %w", err)
	}
	members, present := BuildMembers(epic, records)
	if !present {
		return epicstatus.Result{}, fmt.Errorf("epic %s not found in ledger %s", epic, ledger.Path)
	}
	return epicstatus.Evaluate(epic, members), nil
}

var _ DirectoryUseCases = DirectoryService{}
var _ RecoveryUseCases = RecoveryService{}
