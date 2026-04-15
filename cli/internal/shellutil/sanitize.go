// Package shellutil provides helpers for invoking bash subprocesses with
// a sanitized environment so user shell aliases, profile scripts, and rcfiles
// cannot leak into worker subprocesses spawned by AgentOps.
//
// The motivating scenario: a user has `alias git='hub'` or `alias grep='rg'`
// in ~/.bashrc, or sets BASH_ENV to a file that runs `shopt -s expand_aliases`
// followed by alias definitions. When AgentOps invokes `bash -c "<user check>"`
// for a goal check or worker script, those aliases can silently change the
// meaning of commands inside the script.
//
// SanitizedBashCommand returns an exec.Cmd that runs bash with --noprofile and
// --norc (skipping /etc/profile, ~/.profile, ~/.bash_profile, ~/.bashrc) and an
// environment with BASH_ENV and ENV stripped (which would otherwise be sourced
// even by non-interactive bash invocations).
package shellutil

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// SanitizedBashCommand returns an *exec.Cmd configured to run `script` under
// bash with profile/rcfile loading disabled and BASH_ENV/ENV stripped from the
// inherited environment.
//
// Callers may further customize the returned Cmd (Dir, Stdout, Stderr, etc.)
// before running it. Callers MUST NOT overwrite Env wholesale; if additional
// env vars are needed, append to cmd.Env (which is already populated with the
// sanitized parent environment).
//
// If ctx is nil, exec.Command is used instead of exec.CommandContext.
func SanitizedBashCommand(ctx context.Context, script string) *exec.Cmd {
	args := []string{"--noprofile", "--norc", "-c", script}
	var cmd *exec.Cmd
	if ctx == nil {
		cmd = exec.Command("bash", args...)
	} else {
		cmd = exec.CommandContext(ctx, "bash", args...)
	}
	cmd.Env = SanitizedEnv(os.Environ())
	return cmd
}

// SanitizedEnv returns a copy of env with entries that could trigger rcfile
// loading or alias expansion removed. Specifically it strips BASH_ENV and ENV.
//
// Exported for testing and for callers that want to compose their own exec.Cmd.
func SanitizedEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, entry := range env {
		if strings.HasPrefix(entry, "BASH_ENV=") || strings.HasPrefix(entry, "ENV=") {
			continue
		}
		out = append(out, entry)
	}
	return out
}
