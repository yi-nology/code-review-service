package vcs

import (
	"fmt"
	"strings"
)

// ParseRepoPath 把 "owner/repo"（或含 .git 后缀、gerrit 单层 project）形式的
// 仓库路径拆为 owner 与 repo。此前在 internal/service/provider，因 writeback、
// statuscheck 等非 provider 消费方也需要，下沉到中立的 vcs 包。
func ParseRepoPath(repoPath string) (string, string, error) {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return "", "", fmt.Errorf("repo path is empty")
	}

	repoPath = strings.TrimSuffix(repoPath, ".git")
	repoPath = strings.TrimPrefix(repoPath, "/")

	parts := strings.Split(repoPath, "/")
	if len(parts) < 2 {
		// Gerrit 等平台使用单层 project 命名空间（如 "my-project"），
		// 此时 owner 为空，repo 为完整 project name。
		return "", repoPath, nil
	}

	owner := strings.Join(parts[:len(parts)-1], "/")
	repo := parts[len(parts)-1]
	return owner, repo, nil
}
