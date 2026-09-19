package review

import (
	"time"

	"github.com/oklog/ulid/v2"
)

// NewReviewID 生成 review 唯一 ID。
// PR-14 修复：原实现用 32-bit crypto/rand + 日期前缀（"rv_20260630_xxx"），
// 同一天内的实际熵只有 32 bit，且无单调递增性。
// 改用 ULID（80 bit 熵 + 字典序 = 时间序），便于按 ID 排序做 range scan，
// 外部链接也更短。
func NewReviewID(_ time.Time) string {
	return ulid.Make().String()
}
