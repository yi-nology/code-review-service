// Package ocr 是「open-code-review 能力」的核心包。
//
// 本包是 OCR 能力的单一入口：runner、scanner、preview、7 个 manager
// （Versioner / Lister / Checker / Viewer / Upgrader / SessionLister / Tester）、
// LLM provider 与配置合并都在这里。原先散落在 internal/service/{ocr,runner,llm}
// 的实现已全部迁入，重复的 settingsSource 接口、exec 封装与 config 合并路径
// 随之收敛为单例：BuildConfig 是配置合并的唯一实现，internal/ocr/internal
// 的 Run 是统一的 exec 入口。
//
// 能力面通过 Registry 聚合（见 registry.go），上层按需取用而非直接构造。
//
// 设计背景与演进过程见 docs/OCR_CORE_DESIGN.md。
package ocr

import (
	"github.com/yi-nology/code-review-service/internal/domain/review"
)

// 重新导出 domain 类型，方便 internal/ocr 包内部使用。
type (
	OCROverrides = domain.OCROverrides
	Workspace    = domain.Workspace
)
