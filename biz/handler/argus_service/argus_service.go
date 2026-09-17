// Package argus_service 承接 Argus（PR 自动审查 Bot）审查结果的上报链路（push 模型）。
//
// Argus 审查完成后 POST /api/v1/argus/reviews 落库，前端「Argus 审查」页
// （ArgusReviews.vue / ArgusReviewDetail.vue）展示任务清单与意见明细。
// 契约由 IDL 约束：请求/响应模型为 idl/api.thrift + idl/review.thrift 声明、
// hz 生成的 biz/model/review.Argus*（本包不做本地 wire 结构体）；路由同 IDL
// 声明，注册于 biz/router/review_service。端点经全局 auth middleware 鉴权
// （与其它 /api/v1/* 一致）。
//
// 幂等：任务业务键 (platform, owner, repo, number, head_sha) upsert——同 PR 同
// SHA 重审覆盖；意见整组替换，(review_id, fp) 唯一约束兜底（fp 为 Argus 规范指纹）。
package argus_service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelReview "github.com/yi-nology/code-review-service/biz/model/review"
	"github.com/yi-nology/code-review-service/internal/argusclient"
	"github.com/yi-nology/code-review-service/internal/domain/argusingest"
	"github.com/yi-nology/code-review-service/internal/repository"
	svcReview "github.com/yi-nology/code-review-service/internal/service/review"
	svcReviewDomain "github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var handlerInst *Handler

type Handler struct {
	repo repository.ArgusReviewRepository
	// reviewRepo 用于回报关联：push 落库后把 engine=argus 的 active 评审收敛到终态。
	reviewRepo repository.ReviewRepository
	// argus 出站调用收口（触发/状态查询/误报抑制代理）；Enabled()=false 表示未配置。
	argus *argusclient.Client
	// notifier 回报收敛后的应用内通知出口（nil 安全）。
	notifier svcReviewDomain.Notifier
}

// InitHandler 装配 handler。argus 来自 config.Argus
// （凭证只在服务端，浏览器不接触 argus token）。
// notifier 用于回报收敛后的应用内通知（nil 安全）。
func InitHandler(repo repository.ArgusReviewRepository, reviewRepo repository.ReviewRepository,
	argus *argusclient.Client, notifier svcReviewDomain.Notifier) {
	handlerInst = &Handler{
		repo:       repo,
		reviewRepo: reviewRepo,
		argus:      argus,
		notifier:   notifier,
	}
}

// maxFindingsPerIngest 单次上报意见条数上限（防异常客户端刷库）。
const maxFindingsPerIngest = 500

// errUnknownFP fp 不属于该任务的意见（客户端错误，映射 400）。
var errUnknownFP = errors.New("fp 不属于该任务的任何意见")

// IngestHandler 处理 POST /api/v1/argus/reviews（Argus 服务端到服务端上报）。
func IngestHandler(ctx context.Context, c *app.RequestContext) {
	if !ready(c) {
		return
	}
	req, ok := common.Bind[modelReview.ArgusIngestReq](c)
	if !ok {
		return
	}
	rev, err := buildReview(req)
	if err != nil {
		common.Fail(ctx, c, common.BadRequest(err.Error()))
		return
	}
	id, err := handlerInst.repo.UpsertReview(ctx, rev)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	// 回报关联：把 engine=argus 的 active 评审收敛到终态（不匹配则 no-op，
	// 纯 push 场景不受影响；失败只记日志，不影响上报本身的成功语义）。
	svcReview.CompleteFromArgusReport(ctx, handlerInst.reviewRepo, handlerInst.notifier, rev, id)
	c.JSON(consts.StatusOK, &modelReview.ArgusIngestResp{ID: int64(id), Status: "accepted"})
}

// ListArgusReviews 处理 GET /api/v1/argus/reviews（任务清单，occurred_at 降序）。
// Query：repo（repo 名或 owner/repo）、platform（精确匹配）、
// severity（high|medium|low，"含该档意见"）、since（RFC3339）、page、page_size。
func ListArgusReviews(ctx context.Context, c *app.RequestContext) {
	if !ready(c) {
		return
	}
	req, ok := common.Bind[modelReview.ArgusListReviewsReq](c)
	if !ok {
		return
	}
	filter := argusingest.ListFilter{
		Repo:     strings.TrimSpace(req.GetRepo()),
		Platform: strings.TrimSpace(req.GetPlatform()),
		Severity: strings.TrimSpace(req.GetSeverity()),
		Page:     int(req.GetPage()),
		PageSize: int(req.GetPageSize()),
	}
	switch filter.Severity {
	case "", argusingest.SeverityHigh, argusingest.SeverityMedium, argusingest.SeverityLow:
	default:
		common.Fail(ctx, c, common.BadRequest("severity 仅支持 high、medium、low"))
		return
	}
	if raw := strings.TrimSpace(req.GetSince()); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			common.Fail(ctx, c, common.BadRequest("since 须为 RFC3339 时间"))
			return
		}
		filter.Since = &t
	}
	items, total, err := handlerInst.repo.ListReviews(ctx, filter)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	out := make([]*modelReview.ArgusReviewItem, 0, len(items))
	for i := range items {
		out = append(out, toReviewItem(&items[i]))
	}
	c.JSON(consts.StatusOK, &modelReview.ArgusListReviewsResp{Items: out, Total: total})
}

