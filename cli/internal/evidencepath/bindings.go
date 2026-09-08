package evidencepath

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// resolveDirectory resolves only a supplied path. It never creates missing
// directories or guesses a replacement when a required binding is unavailable.
func resolveDirectory(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("empty Git storage boundary")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("git storage boundary is not a directory")
	}
	return real, nil
}

// storageBindings follows fixed active Git environment inputs and the
// commondir pointer of an explicitly supplied GIT_DIR. It reads no Git config
// and performs no subprocess, repository discovery or reverse-reference scan.
func storageBindings(excluded []string) ([]string, error) {
	roots := []string{}
	add := func(path, label string) (string, error) {
		real, err := resolveDirectory(path)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", label, err)
		}
		roots = append(roots, real)
		return real, nil
	}
	for _, path := range excluded {
		if _, err := add(path, "--exclude-git-root"); err != nil {
			return nil, err
		}
	}
	active := map[string]string{}
	for _, key := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_WORK_TREE"} {
		if value, set := os.LookupEnv(key); set {
			real, err := add(value, key)
			if err != nil {
				return nil, err
			}
			active[key] = real
		}
	}
	if err := addCommonStorageBindings(active, add); err != nil {
		return nil, err
	}
	if value, set := os.LookupEnv("GIT_ALTERNATE_OBJECT_DIRECTORIES"); set {
		paths, err := splitGitPaths(value)
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			if _, err := add(path, "GIT_ALTERNATE_OBJECT_DIRECTORIES"); err != nil {
				return nil, err
			}
		}
	}
	if value, set := os.LookupEnv("GIT_INDEX_FILE"); set {
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("empty GIT_INDEX_FILE")
		}
		// Git may create an index that does not yet exist. Its existing parent is
		// an explicit storage boundary; an existing symlink must resolve first.
		if _, err := os.Lstat(value); err == nil {
			value, err = filepath.EvalSymlinks(value)
			if err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		if _, err := add(filepath.Dir(value), "GIT_INDEX_FILE parent"); err != nil {
			return nil, err
		}
	}
	return roots, nil
}

// readPointer only reads a named optional Git path pointer. Missing is distinct
// from denied, dangling or oversized; the latter cases fail closed.
func readPointer(path string) (pointer string, exists bool, err error) {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", false, err
	}
	if !info.Mode().IsRegular() {
		return "", false, fmt.Errorf("git pointer is not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	info, err = f.Stat()
	if err != nil {
		return "", false, err
	}
	if !info.Mode().IsRegular() {
		return "", false, fmt.Errorf("git pointer is not a regular file")
	}
	b, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil {
		return "", false, err
	}
	if len(b) > 4096 {
		return "", false, fmt.Errorf("git pointer exceeds 4096 bytes")
	}
	return strings.TrimRight(string(b), "\r\n"), true, nil
}

// Git alternate environment entries use the platform path separator and may
// be C-quoted to include that separator. Reject malformed/unresolved entries.
func splitGitPaths(value string) ([]string, error) {
	if value == "" {
		return nil, nil
	} // Git's empty alternate list declares no paths.
	var result []string
	for len(value) > 0 {
		var path string
		if value[0] == '"' {
			end := 1
			for end < len(value) {
				if value[end] == 92 {
					end += 2
					continue
				}
				if value[end] == '"' {
					break
				}
				end++
			}
			if end >= len(value) {
				return nil, fmt.Errorf("invalid quoted Git alternate path")
			}
			var err error
			path, err = strconv.Unquote(value[:end+1])
			if err != nil {
				return nil, err
			}
			value = value[end+1:]
			if value != "" && value[0] != os.PathListSeparator {
				return nil, fmt.Errorf("invalid Git alternate path separator")
			}
		} else {
			end := strings.IndexByte(value, os.PathListSeparator)
			if end < 0 {
				end = len(value)
			}
			path, value = value[:end], value[end:]
		}
		if path == "" {
			return nil, fmt.Errorf("empty Git alternate path")
		}
		result = append(result, path)
		if value != "" {
			value = value[1:]
			if value == "" {
				return nil, fmt.Errorf("empty Git alternate path")
			}
		}
	}
	return result, nil
}

// within uses filesystem identity on ancestors as well as resolved path names,
// so aliases on a case-insensitive filesystem do not evade containment checks.
func within(path, boundary string) (bool, error) {
	target, err := os.Stat(boundary)
	if err != nil {
		return false, err
	}
	for dir := path; ; dir = filepath.Dir(dir) {
		info, err := os.Stat(dir)
		if err != nil {
			return false, err
		}
		if os.SameFile(info, target) {
			return true, nil
		}
		if filepath.Dir(dir) == dir {
			return false, nil
		}
	}
}

func addCommonStorageBindings(active map[string]string, add func(string, string) (string, error)) error {
	common := active["GIT_COMMON_DIR"]
	if admin := active["GIT_DIR"]; admin != "" && common == "" {
		pointer, exists, err := readPointer(filepath.Join(admin, "commondir"))
		if err != nil {
			return fmt.Errorf("resolve GIT_DIR commondir: %w", err)
		}
		if exists {
			if pointer == "" || strings.ContainsAny(pointer, "\x00\r\n") {
				return fmt.Errorf("invalid GIT_DIR commondir pointer")
			}
			if !filepath.IsAbs(pointer) {
				pointer = filepath.Join(admin, pointer)
			}
			common, err = add(pointer, "GIT_DIR commondir")
			if err != nil {
				return err
			}
		} else {
			common = admin
		}
	}
	// These known child directories may themselves be symlinked outside the
	// administration/common root. An explicit relocated object binding overrides
	// the default objects directory, as it does for Git.
	if common != "" {
		for _, name := range []string{"objects", "refs", "logs"} {
			if name == "objects" && active["GIT_OBJECT_DIRECTORY"] != "" {
				continue
			}
			child := filepath.Join(common, name)
			if _, err := os.Lstat(child); os.IsNotExist(err) {
				if name == "objects" {
					return fmt.Errorf("unresolved Git objects directory: %w", err)
				}
				continue
			} else if err != nil {
				return err
			}
			if _, err := add(child, "Git "+name); err != nil {
				return err
			}
		}
	}
	return nil
}
