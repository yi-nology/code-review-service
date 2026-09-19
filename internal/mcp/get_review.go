package mcp

import (
	"context"
	"strings"

	"github.com/yi-nology/code-review-service/internal/app"
	"github.com/yi-nology/code-review-service/internal/service/review"

	"github.com/mark3labs/mcp-go/mcp"
)

// get_review 返完整 review（gate/score/findings/report/raw_ocr）。
//
// LLM 友好设计：
//   - 默认不返 raw_ocr（OCR 原始输出可能有 100k+ tokens，喂给 LLM 浪费 context）。
//   - 客户端如果要看 OCR 原文，用 review_id 走 web console 或调单独的 get_raw_ocr tool
//     （未来 PR 补，本期先保证 detail 不爆 LLM context window）。
func RegisterGetReview(s *Server, svc app.ReviewReader) {
	s.AddTool(getReviewTool(), getReviewHandler(svc))
}

func getReviewHandler(svc app.ReviewReader) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		reviewID, err := requireString(args, "review_id")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_review_id", "传 review_id", "%v", err), nil
		}
		record, ok, err := svc.Get(ctx, reviewID)
		if err != nil {
			return ClassifyError(err), nil
		}
		if !ok {
			return NewToolErrorf(ErrCategoryNotFound, "review_not_found",
				"用 list_reviews 找类似 (provider, repo) 的历史 review",
				"review %s 不存在", reviewID), nil
		}

		// raw_ocr 巨大，按需返回 — 默认 nil。
		// 客户端要看 OCR 原始 JSON 时调 get_raw_ocr 单独拉（后续 PR 补）。
		result := map[string]any{
			"review_id":     record.ID,
			"provider":      record.Provider,
			"repo":          record.Repo,
			"status":        record.Status,
			"summary":       record.Summary,
			"score":         record.Score,
			"grade":         record.Grade,
			"author":        record.Author,
			"author_name":   record.AuthorName,
			"author_avatar": record.AuthorAvatar,
			"request":       record.Request,
			"response":      record.Response,
			"created_at":    record.CreatedAt,
			"updated_at":    record.UpdatedAt,
		}
		return JSONResult(result)
	}
}

// list_reviews 返分页 + 聚合 stats（按 status/gate/repo 过滤）。
//
// 返回结构跟 HTTP API 的 /api/v1/reviews 保持一致（item/stats 字段名一致），
// 客户端既可以用 mcp tool 查，也可以从 web console 切到 HTTP 路径拿同样的数据。
func RegisterListReviews(s *Server, svc app.ReviewReader) {
	s.AddTool(listReviewsTool(), listReviewsHandler(svc))
}

func listReviewsHandler(svc app.ReviewReader) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		opts := review.ListOptions{
			Status:     strings.TrimSpace(optionalString(args, "status")),
			GateStatus: strings.TrimSpace(optionalString(args, "gate_status")),
			Provider:   "", // ListOptions 没 Provider 字段 (用 Status 隐式),但 List 有 — 看下
			Repo:       strings.TrimSpace(optionalString(args, "repo")),
		}
		// page / page_size 转换：HTTP 用 page (1-based) + page_size,这里语义一致。
		page := 1
		if v, ok := args["page"].(float64); ok && v > 0 {
			page = int(v)
		}
		pageSize := 20
		if v, ok := args["page_size"].(float64); ok && v > 0 {
			pageSize = int(v)
			if pageSize > 100 {
				pageSize = 100
			}
			if pageSize < 1 {
				pageSize = 1
			}
		}
		opts.Limit = pageSize
		opts.Offset = (page - 1) * pageSize

		result, err := svc.List(ctx, opts)
		if err != nil {
			return ClassifyError(err), nil
		}
		return JSONResult(map[string]any{
			"items":       result.Items,
			"total":       result.Total,
			"page":        result.Page,
			"page_size":   result.PageSize,
			"total_pages": result.TotalPages,
			"stats":       result.Stats,
		})
	}
}

