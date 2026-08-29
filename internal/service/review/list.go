package review

import (
	"encoding/json"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// 下列类型已下沉至 domain 包（见 domain/list_types.go），此处用 type alias 重导出，
// 保持 review 包既有调用方（service.go 内部、handler、mcp）零改动。
// 这样 internal/app 抽象层可只依赖 domain，不再反向依赖 service/review 实现层。
type (
	ListOptions = domain.ListRequest
	ListItem    = domain.ListEntry
	ListStats   = domain.ListSummary
	ListResult  = domain.ListResponse
)

type Detail struct {
	ReviewID       string           `json:"review_id"`
	Provider       string           `json:"provider"`
	Repo           string           `json:"repo"`
	ProjectID      string           `json:"project_id,omitempty"`
	PullRequestID  string           `json:"pull_request_id,omitempty"`
	MergeRequestID string           `json:"merge_request_iid,omitempty"`
	BaseSHA        string           `json:"base_sha"`
	HeadSHA        string           `json:"head_sha"`
	CallbackMode   string           `json:"callback_mode,omitempty"`
	Status         string           `json:"status"`
	Summary        string           `json:"summary"`
	Gate           domain.Gate      `json:"gate"`
	Analysis       domain.Analysis  `json:"analysis"`
	History        []domain.Event   `json:"history"`
	Findings       []domain.Finding `json:"findings"`
	Report         domain.Report    `json:"report"`
	Author         string           `json:"author,omitempty"`
	AuthorName     string           `json:"author_name,omitempty"`
	AuthorAvatar   string           `json:"author_avatar,omitempty"`
	RawOCR         json.RawMessage  `json:"raw_ocr,omitempty"`
	Error          string           `json:"error,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}
