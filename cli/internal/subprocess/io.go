package subprocess

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

type ownedOutputPipe struct {
	name        string
	reader      *os.File
	childWriter *os.File
	capture     *capture
	done        chan error
	collected   bool
}

// ownedCommandIO gives AgentOps explicit ownership of every file descriptor
// installed on exec.Cmd. Because Cmd sees only *os.File values, os/exec does
// not create copy goroutines or private pipes that require Cmd.Wait to close.
type ownedCommandIO struct {
	mu sync.Mutex

	stdinSource      io.Reader
	stdinChildReader *os.File
	stdinWriter      *os.File
	stdinDone        chan error

	outputs []*ownedOutputPipe

	stdoutCapture   *capture
	stderrCapture   *capture
	combinedCapture *capture

	allFiles []*os.File
	labels   map[*os.File]string
	closed   map[*os.File]bool
}

func newOwnedCommandIO(cmd *exec.Cmd, command Command) (state *ownedCommandIO, returnErr error) {
	state = &ownedCommandIO{
		stdinSource: command.Stdin,
		labels:      make(map[*os.File]string),
		closed:      make(map[*os.File]bool),
	}
	defer func() {
		if returnErr != nil {
			returnErr = errors.Join(returnErr, state.closeAll())
			state = nil
		}
	}()

	if command.Stdin == nil {
		stdin, err := os.Open(os.DevNull)
		if err != nil {
			return nil, fmt.Errorf("open child stdin: %w", err)
		}
		state.stdinChildReader = stdin
		state.own(stdin, "child stdin")
	} else {
		stdinReader, stdinWriter, err := os.Pipe()
		if err != nil {
			return nil, fmt.Errorf("create child stdin pipe: %w", err)
		}
		state.stdinChildReader = stdinReader
		state.stdinWriter = stdinWriter
		state.own(stdinReader, "child stdin pipe reader")
		state.own(stdinWriter, "parent stdin pipe writer")
	}
	cmd.Stdin = state.stdinChildReader

	if command.CombinedOutput {
		state.combinedCapture = newCapture(command.OutputLimit)
		output, err := state.newOutputPipe("combined output", state.combinedCapture)
		if err != nil {
			return nil, err
		}
		state.outputs = append(state.outputs, output)
		cmd.Stdout = output.childWriter
		cmd.Stderr = output.childWriter
	} else {
		state.stdoutCapture = newCapture(command.StdoutLimit)
		stdout, err := state.newOutputPipe("stdout", state.stdoutCapture)
		if err != nil {
			return nil, err
		}
		state.outputs = append(state.outputs, stdout)
		cmd.Stdout = stdout.childWriter

		state.stderrCapture = newCapture(command.StderrLimit)
		stderr, err := state.newOutputPipe("stderr", state.stderrCapture)
		if err != nil {
			return nil, err
		}
		state.outputs = append(state.outputs, stderr)
		cmd.Stderr = stderr.childWriter
	}
	return state, nil
}

func (state *ownedCommandIO) newOutputPipe(name string, capture *capture) (*ownedOutputPipe, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("create child %s pipe: %w", name, err)
	}
	state.own(reader, "parent "+name+" pipe reader")
	state.own(writer, "child "+name+" pipe writer")
	return &ownedOutputPipe{
		name:        name,
		reader:      reader,
		childWriter: writer,
		capture:     capture,
		done:        make(chan error, 1),
	}, nil
}

func (state *ownedCommandIO) own(file *os.File, label string) {
	state.allFiles = append(state.allFiles, file)
	state.labels[file] = label
}

func (state *ownedCommandIO) afterStart() error {
	var closeErr error
	closeErr = errors.Join(closeErr, state.closeFile(state.stdinChildReader))
	for _, output := range state.outputs {
		closeErr = errors.Join(closeErr, state.closeFile(output.childWriter))
		go func(output *ownedOutputPipe) {
			_, err := io.Copy(output.capture, output.reader)
			output.done <- err
		}(output)
	}
	return closeErr
}

func (state *ownedCommandIO) startStdinCopy() {
	if state.stdinSource == nil || state.stdinWriter == nil {
		return
	}
	state.stdinDone = make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(state.stdinWriter, state.stdinSource)
		closeErr := state.closeFile(state.stdinWriter)
		state.stdinDone <- errors.Join(copyErr, closeErr)
	}()
}

