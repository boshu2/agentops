package subprocess

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const processCompletionPollInterval = 10 * time.Millisecond

type processCompletion interface {
	wait(context.Context) error
	observe(time.Duration) error
	close() error
}

type pollingProcessCompletion struct {
	probe   func() (bool, error)
	closeFn func() error
}

func (completion *pollingProcessCompletion) wait(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		completed, err := completion.probe()
		if err != nil {
			return err
		}
		if completed {
			return nil
		}

		timer := time.NewTimer(processCompletionPollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (completion *pollingProcessCompletion) observe(timeout time.Duration) error {
	if timeout <= 0 {
		timeout = defaultWaitDelay
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := completion.wait(ctx); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("process completion was not observed within %s: %w", timeout, err)
		}
		return err
	}
	return nil
}

func (completion *pollingProcessCompletion) close() error {
	if completion.closeFn == nil {
		return nil
	}
	return completion.closeFn()
}
