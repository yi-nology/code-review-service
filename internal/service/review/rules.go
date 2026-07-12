package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/yi-nology/code-review-service/internal/domain/ocrasset"
	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"
	"github.com/yi-nology/code-review-service/internal/repository"

	"github.com/oekit/logx"
)

type ruleEntry struct {
	Path string `json:"path"`
	Rule string `json:"rule"`
}

type ruleFile struct {
	Rules []ruleEntry `json:"rules"`
}

// ResolveRuleFile 合并项目规则、引用的规则包、全局规则集（scope=global）与全局规则（legacy 兜底），
// 写入临时文件并返回路径与清理函数。无规则时返回 ("", nil, nil)。
//
// 合并来源（顺序）：
//  1. 项目内联规则（projectPolicy.Rules）
//  2. 项目引用的规则包（projectPolicy.RulePackageIDs → GetByID）
//  3. 审查员引用的规则包（agentRefs 并集 → GetByID）
//  4. 全局规则集（scope=global 的包，始终合并）
//  5. scope=project 且 project_refs 匹配当前项目的包（legacy 自动匹配）
//  6. 旧 global_rules 表（迁移过渡期兜底，迁移后通常为空）
//
// 作为 review 与 executor 路径的单一实现（含 rule packages），消除重复，
// 并修复此前 executor 路径丢失 rule packages 处理的问题。
func ResolveRuleFile(ctx context.Context, projectPolicy projectpolicy.ProjectPolicy, rulePackageRepo repository.OCRRulePackageRepository, globalRulesRepo repository.GlobalRulesRepository) (string, func(), error) {
	return ResolveRuleFileForAgents(ctx, projectPolicy, rulePackageRepo, globalRulesRepo)
}

