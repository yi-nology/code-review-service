package mcp

import (
	"context"
	"errors"
	"strings"

	"github.com/yi-nology/code-review-service/internal/app"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/mark3labs/mcp-go/mcp"
)

// createReviewTool 是 Tier 1 第一个 tool,也是 minicode 90% 场景的入口。
//
// 设计决策（参考 docs/MCP_DESIGN.md）：
//   - wait 默认 true,timeout 默认 180s。minicode 在 IDE 里写完代码想立刻看到
//     评审结果,这是高频路径;timeout 180s 覆盖大多数 PR(单 agent < 60s,
//     多 agent 100-180s)。超时不报错,改成返 review_id + status=queued 提示轮询,
//     避免 LLM 重试浪费 LLM 调用。
//   - idempotency_key 必填 (mcp side, 实际由 SDK 帮我们填):
//     minicode 触发场景里同一个 SHA 经常重复点击,idempotency 防止创建多份。
//   - requirements 直接透传 domain.Request,workflow 多 agent 评审会用到。
//   - author/author_name/author_avatar 走 request 字段,跟 web console 行为一致,
//     写回 PR 评论时署名用。
func createReviewTool() mcp.Tool {
	opts := append([]mcp.ToolOption{
		mcp.WithDescription("创建一次代码评审。可同步等待结果（wait=true, 默认 180s 超时），或异步返回 review_id 后续轮询。"),
	}, reviewRequestSchema()...)
	opts = append(opts,
		WithBoolParam("wait", false, "是否同步等待评审完成（默认 true，超时降级为异步）", true),
		WithIntParam("timeout_seconds", false, "wait=true 时的同步等待超时（秒），默认 180，最大 600", 180),
		WithStringParam("idempotency_key", false, "幂等键，同一 key 重复请求返回同一 review（防止 IDE 误触双开）"),
	)
	return mcp.NewTool("create_review", opts...)
}

func RegisterCreateReview(s *Server, svc app.ReviewService) {
	s.AddTool(createReviewTool(), createReviewHandler(svc))
}

func createReviewHandler(svc app.ReviewService) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		// 必填参数提取（统一走 requireString，错误直接 NewToolError）。
		provider, err := requireString(args, "provider")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_provider", "传 provider 字段", "%v", err), nil
		}
		repo, err := requireString(args, "repo")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_repo", "传 repo 字段 (owner/repo 格式)", "%v", err), nil
		}
		headSHA, err := requireString(args, "head_sha")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_head_sha", "传 head_sha 字段（head commit SHA）", "%v", err), nil
		}

		// 把 git SDK 接受的 provider 名大小写归一化（"GitHub" -> "github"），
		// 避免 LLM 给大写导致 service 层 ErrUnsupportedProvider。
		provider = strings.ToLower(strings.TrimSpace(provider))

		domainReq := domain.Request{
			Provider:        provider,
			Repo:            strings.TrimSpace(repo),
			ProjectID:       optionalString(args, "project_id"),
			PullRequestID:   optionalString(args, "pull_request_id"),
			MergeRequestIID: optionalString(args, "merge_request_iid"),
			BaseSHA:         optionalString(args, "base_sha"),
			HeadSHA:         headSHA,
			Author:          optionalString(args, "author"),
			AuthorName:      optionalString(args, "author_name"),
			AuthorAvatar:    optionalString(args, "author_avatar"),
		}

		if reqs, ok := args["requirements"].([]any); ok {
			for _, r := range reqs {
				if str, ok := r.(string); ok && strings.TrimSpace(str) != "" {
					domainReq.Requirements = append(domainReq.Requirements, str)
				}
			}
		}

		// wait / timeout 处理：默认 180s, 范围 [10, 600]。
		wait := true
		if v, ok := args["wait"].(bool); ok {
			wait = v
		}
		var timeoutSec float64
		if v, ok := args["timeout_seconds"].(float64); ok {
			timeoutSec = v
		}

		// wait=false：直接创建立刻返回 review_id（status=queued），客户端后续轮询。
		if !wait {
			domainReq.Wait = false
			idempotencyKey := optionalString(args, "idempotency_key")
			resp, err := svc.Create(ctx, domainReq, idempotencyKey)
			if err != nil {
				return classifyCreateError(err), nil
			}
			return JSONResult(map[string]any{
				"review_id": resp.ReviewID,
				"status":    resp.Status,
				"summary":   resp.Summary,
				"wait":      false,
				"hint":      "异步模式：调用 get_review(review_id) 轮询结果。",
			})
		}

		// wait=true：派生带 timeout 的 ctx 调 Create。
		// Create 内部走 Service.Create -> req.Wait=true -> Executor.Execute 同步等。
		// 超时后 ctx.Done() 透传到 executor，会取消 OCR 子进程；
		// 此时 service 把 review 留在 DB status=running,客户端可以继续轮询。
		domainReq.Wait = true
		timeoutCtx, cancel := context.WithTimeout(ctx, parseTimeoutSec(timeoutSec))
		defer cancel()

		idempotencyKey := optionalString(args, "idempotency_key")
		resp, err := svc.Create(timeoutCtx, domainReq, idempotencyKey)
		if err != nil {
			// ctx deadline 超时但 review 仍可能继续在后台跑：返回 timeout 类错误 +
			// review_id 提示客户端轮询。前提：service.Create 在 ctx 取消时仍
			// 返回有效的 review_id（partial response）。review.Service.Create
			// 当前实现里，executor 取消后会返回 error，不带 resp — 此时
			// 我们无法给 review_id，只能让客户端后续按 (provider, repo, head_sha)
			// 调 list_reviews 自己找。
			if errors.Is(err, context.DeadlineExceeded) {
				return NewToolErrorf(ErrCategoryTimeout, "wait_timeout",
					"用 list_reviews(provider, repo, head_sha) 找 status=running 的 review 继续轮询",
					"同步等待超时，评审仍在后台执行"), nil
			}
			return classifyCreateError(err), nil
		}

		// 同步成功：返回完整 review。
		return JSONResult(map[string]any{
			"review_id":      resp.ReviewID,
			"status":         resp.Status,
			"summary":        resp.Summary,
			"gate":           resp.Gate,
			"score":          resp.Report.Score,
			"counts":         resp.Report.Counts,
			"findings_count": len(resp.Findings),
			"wait":           true,
			"timed_out":      false,
		})
	}
}

// classifyCreateError 把 Create 失败的错误翻译到 MCP 错误。
//
// 比 ClassifyError 更细致：要识别 ErrUnsupportedProvider / ErrIdempotencyConflict
// 等业务 sentinel，让客户端得到具体 actionable 提示。
func classifyCreateError(err error) *mcp.CallToolResult {
	switch {
	case errors.Is(err, domain.ErrUnsupportedProvider):
		return NewToolError(ErrCategoryUnsupported, "unsupported_provider",
			err.Error(),
			"调用 list_provider_repos 看已配置的 platform 列表", false)
	case errors.Is(err, domain.ErrIdempotencyConflict):
		return NewToolError(ErrCategoryConflict, "idempotency_conflict",
			"同一 idempotency_key 已被不同请求内容占用",
			"换一个新的 idempotency_key 或省略该字段", false)
	case errors.Is(err, review.ErrInvalidState):
		return NewToolError(ErrCategoryConflict, "invalid_state",
			err.Error(),
			"检查 review 状态后重试", false)
	}
	return ClassifyError(err)
}
