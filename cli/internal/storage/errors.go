package storage

import "errors"

// Sentinel errors for the storage package. Using sentinels instead of ad-hoc
// fmt.Errorf allows callers to match with errors.Is for reliable error handling.
var (
	// ErrSessionIDRequired is returned when a session write is attempted without an ID.
	ErrSessionIDRequired = errors.New("session ID is required")

	// ErrEmptySessionFile is returned when a session file has no content.
	ErrEmptySessionFile = errors.New("empty session file")

	// ErrLineTooLong is returned by the JSONL scan helpers (ScanJSONL,
	// ScanJSONLFile) when a single line exceeds the package buffer cap
	// (scanJSONLMaxLine, 8MB). It signals a hard failure — never a silent
	// truncation — and is wrapped with the offending 1-based line number so the
	// caller can locate the bad record. Match it with errors.Is.
	ErrLineTooLong = errors.New("jsonl line exceeds maximum length")
)
