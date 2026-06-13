package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"review-service/internal/repository/postgres/models"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type ruleEntry struct {
	Path string `json:"path"`
	Rule string `json:"rule"`
}

type ruleFile struct {
	Rules []ruleEntry `json:"rules"`
}

func (s *Service) resolveRuleFile(ctx context.Context, projectPolicy models.ProjectPolicy) (string, func(), error) {
	var merged []ruleEntry

	if len(projectPolicy.Rules) > 0 {
		var projectRules []ruleEntry
		if err := json.Unmarshal(projectPolicy.Rules, &projectRules); err != nil {
			hlog.Warn("failed to decode project rules", "error", err)
		} else {
			merged = append(merged, projectRules...)
		}
	}

	globalRules, err := s.globalRulesRepo.Get(ctx)
	if err != nil {
		hlog.Warn("failed to load global rules", "error", err)
	} else if globalRules != nil && len(globalRules.Rules) > 0 {
		var globalEntries []ruleEntry
		if err := json.Unmarshal(globalRules.Rules, &globalEntries); err != nil {
			hlog.Warn("failed to decode global rules", "error", err)
		} else {
			merged = append(merged, globalEntries...)
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
