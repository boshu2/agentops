package okfprofile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CheckFile reads only the explicitly named regular concept file. It does not
// discover a bundle, consult configuration, resolve sources or write receipts.
// Authorization and OS confinement belong to the invoking native runtime.
func CheckFile(path, profile string) (result Result, err error) {
	if err := checkProfile(profile); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(path) == "" || filepath.Ext(path) != ".md" {
		return Result{}, fmt.Errorf("--file must name a Markdown concept file")
	}
	base := filepath.Base(path)
	if base == "index.md" || base == "log.md" {
		return Result{}, fmt.Errorf("reserved index.md and log.md are not concept pages")
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return Result{}, fmt.Errorf("cannot open input directory")
	}
	defer func() {
		if closeErr := root.Close(); err == nil && closeErr != nil {
			result = Result{}
			err = fmt.Errorf("cannot close input directory")
		}
	}()
	before, err := root.Lstat(base)
	if err != nil {
		return Result{}, fmt.Errorf("cannot inspect input file")
	}
	if !before.Mode().IsRegular() {
		return Result{}, fmt.Errorf("input must be a regular file, not a symlink or special file")
	}
	if before.Size() > MaxPageBytes {
		return Result{}, fmt.Errorf("input exceeds 1 MiB read bound")
	}
	file, err := root.Open(base)
	if err != nil {
		return Result{}, fmt.Errorf("cannot open input file")
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			result = Result{}
			err = fmt.Errorf("cannot close input file")
		}
	}()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return Result{}, fmt.Errorf("input changed while opening")
	}
	payload, err := io.ReadAll(io.LimitReader(file, MaxPageBytes+1))
	if err != nil {
		return Result{}, fmt.Errorf("cannot read input file")
	}
	if len(payload) > MaxPageBytes {
		return Result{}, fmt.Errorf("input exceeds 1 MiB read bound")
	}
	after, err := file.Stat()
	if err != nil || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return Result{}, fmt.Errorf("input changed while reading")
	}
	return Check(payload, profile)
}
