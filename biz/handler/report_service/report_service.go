package report_service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/cloudwego/hertz/pkg/app"
)

var handlerInst *Handler

type Handler struct {
	reviewRepo repository.ReviewRepository
}

func InitHandler(reviewRepo repository.ReviewRepository) {
	handlerInst = &Handler{reviewRepo: reviewRepo}
}

// GenerateWeeklyReport 生成项目维度周报（Markdown 格式）。
// 复用 stats 的 AggregateStats + ProjectOCRMetrics 聚合查询。
// GET /api/v1/reports/weekly?provider=x&repo=y
func GenerateWeeklyReport(ctx context.Context, c *app.RequestContext) {
	if handlerInst == nil {
		common.Fail(c, common.BizErr(500, "report handler not initialized"))
		return
	}
	provider := string(c.QueryArgs().Peek("provider"))
	repo := string(c.QueryArgs().Peek("repo"))
	if provider == "" || repo == "" {
		common.Fail(c, common.BadRequest("provider 和 repo 不能为空"))
		return
	}

	now := time.Now().UTC()
	weekAgo := now.AddDate(0, 0, -7)

	// 复用 AggregateStats（带 provider/repo 过滤）。
	listOpts := domain.ListOptions{
		Provider: provider,
		Repo:     repo,
	}
	agg, err := handlerInst.reviewRepo.AggregateStats(ctx, listOpts)
	if err != nil {
		common.Fail(c, err)
		return
	}

	report := buildWeeklyMarkdown(provider, repo, agg, weekAgo, now)
	common.Success(c, map[string]any{
		"format": "markdown",
		"report": report,
		"period": fmt.Sprintf("%s ~ %s", weekAgo.Format("2006-01-02"), now.Format("2006-01-02")),
		"stats":  agg,
	})
}

// buildWeeklyMarkdown 把聚合统计渲染成 Markdown 周报。
func buildWeeklyMarkdown(provider, repo string, agg *domain.AggregateStats, from, to time.Time) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# 代码评审周报 · %s\n\n", repo))
	b.WriteString(fmt.Sprintf("**周期：** %s ~ %s\n", from.Format("2006-01-02"), to.Format("2006-01-02")))
	b.WriteString(fmt.Sprintf("**仓库：** %s (%s)\n\n", repo, provider))

	b.WriteString("## 概览\n\n")
	b.WriteString(fmt.Sprintf("- 进行中：%d\n", agg.Running))
	b.WriteString(fmt.Sprintf("- 门禁未通过：%d\n", agg.GateFailed))
	b.WriteString(fmt.Sprintf("- 发现问题总数：%d\n\n", agg.Findings))

	b.WriteString("## Token 消耗\n\n")
	b.WriteString(fmt.Sprintf("- 总 Token：%d\n", agg.TotalTokens))
	b.WriteString(fmt.Sprintf("- Input Token：%d\n", agg.InputTokens))
	b.WriteString(fmt.Sprintf("- Output Token：%d\n\n", agg.OutputTokens))

	b.WriteString("---\n*由 review-service 自动生成*")
	return b.String()
}
