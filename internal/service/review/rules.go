package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/domain/projectpolicy"

	"github.com/oekit/logx"
)

type ruleEntry struct {
	Path string `json:"path"`
	Rule string `json:"rule"`
}

type ruleFile struct {
	Rules []ruleEntry `json:"rules"`
}

// ResolveRuleFile 合并项目规则、规则包（rule packages）与全局规则，
// 写入临时文件并返回路径与清理函数。无规则时返回 ("", nil, nil)。
//
// 作为 review 与 executor 路径的单一实现（含 rule packages），消除重复，
// 并修复此前 executor 路径丢失 rule packages 处理的问题。
func ResolveRuleFile(ctx context.Context, projectPolicy projectpolicy.ProjectPolicy, rulePackageRepo repository.OCRRulePackageRepository, globalRulesRepo repository.GlobalRulesRepository) (string, func(), error) {
	var merged []ruleEntry

	if len(projectPolicy.Rules) > 0 {
		var projectRules []ruleEntry
		if err := json.Unmarshal(projectPolicy.Rules, &projectRules); err != nil {
			logx.Warn("failed to decode project rules", "error", err)
		} else {
			merged = append(merged, projectRules...)
		}
	}

	if rulePackageRepo != nil {
		packages, err := rulePackageRepo.List(ctx)
		if err != nil {
			logx.Warn("failed to load rule packages", "error", err)
		} else {
			for i := range packages {
				pkg := packages[i]
				if !pkg.Enabled || !matchesProjectRefs(pkg.ProjectRefs, projectPolicy.Provider, projectPolicy.Repo) {
					continue
				}
				var packageRules []ruleEntry
				if err := json.Unmarshal(pkg.Rules, &packageRules); err != nil {
					logx.Warn("failed to decode rule package", "name", pkg.Name, "error", err)
					continue
				}
				merged = append(merged, packageRules...)
			}
		}
	}

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
