package awsx

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/smithy-go"
)

type apiErr struct{ code string }

func (e apiErr) Error() string                 { return e.code }
func (e apiErr) ErrorCode() string             { return e.code }
func (e apiErr) ErrorMessage() string          { return e.code }
func (e apiErr) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }

func TestIsRetryable(t *testing.T) {
	if !IsRetryable(apiErr{code: "ThrottlingException"}) {
		t.Fatal("expected throttling to be retryable")
	}
	if IsRetryable(errors.New("boom")) {
		t.Fatal("expected generic error to be non-retryable")
	}
}

func TestExecuteWithRetrySucceedsAfterThrottle(t *testing.T) {
	var calls int
	err := ExecuteWithRetry(context.Background(), func(context.Context) error {
		calls++
		if calls == 1 {
			return apiErr{code: "ThrottlingException"}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ExecuteWithRetry: %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}
