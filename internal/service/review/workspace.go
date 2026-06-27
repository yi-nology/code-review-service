package review

import (
	"context"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/vcs"
)

// ApplyProjectScope 按 ignorePaths 收窄工作区范围，生成 scope 元数据与裁剪后的 diff。
// 作为 review 与 executor 路径的单一实现，消除重复。
func ApplyProjectScope(ctx context.Context, workspace domain.Workspace, ignorePaths []string) (domain.Workspace, error) {
	git, err := vcs.Open(workspace.Dir, "")
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("open repo: %w", err)
	}
	changed, err := git.DiffNameOnly(ctx, workspace.BaseSHA, workspace.HeadSHA)
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("list changed files: %w", err)
	}
	included := make([]string, 0, len(changed))
	for _, path := range changed {
		if !policy.MatchesPath(ignorePaths, path) {
			included = append(included, path)
		}
	}
	workspace.Scope = domain.ScopeMetadata{
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
		return domain.Workspace{}, fmt.Errorf("prepare scoped review base: %w", err)
	}
	if err := git.CheckoutFiles(ctx, originalHead, included); err != nil {
		return domain.Workspace{}, fmt.Errorf("restore scoped review files: %w", err)
	}
	if err := git.CommitWithIdentity(ctx, "Review Service", "review-service@localhost", "review-service scoped diff"); err != nil {
		return domain.Workspace{}, fmt.Errorf("commit scoped review patch: %w", err)
	}
	head, err := git.RevParse(ctx, "HEAD")
	if err != nil {
		return domain.Workspace{}, err
	}
	patch, err := git.Diff(ctx, workspace.BaseSHA, head)
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("build scoped diff: %w", err)
	}
	workspace.HeadSHA = head
	workspace.Diff = string(patch)
	return workspace, nil
}
