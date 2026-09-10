package doctor

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
)

// writeActionBackup creates an exclusive per-action snapshot. Source paths
// never participate in its name, so home paths and repeated writes cannot
// escape the run or replace an earlier snapshot. The rooted handle also
// prevents backup-directory symlinks from escaping the owning run.
func writeActionBackup(ctx *MutateContext, data []byte, info os.FileInfo) (string, error) {
	root, err := os.OpenRoot(ctx.RunDir)
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()
	name := filepath.Join("backups", rand.Text())
	if err := workspaceRootParentsReal(root, name); err != nil {
		return "", err
	}
	if err := root.MkdirAll("backups", 0o700); err != nil {
		return "", err
	}
	file, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	if _, err := file.Write(data); err != nil {
		return "", err
	}
	if err := file.Chmod(info.Mode()); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := root.Chtimes(name, info.ModTime(), info.ModTime()); err != nil {
		return "", err
	}
	_, stored, err := readWorkspaceRootRegular(root, name)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(data, stored) {
		return "", fmt.Errorf("backup verify failed (cmp-strict mismatch)")
	}
	return name, nil
}

type undoAction struct {
	record ActionRecord
	data   []byte
	info   os.FileInfo
}

// prepareUndo validates all backup bytes before the first live mutation. This
// also protects older journals whose repeated writes shared one overwritten
// backup: a mismatch anywhere rejects the run without partially restoring it.
func prepareUndo(runDir string, records []ActionRecord, strict bool) ([]undoAction, error) {
	root, err := os.OpenRoot(runDir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	prepared := make([]undoAction, 0, len(records))
	for _, rec := range records {
		action := undoAction{record: rec}
		if rec.OK && !rec.RolledBack && rec.Existed && rec.Op != "Rename" {
			action.info, action.data, err = readActionBackup(root, rec)
			if err != nil && (strict || !os.IsNotExist(err)) {
				return nil, err
			}
		}
		prepared = append(prepared, action)
	}
	return prepared, nil
}

func readActionBackup(root *os.Root, rec ActionRecord) (os.FileInfo, []byte, error) {
	name := rec.BackupPath
	if name == "" {
		// Legacy action records remain readable only when their old layout is
		// contained. Escaped home backups may have been shared across runs.
		if !filepath.IsLocal(rec.Path) {
			return nil, nil, fmt.Errorf("doctor: unsafe legacy backup path for %s", rec.Path)
		}
		name = filepath.Join("backups", rec.Path)
	}
	if !filepath.IsLocal(name) {
		return nil, nil, fmt.Errorf("doctor: backup path escapes run: %s", name)
	}
	if err := workspaceRootParentsReal(root, name); err != nil {
		return nil, nil, err
	}
	info, data, err := readWorkspaceRootRegular(root, name)
	if err != nil {
		return nil, nil, fmt.Errorf("doctor: read backup for %s: %w", rec.Path, err)
	}
	if sha256Hex(data) != rec.BeforeHash {
		return nil, nil, fmt.Errorf("doctor: backup hash mismatch for %s", rec.Path)
	}
	return info, data, nil
}

func undoOne(repoRoot string, action undoAction, locks *LockManager, strict, dryRun bool, res *UndoResult) error {
	rec := action.record
	if !rec.OK || rec.RolledBack || (rec.Op != "Rename" && action.info == nil) {
		res.Skipped++
		return nil
	}
	target := filepath.Join(repoRoot, rec.Path)
	if dryRun {
		fmt.Fprintf(os.Stderr, "[dry-run] would restore %s\n", target)
		res.Skipped++
		return nil
	}
	guard, err := locks.Acquire(target)
	if err != nil {
		return err
	}
	defer func() { _ = guard.Release() }()
	if rec.Op == "Rename" {
		if err := undoRename(target, rec.RenameTo, locks); err != nil {
			if strict {
				return fmt.Errorf("doctor: un-rename %s: %w", target, err)
			}
			res.Skipped++
			return nil
		}
	} else {
		// Use the exact bytes validated during preflight, never re-open an
		// untrusted backup after truncating or replacing the live target.
		if err := atomicWrite(target, action.data, action.info.Mode()); err != nil {
			return fmt.Errorf("doctor: restore %s: %w", target, err)
		}
		if err := os.Chtimes(target, action.info.ModTime(), action.info.ModTime()); err != nil {
			return fmt.Errorf("doctor: restore mtime %s: %w", target, err)
		}
		restored, err := os.ReadFile(target)
		if err != nil {
			return fmt.Errorf("doctor: read restored %s: %w", target, err)
		}
		if strict && sha256Hex(restored) != rec.BeforeHash {
			return fmt.Errorf("doctor: restored hash mismatch for %s", rec.Path)
		}
	}
	res.Restored++
	return nil
}

// undoRename holds both endpoints' advisory locks. Lstat treats a dangling
// symlink or empty directory as a conflict too. Non-directory moves additionally
// use link's atomic no-replace guarantee against uncoordinated file creators.
// Directory renames retain the workspace fixers' advisory-lock race boundary.
func undoRename(target, source string, locks *LockManager) error {
	if source == "" || filepath.Clean(source) == filepath.Clean(target) {
		return fmt.Errorf("invalid rename source %q", source)
	}
	guard, err := locks.Acquire(source)
	if err != nil {
		return err
	}
	defer func() { _ = guard.Release() }()
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("restore conflict: %s already exists", target)
	} else if !os.IsNotExist(err) {
		return err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		if err := os.Link(source, target); err != nil {
			return err
		}
		// On a removal failure preserve both links; never delete a replacement
		// at the target in an attempt to compensate.
		return os.Remove(source)
	}
	return os.Rename(source, target)
}
