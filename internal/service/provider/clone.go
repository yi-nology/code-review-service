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

// validateGitBranchRef 校验 PR 作者可控的分支名可安全拼入 git fetch refspec。
//
// 分支名经 SDK 传给 `git fetch origin <refspec>`：前导 '-' 会被解析成 git 选项
// （option injection），':' 是 refspec 分隔符可注入额外 src:dst，'..' / 空白 /
// 控制字符可构造非预期 revision 表达式。保留 unicode 等其余合法分支字符，
// 避免误伤非 ASCII 分支名。空值放行（由调用方决定可选语义）。
func validateGitBranchRef(kind, name string) error {
	if name == "" {
		return nil
	}
	switch {
	case strings.HasPrefix(name, "-"),
		strings.ContainsAny(name, ":~^*?[\\\t\n\r\x00 "),
		strings.Contains(name, ".."),
		strings.Contains(name, "@{"):
		return fmt.Errorf("%s 含非法字符或序列, 拒绝用于 git fetch: %q", kind, name)
	}
	return nil
}

// validateGitSHA 校验来自平台 API 的 commit SHA 为十六进制串（6-64 位，兼容缩写与 SHA-256）。
// SHA 会进入 refspec（headSHA:localRef）与 RevParse/MergeBase 参数，非 hex 输入一律拒绝。
func validateGitSHA(kind, sha string) error {
	if sha == "" {
		return nil
	}
	for _, r := range sha {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
		if !isHex {
			return fmt.Errorf("%s 不是合法的十六进制 SHA: %q", kind, sha)
		}
	}
	if len(sha) < 6 || len(sha) > 64 {
		return fmt.Errorf("%s 长度 %d 超出 [6,64], 拒绝: %q", kind, len(sha), sha)
	}
	return nil
}

func cloneAndResolvePR(
	ctx context.Context,
	cloneURL, token string,
	skipTLS bool,
	prNumber int,
	platform, sourceBranch, targetBranch, apiHeadSHA, apiBaseSHA string,
) (string, string, string, error) {
	if err := validateGitBranchRef("sourceBranch", sourceBranch); err != nil {
		return "", "", "", err
	}
	if err := validateGitBranchRef("targetBranch", targetBranch); err != nil {
		return "", "", "", err
	}
	if err := validateGitSHA("headSHA", apiHeadSHA); err != nil {
		return "", "", "", err
	}
	if err := validateGitSHA("baseSHA", apiBaseSHA); err != nil {
		return "", "", "", err
	}

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
		// 获取 base 分支是后续 MergeBase / base SHA 解析的前提。失败时不中断
		//（有些场景 base 可由 SHA 直接解析），但此前被静默吞掉，
		// base 解析错误会产生对照错误 diff 基线的评审。记录告警便于诊断。
		if err := git.Fetch(ctx, targetBranch+":"+targetBranch); err != nil {
			logx.Warnf("fetch target branch %q failed, base SHA resolution may be inaccurate: %v", targetBranch, err)
		}
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
	if err := validateGitSHA("baseSHA", baseSHA); err != nil {
		return "", "", "", "", err
	}
	if err := validateGitSHA("headSHA", headSHA); err != nil {
		return "", "", "", "", err
	}

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
