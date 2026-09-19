package mcp

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/yi-nology/code-review-service/internal/app"
	types "github.com/yi-nology/code-review-service/internal/domain/finding"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/suggestion"
	"github.com/yi-nology/code-review-service/internal/strutil"

	"github.com/mark3labs/mcp-go/mcp"
)

// get_review_findings 返 findings 裁剪版（按 severity/category/path_glob 过滤）。
//
// LLM 友好设计：
//   - 默认不返 ExistingCode / SuggestionCode / Thinking（可能很大），保留 Title /
//     Severity / Path / Line / Body（截 500 字）。
//   - 路径 glob 过滤用 filepath.Match，规则跟 .gitignore 风格一致（** 不支持，
//     用 filepath.Match 的单 * 通配，够覆盖 90% 场景）。
//   - 大 review (>50 findings) 默认截 30 条 + truncated 提示。
func RegisterGetReviewFindings(s *Server, svc app.ReviewReader) {
	s.AddTool(getReviewFindingsTool(), getReviewFindingsHandler(svc))
}

func getReviewFindingsHandler(svc app.ReviewReader) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			return NewToolErrorf(ErrCategoryNotFound, "review_not_found", "用 list_reviews 找 review",
				"review %s 不存在", reviewID), nil
		}

		severity := strings.ToLower(strings.TrimSpace(optionalString(args, "severity")))
		category := strings.ToLower(strings.TrimSpace(optionalString(args, "category")))
		pathGlob := strings.TrimSpace(optionalString(args, "path_glob"))
		limit := 30
		if v, ok := args["limit"].(float64); ok && v > 0 {
			limit = int(v)
			if limit > 200 {
				limit = 200
			}
		}

		// 过滤 + 裁剪。
		filtered := make([]map[string]any, 0, limit)
		total := 0
		truncated := false
		for _, f := range record.Response.Findings {
			if severity != "" && !strings.EqualFold(f.Severity, severity) {
				continue
			}
			if category != "" && !strings.EqualFold(f.Category, category) {
				continue
			}
			if pathGlob != "" {
				matched, _ := filepath.Match(pathGlob, f.Path)
				if !matched {
					continue
				}
			}
			total++
			if len(filtered) >= limit {
				truncated = true
				continue
			}
			filtered = append(filtered, trimFindingForLLM(f))
		}

		return JSONResult(map[string]any{
			"review_id": reviewID,
			"findings":  filtered,
			"total":     total,
			"truncated": truncated,
			"limit":     limit,
		})
	}
}

// trimFindingForLLM 删掉 finding 的大字段,只保留 LLM 决策需要的核心信息。
// Body 截 500 字:评审 description 经常很长,完整喂 LLM 浪费 context。
func trimFindingForLLM(f types.Finding) map[string]any {
	body := f.Body
	if len(body) > 500 {
		body = body[:500] + "..."
	}
	return map[string]any{
		"id":         f.ID,
		"severity":   f.Severity,
		"category":   f.Category,
		"path":       f.Path,
		"line":       f.Line,
		"start_line": f.StartLine,
		"end_line":   f.EndLine,
		"title":      f.Title,
		"body":       body,
		"suggestion": strutil.Truncate(f.Suggestion, 300),
		"agents":     f.Agents,
	}
}

// get_suggestion_patch 返 unified-diff patch（可直接 git apply）。
//
// 复用 internal/service/suggestion.BuildSuggestionPatch，跟 HTTP API
// /api/v1/reviews/:id/suggestion-patch/preview 同源 — 客户端不论走 mcp tool
// 还是 web console 都拿到一样的 patch。
//
// 不实现 apply — apply 涉及 git push + 创 PR,有副作用,不暴露给 LLM
// (LLM 误调一次会污染仓库)。要走 apply 走 web console 人工确认。
func RegisterGetSuggestionPatch(s *Server, svc app.ReviewReader) {
	s.AddTool(getSuggestionPatchTool(), getSuggestionPatchHandler(svc))
}

