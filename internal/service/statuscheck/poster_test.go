package statuscheck

import (
	"context"
	"testing"

	sdk "github.com/yi-nology/git-platform-sdk/provider"
)

// captureWriter 捕获最近一次 CreateCommitStatus 的入参，校验 target_url 规则。
type captureWriter struct {
	opts sdk.CommitStatusOptions
}

func (c *captureWriter) CreateCommitStatus(_ context.Context, _, _, _, _ string, opts sdk.CommitStatusOptions) error {
	c.opts = opts
	return nil
}

// TestPostTargetURL Must 为绝对地址或省略：GitLab 对相对路径整条拒绝（HTTP 400）。
func TestPostTargetURL(t *testing.T) {
	tests := []struct {
		name       string
		base       string
		reviewID   string
		wantTarget string
	}{
		{name: "no base omits target_url", base: "", reviewID: "01M2", wantTarget: ""},
		{name: "base builds absolute review link", base: "http://review.example.com", reviewID: "01M2", wantTarget: "http://review.example.com/reviews/01M2"},
		{name: "base trailing slash trimmed", base: "http://review.example.com/", reviewID: "01M2", wantTarget: "http://review.example.com/reviews/01M2"},
		{name: "empty reviewID omits target_url even with base", base: "http://review.example.com", reviewID: "", wantTarget: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &captureWriter{}
			NewPoster(w, tt.base).Post(context.Background(), "gitlab", "obs/demo", "cc499ba1",
				StateSuccess, "argus 裁决：通过", tt.reviewID)
			if w.opts.TargetURL != tt.wantTarget {
				t.Errorf("TargetURL = %q, want %q", w.opts.TargetURL, tt.wantTarget)
			}
			if w.opts.Context != ContextName || w.opts.State != StateSuccess {
				t.Errorf("context/state = %q/%q, want %q/%q", w.opts.Context, w.opts.State, ContextName, StateSuccess)
			}
		})
	}
}
