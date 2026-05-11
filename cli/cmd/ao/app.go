// practices: [pragmatic-programmer, twelve-factor-app]
package main

import (
	"context"
	"crypto/rand"
	"io"
	"os"
	"os/exec"
)

// App holds shared application state that was previously scattered across
// global variables. It follows the Terraform Meta + kubectl Options hybrid
// pattern: flag values live here (replacing mutable globals), and function
// fields enable dependency injection for testing.
type App struct {
	// Flag values (wired from root persistent flags)
	DryRun  bool
	Verbose bool
	Output  string
	JSON    bool
	CfgFile string
	WorkDir string

	// Dependency injection points for testing
	ExecCommand func(name string, arg ...string) *exec.Cmd
	LookPath    func(file string) (string, error)
	RandReader  io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

// NewApp creates an App with production defaults.
func NewApp() *App {
	return &App{
		Output:      "table",
		ExecCommand: exec.Command,
		LookPath:    exec.LookPath,
		RandReader:  rand.Reader,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	}
}

// appKeyType is the context key for the App struct.
type appKeyType struct{}

var appKey = appKeyType{}

// AppFromContext retrieves the App from a cobra command's context.
func AppFromContext(ctx context.Context) *App {
	if v, ok := ctx.Value(appKey).(*App); ok {
		return v
	}
	return nil
}
