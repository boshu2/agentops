package claim

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/boshu2/agentops/cli/internal/claimproof"
)

type Proof struct {
	repoRoot func() (string, error)
}

func NewProof(repoRoot func() (string, error)) Proof {
	return Proof{repoRoot: repoRoot}
}

func (proof Proof) Check(ctx context.Context, base string, changedOnly bool) (claimproof.Report, error) {
	if proof.repoRoot == nil {
		return claimproof.Report{}, errors.New("claim proof root resolver is not configured")
	}
	root, err := proof.repoRoot()
	if err != nil {
		return claimproof.Report{}, err
	}
	return claimproof.Check(ctx, claimproof.Options{
		RepoRoot: root, Base: base, ChangedOnly: changedOnly, Workspace: LocalWorkspace{},
	})
}

type LocalWorkspace struct{}

func (LocalWorkspace) WorkingDirectory() (string, error)    { return os.Getwd() }
func (LocalWorkspace) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (LocalWorkspace) Stat(path string) error {
	_, err := os.Stat(path)
	return err
}
func (LocalWorkspace) IsNotExist(err error) bool { return os.IsNotExist(err) }
func (LocalWorkspace) Git(ctx context.Context, repoRoot string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = repoRoot
	output, err := command.Output()
	return string(output), err
}

var _ claimproof.Workspace = LocalWorkspace{}
