package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/argusingest"
)

// argusTables argus 集成测试读写的表，测试间 TRUNCATE 隔离。
var argusTables = []string{"argus_reviews", "argus_findings"}

func resetArgusTables(t *testing.T) {
	t.Helper()
	if err := truncateTables(t, argusTables...); err != nil {
		t.Fatalf("truncate argus tables: %v", err)
	}
}

// ensureArgusPartialIndex 建与生产 db.go 启动路径同款的 (review_id, fp) 部分唯一索引。
// testutil.StartPostgres 只跑 AutoMigrate（模型标签刻意不带唯一索引），此处补齐，
// 让"多条空 fp 意见共存 / 重复 fp 被唯一约束拒绝"有真实约束可回归。
func ensureArgusPartialIndex(t *testing.T) {
	t.Helper()
	const createIdx = `CREATE UNIQUE INDEX IF NOT EXISTS idx_argus_finding_review_fp ON argus_findings (review_id, fp) WHERE fp <> ''`
	if err := testDB.Exec(createIdx).Error; err != nil {
		t.Fatalf("create partial index: %v", err)
	}
}

func argusSampleReview() *argusingest.Review {
	return &argusingest.Review{
		ArgusTaskID: "task-1", Platform: "gitea", Owner: "zhangyi", Repo: "demo",
		Number: "7", Title: "fix login", HeadSHA: "abc123", BaseBranch: "main",
		Verdict: argusingest.VerdictComment,
		Plugins: []string{"correctness"}, DurationMS: 1500, TokenUsage: 42,
		OccurredAt: time.Now().UTC(),
	}
}

// 首次上报落库 + GetReview 取回（意见按 file/line 排序、error_message 透传）。
func TestArgusRepo_UpsertAndGet(t *testing.T) {
	if testDB == nil {
		t.Skip("postgres container unavailable")
	}
	resetArgusTables(t)
	ensureArgusPartialIndex(t)
	repo := NewArgusReviewRepo(testDB)
	ctx := context.Background()

	rev := argusSampleReview()
	rev.Verdict = argusingest.VerdictError
	rev.ErrorMessage = "llm timeout"
	rev.Findings = []argusingest.Finding{
		{FP: "fp-b", File: "b.go", Line: 9, Severity: argusingest.SeverityLow, Comment: "命名"},
		{FP: "fp-a", File: "a.go", Line: 3, Severity: argusingest.SeverityHigh, Comment: "竞态"},
	}
	id, err := repo.UpsertReview(ctx, rev)
	if err != nil {
		t.Fatalf("UpsertReview: %v", err)
	}

	got, err := repo.GetReview(ctx, id)
	if err != nil {
		t.Fatalf("GetReview: %v", err)
	}
	if got.Platform != "gitea" || got.ErrorMessage != "llm timeout" {
		t.Fatalf("unexpected review: %+v", got)
	}
	if len(got.Findings) != 2 || got.Findings[0].File != "a.go" || got.Findings[1].File != "b.go" {
		t.Fatalf("findings 应按 file 排序: %+v", got.Findings)
	}
}

