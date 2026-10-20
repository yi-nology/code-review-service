package workflow

import (
	"context"
	"testing"

	dsession "github.com/yi-nology/code-review-service/internal/domain/session"
)

// TestPhaseControllerFailCurrent 回归测试：步骤失败时当前阶段必须落到 failed 终态。
//
// 此前 phaseController.end 支持 phaseFailed，但全仓只有 completePhase 调它、
// 且只用 succeeded —— failed 分支不可达：失败阶段在底座 statemachine 里
// 永远停在 running，底座从不记录「这一阶段失败了」。
func TestPhaseControllerFailCurrent(t *testing.T) {
	kernel, _ := newTestKernel(t)
	ctx := context.Background()

	if err := kernel.begin(ctx, dsession.PhaseContext, ""); err != nil {
		t.Fatalf("begin context: %v", err)
	}
	if err := kernel.failCurrent("boom"); err != nil {
		t.Fatalf("failCurrent: %v", err)
	}

	kernel.mu.Lock()
	status := kernel.statuses[dsession.PhaseContext]
	kernel.mu.Unlock()
	if status != phaseFailed {
		t.Errorf("阶段状态 = %q, want %q（failed）", status, phaseFailed)
	}
}

// TestPhaseControllerFailCurrentWithoutBegin 验证尚未起步时 failCurrent 是 no-op。
func TestPhaseControllerFailCurrentWithoutBegin(t *testing.T) {
	kernel, _ := newTestKernel(t)
	// 未 begin 过任何阶段时 currentStep 是初始阶段——这里显式清空模拟
	//「一步都没走到」的情况。
	kernel.mu.Lock()
	kernel.currentStep = ""
	kernel.mu.Unlock()

	if err := kernel.failCurrent("boom"); err != nil {
		t.Fatalf("无当前阶段时应为 no-op, got %v", err)
	}
}

// TestPhaseControllerFailCurrentTwice 验证重复失败不会破坏状态机
// （running → failed 是合法迁移；failed → failed 不是）。
func TestPhaseControllerFailCurrentTwice(t *testing.T) {
	kernel, _ := newTestKernel(t)
	ctx := context.Background()

	if err := kernel.begin(ctx, dsession.PhaseContext, ""); err != nil {
		t.Fatalf("begin context: %v", err)
	}
	if err := kernel.failCurrent("boom"); err != nil {
		t.Fatalf("first failCurrent: %v", err)
	}
	// 第二次：failed → failed 不是合法迁移，应报错而不是静默通过。
	if err := kernel.failCurrent("boom again"); err == nil {
		t.Error("重复失败应被状态机拒绝（failed → failed 非法），实际放行")
	}
}