func getSuggestionPatchHandler(svc app.ReviewReader) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			return NewToolErrorf(ErrCategoryNotFound, "review_not_found", "用 list_reviews 找 review",
				"review %s 不存在", reviewID), nil
		}

		// finding_indexes 解析。
		var indexes []int
		if raw, ok := args["finding_indexes"].([]any); ok {
			for _, v := range raw {
				switch n := v.(type) {
				case float64:
					indexes = append(indexes, int(n))
				case int:
					indexes = append(indexes, n)
				}
			}
		}

		// 校验 indexes 范围。
		for _, idx := range indexes {
			if idx < 0 || idx >= len(record.Response.Findings) {
				return NewToolErrorf(ErrCategoryInvalidParam, "finding_index_out_of_range",
					"用 get_review_findings 拿当前 finding 数量",
					"finding index %d 超出范围 (0..%d)", idx, len(record.Response.Findings)-1), nil
			}
		}

		preview := suggestion.BuildSuggestionPatch(record.ID, record.Response.Findings, indexes)
		return JSONResult(map[string]any{
			"review_id":       preview.ReviewID,
			"patch":           preview.Patch,
			"items":           preview.Items,
			"skipped":         preview.Skipped,
			"generated_count": preview.GeneratedCount,
			"skipped_count":   preview.SkippedCount,
			"hint":            "patch 字段是 unified diff，可用 `git apply` 直接落盘。apply 走 web console 人工确认。",
		})
	}
}

// preview_change_files 调 svc.Preview 返「本次评审会扫描的文件列表」。
//
// 用途:评审前先看 scope,避免 LLM 给超大 PR 触发 review 把 OCR 跑爆。
// 行为是秒级（clone + listChanged,不调 LLM）。
func RegisterPreviewChangeFiles(s *Server, svc app.ReviewReader) {
	s.AddTool(previewChangeFilesTool(), previewChangeFilesHandler(svc))
}

func previewChangeFilesHandler(svc app.ReviewReader) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		// 复用 create_review 的参数解析,但有 PR/MR ID 时不需要 base_sha。
		provider, err := requireString(args, "provider")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_provider", "传 provider", "%v", err), nil
		}
		repo, err := requireString(args, "repo")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_repo", "传 repo", "%v", err), nil
		}
		headSHA, err := requireString(args, "head_sha")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_head_sha", "传 head_sha", "%v", err), nil
		}

		domainReq := domain.Request{
			Provider:        strings.ToLower(strings.TrimSpace(provider)),
			Repo:            strings.TrimSpace(repo),
			ProjectID:       optionalString(args, "project_id"),
			PullRequestID:   optionalString(args, "pull_request_id"),
			MergeRequestIID: optionalString(args, "merge_request_iid"),
			BaseSHA:         optionalString(args, "base_sha"),
			HeadSHA:         headSHA,
		}

		files, err := svc.Preview(ctx, domainReq)
		if err != nil {
			return classifyCreateError(err), nil
		}
		return JSONResult(map[string]any{
			"provider":   domainReq.Provider,
			"repo":       domainReq.Repo,
			"head_sha":   domainReq.HeadSHA,
			"file_count": len(files),
			"files":      files,
		})
	}
}

// scan_repository 跑全仓库扫描（不基于 PR diff），返 review_id 给客户端轮询。
//
// 行为：
//   - preview=true 走轻量预览（秒级,只列文件）
//   - preview=false 走异步扫描（OCR scan 模式,几十秒到几分钟），返 review_id
//     客户端用 get_review 轮询
func RegisterScanRepository(s *Server, svc app.ReviewReader) {
	s.AddTool(scanRepositoryTool(), scanRepositoryHandler(svc))
}

func scanRepositoryHandler(svc app.ReviewReader) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		provider, err := requireString(args, "provider")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_provider", "传 provider", "%v", err), nil
		}
		repo, err := requireString(args, "repo")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_repo", "传 repo", "%v", err), nil
		}
		headSHA, err := requireString(args, "head_sha")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_head_sha", "传 head_sha", "%v", err), nil
		}

		scanReq := domain.ScanRequest{
			Provider:   strings.ToLower(strings.TrimSpace(provider)),
			Repo:       strings.TrimSpace(repo),
			ProjectID:  optionalString(args, "project_id"),
			HeadSHA:    headSHA,
			Background: optionalString(args, "background"),
		}

		if v, ok := args["preview"].(bool); ok {
			scanReq.Preview = v
		}
		if raw, ok := args["paths"].([]any); ok {
			for _, p := range raw {
				if s, ok := p.(string); ok && strings.TrimSpace(s) != "" {
					scanReq.Paths = append(scanReq.Paths, s)
				}
			}
		}
		if raw, ok := args["exclude"].([]any); ok {
			for _, p := range raw {
				if s, ok := p.(string); ok && strings.TrimSpace(s) != "" {
					scanReq.Exclude = append(scanReq.Exclude, s)
				}
			}
		}

		result, err := svc.Scan(ctx, scanReq)
		if err != nil {
			return classifyCreateError(err), nil
		}
		return JSONResult(map[string]any{
			"review_id":     result.ReviewID,
			"preview":       scanReq.Preview,
			"preview_files": result.PreviewFiles,
			"file_count":    len(result.PreviewFiles),
			"hint":          "异步模式：用 get_review(review_id) 轮询结果。",
		})
	}
}
