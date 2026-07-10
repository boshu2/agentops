// Package trackerresolve selects the one tracker backend every CLI consumer
// must use. It resolves identity only; subprocess execution belongs to tracker
// adapters or an explicitly declared raw passthrough boundary.
package trackerresolve

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	BR = "br"
	BD = "bd"
)

type Resolution struct {
	Tracker   string
	Binary    string
	LedgerDir string
	Source    string
}

type LookPath func(string) (string, error)

func Resolve(cwd string, env []string) (Resolution, error) {
	return ResolveWithLookPath(cwd, env, exec.LookPath)
}

func ResolveWithLookPath(cwd string, env []string, look LookPath) (Resolution, error) {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	if cwd == "" {
		cwd = "."
	}
	if env == nil {
		env = os.Environ()
	}
	if raw, ok := envValue(env, "AGENTOPS_TRACKER"); ok {
		kind, err := normalize(raw)
		if err != nil {
			return Resolution{}, err
		}
		return finish(kind, "env", cwd, env, look), nil
	}
	if raw, ok := configValue(cwd, env); ok {
		kind, err := normalize(raw)
		if err != nil {
			return Resolution{}, err
		}
		return finish(kind, "config", cwd, env, look), nil
	}
	if isDir(filepath.Join(cwd, "_beads")) {
		return finish(BR, "ledger", cwd, env, look), nil
	}
	if isDir(filepath.Join(cwd, ".beads")) {
		return finish(BD, "ledger", cwd, env, look), nil
	}
	if _, err := look(BR); err == nil {
		return finish(BR, "binary", cwd, env, look), nil
	}
	if _, err := look(BD); err == nil {
		return finish(BD, "binary", cwd, env, look), nil
	}
	return Resolution{}, fmt.Errorf("no tracker selected: set AGENTOPS_TRACKER=br|bd, create _beads/.beads, or install br/bd")
}

func finish(kind, source, cwd string, env []string, look LookPath) Resolution {
	ledger := filepath.Join(cwd, "_beads")
	if kind == BD {
		ledger = filepath.Join(cwd, ".beads")
	} else if value, ok := envValue(env, "BEADS_DIR"); ok {
		if filepath.IsAbs(value) {
			ledger = filepath.Clean(value)
		} else {
			ledger = filepath.Join(cwd, value)
		}
	}
	binary := kind
	if path, err := look(kind); err == nil {
		binary = path
	}
	return Resolution{Tracker: kind, Binary: binary, LedgerDir: ledger, Source: source}
}

func normalize(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case BR:
		return BR, nil
	case BD:
		return BD, nil
	default:
		return "", fmt.Errorf("unknown tracker %q (want br or bd)", raw)
	}
}

func configValue(cwd string, env []string) (string, bool) {
	paths := []string{filepath.Join(cwd, ".agentops", "config.yaml")}
	if home, ok := envValue(env, "HOME"); ok {
		paths = append(paths, filepath.Join(home, ".agentops", "config.yaml"))
	}
	for _, path := range paths {
		data, err := os.ReadFile(path) // #nosec G304 -- fixed config path under cwd/home.
		if err != nil {
			continue
		}
		var value struct {
			Tracker string `yaml:"tracker"`
		}
		if yaml.Unmarshal(data, &value) == nil && strings.TrimSpace(value.Tracker) != "" {
			return value.Tracker, true
		}
	}
	return "", false
}

func envValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for i := len(env) - 1; i >= 0; i-- {
		if strings.HasPrefix(env[i], prefix) {
			value := strings.TrimSpace(strings.TrimPrefix(env[i], prefix))
			return value, value != ""
		}
	}
	return "", false
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
