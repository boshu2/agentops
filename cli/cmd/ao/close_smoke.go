package main

import (
	"context"
	"os"
	"path/filepath"

	closeadapter "github.com/boshu2/agentops/cli/internal/adapters/close"
	closeapp "github.com/boshu2/agentops/cli/internal/close"
)

func runCloseSmokeFailure(rt tickRuntime, tmp string) bool {
	fakebin := filepath.Join(tmp, "fakebin")
	_ = os.MkdirAll(fakebin, 0o755)
	fakeBR := `#!/usr/bin/env bash
if [ "${1:-}" = "close" ]; then echo "stubbed br close failure" >&2; exit 42; fi
if [ "${1:-}" = "sync" ] || [ "${1:-}" = "update" ]; then exit 0; fi
echo "unexpected br call: $*" >&2; exit 43
`
	fakeGitLog := filepath.Join(tmp, "fake-git.log")
	fakeGit := `#!/usr/bin/env bash
case "${1:-}" in
  rev-parse) echo fakehead ;;
  add) : ;;
  commit) printf 'commit\n' >> "${TICK_SMOKE_FAKE_GIT_LOG:?}" ;;
  *) : ;;
esac
`
	_ = os.WriteFile(filepath.Join(fakebin, "br"), []byte(fakeBR), 0o755)
	_ = os.WriteFile(filepath.Join(fakebin, "git"), []byte(fakeGit), 0o755)
	evidence := filepath.Join(tmp, "close-evidence.md")
	_ = os.WriteFile(evidence, []byte("evidence"), 0o644)
	env := append(os.Environ(), rt.env...)
	env = append(env,
		"PATH="+fakebin+string(os.PathListSeparator)+envValue(env, "PATH"),
		"TICK_SMOKE_FAKE_GIT_LOG="+fakeGitLog,
	)
	service := newCloseService(closeadapter.StaticRuntime{WorkDir: rt.workDir, Env: env}, closeadapter.NewTracker())
	_, err := service.Execute(context.Background(), closeapp.Request{
		ID: "cp-smoke", Message: "smoke close should not commit", Evidence: evidence, Mode: closeapp.ModeStrict,
	})
	if err == nil {
		return false
	}
	info, statErr := os.Stat(fakeGitLog)
	return os.IsNotExist(statErr) || (statErr == nil && info.Size() == 0)
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for index := len(env) - 1; index >= 0; index-- {
		if len(env[index]) >= len(prefix) && env[index][:len(prefix)] == prefix {
			return env[index][len(prefix):]
		}
	}
	return ""
}