// GetArgusReview 处理 GET /api/v1/argus/reviews/:id（任务 + 意见明细）。
// :id 手动提取（house 惯例，同 GetReview：thrift struct 无法自动绑定路径参数）。
func GetArgusReview(ctx context.Context, c *app.RequestContext) {
	if !ready(c) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		common.Fail(ctx, c, common.BadRequest("非法任务 ID"))
		return
	}
	rev, err := handlerInst.repo.GetReview(ctx, uint(id))
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	c.JSON(consts.StatusOK, &modelReview.ArgusGetReviewResp{Review: toReviewItem(rev)})
}

// SuppressArgusFinding 处理 POST /api/v1/argus/reviews/:id/findings/suppress。
// 流程：校验 fp 属于该任务的意见 → 服务端代理调用 argus POST /findings/suppress
// （fingerprint 直传全值，argus 侧按前缀解析）→ 命中则本地落 suppress_state='suppressed'。
// matched=false（argus 未命中，行号/措辞漂移等）不落状态，由响应告知前端。
func SuppressArgusFinding(ctx context.Context, c *app.RequestContext) {
	if !ready(c) {
		return
	}
	if handlerInst.argus == nil || !handlerInst.argus.Enabled() {
		common.Fail(ctx, c, common.BizErr(400, "argus 对接未配置（argus.bot_url 为空），反馈回流关闭"))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		common.Fail(ctx, c, common.BadRequest("非法任务 ID"))
		return
	}
	req, ok := common.Bind[modelReview.ArgusSuppressReq](c)
	if !ok {
		return
	}
	fp := strings.TrimSpace(req.Fp)
	if fp == "" {
		common.Fail(ctx, c, common.BadRequest("fp 必填"))
		return
	}
	matched, err := doSuppress(ctx, handlerInst, uint(id), fp)
	if err != nil {
		if errors.Is(err, errUnknownFP) {
			common.Fail(ctx, c, common.BadRequest(err.Error()))
			return
		}
		common.Fail(ctx, c, err)
		return
	}
	if !matched {
		c.JSON(consts.StatusOK, &modelReview.ArgusSuppressResp{Matched: false,
			Message: strPtr("argus 侧未命中该指纹（意见可能已被重审覆盖或措辞漂移）")})
		return
	}
	c.JSON(consts.StatusOK, &modelReview.ArgusSuppressResp{Matched: true, State: strPtr("suppressed")})
}

// ---------- 内部 ----------

func ready(c *app.RequestContext) bool {
	if handlerInst == nil || handlerInst.repo == nil {
		common.Fail(context.Background(), c, common.BizErr(500, "argus handler not initialized"))
		return false
	}
	return true
}

