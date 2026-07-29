// Package procrun runs child subprocesses with a bounded output capture, a
// caller context that is honored end-to-end, and process-group termination so
// that cancelling or timing out a run reaps the whole descendant tree rather
// than orphaning grandchildren.
//
// # Bounded capture semantics
//
// The output writer retains at most a fixed head+tail window of the child's
// stream and DISCARDS the middle, so peak retained memory is bounded to
// head+tail+len(marker) no matter how many bytes a runaway child emits. The
// runner never stops reading the child (it keeps draining the pipe so the child
// cannot deadlock on a full pipe) — it simply forgets the middle. When any
// bytes were discarded the assembled output is `head + truncationMarker + tail`
// and Result.Truncated is true; otherwise the output is the child's exact bytes.
//
// Because the retained window is generous (DefaultMaxCaptureBytes) and every
// downstream truncation window in the callers (goals' 500-rune head+tail, gates'
// 4 KiB tail) is far smaller, any output that fits within the cap is returned
// byte-for-byte, and even oversize output preserves the true head and true tail
// — so a caller that keeps the last N bytes still sees the real last N bytes.
//
// # Cancellation
//
// Run relies on the standard library's context-driven cancellation: callers
// MUST build cmd with exec.CommandContext(ctx, ...) using the SAME ctx they pass
// to Run. Run installs a process-group Cancel (unix: kill(-pgid); windows:
// taskkill /T) and a WaitDelay so that when ctx is cancelled or its deadline
// expires the entire group is killed and Wait cannot hang forever on pipes still
// held open by a surviving grandchild. Run itself starts no background
// goroutine, so it adds no goroutine-leak surface.
package procrun

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// DefaultMaxCaptureBytes is the default per-stream retained output ceiling
// (split evenly between a head window and a tail window). One mebibyte is far
// above any legitimate check output yet bounds peak memory hard.
const DefaultMaxCaptureBytes = 1 << 20

// DefaultWaitDelay bounds how long Wait blocks draining inherited pipes after
// the context is cancelled or the process exits, before the pipes are force
// closed. It matches the proven value used by the goals measurement path.
const DefaultWaitDelay = 3 * time.Second

// truncationMarker is inserted between the retained head and tail when middle
// bytes were discarded. It is intentionally distinct and self-describing so a
// human reading truncated output can tell capture bounding occurred.
const truncationMarker = "\n…[procrun: output truncated]…\n"

// Options configures a Run.
type Options struct {
	// MaxCaptureBytes caps retained output per stream (head+tail combined).
	// Values <= 0 select DefaultMaxCaptureBytes.
	MaxCaptureBytes int
	// Combined routes both stdout and stderr into a single capture surfaced as
	// Result.Combined (Result.Stdout/Stderr stay nil). When false the two
	// streams are captured separately.
	Combined bool
	// WaitDelay overrides the post-cancellation pipe-drain bound. Values <= 0
	// select DefaultWaitDelay.
	WaitDelay time.Duration
	// OnStart, if set, is called with the child PID immediately after a
	// successful Start (before Wait). Used by callers that track live children
	// for signal-driven cleanup.
	OnStart func(pid int)
	// OnExit, if set, is called with the child PID immediately after Wait
	// returns.
	OnExit func(pid int)
}

// Result holds the outcome of a Run.
type Result struct {
	// Stdout and Stderr hold the separately captured streams (nil when the run
	// used Options.Combined).
	Stdout, Stderr []byte
	// Combined holds the merged stdout+stderr stream (nil unless Options.Combined).
	Combined []byte
	// ExitCode is 0 on success, the child's exit status for a normal non-zero
	// exit, and -1 when the child failed to run or was terminated by a signal.
	ExitCode int
	// Err is the raw error returned by Wait: nil, an *exec.ExitError for a
	// non-zero exit, or another error for an infrastructure failure. Callers
	// classify timeout/cancellation from their own ctx.Err(), which takes
	// precedence over Err.
	Err error
	// Truncated reports whether any captured stream discarded middle bytes.
	Truncated bool
	// Duration is the wall-clock time from Start to Wait return.
	Duration time.Duration
}

