package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type beadsDirResolution struct {
	Path   string
	Source string
}

const (
	beadsDirSourceEnv       = "env"
	beadsDirSourceGitCommon = "git-common-dir"
	beadsDirSourceRepoRoot  = "repo-root"
	beadsDirSourceCWD       = "cwd"
)

func beadsTrackerCommandContext(ctx context.Context, args ...string) *exec.Cmd {
	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		cwd = "."
	}
	return beadsTrackerCommandContextInDir(ctx, cwd, args...)
}

func beadsTrackerCommandContextInDir(ctx context.Context, cwd string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "br", args...)
	cmd.Dir = cwd
	cmd.Env = beadsTrackerEnvForDir(cwd)
	return cmd
}

func beadsTrackerEnvForDir(cwd string) []string {
	if _, ok := beadsEnvValue(os.Environ()); ok {
		return os.Environ()
	}
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "BEADS_DIR=") {
			continue
		}
		env = append(env, entry)
	}
	resolved := resolveBeadsDir(cwd, os.Environ())
	return append(env, "BEADS_DIR="+resolved.Path)
}

func resolveBeadsDir(cwd string, env []string) beadsDirResolution {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil || cwd == "" {
			cwd = "."
		}
	}
	if dir, ok := beadsEnvValue(env); ok {
		if filepath.IsAbs(dir) {
			return beadsDirResolution{Path: filepath.Clean(dir), Source: beadsDirSourceEnv}
		}
		return beadsDirResolution{Path: filepath.Clean(filepath.Join(cwd, dir)), Source: beadsDirSourceEnv}
	}
	if path := beadsDirFromGitCommon(cwd); path != "" {
		return beadsDirResolution{Path: path, Source: beadsDirSourceGitCommon}
	}
	if root, err := repoRootForBeads(cwd); err == nil && root != "" {
		return beadsDirResolution{Path: filepath.Join(root, "_beads"), Source: beadsDirSourceRepoRoot}
	}
	return beadsDirResolution{Path: filepath.Join(cwd, "_beads"), Source: beadsDirSourceCWD}
}

func beadsEnvValue(env []string) (string, bool) {
	for i := len(env) - 1; i >= 0; i-- {
		entry := env[i]
		if strings.HasPrefix(entry, "BEADS_DIR=") {
			value := strings.TrimSpace(strings.TrimPrefix(entry, "BEADS_DIR="))
			if value != "" {
				return value, true
			}
			return "", false
		}
	}
	return "", false
}

func beadsDirFromGitCommon(cwd string) string {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	common := strings.TrimSpace(string(out))
	if common == "" {
		return ""
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(cwd, common)
	}
	return filepath.Join(filepath.Dir(filepath.Clean(common)), "_beads")
}

// repoRootForBeads finds the git repo root for dir, falling back to dir.
func repoRootForBeads(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		if dir != "" {
			return dir, nil
		}
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return "", cwdErr
		}
		return cwd, nil
	}
	return strings.TrimSpace(string(out)), nil
}
