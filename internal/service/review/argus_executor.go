// argus 引擎执行器：engine=argus 的评审不经 OCR 子进程，而是触发外部 Argus
// 审查任务（POST /tasks），argus 完成后经 push 回报（/api/v1/argus/reviews），
// 由 CompleteFromArgusReport 把 active Record 收敛到终态。
//
// 设计约束：OCR 路径一行不改——EngineExecutor 按 req.Engine 分流，ocr 分支
// 原样转发；argus 分支复用 worker 并发/重试/死信与 active-review 去重，
// 触发成功即返回（Record 停在 running），不阻塞等待回报（stale 重排兜底，
// argus 侧同 PR 在飞任务幂等去重，重复触发不叠加审查）。
package review

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/domain/argusingest"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/oekit/logx"
)

// EngineExecutor 按审查引擎分流：ocr（默认）→ 原执行器；argus → ArgusExecutor。
type EngineExecutor struct {
	ocr   Executor
	argus Executor
}

// NewEngineExecutor 包装 OCR 执行器；argus 为 nil 时 engine=argus 请求报错
// （正常装配下创建期已被 ArgusEnabled 拦截，此处兜底）。
func NewEngineExecutor(ocr Executor, argus Executor) *EngineExecutor {
	return &EngineExecutor{ocr: ocr, argus: argus}
}

func (e *EngineExecutor) Execute(ctx context.Context, id string, req domain.Request,
	provider domain.Provider, reporter progress.ProgressReporter) (domain.Response, error) {
	if req.Engine == domain.EngineArgus {
		if e.argus == nil {
			return domain.Response{}, fmt.Errorf("argus 引擎未装配（bot_url 未配置？）: review=%s", id)
		}
		return e.argus.Execute(ctx, id, req, provider, reporter)
	}
	return e.ocr.Execute(ctx, id, req, provider, reporter)
}

// ArgusExecutor 触发 Argus 审查任务并维护 Record 生命周期。
type ArgusExecutor struct {
	client     *argusclient.Client
	reviewRepo repository.ReviewRepository
}

// NewArgusExecutor 构造 argus 执行器（client.Enabled()=false 时不应被装配）。
func NewArgusExecutor(client *argusclient.Client, reviewRepo repository.ReviewRepository) *ArgusExecutor {
	return &ArgusExecutor{client: client, reviewRepo: reviewRepo}
}

func (a *ArgusExecutor) Execute(ctx context.Context, id string, req domain.Request,
	_ domain.Provider, _ progress.ProgressReporter) (domain.Response, error) {
	owner, repo, err := splitOwnerRepo(req.Repo)
	if err != nil {
		a.failRecord(ctx, id, req, err)
		return domain.Response{}, err
	}
	number := req.PullRequestID
	if number == "" {
		number = req.MergeRequestIID
	}

	started := time.Now()
	res, err := a.client.TriggerReview(ctx, req.Provider, owner, repo, number)
	duration := time.Since(started).Milliseconds()
	if err != nil {
		a.failRecord(ctx, id, req, err)
		return domain.Response{}, err
	}

	msg := "argus 审查任务已创建，等待回报"
	if res.Duplicated {
		msg = "argus 同 PR 已有在飞任务，复用既有任务等待回报"
	}
	evt := domain.Event{
		Stage:      "dispatch",
		Status:     "completed",
		Message:    msg,
		How:        "argus trigger (POST /tasks, kind=CODE_REVIEW)",
		Input:      map[string]any{"platform": req.Provider, "owner": owner, "repo": repo, "number": number},
		Output:     map[string]any{"taskId": res.TaskID, "status": res.Status, "duplicated": res.Duplicated},
		DurationMs: duration,
		CreatedAt:  started.UTC(),
	}

	var out domain.Response
	err = a.reviewRepo.UpdateInTransaction(ctx, id, func(rec *domain.Record) error {
		rec.Status = statusRunning
		rec.Response.Status = statusRunning
		rec.Response.Summary = msg
		rec.Response.Gate = domain.PendingGate()
		// 任务 ID 落记录：失败轮询（argus 侧任务失败不回推）与回报关联都靠它。
		rec.Response.ArgusTaskID = res.TaskID
		rec.Response.History = append(rec.Response.History, evt)
		out = rec.Response
		return nil
	})
	if err != nil {
		logx.CtxErrorf(ctx, "argus dispatch: persist running record failed: review=%s err=%v", id, err)
		return domain.Response{}, err
	}
	logx.Infof("argus dispatch: review=%s task=%s duplicated=%v", id, res.TaskID, res.Duplicated)
	return out, nil
}

