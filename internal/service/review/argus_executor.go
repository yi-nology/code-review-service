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
	notifdomain "github.com/yi-nology/code-review-service/internal/domain/notification"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/progress"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/statuscheck"

	"github.com/oekit/concurrency/async"
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
	// statusPoster 派发成功后向 PR head 写 pending status（nil 安全）。
	statusPoster domain.StatusPoster
}

// NewArgusExecutor 构造 argus 执行器（client.Enabled()=false 时不应被装配）。
func NewArgusExecutor(client *argusclient.Client, reviewRepo repository.ReviewRepository,
	statusPoster domain.StatusPoster) *ArgusExecutor {
	return &ArgusExecutor{client: client, reviewRepo: reviewRepo, statusPoster: statusPoster}
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
	res, err := a.client.TriggerTask(ctx, argusclient.TriggerOptions{
		Platform:  req.Provider,
		Owner:     owner,
		Repo:      repo,
		Number:    number,
		Kind:      argusKind(req.TaskKind),
		Artifacts: argusArtifacts(req),
	})
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
		Stage:   "dispatch",
		Status:  "completed",
		Message: msg,
		How:     "argus trigger (POST /tasks, kind=" + domain.NormalizeTaskKind(req.TaskKind) + ")",
		Input: map[string]any{
			"platform": req.Provider, "owner": owner, "repo": repo, "number": number,
			"task_kind": domain.NormalizeTaskKind(req.TaskKind),
		},
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
	// 派发即发 pending status：PR 作者立刻可见审查已启动（与 OCR 路径 notifyStart 对齐）。
	domain.PostStatus(a.statusPoster, context.Background(), req.Provider, req.Repo, req.HeadSHA,
		statuscheck.StatePending, "Argus 代码审查进行中…", id)
	logx.Infof("argus dispatch: review=%s task=%s duplicated=%v kind=%s",
		id, res.TaskID, res.Duplicated, domain.NormalizeTaskKind(req.TaskKind))
	return out, nil
}

// argusKind 任务类型词表 → argus 整数枚举（NormalizeTaskKind 已在 validate 保证合法）。
func argusKind(kind string) int {
	switch domain.NormalizeTaskKind(kind) {
	case domain.TaskKindRequirementCompliance:
		return argusclient.TaskKindRequirementCompliance
	case domain.TaskKindVisualCheck:
		return argusclient.TaskKindVisualCheck
	default:
		return argusclient.TaskKindCodeReview
	}
}

// argusArtifacts 请求携带需求时注入 artifacts（argus 需求复核 workflow 的需求来源）。
func argusArtifacts(req domain.Request) map[string]string {
	if len(req.Requirements) == 0 {
		return nil
	}
	return map[string]string{"requirements": strings.Join(req.Requirements, "\n")}
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
		return
	}
	// 门禁留痕按 gate_mode：strong 触发失败写 error status（与 OCR 失败语义一致）；
	// weak（argus 默认）不发——此时尚未发过 pending，PR 保持干净，留痕走记录+通知。
	if domain.EffectiveGateMode(req.GateMode, domain.EngineArgus) == domain.GateModeStrong {
		domain.PostStatus(a.statusPoster, context.Background(), req.Provider, req.Repo, req.HeadSHA,
			statuscheck.StateError, "Argus 审查触发失败："+cause.Error(), id)
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
// notifier 为收敛完成后的应用内通知出口（nil 安全，与 OCR 路径 notifyCompletion 对齐）；
// statusPoster 收敛后按 gate_mode 向 PR head 写门禁 commit status（nil 安全，弱门禁默认）；
// 由 argus_service.IngestHandler 在 upsert 成功后调用，亦被 ArgusPoller 恢复路径复用。
func CompleteFromArgusReport(ctx context.Context, reviewRepo repository.ReviewRepository,
	notifier domain.Notifier, statusPoster domain.StatusPoster, rep *argusingest.Review, argusID uint) {
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
	applied := false
	err = reviewRepo.UpdateInTransaction(ctx, rec.ID, func(r *domain.Record) error {
		if r.Status != statusRunning {
			return nil // 已被并发收敛（sink push 与轮询器竞态），不覆盖、不重复通知
		}
		applied = true
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
	if !applied {
		logx.CtxInfof(ctx, "argus report: review=%s already converged by concurrent path, skip", rec.ID)
		return
	}
	logx.Infof("argus report: review=%s completed by argus push (verdict=%s)", rec.ID, rep.Verdict)
	postArgusGateStatus(ctx, statusPoster, rec.Request.Provider, rec.Request.Repo,
		shaOr(rep.HeadSHA, rec.HeadSHA), rec.Request.GateMode, status, gate, summary, rec.ID)
	notifyArgusCompletion(ctx, notifier, rec, rep, summary, status, gate)
}

// postArgusGateStatus 按 gate_mode 向 PR head 写门禁 status。strong 沿用 OCR 语义
// （passed→success / request_changes→failed / error→error）；weak（argus 默认）任何
// 终态一律 success 留痕——红色 status 会卡「所有检查通过后合并」类自动合并。
func postArgusGateStatus(ctx context.Context, poster domain.StatusPoster, platform, repoPath, sha, gateMode, status string,
	gate domain.Gate, summary, reviewID string) {
	state := statuscheck.StateSuccess
	if domain.EffectiveGateMode(gateMode, domain.EngineArgus) == domain.GateModeStrong {
		if gate.Status == domain.GateFailed {
			state = statuscheck.StateFailed
		}
		if status == statusFailed {
			state = statuscheck.StateError
		}
	} else {
		summary = domain.WeakGateDesc(summary)
	}
	domain.PostStatus(poster, ctx, platform, repoPath, sha, state, summary, reviewID)
}

// shaOr 返回第一个非空 SHA。
func shaOr(preferred, fallback string) string {
	if preferred != "" {
		return preferred
	}
	return fallback
}

// notifyArgusCompletion argus 回报收敛后发应用内通知（复用 OCR 路径的事件契约，
// 前端通知中心零改动）。notifier nil 安全（未装配/单测静默跳过）；通知失败只告警，
// 不影响已落库的收敛结果。
func notifyArgusCompletion(ctx context.Context, notifier domain.Notifier,
	rec *domain.Record, rep *argusingest.Review, summary, status string, gate domain.Gate) {
	if notifier == nil {
		return
	}
	kind := notifdomain.KindCompleted
	if gate.Status == domain.GateFailed {
		kind = notifdomain.KindGateFailed
	}
	if status == statusFailed {
		kind = notifdomain.KindFailed
	}
	event := notifdomain.Event{
		ReviewID:   rec.ID,
		Provider:   rec.Request.Provider,
		Repo:       rec.Request.Repo,
		Ref:        rep.Number,
		Kind:       kind,
		GateStatus: gate.Status,
		Findings:   rep.FindingsTotal,
		Summary:    summary,
	}
	async.GoSafe(func() {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := notifier.Notify(notifyCtx, event); err != nil {
			logx.CtxWarnf(notifyCtx, "[notification] argus review %s kind=%s notify failed: %v",
				event.ReviewID, event.Kind, err)
		}
	})
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
