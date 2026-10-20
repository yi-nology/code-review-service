package memory

import (
	"context"
	"testing"
	"time"
)

// TestMaintainCollectsRawEvents 钉住「记忆库不能只进不出」。
//
// 此前记忆捕获只增不减：每次评审一条原始事件，永不回收。
// 维护必须能把过期事件收掉，且**不伤事实卡**——
// 事实卡是长期资产，原始事件只是它的证据来源。
func TestMaintainCollectsRawEvents(t *testing.T) {
	svc := newTestService(t, &fakeLLM{resp: `[
		{
			"assertion": "acme/app 的鉴权中间件必须校验令牌签名有效期",
			"quote": "未校验令牌签名有效期",
			"event_id": "review-review-1",
			"type": "fact",
			"subject_name": "github/acme/app",
			"importance": "high",
			"confidence": "high"
		}
	]`})
	ctx := context.Background()
	ns := svc.Namespace("github", "acme/app")

	if _, err := svc.CaptureReview(ctx, ReviewCapture{
		Provider: "github",
		Repo:     "acme/app",
		ReviewID: "review-1",
		Findings: []FindingSummary{
			{Severity: "critical", Path: "internal/auth/token.go", Title: "未校验令牌签名有效期"},
		},
	}); err != nil {
		t.Fatalf("capture review: %v", err)
	}

	// 维护前应有一条原始事件。
	before, err := svc.app.Stats(ctx, ns)
	if err != nil {
		t.Fatalf("stats before: %v", err)
	}
	if before.RawEvents == 0 {
		t.Fatal("捕获后应有原始事件")
	}

	// 保留期为 0 = 立即回收全部过期事件（before 时刻为「现在」，事件更早）。
	res, err := svc.Maintain(ctx, ns, time.Nanosecond)
	if err != nil {
		t.Fatalf("maintain: %v", err)
	}
	if res.RawDeleted == 0 {
		t.Fatalf("维护应回收原始事件，实际回收 0 条（结果=%+v）", res)
	}

	after, err := svc.app.Stats(ctx, ns)
	if err != nil {
		t.Fatalf("stats after: %v", err)
	}
	if after.RawEvents >= before.RawEvents {
		t.Fatalf("原始事件数应减少：维护前 %d，维护后 %d", before.RawEvents, after.RawEvents)
	}
}

// TestMaintainIsIdempotent 维护必须是幂等的：连做两遍，
// 第二遍不该再动任何东西（后台周期任务会反复跑同一套动作）。
func TestMaintainIsIdempotent(t *testing.T) {
	svc := newTestService(t, &fakeLLM{resp: `[
		{
			"assertion": "acme/app 必须校验令牌签名有效期",
			"quote": "未校验令牌签名有效期",
			"event_id": "review-review-1",
			"type": "fact",
			"subject_name": "github/acme/app",
			"importance": "high",
			"confidence": "high"
		}
	]`})
	ctx := context.Background()
	ns := svc.Namespace("github", "acme/app")

	if _, err := svc.CaptureReview(ctx, ReviewCapture{
		Provider: "github",
		Repo:     "acme/app",
		ReviewID: "review-1",
		Findings: []FindingSummary{
			{Severity: "critical", Path: "a.go", Title: "未校验令牌签名有效期"},
		},
	}); err != nil {
		t.Fatalf("capture review: %v", err)
	}

	// 第一轮：回收。
	if _, err := svc.Maintain(ctx, ns, time.Nanosecond); err != nil {
		t.Fatalf("first maintain: %v", err)
	}
	// 第二轮：无可回收。
	res, err := svc.Maintain(ctx, ns, time.Nanosecond)
	if err != nil {
		t.Fatalf("second maintain: %v", err)
	}
	if res.RawDeleted != 0 {
		t.Fatalf("第二轮不应再回收原始事件，实际回收 %d 条", res.RawDeleted)
	}
}

// TestMaintainAllCoversRegisteredNamespaces 维护必须覆盖所有已登记的命名空间。
//
// 命名空间来自宿主的登记表（底座存储按 ns 过滤，没有跨 ns 列举能力），
// 所以「登记」这一步是维护能生效的前提。
func TestMaintainAllCoversRegisteredNamespaces(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	// 捕获会登记命名空间。
	if _, err := svc.CaptureReview(ctx, ReviewCapture{
		Provider: "github",
		Repo:     "acme/app",
		ReviewID: "review-1",
		Findings: []FindingSummary{
			{Severity: "critical", Path: "a.go", Title: "未校验令牌签名有效期"},
		},
	}); err != nil {
		t.Fatalf("capture review: %v", err)
	}

	known := svc.knownNamespaces()
	if len(known) == 0 {
		t.Fatal("捕获后应登记命名空间")
	}

	// MaintainAll 应覆盖到它。
	res, err := svc.MaintainAll(ctx, time.Nanosecond)
	if err != nil {
		t.Fatalf("maintain all: %v", err)
	}
	if res.RawDeleted == 0 {
		t.Fatalf("MaintainAll 应回收已登记命名空间下的原始事件，实际 %+v", res)
	}
}

// TestRecallAlsoRegistersNamespace 只被召回过的仓库也要进维护范围。
//
// 否则「只被召回过、从没捕获过」的 ns 永远不被维护，
// 而它恰恰是召回路径长期读取的那部分数据。
func TestRecallAlsoRegistersNamespace(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	if _, err := svc.RecallReview(ctx, RecallRequest{
		Provider: "github",
		Repo:     "acme/recalled-only",
		ReviewID: "review-x",
	}); err != nil {
		t.Fatalf("recall: %v", err)
	}

	found := false
	for _, ns := range svc.knownNamespaces() {
		if ns == svc.Namespace("github", "acme/recalled-only") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("召回路径应登记命名空间")
	}
}