// ResolveRuleFileForAgents 在 ResolveRuleFile 基础上，把审查员引用的规则包一并合并。
// agentRefs 为本次选中的每个审查员自己的 RulePackageIDs（json.RawMessage），函数内做合并去重。
// 这样所有审查员共享同一份合并后的规则文件（单一数据源，且修复了 workflow 路径丢失规则的 bug）。
func ResolveRuleFileForAgents(ctx context.Context, projectPolicy projectpolicy.ProjectPolicy, rulePackageRepo repository.OCRRulePackageRepository, globalRulesRepo repository.GlobalRulesRepository, agentRefs ...[]byte) (string, func(), error) {
	var merged []ruleEntry

	// 1. 项目内联规则
	if len(projectPolicy.Rules) > 0 {
		var projectRules []ruleEntry
		if err := json.Unmarshal(projectPolicy.Rules, &projectRules); err != nil {
			logx.Warn("failed to decode project rules", "error", err)
		} else {
			merged = append(merged, projectRules...)
		}
	}

	// 收集所有需要按 id 取的规则包：项目引用 + 审查员引用，去重。
	referenced := MergeRulePackageIDs(append([][]byte{projectPolicy.RulePackageIDs}, agentRefs...)...)

	if rulePackageRepo != nil {
		// 2-4. 按 id 取引用的包 + 全局 scope 包；同时遍历全部包处理 project_refs 自动匹配。
		if err := resolvePackageRules(ctx, rulePackageRepo, referenced, projectPolicy.Provider, projectPolicy.Repo, &merged); err != nil {
			// resolvePackageRules 内部对单包失败只 warn，这里仅记录整体错误。
			logx.Warn("resolve package rules completed with warnings", "error", err)
		}
	}

	// 6. 旧 global_rules 表兜底（迁移过渡期）
	if globalRulesRepo != nil {
		globalRules, err := globalRulesRepo.Get(ctx)
		if err != nil {
			logx.Warn("failed to load global rules", "error", err)
		} else if globalRules != nil && len(globalRules.Content) > 0 {
			var globalEntries []ruleEntry
			if err := json.Unmarshal(globalRules.Content, &globalEntries); err != nil {
				logx.Warn("failed to decode global rules", "error", err)
			} else {
				merged = append(merged, globalEntries...)
			}
		}
	}

	if len(merged) == 0 {
		return "", nil, nil
	}

	data, err := json.Marshal(ruleFile{Rules: merged})
	if err != nil {
		return "", nil, fmt.Errorf("marshal rules: %w", err)
	}

	f, err := os.CreateTemp("", "review-rules-*.json")
	if err != nil {
		return "", nil, fmt.Errorf("create temp rule file: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("write rule file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("close rule file: %w", err)
	}

	cleanup := func() { _ = os.Remove(f.Name()) }
	return f.Name(), cleanup, nil
}

// ParseRulePackageIDs 解析一个 json.RawMessage 形式的规则包 id 列表（含 float64 兼容），去重并过滤 0。
// 空输入或解析失败返回 nil。
func ParseRulePackageIDs(raw []byte) []uint {
	ids := decodeUintIDs(raw)
	if len(ids) == 0 {
		return nil
	}
	return dedupUintIDs(ids)
}

// MergeRulePackageIDs 把多个 json.RawMessage 形式的规则包 id 列表合并去重。
// 用于「项目引用 + 多个 agent 引用」一次取并集的场景（如 ResolveRuleFileForAgents）。
// 全部为空返回 nil。
func MergeRulePackageIDs(raws ...[]byte) []uint {
	if len(raws) == 0 {
		return nil
	}
	seen := make(map[uint]struct{})
	var out []uint
	for _, raw := range raws {
		for _, id := range ParseRulePackageIDs(raw) {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

// decodeUintIDs 解析 raw 为 []uint，float64 兜底（历史数据中 id 偶尔被序列化成 JSON number）。
// 返回 nil 表示解析失败或输入为空。
func decodeUintIDs(raw []byte) []uint {
	if len(raw) == 0 {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal(raw, &ids); err == nil {
		return ids
	}
	var fids []float64
	if err := json.Unmarshal(raw, &fids); err != nil {
		return nil
	}
	ids = make([]uint, 0, len(fids))
	for _, fid := range fids {
		ids = append(ids, uint(fid))
	}
	return ids
}

// dedupUintIDs 去重并过滤 0（保留顺序）。空输入返回 nil。
func dedupUintIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// resolvePackageRules 合并规则包规则到 merged：
//   - 按 id 取的引用包（项目 + 审查员）→ 直接合并（不论 scope，引用即生效）
//   - scope=global 的包 → 始终合并
//   - scope=project 且 project_refs 匹配当前项目的包 → 合并（legacy 自动匹配）
//
// 单个包加载失败只 warn 不中断；已通过引用取过的包不重复合并。
func resolvePackageRules(ctx context.Context, repo repository.OCRRulePackageRepository, referencedIDs []uint, provider, repoName string, merged *[]ruleEntry) error {
	// 先按 id 取引用的包（优先级高，标记为已处理避免 List 时重复）。
	handled := make(map[uint]struct{})
	for _, id := range referencedIDs {
		pkg, err := repo.GetByID(ctx, id)
		if err != nil {
			logx.Warn("failed to load referenced rule package", "id", id, "error", err)
			continue
		}
		if pkg == nil || !pkg.Enabled {
			continue
		}
		appendPackageRules(pkg.Rules, pkg.Name, merged)
		handled[id] = struct{}{}
	}

	// 再遍历全部包，处理 global scope 与 project_refs 自动匹配。
	packages, err := repo.List(ctx)
	if err != nil {
		return fmt.Errorf("load rule packages: %w", err)
	}
	for i := range packages {
		pkg := packages[i]
		if !pkg.Enabled {
			continue
		}
		if _, ok := handled[pkg.ID]; ok {
			continue // 引用取过的不再重复
		}
		scope := pkg.Scope
		if scope == "" {
			scope = ocrasset.ScopeProject
		}
		if scope == ocrasset.ScopeGlobal {
			// 全局规则集：始终合并。
			appendPackageRules(pkg.Rules, pkg.Name, merged)
			continue
		}
		// scope=project：仅 project_refs 匹配时合并（legacy 自动匹配行为）。
		if matchesProjectRefs(pkg.ProjectRefs, provider, repoName) {
			appendPackageRules(pkg.Rules, pkg.Name, merged)
		}
	}
	return nil
}

// appendPackageRules 把单个规则包的 rules（json.RawMessage，裸 [{path,rule}] 数组）
// 反序列化后追加到 merged。解码失败只 warn 不中断。
func appendPackageRules(rules []byte, name string, merged *[]ruleEntry) {
	if len(rules) == 0 {
		return
	}
	var entries []ruleEntry
	if err := json.Unmarshal(rules, &entries); err != nil {
		logx.Warn("failed to decode rule package", "name", name, "error", err)
		return
	}
	*merged = append(*merged, entries...)
}

func matchesProjectRefs(raw []byte, provider, repo string) bool {
	if len(raw) == 0 {
		return true
	}
	var refs []string
	if err := json.Unmarshal(raw, &refs); err != nil || len(refs) == 0 {
		return true
	}
	current := strings.Trim(strings.ToLower(provider+"/"+repo), "/")
	repoOnly := strings.Trim(strings.ToLower(repo), "/")
	for _, ref := range refs {
		ref = strings.Trim(strings.ToLower(ref), "/")
		if ref == "" || ref == current || ref == repoOnly {
			return true
		}
	}
	return false
}
