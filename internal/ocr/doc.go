// Package ocr 是「open-code-review 能力」的核心包。
//
// 阶段 A 设计：把 internal/service/ocr + internal/service/runner + internal/service/llm
// 三个包的内容全部迁入本包，消除 5 个等价 settingsSource 接口、4 处 exec 重复、
// 7 个 manager 重复 read settings。
//
// 阶段 A 完成后，internal/service/{ocr,runner,llm} 三个包标记为 deprecated，
// 保留 type alias 兼容 3 个月，consumer 逐步迁移。
package ocr

import (
	"github.com/oklog/ulid/v2"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// newID 生成 review 唯一 ID（ULID 格式，26 字符 Base32，80 bit 熵）。
//
// 阶段 A 完整化后此函数应改为接受 time.Time 参数，由调用方传入；
// 当前为了简化直接使用 ulid.Make()。
func newID() string {
	return ulid.Make().String()
}

// 重新导出 domain 类型，方便 internal/ocr 包内部使用。
type (
	OCROverrides = domain.OCROverrides
	Workspace    = domain.Workspace
)
