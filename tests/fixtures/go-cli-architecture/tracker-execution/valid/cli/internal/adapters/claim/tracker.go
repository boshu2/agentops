package claim

import (
	"context"
	"os/exec"
)

type resolution struct {
	Binary   string
	WorkDir  string
	ChildEnv []string
}

func launch(ctx context.Context, resolved resolution) error {
	command := exec.CommandContext(ctx, resolved.Binary, "ready")
	command.Dir = resolved.WorkDir
	command.Env = resolved.ChildEnv
	return command.Run()
}
