package eino

import (
	"context"
	"fmt"
	"strings"
	"time"

	bizmetrics "github.com/yi-nology/code-review-service/internal/observability/metrics"
	"github.com/yi-nology/code-review-service/internal/service/judge"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/statuscheck"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
	notifdomain "github.com/yi-nology/code-review-service/internal/domain/notification"
)

// 执行器的对外副作用：writeback、commit status、通知、指标、OCR 入参构造。
// publishWriteback publishes review findings to git platform
func (e *Executor) publishWriteback(ctx context.Context, id string, req domain.Request, workspace domain.Workspace, result domain.Result) (domain.WritebackSummary, error) {
	if e.writeback == nil {
		return domain.WritebackSummary{Status: domain.WritebackStatusDisabled}, nil
	}

	callbackMode := req.CallbackMode
	hasPR := strings.TrimSpace(req.PullRequestID) != "" || strings.TrimSpace(req.MergeRequestIID) != ""
	if !hasPR && (callbackMode == "" || callbackMode == "comment") {
		callbackMode = "none"
	}

	if callbackMode == "none" || callbackMode == "summary" {
		return domain.WritebackSummary{Status: domain.WritebackStatusSkipped}, nil
	}

	e.appendHistory(ctx, id, "writeback", "running", "publishing review comments to git platform", nil)

	writebackReq := req
	writebackReq.ReviewID = id
	if workspace.OriginalHeadSHA != "" {
		writebackReq.HeadSHA = workspace.OriginalHeadSHA
	}
	wbResult, err := e.writeback.PublishWithDiff(ctx, writebackReq, result, workspace.Diff)
	if err != nil {
		logx.Warn("writeback publish failed", "review_id", id, "error", err)
		e.appendHistory(ctx, id, "writeback", "failed", fmt.Sprintf("writeback failed: %v", err), nil)
		return domain.WritebackSummary{
			Status: domain.WritebackStatusFailed,
			Error:  err.Error(),
		}, err
	}

	summary := domain.WritebackSummary{
		Status:    domain.WritebackStatusOK,
		Published: wbResult.Published,
		Failed:    wbResult.Failed,
		Skipped:   wbResult.Skipped,
	}
	switch {
	case wbResult.Stale:
		summary.Status = domain.WritebackStatusStale
		summary.Stale = true
		summary.ExpectedSHA = wbResult.ExpectedSHA
		summary.ActualSHA = wbResult.ActualSHA
		e.appendHistory(ctx, id, "writeback", "completed",
			fmt.Sprintf("writeback degraded (stale): head %.8s -> %.8s, summary-only published", wbResult.ExpectedSHA, wbResult.ActualSHA),
			map[string]any{"expected": wbResult.ExpectedSHA, "actual": wbResult.ActualSHA, "published": wbResult.Published})
	case wbResult.Skipped:
		summary.Status = domain.WritebackStatusSkipped
		e.appendHistory(ctx, id, "writeback", "completed", "writeback skipped (stale or empty)", nil)
	default:
		e.appendHistory(ctx, id, "writeback", "completed",
			fmt.Sprintf("comments published to git platform (published=%d, failed=%d)", wbResult.Published, wbResult.Failed),
			map[string]any{"published": wbResult.Published, "failed": wbResult.Failed})
	}
	if wbResult.Published > 0 {
		bizmetrics.WritebackFindingsTotal.WithLabelValues(req.Provider, "published").Add(float64(wbResult.Published))
	}
	if wbResult.Failed > 0 {
		bizmetrics.WritebackFindingsTotal.WithLabelValues(req.Provider, "failed").Add(float64(wbResult.Failed))
	}
	return summary, nil
}

// recordFailure/recordFailureWithOCR 委托给统一的 FailureReporter
// （internal/service/review），失败上报的副作用集与 worker 执行前失败完全一致。
func (e *Executor) recordFailure(ctx context.Context, id string, req domain.Request, stage string, runErr error) {
	e.failureReporter.Report(ctx, id, req, stage, runErr, domain.OCRMetadata{})
}

func (e *Executor) postCompletionStatus(req domain.Request, resp domain.Response, workspace domain.Workspace) {
	if e.statusPoster == nil {
		return
	}
	state := statuscheck.StateSuccess
	description := "代码评审通过"
	if resp.Gate.Status == domain.GateFailed {
		description = "质量门禁未通过：" + truncateForStatus(resp.Gate.Reason)
		// 弱门禁：门禁未过同样 success 留痕，细节放 description。
		state, description = domain.ApplyWeakGate(req.GateMode, req.Engine, statuscheck.StateFailed, description)
	}
	sha := workspace.OriginalHeadSHA
	if sha == "" {
		sha = workspace.HeadSHA
	}
	if sha == "" {
		sha = req.HeadSHA
	}
	e.statusPoster.Post(context.Background(), req.Provider, req.Repo, sha, state, description, resp.ReviewID)
}

