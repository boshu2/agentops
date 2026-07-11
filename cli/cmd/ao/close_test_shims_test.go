package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	closeadapter "github.com/boshu2/agentops/cli/internal/adapters/close"
	closeapp "github.com/boshu2/agentops/cli/internal/close"
	"github.com/boshu2/agentops/cli/internal/trackerresolve"
)

const (
	tickExitCloseRef  = closeapp.ExitRefused
	tickExitNoCommit  = closeapp.ExitPersistence
	tickExitCloseFail = closeapp.ExitTracker
)

func tickClosePort(rt tickRuntime, id, message, evidence string, paths []string) error {
	return runCloseTestShim(rt, closeapp.Request{
		ID: id, Message: message, Evidence: evidence, Paths: paths, Mode: closeapp.ModeEnsure,
	})
}

func tickClose(rt tickRuntime, id, message, evidence string, paths []string) error {
	return runCloseTestShim(rt, closeapp.Request{
		ID: id, Message: message, Evidence: evidence, Paths: paths, Mode: closeapp.ModeStrict,
	})
}

func runCloseTestShim(rt tickRuntime, request closeapp.Request) error {
	env := append(os.Environ(), rt.env...)
	service := newCloseService(closeadapter.StaticRuntime{WorkDir: rt.workDir, Env: env})
	result, err := service.Execute(context.Background(), request)
	if err != nil {
		if failure, ok := err.(*closeapp.Failure); ok && failure.Message != "" {
			fmt.Fprintln(testWriter(rt.stderr), failure.Message)
		}
		return err
	}
	verb := "closed"
	if result.AlreadyClosed {
		verb = "already closed"
	}
	fmt.Fprintf(testWriter(rt.stdout), "%s %s @ %s\n", verb, result.ID, closeapp.ShortRef(result.Ref))
	return nil
}

func testWriter(writer io.Writer) io.Writer {
	if writer == nil {
		return io.Discard
	}
	return writer
}

func tickEvidenceRefusal(rt tickRuntime, _ string, first string) string {
	return closeadapter.EvidenceRefusal(context.Background(), closeapp.Snapshot{
		WorkDir: rt.workDir, Env: append(os.Environ(), rt.env...),
	}, first)
}

func tickFirstEvidenceToken(evidence string) string {
	evidence = strings.TrimSpace(evidence)
	if index := strings.IndexByte(evidence, ' '); index >= 0 {
		return evidence[:index]
	}
	return evidence
}

func tickPublicStagePaths(rt tickRuntime, ledgerDir, _ string, paths []string) []string {
	return closeadapter.PublicStagePaths(rt.workDir, ledgerDir, paths)
}

func tickLedgerShowsClosed(path, id string) bool {
	closed, err := (closeadapter.Repository{}).LedgerStatus(context.Background(), closeapp.Resolution{
		LedgerDir: filepath.Dir(path),
	}, id)
	return err == nil && closed
}

func tickShortSHA(ref string) string { return closeapp.ShortRef(ref) }

func tickLedgerDir(rt tickRuntime) string {
	return trackerresolve.ResolveLedger(rt.workDir, append(os.Environ(), rt.env...), trackerresolve.BR).Path
}

func tickStagePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return path
	}
	return relative
}
