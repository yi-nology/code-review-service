package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/yi-nology/code-review-service/internal/vcs"

	"github.com/oekit/logx"
	"github.com/oekit/pkg/convert"
)

func cloneAndResolvePR(
	ctx context.Context,
	cloneURL, token string,
	skipTLS bool,
	prNumber int,
	platform, sourceBranch, targetBranch, apiHeadSHA, apiBaseSHA string,
) (string, string, string, error) {
	dir, err := os.MkdirTemp("", "review-workspace-*")
	if err != nil {
		return "", "", "", fmt.Errorf("create temp dir: %w", err)
	}

	git, err := vcs.Clone(ctx, dir, cloneURL, token, skipTLS)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", "", "", err
	}
	defer func() { _ = git.Close() }()

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

	headRef := "pr-" + convert.IntToStr(prNumber)
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
			} else if p, pErr := git.RevParse(ctx, apiBaseSHA+"~1"); pErr == nil {
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

	headSHA = restoreDeletedFiles(ctx, git, baseSHA, headRef, headSHA)
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

func restoreDeletedFiles(ctx context.Context, git *vcs.Git, baseSHA, headRef, headSHA string) string {
	deletedFiles, delErr := git.DeletedFiles(ctx, baseSHA, headRef)
	if delErr != nil {
		logx.CtxWarnf(ctx, "failed to get deleted files: %v", delErr)
		return headSHA
	}
	if len(deletedFiles) == 0 {
		return headSHA
	}

	if err := git.CheckoutFiles(ctx, baseSHA, deletedFiles); err != nil {
		logx.CtxWarnf(ctx, "failed to checkout deleted files: %v", err)
		return headSHA
	}

	if err := git.CommitWithIdentity(ctx, "Review Service", "review-service@localhost", "restore deleted files for OCR context"); err != nil {
		logx.CtxWarnf(ctx, "failed to commit restored deleted files: %v", err)
		return headSHA
	}

	newHead, err := git.RevParse(ctx, "HEAD")
	if err != nil {
		logx.CtxWarnf(ctx, "failed to resolve new HEAD after restoring deleted files: %v", err)
		return headSHA
	}
	return newHead
}

func cloneAndResolveSHA(ctx context.Context, cloneURL, token string, skipTLS bool, baseSHA, headSHA string, maxDiffBytes int64) (string, string, string, string, error) {
	dir, err := os.MkdirTemp("", "review-workspace-*")
	if err != nil {
		return "", "", "", "", fmt.Errorf("create temp dir: %w", err)
	}

	git, err := vcs.Clone(ctx, dir, cloneURL, token, skipTLS)
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", "", "", "", err
	}
	defer func() { _ = git.Close() }()

	if err := git.FetchAll(ctx); err != nil {
		_ = os.RemoveAll(dir)
		return "", "", "", "", fmt.Errorf("git fetch: %w", err)
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

	headSHA = restoreDeletedFiles(ctx, git, baseSHA, headSHA, headSHA)
	return dir, baseSHA, headSHA, diff, nil
}
