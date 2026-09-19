// argus 失败兜底轮询：argus 侧任务**失败终态不回推**（仅成功终态走 sink push），
// engine=argus 的评审在 argus 任务失败后会停在 running。本轮询器定期把
// running 且带 ArgusTaskID 的记录拿去查 argus（GET /tasks/:id）：
//   - task failed/stale → 记录落 failed 终态（带 argus 失败原因）+ PR error status；
//   - task done 但记录仍 running → sink 回推丢失：拉全量任务详情（verdict+findings
//     +report）落 Argus 承接表并复用 CompleteFromArgusReport 收敛（意见不再丢，
//     降级点：pull 通道无 fp/suggested_fix，恢复的意见不参与抑制回流）；
//     恢复失败才落 failed 提示去 Argus 审查页；
//   - task pending/running / 查询失败 → 下个周期再看。
//
// 已被 CompleteFromArgusReport 收敛到终态的记录自然不再满足 running 条件。
package review

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/domain/argusingest"
	notifdomain "github.com/yi-nology/code-review-service/internal/domain/notification"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/statuscheck"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
)

// ArgusPoller argus 任务状态兜底轮询器。
type ArgusPoller struct {
	reviewRepo repository.ReviewRepository
	// argusRepo 承接表：恢复路径把拉回的 verdict/findings 先落库（与 push 同一幂等键）。
	argusRepo repository.ArgusReviewRepository
	client    *argusclient.Client
	// notifier 强制收敛/恢复后的应用内通知出口；nil 安全。
	notifier domain.Notifier
	// statusPoster 收敛后向 PR head 写门禁 status；nil 安全。
	statusPoster domain.StatusPoster
	interval     time.Duration
}

// NewArgusPoller 构造；client 未启用时 Run 直接退出（argus 引擎不可用）。
func NewArgusPoller(reviewRepo repository.ReviewRepository, argusRepo repository.ArgusReviewRepository,
	client *argusclient.Client, notifier domain.Notifier, statusPoster domain.StatusPoster,
	interval time.Duration) *ArgusPoller {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &ArgusPoller{
		reviewRepo: reviewRepo, argusRepo: argusRepo, client: client,
		notifier: notifier, statusPoster: statusPoster, interval: interval,
	}
}

// Run 阻塞轮询直到 ctx 取消（fx lifecycle OnStart 里 async.GoSafe）。
func (p *ArgusPoller) Run(ctx context.Context) {
	if !p.client.Enabled() {
		logx.Info("argus poller disabled (bot_url not configured)")
		return
	}
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	logx.Infof("argus poller started (interval=%s)", p.interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.sweep(ctx); err != nil {
				logx.CtxWarnf(ctx, "argus poller sweep failed: %v", err)
			}
		}
	}
}

// sweep 扫一轮 running 的 argus 记录并按 argus 侧终态收敛。
func (p *ArgusPoller) sweep(ctx context.Context) error {
	page, size := 1, 200
	for {
		res, err := p.reviewRepo.List(ctx, domain.ListOptions{Status: statusRunning, Page: page, PageSize: size})
		if err != nil {
			return fmt.Errorf("list running reviews: %w", err)
		}
		for i := range res.Items {
			p.checkRecord(ctx, &res.Items[i])
		}
		if len(res.Items) < size {
			return nil
		}
		page++
	}
}

func (p *ArgusPoller) checkRecord(ctx context.Context, rec *domain.Record) {
	if rec.Request.Engine != domain.EngineArgus || rec.Response.ArgusTaskID == "" {
		return // argus 侧建任务失败前不会被派发；无任务 ID 无从轮询
	}
	task, err := p.client.GetTask(ctx, rec.Response.ArgusTaskID)
	if err != nil {
		// 网络抖动/argus 短暂不可用：跳过，下个周期再查。
		logx.CtxWarnf(ctx, "argus poller: get task %s for review %s failed: %v",
			rec.Response.ArgusTaskID, rec.ID, err)
		return
	}
	switch task.Status {
	case "failed", "stale":
		p.markFailed(ctx, rec, task)
	case "done":
		p.recoverReport(ctx, rec, task)
	default:
		// pending/running：继续等。
	}
}