// failRecord 触发失败落终态（worker 只记日志，持久化由执行器自负）。
func (a *ArgusExecutor) failRecord(ctx context.Context, id string, req domain.Request, cause error) {
	err := a.reviewRepo.UpdateInTransaction(ctx, id, func(rec *domain.Record) error {
		rec.Status = statusFailed
		rec.Response.Status = statusFailed
		rec.Response.Error = cause.Error()
		rec.Response.Summary = "argus 任务触发失败"
		rec.Response.Gate = domain.ErrorGate(cause.Error())
		rec.Response.History = append(rec.Response.History, domain.Event{
			Stage: "dispatch", Status: "failed", Message: cause.Error(),
			How: "argus trigger (POST /tasks)", CreatedAt: time.Now().UTC(),
		})
		return nil
	})
	if err != nil {
		logx.CtxErrorf(ctx, "argus dispatch: persist failure record failed: review=%s err=%v", id, err)
	}
}

// splitOwnerRepo 拆 "owner/repo"；无 "/" 视为非法（argus 契约 owner/repo 分列出参）。
func splitOwnerRepo(full string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(full), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("argus 引擎要求 repo 为 owner/repo 形式，got %q", full)
	}
	return parts[0], parts[1], nil
}

// CompleteFromArgusReport 把 argus push 回报收敛到对应的 active Record 终态。
// 匹配键：provider=platform（小写）、repo=owner/repo、PR number、（可选）head_sha，
// 且 Record 引擎为 argus。不匹配（纯 push 场景/engine=ocr/head 不一致）为 no-op。
// 由 argus_service.IngestHandler 在 upsert 成功后调用。
func CompleteFromArgusReport(ctx context.Context, reviewRepo repository.ReviewRepository,
	rep *argusingest.Review, argusID uint) {
	if reviewRepo == nil || rep == nil {
		return
	}
	repo := rep.Owner + "/" + rep.Repo
	rec, err := reviewRepo.GetActiveByProviderRepo(ctx, strings.ToLower(rep.Platform), repo, rep.Number, "")
	if err != nil {
		return // 无 active 记录（纯 push 场景）或查询失败——push 落库本身已成功
	}
	if rec.Request.Engine != domain.EngineArgus {
		return
	}
	if rec.HeadSHA != "" && rec.HeadSHA != rep.HeadSHA {
		return // active 记录对应更新的 head，本次回报是旧 SHA 的补报
	}

	status := statusSuccess
	summary := fmt.Sprintf("argus 裁决：%s（H%d M%d L%d）", argusVerdictLabel(rep.Verdict),
		rep.FindingsHigh, rep.FindingsMedium, rep.FindingsLow)
	gate := domain.CompletedGate(true, "argus: "+rep.Verdict)
	switch rep.Verdict {
	case argusingest.VerdictRequestChanges:
		gate = domain.CompletedGate(false, "argus: request_changes")
	case argusingest.VerdictError:
		status = statusFailed
		gate = domain.ErrorGate("argus: " + rep.ErrorMessage)
		if rep.ErrorMessage != "" {
			summary = "argus 审查失败：" + rep.ErrorMessage
		}
	}

	evt := domain.Event{
		Stage:   "argus_report",
		Status:  "completed",
		Message: summary,
		How:     "argus push (POST /api/v1/argus/reviews)",
		Input: map[string]any{
			"argus_review_id": argusID, "argus_task_id": rep.ArgusTaskID,
			"verdict": rep.Verdict, "platform": rep.Platform,
			"findings_total": rep.FindingsTotal, "findings_high": rep.FindingsHigh,
			"findings_medium": rep.FindingsMedium, "findings_low": rep.FindingsLow,
		},
		DurationMs: rep.DurationMS,
		CreatedAt:  rep.OccurredAt.UTC(),
	}

	findings := argusFindingsToDomain(rep.Findings)
	err = reviewRepo.UpdateInTransaction(ctx, rec.ID, func(r *domain.Record) error {
		r.Status = status
		r.Response.Status = status
		r.Response.Summary = summary
		r.Response.Gate = gate
		// 意见随收敛写入评审记录：详情页直接展示（此前只在 Argus 承接表，
		// 详情页会显示「未发现问题」，需要跳页才能看到意见）。
		r.Response.Findings = findings
		if status == statusFailed {
			r.Response.Error = rep.ErrorMessage
		}
		r.Response.History = append(r.Response.History, evt)
		return nil
	})
	if err != nil {
		logx.CtxErrorf(ctx, "argus report: complete record failed: review=%s argus=%d err=%v", rec.ID, argusID, err)
		return
	}
	logx.Infof("argus report: review=%s completed by argus push (verdict=%s)", rec.ID, rep.Verdict)
}

