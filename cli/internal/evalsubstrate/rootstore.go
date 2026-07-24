package evalsubstrate

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

// RootStore performs eval-owned filesystem operations through os.Root, so a
// symlink or traversal component cannot escape the declared store.
type RootStore struct {
	name string
	root *os.Root
}

func OpenRootStore(name string) (*RootStore, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("open eval root store: empty root")
	}
	root, err := os.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("open eval root store %q: %w", name, err)
	}
	return &RootStore{name: name, root: root}, nil
}

func CreateRootStore(name string, mode fs.FileMode) (*RootStore, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("create eval root store: empty root")
	}
	if err := os.MkdirAll(name, mode.Perm()); err != nil {
		return nil, fmt.Errorf("create eval root store %q: %w", name, err)
	}
	return OpenRootStore(name)
}

func (store *RootStore) Close() error {
	if store == nil || store.root == nil {
		return nil
	}
	return store.root.Close()
}

func (store *RootStore) Name() string { return store.name }

func (store *RootStore) Path(relative string) (string, error) {
	clean, err := cleanRootRelative(relative, true)
	if err != nil {
		return "", err
	}
	if clean == "." {
		return store.name, nil
	}
	return filepath.Join(store.name, filepath.FromSlash(clean)), nil
}

func (store *RootStore) ReadFile(relative string) ([]byte, error) {
	clean, err := cleanRootRelative(relative, false)
	if err != nil {
		return nil, err
	}
	data, err := store.root.ReadFile(filepath.FromSlash(clean))
	if err != nil {
		return nil, fmt.Errorf("read eval store path %q: %w", relative, err)
	}
	return data, nil
}

func (store *RootStore) ReadDir(relative string) ([]os.DirEntry, error) {
	clean, err := cleanRootRelative(relative, true)
	if err != nil {
		return nil, err
	}
	file, err := store.root.Open(filepath.FromSlash(clean))
	if err != nil {
		return nil, fmt.Errorf("open eval store directory %q: %w", relative, err)
	}
	defer func() { _ = file.Close() }()
	entries, err := file.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("read eval store directory %q: %w", relative, err)
	}
	return entries, nil
}

func (store *RootStore) Stat(relative string) (os.FileInfo, error) {
	clean, err := cleanRootRelative(relative, true)
	if err != nil {
		return nil, err
	}
	return store.root.Stat(filepath.FromSlash(clean))
}

func (store *RootStore) Lstat(relative string) (os.FileInfo, error) {
	clean, err := cleanRootRelative(relative, true)
	if err != nil {
		return nil, err
	}
	return store.root.Lstat(filepath.FromSlash(clean))
}

func (store *RootStore) WalkDir(fn fs.WalkDirFunc) error {
	return fs.WalkDir(store.root.FS(), ".", fn)
}

func (store *RootStore) MkdirAll(relative string, mode fs.FileMode) error {
	clean, err := cleanRootRelative(relative, true)
	if err != nil {
		return err
	}
	if clean == "." {
		return nil
	}
	if err := store.root.MkdirAll(filepath.FromSlash(clean), mode.Perm()); err != nil {
		return fmt.Errorf("create eval store directory %q: %w", relative, err)
	}
	return nil
}

func (store *RootStore) Remove(relative string) error {
	clean, err := cleanRootRelative(relative, false)
	if err != nil {
		return err
	}
	if err := store.root.Remove(filepath.FromSlash(clean)); err != nil {
		return fmt.Errorf("remove eval store path %q: %w", relative, err)
	}
	return nil
}

func (store *RootStore) RemoveAll(relative string) error {
	clean, err := cleanRootRelative(relative, false)
	if err != nil {
		return err
	}
	if err := store.root.RemoveAll(filepath.FromSlash(clean)); err != nil {
		return fmt.Errorf("remove eval store tree %q: %w", relative, err)
	}
	return nil
}

// WriteAtomic writes through a unique sibling temp file, syncs the file,
// renames it through the rooted handle, and syncs the parent directory.
func (store *RootStore) WriteAtomic(relative string, data []byte, mode fs.FileMode) error {
	clean, err := cleanRootRelative(relative, false)
	if err != nil {
		return err
	}
	parent := path.Dir(clean)
	if err := store.MkdirAll(parent, 0o755); err != nil {
		return err
	}

	temp, file, err := store.openUniqueTemp(parent, path.Base(clean), mode)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = store.root.Remove(filepath.FromSlash(temp))
		}
	}()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write eval store temp for %q: %w", relative, err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync eval store temp for %q: %w", relative, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close eval store temp for %q: %w", relative, err)
	}
	if err := store.root.Rename(filepath.FromSlash(temp), filepath.FromSlash(clean)); err != nil {
		return fmt.Errorf("replace eval store path %q: %w", relative, err)
	}
	committed = true
	parentFile, err := store.root.Open(filepath.FromSlash(parent))
	if err != nil {
		return fmt.Errorf("open eval store parent %q for sync: %w", parent, err)
	}
	defer func() { _ = parentFile.Close() }()
	if err := parentFile.Sync(); err != nil {
		return fmt.Errorf("sync eval store parent %q: %w", parent, err)
	}
	return nil
}

func (store *RootStore) openUniqueTemp(parent, base string, mode fs.FileMode) (string, *os.File, error) {
	for attempt := 0; attempt < 32; attempt++ {
		var suffix [12]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return "", nil, fmt.Errorf("generate eval store temp name: %w", err)
		}
		name := "." + base + ".tmp-" + hex.EncodeToString(suffix[:])
		relative := path.Join(parent, name)
		file, err := store.root.OpenFile(filepath.FromSlash(relative), os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
		if err == nil {
			return relative, file, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", nil, fmt.Errorf("create eval store temp for %q: %w", base, err)
		}
	}
	return "", nil, fmt.Errorf("create eval store temp for %q: exhausted unique names", base)
}

func cleanRootRelative(value string, allowRoot bool) (string, error) {
	if value == "" {
		return "", errors.New("eval store path is empty")
	}
	if strings.Contains(value, `\`) {
		return "", fmt.Errorf("eval store path %q contains a backslash", value)
	}
	hasDrivePrefix := len(value) >= 2 && value[1] == ':' &&
		((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z'))
	if filepath.IsAbs(value) || filepath.VolumeName(value) != "" || strings.HasPrefix(value, "//") || hasDrivePrefix {
		return "", fmt.Errorf("eval store path %q is absolute or volume-qualified", value)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("eval store path %q contains control characters", value)
		}
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." {
			if allowRoot && value == "." {
				return ".", nil
			}
			return "", fmt.Errorf("eval store path %q contains an empty or dot component", value)
		}
	}
	clean := path.Clean(value)
	if clean == "." {
		if allowRoot {
			return clean, nil
		}
		return "", fmt.Errorf("eval store path %q names the root", value)
	}
	return clean, nil
}
