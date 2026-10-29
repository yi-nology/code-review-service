package report_service

import (
	"strings"
	"testing"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/review"
	"github.com/yi-nology/code-review-service/internal/domain/stats"
)

// TestBuildWeeklyMarkdownPassRateScale 锁住「通过率」的量纲。
//
// stats.ContributorSummary.PassRate 已经是百分数（仓储层把 SQL 比值 ×100
// 后的值透传上来，取值 0~100）。这里若再乘一次 100，表格里会写成 10000%。
// 历史上正是这样错的，且没有测试覆盖——所以本用例专门盯这个值。
func TestBuildWeeklyMarkdownPassRateScale(t *testing.T) {
	agg := &domain.AggregateStats{Running: 0, GateFailed: 1, Findings: 3}
	contributors := []stats.ContributorSummary{
		{Author: "u1", AuthorName: "孙萌", ReviewCount: 2, PassRate: 100, TotalFindings: 1},
		{Author: "u2", AuthorName: "梁欣", ReviewCount: 1, PassRate: 50, TotalFindings: 2},
		{Author: "u3", AuthorName: "王强", ReviewCount: 1, PassRate: 0, TotalFindings: 0},
	}
	from := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)

	md := buildWeeklyMarkdown("github", "acme/console-web", agg, contributors, from, to)

	for _, want := range []string{"| 孙萌 |", "| 梁欣 |", "| 王强 |"} {
		if !strings.Contains(md, want) {
			t.Fatalf("提交人 %s 未出现在周报正文：\n%s", want, md)
		}
	}

	// 通过率列：100 → 100%、50 → 50%、0 → 0%
	for _, want := range []string{"| 100% |", "| 50% |", "| 0% |"} {
		if !strings.Contains(md, want) {
			t.Fatalf("周报正文缺少通过率 %s：\n%s", want, md)
		}
	}
	// 回归护栏：出现过 10000% 说明量纲又被乘了一次。
	if strings.Contains(md, "10000%") {
		t.Fatalf("通过率被重复放大（出现 10000%%）：\n%s", md)
	}
}
