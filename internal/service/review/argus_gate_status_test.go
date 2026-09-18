package review

import (
	"context"
	"strings"
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/statuscheck"
)

// capturePoster 捕获最近一次回写的 state/description，验证门禁模式分流。
type capturePoster struct {
	state       string
	description string
}

func (c *capturePoster) Post(_ context.Context, _, _, _, state, description, _ string) {
	c.state, c.description = state, description
}

func TestPostArgusGateStatusModes(t *testing.T) {
	requestChangesGate := domain.CompletedGate(false, "argus: request_changes")
	passedGate := domain.CompletedGate(true, "argus: approve")
	errorGate := domain.ErrorGate("argus: boom")

	tests := []struct {
		name       string
		gateMode   string
		status     string
		gate       domain.Gate
		wantState  string
		wantPrefix bool
	}{
		{
			name: "weak (auto default) request_changes stays success with prefix",
			gateMode:  "",
			status:    statusSuccess,
			gate:      requestChangesGate,
			wantState: statuscheck.StateSuccess, wantPrefix: true,
		},
		{
			name: "weak execution error stays success with prefix",
			gateMode:  domain.GateModeWeak,
			status:    statusFailed,
			gate:      errorGate,
			wantState: statuscheck.StateSuccess, wantPrefix: true,
		},
		{
			name: "strong request_changes fails the gate",
			gateMode:  domain.GateModeStrong,
			status:    statusSuccess,
			gate:      requestChangesGate,
			wantState: statuscheck.StateFailed, wantPrefix: false,
		},
		{
			name: "strong execution error posts error state",
			gateMode:  domain.GateModeStrong,
			status:    statusFailed,
			gate:      errorGate,
			wantState: statuscheck.StateError, wantPrefix: false,
		},
		{
			name: "strong passed stays success without prefix",
			gateMode:  domain.GateModeStrong,
			status:    statusSuccess,
			gate:      passedGate,
			wantState: statuscheck.StateSuccess, wantPrefix: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poster := &capturePoster{}
			postArgusGateStatus(context.Background(), poster, "gitlab", "obs/demo", "4a397b4",
				tt.gateMode, tt.status, tt.gate, "argus 裁决：要求修改（H1 M0 L0）", "rev-1")
			if poster.state != tt.wantState {
				t.Errorf("state = %q, want %q", poster.state, tt.wantState)
			}
			if got := strings.HasPrefix(poster.description, domain.WeakGatePrefix); got != tt.wantPrefix {
				t.Errorf("description %q prefix = %v, want %v", poster.description, got, tt.wantPrefix)
			}
		})
	}
}