// P1 回归：同业务键重报覆盖任务并整组替换意见；一次上报多条空 fp 意见
// （上游旧数据形态）必须能共存——这正是部分唯一索引存在的意义。
func TestArgusRepo_ReReportReplacesFindings_AllowsEmptyFP(t *testing.T) {
	if testDB == nil {
		t.Skip("postgres container unavailable")
	}
	resetArgusTables(t)
	ensureArgusPartialIndex(t)
	repo := NewArgusReviewRepo(testDB)
	ctx := context.Background()

	first := argusSampleReview()
	first.Findings = []argusingest.Finding{
		{FP: "fp-1", File: "a.go", Line: 3, Severity: argusingest.SeverityHigh, Comment: "旧意见"},
	}
	id1, err := repo.UpsertReview(ctx, first)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	second := argusSampleReview()
	second.ArgusTaskID = "task-2"
	second.Findings = []argusingest.Finding{
		{FP: "", File: "x.go", Line: 1, Severity: argusingest.SeverityLow, Comment: "无指纹旧数据 1"},
		{FP: "", File: "y.go", Line: 2, Severity: argusingest.SeverityLow, Comment: "无指纹旧数据 2"},
		{FP: "fp-2", File: "z.go", Line: 5, Severity: argusingest.SeverityMedium, Comment: "新意见"},
	}
	id2, err := repo.UpsertReview(ctx, second)
	if err != nil {
		t.Fatalf("重报（含多条空 fp）不应撞唯一约束: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("同业务键重报应覆盖同一任务: id1=%d id2=%d", id1, id2)
	}

	got, err := repo.GetReview(ctx, id1)
	if err != nil {
		t.Fatalf("GetReview: %v", err)
	}
	if got.ArgusTaskID != "task-2" {
		t.Fatalf("任务字段未覆盖: %+v", got)
	}
	if len(got.Findings) != 3 {
		t.Fatalf("意见应整组替换为 3 条: %+v", got.Findings)
	}
}

// P2 回归：重报同 SHA 时，已确认的误报抑制标记按 fp 继承，不被整组替换清空。
func TestArgusRepo_ReReportInheritsSuppressState(t *testing.T) {
	if testDB == nil {
		t.Skip("postgres container unavailable")
	}
	resetArgusTables(t)
	ensureArgusPartialIndex(t)
	repo := NewArgusReviewRepo(testDB)
	ctx := context.Background()

	first := argusSampleReview()
	first.Findings = []argusingest.Finding{
		{FP: "fp-1", File: "a.go", Line: 3, Severity: argusingest.SeverityHigh, Comment: "误报"},
		{FP: "fp-2", File: "b.go", Line: 9, Severity: argusingest.SeverityLow, Comment: "真实问题"},
	}
	id, err := repo.UpsertReview(ctx, first)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := repo.SetFindingSuppressState(ctx, id, "fp-1", "suppressed"); err != nil {
		t.Fatalf("SetFindingSuppressState: %v", err)
	}

	reReport := argusSampleReview()
	reReport.Findings = first.Findings
	if _, err := repo.UpsertReview(ctx, reReport); err != nil {
		t.Fatalf("re-report: %v", err)
	}

	got, err := repo.GetReview(ctx, id)
	if err != nil {
		t.Fatalf("GetReview: %v", err)
	}
	for _, f := range got.Findings {
		if f.FP == "fp-1" && f.SuppressState != "suppressed" {
			t.Fatalf("重报后抑制标记应继承: %+v", f)
		}
		if f.FP == "fp-2" && f.SuppressState != "" {
			t.Fatalf("未抑制的意见不应带状态: %+v", f)
		}
	}

	// 非法状态词表应被拒绝。
	if err := repo.SetFindingSuppressState(ctx, id, "fp-1", "bogus"); err == nil {
		t.Fatal("非法 suppress_state 应报错")
	}
}

// 列表过滤：repo 名 / owner\/repo、platform 精确、severity 含该档、since、分页排序。
func TestArgusRepo_ListFilters(t *testing.T) {
	if testDB == nil {
		t.Skip("postgres container unavailable")
	}
	resetArgusTables(t)
	ensureArgusPartialIndex(t)
	repo := NewArgusReviewRepo(testDB)
	ctx := context.Background()

	base := time.Now().UTC().Add(-2 * time.Hour)
	mk := func(platform, owner, name, number string, high int, at time.Time) *argusingest.Review {
		rev := argusSampleReview()
		rev.Platform = platform
		rev.Owner, rev.Repo, rev.Number = owner, name, number
		rev.HeadSHA = "sha-" + platform + "-" + owner + "-" + name + "-" + number
		rev.ArgusTaskID = "task-" + rev.HeadSHA
		rev.OccurredAt = at
		rev.FindingsHigh = high
		if high > 0 {
			rev.Findings = []argusingest.Finding{{FP: "fp-" + rev.HeadSHA, File: "a.go", Severity: argusingest.SeverityHigh, Comment: "x"}}
		}
		return rev
	}
	seed := []*argusingest.Review{
		mk("gitea", "zhangyi", "demo", "1", 1, base),
		mk("github", "other", "demo", "2", 0, base.Add(time.Minute)), // 同名 repo 不同平台
		mk("gitea", "zhangyi", "another", "3", 0, base.Add(2*time.Minute)),
	}
	for _, rev := range seed {
		if _, err := repo.UpsertReview(ctx, rev); err != nil {
			t.Fatalf("seed upsert: %v", err)
		}
	}

	// repo 名匹配两平台同名 repo。
	items, total, err := repo.ListReviews(ctx, argusingest.ListFilter{Repo: "demo"})
	if err != nil || total != 2 {
		t.Fatalf("Repo=demo 应命中 2: total=%d err=%v", total, err)
	}
	// owner/repo 组合只命中一条。
	items, total, err = repo.ListReviews(ctx, argusingest.ListFilter{Repo: "zhangyi/demo"})
	if err != nil || total != 1 || items[0].Platform != "gitea" {
		t.Fatalf("Repo=zhangyi/demo 应命中 gitea 一条: total=%d err=%v", total, err)
	}
	// platform 精确过滤。
	items, total, err = repo.ListReviews(ctx, argusingest.ListFilter{Repo: "demo", Platform: "github"})
	if err != nil || total != 1 || items[0].Owner != "other" {
		t.Fatalf("platform=github 应命中 other/demo: total=%d err=%v", total, err)
	}
	// severity=high 只命中含 high 意见的任务。
	_, total, err = repo.ListReviews(ctx, argusingest.ListFilter{Severity: argusingest.SeverityHigh})
	if err != nil || total != 1 {
		t.Fatalf("severity=high 应命中 1: total=%d err=%v", total, err)
	}
	// since 过滤掉旧任务。
	since := base.Add(90 * time.Second)
	_, total, err = repo.ListReviews(ctx, argusingest.ListFilter{Since: &since})
	if err != nil || total != 1 {
		t.Fatalf("since 应命中 1: total=%d err=%v", total, err)
	}
	// occurred_at 降序。
	items, _, err = repo.ListReviews(ctx, argusingest.ListFilter{})
	if err != nil || len(items) != 3 {
		t.Fatalf("全量应 3 条: %v", err)
	}
	if !items[0].OccurredAt.After(items[1].OccurredAt) || !items[1].OccurredAt.After(items[2].OccurredAt) {
		t.Fatalf("应按 occurred_at 降序: %+v", items)
	}
}

// suppress 状态词表约束（” 与 'suppressed' 之外的值拒绝）在无行命中时也不报错。
func TestArgusRepo_SuppressStateWordList(t *testing.T) {
	if testDB == nil {
		t.Skip("postgres container unavailable")
	}
	repo := NewArgusReviewRepo(testDB)
	if err := repo.SetFindingSuppressState(context.Background(), 99999, "fp-x", "suppressed"); err != nil {
		t.Fatalf("无命中行不应报错: %v", err)
	}
}
