package frontier

// waiver.go is the Go port of scripts/lib/trivial-waiver.sh — the SINGLE
// shell implementation of the #trivial provenance-only waiver (age-w2ny
// marker detection + age-u43w diff verification).
//
// LOCKSTEP PAIRING (documented both ways; the script header points back
// here): this port and the script MUST implement the same predicate. Any
// semantic change to either side must land in the other in the same commit;
// TestTrivialWaiver_Lockstep is the tripwire table. The four outcomes map
// 1:1 to the script's return codes:
//
//	WaiverWaived     (rc 0) — explicit #trivial marker AND every changed file
//	                          under docs/provenance/
//	WaiverFailClosed (rc 1) — marker present but triviality is unprovable
//	                          (diff-tree failed or empty file list): HOLD
//	WaiverRefused    (rc 2) — marker present but the diff touches
//	                          non-provenance path(s): normal pawl path
//	WaiverNoMarker   (rc 3) — no explicit #trivial marker
//
// WHY A PORT, NOT AN EXEC (the sturdiness call, age-ekam): the frontier is a
// library consumed in-process by the yield report, the close gate, and the
// land lane. Exec'ing a repo-relative bash script from library code (a)
// breaks outside this repo — product installs ship the ao binary, not
// scripts/ — (b) reopens the run-a-repo-script trust hole the portable
// pre-push gate deliberately closed (age-rk3r.6: never execute repo content
// to decide a gate), and (c) adds a bash/platform dependency to what is 30
// lines of pure logic over two git plumbing reads. The gate twins in
// cli/cmd/ao (shaBindsCommit / parseDisposition vs pawl-verdict.sh) set the
// same precedent: shell authority stays canonical for shell surfaces; Go
// consumers carry a documented, test-pinned twin.

import "regexp"

// WaiverStatus mirrors the four return codes of scripts/lib/trivial-waiver.sh
// (the lockstep pairing above).
type WaiverStatus int

// Waiver outcomes, in the script's return-code order.
const (
	// WaiverWaived (rc 0): explicit #trivial marker and a provenance-only diff.
	WaiverWaived WaiverStatus = iota
	// WaiverFailClosed (rc 1): marker present but triviality unprovable — HOLD.
	WaiverFailClosed
	// WaiverRefused (rc 2): marker present but non-provenance paths touched.
	WaiverRefused
	// WaiverNoMarker (rc 3): no explicit #trivial marker.
	WaiverNoMarker
)

// waiverSubjectMarkerRe is the Go twin of the script's subject grep:
// `(^|[[:space:]])#trivial[[:space:]]*$` — a TRAILING tag at the END of the
// subject line. POSIX [[:space:]] is [ \t\v\f\r] here (a subject never
// contains \n).
var waiverSubjectMarkerRe = regexp.MustCompile(`(?i)(^|[ \t\v\f\r])#trivial[ \t\v\f\r]*$`)

// waiverBodyMarkerRe is the Go twin of the script's body grep:
// `^[[:space:]]*#trivial[[:space:]]*$` per line — a standalone trailer line.
var waiverBodyMarkerRe = regexp.MustCompile(`(?im)^[ \t\v\f\r]*#trivial[ \t\v\f\r]*$`)

// waiverPathPrefix is the sole allowlisted path prefix — the provenance
// ledger, the only established #trivial use (100% of historical #trivial
// commits touch only docs/provenance/).
const waiverPathPrefix = "docs/provenance/"

// TrivialWaiver decides whether commit sha in repo is waived from the
// cross-family pawl as a #trivial provenance-only commit. The second return
// is a human-readable detail for HOLD/refusal messages.
//
// Discipline ported verbatim from the script:
//   - age-w2ny: waive ONLY on an explicit marker — a trailing #trivial tag at
//     the end of the subject, or a standalone #trivial trailer line in the
//     body. A #trivial merely MENTIONED in prose (mid-subject or inside a
//     body line) must NOT waive — that was a fail-open.
//   - age-u43w: #trivial is an AUTHOR ASSERTION, not a fact. Verify the DIFF:
//     every changed file must be under docs/provenance/. --no-renames forces
//     a rename INTO docs/provenance/ to expose its non-allowlisted old path.
//     A failed diff-tree or an empty file list cannot PROVE triviality and is
//     fail-closed (HOLD), never trusted.
func TrivialWaiver(repo, sha string) (WaiverStatus, string) {
	// The script tolerates git failures on subject/body reads (`|| true`):
	// an unreadable message simply has no marker.
	subject, err := gitOutput(repo, "log", "-1", "--format=%s", sha)
	if err != nil {
		subject = ""
	}
	body, err := gitOutput(repo, "log", "-1", "--format=%b", sha)
	if err != nil {
		body = ""
	}
	if !waiverSubjectMarkerRe.MatchString(subject) && !waiverBodyMarkerRe.MatchString(body) {
		return WaiverNoMarker, ""
	}

	changed, err := gitOutput(repo, "diff-tree", "--no-commit-id", "--no-renames", "--name-only", "-r", sha)
	if err != nil {
		return WaiverFailClosed, "diff-tree failed; cannot prove triviality — fail-closed, pawl required"
	}
	if changed == "" {
		return WaiverFailClosed, "empty changed-file list — cannot prove triviality — fail-closed, pawl required"
	}
	var nontrivial []string
	for _, f := range splitLines(changed) {
		if f != "" && !hasPathPrefix(f, waiverPathPrefix) {
			nontrivial = append(nontrivial, f)
		}
	}
	if len(nontrivial) == 0 {
		return WaiverWaived, "provenance-ledger only"
	}
	return WaiverRefused, "touches non-trivial path(s): " + joinLines(nontrivial)
}

// hasPathPrefix reports whether path starts with prefix (byte-exact, the same
// anchor semantics as the script's `grep -vE '^docs/provenance/'`).
func hasPathPrefix(path, prefix string) bool {
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}

// splitLines splits s on newlines without pulling in strings for two calls.
func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			out = append(out, line)
			start = i + 1
		}
	}
	return out
}

// joinLines joins parts with ", " for refusal detail.
func joinLines(parts []string) string {
	s := ""
	for i, p := range parts {
		if i > 0 {
			s += ", "
		}
		s += p
	}
	return s
}
