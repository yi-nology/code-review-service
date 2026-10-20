package memory

import (
	"context"
	"testing"

	"github.com/yi-nology/agent-memory/ports"
	"github.com/yi-nology/agent-memory/storage/sqlite"
	"github.com/yi-nology/agent-memory/store"
	"github.com/yi-nology/agent-memory/types"
)

// TestUpgradeKeepsOldDataVisible 钉住升级到 agent-memory v0.6.0 后，
// **既有记忆数据仍然可召回**。
//
// v0.6.0 给 atoms / raw_events / candidates 三张表加了 scope 列（记录级归属），
// 迁移对老行填 ” ——「空 = 全局可见」。本服务不传 Scope（空），
// 所以升级前后可见性必须一致。
//
// 这条性质值得钉住：若迁移把老数据填成非空 scope，
// 而本服务读的时候也传空，那「全局」的老数据就再也召不回来了——
// 症状是升级后记忆静默失效，没有任何报错。
func TestUpgradeKeepsOldDataVisible(t *testing.T) {
	st, err := sqlite.OpenMemory("review-memory-upgrade-test")
	if err != nil {
		t.Fatalf("open memory store: %v", err)
	}
	defer func() { _ = st.Close() }()

	ctx := context.Background()
	ns := store.Namespace("review/github/acme/app")

	svc, err := New(Config{Namespace: "review", Host: "review-service", RecallLimit: 10}, Deps{
		Store:    st,
		Redactor: NewRedactor(),
		Bander:   ports.MemoryBand{},
		LLM: &fakeLLM{resp: `[
			{
				"assertion": "acme/app 的鉴权中间件必须校验令牌签名有效期",
				"quote": "未校验令牌签名有效期",
				"event_id": "review-review-upgrade",
				"type": "fact",
				"subject_name": "github/acme/app",
				"importance": "high",
				"confidence": "high"
			}
		]`},
		Escalation: NewRepositoryResolver(),
		Estimator:  ports.StaticEstimator{},
	})
	if err != nil {
		t.Fatalf("assemble memory service: %v", err)
	}

	if _, err := svc.CaptureReview(ctx, ReviewCapture{
		Provider: "github",
		Repo:     "acme/app",
		ReviewID: "review-upgrade",
		Findings: []FindingSummary{
			{Severity: "critical", Path: "internal/auth/token.go", Title: "未校验令牌签名有效期"},
		},
	}); err != nil {
		t.Fatalf("capture review: %v", err)
	}

	// 事实卡应已落库。
	atoms, err := st.ListAtoms(ctx, ns, store.AtomFilter{ActiveOnly: true, Limit: 10})
	if err != nil {
		t.Fatalf("list atoms: %v", err)
	}
	if len(atoms) == 0 {
		t.Fatal("捕获后应有事实卡")
	}

	// 本服务不传 Scope（全局），落库的 scope 必须为空——
	// 非空就意味着这条事实只对某个归属可见，而本服务读时也传空，
	// 结果是「写进去了但永远召不回」。
	for _, a := range atoms {
		if a.Scope != types.Scope("") {
			t.Fatalf("事实卡 %q 的 scope 应为空（本服务用全局归属），实际 %q", a.ID, a.Scope)
		}
	}

	// 召回必须拿得到。
	recalled, err := svc.RecallReview(ctx, RecallRequest{
		Provider: "github",
		Repo:     "acme/app",
		ReviewID: "review-upgrade-2",
	})
	if err != nil {
		t.Fatalf("recall after upgrade: %v", err)
	}
	if recalled == "" {
		t.Fatal("全局归属的记忆应仍可召回 —— 升级后记忆静默失效")
	}
}
