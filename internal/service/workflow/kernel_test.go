package workflow

import (
	"context"
	"testing"

	dsession "github.com/yi-nology/code-review-service/internal/domain/session"
)

// newTestKernel 构造一个带 mock 会话的阶段推进器。
func newTestKernel(t *testing.T) (*phaseController, *MockSessionService) {
	t.Helper()
	sessions := NewMockSessionService()
	session, err := sessions.CreateSession(context.Background(), "review-1", 1)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	kernel := newPhaseController(session.SessionID, sessions, nil, session.Phase)
	return kernel, sessions
}

// TestPhaseControllerRejectsIllegalOrder 阶段顺序非法时必须拒绝推进。
//
// 反例（放行）会让会话的 phase 列跳过中间阶段，实际进度与记录对不上。
func TestPhaseControllerRejectsIllegalOrder(t *testing.T) {
	kernel, _ := newTestKernel(t)
	ctx := context.Background()

	// context → analysis 跳过了 change_context。
	if err := kernel.begin(ctx, dsession.PhaseContext, ""); err != nil {
		t.Fatalf("begin context: %v", err)
	}
	if err := kernel.end(dsession.PhaseContext, phaseSucceeded, ""); err != nil {
		t.Fatalf("end context: %v", err)
	}
	err := kernel.begin(ctx, dsession.PhaseAnalysis, "")
	if err == nil {
		t.Fatal("阶段顺序非法（context → analysis）应被拒绝，实际放行")
	}
}

// TestPhaseControllerRejectsIllegalStatus 状态迁移非法时必须拒绝收口。
//
// created → succeeded 是非法的（必须先 running）。
func TestPhaseControllerRejectsIllegalStatus(t *testing.T) {
	kernel, _ := newTestKernel(t)

	// 未经 begin 直接 end：created → succeeded 非法。
	err := kernel.end(dsession.PhaseContext, phaseSucceeded, "")
	if err == nil {
		t.Fatal("状态迁移非法（created → succeeded）应被拒绝，实际放行")
	}
}

// TestPhaseControllerRejectsDoubleBegin 同一阶段不得重复开始。
//
// running → running 不是合法迁移：重复推进说明流程被走岔了。
func TestPhaseControllerRejectsDoubleBegin(t *testing.T) {
	kernel, _ := newTestKernel(t)
	ctx := context.Background()

	if err := kernel.begin(ctx, dsession.PhaseContext, ""); err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := kernel.begin(ctx, dsession.PhaseContext, ""); err == nil {
		t.Fatal("同一阶段重复 begin 应被拒绝，实际放行")
	}
}

// TestPhaseControllerAdvancesSessionPhase 阶段推进应写进会话的 phase 列。
func TestPhaseControllerAdvancesSessionPhase(t *testing.T) {
	kernel, sessions := newTestKernel(t)
	ctx := context.Background()

	if err := kernel.begin(ctx, dsession.PhaseChangeContext, ""); err != nil {
		t.Fatalf("begin: %v", err)
	}
	sess, err := sessions.GetSession(ctx, kernel.sessionID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if sess.Phase != dsession.PhaseChangeContext {
		t.Fatalf("会话阶段应为 change_context，实际 %q", sess.Phase)
	}
}

// TestPhaseControllerRejectsUnknownPhase 未知阶段必须被拒绝。
func TestPhaseControllerRejectsUnknownPhase(t *testing.T) {
	kernel, _ := newTestKernel(t)

	if err := kernel.rcSetCurrentStep("not_a_phase"); err == nil {
		t.Fatal("未知评审阶段应被拒绝，实际放行")
	}
}
