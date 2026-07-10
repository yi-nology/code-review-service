// Package notification 定义评审事件通知的领域类型。
//
// 通知在评审完成/失败/门禁未通过时触发，推送到配置的渠道
// （站内消息、飞书/企业微信 webhook）。渠道实现见 internal/service/notification。
package notification

import "time"

// Event 表示一次可触发通知的评审事件。
// 由 executor 在评审结束时构造，传给 Notifier。
type Event struct {
	ReviewID string `json:"review_id"`
	Provider string `json:"provider"`
	Repo     string `json:"repo"`
	// Ref 标识评审对象（PR/MR 标题或编号描述），用于通知文案。
	Ref string `json:"ref,omitempty"`
	// Kind 标识事件类型：completed / failed / gate_failed。
	Kind       Kind   `json:"kind"`
	GateStatus string `json:"gate_status,omitempty"`
	Score      int    `json:"score,omitempty"`
	Grade      string `json:"grade,omitempty"`
	Findings   int    `json:"findings,omitempty"`
	// Stage 仅 Kind==Failed 时有意义，标识失败环节（repository/ocr/workflow 等）。
	Stage string `json:"stage,omitempty"`
	Error string `json:"error,omitempty"`
	// Summary 是评审摘要（完成时），供通知正文引用。
	Summary string `json:"summary,omitempty"`
}

// Kind 标识通知事件类型。
type Kind string

const (
	// KindCompleted 评审成功完成。
	KindCompleted Kind = "completed"
	// KindFailed 评审失败（执行异常）。
	KindFailed Kind = "failed"
	// KindGateFailed 评审完成但门禁未通过。
	KindGateFailed Kind = "gate_failed"
)

// Item 是一条站内通知记录（持久化到 notifications 表）。
type Item struct {
	ID        uint      `json:"id"`
	ReviewID  string    `json:"review_id"`
	Kind      Kind      `json:"kind"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Read      bool      `json:"read"`
	Provider  string    `json:"provider,omitempty"`
	Repo      string    `json:"repo,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// WebhookChannelConfig 描述一个外部 webhook 通知渠道的配置（飞书/企微/钉钉等）。
type WebhookChannelConfig struct {
	// Type 渠道类型：feishu / wecom / dingtalk / generic。
	Type string `yaml:"type" json:"type"`
	// URL webhook 推送地址。
	URL string `yaml:"url" json:"url"`
	// Secret 签名密钥（飞书/钉钉签名校验用），可选。
	Secret string `yaml:"secret" json:"secret,omitempty"`
	// On 触发条件，空表示所有事件都推送。可选值：completed/failed/gate_failed。
	On []string `yaml:"on" json:"on,omitempty"`
}

// ListOptions 控制通知列表查询。定义在 domain 包避免 repository ↔ postgres 循环依赖。
type ListOptions struct {
	OnlyUnread bool
	Limit      int
	Offset     int
}
