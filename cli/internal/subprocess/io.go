package subprocess

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// MaxStdinBytes is the largest owned stdin snapshot accepted by Run.
const MaxStdinBytes = 1 << 20

type stdinRelayResult struct {
	copyErr  error
	closeErr error
}

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

	stdinBytes       []byte
	stdinChildReader *os.File
	stdinWriter      *os.File
	stdinDone        chan stdinRelayResult

	outputs []*ownedOutputPipe

	stdoutCapture   *capture
	stderrCapture   *capture
	combinedCapture *capture

	allFiles []*os.File
	labels   map[*os.File]string
	closed   map[*os.File]bool
}

func newOwnedCommandIO(cmd *exec.Cmd, command Command) (state *ownedCommandIO, returnErr error) {
	if len(command.Stdin) > MaxStdinBytes {
		return nil, fmt.Errorf(
			"child stdin is %d bytes; maximum is %d",
			len(command.Stdin),
			MaxStdinBytes,
		)
	}
	state = &ownedCommandIO{
		stdinBytes: bytes.Clone(command.Stdin),
		labels:     make(map[*os.File]string),
		closed:     make(map[*os.File]bool),
	}
	defer func(owned *ownedCommandIO) {
		if returnErr != nil {
			returnErr = errors.Join(returnErr, owned.closeAll())
			state = nil
		}
	}(state)

	if len(command.Stdin) == 0 {
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
	if len(state.stdinBytes) == 0 || state.stdinWriter == nil {
		return
	}
	state.stdinDone = make(chan stdinRelayResult, 1)
	go func() {
		_, copyErr := io.Copy(state.stdinWriter, bytes.NewReader(state.stdinBytes))
		closeErr := state.closeFile(state.stdinWriter)
		state.stdinDone <- stdinRelayResult{
			copyErr:  copyErr,
			closeErr: closeErr,
		}
	}()
}

func (state *ownedCommandIO) closeBeforeStart() error {
	return state.closeAll()
}

func (state *ownedCommandIO) abortAfterUnprovenTermination(timeout time.Duration) error {
	timeout = normalizedWaitDelay(timeout)
	deadline := time.Now().Add(timeout)
	var cleanupErr error
	cleanupErr = errors.Join(cleanupErr, state.closeFile(state.stdinWriter))
	for _, output := range state.outputs {
		cleanupErr = errors.Join(cleanupErr, state.closeFile(output.reader))
	}
	cleanupErr = errors.Join(cleanupErr, state.awaitStdinRelay(deadline, timeout))
	cleanupErr = errors.Join(cleanupErr, state.awaitOutputDrains(deadline, timeout, true))
	cleanupErr = errors.Join(cleanupErr, state.closeAll())
	return cleanupErr
}

func (state *ownedCommandIO) finishAfterWait(timeout time.Duration) error {
	timeout = normalizedWaitDelay(timeout)
	deadline := time.Now().Add(timeout)
	var cleanupErr error
	cleanupErr = errors.Join(cleanupErr, state.closeFile(state.stdinWriter))
	cleanupErr = errors.Join(cleanupErr, state.awaitStdinRelay(deadline, timeout))
	cleanupErr = errors.Join(cleanupErr, state.awaitOutputDrains(deadline, timeout, false))
	cleanupErr = errors.Join(cleanupErr, state.closeAll())
	return cleanupErr
}

func normalizedWaitDelay(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return defaultWaitDelay
	}
	return timeout
}

func (state *ownedCommandIO) awaitOutputDrains(
	overallDeadline time.Time,
	timeout time.Duration,
	forced bool,
) error {
	var drainErr error
	started := time.Now()
	deadline := overallDeadline
	if !forced {
		// Reserve half of the caller's bound for forced pipe closure and
		// joining if a descendant keeps a writer open.
		deadline = started.Add(time.Until(overallDeadline) / 2)
	}

	for _, output := range state.outputs {
		for !output.collected {
			select {
			case err := <-output.done:
				output.collected = true
				if err != nil && (!forced || !errors.Is(err, os.ErrClosed)) {
					drainErr = errors.Join(drainErr, fmt.Errorf("drain child %s: %w", output.name, err))
				}
				drainErr = errors.Join(drainErr, state.closeFile(output.reader))
				continue
			default:
			}
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

func (state *ownedCommandIO) awaitStdinRelay(deadline time.Time, timeout time.Duration) error {
	if state.stdinDone == nil {
		return nil
	}
	collect := func(result stdinRelayResult) error {
		var relayErr error
		if result.copyErr != nil &&
			!errors.Is(result.copyErr, os.ErrClosed) &&
			!isIgnorableStdinCopyError(result.copyErr) {
			relayErr = errors.Join(relayErr, fmt.Errorf("copy child stdin: %w", result.copyErr))
		}
		if result.closeErr != nil {
			relayErr = errors.Join(relayErr, result.closeErr)
		}
		return relayErr
	}
	select {
	case result := <-state.stdinDone:
		return collect(result)
	default:
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return fmt.Errorf("stdin relay did not stop within %s after pipe closure", timeout)
	}
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	select {
	case result := <-state.stdinDone:
		return collect(result)
	case <-timer.C:
		return fmt.Errorf("stdin relay did not stop within %s after pipe closure", timeout)
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
