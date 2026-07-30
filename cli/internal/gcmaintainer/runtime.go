package gcmaintainer

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

var packMarkerCommit = regexp.MustCompile(`^[ \t]*commit[ \t]*=[ \t]*"([^"]*)"`)

// resolvePackDir returns the resolved official gascity pack root: the explicit
// override when given, otherwise the bundled pack cache under GC_HOME whose
// provenance marker records the accepted commit.
func resolvePackDir(explicit string) (string, error) {
	if explicit != "" {
		if !isDir(explicit) {
			return "", fmt.Errorf("pack directory does not exist: %s", explicit)
		}
		return canonical(explicit)
	}
	gcHome := os.Getenv("GC_HOME")
	if gcHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("HOME is required to locate the gc pack cache: %w", err)
		}
		gcHome = filepath.Join(home, ".gc")
	}
	markers, _ := filepath.Glob(filepath.Join(gcHome, "cache", "repos", "*", ".gc-bundled-pack-cache.toml"))
	for _, marker := range markers {
		if markerCommit(marker) != maintainerCommit {
			continue
		}
		candidate := filepath.Join(filepath.Dir(marker), "gascity")
		if !isDir(filepath.Join(candidate, "assets", "scripts", "checks")) || !isDir(filepath.Join(candidate, "schemas")) {
			continue
		}
		return canonical(candidate)
	}
	return "", fmt.Errorf("cannot locate the resolved official gascity %s pack cache; pass --pack-dir", maintainerCommit)
}

// markerCommit extracts the first commit pin recorded in a bundled pack cache
// provenance marker, or "" when none is present.
func markerCommit(marker string) string {
	data, err := os.ReadFile(marker)
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if m := packMarkerCommit.FindStringSubmatch(line); m != nil {
			return m[1]
		}
	}
	return ""
}

// validatePackDir requires the resolved pack to carry the upstream validation
// assets and a provenance marker matching the accepted commit.
func validatePackDir(packDir string) error {
	if !isDir(filepath.Join(packDir, "assets", "scripts", "checks")) {
		return fmt.Errorf("maintainer pack has no assets/scripts/checks")
	}
	if !isRegularFile(filepath.Join(packDir, "assets", "scripts", "validate_build_artifact.py")) {
		return fmt.Errorf("maintainer pack has no build artifact validator")
	}
	if !isDir(filepath.Join(packDir, "schemas")) {
		return fmt.Errorf("maintainer pack has no schemas")
	}
	marker := filepath.Join(filepath.Dir(packDir), ".gc-bundled-pack-cache.toml")
	if !isRegularFile(marker) {
		return fmt.Errorf("resolved pack has no bundled cache provenance marker")
	}
	if markerCommit(marker) != maintainerCommit {
		return fmt.Errorf("resolved pack cache marker does not match %s", maintainerCommit)
	}
	return nil
}

// selectPython returns the first existing python3 interpreter that can import
// PyYAML, preferring GC_PYTHON_BIN, then the stable system locations, then
// PATH.
func selectPython() (string, error) {
	pathPython, _ := exec.LookPath("python3")
	seen := map[string]bool{}
	for _, candidate := range []string{
		os.Getenv("GC_PYTHON_BIN"),
		"/opt/homebrew/bin/python3",
		"/usr/bin/python3",
		pathPython,
	} {
		if candidate == "" || seen[candidate] || !isExecutableFile(candidate) {
			continue
		}
		seen[candidate] = true
		if exec.Command(candidate, "-c", "import yaml").Run() == nil {
			return canonical(candidate)
		}
	}
	return "", fmt.Errorf("no existing python3 interpreter imports PyYAML; install PyYAML or set GC_PYTHON_BIN")
}

