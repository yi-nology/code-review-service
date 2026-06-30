// Package llm 提供 LLM 补全抽象。
//
// 阶段 A10 状态：本包被阶段 A 拆分为两部分：
//   - 保留（核心）：Provider / CompleteRequest / CompleteResponse / Presets / Metrics / NoopProvider / JSONHelper
//   - 弃用（迁移到 internal/ocr）：DynamicProvider → ocr.LLMProvider / ocr.LLMProviderAdapter
//
// 阶段 A+10 完成：所有 consumer 改用 ocr.LLMProvider 后删除 openai.go / openai_retry_test.go。
// 当前 PR 保留 openai.go 以维持向后兼容，标注 Deprecated。
package llm

// 本文件作为阶段 A10 的标记：明确标注哪些类型迁移到 ocr 包，
// 哪些保留在本包。详细迁移列表见 docs/OCR_CORE_DESIGN.md 第 3.7 节。