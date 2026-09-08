// Package evidencepath validates explicit non-Git evidence destinations.
// It checks active Git path bindings but resolves no configuration and creates nothing. Native access controls
// still own confidentiality and protection against concurrent hostile writers.
package evidencepath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Validate requires an existing directory and rejects ordinary repositories,
// linked worktrees, bare repositories and aliases into any of them before a
// caller creates directories or temporary files. Active Git path environment
// bindings and caller-supplied excluded roots are resolved before any write.
// Missing/denied required boundaries fail closed. This is not proof of absence
// of unknown external Git references to an unmarked directory.
func Validate(root string, excludedGitRoots ...string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", fmt.Errorf("explicit evidence root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("evidence root: %w", err)
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("evidence root is not a directory")
	}

	boundaries, err := storageBindings(excludedGitRoots)
	if err != nil {
		return "", err
	}
	for _, boundary := range boundaries {
		inside, err := within(real, boundary)
		if err != nil {
			return "", err
		}
		contains, err := within(boundary, real)
		if err != nil {
			return "", err
		}
		if inside || contains {
			return "", fmt.Errorf("evidence root overlaps declared Git storage")
		}
	}
	for _, start := range []string{abs, real} {
		for dir := start; ; dir = filepath.Dir(dir) {
			// Merely seeing a .git entry is sufficient; never open its private content.
			if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
				return "", fmt.Errorf("evidence root is nested in Git")
			} else if !os.IsNotExist(err) {
				return "", fmt.Errorf("check Git boundary: %w", err)
			}
			// Common storage need not have HEAD: split layouts keep it in
			// GIT_DIR. Both the ordinary/bare and split common shapes expose
			// objects + refs. Linked administration exposes HEAD + commondir.
			for _, markers := range [][]string{{"objects", "refs"}, {"HEAD", "commondir"}} {
				found := true
				for _, name := range markers {
					if _, err := os.Lstat(filepath.Join(dir, name)); err != nil {
						if !os.IsNotExist(err) {
							return "", fmt.Errorf("check Git storage boundary: %w", err)
						}
						found = false
					}
				}
				if found {
					return "", fmt.Errorf("evidence root is nested in Git storage")
				}
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return real, nil
}