// packCheckScripts lists the upstream check script basenames shipped by the
// resolved pack.
func (o *ops) packCheckScripts() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(o.packDir, "assets", "scripts", "checks"))
	if err != nil {
		return nil, fmt.Errorf("read maintainer pack checks: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".sh") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// checkWrapperConflicts refuses to overwrite a check path that AgentOps does
// not manage, before any mutation happens.
func (o *ops) checkWrapperConflicts() error {
	names, err := o.packCheckScripts()
	if err != nil {
		return err
	}
	for _, name := range names {
		destination := filepath.Join(o.checksDir, name)
		if _, err := os.Lstat(destination); err != nil {
			continue
		}
		if !fileContains(destination, managedMarker) {
			return fmt.Errorf("refusing to overwrite unmanaged check: %s", destination)
		}
	}
	return nil
}

// prepareRuntime stages the contained maintainer runtime and managed check
// wrappers, links AgentOps skills into the Codex sinks, then re-verifies the
// installed state.
func (o *ops) prepareRuntime() error {
	if err := o.checkWrapperConflicts(); err != nil {
		return err
	}
	if err := o.checkSkillLinkConflicts(); err != nil {
		return err
	}
	if err := o.stageRuntime(); err != nil {
		return err
	}
	if err := o.installWrappers(); err != nil {
		return err
	}
	if err := o.linkSkills(); err != nil {
		return err
	}
	if err := o.checkRuntime(); err != nil {
		return err
	}
	return o.checkSkillLinks()
}

// stageRuntime snapshots the upstream validation assets and a python shim into
// the versioned runtime directory, atomically via a temp dir rename. An
// already-present runtime is left untouched (idempotent).
func (o *ops) stageRuntime() error {
	if isDir(o.runtime) {
		return nil
	}
	versions := filepath.Dir(o.runtime)
	if err := os.MkdirAll(versions, 0o755); err != nil {
		return fmt.Errorf("create runtime versions dir: %w", err)
	}
	tmp, err := os.MkdirTemp(versions, "."+maintainerCommit+".")
	if err != nil {
		return fmt.Errorf("stage maintainer runtime: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	if err := copyTree(filepath.Join(o.packDir, "assets", "scripts"), filepath.Join(tmp, "gascity", "assets", "scripts")); err != nil {
		return fmt.Errorf("snapshot upstream scripts: %w", err)
	}
	if err := copyTree(filepath.Join(o.packDir, "schemas"), filepath.Join(tmp, "gascity", "schemas")); err != nil {
		return fmt.Errorf("snapshot upstream schemas: %w", err)
	}
	shim := "#!/bin/sh\nexec " + quoteShell(o.pythonBin) + " \"$@\"\n"
	if err := writeExecutable(filepath.Join(tmp, "bin", "python3"), shim); err != nil {
		return fmt.Errorf("write python shim: %w", err)
	}
	env := fmt.Sprintf("commit=%s\nsource=%s\npython=%s\n", maintainerCommit, workflowSource, o.pythonBin)
	if err := os.WriteFile(filepath.Join(tmp, "agentops-runtime.env"), []byte(env), 0o644); err != nil {
		return fmt.Errorf("write runtime marker: %w", err)
	}
	if err := os.Rename(tmp, o.runtime); err != nil {
		// A concurrent prepare may have won the rename; that runtime is as good.
		if !isDir(o.runtime) {
			return fmt.Errorf("cannot install maintainer runtime at %s", o.runtime)
		}
	}
	return nil
}

// installWrappers writes an AgentOps-managed wrapper at each formula check
// path, delegating to the contained upstream script with the runtime python
// first on PATH.
func (o *ops) installWrappers() error {
	names, err := o.packCheckScripts()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(o.checksDir, 0o755); err != nil {
		return fmt.Errorf("create checks dir: %w", err)
	}
	for _, name := range names {
		upstream := filepath.Join(o.runtime, "gascity", "assets", "scripts", "checks", name)
		wrapper := "#!/bin/sh\n" +
			"# " + managedMarker + "\n" +
			"set -eu\n" +
			"runtime=" + quoteShell(o.runtime) + "\n" +
			`PATH="$runtime/bin:$PATH"` + "\n" +
			"export PATH\n" +
			"exec " + quoteShell(upstream) + " \"$@\"\n"
		destination := filepath.Join(o.checksDir, name)
		if err := writeExecutableAtomic(destination, wrapper); err != nil {
			return fmt.Errorf("install check wrapper %s: %w", name, err)
		}
	}
	return nil
}

// checkRuntime verifies the contained maintainer runtime matches the resolved
// upstream pack and that every managed wrapper is intact.
func (o *ops) checkRuntime() error {
	envPath := filepath.Join(o.runtime, "agentops-runtime.env")
	if !isRegularFile(envPath) {
		return fmt.Errorf("maintainer runtime is not prepared at %s", o.runtime)
	}
	if !fileHasLine(envPath, "commit="+maintainerCommit) {
		return fmt.Errorf("maintainer runtime commit marker is invalid")
	}
	shim := filepath.Join(o.runtime, "bin", "python3")
	if !isExecutableFile(shim) {
		return fmt.Errorf("maintainer runtime python shim is missing")
	}
	if exec.Command(shim, "-c", "import yaml").Run() != nil {
		return fmt.Errorf("maintainer runtime Python can no longer import PyYAML")
	}
	if !treesEqual(filepath.Join(o.packDir, "assets", "scripts"), filepath.Join(o.runtime, "gascity", "assets", "scripts")) {
		return fmt.Errorf("contained maintainer scripts differ from the resolved upstream pack")
	}
	if !treesEqual(filepath.Join(o.packDir, "schemas"), filepath.Join(o.runtime, "gascity", "schemas")) {
		return fmt.Errorf("contained maintainer schemas differ from the resolved upstream pack")
	}
	names, err := o.packCheckScripts()
	if err != nil {
		return err
	}
	for _, name := range names {
		destination := filepath.Join(o.checksDir, name)
		if !isExecutableFile(destination) {
			return fmt.Errorf("managed check wrapper is missing: %s", destination)
		}
		if !fileContains(destination, managedMarker) {
			return fmt.Errorf("check wrapper is no longer AgentOps-managed: %s", destination)
		}
	}
	return nil
}

// quoteShell single-quotes value for safe embedding in a POSIX shell script.
func quoteShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func fileContains(path, needle string) bool {
	data, err := os.ReadFile(path)
	return err == nil && strings.Contains(string(data), needle)
}

func fileHasLine(path, line string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return slices.Contains(strings.Split(string(data), "\n"), line)
}

func writeExecutable(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o755)
}

// writeExecutableAtomic writes content to path via a same-directory temp file
// and rename, so a concurrent reader never observes a partial wrapper.
func writeExecutableAtomic(path, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// copyTree copies a directory tree preserving file modes and symlink targets.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			return os.Symlink(link, target)
		default:
			return copyFile(path, target, info.Mode().Perm())
		}
	})
}

func copyFile(src, dst string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// treesEqual reports whether two directory trees hold the same entries with
// identical regular-file contents and symlink targets (the port of the shell
// helper's `diff -qr`).
func treesEqual(a, b string) bool {
	sigA, errA := treeSignature(a)
	sigB, errB := treeSignature(b)
	if errA != nil || errB != nil || len(sigA) != len(sigB) {
		return false
	}
	for rel, sig := range sigA {
		if sigB[rel] != sig {
			return false
		}
	}
	return true
}

func treeSignature(root string) (map[string]string, error) {
	signature := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		switch {
		case entry.IsDir():
			signature[rel] = "dir"
		case entry.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			signature[rel] = "link:" + link
		default:
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			signature[rel] = fmt.Sprintf("file:%x", sha256.Sum256(data))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return signature, nil
}
