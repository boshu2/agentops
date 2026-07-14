package claim

import (
	"context"
	"os/exec"
)

type resolution struct {
	Binary, WorkDir string
	ChildEnv        []string
}

func launch(ctx context.Context, resolved, wrong resolution) error {
	command := exec.CommandContext(ctx, resolved.Binary, "ready")
	command.Dir = wrong.WorkDir
	command.Env = wrong.ChildEnv
	return command.Run()
}
