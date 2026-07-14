// Package done contains runtime adapters for the verdict-backed close use case.
package done

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"

	doneapp "github.com/boshu2/agentops/cli/internal/done"
	"github.com/boshu2/agentops/cli/internal/provenancegraph"
	"github.com/boshu2/agentops/cli/internal/trackerexec"
)

const originLedger = "origin/main:" + provenancegraph.LedgerRelativePath

type Repository struct {
	WorkingDirFunc func() (string, error)
	GitOutput      func(context.Context, string, ...string) ([]byte, error)
}

func SystemRepository() Repository {
	return Repository{
		WorkingDirFunc: os.Getwd,
		GitOutput: func(ctx context.Context, cwd string, args ...string) ([]byte, error) {
			command := exec.CommandContext(ctx, "git", append([]string{"-C", cwd}, args...)...)
			return command.Output()
		},
	}
}

func (repository Repository) WorkingDir() (string, error) { return repository.WorkingDirFunc() }

func (repository Repository) ResolveHead(ctx context.Context, cwd string) (string, error) {
	output, err := repository.GitOutput(ctx, cwd, "rev-parse", "HEAD")
	return strings.TrimSpace(string(output)), err
}

func (repository Repository) CommitProvenanceOnly(ctx context.Context, cwd, sha string) bool {
	output, err := repository.GitOutput(ctx, cwd, "diff-tree", "--no-commit-id", "--no-renames", "--name-only", "-z", "-r", sha)
	return err == nil && doneapp.ProvenanceOnlyChangedFiles(string(output))
}

func (repository Repository) OriginEdges(ctx context.Context, cwd string) ([]doneapp.Edge, bool) {
	output, err := repository.GitOutput(ctx, cwd, "show", originLedger)
	if err != nil {
		return nil, false
	}
	edges, err := provenancegraph.DecodeEdges(bytes.NewReader(output))
	if err != nil {
		return nil, false
	}
	return mapEdges(edges), true
}

type Ledger struct {
	ReadEdges func() ([]provenancegraph.Edge, error)
}

func SystemLedger(path string) Ledger {
	return Ledger{ReadEdges: provenancegraph.NewStore(path).Read}
}

func (ledger Ledger) Read(context.Context) ([]doneapp.Edge, error) {
	edges, err := ledger.ReadEdges()
	if err != nil {
		return nil, err
	}
	return mapEdges(edges), nil
}

type Tracker struct {
	Command func(context.Context, ...string) *trackerexec.ResolvedCommand
}

func NewTracker(command func(context.Context, ...string) *trackerexec.ResolvedCommand) Tracker {
	return Tracker{Command: command}
}

func (tracker Tracker) Close(ctx context.Context, id, reason string) (string, error) {
	output, err := tracker.Command(ctx, "close", id, "-r", reason).CombinedOutput()
	return string(output), err
}

func mapEdges(edges []provenancegraph.Edge) []doneapp.Edge {
	result := make([]doneapp.Edge, 0, len(edges))
	for _, edge := range edges {
		result = append(result, doneapp.Edge{FromID: edge.FromID, FromType: edge.FromType, ToID: edge.ToID,
			ToType: edge.ToType, Relation: edge.Relation, EvidenceRef: edge.EvidenceRef})
	}
	return result
}
