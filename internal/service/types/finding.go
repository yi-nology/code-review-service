package types

import "strings"

type Finding struct {
	ID        string `json:"id"`
	Severity  string `json:"severity"`
	Category  string `json:"category"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	// ExistingCode/SuggestionCode 来自原始 OCR 输出的代码建议（review 路径）。
	// 聚合路径（agent/discussion）通常不填充。
	ExistingCode   string   `json:"existing_code,omitempty"`
	SuggestionCode string   `json:"suggestion_code,omitempty"`
	Suggestion     string   `json:"suggestion"`
	Confidence     float64  `json:"confidence"`
	Agents         []string `json:"agents"`
	Thinking       string   `json:"thinking,omitempty"`
	// RedundancyStatus 标记同一 persona 多次运行是否都发现此问题（路径 A4）
	// "" 未参与冗余；"confirmed" 冗余确认；"single" 单次观察
	RedundancyStatus string `json:"redundancy_status,omitempty"`
	// Fingerprint 是跨 review 稳定的问题指纹（path+category+title签名+行桶的哈希）。
	// 用于支撑「新增/存量/已修复」生命周期与误报抑制，与随机 ID 不同。
	// 在 findings 组装时由 ComputeFingerprints 统一填充。
	Fingerprint string `json:"fingerprint,omitempty"`
}

// 冗余状态常量（路径 A4）
const (
	RedundancyConfirmed = "confirmed"
	RedundancySingle    = "single"
)

func SeverityRank(severity string) int {
	switch strings.ToLower(severity) {
	case "critical":
		return 4
	case "major":
		return 3
	case "minor":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

// NormalizeSeverity 将严重级别字符串规范化为标准枚举值。
// 大小写/空白容错；空值、未知值兜底为 "major"。
// 作为 domain.NormalizeSeverity 与 workflow.normalizeSeverity 的单一实现，
// 消除两处重复（避免 workflow → review 反向依赖）。
func NormalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return "critical"
	case "minor":
		return "minor"
	case "info":
		return "info"
	default:
		return "major"
	}
}
