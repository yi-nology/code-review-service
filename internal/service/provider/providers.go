package provider

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/vcs"
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
	var targetBranch string
	var sourceBranch string
	var headSHAFromAPI string
	var baseSHAFromAPI string
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

	var dir, baseSHA, headSHA string
	if hasPR {
		dir, baseSHA, headSHA, err = cloneAndResolvePR(
			ctx, cloneURL, token, prNumber, a.platform, sourceBranch, targetBranch, headSHAFromAPI, baseSHAFromAPI,
		)
	} else {
		var shaDiff string
		dir, baseSHA, headSHA, shaDiff, err = cloneAndResolveSHA(ctx, cloneURL, token, req.BaseSHA, req.HeadSHA, a.maxDiffBytes)
		if err == nil && shaDiff != "" {
			diff = shaDiff
		}
	}
	if err != nil {
		return review.Workspace{}, fmt.Errorf("clone repository: %w", err)
	}

	return review.Workspace{
		Provider: a.platform,
		Repo:     strings.TrimPrefix(req.Repo, "/"),
		BaseSHA:  baseSHA,
		HeadSHA:  headSHA,
		Diff:     diff,
		Dir:      dir,
	}, nil
}

func cloneAndResolvePR(
	ctx context.Context,
	cloneURL, token string,
	prNumber int,
	platform, sourceBranch, targetBranch, apiHeadSHA, apiBaseSHA string,
) (string, string, string, error) {
	dir, err := os.MkdirTemp("", "review-workspace-*")
	if err != nil {
		return "", "", "", fmt.Errorf("create temp dir: %w", err)
	}

	git := &vcs.Git{Dir: dir, Token: token}

	if err := git.Clone(ctx, cloneURL); err != nil {
		_ = os.RemoveAll(dir)
		return "", "", "", fmt.Errorf("git clone: %w", err)
	}

	fetchErrors := make([]string, 0)
	fetched := false
	for _, ref := range pullRequestFetchRefs(platform, prNumber, sourceBranch, apiHeadSHA) {
		if err := git.Fetch(ctx, ref); err == nil {
			fetched = true
			break
		} else {
			fetchErrors = append(fetchErrors, err.Error())
		}
	}
	if !fetched {
		_ = os.RemoveAll(dir)
		return "", "", "", fmt.Errorf("git fetch PR: %s", strings.Join(fetchErrors, " / "))
	}

	if targetBranch != "" {
		_ = git.Fetch(ctx, targetBranch+":"+targetBranch)
	}

	headRef := "pr-" + strconv.Itoa(prNumber)
	headSHA, err := git.RevParse(ctx, headRef)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", "", "", fmt.Errorf("resolve head SHA: %w", err)
	}

	var baseSHA string
	if apiBaseSHA != "" {
		if _, err := git.RevParse(ctx, apiBaseSHA); err == nil {
			mb, mbErr := git.MergeBase(ctx, apiBaseSHA, headRef)
			if mbErr == nil && mb != "" && mb != headSHA {
				baseSHA = mb
			} else if p, pErr := git.RevParse(ctx, apiBaseSHA+"^1"); pErr == nil {
				mb2, _ := git.MergeBase(ctx, p, headRef)
				if mb2 != "" && mb2 != headSHA {
					baseSHA = mb2
				}
			}
		}
	}
	if baseSHA == "" {
		baseRef := "HEAD"
		if targetBranch != "" {
			baseRef = targetBranch
		}
		mb, mbErr := git.MergeBase(ctx, baseRef, headRef)
		if mbErr == nil && mb != "" && mb != headSHA {
			baseSHA = mb
		} else {
			baseSHA, err = git.RevParse(ctx, baseRef)
			if err != nil {
				_ = os.RemoveAll(dir)
				return "", "", "", fmt.Errorf("resolve base SHA: %w", err)
			}
		}
	}

	if err := git.Checkout(ctx, headRef); err != nil {
		if err := git.Checkout(ctx, baseSHA); err != nil {
			_ = os.RemoveAll(dir)
			return "", "", "", fmt.Errorf("checkout: %w", err)
		}
		return dir, baseSHA, headSHA, nil
	}

	deletedFiles, delErr := git.DeletedFiles(ctx, baseSHA, headRef)
	if delErr == nil && len(deletedFiles) > 0 {
		_ = git.CheckoutFiles(ctx, baseSHA, deletedFiles)
	}

	return dir, baseSHA, headSHA, nil
}

func pullRequestFetchRefs(platform string, prNumber int, sourceBranch, headSHA string) []string {
	localRef := fmt.Sprintf("refs/heads/pr-%d", prNumber)
	refs := make([]string, 0, 4)

	if platform == "gitcode" && sourceBranch != "" {
		refs = append(refs, sourceBranch+":"+localRef)
	}

	switch platform {
	case "gitlab":
		refs = append(refs, fmt.Sprintf("merge-requests/%d/head:%s", prNumber, localRef))
	default:
		refs = append(refs,
			fmt.Sprintf("pull/%d/head:%s", prNumber, localRef),
			fmt.Sprintf("refs/pull/%d/head:%s", prNumber, localRef),
		)
	}

	if sourceBranch != "" && platform != "gitcode" {
		refs = append(refs, sourceBranch+":"+localRef)
	}
	if headSHA != "" {
		refs = append(refs, headSHA+":"+localRef)
	}
	return refs
}

func cloneAndResolveSHA(ctx context.Context, cloneURL, token, baseSHA, headSHA string, maxDiffBytes int64) (string, string, string, string, error) {
	dir, err := os.MkdirTemp("", "review-workspace-*")
	if err != nil {
		return "", "", "", "", fmt.Errorf("create temp dir: %w", err)
	}

	git := &vcs.Git{Dir: dir, Token: token}

	if err := git.Clone(ctx, cloneURL); err != nil {
		_ = os.RemoveAll(dir)
		return "", "", "", "", fmt.Errorf("git clone: %w", err)
	}

	for _, sha := range []string{baseSHA, headSHA} {
		if err := git.Fetch(ctx, sha); err != nil {
			_ = os.RemoveAll(dir)
			return "", "", "", "", fmt.Errorf("git fetch SHA %s: %w", sha, err)
		}
	}

	if err := git.Checkout(ctx, headSHA); err != nil {
		_ = os.RemoveAll(dir)
		return "", "", "", "", fmt.Errorf("git checkout head: %w", err)
	}

	diffBytes, diffErr := git.Diff(ctx, baseSHA, headSHA)
	var diff string
	if diffErr == nil {
		diff = string(diffBytes)
	}

	if maxDiffBytes > 0 && int64(len(diff)) > maxDiffBytes {
		_ = os.RemoveAll(dir)
		return "", "", "", "", fmt.Errorf("diff size %d bytes exceeds limit of %d bytes", len(diff), maxDiffBytes)
	}

	deletedFiles, delErr := git.DeletedFiles(ctx, baseSHA, headSHA)
	if delErr == nil && len(deletedFiles) > 0 {
		_ = git.CheckoutFiles(ctx, baseSHA, deletedFiles)
	}

	return dir, baseSHA, headSHA, diff, nil
}
