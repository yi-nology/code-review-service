package suggestion

import (
	"path/filepath"
	"strings"
)

// 安全校验：suggestion patch 会自动生成代码修复并可能推送到远程仓库，
// 是高危写操作。这里集中实现路径逃逸防护、敏感路径过滤与分支保护。
// 参考 EXPANSION_ROADMAP「自动修复须防越权/路径逃逸」。

// normalizePatchPath 校验并归一化补丁中的文件路径，返回安全路径与是否通过。
//
// 安全要求（与 writeback.normalizePath 语义对齐，独立实现避免反向依赖）：
//   - 拒绝空路径
//   - 拒绝绝对路径（/etc/passwd）
//   - 拒绝路径穿越（../ 或规整后仍以 .. 开头）
//   - 拒绝反斜杠路径分隔符（Windows 风格穿越）转斜杠后同样校验
//   - 去除 ./ 前缀等冗余
//
// 不通过的路径会导致对应 finding 被 BuildSuggestionPatch 跳过（防注入到补丁头）。
func normalizePatchPath(p string) (string, bool) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", false
	}
	p = filepath.ToSlash(p)
	cleaned := filepath.Clean(p)
	// Clean 后仍以 / 开头（绝对路径）或以 .. 开头（穿越），一律拒绝。
	if strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", false
	}
	// 原始输入若显式包含 .. 段（即使 Clean 能简化，也保守拒绝，防符号链接等绕过）。
	if containsTraversal(p) {
		return "", false
	}
	cleaned = strings.TrimPrefix(cleaned, "./")
	return cleaned, true
}

// containsTraversal 检测路径中是否含 ".." 路径段（前后为 / 或首尾）。
func containsTraversal(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// sensitivePathPrefixes 列出补丁不应自动修改的高风险路径前缀。
// 命中这些路径的 finding 会被跳过（不进入补丁），防止供应链攻击：
// 修改 CI 配置 / 构建脚本 / git hook / SSH 密钥等可被执行或提升权限的文件。
var sensitivePathPrefixes = []string{
	".github/workflows/", // GitHub Actions CI —— 可定义任意命令执行
	".gitlab-ci.yml",     // GitLab CI 配置
	".gitea/",            // Gitea CI/Action
	".forgejo/",          // Forgejo CI/Action
	"Dockerfile",         // 容器构建 —— 可植入恶意镜像层
	"Makefile",           // 构建脚本 —— 可能执行任意命令
	".git/hooks/",        // git hook —— 仓库级命令执行
	".ssh/",              // SSH 密钥/配置
	".env",               // 环境变量/密钥
	".npmrc",             // npm 凭证
	".pypirc",            // PyPI 凭证
	".cargo/credentials", // Cargo 凭证
}

// isSensitivePath 判断路径是否属于不应被自动补丁修改的高风险类别。
func isSensitivePath(p string) bool {
	lower := strings.ToLower(strings.TrimSpace(p))
	if lower == "" {
		return false
	}
	for _, prefix := range sensitivePathPrefixes {
		if strings.HasPrefix(lower, strings.ToLower(prefix)) {
			return true
		}
	}
	return false
}

// protectedBranches 是自动修复补丁禁止推送/合入的主干分支黑名单。
// 防止自动生成的修复直接推到主干或合入主干（应走 review 流程）。
// 小写匹配，兼容各平台分支命名约定。
var protectedBranches = map[string]bool{
	"main":       true,
	"master":     true,
	"develop":    true,
	"dev":        true,
	"release":    true,
	"production": true,
	"prod":       true,
}

// IsProtectedBranch 判断分支名是否为受保护主干（禁止自动推送/合入）。
// 大小写不敏感。调用方应在 normalizeSuggestionPatchBranch 之后、推送之前调用。
func IsProtectedBranch(branch string) bool {
	return protectedBranches[strings.ToLower(strings.TrimSpace(branch))]
}
