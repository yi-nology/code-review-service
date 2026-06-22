package workflow

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// projectConventionFiles 是会被自动发现并注入评审上下文的项目约定文件（阶段 5）。
// 对齐 OCR-spencermarx 的 Context Discovery 能力。
var projectConventionFiles = []string{
	"AGENTS.md",
	"CLAUDE.md",
	".cursorrules",
	"CONTRIBUTING.md",
	"CONVENTION.md",
	".opencodereview/rule.json",
}

// discoverProjectStandards 扫描 repoDir 下的项目约定文件，返回 文件名→内容 的映射。
// 每个文件最多读 64KB，避免大文件拖累。读取失败或不存在则静默跳过。
func discoverProjectStandards(repoDir string) map[string]string {
	if strings.TrimSpace(repoDir) == "" {
		return nil
	}
	if _, err := os.Stat(repoDir); err != nil {
		return nil
	}

	standards := make(map[string]string)
	for _, name := range projectConventionFiles {
		path := filepath.Join(repoDir, name)
		content, err := readCapped(path, 64*1024)
		if err != nil || strings.TrimSpace(content) == "" {
			continue
		}
		standards[name] = content
		hlog.Infof("discovered project standard: %s (%d chars)", name, len(content))
	}
	if len(standards) == 0 {
		return nil
	}
	return standards
}

// readCapped 读取文件前 maxBytes 字节。
func readCapped(path string, maxBytes int64) (string, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", os.ErrNotExist
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if int64(len(data)) > maxBytes {
		data = data[:maxBytes]
	}
	return string(data), nil
}

// extractChangedFiles 从 unified diff 中提取变更文件路径（A3：为 Tech Lead 提供范围感知）。
// 只用 "+++ b/path" 行确定路径，"+++ /dev/null"（删除文件）被自然排除。
func extractChangedFiles(diff string) []string {
	var files []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+++ ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "+++"))
		path = strings.TrimSpace(strings.TrimPrefix(path, "b/"))
		path = strings.TrimSpace(path)
		if path == "" || path == "/dev/null" || seen[path] {
			continue
		}
		seen[path] = true
		files = append(files, path)
	}
	return files
}
