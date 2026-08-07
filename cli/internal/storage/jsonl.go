// Package storage provides atomic file-write primitives and the canonical
// JSONL scan policy shared across the CLI. The former session/index/provenance
// store API was retired with the knowledge-store surface: it had no callers,
// and `ao init` no longer scaffolds its directories.
package storage

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
)

// DefaultBaseDir is the default local requested-proof directory.
const DefaultBaseDir = ".agents/ao"

// ErrLineTooLong is returned by the JSONL scan helpers (ScanJSONL,
// ScanJSONLFile) when a single line exceeds the package buffer cap
// (scanJSONLMaxLine, 8MB). It signals a hard failure — never a silent
// truncation — and is wrapped with the offending 1-based line number so the
// caller can locate the bad record. Match it with errors.Is.
var ErrLineTooLong = errors.New("jsonl line exceeds maximum length")

const (
	scanJSONLInitBuf = 64 * 1024 // 64KB initial scanner buffer (grows to max)
	scanJSONLMaxLine = 8 << 20   // 8MB hard cap per line before ErrLineTooLong
)

// ScanJSONL reads r line by line and calls fn for each line's bytes, applying the
// package JSONL buffer policy: lines up to scanJSONLMaxLine (8MB) are read; a
// longer line stops iteration and returns an error wrapping ErrLineTooLong that
// names the 1-based line number. The line slice passed to fn is only valid for
// the duration of the call — copy it to retain it. A read error from r is
// returned as-is. This never silently truncates a line.
func ScanJSONL(r io.Reader, fn func(line []byte)) error {
	scanner := bufio.NewScanner(r)
	// +1: bufio.Scanner must hold the token AND its newline delimiter in the
	// buffer to emit the token, so a max size of exactly scanJSONLMaxLine would
	// reject a line of exactly scanJSONLMaxLine bytes — off by one against the
	// documented inclusive cap. The extra byte makes the cap inclusive.
	scanner.Buffer(make([]byte, 0, scanJSONLInitBuf), scanJSONLMaxLine+1)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		fn(scanner.Bytes())
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return fmt.Errorf("line %d: %w (cap %d bytes)", lineNo+1, ErrLineTooLong, scanJSONLMaxLine)
		}
		return err
	}
	return nil
}

// ScanJSONLFile opens the JSONL file at path and calls fn for each line, applying
// the package JSONL buffer policy (see ScanJSONL). A missing file is not an error
// (fn is simply never called); any other open error, read error, or an oversized
// line (ErrLineTooLong, naming the line number) is returned. The file is always
// closed; a close error is surfaced only if the scan itself succeeded.
func ScanJSONLFile(path string, fn func(line []byte)) (err error) {
	f, err := os.Open(path) // #nosec G304 -- caller-owned local path by contract.
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	return ScanJSONL(f, fn)
}
