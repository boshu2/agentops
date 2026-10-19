package doctor

import (
	"fmt"

	"github.com/spf13/cobra"

	doctorapp "github.com/boshu2/agentops/cli/internal/doctor"
)

// A failed Fix must not emit the normal report: its OK and ExitCode describe
// the fixers and can indicate success even when saving run artifacts failed.
// Keep only the returned recovery context, without asserting files survived.
type fixFailureOutput struct {
	OK       bool               `json:"ok"`
	Error    string             `json:"error"`
	ExitCode int                `json:"exit_code"`
	Recovery *fixRecoveryOutput `json:"recovery,omitempty"`
}

type fixRecoveryOutput struct {
	RunID        string   `json:"run_id,omitempty"`
	RunDir       string   `json:"run_dir,omitempty"`
	ActionsTaken int      `json:"actions_taken"`
	ActionsPath  string   `json:"actions_jsonl_path,omitempty"`
	BackupsDir   string   `json:"backups_dir,omitempty"`
	UndoCommand  string   `json:"undo_command,omitempty"`
	Notes        []string `json:"notes"`
}

func (module Module) presentFixFailure(command *cobra.Command, report *doctorapp.Report, primary error, structured bool) error {
	failure := fixFailureOutput{Error: primary.Error(), ExitCode: doctorapp.ExitIOError}
	if report != nil {
		failure.Recovery = &fixRecoveryOutput{
			RunID: report.RunID, RunDir: report.RunDir, ActionsTaken: report.ActionsTaken,
			ActionsPath: report.ActionsPath, BackupsDir: report.BackupsDir, UndoCommand: report.UndoCommand,
			Notes: []string{
				"Changes may already have occurred. Run artifacts may be incomplete; these fields come from the in-memory report, not verified saved artifacts.",
				"Inspect live affected files and verify surviving exact-run artifacts, actions journal, and backups before retrying Fix or choosing Undo. Preserve surviving evidence; the reported action count does not establish complete recovery.",
				"If the exact run and required backups are verified and Undo is appropriate, preview that exact run with --dry-run --strict and review the preview before any real undo. Missing or inconsistent evidence requires manual reconciliation.",
			},
		}
	}
	if structured {
		if err := module.emitStructured(command, failure); err != nil {
			// An output failure must not replace the original mutation/persistence
			// error, which is still surfaced by the executable with exit 74.
			fmt.Fprintf(command.ErrOrStderr(), "ao doctor: could not write recovery output: %v\n", err)
		}
	} else if recovery := failure.Recovery; recovery != nil {
		renderFixRecovery(command, recovery)
	}
	return exit(doctorapp.ExitIOError, primary.Error())
}

func renderFixRecovery(command *cobra.Command, recovery *fixRecoveryOutput) {
	w := command.ErrOrStderr()
	for _, note := range recovery.Notes {
		fmt.Fprintln(w, note)
	}
	for _, field := range []struct{ label, value string }{
		{"run id", recovery.RunID},
		{"run directory", recovery.RunDir},
		{"actions journal", recovery.ActionsPath},
		{"backups directory", recovery.BackupsDir},
		{"undo reference (inspect before use)", recovery.UndoCommand},
	} {
		if field.value != "" {
			fmt.Fprintf(w, "  %s: %s\n", field.label, field.value)
		}
	}
	fmt.Fprintf(w, "  reported actions: %d\n", recovery.ActionsTaken)
}
