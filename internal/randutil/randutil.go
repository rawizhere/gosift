package randutil

import (
	"math/rand/v2"
	"time"
)

func Duration(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(max)))
}