// Run starts cmd under ctx, captures its output within a hard byte ceiling, and
// waits for it to finish, killing the child's whole process group on
// cancellation or timeout.
//
// cmd MUST have been created with exec.CommandContext(ctx, ...) using the same
// ctx passed here; Run wires the process-group Cancel and WaitDelay onto it and
// sets cmd.Stdout/cmd.Stderr. The returned error is non-nil only when the child
// could not be started; a child that starts and then fails reports through
// Result.Err and Result.ExitCode.
func Run(ctx context.Context, cmd *exec.Cmd, opts Options) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	maxBytes := opts.MaxCaptureBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxCaptureBytes
	}
	head := maxBytes / 2
	tail := maxBytes - head

	var stdoutCap, stderrCap, combinedCap *capture
	if opts.Combined {
		combinedCap = newCapture(head, tail)
		cmd.Stdout = combinedCap
		cmd.Stderr = combinedCap
	} else {
		stdoutCap = newCapture(head, tail)
		stderrCap = newCapture(head, tail)
		cmd.Stdout = stdoutCap
		cmd.Stderr = stderrCap
	}

	configureProcessGroup(cmd)
	waitDelay := opts.WaitDelay
	if waitDelay <= 0 {
		waitDelay = DefaultWaitDelay
	}
	cmd.WaitDelay = waitDelay

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return Result{Duration: time.Since(start)}, fmt.Errorf("start command: %w", err)
	}
	pid := cmd.Process.Pid
	if opts.OnStart != nil {
		opts.OnStart(pid)
	}

	waitErr := cmd.Wait()
	if opts.OnExit != nil {
		opts.OnExit(pid)
	}

	res := Result{Duration: time.Since(start), Err: waitErr}
	if combinedCap != nil {
		res.Combined = combinedCap.Bytes()
		res.Truncated = combinedCap.Truncated()
	} else {
		res.Stdout = stdoutCap.Bytes()
		res.Stderr = stderrCap.Bytes()
		res.Truncated = stdoutCap.Truncated() || stderrCap.Truncated()
	}

	var exitErr *exec.ExitError
	switch {
	case waitErr == nil:
		res.ExitCode = 0
	case errors.As(waitErr, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		res.ExitCode = -1
	}
	return res, nil
}

// capture is an io.Writer that retains at most headCap bytes of the beginning
// and tailCap bytes of the end of the stream written to it, discarding whatever
// falls in between. Retained memory is bounded to headCap+tailCap regardless of
// total bytes written.
//
// os/exec serializes writes to a single writer (one copy goroutine per pipe;
// stdout and stderr routed to the same writer share one pipe), so the mutex only
// guards against callers that deliberately share a capture across goroutines and
// keeps the race detector satisfied unconditionally.
type capture struct {
	mu      sync.Mutex
	headCap int
	tailCap int
	head    []byte // first headCap bytes, in order
	ring    []byte // fixed-size ring of the last tailCap post-head bytes
	ringLen int    // valid bytes in ring (<= tailCap)
	ringPos int    // next write index into ring
	total   int64  // total bytes written
}

func newCapture(headCap, tailCap int) *capture {
	if headCap < 0 {
		headCap = 0
	}
	if tailCap < 0 {
		tailCap = 0
	}
	return &capture{
		headCap: headCap,
		tailCap: tailCap,
		head:    make([]byte, 0, headCap),
	}
}

// Write implements io.Writer. It never returns an error and always reports the
// full length as written so the child is never back-pressured by the cap.
func (c *capture) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(p)
	c.total += int64(n)

	if len(c.head) < c.headCap {
		take := c.headCap - len(c.head)
		if take > len(p) {
			take = len(p)
		}
		c.head = append(c.head, p[:take]...)
		p = p[take:]
	}
	if len(p) == 0 || c.tailCap == 0 {
		return n, nil
	}
	if c.ring == nil {
		c.ring = make([]byte, c.tailCap)
	}
	// If this write alone exceeds the tail window, only its last tailCap bytes
	// can survive; drop straight to them.
	if len(p) >= c.tailCap {
		copy(c.ring, p[len(p)-c.tailCap:])
		c.ringPos = 0
		c.ringLen = c.tailCap
		return n, nil
	}
	for _, b := range p {
		c.ring[c.ringPos] = b
		c.ringPos = (c.ringPos + 1) % c.tailCap
		if c.ringLen < c.tailCap {
			c.ringLen++
		}
	}
	return n, nil
}

// Truncated reports whether any middle bytes were discarded.
func (c *capture) Truncated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total > int64(c.headCap+c.tailCap)
}

// Total returns the total number of bytes written, including discarded ones.
func (c *capture) Total() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total
}

// Bytes assembles the retained output: the exact stream when nothing was
// discarded, otherwise head + truncationMarker + tail.
func (c *capture) Bytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ringLen == 0 {
		return append([]byte(nil), c.head...)
	}
	tail := make([]byte, c.ringLen)
	if c.ringLen < c.tailCap {
		// Never wrapped: bytes sit at ring[0:ringLen] in order.
		copy(tail, c.ring[:c.ringLen])
	} else {
		// Wrapped: oldest retained byte is at ringPos.
		copy(tail, c.ring[c.ringPos:])
		copy(tail[c.tailCap-c.ringPos:], c.ring[:c.ringPos])
	}
	truncated := c.total > int64(c.headCap+c.tailCap)
	out := make([]byte, 0, len(c.head)+len(tail)+len(truncationMarker))
	out = append(out, c.head...)
	if truncated {
		out = append(out, truncationMarker...)
	}
	out = append(out, tail...)
	return out
}