// notifyStart 在评审开始时发 commit status（pending）+ PR 评论。
// 让 PR 作者立即知道审查已启动，正在分析代码。
func (e *Executor) notifyStart(ctx context.Context, id string, req domain.Request) {
	// 1. 发 commit status: pending
	if e.statusPoster != nil && req.HeadSHA != "" {
		e.statusPoster.Post(ctx, req.Provider, req.Repo, req.HeadSHA,
			statuscheck.StatePending,
			"AI 代码审查进行中，正在分析代码变更...",
			"")
	}

	// 2. 发 PR 评论：审查开始通知
	hasPR := strings.TrimSpace(req.PullRequestID) != "" || strings.TrimSpace(req.MergeRequestIID) != ""
	if e.writeback != nil && hasPR {
		startComment := fmt.Sprintf(
			"**AI 代码审查已启动**\n\n"+
				"正在分析 `%s/%s` 的代码变更（`%s` → `%s`）...\n\n"+
				"审查完成后会在此处发布详细结果。",
			req.Provider, req.Repo,
			truncateSHA(req.BaseSHA), truncateSHA(req.HeadSHA),
		)
		// 异步发评论，不阻塞主流程。
		// 走 async.GoSafeLabeled：裸 go func 里的 panic 会终止整个进程
		//（Go 的 goroutine panic 不跨 goroutine 传播），而这里正处在外部
		// 调用边界（writeback 发布）上——一次 panic 不该把服务带走。
		async.GoSafeLabeled("post-start-comment", func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if _, err := e.writeback.PublishWithDiff(bgCtx, req, domain.Result{
				Summary: startComment,
			}, ""); err != nil {
				logx.Warnf("[executor] failed to post start comment: %v", err)
			}
		})
	}

	e.appendHistory(ctx, id, "notify", "started", "review start notification sent", nil)
}

func countTruePositives(j *judge.Judgment) int {
	if j == nil {
		return 0
	}
	n := 0
	for _, f := range j.PrioritizedFindings {
		if f.IsLikelyTrue {
			n++
		}
	}
	return n
}

func (e *Executor) currentModel() string {
	if e.modelNameFunc != nil {
		return e.modelNameFunc()
	}
	return ""
}

func (e *Executor) recordReviewMetrics(req domain.Request, resp domain.Response, workspace domain.Workspace) {
	status := resp.Gate.Status
	if status == "" {
		status = resp.Status
	}
	bizmetrics.ReviewTotal.WithLabelValues(req.Provider, status).Inc()

	if elapsed := parseElapsedSeconds(resp.Report.OCR.Elapsed); elapsed > 0 {
		bizmetrics.ReviewDurationSeconds.WithLabelValues(req.Provider, status).Observe(elapsed)
	}

	if e.costCalc != nil && resp.Report.OCR.TotalTokens > 0 {
		model := resp.Report.OCR.Model
		cost := e.costCalc.Estimate(context.Background(), req.Provider, model,
			resp.Report.OCR.InputTokens, resp.Report.OCR.OutputTokens,
			resp.Report.OCR.CacheReadTokens, resp.Report.OCR.CacheWriteTokens)
		if cost > 0 {
			bizmetrics.LLMCostUSDTotal.WithLabelValues(req.Provider, model).Add(cost)
		}
	}
}

func parseElapsedSeconds(elapsed string) float64 {
	if elapsed == "" {
		return 0
	}
	s := strings.TrimSuffix(elapsed, "s")
	var v float64
	if _, err := fmt.Sscanf(s, "%f", &v); err != nil {
		return 0
	}
	if strings.HasSuffix(elapsed, "ms") {
		return v / 1000
	}
	return v
}

func (e *Executor) notifyCompletion(req domain.Request, resp domain.Response) {
	kind := notifdomain.KindCompleted
	if resp.Gate.Status == domain.GateFailed {
		kind = notifdomain.KindGateFailed
	}
	e.notifyAsync(notifdomain.Event{
		ReviewID:   resp.ReviewID,
		Provider:   req.Provider,
		Repo:       req.Repo,
		Ref:        req.PullRequestID + req.MergeRequestIID,
		Kind:       kind,
		GateStatus: resp.Gate.Status,
		Score:      resp.Report.Score.Value,
		Grade:      resp.Report.Score.Grade,
		Findings:   len(resp.Findings),
		Summary:    resp.Summary,
	})
}

func (e *Executor) notifyAsync(event notifdomain.Event) {
	if e.notifier == nil {
		return
	}
	async.GoSafe(func() {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := e.notifier.Notify(notifyCtx, event); err != nil {
			logx.CtxWarnf(context.Background(),
				"[notification] review %s kind=%s notify failed: %v", event.ReviewID, event.Kind, err)
		}
	})
}
