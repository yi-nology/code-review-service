package review

import (
	"context"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/argusingest"
	pg "github.com/yi-nology/code-review-service/internal/repository/postgres"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/testutil"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// 回报关联（真库集成）：engine=argus 的 active Record 在 push 落库后收敛终态。
// 自起一次性 postgres 容器（docker 不可用整测跳过）——postgres 包内测试无法
// 引用本包（repository→postgres 的 import 环），故放在被测函数所在包。
func TestCompleteFromArgusReport(t *testing.T) {
	ctx := context.Background()
	container, db, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Skipf("postgres testcontainer unavailable: %v", err)
	}
	defer func() { _ = container.Terminate(ctx) }()
	if err := testutil.Truncate(db, "reviews"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	reviewRepo := pg.NewReviewRepo(db)

	// 每个子测试独立 PR number，避免同 PR 多条 active 记录干扰
	// GetActiveByProviderRepo 的 created_at DESC 取首行（同微秒时间戳排序不稳）。
	mk := func(id, engine, headSHA, status, number string) {
		t.Helper()
		rec := &domain.Record{
			ID: id, RequestFingerprint: "fp-" + id,
			Provider: "gitea", Repo: "zhangyi/demo", Status: status,
			PullRequestID: number, HeadSHA: headSHA,
			Request:  domain.Request{Provider: "gitea", Repo: "zhangyi/demo", PullRequestID: number, Engine: engine},
			Response: domain.Response{ReviewID: id, Status: status, Gate: domain.PendingGate()},
		}
		if err := reviewRepo.Create(ctx, rec); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}
	rep := func(verdict, headSHA, number string) *argusingest.Review {
		return &argusingest.Review{
			ArgusTaskID: "task-1", Platform: "gitea", Owner: "zhangyi", Repo: "demo",
			Number: number, HeadSHA: headSHA, Verdict: verdict, ErrorMessage: "llm timeout",
			FindingsTotal: 3, FindingsHigh: 1, FindingsMedium: 1, FindingsLow: 1,
			Findings: []argusingest.Finding{
				{FP: "fp-h", File: "a.go", Line: 28, Severity: argusingest.SeverityHigh,
					Comment: "Get 方法读取 map 未加锁，与 Set 并发会 panic。详见下文分析……", By: "security,correctness", SuggestedFix: "加 s.mu.Lock()"},
				{FP: "fp-l", File: "b.go", Line: 48, Severity: argusingest.SeverityLow,
					Comment: "neverUsed 是死代码"},
			},
			DurationMS: 1500, OccurredAt: time.Now().UTC(),
		}
	}

	t.Run("request_changes 收敛 success+门禁失败", func(t *testing.T) {
		mk("rev-rc-1", domain.EngineArgus, "sha-rc", statusRunning, "101")
		CompleteFromArgusReport(ctx, reviewRepo, nil, rep(argusingest.VerdictRequestChanges, "sha-rc", "101"), 42)
		got, err := reviewRepo.GetByReviewID(ctx, "rev-rc-1")
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != statusSuccess || got.Response.Gate.Status != domain.GateFailed {
			t.Fatalf("status=%s gate=%s", got.Status, got.Response.Gate.Status)
		}
		if len(got.Response.History) == 0 || got.Response.History[len(got.Response.History)-1].Stage != "argus_report" {
			t.Fatalf("history 应含 argus_report 事件: %+v", got.Response.History)
		}
		// 意见应随收敛写入评审记录（详情页直接可见）
		if len(got.Response.Findings) != 2 {
			t.Fatalf("findings 应写入记录: %d", len(got.Response.Findings))
		}
		f := got.Response.Findings[0]
		if f.Path != "a.go" || f.Line != 28 || f.Severity != domain.SeverityMajor || f.Fingerprint != "fp-h" {
			t.Fatalf("finding 映射不符: %+v", f)
		}
		if f.Title == "" || f.Body == "" || len(f.Agents) != 2 {
			t.Fatalf("title/body/agents 不应缺省: %+v", f)
		}
	})

	t.Run("error 收敛 failed", func(t *testing.T) {
		mk("rev-err-1", domain.EngineArgus, "sha-err", statusRunning, "102")
		CompleteFromArgusReport(ctx, reviewRepo, nil, rep(argusingest.VerdictError, "sha-err", "102"), 43)
		got, err := reviewRepo.GetByReviewID(ctx, "rev-err-1")
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != statusFailed || got.Response.Gate.Status != domain.GateError {
			t.Fatalf("status=%s gate=%s", got.Status, got.Response.Gate.Status)
		}
		if got.Response.Error != "llm timeout" {
			t.Fatalf("error_message 应写入 Response.Error: %q", got.Response.Error)
		}
	})

	t.Run("engine=ocr 不被收敛", func(t *testing.T) {
		mk("rev-ocr-1", domain.EngineOCR, "sha-x", statusRunning, "103")
		CompleteFromArgusReport(ctx, reviewRepo, nil, rep(argusingest.VerdictApprove, "sha-x", "103"), 44)
		got, _ := reviewRepo.GetByReviewID(ctx, "rev-ocr-1")
		if got.Status != statusRunning {
			t.Fatalf("ocr 引擎记录不应被动: %s", got.Status)
		}
	})

	t.Run("head_sha 不一致不收敛", func(t *testing.T) {
		mk("rev-stale-1", domain.EngineArgus, "sha-new", statusRunning, "104")
		CompleteFromArgusReport(ctx, reviewRepo, nil, rep(argusingest.VerdictApprove, "sha-old", "104"), 45)
		got, _ := reviewRepo.GetByReviewID(ctx, "rev-stale-1")
		if got.Status != statusRunning {
			t.Fatalf("旧 SHA 补报不应收敛新任务: %s", got.Status)
		}
	})

	t.Run("无 active 记录 no-op", func(t *testing.T) {
		CompleteFromArgusReport(ctx, reviewRepo, nil, rep(argusingest.VerdictApprove, "sha-none", "105"), 46)
	})
}
