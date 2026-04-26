package retry

import (
	"errors"
	"net"
	"net/http"
)

func IsRetryable(resp *http.Response, err error) bool {
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) {
			return netErr.Timeout() || netErr.Temporary()
		}
		return true
	}

	if resp == nil {
		return false
	}

	switch resp.StatusCode {
	case 429, 500, 502, 503, 504:
		return true
	case 401, 404:
		return false
	}

	return false
}
