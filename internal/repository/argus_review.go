package repository

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/argusingest"
)

// ArgusReviewRepository Argus 上报审查结果的持久化接口（postgres 实现见 postgres 包）。
type ArgusReviewRepository interface {
	// UpsertReview 事务内落库：任务按业务键 upsert（同 PR 同 SHA 重审覆盖），
	// 意见整组替换（先删后插，(review, fp) 唯一约束兜底）。返回任务 ID。
	UpsertReview(ctx context.Context, rev *argusingest.Review) (uint, error)
	// GetReview 任务 + 全部意见（意见按 file/line 排序）。
	GetReview(ctx context.Context, id uint) (*argusingest.Review, error)
	// ListReviews 分页列表（occurred_at 降序），返回页数据与总数。
	ListReviews(ctx context.Context, filter argusingest.ListFilter) ([]argusingest.Review, int64, error)
	// SetFindingSuppressState 标记意见的抑制回流状态（'' | 'suppressed'）。
	SetFindingSuppressState(ctx context.Context, reviewID uint, fp, state string) error
}
