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
	"github.com/yi-nology/code-review-service/internal/domain/review"
)

// 重新导出 domain 类型，方便 internal/ocr 包内部使用。
type (
	OCROverrides = domain.OCROverrides
	Workspace    = domain.Workspace
)