// recoverReport sink 回推丢失的恢复：拉全量任务详情 → 落承接表 → 复用
// CompleteFromArgusReport 收敛（findings/通知/commit status 一条龙）。
// 恢复链路任一步失败，或记录因 SHA 漂移未被收敛，降级为 markReportLost。
func (p *ArgusPoller) recoverReport(ctx context.Context, rec *domain.Record, task *argusclient.TaskState) {
	detail, err := p.client.GetTaskDetail(ctx, rec.Response.ArgusTaskID)
	if err != nil {
		logx.CtxWarnf(ctx, "argus poller: fetch detail of task %s failed: %v", task.TaskID, err)
		p.markReportLost(ctx, rec, task)
		return
	}
	rep := buildRecoveredReview(rec, detail)
	id, err := p.argusRepo.UpsertReview(ctx, rep)
	if err != nil {
		logx.CtxWarnf(ctx, "argus poller: persist recovered report for review %s failed: %v", rec.ID, err)
		p.markReportLost(ctx, rec, task)
		return
	}
	CompleteFromArgusReport(ctx, p.reviewRepo, p.notifier, p.statusPoster, rep, id)
	// SHA 漂移（PR 已前移）等场景 CompleteFromArgusReport 是 no-op：记录仍 running，
	// 回退到「报告丢失」终态，避免永久 running。
	if latest, err := p.reviewRepo.GetByReviewID(ctx, rec.ID); err == nil && latest.Status == statusRunning {
		p.markReportLost(ctx, rec, task)
		return
	}
	logx.Warnf("argus poller: review=%s task=%s done but sink report lost → recovered via pull (verdict=%s findings=%d)",
		rec.ID, task.TaskID, rep.Verdict, rep.FindingsTotal)
}

func (p *ArgusPoller) markFailed(ctx context.Context, rec *domain.Record, task *argusclient.TaskState) {
	reason := task.FailReason
	if reason == "" {
		reason = "argus 任务 " + task.Status
	}
	msg := fmt.Sprintf("argus 任务失败：%s", reason)
	applied := false
	err := p.reviewRepo.UpdateInTransaction(ctx, rec.ID, func(r *domain.Record) error {
		if r.Status != statusRunning {
			return nil // 已被 sink 回推收敛，不覆盖
		}
		applied = true
		r.Status = statusFailed
		r.Response.Status = statusFailed
		r.Response.Error = msg
		r.Response.Summary = msg
		r.Response.Gate = domain.ErrorGate(msg)
		r.Response.History = append(r.Response.History, domain.Event{
			Stage: "argus_poll", Status: "failed", Message: msg,
			How: "argus task poll (GET /tasks/:id)", CreatedAt: time.Now().UTC(),
			Output: map[string]any{"taskId": task.TaskID, "status": task.Status, "failReason": task.FailReason},
		})
		return nil
	})
	if err != nil {
		logx.CtxErrorf(ctx, "argus poller: mark review %s failed: %v", rec.ID, err)
		return
	}
	if !applied {
		return // 已被并发收敛，不重复通知
	}
	logx.Warnf("argus poller: review=%s task=%s %s → failed", rec.ID, task.TaskID, task.Status)
	// 门禁留痕按 gate_mode（语义同 postArgusGateStatus）：pending 已在派发时发出，终态
	// 必达——weak（argus 默认）一律 success 留痕清掉琥珀色；strong 任务失败写 error。
	state, desc := domain.ApplyWeakGate(rec.Request.GateMode, domain.EngineArgus, statuscheck.StateError, msg)
	domain.PostStatus(p.statusPoster, ctx, rec.Request.Provider, rec.Request.Repo, rec.HeadSHA, state, desc, rec.ID)
	p.notify(ctx, rec, msg)
}

func (p *ArgusPoller) markReportLost(ctx context.Context, rec *domain.Record, task *argusclient.TaskState) {
	msg := "argus 审查已完成但结果回推未达且拉取失败（详见「Argus 审查」页的意见明细）"
	applied := false
	err := p.reviewRepo.UpdateInTransaction(ctx, rec.ID, func(r *domain.Record) error {
		if r.Status != statusRunning {
			return nil
		}
		applied = true
		r.Status = statusFailed
		r.Response.Status = statusFailed
		r.Response.Error = msg
		r.Response.Summary = msg
		r.Response.Gate = domain.ErrorGate(msg)
		r.Response.History = append(r.Response.History, domain.Event{
			Stage: "argus_poll", Status: "completed", Message: msg,
			How: "argus task poll (GET /tasks/:id)", CreatedAt: time.Now().UTC(),
			Output: map[string]any{"taskId": task.TaskID, "status": task.Status, "verdict": task.Verdict},
		})
		return nil
	})
	if err != nil {
		logx.CtxErrorf(ctx, "argus poller: mark review %s report-lost: %v", rec.ID, err)
		return
	}
	if !applied {
		return // 已被并发收敛，不重复通知
	}
	logx.Warnf("argus poller: review=%s task=%s done but sink report lost and pull failed → failed", rec.ID, task.TaskID)
	// 门禁留痕按 gate_mode（语义同 markFailed）：weak 终态 success 留痕，细节放 description。
	state, desc := domain.ApplyWeakGate(rec.Request.GateMode, domain.EngineArgus, statuscheck.StateError, msg)
	domain.PostStatus(p.statusPoster, ctx, rec.Request.Provider, rec.Request.Repo, rec.HeadSHA, state, desc, rec.ID)
	p.notify(ctx, rec, msg)
}