func argusVerdictLabel(v string) string {
	switch v {
	case argusingest.VerdictApprove:
		return "通过"
	case argusingest.VerdictComment:
		return "评论"
	case argusingest.VerdictRequestChanges:
		return "要求修改"
	case argusingest.VerdictError:
		return "审查失败"
	}
	return v
}

// argusFindingsToDomain argus 三档意见 → 评审记录的标准 Finding。
// 严重级对齐 OCR 分级：high→major（高风险）、medium→minor、low→info；
// argus 规范指纹直接用作 Finding.Fingerprint（语义同源：跨轮次识别同一问题）。
func argusFindingsToDomain(fs []argusingest.Finding) []domain.Finding {
	out := make([]domain.Finding, 0, len(fs))
	for _, f := range fs {
		out = append(out, domain.Finding{
			Severity:    argusSeverityToDomain(f.Severity),
			Category:    "argus",
			Path:        f.File,
			Line:        f.Line,
			StartLine:   f.Line,
			EndLine:     f.Line,
			Title:       argusFindingTitle(f.Comment),
			Body:        f.Comment,
			Suggestion:  f.SuggestedFix,
			Fingerprint: f.FP,
			Agents:      argusFindingAgents(f.By),
		})
	}
	return out
}

func argusSeverityToDomain(s string) string {
	switch s {
	case argusingest.SeverityHigh:
		return domain.SeverityMajor
	case argusingest.SeverityMedium:
		return domain.SeverityMinor
	default:
		return domain.SeverityInfo
	}
}

// argusFindingTitle 取意见首句做标题（argus 评语是整段文字，前端列表只展示标题）。
func argusFindingTitle(comment string) string {
	trimmed := strings.TrimSpace(comment)
	if i := strings.IndexAny(trimmed, "。；\n"); i > 0 && i <= 120 {
		return trimmed[:i]
	}
	if len(trimmed) > 120 {
		cut := trimmed[:120]
		if i := strings.LastIndexAny(cut, "，, "); i > 60 {
			cut = cut[:i]
		}
		return cut + "…"
	}
	return trimmed
}

func argusFindingAgents(by string) []string {
	by = strings.TrimSpace(by)
	if by == "" {
		return []string{"argus"}
	}
	parts := strings.Split(by, ",")
	agents := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			agents = append(agents, p)
		}
	}
	if len(agents) == 0 {
		return []string{"argus"}
	}
	return agents
}