// cancel_review 把 queued/running review 置为 cancelled。已完成的 review 拒绝。
func RegisterCancelReview(s *Server, svc app.ReviewLifecycle) {
	s.AddTool(cancelReviewTool(), cancelReviewHandler(svc))
}

func cancelReviewHandler(svc app.ReviewLifecycle) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		reviewID, err := requireString(args, "review_id")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_review_id", "传 review_id", "%v", err), nil
		}
		resp, err := svc.Cancel(ctx, reviewID)
		if err != nil {
			// ErrNotFound→not_found / ErrInvalidState→conflict 由 ClassifyError 统一判定。
			return ClassifyError(err), nil
		}
		return JSONResult(map[string]any{
			"review_id": resp.ReviewID,
			"status":    resp.Status,
			"summary":   resp.Summary,
		})
	}
}

// rerun_review 重新跑一次评审：基于原 review_id 拿 Request，create 新 review。
//
// 为什么不直接 Requeue？
//   - Requeue 只把 status 改回 queued,**不主动调 Executor**;主服务靠 WorkerModule
//     拉任务。MCP 进程不跑 worker,Requeue 完 review 永远卡在 queued。
//   - Create(wait=true) 同步等结果,跟 minicode 「再评一次拿结论」的场景匹配。
//   - 副作用：会创建新 review_id,原 review 历史保留,关联靠同 (provider, repo,
//     head_sha) + Request 字段 (后续 list_reviews 可以 dedup)。
func RegisterRerunReview(s *Server, svc app.ReviewService) {
	s.AddTool(rerunReviewTool(), rerunReviewHandler(svc))
}

func rerunReviewHandler(svc app.ReviewService) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		reviewID, err := requireString(args, "review_id")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_review_id", "传 review_id", "%v", err), nil
		}
		// 幂等键：客户端对同一目标的重复 rerun 传同值可去重（双击/重试场景）。
		idempotencyKey, _ := args["idempotency_key"].(string)

		// 拿原 review 的 Request 字段重建。
		record, ok, err := svc.Get(ctx, reviewID)
		if err != nil {
			return ClassifyError(err), nil
		}
		if !ok {
			return NewToolErrorf(ErrCategoryNotFound, "review_not_found",
				"用 list_reviews 找原 review",
				"原 review %s 不存在,无法重跑", reviewID), nil
		}

		// 复用原 request 字段,清空 review_id 跟 wait（rerun 跟 create 走同一路径）。
		domainReq := record.Request
		domainReq.ReviewID = ""
		domainReq.Wait = false // wait 由下面的 wait 参数决定

		// 跟 create_review 一致：wait / timeout 语义。
		wait := true
		if v, ok := args["wait"].(bool); ok {
			wait = v
		}
		var timeoutSec float64
		if v, ok := args["timeout_seconds"].(float64); ok {
			timeoutSec = v
		}

		if !wait {
			domainReq.Wait = false
			resp, err := svc.Create(ctx, domainReq, idempotencyKey)
			if err != nil {
				return classifyCreateError(err), nil
			}
			return JSONResult(map[string]any{
				"original_review_id": reviewID,
				"new_review_id":      resp.ReviewID,
				"status":             resp.Status,
				"summary":            resp.Summary,
				"wait":               false,
			})
		}

		domainReq.Wait = true
		newCtx, cancel := context.WithTimeout(ctx, parseTimeoutSec(timeoutSec))
		defer cancel()
		resp, err := svc.Create(newCtx, domainReq, idempotencyKey)
		if err != nil {
			return classifyCreateError(err), nil
		}
		return JSONResult(map[string]any{
			"original_review_id": reviewID,
			"new_review_id":      resp.ReviewID,
			"status":             resp.Status,
			"summary":            resp.Summary,
			"findings_count":     len(resp.Findings),
			"wait":               true,
		})
	}
}
