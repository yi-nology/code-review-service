package review

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/oekit/logx"
)

// DefaultOCRBinary 是 OCRSettings.Binary 未配置时的兜底路径。
// 历史上此值在 review/executor 两处各自硬编码，现收敛为单一真源。
const DefaultOCRBinary = "/usr/local/bin/ocr"

// ResolveOCRBinary 延迟读取当前 OCR 设置中的二进制路径，未配置或仓库不可用时返回默认值。
// 接收请求 ctx，避免在请求路径上用 context.Background() 绕过取消/超时。
func ResolveOCRBinary(ctx context.Context, repo repository.OCRSettingsRepository) string {
	if repo == nil {
		return DefaultOCRBinary
	}
	if settings, err := repo.Get(ctx); err == nil && settings != nil && settings.Binary != "" {
		return settings.Binary
	}
	return DefaultOCRBinary
}

// AgentRulePackageRefs 汇总所有启用审查员的 RulePackageIDs（json.RawMessage 列表）。
// ResolveRuleFileForAgents 内部会基于 MergeRulePackageIDs 合并去重，这里只做收集不去重。
// 所有审查员共享同一份合并规则文件，因此取 RawMessage 列表交给 rules 层统一处理。
func AgentRulePackageRefs(ctx context.Context, repo repository.ReviewerAgentRepository) [][]byte {
	if repo == nil {
		return nil
	}
	agents, err := repo.ListEnabled(ctx)
	if err != nil {
		logx.Warn("failed to load enabled agents for rule package refs", "error", err)
		return nil
	}
	refs := make([][]byte, 0, len(agents))
	for i := range agents {
		if len(agents[i].RulePackageIDs) == 0 {
			continue
		}
		refs = append(refs, agents[i].RulePackageIDs)
	}
	return refs
}
