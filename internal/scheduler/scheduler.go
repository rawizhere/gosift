package scheduler

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/go-co-op/gocron/v2"
)

func Run(ctx context.Context, interval, startJitter time.Duration, fn func()) error {
	s, err := gocron.NewScheduler(gocron.WithLocation(time.UTC))
	if err != nil {
		return err
	}
	var opts []gocron.JobOption
	if startJitter > 0 {
		delay := time.Duration(rand.Int64N(int64(startJitter)))
		opts = append(opts, gocron.WithStartAt(gocron.WithStartDateTime(time.Now().Add(delay))))
	}
	if _, err := s.NewJob(gocron.DurationJob(interval), gocron.NewTask(fn), opts...); err != nil {
		return err
	}
	s.Start()
	<-ctx.Done()
	return s.Shutdown()
}
