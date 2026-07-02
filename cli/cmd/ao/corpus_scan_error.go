package main

// corpusScanExitError carries the fail-closed exit code out of RunE so that a
// detected leak (or read failure) maps to a nonzero process exit without cobra
// printing its own error noise. Exit semantics:
//
//	0  clean — no markers, no read errors (publishable)
//	1  leak detected OR a file could not be read (FAIL CLOSED)
//	2  internal error invoking the scan
//
// This typed error lives in an UNTAGGED file (extracted from corpus_scan.go,
// which is archived behind //go:build flywheel per ADR-0012 / age-nzwo) because
// the spine's Execute() error switch in root.go type-asserts *corpusScanExitError
// to map the verdict to a process exit code. Keeping the type spine-resident lets
// the `ao corpus scan` command archive without breaking the default build.
type corpusScanExitError struct {
	code int
	msg  string
}

func (e *corpusScanExitError) Error() string { return e.msg }
func (e *corpusScanExitError) ExitCode() int { return e.code }
