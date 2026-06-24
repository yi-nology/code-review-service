package agent

import (
	"os"
	"path/filepath"

	"github.com/oekit/pkg/slicex"
)

// 约定：技术栈标签使用 "stack:" 前缀；特殊标签 "always" 表示任何项目都参与。
const (
	tagAlways = "always"
	stackPrefix = "stack:"
)

// stackSentinels 把工作区根目录的哨兵文件映射到技术栈标签。
// 顺序遍历，命中即收集；多个哨兵可叠加（例如同时存在 package.json 和 *.vue）。
var stackSentinels = []struct {
	file  string
	stack string
}{
	{"go.mod", stackPrefix + "go"},
	{"go.sum", stackPrefix + "go"},
	{"package.json", stackPrefix + "node"},
	{"yarn.lock", stackPrefix + "node"},
	{"pnpm-lock.yaml", stackPrefix + "node"},
	{"package-lock.json", stackPrefix + "node"},
	{"requirements.txt", stackPrefix + "python"},
	{"pyproject.toml", stackPrefix + "python"},
	{"Pipfile", stackPrefix + "python"},
	{"setup.py", stackPrefix + "python"},
	{"pom.xml", stackPrefix + "java"},
	{"build.gradle", stackPrefix + "java"},
	{"build.gradle.kts", stackPrefix + "java"},
	{"Cargo.toml", stackPrefix + "rust"},
	{"composer.json", stackPrefix + "php"},
	{"Gemfile", stackPrefix + "ruby"},
	{"go.work", stackPrefix + "go"},
}

// frontendMarkers 用于在 Node 项目里进一步判定前端 stack（Vue/React 等组件代码）。
// 仅当已识别 stack:node 时追加 stack:frontend。
var frontendExtensions = []string{".vue", ".jsx", ".tsx", ".svelte"}

// ProbeStacks 扫描工作区根目录的哨兵文件，返回识别到的技术栈标签集合。
// 当 dir 为空或不存在时返回 nil（调用方据此走 always/default 兜底）。
// 纯函数、无副作用、易测。
func ProbeStacks(dir string) []string {
	if dir == "" {
		return nil
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	// 根目录直接命名的哨兵文件
	nameSet := make(map[string]bool, len(entries))
	for _, e := range entries {
		nameSet[e.Name()] = true
	}

	stacks := make([]string, 0, 4)
	seen := make(map[string]bool)
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			stacks = append(stacks, s)
		}
	}

	for _, sentinel := range stackSentinels {
		if nameSet[sentinel.file] {
			add(sentinel.stack)
		}
	}

	// Node 项目里若存在前端组件目录/文件，追加前端 stack
	if seen[stackPrefix+"node"] || hasFrontendSources(dir, entries) {
		if hasFrontendSources(dir, entries) {
			add(stackPrefix + "frontend")
		}
	}

	if len(stacks) == 0 {
		return nil
	}
	return stacks
}

// hasFrontendSources 在根目录直接子文件里查找前端组件扩展名。
// 避免递归全仓库（可能很大）；根目录的 src/ 常见前端入口也扫一层。
func hasFrontendSources(dir string, entries []os.DirEntry) bool {
	// 根目录直接子文件
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if slicex.Contains(frontendExtensions, filepath.Ext(e.Name())) {
			return true
		}
	}
	// 扫描常见前端入口目录一层
	for _, sub := range []string{"src", "app", "pages"} {
		subEntries, err := os.ReadDir(filepath.Join(dir, sub))
		if err != nil {
			continue
		}
		for _, e := range subEntries {
			if e.IsDir() {
				continue
			}
			if slicex.Contains(frontendExtensions, filepath.Ext(e.Name())) {
				return true
			}
		}
	}
	return false
}

// tagMatchesStacks 判断一组 tags 是否匹配给定 stacks 或 always 语义。
// 匹配规则：tags 含 "always" → 命中；tags 与 stacks 有任意交集 → 命中。
func tagMatchesStacks(tags, stacks []string) bool {
	for _, tag := range tags {
		if tag == tagAlways {
			return true
		}
		if slicex.Contains(stacks, tag) {
			return true
		}
	}
	return false
}
