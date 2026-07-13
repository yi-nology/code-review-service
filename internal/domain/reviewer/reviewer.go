// Package reviewer 是审查员（reviewer agent）配置的领域类型（#6 迁移）。
package reviewer

import "encoding/json"

// Agent 表示一个审查员的定义（persona、规则、优先级等）。
// Tags 为 JSONB，领域层用 json.RawMessage 解耦 GORM datatypes.JSON。
// ID 为持久化主键（update/delete-by-id 需要），领域层显式携带以避免往返丢失。
type Agent struct {
	ID          uint            `json:"-"`
	Name        string          `json:"name"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description"`
	Background  string          `json:"background"`
	RuleFile    string          `json:"rule_file,omitempty"`
	// RulePackageIDs 引用的规则包 id 列表（常驻审查员的规则来源）。
	RulePackageIDs json.RawMessage `json:"rule_package_ids,omitempty"`
	Redundancy  int             `json:"redundancy"`
	Enabled     bool            `json:"enabled"`
	Priority    int             `json:"priority"`
	IsDefault   bool            `json:"is_default"`
	Tags        json.RawMessage `json:"tags,omitempty"`
}

// ProjectAssignment 表示某项目启用的审查员关联（含 join 出来的 agent 元信息）。
type ProjectAssignment struct {
	ID          uint   `json:"-"`
	Provider    string `json:"provider"`
	Repo        string `json:"repo"`
	AgentID     uint   `json:"agent_id"`
	Enabled     bool   `json:"enabled"`
	Priority    int    `json:"priority"`
	AgentName   string `json:"agent_name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Background  string `json:"background,omitempty"`
}

// ProjectConfig 表示某项目的完整审查员配置。
type ProjectConfig struct {
	Provider string              `json:"provider"`
	Repo     string              `json:"repo"`
	Agents   []ProjectAssignment `json:"agents"`
}
