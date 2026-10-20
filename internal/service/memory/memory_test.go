package memory

import (
	"context"
	"strings"
	"testing"

	"github.com/yi-nology/agent-memory/ports"
	"github.com/yi-nology/agent-memory/storage/sqlite"
)

// fakeLLM 是模型端口的测试替身：返回预置内容，并记录调用次数。
type fakeLLM struct {
	resp  string
	err   error
	calls int
}

func (f *fakeLLM) Complete(_ context.Context, _ ports.LLMRequest) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return f.resp, nil
}

// newTestService 装配一个跑在内存 SQLite 上的记忆服务。
func newTestService(t *testing.T, llm ports.LLM) *Service {
	t.Helper()
	st, err := sqlite.OpenMemory("review-memory-test")
	if err != nil {
		t.Fatalf("open memory store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	svc, err := New(Config{Namespace: "review", Host: "review-service", RecallLimit: 10}, Deps{
		Store:      st,
		Redactor:   NewRedactor(),
		Bander:     ports.MemoryBand{},
		LLM:        llm,
		Escalation: NewRepositoryResolver(),
		Estimator:  ports.StaticEstimator{},
	})
	if err != nil {
		t.Fatalf("assemble memory service: %v", err)
	}
	return svc
}

// TestCaptureReviewPromotesCandidate 一次评审的产出应能被提炼并晋升成事实卡。
//
// 这条用例走真实内核（真实 SQLite、真实提炼器、真实晋升判据），
// 不 mock 任何一层——mock 掉内核等于把要验的东西验没了。
func TestCaptureReviewPromotesCandidate(t *testing.T) {
	llm := &fakeLLM{resp: `[
		{
			"assertion": "acme/app 的鉴权中间件必须校验令牌签名有效期",
			"quote": "未校验令牌签名有效期",
			"event_id": "review-review-1",
			"type": "fact",
			"subject_name": "github/acme/app",
			"importance": "high",
			"confidence": "high"
		}
	]`}
	svc := newTestService(t, llm)

	outcome, err := svc.CaptureReview(context.Background(), ReviewCapture{
		Provider: "github",
		Repo:     "acme/app",
		ReviewID: "review-1",
		Verdict:  "REQUEST_CHANGES",
		Summary:  "发现 1 个高危问题",
		Findings: []FindingSummary{
			{Severity: "critical", Category: "security", Path: "internal/auth/token.go", Title: "未校验令牌签名有效期"},
		},
	})
	if err != nil {
		t.Fatalf("capture review: %v", err)
	}
	if !outcome.Accepted {
		t.Fatalf("事件应被接收，实际 SkipReason=%q", outcome.SkipReason)
	}
	if outcome.Candidates != 1 {
		t.Fatalf("应提炼出 1 条候选，实际 %d", outcome.Candidates)
	}
	if outcome.Promoted != 1 {
		t.Fatalf("候选应晋升成事实卡，实际 promoted=%d merged=%d dropped=%d review=%d",
			outcome.Promoted, outcome.Merged, outcome.Dropped, outcome.NeedsReview)
	}
}

// TestRecallReviewReturnsPromotedMemory 晋升后的记忆应能被召回。
func TestRecallReviewReturnsPromotedMemory(t *testing.T) {
	llm := &fakeLLM{resp: `[
		{
			"assertion": "acme/app 的鉴权中间件必须校验令牌签名有效期",
			"quote": "未校验令牌签名有效期",
			"event_id": "review-review-1",
			"type": "fact",
			"subject_name": "github/acme/app",
			"importance": "high",
			"confidence": "high"
		}
	]`}
	svc := newTestService(t, llm)
	ctx := context.Background()

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

	recalled, err := svc.RecallReview(ctx, RecallRequest{
		Provider: "github",
		Repo:     "acme/app",
		ReviewID: "review-2",
		Query:    "acme/app internal/auth/token.go",
	})
	if err != nil {
		t.Fatalf("recall review: %v", err)
	}
	if strings.TrimSpace(recalled) == "" {
		t.Fatal("应召回得到记忆，实际为空")
	}
	if !strings.Contains(recalled, "签名有效期") {
		t.Fatalf("召回内容应含事实断言，实际 %q", recalled)
	}
	// 召回内容是不可信内容，必须包成数据区（防伪造闭合标签）。
	if !strings.Contains(recalled, ports.BandOpen) || !strings.Contains(recalled, ports.BandClose) {
		t.Fatalf("召回内容应被包成 %s 数据区，实际 %q", ports.BandOpen, recalled)
	}
}

