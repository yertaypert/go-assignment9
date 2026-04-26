package payment

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/yertaypert/go-assignment9/pkg/retry"
)

func ExecutePayment(
	ctx context.Context,
	client *http.Client,
	req *http.Request,
	maxAttempts int,
) (*http.Response, error) {

	var resp *http.Response
	var err error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		reqWithCtx := req.Clone(ctx)
		resp, err = client.Do(reqWithCtx)

		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		if !retry.IsRetryable(resp, err) {
			return resp, err
		}

		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}

		if attempt == maxAttempts-1 {
			break
		}

		backoff := retry.CalculateBackoff(attempt)

		log.Printf("Attempt %d failed: waiting %v...", attempt+1, backoff)

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return resp, fmt.Errorf("payment failed after %d attempts: %w", maxAttempts, err)
}
