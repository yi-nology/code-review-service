package review_service

import (
	"errors"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/runner"
	mw "github.com/oekit/middleware"
)

func TestMapReviewErrorMapsRunnerErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
	}{
		{name: "binary missing", err: &runner.BinaryMissingError{Binary: "ocr", Err: errors.New("not found")}, code: 400},
		{name: "timeout", err: &runner.TimeoutError{Timeout: time.Minute}, code: 504},
		{name: "exit", err: &runner.ExitError{Stderr: "llm gateway failed", Err: errors.New("exit status 1")}, code: 502},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := mapReviewError(tt.err)
			appErr, ok := mapped.(*mw.BizError)
			if !ok {
				t.Fatalf("mapReviewError() = %T, want *mw.BizError", mapped)
			}
			if appErr.Code != tt.code {
				t.Fatalf("code = %d, want %d", appErr.Code, tt.code)
			}
			if appErr.Message == "" {
				t.Fatalf("message is empty")
			}
		})
	}
}
