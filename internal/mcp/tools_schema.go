package mcp

import "github.com/mark3labs/mcp-go/mcp"

// tools_schema.go 集中所有 tool 的 schema 定义（NewTool + WithDescription + 输入参数）。
//
// 设计：
//   - 每个 tool 一个 XxxTool() 函数，返回 mcp.Tool。
//   - 真正的 handler 在同名文件（如 get_review.go），本文件只放 schema。
//   - 把 schema 集中放一个文件而不是分散在 handler 文件里，是因为：
//     1. 一眼能看完所有 tool 的输入字段（review / 维护时方便）
//     2. 后续要做 OpenAPI 转换 / 自动生成文档时，只用扫描这一个文件
//
// schema 描述遵循 "Description first"：description 字段既要给 LLM 看（决定是否调用），
// 也要给开发者看（IDE 提示）。所以写得详细一些，包含具体取值示例。

// ---- Tier 1 评审核心 ----

func getReviewTool() mcp.Tool {
	return mcp.NewTool("get_review",
		mcp.WithDescription("按 review_id 查询完整评审结果（gate/score/findings/report）。"),
		WithStringParam("review_id", true, "create_review 返回的 review ID（ULID 格式）"),
	)
}

func listReviewsTool() mcp.Tool {
	return mcp.NewTool("list_reviews",
		mcp.WithDescription("按条件列出历史评审记录。"),
		WithStringParam("provider", false, "按平台过滤 (github/gitlab/gitea/gitcode/tencent_code)"),
		WithStringParam("repo", false, "按 owner/repo 过滤"),
		WithStringParam("status", false, "按状态过滤 (queued/running/success/failed/cancelled)"),
		WithStringParam("gate_status", false, "按 gate 状态过滤 (pending/passed/failed/error)"),
		WithIntParam("page", false, "页码，从 1 开始，默认 1", 1),
		WithIntParam("page_size", false, "每页条数，默认 20，最大 100", 20),
	)
}

func cancelReviewTool() mcp.Tool {
	return mcp.NewTool("cancel_review",
		mcp.WithDescription("取消一个 queued 或 running 状态的评审。已完成的评审无法取消。"),
		WithStringParam("review_id", true, "要取消的 review ID"),
	)
}

func rerunReviewTool() mcp.Tool {
	return mcp.NewTool("rerun_review",
		mcp.WithDescription("重新跑一次评审（基于原始 request 字段，新生成 review_id 写入 DB）。"),
		WithStringParam("review_id", true, "原 review ID（用其 request 字段重建评审）"),
		WithStringParam("idempotency_key", false, "幂等键：同一目标的重复 rerun 传同值可去重"),
		WithBoolParam("wait", false, "是否同步等待", true),
		WithIntParam("timeout_seconds", false, "wait=true 超时（秒）", 180),
	)
}

func getReviewFindingsTool() mcp.Tool {
	return mcp.NewTool("get_review_findings",
		mcp.WithDescription("按条件裁剪 review 的 findings（按 severity / 路径过滤），结果 LLM 友好（不返大段 code context）。"),
		WithStringParam("review_id", true, "review ID"),
		WithStringParam("severity", false, "按严重级别过滤 (critical/major/minor/info)"),
		WithStringParam("category", false, "按 category 过滤（来自 finding 字段）"),
		WithStringParam("path_glob", false, "按文件路径 glob 过滤（例: internal/mcp/*.go）"),
		WithIntParam("limit", false, "返回条数上限，默认 30，最大 200", 30),
	)
}

func getSuggestionPatchTool() mcp.Tool {
	return mcp.NewTool("get_suggestion_patch",
		mcp.WithDescription("获取 review 建议的 unified diff 修复 patch（可直接 git apply）。"),
		WithStringParam("review_id", true, "review ID"),
		WithStringArrayParam("finding_indexes", false, "限定要拿 patch 的 finding 索引（不传返全部）"),
	)
}

func previewChangeFilesTool() mcp.Tool {
	opts := append([]mcp.ToolOption{
		mcp.WithDescription("预览一次评审将扫描的文件列表（秒级返回，clone + listChanged）。"),
	}, reviewRequestSchema()...)
	return mcp.NewTool("preview_change_files", opts...)
}

func scanRepositoryTool() mcp.Tool {
	return mcp.NewTool("scan_repository",
		mcp.WithDescription("对仓库做全量扫描评审（不基于 PR diff）。"),
		WithStringParam("provider", true, "git 平台标识"),
		WithStringParam("repo", true, "owner/repo"),
		WithStringParam("head_sha", true, "扫描 commit SHA（通常传分支 HEAD）"),
		WithStringArrayParam("paths", false, "限定扫描的路径前缀（白名单）"),
		WithStringArrayParam("exclude", false, "排除的路径 glob（黑名单）"),
		WithStringParam("background", false, "OCR --background 内容（项目背景/约定）"),
		WithBoolParam("preview", false, "只返文件列表，不跑 LLM（秒级）", false),
	)
}

// ---- Tier 2 平台辅助 ----

func listProviderReposTool() mcp.Tool {
	return mcp.NewTool("list_provider_repos",
		mcp.WithDescription("列出已配置 platform 下的仓库（onboarding 阶段用）。"),
		WithStringParam("provider", true, "平台标识 (github/gitlab/gitea/gitcode/tencent)"),
		WithIntParam("page", false, "页码", 1),
		WithIntParam("per_page", false, "每页条数", 30),
	)
}

func listPullRequestsTool() mcp.Tool {
	return mcp.NewTool("list_pull_requests",
		mcp.WithDescription("列出某 repo 的 PR/MR（按 state 过滤）。"),
		WithStringParam("provider", true, "平台标识"),
		WithStringParam("owner", true, "owner / namespace"),
		WithStringParam("repo", true, "repo name"),
		WithStringParam("state", false, "open / closed / merged / all（默认 open）"),
		WithIntParam("page", false, "页码", 1),
		WithIntParam("per_page", false, "每页条数", 20),
	)
}

func getBranchSHATool() mcp.Tool {
	return mcp.NewTool("get_branch_sha",
		mcp.WithDescription("查某分支的最新 commit SHA（拉 base/head 用）。"),
		WithStringParam("provider", true, "平台标识"),
		WithStringParam("owner", true, "owner / namespace"),
		WithStringParam("repo", true, "repo name"),
		WithStringParam("branch", true, "分支名（如 main / master / develop）"),
	)
}

func getPullRequestTool() mcp.Tool {
	return mcp.NewTool("get_pull_request",
		mcp.WithDescription("查单个 PR/MR 的详情（含 base/head SHA、author）。"),
		WithStringParam("provider", true, "平台标识"),
		WithStringParam("owner", true, "owner / namespace"),
		WithStringParam("repo", true, "repo name"),
		WithIntParam("number", true, "PR/MR 编号"),
	)
}
