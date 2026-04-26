package retry

import (
	"math/rand"
	"time"
)

const (
	baseDelay = 500 * time.Millisecond
	maxDelay  = 10 * time.Second
)

func CalculateBackoff(attempt int) time.Duration {
	delay := baseDelay * (1 << attempt)
	if delay > maxDelay {
		delay = maxDelay
	}

	jitter := time.Duration(rand.Int63n(int64(delay / 2)))
	return delay + jitter
}
