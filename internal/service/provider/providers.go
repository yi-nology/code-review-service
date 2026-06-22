package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/review"
)

type GitProviderProviders struct {
	svc          *GitProviderService
	maxDiffBytes int64
}

func NewGitProviderProviders(svc *GitProviderService, maxDiffBytes int64) *GitProviderProviders {
	return &GitProviderProviders{svc: svc, maxDiffBytes: maxDiffBytes}
}

func (p *GitProviderProviders) Get(name string) (review.Provider, bool) {
	if !p.svc.HasProvider(name) {
		return nil, false
	}
	return &gitProviderAdapter{svc: p.svc, platform: name, maxDiffBytes: p.maxDiffBytes}, true
}

type gitProviderAdapter struct {
	svc          *GitProviderService
	platform     string
	maxDiffBytes int64
}

func (a *gitProviderAdapter) Prepare(ctx context.Context, req review.Request) (review.Workspace, error) {
	owner, repoName, err := ParseRepoPath(req.Repo)
	if err != nil {
		return review.Workspace{}, fmt.Errorf("invalid repo path %q: %w", req.Repo, err)
	}

	var diff string
	var prNumber int
	var targetBranch, sourceBranch, headSHAFromAPI, baseSHAFromAPI string
	hasPR := req.PullRequestID != "" || req.MergeRequestIID != ""

	if hasPR {
		if req.PullRequestID != "" {
			prNumber, err = strconv.Atoi(req.PullRequestID)
		} else {
			prNumber, err = strconv.Atoi(req.MergeRequestIID)
		}
		if err != nil || prNumber <= 0 {
			return review.Workspace{}, fmt.Errorf("invalid PR/MR number: %w", err)
		}

		diff, err = a.svc.GetCRDiffString(ctx, a.platform, owner, repoName, prNumber)
		if err != nil {
			return review.Workspace{}, fmt.Errorf("get diff via API: %w", err)
		}

		cr, crErr := a.svc.GetCR(ctx, a.platform, owner, repoName, prNumber)
		if crErr == nil && cr != nil {
			targetBranch = cr.TargetBranch
			sourceBranch = cr.SourceBranch
			headSHAFromAPI = cr.HeadSHA
			baseSHAFromAPI = cr.BaseSHA
		}
	}

	if a.maxDiffBytes > 0 && int64(len(diff)) > a.maxDiffBytes {
		return review.Workspace{}, fmt.Errorf("diff size %d bytes exceeds limit of %d bytes", len(diff), a.maxDiffBytes)
	}

	cloneURL := a.svc.BuildCloneURL(a.platform, owner, repoName)
	token := a.svc.GetToken(a.platform)
	skipTLS := a.svc.GetSkipTLS(a.platform)

	var dir, baseSHA, headSHA string
	if hasPR {
		dir, baseSHA, headSHA, err = cloneAndResolvePR(
			ctx, cloneURL, token, skipTLS, prNumber, a.platform, sourceBranch, targetBranch, headSHAFromAPI, baseSHAFromAPI,
		)
	} else {
		var shaDiff string
		dir, baseSHA, headSHA, shaDiff, err = cloneAndResolveSHA(ctx, cloneURL, token, skipTLS, req.BaseSHA, req.HeadSHA, a.maxDiffBytes)
		if err == nil && shaDiff != "" {
			diff = shaDiff
		}
	}
	if err != nil {
		return review.Workspace{}, fmt.Errorf("clone repository: %w", err)
	}

	return review.Workspace{
		Provider:        a.platform,
		Repo:            strings.TrimPrefix(req.Repo, "/"),
		BaseSHA:         baseSHA,
		HeadSHA:         headSHA,
		OriginalHeadSHA: req.HeadSHA,
		Diff:            diff,
		Dir:             dir,
	}, nil
}
