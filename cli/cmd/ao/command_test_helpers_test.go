package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

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

func containsStr(value, fragment string) bool { return strings.Contains(value, fragment) }

func resetFlagChangesRecursive(command *cobra.Command) {
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		flag.Changed = false
		_ = flag.Value.Set(flag.DefValue)
	})
	for _, child := range command.Commands() {
		resetFlagChangesRecursive(child)
	}
}

type fakeFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (info fakeFileInfo) Name() string       { return info.name }
func (info fakeFileInfo) Size() int64        { return info.size }
func (info fakeFileInfo) Mode() os.FileMode  { return info.mode }
func (info fakeFileInfo) ModTime() time.Time { return info.modTime }
func (info fakeFileInfo) IsDir() bool        { return info.isDir }
func (info fakeFileInfo) Sys() any           { return nil }
