package review

import (
	"context"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/service/policy"
	"github.com/yi-nology/code-review-service/internal/vcs"
)

func applyProjectScope(ctx context.Context, workspace Workspace, ignorePaths []string) (Workspace, error) {
	git := &vcs.Git{Dir: workspace.Dir}
	changed, err := git.DiffNameOnly(ctx, workspace.BaseSHA, workspace.HeadSHA)
	if err != nil {
		return Workspace{}, fmt.Errorf("list changed files: %w", err)
	}
	included := make([]string, 0, len(changed))
	for _, path := range changed {
		if !policy.MatchesPath(ignorePaths, path) {
			included = append(included, path)
		}
	}
	workspace.Scope = ScopeMetadata{
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
		return Workspace{}, fmt.Errorf("prepare scoped review base: %w", err)
	}
	patch, err := git.Diff(ctx, workspace.BaseSHA, originalHead, included...)
	if err != nil {
		return Workspace{}, fmt.Errorf("build scoped review patch: %w", err)
	}
	if err := git.Apply(ctx, patch); err != nil {
		return Workspace{}, fmt.Errorf("apply scoped review patch: %w", err)
	}
	if err := git.CommitWithIdentity(ctx, "Review Service", "review-service@localhost", "review-service scoped diff"); err != nil {
		return Workspace{}, fmt.Errorf("commit scoped review patch: %w", err)
	}
	head, err := git.RevParse(ctx, "HEAD")
	if err != nil {
		return Workspace{}, err
	}
	workspace.HeadSHA = head
	workspace.Diff = string(patch)
	return workspace, nil
}
