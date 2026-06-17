package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/vcs"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

func ApplyProjectScope(ctx context.Context, workspace review.Workspace, ignorePaths []string) (review.Workspace, error) {
	git, err := vcs.Open(workspace.Dir, "")
	if err != nil {
		return review.Workspace{}, fmt.Errorf("open repo: %w", err)
	}
	changed, err := git.DiffNameOnly(ctx, workspace.BaseSHA, workspace.HeadSHA)
	if err != nil {
		return review.Workspace{}, fmt.Errorf("list changed files: %w", err)
	}
	included := make([]string, 0, len(changed))
	for _, path := range changed {
		if !policy.MatchesPath(ignorePaths, path) {
			included = append(included, path)
		}
	}
	workspace.Scope = review.ScopeMetadata{
		ChangedFiles:  len(changed),
		ReviewedFiles: len(included),
		ExcludedFiles: len(changed) - len(included),
		IgnorePaths:   append([]string(nil), ignorePaths...),
	}
	if len(included) == len(changed) {
		return workspace, nil
	}
	if len(included) == 0 {
		workspace.HeadSHA = workspace.BaseSHA
		workspace.Diff = ""
		return workspace, nil
	}

	originalHead := workspace.HeadSHA
	if err := git.CheckoutDetached(ctx, workspace.BaseSHA); err != nil {
		return review.Workspace{}, fmt.Errorf("prepare scoped review base: %w", err)
	}
	if err := git.CheckoutFiles(ctx, originalHead, included); err != nil {
		return review.Workspace{}, fmt.Errorf("restore scoped review files: %w", err)
	}
	if err := git.CommitWithIdentity(ctx, "Review Service", "review-service@localhost", "review-service scoped diff"); err != nil {
		return review.Workspace{}, fmt.Errorf("commit scoped review patch: %w", err)
	}
	head, err := git.RevParse(ctx, "HEAD")
	if err != nil {
		return review.Workspace{}, err
	}
	patch, err := git.Diff(ctx, workspace.BaseSHA, head)
	if err != nil {
		return review.Workspace{}, fmt.Errorf("build scoped diff: %w", err)
	}
	workspace.HeadSHA = head
	workspace.Diff = string(patch)
	return workspace, nil
}

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

	if s.globalRulesRepo != nil {
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
