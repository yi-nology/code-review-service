// Package webhook 共享类型与辅助函数。
//
// PR-18 修复：原 5 个平台 normalizer 各自重复实现：
//   - 解析 JSON 顶层 map[string]any
//   - 提取 sender / repo / SHA / PR/MR number
//   - 转换为 ChangeEvent
//
// 抽象出 NormalizedPayload 中间结构：让 normalizer 共享「解析 + 提取」框架，
// 平台特定字段通过钩子函数注入。
package webhook

import (
	"encoding/json"
	"strconv"
)

// NormalizedPayload 是 webhook 原始 payload 的解析中间结构。
// 5 个平台 normalizer 都先解析出 NormalizedPayload，再做平台特定字段映射。
type NormalizedPayload struct {
	Raw           map[string]any
	SenderLogin   string
	SenderName    string
	Action        string
	Repository    string
	RepositoryID  string
	Number        int
	BaseSHA       string
	HeadSHA       string
	StartSHA      string
	PullRequestURL string
	IsDraft       bool
}

// ParseNormalizedPayload 把 raw body 解析为 NormalizedPayload。
// 失败时返回 (zero, err)；调用方应判 err 后再尝试平台特定 parse。
func ParseNormalizedPayload(raw []byte) (NormalizedPayload, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return NormalizedPayload{}, err
	}
	return NormalizedPayload{Raw: m}, nil
}

// IntFromAny 把任意 JSON number 字符串或 float64 转换为 int。
// GitHub 的 PR number 通常是 float64（默认 JSON 解码），GitLab 可能是 string。
func IntFromAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	}
	return 0
}

// StringFromAny 把 any 转换为 string，安全处理 nil。
func StringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
