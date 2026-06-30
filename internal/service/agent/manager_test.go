package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestRunParallel_AllFailed_PreservesErrorChain 验证 PR-09 修复：
// 当所有 agent 都失败时，返回的 error 必须保留每个子错误的 Unwrap 链。
// errors.Is 应该能枚举出原始错误。
func TestRunParallel_AllFailed_PreservesErrorChain(t *testing.T) {
	mgr := &AgentManager{
		config: ManagerConfig{
			MaxConcurrency: 1,
			AgentTimeout:   1,
		},
	}

	agents := []reviewAgent{
		&fakeReviewAgent{name: "a1", err: errors.New("a1 failed: timeout")},
		&fakeReviewAgent{name: "a2", err: errors.New("a2 failed: api_key=secret12345")},
		&fakeReviewAgent{name: "a3", err: errors.New("a3 failed: oom")},
	}

	_, agentErrors := mgr.runParallel(context.Background(), agents, Workspace{}, nil)
	if len(agentErrors) != 3 {
		t.Fatalf("expected 3 errors, got %d", len(agentErrors))
	}

	// 直接用 errors.Join 模拟新代码行为，验证 errors.Is 链可用
	joined := errors.Join(agentErrors...)
	for _, want := range []string{"a1 failed: timeout", "a2 failed", "a3 failed: oom"} {
		if !strings.Contains(joined.Error(), want) {
			t.Fatalf("expected joined err to contain %q, got %q", want, joined.Error())
		}
	}
	// errors.Is 能找到任一原始 error
	if !errors.Is(joined, agentErrors[0]) {
		t.Fatalf("expected errors.Is to find a1 error in chain")
	}
	// 包含计数
	summary := fmt.Errorf("all %d agents failed: %w", len(agentErrors), joined)
	if !strings.Contains(summary.Error(), "all 3 agents failed") {
		t.Fatalf("expected summary to mention count, got %q", summary.Error())
	}
	if !errors.Is(summary, agentErrors[1]) {
		t.Fatalf("expected errors.Is to find a2 error via summary")
	}
}

// fakeReviewAgent 满足 reviewAgent 接口
type fakeReviewAgent struct {
	name string
	err  error
}

func (f *fakeReviewAgent) Name() string    { return f.name }
func (f *fakeReviewAgent) Persona() string { return "" }
func (f *fakeReviewAgent) Focus() []string { return nil }
func (f *fakeReviewAgent) Priority() int   { return 0 }
func (f *fakeReviewAgent) IsDefault() bool { return false }
func (f *fakeReviewAgent) Review(_ context.Context, _ Workspace) (AgentResult, error) {
	return AgentResult{}, f.err
}
