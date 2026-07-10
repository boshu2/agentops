package frontier

// inverse.go implements the deterministic machine verification behind the
// verified-by-compensation arm (A5): git-diff inverse-equality. A mechanical
// revert is admitted to the frontier with NO second model review precisely
// because this check is exact — so the check must be strict enough to carry
// that weight on its own.

import (
	"fmt"
	"strings"
)

// CheckInverse verifies that compensator is the machine-verified inverse
// patch of refuted: the patch compensator introduces (compensator^ →
// compensator) is BYTE-IDENTICAL to the reverse of the patch refuted
// introduced (refuted → refuted^), under normalized diff flags with full
// blob indices.
//
// Byte-exactness is deliberate — `git patch-id` is whitespace-insensitive
// and blind to mode bits, so it is too weak for an unreviewed admission (the
// same caveat cli/cmd/ao/diff_identity.go records for the REBOUND lane).
// Byte-exact tree diffs also mean: if ANY intervening commit touched the
// same blobs, the revert's diff no longer matches the refuted commit's
// inverse and the check fails — which is correct, because such a revert is
// no longer a pure inverse and must route through the model-reviewed
// fix-forward arm instead (fail-closed by construction).
//
// Fail-closed edges: root commits and merge commits (parent count != 1) are
// rejected; an empty diff proves nothing and is rejected.
func CheckInverse(repo, compensator, refuted string) error {
	comp, err := gitOutput(repo, "rev-parse", "--verify", "--quiet", compensator+"^{commit}")
	if err != nil {
		return fmt.Errorf("compensator %s does not resolve to a commit: %w", short7(compensator), err)
	}
	ref, err := gitOutput(repo, "rev-parse", "--verify", "--quiet", refuted+"^{commit}")
	if err != nil {
		return fmt.Errorf("refuted %s does not resolve to a commit: %w", short7(refuted), err)
	}
	if comp == ref {
		return fmt.Errorf("compensator and refuted are the same commit %s — self-inversion proves nothing", short7(comp))
	}
	for _, sha := range []string{comp, ref} {
		n, err := parentCount(repo, sha)
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("commit %s has %d parents — inverse verification requires a single-parent commit (fail-closed)", short7(sha), n)
		}
	}

	// The patch that undoes the refuted commit: tree(refuted) → tree(refuted^).
	wantPatch, err := treeDiff(repo, ref, ref+"^")
	if err != nil {
		return err
	}
	// The patch the compensator introduces: tree(comp^) → tree(comp).
	gotPatch, err := treeDiff(repo, comp+"^", comp)
	if err != nil {
		return err
	}
	if strings.TrimSpace(wantPatch) == "" {
		return fmt.Errorf("refuted commit %s has an empty diff — nothing to invert (fail-closed)", short7(ref))
	}
	if gotPatch != wantPatch {
		return fmt.Errorf("compensator %s is NOT the byte-exact inverse of %s — route through the model-reviewed fix-forward arm", short7(comp), short7(ref))
	}
	return nil
}

// parentCount returns the number of parents of sha.
func parentCount(repo, sha string) (int, error) {
	out, err := gitOutput(repo, "rev-list", "--parents", "-n", "1", sha)
	if err != nil {
		return 0, fmt.Errorf("reading parents of %s: %w", short7(sha), err)
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return 0, fmt.Errorf("reading parents of %s: empty rev-list output", short7(sha))
	}
	return len(fields) - 1, nil
}

// treeDiff returns the normalized tree-to-tree patch a → b. Flags pin the
// output byte-stable across environments: no external/textconv drivers, no
// color, no rename detection, full blob indices (mode + content identity).
func treeDiff(repo, a, b string) (string, error) {
	out, err := gitOutput(repo, "-c", "diff.mnemonicPrefix=false", "-c", "diff.noprefix=false",
		"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--no-renames", "--full-index", a, b)
	if err != nil {
		return "", fmt.Errorf("diff %s..%s: %w", short7(a), short7(b), err)
	}
	return out, nil
}
