package main

import (
	"bytes"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// executeCommand runs the root command tree with args and returns everything it
// wrote to both cobra's out/err buffer and os.Stdout.
//
// BOUNDED PURPOSE (carve-out finish line, age-a-plus-report-card-ieyp2.14): this
// harness saves/restores ONLY still-existing root spine state — the five root
// persistent-flag globals (dryRun/verbose/output/jsonFlag/cfgFile) plus cobra's
// per-command Changed state. After the 12-family carve-out no module-scoped
// global remains to save here; each carved family owns its flag state
// constructor-scoped inside internal/commands/<family>, tested there. If you find
// yourself adding another save/restore line, the state you are reaching for is
// almost certainly a module global that should not exist — fix the module, not
// this helper. TestPackageVarsAreAllowlisted enforces that no new such global
// appears in cmd/ao.
func executeCommand(args ...string) (string, error) {
	originalDryRun, originalVerbose := dryRun, verbose
	originalOutput, originalJSON, originalConfig := output, jsonFlag, cfgFile
	defer func() {
		dryRun, verbose = originalDryRun, originalVerbose
		output, jsonFlag, cfgFile = originalOutput, originalJSON, originalConfig
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()
	dryRun, verbose, jsonFlag = false, false, false
	output, cfgFile = "table", ""
	resetFlagChangesRecursive(rootCmd)

	var commandOutput bytes.Buffer
	rootCmd.SetOut(&commandOutput)
	rootCmd.SetErr(&commandOutput)
	rootCmd.SetArgs(args)

	originalStdout := os.Stdout
	reader, writer, pipeErr := os.Pipe()
	if pipeErr != nil {
		return "", pipeErr
	}
	os.Stdout = writer
	defer func() { os.Stdout = originalStdout }()
	var stdout bytes.Buffer
	copyDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stdout, reader)
		close(copyDone)
	}()

	err := rootCmd.Execute()
	_ = writer.Close()
	os.Stdout = originalStdout
	<-copyDone
	_ = reader.Close()
	return commandOutput.String() + stdout.String(), err
}

func resetFlagChangesRecursive(command *cobra.Command) {
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		flag.Changed = false
		_ = flag.Value.Set(flag.DefValue)
	})
	for _, child := range command.Commands() {
		resetFlagChangesRecursive(child)
	}
}
