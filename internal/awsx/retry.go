package awsx

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	smithy "github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

const (
	defaultMaxAttempts = 5
	defaultBaseDelay   = 200 * time.Millisecond
	defaultMaxDelay    = 5 * time.Second
)

// ExecuteWithRetry runs fn with exponential backoff when AWS returns throttling or 5xx errors.
func ExecuteWithRetry(ctx context.Context, fn func(context.Context) error) error {
	var lastErr error
	delay := defaultBaseDelay
	for attempt := 1; attempt <= defaultMaxAttempts; attempt++ {
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}
		if !IsRetryable(lastErr) || attempt == defaultMaxAttempts {
			return lastErr
		}
		jitter := time.Duration(rand.Int63n(int64(delay / 2)))
		sleep := delay + jitter
		if sleep > defaultMaxDelay {
			sleep = defaultMaxDelay
		}
		timer := time.NewTimer(sleep)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-timer.C:
		}
		delay *= 2
	}
	return lastErr
}

// IsRetryable reports whether err is a throttling or server-side AWS error.
func IsRetryable(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "ThrottlingException", "TooManyRequestsException", "ServiceUnavailable":
			return true
		}
	}
	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) {
		return respErr.HTTPStatusCode() >= 500
	}
	return false
}
