//go:build darwin

package subprocess

import (
	"fmt"
	"os"
	"sync"
	"syscall"
)

type darwinProcessProbe struct {
	mu     sync.Mutex
	pid    int
	kqueue int
	closed bool
}

func newProcessCompletion(process *os.Process) processCompletion {
	probe := &darwinProcessProbe{
		pid:    process.Pid,
		kqueue: -1,
	}
	return &pollingProcessCompletion{
		probe:   probe.completed,
		closeFn: probe.close,
	}
}

func (probe *darwinProcessProbe) completed() (bool, error) {
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if probe.closed {
		return false, fmt.Errorf("process completion observer is closed")
	}
	if probe.kqueue < 0 {
		kqueue, err := syscall.Kqueue()
		if err != nil {
			return false, fmt.Errorf("create process completion kqueue: %w", err)
		}
		event := syscall.Kevent_t{}
		syscall.SetKevent(
			&event,
			probe.pid,
			syscall.EVFILT_PROC,
			syscall.EV_ADD|syscall.EV_ENABLE|syscall.EV_ONESHOT,
		)
		event.Fflags = syscall.NOTE_EXIT
		if _, err := syscall.Kevent(kqueue, []syscall.Kevent_t{event}, nil, nil); err != nil {
			_ = syscall.Close(kqueue)
			return false, fmt.Errorf("register process completion event for pid %d: %w", probe.pid, err)
		}
		probe.kqueue = kqueue
	}

	events := make([]syscall.Kevent_t, 1)
	timeout := syscall.NsecToTimespec(0)
	count, err := syscall.Kevent(probe.kqueue, nil, events, &timeout)
	if err != nil {
		if err == syscall.EINTR {
			return false, nil
		}
		return false, fmt.Errorf("observe process completion for pid %d: %w", probe.pid, err)
	}
	if count == 0 {
		return false, nil
	}
	event := events[0]
	if event.Flags&syscall.EV_ERROR != 0 && event.Data != 0 {
		return false, fmt.Errorf(
			"observe process completion for pid %d: %w",
			probe.pid,
			syscall.Errno(event.Data),
		)
	}
	return event.Fflags&syscall.NOTE_EXIT != 0 || event.Flags&syscall.EV_EOF != 0, nil
}

func (probe *darwinProcessProbe) close() error {
	probe.mu.Lock()
	defer probe.mu.Unlock()
	if probe.closed {
		return nil
	}
	probe.closed = true
	if probe.kqueue < 0 {
		return nil
	}
	if err := syscall.Close(probe.kqueue); err != nil {
		return fmt.Errorf("close process completion kqueue: %w", err)
	}
	probe.kqueue = -1
	return nil
}
