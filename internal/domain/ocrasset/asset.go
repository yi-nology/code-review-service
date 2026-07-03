// Package ocrasset 是 OCR 规则包/工具包配置的领域类型（#6 迁移）。
// 规则包与工具包均为带版本管理的 OCR「资产」，统一放一个包。
package ocrasset

import (
	"encoding/json"
	"time"
)

// RulePackage 是 OCR 规则包（rule package）配置。
// 多个 JSONB 字段用 json.RawMessage 解耦 GORM datatypes.JSON。
type RulePackage struct {
	ID          uint            `json:"-"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Languages   json.RawMessage `json:"languages,omitempty"`
	Tags        json.RawMessage `json:"tags,omitempty"`
	Includes    json.RawMessage `json:"includes,omitempty"`
	Excludes    json.RawMessage `json:"excludes,omitempty"`
	Rules       json.RawMessage `json:"rules"`
	Enabled     bool            `json:"enabled"`
	ProjectRefs json.RawMessage `json:"project_refs,omitempty"`
	BuiltIn     bool            `json:"built_in"`
}

// RulePackageVersion 是规则包的版本快照。
type RulePackageVersion struct {
	ID        uint            `json:"id"`
	PackageID uint            `json:"package_id"`
	Version   int             `json:"version"`
	Snapshot  json.RawMessage `json:"snapshot"`
	CreatedAt time.Time       `json:"created_at"`
}

// ToolPackage 是 OCR 工具包配置。
type ToolPackage struct {
	ID          uint            `json:"-"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	ScanType    string          `json:"scan_type"`
	Config      json.RawMessage `json:"config"`
	Enabled     bool            `json:"enabled"`
	ProjectRefs json.RawMessage `json:"project_refs,omitempty"`
	BuiltIn     bool            `json:"built_in"`
}

// ToolPackageVersion 是工具包的版本快照。
type ToolPackageVersion struct {
	ID        uint            `json:"id"`
	PackageID uint            `json:"package_id"`
	Version   int             `json:"version"`
	Snapshot  json.RawMessage `json:"snapshot"`
	CreatedAt time.Time       `json:"created_at"`
}
