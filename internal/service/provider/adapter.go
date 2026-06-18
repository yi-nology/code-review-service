package provider

import "context"

type GitProviderAdapter struct {
	svc *GitProviderService
}

func NewGitProviderAdapter(svc *GitProviderService) *GitProviderAdapter {
	return &GitProviderAdapter{svc: svc}
}

func (a *GitProviderAdapter) GetCRDiff(ctx context.Context, platform, owner, repo string, number int) (string, error) {
	return a.svc.GetCRDiffString(ctx, platform, owner, repo, number)
}

func (a *GitProviderAdapter) CreateComment(ctx context.Context, platform, owner, repo string, number int, body string) (string, error) {
	return a.svc.CreateComment(ctx, platform, owner, repo, number, body)
}

func (a *GitProviderAdapter) HasProvider(platform string) bool {
	return a.svc.HasProvider(platform)
}
