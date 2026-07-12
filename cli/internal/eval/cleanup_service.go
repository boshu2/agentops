package eval

import (
	"context"
	"fmt"
	"time"

	"github.com/boshu2/agentops/cli/internal/evalsubstrate"
)

type CleanupRuntime interface {
	Root() string
	ListRunIDs(string) ([]string, error)
	LoadManifest(string, string) (*evalsubstrate.Manifest, error)
	Transition(string, string, evalsubstrate.RunStatus, string, time.Time) error
	DeleteRun(string, string) error
	SweepTempFiles(string, int64) ([]string, error)
	Now() time.Time
}

type CleanupService struct{ Runtime CleanupRuntime }

type CleanupRequest struct {
	Delete, TmpFiles, DryRun bool
	TmpAgeSeconds            int64
}

type CleanupReport struct {
	TransitionsAborted int      `json:"transitions_to_aborted"`
	TransitionsFailed  int      `json:"transitions_to_failed"`
	RunsDeleted        int      `json:"runs_deleted"`
	TmpFilesSwept      int      `json:"tmp_files_swept"`
	Touched            []string `json:"touched"`
}

func (service CleanupService) Execute(_ context.Context, request CleanupRequest) (CleanupReport, error) {
	root := service.Runtime.Root()
	report := CleanupReport{Touched: []string{}}
	ids, err := service.Runtime.ListRunIDs(root)
	if err != nil {
		return report, fmt.Errorf("eval cleanup: read runs/: %w", err)
	}
	for _, id := range ids {
		manifest, err := service.Runtime.LoadManifest(root, id)
		if err != nil {
			report.Touched = append(report.Touched, id+":unreadable")
			continue
		}
		age := service.Runtime.Now().Sub(time.UnixMilli(manifest.StartedAtUnixMs).UTC())
		switch {
		case manifest.Status == evalsubstrate.StatusPending && age >= time.Minute:
			if err := service.Runtime.Transition(root, id, evalsubstrate.StatusAborted, "never_started", service.Runtime.Now()); err != nil {
				return report, err
			}
			report.TransitionsAborted++
			report.Touched = append(report.Touched, id+":pending->aborted")
		case manifest.Status == evalsubstrate.StatusRunning && age >= 5*time.Minute:
			if err := service.Runtime.Transition(root, id, evalsubstrate.StatusFailed, "orphaned_process", service.Runtime.Now()); err != nil {
				return report, err
			}
			report.TransitionsFailed++
			report.Touched = append(report.Touched, id+":running->failed")
		}
	}
	if request.Delete {
		if err := service.deleteRuns(root, ids, request.DryRun, &report); err != nil {
			return report, err
		}
	}
	if request.TmpFiles {
		if request.DryRun {
			report.Touched = append(report.Touched, "tmp-files: dry-run preview not implemented (sweep is a write op)")
		} else {
			swept, err := service.Runtime.SweepTempFiles(root, request.TmpAgeSeconds)
			if err != nil {
				return report, fmt.Errorf("eval cleanup: sweep tmp: %w", err)
			}
			report.TmpFilesSwept = len(swept)
			for _, path := range swept {
				report.Touched = append(report.Touched, "tmp:"+path)
			}
		}
	}
	return report, nil
}

func (service CleanupService) deleteRuns(root string, ids []string, dryRun bool, report *CleanupReport) error {
	for _, id := range ids {
		manifest, err := service.Runtime.LoadManifest(root, id)
		if err != nil {
			continue
		}
		if manifest.Status != evalsubstrate.StatusFailed && manifest.Status != evalsubstrate.StatusAborted {
			continue
		}
		if dryRun {
			report.Touched = append(report.Touched, id+":would-delete")
			continue
		}
		if err := service.Runtime.DeleteRun(root, id); err != nil {
			return fmt.Errorf("eval cleanup: remove %q: %w", id, err)
		}
		report.RunsDeleted++
		report.Touched = append(report.Touched, id+":deleted")
	}
	return nil
}
