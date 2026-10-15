// Package lease renews active tasks and cancels their work on lease loss.
package lease

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrLost = errors.New("task lease lost")

// Run checks ownership before work. Work must observe cancellation. Durable
// completion must also check its token and expiry; renewal is not a fence.
func Run(ctx context.Context, duration time.Duration, renew func(context.Context) error, work func(context.Context) error) error {
	interval := max(duration/3, time.Millisecond)
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	check := func() error {
		bounded, stop := context.WithTimeout(workCtx, interval)
		defer stop()
		return renew(bounded)
	}
	if err := check(); err != nil {
		return fmt.Errorf("%w: %v", ErrLost, err)
	}
	stopped := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stopped:
				done <- nil
				return
			case <-workCtx.Done():
				done <- workCtx.Err()
				return
			case <-ticker.C:
				if err := check(); err != nil {
					cancel()
					done <- fmt.Errorf("%w: %v", ErrLost, err)
					return
				}
			}
		}
	}()
	workErr := work(workCtx)
	close(stopped)
	if renewErr := <-done; renewErr != nil {
		return renewErr
	}
	return workErr
}
