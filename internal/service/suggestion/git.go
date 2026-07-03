package suggestion

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type suggestionPatchPushAuth struct {
	Token   string
	SkipTLS bool
}

func runGitApplyCheck(ctx context.Context, dir, patch string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("workspace dir is empty")
	}
	cmd := exec.CommandContext(ctx, "git", "apply", "--check", "--unidiff-zero", "--whitespace=nowarn", "-")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(patch)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(out.String()), fmt.Errorf("git apply --check failed")
	}
	return strings.TrimSpace(out.String()), nil
}

func applyPatchCommit(ctx context.Context, dir, branch, patch string, push bool, pushAuth suggestionPatchPushAuth) (string, string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", "", fmt.Errorf("workspace dir is empty")
	}
	var combined strings.Builder
	run := func(args ...string) (string, error) {
		out, err := runGitCommand(ctx, dir, nil, args...)
		if strings.TrimSpace(out) != "" {
			combined.WriteString(out)
			combined.WriteByte('\n')
		}
		return out, err
	}
	if _, err := run("checkout", "-B", branch); err != nil {
		return strings.TrimSpace(combined.String()), "", fmt.Errorf("create repair branch: %w", err)
	}
	if _, err := runGitCommand(ctx, dir, strings.NewReader(patch), "apply", "--unidiff-zero", "--whitespace=nowarn", "-"); err != nil {
		return strings.TrimSpace(combined.String()), "", fmt.Errorf("git apply failed: %w", err)
	}
	if _, err := run("add", "-A"); err != nil {
		return strings.TrimSpace(combined.String()), "", fmt.Errorf("git add failed: %w", err)
	}
	if _, err := run("diff", "--cached", "--quiet"); err == nil {
		return strings.TrimSpace(combined.String()), "", fmt.Errorf("patch produced no staged changes")
	}
	if _, err := run("-c", "user.name=Review Service", "-c", "user.email=review-service@localhost", "commit", "-m", "Apply review suggestions"); err != nil {
		return strings.TrimSpace(combined.String()), "", fmt.Errorf("git commit failed: %w", err)
	}
	commitSHA, err := run("rev-parse", "HEAD")
	if err != nil {
		return strings.TrimSpace(combined.String()), "", fmt.Errorf("resolve repair commit: %w", err)
	}
	commitSHA = strings.TrimSpace(commitSHA)
	if push {
		args := []string{}
		if pushAuth.SkipTLS {
			args = append(args, "-c", "http.sslVerify=false")
		}
		if strings.TrimSpace(pushAuth.Token) != "" {
			cred := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + pushAuth.Token))
			args = append(args, "-c", "http.extraHeader=Authorization: Basic "+cred)
		}
		args = append(args, "push", "-u", "origin", branch)
		if _, err := run(args...); err != nil {
			return strings.TrimSpace(combined.String()), commitSHA, fmt.Errorf("git push failed: %w", err)
		}
	}
	return strings.TrimSpace(combined.String()), commitSHA, nil
}

func runGitCommand(ctx context.Context, dir string, stdin *strings.Reader, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(out.String()), err
	}
	return strings.TrimSpace(out.String()), nil
}

var invalidBranchChars = regexp.MustCompile(`[^A-Za-z0-9._/-]+`)

func normalizeSuggestionPatchBranch(raw, reviewID string) string {
	branch := strings.TrimSpace(raw)
	if branch == "" {
		id := strings.TrimSpace(reviewID)
		if len(id) > 12 {
			id = id[:12]
		}
		branch = "review-service/fix-" + id
	}
	branch = strings.Trim(branch, "/.")
	branch = invalidBranchChars.ReplaceAllString(branch, "-")
	branch = strings.ReplaceAll(branch, "..", ".")
	branch = strings.Trim(branch, "/.")
	// 防止以 - 开头被 git 解析为选项（参数注入）。
	branch = strings.TrimLeft(branch, "-")
	if branch == "" {
		return "review-service/fix"
	}
	return branch
}
