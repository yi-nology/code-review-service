package domain

import (
	"time"
)

// 本文件承载 review service 的"列表查询"API 契约类型。
// 它们历史上定义在 internal/service/review（实现层），导致 internal/app（抽象层）
// 反向依赖实现层包。现下沉至 domain（纯类型包），由 review 包用 type alias 重导出。
//
// 命名上与同包的 ListOptions（repository 分页查询条件，Page/PageSize 风格）区分：
// 本组的 ListRequest 是 service API 入参（Limit/Offset 风格），语义不同，不可混用。

// ListRequest 是 review 列表查询的 API 入参（Limit/Offset 分页风格）。
// 注意：与 repository 层的 ListOptions（Page/PageSize 风格）是不同类型，
// service.List 内部会做转换。
type ListRequest struct {
	Status     string
	GateStatus string
	Provider   string
	Repo       string
	// Engine 按生效审查引擎过滤：''（不过滤）| EngineOCR | EngineArgus。
	Engine    string
	Limit     int
	Offset    int
	SortBy    string
	SortOrder string
}

// ListEntry 是列表中的一条评审摘要。
type ListEntry struct {
	ReviewID        string `json:"review_id"`
	Provider        string `json:"provider"`
	Repo            string `json:"repo"`
	Status          string `json:"status"`
	Summary         string `json:"summary"`
	Gate            Gate   `json:"gate"`
	Findings        int    `json:"findings"`
	Score           int    `json:"score"`
	Grade           string `json:"grade,omitempty"`
	ReportGenerated bool   `json:"report_generated"`
	RetryCount      int    `json:"retry_count,omitempty"`
	Author          string `json:"author,omitempty"`
	AuthorName      string `json:"author_name,omitempty"`
	AuthorAvatar    string `json:"author_avatar,omitempty"`
	// Engine 生效审查引擎（历史记录无该字段时归一为 ocr）。
	Engine    string    `json:"engine,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListSummary 是列表结果的聚合统计。
type ListSummary struct {
	Total       int   `json:"total"`
	Running     int   `json:"running"`
	GateFailed  int   `json:"gate_failed"`
	Findings    int   `json:"findings"`
	TotalTokens int64 `json:"total_tokens"`
}

// ListResponse 是 review 列表查询的完整响应。
type ListResponse struct {
	Items      []ListEntry `json:"items"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
	Stats      ListSummary `json:"stats"`
}
