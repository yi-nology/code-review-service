package mcp

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/app"

	"github.com/mark3labs/mcp-go/mcp"
)

// 本文件是 stub 占位集合：每个未实现的 tool 返回 notImplemented 错误。
// 真正实现由同名文件（get_review.go / list_reviews.go / …）覆盖 Register* 函数。
//
// 设计：
//   - stub 注册器只调 AddTool 一次；同名文件覆盖 Register* 时，stub 的注册就被 Go 的
//     「不允许重复定义函数」规则挡住。所以同名文件的实现会**替换** stub 的 Register*，
//     而不是两者并存。
//   - 走这套设计是因为 init 顺序不固定（fx.Invoke 拉的是顶层 mcp.RegisterTools 入口），
//     让 stub 跟真实现走同一函数名，最简单。

func notImplemented(name string) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return NewToolErrorf(ErrCategoryInternal, "not_implemented",
			"该 tool 在当前 mcp-server 版本尚未实现，请升级或反馈",
			"tool %s is not implemented yet", name), nil
	}
}

func RegisterGetReview(s *Server, _ app.ReviewService) {
	s.AddTool(getReviewTool(), notImplemented("get_review"))
}

func RegisterListReviews(s *Server, _ app.ReviewService) {
	s.AddTool(listReviewsTool(), notImplemented("list_reviews"))
}

func RegisterCancelReview(s *Server, _ app.ReviewService) {
	s.AddTool(cancelReviewTool(), notImplemented("cancel_review"))
}

func RegisterRerunReview(s *Server, _ app.ReviewService) {
	s.AddTool(rerunReviewTool(), notImplemented("rerun_review"))
}

func RegisterGetReviewFindings(s *Server, _ app.ReviewService) {
	s.AddTool(getReviewFindingsTool(), notImplemented("get_review_findings"))
}

func RegisterGetSuggestionPatch(s *Server, _ app.ReviewService) {
	s.AddTool(getSuggestionPatchTool(), notImplemented("get_suggestion_patch"))
}

func RegisterPreviewChangeFiles(s *Server, _ app.ReviewService) {
	s.AddTool(previewChangeFilesTool(), notImplemented("preview_change_files"))
}

func RegisterScanRepository(s *Server, _ app.ReviewService) {
	s.AddTool(scanRepositoryTool(), notImplemented("scan_repository"))
}

func RegisterListProviderRepos(s *Server, _ app.GitProviderManager) {
	s.AddTool(listProviderReposTool(), notImplemented("list_provider_repos"))
}

func RegisterListPullRequests(s *Server, _ app.GitProviderManager) {
	s.AddTool(listPullRequestsTool(), notImplemented("list_pull_requests"))
}

func RegisterGetBranchSHA(s *Server, _ app.GitProviderManager) {
	s.AddTool(getBranchSHATool(), notImplemented("get_branch_sha"))
}

func RegisterGetPullRequest(s *Server, _ app.GitProviderManager) {
	s.AddTool(getPullRequestTool(), notImplemented("get_pull_request"))
}
