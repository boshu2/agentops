package evalsubstrate

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// maxIDBytes caps the length of an eval-store identifier once it is the final,
// encoding-applied path component. 128 bytes stays well under every common
// filesystem's per-component limit while allowing dated, descriptive IDs.
const maxIDBytes = 128

// ValidateID rejects an eval-store identifier (task, suite, run, or model-spec
// ID) that is not a safe, canonical, single path component — the form that may
// become a directory name once interpolated into filepath.Join(root, "<kind>",
// id, "<file>"). It closes the 2026-07-24 Go CLI audit's High "eval IDs permit
// filesystem escape" finding and the cross-family round-1 portability findings.
//
// An ID is valid only when it:
//   - is non-empty and at most maxIDBytes bytes;
//   - is valid UTF-8 in Unicode NFC form (non-NFC input is REJECTED, not
//     silently normalized, so two visually identical IDs can never address two
//     distinct on-disk entries);
//   - contains no path separator ('/' or '\'), and is neither an absolute path
//     nor a volume/drive reference (filepath.VolumeName);
//   - has no leading or trailing space or dot — Win32 path normalization strips
//     trailing spaces and dots, so ".. " would otherwise renormalize to the
//     ".." traversal component; this rule also subsumes "." and "..";
//   - contains no control characters (C0 0x00–0x1F, DEL 0x7F, or C1 0x80–0x9F);
//   - contains no ':' — ':' denotes a Windows alternate-data-stream and must be
//     percent-encoded away (see ModelSpecPath) BEFORE an ID becomes a path
//     component, never passed through raw. Model-spec IDs such as "ms:stable"
//     are therefore validated in their encoded ("ms%3Astable") form.
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("id is empty")
	}
	if len(id) > maxIDBytes {
		return fmt.Errorf("id is %d bytes, exceeds the %d-byte cap", len(id), maxIDBytes)
	}
	if !utf8.ValidString(id) {
		return fmt.Errorf("id %q is not valid UTF-8", id)
	}
	if strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("id %q contains a path separator", id)
	}
	if filepath.IsAbs(id) || filepath.VolumeName(id) != "" {
		return fmt.Errorf("id %q is an absolute path or volume reference", id)
	}
	if strings.ContainsRune(id, ':') {
		return fmt.Errorf("id %q contains a reserved ':' character (must be encoded before path use)", id)
	}
	first, _ := utf8.DecodeRuneInString(id)
	last, _ := utf8.DecodeLastRuneInString(id)
	if first == '.' || last == '.' || unicode.IsSpace(first) || unicode.IsSpace(last) {
		return fmt.Errorf("id %q has a leading or trailing space or dot", id)
	}
	for _, r := range id {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return fmt.Errorf("id %q contains a control character", id)
		}
	}
	if !norm.NFC.IsNormalString(id) {
		return fmt.Errorf("id %q is not in Unicode NFC form", id)
	}
	return nil
}
