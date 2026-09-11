package scheduler

import (
	"context"
	"time"

	"github.com/rawizhere/gosift/internal/randutil"
)

// Run calls fn every interval after a random start delay, until ctx is done.
func Run(ctx context.Context, interval, startJitter time.Duration, fn func()) error {
	if startJitter > 0 {
		timer := time.NewTimer(randutil.Duration(startJitter))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			fn()
		}
	}
}
