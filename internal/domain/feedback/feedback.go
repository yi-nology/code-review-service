// Package feedback 是 finding 反馈的领域类型。
//
// 这是 #6 架构迁移引入的领域层：仓储接口将以此为参数类型，
// postgres 实现在边界处做 domain ↔ model 映射。领域类型不携带任何
// GORM 标签或持久化细节（如自增主键），保持纯净。
package feedback

import "time"

// Feedback 表示对评审中某条 finding 的人工反馈（误报/接受风险/延期/已修复）。
type Feedback struct {
	ReviewID     string `json:"review_id"`
	FindingIndex int    `json:"finding_index"`
	// Fingerprint 是 finding 的跨 review 稳定指纹。
	// 用于支撑误报抑制：同一指纹若曾被标记 false_positive，后续评审可自动降级/过滤。
	// 向后兼容：历史反馈无指纹，此字段为空。
	Fingerprint string    `json:"fingerprint,omitempty"`
	Status      string    `json:"status"`
	Note        string    `json:"note,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
