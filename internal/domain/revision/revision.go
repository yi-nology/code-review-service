// Package revision 是变更（PR/MR）修订追踪的领域类型（#6 迁移）。
package revision

import "time"

// Change 记录某次变更（PR/MR）的最新已评审 SHA 基线，用于增量评审。
// ChangeNumber 与平台一致使用 string；DB 列 change_number 仍为 int，
// 转换收敛在仓储 mapping 层。
type Change struct {
	Provider       string    `json:"provider"`
	InstallationID string    `json:"installation_id"`
	RepositoryID   string    `json:"repository_id"`
	Repository     string    `json:"repository"`
	ChangeNumber   string    `json:"change_number"`
	BaseSHA        string    `json:"base_sha"`
	StartSHA       string    `json:"start_sha"`
	HeadSHA        string    `json:"head_sha"`
	LatestReviewID string    `json:"latest_review_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}
