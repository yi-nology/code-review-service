package finding

import (
	"github.com/oekit/pkg/stringx"
)

// GenerateID 生成基于时间戳的唯一 ID。
// 委托给 ekit 的 stringx.GenerateID。
func GenerateID() string {
	return stringx.GenerateID()
}

// ContainsAny 检查 text 中是否包含 keywords 中的任意一个关键词（不区分大小写）。
// 委托给 ekit 的 stringx.ContainsAny。
func ContainsAny(text string, keywords ...string) bool {
	return stringx.ContainsAny(text, keywords...)
}

// ResolvePRNumber 根据 provider 类型解析 PR/MR 编号，统一返回 string；
// 返回空串表示无法确定编号。GitLab 家族用 IID（mergeRequestIID），
// 其余平台用 PR 编号（pullRequestID）。
func ResolvePRNumber(provider string, pullRequestID, mergeRequestIID string) string {
	if provider == "gitlab" {
		return mergeRequestIID
	}
	return pullRequestID
}