func (state *ownedCommandIO) closeBeforeStart() error {
	return state.closeAll()
}

func (state *ownedCommandIO) abortAfterUnprovenTermination(timeout time.Duration) error {
	var cleanupErr error
	cleanupErr = errors.Join(cleanupErr, state.closeFile(state.stdinWriter))
	for _, output := range state.outputs {
		cleanupErr = errors.Join(cleanupErr, state.closeFile(output.reader))
	}
	cleanupErr = errors.Join(cleanupErr, state.awaitOutputDrains(timeout, true))
	cleanupErr = errors.Join(cleanupErr, state.closeAll())
	return cleanupErr
}

func (state *ownedCommandIO) finishAfterWait(timeout time.Duration) error {
	var cleanupErr error
	cleanupErr = errors.Join(cleanupErr, state.closeFile(state.stdinWriter))
	cleanupErr = errors.Join(cleanupErr, state.awaitOutputDrains(timeout, false))
	cleanupErr = errors.Join(cleanupErr, state.readyStdinError())
	cleanupErr = errors.Join(cleanupErr, state.closeAll())
	return cleanupErr
}

func (state *ownedCommandIO) awaitOutputDrains(timeout time.Duration, forced bool) error {
	if timeout <= 0 {
		timeout = defaultWaitDelay
	}
	var drainErr error
	started := time.Now()
	overallDeadline := started.Add(timeout)
	deadline := overallDeadline
	if !forced {
		// Reserve half of the caller's bound for forced pipe closure and
		// joining if a descendant keeps a writer open.
		deadline = started.Add(timeout / 2)
	}

	for _, output := range state.outputs {
		for !output.collected {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				if forced {
					drainErr = errors.Join(
						drainErr,
						fmt.Errorf("%s drain did not stop within %s after pipe closure", output.name, timeout),
					)
					break
				}
				drainErr = errors.Join(
					drainErr,
					fmt.Errorf("%s drain exceeded %s: %w", output.name, timeout, exec.ErrWaitDelay),
				)
				forced = true
				for _, pending := range state.outputs {
					drainErr = errors.Join(drainErr, state.closeFile(pending.reader))
				}
				deadline = overallDeadline
				continue
			}

			timer := time.NewTimer(remaining)
			select {
			case err := <-output.done:
				timer.Stop()
				output.collected = true
				if err != nil && (!forced || !errors.Is(err, os.ErrClosed)) {
					drainErr = errors.Join(drainErr, fmt.Errorf("drain child %s: %w", output.name, err))
				}
				drainErr = errors.Join(drainErr, state.closeFile(output.reader))
			case <-timer.C:
				if forced {
					drainErr = errors.Join(
						drainErr,
						fmt.Errorf("%s drain did not stop within %s after pipe closure", output.name, timeout),
					)
					break
				}
				drainErr = errors.Join(
					drainErr,
					fmt.Errorf("%s drain exceeded %s: %w", output.name, timeout, exec.ErrWaitDelay),
				)
				forced = true
				for _, pending := range state.outputs {
					drainErr = errors.Join(drainErr, state.closeFile(pending.reader))
				}
				deadline = overallDeadline
			}
		}
	}
	return drainErr
}

func (state *ownedCommandIO) readyStdinError() error {
	if state.stdinDone == nil {
		return nil
	}
	select {
	case err := <-state.stdinDone:
		if err == nil || errors.Is(err, os.ErrClosed) || isIgnorableStdinCopyError(err) {
			return nil
		}
		return fmt.Errorf("copy child stdin: %w", err)
	default:
		return nil
	}
}

func (state *ownedCommandIO) closeAll() error {
	var closeErr error
	for _, file := range state.allFiles {
		closeErr = errors.Join(closeErr, state.closeFile(file))
	}
	return closeErr
}

func (state *ownedCommandIO) closeFile(file *os.File) error {
	if file == nil {
		return nil
	}
	state.mu.Lock()
	if state.closed[file] {
		state.mu.Unlock()
		return nil
	}
	state.closed[file] = true
	label := state.labels[file]
	state.mu.Unlock()

	if err := file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		return fmt.Errorf("close %s: %w", label, err)
	}
	return nil
}

func (state *ownedCommandIO) snapshot(result *Result) {
	snapshotResult(result, state.stdoutCapture, state.stderrCapture, state.combinedCapture)
}