func buildReview(req *modelReview.ArgusIngestReq) (*argusingest.Review, error) {
	trim := func(s string) string { return strings.TrimSpace(s) }
	platform, owner, repo := trim(req.Platform), trim(req.Owner), trim(req.Repo)
	number, headSHA, verdict := trim(req.Number), trim(req.HeadSha), trim(req.Verdict)
	for _, f := range []struct{ v, name string }{
		{platform, "platform"}, {owner, "owner"}, {repo, "repo"},
		{number, "number"}, {headSHA, "head_sha"},
	} {
		if f.v == "" {
			return nil, fmt.Errorf("%s 必填", f.name)
		}
	}
	switch verdict {
	case argusingest.VerdictApprove, argusingest.VerdictComment,
		argusingest.VerdictRequestChanges, argusingest.VerdictError:
	default:
		return nil, errors.New("verdict 仅支持 approve、comment、request_changes、error")
	}
	if len(req.Findings) > maxFindingsPerIngest {
		return nil, fmt.Errorf("findings 超过单次上限 %d", maxFindingsPerIngest)
	}
	occurredAt := time.Now()
	if raw := strings.TrimSpace(req.GetOccurredAt()); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, errors.New("occurred_at 须为 RFC3339 时间")
		}
		occurredAt = t
	}
	rev := &argusingest.Review{
		ArgusTaskID: trim(req.GetArgusTaskID()),
		Platform:    platform, Owner: owner, Repo: repo,
		Number: number, Title: strings.TrimSpace(req.GetTitle()),
		HeadSHA: headSHA, BaseBranch: trim(req.GetBaseBranch()),
		Verdict: verdict, Plugins: req.Plugins,
		DurationMS: req.GetDurationMs(), TokenUsage: int(req.GetTokenUsage()),
		ErrorMessage: trim(req.GetErrorMessage()),
		OccurredAt:   occurredAt,
	}
	// 同 payload 内非空 fp 去重（last-wins）：重复 fp 直接落库会撞
	// (review_id, fp) 部分唯一索引、整组入库失败；计数在去重后聚合。
	seenFP := make(map[string]struct{})
	for _, f := range req.Findings {
		if f == nil {
			continue
		}
		file, comment, severity := trim(f.File), strings.TrimSpace(f.Comment), trim(f.Severity)
		if file == "" || comment == "" {
			return nil, errors.New("finding 的 file 与 comment 必填")
		}
		switch severity {
		case argusingest.SeverityHigh, argusingest.SeverityMedium, argusingest.SeverityLow:
		default:
			return nil, errors.New("finding.severity 仅支持 high、medium、low")
		}
		finding := argusingest.Finding{
			FP: strings.TrimSpace(f.GetFp()), File: file, Line: int(f.Line),
			Severity: severity, Comment: comment,
			SuggestedFix: strings.TrimSpace(f.GetSuggestedFix()), By: strings.TrimSpace(f.GetBy()),
		}
		if finding.FP != "" {
			if _, dup := seenFP[finding.FP]; dup {
				for i := range rev.Findings {
					if rev.Findings[i].FP == finding.FP {
						rev.Findings[i] = finding
						break
					}
				}
				continue
			}
			seenFP[finding.FP] = struct{}{}
		}
		rev.Findings = append(rev.Findings, finding)
	}
	for _, f := range rev.Findings {
		switch f.Severity {
		case argusingest.SeverityHigh:
			rev.FindingsHigh++
		case argusingest.SeverityMedium:
			rev.FindingsMedium++
		case argusingest.SeverityLow:
			rev.FindingsLow++
		}
	}
	rev.FindingsTotal = len(rev.Findings)
	return rev, nil
}

// toReviewItem 领域实体 → IDL 响应模型（OccurredAt 转 RFC3339 文本，
// 可选字段空值出 nil→omitempty）。
func toReviewItem(rev *argusingest.Review) *modelReview.ArgusReviewItem {
	item := &modelReview.ArgusReviewItem{
		ID: int64(rev.ID), ArgusTaskID: rev.ArgusTaskID,
		Platform: rev.Platform, Owner: rev.Owner, Repo: rev.Repo,
		Number: rev.Number, Title: rev.Title, HeadSha: rev.HeadSHA,
		BaseBranch: strPtr(rev.BaseBranch), Verdict: rev.Verdict,
		FindingsTotal: int32(rev.FindingsTotal), FindingsHigh: int32(rev.FindingsHigh),
		FindingsMedium: int32(rev.FindingsMedium), FindingsLow: int32(rev.FindingsLow),
		Plugins: rev.Plugins, DurationMs: i64Ptr(rev.DurationMS),
		TokenUsage: i32Ptr(rev.TokenUsage), OccurredAt: rev.OccurredAt.Format(time.RFC3339),
		ErrorMessage: strPtr(rev.ErrorMessage),
		Findings:     make([]*modelReview.ArgusFindingItem, 0, len(rev.Findings)),
	}
	for _, f := range rev.Findings {
		item.Findings = append(item.Findings, &modelReview.ArgusFindingItem{
			Fp: strPtr(f.FP), File: f.File, Line: int32(f.Line), Severity: f.Severity,
			Comment: f.Comment, SuggestedFix: strPtr(f.SuggestedFix), By: strPtr(f.By),
			SuppressState: strPtr(f.SuppressState),
		})
	}
	return item
}

// doSuppress 可测核心：fp 归属校验 → 代理调用 argus → 落本地状态。
func doSuppress(ctx context.Context, h *Handler, reviewID uint, fp string) (bool, error) {
	rev, err := h.repo.GetReview(ctx, reviewID)
	if err != nil {
		return false, err
	}
	known := false
	for _, f := range rev.Findings {
		if f.FP == fp {
			known = true
			break
		}
	}
	if !known {
		return false, fmt.Errorf("fp %q 不属于任务 %d 的任何意见: %w", fp, reviewID, errUnknownFP)
	}

	matched, err := h.argus.SuppressFinding(ctx, rev.Platform, rev.Owner, rev.Repo, rev.Number, fp)
	if err != nil {
		return false, err
	}
	if !matched {
		return false, nil
	}
	if err := h.repo.SetFindingSuppressState(ctx, reviewID, fp, "suppressed"); err != nil {
		return false, fmt.Errorf("落抑制状态失败: %w", err)
	}
	return true, nil
}

func strPtr(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func i64Ptr(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func i32Ptr(v int) *int32 {
	if v == 0 {
		return nil
	}
	i := int32(v)
	return &i
}
