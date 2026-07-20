package clicontract

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// HostOptions is the single canonical host-seam contract every command module
// in internal/commands/<family> receives from the root executable. It replaced
// the four incompatible per-module seam shapes (positional funcs, bespoke
// HostOptions structs, and a GlobalOptions bundle) that drifted apart across
// the cmd/ao carve-out waves.
//
// A module reads only the seams it needs; an unset (nil) field means the seam
// is absent for that module (zero-value = absent). Modules keep their own
// per-domain UseCases types — only the host seams are unified here.
//
// The flag-derived seams (OutputMode, Verbose, DryRun) resolve the negotiated
// global flag values the root wires from its persistent flags. The remaining
// seams inject host-owned environment, filesystem, and clock facts that a
// module must never read directly (the carveout drift guards enforce that a
// module.go imports no os/os-exec package and calls no time.Now).
type HostOptions struct {
	// OutputMode resolves the active output format ("table" | "json" | "yaml").
	OutputMode func() string
	// Verbose reports whether -v/--verbose is enabled.
	Verbose func() bool
	// Verbosef is the host verbose diagnostic printer. It is safe to leave nil
	// only when a module never calls it; modules that use it must nil-check.
	Verbosef func(format string, args ...any)
	// DryRun reports whether --dry-run is enabled.
	DryRun func() bool
	// ProjectRoot resolves the working project root directory.
	ProjectRoot func() string
	// GoalsPath resolves the declared goals file path, for modules that read it.
	GoalsPath func() string
	// LedgerPath resolves the provenance ledger path for the current tree.
	LedgerPath func() string
	// Version supplies the build-time tool version string (ldflags-injected).
	Version func() string
	// Now supplies the wall clock (injected so modules never call time.Now).
	Now func() time.Time
	// EnrichFlagErr decorates a flag-parse error with a suggestion before the
	// root surfaces it.
	EnrichFlagErr func(*cobra.Command, error) error
}

// ExitError carries a process exit status and message from a command module up
// to the root executable, which owns process-code mapping. Modules return it
// instead of calling os.Exit so a fresh context can observe the outcome. It is
// the single shared exit-error type; the doctor and gate families previously
// each declared a byte-identical copy.
type ExitError struct {
	// Code is the process exit status.
	Code int
	// Message is the human-readable failure reason.
	Message string
	// Label, when non-empty, is the stderr prefix the root prints for a
	// non-zero failure (e.g. "ao doctor"). An empty Label means the module
	// already surfaced its own output and the root stays silent — the behavior
	// the gate family relies on.
	Label string
}

// Error implements the error interface.
func (failure *ExitError) Error() string { return failure.Message }

// ExitCode returns the process exit status the root should use.
func (failure *ExitError) ExitCode() int { return failure.Code }

// WriteJSON marshals value as two-space-indented JSON with a single trailing
// newline to w. The emitted bytes are identical to both
// json.MarshalIndent(value, "", "  ") followed by a newline and a
// json.Encoder configured with SetIndent("", "  ") — encoding/json HTML
// escaping stays on — so it is a byte-for-byte drop-in for every command
// module's prior hand-rolled JSON writer.
//
// WriteJSON has a YAML sibling (WriteYAML) beside it so a caller can switch on
// HostOptions.OutputMode without re-rolling an encoder.
func WriteJSON(w io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

// WriteYAML marshals value as YAML to w. It marshals through a JSON round-trip
// so the emitted YAML keys mirror the value's json struct tags exactly
// (schema_version, not the field-name-lowercased schemaversion that a direct
// yaml.Marshal of a json-tagged struct would produce) and the emitted tree is
// semantically identical to WriteJSON's for the same value. This is what makes
// `-o yaml` a truthful sibling of `-o json` everywhere json is implemented:
// the same payload, the same keys, the same values — only the serialization
// differs. yaml.v3 renders the generic tree in map-key order (alphabetical)
// with a single trailing newline; integral numbers stay integral (5, not 5.0).
func WriteYAML(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return JSONToYAML(w, data)
}

// JSONToYAML converts a single JSON document to YAML and writes it to w. It is
// the seam a command uses to add `-o yaml` without re-plumbing an app package
// that already emits JSON: capture the existing JSON bytes into a buffer, hand
// them here, and emit the YAML equivalent. The bytes must be one JSON document
// (not JSONL); a decode failure is returned so a caller never silently emits
// malformed YAML.
func JSONToYAML(w io.Writer, jsonData []byte) error {
	var generic any
	if err := json.Unmarshal(jsonData, &generic); err != nil {
		return fmt.Errorf("decode json for yaml conversion: %w", err)
	}
	out, err := yaml.Marshal(generic)
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}

// EmitStructured writes value as JSON or YAML per the negotiated output mode and
// reports whether it handled the mode. A "table" (or any non-structured) mode
// returns handled=false so the caller renders its own human view; "json" and
// "yaml" return handled=true. This is the single dispatch every command with
// structured output routes through, so `-o json` and `-o yaml` stay paired —
// there is no code path where one exists without the other.
func EmitStructured(w io.Writer, mode string, value any) (handled bool, err error) {
	switch mode {
	case "json":
		return true, WriteJSON(w, value)
	case "yaml":
		return true, WriteYAML(w, value)
	default:
		return false, nil
	}
}
