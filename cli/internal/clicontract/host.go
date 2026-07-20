package clicontract

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
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
// A YAML sibling (WriteYAML) belongs beside this helper when the eval/config
// yaml-output bead lands; keeping the structured writers in one home lets a
// caller switch on HostOptions.OutputMode without re-rolling the encoder.
func WriteJSON(w io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
