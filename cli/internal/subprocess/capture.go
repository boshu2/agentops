package subprocess

import (
	"fmt"
	"math"
	"sync"
)

// CaptureLimit is a hard in-memory bound for one subprocess stream. The first
// HeadBytes and last TailBytes are retained while all bytes are counted.
type CaptureLimit struct {
	HeadBytes int
	TailBytes int
}

func (limit CaptureLimit) normalized() CaptureLimit {
	if limit.HeadBytes < 0 {
		limit.HeadBytes = 0
	}
	if limit.TailBytes < 0 {
		limit.TailBytes = 0
	}
	if limit.HeadBytes == 0 && limit.TailBytes == 0 {
		return CaptureLimit{HeadBytes: 512 * 1024, TailBytes: 512 * 1024}
	}
	return limit
}

// Output is an immutable snapshot of a bounded capture.
type Output struct {
	Prefix     []byte
	Suffix     []byte
	TotalBytes int64
	Truncated  bool
}

// RetainedBytes reports the bytes held in memory for this output.
func (output Output) RetainedBytes() int {
	return len(output.Prefix) + len(output.Suffix)
}

// Bytes returns the retained bytes without a truncation marker.
func (output Output) Bytes() []byte {
	joined := make([]byte, 0, output.RetainedBytes())
	joined = append(joined, output.Prefix...)
	return append(joined, output.Suffix...)
}

// String returns the retained output and inserts explicit byte-count telemetry
// between the prefix and suffix when bytes were omitted.
func (output Output) String() string {
	if !output.Truncated {
		return string(output.Bytes())
	}
	omitted := output.TotalBytes - int64(output.RetainedBytes())
	return string(output.Prefix) +
		fmt.Sprintf("\n…[%d bytes omitted; %d/%d bytes retained]…\n", omitted, output.RetainedBytes(), output.TotalBytes) +
		string(output.Suffix)
}

type capture struct {
	mu         sync.Mutex
	limit      CaptureLimit
	prefix     []byte
	suffix     []byte
	suffixPos  int
	suffixFull bool
	total      int64
}

func newCapture(limit CaptureLimit) *capture {
	limit = limit.normalized()
	return &capture{
		limit:  limit,
		prefix: make([]byte, 0, limit.HeadBytes),
		suffix: make([]byte, 0, limit.TailBytes),
	}
}

func (capture *capture) Write(data []byte) (int, error) {
	capture.mu.Lock()
	defer capture.mu.Unlock()

	written := len(data)
	if int64(written) > math.MaxInt64-capture.total {
		capture.total = math.MaxInt64
	} else {
		capture.total += int64(written)
	}

	if remaining := capture.limit.HeadBytes - len(capture.prefix); remaining > 0 {
		take := min(remaining, len(data))
		capture.prefix = append(capture.prefix, data[:take]...)
		data = data[take:]
	}
	capture.writeSuffix(data)
	return written, nil
}

func (capture *capture) writeSuffix(data []byte) {
	limit := capture.limit.TailBytes
	if limit == 0 || len(data) == 0 {
		return
	}
	if len(data) >= limit {
		if cap(capture.suffix) < limit {
			capture.suffix = make([]byte, limit)
		} else {
			capture.suffix = capture.suffix[:limit]
		}
		copy(capture.suffix, data[len(data)-limit:])
		capture.suffixPos = 0
		capture.suffixFull = true
		return
	}
	if !capture.suffixFull {
		space := limit - len(capture.suffix)
		take := min(space, len(data))
		capture.suffix = append(capture.suffix, data[:take]...)
		data = data[take:]
		if len(capture.suffix) == limit {
			capture.suffixFull = true
			capture.suffixPos = 0
		}
	}
	for len(data) > 0 {
		take := min(limit-capture.suffixPos, len(data))
		copy(capture.suffix[capture.suffixPos:], data[:take])
		capture.suffixPos = (capture.suffixPos + take) % limit
		data = data[take:]
	}
}

func (capture *capture) snapshot() Output {
	capture.mu.Lock()
	defer capture.mu.Unlock()

	prefix := append([]byte(nil), capture.prefix...)
	var suffix []byte
	if !capture.suffixFull || capture.suffixPos == 0 {
		suffix = append([]byte(nil), capture.suffix...)
	} else {
		suffix = make([]byte, 0, len(capture.suffix))
		suffix = append(suffix, capture.suffix[capture.suffixPos:]...)
		suffix = append(suffix, capture.suffix[:capture.suffixPos]...)
	}
	retained := int64(len(prefix) + len(suffix))
	return Output{
		Prefix:     prefix,
		Suffix:     suffix,
		TotalBytes: capture.total,
		Truncated:  capture.total > retained,
	}
}
