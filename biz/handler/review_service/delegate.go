package review_service

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/domain/review"

	"github.com/oekit/pkg/ptrx"

	"github.com/cloudwego/hertz/pkg/app"
)

// DelegatePreviewReq 是 POST /api/v1/delegate/preview 的请求体。
//
// 与 PreviewReq 字段一致（provider+repo+PR/SHA），额外接受可选 exclude（gitignore-style 排除模式）。
// 不走 thrift IDL：delegate 是 OCR v1.7.11+ 的可选能力，作为手写路由暴露（见 server.go
// registerDelegateRoutes），避免对 generated router/biz/model 的大规模手改。
type DelegatePreviewReq struct {
	Provider        string  `json:"provider" vd:"required"`
	Repo            string  `json:"repo" vd:"required"`
	ProjectID       *string `json:"project_id,omitempty"`
	PullRequestID   *string `json:"pull_request_id,omitempty"`
	MergeRequestIID *string `json:"merge_request_iid,omitempty"`
	BaseSHA         *string `json:"base_sha,omitempty"`
	HeadSHA         *string `json:"head_sha,omitempty"`
	// Exclude 是逗号分隔的 gitignore-style 排除模式，透传给 OCR --exclude。
	Exclude *string `json:"exclude,omitempty"`
}

// DelegateRuleReq 是 POST /api/v1/delegate/rule 的请求体。
type DelegateRuleReq struct {
	Provider        string  `json:"provider" vd:"required"`
	Repo            string  `json:"repo" vd:"required"`
	ProjectID       *string `json:"project_id,omitempty"`
	PullRequestID   *string `json:"pull_request_id,omitempty"`
	MergeRequestIID *string `json:"merge_request_iid,omitempty"`
	BaseSHA         *string `json:"base_sha,omitempty"`
	HeadSHA         *string `json:"head_sha,omitempty"`
	// Paths 是要查询规则的目标文件路径列表；为空时返回全量规则。
	Paths []string `json:"paths,omitempty"`
}

// DelegatePreviewHandler 处理 POST /api/v1/delegate/preview。
//
// 调用 OCR v1.7.11+ `ocr delegate preview`，返回将参与评审的文件列表 + mode/from/to/merge_base/
// total_*/Files 元数据。host-agent 范式：不调 LLM，OCR 只做文件选择。
//
// 失败模式：
//   - provider 不存在 → reviewError 映射
//   - OCR < v1.7.11 或 Delegator 未注入 → 503-like 错误
//   - clone 失败 / OCR 子进程失败 → 原始错误
func (h *ReviewHandler) delegatePreviewHandler(ctx context.Context, c *app.RequestContext) {
	var req DelegatePreviewReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(ctx, c, common.BadRequest(fmt.Sprintf("invalid request: %v", err)))
		return
	}

	svcReq := domain.Request{
		Provider:        req.Provider,
		Repo:            req.Repo,
		ProjectID:       ptrx.Deref(req.ProjectID, ""),
		PullRequestID:   ptrx.Deref(req.PullRequestID, ""),
		MergeRequestIID: ptrx.Deref(req.MergeRequestIID, ""),
		BaseSHA:         ptrx.Deref(req.BaseSHA, ""),
		HeadSHA:         ptrx.Deref(req.HeadSHA, ""),
	}
	var exclude []string
	if e := strings.TrimSpace(ptrx.Deref(req.Exclude, "")); e != "" {
		exclude = strings.Split(e, ",")
	}

	result, err := h.reviewSvc.DelegatePreview(ctx, svcReq, exclude)
	if err != nil {
		common.Fail(ctx, c, mapReviewError(err))
		return
	}
	common.Success(c, result)
}

// DelegatePreviewHandler 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func DelegatePreviewHandler(ctx context.Context, c *app.RequestContext) {
	handlerInst.delegatePreviewHandler(ctx, c)
}

// DelegateRuleHandler 处理 POST /api/v1/delegate/rule。
//
// 调用 OCR v1.7.11+ `ocr delegate rule <paths...>`，返回按内容分组的解析规则 markdown。
func (h *ReviewHandler) delegateRuleHandler(ctx context.Context, c *app.RequestContext) {
	var req DelegateRuleReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(ctx, c, common.BadRequest(fmt.Sprintf("invalid request: %v", err)))
		return
	}

	svcReq := domain.Request{
		Provider:        req.Provider,
		Repo:            req.Repo,
		ProjectID:       ptrx.Deref(req.ProjectID, ""),
		PullRequestID:   ptrx.Deref(req.PullRequestID, ""),
		MergeRequestIID: ptrx.Deref(req.MergeRequestIID, ""),
		BaseSHA:         ptrx.Deref(req.BaseSHA, ""),
		HeadSHA:         ptrx.Deref(req.HeadSHA, ""),
	}

	result, err := h.reviewSvc.DelegateRule(ctx, svcReq, req.Paths)
	if err != nil {
		common.Fail(ctx, c, mapReviewError(err))
		return
	}
	common.Success(c, result)
}

// DelegateRuleHandler 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func DelegateRuleHandler(ctx context.Context, c *app.RequestContext) {
	handlerInst.delegateRuleHandler(ctx, c)
}
