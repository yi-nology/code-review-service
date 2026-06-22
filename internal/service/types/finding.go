package types

import "strings"

type Finding struct {
	ID         string   `json:"id"`
	Severity   string   `json:"severity"`
	Category   string   `json:"category"`
	Path       string   `json:"path"`
	Line       int      `json:"line"`
	EndLine    int      `json:"end_line"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Suggestion string   `json:"suggestion"`
	Confidence float64  `json:"confidence"`
	Agents     []string `json:"agents"`
	Thinking   string   `json:"thinking,omitempty"`
	// RedundancyStatus 标记同一 persona 多次运行是否都发现此问题（路径 A4）
	// "" 未参与冗余；"confirmed" 冗余确认；"single" 单次观察
	RedundancyStatus string `json:"redundancy_status,omitempty"`
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
