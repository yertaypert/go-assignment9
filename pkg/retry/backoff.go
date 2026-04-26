package retry

import (
	"math/rand"
	"time"
)

const (
	baseDelay = 100 * time.Millisecond
	maxDelay  = 5 * time.Second
)

func CalculateBackoff(attempt int) time.Duration {
	delay := baseDelay * (1 << attempt)
	if delay > maxDelay {
		delay = maxDelay
	}

	minDelay := delay / 2
	if minDelay == 0 {
		return time.Duration(rand.Int63n(int64(delay)))
	}

	return minDelay + time.Duration(rand.Int63n(int64(delay-minDelay)))
}
