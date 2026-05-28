// practices: [distributed-systems-design, event-sourcing-cqrs]
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// PhaseProgress tracks cumulative progress while parsing a stream of
// Claude Code JSON events.
type PhaseProgress struct {
	Name          string
	SessionID     string
	Model         string
	LastToolCall  string
	CurrentAction string
	RetryCount    int
	LastError     string
	ToolCount     int
	TurnCount     int
	Tokens        int
	CostUSD       float64
	Elapsed       time.Duration
	LastUpdate    time.Time
}

// applyEventToProgress updates PhaseProgress based on the event type.
func applyEventToProgress(p *PhaseProgress, ev StreamEvent) {
	switch ev.Type {
	case EventTypeInit:
		p.SessionID = ev.SessionID
		p.Model = ev.Model
		p.CurrentAction = "initialized"
	case EventTypeAssistant:
		if ev.ToolName != "" {
			p.ToolCount++
			p.LastToolCall = ev.ToolName
			p.CurrentAction = "tool: " + ev.ToolName
			break
		}
		if ev.Message != "" {
			p.CurrentAction = summarizeStatusAction(ev.Message)
		}
	case EventTypeResult:
		p.CostUSD = ev.CostUSD
		p.TurnCount = ev.NumTurns
		if ev.DurationMS > 0 {
			p.Elapsed = time.Duration(ev.DurationMS * float64(time.Millisecond))
		}
		if ev.IsError {
			p.CurrentAction = "result error"
			if ev.Message != "" {
				p.LastError = summarizeStatusAction(ev.Message)
			} else {
				p.LastError = "result event reported error"
			}
		} else {
			p.CurrentAction = "result received"
		}
	}
}

// ParseStreamEvents reads newline-delimited JSON events from r, updating
// a PhaseProgress as it goes. If onUpdate is non-nil it is called after
// every successfully parsed event. The final PhaseProgress is returned
// along with the first non-EOF read error (malformed JSON lines are
// silently skipped so that a partial stream still yields useful data).
func ParseStreamEvents(r io.Reader, onUpdate func(PhaseProgress)) (PhaseProgress, error) {
	return ParseStreamEventsWithHandler(r, nil, onUpdate)
}

// ParseStreamEventsWithHandler is ParseStreamEvents with an additional callback
// for each parsed StreamEvent before progress is merged.
func ParseStreamEventsWithHandler(r io.Reader, onEvent func(StreamEvent), onUpdate func(PhaseProgress)) (PhaseProgress, error) {
	reader := newStreamLineReader(r)

	var p PhaseProgress

	for {
		line, readErr := reader.readLine()
		if len(line) > 0 {
			if ev, err := ParseStreamEvent(line); err == nil {
				if onEvent != nil {
					onEvent(ev)
				}
				applyEventToProgress(&p, ev)
				p.LastUpdate = time.Now()
				if onUpdate != nil {
					onUpdate(p)
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return p, readErr
		}
	}

	return p, nil
}

func summarizeStatusAction(s string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	const maxLen = 72
	runes := []rune(trimmed)
	if len(runes) <= maxLen {
		return trimmed
	}
	return string(runes[:maxLen-3]) + "..."
}

// maxStreamLineLength is the maximum number of bytes buffered for a single
// line before the reader returns an error. This prevents unbounded memory
// growth when the stream emits very long lines without newlines.
const maxStreamLineLength = 1 << 20 // 1MB

type streamLineReader struct {
	buf     []byte
	r       io.Reader
	readBuf []byte
}

func newStreamLineReader(r io.Reader) *streamLineReader {
	return &streamLineReader{
		buf: make([]byte, 0, 64*1024),
		r:   r,
	}
}

func (lr *streamLineReader) readLine() ([]byte, error) {
	for {
		if idx := bytes.IndexByte(lr.buf, '\n'); idx >= 0 {
			line := bytes.TrimSpace(lr.buf[:idx])
			lr.buf = lr.buf[idx+1:]
			return line, nil
		}

		if lr.readBuf == nil {
			lr.readBuf = make([]byte, 64*1024)
		}
		n, err := lr.r.Read(lr.readBuf)
		if n > 0 {
			lr.buf = append(lr.buf, lr.readBuf[:n]...)
		}
		if len(lr.buf) > maxStreamLineLength {
			return nil, fmt.Errorf("stream line exceeds max length (%d bytes)", maxStreamLineLength)
		}
		if err != nil {
			if err == io.EOF {
				line := bytes.TrimSpace(lr.buf)
				lr.buf = lr.buf[:0]
				return line, io.EOF
			}
			return nil, err
		}
	}
}
