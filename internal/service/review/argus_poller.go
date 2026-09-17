// argus 失败兜底轮询：argus 侧任务**失败终态不回推**（仅成功终态走 sink push），
// engine=argus 的评审在 argus 任务失败后会停在 running。本轮询器定期把
// running 且带 ArgusTaskID 的记录拿去查 argus（GET /tasks/:id）：
//   - task failed/stale → 记录落 failed 终态（带 argus 失败原因）；
//   - task done 但记录仍 running → sink 回推已丢失（argus 重试耗尽等），
//     落 failed 并提示查看 Argus 审查页（意见已在该页可见）；
//   - task pending/running / 查询失败 → 下个周期再看。
//
// 已被 CompleteFromArgusReport 收敛到终态的记录自然不再满足 running 条件。
package review

import (
	"context"
	"fmt"
	"time"

	"github.com/yi-nology/code-review-service/internal/argusclient"
	notifdomain "github.com/yi-nology/code-review-service/internal/domain/notification"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
)

// ArgusPoller argus 任务状态兜底轮询器。
type ArgusPoller struct {
	reviewRepo repository.ReviewRepository
	client     *argusclient.Client
	// notifier 强制收敛（任务失败/回推丢失）后的应用内通知出口；nil 安全。
	notifier domain.Notifier
	interval time.Duration
}

// NewArgusPoller 构造；client 未启用时 Run 直接退出（argus 引擎不可用）。
func NewArgusPoller(reviewRepo repository.ReviewRepository, client *argusclient.Client,
	notifier domain.Notifier, interval time.Duration) *ArgusPoller {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &ArgusPoller{reviewRepo: reviewRepo, client: client, notifier: notifier, interval: interval}
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
		// argus 审查完成但 sink 回推丢失（push 有重试，仅极端情况）：
		// 意见已落 Argus 审查承接表（argus push 前半段成功才会 done），
		// 这里只把评审记录收敛到终态，避免永久 running。
		p.markReportLost(ctx, rec, task)
	default:
		// pending/running：继续等。
	}
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
	p.notify(ctx, rec, msg)
}

func (p *ArgusPoller) markReportLost(ctx context.Context, rec *domain.Record, task *argusclient.TaskState) {
	msg := "argus 审查已完成但结果回推未达（详见「Argus 审查」页的意见明细）"
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
	logx.Warnf("argus poller: review=%s task=%s done but sink report lost → failed", rec.ID, task.TaskID)
	p.notify(ctx, rec, msg)
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