// buildRecoveredReview 把 pull 通道的任务详情还原为上报实体（与 sink push 同一
// 幂等键落承接表）。降级点：pull 通道无 fp/suggested_fix，恢复的意见 fp 为空、
// 不参与抑制回流；title 打恢复标记便于溯源。
func buildRecoveredReview(rec *domain.Record, d *argusclient.TaskDetail) *argusingest.Review {
	platform := strings.ToLower(rec.Request.Provider)
	owner, repo, _ := splitOwnerRepo(rec.Request.Repo)
	number := rec.Request.PullRequestID
	if number == "" {
		number = rec.Request.MergeRequestIID
	}
	rep := &argusingest.Review{
		ArgusTaskID: firstNonEmpty(d.TaskID, rec.Response.ArgusTaskID),
		Platform:    platform, Owner: owner, Repo: repo, Number: number,
		Title:      "（回推丢失，Argus Poller 拉取恢复）",
		HeadSHA:    rec.HeadSHA,
		Verdict:    recoveredVerdict(d),
		OccurredAt: time.Now(),
	}
	if rep.Verdict == argusingest.VerdictError {
		rep.ErrorMessage = d.FailReason
	}
	pluginSeen := make(map[string]struct{})
	for _, f := range d.Findings {
		severity := f.Severity
		switch severity {
		case argusingest.SeverityHigh, argusingest.SeverityMedium, argusingest.SeverityLow:
		default:
			severity = argusingest.SeverityLow // 与 argus 侧越界归 low 同语义
		}
		by := strings.TrimSpace(f.By)
		if by != "" {
			if _, dup := pluginSeen[by]; !dup {
				pluginSeen[by] = struct{}{}
				rep.Plugins = append(rep.Plugins, by)
			}
		}
		rep.Findings = append(rep.Findings, argusingest.Finding{
			File: f.File, Line: f.Line, Severity: severity,
			Comment: f.Comment, By: by,
		})
		switch severity {
		case argusingest.SeverityHigh:
			rep.FindingsHigh++
		case argusingest.SeverityMedium:
			rep.FindingsMedium++
		default:
			rep.FindingsLow++
		}
	}
	rep.FindingsTotal = len(rep.Findings)
	return rep
}

// recoveredVerdict done 任务的裁决归一：failed 走 error（带原因）；verdict 缺失
// 按 comment（中性）收敛，保证记录不因上游数据缺口永久悬挂。
func recoveredVerdict(d *argusclient.TaskDetail) string {
	if d.Status == "failed" || d.Status == "stale" {
		return argusingest.VerdictError
	}
	switch d.Verdict {
	case argusingest.VerdictApprove, argusingest.VerdictComment, argusingest.VerdictRequestChanges:
		return d.Verdict
	default:
		return argusingest.VerdictComment
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// notify 强制收敛后发应用内通知：argus 任务失败/回推丢失时记录停在 running，
// 用户在通知中心以外无从感知。nil 安全；失败只告警，不影响已落库的收敛。
func (p *ArgusPoller) notify(ctx context.Context, rec *domain.Record, msg string) {
	if p.notifier == nil {
		return
	}
	event := notifdomain.Event{
		ReviewID: rec.ID,
		Provider: rec.Request.Provider,
		Repo:     rec.Request.Repo,
		Ref:      rec.Request.PullRequestID + rec.Request.MergeRequestIID,
		Kind:     notifdomain.KindFailed,
		Stage:    "argus_poll",
		Error:    msg,
		Summary:  msg,
	}
	async.GoSafe(func() {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := p.notifier.Notify(notifyCtx, event); err != nil {
			logx.CtxWarnf(notifyCtx, "[notification] argus poller review %s notify failed: %v", event.ReviewID, err)
		}
	})
}
