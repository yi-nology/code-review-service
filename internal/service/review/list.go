package review

import (
	"encoding/json"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

type ListOptions struct {
	Status     string
	GateStatus string
	Provider   string
	Repo       string
	Limit      int
	Offset     int
	SortBy     string
	SortOrder  string
}

type ListItem struct {
	ReviewID        string      `json:"review_id"`
	Provider        string      `json:"provider"`
	Repo            string      `json:"repo"`
	Status          string      `json:"status"`
	Summary         string      `json:"summary"`
	Gate            domain.Gate `json:"gate"`
	Findings        int         `json:"findings"`
	Score           int         `json:"score"`
	Grade           string      `json:"grade,omitempty"`
	ReportGenerated bool        `json:"report_generated"`
	RetryCount      int         `json:"retry_count,omitempty"`
	Author          string      `json:"author,omitempty"`
	AuthorName      string      `json:"author_name,omitempty"`
	AuthorAvatar    string      `json:"author_avatar,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

type ListStats struct {
	Total       int   `json:"total"`
	Running     int   `json:"running"`
	GateFailed  int   `json:"gate_failed"`
	Findings    int   `json:"findings"`
	TotalTokens int64 `json:"total_tokens"`
}

type ListResult struct {
	Items      []ListItem `json:"items"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
	Stats      ListStats  `json:"stats"`
}

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