// TestRecallIsolatesByRepository 记忆按仓库隔离：别的仓库召不回。
func TestRecallIsolatesByRepository(t *testing.T) {
	llm := &fakeLLM{resp: `[
		{
			"assertion": "acme/app 的鉴权中间件必须校验令牌签名有效期",
			"quote": "未校验令牌签名有效期",
			"event_id": "review-review-1",
			"type": "fact",
			"subject_name": "github/acme/app",
			"importance": "high",
			"confidence": "high"
		}
	]`}
	svc := newTestService(t, llm)
	ctx := context.Background()

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

	other, err := svc.RecallReview(ctx, RecallRequest{
		Provider: "github",
		Repo:     "other/repo",
		ReviewID: "review-9",
		Query:    "acme/app",
	})
	if err != nil {
		t.Fatalf("recall other repo: %v", err)
	}
	if strings.TrimSpace(other) != "" {
		t.Fatalf("别的仓库不该召回得到本仓库的记忆，实际 %q", other)
	}
}

// TestEntityRegistrationIsIdempotent 同一仓库反复登记必须得到同一实体。
//
// 登记走底座 entity.Resolver：它按名字归一并保证幂等。
// 若登记每次都新建实体，同一个仓库会裂成多个实体——
// 晋升的事实卡挂到不同实体上，召回时一部分召不回，
// 而症状是「记忆偶尔不完整」，极难归因。
func TestEntityRegistrationIsIdempotent(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()
	ns := svc.Namespace("github", "acme/app")

	id1, err := svc.ensureRepositoryEntity(ctx, ns, "github", "acme/app")
	if err != nil {
		t.Fatalf("register entity: %v", err)
	}
	id2, err := svc.ensureRepositoryEntity(ctx, ns, "github", "acme/app")
	if err != nil {
		t.Fatalf("re-register entity: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("同一仓库重复登记应得同一实体 ID，实际 %q vs %q", id1, id2)
	}
	if id1 == "" {
		t.Fatal("实体 ID 不应为空")
	}
}

// TestEntityRegistrationIsolatesRepositories 不同仓库必须是不同实体。
func TestEntityRegistrationIsolatesRepositories(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	a, err := svc.ensureRepositoryEntity(ctx, svc.Namespace("github", "acme/app"), "github", "acme/app")
	if err != nil {
		t.Fatalf("register entity a: %v", err)
	}
	b, err := svc.ensureRepositoryEntity(ctx, svc.Namespace("github", "acme/other"), "github", "acme/other")
	if err != nil {
		t.Fatalf("register entity b: %v", err)
	}
	if a == b {
		t.Fatalf("不同仓库应得不同实体 ID，实际都是 %q", a)
	}
}

// TestCaptureWithoutLLMKeepsEvent 没有模型时事件照样入库，只是不提炼。
//
// 反例（把没有模型当失败）会让「LLM 没配」的部署完全无法使用记忆。
func TestCaptureWithoutLLMKeepsEvent(t *testing.T) {
	svc := newTestService(t, nil)
	ctx := context.Background()

	outcome, err := svc.CaptureReview(ctx, ReviewCapture{
		Provider: "github",
		Repo:     "acme/app",
		ReviewID: "review-1",
		Findings: []FindingSummary{
			{Severity: "critical", Path: "a.go", Title: "未校验令牌签名有效期"},
		},
	})
	if err != nil {
		t.Fatalf("capture review without llm: %v", err)
	}
	if !outcome.Accepted {
		t.Fatalf("事件应被接收，实际 SkipReason=%q", outcome.SkipReason)
	}
	if outcome.Candidates != 0 {
		t.Fatalf("没有模型时不应有候选，实际 %d", outcome.Candidates)
	}
}
