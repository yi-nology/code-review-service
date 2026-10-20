package progress

import (
	"context"
	"testing"
	"time"
)

// TestTrackerPublishesToBus 进度事件必须经事件总线单源发布。
//
// 反例（回到「tracker 直连 websocket」）会让事件面出现第二个发布点，
// 编排层想发事件就得知道前端的存在。
func TestTrackerPublishesToBus(t *testing.T) {
	pm := NewProgressManager()
	defer pm.Close()

	frames, unsubscribe := pm.Bus().Subscribe(context.Background())
	defer unsubscribe()

	tracker := pm.CreateTracker("review-1", "session-1", 1)
	tracker.UpdatePhase(PhaseAnalysis, "active", 0, "Analyzing changes")

	select {
	case ev := <-frames:
		if ev.CaseID != "review-1" {
			t.Fatalf("事件 CaseID 应为 review-1，实际 %q", ev.CaseID)
		}
		if ev.SessionID != "session-1" {
			t.Fatalf("事件 SessionID 应为 session-1，实际 %q", ev.SessionID)
		}
		raw, ok := ev.Payload[progressEventKey].(progressEvent)
		if !ok {
			t.Fatalf("事件载荷应带 progressEvent，实际 %T", ev.Payload[progressEventKey])
		}
		if raw.Phase != PhaseAnalysis {
			t.Fatalf("事件阶段应为 analysis，实际 %q", raw.Phase)
		}
		if raw.Message != "Analyzing changes" {
			t.Fatalf("事件消息应为 'Analyzing changes'，实际 %q", raw.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未在超时内收到总线事件")
	}
}

// TestTrackerPublishesAgentStatus agent 状态事件同样走总线。
func TestTrackerPublishesAgentStatus(t *testing.T) {
	pm := NewProgressManager()
	defer pm.Close()

	frames, unsubscribe := pm.Bus().Subscribe(context.Background())
	defer unsubscribe()

	tracker := pm.CreateTracker("review-1", "session-1", 1)
	tracker.UpdateAgentStatus("security", "running", 0.5, "scanning")

	select {
	case ev := <-frames:
		raw, ok := ev.Payload[progressEventKey].(progressEvent)
		if !ok {
			t.Fatalf("事件载荷应带 progressEvent，实际 %T", ev.Payload[progressEventKey])
		}
		if raw.Type != eventAgentStatus {
			t.Fatalf("事件类型应为 agent_status，实际 %q", raw.Type)
		}
		if _, ok := raw.AgentStatus["security"]; !ok {
			t.Fatalf("事件应带 security agent 状态，实际 %v", raw.AgentStatus)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("未在超时内收到总线事件")
	}
}
