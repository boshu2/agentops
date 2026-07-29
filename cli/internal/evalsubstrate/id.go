package evalsubstrate

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateID rejects an eval-store identifier (task, suite, run, or model-spec
// ID) that could escape its declared subtree once it is interpolated into a
// store path such as filepath.Join(root, "<kind>", id, "<file>").
//
// A valid ID is a single, non-empty path component: it contains no path
// separator, is not the "." or ".." traversal component, is not an absolute
// path or a volume/drive reference, and contains no control characters. This
// closes the 2026-07-24 Go CLI audit's High "eval IDs permit filesystem escape"
// finding, where a YAML-supplied id such as "../../escape" was cleaned by
// filepath.Join and written or read outside the eval root.
//
// Colons are intentionally permitted: legitimate model-spec IDs use them
// (for example "ms:stable"). Windows drive references (for example "C:") are
// still rejected via filepath.VolumeName, which returns "" for a plain colon.
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("id is empty")
	}
	if strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("id %q contains a path separator", id)
	}
	if id == "." || id == ".." {
		return fmt.Errorf("id %q is a path-traversal component", id)
	}
	if filepath.IsAbs(id) || filepath.VolumeName(id) != "" {
		return fmt.Errorf("id %q is an absolute path or volume reference", id)
	}
	for _, r := range id {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("id %q contains a control character", id)
		}
	}
	return nil
}
