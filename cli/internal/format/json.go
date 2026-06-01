// Package format provides shared output-encoding helpers for the ao CLI.
//
// The helpers here centralize formatting conventions (such as indented JSON
// with a trailing newline) that were previously duplicated across dozens of
// command sites, so every call site emits byte-identical output.
package format

import (
	"encoding/json"
	"io"
)

// EncodeJSON writes v to w as indented JSON (two-space indent) followed by a
// newline, matching the encoding/json.Encoder default behavior used across the
// CLI.
//
// It intentionally uses json.Encoder (not json.MarshalIndent): the encoder
// appends a trailing '\n' after the value, which MarshalIndent does not. That
// trailing newline is load-bearing for existing output-format tests, so this
// helper preserves it.
func EncodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
