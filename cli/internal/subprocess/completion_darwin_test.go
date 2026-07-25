//go:build darwin

package subprocess

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"unsafe"
)

func TestDarwinSiginfoChildLayout(t *testing.T) {
	var info darwinSiginfoChild
	if got, want := unsafe.Sizeof(info), uintptr(104); got != want {
		t.Fatalf("darwin siginfo size = %d, want %d", got, want)
	}
	if got, want := unsafe.Offsetof(info.pid), uintptr(12); got != want {
		t.Fatalf("darwin siginfo pid offset = %d, want %d", got, want)
	}
	if got, want := unsafe.Offsetof(info.status), uintptr(20); got != want {
		t.Fatalf("darwin siginfo status offset = %d, want %d", got, want)
	}
}

func TestRunDarwinFastExitUnderLoad(t *testing.T) {
	const (
		workers    = 96
		iterations = 4
	)
	var (
		wg       sync.WaitGroup
		failOnce sync.Once
		first    string
	)
	wantCleanup := CleanupOutcome{
		Status:    CleanupCompleted,
		Attempted: true,
		Completed: true,
	}
	start := make(chan struct{})
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for iteration := range iterations {
				result, err := Run(context.Background(), Command{
					Name: "/bin/bash",
					Args: []string{"--noprofile", "--norc", "-c", "exit 0"},
				})
				if err == nil && result.ExitCode == 0 && result.Cleanup == wantCleanup {
					continue
				}
				failOnce.Do(func() {
					first = fmt.Sprintf(
						"worker=%d iteration=%d exit=%d cleanup=%#v err=%v",
						worker,
						iteration,
						result.ExitCode,
						result.Cleanup,
						err,
					)
				})
				return
			}
		}()
	}
	close(start)
	wg.Wait()
	if first != "" {
		t.Fatal(first)
	}
}
